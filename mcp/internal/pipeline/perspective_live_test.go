//go:build integration
// +build integration

package pipeline

import (
	"context"
	"os"
	"testing"
	"time"

	taskpkg "github.com/heechul/prism-mcp/internal/task"
)

func TestLiveRunPerspectiveGenerationFromExistingState(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live perspective generation test in short mode")
	}

	stateDir := os.Getenv("PRISM_TEST_STATE_DIR")
	if stateDir == "" {
		t.Skip("PRISM_TEST_STATE_DIR not set")
	}

	reportDir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	task := taskpkg.NewAnalysisTask("live-perspective", "claude-sonnet-4-6", stateDir, reportDir, "")
	task.Ctx = ctx
	task.Cancel = cancel

	cfg, err := ReadAnalysisConfig(stateDir)
	if err != nil {
		t.Fatalf("ReadAnalysisConfig() error = %v", err)
	}

	if err := RunPerspectiveGeneration(task, cfg); err != nil {
		t.Fatalf("RunPerspectiveGeneration() error = %v", err)
	}
}
