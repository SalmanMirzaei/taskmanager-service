package metric

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	RequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "requests_total",
			Help: "Total number of HTTP requests processed, labeled by method, path and status",
		},
		[]string{"method", "path", "status"},
	)

	RequestLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "request_latency_seconds",
			Help:    "Histogram of request latencies labeled by method and path",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	TasksCount = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "tasks_count",
			Help: "Current number of tasks in the database",
		},
	)
)

// InitMetrics registers the Prometheus metrics. Call once at program startup.
func InitMetrics() {
	prometheus.MustRegister(RequestsTotal, RequestLatency, TasksCount)
}

// PrometheusMiddleware returns a Gin middleware that instruments requests.
// It records request count and latency (method + path + status labels).
func PrometheusMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next() // process request
		duration := time.Since(start).Seconds()

		status := strconv.Itoa(c.Writer.Status())
		route := c.FullPath()
		if route == "" {
			route = c.Request.URL.Path
		}

		RequestsTotal.WithLabelValues(c.Request.Method, route, status).Inc()
		RequestLatency.WithLabelValues(c.Request.Method, route).Observe(duration)
	}
}

// PromhttpHandler returns the standard promhttp handler to expose /metrics.
func PromhttpHandler() http.Handler {
	return promhttp.Handler()
}

// SetTasksCount sets the tasks_count gauge to the provided value.
// Exported so application code can update the metric after DB changes.
func SetTasksCount(n int) {
	TasksCount.Set(float64(n))
}

// IncTaskCount increments the tasks_count gauge by 1.
func IncTaskCount() {
	TasksCount.Add(1)
}

// DecTaskCount decrements the tasks_count gauge by 1.
func DecTaskCount() {
	TasksCount.Sub(1)
}
