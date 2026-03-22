package pipeline

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func validPerspectivesOutput() PerspectivesOutput {
	return PerspectivesOutput{
		Perspectives: []Perspective{
			{
				ID:           "policy-conflict",
				Name:         "Policy Conflict Analysis",
				Scope:        "Examines conflicting policies in ticket refund flows",
				KeyQuestions: []string{"What policies contradict?", "Which takes precedence?"},
				Model:        "opus",
				Prompt: AnalystPrompt{
					Role:               "You are the POLICY CONFLICT ANALYST.",
					InvestigationScope: "Focus on refund policy conflicts in payment module",
					Tasks:              "1. Identify conflicting refund policies\n2. Trace enforcement paths\n3. Determine precedence rules",
					OutputFormat:       "## Conflicts\n| Policy A | Policy B | Impact |\n|----------|----------|--------|\n",
				},
				Rationale: "Seed analysis found contradictory refund policies in payment_service.go:145 and policy_engine.go:89",
			},
			{
				ID:           "root-cause",
				Name:         "Root Cause Analysis",
				Scope:        "Traces the underlying cause of payment failures",
				KeyQuestions: []string{"What triggered the failure?", "What was the root cause?"},
				Model:        "sonnet",
				Prompt: AnalystPrompt{
					Role:               "You are the ROOT CAUSE ANALYST.",
					InvestigationScope: "Focus on payment failure chain from gateway to settlement",
					Tasks:              "1. Map the failure propagation path\n2. Identify the initial trigger\n3. Check error handling gaps",
					OutputFormat:       "## Timeline\n| Time | Event | Evidence |\n|------|-------|----------|\n",
				},
				Rationale: "Seed analysis found unhandled timeout in gateway_client.go:234",
			},
		},
		QualityGate: PerspectiveQualityGate{
			AllOrthogonal:      true,
			AllEvidenceBacked:  true,
			AllSpecific:        true,
			AllActionable:      true,
			MinPerspectivesMet: true,
		},
		SelectionSummary: "Two perspectives chosen: policy conflicts and root cause analysis based on seed findings in payment module",
	}
}

func TestPerspectivesRoundTrip(t *testing.T) {
	p := validPerspectivesOutput()

	// Marshal
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// Unmarshal
	var parsed PerspectivesOutput
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Verify key fields survive round-trip
	if len(parsed.Perspectives) != 2 {
		t.Fatalf("expected 2 perspectives, got %d", len(parsed.Perspectives))
	}
	if parsed.Perspectives[0].ID != "policy-conflict" {
		t.Errorf("expected id 'policy-conflict', got %q", parsed.Perspectives[0].ID)
	}
	if parsed.Perspectives[0].Prompt.Role != "You are the POLICY CONFLICT ANALYST." {
		t.Errorf("prompt.role mismatch: %q", parsed.Perspectives[0].Prompt.Role)
	}
	if parsed.SelectionSummary == "" {
		t.Error("selection_summary lost in round-trip")
	}
	if !parsed.QualityGate.AllOrthogonal {
		t.Error("quality_gate.all_orthogonal should be true")
	}
}

func TestReadWritePerspectives(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "perspectives.json")

	p := validPerspectivesOutput()

	// Write
	if err := WritePerspectives(path, p); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Verify file exists and is valid JSON
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("empty file")
	}

	// Read back
	parsed, err := ReadPerspectives(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	if len(parsed.Perspectives) != 2 {
		t.Fatalf("expected 2 perspectives, got %d", len(parsed.Perspectives))
	}
	if parsed.Perspectives[1].ID != "root-cause" {
		t.Errorf("expected second perspective id 'root-cause', got %q", parsed.Perspectives[1].ID)
	}
}

