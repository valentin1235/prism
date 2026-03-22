package handler

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	taskpkg "github.com/heechul/prism-mcp/internal/task"
	"github.com/mark3labs/mcp-go/mcp"
)

func makeResultRequest(taskID string) mcp.CallToolRequest {
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

func getResultText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	if len(result.Content) == 0 {
		t.Fatal("expected at least one content block")
	}
	tc, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}
	return tc.Text
}

func TestHandleAnalyzeResultMissingID(t *testing.T) {
	TaskStore = taskpkg.NewTaskStore()

	result, err := HandleAnalyzeResult(context.Background(), makeResultRequest(""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for missing task_id")
	}
	text := getResultText(t, result)
	if !strings.Contains(text, "required") {
		t.Errorf("expected 'required' in error, got %q", text)
	}
}

func TestHandleAnalyzeResultNotFound(t *testing.T) {
	TaskStore = taskpkg.NewTaskStore()

	result, err := HandleAnalyzeResult(context.Background(), makeResultRequest("analyze-nonexistent"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for unknown task_id")
	}
	text := getResultText(t, result)
	if !strings.Contains(text, "not found") {
		t.Errorf("expected 'not found' in error, got %q", text)
	}
}

func TestHandleAnalyzeResultStillRunning(t *testing.T) {
	TaskStore = taskpkg.NewTaskStore()
	task := TaskStore.Create("ctx-run", "claude-sonnet-4-6", "/tmp/state", "/tmp/reports", "")
	task.SetStatus(taskpkg.TaskStatusRunning)

	result, err := HandleAnalyzeResult(context.Background(), makeResultRequest(task.ID))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for running task")
	}
	text := getResultText(t, result)
	if !strings.Contains(text, "still running") {
		t.Errorf("expected 'still running' in error, got %q", text)
	}
	if !strings.Contains(text, "prism_task_status") {
		t.Errorf("expected hint to use prism_task_status, got %q", text)
	}
}

func TestHandleAnalyzeResultQueued(t *testing.T) {
	TaskStore = taskpkg.NewTaskStore()
	task := TaskStore.Create("ctx-q", "claude-sonnet-4-6", "/tmp/state", "/tmp/reports", "")
	// Default status is queued

	result, err := HandleAnalyzeResult(context.Background(), makeResultRequest(task.ID))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for queued task")
	}
	text := getResultText(t, result)
	if !strings.Contains(text, "still queued") {
		t.Errorf("expected 'still queued' in error, got %q", text)
	}
}

func TestHandleAnalyzeResultFailed(t *testing.T) {
	TaskStore = taskpkg.NewTaskStore()
	task := TaskStore.Create("ctx-fail", "claude-sonnet-4-6", "/tmp/state", "/tmp/reports", "")
	task.SetError("scope analysis exploded")

	result, err := HandleAnalyzeResult(context.Background(), makeResultRequest(task.ID))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for failed task")
	}
	text := getResultText(t, result)
	if !strings.Contains(text, "failed") {
		t.Errorf("expected 'failed' in error, got %q", text)
	}
	if !strings.Contains(text, "scope analysis exploded") {
		t.Errorf("expected error message in response, got %q", text)
	}
}

func TestHandleAnalyzeResultCompleted(t *testing.T) {
	TaskStore = taskpkg.NewTaskStore()

	// Create a temp report file with executive summary
	tmpDir := t.TempDir()
	reportPath := filepath.Join(tmpDir, "report.md")
	reportContent := `# Analysis Report

## Executive Summary

This analysis identified 3 critical issues in the payment processing pipeline.
The system exhibits race conditions under high load, leading to duplicate charges.

## Analysis Overview

Detailed analysis follows below.

## Perspective Findings

### Payment Flow Analyst
- Finding 1: Race condition in checkout

## Integrated Analysis

Cross-cutting concerns identified.

## Socratic Verification Summary

All findings verified with score > 0.8.

## Recommendations

1. Fix the race condition (Critical, High Impact)

## Appendix

Raw data available in state directory.
`
	if err := os.WriteFile(reportPath, []byte(reportContent), 0644); err != nil {
		t.Fatalf("failed to write test report: %v", err)
	}

	task := TaskStore.Create("ctx-done", "claude-sonnet-4-6", "/tmp/state", tmpDir, "")
	task.SetReportPath(reportPath)

	result, err := HandleAnalyzeResult(context.Background(), makeResultRequest(task.ID))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %s", getResultText(t, result))
	}

	text := getResultText(t, result)
	var resp AnalyzeResultResponse
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.TaskID != task.ID {
		t.Errorf("expected task_id %s, got %s", task.ID, resp.TaskID)
	}
	if resp.Status != "completed" {
		t.Errorf("expected status 'completed', got %s", resp.Status)
	}
	if resp.ReportPath != reportPath {
		t.Errorf("expected report_path %s, got %s", reportPath, resp.ReportPath)
	}
	// Summary should contain Executive Summary content
	if !strings.Contains(resp.Summary, "3 critical issues") {
		t.Errorf("expected summary to contain executive summary content, got %q", resp.Summary)
	}
	if !strings.Contains(resp.Summary, "duplicate charges") {
		t.Errorf("expected summary to contain 'duplicate charges', got %q", resp.Summary)
	}
}

