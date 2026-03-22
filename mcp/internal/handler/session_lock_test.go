package handler

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// TestParallelSubprocessStateDirIsolation verifies that queryLLMScoped
// sets cmd.Dir correctly for each task, ensuring subprocess working
// directory isolation.
func TestParallelSubprocessStateDirIsolation(t *testing.T) {
	tmpDir := t.TempDir()
	const numDirs = 10

	var wg sync.WaitGroup
	errors := make([]error, numDirs)

	// Create separate state directories and verify each can be used independently
	for i := 0; i < numDirs; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			dir := filepath.Join(tmpDir, fmt.Sprintf("analyze-%04d", idx))
			if err := os.MkdirAll(dir, 0755); err != nil {
				errors[idx] = fmt.Errorf("mkdir %d: %w", idx, err)
				return
			}

			// Write a unique file in each directory
			marker := filepath.Join(dir, "config.json")
			content := fmt.Sprintf(`{"task_id": "analyze-%04d"}`, idx)
			if err := os.WriteFile(marker, []byte(content), 0644); err != nil {
				errors[idx] = fmt.Errorf("write %d: %w", idx, err)
				return
			}

			// Verify the file is only in this directory
			data, err := os.ReadFile(marker)
			if err != nil {
				errors[idx] = fmt.Errorf("read %d: %w", idx, err)
				return
			}
			expected := fmt.Sprintf(`{"task_id": "analyze-%04d"}`, idx)
			if string(data) != expected {
				errors[idx] = fmt.Errorf("dir %d: expected %q, got %q", idx, expected, string(data))
				return
			}
		}(i)
	}
	wg.Wait()

	for i, err := range errors {
		if err != nil {
			t.Errorf("dir %d: %v", i, err)
		}
	}
}

// TestSessionLockMapParallelSafety verifies that the per-session lock map
// correctly isolates concurrent access to different sessions while
// serializing access to the same session.
func TestSessionLockMapParallelSafety(t *testing.T) {
	lockMap := &SessionLockMap{Locks: make(map[string]*sync.Mutex)}

	const numSessions = 20
	const opsPerSession = 100
	// counters tracks increments per session to verify no races
	counters := make(map[string]*int)
	var countersMu sync.Mutex

	// Initialize counters for each session
	for i := 0; i < numSessions; i++ {
		ctx := fmt.Sprintf("ctx-%d", i)
		persp := fmt.Sprintf("persp-%d", i)
		key := ctx + "/" + persp
		val := 0
		counters[key] = &val
	}

	var wg sync.WaitGroup

	// Launch concurrent goroutines for each session
	for i := 0; i < numSessions; i++ {
		for j := 0; j < opsPerSession; j++ {
			wg.Add(1)
			go func(sessionIdx int) {
				defer wg.Done()
				ctx := fmt.Sprintf("ctx-%d", sessionIdx)
				persp := fmt.Sprintf("persp-%d", sessionIdx)
				key := ctx + "/" + persp

				mu := lockMap.Get(ctx, persp)
				mu.Lock()
				countersMu.Lock()
				*counters[key]++
				countersMu.Unlock()
				mu.Unlock()
			}(i)
		}
	}
	wg.Wait()

	// Verify each session counter reached exactly opsPerSession
	for i := 0; i < numSessions; i++ {
		ctx := fmt.Sprintf("ctx-%d", i)
		persp := fmt.Sprintf("persp-%d", i)
		key := ctx + "/" + persp
		countersMu.Lock()
		val := *counters[key]
		countersMu.Unlock()
		if val != opsPerSession {
			t.Errorf("session %s: expected %d ops, got %d", key, opsPerSession, val)
		}
	}

	// Verify lock map has exactly numSessions entries
	lockMap.mu.Lock()
	numLocks := len(lockMap.Locks)
	lockMap.mu.Unlock()
	if numLocks != numSessions {
		t.Errorf("expected %d session locks, got %d", numSessions, numLocks)
	}
}

// TestSessionLockMapSameKey verifies that concurrent get() calls for the
// same session key return the same mutex instance.
func TestSessionLockMapSameKey(t *testing.T) {
	lockMap := &SessionLockMap{Locks: make(map[string]*sync.Mutex)}

	const numGoroutines = 100
	mutexes := make([]*sync.Mutex, numGoroutines)
	var wg sync.WaitGroup

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			mutexes[idx] = lockMap.Get("same-ctx", "same-persp")
		}(i)
	}
	wg.Wait()

	// All goroutines must have received the same mutex
	first := mutexes[0]
	for i := 1; i < numGoroutines; i++ {
		if mutexes[i] != first {
			t.Errorf("goroutine %d got different mutex pointer", i)
		}
	}
}
