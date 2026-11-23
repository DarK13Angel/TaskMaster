package models

// StatusTag represents an ordered column/stage in the task pipeline
type StatusTag struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Color      string `json:"color,omitempty"`
	SortOrder  int    `json:"sort_order"`
	IsSystem   bool   `json:"is_system"`
	IsInitial  bool   `json:"is_initial"`
	IsTerminal bool   `json:"is_terminal"`
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
