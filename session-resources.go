package piai

import (
	"sync"
)

var (
	sessionCleanups []func(sessionID string) error
	sessionMu       sync.RWMutex
)

// RegisterSessionResourceCleanup registers a cleanup callback for session resources.
func RegisterSessionResourceCleanup(cleanup func(sessionID string) error) {
	sessionMu.Lock()
	defer sessionMu.Unlock()
	sessionCleanups = append(sessionCleanups, cleanup)
}

// CleanupSessionResources invokes all registered cleanup callbacks.
func CleanupSessionResources(sessionID string) error {
	sessionMu.RLock()
	// Copy to avoid holding lock during cleanup
	cleanups := make([]func(string) error, len(sessionCleanups))
	copy(cleanups, sessionCleanups)
	sessionMu.RUnlock()

	var errs []error
	for _, cleanup := range cleanups {
		if err := cleanup(sessionID); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errs[0] // Return first error
	}
	return nil
}
