package test

import (
	"net/http/httptest"
	"testing"

	"taskmaster/internal/utils"
)

func TestParsePagination(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		wantSkip  int64
		wantLimit int64
		wantErr   bool
	}{
		{
			name:      "defaults when omitted",
			url:       "/api/v1/projects",
			wantSkip:  0,
			wantLimit: utils.DefaultLimit,
		},
		{
			name:      "valid skip and limit",
			url:       "/api/v1/projects?skip=20&limit=50",
			wantSkip:  20,
			wantLimit: 50,
		},
		{
			name:      "limit capped at MaxLimit",
			url:       "/api/v1/projects?skip=0&limit=5000",
			wantSkip:  0,
			wantLimit: utils.MaxLimit,
		},
		{
			name:    "negative skip",
			url:     "/api/v1/projects?skip=-1",
			wantErr: true,
		},
		{
			name:    "non-numeric skip",
			url:     "/api/v1/projects?skip=abc",
			wantErr: true,
		},
		{
			name:    "zero limit",
			url:     "/api/v1/projects?limit=0",
			wantErr: true,
		},
		{
			name:    "negative limit",
			url:     "/api/v1/projects?limit=-5",
			wantErr: true,
		},
		{
			name:    "non-numeric limit",
			url:     "/api/v1/projects?limit=xyz",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tc.url, nil)
			skip, limit, err := utils.ParsePagination(req)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (skip=%d, limit=%d)", skip, limit)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if skip != tc.wantSkip || limit != tc.wantLimit {
				t.Fatalf("got skip=%d limit=%d, want skip=%d limit=%d", skip, limit, tc.wantSkip, tc.wantLimit)
			}
		})
	}
}
