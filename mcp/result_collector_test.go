package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/heechul/prism-mcp/internal/parallel"
)

func TestCollectSpecialistResults_AllSuccess(t *testing.T) {
	// Set up temp directory with findings files
	tmpDir := t.TempDir()

	perspectives := []Perspective{
		{ID: "security-analysis"},
		{ID: "performance-analysis"},
		{ID: "reliability-analysis"},
	}

	// Write findings files
	for _, p := range perspectives {
		dir := filepath.Join(tmpDir, "perspectives", p.ID)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		findings := SpecialistFindings{
			Analyst: p.ID,
			Input:   "test topic",
			Findings: []SpecialistFinding{
				{Finding: "finding-1 from " + p.ID, Evidence: "file.go:10", Severity: "high"},
				{Finding: "finding-2 from " + p.ID, Evidence: "file.go:20", Severity: "medium"},
			},
		}
		if err := WriteSpecialistFindings(FindingsPath(tmpDir, p.ID), findings); err != nil {
			t.Fatal(err)
		}
	}

	stageResults := make([]StageResult, len(perspectives))
	for i, p := range perspectives {
		stageResults[i] = StageResult{
			PerspectiveID: p.ID,
			OutputPath:    FindingsPath(tmpDir, p.ID),
		}
	}

	collected := CollectSpecialistResults("analyze-test123", stageResults, perspectives)

	if collected.TotalSpecialists != 3 {
		t.Errorf("TotalSpecialists = %d, want 3", collected.TotalSpecialists)
	}
	if collected.Succeeded != 3 {
		t.Errorf("Succeeded = %d, want 3", collected.Succeeded)
	}
	if collected.Failed != 0 {
		t.Errorf("Failed = %d, want 0", collected.Failed)
	}
	if collected.TotalFindings != 6 {
		t.Errorf("TotalFindings = %d, want 6", collected.TotalFindings)
	}
	if collected.PartialFailure {
		t.Error("PartialFailure should be false when all succeed")
	}
	if len(collected.AllFindings) != 6 {
		t.Errorf("AllFindings length = %d, want 6", len(collected.AllFindings))
	}
	if len(collected.FailedSpecialists) != 0 {
		t.Errorf("FailedSpecialists length = %d, want 0", len(collected.FailedSpecialists))
	}

	// Verify annotation
	for _, af := range collected.AllFindings {
		if af.PerspectiveID == "" {
			t.Error("AnnotatedFinding missing PerspectiveID")
		}
	}

	// Verify SuccessfulPerspectiveIDs
	ids := collected.SuccessfulPerspectiveIDs()
	if len(ids) != 3 {
		t.Errorf("SuccessfulPerspectiveIDs length = %d, want 3", len(ids))
	}
}

func TestCollectSpecialistResults_PartialFailure(t *testing.T) {
	tmpDir := t.TempDir()

	perspectives := []Perspective{
		{ID: "security-analysis"},
		{ID: "performance-analysis"},
		{ID: "reliability-analysis"},
	}

	// Only write findings for first perspective
	dir := filepath.Join(tmpDir, "perspectives", "security-analysis")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	findings := SpecialistFindings{
		Analyst: "security-analysis",
		Input:   "test topic",
		Findings: []SpecialistFinding{
			{Finding: "security issue", Evidence: "auth.go:42", Severity: "critical"},
		},
	}
	if err := WriteSpecialistFindings(FindingsPath(tmpDir, "security-analysis"), findings); err != nil {
		t.Fatal(err)
	}

	stageResults := []StageResult{
		{PerspectiveID: "security-analysis", OutputPath: FindingsPath(tmpDir, "security-analysis")},
		{PerspectiveID: "performance-analysis", Err: fmt.Errorf("context deadline exceeded")},
		{PerspectiveID: "reliability-analysis", Err: fmt.Errorf("exit status 1: killed")},
	}

	collected := CollectSpecialistResults("analyze-test456", stageResults, perspectives)

	if collected.Succeeded != 1 {
		t.Errorf("Succeeded = %d, want 1", collected.Succeeded)
	}
	if collected.Failed != 2 {
		t.Errorf("Failed = %d, want 2", collected.Failed)
	}
	if !collected.PartialFailure {
		t.Error("PartialFailure should be true with partial failure")
	}
	if collected.TotalFindings != 1 {
		t.Errorf("TotalFindings = %d, want 1", collected.TotalFindings)
	}
	if len(collected.FailedSpecialists) != 2 {
		t.Errorf("FailedSpecialists length = %d, want 2", len(collected.FailedSpecialists))
	}

	// Verify error classification
	for _, fs := range collected.FailedSpecialists {
		switch fs.PerspectiveID {
		case "performance-analysis":
			if fs.Outcome != OutcomeTimeout {
				t.Errorf("performance-analysis outcome = %s, want %s", fs.Outcome, OutcomeTimeout)
			}
		case "reliability-analysis":
			if fs.Outcome != OutcomeCrashed {
				t.Errorf("reliability-analysis outcome = %s, want %s", fs.Outcome, OutcomeCrashed)
			}
		}
	}

	// Verify degradation notice
	notice := collected.DegradationNotice()
	if notice == "" {
		t.Error("DegradationNotice should not be empty for degraded results")
	}

	// Verify only successful IDs returned
	ids := collected.SuccessfulPerspectiveIDs()
	if len(ids) != 1 || ids[0] != "security-analysis" {
		t.Errorf("SuccessfulPerspectiveIDs = %v, want [security-analysis]", ids)
	}
}

func TestCollectSpecialistResults_AllFailed(t *testing.T) {
	perspectives := []Perspective{
		{ID: "p1"},
		{ID: "p2"},
	}

	stageResults := []StageResult{
		{PerspectiveID: "p1", Err: fmt.Errorf("context deadline exceeded")},
		{PerspectiveID: "p2", Err: fmt.Errorf("signal: killed")},
	}

	collected := CollectSpecialistResults("analyze-allfail", stageResults, perspectives)

	if collected.Succeeded != 0 {
		t.Errorf("Succeeded = %d, want 0", collected.Succeeded)
	}
	if collected.Failed != 2 {
		t.Errorf("Failed = %d, want 2", collected.Failed)
	}
	// PartialFailure is false when ALL fail (nothing to degrade to)
	if collected.PartialFailure {
		t.Error("PartialFailure should be false when all fail (no partial results)")
	}
	if len(collected.AllFindings) != 0 {
		t.Errorf("AllFindings should be empty, got %d", len(collected.AllFindings))
	}
}

func TestCollectSpecialistResults_EmptyFindings(t *testing.T) {
	tmpDir := t.TempDir()

	dir := filepath.Join(tmpDir, "perspectives", "empty-analyst")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write findings with empty findings array
	findings := SpecialistFindings{
		Analyst:  "empty-analyst",
		Input:    "test",
		Findings: []SpecialistFinding{},
	}
	if err := WriteSpecialistFindings(FindingsPath(tmpDir, "empty-analyst"), findings); err != nil {
		t.Fatal(err)
	}

	stageResults := []StageResult{
		{PerspectiveID: "empty-analyst", OutputPath: FindingsPath(tmpDir, "empty-analyst")},
	}

	collected := CollectSpecialistResults("analyze-empty", stageResults, []Perspective{{ID: "empty-analyst"}})

	if collected.Succeeded != 0 {
		t.Errorf("Succeeded = %d, want 0 (empty findings treated as failure)", collected.Succeeded)
	}
	if collected.Failed != 1 {
		t.Errorf("Failed = %d, want 1", collected.Failed)
	}

	// Should be classified as empty_output
	if len(collected.Results) != 1 {
		t.Fatal("expected 1 result")
	}
	if collected.Results[0].Outcome != OutcomeEmptyOutput {
		t.Errorf("Outcome = %s, want %s", collected.Results[0].Outcome, OutcomeEmptyOutput)
	}
}

