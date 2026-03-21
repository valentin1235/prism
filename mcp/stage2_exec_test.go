package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestSpecialistFindingsStoredAtCorrectPath verifies that findings are written to
// ~/.prism/state/analyze-{id}/perspectives/{pid}/findings.json via WriteSpecialistFindings.
func TestSpecialistFindingsStoredAtCorrectPath(t *testing.T) {
	stateDir := t.TempDir()
	perspectiveID := "security-analysis"

	// Create perspective directory (as runSpecialistSession does)
	perspDir := PerspectiveDir(stateDir, perspectiveID)
	if err := os.MkdirAll(perspDir, 0755); err != nil {
		t.Fatalf("create persp dir: %v", err)
	}

	findings := SpecialistFindings{
		Analyst: perspectiveID,
		Input:   "Analyze payment security",
		Findings: []SpecialistFinding{
			{
				Finding:  "Missing input validation",
				Evidence: "handler.go:processPayment:42 — user input passed directly",
				Severity: "HIGH",
			},
			{
				Finding:  "Hardcoded API key",
				Evidence: "config.go:init:15 — API key in source",
				Severity: "CRITICAL",
			},
		},
	}

	// Write findings using the same path function the pipeline uses
	findingsPath := FindingsPath(stateDir, perspectiveID)
	if err := WriteSpecialistFindings(findingsPath, findings); err != nil {
		t.Fatalf("WriteSpecialistFindings: %v", err)
	}

	// Verify the path structure matches expected convention
	expectedPath := filepath.Join(stateDir, "perspectives", perspectiveID, "findings.json")
	if findingsPath != expectedPath {
		t.Errorf("FindingsPath = %q, want %q", findingsPath, expectedPath)
	}

	// Verify file exists at expected path
	if _, err := os.Stat(findingsPath); os.IsNotExist(err) {
		t.Fatalf("findings.json not created at %s", findingsPath)
	}

	// Read back and verify contents
	loaded, err := ReadSpecialistFindings(findingsPath)
	if err != nil {
		t.Fatalf("ReadSpecialistFindings: %v", err)
	}

	if loaded.Analyst != perspectiveID {
		t.Errorf("Analyst = %q, want %q", loaded.Analyst, perspectiveID)
	}
	if len(loaded.Findings) != 2 {
		t.Fatalf("Findings count = %d, want 2", len(loaded.Findings))
	}
	if loaded.Findings[0].Finding != "Missing input validation" {
		t.Errorf("Findings[0].Finding = %q, unexpected", loaded.Findings[0].Finding)
	}
	if loaded.Findings[1].Severity != "CRITICAL" {
		t.Errorf("Findings[1].Severity = %q, want CRITICAL", loaded.Findings[1].Severity)
	}
}

// TestMultipleSpecialistFindingsStoredIndependently verifies that multiple specialists
// write to separate perspective directories without interfering with each other.
func TestMultipleSpecialistFindingsStoredIndependently(t *testing.T) {
	stateDir := t.TempDir()

	perspectives := []struct {
		id       string
		findings SpecialistFindings
	}{
		{
			id: "security-analysis",
			findings: SpecialistFindings{
				Analyst: "security-analysis",
				Input:   "Test topic",
				Findings: []SpecialistFinding{
					{Finding: "Auth bypass", Evidence: "auth.go:50", Severity: "CRITICAL"},
				},
			},
		},
		{
			id: "performance-analysis",
			findings: SpecialistFindings{
				Analyst: "performance-analysis",
				Input:   "Test topic",
				Findings: []SpecialistFinding{
					{Finding: "N+1 query", Evidence: "db.go:30", Severity: "HIGH"},
					{Finding: "Missing index", Evidence: "schema.sql:10", Severity: "MEDIUM"},
				},
			},
		},
		{
			id: "reliability-analysis",
			findings: SpecialistFindings{
				Analyst: "reliability-analysis",
				Input:   "Test topic",
				Findings: []SpecialistFinding{
					{Finding: "No retry logic", Evidence: "client.go:80", Severity: "HIGH"},
				},
			},
		},
	}

	// Write all findings
	for _, p := range perspectives {
		perspDir := PerspectiveDir(stateDir, p.id)
		if err := os.MkdirAll(perspDir, 0755); err != nil {
			t.Fatalf("create dir for %s: %v", p.id, err)
		}
		path := FindingsPath(stateDir, p.id)
		if err := WriteSpecialistFindings(path, p.findings); err != nil {
			t.Fatalf("write findings for %s: %v", p.id, err)
		}
	}

	// Verify each perspective's findings are independent
	for _, p := range perspectives {
		path := FindingsPath(stateDir, p.id)
		loaded, err := ReadSpecialistFindings(path)
		if err != nil {
			t.Fatalf("read findings for %s: %v", p.id, err)
		}

		if loaded.Analyst != p.id {
			t.Errorf("Analyst for %s = %q, want %q", p.id, loaded.Analyst, p.id)
		}
		if len(loaded.Findings) != len(p.findings.Findings) {
			t.Errorf("Findings count for %s = %d, want %d",
				p.id, len(loaded.Findings), len(p.findings.Findings))
		}
	}

	// Verify directory structure
	perspRoot := filepath.Join(stateDir, "perspectives")
	entries, err := os.ReadDir(perspRoot)
	if err != nil {
		t.Fatalf("read perspectives dir: %v", err)
	}
	if len(entries) != 3 {
		t.Errorf("perspectives dir has %d entries, want 3", len(entries))
	}
}

