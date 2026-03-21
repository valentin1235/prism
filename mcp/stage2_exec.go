package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
)

// runSpecialistSession runs a single specialist finding session via claude CLI subprocess.
// It builds the specialist command from the perspective and analysis config, invokes the
// claude CLI with tool access and JSON schema enforcement, parses the structured output,
// and writes findings.json to ~/.prism/state/analyze-{id}/perspectives/{pid}/findings.json.
//
// The subprocess runs in the perspective-specific working directory with no shared state,
// making it safe for concurrent execution via goroutines.
//
// The ctx parameter carries the per-job timeout set by the ParallelExecutor. This function
// does NOT create its own timeout — the executor manages timeouts centrally to ensure
// consistent behavior across all parallel jobs.
func runSpecialistSession(ctx context.Context, task *AnalysisTask, cfg AnalysisConfig, perspective Perspective) error {
	task.mu.RLock()
	stateDir := task.StateDir
	task.mu.RUnlock()

	// Build the specialist command (includes system prompt, user prompt, paths)
	sctx, err := LoadSpecialistContext(cfg)
	if err != nil {
		return fmt.Errorf("load specialist context for %s: %w", perspective.ID, err)
	}

	// Ensure perspective directory exists
	perspDir := PerspectiveDir(stateDir, perspective.ID)
	if err := os.MkdirAll(perspDir, 0755); err != nil {
		return fmt.Errorf("create perspective directory for %s: %w", perspective.ID, err)
	}

	cmd := BuildSpecialistCommand(sctx, perspective)

	log.Printf("[%s] Specialist %s: starting CLI subprocess (model=%s, maxTurns=%d, workDir=%s)",
		task.ID, perspective.ID, cmd.Model, cmd.MaxTurns, cmd.WorkDir)

	// Run claude CLI with tool access and structured output.
	// The ctx already carries a per-job timeout from the ParallelExecutor.
	rawOutput, err := queryLLMScopedWithToolsAndSchema(
		ctx,
		cmd.WorkDir,
		cmd.Model,
		cmd.JSONSchema,
		cmd.SystemPrompt,
		cmd.UserPrompt,
		cmd.MaxTurns,
	)
	if err != nil {
		return fmt.Errorf("specialist %s subprocess: %w", perspective.ID, err)
	}

	// Extract JSON from potentially wrapped output
	jsonStr, err := extractJSON(rawOutput)
	if err != nil {
		return fmt.Errorf("extract specialist %s JSON: %w (raw length: %d)", perspective.ID, err, len(rawOutput))
	}

	// Parse into SpecialistFindings struct
	var findings SpecialistFindings
	if err := json.Unmarshal([]byte(jsonStr), &findings); err != nil {
		return fmt.Errorf("parse specialist %s findings: %w", perspective.ID, err)
	}

	// Validate: must have at least one finding
	if len(findings.Findings) == 0 {
		return fmt.Errorf("specialist %s produced no findings", perspective.ID)
	}

	// Ensure analyst field matches perspective ID
	if findings.Analyst == "" {
		findings.Analyst = perspective.ID
	}

	// Write findings.json to the perspective directory
	findingsPath := FindingsPath(stateDir, perspective.ID)
	if err := WriteSpecialistFindings(findingsPath, findings); err != nil {
		return fmt.Errorf("write specialist %s findings: %w", perspective.ID, err)
	}

	log.Printf("[%s] Specialist %s: completed with %d findings → %s",
		task.ID, perspective.ID, len(findings.Findings), findingsPath)

	return nil
}
