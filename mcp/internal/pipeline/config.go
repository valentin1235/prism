package pipeline

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	prismconfig "github.com/heechul/prism-mcp/internal/config"
	"github.com/heechul/prism-mcp/internal/parallel"
)

// AnalysisConfig holds the configuration read from config.json in the state directory.
type AnalysisConfig struct {
	Topic                string `json:"topic"`
	Model                string `json:"model"`
	Adaptor              string `json:"adaptor,omitempty"`
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

type ontologyScopeSource struct {
	Type   string `json:"type"`
	Path   string `json:"path,omitempty"`
	Status string `json:"status"`
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

// ResolveAnalysisWorkDir picks a filesystem workspace root for tool-driven Codex
// sessions. Seed analysis, specialists, and interview verification need Grep/Glob
// to run against the actual target repositories, not the Prism state directory.
func ResolveAnalysisWorkDir(cfg AnalysisConfig) string {
	roots := ontologyScopePaths(cfg.OntologyScope)
	if len(roots) == 0 {
		if root := normalizeExistingDir(cfg.InputContext); root != "" {
			return root
		}
		if root := normalizeExistingDir(cfg.StateDir); root != "" {
			return root
		}
		return "."
	}

	if len(roots) == 1 {
		return roots[0]
	}

	if common := longestExistingCommonAncestor(roots); common != "" {
		return common
	}

	if root := normalizeExistingDir(cfg.StateDir); root != "" {
		return root
	}
	return roots[0]
}

func ontologyScopePaths(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	var scope struct {
		Sources []ontologyScopeSource `json:"sources"`
	}
	if err := json.Unmarshal([]byte(raw), &scope); err != nil {
		return nil
	}

	roots := make([]string, 0, len(scope.Sources))
	seen := map[string]struct{}{}
	for _, src := range scope.Sources {
		if src.Status != "" && src.Status != "available" {
			continue
		}
		switch src.Type {
		case "doc", "file":
		default:
			continue
		}

		root := normalizeExistingDir(src.Path)
		if root == "" {
			continue
		}
		if _, ok := seen[root]; ok {
			continue
		}
		seen[root] = struct{}{}
		roots = append(roots, root)
	}

	return roots
}

func normalizeExistingDir(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return ""
	}
	if eval, err := filepath.EvalSymlinks(abs); err == nil {
		abs = eval
	}

	info, err := os.Stat(abs)
	if err != nil {
		return ""
	}
	if info.IsDir() {
		return abs
	}
	return filepath.Dir(abs)
}

func longestExistingCommonAncestor(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	if len(paths) == 1 {
		return normalizeExistingDir(paths[0])
	}

	parts := make([][]string, 0, len(paths))
	for _, p := range paths {
		root := normalizeExistingDir(p)
		if root == "" {
			return ""
		}
		parts = append(parts, splitPathComponents(root))
	}

	limit := len(parts[0])
	for _, partSet := range parts[1:] {
		if len(partSet) < limit {
			limit = len(partSet)
		}
	}

	common := make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		candidate := parts[0][i]
		match := true
		for _, partSet := range parts[1:] {
			if partSet[i] != candidate {
				match = false
				break
			}
		}
		if !match {
			break
		}
		common = append(common, candidate)
	}

	if len(common) == 0 {
		return ""
	}

	var joined string
	if filepath.IsAbs(paths[0]) {
		if runtime.GOOS == "windows" && strings.HasSuffix(common[0], ":") {
			joined = filepath.Join(common...)
		} else {
			joined = string(filepath.Separator) + filepath.Join(common...)
		}
	} else {
		joined = filepath.Join(common...)
	}

	return normalizeExistingDir(joined)
}

