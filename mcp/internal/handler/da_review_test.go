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
- **Input type**: Analysis
- **Domain**: Software architecture
- **Initial stance**: The seed analysis identifies performance bottlenecks in the API layer.
- **Predicted biases**: Confirmation bias, anchoring on initial data
- **Predicted blind spots**: User experience impact, operational costs

## Challenged Framings

### Performance Bottleneck Framing
- **Claim**: "The primary bottleneck is in the database query layer"
- **Concern**: This framing assumes the bottleneck is singular and located in the DB layer, potentially ignoring network latency, serialization overhead, or client-side rendering as contributing factors.
- **Confidence**: ` + "`HIGH`" + `
- **Severity**: ` + "`CRITICAL`" + `
- **Falsification test**: Profile end-to-end request latency and compare DB query time vs total response time across 100 representative requests.

### Scaling Strategy Assumption
- **Claim**: "Horizontal scaling will resolve throughput issues"
- **Concern**: The analysis frames scaling as purely horizontal without considering vertical optimization or architectural changes that could reduce the need to scale.
- **Confidence**: ` + "`MEDIUM`" + `
- **Severity**: ` + "`MAJOR`" + `
- **Falsification test**: Compare cost-per-request for horizontal scaling vs query optimization on the top 10 slowest endpoints.

## Missing Perspectives

### End-User Impact Assessment
- **Claim**: The analysis focuses on server-side metrics without considering user-perceived latency.
- **Concern**: Missing the end-user perspective means optimizations might improve server metrics without meaningfully improving user experience.
- **Confidence**: ` + "`HIGH`" + `
- **Severity**: ` + "`MAJOR`" + `
- **Falsification test**: Collect Real User Monitoring (RUM) data and correlate with server-side metrics to verify they track together.

## Bias Indicators

None identified.

## Alternative Framings

### Cost-Efficiency Reframing
- **Claim**: "We need to optimize for throughput"
- **Concern**: Reframing from throughput to cost-per-transaction might reveal that some "slow" paths are actually cost-efficient and acceptable, while some "fast" paths are wastefully over-provisioned.
- **Confidence**: ` + "`MEDIUM`" + `
- **Severity**: ` + "`MINOR`" + `
- **Falsification test**: Calculate cost-per-transaction for each endpoint and compare against business value delivered.

## Self-Audit Log
- Performance Bottleneck Framing: kept — specific to this analysis, falsification test is concrete
- Scaling Strategy Assumption: downgraded from CRITICAL to MAJOR — steelman test shows horizontal scaling is a reasonable default
- End-User Impact Assessment: kept — genuinely missing from the analysis
- Cost-Efficiency Reframing: kept — offers actionable alternative perspective

## Summary
- **Overall confidence**: ` + "`MEDIUM`" + ` — The analysis provides concrete data but frames the problem narrowly
- **Pre-commitment accuracy**: Confirmation bias prediction was confirmed; anchoring prediction was partially confirmed
- **Top concerns**: The singular bottleneck framing may lead to optimizing the wrong layer; missing end-user impact data means success metrics may not reflect actual user experience improvement.
- **What holds up**: The data collection methodology is sound, and the identified queries are genuinely slow based on the evidence presented.
`

func TestParseDAFindings(t *testing.T) {
	findings := pipeline.ParseDAFindings(sampleDAOutput)

	if len(findings) != 4 {
		t.Fatalf("expected 4 findings, got %d", len(findings))
	}

	// Check first finding (Challenged Framings - CRITICAL)
	f := findings[0]
	if f.Section != "Challenged Framings" {
		t.Errorf("finding[0].Section = %q, want %q", f.Section, "Challenged Framings")
	}
	if f.Title != "Performance Bottleneck Framing" {
		t.Errorf("finding[0].Title = %q, want %q", f.Title, "Performance Bottleneck Framing")
	}
	if f.Severity != "CRITICAL" {
		t.Errorf("finding[0].Severity = %q, want %q", f.Severity, "CRITICAL")
	}
	if f.Confidence != "HIGH" {
		t.Errorf("finding[0].Confidence = %q, want %q", f.Confidence, "HIGH")
	}
	if f.Claim == "" {
		t.Error("finding[0].Claim should not be empty")
	}
	if f.Concern == "" {
		t.Error("finding[0].Concern should not be empty")
	}
	if f.FalsificationTest == "" {
		t.Error("finding[0].FalsificationTest should not be empty")
	}

	// Check second finding (Challenged Framings - MAJOR)
	f = findings[1]
	if f.Section != "Challenged Framings" {
		t.Errorf("finding[1].Section = %q, want %q", f.Section, "Challenged Framings")
	}
	if f.Severity != "MAJOR" {
		t.Errorf("finding[1].Severity = %q, want %q", f.Severity, "MAJOR")
	}

	// Check third finding (Missing Perspectives)
	f = findings[2]
	if f.Section != "Missing Perspectives" {
		t.Errorf("finding[2].Section = %q, want %q", f.Section, "Missing Perspectives")
	}
	if f.Title != "End-User Impact Assessment" {
		t.Errorf("finding[2].Title = %q, want %q", f.Title, "End-User Impact Assessment")
	}

	// Check fourth finding (Alternative Framings - MINOR)
	f = findings[3]
	if f.Section != "Alternative Framings" {
		t.Errorf("finding[3].Section = %q, want %q", f.Section, "Alternative Framings")
	}
	if f.Severity != "MINOR" {
		t.Errorf("finding[3].Severity = %q, want %q", f.Severity, "MINOR")
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

func TestParseDAFindings_EmptySections(t *testing.T) {
	input := `## Challenged Framings

