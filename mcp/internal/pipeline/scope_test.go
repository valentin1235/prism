package pipeline

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	taskpkg "github.com/heechul/prism-mcp/internal/task"
)

func TestBuildSeedAnalystPrompt_BasicStructure(t *testing.T) {
	prompt := BuildSeedAnalystPrompt(
		"Analyze payment processing flow",
		"analyze-abc123def456",
		"",
		"",
	)

	// Must contain the topic
	if !strings.Contains(prompt, "Analyze payment processing flow") {
		t.Error("prompt must contain the topic")
	}

	// Must contain SEED ANALYST role
	if !strings.Contains(prompt, "SEED ANALYST") {
		t.Error("prompt must identify as SEED ANALYST")
	}

	// Must contain breadth-over-depth instruction
	if !strings.Contains(prompt, "Breadth over depth") {
		t.Error("prompt must emphasize breadth over depth")
	}

	// Must contain research protocol steps
	if !strings.Contains(prompt, "Research Protocol") {
		t.Error("prompt must include research protocol")
	}

	// Must reference tool usage
	for _, tool := range []string{"Grep", "Read", "Bash", "Glob"} {
		if !strings.Contains(prompt, tool) {
			t.Errorf("prompt must reference %s tool", tool)
		}
	}

	// Must NOT contain team-related instructions
	for _, forbidden := range []string{"SendMessage", "TaskGet", "TaskUpdate", "team-lead", "team_name"} {
		if strings.Contains(prompt, forbidden) {
			t.Errorf("prompt must NOT contain team instruction %q", forbidden)
		}
	}

	// Must NOT reference ToolSearch for MCP tools
	if strings.Contains(prompt, "ToolSearch") {
		t.Error("prompt must NOT reference ToolSearch (no MCP in subprocess)")
	}

	// Must NOT reference prism_da_review
	if strings.Contains(prompt, "prism_da_review") {
		t.Error("prompt must NOT reference prism_da_review (DA handled externally)")
	}
}

func TestBuildSeedAnalystPrompt_WithSeedHints(t *testing.T) {
	prompt := BuildSeedAnalystPrompt(
		"Test topic",
		"analyze-000000000000",
		"Focus on the authentication module specifically",
		"",
	)

	if !strings.Contains(prompt, "Focus on the authentication module specifically") {
		t.Error("prompt must include seed hints when provided")
	}
	if !strings.Contains(prompt, "ADDITIONAL GUIDANCE") {
		t.Error("prompt must label seed hints as additional guidance")
	}
}

func TestBuildSeedAnalystPrompt_WithoutSeedHints(t *testing.T) {
	prompt := BuildSeedAnalystPrompt(
		"Test topic",
		"analyze-000000000000",
		"",
		"",
	)

	if strings.Contains(prompt, "ADDITIONAL GUIDANCE") {
		t.Error("prompt must NOT include guidance section when no seed hints")
	}
}

func TestBuildSeedAnalystPrompt_WithOntologyScope(t *testing.T) {
	scope := "Your reference documents:\n- doc: API docs (available)\n  Path: /docs/api"
	prompt := BuildSeedAnalystPrompt(
		"Test topic",
		"analyze-000000000000",
		"",
		scope,
	)

	if !strings.Contains(prompt, "Reference Documents") {
		t.Error("prompt must include Reference Documents section")
	}
	if !strings.Contains(prompt, scope) {
		t.Error("prompt must include the ontology scope content")
	}
}


