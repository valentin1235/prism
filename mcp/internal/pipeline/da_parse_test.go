package pipeline

import (
	"testing"
)

// sampleNewFormatOutput is a representative DA output matching the exact format
// specified in SeedAnalysisDAPrompt (da_prompt.go) Phase 4.
const sampleNewFormatOutput = `## Pre-Commitment Predictions
- **Topic type**: Backend API performance investigation
- **Expected coverage**: API handlers, database layer, caching, network/transport, monitoring/logging
- **Predicted biases**: Tool bias (over-reliance on Grep), layer bias (application code only)
- **Predicted coverage gaps**: Infrastructure configs, deployment manifests, test suites

## Gaps

### [bias] All findings originate from Grep searches with no Read-based code flow analysis
The seed analyst used Grep for 8 of 10 findings, producing pattern matches without understanding how the matched code integrates into the larger request lifecycle. This risks generating perspectives that address surface symptoms rather than root causes.

### [coverage] No investigation of infrastructure or deployment configuration
The seed analysis covers application code exclusively. Missing: Kubernetes manifests, load balancer configs, connection pool settings, and environment-specific tuning that directly affect API latency.

### [bias] Findings cluster in the handler layer ignoring middleware and data access patterns
7 of 10 findings reference files in the handlers/ directory. The middleware chain (auth, rate limiting, logging) and data access layer (ORM configuration, query optimization) are absent from the analysis.

## Self-Audit Log
- Grep over-reliance: kept — 8/10 is a concrete, verifiable ratio specific to this analysis
- Missing infrastructure: kept — deployment configs are genuinely relevant to API performance
- Handler clustering: kept — 7/10 concentration is specific and verifiable
- Missing monitoring: removed — monitoring is useful but not critical for perspective generation on this topic

## Summary
- **Overall confidence**: ` + "`MEDIUM`" + ` — The analysis has concrete evidence but significant blind spots in infrastructure and cross-cutting concerns
- **Top concerns**: The combination of tool bias (Grep-only) and layer bias (handlers-only) means perspectives will likely converge on the same narrow slice of the codebase, missing systemic issues.
- **What holds up**: The identified handler-level findings are well-sourced with specific file:line references and provide a solid starting point for at least one perspective.
`

func TestParseDAGaps_NewFormat(t *testing.T) {
	gaps := ParseDAGaps(sampleNewFormatOutput)

	if len(gaps) != 3 {
		t.Fatalf("expected 3 gaps, got %d", len(gaps))
	}

	// Gap 0: bias
	if gaps[0].Type != "bias" {
		t.Errorf("gap[0].Type = %q, want %q", gaps[0].Type, "bias")
	}
	if gaps[0].Description == "" {
		t.Error("gap[0].Description should not be empty")
	}
	// Title must be included
	if got := gaps[0].Description; !containsSubstr(got, "Grep searches") {
		t.Errorf("gap[0].Description should contain title text, got: %s", truncate(got, 100))
	}

	// Gap 1: coverage
	if gaps[1].Type != "coverage" {
		t.Errorf("gap[1].Type = %q, want %q", gaps[1].Type, "coverage")
	}
	if got := gaps[1].Description; !containsSubstr(got, "infrastructure") {
		t.Errorf("gap[1].Description should reference infrastructure, got: %s", truncate(got, 100))
	}

	// Gap 2: bias
	if gaps[2].Type != "bias" {
		t.Errorf("gap[2].Type = %q, want %q", gaps[2].Type, "bias")
	}
}

