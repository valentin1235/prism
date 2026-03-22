package task

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestGenerateTaskID(t *testing.T) {
	id := generateTaskID("")
	if len(id) < 8 {
		t.Errorf("expected prefix 'analyze-' with suffix, got %q", id)
	}
	if id[:8] != "analyze-" {
		t.Errorf("expected prefix 'analyze-', got %q", id)
	}
	// "analyze-" (8) + 12 hex chars = 20
	if len(id) != 20 {
		t.Errorf("expected length 20, got %d for %q", len(id), id)
	}

	// Uniqueness
	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		id := generateTaskID("")
		if seen[id] {
			t.Fatalf("duplicate ID generated: %s", id)
		}
		seen[id] = true
	}
}

func TestGenerateTaskIDWithSessionID(t *testing.T) {
	// When session_id is provided, task_id should be "analyze-{session_id}"
	id := generateTaskID("my-session-123")
	if id != "analyze-my-session-123" {
		t.Errorf("expected 'analyze-my-session-123', got %q", id)
	}

	// Empty session_id should generate random ID
	id = generateTaskID("")
	if id[:8] != "analyze-" {
		t.Errorf("expected prefix 'analyze-', got %q", id)
	}
	if id == "analyze-" {
		t.Error("empty session_id should not produce 'analyze-' with no suffix")
	}
}

func TestTaskStatusIsTerminal(t *testing.T) {
	tests := []struct {
		status   TaskStatus
		terminal bool
	}{
		{TaskStatusQueued, false},
		{TaskStatusRunning, false},
		{TaskStatusCompleted, true},
		{TaskStatusFailed, true},
	}
	for _, tt := range tests {
		if got := tt.status.IsTerminal(); got != tt.terminal {
			t.Errorf("TaskStatus(%q).IsTerminal() = %v, want %v", tt.status, got, tt.terminal)
		}
	}
}

func TestStageStatusIsTerminal(t *testing.T) {
	tests := []struct {
		status   StageStatus
		terminal bool
	}{
		{StageStatusPending, false},
		{StageStatusRunning, false},
		{StageStatusCompleted, true},
		{StageStatusFailed, true},
		{StageStatusSkipped, true},
	}
	for _, tt := range tests {
		if got := tt.status.IsTerminal(); got != tt.terminal {
			t.Errorf("StageStatus(%q).IsTerminal() = %v, want %v", tt.status, got, tt.terminal)
		}
	}
}

func TestNewAnalysisTask(t *testing.T) {
	task := NewAnalysisTask("ctx-123", "claude-sonnet-4-6", "/tmp/state", "/tmp/reports", "")

	if task.Status != TaskStatusQueued {
		t.Errorf("expected status queued, got %s", task.Status)
	}
	if task.ContextID != "ctx-123" {
		t.Errorf("expected context_id ctx-123, got %s", task.ContextID)
	}
	if task.Model != "claude-sonnet-4-6" {
		t.Errorf("expected model claude-sonnet-4-6, got %s", task.Model)
	}
	if len(task.Stages) != 4 {
		t.Errorf("expected 4 stages, got %d", len(task.Stages))
	}
	for _, name := range AllStages() {
		s, ok := task.Stages[name]
		if !ok {
			t.Errorf("missing stage %s", name)
			continue
		}
		if s.Status != StageStatusPending {
			t.Errorf("stage %s: expected pending, got %s", name, s.Status)
		}
	}
}

