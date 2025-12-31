package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"

	"taskmanager/internal/model"
)

var ErrNotFound = errors.New("task not found")

// TaskRepository defines DB operations for tasks.
type TaskRepository interface {
	Create(task *model.Task) error
	GetByID(id string) (*model.Task, error)
	List(limit, offset int, completed *bool, assignee *string) ([]model.Task, error)
	Update(task *model.Task) error
	Delete(id string) (bool, error)
	Count() (int, error)
	// CountFiltered returns the number of tasks matching optional filters.
	// If both filters are nil/empty, returns the total count (same as Count()).
	CountFiltered(completed *bool, assignee *string) (int, error)

	// Optional: attach a Redis client for cache-aside behavior
	SetCacheClient(rdb *redis.Client)
}

type taskRepo struct {
	db  *sqlx.DB
	rdb *redis.Client
}

// NewTaskRepository creates a new TaskRepository backed by sqlx.DB.
func NewTaskRepository(db *sqlx.DB) TaskRepository {
	return &taskRepo{db: db}
}

// SetCacheClient attaches a Redis client to the repository to enable cache-aside
// behavior for List() and invalidation on Create/Update/Delete.
func (r *taskRepo) SetCacheClient(rdb *redis.Client) {
	r.rdb = rdb
}

func (r *taskRepo) cacheKeyForList(limit, offset int, completed *bool, assignee *string) string {
	compVal := "any"
	if completed != nil {
		compVal = fmt.Sprintf("%v", *completed)
	}
	assVal := "any"
	if assignee != nil {
		assVal = *assignee
	}
	return fmt.Sprintf("tasks:list:limit=%d:offset=%d:completed=%s:assignee=%s", limit, offset, compVal, assVal)
}

// invalidateListCache removes cached list entries. For simplicity we remove the specific key used,
// and also attempt a simple pattern delete for task lists. If r.rdb is nil, this is a no-op.
func (r *taskRepo) invalidateListCache(ctx context.Context) {
	if r.rdb == nil {
		return
	}
	// It's expensive to scan by pattern in Redis at scale; for MVP we attempt to delete keys with known prefix.
	pattern := "tasks:list:*"
	iter := r.rdb.Scan(ctx, 0, pattern, 100).Iterator()
	for iter.Next(ctx) {
		_ = r.rdb.Del(ctx, iter.Val()).Err()
	}
	// ignore iter.Err() for now
}

// Create inserts a new task and invalidates list caches.
func (r *taskRepo) Create(task *model.Task) error {
	if task == nil {
		return errors.New("task is nil")
	}
	if task.ID == "" {
		task.ID = uuid.New().String()
	}
	now := time.Now().UTC()
	task.CreatedAt = now
	task.UpdatedAt = now

	query := `INSERT INTO tasks (id, title, description, assignee, completed, due_date, created_at, updated_at)
VALUES (:id, :title, :description, :assignee, :completed, :due_date, :created_at, :updated_at)`

	_, err := r.db.NamedExec(query, task)
	if err != nil {
		return err
	}

	// invalidate list cache after create
	r.invalidateListCache(context.Background())
	return nil
}

