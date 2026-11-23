package models

import "time"

type Project struct {
	ID                  string    `json:"id"`
	Name                string    `json:"name"`
	Description         string    `json:"description,omitempty"`
	SortOrder           int       `json:"sort_order"`
	IsArchived          bool      `json:"is_archived"`
	HistoricalDoneCount int       `json:"historical_done_count"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}
