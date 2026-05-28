package piai

import (
	"errors"
	"testing"
)

func TestRegisterAndCleanupSessionResources(t *testing.T) {
	// Reset
	sessionCleanups = nil

	called := false
	RegisterSessionResourceCleanup(func(sessionID string) error {
		called = true
		if sessionID != "test-session" {
			t.Errorf("expected session ID 'test-session', got '%s'", sessionID)
		}
		return nil
	})

	err := CleanupSessionResources("test-session")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("expected cleanup to be called")
	}
}

func TestCleanupSessionResourcesError(t *testing.T) {
	sessionCleanups = nil

	expectedErr := errors.New("cleanup failed")
	RegisterSessionResourceCleanup(func(sessionID string) error {
		return expectedErr
	})

	err := CleanupSessionResources("test")
	if err != expectedErr {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
}

func TestCleanupSessionResourcesMultiple(t *testing.T) {
	sessionCleanups = nil

	callCount := 0
	RegisterSessionResourceCleanup(func(sessionID string) error {
		callCount++
		return nil
	})
	RegisterSessionResourceCleanup(func(sessionID string) error {
		callCount++
		return nil
	})

	CleanupSessionResources("test")
	if callCount != 2 {
		t.Errorf("expected 2 calls, got %d", callCount)
	}
}