func TestClassifyError(t *testing.T) {
	tests := []struct {
		err      string
		outcome  SpecialistOutcome
		class    string
	}{
		{"context deadline exceeded", OutcomeTimeout, "subprocess_timeout"},
		{"operation timed out waiting for response", OutcomeTimeout, "subprocess_timeout"},
		{"context canceled before start: context canceled", OutcomeCancelled, "context_cancelled"},
		{"exit status 1", OutcomeCrashed, "subprocess_crash"},
		{"signal: killed", OutcomeCrashed, "subprocess_crash"},
		{"broken pipe", OutcomeCrashed, "subprocess_crash"},
		{"json: cannot unmarshal string", OutcomeParseError, "output_parse_error"},
		{"unexpected end of JSON input", OutcomeParseError, "output_parse_error"},
		{"invalid character 'x'", OutcomeParseError, "output_parse_error"},
		{"some unknown error", OutcomeCrashed, "unknown_error"},
	}

	for _, tt := range tests {
		outcome, class := classifyError(fmt.Errorf("%s", tt.err))
		if outcome != tt.outcome {
			t.Errorf("classifyError(%q) outcome = %s, want %s", tt.err, outcome, tt.outcome)
		}
		if class != tt.class {
			t.Errorf("classifyError(%q) class = %s, want %s", tt.err, class, tt.class)
		}
	}
}

func TestWriteReadCollectedFindings(t *testing.T) {
	tmpDir := t.TempDir()

	cf := CollectedFindings{
		TaskID:           "analyze-roundtrip",
		TotalSpecialists: 2,
		Succeeded:        1,
		Failed:           1,
		TotalFindings:    3,
		PartialFailure:         true,
		Results: []SpecialistResult{
			{
				PerspectiveID: "p1",
				Outcome:       OutcomeSuccess,
				FindingsCount: 3,
				Findings: &SpecialistFindings{
					Analyst: "p1",
					Input:   "test",
					Findings: []SpecialistFinding{
						{Finding: "f1", Evidence: "e1", Severity: "high"},
						{Finding: "f2", Evidence: "e2", Severity: "medium"},
						{Finding: "f3", Evidence: "e3", Severity: "low"},
					},
				},
			},
			{
				PerspectiveID: "p2",
				Outcome:       OutcomeTimeout,
				ErrorMessage:  "context deadline exceeded",
				ErrorClass:    "subprocess_timeout",
			},
		},
		AllFindings: []AnnotatedFinding{
			{PerspectiveID: "p1", Finding: "f1", Evidence: "e1", Severity: "high"},
			{PerspectiveID: "p1", Finding: "f2", Evidence: "e2", Severity: "medium"},
			{PerspectiveID: "p1", Finding: "f3", Evidence: "e3", Severity: "low"},
		},
		FailedSpecialists: []FailedSpecialist{
			{PerspectiveID: "p2", Outcome: OutcomeTimeout, ErrorMessage: "context deadline exceeded"},
		},
	}

	if err := WriteCollectedFindings(tmpDir, cf); err != nil {
		t.Fatal(err)
	}

	// Verify file exists
	path := CollectedFindingsPath(tmpDir)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("collected-findings.json not created: %v", err)
	}

	// Read back
	readCF, err := ReadCollectedFindings(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	if readCF.TaskID != cf.TaskID {
		t.Errorf("TaskID = %s, want %s", readCF.TaskID, cf.TaskID)
	}
	if readCF.Succeeded != cf.Succeeded {
		t.Errorf("Succeeded = %d, want %d", readCF.Succeeded, cf.Succeeded)
	}
	if readCF.Failed != cf.Failed {
		t.Errorf("Failed = %d, want %d", readCF.Failed, cf.Failed)
	}
	if readCF.TotalFindings != cf.TotalFindings {
		t.Errorf("TotalFindings = %d, want %d", readCF.TotalFindings, cf.TotalFindings)
	}
	if readCF.PartialFailure != cf.PartialFailure {
		t.Errorf("PartialFailure = %v, want %v", readCF.PartialFailure, cf.PartialFailure)
	}
	if len(readCF.AllFindings) != len(cf.AllFindings) {
		t.Errorf("AllFindings length = %d, want %d", len(readCF.AllFindings), len(cf.AllFindings))
	}
	if len(readCF.FailedSpecialists) != len(cf.FailedSpecialists) {
		t.Errorf("FailedSpecialists length = %d, want %d", len(readCF.FailedSpecialists), len(cf.FailedSpecialists))
	}
}

func TestDegradationNotice(t *testing.T) {
	// No degradation
	cf := CollectedFindings{PartialFailure: false, Failed: 0}
	if notice := cf.DegradationNotice(); notice != "" {
		t.Error("DegradationNotice should be empty when not degraded")
	}

	// PartialFailure
	cf = CollectedFindings{
		TotalSpecialists: 3,
		Succeeded:        1,
		Failed:           2,
		PartialFailure:         true,
		FailedSpecialists: []FailedSpecialist{
			{PerspectiveID: "perf", Outcome: OutcomeTimeout, ErrorMessage: "timeout"},
			{PerspectiveID: "sec", Outcome: OutcomeCrashed, ErrorMessage: "crash"},
		},
	}
	notice := cf.DegradationNotice()
	if notice == "" {
		t.Fatal("DegradationNotice should not be empty for degraded results")
	}
	if !strings.Contains(notice, "2 of 3") {
		t.Error("notice should mention count")
	}
	if !strings.Contains(notice, "perf") {
		t.Error("notice should mention failed perspective 'perf'")
	}
	if !strings.Contains(notice, "sec") {
		t.Error("notice should mention failed perspective 'sec'")
	}
}

func TestCollectSpecialistResults_MissingOutputPath(t *testing.T) {
	stageResults := []StageResult{
		{PerspectiveID: "no-path", OutputPath: ""},
	}

	collected := CollectSpecialistResults("analyze-nopath", stageResults, []Perspective{{ID: "no-path"}})

	if collected.Succeeded != 0 {
		t.Errorf("Succeeded = %d, want 0", collected.Succeeded)
	}
	if collected.Results[0].Outcome != OutcomeEmptyOutput {
		t.Errorf("Outcome = %s, want %s", collected.Results[0].Outcome, OutcomeEmptyOutput)
	}
}

func TestCollectSpecialistResults_MissingFile(t *testing.T) {
	stageResults := []StageResult{
		{PerspectiveID: "missing-file", OutputPath: "/nonexistent/path/findings.json"},
	}

	collected := CollectSpecialistResults("analyze-missing", stageResults, []Perspective{{ID: "missing-file"}})

	if collected.Succeeded != 0 {
		t.Errorf("Succeeded = %d, want 0", collected.Succeeded)
	}
	if collected.Results[0].Outcome != OutcomeParseError {
		t.Errorf("Outcome = %s, want %s", collected.Results[0].Outcome, OutcomeParseError)
	}
}

func TestCollectSpecialistResults_PerspectiveIDFallback(t *testing.T) {
	// StageResult with empty PerspectiveID should fall back to perspectives list
	tmpDir := t.TempDir()

	dir := filepath.Join(tmpDir, "perspectives", "fallback-id")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	findings := SpecialistFindings{
		Analyst:  "fallback-id",
		Input:    "test",
		Findings: []SpecialistFinding{{Finding: "f1", Evidence: "e1", Severity: "low"}},
	}
	if err := WriteSpecialistFindings(FindingsPath(tmpDir, "fallback-id"), findings); err != nil {
		t.Fatal(err)
	}

	stageResults := []StageResult{
		{PerspectiveID: "", OutputPath: FindingsPath(tmpDir, "fallback-id")},
	}

	collected := CollectSpecialistResults("analyze-fb", stageResults, []Perspective{{ID: "fallback-id"}})

	if collected.Results[0].PerspectiveID != "fallback-id" {
		t.Errorf("PerspectiveID = %q, want %q", collected.Results[0].PerspectiveID, "fallback-id")
	}
}

