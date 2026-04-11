package handler

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/heechul/prism-mcp/internal/analysisstore"
	taskpkg "github.com/heechul/prism-mcp/internal/task"
	"github.com/mark3labs/mcp-go/mcp"
)

func makeStatusRequest(taskID string) mcp.CallToolRequest {
	args := map[string]interface{}{}
	if taskID != "" {
		args["task_id"] = taskID
	}
	return mcp.CallToolRequest{
		Params: struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments,omitempty"`
			Meta      *struct {
				ProgressToken mcp.ProgressToken `json:"progressToken,omitempty"`
			} `json:"_meta,omitempty"`
		}{
			Arguments: args,
		},
	}
}

func TestHandleTaskStatusMissingID(t *testing.T) {
	TaskStore = taskpkg.NewTaskStore()

	result, err := HandleTaskStatus(context.Background(), makeStatusRequest(""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for missing task_id")
	}
}

func TestHandleTaskStatusNotFound(t *testing.T) {
	TaskStore = taskpkg.NewTaskStore()

	// Table-driven test for various unknown/lost/malformed task_ids
	tests := []struct {
		name   string
		taskID string
		// wantContains checks that the error message includes this substring
		wantContains string
	}{
		{
			name:         "nonexistent well-formed ID",
			taskID:       "analyze-nonexistent",
			wantContains: "not found",
		},
		{
			name:         "random string",
			taskID:       "random-garbage-id",
			wantContains: "not found",
		},
		{
			name:         "empty-looking hex ID",
			taskID:       "analyze-000000000000",
			wantContains: "not found",
		},
		{
			name:         "numeric only",
			taskID:       "12345",
			wantContains: "not found",
		},
		{
			name:         "single character",
			taskID:       "x",
			wantContains: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := HandleTaskStatus(context.Background(), makeStatusRequest(tt.taskID))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !result.IsError {
				t.Fatal("expected error result for unknown task_id")
			}
			// Verify the error message contains "not found" and the task_id
			errText := result.Content[0].(mcp.TextContent).Text
			if !strings.Contains(errText, tt.wantContains) {
				t.Errorf("error message %q should contain %q", errText, tt.wantContains)
			}
			if !strings.Contains(errText, tt.taskID) {
				t.Errorf("error message %q should contain task_id %q", errText, tt.taskID)
			}
		})
	}
}

func TestHandleTaskStatusWhitespaceOnly(t *testing.T) {
	TaskStore = taskpkg.NewTaskStore()

	// Whitespace-only task_id should be treated as missing (empty after trim)
	result, err := HandleTaskStatus(context.Background(), makeStatusRequest("   "))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for whitespace-only task_id")
	}
	errText := result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(errText, "required") {
		t.Errorf("expected 'required' in error for whitespace-only, got %q", errText)
	}
}

func TestHandleTaskStatusRemovedTask(t *testing.T) {
	TaskStore = taskpkg.NewTaskStore()

	// Create a task, then remove it (simulates lost state or cleanup)
	task := TaskStore.Create("ctx-removed", "claude-sonnet-4-6", "/tmp/state", "/tmp/reports", "")
	taskID := task.ID

	// Verify it exists first
	result, err := HandleTaskStatus(context.Background(), makeStatusRequest(taskID))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatal("task should be found before removal")
	}

	// Remove the task (simulates server restart losing in-memory state)
	TaskStore.Remove(taskID)

	// Now polling should return "not found"
	result, err = HandleTaskStatus(context.Background(), makeStatusRequest(taskID))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for removed task")
	}
	errText := result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(errText, "not found") {
		t.Errorf("error should contain 'not found', got %q", errText)
	}
	if !strings.Contains(errText, taskID) {
		t.Errorf("error should contain task_id %q, got %q", taskID, errText)
	}
}

