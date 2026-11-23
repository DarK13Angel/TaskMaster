package api

import (
	"net/http"
	"taskmaster/internal/models"
)

// ListStatusTags returns all pipeline status columns ordered by sort_order
func (s *Server) ListStatusTags(w http.ResponseWriter, r *http.Request) {
	rows, err := s.DB.Query(`
		SELECT id, name, sort_order, is_system, is_initial, is_terminal
		FROM status_tags
		ORDER BY sort_order ASC
	`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to query status tags")
		return
	}
	defer rows.Close()

	tags := []models.StatusTag{}
	for rows.Next() {
		var tag models.StatusTag
		if err := rows.Scan(&tag.ID, &tag.Name, &tag.SortOrder, &tag.IsSystem, &tag.IsInitial, &tag.IsTerminal); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to scan status tag")
			return
		}
		tags = append(tags, tag)
	}

	writeJSON(w, http.StatusOK, tags)
}

// ListPriorities returns the available priority options
func (s *Server) ListPriorities(w http.ResponseWriter, r *http.Request) {
	priorities := []models.Priority{
		models.PriorityLow,
		models.PriorityMedium,
		models.PriorityHigh,
		models.PriorityUrgent,
	}
	writeJSON(w, http.StatusOK, priorities)
}
