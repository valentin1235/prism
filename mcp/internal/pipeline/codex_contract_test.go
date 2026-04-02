package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	taskpkg "github.com/heechul/prism-mcp/internal/task"
)

func TestRunSpecialistSession_CodexInvocationContractAndArtifacts(t *testing.T) {
	stateDir := t.TempDir()
	reportDir := t.TempDir()
	workDir := PerspectiveDir(stateDir, "security-analysis")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("mkdir workdir: %v", err)
	}

	argsPath := filepath.Join(t.TempDir(), "args.txt")
	stdinPath := filepath.Join(t.TempDir(), "stdin.txt")
	schemaCapturePath := filepath.Join(t.TempDir(), "schema.json")
	fakeCodexPath := writeFixtureCodexStub(t, codexFixtureStubOptions{
		fixturePath:       contractFixtureAbsPath(t, "specialist-findings.json"),
		argsCapturePath:   argsPath,
		stdinCapturePath:  stdinPath,
		schemaCapturePath: schemaCapturePath,
	})
	t.Setenv("PRISM_CODEX_CLI_PATH", fakeCodexPath)

	task := newContractTestTask(t, stateDir, reportDir)
	cmd := SpecialistCommand{
		PerspectiveID: "security-analysis",
		SystemPrompt:  "Follow the shared specialist workflow exactly.",
		UserPrompt:    "Investigate checkout reliability and emit structured findings.",
		Model:         "claude-sonnet-4-6",
		WorkDir:       workDir,
		OutputPath:    FindingsPath(stateDir, "security-analysis"),
		MaxTurns:      10,
		JSONSchema:    SpecialistFindingsSchema(),
	}

	if err := RunSpecialistSession(context.Background(), task, cmd); err != nil {
		t.Fatalf("RunSpecialistSession() error = %v", err)
	}

	assertCodexArgsContain(t, argsPath,
		"exec",
		"--json",
		"--skip-git-repo-check",
		"-C",
		workDir,
		"--output-last-message",
		"--output-schema",
		"--dangerously-bypass-approvals-and-sandbox",
	)
	assertCodexArgsNotContain(t, argsPath, "--model claude-sonnet-4-6")
	assertFileContainsAll(t, stdinPath,
		"## System Instructions\nFollow the shared specialist workflow exactly.",
		"## Tooling Guidance",
		"Honor this routing contract as if it were CLI-enforced.",
		"- Glob: discover candidate files and directories by path pattern before reading them",
		"- Grep: search repository text for identifiers, symbols, or error strings before deeper inspection",
		"- Read: inspect specific files once Glob or Grep has identified relevant targets",
		"- Bash: run tightly scoped terminal commands when file tools are insufficient",
		"Do NOT use these tools or capability routes:",
		"- MCP",
		"- ToolSearch",
		"- WebFetch",
		"- Browser",
		"- Write",
		"- Edit",
		"Investigate checkout reliability and emit structured findings.",
	)
	assertJSONFileMatchesFixture(t, cmd.OutputPath, contractFixturePath("specialist-findings.json"))
	assertFileEquals(t, schemaCapturePath, SpecialistFindingsSchema())
}