// TestCollectSpecialistResults_JSONSerializable verifies the collected findings
// can be serialized to JSON without error (important for file persistence).
func TestCollectSpecialistResults_JSONSerializable(t *testing.T) {
	collected := CollectedFindings{
		TaskID:           "analyze-serial",
		TotalSpecialists: 1,
		Succeeded:        1,
		TotalFindings:    1,
		Results: []SpecialistResult{
			{
				PerspectiveID: "p1",
				Outcome:       OutcomeSuccess,
				FindingsCount: 1,
				Findings: &SpecialistFindings{
					Analyst:  "p1",
					Input:    "test",
					Findings: []SpecialistFinding{{Finding: "f", Evidence: "e", Severity: "s"}},
				},
			},
		},
		AllFindings: []AnnotatedFinding{
			{PerspectiveID: "p1", Finding: "f", Evidence: "e", Severity: "s"},
		},
	}

	data, err := json.Marshal(collected)
	if err != nil {
		t.Fatalf("JSON marshal failed: %v", err)
	}

	var roundTrip CollectedFindings
	if err := json.Unmarshal(data, &roundTrip); err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}

	if roundTrip.TaskID != collected.TaskID {
		t.Error("roundtrip TaskID mismatch")
	}
}

// ============================================================
// Interview Result Collector Tests
// ============================================================

func TestCollectInterviewResults_AllSuccess(t *testing.T) {
	tmpDir := t.TempDir()

	perspectives := []Perspective{
		{ID: "security-analysis"},
		{ID: "performance-analysis"},
	}

	// Write verified findings files
	for _, p := range perspectives {
		dir := filepath.Join(tmpDir, "perspectives", p.ID)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		verified := VerifiedFindings{
			Analyst: p.ID,
			Topic:   "test topic",
			Verdict: "pass",
			Score: VerificationScore{
				Assumption:    0.9,
				Relevance:     0.85,
				Constraints:   0.8,
				WeightedTotal: 0.86,
			},
			Findings: []VerifiedFinding{
				{Finding: "finding-1 from " + p.ID, Evidence: "file.go:10", Severity: "high", Status: "confirmed", Verification: "verified via code inspection"},
				{Finding: "finding-2 from " + p.ID, Evidence: "file.go:20", Severity: "medium", Status: "revised", Verification: "revised after investigation"},
			},
			Summary: "all findings verified",
		}
		path := filepath.Join(dir, "verified-findings.json")
		if err := WriteVerifiedFindings(path, verified); err != nil {
			t.Fatal(err)
		}
	}

	stageResults := make([]StageResult, len(perspectives))
	for i, p := range perspectives {
		stageResults[i] = StageResult{
			PerspectiveID: p.ID,
			OutputPath:    filepath.Join(tmpDir, "perspectives", p.ID, "verified-findings.json"),
		}
	}

	collected := CollectInterviewResults("analyze-test123", stageResults, perspectives)

	if collected.TotalInterviews != 2 {
		t.Errorf("TotalInterviews = %d, want 2", collected.TotalInterviews)
	}
	if collected.Succeeded != 2 {
		t.Errorf("Succeeded = %d, want 2", collected.Succeeded)
	}
	if collected.Failed != 0 {
		t.Errorf("Failed = %d, want 0", collected.Failed)
	}
	if collected.PartialFailure {
		t.Error("PartialFailure should be false when all succeed")
	}
	if len(collected.VerifiedFindings) != 4 {
		t.Errorf("VerifiedFindings length = %d, want 4", len(collected.VerifiedFindings))
	}
	if len(collected.FailedInterviews) != 0 {
		t.Errorf("FailedInterviews length = %d, want 0", len(collected.FailedInterviews))
	}
	// Average score should be 0.86
	if collected.AverageScore < 0.85 || collected.AverageScore > 0.87 {
		t.Errorf("AverageScore = %f, want ~0.86", collected.AverageScore)
	}

	// Verify annotations
	for _, avf := range collected.VerifiedFindings {
		if avf.PerspectiveID == "" {
			t.Error("AnnotatedVerifiedFinding missing PerspectiveID")
		}
		if avf.Status == "" {
			t.Error("AnnotatedVerifiedFinding missing Status")
		}
	}

	// Verify SuccessfulInterviewIDs
	ids := collected.SuccessfulInterviewIDs()
	if len(ids) != 2 {
		t.Errorf("SuccessfulInterviewIDs length = %d, want 2", len(ids))
	}
}

func TestCollectInterviewResults_PartialFailure(t *testing.T) {
	tmpDir := t.TempDir()

	perspectives := []Perspective{
		{ID: "security-analysis"},
		{ID: "performance-analysis"},
		{ID: "reliability-analysis"},
	}

	// Only write verified findings for first perspective
	dir := filepath.Join(tmpDir, "perspectives", "security-analysis")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	verified := VerifiedFindings{
		Analyst: "security-analysis",
		Topic:   "test topic",
		Verdict: "pass_with_caveats",
		Score: VerificationScore{
			Assumption:    0.75,
			Relevance:     0.8,
			Constraints:   0.7,
			WeightedTotal: 0.76,
		},
		Findings: []VerifiedFinding{
			{Finding: "security issue", Evidence: "auth.go:42", Severity: "critical", Status: "confirmed", Verification: "verified"},
		},
		Summary: "verified with caveats",
	}
	path := filepath.Join(dir, "verified-findings.json")
	if err := WriteVerifiedFindings(path, verified); err != nil {
		t.Fatal(err)
	}

	stageResults := []StageResult{
		{PerspectiveID: "security-analysis", OutputPath: path},
		{PerspectiveID: "performance-analysis", Err: fmt.Errorf("context deadline exceeded")},
		{PerspectiveID: "reliability-analysis", Err: fmt.Errorf("exit status 1: killed")},
	}

	collected := CollectInterviewResults("analyze-test456", stageResults, perspectives)

	if collected.Succeeded != 1 {
		t.Errorf("Succeeded = %d, want 1", collected.Succeeded)
	}
	if collected.Failed != 2 {
		t.Errorf("Failed = %d, want 2", collected.Failed)
	}
	if !collected.PartialFailure {
		t.Error("PartialFailure should be true with partial failure")
	}
	if len(collected.FailedInterviews) != 2 {
		t.Errorf("FailedInterviews length = %d, want 2", len(collected.FailedInterviews))
	}

	// Verify error classification
	for _, fi := range collected.FailedInterviews {
		switch fi.PerspectiveID {
		case "performance-analysis":
			if fi.Outcome != InterviewTimeout {
				t.Errorf("performance-analysis outcome = %s, want %s", fi.Outcome, InterviewTimeout)
			}
		case "reliability-analysis":
			if fi.Outcome != InterviewCrashed {
				t.Errorf("reliability-analysis outcome = %s, want %s", fi.Outcome, InterviewCrashed)
			}
		}
	}

	// Verify degradation notice
	notice := collected.InterviewDegradationNotice()
	if notice == "" {
		t.Error("InterviewDegradationNotice should not be empty for degraded results")
	}
	if !strings.Contains(notice, "2 of 3") {
		t.Error("notice should mention count")
	}
	if !strings.Contains(notice, "unverified") {
		t.Error("notice should mention unverified findings")
	}

	// Verify only successful IDs returned
	ids := collected.SuccessfulInterviewIDs()
	if len(ids) != 1 || ids[0] != "security-analysis" {
		t.Errorf("SuccessfulInterviewIDs = %v, want [security-analysis]", ids)
	}

	// Average score should be from the single successful interview
	if collected.AverageScore < 0.75 || collected.AverageScore > 0.77 {
		t.Errorf("AverageScore = %f, want ~0.76", collected.AverageScore)
	}
}