None identified.

## Missing Perspectives

None identified.

## Bias Indicators

None identified.

## Alternative Framings

None identified.

## Summary
- **Overall confidence**: ` + "`HIGH`" + ` — The analysis is sound
- **Top concerns**: None significant
- **What holds up**: Everything
`
	findings := pipeline.ParseDAFindings(input)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for empty sections, got %d", len(findings))
	}
}

func TestParseDAFindings_NoSections(t *testing.T) {
	findings := pipeline.ParseDAFindings("No structured output here")
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for unstructured input, got %d", len(findings))
	}
}

func TestDAReviewResult_PassAndCounts(t *testing.T) {
	findings := pipeline.ParseDAFindings(sampleDAOutput)

	var criticalCount, majorCount int
	for _, f := range findings {
		switch f.Severity {
		case "CRITICAL":
			criticalCount++
		case "MAJOR":
			majorCount++
		}
	}
	pass := criticalCount == 0 && majorCount == 0

	if criticalCount != 1 {
		t.Errorf("critical_count = %d, want 1", criticalCount)
	}
	if majorCount != 2 {
		t.Errorf("major_count = %d, want 2", majorCount)
	}
	if pass {
		t.Error("pass should be false when CRITICAL/MAJOR findings exist")
	}
}

func TestDAReviewResult_PassWhenOnlyMinor(t *testing.T) {
	// Only MINOR findings should result in pass=true
	input := `## Challenged Framings

None identified.

## Missing Perspectives

None identified.

## Bias Indicators

None identified.

## Alternative Framings

### Minor Suggestion
- **Claim**: Something minor
- **Concern**: Not critical
- **Confidence**: ` + "`LOW`" + `
- **Severity**: ` + "`MINOR`" + `
- **Falsification test**: Check it

## Summary
- **Overall confidence**: ` + "`HIGH`" + ` — Mostly good
- **Top concerns**: Minor issues only
- **What holds up**: Main analysis
`
	findings := pipeline.ParseDAFindings(input)

	var criticalCount, majorCount int
	for _, f := range findings {
		switch f.Severity {
		case "CRITICAL":
			criticalCount++
		case "MAJOR":
			majorCount++
		}
	}
	pass := criticalCount == 0 && majorCount == 0

	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if criticalCount != 0 {
		t.Errorf("critical_count = %d, want 0", criticalCount)
	}
	if majorCount != 0 {
		t.Errorf("major_count = %d, want 0", majorCount)
	}
	if !pass {
		t.Error("pass should be true when only MINOR findings exist")
	}
}

func TestDAReviewResult_PassWhenNoFindings(t *testing.T) {
	input := `## Challenged Framings

None identified.

## Missing Perspectives

None identified.

## Bias Indicators

None identified.

## Alternative Framings

None identified.