func (r *taskRepo) GetByID(id string) (*model.Task, error) {
	var t model.Task
	err := r.db.Get(&t, "SELECT id, title, description, assignee, completed, due_date, created_at, updated_at FROM tasks WHERE id = $1", id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &t, nil
}

// List attempts to return a cached result (if Redis client provided) using cache-aside pattern.
// If cache miss or no Redis configured, it queries DB and populates cache.
func (r *taskRepo) List(limit, offset int, completed *bool, assignee *string) ([]model.Task, error) {
	// Attempt cache read first (cache-aside). If Redis client not configured or cache miss,
	// fall back to DB and then populate cache.
	cacheKey := r.cacheKeyForList(limit, offset, completed, assignee)
	if r.rdb != nil {
		if s, err := r.rdb.Get(context.Background(), cacheKey).Result(); err == nil {
			var cached []model.Task
			if jerr := json.Unmarshal([]byte(s), &cached); jerr == nil {
				return cached, nil
			}
		}
	}

	if limit <= 0 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	ctx := context.Background()

	baseSelect := `
SELECT id, title, description, assignee, completed, due_date, created_at, updated_at
FROM tasks
`
	var query string
	var args []interface{}

	if completed == nil && (assignee == nil || *assignee == "") {
		query = baseSelect + " ORDER BY created_at DESC LIMIT $1 OFFSET $2"
		args = []interface{}{limit, offset}
	} else if completed != nil && (assignee == nil || *assignee == "") {
		query = baseSelect + " WHERE completed = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3"
		args = []interface{}{*completed, limit, offset}
	} else if completed == nil && assignee != nil {
		query = baseSelect + " WHERE assignee = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3"
		args = []interface{}{*assignee, limit, offset}
	} else {
		query = baseSelect + " WHERE completed = $1 AND assignee = $2 ORDER BY created_at DESC LIMIT $3 OFFSET $4"
		args = []interface{}{*completed, *assignee, limit, offset}
	}

	var tasks []model.Task
	if err := r.db.Select(&tasks, query, args...); err != nil {
		// If no rows found, return empty slice and total=0
		if err == sql.ErrNoRows {
			return []model.Task{}, nil
		}
		return nil, err
	}

	// Populate cache (repositories only items for backward compatibility with prior cache format)
	// Note: cache key remains the same. We continue to cache items array.
	if r.rdb != nil {
		if b, merr := json.Marshal(tasks); merr == nil {
			_ = r.rdb.Set(ctx, cacheKey, string(b), 60*time.Second).Err()
		}
	}

	return tasks, nil
}

func (r *taskRepo) Update(task *model.Task) error {
	if task == nil {
		return errors.New("task is nil")
	}
	task.UpdatedAt = time.Now()

	query := `UPDATE tasks SET title = :title, description = :description, completed = :completed, due_date = :due_date, updated_at = :updated_at WHERE id = :id`
	res, err := r.db.NamedExec(query, task)
	if err != nil {
		return err
	}
	ra, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if ra == 0 {
		return ErrNotFound
	}

	// invalidate list cache after update
	r.invalidateListCache(context.Background())
	return nil
}

func (r *taskRepo) Delete(id string) (bool, error) {
	res, err := r.db.Exec("DELETE FROM tasks WHERE id = $1", id)
	if err != nil {
		return false, err
	}
	ra, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	deleted := ra > 0

	// invalidate list cache after delete
	if deleted {
		r.invalidateListCache(context.Background())
	}
	return deleted, nil
}

func (r *taskRepo) Count() (int, error) {
	var count int
	if err := r.db.Get(&count, "SELECT count(1) FROM tasks"); err != nil {
		return 0, err
	}
	return count, nil
}

// CountFiltered counts tasks using the same filter semantics as List.
// It supports optional filtering by `completed` and `assignee`.
func (r *taskRepo) CountFiltered(completed *bool, assignee *string) (int, error) {
	var count int
	var err error

	// No filters: simple count
	if completed == nil && (assignee == nil || *assignee == "") {
		err = r.db.Get(&count, "SELECT count(1) FROM tasks")
	} else if completed != nil && (assignee == nil || *assignee == "") {
		// Filter by completed only
		err = r.db.Get(&count, "SELECT count(1) FROM tasks WHERE completed = $1", *completed)
	} else if completed == nil && assignee != nil {
		// Filter by assignee only
		err = r.db.Get(&count, "SELECT count(1) FROM tasks WHERE assignee = $1", *assignee)
	} else {
		// Both filters present
		err = r.db.Get(&count, "SELECT count(1) FROM tasks WHERE completed = $1 AND assignee = $2", *completed, *assignee)
	}

	if err != nil {
		return 0, err
	}
	return count, nil
}
