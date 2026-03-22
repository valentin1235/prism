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

// DAFinding represents a single finding extracted from DA markdown output.
type DAFinding struct {
	Section           string `json:"section"`
	Title             string `json:"title"`
	Claim             string `json:"claim"`
	Concern           string `json:"concern"`
	Confidence        string `json:"confidence"`
	Severity          string `json:"severity"`
	FalsificationTest string `json:"falsification_test"`
}

// MaxDARounds is the hard limit for the DA review loop.
// After this many rounds, the loop must stop regardless of findings.
const MaxDARounds = 3

// DAReviewResult is the structured result returned by prism_da_review.
type DAReviewResult struct {
	Pass              bool        `json:"pass"`
	CriticalCount     int         `json:"critical_count"`
	MajorCount        int         `json:"major_count"`
	Findings          []DAFinding `json:"findings"`
	Round             int         `json:"round"`
	MaxRounds         int         `json:"max_rounds"`
	HardStop          bool        `json:"hard_stop"`
	ParseWarning      string      `json:"parse_warning,omitempty"`
	OverallConfidence string      `json:"overall_confidence"`
	TopConcerns       string      `json:"top_concerns"`
	WhatHoldsUp       string      `json:"what_holds_up"`
	RawOutput         string      `json:"raw_output"`
}

// Package-level compiled regexes for DA markdown parsing.
// Hoisted from ParseDAFindings/ParseDASummary to avoid recompilation on every call.
var (
	// SeverityKeywordRe detects severity keywords in raw output for parse failure detection.
	SeverityKeywordRe = regexp.MustCompile(`(?i)\b(CRITICAL|MAJOR)\b`)

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

// LoadDASystemPrompt reads the devils-advocate.md from the agents directory.
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

// ParseDAFindings extracts structured findings from DA markdown output using regex.
// All regex patterns are compiled once at package level for performance.
func ParseDAFindings(raw string) []DAFinding {
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

// CountSeverities counts CRITICAL and MAJOR findings in the given slice.
func CountSeverities(findings []DAFinding) (criticalCount, majorCount int) {
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

// ShouldPassDA returns true when there are no CRITICAL or MAJOR findings,
// signaling that the DA review loop can terminate early.
func ShouldPassDA(criticalCount, majorCount int) bool {
	return criticalCount == 0 && majorCount == 0
}

// FilterActionableFindings returns only CRITICAL and MAJOR severity findings,
// discarding MINOR, INFO, and any unrecognized severity levels.
func FilterActionableFindings(findings []DAFinding) []DAFinding {
	var actionable []DAFinding
	for _, f := range findings {
		if f.Severity == "CRITICAL" || f.Severity == "MAJOR" {
			actionable = append(actionable, f)
		}
	}
	return actionable
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