func splitPathComponents(path string) []string {
	clean := filepath.Clean(path)
	vol := filepath.VolumeName(clean)
	clean = strings.TrimPrefix(clean, vol)
	clean = strings.TrimPrefix(clean, string(filepath.Separator))
	if clean == "" {
		if vol != "" {
			return []string{vol}
		}
		return []string{}
	}

	parts := strings.Split(clean, string(filepath.Separator))
	if vol != "" {
		return append([]string{vol}, parts...)
	}
	return parts
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
	Pass              bool    `json:"pass"`
	GapCount          int     `json:"gap_count"`
	BiasCount         int     `json:"bias_count"`
	CoverageCount     int     `json:"coverage_count"`
	Gaps              []DAGap `json:"gaps"`
	Round             int     `json:"round"`
	MaxRounds         int     `json:"max_rounds"`
	HardStop          bool    `json:"hard_stop"`
	ParseWarning      string  `json:"parse_warning,omitempty"`
	OverallConfidence string  `json:"overall_confidence"`
	TopConcerns       string  `json:"top_concerns"`
	WhatHoldsUp       string  `json:"what_holds_up"`
	RawOutput         string  `json:"raw_output"`
}

// DAReviewRound captures the result of a single DA review round for history tracking.
type DAReviewRound struct {
	Round             int     `json:"round"`
	Pass              bool    `json:"pass"`
	GapCount          int     `json:"gap_count"`
	BiasCount         int     `json:"bias_count"`
	CoverageCount     int     `json:"coverage_count"`
	Gaps              []DAGap `json:"gaps"`
	OverallConfidence string  `json:"overall_confidence,omitempty"`
	TopConcerns       string  `json:"top_concerns,omitempty"`
	WhatHoldsUp       string  `json:"what_holds_up,omitempty"`
	ParseWarning      string  `json:"parse_warning,omitempty"`
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
	markers := []string{
		filepath.Join("skills", "setup", "SKILL.md"),
		filepath.Join("agents", "devils-advocate.md"),
	}
	var tried []string

	// First priority: Codex install path override.
	if root := os.Getenv("PRISM_REPO_PATH"); root != "" {
		if prismRepoRootMatches(root, markers...) {
			return root
		}
		tried = append(tried, "PRISM_REPO_PATH="+root)
	}

	// Second priority: legacy PRISM_ROOT environment variable.
	if root := os.Getenv("PRISM_ROOT"); root != "" {
		if prismRepoRootMatches(root, markers...) {
			return root
		}
		tried = append(tried, "PRISM_ROOT="+root)
	}

	// Third priority: the repo-root pointer written into ~/.codex during setup.
	if pointerPath := prismconfig.CodexRepoRootPointerPath(); pointerPath != "" {
		if pointedRoot, err := os.ReadFile(pointerPath); err == nil {
			root := strings.TrimSpace(string(pointedRoot))
			if prismRepoRootMatches(root, markers...) {
				return root
			}
			if root != "" {
				tried = append(tried, pointerPath+"="+root)
			}
		} else {
			tried = append(tried, pointerPath)
		}
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
			if prismRepoRootMatches(abs, markers...) {
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
		if prismRepoRootMatches(abs, markers...) {
			return abs
		}
		tried = append(tried, abs)
	}

	log.Printf(
		"WARNING: could not locate Prism repo markers %v. Tried paths: %v. Set PRISM_REPO_PATH or PRISM_ROOT to override.",
		markers,
		tried,
	)
	return cwd
}

func prismRepoRootMatches(root string, markers ...string) bool {
	root = strings.TrimSpace(root)
	if root == "" {
		return false
	}

	for _, marker := range markers {
		if _, err := os.Stat(filepath.Join(root, marker)); err != nil {
			return false
		}
	}

	return true
}

// ResolveRepoAssetPath resolves a repository-relative Prism asset path using the
// same root detection logic as the rest of the pipeline.
func ResolveRepoAssetPath(relativePath string) (string, error) {
	root := GetRepoRoot()
	if root == "" {
		return "", fmt.Errorf("resolve repo root for asset %q", relativePath)
	}

	assetPath := filepath.Join(root, filepath.FromSlash(relativePath))
	if _, err := os.Stat(assetPath); err != nil {
		return "", fmt.Errorf("resolve repo asset %s: %w", assetPath, err)
	}

	return assetPath, nil
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
