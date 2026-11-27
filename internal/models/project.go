package models

import (
	"time"

	"github.com/uptrace/bun"
)

type Project struct {
	bun.BaseModel `bun:"table:projects,alias:p"`

	ID                  string    `json:"id" bun:"id,pk"`
	Name                string    `json:"name" bun:"name,notnull"`
	Description         string    `json:"description,omitempty" bun:"description"`
	SortOrder           int       `json:"sort_order" bun:"sort_order,default:0"`
	IsArchived          bool      `json:"is_archived" bun:"is_archived,default:false"`
	HistoricalDoneCount int       `json:"historical_done_count" bun:"historical_done_count,default:0"`
	CreatedAt           time.Time `json:"created_at" bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt           time.Time `json:"updated_at" bun:"updated_at,nullzero,notnull,default:current_timestamp"`

	Tasks []Task `json:"tasks,omitempty" bun:"rel:has-many,join:id=project_id"`
}