func TestTaskLifecycle(t *testing.T) {
	task := NewAnalysisTask("ctx-1", "model", "/state", "/reports", "")

	// Start task
	task.SetStatus(TaskStatusRunning)
	if task.Status != TaskStatusRunning {
		t.Fatalf("expected running, got %s", task.Status)
	}

	// Start scope stage
	task.StartStage(StageScope, "analyzing scope")
	snap := task.Snapshot()
	if snap.Stages[0].Status != StageStatusRunning {
		t.Errorf("scope stage: expected running, got %s", snap.Stages[0].Status)
	}
	if snap.Stages[0].StartedAt == nil {
		t.Error("scope stage: expected non-nil StartedAt")
	}

	// Complete scope stage
	task.CompleteStage(StageScope, "3 perspectives identified")
	snap = task.Snapshot()
	if snap.Stages[0].Status != StageStatusCompleted {
		t.Errorf("scope stage: expected completed, got %s", snap.Stages[0].Status)
	}

	// Parallel specialist stage
	task.StartStage(StageSpecialist, "running specialists")
	task.SetStageParallel(StageSpecialist, 3)
	task.IncrStageCompleted(StageSpecialist)
	task.IncrStageCompleted(StageSpecialist)
	task.IncrStageFailed(StageSpecialist)
	task.CompleteStage(StageSpecialist, "2/3 succeeded")

	snap = task.Snapshot()
	specStage := snap.Stages[1]
	if specStage.Total != 3 || specStage.Completed != 2 || specStage.Failed != 1 {
		t.Errorf("specialist stage: expected 3/2/1, got %d/%d/%d", specStage.Total, specStage.Completed, specStage.Failed)
	}

	// Complete task
	task.SetReportPath("/reports/final.md")
	if task.Status != TaskStatusCompleted {
		t.Errorf("expected completed, got %s", task.Status)
	}
}

func TestTaskSetError(t *testing.T) {
	task := NewAnalysisTask("ctx-err", "model", "/state", "/reports", "")
	task.SetStatus(TaskStatusRunning)
	task.SetError("something went wrong")

	if task.Status != TaskStatusFailed {
		t.Errorf("expected failed, got %s", task.Status)
	}
	if task.Error != "something went wrong" {
		t.Errorf("expected error message, got %q", task.Error)
	}
}

func TestTaskStoreCreateAndGet(t *testing.T) {
	store := NewTaskStore()

	task := store.Create("ctx-1", "model", "/state", "/reports", "")
	if task == nil {
		t.Fatal("expected non-nil task")
	}

	got := store.Get(task.ID)
	if got == nil {
		t.Fatal("expected to find task by ID")
	}
	if got.ID != task.ID {
		t.Errorf("expected ID %s, got %s", task.ID, got.ID)
	}

	// Not found
	if store.Get("nonexistent") != nil {
		t.Error("expected nil for nonexistent task")
	}
}

func TestTaskStoreSnapshot(t *testing.T) {
	store := NewTaskStore()
	task := store.Create("ctx-snap", "model", "/state", "/reports", "")

	snap, ok := store.Snapshot(task.ID)
	if !ok {
		t.Fatal("expected to find snapshot")
	}
	if snap.ID != task.ID {
		t.Errorf("expected ID %s, got %s", task.ID, snap.ID)
	}

	_, ok = store.Snapshot("nonexistent")
	if ok {
		t.Error("expected not found for nonexistent task")
	}
}

func TestTaskStoreList(t *testing.T) {
	store := NewTaskStore()

	store.Create("ctx-1", "model", "/state", "/reports", "")
	store.Create("ctx-2", "model", "/state", "/reports", "")
	store.Create("ctx-3", "model", "/state", "/reports", "")

	list := store.List()
	if len(list) != 3 {
		t.Errorf("expected 3 tasks, got %d", len(list))
	}
}

func TestTaskStoreConcurrency(t *testing.T) {
	store := NewTaskStore()
	var wg sync.WaitGroup

	// Concurrent creates
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			store.Create("ctx-concurrent", "model", "/state", "/reports", "")
		}()
	}
	wg.Wait()

	list := store.List()
	if len(list) != 100 {
		t.Errorf("expected 100 tasks, got %d", len(list))
	}

	// Concurrent reads + writes
	task := store.Get(list[0].ID)
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			task.IncrStageCompleted(StageSpecialist)
		}()
		go func() {
			defer wg.Done()
			_ = task.Snapshot()
		}()
	}
	wg.Wait()
}

func TestCrossTaskIsolation(t *testing.T) {
	store := NewTaskStore()
	task1 := store.Create("ctx-iso-1", "model", "/state/1", "/reports/1", "")
	task2 := store.Create("ctx-iso-2", "model", "/state/2", "/reports/2", "")

	// Mutate task1 — task2 must be unaffected
	task1.SetStatus(TaskStatusRunning)
	task1.StartStage(StageScope, "analyzing")
	task1.SetStageParallel(StageSpecialist, 5)
	task1.IncrStageCompleted(StageSpecialist)
	task1.SetError("task1 failed")

	snap2 := task2.Snapshot()
	if snap2.Status != TaskStatusQueued {
		t.Errorf("task2 status leaked from task1: got %s, want queued", snap2.Status)
	}
	if snap2.Error != "" {
		t.Errorf("task2 error leaked from task1: got %q", snap2.Error)
	}
	for _, s := range snap2.Stages {
		if s.Status != StageStatusPending {
			t.Errorf("task2 stage %s leaked from task1: got %s, want pending", s.Name, s.Status)
		}
		if s.Total != 0 || s.Completed != 0 {
			t.Errorf("task2 stage %s parallel counters leaked from task1", s.Name)
		}
	}
}

