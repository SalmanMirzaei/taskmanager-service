package service

import (
	"context"
	"errors"
	"strings"

	"github.com/redis/go-redis/v9"

	"taskmanager/internal/metric"
	"taskmanager/internal/model"
	"taskmanager/internal/repositories"
)

var ErrInvalidInput = errors.New("invalid input")

// TaskService defines business-logic operations for tasks.
type TaskService interface {
	Create(ctx context.Context, task *model.Task) (*model.Task, error)

	GetByID(ctx context.Context, id string) (*model.Task, error)

	List(ctx context.Context, limit, offset int, completed *bool, assignee *string) ([]model.Task, int, error)

	Update(ctx context.Context, task *model.Task) (*model.Task, error)

	Delete(ctx context.Context, id string) error
	Count(ctx context.Context) (int, error)

	SetCacheClient(rdb *redis.Client)
}

type taskService struct {
	repo repositories.TaskRepository
}

func NewTaskService(repo repositories.TaskRepository) TaskService {
	return &taskService{repo: repo}
}

func (s *taskService) SetCacheClient(rdb *redis.Client) {
	s.repo.SetCacheClient(rdb)
}

func (s *taskService) Create(ctx context.Context, task *model.Task) (*model.Task, error) {
	task.Title = strings.TrimSpace(task.Title)
	if task.Title == "" {
		return nil, ErrInvalidInput
	}

	if err := s.repo.Create(task); err != nil {
		return nil, err
	}

	// Update metrics
	metric.IncTaskCount()

	return task, nil
}

func (s *taskService) GetByID(ctx context.Context, id string) (*model.Task, error) {
	t, err := s.repo.GetByID(id)
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (s *taskService) List(ctx context.Context, limit, offset int, completed *bool, assignee *string) ([]model.Task, int, error) {
	tasks, err := s.repo.List(limit, offset, completed, assignee)
	if err != nil {
		return nil, 0, err
	}
	total, err := s.repo.CountFiltered(completed, assignee)
	if err != nil {
		return nil, 0, err
	}
	return tasks, total, nil
}

func (s *taskService) Update(ctx context.Context, task *model.Task) (*model.Task, error) {
	t, err := s.repo.GetByID(task.ID)
	if err != nil {
		return nil, err
	}

	if task.Title != "" {
		tt := strings.TrimSpace(task.Title)
		if tt == "" {
			return nil, ErrInvalidInput
		}
		t.Title = tt
	}

	if err := s.repo.Update(t); err != nil {
		return nil, err
	}
	updated, err := s.repo.GetByID(task.ID)
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func (s *taskService) Delete(ctx context.Context, id string) error {
	ok, err := s.repo.Delete(id)
	if err != nil {
		return err
	}
	if !ok {
		return repositories.ErrNotFound
	}

	// Update metrics
	metric.DecTaskCount()

	return nil
}

func (s *taskService) Count(ctx context.Context) (int, error) {
	return s.repo.Count()
}