func TestBuildPerspectiveGeneratorPrompt_BasicStructure(t *testing.T) {
	seedJSON := `{"topic":"test","summary":"test summary","findings":[],"key_areas":[]}`

	prompt := BuildPerspectiveGeneratorPrompt("Test topic", seedJSON)

	// Must contain the topic
	if !strings.Contains(prompt, "Test topic") {
		t.Error("prompt must contain the topic")
	}

	// Must identify as PERSPECTIVE GENERATOR
	if !strings.Contains(prompt, "PERSPECTIVE GENERATOR") {
		t.Error("prompt must identify as PERSPECTIVE GENERATOR")
	}

	// Must include seed analysis JSON
	if !strings.Contains(prompt, seedJSON) {
		t.Error("prompt must include seed analysis JSON content")
	}

	// Must include analyst prompt structure
	if !strings.Contains(prompt, "ROLE_NAME") {
		t.Error("prompt must include analyst prompt structure template")
	}
	if !strings.Contains(prompt, "INVESTIGATION_SCOPE") {
		t.Error("prompt must include investigation scope placeholder")
	}

	// Must include perspective quality gate requirements
	if !strings.Contains(prompt, "Quality Gate") {
		t.Error("prompt must include quality gate requirements")
	}
	if !strings.Contains(prompt, "Orthogonal") {
		t.Error("prompt must mention orthogonality check")
	}
	if !strings.Contains(prompt, "Evidence-backed") {
		t.Error("prompt must mention evidence-backed check")
	}

	// Must include perspective count guidance
	if !strings.Contains(prompt, "Minimum: 2") {
		t.Error("prompt must specify minimum 2 perspectives")
	}

	// Must NOT contain team-related instructions
	for _, forbidden := range []string{"SendMessage", "TaskGet", "TaskUpdate", "team-lead"} {
		if strings.Contains(prompt, forbidden) {
			t.Errorf("prompt must NOT contain team instruction %q", forbidden)
		}
	}

	// Must describe output format with field rules
	if !strings.Contains(prompt, "kebab-case") {
		t.Error("prompt must specify kebab-case ID format")
	}
	if !strings.Contains(prompt, "selection_summary") {
		t.Error("prompt must describe selection_summary field")
	}
}

func TestBuildPerspectiveGeneratorPrompt_TaskGenerationRules(t *testing.T) {
	prompt := BuildPerspectiveGeneratorPrompt("test", "{}")

	rules := []string{
		"Evidence-grounded",
		"Tool-oriented",
		"Specific",
		"Completeness",
		"Code-first",
	}
	for _, rule := range rules {
		if !strings.Contains(prompt, rule) {
			t.Errorf("prompt must include task generation rule: %s", rule)
		}
	}
}

func TestBuildPerspectiveGeneratorPrompt_OutputFormatRules(t *testing.T) {
	prompt := BuildPerspectiveGeneratorPrompt("test", "{}")

	rules := []string{
		"Structured",
		"Evidence-required",
		"Severity-rated",
		"Consistent",
	}
	for _, rule := range rules {
		if !strings.Contains(prompt, rule) {
			t.Errorf("prompt must include output format rule: %s", rule)
		}
	}
}

func TestSeedAnalysisSchema_ValidJSON(t *testing.T) {
	schema := SeedAnalysisSchema()

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(schema), &parsed); err != nil {
		t.Fatalf("seed analysis schema must be valid JSON: %v", err)
	}

	// Verify required fields
	required, ok := parsed["required"].([]interface{})
	if !ok {
		t.Fatal("schema must have required array")
	}
	requiredSet := make(map[string]bool)
	for _, r := range required {
		requiredSet[r.(string)] = true
	}
	for _, field := range []string{"topic", "summary", "findings", "key_areas"} {
		if !requiredSet[field] {
			t.Errorf("schema must require field %q", field)
		}
	}
}

func TestSeedAnalysisSchema_TopLevelFields(t *testing.T) {
	schema := SeedAnalysisSchema()

	var parsed map[string]interface{}
	json.Unmarshal([]byte(schema), &parsed)

	props := parsed["properties"].(map[string]interface{})

	expectedFields := []string{"topic", "summary", "findings", "key_areas"}
	for _, field := range expectedFields {
		if _, ok := props[field]; !ok {
			t.Errorf("schema must include top-level field %q", field)
		}
	}
}

func TestSeedAnalysisSchema_FindingsFields(t *testing.T) {
	schema := SeedAnalysisSchema()

	var parsed map[string]interface{}
	json.Unmarshal([]byte(schema), &parsed)

	props := parsed["properties"].(map[string]interface{})
	findings := props["findings"].(map[string]interface{})
	items := findings["items"].(map[string]interface{})
	itemProps := items["properties"].(map[string]interface{})

	expectedFields := []string{"id", "area", "description", "source", "tool_used"}
	for _, field := range expectedFields {
		if _, ok := itemProps[field]; !ok {
			t.Errorf("finding schema must include field %q", field)
		}
	}
}