func TestRunInterviewSession_CodexInvocationContractAndArtifacts(t *testing.T) {
	stateDir := t.TempDir()
	reportDir := t.TempDir()
	workDir := PerspectiveDir(stateDir, "security-analysis")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("mkdir workdir: %v", err)
	}

	argsPath := filepath.Join(t.TempDir(), "args.txt")
	stdinPath := filepath.Join(t.TempDir(), "stdin.txt")
	schemaCapturePath := filepath.Join(t.TempDir(), "schema.json")
	fakeCodexPath := writeFixtureCodexStub(t, codexFixtureStubOptions{
		fixturePath:       contractFixtureAbsPath(t, "verified-findings.json"),
		argsCapturePath:   argsPath,
		stdinCapturePath:  stdinPath,
		schemaCapturePath: schemaCapturePath,
	})
	t.Setenv("PRISM_CODEX_CLI_PATH", fakeCodexPath)

	task := newContractTestTask(t, stateDir, reportDir)
	cmd := InterviewCommand{
		PerspectiveID: "security-analysis",
		SystemPrompt:  "Follow the shared verification workflow exactly.",
		UserPrompt:    "Verify the checkout reliability findings and emit structured JSON.",
		Model:         "claude-sonnet-4-6",
		WorkDir:       workDir,
		OutputPath:    filepath.Join(workDir, "verified-findings.json"),
		MaxTurns:      10,
		JSONSchema:    VerifiedFindingsSchema(),
	}

	if err := runInterviewSession(context.Background(), task, cmd); err != nil {
		t.Fatalf("runInterviewSession() error = %v", err)
	}

	assertCodexArgsContain(t, argsPath,
		"exec",
		"--json",
		"--skip-git-repo-check",
		"-C",
		workDir,
		"--output-last-message",
		"--output-schema",
		"--dangerously-bypass-approvals-and-sandbox",
	)
	assertCodexArgsNotContain(t, argsPath, "--model claude-sonnet-4-6")
	assertFileContainsAll(t, stdinPath,
		"## System Instructions\nFollow the shared verification workflow exactly.",
		"## Tooling Guidance",
		"Honor this routing contract as if it were CLI-enforced.",
		"- Glob: discover candidate files and directories by path pattern before reading them",
		"- Grep: search repository text for identifiers, symbols, or error strings before deeper inspection",
		"- Read: inspect specific files once Glob or Grep has identified relevant targets",
		"- Bash: run tightly scoped terminal commands when file tools are insufficient",
		"Do NOT use these tools or capability routes:",
		"- MCP",
		"- ToolSearch",
		"- WebFetch",
		"- Browser",
		"- Write",
		"- Edit",
		"Verify the checkout reliability findings and emit structured JSON.",
	)
	assertJSONFileMatchesFixture(t, cmd.OutputPath, contractFixturePath("verified-findings.json"))
	assertFileEquals(t, schemaCapturePath, VerifiedFindingsSchema())
}

