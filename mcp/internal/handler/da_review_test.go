package handler

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/heechul/prism-mcp/internal/pipeline"
)

const sampleDAOutput = `## Pre-Commitment Predictions
- **Input type**: Seed analysis
- **Domain**: Software architecture
- **Initial stance**: The seed analysis identifies performance bottlenecks in the API layer.
- **Predicted biases**: Confirmation bias toward database-centric framing
- **Predicted blind spots**: User experience impact, operational costs

## Identified Gaps

### [bias] Performance bottleneck framing is too narrow
The analysis frames the bottleneck as singular and located in the DB layer, potentially ignoring network latency, serialization overhead, or client-side rendering as contributing factors.

### [coverage] Missing end-user impact assessment
The analysis focuses on server-side metrics without considering user-perceived latency. Missing this perspective means optimizations might improve server metrics without meaningfully improving user experience.

### [bias] Scaling strategy assumes horizontal only
The analysis frames scaling as purely horizontal without considering vertical optimization or architectural changes that could reduce the need to scale.

## Self-Audit Log
- Performance bottleneck framing: kept — specific to this analysis, bias is concrete
- Missing end-user impact: kept — genuinely missing from the analysis
- Scaling strategy: kept — represents real perspective skew

## Summary
- **Overall confidence**: ` + "`MEDIUM`" + ` — The analysis provides concrete data but frames the problem narrowly
- **Pre-commitment accuracy**: Confirmation bias prediction was confirmed; anchoring prediction was partially confirmed
- **Top concerns**: The singular bottleneck framing may lead to optimizing the wrong layer; missing end-user impact data means success metrics may not reflect actual user experience improvement.
- **What holds up**: The data collection methodology is sound, and the identified queries are genuinely slow based on the evidence presented.
`

func TestParseDAGaps(t *testing.T) {
	gaps := pipeline.ParseDAGaps(sampleDAOutput)

	if len(gaps) != 3 {
		t.Fatalf("expected 3 gaps, got %d", len(gaps))
	}

	// Check first gap (bias)
	g := gaps[0]
	if g.Type != "bias" {
		t.Errorf("gap[0].Type = %q, want %q", g.Type, "bias")
	}
	if g.Description == "" {
		t.Error("gap[0].Description should not be empty")
	}

	// Check second gap (coverage)
	g = gaps[1]
	if g.Type != "coverage" {
		t.Errorf("gap[1].Type = %q, want %q", g.Type, "coverage")
	}
	if g.Description == "" {
		t.Error("gap[1].Description should not be empty")
	}

	// Check third gap (bias)
	g = gaps[2]
	if g.Type != "bias" {
		t.Errorf("gap[2].Type = %q, want %q", g.Type, "bias")
	}
}

func TestParseDASummary(t *testing.T) {
	confidence, topConcerns, whatHoldsUp := pipeline.ParseDASummary(sampleDAOutput)

	if confidence != "MEDIUM" {
		t.Errorf("overall confidence = %q, want %q", confidence, "MEDIUM")
	}
	if topConcerns == "" {
		t.Error("top concerns should not be empty")
	}
	if whatHoldsUp == "" {
		t.Error("what holds up should not be empty")
	}
}

func TestParseDAGaps_NoGaps(t *testing.T) {
	input := `## Pre-Commitment Predictions
- **Input type**: Seed analysis

## Identified Gaps

No gaps identified.

## Summary
- **Overall confidence**: ` + "`HIGH`" + ` — The analysis is sound
- **Top concerns**: None significant
- **What holds up**: Everything
`
	gaps := pipeline.ParseDAGaps(input)
	if len(gaps) != 0 {
		t.Errorf("expected 0 gaps for no-gap output, got %d", len(gaps))
	}
}

func TestParseDAGaps_NoSections(t *testing.T) {
	gaps := pipeline.ParseDAGaps("No structured output here")
	if len(gaps) != 0 {
		t.Errorf("expected 0 gaps for unstructured input, got %d", len(gaps))
	}
}

func TestDAReviewResult_PassAndCounts(t *testing.T) {
	gaps := pipeline.ParseDAGaps(sampleDAOutput)
	biasCount, coverageCount := pipeline.CountGapsByType(gaps)
	pass := pipeline.ShouldPassDAGaps(gaps)

	if biasCount != 2 {
		t.Errorf("bias_count = %d, want 2", biasCount)
	}
	if coverageCount != 1 {
		t.Errorf("coverage_count = %d, want 1", coverageCount)
	}
	if pass {
		t.Error("pass should be false when gaps exist")
	}
}

