package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"taskmanager/internal/model"
	"taskmanager/internal/repositories"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// fakeService implements service.TaskService for handler tests.
type fakeService struct {
	createFn func(ctx context.Context, task *model.Task) (*model.Task, error)
	listFn   func(ctx context.Context, limit, offset int, completed *bool, assignee *string) ([]model.Task, int, error)
	getFn    func(ctx context.Context, id string) (*model.Task, error)
	updateFn func(ctx context.Context, task *model.Task) (*model.Task, error)
	deleteFn func(ctx context.Context, id string) error
	countFn  func(ctx context.Context) (int, error)
}

func (f *fakeService) Create(ctx context.Context, task *model.Task) (*model.Task, error) {
	return f.createFn(ctx, task)
}
func (f *fakeService) GetByID(ctx context.Context, id string) (*model.Task, error) {
	return f.getFn(ctx, id)
}
func (f *fakeService) List(ctx context.Context, limit, offset int, completed *bool, assignee *string) ([]model.Task, int, error) {
	return f.listFn(ctx, limit, offset, completed, assignee)
}
func (f *fakeService) Update(ctx context.Context, task *model.Task) (*model.Task, error) {
	return f.updateFn(ctx, task)
}
func (f *fakeService) Delete(ctx context.Context, id string) error { return f.deleteFn(ctx, id) }
func (f *fakeService) Count(ctx context.Context) (int, error)      { return f.countFn(ctx) }
func (f *fakeService) SetCacheClient(_ *redis.Client)              {}

func TestTaskHandler_Group(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svc := &fakeService{
		createFn: func(ctx context.Context, task *model.Task) (*model.Task, error) {
			task.ID = "id-1"
			return task, nil
		},
		listFn: func(ctx context.Context, limit, offset int, completed *bool, assignee *string) ([]model.Task, int, error) {
			return []model.Task{{ID: "id-1", Title: "t1"}}, 1, nil
		},
		getFn: func(ctx context.Context, id string) (*model.Task, error) {
			return &model.Task{ID: id, Title: "t1"}, nil
		},
		updateFn: func(ctx context.Context, task *model.Task) (*model.Task, error) {
			return &model.Task{ID: task.ID, Title: "updated"}, nil
		},
		deleteFn: func(ctx context.Context, id string) error { return nil },
		countFn:  func(ctx context.Context) (int, error) { return 1, nil },
	}

	h := NewTaskHandler(svc)

	t.Run("Create_Success", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		body := `{"title":"hello"}`
		c.Request = httptest.NewRequest(http.MethodPost, "/tasks", strings.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		h.CreateTask(c)
		if w.Code != http.StatusCreated {
			t.Fatalf("expected 201 got %d body=%s", w.Code, w.Body.String())
		}
	})

	t.Run("List_Success", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/tasks", nil)
		h.ListTasks(c)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 got %d", w.Code)
		}
	})

	t.Run("Get_NotFound", func(t *testing.T) {
		svc.getFn = func(ctx context.Context, id string) (*model.Task, error) { return nil, repositories.ErrNotFound }
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: "missing"}}
		c.Request = httptest.NewRequest(http.MethodGet, "/tasks/missing", nil)
		h.GetTask(c)
		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404 got %d", w.Code)
		}
	})

	t.Run("Update_BadID", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: ""}}
		c.Request = httptest.NewRequest(http.MethodPut, "/tasks/", nil)
		h.UpdateTask(c)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 got %d", w.Code)
		}
	})

	t.Run("Delete_Success", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: "id-1"}}
		c.Request = httptest.NewRequest(http.MethodDelete, "/tasks/id-1", nil)
		h.DeleteTask(c)
		if w.Code != http.StatusNoContent && w.Code != http.StatusOK {
			t.Fatalf("expected 204 or 200 got %d", w.Code)
		}
	})
}