func TestCollectInterviewResults_AllFailed(t *testing.T) {
	perspectives := []Perspective{
		{ID: "p1"},
		{ID: "p2"},
	}

	stageResults := []StageResult{
		{PerspectiveID: "p1", Err: fmt.Errorf("context deadline exceeded")},
		{PerspectiveID: "p2", Err: fmt.Errorf("signal: killed")},
	}

	collected := CollectInterviewResults("analyze-allfail", stageResults, perspectives)

	if collected.Succeeded != 0 {
		t.Errorf("Succeeded = %d, want 0", collected.Succeeded)
	}
	if collected.Failed != 2 {
		t.Errorf("Failed = %d, want 2", collected.Failed)
	}
	// PartialFailure is false when ALL fail (nothing to degrade to)
	if collected.PartialFailure {
		t.Error("PartialFailure should be false when all fail (no partial results)")
	}
	if len(collected.VerifiedFindings) != 0 {
		t.Errorf("VerifiedFindings should be empty, got %d", len(collected.VerifiedFindings))
	}
	if collected.AverageScore != 0 {
		t.Errorf("AverageScore should be 0 when all fail, got %f", collected.AverageScore)
	}
}

func TestCollectInterviewResults_EmptyFindings(t *testing.T) {
	tmpDir := t.TempDir()

	dir := filepath.Join(tmpDir, "perspectives", "empty-verifier")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write verified findings with empty findings array
	verified := VerifiedFindings{
		Analyst:  "empty-verifier",
		Topic:    "test",
		Verdict:  "fail",
		Score:    VerificationScore{WeightedTotal: 0.3},
		Findings: []VerifiedFinding{},
		Summary:  "no findings to verify",
	}
	path := filepath.Join(dir, "verified-findings.json")
	if err := WriteVerifiedFindings(path, verified); err != nil {
		t.Fatal(err)
	}

	stageResults := []StageResult{
		{PerspectiveID: "empty-verifier", OutputPath: path},
	}

	collected := CollectInterviewResults("analyze-empty", stageResults, []Perspective{{ID: "empty-verifier"}})

	if collected.Succeeded != 0 {
		t.Errorf("Succeeded = %d, want 0 (empty findings treated as failure)", collected.Succeeded)
	}
	if collected.Failed != 1 {
		t.Errorf("Failed = %d, want 1", collected.Failed)
	}
	if collected.Results[0].Outcome != InterviewEmptyOutput {
		t.Errorf("Outcome = %s, want %s", collected.Results[0].Outcome, InterviewEmptyOutput)
	}
}

func TestCollectInterviewResults_MissingOutputPath(t *testing.T) {
	stageResults := []StageResult{
		{PerspectiveID: "no-path", OutputPath: ""},
	}

	collected := CollectInterviewResults("analyze-nopath", stageResults, []Perspective{{ID: "no-path"}})

	if collected.Succeeded != 0 {
		t.Errorf("Succeeded = %d, want 0", collected.Succeeded)
	}
	if collected.Results[0].Outcome != InterviewEmptyOutput {
		t.Errorf("Outcome = %s, want %s", collected.Results[0].Outcome, InterviewEmptyOutput)
	}
}

func TestCollectInterviewResults_MissingFile(t *testing.T) {
	stageResults := []StageResult{
		{PerspectiveID: "missing-file", OutputPath: "/nonexistent/path/verified-findings.json"},
	}

	collected := CollectInterviewResults("analyze-missing", stageResults, []Perspective{{ID: "missing-file"}})

	if collected.Succeeded != 0 {
		t.Errorf("Succeeded = %d, want 0", collected.Succeeded)
	}
	if collected.Results[0].Outcome != InterviewParseError {
		t.Errorf("Outcome = %s, want %s", collected.Results[0].Outcome, InterviewParseError)
	}
}

func TestCollectInterviewResults_PerspectiveIDFallback(t *testing.T) {
	tmpDir := t.TempDir()

	dir := filepath.Join(tmpDir, "perspectives", "fallback-id")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	verified := VerifiedFindings{
		Analyst:  "fallback-id",
		Topic:    "test",
		Verdict:  "pass",
		Score:    VerificationScore{WeightedTotal: 0.9},
		Findings: []VerifiedFinding{{Finding: "f1", Evidence: "e1", Severity: "low", Status: "confirmed", Verification: "ok"}},
		Summary:  "ok",
	}
	path := filepath.Join(dir, "verified-findings.json")
	if err := WriteVerifiedFindings(path, verified); err != nil {
		t.Fatal(err)
	}

	stageResults := []StageResult{
		{PerspectiveID: "", OutputPath: path},
	}

	collected := CollectInterviewResults("analyze-fb", stageResults, []Perspective{{ID: "fallback-id"}})

	if collected.Results[0].PerspectiveID != "fallback-id" {
		t.Errorf("PerspectiveID = %q, want %q", collected.Results[0].PerspectiveID, "fallback-id")
	}
}

func TestWriteReadCollectedVerifications(t *testing.T) {
	tmpDir := t.TempDir()

	cv := CollectedVerifications{
		TaskID:          "analyze-roundtrip",
		TotalInterviews: 2,
		Succeeded:       1,
		Failed:          1,
		PartialFailure:        true,
		AverageScore:    0.86,
		Results: []InterviewResult{
			{
				PerspectiveID: "p1",
				Outcome:       InterviewSuccess,
				FindingsCount: 2,
				Verdict:       "pass",
				Score:         0.86,
				Verified: &VerifiedFindings{
					Analyst: "p1",
					Topic:   "test",
					Verdict: "pass",
					Score:   VerificationScore{WeightedTotal: 0.86},
					Findings: []VerifiedFinding{
						{Finding: "f1", Evidence: "e1", Severity: "high", Status: "confirmed", Verification: "ok"},
						{Finding: "f2", Evidence: "e2", Severity: "medium", Status: "revised", Verification: "revised"},
					},
					Summary: "ok",
				},
			},
			{
				PerspectiveID: "p2",
				Outcome:       InterviewTimeout,
				ErrorMessage:  "context deadline exceeded",
				ErrorClass:    "subprocess_timeout",
			},
		},
		VerifiedFindings: []AnnotatedVerifiedFinding{
			{PerspectiveID: "p1", Finding: "f1", Evidence: "e1", Severity: "high", Status: "confirmed", Verification: "ok"},
			{PerspectiveID: "p1", Finding: "f2", Evidence: "e2", Severity: "medium", Status: "revised", Verification: "revised"},
		},
		FailedInterviews: []FailedInterview{
			{PerspectiveID: "p2", Outcome: InterviewTimeout, ErrorMessage: "context deadline exceeded"},
		},
	}

	if err := WriteCollectedVerifications(tmpDir, cv); err != nil {
		t.Fatal(err)
	}

	// Verify file exists
	path := CollectedVerificationsPath(tmpDir)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("collected-verifications.json not created: %v", err)
	}

	// Read back
	readCV, err := ReadCollectedVerifications(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	if readCV.TaskID != cv.TaskID {
		t.Errorf("TaskID = %s, want %s", readCV.TaskID, cv.TaskID)
	}
	if readCV.Succeeded != cv.Succeeded {
		t.Errorf("Succeeded = %d, want %d", readCV.Succeeded, cv.Succeeded)
	}
	if readCV.Failed != cv.Failed {
		t.Errorf("Failed = %d, want %d", readCV.Failed, cv.Failed)
	}
	if readCV.PartialFailure != cv.PartialFailure {
		t.Errorf("PartialFailure = %v, want %v", readCV.PartialFailure, cv.PartialFailure)
	}
	if readCV.AverageScore != cv.AverageScore {
		t.Errorf("AverageScore = %f, want %f", readCV.AverageScore, cv.AverageScore)
	}
	if len(readCV.VerifiedFindings) != len(cv.VerifiedFindings) {
		t.Errorf("VerifiedFindings length = %d, want %d", len(readCV.VerifiedFindings), len(cv.VerifiedFindings))
	}
	if len(readCV.FailedInterviews) != len(cv.FailedInterviews) {
		t.Errorf("FailedInterviews length = %d, want %d", len(readCV.FailedInterviews), len(cv.FailedInterviews))
	}
}