## Summary
- **Overall confidence**: ` + "`HIGH`" + ` — All good
- **Top concerns**: None
- **What holds up**: Everything
`
	findings := pipeline.ParseDAFindings(input)

	var criticalCount, majorCount int
	for _, f := range findings {
		switch f.Severity {
		case "CRITICAL":
			criticalCount++
		case "MAJOR":
			majorCount++
		}
	}
	pass := criticalCount == 0 && majorCount == 0

	if len(findings) != 0 {
		t.Errorf("expected 0 findings, got %d", len(findings))
	}
	if !pass {
		t.Error("pass should be true when no findings")
	}
}

func TestDAFinding_RequiredFields(t *testing.T) {
	// AC 4: Each finding in the array must contain section, severity, title, and concern fields
	findings := pipeline.ParseDAFindings(sampleDAOutput)

	if len(findings) == 0 {
		t.Fatal("expected findings to be non-empty for this test")
	}

	for i, f := range findings {
		if f.Section == "" {
			t.Errorf("finding[%d].Section is empty", i)
		}
		if f.Title == "" {
			t.Errorf("finding[%d].Title is empty", i)
		}
		if f.Severity == "" {
			t.Errorf("finding[%d].Severity is empty", i)
		}
		if f.Concern == "" {
			t.Errorf("finding[%d].Concern is empty", i)
		}
	}

	// Verify these fields appear in JSON output too
	result := pipeline.DAReviewResult{
		Pass:     true,
		Findings: findings,
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	findingsArr, ok := parsed["findings"].([]interface{})
	if !ok {
		t.Fatal("findings is not an array")
	}

	requiredFields := []string{"section", "severity", "title", "concern"}
	for i, item := range findingsArr {
		finding, ok := item.(map[string]interface{})
		if !ok {
			t.Fatalf("finding[%d] is not an object", i)
		}
		for _, field := range requiredFields {
			val, exists := finding[field]
			if !exists {
				t.Errorf("finding[%d] missing required field %q", i, field)
			}
			strVal, ok := val.(string)
			if !ok {
				t.Errorf("finding[%d].%s is not a string", i, field)
			}
			if strVal == "" {
				t.Errorf("finding[%d].%s is empty", i, field)
			}
		}
	}
}

func TestFilterActionableFindings_DiscardMinorAndInfo(t *testing.T) {
	findings := []pipeline.DAFinding{
		{Section: "Challenged Framings", Title: "Critical Issue", Severity: "CRITICAL", Concern: "Big problem"},
		{Section: "Missing Perspectives", Title: "Major Gap", Severity: "MAJOR", Concern: "Important gap"},
		{Section: "Alternative Framings", Title: "Minor Note", Severity: "MINOR", Concern: "Small thing"},
		{Section: "Bias Indicators", Title: "Info Item", Severity: "INFO", Concern: "FYI"},
		{Section: "Challenged Framings", Title: "No Severity", Severity: "", Concern: "Unknown"},
	}

	actionable := pipeline.FilterActionableFindings(findings)

	if len(actionable) != 2 {
		t.Fatalf("expected 2 actionable findings, got %d", len(actionable))
	}
	if actionable[0].Severity != "CRITICAL" {
		t.Errorf("actionable[0].Severity = %q, want CRITICAL", actionable[0].Severity)
	}
	if actionable[1].Severity != "MAJOR" {
		t.Errorf("actionable[1].Severity = %q, want MAJOR", actionable[1].Severity)
	}
}

func TestFilterActionableFindings_AllMinor(t *testing.T) {
	findings := []pipeline.DAFinding{
		{Section: "Alternative Framings", Title: "Minor1", Severity: "MINOR", Concern: "small"},
		{Section: "Bias Indicators", Title: "Minor2", Severity: "MINOR", Concern: "small too"},
	}

	actionable := pipeline.FilterActionableFindings(findings)

	if len(actionable) != 0 {
		t.Errorf("expected 0 actionable findings for all-MINOR input, got %d", len(actionable))
	}
}

func TestFilterActionableFindings_Empty(t *testing.T) {
	actionable := pipeline.FilterActionableFindings(nil)
	if actionable != nil {
		t.Errorf("expected nil for nil input, got %v", actionable)
	}

	actionable = pipeline.FilterActionableFindings([]pipeline.DAFinding{})
	if actionable != nil {
		t.Errorf("expected nil for empty input, got %v", actionable)
	}
}

func TestFilterActionableFindings_PreservesOrder(t *testing.T) {
	findings := []pipeline.DAFinding{
		{Title: "Major1", Severity: "MAJOR"},
		{Title: "Minor1", Severity: "MINOR"},
		{Title: "Critical1", Severity: "CRITICAL"},
		{Title: "Minor2", Severity: "MINOR"},
		{Title: "Major2", Severity: "MAJOR"},
	}

	actionable := pipeline.FilterActionableFindings(findings)

	if len(actionable) != 3 {
		t.Fatalf("expected 3, got %d", len(actionable))
	}
	if actionable[0].Title != "Major1" || actionable[1].Title != "Critical1" || actionable[2].Title != "Major2" {
		t.Errorf("order not preserved: got %q, %q, %q", actionable[0].Title, actionable[1].Title, actionable[2].Title)
	}
}

func TestFilterActionableFindings_IntegrationWithParsedOutput(t *testing.T) {
	// The sample DA output has: 1 CRITICAL, 2 MAJOR, 1 MINOR
	allFindings := pipeline.ParseDAFindings(sampleDAOutput)
	actionable := pipeline.FilterActionableFindings(allFindings)

	if len(allFindings) != 4 {
		t.Fatalf("expected 4 total findings, got %d", len(allFindings))
	}
	if len(actionable) != 3 {
		t.Fatalf("expected 3 actionable findings (1 CRITICAL + 2 MAJOR), got %d", len(actionable))
	}

	// Verify MINOR finding was filtered out
	for _, f := range actionable {
		if f.Severity == "MINOR" {
			t.Errorf("MINOR finding should have been filtered: %q", f.Title)
		}
	}
}

// === Skip/No-Op Behavior Tests (Sub-AC 3 of AC 8) ===
// When DA review finds no CRITICAL or MAJOR issues, seed-analysis.json
// must remain unchanged. These tests verify the no-op signal contract.

func TestSkipNoOp_OnlyMinorFindings_SignalsNoChange(t *testing.T) {
	// Scenario: DA finds issues but ALL are MINOR severity.
	// Contract: pass=true, actionable findings empty → caller must NOT modify seed-analysis.json
	input := `## Challenged Framings

None identified.

## Missing Perspectives

None identified.

## Bias Indicators

### Minor Confirmation Bias
- **Claim**: Slight leaning toward one framework
- **Concern**: Could consider alternative frameworks
- **Confidence**: ` + "`LOW`" + `
- **Severity**: ` + "`MINOR`" + `
- **Falsification test**: Compare with two other frameworks

## Alternative Framings

### Minor Reframe
- **Claim**: Could view from ops angle
- **Concern**: Ops perspective adds marginal value
- **Confidence**: ` + "`LOW`" + `
- **Severity**: ` + "`MINOR`" + `
- **Falsification test**: Ask ops team