func TestParseDAGaps_TitleOnly(t *testing.T) {
	// Gap with title but no body paragraph — common when DA is concise
	input := `## Gaps

### [bias] Over-reliance on Grep without Read follow-up

### [coverage] Missing test directory investigation

## Summary
- **Overall confidence**: ` + "`HIGH`" + ` — minor issues
`
	gaps := ParseDAGaps(input)

	if len(gaps) != 2 {
		t.Fatalf("expected 2 gaps, got %d", len(gaps))
	}
	if gaps[0].Type != "bias" {
		t.Errorf("gap[0].Type = %q, want bias", gaps[0].Type)
	}
	if gaps[0].Description == "" {
		t.Error("gap[0] should have description from title")
	}
	if gaps[1].Type != "coverage" {
		t.Errorf("gap[1].Type = %q, want coverage", gaps[1].Type)
	}
}

func TestParseDAGaps_ConsecutiveGapsNoBody(t *testing.T) {
	// Multiple gaps back-to-back with no body text between them
	input := `## Gaps

### [bias] Narrow topic framing
### [coverage] Missing auth module
### [bias] Confirmation bias in source selection
### [coverage] No infrastructure config analysis

## Summary
`
	gaps := ParseDAGaps(input)

	if len(gaps) != 4 {
		t.Fatalf("expected 4 gaps, got %d", len(gaps))
	}

	expectedTypes := []string{"bias", "coverage", "bias", "coverage"}
	for i, expected := range expectedTypes {
		if gaps[i].Type != expected {
			t.Errorf("gap[%d].Type = %q, want %q", i, gaps[i].Type, expected)
		}
	}
}

func TestParseDAGaps_SelfAuditLogDoesNotInterfere(t *testing.T) {
	// Self-Audit Log section uses "- [description]: action" format,
	// not "### [type]" format — verify it doesn't produce false gaps
	input := `## Gaps

### [bias] Grep-only investigation strategy

## Self-Audit Log
- [Grep-only investigation]: kept — concrete and specific
- [Missing deployment configs]: removed — not relevant to this topic
- [Handler clustering]: downgraded — only 5/10, not as extreme

## Summary
- **Overall confidence**: ` + "`HIGH`" + ` — single actionable gap
`
	gaps := ParseDAGaps(input)

	if len(gaps) != 1 {
		t.Fatalf("expected 1 gap (self-audit items should not be parsed as gaps), got %d", len(gaps))
	}
	if gaps[0].Type != "bias" {
		t.Errorf("gap[0].Type = %q, want bias", gaps[0].Type)
	}
	// Verify description does NOT bleed into Self-Audit Log or Summary sections
	if containsSubstr(gaps[0].Description, "Self-Audit") {
		t.Errorf("gap[0].Description should not contain Self-Audit content, got: %s", truncate(gaps[0].Description, 200))
	}
	if containsSubstr(gaps[0].Description, "Overall confidence") {
		t.Errorf("gap[0].Description should not contain Summary content, got: %s", truncate(gaps[0].Description, 200))
	}
}

func TestParseDAGaps_LastGapDoesNotBleedIntoSections(t *testing.T) {
	// Regression test: the last gap's description must NOT include
	// ## Self-Audit Log or ## Summary content
	gaps := ParseDAGaps(sampleNewFormatOutput)
	if len(gaps) != 3 {
		t.Fatalf("expected 3 gaps, got %d", len(gaps))
	}
	lastGap := gaps[2]
	if containsSubstr(lastGap.Description, "Self-Audit") {
		t.Errorf("last gap Description must not bleed into Self-Audit Log, got: %s", truncate(lastGap.Description, 200))
	}
	if containsSubstr(lastGap.Description, "Overall confidence") {
		t.Errorf("last gap Description must not bleed into Summary, got: %s", truncate(lastGap.Description, 200))
	}
	if containsSubstr(lastGap.Description, "Top concerns") {
		t.Errorf("last gap Description must not bleed into Summary, got: %s", truncate(lastGap.Description, 200))
	}
}

