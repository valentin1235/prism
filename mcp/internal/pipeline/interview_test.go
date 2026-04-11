package pipeline

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildInterviewCommand_BasicFields(t *testing.T) {
	ictx := InterviewContext{
		Topic:             "analyze memory leak in worker pool",
		ContextID:         "analyze-abc123",
		Model:             "claude-sonnet-4-6",
		StateDir:          "/tmp/test-state",
		WorkDir:           "/tmp/workspace-root",
		SeedSummary:       "Worker pool has potential goroutine leak under high load.",
		OntologyScopeText: "- doc: worker pool implementation",
	}

	perspective := Perspective{
		ID:    "concurrency-analysis",
		Name:  "Concurrency Analysis",
		Scope: "Thread safety and goroutine lifecycle management",
		KeyQuestions: []string{
			"Are goroutines properly cleaned up on shutdown?",
			"Is the work queue bounded?",
		},
		Prompt: AnalystPrompt{
			Role:               "You are the CONCURRENCY ANALYST.",
			InvestigationScope: "Analyze goroutine lifecycle in worker pool",
			Tasks:              "1. Check shutdown paths\n2. Verify queue bounds",
			OutputFormat:       "Markdown with evidence",
		},
	}

	findings := SpecialistFindings{
		Analyst: "concurrency-analysis",
		Input:   "analyze memory leak in worker pool",
		Findings: []SpecialistFinding{
			{
				Finding:  "Goroutines not cleaned up on context cancellation",
				Evidence: "pkg/worker/pool.go:Start:42 — no select on ctx.Done()",
				Severity: "high",
			},
			{
				Finding:  "Unbounded work queue allows memory growth",
				Evidence: "pkg/worker/queue.go:Push:15 — channel has no buffer limit",
				Severity: "medium",
			},
		},
	}

	cmd := BuildInterviewCommand(ictx, perspective, findings)

	// Verify basic fields
	if cmd.PerspectiveID != "concurrency-analysis" {
		t.Errorf("PerspectiveID = %q, want %q", cmd.PerspectiveID, "concurrency-analysis")
	}
	if cmd.Model != "claude-sonnet-4-6" {
		t.Errorf("Model = %q, want %q", cmd.Model, "claude-sonnet-4-6")
	}
	if cmd.MaxTurns != 10 {
		t.Errorf("MaxTurns = %d, want 10", cmd.MaxTurns)
	}
	if cmd.WorkDir != "/tmp/workspace-root" {
		t.Errorf("WorkDir = %q, want analysis work dir", cmd.WorkDir)
	}
	if !strings.HasSuffix(cmd.OutputPath, "verified-findings.json") {
		t.Errorf("OutputPath = %q, should end with verified-findings.json", cmd.OutputPath)
	}
	if cmd.JSONSchema == "" {
		t.Error("JSONSchema should not be empty")
	}
}

func TestBuildInterviewCommand_SystemPromptContainsFindings(t *testing.T) {
	ictx := InterviewContext{
		Topic:       "analyze error handling",
		ContextID:   "analyze-def456",
		Model:       "claude-sonnet-4-6",
		StateDir:    "/tmp/test-state",
		WorkDir:     "/tmp/workspace-root",
		SeedSummary: "Error handling patterns vary across modules.",
	}

	perspective := Perspective{
		ID:           "error-handling",
		Name:         "Error Handling Analysis",
		Scope:        "Error propagation and recovery patterns",
		KeyQuestions: []string{"Are errors wrapped with context?", "Is there a recovery mechanism?"},
		Prompt: AnalystPrompt{
			Role:               "You are the ERROR HANDLING ANALYST.",
			InvestigationScope: "Analyze error handling patterns",
			Tasks:              "1. Check error wrapping",
			OutputFormat:       "Markdown",
		},
	}

	findings := SpecialistFindings{
		Analyst: "error-handling",
		Input:   "analyze error handling",
		Findings: []SpecialistFinding{
			{
				Finding:  "Errors silently swallowed in background workers",
				Evidence: "pkg/bg/runner.go:Run:78 — error return ignored",
				Severity: "critical",
			},
		},
	}

	cmd := BuildInterviewCommand(ictx, perspective, findings)

	// System prompt must contain the findings to verify
	if !strings.Contains(cmd.SystemPrompt, "Errors silently swallowed in background workers") {
		t.Error("SystemPrompt should contain the finding text")
	}
	if !strings.Contains(cmd.SystemPrompt, "pkg/bg/runner.go:Run:78") {
		t.Error("SystemPrompt should contain the evidence reference")
	}
	if !strings.Contains(cmd.SystemPrompt, "critical") {
		t.Error("SystemPrompt should contain the severity")
	}
}