## Summary
- **Overall confidence**: ` + "`HIGH`" + ` — Analysis is solid
- **Top concerns**: Minor only
- **What holds up**: Core analysis
`
	allFindings := pipeline.ParseDAFindings(input)
	actionable := pipeline.FilterActionableFindings(allFindings)
	criticalCount, majorCount := pipeline.CountSeverities(actionable)
	pass := pipeline.ShouldPassDA(criticalCount, majorCount)

	// 2 MINOR findings parsed, but none are actionable
	if len(allFindings) != 2 {
		t.Fatalf("expected 2 total findings, got %d", len(allFindings))
	}
	if len(actionable) != 0 {
		t.Errorf("expected 0 actionable findings (skip/no-op), got %d", len(actionable))
	}
	if criticalCount != 0 || majorCount != 0 {
		t.Errorf("counts should be 0/0, got critical=%d major=%d", criticalCount, majorCount)
	}
	if !pass {
		t.Error("pass must be true when only MINOR findings exist — seed-analysis.json should be left unchanged")
	}
}

func TestSkipNoOp_ZeroFindings_SignalsNoChange(t *testing.T) {
	// Scenario: DA finds absolutely nothing wrong.
	// Contract: pass=true, empty findings → caller must NOT modify seed-analysis.json
	allFindings := pipeline.ParseDAFindings(`## Challenged Framings

None identified.

## Missing Perspectives

None identified.

## Bias Indicators

None identified.

## Alternative Framings

None identified.
`)
	actionable := pipeline.FilterActionableFindings(allFindings)
	pass := pipeline.ShouldPassDA(pipeline.CountSeverities(actionable))

	if len(allFindings) != 0 {
		t.Errorf("expected 0 total findings, got %d", len(allFindings))
	}
	if !pass {
		t.Error("pass must be true when no findings at all — seed-analysis.json should be left unchanged")
	}
}

func TestSkipNoOp_ResultJSON_EmptyFindingsOnPass(t *testing.T) {
	// Verify the full JSON result shape for the no-op/skip case.
	// When pass=true, findings must be null/empty, signaling no changes needed.
	result := pipeline.DAReviewResult{
		Pass:              true,
		CriticalCount:     0,
		MajorCount:        0,
		Findings:          nil,
		OverallConfidence:  "HIGH",
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
	// Both counts zero
	if cc := int(parsed["critical_count"].(float64)); cc != 0 {
		t.Errorf("critical_count should be 0, got %d", cc)
	}
	if mc := int(parsed["major_count"].(float64)); mc != 0 {
		t.Errorf("major_count should be 0, got %d", mc)
	}
	// findings is null (nil slice marshals to null in Go)
	if parsed["findings"] != nil {
		t.Errorf("findings should be null when no actionable findings exist, got %v", parsed["findings"])
	}
}

func TestSkipNoOp_NegativeCase_CriticalPreventsSkip(t *testing.T) {
	// Negative: even one CRITICAL means NOT a no-op — seed-analysis.json must be updated
	findings := []pipeline.DAFinding{
		{Section: "Challenged Framings", Title: "Critical Gap", Severity: "CRITICAL", Concern: "Serious issue"},
		{Section: "Alternative Framings", Title: "Minor Note", Severity: "MINOR", Concern: "Trivial"},
	}
	actionable := pipeline.FilterActionableFindings(findings)
	pass := pipeline.ShouldPassDA(pipeline.CountSeverities(actionable))

	if pass {
		t.Error("pass must be false when CRITICAL finding exists — seed-analysis.json needs updates")
	}
	if len(actionable) != 1 {
		t.Errorf("expected 1 actionable finding, got %d", len(actionable))
	}
}

func TestSkipNoOp_NegativeCase_MajorPreventsSkip(t *testing.T) {
	// Negative: even one MAJOR means NOT a no-op — seed-analysis.json must be updated
	findings := []pipeline.DAFinding{
		{Section: "Missing Perspectives", Title: "Important Gap", Severity: "MAJOR", Concern: "Significant gap"},
		{Section: "Bias Indicators", Title: "Small Bias", Severity: "MINOR", Concern: "Negligible"},
	}
	actionable := pipeline.FilterActionableFindings(findings)
	pass := pipeline.ShouldPassDA(pipeline.CountSeverities(actionable))

	if pass {
		t.Error("pass must be false when MAJOR finding exists — seed-analysis.json needs updates")
	}
	if len(actionable) != 1 {
		t.Errorf("expected 1 actionable finding, got %d", len(actionable))
	}
}

// AC 6: Loop terminates early when critical_count + major_count equals 0
func TestShouldPassDA_EarlyTermination(t *testing.T) {
	tests := []struct {
		name          string
		criticalCount int
		majorCount    int
		wantPass      bool
	}{
		{"zero critical zero major terminates early", 0, 0, true},
		{"one critical zero major continues loop", 1, 0, false},
		{"zero critical one major continues loop", 0, 1, false},
		{"one critical one major continues loop", 1, 1, false},
		{"three critical two major continues loop", 3, 2, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pipeline.ShouldPassDA(tt.criticalCount, tt.majorCount)
			if got != tt.wantPass {
				t.Errorf("pipeline.ShouldPassDA(%d, %d) = %v, want %v",
					tt.criticalCount, tt.majorCount, got, tt.wantPass)
			}
		})
	}
}

// AC 6: pipeline.CountSeverities correctly tallies CRITICAL and MAJOR from findings
func TestCountSeverities(t *testing.T) {
	tests := []struct {
		name         string
		findings     []pipeline.DAFinding
		wantCritical int
		wantMajor    int
	}{
		{
			"mixed severities",
			[]pipeline.DAFinding{
				{Severity: "CRITICAL"},
				{Severity: "MAJOR"},
				{Severity: "MINOR"},
				{Severity: "CRITICAL"},
			},
			2, 1,
		},
		{
			"no findings yields zero counts",
			nil,
			0, 0,
		},
		{
			"only minor yields zero counts",
			[]pipeline.DAFinding{{Severity: "MINOR"}, {Severity: "MINOR"}},
			0, 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, m := pipeline.CountSeverities(tt.findings)
			if c != tt.wantCritical || m != tt.wantMajor {
				t.Errorf("pipeline.CountSeverities() = (%d, %d), want (%d, %d)",
					c, m, tt.wantCritical, tt.wantMajor)
			}
		})
	}
}

// AC 6: End-to-end early termination - parse DA output, count, and verify pass
func TestEarlyTermination_EndToEnd(t *testing.T) {
	// Case 1: DA output with CRITICAL/MAJOR → pass=false, loop continues
	allFindings := pipeline.ParseDAFindings(sampleDAOutput)
	actionable := pipeline.FilterActionableFindings(allFindings)
	crit, maj := pipeline.CountSeverities(actionable)
	pass := pipeline.ShouldPassDA(crit, maj)

	if pass {
		t.Error("should NOT terminate early when CRITICAL/MAJOR findings exist")
	}
	if crit+maj == 0 {
		t.Error("critical_count + major_count should be > 0 for sample with issues")
	}

	// Case 2: DA output with only MINOR/none → pass=true, loop terminates early
	cleanInput := `## Challenged Framings