func TestInterviewDegradationNotice(t *testing.T) {
	// No degradation
	cv := CollectedVerifications{PartialFailure: false, Failed: 0}
	if notice := cv.InterviewDegradationNotice(); notice != "" {
		t.Error("InterviewDegradationNotice should be empty when not degraded")
	}

	// PartialFailure
	cv = CollectedVerifications{
		TotalInterviews: 3,
		Succeeded:       1,
		Failed:          2,
		PartialFailure:        true,
		FailedInterviews: []FailedInterview{
			{PerspectiveID: "perf", Outcome: InterviewTimeout, ErrorMessage: "timeout"},
			{PerspectiveID: "sec", Outcome: InterviewCrashed, ErrorMessage: "crash"},
		},
	}
	notice := cv.InterviewDegradationNotice()
	if notice == "" {
		t.Fatal("InterviewDegradationNotice should not be empty for degraded results")
	}
	if !strings.Contains(notice, "2 of 3") {
		t.Error("notice should mention count")
	}
	if !strings.Contains(notice, "perf") {
		t.Error("notice should mention failed perspective 'perf'")
	}
	if !strings.Contains(notice, "sec") {
		t.Error("notice should mention failed perspective 'sec'")
	}
	if !strings.Contains(notice, "unverified") {
		t.Error("notice should mention unverified findings")
	}
}

func TestConfirmedFindings(t *testing.T) {
	cv := CollectedVerifications{
		VerifiedFindings: []AnnotatedVerifiedFinding{
			{PerspectiveID: "p1", Finding: "f1", Status: "confirmed"},
			{PerspectiveID: "p1", Finding: "f2", Status: "revised"},
			{PerspectiveID: "p2", Finding: "f3", Status: "confirmed"},
			{PerspectiveID: "p2", Finding: "f4", Status: "withdrawn"},
		},
	}

	confirmed := cv.ConfirmedFindings()
	if len(confirmed) != 2 {
		t.Errorf("ConfirmedFindings length = %d, want 2", len(confirmed))
	}
	for _, f := range confirmed {
		if f.Status != "confirmed" {
			t.Errorf("Expected status 'confirmed', got %q", f.Status)
		}
	}
}

func TestClassifyInterviewError(t *testing.T) {
	tests := []struct {
		err     string
		outcome InterviewOutcome
		class   string
	}{
		{"context deadline exceeded", InterviewTimeout, "subprocess_timeout"},
		{"operation timed out waiting for response", InterviewTimeout, "subprocess_timeout"},
		{"context canceled before start: context canceled", InterviewCancelled, "context_cancelled"},
		{"exit status 1", InterviewCrashed, "subprocess_crash"},
		{"signal: killed", InterviewCrashed, "subprocess_crash"},
		{"broken pipe", InterviewCrashed, "subprocess_crash"},
		{"json: cannot unmarshal string", InterviewParseError, "output_parse_error"},
		{"unexpected end of JSON input", InterviewParseError, "output_parse_error"},
		{"invalid character 'x'", InterviewParseError, "output_parse_error"},
		{"some unknown error", InterviewCrashed, "unknown_error"},
	}

	for _, tt := range tests {
		outcome, class := classifyInterviewError(fmt.Errorf("%s", tt.err))
		if outcome != tt.outcome {
			t.Errorf("classifyInterviewError(%q) outcome = %s, want %s", tt.err, outcome, tt.outcome)
		}
		if class != tt.class {
			t.Errorf("classifyInterviewError(%q) class = %s, want %s", tt.err, class, tt.class)
		}
	}
}

func TestCollectInterviewResults_JSONSerializable(t *testing.T) {
	collected := CollectedVerifications{
		TaskID:          "analyze-serial",
		TotalInterviews: 1,
		Succeeded:       1,
		AverageScore:    0.9,
		Results: []InterviewResult{
			{
				PerspectiveID: "p1",
				Outcome:       InterviewSuccess,
				FindingsCount: 1,
				Verdict:       "pass",
				Score:         0.9,
				Verified: &VerifiedFindings{
					Analyst: "p1",
					Topic:   "test",
					Verdict: "pass",
					Score:   VerificationScore{WeightedTotal: 0.9},
					Findings: []VerifiedFinding{
						{Finding: "f", Evidence: "e", Severity: "s", Status: "confirmed", Verification: "v"},
					},
					Summary: "ok",
				},
			},
		},
		VerifiedFindings: []AnnotatedVerifiedFinding{
			{PerspectiveID: "p1", Finding: "f", Evidence: "e", Severity: "s", Status: "confirmed", Verification: "v"},
		},
	}

	data, err := json.Marshal(collected)
	if err != nil {
		t.Fatalf("JSON marshal failed: %v", err)
	}

	var roundTrip CollectedVerifications
	if err := json.Unmarshal(data, &roundTrip); err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}

	if roundTrip.TaskID != collected.TaskID {
		t.Error("roundtrip TaskID mismatch")
	}
	if roundTrip.AverageScore != collected.AverageScore {
		t.Error("roundtrip AverageScore mismatch")
	}
}

func TestCollectInterviewResults_VerdictAndScoreExtracted(t *testing.T) {
	tmpDir := t.TempDir()

	dir := filepath.Join(tmpDir, "perspectives", "test-persp")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	verified := VerifiedFindings{
		Analyst: "test-persp",
		Topic:   "test topic",
		Verdict: "pass_with_caveats",
		Score: VerificationScore{
			Assumption:    0.7,
			Relevance:     0.8,
			Constraints:   0.6,
			WeightedTotal: 0.72,
		},
		Findings: []VerifiedFinding{
			{Finding: "f1", Evidence: "e1", Severity: "high", Status: "confirmed", Verification: "ok"},
		},
		Summary: "caveats noted",
	}
	path := filepath.Join(dir, "verified-findings.json")
	if err := WriteVerifiedFindings(path, verified); err != nil {
		t.Fatal(err)
	}

	stageResults := []StageResult{
		{PerspectiveID: "test-persp", OutputPath: path},
	}

	collected := CollectInterviewResults("analyze-verdict", stageResults, []Perspective{{ID: "test-persp"}})

	if len(collected.Results) != 1 {
		t.Fatal("expected 1 result")
	}
	r := collected.Results[0]
	if r.Verdict != "pass_with_caveats" {
		t.Errorf("Verdict = %q, want %q", r.Verdict, "pass_with_caveats")
	}
	if r.Score < 0.71 || r.Score > 0.73 {
		t.Errorf("Score = %f, want ~0.72", r.Score)
	}
}

// === Skip Logic Tests (Sub-AC 2 of AC 10) ===

