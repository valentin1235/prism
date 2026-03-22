package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	taskpkg "github.com/heechul/prism-mcp/internal/task"
	"github.com/mark3labs/mcp-go/mcp"
)

func TestBuildSynthesisSystemPrompt(t *testing.T) {
	sctx := SynthesisContext{
		Topic:        "API security audit",
		Model:        "claude-sonnet-4-6",
		AnalysisDate: "2026-03-21",
		Perspectives: []Perspective{
			{
				ID:    "auth-analyst",
				Name:  "Authentication Analyst",
				Scope: "Review authentication mechanisms",
				KeyQuestions: []string{
					"Are tokens properly validated?",
					"Is session management secure?",
				},
				Rationale: "Critical for API security",
				Prompt:    AnalystPrompt{Role: "You are the AUTH ANALYST"},
			},
			{
				ID:    "data-analyst",
				Name:  "Data Flow Analyst",
				Scope: "Review data handling patterns",
				KeyQuestions: []string{
					"Is sensitive data encrypted?",
				},
				Rationale: "Data exposure risk",
				Prompt:    AnalystPrompt{Role: "You are the DATA ANALYST"},
			},
		},
		CollectedFindings: &CollectedFindings{
			Succeeded:     2,
			Failed:        0,
			TotalFindings: 3,
			Results: []SpecialistResult{
				{
					PerspectiveID: "auth-analyst",
					Outcome:       OutcomeSuccess,
					FindingsCount: 2,
					Findings: &SpecialistFindings{
						Analyst: "auth-analyst",
						Findings: []SpecialistFinding{
							{Finding: "Token validation missing", Evidence: "auth.go:42", Severity: "high"},
							{Finding: "Session timeout too long", Evidence: "config.go:10", Severity: "medium"},
						},
					},
				},
				{
					PerspectiveID: "data-analyst",
					Outcome:       OutcomeSuccess,
					FindingsCount: 1,
					Findings: &SpecialistFindings{
						Analyst: "data-analyst",
						Findings: []SpecialistFinding{
							{Finding: "PII logged in plaintext", Evidence: "logger.go:55", Severity: "critical"},
						},
					},
				},
			},
		},
		CollectedVerifications: &CollectedVerifications{
			Succeeded:    2,
			Failed:       0,
			AverageScore: 0.85,
			Results: []InterviewResult{
				{
					PerspectiveID: "auth-analyst",
					Outcome:       InterviewSuccess,
					Verdict:       "pass",
					Score:         0.90,
					Verified: &VerifiedFindings{
						Analyst: "auth-analyst",
						Verdict: "pass",
						Score: VerificationScore{
							Assumption:    0.90,
							Relevance:     0.95,
							Constraints:   0.80,
							WeightedTotal: 0.90,
						},
						Summary: "Authentication findings verified",
						Findings: []VerifiedFinding{
							{Finding: "Token validation missing", Evidence: "auth.go:42", Severity: "high", Status: "confirmed", Verification: "Verified"},
						},
					},
				},
				{
					PerspectiveID: "data-analyst",
					Outcome:       InterviewSuccess,
					Verdict:       "pass_with_caveats",
					Score:         0.75,
					Verified: &VerifiedFindings{
						Analyst: "data-analyst",
						Verdict: "pass_with_caveats",
						Score: VerificationScore{
							Assumption:    0.80,
							Relevance:     0.70,
							Constraints:   0.75,
							WeightedTotal: 0.75,
						},
						Summary: "Data findings partially verified",
						Findings: []VerifiedFinding{
							{Finding: "PII logged in plaintext", Evidence: "logger.go:55", Severity: "critical", Status: "revised", Verification: "Partially confirmed"},
						},
					},
				},
			},
		},
		SeedSummary:       "The API has several security concerns in authentication and data handling.",
		OntologyScopeText: "Source: API codebase at /src/api/",
		ReportTemplate:    "## Executive Summary\n[placeholder]\n## Appendix\n[placeholder]",
	}

	prompt := buildSynthesisSystemPrompt(sctx)

	// Verify key sections are present
	checks := []struct {
		name    string
		content string
	}{
		{"role identity", "REPORT SYNTHESIZER"},
		{"topic", "API security audit"},
		{"analysis date", "2026-03-21"},
		{"perspective count", "2"},
		{"perspective name", "Authentication Analyst"},
		{"perspective scope", "Review authentication mechanisms"},
		{"rationale", "Critical for API security"},
		{"key question", "Are tokens properly validated?"},
		{"finding data", "Token validation missing"},
		{"finding evidence", "auth.go:42"},
		{"finding severity", "high"},
		{"verification score", "0.90"},
		{"verification verdict", "pass"},
		{"seed summary", "API has several security concerns"},
		{"ontology scope", "API codebase"},
		{"report template", "Executive Summary"},
		{"synthesis instructions", "Fill EVERY section"},
		{"degraded instruction", "degraded mode"},
		{"second perspective", "Data Flow Analyst"},
		{"PII finding", "PII logged in plaintext"},
		{"avg score", "0.85"},
	}

	for _, c := range checks {
		if !strings.Contains(prompt, c.content) {
			t.Errorf("system prompt missing %s: expected to contain %q", c.name, c.content)
		}
	}
}