func TestBuildInterviewCommand_SystemPromptContainsVerificationProtocol(t *testing.T) {
	ictx := InterviewContext{
		Topic:       "test topic",
		ContextID:   "analyze-test",
		Model:       "claude-sonnet-4-6",
		StateDir:    "/tmp/test",
		SeedSummary: "summary",
	}

	perspective := Perspective{
		ID:           "test-persp",
		Name:         "Test",
		Scope:        "testing",
		KeyQuestions: []string{"q1?", "q2?"},
		Prompt:       AnalystPrompt{Role: "You are TEST.", InvestigationScope: "test", Tasks: "1", OutputFormat: "md"},
	}

	findings := SpecialistFindings{
		Analyst:  "test-persp",
		Input:    "test topic",
		Findings: []SpecialistFinding{{Finding: "f1", Evidence: "e1", Severity: "low"}},
	}

	cmd := BuildInterviewCommand(ictx, perspective, findings)

	// Must contain verification protocol sections
	checks := []string{
		"Verification Protocol",
		"Assumption Check",
		"Relevance Check",
		"Constraints Check",
		"Re-investigate Weak Points",
		"confirmed",
		"revised",
		"withdrawn",
		"weighted_total",
	}
	for _, check := range checks {
		if !strings.Contains(cmd.SystemPrompt, check) {
			t.Errorf("SystemPrompt should contain %q", check)
		}
	}
}

func TestBuildInterviewCommand_SystemPromptContainsContext(t *testing.T) {
	ictx := InterviewContext{
		Topic:             "analyze API design",
		ContextID:         "analyze-ctx123",
		Model:             "claude-sonnet-4-6",
		StateDir:          "/tmp/test",
		SeedSummary:       "API has inconsistent naming conventions.",
		OntologyScopeText: "- doc: API specification v2",
	}

	perspective := Perspective{
		ID:           "api-design",
		Name:         "API Design Analysis",
		Scope:        "REST API consistency",
		KeyQuestions: []string{"Are naming conventions consistent?", "Is versioning handled?"},
		Prompt:       AnalystPrompt{Role: "You are the API ANALYST.", InvestigationScope: "API", Tasks: "1", OutputFormat: "md"},
	}

	findings := SpecialistFindings{
		Analyst:  "api-design",
		Findings: []SpecialistFinding{{Finding: "f", Evidence: "e", Severity: "low"}},
	}

	cmd := BuildInterviewCommand(ictx, perspective, findings)

	// Must contain topic, seed summary, ontology scope
	if !strings.Contains(cmd.SystemPrompt, "analyze API design") {
		t.Error("SystemPrompt should contain the topic")
	}
	if !strings.Contains(cmd.SystemPrompt, "API has inconsistent naming conventions.") {
		t.Error("SystemPrompt should contain seed summary")
	}
	if !strings.Contains(cmd.SystemPrompt, "API specification v2") {
		t.Error("SystemPrompt should contain ontology scope text")
	}
	// Key questions should appear
	if !strings.Contains(cmd.SystemPrompt, "Are naming conventions consistent?") {
		t.Error("SystemPrompt should contain key questions")
	}
}

func TestBuildInterviewCommand_DataSourceConstraint(t *testing.T) {
	ictx := InterviewContext{
		Topic: "test", ContextID: "analyze-t", Model: "claude-sonnet-4-6",
		StateDir: "/tmp/t", SeedSummary: "s",
	}
	perspective := Perspective{
		ID: "t", Name: "T", Scope: "t", KeyQuestions: []string{"q1?", "q2?"},
		Prompt: AnalystPrompt{Role: "R", InvestigationScope: "I", Tasks: "T", OutputFormat: "O"},
	}
	findings := SpecialistFindings{
		Analyst:  "t",
		Findings: []SpecialistFinding{{Finding: "f", Evidence: "e", Severity: "low"}},
	}

	cmd := BuildInterviewCommand(ictx, perspective, findings)

	if !strings.Contains(cmd.SystemPrompt, "Data Source Constraint") {
		t.Error("SystemPrompt should contain data source constraint")
	}
	if !strings.Contains(cmd.SystemPrompt, "MUST only use data sources") {
		t.Error("SystemPrompt should enforce data source restriction")
	}
}

