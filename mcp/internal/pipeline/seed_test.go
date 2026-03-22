package pipeline

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func baseSeedAnalysis() SeedAnalysis {
	return SeedAnalysis{
		Topic:    "Investigate API latency spikes",
		DAPassed: false,
		Research: SeedResearch{
			Summary: "Found 3 areas related to API latency",
			Findings: []SeedFinding{
				{ID: 1, Area: "api-gateway", Description: "Routes requests", Source: "src/gateway/main.go:42", ToolUsed: "Grep"},
				{ID: 2, Area: "db-pool", Description: "Connection pooling", Source: "src/db/pool.go:15", ToolUsed: "Read"},
				{ID: 3, Area: "cache-layer", Description: "Redis caching", Source: "src/cache/redis.go:88", ToolUsed: "Grep"},
			},
			KeyAreas:   []string{"api-gateway", "database", "caching"},
			MCPQueries: []string{"prism_docs: API gateway architecture → found routing layer docs"},
		},
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
	if len(merged.Research.Findings) != 5 {
		t.Fatalf("expected 5 findings, got %d", len(merged.Research.Findings))
	}

	// Original findings unchanged
	for i := 0; i < 3; i++ {
		if merged.Research.Findings[i].ID != existing.Research.Findings[i].ID {
			t.Errorf("finding[%d].ID changed from %d to %d", i, existing.Research.Findings[i].ID, merged.Research.Findings[i].ID)
		}
		if merged.Research.Findings[i].Area != existing.Research.Findings[i].Area {
			t.Errorf("finding[%d].Area changed", i)
		}
	}

	// New findings have sequential IDs starting after max existing
	if merged.Research.Findings[3].ID != 4 {
		t.Errorf("new finding[0].ID = %d, want 4", merged.Research.Findings[3].ID)
	}
	if merged.Research.Findings[4].ID != 5 {
		t.Errorf("new finding[1].ID = %d, want 5", merged.Research.Findings[4].ID)
	}

	// New finding content preserved
	if merged.Research.Findings[3].Area != "rate-limiter" {
		t.Errorf("new finding[0].Area = %q, want %q", merged.Research.Findings[3].Area, "rate-limiter")
	}
}

func TestMergeSeedAnalysis_PreservesExistingFindings(t *testing.T) {
	existing := baseSeedAnalysis()
	original := make([]SeedFinding, len(existing.Research.Findings))
	copy(original, existing.Research.Findings)

	patch := SeedPatch{
		NewFindings: []SeedFinding{
			{Area: "new-area", Description: "new desc", Source: "new.go:1", ToolUsed: "Grep"},
		},
	}

	merged := MergeSeedAnalysis(existing, patch)

	// Verify originals are untouched in merged output
	for i, orig := range original {
		got := merged.Research.Findings[i]
		if got.ID != orig.ID || got.Area != orig.Area || got.Description != orig.Description || got.Source != orig.Source {
			t.Errorf("existing finding[%d] was modified during merge", i)
		}
	}

	// Verify source struct wasn't mutated
	if len(existing.Research.Findings) != 3 {
		t.Errorf("source findings mutated: len = %d, want 3", len(existing.Research.Findings))
	}
}

func TestMergeSeedAnalysis_IDAutoIncrement_NonSequential(t *testing.T) {
	// Test with non-sequential IDs (e.g., gaps from manual editing)
	existing := SeedAnalysis{
		Topic: "test",
		Research: SeedResearch{
			Findings: []SeedFinding{
				{ID: 1, Area: "a"},
				{ID: 5, Area: "b"}, // gap in IDs
				{ID: 3, Area: "c"},
			},
		},
	}

	patch := SeedPatch{
		NewFindings: []SeedFinding{
			{Area: "new"},
		},
	}

	merged := MergeSeedAnalysis(existing, patch)

	// Should use max(1,5,3) + 1 = 6
	if merged.Research.Findings[3].ID != 6 {
		t.Errorf("new finding ID = %d, want 6 (max existing is 5)", merged.Research.Findings[3].ID)
	}
}

func TestMergeSeedAnalysis_IDAutoIncrement_EmptyExisting(t *testing.T) {
	existing := SeedAnalysis{
		Topic:    "test",
		Research: SeedResearch{},
	}

	patch := SeedPatch{
		NewFindings: []SeedFinding{
			{Area: "first"},
			{Area: "second"},
		},
	}

	merged := MergeSeedAnalysis(existing, patch)

	if len(merged.Research.Findings) != 2 {
		t.Fatalf("expected 2 findings, got %d", len(merged.Research.Findings))
	}
	if merged.Research.Findings[0].ID != 1 {
		t.Errorf("first finding ID = %d, want 1", merged.Research.Findings[0].ID)
	}
	if merged.Research.Findings[1].ID != 2 {
		t.Errorf("second finding ID = %d, want 2", merged.Research.Findings[1].ID)
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
	if len(merged.Research.KeyAreas) != len(expected) {
		t.Fatalf("key_areas = %v, want %v", merged.Research.KeyAreas, expected)
	}
	for i, v := range expected {
		if merged.Research.KeyAreas[i] != v {
			t.Errorf("key_areas[%d] = %q, want %q", i, merged.Research.KeyAreas[i], v)
		}
	}
}

func TestMergeSeedAnalysis_AppendMCPQueries(t *testing.T) {
	existing := baseSeedAnalysis()
	patch := SeedPatch{
		NewMCPQueries: []string{"sentry: latency errors → found timeout patterns"},
	}

	merged := MergeSeedAnalysis(existing, patch)

	if len(merged.Research.MCPQueries) != 2 {
		t.Fatalf("mcp_queries length = %d, want 2", len(merged.Research.MCPQueries))
	}
}

func TestMergeSeedAnalysis_UpdateSummary(t *testing.T) {
	existing := baseSeedAnalysis()
	patch := SeedPatch{
		Summary: "Found 5 areas including rate limiting and load balancing",
	}

	merged := MergeSeedAnalysis(existing, patch)

	if merged.Research.Summary != patch.Summary {
		t.Errorf("summary = %q, want %q", merged.Research.Summary, patch.Summary)
	}
}

func TestMergeSeedAnalysis_EmptySummaryPreservesExisting(t *testing.T) {
	existing := baseSeedAnalysis()
	patch := SeedPatch{
		NewFindings: []SeedFinding{{Area: "new"}},
		// Summary intentionally empty
	}

	merged := MergeSeedAnalysis(existing, patch)

	if merged.Research.Summary != existing.Research.Summary {
		t.Errorf("empty summary patch should preserve existing; got %q", merged.Research.Summary)
	}
}

func TestMergeSeedAnalysis_SetDAPassed(t *testing.T) {
	existing := baseSeedAnalysis()
	existing.DAPassed = false

	// Explicitly set da_passed to true
	patch := SeedPatch{
		DAPassed:    true,
		SetDAPassed: true,
	}

	merged := MergeSeedAnalysis(existing, patch)
	if !merged.DAPassed {
		t.Error("da_passed should be true after patch")
	}

	// Set to false explicitly
	patch2 := SeedPatch{
		DAPassed:    false,
		SetDAPassed: true,
	}
	merged2 := MergeSeedAnalysis(existing, patch2)
	if merged2.DAPassed {
		t.Error("da_passed should be false after patch")
	}
}

func TestMergeSeedAnalysis_NoSetDAPassedPreservesExisting(t *testing.T) {
	existing := baseSeedAnalysis()
	existing.DAPassed = true

	patch := SeedPatch{
		NewFindings: []SeedFinding{{Area: "x"}},
		// SetDAPassed not set, so da_passed should stay true
	}

	merged := MergeSeedAnalysis(existing, patch)
	if !merged.DAPassed {
		t.Error("da_passed should be preserved when SetDAPassed is false")
	}
}

func TestMergeSeedAnalysis_EmptyPatchNoChange(t *testing.T) {
	existing := baseSeedAnalysis()
	patch := SeedPatch{}

	merged := MergeSeedAnalysis(existing, patch)

	if len(merged.Research.Findings) != len(existing.Research.Findings) {
		t.Error("empty patch should not change findings count")
	}
	if merged.Research.Summary != existing.Research.Summary {
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
	origLen := len(existing.Research.Findings)
	origKeyLen := len(existing.Research.KeyAreas)

	patch := SeedPatch{
		NewFindings: []SeedFinding{{Area: "new"}},
		NewKeyAreas: []string{"new-area"},
	}

	_ = MergeSeedAnalysis(existing, patch)

	// Verify source was not mutated
	if len(existing.Research.Findings) != origLen {
		t.Errorf("source findings mutated: %d -> %d", origLen, len(existing.Research.Findings))
	}
	if len(existing.Research.KeyAreas) != origKeyLen {
		t.Errorf("source key_areas mutated: %d -> %d", origKeyLen, len(existing.Research.KeyAreas))
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
		Summary:          "Expanded to 4 areas including rate limiting",
		NewKeyAreas: []string{"rate-limiting"},
		DAPassed:         true,
		SetDAPassed:      true,
	}

	result, err := PatchSeedAnalysisFile(path, patch)
	if err != nil {
		t.Fatalf("PatchSeedAnalysisFile: %v", err)
	}

	// Verify in-memory result
	if len(result.Research.Findings) != 4 {
		t.Errorf("expected 4 findings, got %d", len(result.Research.Findings))
	}
	if !result.DAPassed {
		t.Error("da_passed should be true")
	}

	// Re-read from disk and verify persistence
	reread, err := ReadSeedAnalysis(path)
	if err != nil {
		t.Fatalf("ReadSeedAnalysis: %v", err)
	}
	if len(reread.Research.Findings) != 4 {
		t.Errorf("re-read: expected 4 findings, got %d", len(reread.Research.Findings))
	}
	if !reread.DAPassed {
		t.Error("re-read: da_passed should be true")
	}
	if reread.Research.Summary != patch.Summary {
		t.Errorf("re-read: summary = %q, want %q", reread.Research.Summary, patch.Summary)
	}
	if reread.Research.Findings[3].ID != 4 {
		t.Errorf("re-read: new finding ID = %d, want 4", reread.Research.Findings[3].ID)
	}
}

func TestPatchSeedAnalysisFile_MultipleRounds(t *testing.T) {
	// Simulate the DA self-loop: initial write, then two rounds of patches
	dir := t.TempDir()
	path := filepath.Join(dir, "seed-analysis.json")

	// Round 0: initial seed analysis
	initial := SeedAnalysis{
		Topic: "Investigate payment failures",
		Research: SeedResearch{
			Summary: "Found payment processing module",
			Findings: []SeedFinding{
				{ID: 1, Area: "payment-processor", Description: "Handles payments", Source: "src/pay.go:1", ToolUsed: "Grep"},
			},
			KeyAreas: []string{"payments"},
		},
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
	if len(r1.Research.Findings) != 2 {
		t.Fatalf("round 1: expected 2 findings, got %d", len(r1.Research.Findings))
	}
	if r1.Research.Findings[1].ID != 2 {
		t.Errorf("round 1: new finding ID = %d, want 2", r1.Research.Findings[1].ID)
	}

	// Round 2: DA found missing retry logic
	patch2 := SeedPatch{
		NewFindings: []SeedFinding{
			{Area: "retry-logic", Description: "Retry with backoff", Source: "src/retry.go:5", ToolUsed: "Grep"},
		},
		Summary:     "Found payment, error handling, and retry logic",
		NewKeyAreas: []string{"retry-mechanisms"},
		DAPassed:    true,
		SetDAPassed: true,
	}
	r2, err := PatchSeedAnalysisFile(path, patch2)
	if err != nil {
		t.Fatalf("round 2: %v", err)
	}
	if len(r2.Research.Findings) != 3 {
		t.Fatalf("round 2: expected 3 findings, got %d", len(r2.Research.Findings))
	}

	// Verify all original findings preserved with original IDs
	if r2.Research.Findings[0].ID != 1 || r2.Research.Findings[0].Area != "payment-processor" {
		t.Error("round 2: original finding[0] was modified")
	}
	if r2.Research.Findings[1].ID != 2 || r2.Research.Findings[1].Area != "error-handling" {
		t.Error("round 2: round 1 finding was modified")
	}
	if r2.Research.Findings[2].ID != 3 || r2.Research.Findings[2].Area != "retry-logic" {
		t.Error("round 2: new finding incorrect")
	}

	// key_areas should have all 3 unique areas
	if len(r2.Research.KeyAreas) != 3 {
		t.Errorf("round 2: key_areas = %v, want 3 items", r2.Research.KeyAreas)
	}

	if !r2.DAPassed {
		t.Error("round 2: da_passed should be true")
	}
}

// === AC 9: da_passed field always written as true or false ===

func TestDAPassed_AlwaysPresentInJSON_True(t *testing.T) {
	sa := SeedAnalysis{
		Topic:    "test topic",
		DAPassed: true,
		Research: SeedResearch{Summary: "test"},
	}

	data, err := json.MarshalIndent(sa, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	val, exists := raw["da_passed"]
	if !exists {
		t.Fatal("da_passed field missing from JSON output")
	}
	boolVal, ok := val.(bool)
	if !ok {
		t.Fatalf("da_passed is not a boolean, got %T", val)
	}
	if boolVal != true {
		t.Error("da_passed should be true")
	}
}

func TestDAPassed_AlwaysPresentInJSON_False(t *testing.T) {
	sa := SeedAnalysis{
		Topic:    "test topic",
		DAPassed: false,
		Research: SeedResearch{Summary: "test"},
	}

	data, err := json.MarshalIndent(sa, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	val, exists := raw["da_passed"]
	if !exists {
		t.Fatal("da_passed field missing from JSON output when false — ensure no omitempty tag")
	}
	boolVal, ok := val.(bool)
	if !ok {
		t.Fatalf("da_passed is not a boolean, got %T", val)
	}
	if boolVal != false {
		t.Error("da_passed should be false")
	}
}

func TestDAPassed_WrittenToFile_True(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "seed-analysis.json")

	sa := SeedAnalysis{
		Topic:    "test topic",
		DAPassed: true,
		Research: SeedResearch{
			Summary:  "test summary",
			Findings: []SeedFinding{{ID: 1, Area: "test", Description: "desc", Source: "f.go:1", ToolUsed: "Grep"}},
		},
	}

	if err := WriteSeedAnalysis(path, sa); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Read raw bytes and verify da_passed is present as boolean true
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	val, exists := parsed["da_passed"]
	if !exists {
		t.Fatal("da_passed field missing from written file")
	}
	if val != true {
		t.Errorf("da_passed = %v, want true", val)
	}

	// Also verify round-trip through ReadSeedAnalysis
	reread, err := ReadSeedAnalysis(path)
	if err != nil {
		t.Fatalf("ReadSeedAnalysis: %v", err)
	}
	if !reread.DAPassed {
		t.Error("da_passed should be true after round-trip")
	}
}

func TestDAPassed_WrittenToFile_False(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "seed-analysis.json")

	sa := SeedAnalysis{
		Topic:    "test topic",
		DAPassed: false,
		Research: SeedResearch{
			Summary:  "test summary",
			Findings: []SeedFinding{{ID: 1, Area: "test", Description: "desc", Source: "f.go:1", ToolUsed: "Grep"}},
		},
	}

	if err := WriteSeedAnalysis(path, sa); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Read raw bytes and verify da_passed is present as boolean false
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	val, exists := parsed["da_passed"]
	if !exists {
		t.Fatal("da_passed field missing from written file when false — field must never be omitted")
	}
	if val != false {
		t.Errorf("da_passed = %v, want false", val)
	}

	// Also verify round-trip through ReadSeedAnalysis
	reread, err := ReadSeedAnalysis(path)
	if err != nil {
		t.Fatalf("ReadSeedAnalysis: %v", err)
	}
	if reread.DAPassed {
		t.Error("da_passed should be false after round-trip")
	}
}

func TestDAPassed_PatchSetsTrue(t *testing.T) {
	// Simulate DA pass: patch sets da_passed=true on seed-analysis.json
	dir := t.TempDir()
	path := filepath.Join(dir, "seed-analysis.json")

	initial := SeedAnalysis{
		Topic:    "test",
		DAPassed: false,
		Research: SeedResearch{
			Summary:  "initial",
			Findings: []SeedFinding{{ID: 1, Area: "a", Description: "d", Source: "s", ToolUsed: "Grep"}},
		},
	}
	data, _ := json.MarshalIndent(initial, "", "  ")
	os.WriteFile(path, data, 0644)

	result, err := PatchSeedAnalysisFile(path, SeedPatch{
		DAPassed:    true,
		SetDAPassed: true,
	})
	if err != nil {
		t.Fatalf("patch: %v", err)
	}
	if !result.DAPassed {
		t.Error("da_passed should be true after pass patch")
	}

	// Verify on disk
	raw, _ := os.ReadFile(path)
	var parsed map[string]interface{}
	json.Unmarshal(raw, &parsed)
	if parsed["da_passed"] != true {
		t.Errorf("da_passed on disk = %v, want true", parsed["da_passed"])
	}
}

func TestDAPassed_PatchSetsFalse(t *testing.T) {
	// Simulate DA hard-stop at round 3: patch sets da_passed=false
	dir := t.TempDir()
	path := filepath.Join(dir, "seed-analysis.json")

	initial := SeedAnalysis{
		Topic:    "test",
		DAPassed: false,
		Research: SeedResearch{
			Summary:  "initial",
			Findings: []SeedFinding{{ID: 1, Area: "a", Description: "d", Source: "s", ToolUsed: "Grep"}},
		},
	}
	data, _ := json.MarshalIndent(initial, "", "  ")
	os.WriteFile(path, data, 0644)

	result, err := PatchSeedAnalysisFile(path, SeedPatch{
		DAPassed:    false,
		SetDAPassed: true,
	})
	if err != nil {
		t.Fatalf("patch: %v", err)
	}
	if result.DAPassed {
		t.Error("da_passed should be false after hard-stop patch")
	}

	// Verify on disk
	raw, _ := os.ReadFile(path)
	var parsed map[string]interface{}
	json.Unmarshal(raw, &parsed)
	if parsed["da_passed"] != false {
		t.Errorf("da_passed on disk = %v, want false", parsed["da_passed"])
	}
}

func TestDAPassed_StructTagNoOmitempty(t *testing.T) {
	// Zero-value SeedAnalysis (DAPassed defaults to false) must still include da_passed in JSON
	sa := SeedAnalysis{}
	data, err := json.Marshal(sa)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var raw map[string]interface{}
	json.Unmarshal(data, &raw)

	if _, exists := raw["da_passed"]; !exists {
		t.Fatal("da_passed must always be present in JSON — zero-value bool must not be omitted")
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
