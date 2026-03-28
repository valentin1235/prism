package pipeline

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/heechul/prism-mcp/internal/parallel"
)

// AnalysisConfig holds the configuration read from config.json in the state directory.
type AnalysisConfig struct {
	Topic                string `json:"topic"`
	Model                string `json:"model"`
	TaskID               string `json:"task_id"`
	ContextID            string `json:"context_id"`
	StateDir             string `json:"state_dir"`
	ReportDir            string `json:"report_dir"`
	InputContext         string `json:"input_context,omitempty"`
	OntologyScope        string `json:"ontology_scope,omitempty"`
	SeedHints            string `json:"seed_hints,omitempty"`
	ReportTemplate       string `json:"report_template,omitempty"`
	Language             string `json:"language,omitempty"`
	PerspectiveInjection string `json:"perspective_injection,omitempty"`
}

// ReadAnalysisConfig reads config.json from the task's state directory.
func ReadAnalysisConfig(stateDir string) (AnalysisConfig, error) {
	var cfg AnalysisConfig
	data, err := os.ReadFile(filepath.Join(stateDir, "config.json"))
	if err != nil {
		return cfg, fmt.Errorf("read config.json: %w", err)
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config.json: %w", err)
	}
	return cfg, nil
}

// StageResult holds the outcome of a single parallel sub-task (specialist or interview).
type StageResult struct {
	PerspectiveID string // which perspective this result belongs to
	OutputPath    string // path to the output file (findings.json or verified-findings.json)
	Err           error  // nil on success
}

// DAGap represents a single gap identified by the DA in seed analysis validation.
// Type is either "bias" (perspective skew) or "coverage" (overlooked codebase area).
type DAGap struct {
	Type        string `json:"type"`        // "bias" or "coverage"
	Description string `json:"description"` // human-readable description of the gap
}

// MaxDARounds is the hard limit for the DA review loop.
// After this many rounds, the loop must stop regardless of findings.
const MaxDARounds = 1

// DAReviewResult is the structured result returned by prism_da_review.
type DAReviewResult struct {
	Pass              bool     `json:"pass"`
	GapCount          int      `json:"gap_count"`
	BiasCount         int      `json:"bias_count"`
	CoverageCount     int      `json:"coverage_count"`
	Gaps              []DAGap  `json:"gaps"`
	Round             int      `json:"round"`
	MaxRounds         int      `json:"max_rounds"`
	HardStop          bool     `json:"hard_stop"`
	ParseWarning      string   `json:"parse_warning,omitempty"`
	OverallConfidence string   `json:"overall_confidence"`
	TopConcerns       string   `json:"top_concerns"`
	WhatHoldsUp       string   `json:"what_holds_up"`
	RawOutput         string   `json:"raw_output"`
}

// DAReviewRound captures the result of a single DA review round for history tracking.
type DAReviewRound struct {
	Round             int      `json:"round"`
	Pass              bool     `json:"pass"`
	GapCount          int      `json:"gap_count"`
	BiasCount         int      `json:"bias_count"`
	CoverageCount     int      `json:"coverage_count"`
	Gaps              []DAGap  `json:"gaps"`
	OverallConfidence string   `json:"overall_confidence,omitempty"`
	TopConcerns       string   `json:"top_concerns,omitempty"`
	WhatHoldsUp       string   `json:"what_holds_up,omitempty"`
	ParseWarning      string   `json:"parse_warning,omitempty"`
}

// DAReviewHistory stores the complete DA review history for a session.
type DAReviewHistory struct {
	FinalPassed bool            `json:"final_passed"`
	TotalRounds int             `json:"total_rounds"`
	Rounds      []DAReviewRound `json:"rounds"`
}

// Package-level compiled regexes for DA markdown parsing.
// Hoisted from ParseDAGaps/ParseDASummary to avoid recompilation on every call.
var (
	// GapKeywordRe detects gap type keywords in raw output for parse failure detection.
	GapKeywordRe = regexp.MustCompile(`(?i)\b(bias|coverage)\b`)

	// Gap entry: ### [bias] or ### [coverage] followed by description text (case-insensitive)
	gapEntryRe = regexp.MustCompile(`(?mi)^###\s+\[(bias|coverage)\]\s*(.*)$`)

	// Level-2 markdown header boundary (## ) for truncating last gap body
	sectionHeaderRe = regexp.MustCompile(`(?m)^## `)

	// Summary fields
	summaryConfRe  = regexp.MustCompile("(?m)^-\\s+\\*\\*Overall confidence\\*\\*:\\s*`?(HIGH|MEDIUM|LOW)`?\\s*[—–-]?\\s*(.*)")
	summaryTopRe   = regexp.MustCompile(`(?m)^-\s+\*\*Top concerns\*\*:\s*(.+)`)
	summaryHoldsRe = regexp.MustCompile(`(?m)^-\s+\*\*What holds up\*\*:\s*(.+)`)
)

