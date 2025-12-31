package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	dtos "taskmanager/internal/model/DTOs"
	"taskmanager/internal/repositories"
	"taskmanager/internal/service"
)

// TaskHandler holds dependencies for HTTP handlers.
type TaskHandler struct {
	svc service.TaskService
}

// NewTaskHandler creates a new TaskHandler.
func NewTaskHandler(s service.TaskService) *TaskHandler {
	return &TaskHandler{svc: s}
}

// CreateTask handles POST /tasks
func (h *TaskHandler) CreateTask(c *gin.Context) {
	var dto dtos.CreateTaskDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	// Convert DTO to model and then call service
	tmodel := dto.ToModel()

	ctx := c.Request.Context()
	task, err := h.svc.Create(ctx, tmodel)
	if err != nil {
		// service returns ErrInvalidInput for validation problems
		if errors.Is(err, service.ErrInvalidInput) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid input"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}

	c.JSON(http.StatusCreated, task)
}

// ListTasks handles GET /tasks
// Supports query params: limit, offset, completed, assignee
func (h *TaskHandler) ListTasks(c *gin.Context) {
	limit := 100
	offset := 0

	if s := c.Query("limit"); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v > 0 {
			limit = v
		}
	}
	if s := c.Query("offset"); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v >= 0 {
			offset = v
		}
	}

	var completed *bool
	if s := c.Query("completed"); s != "" {
		if v, err := strconv.ParseBool(s); err == nil {
			completed = &v
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid completed query param"})
			return
		}
	}

	var assignee *string
	if s := c.Query("assignee"); s != "" {
		assignee = &s
	}

	ctx := c.Request.Context()
	items, total, err := h.svc.List(ctx, limit, offset, completed, assignee)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list tasks"})
		return
	}

	// Include pagination metadata in the response and X-Total-Count header for clients.
	c.Header("X-Total-Count", strconv.Itoa(total))
	c.JSON(http.StatusOK, gin.H{
		"items":  items,
		"limit":  limit,
		"offset": offset,
		"total":  total,
	})
}

// GetTask handles GET /tasks/:id
func (h *TaskHandler) GetTask(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing id"})
		return
	}

	ctx := c.Request.Context()
	t, err := h.svc.GetByID(ctx, id)
	if err != nil {
		// map repository not-found to 404
		if errors.Is(err, repositories.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch task"})
		return
	}
	c.JSON(http.StatusOK, t)
}

// UpdateTask handles PUT /tasks/:id
// Now accepts partial update for assignee as well.
func (h *TaskHandler) UpdateTask(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing id"})
		return
	}
	var dto dtos.UpdateTaskDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	// Convert DTO to model and call service.Update with the partial Task.
	tmodel := dto.ToModel(id)
	ctx := c.Request.Context()
	updated, err := h.svc.Update(ctx, tmodel)
	if err != nil {
		if errors.Is(err, service.ErrInvalidInput) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid input"})
			return
		}
		if errors.Is(err, repositories.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update task"})
		return
	}

	c.JSON(http.StatusOK, updated)
}

// DeleteTask handles DELETE /tasks/:id
func (h *TaskHandler) DeleteTask(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing id"})
		return
	}

	ctx := c.Request.Context()
	if err := h.svc.Delete(ctx, id); err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete task"})
		return
	}

	c.Status(http.StatusNoContent)
}
