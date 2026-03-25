package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/heechul/prism-mcp/internal/engine"
	"github.com/heechul/prism-mcp/internal/pipeline"
	"github.com/mark3labs/mcp-go/mcp"
)

// HandleDAReview is the MCP tool handler for prism_da_review.
func HandleDAReview(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.Params.Arguments

	// Extract parameters
	seedAnalysisPath, _ := args["seed_analysis_path"].(string)
	extraContext, _ := args["context"].(string)

	// Extract round parameter (defaults to 1 if not provided)
	round := 1
	if r, ok := args["round"].(float64); ok && r > 0 {
		round = int(r)
	}

	// Validate required parameter
	if seedAnalysisPath == "" {
		return mcp.NewToolResultError("seed_analysis_path is required"), nil
	}

	// Path validation: restrict to ~/.prism/state/ or /tmp/ to prevent arbitrary file reads.
	// Resolve symlinks to prevent bypass via ~/.prism/state/evil -> /etc/passwd.
	cleanPath := filepath.Clean(seedAnalysisPath)
	resolvedPath, err := filepath.EvalSymlinks(cleanPath)
	if err != nil {
		// File may not exist yet on first call — fall back to Clean'd path
		resolvedPath = cleanPath
	}
	homeDir, _ := os.UserHomeDir()
	prismStateDir := filepath.Join(homeDir, ".prism", "state")
	if !strings.HasPrefix(resolvedPath, prismStateDir) && !strings.HasPrefix(resolvedPath, "/tmp/") {
		return mcp.NewToolResultError(fmt.Sprintf("seed_analysis_path must be within %s or /tmp/, got: %s", prismStateDir, resolvedPath)), nil
	}

	// Hard-stop: if round exceeds MaxDARounds, return immediately without calling LLM
	if round > pipeline.MaxDARounds {
		hardStopResult := pipeline.DAReviewResult{
			Pass:      false,
			Round:     round,
			MaxRounds: pipeline.MaxDARounds,
			HardStop:  true,
			Gaps:      []pipeline.DAGap{},
			RawOutput: fmt.Sprintf("hard stop: round %d exceeds maximum of %d rounds", round, pipeline.MaxDARounds),
		}
		resultJSON, _ := json.MarshalIndent(hardStopResult, "", "  ")
		return mcp.NewToolResultText(string(resultJSON)), nil
	}

	// Read seed-analysis.json from disk FRESH each round.
	seedData, err := os.ReadFile(seedAnalysisPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to read seed analysis: %v", err)), nil
	}

	// Validate it's valid JSON
	var seedJSON map[string]interface{}
	if err := json.Unmarshal(seedData, &seedJSON); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid JSON in seed analysis: %v", err)), nil
	}

	// Use seed-analysis-specialized DA prompt (Go-defined, no file I/O)
	daPrompt := pipeline.SeedAnalysisDAPrompt

	// Build user prompt with the COMPLETE seed analysis content.
	var userPrompt strings.Builder
	userPrompt.WriteString("Apply your full 4-phase protocol to critique this seed analysis. Identify perspective biases and codebase coverage gaps:\n\n")
	userPrompt.WriteString(string(seedData))

	if extraContext != "" {
		userPrompt.WriteString("\n\n---\n\nAdditional context from the caller:\n")
		userPrompt.WriteString(extraContext)
	}

	// Call LLM with DA system prompt separated from user message
	rawOutput, err := engine.QueryLLMWithSystemPrompt(ctx, daPrompt, userPrompt.String())
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("DA review LLM call failed: %v", err)), nil
	}

	// Parse gaps from markdown
	gaps := pipeline.ParseDAGaps(rawOutput)
	overallConfidence, topConcerns, whatHoldsUp := pipeline.ParseDASummary(rawOutput)

	// Detect parse failure
	var parseWarning string
	if len(gaps) == 0 && pipeline.GapKeywordRe.MatchString(rawOutput) {
		parseWarning = "WARNING: No gaps were parsed from DA output, but bias/coverage keywords were detected in the raw output. The DA likely produced gaps in a non-standard format. Check raw_output for details."
	}

	// Count by gap type
	biasCount, coverageCount := pipeline.CountGapsByType(gaps)

	// Pass (signals early loop termination) when no gaps found
	pass := pipeline.ShouldPassDAGaps(gaps)

	// On the final allowed round, hard_stop signals the caller to exit regardless of pass
	hardStop := round >= pipeline.MaxDARounds

	result := pipeline.DAReviewResult{
		Pass:              pass,
		GapCount:          len(gaps),
		BiasCount:         biasCount,
		CoverageCount:     coverageCount,
		Gaps:              gaps,
		Round:             round,
		MaxRounds:         pipeline.MaxDARounds,
		HardStop:          hardStop,
		ParseWarning:      parseWarning,
		OverallConfidence: overallConfidence,
		TopConcerns:       topConcerns,
		WhatHoldsUp:       whatHoldsUp,
		RawOutput:         rawOutput,
	}

	resultJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal result: %v", err)), nil
	}

	return mcp.NewToolResultText(string(resultJSON)), nil
}
