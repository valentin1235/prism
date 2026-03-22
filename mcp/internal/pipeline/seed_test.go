package pipeline

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func baseSeedAnalysis() SeedAnalysis {
	return SeedAnalysis{
		Topic:   "Investigate API latency spikes",
		Summary: "Found 3 areas related to API latency",
		Findings: []SeedFinding{
			{ID: 1, Area: "api-gateway", Description: "Routes requests", Source: "src/gateway/main.go:42", ToolUsed: "Grep"},
			{ID: 2, Area: "db-pool", Description: "Connection pooling", Source: "src/db/pool.go:15", ToolUsed: "Read"},
			{ID: 3, Area: "cache-layer", Description: "Redis caching", Source: "src/cache/redis.go:88", ToolUsed: "Grep"},
		},
		KeyAreas: []string{"api-gateway", "database", "caching"},
	}
}

func TestMergeSeedAnalysis_AppendFindings(t *testing.T) {
	existing := baseSeedAnalysis()
	patch := SeedPatch{
		NewFindings: []SeedFinding{
			{Area: "rate-limiter", Description: "Rate limiting middleware", Source: "src/middleware/ratelimit.go:10", ToolUsed: "Grep"},
			{Area: "load-balancer", Description: "LB health checks", Source: "infra/lb/config.yaml:5", ToolUsed: "Read"},
		},
	}

	merged := MergeSeedAnalysis(existing, patch)

	// Existing findings preserved
	if len(merged.Findings) != 5 {
		t.Fatalf("expected 5 findings, got %d", len(merged.Findings))
	}

	// Original findings unchanged
	for i := 0; i < 3; i++ {
		if merged.Findings[i].ID != existing.Findings[i].ID {
			t.Errorf("finding[%d].ID changed from %d to %d", i, existing.Findings[i].ID, merged.Findings[i].ID)
		}
		if merged.Findings[i].Area != existing.Findings[i].Area {
			t.Errorf("finding[%d].Area changed", i)
		}
	}

	// New findings have sequential IDs starting after max existing
	if merged.Findings[3].ID != 4 {
		t.Errorf("new finding[0].ID = %d, want 4", merged.Findings[3].ID)
	}
	if merged.Findings[4].ID != 5 {
		t.Errorf("new finding[1].ID = %d, want 5", merged.Findings[4].ID)
	}

	// New finding content preserved
	if merged.Findings[3].Area != "rate-limiter" {
		t.Errorf("new finding[0].Area = %q, want %q", merged.Findings[3].Area, "rate-limiter")
	}
}

func TestMergeSeedAnalysis_PreservesExistingFindings(t *testing.T) {
	existing := baseSeedAnalysis()
	original := make([]SeedFinding, len(existing.Findings))
	copy(original, existing.Findings)

	patch := SeedPatch{
		NewFindings: []SeedFinding{
			{Area: "new-area", Description: "new desc", Source: "new.go:1", ToolUsed: "Grep"},
		},
	}

	merged := MergeSeedAnalysis(existing, patch)

	// Verify originals are untouched in merged output
	for i, orig := range original {
		got := merged.Findings[i]
		if got.ID != orig.ID || got.Area != orig.Area || got.Description != orig.Description || got.Source != orig.Source {
			t.Errorf("existing finding[%d] was modified during merge", i)
		}
	}

	// Verify source struct wasn't mutated
	if len(existing.Findings) != 3 {
		t.Errorf("source findings mutated: len = %d, want 3", len(existing.Findings))
	}
}

func TestMergeSeedAnalysis_IDAutoIncrement_NonSequential(t *testing.T) {
	// Test with non-sequential IDs (e.g., gaps from manual editing)
	existing := SeedAnalysis{
		Topic: "test",
		Findings: []SeedFinding{
			{ID: 1, Area: "a"},
			{ID: 5, Area: "b"}, // gap in IDs
			{ID: 3, Area: "c"},
		},
	}

	patch := SeedPatch{
		NewFindings: []SeedFinding{
			{Area: "new"},
		},
	}

	merged := MergeSeedAnalysis(existing, patch)

	// Should use max(1,5,3) + 1 = 6
	if merged.Findings[3].ID != 6 {
		t.Errorf("new finding ID = %d, want 6 (max existing is 5)", merged.Findings[3].ID)
	}
}

func TestMergeSeedAnalysis_IDAutoIncrement_EmptyExisting(t *testing.T) {
	existing := SeedAnalysis{
		Topic: "test",
	}

	patch := SeedPatch{
		NewFindings: []SeedFinding{
			{Area: "first"},
			{Area: "second"},
		},
	}

	merged := MergeSeedAnalysis(existing, patch)

	if len(merged.Findings) != 2 {
		t.Fatalf("expected 2 findings, got %d", len(merged.Findings))
	}
	if merged.Findings[0].ID != 1 {
		t.Errorf("first finding ID = %d, want 1", merged.Findings[0].ID)
	}
	if merged.Findings[1].ID != 2 {
		t.Errorf("second finding ID = %d, want 2", merged.Findings[1].ID)
	}
}