// TestSkipLogic_SpecialistRetryExhausted verifies that a specialist whose
// retry is exhausted is marked as Skipped=true and recorded in SkippedPerspectives.
func TestSkipLogic_SpecialistRetryExhausted(t *testing.T) {
	tmpDir := t.TempDir()

	// Write findings for the successful specialist
	dir := filepath.Join(tmpDir, "perspectives", "security")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	findings := SpecialistFindings{
		Analyst: "security",
		Input:   "test",
		Findings: []SpecialistFinding{
			{Finding: "SQL injection", Evidence: "api.go:42", Severity: "critical"},
		},
	}
	if err := WriteSpecialistFindings(FindingsPath(tmpDir, "security"), findings); err != nil {
		t.Fatal(err)
	}

	// Simulate: security succeeds, performance fails with retry exhaustion
	stageResults := []StageResult{
		{PerspectiveID: "security", OutputPath: FindingsPath(tmpDir, "security")},
		{PerspectiveID: "performance", Err: fmt.Errorf("all attempts failed for performance (tried 2 times): context deadline exceeded")},
	}
	perspectives := []Perspective{
		{ID: "security"},
		{ID: "performance"},
	}

	collected := CollectSpecialistResults("analyze-skip1", stageResults, perspectives)

	// Basic counts
	if collected.Succeeded != 1 {
		t.Errorf("Succeeded = %d, want 1", collected.Succeeded)
	}
	if collected.Failed != 1 {
		t.Errorf("Failed = %d, want 1", collected.Failed)
	}
	if !collected.PartialFailure {
		t.Error("PartialFailure should be true")
	}

	// Verify the failed specialist is classified as retry_exhausted
	if len(collected.FailedSpecialists) != 1 {
		t.Fatalf("FailedSpecialists length = %d, want 1", len(collected.FailedSpecialists))
	}
	if collected.FailedSpecialists[0].Outcome != OutcomeRetryFailed {
		t.Errorf("FailedSpecialists[0].Outcome = %s, want %s",
			collected.FailedSpecialists[0].Outcome, OutcomeRetryFailed)
	}

	// Verify SkippedPerspectives is populated
	if len(collected.SkippedPerspectives) != 1 {
		t.Fatalf("SkippedPerspectives length = %d, want 1", len(collected.SkippedPerspectives))
	}
	sp := collected.SkippedPerspectives[0]
	if sp.PerspectiveID != "performance" {
		t.Errorf("SkippedPerspectives[0].PerspectiveID = %q, want %q", sp.PerspectiveID, "performance")
	}
	if sp.Reason == "" {
		t.Error("SkippedPerspectives[0].Reason should not be empty")
	}
	if sp.ErrorMessage == "" {
		t.Error("SkippedPerspectives[0].ErrorMessage should not be empty")
	}
	if !strings.Contains(sp.ErrorMessage, "all attempts failed") {
		t.Errorf("ErrorMessage should mention retry exhaustion, got: %s", sp.ErrorMessage)
	}

	// Verify the SpecialistResult has Skipped=true
	var perfResult *SpecialistResult
	for i := range collected.Results {
		if collected.Results[i].PerspectiveID == "performance" {
			perfResult = &collected.Results[i]
			break
		}
	}
	if perfResult == nil {
		t.Fatal("performance result not found")
	}
	if !perfResult.Skipped {
		t.Error("performance SpecialistResult.Skipped should be true")
	}

	// Security result should NOT be skipped
	for _, r := range collected.Results {
		if r.PerspectiveID == "security" && r.Skipped {
			t.Error("security SpecialistResult.Skipped should be false")
		}
	}
}

// TestSkipLogic_NoSkippedWhenNonRetryFailure verifies that failures other than
// retry exhaustion are NOT marked as skipped.
func TestSkipLogic_NoSkippedWhenNonRetryFailure(t *testing.T) {
	stageResults := []StageResult{
		{PerspectiveID: "timeout-spec", Err: fmt.Errorf("context deadline exceeded")},
		{PerspectiveID: "crash-spec", Err: fmt.Errorf("exit status 1: signal: killed")},
	}
	perspectives := []Perspective{
		{ID: "timeout-spec"},
		{ID: "crash-spec"},
	}

	collected := CollectSpecialistResults("analyze-noskip", stageResults, perspectives)

	if len(collected.SkippedPerspectives) != 0 {
		t.Errorf("SkippedPerspectives length = %d, want 0 (non-retry failures)", len(collected.SkippedPerspectives))
	}

	for _, r := range collected.Results {
		if r.Skipped {
			t.Errorf("Result %s should not be Skipped (non-retry failure)", r.PerspectiveID)
		}
	}
}

// TestSkipLogic_InterviewRetryExhausted verifies that an interview whose
// retry is exhausted is marked as Skipped=true and recorded in SkippedInterviews.
func TestSkipLogic_InterviewRetryExhausted(t *testing.T) {
	tmpDir := t.TempDir()

	// Write verified findings for the successful interview
	dir := filepath.Join(tmpDir, "perspectives", "sec-verifier")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	verified := VerifiedFindings{
		Analyst: "sec-verifier",
		Topic:   "test",
		Verdict: "pass",
		Score:   VerificationScore{WeightedTotal: 0.85},
		Findings: []VerifiedFinding{
			{Finding: "SQL injection", Evidence: "api.go:42", Severity: "critical", Status: "confirmed", Verification: "verified via code review"},
		},
		Summary: "findings confirmed",
	}
	path := filepath.Join(dir, "verified-findings.json")
	if err := WriteVerifiedFindings(path, verified); err != nil {
		t.Fatal(err)
	}

	stageResults := []StageResult{
		{PerspectiveID: "sec-verifier", OutputPath: path},
		{PerspectiveID: "perf-verifier", Err: fmt.Errorf("all attempts failed for perf-verifier (tried 2 times): broken pipe")},
	}
	perspectives := []Perspective{
		{ID: "sec-verifier"},
		{ID: "perf-verifier"},
	}

	collected := CollectInterviewResults("analyze-intskip", stageResults, perspectives)

	if collected.Succeeded != 1 {
		t.Errorf("Succeeded = %d, want 1", collected.Succeeded)
	}
	if collected.Failed != 1 {
		t.Errorf("Failed = %d, want 1", collected.Failed)
	}
	if !collected.PartialFailure {
		t.Error("PartialFailure should be true")
	}

	// Verify SkippedInterviews
	if len(collected.SkippedInterviews) != 1 {
		t.Fatalf("SkippedInterviews length = %d, want 1", len(collected.SkippedInterviews))
	}
	si := collected.SkippedInterviews[0]
	if si.PerspectiveID != "perf-verifier" {
		t.Errorf("SkippedInterviews[0].PerspectiveID = %q, want %q", si.PerspectiveID, "perf-verifier")
	}
	if si.Reason == "" {
		t.Error("SkippedInterviews[0].Reason should not be empty")
	}
	if !strings.Contains(si.ErrorMessage, "all attempts failed") {
		t.Errorf("ErrorMessage should mention retry exhaustion, got: %s", si.ErrorMessage)
	}

	// Verify the InterviewResult has Skipped=true
	var perfResult *InterviewResult
	for i := range collected.Results {
		if collected.Results[i].PerspectiveID == "perf-verifier" {
			perfResult = &collected.Results[i]
			break
		}
	}
	if perfResult == nil {
		t.Fatal("perf-verifier result not found")
	}
	if !perfResult.Skipped {
		t.Error("perf-verifier InterviewResult.Skipped should be true")
	}
}

