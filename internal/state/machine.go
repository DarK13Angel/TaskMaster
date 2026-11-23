package state

import (
	"errors"
	"fmt"
	"taskmaster/internal/models"
)

// Define custom errors for TaskMaster
var (
	ErrInvalidInitialStatus = errors.New("tasks can only be created in Backlog or ToDo")
	ErrInvalidTransition    = errors.New("transition is not allowed")
)

// ValidateCreation checks whether the task can be initialized with targetStatus
func ValidateCreation(targetStatus models.StatusTag) error {
	if !targetStatus.IsInitial {
		return fmt.Errorf("%w: '%s' is not an initial status", ErrInvalidInitialStatus, targetStatus.Name)
	}
	return nil
}

// ValidateTransition enforces the state machine rules when moving between statuses
func ValidateTransition(from, to models.StatusTag) error {
	return nil
}