func TestMergeSeedAnalysis_DeduplicateKeyAreas(t *testing.T) {
	existing := baseSeedAnalysis()
	patch := SeedPatch{
		NewKeyAreas: []string{"database", "monitoring", "caching", "networking"},
	}

	merged := MergeSeedAnalysis(existing, patch)

	// "database" and "caching" already exist, should not be duplicated
	expected := []string{"api-gateway", "database", "caching", "monitoring", "networking"}
	if len(merged.KeyAreas) != len(expected) {
		t.Fatalf("key_areas = %v, want %v", merged.KeyAreas, expected)
	}
	for i, v := range expected {
		if merged.KeyAreas[i] != v {
			t.Errorf("key_areas[%d] = %q, want %q", i, merged.KeyAreas[i], v)
		}
	}
}

func TestMergeSeedAnalysis_UpdateSummary(t *testing.T) {
	existing := baseSeedAnalysis()
	patch := SeedPatch{
		Summary: "Found 5 areas including rate limiting and load balancing",
	}

	merged := MergeSeedAnalysis(existing, patch)

	if merged.Summary != patch.Summary {
		t.Errorf("summary = %q, want %q", merged.Summary, patch.Summary)
	}
}

func TestMergeSeedAnalysis_EmptySummaryPreservesExisting(t *testing.T) {
	existing := baseSeedAnalysis()
	patch := SeedPatch{
		NewFindings: []SeedFinding{{Area: "new"}},
		// Summary intentionally empty
	}

	merged := MergeSeedAnalysis(existing, patch)

	if merged.Summary != existing.Summary {
		t.Errorf("empty summary patch should preserve existing; got %q", merged.Summary)
	}
}

func TestMergeSeedAnalysis_EmptyPatchNoChange(t *testing.T) {
	existing := baseSeedAnalysis()
	patch := SeedPatch{}

	merged := MergeSeedAnalysis(existing, patch)

	if len(merged.Findings) != len(existing.Findings) {
		t.Error("empty patch should not change findings count")
	}
	if merged.Summary != existing.Summary {
		t.Error("empty patch should not change summary")
	}
	if merged.Topic != existing.Topic {
		t.Error("empty patch should not change topic")
	}
}

func TestMergeSeedAnalysis_TopicPreserved(t *testing.T) {
	existing := baseSeedAnalysis()
	patch := SeedPatch{
		NewFindings: []SeedFinding{{Area: "new"}},
		Summary:     "Updated summary",
	}

	merged := MergeSeedAnalysis(existing, patch)

	if merged.Topic != existing.Topic {
		t.Errorf("topic should be preserved; got %q, want %q", merged.Topic, existing.Topic)
	}
}

func TestMergeSeedAnalysis_NoMutationOfSource(t *testing.T) {
	existing := baseSeedAnalysis()
	origLen := len(existing.Findings)
	origKeyLen := len(existing.KeyAreas)

	patch := SeedPatch{
		NewFindings: []SeedFinding{{Area: "new"}},
		NewKeyAreas: []string{"new-area"},
	}

	_ = MergeSeedAnalysis(existing, patch)

	// Verify source was not mutated
	if len(existing.Findings) != origLen {
		t.Errorf("source findings mutated: %d -> %d", origLen, len(existing.Findings))
	}
	if len(existing.KeyAreas) != origKeyLen {
		t.Errorf("source key_areas mutated: %d -> %d", origKeyLen, len(existing.KeyAreas))
	}
}

