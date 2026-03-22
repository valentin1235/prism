package handler

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

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
	// Simulate server restart: old task_id from previous session
	// is polled against a fresh (empty) TaskStore
	TaskStore = taskpkg.NewTaskStore()
	oldTask := TaskStore.Create("ctx-old", "claude-sonnet-4-6", "/tmp/state", "/tmp/reports", "")
	oldTaskID := oldTask.ID

	// "Restart" — replace the store with a fresh one
	TaskStore = taskpkg.NewTaskStore()

	// Polling the old task_id should return "not found"
	result, err := HandleTaskStatus(context.Background(), makeStatusRequest(oldTaskID))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for task from previous server session")
	}
	errText := result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(errText, "not found") {
		t.Errorf("error should contain 'not found', got %q", errText)
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
