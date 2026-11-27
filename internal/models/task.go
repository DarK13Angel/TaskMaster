package models

import (
	"time"

	"github.com/uptrace/bun"
)

// Task represents a entry in Project
type Task struct {
	bun.BaseModel `bun:"table:tasks,alias:t"`

	ID           string     `json:"id" bun:"id,pk"`
	ProjectID    string     `json:"project_id" bun:"project_id,notnull"`
	Title        string     `json:"title" bun:"title,notnull"`
	Notes        string     `json:"notes,omitempty" bun:"notes"`
	StatusID     string     `json:"status_id" bun:"status_id,notnull"`
	Priority     Priority   `json:"priority" bun:"priority,notnull,default:'low'"`
	DueDate      *time.Time `json:"due_date,omitempty" bun:"due_date,nullzero"`
	CreatedAt    time.Time  `json:"created_at" bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt    time.Time  `json:"updated_at" bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	CompletedAt  *time.Time `json:"completed_at,omitempty" bun:"completed_at,nullzero"`
	StatusName   string     `json:"status_name,omitempty" bun:"status_name,scanonly"`
	PriorityName string     `json:"priority_name,omitempty" bun:"priority_name,scanonly"`
	ProjectName  string     `json:"project_name,omitempty" bun:"project_name,scanonly"`

	// Subtasks metadata
	TotalSubtasks int       `json:"total_subtasks" bun:"total_subtasks,scanonly"`
	DoneSubtasks  int       `json:"done_subtasks" bun:"done_subtasks,scanonly"`
	Subtasks      []Subtask `json:"subtasks,omitempty" bun:"rel:has-many,join:id=task_id"`
}

// Subtask represents a to-do item within a task
type Subtask struct {
	bun.BaseModel `bun:"table:subtasks,alias:sbt"`

	ID        string `json:"id" bun:"id,pk"`
	TaskID    string `json:"task_id" bun:"task_id,notnull"`
	Title     string `json:"title" bun:"title,notnull"`
	IsDone    bool   `json:"is_done" bun:"is_done,default:false"`
	SortOrder int    `json:"sort_order" bun:"sort_order"`
}

// MoveTaskRequest - moves task from one task to another
type MoveTaskRequest struct {
	TargetStatusID string `json:"target_status_id"`
}
