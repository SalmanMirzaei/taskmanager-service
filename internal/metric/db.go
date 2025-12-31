package metric

import (
	"database/sql"

	"github.com/jmoiron/sqlx"
)

// UpdateTasksCountFromDB queries the tasks table and updates the TasksCount gauge.
// It returns any underlying error from the DB query.
func UpdateTasksCountFromDB(db *sqlx.DB) error {
	var count int
	if err := db.Get(&count, "SELECT count(1) FROM tasks"); err != nil {
		if err == sql.ErrNoRows {
			SetTasksCount(0)
			return nil
		}
		return err
	}
	SetTasksCount(count)
	return nil
}
