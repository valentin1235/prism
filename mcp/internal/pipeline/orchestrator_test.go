package pipeline

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	taskpkg "github.com/heechul/prism-mcp/internal/task"
)

// --- Helper: create a task with temp state/report dirs and a valid config.json ---

func createTestTask(t *testing.T, topic, model string) (*taskpkg.AnalysisTask, string) {
	t.Helper()

	tmpDir := t.TempDir()
	stateDir := filepath.Join(tmpDir, "state")
	reportDir := filepath.Join(tmpDir, "reports")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatalf("mkdir state: %v", err)
	}
	if err := os.MkdirAll(reportDir, 0755); err != nil {
		t.Fatalf("mkdir reports: %v", err)
	}

	store := taskpkg.NewTaskStore()
	task := store.Create("", model, stateDir, reportDir, "")
	task.UpdateDirs(task.ID, stateDir, reportDir)

	// Set up a context with a short timeout so that any LLM subprocess calls
	// (e.g. Claude CLI via QuerySync) fail fast instead of hanging until the
	// Go test timeout. Tests using createTestTask verify state transitions,
	// not actual LLM output, so a quick failure at the CLI call is expected.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	task.Ctx = ctx
	task.Cancel = cancel

	// Write config.json
	cfg := AnalysisConfig{
		Topic:     topic,
		Model:     model,
		TaskID:    task.ID,
		ContextID: task.ID,
		StateDir:  stateDir,
		ReportDir: reportDir,
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(stateDir, "config.json"), data, 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	return task, tmpDir
}

// --- Tests for ReadAnalysisConfig ---

func TestReadAnalysisConfig(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := AnalysisConfig{
		Topic:     "test topic",
		Model:     "claude-sonnet-4-6",
		TaskID:    "analyze-abc123",
		ContextID: "analyze-abc123",
		StateDir:  tmpDir,
		ReportDir: "/tmp/reports",
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(filepath.Join(tmpDir, "config.json"), data, 0644)

	got, err := ReadAnalysisConfig(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Topic != "test topic" {
		t.Errorf("expected topic 'test topic', got %q", got.Topic)
	}
	if got.Model != "claude-sonnet-4-6" {
		t.Errorf("expected model, got %q", got.Model)
	}
}

func TestReadAnalysisConfigMissing(t *testing.T) {
	tmpDir := t.TempDir()
	_, err := ReadAnalysisConfig(tmpDir)
	if err == nil {
		t.Fatal("expected error for missing config.json")
	}
}

func TestReadAnalysisConfigInvalid(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "config.json"), []byte("not json"), 0644)
	_, err := ReadAnalysisConfig(tmpDir)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

// --- Tests for pipeline progress tracking ---

func TestPipelineFailsOnMissingConfig(t *testing.T) {
	tmpDir := t.TempDir()
	stateDir := filepath.Join(tmpDir, "state")
	os.MkdirAll(stateDir, 0755)

	store := taskpkg.NewTaskStore()
	task := store.Create("", "model", stateDir, "/tmp/reports", "")
	task.UpdateDirs(task.ID, stateDir, "/tmp/reports")
	// No config.json written

	RunAnalysisPipeline(task)

	snap := task.Snapshot()
	if snap.Status != taskpkg.TaskStatusFailed {
		t.Errorf("expected failed, got %s", snap.Status)
	}
	if snap.Error == "" {
		t.Error("expected error message")
	}
	// Scope stage should be failed
	if snap.Stages[0].Status != taskpkg.StageStatusFailed {
		t.Errorf("expected scope stage failed, got %s", snap.Stages[0].Status)
	}
}

func TestPipelineStartsWithRunningStatus(t *testing.T) {
	task, _ := createTestTask(t, "test analysis", "claude-sonnet-4-6")

	// Pipeline will fail at scope (stub returns error), but we can observe the state transitions
	RunAnalysisPipeline(task)

	snap := task.Snapshot()
	// Should have tried to run scope, which fails because stubs return errors
	if snap.Status != taskpkg.TaskStatusFailed {
		t.Errorf("expected failed (stubs not implemented), got %s", snap.Status)
	}
	// Scope stage should be failed (since seed analysis stub returns error)
	if snap.Stages[0].Status != taskpkg.StageStatusFailed {
		t.Errorf("expected scope stage failed, got %s", snap.Stages[0].Status)
	}
	if snap.Stages[0].Detail == "" {
		t.Error("expected scope stage to have failure detail")
	}
	// Remaining stages should still be pending
	for i := 1; i < 4; i++ {
		if snap.Stages[i].Status != taskpkg.StageStatusPending {
			t.Errorf("stage %d: expected pending, got %s", i, snap.Stages[i].Status)
		}
	}
}

// --- Tests for UpdateStageDetail ---

func TestUpdateStageDetail(t *testing.T) {
	task := taskpkg.NewAnalysisTask("ctx-1", "model", "/state", "/reports", "")
	task.StartStage(taskpkg.StageScope, "initial detail")

	snap := task.Snapshot()
	if snap.Stages[0].Detail != "initial detail" {
		t.Errorf("expected 'initial detail', got %q", snap.Stages[0].Detail)
	}

	task.UpdateStageDetail(taskpkg.StageScope, "updated detail")
	snap = task.Snapshot()
	if snap.Stages[0].Detail != "updated detail" {
		t.Errorf("expected 'updated detail', got %q", snap.Stages[0].Detail)
	}
	// Status should still be running (not reset)
	if snap.Stages[0].Status != taskpkg.StageStatusRunning {
		t.Errorf("expected running, got %s", snap.Stages[0].Status)
	}
	// StartedAt should still be set (not reset)
	if snap.Stages[0].StartedAt == nil {
		t.Error("expected StartedAt to remain set")
	}
}

func TestUpdateStageDetailConcurrency(t *testing.T) {
	task := taskpkg.NewAnalysisTask("ctx-conc", "model", "/state", "/reports", "")
	task.StartStage(taskpkg.StageScope, "initial")

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			task.UpdateStageDetail(taskpkg.StageScope, "concurrent update")
		}()
		go func() {
			defer wg.Done()
			_ = task.Snapshot()
		}()
	}
	wg.Wait()
	// If we get here without data race, the test passes
}

