package models

import "time"

// Task represents a entry in Project
type Task struct {
	ID           string     `json:"id"`
	ProjectID    string     `json:"project_id"`
	Title        string     `json:"title"`
	Notes        string     `json:"notes"`
	StatusID     string     `json:"status_id"`
	Priority     Priority   `json:"priority"`
	DueDate      *time.Time `json:"due_date,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
	StatusName   string     `json:"status_name,omitempty"`
	PriorityName string     `json:"priority_name,omitempty"`
	ProjectName  string     `json:"project_name,omitempty"`

	// Subtasks metadata
	TotalSubtasks int       `json:"total_subtasks"`
	DoneSubtasks  int       `json:"done_subtasks"`
	Subtasks      []Subtask `json:"subtasks,omitempty"`
}

// Subtask represents a to-do item within a task
type Subtask struct {
	ID        string `json:"id"`
	TaskID    string `json:"task_id"`
	Title     string `json:"title"`
	IsDone    bool   `json:"is_done"`
	SortOrder int    `json:"sort_order"`
}

// MoveTaskRequest - moves task from one task to another
type MoveTaskRequest struct {
	TargetStatusID string `json:"target_status_id"`
}
