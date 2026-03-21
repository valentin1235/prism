package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

// runSeedAnalysis runs the seed analyst via claude CLI subprocess with --json-schema.
// The subprocess uses tools (Grep, Read, Glob, Bash) for breadth-first research
// across the ontology document directories. Output is parsed into SeedAnalysis
// and written to seed-analysis.json in the task's state directory.
//
// The seed analyst focuses purely on breadth of discovery. DA review is handled
// separately by runDAReviewLoop after this step completes.
func runSeedAnalysis(task *AnalysisTask, cfg AnalysisConfig) error {
	task.mu.RLock()
	stateDir := task.StateDir
	task.mu.RUnlock()

	// Resolve ontology doc paths for targeted search
	docPaths := LoadOntologyDocPaths()

	// Build the seed analyst system prompt
	systemPrompt := BuildSeedAnalystPrompt(
		cfg.Topic,
		cfg.ContextID,
		cfg.SeedHints,
		cfg.OntologyScope,
		docPaths,
	)

	// User prompt is a concise task instruction
	userPrompt := fmt.Sprintf(
		"Investigate this topic using available tools and output your findings as structured JSON:\n\n%s",
		cfg.Topic,
	)

	// Run claude CLI with tool access and structured output
	// 10-minute timeout for multi-turn tool-using research
	ctx, cancel := context.WithTimeout(task.Ctx, 10*time.Minute)
	defer cancel()

	rawOutput, err := queryLLMScopedWithToolsAndSchema(
		ctx,
		stateDir,
		cfg.Model,
		SeedAnalysisSchema(),
		systemPrompt,
		userPrompt,
		10, // max turns for tool use
	)
	if err != nil {
		return fmt.Errorf("seed analysis subprocess: %w", err)
	}

	// Extract JSON from potentially wrapped output
	jsonStr, err := extractJSON(rawOutput)
	if err != nil {
		return fmt.Errorf("extract seed analysis JSON: %w (raw length: %d)", err, len(rawOutput))
	}

	// Parse into SeedAnalysis struct
	var seedAnalysis SeedAnalysis
	if err := json.Unmarshal([]byte(jsonStr), &seedAnalysis); err != nil {
		return fmt.Errorf("parse seed analysis: %w", err)
	}

	// Basic validation: must have at least one finding
	if len(seedAnalysis.Research.Findings) == 0 {
		return fmt.Errorf("seed analysis produced no findings")
	}

	// Write seed-analysis.json to state directory
	outputPath := SeedAnalysisPath(stateDir)
	if err := WriteSeedAnalysis(outputPath, seedAnalysis); err != nil {
		return fmt.Errorf("write seed analysis: %w", err)
	}

	log.Printf("[%s] Seed analysis complete: %d findings, %d key areas",
		task.ID, len(seedAnalysis.Research.Findings), len(seedAnalysis.Research.KeyAreas))

	return nil
}

