package repositories

import (
	"database/sql"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	redismock "github.com/go-redis/redismock/v9"
	"github.com/jmoiron/sqlx"

	"taskmanager/internal/model"
)

func TestList_CacheHit(t *testing.T) {
	// create sqlmock DB but we won't expect any DB calls for cache-hit
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()
	sx := sqlx.NewDb(db, "sqlmock")

	// redis mock
	rdb, mock := redismock.NewClientMock()
	repo := &taskRepo{db: sx, rdb: rdb}

	tasks := []model.Task{{ID: "t1", Title: "one"}}
	b, _ := json.Marshal(tasks)
	key := repo.cacheKeyForList(100, 0, nil, nil)
	mock.ExpectGet(key).SetVal(string(b))

	got, err := repo.List(100, 0, nil, nil)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(got) != 1 || got[0].ID != "t1" {
		t.Fatalf("unexpected result: %+v", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("redis expectations: %v", err)
	}
}

func TestList_CacheMiss_DBAndSet(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()
	sx := sqlx.NewDb(db, "sqlmock")

	rdb, rmock := redismock.NewClientMock()
	repo := &taskRepo{db: sx, rdb: rdb}

	key := repo.cacheKeyForList(100, 0, nil, nil)
	rmock.ExpectGet(key).RedisNil()

	// expect select - provide non-nil timestamps to satisfy Scan into time.Time
	now := time.Now()
	rows := sqlmock.NewRows([]string{"id", "title", "description", "assignee", "completed", "due_date", "created_at", "updated_at"}).AddRow("t1", "one", nil, nil, false, nil, now, now)
	mock.ExpectQuery("SELECT id, title, description").WillReturnRows(rows)

	got, err := repo.List(100, 0, nil, nil)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(got) != 1 || got[0].ID != "t1" {
		t.Fatalf("unexpected rows: %+v", got)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
	if err := rmock.ExpectationsWereMet(); err != nil {
		t.Fatalf("redis expectations: %v", err)
	}
}

func TestCreate_NilAndSuccess(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()
	sx := sqlx.NewDb(db, "sqlmock")
	repo := &taskRepo{db: sx}

	if err := repo.Create(nil); err == nil {
		t.Fatalf("expected error when task is nil")
	}

	// success path: expect NamedExec insert
	mock.ExpectExec("INSERT INTO tasks").WithArgs(sqlmock.AnyArg(), "t", sqlmock.AnyArg(), sqlmock.AnyArg(), false, sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(1, 1))
	tsk := &model.Task{Title: "t"}
	if err := repo.Create(tsk); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestGetByID_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()
	sx := sqlx.NewDb(db, "sqlmock")
	repo := &taskRepo{db: sx}

	mock.ExpectQuery("SELECT id, title, description").WillReturnError(sql.ErrNoRows)
	_, err = repo.GetByID("missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestUpdate_Delete_NotFound_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()
	sx := sqlx.NewDb(db, "sqlmock")
	repo := &taskRepo{db: sx}

	// Update not found -> RowsAffected 0
	mock.ExpectExec("UPDATE tasks SET").WillReturnResult(sqlmock.NewResult(0, 0))
	if err := repo.Update(&model.Task{ID: "x"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound got %v", err)
	}

	// Delete success
	mock.ExpectExec("DELETE FROM tasks").WillReturnResult(sqlmock.NewResult(1, 1))
	ok, err := repo.Delete("x")
	if err != nil || !ok {
		t.Fatalf("expected deleted got ok=%v err=%v", ok, err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}