func TestSnapshotImmutability(t *testing.T) {
	task := NewAnalysisTask("ctx-immut", "model", "/state", "/reports", "")
	task.SetStatus(TaskStatusRunning)
	task.StartStage(StageScope, "running scope")

	snap := task.Snapshot()

	// Mutate original task after snapshot
	task.CompleteStage(StageScope, "done")
	task.SetStageParallel(StageSpecialist, 10)
	task.SetReportPath("/reports/final.md")

	// Snapshot must reflect the state at the time it was taken
	if snap.Status != TaskStatusRunning {
		t.Errorf("snapshot status mutated: got %s, want running", snap.Status)
	}
	if snap.ReportPath != "" {
		t.Errorf("snapshot report_path mutated: got %q", snap.ReportPath)
	}
	if snap.Stages[0].Status != StageStatusRunning {
		t.Errorf("snapshot scope stage mutated: got %s, want running", snap.Stages[0].Status)
	}
	// Verify time pointer deep copy: modifying a stage's StartedAt after snapshot
	// shouldn't affect the snapshot
	if snap.Stages[0].StartedAt == nil {
		t.Fatal("expected non-nil StartedAt in snapshot")
	}
}

func TestConcurrentCrossTaskOperations(t *testing.T) {
	store := NewTaskStore()
	const numTasks = 50
	const opsPerTask = 100

	// Create tasks
	tasks := make([]*AnalysisTask, numTasks)
	for i := 0; i < numTasks; i++ {
		tasks[i] = store.Create(fmt.Sprintf("ctx-%d", i), "model", "/state", "/reports", "")
	}

	var wg sync.WaitGroup
	// Hammer each task independently and concurrently
	for i := 0; i < numTasks; i++ {
		wg.Add(1)
		go func(task *AnalysisTask) {
			defer wg.Done()
			task.SetStatus(TaskStatusRunning)
			task.StartStage(StageSpecialist, "running")
			task.SetStageParallel(StageSpecialist, opsPerTask)
			for j := 0; j < opsPerTask; j++ {
				task.IncrStageCompleted(StageSpecialist)
				_ = task.Snapshot() // concurrent reads
			}
			task.CompleteStage(StageSpecialist, "done")
		}(tasks[i])
	}
	wg.Wait()

	// Verify each task independently reached correct state
	for i, task := range tasks {
		snap := task.Snapshot()
		if snap.Status != TaskStatusRunning {
			t.Errorf("task %d: expected running, got %s", i, snap.Status)
		}
		specStage := snap.Stages[1] // specialist is index 1
		if specStage.Total != opsPerTask {
			t.Errorf("task %d: expected total %d, got %d", i, opsPerTask, specStage.Total)
		}
		if specStage.Completed != opsPerTask {
			t.Errorf("task %d: expected completed %d, got %d", i, opsPerTask, specStage.Completed)
		}
	}

	// Verify store list is consistent
	list := store.List()
	if len(list) != numTasks {
		t.Errorf("expected %d tasks in store, got %d", numTasks, len(list))
	}
}

func TestSnapshotStageOrdering(t *testing.T) {
	task := NewAnalysisTask("ctx-order", "model", "/state", "/reports", "")
	snap := task.Snapshot()

	expected := AllStages()
	if len(snap.Stages) != len(expected) {
		t.Fatalf("expected %d stages, got %d", len(expected), len(snap.Stages))
	}
	for i, name := range expected {
		if snap.Stages[i].Name != name {
			t.Errorf("stage %d: expected %s, got %s", i, name, snap.Stages[i].Name)
		}
	}
}

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
			task := store.Create("", "claude-sonnet-4-6", "", "", "")
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
			task := store.Create("", "claude-sonnet-4-6", "", "", "")
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
			if !strings.Contains(string(fData), snap.ID) {
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
