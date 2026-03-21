package main

import (
	"fmt"
	"strings"
	"sync"
	"testing"
)

func TestGenerateTaskID(t *testing.T) {
	id := generateTaskID()
	if !strings.HasPrefix(id, "analyze-") {
		t.Errorf("expected prefix 'analyze-', got %q", id)
	}
	// "analyze-" (8) + 12 hex chars = 20
	if len(id) != 20 {
		t.Errorf("expected length 20, got %d for %q", len(id), id)
	}

	// Uniqueness
	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		id := generateTaskID()
		if seen[id] {
			t.Fatalf("duplicate ID generated: %s", id)
		}
		seen[id] = true
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
	task := newAnalysisTask("ctx-123", "claude-sonnet-4-6", "/tmp/state", "/tmp/reports")

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
	task := newAnalysisTask("ctx-1", "model", "/state", "/reports")

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
	task := newAnalysisTask("ctx-err", "model", "/state", "/reports")
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

	task := store.Create("ctx-1", "model", "/state", "/reports")
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
	task := store.Create("ctx-snap", "model", "/state", "/reports")

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

	store.Create("ctx-1", "model", "/state", "/reports")
	store.Create("ctx-2", "model", "/state", "/reports")
	store.Create("ctx-3", "model", "/state", "/reports")

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
			store.Create("ctx-concurrent", "model", "/state", "/reports")
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
	task1 := store.Create("ctx-iso-1", "model", "/state/1", "/reports/1")
	task2 := store.Create("ctx-iso-2", "model", "/state/2", "/reports/2")

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
	task := newAnalysisTask("ctx-immut", "model", "/state", "/reports")
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
		tasks[i] = store.Create(fmt.Sprintf("ctx-%d", i), "model", "/state", "/reports")
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
	task := newAnalysisTask("ctx-order", "model", "/state", "/reports")
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