func TestHandleTaskStatusServerRestart(t *testing.T) {
	prismDir := t.TempDir()
	origBase := PrismBaseDir
	PrismBaseDir = prismDir
	defer func() { PrismBaseDir = origBase }()

	TaskStore = taskpkg.NewTaskStore()
	oldTask := TaskStore.Create("ctx-old", "claude-sonnet-4-6", filepath.Join(prismDir, "state", "analyze-old"), filepath.Join(prismDir, "reports", "analyze-old"), "")
	if err := analysisstore.SaveAnalysisConfig(prismDir, analysisstore.AnalysisConfigRecord{
		TaskID:    oldTask.ID,
		Topic:     "restart test",
		Model:     "default",
		Adaptor:   "codex",
		ContextID: oldTask.ContextID,
		StateDir:  oldTask.StateDir,
		ReportDir: oldTask.ReportDir,
	}); err != nil {
		t.Fatalf("persist config: %v", err)
	}
	if err := oldTask.SetPersistenceHook(func(snapshot taskpkg.TaskSnapshot, pollCount int) error {
		return analysisstore.SaveTaskSnapshot(prismDir, snapshot, pollCount)
	}); err != nil {
		t.Fatalf("set persistence hook: %v", err)
	}
	oldTask.SetStatus(taskpkg.TaskStatusRunning)
	oldTask.StartStage(taskpkg.StageScope, "running after restart")
	oldTaskID := oldTask.ID

	// "Restart" — replace the store with a fresh one
	TaskStore = taskpkg.NewTaskStore()

	// Polling the old task_id should fall back to persisted sqlite snapshot
	result, err := HandleTaskStatus(context.Background(), makeStatusRequest(oldTaskID))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %s", result.Content[0].(mcp.TextContent).Text)
	}
	var snap taskpkg.TaskSnapshot
	if err := json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text), &snap); err != nil {
		t.Fatalf("unmarshal snapshot: %v", err)
	}
	if snap.Status != taskpkg.TaskStatusRunning {
		t.Fatalf("expected running status from persisted snapshot, got %s", snap.Status)
	}
	if snap.Stages[0].Detail != "running after restart" {
		t.Fatalf("expected persisted stage detail, got %q", snap.Stages[0].Detail)
	}
}

func TestHandleTaskStatusServerRestartConsumesPersistedPollBudget(t *testing.T) {
	prismDir := t.TempDir()
	origBase := PrismBaseDir
	PrismBaseDir = prismDir
	defer func() { PrismBaseDir = origBase }()

	TaskStore = taskpkg.NewTaskStore()
	taskID := "analyze-restart-poll"
	if err := analysisstore.SaveAnalysisConfig(prismDir, analysisstore.AnalysisConfigRecord{
		TaskID:    taskID,
		Topic:     "restart poll",
		Model:     "default",
		Adaptor:   "codex",
		CreatedAt: time.Now().UTC(),
		ContextID: taskID,
		StateDir:  filepath.Join(prismDir, "state", taskID),
		ReportDir: filepath.Join(prismDir, "reports", taskID),
	}); err != nil {
		t.Fatalf("persist config: %v", err)
	}

	task := taskpkg.NewAnalysisTask(taskID, "default", filepath.Join(prismDir, "state", taskID), filepath.Join(prismDir, "reports", taskID), "restart-poll")
	task.SetStatus(taskpkg.TaskStatusRunning)
	task.StartStage(taskpkg.StageScope, "still running")
	if err := analysisstore.SaveTaskSnapshot(prismDir, task.Snapshot(), taskpkg.MaxPollIterations); err != nil {
		t.Fatalf("persist running snapshot: %v", err)
	}

	result, err := HandleTaskStatus(context.Background(), makeStatusRequest(taskID))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %s", result.Content[0].(mcp.TextContent).Text)
	}

	var snap taskpkg.TaskSnapshot
	if err := json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text), &snap); err != nil {
		t.Fatalf("unmarshal snapshot: %v", err)
	}
	if snap.Status != taskpkg.TaskStatusFailed {
		t.Fatalf("expected failed status after persisted poll timeout, got %s", snap.Status)
	}
	if !strings.Contains(snap.Error, "poll limit exceeded") {
		t.Fatalf("expected poll timeout error, got %q", snap.Error)
	}

	persisted, pollCount, ok, err := analysisstore.LoadTaskSnapshot(prismDir, taskID)
	if err != nil || !ok {
		t.Fatalf("load persisted snapshot: ok=%v err=%v", ok, err)
	}
	if persisted.Status != taskpkg.TaskStatusFailed {
		t.Fatalf("expected persisted failed status, got %s", persisted.Status)
	}
	if pollCount != taskpkg.MaxPollIterations+1 {
		t.Fatalf("expected poll count %d, got %d", taskpkg.MaxPollIterations+1, pollCount)
	}
	if len(persisted.Stages) == 0 || persisted.Stages[0].Status != taskpkg.StageStatusFailed {
		t.Fatalf("expected running stage to be marked failed, got %+v", persisted.Stages)
	}
	if !strings.Contains(persisted.Stages[0].Detail, "poll limit exceeded") {
		t.Fatalf("expected timed-out stage detail to mention poll limit, got %q", persisted.Stages[0].Detail)
	}
}