// LoadDASystemPrompt reads the devils-advocate.md from the agents directory.
// Deprecated: Both call sites now use SeedAnalysisDAPrompt (da_prompt.go) instead.
// Retained because GetRepoRoot() uses devils-advocate.md as a repo root marker file,
// and external callers may still reference this function.
func LoadDASystemPrompt() (string, error) {
	// Resolve path relative to the binary's expected location
	// The agents dir is at the repo root, sibling to mcp/
	candidates := []string{
		filepath.Join(GetRepoRoot(), "agents", "devils-advocate.md"),
	}

	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if err == nil {
			return string(data), nil
		}
	}

	return "", fmt.Errorf("devils-advocate.md not found")
}

// GetRepoRoot determines the repository root from the executable path or known markers.
func GetRepoRoot() string {
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

// ParseDAGaps extracts structured gaps from DA markdown output using regex.
// Expected format: ### [bias] Title\nDescription text or ### [coverage] Title\nDescription text
// All regex patterns are compiled once at package level for performance.
func ParseDAGaps(raw string) []DAGap {
	var gaps []DAGap

	matches := gapEntryRe.FindAllStringSubmatchIndex(raw, -1)
	if len(matches) == 0 {
		return gaps
	}

	for i, m := range matches {
		gapType := strings.ToLower(raw[m[2]:m[3]])
		title := strings.TrimSpace(raw[m[4]:m[5]])

		// Determine the description text boundary (until next gap entry or section header)
		bodyStart := m[1]
		var bodyEnd int
		if i+1 < len(matches) {
			bodyEnd = matches[i+1][0]
		} else {
			bodyEnd = len(raw)
		}
		// Truncate at next ## header to prevent bleeding into Self-Audit Log / Summary
		if loc := sectionHeaderRe.FindStringIndex(raw[bodyStart:bodyEnd]); loc != nil {
			bodyEnd = bodyStart + loc[0]
		}
		body := strings.TrimSpace(raw[bodyStart:bodyEnd])

		// Combine title and body into description
		description := title
		if body != "" {
			description = title + ": " + body
		}

		gaps = append(gaps, DAGap{
			Type:        gapType,
			Description: description,
		})
	}

	return gaps
}

// ParseDASummary extracts summary fields from DA markdown output.
// All regex patterns are compiled once at package level for performance.
func ParseDASummary(raw string) (overallConfidence, topConcerns, whatHoldsUp string) {
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

// CountGapsByType counts bias and coverage gaps in the given slice.
func CountGapsByType(gaps []DAGap) (biasCount, coverageCount int) {
	for _, g := range gaps {
		switch g.Type {
		case "bias":
			biasCount++
		case "coverage":
			coverageCount++
		}
	}
	return
}

// ShouldPassDAGaps returns true when there are no gaps,
// signaling that the DA review loop can terminate early.
func ShouldPassDAGaps(gaps []DAGap) bool {
	return len(gaps) == 0
}

// JobResultsToStageResults converts parallel.JobResult slice to StageResult slice.
func JobResultsToStageResults(jobs []parallel.JobResult) []StageResult {
	results := make([]StageResult, len(jobs))
	for i, j := range jobs {
		results[i] = StageResult{
			PerspectiveID: j.PerspectiveID,
			OutputPath:    j.OutputPath,
			Err:           j.Err,
		}
	}
	return results
}

// LoadInjectedPerspectives reads a JSON file containing an array of Perspective objects
// to be merged into the generated perspective set after stage1.
func LoadInjectedPerspectives(filePath string) ([]Perspective, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read perspective injection file: %w", err)
	}

	var perspectives []Perspective
	if err := json.Unmarshal(data, &perspectives); err != nil {
		return nil, fmt.Errorf("parse perspective injection file: %w", err)
	}

	return perspectives, nil
}

// MergeInjectedPerspectives appends injected perspectives to the generated set,
// skipping any that have duplicate IDs with already-generated perspectives.
// Injected perspectives are appended at the end to preserve the generated ordering.
func MergeInjectedPerspectives(generated, injected []Perspective) []Perspective {
	// Build a set of existing IDs for dedup
	existingIDs := make(map[string]bool, len(generated))
	for _, p := range generated {
		existingIDs[p.ID] = true
	}

	merged := make([]Perspective, len(generated))
	copy(merged, generated)

	for _, p := range injected {
		if existingIDs[p.ID] {
			log.Printf("Perspective injection: skipping duplicate id %q (already in generated set)", p.ID)
			continue
		}
		merged = append(merged, p)
		existingIDs[p.ID] = true
	}

	return merged
}
