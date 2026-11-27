package utils

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// WriteJSON writes a JSON payload with the given HTTP status code.
func WriteJSON(w http.ResponseWriter, status int, v any) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(v)
}

// WriteError writes a standardized JSON error response.
func WriteError(w http.ResponseWriter, status int, message string) {
	_ = WriteJSON(w, status, map[string]string{"error": message})
}

// DecodeJSON decodes the request body into dst.
func DecodeJSON(r *http.Request, dst any) error {
	return json.NewDecoder(r.Body).Decode(dst)
}

// RequirePathID extracts a required path parameter or returns a descriptive error.
func RequirePathID(r *http.Request, key, label string) (string, error) {
	val := strings.TrimSpace(r.PathValue(key))
	if val == "" {
		return "", fmt.Errorf("%s is required", label)
	}
	return val, nil
}
