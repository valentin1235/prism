//go:build integration
// +build integration

package pipeline

import (
	"context"
	"testing"
	"time"

	taskpkg "github.com/heechul/prism-mcp/internal/task"
)

func TestLiveRunSeedAnalysisCompletesWithMinimalExplicitScope(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live Codex scope test in short mode")
	}

	stateDir := t.TempDir()
	reportDir := t.TempDir()

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	task := taskpkg.NewAnalysisTask("live-scope", "claude-sonnet-4-6", stateDir, reportDir, "")
	task.Ctx = ctx
	task.Cancel = cancel

	cfg := AnalysisConfig{
		Topic:     "Smoke-test Prism Codex wrapper end-to-end",
		Model:     "claude-sonnet-4-6",
		TaskID:    task.ID,
		ContextID: task.ID,
		StateDir:  stateDir,
		ReportDir: reportDir,
		OntologyScope: `{"sources":[{"id":1,"type":"doc","path":"/tmp/prism-psm-smoke-scope","domain":"smoke","summary":"Smoke test scope","status":"available","access":{"tools":["Read"],"instructions":"Use the Read tool with offset/limit to read files in the directory."}}],"totals":{"doc":1}}`,
	}

	if err := RunSeedAnalysis(task, cfg); err != nil {
		t.Fatalf("RunSeedAnalysis() error = %v", err)
	}
}