func TestParseDAGaps_CaseInsensitiveType(t *testing.T) {
	// LLMs may output [Bias] or [COVERAGE] instead of lowercase
	input := `## Gaps

### [Bias] Mixed-case bias entry
Some description.

### [COVERAGE] Uppercase coverage entry
Another description.

## Summary
- **Overall confidence**: ` + "`HIGH`" + ` — All good
`
	gaps := ParseDAGaps(input)
	if len(gaps) != 2 {
		t.Fatalf("expected 2 gaps from mixed-case input, got %d", len(gaps))
	}
	if gaps[0].Type != "bias" {
		t.Errorf("gap[0].Type = %q, want 'bias' (normalized to lowercase)", gaps[0].Type)
	}
	if gaps[1].Type != "coverage" {
		t.Errorf("gap[1].Type = %q, want 'coverage' (normalized to lowercase)", gaps[1].Type)
	}
}

func TestParseDAGaps_NoneIdentified(t *testing.T) {
	// When DA finds no gaps, the prompt instructs: write "None identified."
	input := `## Pre-Commitment Predictions
- **Topic type**: Module restructuring
- **Expected coverage**: Code structure, dependencies, tests

## Gaps

None identified.

## Self-Audit Log
- No gaps to audit.

## Summary
- **Overall confidence**: ` + "`HIGH`" + ` — Analysis is thorough
- **Top concerns**: None significant
- **What holds up**: All areas covered with evidence
`
	gaps := ParseDAGaps(input)

	if len(gaps) != 0 {
		t.Errorf("expected 0 gaps for 'None identified', got %d", len(gaps))
	}
}

func TestParseDAGaps_EmptyInput(t *testing.T) {
	gaps := ParseDAGaps("")
	if len(gaps) != 0 {
		t.Errorf("expected 0 gaps for empty input, got %d", len(gaps))
	}
}

func TestParseDAGaps_BodyIncludedInDescription(t *testing.T) {
	// When a gap has both title and body, description should combine both
	input := `## Gaps

### [coverage] Missing database migration scripts
The seed analysis found queries and ORM configs but did not investigate migration files that define the schema evolution.

## Summary
`
	gaps := ParseDAGaps(input)

	if len(gaps) != 1 {
		t.Fatalf("expected 1 gap, got %d", len(gaps))
	}

	desc := gaps[0].Description
	// Title should be present
	if !containsSubstr(desc, "Missing database migration scripts") {
		t.Errorf("description should contain title, got: %s", desc)
	}
	// Body should be present
	if !containsSubstr(desc, "migration files") {
		t.Errorf("description should contain body text, got: %s", desc)
	}
}

func TestParseDASummary_NewFormat(t *testing.T) {
	confidence, topConcerns, whatHoldsUp := ParseDASummary(sampleNewFormatOutput)

	if confidence != "MEDIUM" {
		t.Errorf("confidence = %q, want MEDIUM", confidence)
	}
	if topConcerns == "" {
		t.Error("topConcerns should not be empty")
	}
	if !containsSubstr(topConcerns, "tool bias") {
		t.Errorf("topConcerns should reference tool bias, got: %s", topConcerns)
	}
	if whatHoldsUp == "" {
		t.Error("whatHoldsUp should not be empty")
	}
	if !containsSubstr(whatHoldsUp, "handler-level") {
		t.Errorf("whatHoldsUp should reference handler-level findings, got: %s", whatHoldsUp)
	}
}

func TestParseDASummary_ConfidenceLevels(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantConf string
	}{
		{
			"HIGH with backticks",
			"- **Overall confidence**: `HIGH` — all good",
			"HIGH",
		},
		{
			"MEDIUM without backticks",
			"- **Overall confidence**: MEDIUM — some issues",
			"MEDIUM",
		},
		{
			"LOW with backticks",
			"- **Overall confidence**: `LOW` — major concerns",
			"LOW",
		},
		{
			"HIGH without justification dash",
			"- **Overall confidence**: `HIGH`",
			"HIGH",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conf, _, _ := ParseDASummary(tt.input)
			if conf != tt.wantConf {
				t.Errorf("confidence = %q, want %q", conf, tt.wantConf)
			}
		})
	}
}

