package utils

import (
	"errors"
	"net/http"
	"strconv"
)

const (
	DefaultLimit = 100
	MaxLimit     = 1000
)

// ParsePagination extracts and validates skip and limit query parameters from an HTTP request.
func ParsePagination(r *http.Request) (skip int64, limit int64, err error) {
	skip = 0
	if skipStr := r.URL.Query().Get("skip"); skipStr != "" {
		parsed, err := strconv.Atoi(skipStr)
		if err != nil || parsed < 0 {
			return 0, 0, errors.New("invalid skip parameter")
		}
		skip = int64(parsed)
	}

	limit = DefaultLimit
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		parsed, err := strconv.Atoi(limitStr)
		if err != nil || parsed <= 0 {
			return 0, 0, errors.New("invalid limit parameter")
		}
		if parsed > MaxLimit {
			parsed = MaxLimit
		}
		limit = int64(parsed)
	}

	return skip, limit, nil
}