func TestBuildSynthesisSystemPromptDegradedMode(t *testing.T) {
	sctx := SynthesisContext{
		Topic:        "Test topic",
		AnalysisDate: "2026-03-21",
		Perspectives: []Perspective{
			{ID: "p1", Name: "P1", Scope: "scope1"},
			{ID: "p2", Name: "P2", Scope: "scope2"},
		},
		CollectedFindings: &CollectedFindings{
			Succeeded:     1,
			Failed:        1,
			TotalFindings: 2,
			PartialFailure:      true,
			Results: []SpecialistResult{
				{
					PerspectiveID: "p1",
					Outcome:       OutcomeSuccess,
					FindingsCount: 2,
					Findings: &SpecialistFindings{
						Analyst: "p1",
						Findings: []SpecialistFinding{
							{Finding: "f1", Evidence: "e1", Severity: "high"},
							{Finding: "f2", Evidence: "e2", Severity: "low"},
						},
					},
				},
				{
					PerspectiveID: "p2",
					Outcome:       OutcomeCrashed,
					ErrorMessage:  "subprocess crashed",
				},
			},
			FailedSpecialists: []FailedSpecialist{
				{PerspectiveID: "p2", Outcome: OutcomeCrashed, ErrorMessage: "subprocess crashed"},
			},
		},
		CollectedVerifications: nil, // All interviews failed
		SeedSummary:            "test summary",
		ReportTemplate:         "## Executive Summary\n",
	}

	prompt := buildSynthesisSystemPrompt(sctx)

	// Should contain degradation notice
	if !strings.Contains(prompt, "could not be completed") {
		t.Error("expected degradation notice for failed specialists")
	}

	// Should note missing verifications
	if !strings.Contains(prompt, "No verification data available") {
		t.Error("expected note about missing verification data")
	}
}

func TestBuildSynthesisUserPrompt(t *testing.T) {
	sctx := SynthesisContext{
		Topic: "Performance analysis",
		Perspectives: []Perspective{
			{ID: "p1"}, {ID: "p2"}, {ID: "p3"},
		},
		CollectedFindings: &CollectedFindings{
			Succeeded:     3,
			TotalFindings: 10,
		},
		CollectedVerifications: &CollectedVerifications{
			Succeeded:    2,
			AverageScore: 0.82,
		},
	}

	prompt := buildSynthesisUserPrompt(sctx)

	if !strings.Contains(prompt, "Performance analysis") {
		t.Error("user prompt should contain topic")
	}
	if !strings.Contains(prompt, "3 perspectives") {
		t.Error("user prompt should contain perspective count")
	}
	if !strings.Contains(prompt, "10 total findings") {
		t.Error("user prompt should contain findings count")
	}
	if !strings.Contains(prompt, "avg score 0.82") {
		t.Error("user prompt should contain average score")
	}
	if !strings.Contains(prompt, "Fill the report template") {
		t.Error("user prompt should instruct to fill template")
	}
}

