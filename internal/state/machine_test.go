package state

import (
	"testing"

	"taskmaster/internal/models"
)

func TestValidateCreation(t *testing.T) {
	initialTag := models.StatusTag{Name: "ToDo", IsInitial: true}
	if err := ValidateCreation(initialTag); err != nil {
		t.Errorf("expected nil error for initial status, got: %v", err)
	}

	nonInitialTag := models.StatusTag{Name: "Done", IsInitial: false}
	if err := ValidateCreation(nonInitialTag); err == nil {
		t.Errorf("expected error for non-initial status, got nil")
	}
}

func TestValidateTransition(t *testing.T) {
	from := models.StatusTag{Name: "ToDo"}
	to := models.StatusTag{Name: "In Progress"}
	if err := ValidateTransition(from, to); err != nil {
		t.Errorf("expected nil error, got: %v", err)
	}
}
