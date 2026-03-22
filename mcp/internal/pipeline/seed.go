package pipeline

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

// SeedAnalysis represents the full seed-analysis.json structure.
type SeedAnalysis struct {
	Topic    string        `json:"topic"`
	Summary  string        `json:"summary"`
	Findings []SeedFinding `json:"findings"`
	KeyAreas []string      `json:"key_areas"`
}

// SeedPatch contains new data to merge into an existing SeedAnalysis.
// Only non-zero/non-empty fields are applied.
type SeedPatch struct {
	// NewFindings are appended to findings with auto-incremented IDs.
	NewFindings []SeedFinding `json:"new_findings,omitempty"`
	// Summary replaces summary if non-empty.
	Summary string `json:"summary,omitempty"`
	// NewKeyAreas are appended (deduplicated) to key_areas.
	NewKeyAreas []string `json:"new_key_areas,omitempty"`
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
	merged.Findings = make([]SeedFinding, len(existing.Findings))
	copy(merged.Findings, existing.Findings)

	// Append new findings with auto-incremented IDs
	if len(patch.NewFindings) > 0 {
		nextID := maxFindingID(merged.Findings) + 1
		for _, f := range patch.NewFindings {
			f.ID = nextID
			nextID++
			merged.Findings = append(merged.Findings, f)
		}
	}

	// Update summary if provided
	if patch.Summary != "" {
		merged.Summary = patch.Summary
	}

	// Deduplicate and append key_areas
	if len(patch.NewKeyAreas) > 0 {
		merged.KeyAreas = deduplicateStrings(existing.KeyAreas, patch.NewKeyAreas)
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
