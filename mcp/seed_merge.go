package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// SeedFinding represents a single research finding in seed-analysis.json.
type SeedFinding struct {
	ID          int    `json:"id"`
	Area        string `json:"area"`
	Description string `json:"description"`
	Source      string `json:"source"`
	ToolUsed    string `json:"tool_used"`
}

// SeedResearch represents the research section of seed-analysis.json.
type SeedResearch struct {
	Summary       string        `json:"summary"`
	Findings      []SeedFinding `json:"findings"`
	KeyAreas      []string      `json:"key_areas"`
	FilesExamined []string      `json:"files_examined"`
	MCPQueries    []string      `json:"mcp_queries"`
}

// SeedAnalysis represents the full seed-analysis.json structure.
type SeedAnalysis struct {
	Topic    string       `json:"topic"`
	DAPassed bool         `json:"da_passed"`
	Research SeedResearch `json:"research"`
}

// SeedPatch contains new data to merge into an existing SeedAnalysis.
// Only non-zero/non-empty fields are applied.
type SeedPatch struct {
	// NewFindings are appended to research.findings with auto-incremented IDs.
	NewFindings []SeedFinding `json:"new_findings,omitempty"`
	// Summary replaces research.summary if non-empty.
	Summary string `json:"summary,omitempty"`
	// NewKeyAreas are appended (deduplicated) to research.key_areas.
	NewKeyAreas []string `json:"new_key_areas,omitempty"`
	// NewFilesExamined are appended to research.files_examined.
	NewFilesExamined []string `json:"new_files_examined,omitempty"`
	// NewMCPQueries are appended to research.mcp_queries.
	NewMCPQueries []string `json:"new_mcp_queries,omitempty"`
	// DAPassed sets the da_passed flag. Use SetDAPassed to control whether it's applied.
	DAPassed    bool `json:"-"`
	SetDAPassed bool `json:"-"`
}

// maxFindingID returns the highest id among existing findings, or 0 if empty.
func maxFindingID(findings []SeedFinding) int {
	max := 0
	for _, f := range findings {
		if f.ID > max {
			max = f.ID
		}
	}
	return max
}

// deduplicateStrings appends items from additions to base, skipping duplicates.
func deduplicateStrings(base, additions []string) []string {
	seen := make(map[string]bool, len(base))
	for _, s := range base {
		seen[s] = true
	}
	result := make([]string, len(base))
	copy(result, base)
	for _, s := range additions {
		if !seen[s] {
			result = append(result, s)
			seen[s] = true
		}
	}
	return result
}

// MergeSeedAnalysis applies a patch to an existing SeedAnalysis, returning a new copy.
// Existing findings are never modified or removed. New findings get auto-incremented IDs.
func MergeSeedAnalysis(existing SeedAnalysis, patch SeedPatch) SeedAnalysis {
	merged := existing

	// Deep copy existing findings to avoid mutation
	merged.Research.Findings = make([]SeedFinding, len(existing.Research.Findings))
	copy(merged.Research.Findings, existing.Research.Findings)

	// Append new findings with auto-incremented IDs
	if len(patch.NewFindings) > 0 {
		nextID := maxFindingID(merged.Research.Findings) + 1
		for _, f := range patch.NewFindings {
			f.ID = nextID
			nextID++
			merged.Research.Findings = append(merged.Research.Findings, f)
		}
	}

	// Update summary if provided
	if patch.Summary != "" {
		merged.Research.Summary = patch.Summary
	}

	// Deduplicate and append key_areas
	if len(patch.NewKeyAreas) > 0 {
		merged.Research.KeyAreas = deduplicateStrings(existing.Research.KeyAreas, patch.NewKeyAreas)
	}

	// Append files_examined (no dedup — same file can be examined for different reasons)
	if len(patch.NewFilesExamined) > 0 {
		merged.Research.FilesExamined = append(
			append([]string{}, existing.Research.FilesExamined...),
			patch.NewFilesExamined...,
		)
	}

	// Append mcp_queries
	if len(patch.NewMCPQueries) > 0 {
		merged.Research.MCPQueries = append(
			append([]string{}, existing.Research.MCPQueries...),
			patch.NewMCPQueries...,
		)
	}

	// Set da_passed if explicitly requested
	if patch.SetDAPassed {
		merged.DAPassed = patch.DAPassed
	}

	return merged
}

// ReadSeedAnalysis reads and parses seed-analysis.json from disk.
func ReadSeedAnalysis(path string) (SeedAnalysis, error) {
	var sa SeedAnalysis
	data, err := os.ReadFile(path)
	if err != nil {
		return sa, fmt.Errorf("read seed analysis: %w", err)
	}
	if err := json.Unmarshal(data, &sa); err != nil {
		return sa, fmt.Errorf("parse seed analysis: %w", err)
	}
	return sa, nil
}

// WriteSeedAnalysis writes a SeedAnalysis to disk as formatted JSON.
func WriteSeedAnalysis(path string, sa SeedAnalysis) error {
	data, err := json.MarshalIndent(sa, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal seed analysis: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write seed analysis: %w", err)
	}
	return nil
}

// PatchSeedAnalysisFile reads seed-analysis.json, applies a patch, and writes it back.
// This is the primary entry point for incremental updates.
// Currently used as library code for potential future prism_seed_merge MCP tool.
// The seed analyst agent performs incremental updates via direct JSON Write for now.
func PatchSeedAnalysisFile(path string, patch SeedPatch) (SeedAnalysis, error) {
	existing, err := ReadSeedAnalysis(path)
	if err != nil {
		return SeedAnalysis{}, err
	}
	merged := MergeSeedAnalysis(existing, patch)
	if err := WriteSeedAnalysis(path, merged); err != nil {
		return SeedAnalysis{}, err
	}
	return merged, nil
}