// runDAReviewLoop executes the Devil's Advocate review loop (up to 3 rounds).
// Each round:
//  1. Read current seed-analysis.json
//  2. Run DA review (uses existing DA prompt + LLM call)
//  3. If no CRITICAL/MAJOR findings → pass, set da_passed=true, exit
//  4. If actionable findings found and rounds remain → run supplementary research
//  5. Merge new findings into seed-analysis.json
//  6. Repeat
//
// On supplementary research failure, the loop continues with existing findings
// rather than failing the entire pipeline.
func runDAReviewLoop(task *AnalysisTask, cfg AnalysisConfig) error {
	task.mu.RLock()
	stateDir := task.StateDir
	task.mu.RUnlock()

	seedPath := SeedAnalysisPath(stateDir)

	for round := 1; round <= maxDARounds; round++ {
		task.UpdateStageDetail(StageScope, fmt.Sprintf("DA review round %d/%d", round, maxDARounds))
		log.Printf("[%s] DA review round %d/%d", task.ID, round, maxDARounds)

		// Read current seed analysis fresh each round
		seedData, err := os.ReadFile(seedPath)
		if err != nil {
			return fmt.Errorf("read seed analysis for DA round %d: %w", round, err)
		}

		// Load DA system prompt
		daPrompt, err := loadDASystemPrompt()
		if err != nil {
			return fmt.Errorf("load DA system prompt: %w", err)
		}

		// Build user prompt for DA review — full seed analysis content
		userPrompt := fmt.Sprintf(
			"Apply your full 4-phase protocol to critique this seed analysis. Evaluate the ENTIRE content holistically — assess all findings, coverage gaps, and potential biases across the complete analysis:\n\n%s",
			string(seedData),
		)

		// Call LLM for DA review (5-minute timeout per round)
		ctx, cancel := context.WithTimeout(task.Ctx, 5*time.Minute)
		rawOutput, err := queryLLMScopedWithSystemPrompt(ctx, stateDir, cfg.Model, daPrompt, userPrompt)
		cancel()
		if err != nil {
			return fmt.Errorf("DA review LLM call round %d: %w", round, err)
		}

		// Parse DA findings from markdown output
		findings := parseDAFindings(rawOutput)
		actionable := filterActionableFindings(findings)
		criticalCount, majorCount := countSeverities(actionable)
		pass := shouldPassDA(criticalCount, majorCount)

		// Detect parse failure: no findings extracted but severity keywords present
		// in raw output. The DA likely produced non-standard markdown. Treat as
		// not-passed to avoid false positive pass on parse failure.
		if pass && len(findings) == 0 && severityKeywordRe.MatchString(rawOutput) {
			pass = false
			log.Printf("[%s] DA round %d: parse warning — no findings parsed but CRITICAL/MAJOR keywords detected in raw output; treating as not-passed",
				task.ID, round)
		}

		log.Printf("[%s] DA round %d: pass=%v critical=%d major=%d total_actionable=%d",
			task.ID, round, pass, criticalCount, majorCount, len(actionable))

		if pass {
			// No actionable findings — update da_passed and exit loop
			sa, err := ReadSeedAnalysis(seedPath)
			if err != nil {
				return fmt.Errorf("read seed for DA pass update: %w", err)
			}
			sa.DAPassed = true
			if err := WriteSeedAnalysis(seedPath, sa); err != nil {
				return fmt.Errorf("write seed DA pass: %w", err)
			}
			log.Printf("[%s] DA review passed at round %d", task.ID, round)
			return nil
		}

		// Last allowed round — stop regardless of findings
		if round >= maxDARounds {
			log.Printf("[%s] DA review hard stop at round %d with %d actionable findings",
				task.ID, round, len(actionable))
			break
		}

		// Actionable findings found — run supplementary research to address gaps
		task.UpdateStageDetail(StageScope, fmt.Sprintf("DA round %d: re-researching %d issues", round, len(actionable)))
		if err := runSupplementaryResearch(task, cfg, actionable); err != nil {
			// Log but don't fail — continue with existing findings
			log.Printf("[%s] Supplementary research failed round %d: %v — continuing with existing findings",
				task.ID, round, err)
		}
	}

	return nil
}

