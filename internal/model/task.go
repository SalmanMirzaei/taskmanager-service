package model

import (
	"database/sql"
	"time"
)

// DB tags match the columns in the tasks table.
// JSON marshaling and unmarshaling are implemented to render nullable DB types
// (sql.NullString/sql.NullTime) as simple JSON values (string / RFC3339 time)
// or null when not present.
type Task struct {
	ID          string         `db:"id" json:"id"`
	Title       string         `db:"title" json:"title"`
	Description sql.NullString `db:"description" json:"description"`
	Assignee    sql.NullString `db:"assignee" json:"assignee"`
	Completed   bool           `db:"completed" json:"completed"`
	DueDate     sql.NullTime   `db:"due_date" json:"due_date"`
	CreatedAt   time.Time      `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time      `db:"updated_at" json:"updated_at"`
}

// SetDescription sets the description value and marks it valid.
func (t *Task) SetDescription(s string) {
	t.Description = sql.NullString{String: s, Valid: true}
}

// ClearDescription clears the description (sets it to null).
func (t *Task) ClearDescription() {
	t.Description = sql.NullString{Valid: false}
}

// SetAssignee sets the assignee value and marks it valid.
func (t *Task) SetAssignee(s string) {
	t.Assignee = sql.NullString{String: s, Valid: true}
}

// ClearAssignee clears the assignee (sets it to null).
func (t *Task) ClearAssignee() {
	t.Assignee = sql.NullString{Valid: false}
}

// SetDueDate sets the due date (repositoriesd in UTC) and marks it valid.
func (t *Task) SetDueDate(dt time.Time) {
	t.DueDate = sql.NullTime{Time: dt.UTC(), Valid: true}
}

// ClearDueDate clears the due date (sets it to null).
func (t *Task) ClearDueDate() {
	t.DueDate = sql.NullTime{Valid: false}
}
