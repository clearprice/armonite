package main

import (
	"time"
)

func (a *Agent) startProgressDisplay() {
	// Simple progress display - just log periodically
	ticker := time.NewTicker(10 * time.Second)
	go func() {
		defer ticker.Stop()
		for range ticker.C {
			a.mu.RLock()
			testStarted := a.testStarted
			running := a.running
			a.mu.RUnlock()

			if testStarted && running {
				a.metrics.mu.Lock()
				requests := a.metrics.Requests
				errors := a.metrics.Errors
				a.metrics.mu.Unlock()

				LogInfo("Progress: %d requests, %d errors", requests, errors)
			}
		}
	}()
}
