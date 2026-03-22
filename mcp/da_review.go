package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/heechul/prism-mcp/internal/engine"
	"github.com/mark3labs/mcp-go/mcp"
)

// DAFinding represents a single finding extracted from DA markdown output.
type DAFinding struct {
	Section          string `json:"section"`
	Title            string `json:"title"`
	Claim            string `json:"claim"`
	Concern          string `json:"concern"`
	Confidence       string `json:"confidence"`
	Severity         string `json:"severity"`
	FalsificationTest string `json:"falsification_test"`
}

// maxDARounds is the hard limit for the DA review loop.
// After this many rounds, the loop must stop regardless of findings.
const maxDARounds = 3

// DAReviewResult is the structured result returned by prism_da_review.
type DAReviewResult struct {
	Pass             bool        `json:"pass"`
	CriticalCount    int         `json:"critical_count"`
	MajorCount       int         `json:"major_count"`
	Findings         []DAFinding `json:"findings"`
	Round            int         `json:"round"`
	MaxRounds        int         `json:"max_rounds"`
	HardStop         bool        `json:"hard_stop"`
	ParseWarning     string      `json:"parse_warning,omitempty"`
	OverallConfidence string     `json:"overall_confidence"`
	TopConcerns      string     `json:"top_concerns"`
	WhatHoldsUp      string     `json:"what_holds_up"`
	RawOutput        string     `json:"raw_output"`
}

// Package-level compiled regexes for DA markdown parsing.
// Hoisted from parseDAFindings/parseDASummary to avoid recompilation on every call.
var (
	// severityKeywordRe detects severity keywords in raw output for parse failure detection.
	severityKeywordRe = regexp.MustCompile(`(?i)\b(CRITICAL|MAJOR)\b`)

	// Section headers: ## Challenged Framings, ## Missing Perspectives, etc.
	sectionRe = regexp.MustCompile(`(?m)^## (Challenged Framings|Missing Perspectives|Bias Indicators|Alternative Framings)\s*$`)
	// Finding titles: ### [title]
	findingTitleRe = regexp.MustCompile(`(?m)^### (.+)$`)
	// Fields within findings
	claimRe         = regexp.MustCompile(`(?m)^-\s+\*\*Claim\*\*:\s*(.+)$`)
	concernRe       = regexp.MustCompile(`(?m)^-\s+\*\*Concern\*\*:\s*(.+)$`)
	confidenceRe    = regexp.MustCompile("(?m)^-\\s+\\*\\*Confidence\\*\\*:\\s*`?(HIGH|MEDIUM|LOW)`?")
	severityRe      = regexp.MustCompile("(?m)^-\\s+\\*\\*Severity\\*\\*:\\s*`?(CRITICAL|MAJOR|MINOR)`?")
	falsificationRe = regexp.MustCompile(`(?m)^-\s+\*\*Falsification test\*\*:\s*(.+)$`)
	// Next ## header (for section boundary detection)
	nextHeaderRe = regexp.MustCompile(`(?m)^## `)
	// Summary fields
	summaryConfRe  = regexp.MustCompile("(?m)^-\\s+\\*\\*Overall confidence\\*\\*:\\s*`?(HIGH|MEDIUM|LOW)`?\\s*[—–-]?\\s*(.*)")
	summaryTopRe   = regexp.MustCompile(`(?m)^-\s+\*\*Top concerns\*\*:\s*(.+)`)
	summaryHoldsRe = regexp.MustCompile(`(?m)^-\s+\*\*What holds up\*\*:\s*(.+)`)
)

// loadDASystemPrompt reads the devils-advocate.md from the agents directory.
func loadDASystemPrompt() (string, error) {
	// Resolve path relative to the binary's expected location
	// The agents dir is at the repo root, sibling to mcp/
	candidates := []string{
		filepath.Join(getRepoRoot(), "agents", "devils-advocate.md"),
	}

	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if err == nil {
			return string(data), nil
		}
	}

	return "", fmt.Errorf("devils-advocate.md not found")
}