// --- Tests for StageResult type ---

func TestStageResultSuccess(t *testing.T) {
	r := StageResult{
		PerspectiveID: "policy-analysis",
		OutputPath:    "/tmp/findings.json",
		Err:           nil,
	}
	if r.Err != nil {
		t.Error("expected nil error for success")
	}
	if r.PerspectiveID != "policy-analysis" {
		t.Errorf("unexpected perspective ID: %s", r.PerspectiveID)
	}
}

func TestStageResultFailure(t *testing.T) {
	r := StageResult{
		PerspectiveID: "ux-analysis",
		Err:           os.ErrNotExist,
	}
	if r.Err == nil {
		t.Error("expected non-nil error for failure")
	}
}

// --- Tests for specialist stage parallel progress tracking ---

func TestSpecialistStageProgressTracking(t *testing.T) {
	task := taskpkg.NewAnalysisTask("ctx-spec", "model", "/state", "/reports", "")
	task.SetStatus(taskpkg.TaskStatusRunning)

	// Simulate what RunAnalysisPipeline does for the specialist stage
	numPerspectives := 5
	task.StartStage(taskpkg.StageSpecialist, "launching 5 specialists")
	task.SetStageParallel(taskpkg.StageSpecialist, numPerspectives)

	// Simulate 3 successes and 2 failures
	task.IncrStageCompleted(taskpkg.StageSpecialist)
	task.IncrStageCompleted(taskpkg.StageSpecialist)
	task.IncrStageCompleted(taskpkg.StageSpecialist)
	task.IncrStageFailed(taskpkg.StageSpecialist)
	task.IncrStageFailed(taskpkg.StageSpecialist)

	snap := task.Snapshot()
	spec := snap.Stages[1] // specialist is index 1
	if spec.Status != taskpkg.StageStatusRunning {
		t.Errorf("expected running, got %s", spec.Status)
	}
	if spec.Total != 5 {
		t.Errorf("expected total 5, got %d", spec.Total)
	}
	if spec.Completed != 3 {
		t.Errorf("expected 3 completed, got %d", spec.Completed)
	}
	if spec.Failed != 2 {
		t.Errorf("expected 2 failed, got %d", spec.Failed)
	}

	// Complete the stage
	task.CompleteStage(taskpkg.StageSpecialist, "3/5 succeeded (2 failed — degraded)")
	snap = task.Snapshot()
	if snap.Stages[1].Status != taskpkg.StageStatusCompleted {
		t.Errorf("expected completed, got %s", snap.Stages[1].Status)
	}
	if snap.Stages[1].EndedAt == nil {
		t.Error("expected EndedAt to be set after completion")
	}
}

// --- Tests for interview stage progress tracking ---