func TestBuildInterviewCommand_UserPromptContent(t *testing.T) {
	ictx := InterviewContext{
		Topic: "analyze deployment pipeline", ContextID: "analyze-dp", Model: "claude-sonnet-4-6",
		StateDir: "/tmp/dp", SeedSummary: "Pipeline has flaky tests.",
	}
	perspective := Perspective{
		ID: "pipeline", Name: "Pipeline Analysis", Scope: "CI/CD reliability",
		KeyQuestions: []string{"Are retries idempotent?", "Is rollback tested?"},
		Prompt:       AnalystPrompt{Role: "R", InvestigationScope: "I", Tasks: "T", OutputFormat: "O"},
	}
	findings := SpecialistFindings{
		Analyst: "pipeline",
		Findings: []SpecialistFinding{
			{Finding: "f1", Evidence: "e1", Severity: "high"},
			{Finding: "f2", Evidence: "e2", Severity: "medium"},
			{Finding: "f3", Evidence: "e3", Severity: "low"},
		},
	}

	cmd := BuildInterviewCommand(ictx, perspective, findings)

	if !strings.Contains(cmd.UserPrompt, "Pipeline Analysis") {
		t.Error("UserPrompt should contain perspective name")
	}
	if !strings.Contains(cmd.UserPrompt, "analyze deployment pipeline") {
		t.Error("UserPrompt should contain topic")
	}
	if !strings.Contains(cmd.UserPrompt, "3 findings") {
		t.Error("UserPrompt should contain findings count")
	}
}

func TestVerifiedFindingsJSONSchema_Valid(t *testing.T) {
	schema := VerifiedFindingsSchema()
	if schema == "" {
		t.Fatal("Schema should not be empty")
	}

	// Must be valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(schema), &parsed); err != nil {
		t.Fatalf("Schema is not valid JSON: %v", err)
	}

	// Check required fields
	required, ok := parsed["required"].([]interface{})
	if !ok {
		t.Fatal("Schema should have required field")
	}

	requiredFields := make(map[string]bool)
	for _, r := range required {
		requiredFields[r.(string)] = true
	}

	for _, field := range []string{"analyst", "topic", "verdict", "score", "findings", "summary"} {
		if !requiredFields[field] {
			t.Errorf("Schema should require field %q", field)
		}
	}
}

func TestRenderFindingsForVerification(t *testing.T) {
	findings := SpecialistFindings{
		Analyst: "test-analyst",
		Findings: []SpecialistFinding{
			{Finding: "Memory leak in handler", Evidence: "handler.go:42", Severity: "high"},
			{Finding: "Missing input validation", Evidence: "api.go:15", Severity: "medium"},
		},
	}

	rendered := renderFindingsForVerification(findings)

	if !strings.Contains(rendered, "test-analyst") {
		t.Error("Should contain analyst name")
	}
	if !strings.Contains(rendered, "Total findings: 2") {
		t.Error("Should contain findings count")
	}
	if !strings.Contains(rendered, "Finding 1") {
		t.Error("Should contain finding numbers")
	}
	if !strings.Contains(rendered, "Memory leak in handler") {
		t.Error("Should contain finding text")
	}
	if !strings.Contains(rendered, "handler.go:42") {
		t.Error("Should contain evidence")
	}
}

func TestVerifiedFindings_ReadWrite(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "verified-findings.json")

	original := VerifiedFindings{
		Analyst: "test-analyst",
		Topic:   "test topic",
		Verdict: "pass",
		Score: VerificationScore{
			Assumption:    0.9,
			Relevance:     0.85,
			Constraints:   0.8,
			WeightedTotal: 0.86,
		},
		Findings: []VerifiedFinding{
			{
				Finding:      "Memory leak confirmed",
				Evidence:     "handler.go:42",
				Severity:     "high",
				Status:       "confirmed",
				Verification: "Verified via code inspection — goroutine not released on error path",
			},
		},
		Summary: "All findings verified with minor caveats",
	}

	if err := WriteVerifiedFindings(path, original); err != nil {
		t.Fatalf("WriteVerifiedFindings: %v", err)
	}

	loaded, err := ReadVerifiedFindings(path)
	if err != nil {
		t.Fatalf("ReadVerifiedFindings: %v", err)
	}

	if loaded.Analyst != original.Analyst {
		t.Errorf("Analyst = %q, want %q", loaded.Analyst, original.Analyst)
	}
	if loaded.Verdict != original.Verdict {
		t.Errorf("Verdict = %q, want %q", loaded.Verdict, original.Verdict)
	}
	if loaded.Score.WeightedTotal != original.Score.WeightedTotal {
		t.Errorf("WeightedTotal = %f, want %f", loaded.Score.WeightedTotal, original.Score.WeightedTotal)
	}
	if len(loaded.Findings) != 1 {
		t.Fatalf("Findings count = %d, want 1", len(loaded.Findings))
	}
	if loaded.Findings[0].Status != "confirmed" {
		t.Errorf("Finding status = %q, want %q", loaded.Findings[0].Status, "confirmed")
	}
}