func TestPerspectivesSchema_AlreadyExists(t *testing.T) {
	// Verify PerspectivesSchema() from perspectives.go is valid and available
	schema := PerspectivesSchema()
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(schema), &parsed); err != nil {
		t.Fatalf("perspectives schema must be valid JSON: %v", err)
	}
}

func TestSeedAnalysisPath(t *testing.T) {
	path := SeedAnalysisPath("/home/user/.prism/state/analyze-abc123")
	expected := filepath.Join("/home/user/.prism/state/analyze-abc123", "seed-analysis.json")
	if path != expected {
		t.Errorf("SeedAnalysisPath = %q, want %q", path, expected)
	}
}

func TestPerspectivesPath(t *testing.T) {
	path := PerspectivesPath("/home/user/.prism/state/analyze-abc123")
	expected := filepath.Join("/home/user/.prism/state/analyze-abc123", "perspectives.json")
	if path != expected {
		t.Errorf("PerspectivesPath = %q, want %q", path, expected)
	}
}

func TestLoadStage1Config(t *testing.T) {
	// Create temp state directory with config.json
	tmpDir := t.TempDir()

	config := map[string]interface{}{
		"topic":          "Test analysis topic",
		"context_id":     "analyze-test123",
		"model":          "claude-sonnet-4-6",
		"state_dir":      tmpDir,
		"seed_hints":     "Check authentication",
		"ontology_scope": "scope data here",
	}
	configBytes, _ := json.MarshalIndent(config, "", "  ")
	os.WriteFile(filepath.Join(tmpDir, "config.json"), configBytes, 0644)

	task := &taskpkg.AnalysisTask{StateDir: tmpDir}
	sc, err := LoadStage1Config(task)
	if err != nil {
		t.Fatalf("LoadStage1Config failed: %v", err)
	}

	if sc.Topic != "Test analysis topic" {
		t.Errorf("Topic = %q, want %q", sc.Topic, "Test analysis topic")
	}
	if sc.ContextID != "analyze-test123" {
		t.Errorf("ContextID = %q, want %q", sc.ContextID, "analyze-test123")
	}
	if sc.Model != "claude-sonnet-4-6" {
		t.Errorf("Model = %q, want %q", sc.Model, "claude-sonnet-4-6")
	}
	if sc.SeedHints != "Check authentication" {
		t.Errorf("SeedHints = %q, want %q", sc.SeedHints, "Check authentication")
	}
	if sc.OntologyScope != "scope data here" {
		t.Errorf("OntologyScope = %q, want %q", sc.OntologyScope, "scope data here")
	}
}

func TestLoadStage1Config_MissingFile(t *testing.T) {
	task := &taskpkg.AnalysisTask{StateDir: "/nonexistent/path"}
	_, err := LoadStage1Config(task)
	if err == nil {
		t.Error("LoadStage1Config should fail for missing config.json")
	}
}


func TestStringFromMap(t *testing.T) {
	m := map[string]interface{}{
		"key1": "value1",
		"key2": 42,
		"key3": nil,
	}

	if got := stringFromMap(m, "key1"); got != "value1" {
		t.Errorf("stringFromMap(key1) = %q, want %q", got, "value1")
	}
	if got := stringFromMap(m, "key2"); got != "" {
		t.Errorf("stringFromMap(key2) = %q, want empty (wrong type)", got)
	}
	if got := stringFromMap(m, "key3"); got != "" {
		t.Errorf("stringFromMap(key3) = %q, want empty (nil)", got)
	}
	if got := stringFromMap(m, "missing"); got != "" {
		t.Errorf("stringFromMap(missing) = %q, want empty", got)
	}
}

func TestBuildSeedAnalystPrompt_OutputInstructsJSONFormat(t *testing.T) {
	prompt := BuildSeedAnalystPrompt("test", "id", "", "")

	// Must instruct about key JSON fields for structured output
	expectedFields := []string{
		"topic",
		"summary",
		"findings",
		"key_areas",
	}
	for _, field := range expectedFields {
		if !strings.Contains(prompt, field) {
			t.Errorf("prompt must describe output field %q", field)
		}
	}
}

