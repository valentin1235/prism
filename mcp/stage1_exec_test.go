package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// --- Tests for extractJSON ---

func TestExtractJSON_CleanJSON(t *testing.T) {
	input := `{"topic":"test","da_passed":true,"research":{"summary":"s","findings":[],"key_areas":[],"files_examined":[],"mcp_queries":[]}}`
	got, err := extractJSON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != input {
		t.Errorf("expected clean pass-through, got %q", got)
	}
}

func TestExtractJSON_WithWhitespace(t *testing.T) {
	input := `  {"key": "value"}  `
	got, err := extractJSON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != `{"key": "value"}` {
		t.Errorf("expected trimmed JSON, got %q", got)
	}
}

func TestExtractJSON_MarkdownFences(t *testing.T) {
	input := "```json\n{\"key\": \"value\"}\n```"
	got, err := extractJSON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}
	if parsed["key"] != "value" {
		t.Errorf("expected key=value, got %v", parsed["key"])
	}
}

func TestExtractJSON_SurroundingText(t *testing.T) {
	input := `Here is the result:

{"topic":"test","da_passed":true,"research":{"summary":"s","findings":[],"key_areas":[],"files_examined":[],"mcp_queries":[]}}

That completes the analysis.`
	got, err := extractJSON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var parsed SeedAnalysis
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("result is not valid SeedAnalysis: %v", err)
	}
	if parsed.Topic != "test" {
		t.Errorf("expected topic=test, got %q", parsed.Topic)
	}
}

func TestExtractJSON_NestedBraces(t *testing.T) {
	input := `{"outer": {"inner": {"deep": "value"}}}`
	got, err := extractJSON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != input {
		t.Errorf("expected exact match for nested JSON, got %q", got)
	}
}

func TestExtractJSON_StringsWithBraces(t *testing.T) {
	input := `{"code": "func() { return }"}`
	got, err := extractJSON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}
	if parsed["code"] != "func() { return }" {
		t.Errorf("expected braces in string preserved, got %v", parsed["code"])
	}
}

func TestExtractJSON_NoJSON(t *testing.T) {
	input := "This is just plain text without any JSON"
	_, err := extractJSON(input)
	if err == nil {
		t.Error("expected error for input without JSON")
	}
}

func TestExtractJSON_EmptyInput(t *testing.T) {
	_, err := extractJSON("")
	if err == nil {
		t.Error("expected error for empty input")
	}
}

