package dtos

import (
	"taskmanager/internal/model"
	"time"
)

type UpdateTaskDTO struct {
	Title       *string    `json:"title,omitempty"`
	Description *string    `json:"description,omitempty"`
	Assignee    *string    `json:"assignee,omitempty"`
	Completed   *bool      `json:"completed,omitempty"`
	DueDate     *time.Time `json:"due_date,omitempty"`
}

// Only fields that are non-nil in the DTO will be applied on the returned Task (nullable
// fields are represented using model helpers).
func (d *UpdateTaskDTO) ToModel(id string) *model.Task {
	t := &model.Task{ID: id}
	if d.Title != nil {
		t.Title = *d.Title
	}
	if d.Description != nil {
		if *d.Description == "" {
			t.ClearDescription()
		} else {
			t.SetDescription(*d.Description)
		}
	}
	if d.Assignee != nil {
		if *d.Assignee != "" {
			t.SetAssignee(*d.Assignee)
		}
	}
	if d.Completed != nil {
		t.Completed = *d.Completed
	}
	if d.DueDate != nil {
		t.SetDueDate(*d.DueDate)
	}
	return t
}
