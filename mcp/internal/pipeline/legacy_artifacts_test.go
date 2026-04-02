package pipeline

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWriteLegacyContextArtifact(t *testing.T) {
	stateDir := t.TempDir()

	seed := SeedAnalysis{
		Topic:   "Checkout flow analysis",
		Summary: "Discovered checkout, auth, and payment policy concerns.",
		Findings: []SeedFinding{
			{ID: 1, Area: "checkout", Description: "Checkout orchestrates payment confirmation.", Source: "checkout.go:12", ToolUsed: "Read"},
			{ID: 2, Area: "auth", Description: "Auth gates saved payment methods.", Source: "auth.go:44", ToolUsed: "Grep"},
		},
		KeyAreas: []string{"checkout", "auth"},
	}
	if err := WriteSeedAnalysis(SeedAnalysisPath(stateDir), seed); err != nil {
		t.Fatalf("write seed analysis: %v", err)
	}
	if err := writeDAHistory(stateDir, DAReviewHistory{FinalPassed: true, TotalRounds: 1}); err != nil {
		t.Fatalf("write da history: %v", err)
	}

	cfg := AnalysisConfig{Language: "ko"}
	if err := writeLegacyContextArtifact(stateDir, cfg); err != nil {
		t.Fatalf("writeLegacyContextArtifact: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(stateDir, "context.json"))
	if err != nil {
		t.Fatalf("read context.json: %v", err)
	}

	var ctx legacyContextArtifact
	if err := json.Unmarshal(data, &ctx); err != nil {
		t.Fatalf("parse context.json: %v", err)
	}

	if ctx.Summary != seed.Summary {
		t.Fatalf("Summary = %q, want %q", ctx.Summary, seed.Summary)
	}
	if ctx.ReportLanguage != "ko" {
		t.Fatalf("ReportLanguage = %q, want ko", ctx.ReportLanguage)
	}
	if ctx.InvestigationLoops != 1 {
		t.Fatalf("InvestigationLoops = %d, want 1", ctx.InvestigationLoops)
	}
	if len(ctx.ResearchSummary.KeyFindings) != 2 {
		t.Fatalf("expected 2 key findings, got %d", len(ctx.ResearchSummary.KeyFindings))
	}
	if !strings.Contains(ctx.ResearchSummary.KeyFindings[0], "checkout") {
		t.Fatalf("expected key finding to reference seed area, got %q", ctx.ResearchSummary.KeyFindings[0])
	}
}

func TestWriteLegacyVerificationArtifactsProducesDownstreamContracts(t *testing.T) {
	stateDir := t.TempDir()
	perspectives := []Perspective{
		{ID: "security", Name: "Security Review"},
		{ID: "ux", Name: "UX Impact"},
	}

	collected := CollectedVerifications{
		TaskID:      "analyze-abc123",
		CollectedAt: time.Now().UTC(),
		Results: []InterviewResult{
			{
				PerspectiveID: "security",
				Outcome:       InterviewSuccess,
				Verified: &VerifiedFindings{
					Analyst: "security",
					Topic:   "Checkout flow analysis",
					Verdict: "pass",
					Score: VerificationScore{
						Assumption:    0.9,
						Relevance:     0.8,
						Constraints:   0.7,
						WeightedTotal: 0.82,
					},
					Findings: []VerifiedFinding{
						{
							Finding:      "Auth bypass on cached checkout token",
							Evidence:     "checkout.go:88 - token reused without user revalidation",
							Severity:     "CRITICAL",
							Status:       "confirmed",
							Verification: "Confirmed by tracing token reuse path.",
						},
					},
					Summary: "One critical auth flaw was confirmed.",
				},
				FindingsCount: 1,
				Verdict:       "pass",
				Score:         0.82,
			},
			{
				PerspectiveID: "ux",
				Outcome:       InterviewParseError,
				ErrorMessage:  "parse error",
			},
		},
	}

	if err := writeLegacyVerificationArtifacts(stateDir, perspectives, collected); err != nil {
		t.Fatalf("writeLegacyVerificationArtifacts: %v", err)
	}

	interviewPath := filepath.Join(stateDir, "perspectives", "security", "interview.json")
	interviewData, err := os.ReadFile(interviewPath)
	if err != nil {
		t.Fatalf("read interview.json: %v", err)
	}

	var interview legacyInterviewArtifact
	if err := json.Unmarshal(interviewData, &interview); err != nil {
		t.Fatalf("parse interview.json: %v", err)
	}
	if interview.PerspectiveID != "security" {
		t.Fatalf("PerspectiveID = %q, want security", interview.PerspectiveID)
	}
	if interview.WeightedTotal != 0.82 {
		t.Fatalf("WeightedTotal = %v, want 0.82", interview.WeightedTotal)
	}

	verificationLogData, err := os.ReadFile(filepath.Join(stateDir, "verification-log.json"))
	if err != nil {
		t.Fatalf("read verification-log.json: %v", err)
	}
	var verificationLog legacyVerificationLog
	if err := json.Unmarshal(verificationLogData, &verificationLog); err != nil {
		t.Fatalf("parse verification-log.json: %v", err)
	}
	if len(verificationLog.Perspectives) != 1 {
		t.Fatalf("expected 1 verification-log entry, got %d", len(verificationLog.Perspectives))
	}
	if verificationLog.Perspectives[0].Verdict != "pass" {
		t.Fatalf("verification-log verdict = %q, want pass", verificationLog.Perspectives[0].Verdict)
	}

	compiledData, err := os.ReadFile(filepath.Join(stateDir, "analyst-findings.md"))
	if err != nil {
		t.Fatalf("read analyst-findings.md: %v", err)
	}
	compiled := string(compiledData)
	for _, needle := range []string{
		"## Verification Scores Summary",
		"| Perspective | Weighted Total | Verdict | Findings |",
		"Security Review",
		"Auth bypass on cached checkout token",
	} {
		if !strings.Contains(compiled, needle) {
			t.Fatalf("expected %q in analyst-findings.md\n%s", needle, compiled)
		}
	}

	perPerspectiveData, err := os.ReadFile(filepath.Join(stateDir, "verified-findings-security.md"))
	if err != nil {
		t.Fatalf("read verified-findings-security.md: %v", err)
	}
	if !strings.Contains(string(perPerspectiveData), "Weighted Total: 0.82") {
		t.Fatalf("expected weighted total in verified-findings-security.md\n%s", string(perPerspectiveData))
	}
}

func TestWriteLegacyStateReportCopy(t *testing.T) {
	stateDir := t.TempDir()
	reportDir := t.TempDir()
	reportPath := filepath.Join(reportDir, "report.md")
	reportBody := "# Report\n\nLegacy-compatible report copy."

	if err := os.WriteFile(reportPath, []byte(reportBody), 0644); err != nil {
		t.Fatalf("write report: %v", err)
	}

	if err := writeLegacyStateReportCopy(stateDir, reportPath); err != nil {
		t.Fatalf("writeLegacyStateReportCopy: %v", err)
	}

	stateReport, err := os.ReadFile(filepath.Join(stateDir, "report.md"))
	if err != nil {
		t.Fatalf("read state report: %v", err)
	}
	if string(stateReport) != reportBody {
		t.Fatalf("state report copy mismatch: got %q want %q", string(stateReport), reportBody)
	}
}