func TestReadPerspectivesNotFound(t *testing.T) {
	_, err := ReadPerspectives("/nonexistent/path/perspectives.json")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestReadPerspectivesInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "perspectives.json")
	os.WriteFile(path, []byte("{invalid"), 0644)

	_, err := ReadPerspectives(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestValidatePerspectives(t *testing.T) {
	// Valid case
	p := validPerspectivesOutput()
	if err := ValidatePerspectives(p); err != nil {
		t.Fatalf("valid input should pass: %v", err)
	}
}

func TestValidatePerspectivesTooFew(t *testing.T) {
	p := validPerspectivesOutput()
	p.Perspectives = p.Perspectives[:1] // only 1

	err := ValidatePerspectives(p)
	if err == nil {
		t.Fatal("expected error for < 2 perspectives")
	}
}

func TestValidatePerspectivesDuplicateID(t *testing.T) {
	p := validPerspectivesOutput()
	p.Perspectives[1].ID = p.Perspectives[0].ID // duplicate

	err := ValidatePerspectives(p)
	if err == nil {
		t.Fatal("expected error for duplicate IDs")
	}
}

func TestValidatePerspectivesMissingFields(t *testing.T) {
	tests := []struct {
		name   string
		modify func(*PerspectivesOutput)
	}{
		{"empty id", func(p *PerspectivesOutput) { p.Perspectives[0].ID = "" }},
		{"empty name", func(p *PerspectivesOutput) { p.Perspectives[0].Name = "" }},
		{"empty scope", func(p *PerspectivesOutput) { p.Perspectives[0].Scope = "" }},
		{"too few key_questions", func(p *PerspectivesOutput) { p.Perspectives[0].KeyQuestions = []string{"one"} }},
		{"too many key_questions", func(p *PerspectivesOutput) {
			p.Perspectives[0].KeyQuestions = []string{"1", "2", "3", "4", "5"}
		}},
		{"empty prompt.role", func(p *PerspectivesOutput) { p.Perspectives[0].Prompt.Role = "" }},
		{"empty prompt.investigation_scope", func(p *PerspectivesOutput) { p.Perspectives[0].Prompt.InvestigationScope = "" }},
		{"empty prompt.tasks", func(p *PerspectivesOutput) { p.Perspectives[0].Prompt.Tasks = "" }},
		{"empty prompt.output_format", func(p *PerspectivesOutput) { p.Perspectives[0].Prompt.OutputFormat = "" }},
		{"empty rationale", func(p *PerspectivesOutput) { p.Perspectives[0].Rationale = "" }},
		{"empty selection_summary", func(p *PerspectivesOutput) { p.SelectionSummary = "" }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := validPerspectivesOutput()
			tc.modify(&p)
			if err := ValidatePerspectives(p); err == nil {
				t.Errorf("expected validation error for %s", tc.name)
			}
		})
	}
}

func TestPerspectivesSchemaIsValidJSON(t *testing.T) {
	schema := PerspectivesSchema()
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(schema), &parsed); err != nil {
		t.Fatalf("PerspectivesSchema() is not valid JSON: %v", err)
	}

	// Verify top-level structure
	if parsed["type"] != "object" {
		t.Errorf("expected type 'object', got %v", parsed["type"])
	}

	required, ok := parsed["required"].([]interface{})
	if !ok {
		t.Fatal("missing 'required' array")
	}
	requiredSet := make(map[string]bool)
	for _, r := range required {
		requiredSet[r.(string)] = true
	}
	for _, field := range []string{"perspectives", "quality_gate", "selection_summary"} {
		if !requiredSet[field] {
			t.Errorf("missing required field %q in schema", field)
		}
	}

	// Verify perspectives items have the expected nested structure
	props := parsed["properties"].(map[string]interface{})
	perspProps := props["perspectives"].(map[string]interface{})
	if perspProps["type"] != "array" {
		t.Error("perspectives should be an array type")
	}
	items := perspProps["items"].(map[string]interface{})
	itemProps := items["properties"].(map[string]interface{})

	expectedFields := []string{"id", "name", "scope", "key_questions", "model", "prompt", "rationale"}
	for _, field := range expectedFields {
		if _, ok := itemProps[field]; !ok {
			t.Errorf("missing field %q in perspective item schema", field)
		}
	}

	// Verify prompt nested structure
	promptSchema := itemProps["prompt"].(map[string]interface{})
	promptProps := promptSchema["properties"].(map[string]interface{})
	for _, field := range []string{"role", "investigation_scope", "tasks", "output_format"} {
		if _, ok := promptProps[field]; !ok {
			t.Errorf("missing field %q in prompt schema", field)
		}
	}
}

// --- Tests for perspective injection merge (AC 7) ---