func TestHandleTaskStatusPrefersPersistenceFailureSnapshotOverStaleSQLite(t *testing.T) {
	prismDir := t.TempDir()
	origBase := PrismBaseDir
	PrismBaseDir = prismDir
	defer func() { PrismBaseDir = origBase }()

	TaskStore = taskpkg.NewTaskStore()
	taskID := "analyze-persist-failure"
	stateDir := filepath.Join(prismDir, "state", taskID)
	reportDir := filepath.Join(prismDir, "reports", taskID)
	if err := analysisstore.SaveAnalysisConfig(prismDir, analysisstore.AnalysisConfigRecord{
		TaskID:    taskID,
		Topic:     "persist failure",
		Model:     "default",
		Adaptor:   "codex",
		CreatedAt: time.Now().UTC(),
		ContextID: taskID,
		StateDir:  stateDir,
		ReportDir: reportDir,
	}); err != nil {
		t.Fatalf("persist config: %v", err)
	}

	runningTask := taskpkg.NewAnalysisTask(taskID, "default", stateDir, reportDir, "persist-failure")
	runningTask.SetStatus(taskpkg.TaskStatusRunning)
	runningTask.StartStage(taskpkg.StageScope, "stale running row")
	if err := analysisstore.SaveTaskSnapshot(prismDir, runningTask.Snapshot(), 3); err != nil {
		t.Fatalf("persist stale running snapshot: %v", err)
	}

	failedSnapshot := runningTask.Snapshot()
	markSnapshotFailed(&failedSnapshot, "failed to persist task snapshot: disk full")
	if err := savePersistenceFailureSnapshot(stateDir, failedSnapshot, 3); err != nil {
		t.Fatalf("save persistence failure snapshot: %v", err)
	}

	result, err := HandleTaskStatus(context.Background(), makeStatusRequest(taskID))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %s", result.Content[0].(mcp.TextContent).Text)
	}

	var snap taskpkg.TaskSnapshot
	if err := json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text), &snap); err != nil {
		t.Fatalf("unmarshal snapshot: %v", err)
	}
	if snap.Status != taskpkg.TaskStatusFailed {
		t.Fatalf("expected failed status from persistence failure snapshot, got %s", snap.Status)
	}
	if !strings.Contains(snap.Error, "failed to persist task snapshot") {
		t.Fatalf("expected persistence failure error, got %q", snap.Error)
	}
	if len(snap.Stages) == 0 || snap.Stages[0].Status != taskpkg.StageStatusFailed {
		t.Fatalf("expected stage failed from sidecar snapshot, got %+v", snap.Stages)
	}
}

func TestHandleTaskStatusQueued(t *testing.T) {
	TaskStore = taskpkg.NewTaskStore()
	task := TaskStore.Create("ctx-test", "claude-sonnet-4-6", "/tmp/state", "/tmp/reports", "")

	result, err := HandleTaskStatus(context.Background(), makeStatusRequest(task.ID))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatal("unexpected error result")
	}

	// Parse response
	text := result.Content[0].(mcp.TextContent).Text
	var snap taskpkg.TaskSnapshot
	if err := json.Unmarshal([]byte(text), &snap); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if snap.ID != task.ID {
		t.Errorf("expected ID %s, got %s", task.ID, snap.ID)
	}
	if snap.Status != taskpkg.TaskStatusQueued {
		t.Errorf("expected status queued, got %s", snap.Status)
	}
	if len(snap.Stages) != 4 {
		t.Errorf("expected 4 stages, got %d", len(snap.Stages))
	}
}

