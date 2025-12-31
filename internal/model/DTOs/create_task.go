package dtos

import (
	"taskmanager/internal/model"
	"time"
)

type CreateTaskDTO struct {
	Title       string     `json:"title" binding:"required"`
	Description *string    `json:"description,omitempty"`
	Assignee    *string    `json:"assignee,omitempty"`
	DueDate     *time.Time `json:"due_date,omitempty"`
}

// ToModel converts the DTO into a domain Task ready to be used by services or repos.
func (d *CreateTaskDTO) ToModel() *model.Task {
	t := &model.Task{Title: d.Title}
	if d.Description != nil {
		t.SetDescription(*d.Description)
	}
	if d.Assignee != nil {
		if *d.Assignee != "" {
			t.SetAssignee(*d.Assignee)
		}
	}
	if d.DueDate != nil {
		t.SetDueDate(*d.DueDate)
	}
	return t
}