// runSupplementaryResearch runs a focused research subprocess to address
// specific gaps identified by the DA review. New findings are merged into
// the existing seed-analysis.json using MergeSeedAnalysis.
func runSupplementaryResearch(task *AnalysisTask, cfg AnalysisConfig, findings []DAFinding) error {
	task.mu.RLock()
	stateDir := task.StateDir
	task.mu.RUnlock()

	// Build focused re-research system prompt
	var sb strings.Builder
	sb.WriteString("You are conducting SUPPLEMENTARY RESEARCH to address specific gaps identified by a Devil's Advocate review.\n\n")
	sb.WriteString("ORIGINAL TOPIC:\n")
	sb.WriteString(cfg.Topic)
	sb.WriteString("\n\nThe DA review found these issues that need investigation:\n\n")
	for i, f := range findings {
		sb.WriteString(fmt.Sprintf("%d. [%s] %s\n", i+1, f.Severity, f.Title))
		if f.Concern != "" {
			sb.WriteString(fmt.Sprintf("   Concern: %s\n", f.Concern))
		}
		if f.FalsificationTest != "" {
			sb.WriteString(fmt.Sprintf("   Falsification test: %s\n", f.FalsificationTest))
		}
	}
	sb.WriteString("\nInvestigate ONLY these specific gaps. Use tools (Grep, Read, Glob, Bash) to find concrete evidence.\n")
	sb.WriteString("Output your additional findings as structured JSON following the same schema.\n")

	// Include ontology doc paths for targeted search
	docPaths := LoadOntologyDocPaths()
	if len(docPaths) > 0 {
		sb.WriteString("\n## Analysis Target Directories\n\n")
		for _, p := range docPaths {
			sb.WriteString(fmt.Sprintf("- %s\n", p))
		}
	}

	// Run focused research subprocess (5-minute timeout, 8 max turns)
	ctx, cancel := context.WithTimeout(task.Ctx, 5*time.Minute)
	defer cancel()

	rawOutput, err := queryLLMScopedWithToolsAndSchema(
		ctx,
		stateDir,
		cfg.Model,
		SeedAnalysisSchema(),
		sb.String(),
		"Investigate the DA critique gaps listed above and output additional findings as structured JSON.",
		8, // fewer turns than initial research
	)
	if err != nil {
		return fmt.Errorf("supplementary research subprocess: %w", err)
	}

	// Extract and parse JSON
	jsonStr, err := extractJSON(rawOutput)
	if err != nil {
		return fmt.Errorf("extract supplementary JSON: %w (raw length: %d)", err, len(rawOutput))
	}

	var supplementary SeedAnalysis
	if err := json.Unmarshal([]byte(jsonStr), &supplementary); err != nil {
		return fmt.Errorf("parse supplementary research: %w", err)
	}

	// Merge supplementary findings into existing seed analysis
	seedPath := SeedAnalysisPath(stateDir)
	patch := SeedPatch{
		NewFindings:      supplementary.Research.Findings,
		NewKeyAreas:      supplementary.Research.KeyAreas,
		NewFilesExamined: supplementary.Research.FilesExamined,
	}
	if supplementary.Research.Summary != "" {
		patch.Summary = supplementary.Research.Summary
	}

	merged, err := PatchSeedAnalysisFile(seedPath, patch)
	if err != nil {
		return fmt.Errorf("merge supplementary research: %w", err)
	}

	log.Printf("[%s] Supplementary research merged: %d total findings",
		task.ID, len(merged.Research.Findings))

	return nil
}

// runPerspectiveGeneration runs the perspective generator via claude CLI subprocess
// with --json-schema for structured output. This is a single-turn call — all input
// (seed-analysis.json content) is provided inline in the prompt, so no tool access
// is needed.
//
// The output is parsed into PerspectivesOutput, validated with ValidatePerspectives,
// and written to perspectives.json in the task's state directory.
func runPerspectiveGeneration(task *AnalysisTask, cfg AnalysisConfig) error {
	task.mu.RLock()
	stateDir := task.StateDir
	task.mu.RUnlock()

	// Read seed analysis for inline inclusion in the prompt
	seedPath := SeedAnalysisPath(stateDir)
	seedData, err := os.ReadFile(seedPath)
	if err != nil {
		return fmt.Errorf("read seed analysis for perspective generation: %w", err)
	}

	// Build the perspective generator prompt with seed analysis inlined
	prompt := BuildPerspectiveGeneratorPrompt(cfg.Topic, string(seedData))

	// Run claude CLI with structured output (single-turn, no tools)
	// 5-minute timeout for perspective generation
	ctx, cancel := context.WithTimeout(task.Ctx, 5*time.Minute)
	defer cancel()

	rawOutput, err := queryLLMScopedWithSchema(
		ctx,
		stateDir,
		cfg.Model,
		PerspectivesSchema(),
		prompt,
	)
	if err != nil {
		return fmt.Errorf("perspective generation subprocess: %w", err)
	}

	// Extract JSON from potentially wrapped output
	jsonStr, err := extractJSON(rawOutput)
	if err != nil {
		return fmt.Errorf("extract perspectives JSON: %w (raw length: %d)", err, len(rawOutput))
	}

	// Parse into PerspectivesOutput struct
	var perspectives PerspectivesOutput
	if err := json.Unmarshal([]byte(jsonStr), &perspectives); err != nil {
		return fmt.Errorf("parse perspectives: %w", err)
	}

	// Validate the generated perspectives
	if err := ValidatePerspectives(perspectives); err != nil {
		return fmt.Errorf("validate perspectives: %w", err)
	}

	// Write perspectives.json to state directory
	outputPath := PerspectivesPath(stateDir)
	if err := WritePerspectives(outputPath, perspectives); err != nil {
		return fmt.Errorf("write perspectives: %w", err)
	}

	log.Printf("[%s] Perspective generation complete: %d perspectives — %s",
		task.ID, len(perspectives.Perspectives), perspectives.SelectionSummary)

	return nil
}