func TestInterviewStageProgressTracking(t *testing.T) {
	task := taskpkg.NewAnalysisTask("ctx-int", "model", "/state", "/reports", "")
	task.SetStatus(taskpkg.TaskStatusRunning)
	task.CompleteStage(taskpkg.StageScope, "done")
	task.CompleteStage(taskpkg.StageSpecialist, "3/3 succeeded")

	// Set up interview stage
	task.StartStage(taskpkg.StageInterview, "launching 3 interviews")
	task.SetStageParallel(taskpkg.StageInterview, 3)

	task.IncrStageCompleted(taskpkg.StageInterview)
	task.IncrStageCompleted(taskpkg.StageInterview)
	task.IncrStageFailed(taskpkg.StageInterview)

	task.CompleteStage(taskpkg.StageInterview, "2/3 verified (1 failed — unverified findings used)")

	snap := task.Snapshot()
	interview := snap.Stages[2] // interview is index 2
	if interview.Status != taskpkg.StageStatusCompleted {
		t.Errorf("expected completed, got %s", interview.Status)
	}
	if interview.Total != 3 {
		t.Errorf("expected total 3, got %d", interview.Total)
	}
	if interview.Completed != 2 {
		t.Errorf("expected 2 completed, got %d", interview.Completed)
	}
	if interview.Failed != 1 {
		t.Errorf("expected 1 failed, got %d", interview.Failed)
	}
}

// --- Test full stage transition sequence ---

func TestFullStageTransitionSequence(t *testing.T) {
	task := taskpkg.NewAnalysisTask("ctx-full", "model", "/state", "/reports", "")

	// Initial state: all stages pending
	snap := task.Snapshot()
	for i, s := range snap.Stages {
		if s.Status != taskpkg.StageStatusPending {
			t.Errorf("stage %d: expected pending initially, got %s", i, s.Status)
		}
	}

	// Simulate complete pipeline
	task.SetStatus(taskpkg.TaskStatusRunning)

	// Stage 1: Scope
	task.StartStage(taskpkg.StageScope, "starting seed analysis")
	task.UpdateStageDetail(taskpkg.StageScope, "running DA review")
	task.UpdateStageDetail(taskpkg.StageScope, "generating perspectives")
	task.CompleteStage(taskpkg.StageScope, "3 perspectives generated")

	snap = task.Snapshot()
	if snap.Stages[0].Status != taskpkg.StageStatusCompleted {
		t.Errorf("scope: expected completed, got %s", snap.Stages[0].Status)
	}
	if snap.Stages[0].StartedAt == nil {
		t.Error("scope: expected StartedAt")
	}
	if snap.Stages[0].EndedAt == nil {
		t.Error("scope: expected EndedAt")
	}

	// Stage 2: Specialist
	task.StartStage(taskpkg.StageSpecialist, "launching 3 specialists")
	task.SetStageParallel(taskpkg.StageSpecialist, 3)
	task.IncrStageCompleted(taskpkg.StageSpecialist)
	task.IncrStageCompleted(taskpkg.StageSpecialist)
	task.IncrStageCompleted(taskpkg.StageSpecialist)
	task.CompleteStage(taskpkg.StageSpecialist, "3/3 succeeded")

	// Stage 3: Interview
	task.StartStage(taskpkg.StageInterview, "launching 3 interviews")
	task.SetStageParallel(taskpkg.StageInterview, 3)
	task.IncrStageCompleted(taskpkg.StageInterview)
	task.IncrStageCompleted(taskpkg.StageInterview)
	task.IncrStageCompleted(taskpkg.StageInterview)
	task.CompleteStage(taskpkg.StageInterview, "3/3 verified")

	// Stage 4: Synthesis
	task.StartStage(taskpkg.StageSynthesis, "generating report")
	task.CompleteStage(taskpkg.StageSynthesis, "report at /reports/report.md")
	task.SetReportPath("/reports/report.md")

	snap = task.Snapshot()
	if snap.Status != taskpkg.TaskStatusCompleted {
		t.Errorf("expected completed, got %s", snap.Status)
	}
	if snap.ReportPath != "/reports/report.md" {
		t.Errorf("expected report path, got %q", snap.ReportPath)
	}
	// All stages should be completed
	for i, s := range snap.Stages {
		if s.Status != taskpkg.StageStatusCompleted {
			t.Errorf("stage %d (%s): expected completed, got %s", i, s.Name, s.Status)
		}
		if s.StartedAt == nil {
			t.Errorf("stage %d (%s): expected StartedAt", i, s.Name)
		}
		if s.EndedAt == nil {
			t.Errorf("stage %d (%s): expected EndedAt", i, s.Name)
		}
	}
}

