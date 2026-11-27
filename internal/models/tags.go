package models

import "github.com/uptrace/bun"

// StatusTag represents an ordered column/stage in the task pipeline
type StatusTag struct {
	bun.BaseModel `bun:"table:status_tags,alias:st"`

	ID         string `json:"id" bun:"id,pk"`
	Name       string `json:"name" bun:"name,notnull,unique"`
	Color      string `json:"color,omitempty" bun:"-"`
	SortOrder  int    `json:"sort_order" bun:"sort_order,default:0"`
	IsSystem   bool   `json:"is_system" bun:"is_system,default:false"`
	IsInitial  bool   `json:"is_initial" bun:"is_initial,default:false"`
	IsTerminal bool   `json:"is_terminal" bun:"is_terminal,default:false"`
}

// Priority Enum types
type Priority string

const (
	PriorityLow    Priority = "low"
	PriorityMedium Priority = "medium"
	PriorityHigh   Priority = "high"
	PriorityUrgent Priority = "urgent"
)

// IsValid checks Priority is a valid Enum type
func (p Priority) IsValid() bool {
	switch p {
	case PriorityLow, PriorityMedium, PriorityHigh, PriorityUrgent:
		return true
	default:
		return false
	}
}

// PriorityWeight returns a numeric value for sorting or comparisons
func (p Priority) PriorityWeight() int {
	switch p {
	case PriorityLow:
		return 1
	case PriorityMedium:
		return 2
	case PriorityHigh:
		return 3
	case PriorityUrgent:
		return 4
	default:
		return 0 //Default to low if invalid
	}
}