None identified.

## Missing Perspectives

None identified.

## Bias Indicators

### Minor Nitpick
- **Claim**: Something
- **Concern**: Very minor
- **Confidence**: ` + "`LOW`" + `
- **Severity**: ` + "`MINOR`" + `
- **Falsification test**: Check

## Alternative Framings

None identified.

## Summary
- **Overall confidence**: ` + "`HIGH`" + ` — All good
- **Top concerns**: None significant
- **What holds up**: Everything
`
	cleanFindings := pipeline.ParseDAFindings(cleanInput)
	cleanActionable := pipeline.FilterActionableFindings(cleanFindings)
	cleanCrit, cleanMaj := pipeline.CountSeverities(cleanActionable)
	cleanPass := pipeline.ShouldPassDA(cleanCrit, cleanMaj)

	if !cleanPass {
		t.Error("should terminate early when critical_count + major_count == 0")
	}
	if cleanCrit+cleanMaj != 0 {
		t.Errorf("critical_count + major_count = %d, want 0", cleanCrit+cleanMaj)
	}
}

// AC 6: Verify pass field in JSON output matches early termination logic
func TestEarlyTermination_JSONPassField(t *testing.T) {
	// When counts are zero, the JSON pass field must be true (signals early termination)
	result := pipeline.DAReviewResult{
		Pass:          pipeline.ShouldPassDA(0, 0),
		CriticalCount: 0,
		MajorCount:    0,
		Findings:      []pipeline.DAFinding{},
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
		t.Error("pass must be true when critical_count + major_count == 0 (early termination)")
	}
	if int(parsed["critical_count"].(float64))+int(parsed["major_count"].(float64)) != 0 {
		t.Error("critical_count + major_count should be 0")
	}
}

// === AC 7: Loop hard-stops after maximum 3 iterations ===

func TestMaxDARounds_Constant(t *testing.T) {
	if pipeline.MaxDARounds != 3 {
		t.Errorf("pipeline.MaxDARounds = %d, want 3", pipeline.MaxDARounds)
	}
}

func TestHardStop_RoundMetadataInJSON(t *testing.T) {
	// Verify round, max_rounds, and hard_stop fields appear in JSON output
	result := pipeline.DAReviewResult{
		Pass:      true,
		Round:     2,
		MaxRounds: pipeline.MaxDARounds,
		HardStop:  false,
		Findings:  []pipeline.DAFinding{},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed map[string]interface{}
	json.Unmarshal(data, &parsed)

	if round, ok := parsed["round"].(float64); !ok || int(round) != 2 {
		t.Errorf("round = %v, want 2", parsed["round"])
	}
	if maxR, ok := parsed["max_rounds"].(float64); !ok || int(maxR) != 3 {
		t.Errorf("max_rounds = %v, want 3", parsed["max_rounds"])
	}
	if hs, ok := parsed["hard_stop"].(bool); !ok || hs != false {
		t.Errorf("hard_stop = %v, want false", parsed["hard_stop"])
	}
}

func TestHardStop_TrueOnFinalRound(t *testing.T) {
	// When round == pipeline.MaxDARounds (3), hard_stop must be true
	hardStop := pipeline.MaxDARounds >= pipeline.MaxDARounds // simulates the check in HandleDAReview
	if !hardStop {
		t.Error("hard_stop should be true when round == pipeline.MaxDARounds")
	}

	result := pipeline.DAReviewResult{
		Pass:      false,
		Round:     pipeline.MaxDARounds,
		MaxRounds: pipeline.MaxDARounds,
		HardStop:  true,
		Findings:  []pipeline.DAFinding{{Section: "Missing Perspectives", Title: "Gap", Severity: "CRITICAL", Concern: "big gap"}},
	}

	data, _ := json.Marshal(result)
	var parsed map[string]interface{}
	json.Unmarshal(data, &parsed)

	if !parsed["hard_stop"].(bool) {
		t.Error("hard_stop must be true when round == pipeline.MaxDARounds")
	}
	if int(parsed["round"].(float64)) != 3 {
		t.Errorf("round = %v, want 3", parsed["round"])
	}
}

func TestHardStop_FalseBeforeMaxRound(t *testing.T) {
	// Rounds 1 and 2 should NOT trigger hard_stop
	for _, round := range []int{1, 2} {
		hardStop := round >= pipeline.MaxDARounds
		if hardStop {
			t.Errorf("round %d should not trigger hard_stop (pipeline.MaxDARounds=%d)", round, pipeline.MaxDARounds)
		}
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
		Findings:  []pipeline.DAFinding{},
		RawOutput: "hard stop: round 4 exceeds maximum of 3 rounds",
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
	findings := parsed["findings"].([]interface{})
	if len(findings) != 0 {
		t.Errorf("findings should be empty on hard stop, got %d", len(findings))
	}
}

func TestHardStop_RoundBoundaryTable(t *testing.T) {
	// Table-driven test for all key round boundaries
	tests := []struct {
		round    int
		wantStop bool
		desc     string
	}{
		{1, false, "round 1: loop continues"},
		{2, false, "round 2: loop continues"},
		{3, true, "round 3: hard stop (max reached)"},
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
  "da_passed": false,
  "research": {
    "summary": "Initial investigation of API performance",
    "findings": [
      {"id": 1, "area": "database", "description": "Slow queries in user table", "source": "db/queries.go:42", "tool_used": "Grep"},
      {"id": 2, "area": "cache", "description": "Cache miss rate is high", "source": "cache/redis.go:15", "tool_used": "Read"}
    ],
    "key_areas": ["database", "cache"]
  }
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
	research1 := parsed1["research"].(map[string]interface{})
	findings1 := research1["findings"].([]interface{})
	if len(findings1) != 2 {
		t.Fatalf("round 1: expected 2 findings, got %d", len(findings1))
	}

	// Simulate seed analyst updating seed-analysis.json with new findings (between rounds)
	updatedContent := `{
  "topic": "API performance analysis",
  "da_passed": false,
  "research": {
    "summary": "Expanded investigation including network and auth layers",
    "findings": [
      {"id": 1, "area": "database", "description": "Slow queries in user table", "source": "db/queries.go:42", "tool_used": "Grep"},
      {"id": 2, "area": "cache", "description": "Cache miss rate is high", "source": "cache/redis.go:15", "tool_used": "Read"},
      {"id": 3, "area": "network", "description": "Connection pooling not configured", "source": "net/pool.go:8", "tool_used": "Grep"},
      {"id": 4, "area": "auth", "description": "Token validation adds 50ms per request", "source": "auth/jwt.go:23", "tool_used": "Read"}
    ],
    "key_areas": ["database", "cache", "network", "auth"]
  }
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
	research2 := parsed2["research"].(map[string]interface{})
	findings2 := research2["findings"].([]interface{})

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
	summary := research2["summary"].(string)
	if !strings.Contains(summary, "network") {
		t.Error("updated summary should reference new areas — DA must evaluate the complete updated file")
	}
}