// --- Test stage failure leaves subsequent stages pending ---

func TestStageFailureLeavesSubsequentPending(t *testing.T) {
	task := taskpkg.NewAnalysisTask("ctx-early-fail", "model", "/state", "/reports", "")
	task.SetStatus(taskpkg.TaskStatusRunning)

	// Scope completes
	task.StartStage(taskpkg.StageScope, "running")
	task.CompleteStage(taskpkg.StageScope, "done")

	// Specialist fails
	task.StartStage(taskpkg.StageSpecialist, "running")
	task.SetStageParallel(taskpkg.StageSpecialist, 3)
	task.IncrStageFailed(taskpkg.StageSpecialist)
	task.IncrStageFailed(taskpkg.StageSpecialist)
	task.IncrStageFailed(taskpkg.StageSpecialist)
	task.FailStage(taskpkg.StageSpecialist, "all 3 specialists failed")
	task.SetError("all specialist analyses failed")

	snap := task.Snapshot()
	if snap.Status != taskpkg.TaskStatusFailed {
		t.Errorf("expected failed, got %s", snap.Status)
	}
	if snap.Stages[0].Status != taskpkg.StageStatusCompleted {
		t.Errorf("scope: expected completed, got %s", snap.Stages[0].Status)
	}
	if snap.Stages[1].Status != taskpkg.StageStatusFailed {
		t.Errorf("specialist: expected failed, got %s", snap.Stages[1].Status)
	}
	// Interview and synthesis should remain pending
	if snap.Stages[2].Status != taskpkg.StageStatusPending {
		t.Errorf("interview: expected pending, got %s", snap.Stages[2].Status)
	}
	if snap.Stages[3].Status != taskpkg.StageStatusPending {
		t.Errorf("synthesis: expected pending, got %s", snap.Stages[3].Status)
	}
}

// --- Test UpdatedAt advances with each state change ---

func TestUpdatedAtAdvances(t *testing.T) {
	task := taskpkg.NewAnalysisTask("ctx-time", "model", "/state", "/reports", "")
	t0 := task.UpdatedAt

	time.Sleep(time.Millisecond)
	task.SetStatus(taskpkg.TaskStatusRunning)
	t1 := task.Snapshot().UpdatedAt
	if !t1.After(t0) {
		t.Error("expected UpdatedAt to advance after SetStatus")
	}

	time.Sleep(time.Millisecond)
	task.StartStage(taskpkg.StageScope, "running")
	t2 := task.Snapshot().UpdatedAt
	if !t2.After(t1) {
		t.Error("expected UpdatedAt to advance after StartStage")
	}

	time.Sleep(time.Millisecond)
	task.UpdateStageDetail(taskpkg.StageScope, "sub-step 2")
	t3 := task.Snapshot().UpdatedAt
	if !t3.After(t2) {
		t.Error("expected UpdatedAt to advance after UpdateStageDetail")
	}

	time.Sleep(time.Millisecond)
	task.CompleteStage(taskpkg.StageScope, "done")
	t4 := task.Snapshot().UpdatedAt
	if !t4.After(t3) {
		t.Error("expected UpdatedAt to advance after CompleteStage")
	}
}

// --- Test AnalysisConfig optional fields ---

func TestReadAnalysisConfigOptionalFields(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := map[string]interface{}{
		"topic":           "test",
		"model":           "claude-sonnet-4-6",
		"task_id":         "analyze-abc",
		"context_id":      "analyze-abc",
		"state_dir":       tmpDir,
		"report_dir":      "/tmp/reports",
		"input_context":   "/path/to/input.md",
		"ontology_scope":  `{"p1": ["/doc/1"]}`,
		"seed_hints":      "focus on security",
		"report_template": "/path/to/template.md",
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(filepath.Join(tmpDir, "config.json"), data, 0644)

	got, err := ReadAnalysisConfig(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.InputContext != "/path/to/input.md" {
		t.Errorf("input_context: got %q", got.InputContext)
	}
	if got.OntologyScope != `{"p1": ["/doc/1"]}` {
		t.Errorf("ontology_scope: got %q", got.OntologyScope)
	}
	if got.SeedHints != "focus on security" {
		t.Errorf("seed_hints: got %q", got.SeedHints)
	}
	if got.ReportTemplate != "/path/to/template.md" {
		t.Errorf("report_template: got %q", got.ReportTemplate)
	}
}