func TestExtractJSON_InvalidJSON(t *testing.T) {
	input := `{not valid json}`
	_, err := extractJSON(input)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

// --- Tests for runSeedAnalysis integration ---

func TestRunSeedAnalysis_WritesOutputFile(t *testing.T) {
	// This test verifies the file I/O and validation parts of runSeedAnalysis
	// by pre-creating a seed-analysis.json and verifying it can be read back.
	// The actual claude CLI call is tested via integration tests.
	tmpDir := t.TempDir()

	seed := SeedAnalysis{
		Topic:    "Test topic",
		DAPassed: true,
		Research: SeedResearch{
			Summary: "Found 3 areas",
			Findings: []SeedFinding{
				{ID: 1, Area: "auth", Description: "Auth module", Source: "auth.go:10", ToolUsed: "Grep"},
				{ID: 2, Area: "db", Description: "Database layer", Source: "db.go:20", ToolUsed: "Read"},
			},
			KeyAreas:      []string{"auth", "db"},
			FilesExamined: []string{"auth.go:10 — auth module", "db.go:20 — db layer"},
			MCPQueries:    []string{},
		},
	}

	outputPath := SeedAnalysisPath(tmpDir)
	if err := WriteSeedAnalysis(outputPath, seed); err != nil {
		t.Fatalf("write seed analysis: %v", err)
	}

	// Verify the file exists and can be read back
	got, err := ReadSeedAnalysis(outputPath)
	if err != nil {
		t.Fatalf("read seed analysis: %v", err)
	}

	if got.Topic != "Test topic" {
		t.Errorf("topic = %q, want %q", got.Topic, "Test topic")
	}
	if len(got.Research.Findings) != 2 {
		t.Errorf("findings count = %d, want 2", len(got.Research.Findings))
	}
}

// --- Tests for runPerspectiveGeneration integration ---

func TestRunPerspectiveGeneration_WritesAndValidatesOutput(t *testing.T) {
	// This test verifies the validation and file I/O parts of runPerspectiveGeneration.
	tmpDir := t.TempDir()

	perspectives := PerspectivesOutput{
		Perspectives: []Perspective{
			{
				ID:           "auth-analysis",
				Name:         "Authentication Analysis",
				Scope:        "Authentication flow and session management",
				KeyQuestions: []string{"How does auth work?", "Any bypasses?"},
				Model:        "sonnet",
				Prompt: AnalystPrompt{
					Role:               "You are the AUTH ANALYST.",
					InvestigationScope: "Authentication and authorization flows",
					Tasks:              "1. Check auth middleware\n2. Review session handling",
					OutputFormat:       "## Findings\n| Finding | Evidence | Severity |",
				},
				Rationale: "Seed finding #1 identified auth module as critical area",
			},
			{
				ID:           "db-analysis",
				Name:         "Database Analysis",
				Scope:        "Database access patterns and query safety",
				KeyQuestions: []string{"SQL injection risks?", "Connection pooling?"},
				Model:        "sonnet",
				Prompt: AnalystPrompt{
					Role:               "You are the DATABASE ANALYST.",
					InvestigationScope: "Database layer and ORM usage",
					Tasks:              "1. Check query patterns\n2. Review migrations",
					OutputFormat:       "## Findings\n| Finding | Evidence | Severity |",
				},
				Rationale: "Seed finding #2 identified database layer concerns",
			},
		},
		QualityGate: PerspectiveQualityGate{
			AllOrthogonal:    true,
			AllEvidenceBacked: true,
			AllSpecific:      true,
			AllActionable:    true,
			MinPerspectivesMet: true,
		},
		SelectionSummary: "Selected auth and db perspectives based on seed findings",
	}

	// Write perspectives
	outputPath := PerspectivesPath(tmpDir)
	if err := WritePerspectives(outputPath, perspectives); err != nil {
		t.Fatalf("write perspectives: %v", err)
	}

	// Verify the file exists and validates
	got, err := ReadPerspectives(outputPath)
	if err != nil {
		t.Fatalf("read perspectives: %v", err)
	}

	if err := ValidatePerspectives(got); err != nil {
		t.Fatalf("validate perspectives: %v", err)
	}

	if len(got.Perspectives) != 2 {
		t.Errorf("perspectives count = %d, want 2", len(got.Perspectives))
	}
	if got.Perspectives[0].ID != "auth-analysis" {
		t.Errorf("first perspective id = %q, want %q", got.Perspectives[0].ID, "auth-analysis")
	}
}

// --- Tests for runDAReviewLoop file operations ---

func TestRunDAReviewLoop_SeedAnalysisFileRequired(t *testing.T) {
	tmpDir := t.TempDir()
	task := &AnalysisTask{
		ID:       "analyze-test123",
		StateDir: tmpDir,
	}
	cfg := AnalysisConfig{
		Topic: "test",
		Model: "claude-sonnet-4-6",
	}

	// No seed-analysis.json exists — should fail
	err := runDAReviewLoop(task, cfg)
	if err == nil {
		t.Fatal("expected error when seed-analysis.json is missing")
	}
}

// --- Tests for runSupplementaryResearch merging ---

func TestSupplementaryResearchMerge(t *testing.T) {
	// Test that PatchSeedAnalysisFile correctly merges supplementary findings
	tmpDir := t.TempDir()
	seedPath := SeedAnalysisPath(tmpDir)

	// Write initial seed analysis
	initial := SeedAnalysis{
		Topic:    "Test topic",
		DAPassed: false,
		Research: SeedResearch{
			Summary:       "Initial summary",
			Findings:      []SeedFinding{{ID: 1, Area: "area1", Description: "desc1", Source: "file1:10", ToolUsed: "Grep"}},
			KeyAreas:      []string{"area1"},
			FilesExamined: []string{"file1:10 — found area1"},
			MCPQueries:    []string{},
		},
	}
	if err := WriteSeedAnalysis(seedPath, initial); err != nil {
		t.Fatalf("write initial seed: %v", err)
	}

	// Apply supplementary patch
	patch := SeedPatch{
		NewFindings: []SeedFinding{
			{ID: 99, Area: "area2", Description: "desc2", Source: "file2:20", ToolUsed: "Read"},
		},
		NewKeyAreas:      []string{"area2"},
		NewFilesExamined: []string{"file2:20 — found area2"},
		Summary:          "Updated summary with area2",
	}

	merged, err := PatchSeedAnalysisFile(seedPath, patch)
	if err != nil {
		t.Fatalf("patch seed analysis: %v", err)
	}

	// Verify merge results
	if len(merged.Research.Findings) != 2 {
		t.Errorf("expected 2 findings after merge, got %d", len(merged.Research.Findings))
	}
	// New finding should get auto-incremented ID (2, not 99)
	if merged.Research.Findings[1].ID != 2 {
		t.Errorf("expected auto-incremented ID 2, got %d", merged.Research.Findings[1].ID)
	}
	if merged.Research.Summary != "Updated summary with area2" {
		t.Errorf("expected updated summary, got %q", merged.Research.Summary)
	}
	if len(merged.Research.KeyAreas) != 2 {
		t.Errorf("expected 2 key areas, got %d", len(merged.Research.KeyAreas))
	}
}

// --- Tests for queryLLMScopedWithSchema parameter construction ---

func TestExtractJSON_PerspectivesOutput(t *testing.T) {
	// Test extractJSON with a realistic perspectives output
	perspJSON := `{
  "perspectives": [
    {
      "id": "test-perspective",
      "name": "Test Perspective",
      "scope": "Testing scope",
      "key_questions": ["Q1?", "Q2?"],
      "model": "sonnet",
      "prompt": {
        "role": "You are the TEST ANALYST.",
        "investigation_scope": "Test scope",
        "tasks": "1. Test task",
        "output_format": "## Findings"
      },
      "rationale": "Because of finding #1"
    },
    {
      "id": "test-perspective-2",
      "name": "Test Perspective 2",
      "scope": "Testing scope 2",
      "key_questions": ["Q3?", "Q4?"],
      "model": "opus",
      "prompt": {
        "role": "You are the TEST ANALYST 2.",
        "investigation_scope": "Test scope 2",
        "tasks": "1. Test task 2",
        "output_format": "## Findings 2"
      },
      "rationale": "Because of finding #2"
    }
  ],
  "quality_gate": {
    "all_orthogonal": true,
    "all_evidence_backed": true,
    "all_specific": true,
    "all_actionable": true,
    "min_perspectives_met": true
  },
  "selection_summary": "Two test perspectives selected"
}`

	got, err := extractJSON(perspJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var perspectives PerspectivesOutput
	if err := json.Unmarshal([]byte(got), &perspectives); err != nil {
		t.Fatalf("failed to parse extracted JSON: %v", err)
	}

	if len(perspectives.Perspectives) != 2 {
		t.Errorf("expected 2 perspectives, got %d", len(perspectives.Perspectives))
	}

	if err := ValidatePerspectives(perspectives); err != nil {
		t.Errorf("extracted perspectives failed validation: %v", err)
	}
}

func TestExtractJSON_SeedAnalysisOutput(t *testing.T) {
	// Test extractJSON with a realistic seed analysis output
	seedJSON := `{
  "topic": "Payment processing analysis",
  "da_passed": true,
  "research": {
    "summary": "Found 3 key areas related to payment processing",
    "findings": [
      {
        "id": 1,
        "area": "Payment Gateway",
        "description": "Stripe integration in payment_gateway.go",
        "source": "payment_gateway.go:handle_payment:45",
        "tool_used": "Grep"
      },
      {
        "id": 2,
        "area": "Transaction Logger",
        "description": "Transaction audit logging in tx_log.go",
        "source": "tx_log.go:log_transaction:22",
        "tool_used": "Read"
      }
    ],
    "key_areas": ["payment-gateway", "transaction-logging"],
    "files_examined": [
      "payment_gateway.go:45 — Stripe API calls",
      "tx_log.go:22 — audit trail"
    ],
    "mcp_queries": []
  }
}`

	got, err := extractJSON(seedJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var seed SeedAnalysis
	if err := json.Unmarshal([]byte(got), &seed); err != nil {
		t.Fatalf("failed to parse extracted JSON: %v", err)
	}

	if seed.Topic != "Payment processing analysis" {
		t.Errorf("expected topic, got %q", seed.Topic)
	}
	if len(seed.Research.Findings) != 2 {
		t.Errorf("expected 2 findings, got %d", len(seed.Research.Findings))
	}
}

// --- Tests for Stage 1 file path conventions ---

func TestStage1OutputPaths(t *testing.T) {
	stateDir := "/home/user/.prism/state/analyze-abc123"

	seedPath := SeedAnalysisPath(stateDir)
	perspPath := PerspectivesPath(stateDir)

	if seedPath != filepath.Join(stateDir, "seed-analysis.json") {
		t.Errorf("seed path = %q, want suffix seed-analysis.json", seedPath)
	}
	if perspPath != filepath.Join(stateDir, "perspectives.json") {
		t.Errorf("perspectives path = %q, want suffix perspectives.json", perspPath)
	}
}

// --- End-to-end file flow test ---

func TestStage1FileFlow_SeedToPerspectives(t *testing.T) {
	// Simulate the complete Stage 1 file flow:
	// 1. Write seed-analysis.json (as runSeedAnalysis would)
	// 2. Read it back (as runDAReviewLoop would)
	// 3. Update da_passed (as DA review pass would)
	// 4. Read for perspective gen (as runPerspectiveGeneration would)
	// 5. Write perspectives.json

	tmpDir := t.TempDir()

	// Step 1: Write seed analysis
	seed := SeedAnalysis{
		Topic:    "E2E test topic",
		DAPassed: false,
		Research: SeedResearch{
			Summary: "Found auth and db areas",
			Findings: []SeedFinding{
				{ID: 1, Area: "auth", Description: "Authentication module", Source: "auth.go:10", ToolUsed: "Grep"},
				{ID: 2, Area: "db", Description: "Database layer", Source: "db.go:20", ToolUsed: "Read"},
			},
			KeyAreas:      []string{"auth", "db"},
			FilesExamined: []string{"auth.go:10", "db.go:20"},
			MCPQueries:    []string{},
		},
	}
	seedPath := SeedAnalysisPath(tmpDir)
	if err := WriteSeedAnalysis(seedPath, seed); err != nil {
		t.Fatalf("write seed: %v", err)
	}

	// Step 2: Read back (DA review reads this)
	readBack, err := ReadSeedAnalysis(seedPath)
	if err != nil {
		t.Fatalf("read seed: %v", err)
	}
	if readBack.Topic != "E2E test topic" {
		t.Fatalf("topic mismatch after read")
	}

	// Step 3: Update da_passed (DA review pass)
	readBack.DAPassed = true
	if err := WriteSeedAnalysis(seedPath, readBack); err != nil {
		t.Fatalf("write DA passed: %v", err)
	}

	// Step 4: Read for perspective generation
	seedForPersp, err := os.ReadFile(seedPath)
	if err != nil {
		t.Fatalf("read seed for perspective gen: %v", err)
	}
	var seedParsed SeedAnalysis
	if err := json.Unmarshal(seedForPersp, &seedParsed); err != nil {
		t.Fatalf("parse seed for perspective gen: %v", err)
	}
	if !seedParsed.DAPassed {
		t.Error("expected da_passed=true after DA update")
	}

	// Step 5: Write perspectives
	perspectives := PerspectivesOutput{
		Perspectives: []Perspective{
			{
				ID:           "auth-review",
				Name:         "Auth Review",
				Scope:        "Authentication flows",
				KeyQuestions: []string{"Bypasses?", "Session mgmt?"},
				Model:        "sonnet",
				Prompt: AnalystPrompt{
					Role:               "You are the AUTH ANALYST.",
					InvestigationScope: "Auth flows",
					Tasks:              "1. Check middleware",
					OutputFormat:       "## Findings table",
				},
				Rationale: "Finding #1 identified auth module",
			},
			{
				ID:           "db-review",
				Name:         "DB Review",
				Scope:        "Database safety",
				KeyQuestions: []string{"SQL injection?", "Pooling?"},
				Model:        "sonnet",
				Prompt: AnalystPrompt{
					Role:               "You are the DB ANALYST.",
					InvestigationScope: "Database layer",
					Tasks:              "1. Review queries",
					OutputFormat:       "## Findings table",
				},
				Rationale: "Finding #2 identified db concerns",
			},
		},
		QualityGate: PerspectiveQualityGate{
			AllOrthogonal:      true,
			AllEvidenceBacked:  true,
			AllSpecific:        true,
			AllActionable:      true,
			MinPerspectivesMet: true,
		},
		SelectionSummary: "Auth and DB based on seed findings",
	}
	perspPath := PerspectivesPath(tmpDir)
	if err := WritePerspectives(perspPath, perspectives); err != nil {
		t.Fatalf("write perspectives: %v", err)
	}

	// Final verification: read back perspectives and validate
	gotPersp, err := ReadPerspectives(perspPath)
	if err != nil {
		t.Fatalf("read perspectives: %v", err)
	}
	if err := ValidatePerspectives(gotPersp); err != nil {
		t.Fatalf("validate perspectives: %v", err)
	}
	if len(gotPersp.Perspectives) != 2 {
		t.Errorf("expected 2 perspectives, got %d", len(gotPersp.Perspectives))
	}
}