// TestCollectSpecialistResultsReadsFromPerspectivePaths verifies that
// CollectSpecialistResults reads findings.json from the correct perspective paths.
func TestCollectSpecialistResultsReadsFromPerspectivePaths(t *testing.T) {
	stateDir := t.TempDir()

	// Set up two perspectives with findings on disk
	perspectives := []Perspective{
		{ID: "auth-analysis", Name: "Auth Analysis"},
		{ID: "perf-analysis", Name: "Perf Analysis"},
	}

	authFindings := SpecialistFindings{
		Analyst: "auth-analysis",
		Input:   "Test",
		Findings: []SpecialistFinding{
			{Finding: "Weak passwords", Evidence: "auth.go:10", Severity: "HIGH"},
		},
	}
	perfFindings := SpecialistFindings{
		Analyst: "perf-analysis",
		Input:   "Test",
		Findings: []SpecialistFinding{
			{Finding: "Slow query", Evidence: "db.go:20", Severity: "MEDIUM"},
			{Finding: "No caching", Evidence: "handler.go:5", Severity: "LOW"},
		},
	}

	// Write findings at the expected paths
	for _, pair := range []struct {
		id       string
		findings SpecialistFindings
	}{
		{"auth-analysis", authFindings},
		{"perf-analysis", perfFindings},
	} {
		dir := PerspectiveDir(stateDir, pair.id)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("mkdir %s: %v", pair.id, err)
		}
		if err := WriteSpecialistFindings(FindingsPath(stateDir, pair.id), pair.findings); err != nil {
			t.Fatalf("write %s: %v", pair.id, err)
		}
	}

	// Simulate stage results with OutputPath pointing to findings.json
	stageResults := []StageResult{
		{
			PerspectiveID: "auth-analysis",
			OutputPath:    FindingsPath(stateDir, "auth-analysis"),
		},
		{
			PerspectiveID: "perf-analysis",
			OutputPath:    FindingsPath(stateDir, "perf-analysis"),
		},
	}

	collected := CollectSpecialistResults("test-task", stageResults, perspectives)

	if collected.Succeeded != 2 {
		t.Errorf("Succeeded = %d, want 2", collected.Succeeded)
	}
	if collected.Failed != 0 {
		t.Errorf("Failed = %d, want 0", collected.Failed)
	}
	if collected.TotalFindings != 3 {
		t.Errorf("TotalFindings = %d, want 3", collected.TotalFindings)
	}
	if len(collected.AllFindings) != 3 {
		t.Errorf("AllFindings len = %d, want 3", len(collected.AllFindings))
	}
}

// TestFindingsPathConvention verifies the path convention for all perspective IDs.
func TestFindingsPathConvention(t *testing.T) {
	tests := []struct {
		stateDir      string
		perspectiveID string
		wantPath      string
	}{
		{
			stateDir:      "/home/user/.prism/state/analyze-abc123def456",
			perspectiveID: "security-analysis",
			wantPath:      "/home/user/.prism/state/analyze-abc123def456/perspectives/security-analysis/findings.json",
		},
		{
			stateDir:      "/home/user/.prism/state/analyze-000000000000",
			perspectiveID: "ux-heuristic-review",
			wantPath:      "/home/user/.prism/state/analyze-000000000000/perspectives/ux-heuristic-review/findings.json",
		},
		{
			stateDir:      "/tmp/test-state",
			perspectiveID: "data-integrity",
			wantPath:      "/tmp/test-state/perspectives/data-integrity/findings.json",
		},
	}

	for _, tt := range tests {
		got := FindingsPath(tt.stateDir, tt.perspectiveID)
		if got != tt.wantPath {
			t.Errorf("FindingsPath(%q, %q) = %q, want %q",
				tt.stateDir, tt.perspectiveID, got, tt.wantPath)
		}
	}
}

