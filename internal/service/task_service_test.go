package service

import (
	"errors"
	"testing"

	"github.com/redis/go-redis/v9"
	"taskmanager/internal/model"
	"taskmanager/internal/repositories"
)

type fakeRepo struct {
	createFn        func(task *model.Task) error
	getFn           func(id string) (*model.Task, error)
	listFn          func(limit, offset int, completed *bool, assignee *string) ([]model.Task, error)
	countFn         func() (int, error)
	countFilteredFn func(completed *bool, assignee *string) (int, error)
	updateFn        func(task *model.Task) error
	deleteFn        func(id string) (bool, error)
}

func (f *fakeRepo) Create(task *model.Task) error          { return f.createFn(task) }
func (f *fakeRepo) GetByID(id string) (*model.Task, error) { return f.getFn(id) }
func (f *fakeRepo) List(limit, offset int, completed *bool, assignee *string) ([]model.Task, error) {
	return f.listFn(limit, offset, completed, assignee)
}
func (f *fakeRepo) Update(task *model.Task) error  { return f.updateFn(task) }
func (f *fakeRepo) Delete(id string) (bool, error) { return f.deleteFn(id) }
func (f *fakeRepo) Count() (int, error)            { return f.countFn() }
func (f *fakeRepo) CountFiltered(completed *bool, assignee *string) (int, error) {
	return f.countFilteredFn(completed, assignee)
}
func (f *fakeRepo) SetCacheClient(_ *redis.Client) {}

func TestTaskService_CreateAndValidation(t *testing.T) {
	repo := &fakeRepo{
		createFn: func(task *model.Task) error { return nil },
	}
	svc := NewTaskService(repo)

	// success
	t.Run("Create_Success", func(t *testing.T) {
		m := &model.Task{Title: "  hello "}
		got, err := svc.Create(nil, m)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if got.Title != "hello" {
			t.Fatalf("expected trimmed title; got %q", got.Title)
		}
	})

	// invalid input
	t.Run("Create_Invalid", func(t *testing.T) {
		_, err := svc.Create(nil, &model.Task{Title: "   "})
		if !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("expected ErrInvalidInput got %v", err)
		}
	})
}

func TestTaskService_UpdateAndDelete(t *testing.T) {
	// prepare repo behavior
	repo := &fakeRepo{}
	repo.getFn = func(id string) (*model.Task, error) {
		if id == "exists" {
			return &model.Task{ID: "exists", Title: "old"}, nil
		}
		return nil, repositories.ErrNotFound
	}
	repo.updateFn = func(task *model.Task) error { return nil }
	repo.deleteFn = func(id string) (bool, error) {
		if id == "exists" {
			return true, nil
		}
		return false, nil
	}
	svc := NewTaskService(repo)

	t.Run("Update_Success", func(t *testing.T) {
		updated, err := svc.Update(nil, &model.Task{ID: "exists", Title: "new"})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if updated == nil {
			t.Fatalf("expected updated task")
		}
	})

	t.Run("Update_NotFound", func(t *testing.T) {
		_, err := svc.Update(nil, &model.Task{ID: "missing", Title: "new"})
		if !errors.Is(err, repositories.ErrNotFound) {
			t.Fatalf("expected not found got %v", err)
		}
	})

	t.Run("Delete_Success", func(t *testing.T) {
		if err := svc.Delete(nil, "exists"); err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
	})

	t.Run("Delete_NotFound", func(t *testing.T) {
		if err := svc.Delete(nil, "missing"); !errors.Is(err, repositories.ErrNotFound) {
			t.Fatalf("expected not found got %v", err)
		}
	})
}

func TestTaskService_List(t *testing.T) {
	repo := &fakeRepo{
		listFn: func(limit, offset int, completed *bool, assignee *string) ([]model.Task, error) {
			return []model.Task{{ID: "a"}}, nil
		},
		countFilteredFn: func(completed *bool, assignee *string) (int, error) { return 1, nil },
	}
	svc := NewTaskService(repo)
	items, total, err := svc.List(nil, 10, 0, nil, nil)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("expected one item and total=1")
	}
}