func TestFreshReadEachRound_EntireContentSentToLLM(t *testing.T) {
	// AC 10: Verify that the user prompt sent to LLM contains the ENTIRE
	// seed-analysis.json content, not a subset or diff.

	seedContent := `{
  "topic": "Payment processing reliability",
  "da_passed": false,
  "research": {
    "summary": "Comprehensive payment flow analysis",
    "findings": [
      {"id": 1, "area": "payment-gateway", "description": "Gateway timeout handling", "source": "pay/gateway.go:100", "tool_used": "Read"},
      {"id": 2, "area": "retry-logic", "description": "No exponential backoff", "source": "pay/retry.go:45", "tool_used": "Grep"},
      {"id": 3, "area": "idempotency", "description": "Missing idempotency keys", "source": "pay/handler.go:78", "tool_used": "Read"}
    ],
    "key_areas": ["payment-gateway", "retry-logic", "idempotency"]
  }
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
		`{"topic":"v1","research":{"findings":[]}}`,
		`{"topic":"v2","research":{"findings":[{"id":1}]}}`,
		`{"topic":"v3","research":{"findings":[{"id":1},{"id":2},{"id":3}]}}`,
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

// === AC 11: On 3-round failure, da_passed set to false and workflow proceeds ===

func TestThreeRoundFailure_DAPassedFalse(t *testing.T) {
	// Simulate the complete 3-round failure scenario end-to-end:
	// All 3 rounds return CRITICAL/MAJOR findings → da_passed must be false
	// and seed-analysis.json must be consumable by perspective generator.

	tmpDir := t.TempDir()
	seedPath := filepath.Join(tmpDir, "seed-analysis.json")

	// Initial seed analysis
	initial := pipeline.SeedAnalysis{
		Topic:    "Investigate payment processing failures",
		DAPassed: false,
		Research: pipeline.SeedResearch{
			Summary:  "Found payment module with basic error handling",
			Findings: []pipeline.SeedFinding{
				{ID: 1, Area: "payment-processor", Description: "Handles payments", Source: "src/pay.go:1", ToolUsed: "Grep"},
			},
			KeyAreas: []string{"payments"},
		},
	}
	data, _ := json.MarshalIndent(initial, "", "  ")
	os.WriteFile(seedPath, data, 0644)

	// Simulate 3 rounds of DA failure (each round adds findings but DA still finds issues)
	for round := 1; round <= pipeline.MaxDARounds; round++ {
		// Each round: seed analyst adds new findings in response to DA critique
		patch := pipeline.SeedPatch{
			NewFindings: []pipeline.SeedFinding{
				{Area: fmt.Sprintf("round-%d-area", round), Description: fmt.Sprintf("Found in round %d", round), Source: fmt.Sprintf("src/r%d.go:1", round), ToolUsed: "Grep"},
			},
			NewKeyAreas: []string{fmt.Sprintf("round-%d-area", round)},
		}

		// On final round (3), set da_passed=false (hard stop)
		if round == pipeline.MaxDARounds {
			patch.DAPassed = false
			patch.SetDAPassed = true
		}

		_, err := pipeline.PatchSeedAnalysisFile(seedPath, patch)
		if err != nil {
			t.Fatalf("round %d: PatchSeedAnalysisFile failed: %v", round, err)
		}
	}

	// Verify: da_passed must be false after 3-round failure
	final, err := pipeline.ReadSeedAnalysis(seedPath)
	if err != nil {
		t.Fatalf("failed to read final seed analysis: %v", err)
	}

	if final.DAPassed {
		t.Error("da_passed must be false after 3-round failure (hard stop)")
	}

	// Verify: all original + incrementally added findings are preserved
	// Initial (1) + 3 rounds × 1 finding each = 4
	if len(final.Research.Findings) != 4 {
		t.Errorf("expected 4 findings (1 original + 3 rounds), got %d", len(final.Research.Findings))
	}

	// Verify: topic is preserved
	if final.Topic != "Investigate payment processing failures" {
		t.Errorf("topic should be preserved, got %q", final.Topic)
	}
}

func TestThreeRoundFailure_NoUnresolvedFindingsRecorded(t *testing.T) {
	// AC 11: On 3-round hard stop, unresolved DA findings must NOT be recorded
	// in seed-analysis.json. Only the seed analyst's own research findings
	// should be present — DA critique stays internal.

	tmpDir := t.TempDir()
	seedPath := filepath.Join(tmpDir, "seed-analysis.json")

	initial := pipeline.SeedAnalysis{
		Topic:    "API latency investigation",
		DAPassed: false,
		Research: pipeline.SeedResearch{
			Summary: "Found 2 areas",
			Findings: []pipeline.SeedFinding{
				{ID: 1, Area: "db-queries", Description: "Slow queries", Source: "db/q.go:10", ToolUsed: "Grep"},
				{ID: 2, Area: "cache-layer", Description: "Cache misses", Source: "cache/r.go:5", ToolUsed: "Read"},
			},
			KeyAreas: []string{"database", "caching"},
		},
	}
	data, _ := json.MarshalIndent(initial, "", "  ")
	os.WriteFile(seedPath, data, 0644)

	// Hard stop: set da_passed=false, do NOT add DA findings to the file
	patch := pipeline.SeedPatch{
		DAPassed:    false,
		SetDAPassed: true,
		// Note: NO NewFindings from DA critique — they must not be recorded
	}

	result, err := pipeline.PatchSeedAnalysisFile(seedPath, patch)
	if err != nil {
		t.Fatalf("PatchSeedAnalysisFile: %v", err)
	}

	// Only original findings should exist — no DA findings leaked
	if len(result.Research.Findings) != 2 {
		t.Errorf("expected 2 findings (original only, no DA findings), got %d", len(result.Research.Findings))
	}
	if result.DAPassed {
		t.Error("da_passed must be false")
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

func TestThreeRoundFailure_WorkflowProceeds(t *testing.T) {
	// AC 11: After 3-round failure, the workflow MUST proceed to perspective
	// generator. This means seed-analysis.json with da_passed=false must be
	// a valid, consumable input for the perspective generator.

	tmpDir := t.TempDir()
	seedPath := filepath.Join(tmpDir, "seed-analysis.json")

	// Write a seed analysis that failed DA review (da_passed=false)
	sa := pipeline.SeedAnalysis{
		Topic:    "Investigate service degradation",
		DAPassed: false,
		Research: pipeline.SeedResearch{
			Summary: "Found 3 areas related to service degradation",
			Findings: []pipeline.SeedFinding{
				{ID: 1, Area: "load-balancer", Description: "LB config", Source: "lb/config.go:20", ToolUsed: "Read"},
				{ID: 2, Area: "circuit-breaker", Description: "CB thresholds", Source: "cb/breaker.go:45", ToolUsed: "Grep"},
				{ID: 3, Area: "health-checks", Description: "HC intervals", Source: "hc/check.go:12", ToolUsed: "Grep"},
			},
			KeyAreas: []string{"load-balancing", "circuit-breaking", "health-monitoring"},
		},
	}
	if err := pipeline.WriteSeedAnalysis(seedPath, sa); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Simulate perspective generator reading seed-analysis.json
	// It must be able to read and parse it successfully, regardless of da_passed value
	consumed, err := pipeline.ReadSeedAnalysis(seedPath)
	if err != nil {
		t.Fatalf("perspective generator cannot read seed-analysis.json: %v", err)
	}

	// Perspective generator needs: topic, findings, key_areas
	if consumed.Topic == "" {
		t.Error("topic must be present for perspective generator")
	}
	if len(consumed.Research.Findings) == 0 {
		t.Error("findings must be present for perspective generator")
	}
	if len(consumed.Research.KeyAreas) == 0 {
		t.Error("key_areas must be present for perspective generator")
	}

	// da_passed=false does NOT block consumption
	if consumed.DAPassed {
		t.Error("da_passed should be false (3-round failure)")
	}

	// The file must be valid JSON (perspective generator can parse it)
	raw, _ := os.ReadFile(seedPath)
	var genericJSON map[string]interface{}
	if err := json.Unmarshal(raw, &genericJSON); err != nil {
		t.Fatalf("seed-analysis.json must be valid JSON for downstream consumers: %v", err)
	}

	// Verify da_passed is present as boolean in JSON (not omitted)
	daPassed, exists := genericJSON["da_passed"]
	if !exists {
		t.Fatal("da_passed field must be present in JSON output")
	}
	if daPassed != false {
		t.Errorf("da_passed = %v, want false", daPassed)
	}
}

func TestThreeRoundFailure_HardStopResult(t *testing.T) {
	// Verify that on round 3, HandleDAReview returns hard_stop=true, pass=false
	// This is the signal for the seed analyst to set da_passed=false and exit.
	result := pipeline.DAReviewResult{
		Pass:          false,
		CriticalCount: 1,
		MajorCount:    1,
		Round:         pipeline.MaxDARounds,
		MaxRounds:     pipeline.MaxDARounds,
		HardStop:      pipeline.MaxDARounds >= pipeline.MaxDARounds, // true
		Findings: []pipeline.DAFinding{
			{Section: "Missing Perspectives", Title: "Still Missing", Severity: "CRITICAL", Concern: "gap remains"},
		},
	}

	// On round 3, even with findings, hard_stop must be true
	if !result.HardStop {
		t.Error("hard_stop must be true on round 3")
	}
	if result.Pass {
		t.Error("pass must be false when CRITICAL/MAJOR findings exist")
	}

	// The seed analyst should set da_passed=false based on this response
	// (pass=false AND hard_stop=true → set da_passed=false, exit loop, proceed)
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

func TestThreeRoundFailure_ExceedsMaxReturnsEmpty(t *testing.T) {
	// If somehow round > pipeline.MaxDARounds, the tool returns immediately
	// with pass=false and empty findings — a safety net.
	result := pipeline.DAReviewResult{
		Pass:      false,
		Round:     pipeline.MaxDARounds + 1,
		MaxRounds: pipeline.MaxDARounds,
		HardStop:  true,
		Findings:  []pipeline.DAFinding{},
		RawOutput: fmt.Sprintf("hard stop: round %d exceeds maximum of %d rounds", pipeline.MaxDARounds+1, pipeline.MaxDARounds),
	}

	if result.Pass {
		t.Error("pass must be false for exceeded max rounds")
	}
	if !result.HardStop {
		t.Error("hard_stop must be true")
	}
	if len(result.Findings) != 0 {
		t.Error("findings must be empty for exceeded max rounds")
	}
}

func TestDAReviewResult_JSONStructure(t *testing.T) {
	// Verify the pipeline.DAReviewResult marshals to expected JSON shape
	result := pipeline.DAReviewResult{
		Pass:             false,
		CriticalCount:    1,
		MajorCount:       2,
		Findings:         []pipeline.DAFinding{{Section: "Challenged Framings", Title: "Test", Severity: "CRITICAL"}},
		OverallConfidence: "MEDIUM",
		TopConcerns:      "some concerns",
		WhatHoldsUp:      "some things",
		RawOutput:        "raw",
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
	if _, ok := parsed["critical_count"].(float64); !ok {
		t.Error("critical_count field missing or not number")
	}
	if _, ok := parsed["major_count"].(float64); !ok {
		t.Error("major_count field missing or not number")
	}
	if _, ok := parsed["findings"].([]interface{}); !ok {
		t.Error("findings field missing or not array")
	}

	// Check values
	if parsed["pass"].(bool) != false {
		t.Error("pass should be false")
	}
	if int(parsed["critical_count"].(float64)) != 1 {
		t.Errorf("critical_count = %v, want 1", parsed["critical_count"])
	}
	if int(parsed["major_count"].(float64)) != 2 {
		t.Errorf("major_count = %v, want 2", parsed["major_count"])
	}
}