// TestSkipLogic_DegradationNoticeContainsSkippedLabel verifies that the
// degradation notice includes [SKIPPED] label for retry-exhausted specialists.
func TestSkipLogic_DegradationNoticeContainsSkippedLabel(t *testing.T) {
	cf := CollectedFindings{
		TotalSpecialists: 3,
		Succeeded:        1,
		Failed:           2,
		PartialFailure:         true,
		FailedSpecialists: []FailedSpecialist{
			{PerspectiveID: "perf", Outcome: OutcomeRetryFailed, ErrorMessage: "retry failed"},
			{PerspectiveID: "sec", Outcome: OutcomeTimeout, ErrorMessage: "timeout"},
		},
		SkippedPerspectives: []SkippedPerspective{
			{PerspectiveID: "perf", Reason: "analysis failed after retry", ErrorMessage: "retry failed"},
		},
	}

	notice := cf.DegradationNotice()
	if !strings.Contains(notice, "[SKIPPED]") {
		t.Error("DegradationNotice should contain [SKIPPED] for retry-exhausted specialist")
	}
	// The timeout failure should NOT have [SKIPPED]
	lines := strings.Split(notice, "\n")
	for _, line := range lines {
		if strings.Contains(line, "sec") && strings.Contains(line, "[SKIPPED]") {
			t.Error("sec (timeout) should NOT have [SKIPPED] label")
		}
	}
}

// TestSkipLogic_InterviewDegradationNoticeContainsSkippedLabel verifies [SKIPPED]
// in interview degradation notices for retry-exhausted interviews.
func TestSkipLogic_InterviewDegradationNoticeContainsSkippedLabel(t *testing.T) {
	cv := CollectedVerifications{
		TotalInterviews: 2,
		Succeeded:       1,
		Failed:          1,
		PartialFailure:        true,
		FailedInterviews: []FailedInterview{
			{PerspectiveID: "perf", Outcome: InterviewRetryFailed, ErrorMessage: "retry failed"},
		},
		SkippedInterviews: []SkippedPerspective{
			{PerspectiveID: "perf", Reason: "verification failed after retry", ErrorMessage: "retry failed"},
		},
	}

	notice := cv.InterviewDegradationNotice()
	if !strings.Contains(notice, "[SKIPPED]") {
		t.Error("InterviewDegradationNotice should contain [SKIPPED] for retry-exhausted interview")
	}
}

// TestSkipLogic_MultipleSkippedPerspectives verifies correct behavior when
// multiple specialists are skipped after retry exhaustion.
func TestSkipLogic_MultipleSkippedPerspectives(t *testing.T) {
	tmpDir := t.TempDir()

	// One success
	dir := filepath.Join(tmpDir, "perspectives", "arch")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	findings := SpecialistFindings{
		Analyst:  "arch",
		Input:    "test",
		Findings: []SpecialistFinding{{Finding: "f1", Evidence: "e1", Severity: "medium"}},
	}
	if err := WriteSpecialistFindings(FindingsPath(tmpDir, "arch"), findings); err != nil {
		t.Fatal(err)
	}

	stageResults := []StageResult{
		{PerspectiveID: "arch", OutputPath: FindingsPath(tmpDir, "arch")},
		{PerspectiveID: "perf", Err: fmt.Errorf("all attempts failed for perf (tried 2 times): timeout")},
		{PerspectiveID: "sec", Err: fmt.Errorf("all attempts failed for sec (tried 2 times): crash")},
		{PerspectiveID: "data", Err: fmt.Errorf("context deadline exceeded")}, // non-retry failure
	}
	perspectives := []Perspective{{ID: "arch"}, {ID: "perf"}, {ID: "sec"}, {ID: "data"}}

	collected := CollectSpecialistResults("analyze-multi", stageResults, perspectives)

	if collected.Succeeded != 1 {
		t.Errorf("Succeeded = %d, want 1", collected.Succeeded)
	}
	if collected.Failed != 3 {
		t.Errorf("Failed = %d, want 3", collected.Failed)
	}

	// 2 should be skipped (retry-exhausted), 1 should not (timeout without retry wrapper)
	if len(collected.SkippedPerspectives) != 2 {
		t.Fatalf("SkippedPerspectives length = %d, want 2", len(collected.SkippedPerspectives))
	}

	skippedIDs := make(map[string]bool)
	for _, sp := range collected.SkippedPerspectives {
		skippedIDs[sp.PerspectiveID] = true
	}
	if !skippedIDs["perf"] {
		t.Error("perf should be in SkippedPerspectives")
	}
	if !skippedIDs["sec"] {
		t.Error("sec should be in SkippedPerspectives")
	}
	if skippedIDs["data"] {
		t.Error("data should NOT be in SkippedPerspectives (non-retry failure)")
	}
}

// TestSkipLogic_JSONRoundTrip verifies that SkippedPerspectives and Skipped
// fields survive JSON serialization/deserialization.
func TestSkipLogic_JSONRoundTrip(t *testing.T) {
	cf := CollectedFindings{
		TaskID:           "analyze-rt",
		TotalSpecialists: 2,
		Succeeded:        1,
		Failed:           1,
		PartialFailure:         true,
		Results: []SpecialistResult{
			{PerspectiveID: "ok", Outcome: OutcomeSuccess, FindingsCount: 1},
			{PerspectiveID: "fail", Outcome: OutcomeRetryFailed, Skipped: true, ErrorMessage: "retry exhausted"},
		},
		FailedSpecialists: []FailedSpecialist{
			{PerspectiveID: "fail", Outcome: OutcomeRetryFailed, ErrorMessage: "retry exhausted"},
		},
		SkippedPerspectives: []SkippedPerspective{
			{PerspectiveID: "fail", Reason: "analysis failed after retry", ErrorMessage: "retry exhausted"},
		},
	}

	data, err := json.Marshal(cf)
	if err != nil {
		t.Fatalf("JSON marshal failed: %v", err)
	}

	var roundTrip CollectedFindings
	if err := json.Unmarshal(data, &roundTrip); err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}

	if len(roundTrip.SkippedPerspectives) != 1 {
		t.Fatalf("SkippedPerspectives length = %d, want 1", len(roundTrip.SkippedPerspectives))
	}
	if roundTrip.SkippedPerspectives[0].PerspectiveID != "fail" {
		t.Error("SkippedPerspectives[0].PerspectiveID mismatch")
	}
	if !roundTrip.Results[1].Skipped {
		t.Error("Results[1].Skipped should be true after round-trip")
	}
	if roundTrip.Results[0].Skipped {
		t.Error("Results[0].Skipped should be false after round-trip")
	}
}