func TestRunSynthesisSession_CodexInvocationContractAndArtifacts(t *testing.T) {
	stateDir := t.TempDir()
	reportDir := filepath.Join(t.TempDir(), "reports")
	reportPath := filepath.Join(reportDir, "report.md")

	if err := os.WriteFile(filepath.Join(stateDir, "seed-analysis.json"), []byte(`{"topic":"Analyze checkout reliability","summary":"Checkout retries and reconciliation both affect reliability.","findings":[],"key_areas":["checkout","reconciliation"]}`), 0o644); err != nil {
		t.Fatalf("write seed analysis: %v", err)
	}

	perspectives := []Perspective{
		{
			ID:           "security-analysis",
			Name:         "Security Analysis",
			Scope:        "Checkout settlement idempotency and replay safety",
			KeyQuestions: []string{"Can retries duplicate settlement?", "Can reconciliation replay completed work?"},
			Rationale:    "Settlement reliability issues create both operational and security impact.",
			Prompt: AnalystPrompt{
				Role:               "You are the SECURITY ANALYST.",
				InvestigationScope: "Checkout reliability",
				Tasks:              "1. Trace retries\n2. Verify replay behavior",
				OutputFormat:       "Markdown with evidence",
			},
		},
	}

	if err := WriteCollectedFindings(stateDir, CollectedFindings{
		TaskID:           "analyze-test",
		CollectedAt:      time.Now().UTC(),
		TotalSpecialists: 1,
		Succeeded:        1,
		TotalFindings:    2,
		Results: []SpecialistResult{{
			PerspectiveID: "security-analysis",
			Outcome:       OutcomeSuccess,
			FindingsCount: 2,
			Findings: &SpecialistFindings{
				Analyst: "security-analysis",
				Input:   "Analyze checkout reliability",
				Findings: []SpecialistFinding{
					{Finding: "Checkout retries can duplicate settlement attempts", Evidence: "payments/checkout.go:88", Severity: "HIGH"},
					{Finding: "Background reconciliation omits idempotency guardrails", Evidence: "payments/reconcile.go:144", Severity: "CRITICAL"},
				},
			},
			OutputPath: filepath.Join(stateDir, "perspectives", "security-analysis", "findings.json"),
		}},
		AllFindings: []AnnotatedFinding{
			{PerspectiveID: "security-analysis", Finding: "Checkout retries can duplicate settlement attempts", Evidence: "payments/checkout.go:88", Severity: "HIGH"},
			{PerspectiveID: "security-analysis", Finding: "Background reconciliation omits idempotency guardrails", Evidence: "payments/reconcile.go:144", Severity: "CRITICAL"},
		},
	}); err != nil {
		t.Fatalf("write collected findings: %v", err)
	}

	if err := WriteCollectedVerifications(stateDir, CollectedVerifications{
		TaskID:       "analyze-test",
		CollectedAt:  time.Now().UTC(),
		Succeeded:    1,
		AverageScore: 0.89,
		Results: []InterviewResult{{
			PerspectiveID: "security-analysis",
			Outcome:       InterviewSuccess,
			Verdict:       "pass_with_caveats",
			Score:         0.89,
			Verified: &VerifiedFindings{
				Analyst: "security-analysis",
				Topic:   "Analyze checkout reliability",
				Verdict: "pass_with_caveats",
				Score: VerificationScore{
					Assumption:    0.9,
					Relevance:     0.95,
					Constraints:   0.75,
					WeightedTotal: 0.89,
				},
				Findings: []VerifiedFinding{
					{Finding: "Checkout retries can duplicate settlement attempts", Evidence: "payments/checkout.go:88", Severity: "HIGH", Status: "confirmed", Verification: "Confirmed."},
				},
				Summary: "Primary risk confirmed with narrower secondary scope.",
			},
		}},
	}); err != nil {
		t.Fatalf("write collected verifications: %v", err)
	}

	argsPath := filepath.Join(t.TempDir(), "args.txt")
	stdinPath := filepath.Join(t.TempDir(), "stdin.txt")
	fakeCodexPath := writeFixtureCodexStub(t, codexFixtureStubOptions{
		fixturePath:      contractFixtureAbsPath(t, "synthesis-report.md"),
		argsCapturePath:  argsPath,
		stdinCapturePath: stdinPath,
	})
	t.Setenv("PRISM_CODEX_CLI_PATH", fakeCodexPath)

	task := newContractTestTask(t, stateDir, reportDir)
	cfg := AnalysisConfig{
		Topic:          "Analyze checkout reliability",
		Model:          "claude-sonnet-4-6",
		TaskID:         task.ID,
		ContextID:      task.ID,
		StateDir:       stateDir,
		ReportDir:      reportDir,
		ReportTemplate: contractFixtureAbsPath(t, "synthesis-report.md"),
	}

	if err := RunSynthesisSession(context.Background(), task, cfg, perspectives, reportPath); err != nil {
		t.Fatalf("RunSynthesisSession() error = %v", err)
	}

	assertCodexArgsContain(t, argsPath,
		"exec",
		"--json",
		"--skip-git-repo-check",
		"-C",
		stateDir,
		"--output-last-message",
		"--sandbox",
		"read-only",
	)
	assertCodexArgsNotContain(t, argsPath, "--output-schema")
	assertCodexArgsNotContain(t, argsPath, "--model claude-sonnet-4-6")
	assertFileContainsAll(t, stdinPath,
		"## Tooling Guidance\nDo NOT use any tools, shell commands, or MCP calls. Respond with plain text only from the provided context.",
		"## Execution Budget\nKeep the work within at most 1 tool-assisted turns if possible.",
		"REPORT SYNTHESIZER",
		"Analyze checkout reliability",
		"Fill the report template",
	)
	assertFileEquals(t, reportPath, readFixture(t, "synthesis-report.md"))
}