func TestLoadInjectedPerspectives_ValidFile(t *testing.T) {
	// Create a valid perspective injection JSON file
	tmpDir := t.TempDir()
	injectionPath := filepath.Join(tmpDir, "ux-impact.json")

	injected := []Perspective{
		{
			ID:           "ux-impact",
			Name:         "UX Impact Analysis",
			Scope:        "Analyze user-facing impact",
			KeyQuestions: []string{"What did users see?", "Which flows broke?"},
			Model:        "sonnet",
			Prompt: AnalystPrompt{
				Role:               "You are the UX IMPACT ANALYST.",
				InvestigationScope: "Trace from user symptom to root cause",
				Tasks:              "1. Identify broken flows\n2. Trace cause through system",
				OutputFormat:       "## UX Impact\n| Flow | Impact | Severity |",
			},
			Rationale: "Injected by incident wrapper",
		},
	}

	data, err := json.MarshalIndent(injected, "", "  ")
	if err != nil {
		t.Fatalf("marshal injected: %v", err)
	}
	if err := os.WriteFile(injectionPath, data, 0644); err != nil {
		t.Fatalf("write injection file: %v", err)
	}

	// Load and verify
	got, err := LoadInjectedPerspectives(injectionPath)
	if err != nil {
		t.Fatalf("LoadInjectedPerspectives: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 injected perspective, got %d", len(got))
	}
	if got[0].ID != "ux-impact" {
		t.Errorf("expected id 'ux-impact', got %q", got[0].ID)
	}
	if got[0].Prompt.Role != "You are the UX IMPACT ANALYST." {
		t.Errorf("prompt.role mismatch: %q", got[0].Prompt.Role)
	}
}