func TestHandleAnalyzeResultCompletedNoReportPath(t *testing.T) {
	TaskStore = taskpkg.NewTaskStore()
	task := TaskStore.Create("ctx-noreport", "claude-sonnet-4-6", "/tmp/state", "/tmp/reports", "")
	// Manually set completed without report path (edge case)
	task.SetStatus(taskpkg.TaskStatusCompleted)

	result, err := HandleAnalyzeResult(context.Background(), makeResultRequest(task.ID))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error when report path is empty")
	}
	text := getResultText(t, result)
	if !strings.Contains(text, "no report path") {
		t.Errorf("expected 'no report path' in error, got %q", text)
	}
}

func TestHandleAnalyzeResultReportFileUnreadable(t *testing.T) {
	TaskStore = taskpkg.NewTaskStore()
	task := TaskStore.Create("ctx-unreadable", "claude-sonnet-4-6", "/tmp/state", "/tmp/reports", "")
	task.SetReportPath("/nonexistent/path/report.md")

	result, err := HandleAnalyzeResult(context.Background(), makeResultRequest(task.ID))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should still succeed but with a fallback summary
	if result.IsError {
		t.Fatalf("unexpected error result: %s", getResultText(t, result))
	}

	text := getResultText(t, result)
	var resp AnalyzeResultResponse
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.ReportPath != "/nonexistent/path/report.md" {
		t.Errorf("expected report path preserved, got %s", resp.ReportPath)
	}
	if !strings.Contains(resp.Summary, "summary extraction failed") {
		t.Errorf("expected fallback summary message, got %q", resp.Summary)
	}
}

func TestExtractReportSummary(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name            string
		content         string
		wantContains    string
		wantNotContains string
	}{
		{
			name: "extracts executive summary section",
			content: `# Report
## Executive Summary
Key finding: system is unstable.
## Next Section
More details here.`,
			wantContains:    "Key finding: system is unstable.",
			wantNotContains: "More details here.",
		},
		{
			name: "handles nested headings within executive summary",
			content: `# Report
## Executive Summary
Overview of findings.
### Sub-detail
A sub-detail here.
## Analysis Overview
Other stuff.`,
			wantContains: "Sub-detail",
		},
		{
			name:         "falls back to preview when no executive summary",
			content:      "# Report\n\nSome analysis content without executive summary header.\nMore content here.",
			wantContains: "Some analysis content",
		},
		{
			name: "truncates long executive summary",
			content: func() string {
				var sb strings.Builder
				sb.WriteString("## Executive Summary\n")
				for i := 0; i < 300; i++ {
					sb.WriteString("This is a very long line of text that goes on and on. ")
				}
				sb.WriteString("\n## Next Section\n")
				return sb.String()
			}(),
			wantContains: "truncated",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(tmpDir, tt.name+".md")
			if err := os.WriteFile(path, []byte(tt.content), 0644); err != nil {
				t.Fatalf("write test file: %v", err)
			}

			summary, err := ExtractReportSummary(path)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !strings.Contains(summary, tt.wantContains) {
				t.Errorf("summary should contain %q, got %q", tt.wantContains, summary)
			}
			if tt.wantNotContains != "" && strings.Contains(summary, tt.wantNotContains) {
				t.Errorf("summary should NOT contain %q, got %q", tt.wantNotContains, summary)
			}
		})
	}
}

func TestExtractSection(t *testing.T) {
	content := `# Title
## Section A
Content A here.
## Section B
Content B here.
### Sub B
Sub content.
## Section C
Content C.`

	// Extract Section A
	a := ExtractSection(content, "Section A")
	if !strings.Contains(a, "Content A here.") {
		t.Errorf("expected Section A content, got %q", a)
	}
	if strings.Contains(a, "Content B") {
		t.Errorf("Section A should not contain Section B content")
	}

	// Extract Section B (should include sub-heading content)
	b := ExtractSection(content, "Section B")
	if !strings.Contains(b, "Content B here.") {
		t.Errorf("expected Section B content, got %q", b)
	}
	if !strings.Contains(b, "Sub content.") {
		t.Errorf("expected sub-heading content in Section B, got %q", b)
	}

	// Extract non-existent section
	none := ExtractSection(content, "Nonexistent")
	if none != "" {
		t.Errorf("expected empty for nonexistent section, got %q", none)
	}
}
