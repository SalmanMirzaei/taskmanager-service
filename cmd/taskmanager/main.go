package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"

	"taskmanager/internal/handler"
	"taskmanager/internal/metric"
	"taskmanager/internal/repositories"
	"taskmanager/internal/service"
	"taskmanager/migrations"
)

func main() {

	// Init Metrics
	metric.InitMetrics()

	// Configuration via environment variables
	dbURL := getenv("DATABASE_URL", "")
	redisAddr := getenv("REDIS_ADDR", "")
	port := getenv("PORT", "8080")

	// Connect to Postgres
	db, err := sqlx.Connect("postgres", dbURL)
	if err != nil {
		log.Fatalf("unable to connect to database: %v", err)
	}
	defer db.Close()

	// Ensure schema (migration-lite)
	if err := migrations.EnsureSchema(db); err != nil {
		log.Fatalf("failed to ensure schema: %v", err)
	}

	// initialize tasks_count metric
	if err := metric.UpdateTasksCountFromDB(db); err != nil {
		log.Printf("warning: failed to update tasks_count metric: %v", err)
	}

	// Repository / Service / Handler wiring
	repo := repositories.NewTaskRepository(db)

	// Redis cache-aside for list endpoints
	// Accepts REDIS_ADDR like "localhost:6379" or "redis://localhost:6379"
	redisAddr = strings.TrimPrefix(redisAddr, "redis://")
	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Printf("redis not available at %s: %v — continuing without cache", redisAddr, err)
	} else {
		// attach to repository (repo will no-op if not supported)
		repo.SetCacheClient(rdb)
		log.Printf("redis cache enabled (addr=%s)", redisAddr)
	}

	svc := service.NewTaskService(repo)
	// If service exposes SetCacheClient, forward rdb (service will call repo.SetCacheClient)
	// Note: service interface in this codebase implements SetCacheClient on the struct method.
	// Use type assertion to avoid compile-time dependency on optional method
	if ss, ok := svc.(interface{ SetCacheClient(*redis.Client) }); ok {
		ss.SetCacheClient(rdb)
	}

	h := handler.NewTaskHandler(svc)

	// Gin router setup
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(gin.Logger())
	r.Use(metric.PrometheusMiddleware())

	// Health
	r.GET("/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })

	// Prometheus metrics
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Serve OpenAPI spec and minimal Swagger UI
	r.StaticFile("/docs/openapi.yaml", "/app/docs/openapi.yaml")
	r.GET("/docs", func(c *gin.Context) { c.File("/app/docs/swagger.html") })

	// API v1
	api := r.Group("/api/v1")
	{
		api.POST("/tasks", h.CreateTask)
		api.GET("/tasks", h.ListTasks)
		api.GET("/tasks/:id", h.GetTask)
		api.PUT("/tasks/:id", h.UpdateTask)
		api.DELETE("/tasks/:id", h.DeleteTask)
	}

	addr := fmt.Sprintf(":%s", port)
	log.Printf("starting server on %s", addr)
	log.Printf("OpenAPI UI available at http://localhost%s/docs", addr)
	if err := r.Run(addr); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server exited: %v", err)
	}
}

// getenv returns environment variable or defaultVal
func getenv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