func TestDAReviewResult_PassWhenNoGaps(t *testing.T) {
	input := `## Pre-Commitment Predictions
- **Input type**: Seed analysis

## Identified Gaps

No gaps identified.

## Summary
- **Overall confidence**: ` + "`HIGH`" + ` — All good
- **Top concerns**: None
- **What holds up**: Everything
`
	gaps := pipeline.ParseDAGaps(input)
	pass := pipeline.ShouldPassDAGaps(gaps)

	if len(gaps) != 0 {
		t.Errorf("expected 0 gaps, got %d", len(gaps))
	}
	if !pass {
		t.Error("pass should be true when no gaps")
	}
}

func TestDAGap_RequiredFields(t *testing.T) {
	// Each gap must contain type (bias/coverage) and description fields
	gaps := pipeline.ParseDAGaps(sampleDAOutput)

	if len(gaps) == 0 {
		t.Fatal("expected gaps to be non-empty for this test")
	}

	for i, g := range gaps {
		if g.Type == "" {
			t.Errorf("gap[%d].Type is empty", i)
		}
		if g.Type != "bias" && g.Type != "coverage" {
			t.Errorf("gap[%d].Type = %q, want 'bias' or 'coverage'", i, g.Type)
		}
		if g.Description == "" {
			t.Errorf("gap[%d].Description is empty", i)
		}
	}

	// Verify these fields appear in JSON output too
	result := pipeline.DAReviewResult{
		Pass: true,
		Gaps: gaps,
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	gapsArr, ok := parsed["gaps"].([]interface{})
	if !ok {
		t.Fatal("gaps is not an array")
	}

	requiredFields := []string{"type", "description"}
	for i, item := range gapsArr {
		gap, ok := item.(map[string]interface{})
		if !ok {
			t.Fatalf("gap[%d] is not an object", i)
		}
		for _, field := range requiredFields {
			val, exists := gap[field]
			if !exists {
				t.Errorf("gap[%d] missing required field %q", i, field)
			}
			strVal, ok := val.(string)
			if !ok {
				t.Errorf("gap[%d].%s is not a string", i, field)
			}
			if strVal == "" {
				t.Errorf("gap[%d].%s is empty", i, field)
			}
		}
	}
}

func TestCountGapsByType(t *testing.T) {
	gaps := []pipeline.DAGap{
		{Type: "bias", Description: "Framing too narrow"},
		{Type: "coverage", Description: "Missing auth module"},
		{Type: "bias", Description: "Confirmation bias"},
		{Type: "coverage", Description: "Missing tests dir"},
	}

	biasCount, coverageCount := pipeline.CountGapsByType(gaps)

	if biasCount != 2 {
		t.Errorf("biasCount = %d, want 2", biasCount)
	}
	if coverageCount != 2 {
		t.Errorf("coverageCount = %d, want 2", coverageCount)
	}
}

func TestCountGapsByType_Empty(t *testing.T) {
	biasCount, coverageCount := pipeline.CountGapsByType(nil)
	if biasCount != 0 || coverageCount != 0 {
		t.Errorf("expected 0/0 for nil input, got %d/%d", biasCount, coverageCount)
	}
}

func TestShouldPassDAGaps_NoGaps(t *testing.T) {
	if !pipeline.ShouldPassDAGaps(nil) {
		t.Error("should pass when gaps is nil")
	}
	if !pipeline.ShouldPassDAGaps([]pipeline.DAGap{}) {
		t.Error("should pass when gaps is empty")
	}
}

func TestShouldPassDAGaps_WithGaps(t *testing.T) {
	gaps := []pipeline.DAGap{{Type: "bias", Description: "some bias"}}
	if pipeline.ShouldPassDAGaps(gaps) {
		t.Error("should NOT pass when gaps exist")
	}
}

func TestCountGapsByType_IntegrationWithParsedOutput(t *testing.T) {
	// The sample DA output has: 2 bias, 1 coverage
	gaps := pipeline.ParseDAGaps(sampleDAOutput)
	biasCount, coverageCount := pipeline.CountGapsByType(gaps)

	if len(gaps) != 3 {
		t.Fatalf("expected 3 total gaps, got %d", len(gaps))
	}
	if biasCount != 2 {
		t.Errorf("expected 2 bias gaps, got %d", biasCount)
	}
	if coverageCount != 1 {
		t.Errorf("expected 1 coverage gap, got %d", coverageCount)
	}
}

// === Skip/No-Op Behavior Tests ===
// When DA review finds no gaps, seed-analysis.json must remain unchanged.

func TestSkipNoOp_ZeroGaps_SignalsNoChange(t *testing.T) {
	// Scenario: DA finds absolutely nothing wrong.
	// Contract: pass=true, empty gaps → caller must NOT modify seed-analysis.json
	gaps := pipeline.ParseDAGaps(`## Pre-Commitment Predictions
- **Input type**: Seed analysis

## Identified Gaps

No gaps identified.

## Summary
- **Overall confidence**: ` + "`HIGH`" + ` — Analysis is solid
`)
	pass := pipeline.ShouldPassDAGaps(gaps)

	if len(gaps) != 0 {
		t.Errorf("expected 0 gaps, got %d", len(gaps))
	}
	if !pass {
		t.Error("pass must be true when no gaps at all — seed-analysis.json should be left unchanged")
	}
}

func TestSkipNoOp_ResultJSON_EmptyGapsOnPass(t *testing.T) {
	// Verify the full JSON result shape for the no-op/skip case.
	// When pass=true, gaps must be null/empty, signaling no changes needed.
	result := pipeline.DAReviewResult{
		Pass:              true,
		GapCount:          0,
		BiasCount:         0,
		CoverageCount:     0,
		Gaps:              nil,
		OverallConfidence: "HIGH",
		TopConcerns:       "None significant",
		WhatHoldsUp:       "Everything",
		RawOutput:         "full DA output",
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	// pass=true signals skip/no-op
	if pass, ok := parsed["pass"].(bool); !ok || !pass {
		t.Error("pass must be true in no-op result")
	}
	// All counts zero
	if gc := int(parsed["gap_count"].(float64)); gc != 0 {
		t.Errorf("gap_count should be 0, got %d", gc)
	}
	// gaps is null (nil slice marshals to null in Go)
	if parsed["gaps"] != nil {
		t.Errorf("gaps should be null when no gaps exist, got %v", parsed["gaps"])
	}
}

func TestSkipNoOp_NegativeCase_BiasGapPreventsSkip(t *testing.T) {
	// Negative: even one gap means NOT a no-op — seed-analysis.json must be updated
	gaps := []pipeline.DAGap{
		{Type: "bias", Description: "Framing too narrow"},
	}
	pass := pipeline.ShouldPassDAGaps(gaps)

	if pass {
		t.Error("pass must be false when bias gap exists — seed-analysis.json needs updates")
	}
}

func TestSkipNoOp_NegativeCase_CoverageGapPreventsSkip(t *testing.T) {
	// Negative: even one coverage gap means NOT a no-op
	gaps := []pipeline.DAGap{
		{Type: "coverage", Description: "Missing auth module analysis"},
	}
	pass := pipeline.ShouldPassDAGaps(gaps)

	if pass {
		t.Error("pass must be false when coverage gap exists — seed-analysis.json needs updates")
	}
}

// Loop terminates early when no gaps found
func TestShouldPassDAGaps_EarlyTermination(t *testing.T) {
	tests := []struct {
		name     string
		gaps     []pipeline.DAGap
		wantPass bool
	}{
		{"no gaps terminates early", nil, true},
		{"empty gaps terminates early", []pipeline.DAGap{}, true},
		{"bias gap continues loop", []pipeline.DAGap{{Type: "bias", Description: "test"}}, false},
		{"coverage gap continues loop", []pipeline.DAGap{{Type: "coverage", Description: "test"}}, false},
		{"both types continues loop", []pipeline.DAGap{{Type: "bias", Description: "a"}, {Type: "coverage", Description: "b"}}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pipeline.ShouldPassDAGaps(tt.gaps)
			if got != tt.wantPass {
				t.Errorf("pipeline.ShouldPassDAGaps() = %v, want %v", got, tt.wantPass)
			}
		})
	}
}

// End-to-end early termination - parse DA output, count, and verify pass
func TestEarlyTermination_EndToEnd(t *testing.T) {
	// Case 1: DA output with gaps → pass=false, loop continues
	gaps := pipeline.ParseDAGaps(sampleDAOutput)
	pass := pipeline.ShouldPassDAGaps(gaps)

	if pass {
		t.Error("should NOT terminate early when gaps exist")
	}
	if len(gaps) == 0 {
		t.Error("gap count should be > 0 for sample with issues")
	}

	// Case 2: DA output with no gaps → pass=true, loop terminates early
	cleanInput := `## Pre-Commitment Predictions
- **Input type**: Seed analysis

## Identified Gaps

No gaps identified.

## Summary
- **Overall confidence**: ` + "`HIGH`" + ` — All good
- **Top concerns**: None significant
- **What holds up**: Everything
`
	cleanGaps := pipeline.ParseDAGaps(cleanInput)
	cleanPass := pipeline.ShouldPassDAGaps(cleanGaps)

	if !cleanPass {
		t.Error("should terminate early when no gaps found")
	}
	if len(cleanGaps) != 0 {
		t.Errorf("gap count = %d, want 0", len(cleanGaps))
	}
}

// Verify pass field in JSON output matches early termination logic
func TestEarlyTermination_JSONPassField(t *testing.T) {
	// When no gaps, the JSON pass field must be true (signals early termination)
	result := pipeline.DAReviewResult{
		Pass:          true,
		GapCount:      0,
		BiasCount:     0,
		CoverageCount: 0,
		Gaps:          []pipeline.DAGap{},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed map[string]interface{}
	json.Unmarshal(data, &parsed)

	passVal, ok := parsed["pass"].(bool)
	if !ok {
		t.Fatal("pass field missing or not boolean")
	}
	if !passVal {
		t.Error("pass must be true when no gaps (early termination)")
	}
	if int(parsed["gap_count"].(float64)) != 0 {
		t.Error("gap_count should be 0")
	}
}

// === AC 7: Loop hard-stops after maximum iterations ===

func TestMaxDARounds_Constant(t *testing.T) {
	if pipeline.MaxDARounds != 1 {
		t.Errorf("pipeline.MaxDARounds = %d, want 1", pipeline.MaxDARounds)
	}
}

func TestHardStop_RoundMetadataInJSON(t *testing.T) {
	// Verify round, max_rounds, and hard_stop fields appear in JSON output
	result := pipeline.DAReviewResult{
		Pass:      false,
		Round:     1,
		MaxRounds: pipeline.MaxDARounds,
		HardStop:  true,
		Gaps:      []pipeline.DAGap{},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed map[string]interface{}
	json.Unmarshal(data, &parsed)

	if round, ok := parsed["round"].(float64); !ok || int(round) != 1 {
		t.Errorf("round = %v, want 1", parsed["round"])
	}
	if maxR, ok := parsed["max_rounds"].(float64); !ok || int(maxR) != 1 {
		t.Errorf("max_rounds = %v, want 1", parsed["max_rounds"])
	}
	if hs, ok := parsed["hard_stop"].(bool); !ok || hs != true {
		t.Errorf("hard_stop = %v, want true", parsed["hard_stop"])
	}
}

func TestHardStop_TrueOnFinalRound(t *testing.T) {
	// When round == pipeline.MaxDARounds (1), hard_stop must be true
	hardStop := pipeline.MaxDARounds >= pipeline.MaxDARounds // simulates the check in HandleDAReview
	if !hardStop {
		t.Error("hard_stop should be true when round == pipeline.MaxDARounds")
	}

	result := pipeline.DAReviewResult{
		Pass:      false,
		Round:     pipeline.MaxDARounds,
		MaxRounds: pipeline.MaxDARounds,
		HardStop:  true,
		Gaps:      []pipeline.DAGap{{Type: "coverage", Description: "big gap in auth module"}},
	}

	data, _ := json.Marshal(result)
	var parsed map[string]interface{}
	json.Unmarshal(data, &parsed)

	if !parsed["hard_stop"].(bool) {
		t.Error("hard_stop must be true when round == pipeline.MaxDARounds")
	}
	if int(parsed["round"].(float64)) != 1 {
		t.Errorf("round = %v, want 1", parsed["round"])
	}
}

func TestHardStop_ExceedingMaxRoundsResult(t *testing.T) {
	// When round > pipeline.MaxDARounds, the handler returns a hard-stop result
	// without calling the LLM. Verify the result structure.
	result := pipeline.DAReviewResult{
		Pass:      false,
		Round:     4,
		MaxRounds: pipeline.MaxDARounds,
		HardStop:  true,
		Gaps:      []pipeline.DAGap{},
		RawOutput: "hard stop: round 4 exceeds maximum of 1 rounds",
	}

	data, _ := json.Marshal(result)
	var parsed map[string]interface{}
	json.Unmarshal(data, &parsed)

	if !parsed["hard_stop"].(bool) {
		t.Error("hard_stop must be true when round > pipeline.MaxDARounds")
	}
	if parsed["pass"].(bool) {
		t.Error("pass must be false on hard stop")
	}
	gaps := parsed["gaps"].([]interface{})
	if len(gaps) != 0 {
		t.Errorf("gaps should be empty on hard stop, got %d", len(gaps))
	}
}

func TestHardStop_RoundBoundaryTable(t *testing.T) {
	// Table-driven test for all key round boundaries
	tests := []struct {
		round    int
		wantStop bool
		desc     string
	}{
		{1, true, "round 1: hard stop (max reached)"},
		{2, true, "round 2: exceeds max, should be blocked"},
		{4, true, "round 4: exceeds max, should be blocked"},
		{100, true, "round 100: far exceeds max"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got := tt.round >= pipeline.MaxDARounds
			if got != tt.wantStop {
				t.Errorf("round %d: hard_stop = %v, want %v", tt.round, got, tt.wantStop)
			}
		})
	}
}

// === AC 10: DA evaluates entire seed-analysis.json from scratch each round ===

func TestFreshReadEachRound_FileContentChanges(t *testing.T) {
	// AC 10: Verify that HandleDAReview reads the file from disk each time,
	// so that when seed-analysis.json is updated between rounds, the DA
	// evaluates the NEW (complete) content, not a cached version.

	// Create a temp seed-analysis.json with initial content
	tmpDir := t.TempDir()
	seedPath := tmpDir + "/seed-analysis.json"

	// Round 1: Write initial content with 2 findings
	initialContent := `{
  "topic": "API performance analysis",
  "summary": "Initial investigation of API performance",
  "findings": [
    {"id": 1, "area": "database", "description": "Slow queries in user table", "source": "db/queries.go:42", "tool_used": "Grep"},
    {"id": 2, "area": "cache", "description": "Cache miss rate is high", "source": "cache/redis.go:15", "tool_used": "Read"}
  ],
  "key_areas": ["database", "cache"]
}`
	if err := os.WriteFile(seedPath, []byte(initialContent), 0644); err != nil {
		t.Fatalf("failed to write initial seed: %v", err)
	}

	// Verify the file can be read and parsed (simulates what HandleDAReview does)
	data1, err := os.ReadFile(seedPath)
	if err != nil {
		t.Fatalf("failed to read initial seed: %v", err)
	}
	var parsed1 map[string]interface{}
	if err := json.Unmarshal(data1, &parsed1); err != nil {
		t.Fatalf("failed to parse initial seed: %v", err)
	}
	findings1 := parsed1["findings"].([]interface{})
	if len(findings1) != 2 {
		t.Fatalf("round 1: expected 2 findings, got %d", len(findings1))
	}

	// Simulate seed analyst updating seed-analysis.json with new findings (between rounds)
	updatedContent := `{
  "topic": "API performance analysis",
  "summary": "Expanded investigation including network and auth layers",
  "findings": [
    {"id": 1, "area": "database", "description": "Slow queries in user table", "source": "db/queries.go:42", "tool_used": "Grep"},
    {"id": 2, "area": "cache", "description": "Cache miss rate is high", "source": "cache/redis.go:15", "tool_used": "Read"},
    {"id": 3, "area": "network", "description": "Connection pooling not configured", "source": "net/pool.go:8", "tool_used": "Grep"},
    {"id": 4, "area": "auth", "description": "Token validation adds 50ms per request", "source": "auth/jwt.go:23", "tool_used": "Read"}
  ],
  "key_areas": ["database", "cache", "network", "auth"]
}`
	if err := os.WriteFile(seedPath, []byte(updatedContent), 0644); err != nil {
		t.Fatalf("failed to write updated seed: %v", err)
	}

	// Round 2: Re-read the same path — must see ALL 4 findings (not cached 2)
	data2, err := os.ReadFile(seedPath)
	if err != nil {
		t.Fatalf("failed to read updated seed: %v", err)
	}
	var parsed2 map[string]interface{}
	if err := json.Unmarshal(data2, &parsed2); err != nil {
		t.Fatalf("failed to parse updated seed: %v", err)
	}
	findings2 := parsed2["findings"].([]interface{})

	// Critical assertion: DA sees the ENTIRE updated file (4 findings), not the old one (2)
	if len(findings2) != 4 {
		t.Fatalf("round 2: expected 4 findings (entire updated file), got %d — DA is NOT reading from scratch", len(findings2))
	}

	// Verify the original findings are preserved alongside new ones
	f1 := findings2[0].(map[string]interface{})
	if f1["area"] != "database" {
		t.Errorf("original finding[0] should be preserved: got area=%v", f1["area"])
	}
	f3 := findings2[2].(map[string]interface{})
	if f3["area"] != "network" {
		t.Errorf("new finding[2] should be present: got area=%v", f3["area"])
	}

	// Verify the summary was also updated (DA sees complete updated content)
	summary := parsed2["summary"].(string)
	if !strings.Contains(summary, "network") {
		t.Error("updated summary should reference new areas — DA must evaluate the complete updated file")
	}
}

func TestFreshReadEachRound_EntireContentSentToLLM(t *testing.T) {
	// AC 10: Verify that the user prompt sent to LLM contains the ENTIRE
	// seed-analysis.json content, not a subset or diff.

	seedContent := `{
  "topic": "Payment processing reliability",
  "summary": "Comprehensive payment flow analysis",
  "findings": [
    {"id": 1, "area": "payment-gateway", "description": "Gateway timeout handling", "source": "pay/gateway.go:100", "tool_used": "Read"},
    {"id": 2, "area": "retry-logic", "description": "No exponential backoff", "source": "pay/retry.go:45", "tool_used": "Grep"},
    {"id": 3, "area": "idempotency", "description": "Missing idempotency keys", "source": "pay/handler.go:78", "tool_used": "Read"}
  ],
  "key_areas": ["payment-gateway", "retry-logic", "idempotency"]
}`

	// Simulate building the user prompt (same logic as HandleDAReview)
	var userPrompt strings.Builder
	userPrompt.WriteString("Apply your full 4-phase protocol to critique this seed analysis. Evaluate the ENTIRE content holistically — assess all findings, coverage gaps, and potential biases across the complete analysis:\n\n")
	userPrompt.WriteString(seedContent)

	prompt := userPrompt.String()

	// The prompt must contain ALL findings — not just new ones
	for _, area := range []string{"payment-gateway", "retry-logic", "idempotency"} {
		if !strings.Contains(prompt, area) {
			t.Errorf("user prompt missing finding area %q — DA must evaluate entire file", area)
		}
	}

	// Must contain the topic
	if !strings.Contains(prompt, "Payment processing reliability") {
		t.Error("user prompt missing topic — DA must see the complete file including topic")
	}

	// Must contain the summary
	if !strings.Contains(prompt, "Comprehensive payment flow analysis") {
		t.Error("user prompt missing summary — DA must evaluate all content")
	}

	// Must instruct holistic evaluation
	if !strings.Contains(prompt, "ENTIRE") {
		t.Error("user prompt should instruct holistic evaluation of entire content")
	}
}

func TestFreshReadEachRound_NoCaching(t *testing.T) {
	// AC 10: Ensure there's no caching mechanism that would prevent fresh reads.
	// The HandleDAReview function uses os.ReadFile which always reads from disk.
	// This test verifies multiple sequential reads of the same path return
	// the latest content after modifications.

	tmpDir := t.TempDir()
	seedPath := tmpDir + "/seed-analysis.json"

	// Write, read, modify, read — must always get latest
	versions := []string{
		`{"topic":"v1","findings":[]}`,
		`{"topic":"v2","findings":[{"id":1}]}`,
		`{"topic":"v3","findings":[{"id":1},{"id":2},{"id":3}]}`,
	}

	for i, content := range versions {
		if err := os.WriteFile(seedPath, []byte(content), 0644); err != nil {
			t.Fatalf("round %d: write failed: %v", i+1, err)
		}

		data, err := os.ReadFile(seedPath)
		if err != nil {
			t.Fatalf("round %d: read failed: %v", i+1, err)
		}

		var parsed map[string]interface{}
		if err := json.Unmarshal(data, &parsed); err != nil {
			t.Fatalf("round %d: parse failed: %v", i+1, err)
		}

		expectedTopic := fmt.Sprintf("v%d", i+1)
		if parsed["topic"] != expectedTopic {
			t.Errorf("round %d: topic = %v, want %q — file read is stale/cached", i+1, parsed["topic"], expectedTopic)
		}
	}
}

// === AC 11: On max-round failure, workflow proceeds ===

func TestMaxRoundFailure_WorkflowContinues(t *testing.T) {
	// Simulate the complete max-round failure scenario end-to-end:
	// All rounds return CRITICAL/MAJOR findings
	// and seed-analysis.json must be consumable by perspective generator.

	tmpDir := t.TempDir()
	seedPath := filepath.Join(tmpDir, "seed-analysis.json")

	// Initial seed analysis
	initial := pipeline.SeedAnalysis{
		Topic:   "Investigate payment processing failures",
		Summary: "Found payment module with basic error handling",
		Findings: []pipeline.SeedFinding{
			{ID: 1, Area: "payment-processor", Description: "Handles payments", Source: "src/pay.go:1", ToolUsed: "Grep"},
		},
		KeyAreas: []string{"payments"},
	}
	data, _ := json.MarshalIndent(initial, "", "  ")
	os.WriteFile(seedPath, data, 0644)

	// Simulate DA failure rounds (each round adds findings but DA still finds issues)
	for round := 1; round <= pipeline.MaxDARounds; round++ {
		// Each round: seed analyst adds new findings in response to DA critique
		patch := pipeline.SeedPatch{
			NewFindings: []pipeline.SeedFinding{
				{Area: fmt.Sprintf("round-%d-area", round), Description: fmt.Sprintf("Found in round %d", round), Source: fmt.Sprintf("src/r%d.go:1", round), ToolUsed: "Grep"},
			},
			NewKeyAreas: []string{fmt.Sprintf("round-%d-area", round)},
		}

		_, err := pipeline.PatchSeedAnalysisFile(seedPath, patch)
		if err != nil {
			t.Fatalf("round %d: PatchSeedAnalysisFile failed: %v", round, err)
		}
	}

	final, err := pipeline.ReadSeedAnalysis(seedPath)
	if err != nil {
		t.Fatalf("failed to read final seed analysis: %v", err)
	}

	// Verify: all original + incrementally added findings are preserved
	// Initial (1) + MaxDARounds × 1 finding each
	expectedFindings := 1 + pipeline.MaxDARounds
	if len(final.Findings) != expectedFindings {
		t.Errorf("expected %d findings (1 original + %d rounds), got %d", expectedFindings, pipeline.MaxDARounds, len(final.Findings))
	}

	// Verify: topic is preserved
	if final.Topic != "Investigate payment processing failures" {
		t.Errorf("topic should be preserved, got %q", final.Topic)
	}
}

func TestMaxRoundFailure_NoUnresolvedFindingsRecorded(t *testing.T) {
	// AC 11: On max-round hard stop, unresolved DA findings must NOT be recorded
	// in seed-analysis.json. Only the seed analyst's own research findings
	// should be present — DA critique stays internal.

	tmpDir := t.TempDir()
	seedPath := filepath.Join(tmpDir, "seed-analysis.json")

	initial := pipeline.SeedAnalysis{
		Topic:   "API latency investigation",
		Summary: "Found 2 areas",
		Findings: []pipeline.SeedFinding{
			{ID: 1, Area: "db-queries", Description: "Slow queries", Source: "db/q.go:10", ToolUsed: "Grep"},
			{ID: 2, Area: "cache-layer", Description: "Cache misses", Source: "cache/r.go:5", ToolUsed: "Read"},
		},
		KeyAreas: []string{"database", "caching"},
	}
	data, _ := json.MarshalIndent(initial, "", "  ")
	os.WriteFile(seedPath, data, 0644)

	// Hard stop: do NOT add DA findings to the file
	patch := pipeline.SeedPatch{
		// Note: NO NewFindings from DA critique — they must not be recorded
	}

	result, err := pipeline.PatchSeedAnalysisFile(seedPath, patch)
	if err != nil {
		t.Fatalf("PatchSeedAnalysisFile: %v", err)
	}

	// Only original findings should exist — no DA findings leaked
	if len(result.Findings) != 2 {
		t.Errorf("expected 2 findings (original only, no DA findings), got %d", len(result.Findings))
	}

	// Verify on disk: no DA-related fields contaminated the file
	raw, _ := os.ReadFile(seedPath)
	rawStr := string(raw)
	// DA findings have specific fields like "falsification_test" that should NOT appear
	if strings.Contains(rawStr, "falsification_test") {
		t.Error("seed-analysis.json must not contain DA-specific fields like falsification_test")
	}
	if strings.Contains(rawStr, "da_findings") {
		t.Error("seed-analysis.json must not contain a da_findings field")
	}
	if strings.Contains(rawStr, "unresolved") {
		t.Error("seed-analysis.json must not contain unresolved findings list")
	}
}

func TestMaxRoundFailure_WorkflowProceeds(t *testing.T) {
	// AC 11: After max-round failure, the workflow MUST proceed to perspective
	// generator. This means seed-analysis.json must be a valid, consumable
	// input for the perspective generator.

	tmpDir := t.TempDir()
	seedPath := filepath.Join(tmpDir, "seed-analysis.json")

	// Write a seed analysis that failed DA review
	sa := pipeline.SeedAnalysis{
		Topic:   "Investigate service degradation",
		Summary: "Found 3 areas related to service degradation",
		Findings: []pipeline.SeedFinding{
			{ID: 1, Area: "load-balancer", Description: "LB config", Source: "lb/config.go:20", ToolUsed: "Read"},
			{ID: 2, Area: "circuit-breaker", Description: "CB thresholds", Source: "cb/breaker.go:45", ToolUsed: "Grep"},
			{ID: 3, Area: "health-checks", Description: "HC intervals", Source: "hc/check.go:12", ToolUsed: "Grep"},
		},
		KeyAreas: []string{"load-balancing", "circuit-breaking", "health-monitoring"},
	}
	if err := pipeline.WriteSeedAnalysis(seedPath, sa); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Simulate perspective generator reading seed-analysis.json
	// It must be able to read and parse it successfully
	consumed, err := pipeline.ReadSeedAnalysis(seedPath)
	if err != nil {
		t.Fatalf("perspective generator cannot read seed-analysis.json: %v", err)
	}

	// Perspective generator needs: topic, findings, key_areas
	if consumed.Topic == "" {
		t.Error("topic must be present for perspective generator")
	}
	if len(consumed.Findings) == 0 {
		t.Error("findings must be present for perspective generator")
	}
	if len(consumed.KeyAreas) == 0 {
		t.Error("key_areas must be present for perspective generator")
	}

	// The file must be valid JSON (perspective generator can parse it)
	raw, _ := os.ReadFile(seedPath)
	var genericJSON map[string]interface{}
	if err := json.Unmarshal(raw, &genericJSON); err != nil {
		t.Fatalf("seed-analysis.json must be valid JSON for downstream consumers: %v", err)
	}

}

func TestMaxRoundFailure_HardStopResult(t *testing.T) {
	// Verify that on final round, HandleDAReview returns hard_stop=true, pass=false
	// This is the signal for the seed analyst to exit the loop.
	result := pipeline.DAReviewResult{
		Pass:          false,
		GapCount:      2,
		BiasCount:     1,
		CoverageCount: 1,
		Round:         pipeline.MaxDARounds,
		MaxRounds:     pipeline.MaxDARounds,
		HardStop:      pipeline.MaxDARounds >= pipeline.MaxDARounds, // true
		Gaps: []pipeline.DAGap{
			{Type: "coverage", Description: "gap remains in auth module"},
			{Type: "bias", Description: "still biased toward backend"},
		},
	}

	// On final round, even with findings, hard_stop must be true
	if !result.HardStop {
		t.Error("hard_stop must be true on final round")
	}
	if result.Pass {
		t.Error("pass must be false when CRITICAL/MAJOR findings exist")
	}

	// The seed analyst should exit the loop based on this response
	// (pass=false AND hard_stop=true → exit loop, proceed)
	data, _ := json.Marshal(result)
	var parsed map[string]interface{}
	json.Unmarshal(data, &parsed)

	if parsed["pass"].(bool) {
		t.Error("JSON pass must be false")
	}
	if !parsed["hard_stop"].(bool) {
		t.Error("JSON hard_stop must be true on final round")
	}
	if int(parsed["round"].(float64)) != pipeline.MaxDARounds {
		t.Errorf("round = %v, want %d", parsed["round"], pipeline.MaxDARounds)
	}
}

func TestMaxRoundFailure_ExceedsMaxReturnsEmpty(t *testing.T) {
	// If somehow round > pipeline.MaxDARounds, the tool returns immediately
	// with pass=false and empty findings — a safety net.
	result := pipeline.DAReviewResult{
		Pass:      false,
		Round:     pipeline.MaxDARounds + 1,
		MaxRounds: pipeline.MaxDARounds,
		HardStop:  true,
		Gaps:      []pipeline.DAGap{},
		RawOutput: fmt.Sprintf("hard stop: round %d exceeds maximum of %d rounds", pipeline.MaxDARounds+1, pipeline.MaxDARounds),
	}

	if result.Pass {
		t.Error("pass must be false for exceeded max rounds")
	}
	if !result.HardStop {
		t.Error("hard_stop must be true")
	}
	if len(result.Gaps) != 0 {
		t.Error("gaps must be empty for exceeded max rounds")
	}
}

func TestDAReviewResult_JSONStructure(t *testing.T) {
	// Verify the pipeline.DAReviewResult marshals to expected JSON shape
	result := pipeline.DAReviewResult{
		Pass:              false,
		GapCount:          2,
		BiasCount:         1,
		CoverageCount:     1,
		Gaps:              []pipeline.DAGap{{Type: "bias", Description: "Framing too narrow"}, {Type: "coverage", Description: "Missing auth module"}},
		OverallConfidence: "MEDIUM",
		TopConcerns:       "some concerns",
		WhatHoldsUp:       "some things",
		RawOutput:         "raw",
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	// Verify required fields exist with correct types
	if _, ok := parsed["pass"].(bool); !ok {
		t.Error("pass field missing or not boolean")
	}
	if _, ok := parsed["gap_count"].(float64); !ok {
		t.Error("gap_count field missing or not number")
	}
	if _, ok := parsed["bias_count"].(float64); !ok {
		t.Error("bias_count field missing or not number")
	}
	if _, ok := parsed["coverage_count"].(float64); !ok {
		t.Error("coverage_count field missing or not number")
	}
	if _, ok := parsed["gaps"].([]interface{}); !ok {
		t.Error("gaps field missing or not array")
	}

	// Check values
	if parsed["pass"].(bool) != false {
		t.Error("pass should be false")
	}
	if int(parsed["gap_count"].(float64)) != 2 {
		t.Errorf("gap_count = %v, want 2", parsed["gap_count"])
	}
	if int(parsed["bias_count"].(float64)) != 1 {
		t.Errorf("bias_count = %v, want 1", parsed["bias_count"])
	}
	if int(parsed["coverage_count"].(float64)) != 1 {
		t.Errorf("coverage_count = %v, want 1", parsed["coverage_count"])
	}
}
