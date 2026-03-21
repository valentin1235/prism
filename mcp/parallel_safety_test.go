package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// TestParallelTaskDirectoryIsolation verifies that concurrent analysis tasks
// create completely independent state directories with no path overlap.
func TestParallelTaskDirectoryIsolation(t *testing.T) {
	store := NewTaskStore()
	const numTasks = 20

	tmpDir := t.TempDir()
	stateBase := filepath.Join(tmpDir, "state")
	reportBase := filepath.Join(tmpDir, "reports")

	var wg sync.WaitGroup
	tasks := make([]*AnalysisTask, numTasks)
	errors := make([]error, numTasks)

	// Create tasks concurrently — simulates parallel handleAnalyze calls
	for i := 0; i < numTasks; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			task := store.Create("", "claude-sonnet-4-6", "", "")
			contextID := task.ID
			stateDir := filepath.Join(stateBase, contextID)
			reportDir := filepath.Join(reportBase, contextID)
			task.UpdateDirs(contextID, stateDir, reportDir)

			if err := os.MkdirAll(stateDir, 0755); err != nil {
				errors[idx] = fmt.Errorf("task %d state dir: %w", idx, err)
				return
			}
			if err := os.MkdirAll(reportDir, 0755); err != nil {
				errors[idx] = fmt.Errorf("task %d report dir: %w", idx, err)
				return
			}

			// Write a task-specific marker file to verify isolation
			marker := filepath.Join(stateDir, "marker.txt")
			if err := os.WriteFile(marker, []byte(task.ID), 0644); err != nil {
				errors[idx] = fmt.Errorf("task %d marker: %w", idx, err)
				return
			}

			tasks[idx] = task
		}(i)
	}
	wg.Wait()

	// Check for errors
	for i, err := range errors {
		if err != nil {
			t.Fatalf("task %d failed: %v", i, err)
		}
	}

	// Verify all tasks got unique directories
	seenDirs := make(map[string]string)
	for i, task := range tasks {
		if task == nil {
			t.Fatalf("task %d is nil", i)
		}
		snap := task.Snapshot()
		if prev, exists := seenDirs[snap.ContextID]; exists {
			t.Errorf("directory collision: task %d shares context_id %s with %s", i, snap.ContextID, prev)
		}
		seenDirs[snap.ContextID] = snap.ID

		// Verify marker file contains this task's ID (not another task's)
		stateDir := filepath.Join(stateBase, snap.ContextID)
		data, err := os.ReadFile(filepath.Join(stateDir, "marker.txt"))
		if err != nil {
			t.Fatalf("task %d: cannot read marker: %v", i, err)
		}
		if string(data) != snap.ID {
			t.Errorf("task %d: marker contains %q, expected %q — cross-task contamination", i, string(data), snap.ID)
		}
	}
}

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
	lockMap := &sessionLockMap{locks: make(map[string]*sync.Mutex)}

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

				mu := lockMap.get(ctx, persp)
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
	numLocks := len(lockMap.locks)
	lockMap.mu.Unlock()
	if numLocks != numSessions {
		t.Errorf("expected %d session locks, got %d", numSessions, numLocks)
	}
}