type codexFixtureStubOptions struct {
	fixturePath       string
	argsCapturePath   string
	stdinCapturePath  string
	schemaCapturePath string
}

func writeFixtureCodexStub(t *testing.T, opts codexFixtureStubOptions) string {
	t.Helper()

	stubPath := filepath.Join(t.TempDir(), "codex")
	script := fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
printf '%%s\n' "$@" > %q
cat > %q

output_path=""
schema_path=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    --output-last-message)
      output_path="$2"
      shift 2
      ;;
    --output-schema)
      schema_path="$2"
      shift 2
      ;;
    *)
      shift
      ;;
  esac
done

cp %q "$output_path"
if [ -n "$schema_path" ] && [ -n %q ]; then
  cp "$schema_path" %q
fi
`, opts.argsCapturePath, opts.stdinCapturePath, opts.fixturePath, opts.schemaCapturePath, opts.schemaCapturePath)

	if err := os.WriteFile(stubPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake codex stub: %v", err)
	}
	return stubPath
}

func newContractTestTask(t *testing.T, stateDir, reportDir string) *taskpkg.AnalysisTask {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	task := taskpkg.NewAnalysisTask("contract-test", "claude-sonnet-4-6", stateDir, reportDir, "")
	task.Ctx = ctx
	task.Cancel = cancel
	return task
}

func contractFixturePath(name string) string {
	return filepath.Join("testdata", "codex_contract", name)
}

func contractFixtureAbsPath(t *testing.T, name string) string {
	t.Helper()

	path, err := filepath.Abs(contractFixturePath(name))
	if err != nil {
		t.Fatalf("absolute fixture path for %s: %v", name, err)
	}
	return path
}

func readFixture(t *testing.T, name string) string {
	t.Helper()

	data, err := os.ReadFile(contractFixturePath(name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return string(data)
}

func assertCodexArgsContain(t *testing.T, path string, needles ...string) {
	t.Helper()
	assertFileContainsAll(t, path, needles...)
}

func assertCodexArgsNotContain(t *testing.T, path string, needle string) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read args capture %s: %v", path, err)
	}
	if strings.Contains(string(data), needle) {
		t.Fatalf("expected %s to omit %q\n%s", path, needle, string(data))
	}
}

func assertFileContainsAll(t *testing.T, path string, needles ...string) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file %s: %v", path, err)
	}
	text := string(data)
	for _, needle := range needles {
		if !strings.Contains(text, needle) {
			t.Fatalf("expected %s to contain %q\n%s", path, needle, text)
		}
	}
}

func assertJSONFileMatchesFixture(t *testing.T, actualPath, fixturePath string) {
	t.Helper()

	var actual any
	if err := json.Unmarshal([]byte(readRawFile(t, actualPath)), &actual); err != nil {
		t.Fatalf("unmarshal actual JSON %s: %v", actualPath, err)
	}

	var want any
	if err := json.Unmarshal([]byte(readRawFile(t, fixturePath)), &want); err != nil {
		t.Fatalf("unmarshal fixture JSON %s: %v", fixturePath, err)
	}

	if !reflect.DeepEqual(actual, want) {
		t.Fatalf("JSON mismatch for %s\nactual: %s\nwant: %s", actualPath, readRawFile(t, actualPath), readRawFile(t, fixturePath))
	}
}

func assertFileEquals(t *testing.T, path, want string) {
	t.Helper()

	if got := strings.TrimSpace(readRawFile(t, path)); got != strings.TrimSpace(want) {
		t.Fatalf("file %s mismatch\nactual:\n%s\nwant:\n%s", path, got, want)
	}
}

func readRawFile(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file %s: %v", path, err)
	}
	return string(data)
}
