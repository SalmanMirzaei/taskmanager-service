package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"taskmanager/internal/handler"
	"taskmanager/internal/model"
	"taskmanager/internal/repositories"
	"taskmanager/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

type inMemoryRepo struct {
	mu sync.Mutex
	m  map[string]model.Task
}

func newInMemoryRepo() *inMemoryRepo { return &inMemoryRepo{m: make(map[string]model.Task)} }

func (r *inMemoryRepo) Create(task *model.Task) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if task.ID == "" {
		task.ID = "id-" + task.Title
	}
	r.m[task.ID] = *task
	return nil
}
func (r *inMemoryRepo) GetByID(id string) (*model.Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.m[id]
	if !ok {
		return nil, repositories.ErrNotFound
	}
	return &t, nil
}
func (r *inMemoryRepo) List(limit, offset int, completed *bool, assignee *string) ([]model.Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]model.Task, 0, len(r.m))
	for _, v := range r.m {
		out = append(out, v)
	}
	return out, nil
}
func (r *inMemoryRepo) Update(task *model.Task) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.m[task.ID]; !ok {
		return repositories.ErrNotFound
	}
	r.m[task.ID] = *task
	return nil
}
func (r *inMemoryRepo) Delete(id string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.m[id]; !ok {
		return false, nil
	}
	delete(r.m, id)
	return true, nil
}
func (r *inMemoryRepo) Count() (int, error) { r.mu.Lock(); defer r.mu.Unlock(); return len(r.m), nil }
func (r *inMemoryRepo) CountFiltered(completed *bool, assignee *string) (int, error) {
	return r.Count()
}
func (r *inMemoryRepo) SetCacheClient(_ *redis.Client) {}

func TestHandlers_EndToEnd(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := newInMemoryRepo()
	svc := service.NewTaskService(repo)
	h := handler.NewTaskHandler(svc)

	r := gin.New()
	r.POST("/tasks", h.CreateTask)
	r.GET("/tasks", h.ListTasks)
	r.GET("/tasks/:id", h.GetTask)
	r.PUT("/tasks/:id", h.UpdateTask)
	r.DELETE("/tasks/:id", h.DeleteTask)

	ts := httptest.NewServer(r)
	defer ts.Close()

	// Create
	createBody := map[string]string{"title": "task1"}
	cb, _ := json.Marshal(createBody)
	res, err := http.Post(ts.URL+"/tasks", "application/json", bytes.NewReader(cb))
	if err != nil {
		t.Fatalf("create req err: %v", err)
	}
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 got %d", res.StatusCode)
	}
	var created model.Task
	_ = json.NewDecoder(res.Body).Decode(&created)

	// List
	res, err = http.Get(ts.URL + "/tasks")
	if err != nil {
		t.Fatalf("list req err: %v", err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 got %d", res.StatusCode)
	}
	var listResp struct {
		Items []model.Task `json:"items"`
	}
	_ = json.NewDecoder(res.Body).Decode(&listResp)
	if len(listResp.Items) == 0 {
		t.Fatalf("expected items in list")
	}

	// Get
	res, err = http.Get(ts.URL + "/tasks/" + created.ID)
	if err != nil {
		t.Fatalf("get req err: %v", err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 got %d", res.StatusCode)
	}

	// Update
	updBody := map[string]string{"title": "task1-upd"}
	ub, _ := json.Marshal(updBody)
	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/tasks/"+created.ID, bytes.NewReader(ub))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("update req err: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 got %d", resp.StatusCode)
	}

	// Delete
	req, _ = http.NewRequest(http.MethodDelete, ts.URL+"/tasks/"+created.ID, nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("delete req err: %v", err)
	}
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 204 or 200 got %d", resp.StatusCode)
	}

	// Confirm deleted
	res, err = http.Get(ts.URL + "/tasks/" + created.ID)
	if err != nil {
		t.Fatalf("get after delete err: %v", err)
	}
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 got %d", res.StatusCode)
	}
}