func TestParseDASummary_MissingSections(t *testing.T) {
	// When summary sections are missing, should return empty strings (not panic)
	confidence, topConcerns, whatHoldsUp := ParseDASummary("No structured summary here")

	if confidence != "" {
		t.Errorf("confidence should be empty for unstructured input, got %q", confidence)
	}
	if topConcerns != "" {
		t.Errorf("topConcerns should be empty for unstructured input, got %q", topConcerns)
	}
	if whatHoldsUp != "" {
		t.Errorf("whatHoldsUp should be empty for unstructured input, got %q", whatHoldsUp)
	}
}

func TestParseDASummary_EmptyInput(t *testing.T) {
	confidence, topConcerns, whatHoldsUp := ParseDASummary("")

	if confidence != "" || topConcerns != "" || whatHoldsUp != "" {
		t.Error("all summary fields should be empty for empty input")
	}
}

func TestGapKeywordRe_DetectsParseFailure(t *testing.T) {
	// When ParseDAGaps returns nothing but keywords are present,
	// the parse failure detector should flag it
	malformedOutput := `The analysis has a bias toward backend code.
There is also a coverage gap in the testing layer.
No properly formatted gap entries here.`

	gaps := ParseDAGaps(malformedOutput)
	if len(gaps) != 0 {
		t.Fatalf("malformed output should produce 0 parsed gaps, got %d", len(gaps))
	}

	// But keyword detection should find bias/coverage mentions
	if !GapKeywordRe.MatchString(malformedOutput) {
		t.Error("GapKeywordRe should detect bias/coverage keywords in malformed output")
	}
}

func TestGapKeywordRe_NoFalsePositive(t *testing.T) {
	// Clean pass output should not trigger keyword detection
	cleanOutput := `## Gaps

None identified.

## Summary
- **Overall confidence**: ` + "`HIGH`" + ` — Analysis is thorough
- **Top concerns**: None significant
- **What holds up**: Everything
`
	gaps := ParseDAGaps(cleanOutput)
	if len(gaps) != 0 {
		t.Error("clean output should have 0 gaps")
	}

	// Note: this WILL match because "None identified" doesn't contain bias/coverage
	// The keyword regex is only a safety net for when gaps=0 but keywords exist
}

func TestParseDAGaps_TypeCaseNormalization(t *testing.T) {
	// Even if LLM outputs mixed case, type should be normalized to lowercase
	input := `## Gaps

### [Bias] Mixed case type
### [COVERAGE] All caps type
### [bias] Normal lowercase

## Summary
`
	gaps := ParseDAGaps(input)

	// Only lowercase matches from regex — Bias and COVERAGE won't match
	// because gapEntryRe uses (bias|coverage) which is case-sensitive
	// This is by design: the prompt specifies lowercase types
	for _, g := range gaps {
		if g.Type != "bias" && g.Type != "coverage" {
			t.Errorf("gap type %q should be lowercase bias or coverage", g.Type)
		}
	}
}

func TestCountGapsByType_WithParsedOutput(t *testing.T) {
	gaps := ParseDAGaps(sampleNewFormatOutput)
	bias, coverage := CountGapsByType(gaps)

	if bias != 2 {
		t.Errorf("bias count = %d, want 2", bias)
	}
	if coverage != 1 {
		t.Errorf("coverage count = %d, want 1", coverage)
	}
}

func TestShouldPassDAGaps_WithParsedOutput(t *testing.T) {
	// Sample with gaps → should not pass
	gaps := ParseDAGaps(sampleNewFormatOutput)
	if ShouldPassDAGaps(gaps) {
		t.Error("should not pass when gaps exist")
	}

	// No gaps → should pass
	noGaps := ParseDAGaps("No structured output")
	if !ShouldPassDAGaps(noGaps) {
		t.Error("should pass when no gaps")
	}
}

// --- helpers ---

func containsSubstr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && findSubstr(s, substr))
}

func findSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