func TestHandleTaskStatusRunning(t *testing.T) {
	TaskStore = taskpkg.NewTaskStore()
	task := TaskStore.Create("ctx-running", "claude-sonnet-4-6", "/tmp/state", "/tmp/reports", "")
	task.SetStatus(taskpkg.TaskStatusRunning)
	task.StartStage(taskpkg.StageScope, "analyzing seed")

	result, err := HandleTaskStatus(context.Background(), makeStatusRequest(task.ID))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := result.Content[0].(mcp.TextContent).Text
	var snap taskpkg.TaskSnapshot
	if err := json.Unmarshal([]byte(text), &snap); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if snap.Status != taskpkg.TaskStatusRunning {
		t.Errorf("expected running, got %s", snap.Status)
	}
	// First stage (scope) should be running
	if snap.Stages[0].Status != taskpkg.StageStatusRunning {
		t.Errorf("expected scope stage running, got %s", snap.Stages[0].Status)
	}
	if snap.Stages[0].Detail != "analyzing seed" {
		t.Errorf("expected detail 'analyzing seed', got %q", snap.Stages[0].Detail)
	}
}

func TestHandleTaskStatusCompleted(t *testing.T) {
	TaskStore = taskpkg.NewTaskStore()
	task := TaskStore.Create("ctx-done", "claude-sonnet-4-6", "/tmp/state", "/tmp/reports", "")
	task.SetReportPath("/tmp/reports/final.md")

	result, err := HandleTaskStatus(context.Background(), makeStatusRequest(task.ID))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := result.Content[0].(mcp.TextContent).Text
	var snap taskpkg.TaskSnapshot
	if err := json.Unmarshal([]byte(text), &snap); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if snap.Status != taskpkg.TaskStatusCompleted {
		t.Errorf("expected completed, got %s", snap.Status)
	}
	if snap.ReportPath != "/tmp/reports/final.md" {
		t.Errorf("expected report path, got %q", snap.ReportPath)
	}
}

func TestHandleTaskStatusFailed(t *testing.T) {
	TaskStore = taskpkg.NewTaskStore()
	task := TaskStore.Create("ctx-fail", "claude-sonnet-4-6", "/tmp/state", "/tmp/reports", "")
	task.SetError("scope analysis failed")

	result, err := HandleTaskStatus(context.Background(), makeStatusRequest(task.ID))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := result.Content[0].(mcp.TextContent).Text
	var snap taskpkg.TaskSnapshot
	if err := json.Unmarshal([]byte(text), &snap); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if snap.Status != taskpkg.TaskStatusFailed {
		t.Errorf("expected failed, got %s", snap.Status)
	}
	if snap.Error != "scope analysis failed" {
		t.Errorf("expected error message, got %q", snap.Error)
	}
}

func TestHandleTaskStatusParallelProgress(t *testing.T) {
	TaskStore = taskpkg.NewTaskStore()
	task := TaskStore.Create("ctx-parallel", "claude-sonnet-4-6", "/tmp/state", "/tmp/reports", "")
	task.SetStatus(taskpkg.TaskStatusRunning)
	task.CompleteStage(taskpkg.StageScope, "done")
	task.StartStage(taskpkg.StageSpecialist, "running 5 specialists")
	task.SetStageParallel(taskpkg.StageSpecialist, 5)
	task.IncrStageCompleted(taskpkg.StageSpecialist)
	task.IncrStageCompleted(taskpkg.StageSpecialist)
	task.IncrStageFailed(taskpkg.StageSpecialist)

	result, err := HandleTaskStatus(context.Background(), makeStatusRequest(task.ID))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := result.Content[0].(mcp.TextContent).Text
	var snap taskpkg.TaskSnapshot
	if err := json.Unmarshal([]byte(text), &snap); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	// specialist is stage index 1
	spec := snap.Stages[1]
	if spec.Total != 5 {
		t.Errorf("expected total 5, got %d", spec.Total)
	}
	if spec.Completed != 2 {
		t.Errorf("expected completed 2, got %d", spec.Completed)
	}
	if spec.Failed != 1 {
		t.Errorf("expected failed 1, got %d", spec.Failed)
	}
}