// getRepoRoot determines the repository root from the executable path or known markers.
func getRepoRoot() string {
	marker := filepath.Join("agents", "devils-advocate.md")
	var tried []string

	// First priority: PRISM_ROOT environment variable
	if root := os.Getenv("PRISM_ROOT"); root != "" {
		if _, err := os.Stat(filepath.Join(root, marker)); err == nil {
			return root
		}
		tried = append(tried, "PRISM_ROOT="+root)
	}

	// Try from executable path (binary is in mcp/bin/ or mcp/)
	exe, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exe)
		for _, rel := range []string{filepath.Join("..", ".."), ".."} {
			candidate := filepath.Join(exeDir, rel)
			abs, absErr := filepath.Abs(candidate)
			if absErr != nil {
				continue
			}
			if _, err := os.Stat(filepath.Join(abs, marker)); err == nil {
				return abs
			}
			tried = append(tried, abs)
		}
	}

	// Fallback: check working directory patterns
	cwd, _ := os.Getwd()
	for _, dir := range []string{cwd, filepath.Join(cwd, ".."), filepath.Join(cwd, "..", "..")} {
		abs, absErr := filepath.Abs(dir)
		if absErr != nil {
			continue
		}
		if _, err := os.Stat(filepath.Join(abs, marker)); err == nil {
			return abs
		}
		tried = append(tried, abs)
	}

	log.Printf("WARNING: could not locate %s. Tried paths: %v. Set PRISM_ROOT to override.", marker, tried)
	return cwd
}

// parseDAFindings extracts structured findings from DA markdown output using regex.
// All regex patterns are compiled once at package level for performance.
func parseDAFindings(raw string) []DAFinding {
	var findings []DAFinding

	// Find all section positions
	sectionMatches := sectionRe.FindAllStringSubmatchIndex(raw, -1)
	if len(sectionMatches) == 0 {
		return findings
	}

	for i, sm := range sectionMatches {
		sectionName := raw[sm[2]:sm[3]]

		// Determine section text boundaries
		sectionStart := sm[1]
		var sectionEnd int
		if i+1 < len(sectionMatches) {
			sectionEnd = sectionMatches[i+1][0]
		} else {
			// Find the next ## header that isn't one of the 4 sections
			remaining := raw[sectionStart:]
			loc := nextHeaderRe.FindStringIndex(remaining)
			if loc != nil {
				sectionEnd = sectionStart + loc[0]
			} else {
				sectionEnd = len(raw)
			}
		}

		sectionText := raw[sectionStart:sectionEnd]

		// Skip "None identified." sections
		if strings.Contains(sectionText, "None identified") {
			continue
		}

		// Find findings within this section
		titleMatches := findingTitleRe.FindAllStringSubmatchIndex(sectionText, -1)
		for j, tm := range titleMatches {
			title := sectionText[tm[2]:tm[3]]

			// Get finding text
			findingStart := tm[1]
			var findingEnd int
			if j+1 < len(titleMatches) {
				findingEnd = titleMatches[j+1][0]
			} else {
				findingEnd = len(sectionText)
			}
			findingText := sectionText[findingStart:findingEnd]

			finding := DAFinding{
				Section: sectionName,
				Title:   strings.TrimSpace(title),
			}

			if m := claimRe.FindStringSubmatch(findingText); len(m) > 1 {
				finding.Claim = strings.TrimSpace(m[1])
			}
			if m := concernRe.FindStringSubmatch(findingText); len(m) > 1 {
				finding.Concern = strings.TrimSpace(m[1])
			}
			if m := confidenceRe.FindStringSubmatch(findingText); len(m) > 1 {
				finding.Confidence = m[1]
			}
			if m := severityRe.FindStringSubmatch(findingText); len(m) > 1 {
				finding.Severity = m[1]
			}
			if m := falsificationRe.FindStringSubmatch(findingText); len(m) > 1 {
				finding.FalsificationTest = strings.TrimSpace(m[1])
			}

			findings = append(findings, finding)
		}
	}

	return findings
}

// parseDASummary extracts summary fields from DA markdown output.
// All regex patterns are compiled once at package level for performance.
func parseDASummary(raw string) (overallConfidence, topConcerns, whatHoldsUp string) {
	if m := summaryConfRe.FindStringSubmatch(raw); len(m) > 1 {
		overallConfidence = m[1]
	}
	if m := summaryTopRe.FindStringSubmatch(raw); len(m) > 1 {
		topConcerns = strings.TrimSpace(m[1])
	}
	if m := summaryHoldsRe.FindStringSubmatch(raw); len(m) > 1 {
		whatHoldsUp = strings.TrimSpace(m[1])
	}
	return
}

// countSeverities counts CRITICAL and MAJOR findings in the given slice.
func countSeverities(findings []DAFinding) (criticalCount, majorCount int) {
	for _, f := range findings {
		switch f.Severity {
		case "CRITICAL":
			criticalCount++
		case "MAJOR":
			majorCount++
		}
	}
	return
}

