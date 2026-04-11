package analysisstore

import (
	"testing"
	"time"

	taskpkg "github.com/heechul/prism-mcp/internal/task"
)

func TestSaveAnalysisConfigRejectsAdaptorMutation(t *testing.T) {
	baseDir := t.TempDir()
	record := AnalysisConfigRecord{
		TaskID:    "analyze-immutable",
		Topic:     "topic",
		Model:     "default",
		Adaptor:   "codex",
		ContextID: "analyze-immutable",
		StateDir:  "/tmp/state",
		ReportDir: "/tmp/report",
	}

	if err := SaveAnalysisConfig(baseDir, record); err != nil {
		t.Fatalf("initial save failed: %v", err)
	}

	record.Topic = "updated topic"
	if err := SaveAnalysisConfig(baseDir, record); err != nil {
		t.Fatalf("same adaptor update should succeed: %v", err)
	}

	record.Adaptor = "claude"
	if err := SaveAnalysisConfig(baseDir, record); err == nil {
		t.Fatal("expected adaptor mutation to fail")
	}
}

func TestSaveAndLoadTaskSnapshot(t *testing.T) {
	baseDir := t.TempDir()
	task := taskpkg.NewAnalysisTask("ctx-1", "default", "/tmp/state", "/tmp/report", "")

	if err := SaveAnalysisConfig(baseDir, AnalysisConfigRecord{
		TaskID:    task.ID,
		Topic:     "topic",
		Model:     "default",
		Adaptor:   "codex",
		ContextID: task.ContextID,
		StateDir:  "/tmp/state",
		ReportDir: "/tmp/report",
	}); err != nil {
		t.Fatalf("save analysis config: %v", err)
	}

	task.SetStatus(taskpkg.TaskStatusRunning)
	task.StartStage(taskpkg.StageScope, "seed analysis")
	snapshot := task.Snapshot()

	if err := SaveTaskSnapshot(baseDir, snapshot, 3); err != nil {
		t.Fatalf("save task snapshot: %v", err)
	}

	loaded, pollCount, ok, err := LoadTaskSnapshot(baseDir, task.ID)
	if err != nil {
		t.Fatalf("load task snapshot: %v", err)
	}
	if !ok {
		t.Fatal("expected task snapshot to exist")
	}
	if pollCount != 3 {
		t.Fatalf("expected poll count 3, got %d", pollCount)
	}
	if loaded.Status != taskpkg.TaskStatusRunning {
		t.Fatalf("expected running status, got %s", loaded.Status)
	}
	if len(loaded.Stages) != 4 {
		t.Fatalf("expected 4 stages, got %d", len(loaded.Stages))
	}
	if loaded.Stages[0].Detail != "seed analysis" {
		t.Fatalf("expected scope detail to round-trip, got %q", loaded.Stages[0].Detail)
	}
}

func TestSaveAnalysisConfigResetsLifecycleForDeterministicRerun(t *testing.T) {
	baseDir := t.TempDir()
	record := AnalysisConfigRecord{
		TaskID:    "analyze-rerun",
		Topic:     "topic",
		Model:     "default",
		Adaptor:   "codex",
		ContextID: "analyze-rerun",
		StateDir:  "/tmp/state",
		ReportDir: "/tmp/report",
	}

	if err := SaveAnalysisConfig(baseDir, record); err != nil {
		t.Fatalf("initial save: %v", err)
	}
	initialSnapshot, _, ok, err := LoadTaskSnapshot(baseDir, record.TaskID)
	if err != nil || !ok {
		t.Fatalf("load initial snapshot: ok=%v err=%v", ok, err)
	}

	task := taskpkg.NewAnalysisTask("analyze-rerun", "default", "/tmp/state", "/tmp/report", "rerun")
	task.SetStatus(taskpkg.TaskStatusRunning)
	task.StartStage(taskpkg.StageScope, "running")
	task.SetReportPath("/tmp/report/final.md")
	if err := SaveTaskSnapshot(baseDir, task.Snapshot(), 7); err != nil {
		t.Fatalf("save completed snapshot: %v", err)
	}

	record.Topic = "rerun topic"
	time.Sleep(10 * time.Millisecond)
	if err := SaveAnalysisConfig(baseDir, record); err != nil {
		t.Fatalf("rerun save: %v", err)
	}

	snapshot, pollCount, ok, err := LoadTaskSnapshot(baseDir, record.TaskID)
	if err != nil {
		t.Fatalf("load rerun snapshot: %v", err)
	}
	if !ok {
		t.Fatal("expected rerun snapshot to exist")
	}
	if snapshot.Status != taskpkg.TaskStatusQueued {
		t.Fatalf("expected queued after rerun reset, got %s", snapshot.Status)
	}
	if snapshot.ReportPath != "" || snapshot.Error != "" {
		t.Fatalf("expected cleared terminal fields after rerun, got report=%q error=%q", snapshot.ReportPath, snapshot.Error)
	}
	if pollCount != 0 {
		t.Fatalf("expected poll count reset to 0, got %d", pollCount)
	}
	if len(snapshot.Stages) != 4 || snapshot.Stages[0].Status != taskpkg.StageStatusPending {
		t.Fatalf("expected stages reset to pending, got %+v", snapshot.Stages)
	}
	if !snapshot.CreatedAt.After(initialSnapshot.CreatedAt) {
		t.Fatalf("expected rerun created_at to advance, before=%s after=%s", initialSnapshot.CreatedAt, snapshot.CreatedAt)
	}
}