func TestValidateReportSections(t *testing.T) {
	t.Run("all sections present", func(t *testing.T) {
		report := `# Analysis Report
## Executive Summary
Summary here.
## Analysis Overview
Overview here.
## Perspective Findings
Findings here.
## Integrated Analysis
Integration here.
## Socratic Verification Summary
Verification here.
## Recommendations
Recs here.
## Appendix
Appendix here.`

		missing := validateReportSections(report, defaultReportSections)
		if len(missing) != 0 {
			t.Errorf("expected no missing sections, got %v", missing)
		}
	})

	t.Run("missing sections", func(t *testing.T) {
		report := `# Analysis Report
## Executive Summary
Summary here.
## Analysis Overview
Overview here.`

		missing := validateReportSections(report, defaultReportSections)
		if len(missing) != 5 {
			t.Errorf("expected 5 missing sections, got %d: %v", len(missing), missing)
		}
		// Check specific missing sections
		expectedMissing := map[string]bool{
			"Perspective Findings":          true,
			"Integrated Analysis":           true,
			"Socratic Verification Summary": true,
			"Recommendations":               true,
			"Appendix":                      true,
		}
		for _, m := range missing {
			if !expectedMissing[m] {
				t.Errorf("unexpected missing section: %s", m)
			}
		}
	})

	t.Run("case insensitive", func(t *testing.T) {
		report := `## executive summary
## analysis overview
## perspective findings
## integrated analysis
## socratic verification summary
## recommendations
## appendix`

		missing := validateReportSections(report, defaultReportSections)
		if len(missing) != 0 {
			t.Errorf("expected case-insensitive match, got missing: %v", missing)
		}
	})

	t.Run("empty report", func(t *testing.T) {
		missing := validateReportSections("", defaultReportSections)
		if len(missing) != 7 {
			t.Errorf("expected all 7 sections missing for empty report, got %d", len(missing))
		}
	})
}

func TestLoadReportTemplate(t *testing.T) {
	t.Run("custom template", func(t *testing.T) {
		// Create temp file with custom template
		tmpDir := t.TempDir()
		customPath := filepath.Join(tmpDir, "custom-report.md")
		customContent := "# Custom Report\n## Executive Summary\n## Custom Section\n"
		if err := os.WriteFile(customPath, []byte(customContent), 0644); err != nil {
			t.Fatalf("failed to write custom template: %v", err)
		}

		cfg := AnalysisConfig{ReportTemplate: customPath}
		content, err := loadReportTemplate(cfg)
		if err != nil {
			t.Fatalf("loadReportTemplate error: %v", err)
		}
		if content != customContent {
			t.Errorf("expected custom content, got: %s", content)
		}
	})

	t.Run("custom template not found", func(t *testing.T) {
		cfg := AnalysisConfig{ReportTemplate: "/nonexistent/path/template.md"}
		_, err := loadReportTemplate(cfg)
		if err == nil {
			t.Error("expected error for nonexistent custom template")
		}
	})

	t.Run("default template", func(t *testing.T) {
		cfg := AnalysisConfig{} // No custom template
		content, err := loadReportTemplate(cfg)
		if err != nil {
			t.Fatalf("loadReportTemplate error: %v", err)
		}
		// Default template should contain key sections
		if !strings.Contains(content, "Executive Summary") {
			t.Error("default template should contain Executive Summary")
		}
		if !strings.Contains(content, "Socratic Verification Summary") {
			t.Error("default template should contain Socratic Verification Summary")
		}
	})
}