// shouldPassDA returns true when there are no CRITICAL or MAJOR findings,
// signaling that the DA review loop can terminate early.
func shouldPassDA(criticalCount, majorCount int) bool {
	return criticalCount == 0 && majorCount == 0
}

// filterActionableFindings returns only CRITICAL and MAJOR severity findings,
// discarding MINOR, INFO, and any unrecognized severity levels.
func filterActionableFindings(findings []DAFinding) []DAFinding {
	var actionable []DAFinding
	for _, f := range findings {
		if f.Severity == "CRITICAL" || f.Severity == "MAJOR" {
			actionable = append(actionable, f)
		}
	}
	return actionable
}

func handleDAReview(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

	// Hard-stop: if round exceeds maxDARounds, return immediately without calling LLM
	if round > maxDARounds {
		hardStopResult := DAReviewResult{
			Pass:      false,
			Round:     round,
			MaxRounds: maxDARounds,
			HardStop:  true,
			Findings:  []DAFinding{},
			RawOutput: fmt.Sprintf("hard stop: round %d exceeds maximum of %d rounds", round, maxDARounds),
		}
		resultJSON, _ := json.MarshalIndent(hardStopResult, "", "  ")
		return mcp.NewToolResultText(string(resultJSON)), nil
	}

	// Read seed-analysis.json from disk FRESH each round.
	// This is intentional: the DA evaluates the entire seed-analysis.json from scratch
	// each round, not just incremental changes. After the seed analyst adds new findings
	// in response to DA critique, the next DA round re-reads the updated file and
	// evaluates ALL findings (original + newly added) holistically. This ensures the DA
	// can assess whether new findings adequately address previous concerns AND can
	// identify new gaps that emerge from the expanded coverage.
	seedData, err := os.ReadFile(seedAnalysisPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to read seed analysis: %v", err)), nil
	}

	// Validate it's valid JSON
	var seedJSON map[string]interface{}
	if err := json.Unmarshal(seedData, &seedJSON); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid JSON in seed analysis: %v", err)), nil
	}

	// Load DA system prompt
	daPrompt, err := loadDASystemPrompt()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to load DA system prompt: %v", err)), nil
	}

	// Build user prompt with the COMPLETE seed analysis content.
	// The DA receives the entire file — not a diff or summary — so it can
	// holistically evaluate coverage sufficiency across all findings.
	var userPrompt strings.Builder
	userPrompt.WriteString("Apply your full 4-phase protocol to critique this seed analysis. Evaluate the ENTIRE content holistically — assess all findings, coverage gaps, and potential biases across the complete analysis:\n\n")
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

	// Parse findings from markdown
	findings := parseDAFindings(rawOutput)
	overallConfidence, topConcerns, whatHoldsUp := parseDASummary(rawOutput)

	// Detect parse failure: if no findings extracted but raw output contains severity keywords,
	// the LLM likely produced slightly non-standard markdown that our regex missed.
	var parseWarning string
	if len(findings) == 0 && severityKeywordRe.MatchString(rawOutput) {
		parseWarning = "WARNING: No findings were parsed from DA output, but severity keywords (CRITICAL/MAJOR) were detected in the raw output. The DA likely produced findings in a non-standard format. Check raw_output for details."
	}

	// Filter to only actionable (CRITICAL/MAJOR) findings, discard MINOR/INFO
	actionable := filterActionableFindings(findings)

	// Count by severity using shared helper
	criticalCount, majorCount := countSeverities(actionable)

	// Pass (signals early loop termination) when critical_count + major_count == 0
	pass := shouldPassDA(criticalCount, majorCount)

	// On the final allowed round, hard_stop signals the caller to exit regardless of pass
	hardStop := round >= maxDARounds

	result := DAReviewResult{
		Pass:             pass,
		CriticalCount:    criticalCount,
		MajorCount:       majorCount,
		Findings:         actionable,
		Round:            round,
		MaxRounds:        maxDARounds,
		HardStop:         hardStop,
		ParseWarning:     parseWarning,
		OverallConfidence: overallConfidence,
		TopConcerns:      topConcerns,
		WhatHoldsUp:      whatHoldsUp,
		RawOutput:        rawOutput,
	}

	resultJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal result: %v", err)), nil
	}

	return mcp.NewToolResultText(string(resultJSON)), nil
}