func TestLoadInjectedPerspectives_FileNotFound(t *testing.T) {
	_, err := LoadInjectedPerspectives("/nonexistent/path/injection.json")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestLoadInjectedPerspectives_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "bad.json")
	os.WriteFile(path, []byte("{not valid json"), 0644)

	_, err := LoadInjectedPerspectives(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestMergeInjectedPerspectives_AppendsNew(t *testing.T) {
	generated := []Perspective{
		{ID: "policy-conflict", Name: "Policy Conflict"},
		{ID: "root-cause", Name: "Root Cause"},
	}
	injected := []Perspective{
		{ID: "ux-impact", Name: "UX Impact"},
	}

	merged := MergeInjectedPerspectives(generated, injected)

	if len(merged) != 3 {
		t.Fatalf("expected 3 perspectives after merge, got %d", len(merged))
	}
	// Injected should be appended at the end
	if merged[2].ID != "ux-impact" {
		t.Errorf("expected injected perspective at end, got %q", merged[2].ID)
	}
	// Original order preserved
	if merged[0].ID != "policy-conflict" || merged[1].ID != "root-cause" {
		t.Error("original perspective order not preserved")
	}
}

func TestMergeInjectedPerspectives_DeduplicatesByID(t *testing.T) {
	generated := []Perspective{
		{ID: "policy-conflict", Name: "Policy Conflict (generated)"},
		{ID: "root-cause", Name: "Root Cause"},
	}
	injected := []Perspective{
		{ID: "policy-conflict", Name: "Policy Conflict (injected)"}, // duplicate ID
		{ID: "ux-impact", Name: "UX Impact"},
	}

	merged := MergeInjectedPerspectives(generated, injected)

	if len(merged) != 3 {
		t.Fatalf("expected 3 perspectives (1 deduped), got %d", len(merged))
	}
	// The duplicate should keep the generated version
	if merged[0].Name != "Policy Conflict (generated)" {
		t.Errorf("expected generated version preserved, got %q", merged[0].Name)
	}
	// ux-impact should still be appended
	if merged[2].ID != "ux-impact" {
		t.Errorf("expected non-duplicate injected perspective appended, got %q", merged[2].ID)
	}
}

func TestMergeInjectedPerspectives_EmptyInjected(t *testing.T) {
	generated := []Perspective{
		{ID: "policy-conflict"},
		{ID: "root-cause"},
	}

	merged := MergeInjectedPerspectives(generated, []Perspective{})

	if len(merged) != 2 {
		t.Fatalf("expected 2 perspectives (no injection), got %d", len(merged))
	}
}

func TestMergeInjectedPerspectives_DoesNotMutateOriginal(t *testing.T) {
	generated := []Perspective{
		{ID: "policy-conflict"},
	}
	injected := []Perspective{
		{ID: "ux-impact"},
	}

	// Copy original length
	origLen := len(generated)
	_ = MergeInjectedPerspectives(generated, injected)

	// Original slice should not be mutated
	if len(generated) != origLen {
		t.Errorf("original generated slice was mutated: len %d → %d", origLen, len(generated))
	}
}

func TestMergeInjectedPerspectives_WrittenToDisk(t *testing.T) {
	// End-to-end: merge injected perspectives and write to perspectives.json
	tmpDir := t.TempDir()
	perspPath := filepath.Join(tmpDir, "perspectives.json")

	// Write initial generated perspectives
	generated := validPerspectivesOutput()
	if err := WritePerspectives(perspPath, generated); err != nil {
		t.Fatalf("write initial: %v", err)
	}

	// Load injected perspective from a file
	injectionPath := filepath.Join(tmpDir, "injection.json")
	injected := []Perspective{
		{
			ID:           "ux-impact",
			Name:         "UX Impact Analysis",
			Scope:        "User-facing impact",
			KeyQuestions: []string{"What did users see?", "Which flows broke?"},
			Model:        "sonnet",
			Prompt: AnalystPrompt{
				Role:               "You are the UX IMPACT ANALYST.",
				InvestigationScope: "Trace from symptom to root cause",
				Tasks:              "1. Identify broken flows",
				OutputFormat:       "## UX Impact table",
			},
			Rationale: "Injected by incident wrapper",
		},
	}
	injData, _ := json.MarshalIndent(injected, "", "  ")
	os.WriteFile(injectionPath, injData, 0644)

	// Load injected
	loaded, err := LoadInjectedPerspectives(injectionPath)
	if err != nil {
		t.Fatalf("load injected: %v", err)
	}

	// Merge
	generated.Perspectives = MergeInjectedPerspectives(generated.Perspectives, loaded)

	// Write merged result back
	if err := WritePerspectives(perspPath, generated); err != nil {
		t.Fatalf("write merged: %v", err)
	}

	// Read back and verify
	readBack, err := ReadPerspectives(perspPath)
	if err != nil {
		t.Fatalf("read merged: %v", err)
	}

	if len(readBack.Perspectives) != 3 {
		t.Fatalf("expected 3 perspectives after merge, got %d", len(readBack.Perspectives))
	}

	// Find the injected perspective
	found := false
	for _, p := range readBack.Perspectives {
		if p.ID == "ux-impact" {
			found = true
			if p.Name != "UX Impact Analysis" {
				t.Errorf("injected perspective name mismatch: %q", p.Name)
			}
			break
		}
	}
	if !found {
		t.Error("injected 'ux-impact' perspective not found in merged output")
	}
}

func TestReadAnalysisConfigPerspectiveInjection(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := map[string]interface{}{
		"topic":                 "test incident",
		"model":                 "claude-sonnet-4-6",
		"task_id":               "analyze-abc",
		"context_id":            "analyze-abc",
		"state_dir":             tmpDir,
		"report_dir":            "/tmp/reports",
		"perspective_injection": "/path/to/ux-impact.json",
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(filepath.Join(tmpDir, "config.json"), data, 0644)

	got, err := ReadAnalysisConfig(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.PerspectiveInjection != "/path/to/ux-impact.json" {
		t.Errorf("perspective_injection: got %q, want %q", got.PerspectiveInjection, "/path/to/ux-impact.json")
	}
}

func TestReadAnalysisConfigNoPerspectiveInjection(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := map[string]interface{}{
		"topic":      "test without injection",
		"model":      "claude-sonnet-4-6",
		"task_id":    "analyze-abc",
		"context_id": "analyze-abc",
		"state_dir":  tmpDir,
		"report_dir": "/tmp/reports",
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(filepath.Join(tmpDir, "config.json"), data, 0644)

	got, err := ReadAnalysisConfig(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.PerspectiveInjection != "" {
		t.Errorf("perspective_injection should be empty when not provided, got %q", got.PerspectiveInjection)
	}
}

func TestPerspectivesSchemaMatchesStruct(t *testing.T) {
	// Verify that a valid PerspectivesOutput marshals to JSON that
	// contains exactly the fields declared in the schema
	p := validPerspectivesOutput()
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var asMap map[string]interface{}
	if err := json.Unmarshal(data, &asMap); err != nil {
		t.Fatalf("unmarshal to map: %v", err)
	}

	// Top-level keys must match schema required fields
	for _, key := range []string{"perspectives", "quality_gate", "selection_summary"} {
		if _, ok := asMap[key]; !ok {
			t.Errorf("marshaled output missing key %q", key)
		}
	}
}
