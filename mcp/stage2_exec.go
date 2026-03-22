package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/heechul/prism-mcp/internal/engine"
	taskpkg "github.com/heechul/prism-mcp/internal/task"
)

// runSpecialistSession runs a single specialist finding session via claude CLI subprocess.
// It takes a pre-built SpecialistCommand (constructed via BuildAllSpecialistCommands)
// to avoid redundant LoadSpecialistContext calls per perspective.
//
// The subprocess runs in the perspective-specific working directory with no shared state,
// making it safe for concurrent execution via goroutines.
//
// The ctx parameter carries the per-job timeout set by the ParallelExecutor. This function
// does NOT create its own timeout — the executor manages timeouts centrally to ensure
// consistent behavior across all parallel jobs.
func runSpecialistSession(ctx context.Context, task *taskpkg.AnalysisTask, cmd SpecialistCommand) error {
	log.Printf("[%s] Specialist %s: starting CLI subprocess (model=%s, maxTurns=%d, workDir=%s)",
		task.ID, cmd.PerspectiveID, cmd.Model, cmd.MaxTurns, cmd.WorkDir)

	// Run claude CLI with tool access and structured output.
	// The ctx already carries a per-job timeout from the ParallelExecutor.
	rawOutput, err := engine.QueryLLMScopedWithToolsAndSchema(
		ctx,
		cmd.WorkDir,
		cmd.Model,
		cmd.JSONSchema,
		cmd.SystemPrompt,
		cmd.UserPrompt,
		cmd.MaxTurns,
	)
	if err != nil {
		return fmt.Errorf("specialist %s subprocess: %w", cmd.PerspectiveID, err)
	}

	// Extract JSON from potentially wrapped output
	jsonStr, err := engine.ExtractJSON(rawOutput)
	if err != nil {
		return fmt.Errorf("extract specialist %s JSON: %w (raw length: %d)", cmd.PerspectiveID, err, len(rawOutput))
	}

	// Parse into SpecialistFindings struct
	var findings SpecialistFindings
	if err := json.Unmarshal([]byte(jsonStr), &findings); err != nil {
		return fmt.Errorf("parse specialist %s findings: %w", cmd.PerspectiveID, err)
	}

	// Validate: must have at least one finding
	if len(findings.Findings) == 0 {
		return fmt.Errorf("specialist %s produced no findings", cmd.PerspectiveID)
	}

	// Ensure analyst field matches perspective ID
	if findings.Analyst == "" {
		findings.Analyst = cmd.PerspectiveID
	}

	// Write findings.json to the perspective directory
	if err := WriteSpecialistFindings(cmd.OutputPath, findings); err != nil {
		return fmt.Errorf("write specialist %s findings: %w", cmd.PerspectiveID, err)
	}

	log.Printf("[%s] Specialist %s: completed with %d findings → %s",
		task.ID, cmd.PerspectiveID, len(findings.Findings), cmd.OutputPath)

	return nil
}