func TestPatchSeedAnalysisFile_RoundTrip(t *testing.T) {
	// Create a temp file with initial seed analysis
	dir := t.TempDir()
	path := filepath.Join(dir, "seed-analysis.json")

	initial := baseSeedAnalysis()
	data, err := json.MarshalIndent(initial, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Apply a patch
	patch := SeedPatch{
		NewFindings: []SeedFinding{
			{Area: "rate-limiter", Description: "Rate limiting", Source: "src/ratelimit.go:1", ToolUsed: "Grep"},
		},
		Summary:     "Expanded to 4 areas including rate limiting",
		NewKeyAreas: []string{"rate-limiting"},
	}

	result, err := PatchSeedAnalysisFile(path, patch)
	if err != nil {
		t.Fatalf("PatchSeedAnalysisFile: %v", err)
	}

	// Verify in-memory result
	if len(result.Findings) != 4 {
		t.Errorf("expected 4 findings, got %d", len(result.Findings))
	}

	// Re-read from disk and verify persistence
	reread, err := ReadSeedAnalysis(path)
	if err != nil {
		t.Fatalf("ReadSeedAnalysis: %v", err)
	}
	if len(reread.Findings) != 4 {
		t.Errorf("re-read: expected 4 findings, got %d", len(reread.Findings))
	}
	if reread.Summary != patch.Summary {
		t.Errorf("re-read: summary = %q, want %q", reread.Summary, patch.Summary)
	}
	if reread.Findings[3].ID != 4 {
		t.Errorf("re-read: new finding ID = %d, want 4", reread.Findings[3].ID)
	}
}

func TestPatchSeedAnalysisFile_MultipleRounds(t *testing.T) {
	// Simulate the DA self-loop: initial write, then two rounds of patches
	dir := t.TempDir()
	path := filepath.Join(dir, "seed-analysis.json")

	// Round 0: initial seed analysis
	initial := SeedAnalysis{
		Topic:   "Investigate payment failures",
		Summary: "Found payment processing module",
		Findings: []SeedFinding{
			{ID: 1, Area: "payment-processor", Description: "Handles payments", Source: "src/pay.go:1", ToolUsed: "Grep"},
		},
		KeyAreas: []string{"payments"},
	}
	data, _ := json.MarshalIndent(initial, "", "  ")
	os.WriteFile(path, data, 0644)

	// Round 1: DA found missing error handling area
	patch1 := SeedPatch{
		NewFindings: []SeedFinding{
			{Area: "error-handling", Description: "Error recovery logic", Source: "src/errors.go:10", ToolUsed: "Read"},
		},
		Summary:          "Found payment processing and error handling",
		NewKeyAreas: []string{"error-handling"},
	}
	r1, err := PatchSeedAnalysisFile(path, patch1)
	if err != nil {
		t.Fatalf("round 1: %v", err)
	}
	if len(r1.Findings) != 2 {
		t.Fatalf("round 1: expected 2 findings, got %d", len(r1.Findings))
	}
	if r1.Findings[1].ID != 2 {
		t.Errorf("round 1: new finding ID = %d, want 2", r1.Findings[1].ID)
	}

	// Round 2: DA found missing retry logic
	patch2 := SeedPatch{
		NewFindings: []SeedFinding{
			{Area: "retry-logic", Description: "Retry with backoff", Source: "src/retry.go:5", ToolUsed: "Grep"},
		},
		Summary:     "Found payment, error handling, and retry logic",
		NewKeyAreas: []string{"retry-mechanisms"},
	}
	r2, err := PatchSeedAnalysisFile(path, patch2)
	if err != nil {
		t.Fatalf("round 2: %v", err)
	}
	if len(r2.Findings) != 3 {
		t.Fatalf("round 2: expected 3 findings, got %d", len(r2.Findings))
	}

	// Verify all original findings preserved with original IDs
	if r2.Findings[0].ID != 1 || r2.Findings[0].Area != "payment-processor" {
		t.Error("round 2: original finding[0] was modified")
	}
	if r2.Findings[1].ID != 2 || r2.Findings[1].Area != "error-handling" {
		t.Error("round 2: round 1 finding was modified")
	}
	if r2.Findings[2].ID != 3 || r2.Findings[2].Area != "retry-logic" {
		t.Error("round 2: new finding incorrect")
	}

	// key_areas should have all 3 unique areas
	if len(r2.KeyAreas) != 3 {
		t.Errorf("round 2: key_areas = %v, want 3 items", r2.KeyAreas)
	}
}

func TestReadSeedAnalysis_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	os.WriteFile(path, []byte("not json"), 0644)

	_, err := ReadSeedAnalysis(path)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestReadSeedAnalysis_FileNotFound(t *testing.T) {
	_, err := ReadSeedAnalysis("/nonexistent/path/seed-analysis.json")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestDeduplicateStrings(t *testing.T) {
	tests := []struct {
		name      string
		base      []string
		additions []string
		want      []string
	}{
		{"no overlap", []string{"a", "b"}, []string{"c", "d"}, []string{"a", "b", "c", "d"}},
		{"full overlap", []string{"a", "b"}, []string{"a", "b"}, []string{"a", "b"}},
		{"partial overlap", []string{"a", "b"}, []string{"b", "c"}, []string{"a", "b", "c"}},
		{"empty base", nil, []string{"a"}, []string{"a"}},
		{"empty additions", []string{"a"}, nil, []string{"a"}},
		{"both empty", nil, nil, []string{}},
		{"dup in additions", []string{"a"}, []string{"b", "b"}, []string{"a", "b"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deduplicateStrings(tt.base, tt.additions)
			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d; got %v", len(got), len(tt.want), got)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestMaxFindingID(t *testing.T) {
	tests := []struct {
		name     string
		findings []SeedFinding
		want     int
	}{
		{"empty", nil, 0},
		{"single", []SeedFinding{{ID: 5}}, 5},
		{"ascending", []SeedFinding{{ID: 1}, {ID: 2}, {ID: 3}}, 3},
		{"non-sequential", []SeedFinding{{ID: 1}, {ID: 7}, {ID: 3}}, 7},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maxFindingID(tt.findings)
			if got != tt.want {
				t.Errorf("maxFindingID = %d, want %d", got, tt.want)
			}
		})
	}
}