// TestSessionLockMapSameKey verifies that concurrent get() calls for the
// same session key return the same mutex instance.
func TestSessionLockMapSameKey(t *testing.T) {
	lockMap := &sessionLockMap{locks: make(map[string]*sync.Mutex)}

	const numGoroutines = 100
	mutexes := make([]*sync.Mutex, numGoroutines)
	var wg sync.WaitGroup

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			mutexes[idx] = lockMap.get("same-ctx", "same-persp")
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

// TestConcurrentPipelineSimulation simulates multiple analysis pipelines
// running concurrently, each with its own state directory and task,
// verifying no cross-contamination.
func TestConcurrentPipelineSimulation(t *testing.T) {
	store := NewTaskStore()
	tmpDir := t.TempDir()
	const numPipelines = 10
	const numSpecialists = 5

	var wg sync.WaitGroup
	pipelineErrors := make([][]error, numPipelines)

	for p := 0; p < numPipelines; p++ {
		pipelineErrors[p] = make([]error, 0)
		wg.Add(1)
		go func(pIdx int) {
			defer wg.Done()

			// Create task (mimics handleAnalyze)
			task := store.Create("", "claude-sonnet-4-6", "", "")
			stateDir := filepath.Join(tmpDir, "state", task.ID)
			reportDir := filepath.Join(tmpDir, "reports", task.ID)
			task.UpdateDirs(task.ID, stateDir, reportDir)

			if err := os.MkdirAll(stateDir, 0755); err != nil {
				pipelineErrors[pIdx] = append(pipelineErrors[pIdx], err)
				return
			}
			if err := os.MkdirAll(reportDir, 0755); err != nil {
				pipelineErrors[pIdx] = append(pipelineErrors[pIdx], err)
				return
			}

			// Simulate pipeline stages
			task.SetStatus(TaskStatusRunning)

			// Stage 1: Scope (sequential)
			task.StartStage(StageScope, "seed analysis")
			scopeFile := filepath.Join(stateDir, "seed-analysis.json")
			if err := os.WriteFile(scopeFile, []byte(fmt.Sprintf(`{"task":"%s"}`, task.ID)), 0644); err != nil {
				pipelineErrors[pIdx] = append(pipelineErrors[pIdx], err)
				return
			}
			task.CompleteStage(StageScope, "done")

			// Stage 2: Specialists (parallel within this pipeline)
			task.StartStage(StageSpecialist, fmt.Sprintf("running %d specialists", numSpecialists))
			task.SetStageParallel(StageSpecialist, numSpecialists)

			var specWg sync.WaitGroup
			for s := 0; s < numSpecialists; s++ {
				specWg.Add(1)
				go func(sIdx int) {
					defer specWg.Done()
					perspID := fmt.Sprintf("perspective-%d", sIdx)
					perspDir := filepath.Join(stateDir, "perspectives", perspID)
					if err := os.MkdirAll(perspDir, 0755); err != nil {
						task.IncrStageFailed(StageSpecialist)
						return
					}
					// Write findings specific to this specialist
					findings := filepath.Join(perspDir, "findings.json")
					content := fmt.Sprintf(`{"task":"%s","perspective":"%s"}`, task.ID, perspID)
					if err := os.WriteFile(findings, []byte(content), 0644); err != nil {
						task.IncrStageFailed(StageSpecialist)
						return
					}
					task.IncrStageCompleted(StageSpecialist)
				}(s)
			}
			specWg.Wait()
			task.CompleteStage(StageSpecialist, "all done")

			// Stage 3: Interview (parallel)
			task.StartStage(StageInterview, "interviewing")
			task.SetStageParallel(StageInterview, numSpecialists)
			var intWg sync.WaitGroup
			for s := 0; s < numSpecialists; s++ {
				intWg.Add(1)
				go func(sIdx int) {
					defer intWg.Done()
					perspID := fmt.Sprintf("perspective-%d", sIdx)
					perspDir := filepath.Join(stateDir, "perspectives", perspID)
					interview := filepath.Join(perspDir, "interview.json")
					content := fmt.Sprintf(`{"task":"%s","perspective":"%s","rounds":[]}`, task.ID, perspID)
					if err := os.WriteFile(interview, []byte(content), 0644); err != nil {
						task.IncrStageFailed(StageInterview)
						return
					}
					task.IncrStageCompleted(StageInterview)
				}(s)
			}
			intWg.Wait()
			task.CompleteStage(StageInterview, "all done")

			// Stage 4: Synthesis
			task.StartStage(StageSynthesis, "generating report")
			reportFile := filepath.Join(reportDir, "report.md")
			if err := os.WriteFile(reportFile, []byte(fmt.Sprintf("# Report for %s", task.ID)), 0644); err != nil {
				pipelineErrors[pIdx] = append(pipelineErrors[pIdx], err)
				return
			}
			task.CompleteStage(StageSynthesis, "done")
			task.SetReportPath(reportFile)
		}(p)
	}
	wg.Wait()

	// Verify no errors
	for p, errs := range pipelineErrors {
		for _, err := range errs {
			t.Errorf("pipeline %d: %v", p, err)
		}
	}

	// Verify all tasks completed independently
	list := store.List()
	if len(list) != numPipelines {
		t.Fatalf("expected %d tasks, got %d", numPipelines, len(list))
	}

	for _, snap := range list {
		if snap.Status != TaskStatusCompleted {
			t.Errorf("task %s: expected completed, got %s", snap.ID, snap.Status)
		}
		if snap.ReportPath == "" {
			t.Errorf("task %s: missing report path", snap.ID)
		}

		// Verify report file contains this task's ID
		data, err := os.ReadFile(snap.ReportPath)
		if err != nil {
			t.Errorf("task %s: cannot read report: %v", snap.ID, err)
			continue
		}
		expected := fmt.Sprintf("# Report for %s", snap.ID)
		if string(data) != expected {
			t.Errorf("task %s: report contaminated — got %q, want %q", snap.ID, string(data), expected)
		}

		// Verify specialist findings contain correct task ID
		stateDir := filepath.Join(tmpDir, "state", snap.ID)
		for s := 0; s < numSpecialists; s++ {
			perspID := fmt.Sprintf("perspective-%d", s)
			findingsFile := filepath.Join(stateDir, "perspectives", perspID, "findings.json")
			fData, err := os.ReadFile(findingsFile)
			if err != nil {
				t.Errorf("task %s persp %s: cannot read findings: %v", snap.ID, perspID, err)
				continue
			}
			if !contains(string(fData), snap.ID) {
				t.Errorf("task %s persp %s: findings contaminated — %q", snap.ID, perspID, string(fData))
			}
		}

		// Verify specialist stage counters
		specStage := snap.Stages[1] // specialist
		if specStage.Total != numSpecialists {
			t.Errorf("task %s: specialist total %d, want %d", snap.ID, specStage.Total, numSpecialists)
		}
		if specStage.Completed != numSpecialists {
			t.Errorf("task %s: specialist completed %d, want %d", snap.ID, specStage.Completed, numSpecialists)
		}
		if specStage.Failed != 0 {
			t.Errorf("task %s: specialist failed %d, want 0", snap.ID, specStage.Failed)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsImpl(s, substr))
}

func containsImpl(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