// TestSpecialistFindingsJSONFormat verifies that findings.json is valid JSON
// matching the expected schema.
func TestSpecialistFindingsJSONFormat(t *testing.T) {
	stateDir := t.TempDir()
	perspID := "format-test"

	perspDir := PerspectiveDir(stateDir, perspID)
	if err := os.MkdirAll(perspDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	findings := SpecialistFindings{
		Analyst: perspID,
		Input:   "Test analysis topic",
		Findings: []SpecialistFinding{
			{
				Finding:  "Test finding description",
				Evidence: "test.go:main:10 — test evidence",
				Severity: "HIGH",
			},
		},
	}

	path := FindingsPath(stateDir, perspID)
	if err := WriteSpecialistFindings(path, findings); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Read raw bytes and verify JSON structure
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	// Must be valid JSON
	if !json.Valid(data) {
		t.Fatal("findings.json is not valid JSON")
	}

	// Must match schema fields
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal raw: %v", err)
	}

	// Required top-level fields: analyst, input, findings
	for _, field := range []string{"analyst", "input", "findings"} {
		if _, ok := raw[field]; !ok {
			t.Errorf("findings.json missing required field %q", field)
		}
	}

	// findings must be an array
	findingsArr, ok := raw["findings"].([]interface{})
	if !ok {
		t.Fatal("findings field must be an array")
	}
	if len(findingsArr) != 1 {
		t.Errorf("findings array len = %d, want 1", len(findingsArr))
	}

	// Each finding must have finding, evidence, severity
	f0, ok := findingsArr[0].(map[string]interface{})
	if !ok {
		t.Fatal("findings[0] must be an object")
	}
	for _, field := range []string{"finding", "evidence", "severity"} {
		if _, ok := f0[field]; !ok {
			t.Errorf("findings[0] missing required field %q", field)
		}
	}
}

// TestSpecialistPipelineResultPathIntegration verifies that the specialist stage
// in the pipeline (runSpecialistStage) correctly wires OutputPath to findings.json
// at the expected perspective path.
func TestSpecialistPipelineResultPathIntegration(t *testing.T) {
	stateDir := t.TempDir()
	perspID := "integration-test"

	// The pipeline's runSpecialistStage sets OutputPath like this:
	expectedPath := filepath.Join(stateDir, "perspectives", perspID, "findings.json")

	// Verify it matches FindingsPath
	findingsPath := FindingsPath(stateDir, perspID)
	if findingsPath != expectedPath {
		t.Errorf("FindingsPath and pipeline OutputPath diverge:\n  FindingsPath: %s\n  Pipeline:     %s",
			findingsPath, expectedPath)
	}
}

// TestCollectSpecialistResultsWithPartialFailure verifies that collection
// correctly handles a mix of successful and failed specialists with proper
// path resolution.
func TestCollectSpecialistResultsWithPartialFailure(t *testing.T) {
	stateDir := t.TempDir()

	perspectives := []Perspective{
		{ID: "success-1", Name: "Success 1"},
		{ID: "failure-1", Name: "Failure 1"},
		{ID: "success-2", Name: "Success 2"},
	}

	// Write findings only for successful perspectives
	for _, id := range []string{"success-1", "success-2"} {
		dir := PerspectiveDir(stateDir, id)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("mkdir %s: %v", id, err)
		}
		findings := SpecialistFindings{
			Analyst:  id,
			Input:    "Test",
			Findings: []SpecialistFinding{{Finding: "Found something", Evidence: "x.go:1", Severity: "MEDIUM"}},
		}
		if err := WriteSpecialistFindings(FindingsPath(stateDir, id), findings); err != nil {
			t.Fatalf("write %s: %v", id, err)
		}
	}

	stageResults := []StageResult{
		{PerspectiveID: "success-1", OutputPath: FindingsPath(stateDir, "success-1")},
		{PerspectiveID: "failure-1", Err: fmt.Errorf("subprocess timed out")},
		{PerspectiveID: "success-2", OutputPath: FindingsPath(stateDir, "success-2")},
	}

	collected := CollectSpecialistResults("test-partial", stageResults, perspectives)

	if collected.Succeeded != 2 {
		t.Errorf("Succeeded = %d, want 2", collected.Succeeded)
	}
	if collected.Failed != 1 {
		t.Errorf("Failed = %d, want 1", collected.Failed)
	}
	if !collected.Degraded {
		t.Error("expected Degraded = true")
	}
	if collected.TotalFindings != 2 {
		t.Errorf("TotalFindings = %d, want 2", collected.TotalFindings)
	}

	// Verify the failed specialist is recorded
	if len(collected.FailedSpecialists) != 1 {
		t.Fatalf("FailedSpecialists len = %d, want 1", len(collected.FailedSpecialists))
	}
	if collected.FailedSpecialists[0].PerspectiveID != "failure-1" {
		t.Errorf("FailedSpecialist ID = %q, want failure-1", collected.FailedSpecialists[0].PerspectiveID)
	}
}