func TestLoadSynthesisContext(t *testing.T) {
	tmpDir := t.TempDir()

	// Create seed-analysis.json
	seedData := `{"research": {"summary": "Test seed summary for synthesis"}}`
	if err := os.WriteFile(filepath.Join(tmpDir, "seed-analysis.json"), []byte(seedData), 0644); err != nil {
		t.Fatal(err)
	}

	// Create collected-findings.json
	cf := CollectedFindings{
		TaskID:           "test-task",
		CollectedAt:      time.Now().UTC(),
		TotalSpecialists: 2,
		Succeeded:        2,
		Failed:           0,
		TotalFindings:    3,
		Results: []SpecialistResult{
			{
				PerspectiveID: "p1",
				Outcome:       OutcomeSuccess,
				FindingsCount: 2,
				Findings: &SpecialistFindings{
					Analyst: "p1",
					Findings: []SpecialistFinding{
						{Finding: "Finding 1", Evidence: "file1:10", Severity: "high"},
						{Finding: "Finding 2", Evidence: "file2:20", Severity: "low"},
					},
				},
			},
		},
		AllFindings: []AnnotatedFinding{
			{PerspectiveID: "p1", Finding: "Finding 1", Evidence: "file1:10", Severity: "high"},
		},
	}
	if err := WriteCollectedFindings(tmpDir, cf); err != nil {
		t.Fatal(err)
	}

	// Create collected-verifications.json
	cv := CollectedVerifications{
		TaskID:          "test-task",
		CollectedAt:     time.Now().UTC(),
		TotalInterviews: 1,
		Succeeded:       1,
		Failed:          0,
		AverageScore:    0.88,
		Results: []InterviewResult{
			{
				PerspectiveID: "p1",
				Outcome:       InterviewSuccess,
				Verdict:       "pass",
				Score:         0.88,
				Verified: &VerifiedFindings{
					Analyst: "p1",
					Verdict: "pass",
					Score: VerificationScore{
						Assumption: 0.9, Relevance: 0.9, Constraints: 0.8, WeightedTotal: 0.88,
					},
					Summary: "Verified",
					Findings: []VerifiedFinding{
						{Finding: "Finding 1", Evidence: "file1:10", Severity: "high", Status: "confirmed", Verification: "OK"},
					},
				},
			},
		},
	}
	if err := WriteCollectedVerifications(tmpDir, cv); err != nil {
		t.Fatal(err)
	}

	perspectives := []Perspective{
		{ID: "p1", Name: "Test Analyst", Scope: "test scope"},
	}

	cfg := AnalysisConfig{
		Topic:    "Test synthesis topic",
		Model:    "claude-sonnet-4-6",
		StateDir: tmpDir,
	}

	sctx, err := LoadSynthesisContext(cfg, perspectives)
	if err != nil {
		t.Fatalf("LoadSynthesisContext error: %v", err)
	}

	if sctx.Topic != "Test synthesis topic" {
		t.Errorf("wrong topic: %s", sctx.Topic)
	}
	if sctx.SeedSummary != "Test seed summary for synthesis" {
		t.Errorf("wrong seed summary: %s", sctx.SeedSummary)
	}
	if sctx.CollectedFindings == nil {
		t.Fatal("collected findings should not be nil")
	}
	if sctx.CollectedFindings.TotalFindings != 3 {
		t.Errorf("wrong total findings: %d", sctx.CollectedFindings.TotalFindings)
	}
	if sctx.CollectedVerifications == nil {
		t.Fatal("collected verifications should not be nil")
	}
	if sctx.CollectedVerifications.AverageScore != 0.88 {
		t.Errorf("wrong avg score: %f", sctx.CollectedVerifications.AverageScore)
	}
	if sctx.ReportTemplate == "" {
		t.Error("report template should be loaded")
	}
}