// TestSkipLogic_ParallelExecutorWrapsRetryError verifies that ParallelExecutor
// wraps the error with "all attempts failed" when retry is exhausted.
func TestSkipLogic_ParallelExecutorWrapsRetryError(t *testing.T) {
	pe := &parallel.ParallelExecutor{
		Concurrency: 1,
		RetryLimit:  2,
	}

	jobs := []parallel.ParallelJob{
		{
			PerspectiveID: "wrap-test",
			Fn: func(ctx context.Context) (string, error) {
				return "", fmt.Errorf("original error")
			},
		},
	}

	pr := pe.Execute(context.Background(), jobs)

	if pr.Failed != 1 {
		t.Fatalf("Failed = %d, want 1", pr.Failed)
	}

	errMsg := pr.Results[0].Err.Error()
	if !strings.Contains(errMsg, "all attempts failed") {
		t.Errorf("error should contain 'all attempts failed', got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "wrap-test") {
		t.Errorf("error should contain perspective ID 'wrap-test', got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "original error") {
		t.Errorf("error should preserve original error, got: %s", errMsg)
	}

	// Verify classifyError detects it as retry_exhausted
	outcome, class := classifyError(pr.Results[0].Err)
	if outcome != OutcomeRetryFailed {
		t.Errorf("classifyError outcome = %s, want %s", outcome, OutcomeRetryFailed)
	}
	if class != "retry_exhausted" {
		t.Errorf("classifyError class = %s, want retry_exhausted", class)
	}
}

// Suppress unused import warning for context
var _ = context.Background

func TestMissingPerspectivesReport_NoFailures(t *testing.T) {
	cf := &CollectedFindings{
		TotalSpecialists: 3,
		Succeeded:        3,
		Failed:           0,
	}
	cv := &CollectedVerifications{
		TotalInterviews: 3,
		Succeeded:       3,
		Failed:          0,
	}

	report := MissingPerspectivesReport(cf, cv)
	if report != "" {
		t.Errorf("expected empty report for no failures, got: %s", report)
	}
}

func TestMissingPerspectivesReport_FailedSpecialists(t *testing.T) {
	cf := &CollectedFindings{
		TotalSpecialists: 3,
		Succeeded:        1,
		Failed:           2,
		PartialFailure:         true,
		FailedSpecialists: []FailedSpecialist{
			{PerspectiveID: "security", Outcome: OutcomeTimeout, ErrorMessage: "deadline exceeded"},
			{PerspectiveID: "performance", Outcome: OutcomeRetryFailed, ErrorMessage: "all attempts failed"},
		},
	}
	cv := &CollectedVerifications{
		TotalInterviews: 1,
		Succeeded:       1,
		Failed:          0,
	}

	report := MissingPerspectivesReport(cf, cv)

	if !strings.Contains(report, "Missing Perspectives") {
		t.Error("report should contain 'Missing Perspectives' header")
	}
	if !strings.Contains(report, "2 perspective(s)") {
		t.Error("report should mention 2 missing perspectives")
	}
	if !strings.Contains(report, "security") {
		t.Error("report should list security perspective")
	}
	if !strings.Contains(report, "performance") {
		t.Error("report should list performance perspective")
	}
	if !strings.Contains(report, "Specialist Analysis") {
		t.Error("report should indicate Specialist Analysis stage")
	}
	if !strings.Contains(report, "entirely absent") {
		t.Error("report should describe impact as entirely absent")
	}
}

func TestMissingPerspectivesReport_FailedInterviews(t *testing.T) {
	cf := &CollectedFindings{
		TotalSpecialists: 2,
		Succeeded:        2,
		Failed:           0,
	}
	cv := &CollectedVerifications{
		TotalInterviews: 2,
		Succeeded:       1,
		Failed:          1,
		PartialFailure:        true,
		FailedInterviews: []FailedInterview{
			{PerspectiveID: "ux-analyst", Outcome: InterviewCrashed, ErrorMessage: "exit status 1"},
		},
	}

	report := MissingPerspectivesReport(cf, cv)

	if !strings.Contains(report, "1 perspective(s)") {
		t.Error("report should mention 1 missing perspective")
	}
	if !strings.Contains(report, "ux-analyst") {
		t.Error("report should list ux-analyst perspective")
	}
	if !strings.Contains(report, "Socratic Verification") {
		t.Error("report should indicate Socratic Verification stage")
	}
	if !strings.Contains(report, "unverified") {
		t.Error("report should describe findings as unverified")
	}
}

func TestMissingPerspectivesReport_MixedFailures(t *testing.T) {
	cf := &CollectedFindings{
		TotalSpecialists: 3,
		Succeeded:        2,
		Failed:           1,
		PartialFailure:         true,
		FailedSpecialists: []FailedSpecialist{
			{PerspectiveID: "security", Outcome: OutcomeCrashed, ErrorMessage: "killed"},
		},
	}
	cv := &CollectedVerifications{
		TotalInterviews: 2,
		Succeeded:       1,
		Failed:          1,
		PartialFailure:        true,
		FailedInterviews: []FailedInterview{
			{PerspectiveID: "performance", Outcome: InterviewTimeout, ErrorMessage: "deadline exceeded"},
		},
	}

	report := MissingPerspectivesReport(cf, cv)

	if !strings.Contains(report, "2 perspective(s)") {
		t.Error("report should mention 2 missing perspectives")
	}
	// Security failed at specialist stage
	if !strings.Contains(report, "security") {
		t.Error("report should list security (specialist failure)")
	}
	// Performance failed at interview stage
	if !strings.Contains(report, "performance") {
		t.Error("report should list performance (interview failure)")
	}
}

func TestMissingPerspectivesReport_DeduplicatesSpecialistAndInterview(t *testing.T) {
	// If a perspective failed at specialist stage AND has a failed interview entry,
	// it should only appear once (at the specialist stage).
	cf := &CollectedFindings{
		TotalSpecialists: 2,
		Succeeded:        1,
		Failed:           1,
		PartialFailure:         true,
		FailedSpecialists: []FailedSpecialist{
			{PerspectiveID: "security", Outcome: OutcomeCrashed, ErrorMessage: "killed"},
		},
	}
	cv := &CollectedVerifications{
		TotalInterviews: 1,
		Succeeded:       0,
		Failed:          1,
		FailedInterviews: []FailedInterview{
			// This interview also "failed" but the specialist already failed
			{PerspectiveID: "security", Outcome: InterviewCrashed, ErrorMessage: "no findings to verify"},
		},
	}

	report := MissingPerspectivesReport(cf, cv)

	// Should only list 1 entry (not duplicated)
	if !strings.Contains(report, "1 perspective(s)") {
		t.Errorf("expected 1 missing perspective (deduplicated), got report: %s", report)
	}
	// Should only have the specialist stage entry
	if strings.Count(report, "security") != 1 {
		t.Errorf("security should appear exactly once, got report: %s", report)
	}
}

func TestMissingPerspectivesReport_NilInputs(t *testing.T) {
	// Both nil
	report := MissingPerspectivesReport(nil, nil)
	if report != "" {
		t.Error("expected empty report for nil inputs")
	}

	// Only findings nil
	cv := &CollectedVerifications{
		Failed:           1,
		FailedInterviews: []FailedInterview{{PerspectiveID: "p1", Outcome: InterviewTimeout}},
	}
	report = MissingPerspectivesReport(nil, cv)
	if !strings.Contains(report, "p1") {
		t.Error("should report p1 even when findings is nil")
	}
}

func TestHasMissingPerspectives(t *testing.T) {
	tests := []struct {
		name string
		cf   *CollectedFindings
		cv   *CollectedVerifications
		want bool
	}{
		{
			name: "no failures",
			cf:   &CollectedFindings{},
			cv:   &CollectedVerifications{},
			want: false,
		},
		{
			name: "failed specialists",
			cf:   &CollectedFindings{FailedSpecialists: []FailedSpecialist{{PerspectiveID: "p1"}}},
			cv:   &CollectedVerifications{},
			want: true,
		},
		{
			name: "failed interviews only",
			cf:   &CollectedFindings{},
			cv:   &CollectedVerifications{FailedInterviews: []FailedInterview{{PerspectiveID: "p1"}}},
			want: true,
		},
		{
			name: "interview failure deduplicated with specialist",
			cf:   &CollectedFindings{FailedSpecialists: []FailedSpecialist{{PerspectiveID: "p1"}}},
			cv:   &CollectedVerifications{FailedInterviews: []FailedInterview{{PerspectiveID: "p1"}}},
			want: true, // still true because of the specialist failure
		},
		{
			name: "nil inputs",
			cf:   nil,
			cv:   nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HasMissingPerspectives(tt.cf, tt.cv)
			if got != tt.want {
				t.Errorf("HasMissingPerspectives() = %v, want %v", got, tt.want)
			}
		})
	}
}
