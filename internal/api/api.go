package api

import (
	"encoding/json"
	"net/http"
	"taskmaster/internal/db"
)

type Server struct {
	DB *db.DB
}

func NewServer(database *db.DB) *Server {
	return &Server{DB: database}
}

// writeJson write struct or map JSON with given status code
func writeJSON(w http.ResponseWriter, status int, v any) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	return json.NewEncoder(w).Encode(v)
}

// writeError Standardise Err response
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// enableCors allows local Vue FE to talk to API without CORS issues
func enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Routes registers all API endpoints and applies CORS middleware
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("GET /api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "app": "TaskMaster"})
	})

	// Status Tags & Priorities
	mux.HandleFunc("GET /api/v1/status-tags", s.ListStatusTags)
	mux.HandleFunc("GET /api/v1/priorities", s.ListPriorities)

	// Projects
	mux.HandleFunc("GET /api/v1/projects", s.ListProjects)
	mux.HandleFunc("POST /api/v1/projects", s.CreateProject)
	mux.HandleFunc("DELETE /api/v1/projects/{id}", s.DeleteProject)

	// Tasks
	mux.HandleFunc("GET /api/v1/tasks", s.ListTasks)
	mux.HandleFunc("POST /api/v1/tasks", s.CreateTask)
	mux.HandleFunc("POST /api/v1/tasks/{id}/move", s.MoveTask)
	mux.HandleFunc("DELETE /api/v1/tasks/{id}", s.DeleteTask)

	return enableCORS(mux)
}