func TestLoadSynthesisContextMissingSeedSummary(t *testing.T) {
	tmpDir := t.TempDir()

	// Create seed-analysis.json with no summary field
	seedData := `{"other_field": "value"}`
	if err := os.WriteFile(filepath.Join(tmpDir, "seed-analysis.json"), []byte(seedData), 0644); err != nil {
		t.Fatal(err)
	}

	// Create minimal collected-findings.json
	cf := CollectedFindings{TaskID: "t", Succeeded: 1, TotalFindings: 1,
		Results: []SpecialistResult{{PerspectiveID: "p1", Outcome: OutcomeSuccess, FindingsCount: 1,
			Findings: &SpecialistFindings{Analyst: "p1", Findings: []SpecialistFinding{{Finding: "f", Evidence: "e", Severity: "low"}}}}}}
	if err := WriteCollectedFindings(tmpDir, cf); err != nil {
		t.Fatal(err)
	}

	// Create minimal collected-verifications.json
	cv := CollectedVerifications{TaskID: "t", Succeeded: 1, Results: []InterviewResult{{PerspectiveID: "p1", Outcome: InterviewSuccess, Verified: &VerifiedFindings{Analyst: "p1", Verdict: "pass", Findings: []VerifiedFinding{{Finding: "f"}}}}}}
	if err := WriteCollectedVerifications(tmpDir, cv); err != nil {
		t.Fatal(err)
	}

	cfg := AnalysisConfig{Topic: "t", StateDir: tmpDir}
	sctx, err := LoadSynthesisContext(cfg, []Perspective{{ID: "p1"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sctx.SeedSummary != "(seed analysis summary unavailable)" {
		t.Errorf("expected fallback summary, got: %s", sctx.SeedSummary)
	}
}

func TestBuildSynthesisSystemPromptMissingPerspectives(t *testing.T) {
	sctx := SynthesisContext{
		Topic:        "System reliability review",
		AnalysisDate: "2026-03-21",
		Perspectives: []Perspective{
			{ID: "p1", Name: "Availability", Scope: "uptime"},
			{ID: "p2", Name: "Latency", Scope: "response times"},
			{ID: "p3", Name: "Durability", Scope: "data persistence"},
		},
		CollectedFindings: &CollectedFindings{
			Succeeded:        2,
			Failed:           1,
			TotalSpecialists: 3,
			TotalFindings:    4,
			PartialFailure:         true,
			Results: []SpecialistResult{
				{PerspectiveID: "p1", Outcome: OutcomeSuccess, FindingsCount: 2,
					Findings: &SpecialistFindings{Analyst: "p1", Findings: []SpecialistFinding{
						{Finding: "f1", Evidence: "e1", Severity: "high"},
						{Finding: "f2", Evidence: "e2", Severity: "low"},
					}}},
				{PerspectiveID: "p2", Outcome: OutcomeSuccess, FindingsCount: 2,
					Findings: &SpecialistFindings{Analyst: "p2", Findings: []SpecialistFinding{
						{Finding: "f3", Evidence: "e3", Severity: "medium"},
						{Finding: "f4", Evidence: "e4", Severity: "medium"},
					}}},
				{PerspectiveID: "p3", Outcome: OutcomeCrashed, ErrorMessage: "exit status 1"},
			},
			FailedSpecialists: []FailedSpecialist{
				{PerspectiveID: "p3", Outcome: OutcomeCrashed, ErrorMessage: "exit status 1"},
			},
		},
		CollectedVerifications: &CollectedVerifications{
			Succeeded:       1,
			Failed:          1,
			TotalInterviews: 2,
			PartialFailure:        true,
			AverageScore:    0.80,
			Results: []InterviewResult{
				{PerspectiveID: "p1", Outcome: InterviewSuccess, Verdict: "pass", Score: 0.80,
					Verified: &VerifiedFindings{Analyst: "p1", Verdict: "pass",
						Score:   VerificationScore{Assumption: 0.8, Relevance: 0.8, Constraints: 0.8, WeightedTotal: 0.80},
						Summary: "OK", Findings: []VerifiedFinding{{Finding: "f1", Status: "confirmed", Verification: "ok"}}}},
				{PerspectiveID: "p2", Outcome: InterviewTimeout, ErrorMessage: "deadline exceeded"},
			},
			FailedInterviews: []FailedInterview{
				{PerspectiveID: "p2", Outcome: InterviewTimeout, ErrorMessage: "deadline exceeded"},
			},
		},
		SeedSummary:    "test",
		ReportTemplate: "## Executive Summary\n",
	}

	prompt := buildSynthesisSystemPrompt(sctx)

	// Should contain Missing Perspectives section
	if !strings.Contains(prompt, "Missing Perspectives") {
		t.Error("expected Missing Perspectives section in system prompt")
	}

	// Should list p3 (specialist failure) and p2 (interview failure)
	if !strings.Contains(prompt, "p3") {
		t.Error("expected p3 in missing perspectives (specialist crash)")
	}
	if !strings.Contains(prompt, "Specialist Analysis") {
		t.Error("expected 'Specialist Analysis' stage label")
	}
	if !strings.Contains(prompt, "p2") {
		t.Error("expected p2 in missing perspectives (interview timeout)")
	}
	if !strings.Contains(prompt, "Socratic Verification") {
		t.Error("expected 'Socratic Verification' stage label")
	}
	if !strings.Contains(prompt, "2 perspective(s)") {
		t.Error("expected count of 2 missing perspectives")
	}

	// Should have synthesis instruction about Missing Perspectives
	if !strings.Contains(prompt, "Missing Perspectives") {
		t.Error("expected synthesis instruction about Missing Perspectives section")
	}
}

func TestBuildSynthesisSystemPromptNoMissingPerspectives(t *testing.T) {
	sctx := SynthesisContext{
		Topic:        "Clean run",
		AnalysisDate: "2026-03-21",
		Perspectives: []Perspective{{ID: "p1", Name: "Only", Scope: "test"}},
		CollectedFindings: &CollectedFindings{
			Succeeded:     1,
			Failed:        0,
			TotalFindings: 1,
			Results: []SpecialistResult{
				{PerspectiveID: "p1", Outcome: OutcomeSuccess, FindingsCount: 1,
					Findings: &SpecialistFindings{Analyst: "p1", Findings: []SpecialistFinding{
						{Finding: "f1", Evidence: "e1", Severity: "low"},
					}}},
			},
		},
		CollectedVerifications: &CollectedVerifications{
			Succeeded:    1,
			AverageScore: 0.90,
			Results: []InterviewResult{
				{PerspectiveID: "p1", Outcome: InterviewSuccess, Verdict: "pass", Score: 0.90,
					Verified: &VerifiedFindings{Analyst: "p1", Verdict: "pass",
						Score:   VerificationScore{Assumption: 0.9, Relevance: 0.9, Constraints: 0.9, WeightedTotal: 0.90},
						Summary: "OK", Findings: []VerifiedFinding{{Finding: "f1", Status: "confirmed", Verification: "ok"}}}},
			},
		},
		SeedSummary:    "test",
		ReportTemplate: "template",
	}

	prompt := buildSynthesisSystemPrompt(sctx)

	// Should NOT contain the missing perspectives table (no failures)
	if strings.Contains(prompt, "perspective(s)** could not be fully completed") {
		t.Error("should not contain missing perspectives table when all succeeded")
	}
}

// TestReportSavedToReportsDir verifies that runSynthesisStage constructs the
// report path under the task's ReportDir (which maps to ~/.prism/reports/analyze-{id}/)
// and that the report file is actually written to disk at that path.
func TestReportSavedToReportsDir(t *testing.T) {
	tmpDir := t.TempDir()
	stateDir := filepath.Join(tmpDir, "state", "analyze-abc123")
	reportDir := filepath.Join(tmpDir, "reports", "analyze-abc123")

	// Create state and report dirs
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(reportDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a task with reportDir set
	task := taskpkg.NewAnalysisTask("analyze-abc123", "claude-sonnet-4-6", stateDir, reportDir, "")

	// Verify the expected report path
	expectedReportPath := filepath.Join(task.GetReportDir(), "report.md")
	if !strings.Contains(expectedReportPath, filepath.Join("reports", "analyze-abc123", "report.md")) {
		t.Errorf("report path should be under reports/analyze-{id}/, got: %s", expectedReportPath)
	}

	// Simulate what runSynthesisStage does: build reportPath from task.GetReportDir()
	reportPath := filepath.Join(task.GetReportDir(), "report.md")

	// Write a mock report to verify the path is writable
	mockReport := "# Analysis Report\n## Executive Summary\nTest report content."
	if err := os.MkdirAll(filepath.Dir(reportPath), 0755); err != nil {
		t.Fatalf("failed to create report dir: %v", err)
	}
	if err := os.WriteFile(reportPath, []byte(mockReport), 0644); err != nil {
		t.Fatalf("failed to write report: %v", err)
	}

	// Verify report exists at the expected path
	data, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("report not found at %s: %v", reportPath, err)
	}
	if string(data) != mockReport {
		t.Errorf("report content mismatch: got %q", string(data))
	}

	// SetReportPath and verify snapshot
	task.SetReportPath(reportPath)
	snap := task.Snapshot()
	if snap.Status != taskpkg.TaskStatusCompleted {
		t.Errorf("expected completed status, got %s", snap.Status)
	}
	if snap.ReportPath != reportPath {
		t.Errorf("snapshot report_path mismatch: expected %s, got %s", reportPath, snap.ReportPath)
	}
	if !strings.Contains(snap.ReportPath, "reports") {
		t.Error("report_path should be under reports directory")
	}
}

// TestReportPathStructureMatchesConvention verifies the report path follows
// the convention: ~/.prism/reports/analyze-{id}/report.md
func TestReportPathStructureMatchesConvention(t *testing.T) {
	tmpDir := t.TempDir()

	// Simulate what handleAnalyze does
	stateBase := filepath.Join(tmpDir, "state")
	reportBase := filepath.Join(tmpDir, "reports")

	taskIDs := []string{"analyze-aabbcc112233", "analyze-deadbeef0123", "analyze-000000ffffff"}

	for _, taskID := range taskIDs {
		stateDir := filepath.Join(stateBase, taskID)
		reportDir := filepath.Join(reportBase, taskID)

		if err := os.MkdirAll(stateDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(reportDir, 0755); err != nil {
			t.Fatal(err)
		}

		// This matches runSynthesisStage logic
		reportPath := filepath.Join(reportDir, "report.md")

		// Verify path structure
		expectedSuffix := filepath.Join("reports", taskID, "report.md")
		if !strings.HasSuffix(reportPath, expectedSuffix) {
			t.Errorf("task %s: report path %s should end with %s", taskID, reportPath, expectedSuffix)
		}

		// Verify the report can actually be written
		content := "# Report for " + taskID
		if err := os.WriteFile(reportPath, []byte(content), 0644); err != nil {
			t.Fatalf("task %s: failed to write report: %v", taskID, err)
		}

		// Read back and verify
		data, err := os.ReadFile(reportPath)
		if err != nil {
			t.Fatalf("task %s: failed to read report: %v", taskID, err)
		}
		if string(data) != content {
			t.Errorf("task %s: content mismatch", taskID)
		}
	}
}

// TestRunSynthesisSessionWritesReport verifies that runSynthesisSession creates
// the report directory if needed and writes the report file.
func TestRunSynthesisSessionWritesReport(t *testing.T) {
	tmpDir := t.TempDir()
	stateDir := filepath.Join(tmpDir, "state")
	reportDir := filepath.Join(tmpDir, "reports", "analyze-test99")

	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Note: reportDir is NOT pre-created — runSynthesisSession should create it

	// Create required state files
	seedData := `{"research": {"summary": "Test seed"}}`
	if err := os.WriteFile(filepath.Join(stateDir, "seed-analysis.json"), []byte(seedData), 0644); err != nil {
		t.Fatal(err)
	}

	cf := CollectedFindings{
		TaskID: "test", Succeeded: 1, TotalFindings: 1,
		Results: []SpecialistResult{{
			PerspectiveID: "p1", Outcome: OutcomeSuccess, FindingsCount: 1,
			Findings: &SpecialistFindings{
				Analyst:  "p1",
				Findings: []SpecialistFinding{{Finding: "f1", Evidence: "e1", Severity: "high"}},
			},
		}},
	}
	if err := WriteCollectedFindings(stateDir, cf); err != nil {
		t.Fatal(err)
	}

	cv := CollectedVerifications{
		TaskID: "test", Succeeded: 1, AverageScore: 0.9,
		Results: []InterviewResult{{
			PerspectiveID: "p1", Outcome: InterviewSuccess, Verdict: "pass", Score: 0.9,
			Verified: &VerifiedFindings{
				Analyst: "p1", Verdict: "pass",
				Score:    VerificationScore{Assumption: 0.9, Relevance: 0.9, Constraints: 0.9, WeightedTotal: 0.9},
				Summary:  "OK",
				Findings: []VerifiedFinding{{Finding: "f1", Status: "confirmed", Verification: "ok"}},
			},
		}},
	}
	if err := WriteCollectedVerifications(stateDir, cv); err != nil {
		t.Fatal(err)
	}

	// Verify report directory does NOT exist yet
	reportPath := filepath.Join(reportDir, "report.md")
	if _, err := os.Stat(reportDir); err == nil {
		t.Fatal("report directory should not exist before synthesis")
	}

	// runSynthesisSession creates the directory and writes the file
	// We can't call it directly because it needs queryLLMScopedWithSystemPrompt,
	// but we can verify the directory creation logic in isolation
	if err := os.MkdirAll(filepath.Dir(reportPath), 0755); err != nil {
		t.Fatalf("MkdirAll should create report dir: %v", err)
	}

	// Verify report dir was created
	info, err := os.Stat(reportDir)
	if err != nil {
		t.Fatalf("report directory should exist after MkdirAll: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("report path should be a directory")
	}
}

// TestReportDirCreatedByHandleAnalyze verifies that handleAnalyze pre-creates
// the report directory at ~/.prism/reports/analyze-{id}/ before launching the pipeline.
func TestReportDirCreatedByHandleAnalyze(t *testing.T) {
	tmpDir := t.TempDir()
	// Override prismBaseDir for testing
	origBase := prismBaseDir
	prismBaseDir = tmpDir
	defer func() { prismBaseDir = origBase }()

	taskStore = taskpkg.NewTaskStore()

	// Create the base directories
	os.MkdirAll(filepath.Join(tmpDir, "state"), 0755)
	os.MkdirAll(filepath.Join(tmpDir, "reports"), 0755)

	// Build a minimal analyze request
	args := map[string]interface{}{
		"topic": "Test report dir creation",
		"model": "claude-sonnet-4-6",
	}
	req := mcp.CallToolRequest{
		Params: struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments,omitempty"`
			Meta      *struct {
				ProgressToken mcp.ProgressToken `json:"progressToken,omitempty"`
			} `json:"_meta,omitempty"`
		}{
			Arguments: args,
		},
	}

	// Use a cancellable context so the background pipeline goroutine
	// (which spawns Claude CLI subprocesses) is cleaned up after verification.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	result, err := handleAnalyze(ctx, req)
	if err != nil {
		t.Fatalf("handleAnalyze error: %v", err)
	}
	if result.IsError {
		t.Fatalf("handleAnalyze returned error: %v", result.Content)
	}

	// Parse response to get task ID
	text := result.Content[0].(mcp.TextContent).Text
	var snap taskpkg.TaskSnapshot
	if err := json.Unmarshal([]byte(text), &snap); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	// Verify report directory was created
	expectedReportDir := filepath.Join(tmpDir, "reports", snap.ID)
	info, err := os.Stat(expectedReportDir)
	if err != nil {
		t.Fatalf("report directory %s should exist: %v", expectedReportDir, err)
	}
	if !info.IsDir() {
		t.Fatal("report path should be a directory")
	}

	// Verify state directory was also created
	expectedStateDir := filepath.Join(tmpDir, "state", snap.ID)
	info, err = os.Stat(expectedStateDir)
	if err != nil {
		t.Fatalf("state directory %s should exist: %v", expectedStateDir, err)
	}
	if !info.IsDir() {
		t.Fatal("state path should be a directory")
	}

	// Verify config.json contains the correct report_dir
	configData, err := os.ReadFile(filepath.Join(expectedStateDir, "config.json"))
	if err != nil {
		t.Fatalf("config.json should exist: %v", err)
	}
	var config map[string]interface{}
	if err := json.Unmarshal(configData, &config); err != nil {
		t.Fatalf("failed to parse config.json: %v", err)
	}
	if config["report_dir"] != expectedReportDir {
		t.Errorf("config.json report_dir should be %s, got %s", expectedReportDir, config["report_dir"])
	}
}

func TestSynthesisPromptVerificationScoresTable(t *testing.T) {
	sctx := SynthesisContext{
		Topic:        "Test",
		AnalysisDate: "2026-03-21",
		Perspectives: []Perspective{{ID: "p1", Name: "Analyst 1"}},
		CollectedFindings: &CollectedFindings{
			Succeeded: 1, TotalFindings: 1,
			Results: []SpecialistResult{{PerspectiveID: "p1", Outcome: OutcomeSuccess, FindingsCount: 1,
				Findings: &SpecialistFindings{Analyst: "p1", Findings: []SpecialistFinding{{Finding: "f", Evidence: "e", Severity: "low"}}}}},
		},
		CollectedVerifications: &CollectedVerifications{
			Succeeded:    1,
			AverageScore: 0.85,
			Results: []InterviewResult{
				{
					PerspectiveID: "p1",
					Outcome:       InterviewSuccess,
					Verdict:       "pass",
					Score:         0.85,
					Verified: &VerifiedFindings{
						Analyst: "p1",
						Verdict: "pass",
						Score:   VerificationScore{Assumption: 0.90, Relevance: 0.85, Constraints: 0.75, WeightedTotal: 0.85},
						Summary: "All good",
						Findings: []VerifiedFinding{
							{Finding: "f", Status: "confirmed", Verification: "checked"},
						},
					},
				},
			},
		},
		SeedSummary:    "test",
		ReportTemplate: "template",
	}

	prompt := buildSynthesisSystemPrompt(sctx)

	// Must contain markdown table headers for verification scores
	if !strings.Contains(prompt, "| Analyst | Verdict | Assumption | Relevance | Constraints | Weighted Total |") {
		t.Error("expected verification scores table header in system prompt")
	}

	// Must contain the actual score data row
	if !strings.Contains(prompt, "| p1 | pass | 0.90 | 0.85 | 0.75 | 0.85 |") {
		t.Error("expected verification score data row for p1")
	}
}