func TestDAHistoryPath(t *testing.T) {
	got := DAHistoryPath("/tmp/test-state")
	want := filepath.Join("/tmp/test-state", "seed-analysis-da.json")
	if got != want {
		t.Errorf("DAHistoryPath = %q, want %q", got, want)
	}
}

func TestWriteDAHistory_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	history := DAReviewHistory{
		FinalPassed: true,
		TotalRounds: 2,
		Rounds: []DAReviewRound{
			{
				Round:             1,
				Pass:              false,
				GapCount:          1,
				BiasCount:         0,
				CoverageCount:     1,
				Gaps:              []DAGap{{Type: "coverage", Description: "Missing coverage in auth module"}},
				OverallConfidence: "LOW",
				TopConcerns:       "gap in API layer",
			},
			{
				Round:             2,
				Pass:              true,
				GapCount:          0,
				BiasCount:         0,
				CoverageCount:     0,
				Gaps:              []DAGap{},
				OverallConfidence: "HIGH",
				WhatHoldsUp:       "all areas covered",
			},
		},
	}

	err := writeDAHistory(dir, history)
	if err != nil {
		t.Fatalf("writeDAHistory failed: %v", err)
	}

	// Read back and verify
	data, err := os.ReadFile(DAHistoryPath(dir))
	if err != nil {
		t.Fatalf("read DA history file: %v", err)
	}

	var loaded DAReviewHistory
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("parse DA history: %v", err)
	}

	if !loaded.FinalPassed {
		t.Error("final_passed should be true")
	}
	if loaded.TotalRounds != 2 {
		t.Errorf("total_rounds = %d, want 2", loaded.TotalRounds)
	}
	if len(loaded.Rounds) != 2 {
		t.Fatalf("rounds count = %d, want 2", len(loaded.Rounds))
	}

	// Round 1 checks
	r1 := loaded.Rounds[0]
	if r1.Pass {
		t.Error("round 1 should not pass")
	}
	if r1.GapCount != 1 {
		t.Errorf("round 1 gap_count = %d, want 1", r1.GapCount)
	}
	if r1.CoverageCount != 1 {
		t.Errorf("round 1 coverage_count = %d, want 1", r1.CoverageCount)
	}
	if len(r1.Gaps) != 1 {
		t.Errorf("round 1 gaps count = %d, want 1", len(r1.Gaps))
	}
	// Round 2 checks
	r2 := loaded.Rounds[1]
	if !r2.Pass {
		t.Error("round 2 should pass")
	}
	if r2.GapCount != 0 {
		t.Error("round 2 should have zero gap count")
	}
}

func TestWriteDAHistory_HardStop(t *testing.T) {
	dir := t.TempDir()
	history := DAReviewHistory{
		FinalPassed: false,
		TotalRounds: MaxDARounds,
		Rounds: []DAReviewRound{
			{Round: 1, Pass: false, GapCount: 2, BiasCount: 1, CoverageCount: 1},
		},
	}

	if err := writeDAHistory(dir, history); err != nil {
		t.Fatalf("writeDAHistory failed: %v", err)
	}

	data, err := os.ReadFile(DAHistoryPath(dir))
	if err != nil {
		t.Fatalf("read DA history file: %v", err)
	}

	var loaded DAReviewHistory
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("parse DA history: %v", err)
	}

	if loaded.FinalPassed {
		t.Error("final_passed should be false after hard stop")
	}
	if loaded.TotalRounds != MaxDARounds {
		t.Errorf("total_rounds = %d, want %d", loaded.TotalRounds, MaxDARounds)
	}
	if len(loaded.Rounds) != MaxDARounds {
		t.Errorf("rounds count = %d, want %d", len(loaded.Rounds), MaxDARounds)
	}
}

func TestBuildPerspectiveGeneratorPrompt_ModelSelection(t *testing.T) {
	prompt := BuildPerspectiveGeneratorPrompt("test", "{}")

	// Must include model selection guidance
	if !strings.Contains(prompt, "opus") {
		t.Error("prompt must mention opus model option")
	}
	if !strings.Contains(prompt, "sonnet") {
		t.Error("prompt must mention sonnet model option")
	}
}