func TestVerifiedFindingsPath(t *testing.T) {
	path := VerifiedFindingsPath("/home/user/.prism/state/analyze-abc123", "concurrency-analysis")
	expected := "/home/user/.prism/state/analyze-abc123/perspectives/concurrency-analysis/verified-findings.json"
	if path != expected {
		t.Errorf("VerifiedFindingsPath = %q, want %q", path, expected)
	}
}

func TestBuildAllInterviewCommands_SkipsFailedSpecialists(t *testing.T) {
	// This test verifies that BuildAllInterviewCommands skips perspectives
	// whose specialist stage failed (StageResult.Err != nil)
	tmpDir := t.TempDir()
	stateDir := filepath.Join(tmpDir, "state")

	// Create seed analysis
	seedDir := stateDir
	os.MkdirAll(seedDir, 0755)
	seedAnalysis := SeedAnalysis{
		Summary: "Test seed summary",
	}
	seedData, _ := json.MarshalIndent(seedAnalysis, "", "  ")
	os.WriteFile(filepath.Join(seedDir, "seed-analysis.json"), seedData, 0644)

	// Create findings for perspective 1 only
	perspDir1 := filepath.Join(stateDir, "perspectives", "persp-1")
	os.MkdirAll(perspDir1, 0755)
	f1 := SpecialistFindings{
		Analyst:  "persp-1",
		Findings: []SpecialistFinding{{Finding: "f1", Evidence: "e1", Severity: "high"}},
	}
	f1Data, _ := json.MarshalIndent(f1, "", "  ")
	os.WriteFile(filepath.Join(perspDir1, "findings.json"), f1Data, 0644)

	cfg := AnalysisConfig{
		Topic:     "test",
		ContextID: "analyze-test",
		Model:     "claude-sonnet-4-6",
		Adaptor:   "codex",
		StateDir:  stateDir,
	}

	perspectives := []Perspective{
		{ID: "persp-1", Name: "P1", Scope: "s1", KeyQuestions: []string{"q1?", "q2?"},
			Prompt: AnalystPrompt{Role: "R1", InvestigationScope: "I1", Tasks: "T1", OutputFormat: "O1"}},
		{ID: "persp-2", Name: "P2", Scope: "s2", KeyQuestions: []string{"q1?", "q2?"},
			Prompt: AnalystPrompt{Role: "R2", InvestigationScope: "I2", Tasks: "T2", OutputFormat: "O2"}},
	}

	results := []StageResult{
		{PerspectiveID: "persp-1", Err: nil},            // succeeded
		{PerspectiveID: "persp-2", Err: os.ErrNotExist}, // failed
	}

	commands, err := BuildAllInterviewCommands(cfg, perspectives, results)
	if err != nil {
		t.Fatalf("BuildAllInterviewCommands: %v", err)
	}

	// Should only have 1 command (persp-2 was skipped due to error)
	if len(commands) != 1 {
		t.Fatalf("Expected 1 command, got %d", len(commands))
	}
	if commands[0].PerspectiveID != "persp-1" {
		t.Errorf("Expected persp-1, got %q", commands[0].PerspectiveID)
	}
	if commands[0].Adaptor != "codex" {
		t.Fatalf("Adaptor = %q, want %q", commands[0].Adaptor, "codex")
	}
}

func TestBuildInterviewCommand_NoOntologyScope(t *testing.T) {
	ictx := InterviewContext{
		Topic: "test", ContextID: "analyze-t", Model: "claude-sonnet-4-6",
		StateDir: "/tmp/t", SeedSummary: "s",
		OntologyScopeText: "", // no ontology scope
	}
	perspective := Perspective{
		ID: "t", Name: "T", Scope: "t", KeyQuestions: []string{"q1?", "q2?"},
		Prompt: AnalystPrompt{Role: "R", InvestigationScope: "I", Tasks: "T", OutputFormat: "O"},
	}
	findings := SpecialistFindings{
		Analyst:  "t",
		Findings: []SpecialistFinding{{Finding: "f", Evidence: "e", Severity: "low"}},
	}

	cmd := BuildInterviewCommand(ictx, perspective, findings)

	// Should contain fallback message
	if !strings.Contains(cmd.SystemPrompt, "N/A") {
		t.Error("SystemPrompt should contain N/A fallback when no ontology scope")
	}
	// Should NOT contain "Analysis Target Directories" section
	if strings.Contains(cmd.SystemPrompt, "Analysis Target Directories") {
		t.Error("SystemPrompt should not contain target directories section when no doc paths")
	}
}
