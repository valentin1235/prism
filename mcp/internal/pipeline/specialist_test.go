package pipeline

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildSpecialistCommand_BasicStructure(t *testing.T) {
	sctx := SpecialistContext{
		Topic:             "Analyze payment processing security",
		ContextID:         "analyze-abc123def456",
		Model:             "claude-sonnet-4-6",
		StateDir:          "/tmp/test-state",
		WorkDir:           "/tmp/workspace-root",
		SeedSummary:       "Payment processing spans 3 modules with 12 findings.",
		OntologyScopeText: "Your reference documents:\n- doc: Payment API docs",
	}

	perspective := Perspective{
		ID:    "security-analysis",
		Name:  "Security Analysis",
		Scope: "Authentication and authorization in payment flows",
		KeyQuestions: []string{
			"Are payment tokens properly validated?",
			"Is there proper input sanitization?",
			"Are API keys rotated regularly?",
		},
		Prompt: AnalystPrompt{
			Role:               "You are the SECURITY ANALYST.",
			InvestigationScope: "Focus on authentication, authorization, and input validation in the payment processing pipeline.",
			Tasks:              "1. Identify all authentication entry points\n2. Check token validation logic\n3. Review input sanitization patterns",
			OutputFormat:       "## Findings\n| Finding | Evidence | Severity |\n|---------|----------|----------|\n| ... | file:func:line | HIGH/MED/LOW |",
		},
		Rationale: "Seed analysis found 3 payment modules with no apparent auth validation.",
	}

	cmd := BuildSpecialistCommand(sctx, perspective)

	// Check perspective ID
	if cmd.PerspectiveID != "security-analysis" {
		t.Errorf("PerspectiveID = %q, want %q", cmd.PerspectiveID, "security-analysis")
	}

	// Check model is propagated
	if cmd.Model != "claude-sonnet-4-6" {
		t.Errorf("Model = %q, want %q", cmd.Model, "claude-sonnet-4-6")
	}

	// Check working directory
	expectedWorkDir := "/tmp/workspace-root"
	if cmd.WorkDir != expectedWorkDir {
		t.Errorf("WorkDir = %q, want %q", cmd.WorkDir, expectedWorkDir)
	}

	// Check output path
	expectedOutput := filepath.Join("/tmp/test-state", "perspectives", "security-analysis", "findings.json")
	if cmd.OutputPath != expectedOutput {
		t.Errorf("OutputPath = %q, want %q", cmd.OutputPath, expectedOutput)
	}

	// Check MaxTurns
	if cmd.MaxTurns != 10 {
		t.Errorf("MaxTurns = %d, want 10", cmd.MaxTurns)
	}

	// Check JSONSchema is set
	if cmd.JSONSchema == "" {
		t.Error("JSONSchema must be non-empty")
	}

	// Check system prompt is non-empty
	if cmd.SystemPrompt == "" {
		t.Error("SystemPrompt must be non-empty")
	}

	// Check user prompt contains topic and perspective
	if !strings.Contains(cmd.UserPrompt, "Analyze payment processing security") {
		t.Error("UserPrompt must contain the topic")
	}
	if !strings.Contains(cmd.UserPrompt, "Security Analysis") {
		t.Error("UserPrompt must contain perspective name")
	}
}

func TestBuildSpecialistSystemPrompt_FollowsAnalystPromptStructure(t *testing.T) {
	sctx := SpecialistContext{
		Topic:             "Analyze API rate limiting",
		ContextID:         "analyze-test123",
		Model:             "claude-sonnet-4-6",
		StateDir:          "/tmp/test-state",
		WorkDir:           "/tmp/workspace-root",
		SeedSummary:       "API rate limiting implemented across 5 services.",
		OntologyScopeText: "Your reference documents:\n- doc: API Gateway docs",
	}

	perspective := Perspective{
		ID:    "rate-limit-analysis",
		Name:  "Rate Limit Analysis",
		Scope: "Rate limiting configuration and enforcement",
		KeyQuestions: []string{
			"Are rate limits consistently applied?",
			"What happens when limits are exceeded?",
		},
		Prompt: AnalystPrompt{
			Role:               "You are the RATE LIMIT ANALYST.",
			InvestigationScope: "Focus on rate limiting configuration, enforcement, and bypass vectors.",
			Tasks:              "1. Map rate limit configurations\n2. Test enforcement boundaries\n3. Check for bypass vectors",
			OutputFormat:       "## Rate Limit Findings\n| Finding | Evidence | Severity |\n...",
		},
		Rationale: "Seed found inconsistent rate limit configs.",
	}

	prompt := buildSpecialistSystemPrompt(sctx, perspective)

	// Section order: Role → CONTEXT → Reference Documents → Investigation Scope → TASKS → OUTPUT → Protocol
	sections := []struct {
		name    string
		content string
	}{
		{"Role", "You are the RATE LIMIT ANALYST."},
		{"Context", "CONTEXT:\nAPI rate limiting implemented across 5 services."},
		{"Reference Documents", "### Reference Documents"},
		{"Ontology Scope", "Your reference documents:\n- doc: API Gateway docs"},
		{"Investigation Scope", "Focus on rate limiting configuration, enforcement, and bypass vectors."},
		{"Tasks", "TASKS:\n1. Map rate limit configurations"},
		{"Output Format", "OUTPUT:\n## Rate Limit Findings"},
		{"Finding Protocol", "# Finding Protocol"},
	}

	for _, s := range sections {
		if !strings.Contains(prompt, s.content) {
			t.Errorf("prompt must contain %s section with %q", s.name, s.content)
		}
	}

	// Verify ordering: each section appears after the previous one
	lastIdx := -1
	for _, s := range sections {
		idx := strings.Index(prompt, s.content)
		if idx < 0 {
			t.Errorf("section %s not found in prompt", s.name)
			continue
		}
		if idx < lastIdx {
			t.Errorf("section %s (at %d) appears before previous section (at %d) — violates required order", s.name, idx, lastIdx)
		}
		lastIdx = idx
	}
}

func TestBuildSpecialistSystemPrompt_KeyQuestionsInProtocol(t *testing.T) {
	sctx := SpecialistContext{
		Topic:       "Test topic",
		ContextID:   "analyze-test",
		Model:       "claude-sonnet-4-6",
		StateDir:    "/tmp/test",
		SeedSummary: "Summary",
	}

	perspective := Perspective{
		ID:    "test-persp",
		Name:  "Test",
		Scope: "Test scope",
		KeyQuestions: []string{
			"Is the widget properly configured?",
			"Are there race conditions?",
			"How is error handling done?",
		},
		Prompt: AnalystPrompt{
			Role:               "You are the TEST ANALYST.",
			InvestigationScope: "Test scope",
			Tasks:              "1. Test",
			OutputFormat:       "## Test",
		},
	}

	prompt := buildSpecialistSystemPrompt(sctx, perspective)

	// All key questions must appear in the prompt
	for _, q := range perspective.KeyQuestions {
		if !strings.Contains(prompt, q) {
			t.Errorf("prompt must contain key question %q", q)
		}
	}
}

func TestBuildSpecialistSystemPrompt_NoTeamCoordination(t *testing.T) {
	sctx := SpecialistContext{
		Topic:       "Test topic",
		ContextID:   "analyze-test",
		Model:       "claude-sonnet-4-6",
		StateDir:    "/tmp/test",
		SeedSummary: "Summary",
	}

	perspective := Perspective{
		ID:           "test-persp",
		Name:         "Test",
		Scope:        "Test scope",
		KeyQuestions: []string{"Q1?", "Q2?"},
		Prompt: AnalystPrompt{
			Role:               "You are the TEST ANALYST.",
			InvestigationScope: "Test scope",
			Tasks:              "1. Test",
			OutputFormat:       "## Test",
		},
	}

	prompt := buildSpecialistSystemPrompt(sctx, perspective)

	// Must NOT contain team coordination instructions
	forbidden := []string{
		"SendMessage",
		"TaskGet",
		"TaskUpdate",
		"team-lead",
		"team_name",
		"TeamCreate",
	}
	for _, f := range forbidden {
		if strings.Contains(prompt, f) {
			t.Errorf("prompt must NOT contain team coordination instruction %q", f)
		}
	}
}

func TestBuildSpecialistSystemPrompt_NoSelfVerification(t *testing.T) {
	sctx := SpecialistContext{
		Topic:       "Test topic",
		ContextID:   "analyze-test",
		Model:       "claude-sonnet-4-6",
		StateDir:    "/tmp/test",
		SeedSummary: "Summary",
	}

	perspective := Perspective{
		ID:           "test-persp",
		Name:         "Test",
		Scope:        "Test scope",
		KeyQuestions: []string{"Q1?", "Q2?"},
		Prompt: AnalystPrompt{
			Role:               "You are the TEST ANALYST.",
			InvestigationScope: "Test scope",
			Tasks:              "1. Test",
			OutputFormat:       "## Test",
		},
	}

	prompt := buildSpecialistSystemPrompt(sctx, perspective)

	// Must instruct NOT to run self-verification
	if !strings.Contains(prompt, "Do NOT run self-verification") {
		t.Error("prompt must instruct not to run self-verification")
	}
}

func TestBuildSpecialistSystemPrompt_DataSourceConstraint(t *testing.T) {
	sctx := SpecialistContext{
		Topic:       "Test topic",
		ContextID:   "analyze-test",
		Model:       "claude-sonnet-4-6",
		StateDir:    "/tmp/test",
		SeedSummary: "Summary",
	}

	perspective := Perspective{
		ID:           "test-persp",
		Name:         "Test",
		Scope:        "Test scope",
		KeyQuestions: []string{"Q1?", "Q2?"},
		Prompt: AnalystPrompt{
			Role:               "You are the TEST ANALYST.",
			InvestigationScope: "Test scope",
			Tasks:              "1. Test",
			OutputFormat:       "## Test",
		},
	}

	prompt := buildSpecialistSystemPrompt(sctx, perspective)

	// Must contain data source constraint
	if !strings.Contains(prompt, "Data Source Constraint") {
		t.Error("prompt must contain data source constraint")
	}
	if !strings.Contains(prompt, "MUST only use data sources") {
		t.Error("prompt must enforce data source restriction")
	}
}

func TestBuildSpecialistSystemPrompt_NoOntologyScope(t *testing.T) {
	sctx := SpecialistContext{
		Topic:             "Test topic",
		ContextID:         "analyze-test",
		Model:             "claude-sonnet-4-6",
		StateDir:          "/tmp/test",
		SeedSummary:       "Summary",
		OntologyScopeText: "", // no ontology scope
	}

	perspective := Perspective{
		ID:           "test-persp",
		Name:         "Test",
		Scope:        "Test scope",
		KeyQuestions: []string{"Q1?", "Q2?"},
		Prompt: AnalystPrompt{
			Role:               "You are the TEST ANALYST.",
			InvestigationScope: "Test scope",
			Tasks:              "1. Test",
			OutputFormat:       "## Test",
		},
	}

	prompt := buildSpecialistSystemPrompt(sctx, perspective)

	// Must contain fallback message
	if !strings.Contains(prompt, "N/A") {
		t.Error("prompt must contain N/A fallback when no ontology scope")
	}
}

func TestBuildAllSpecialistCommands(t *testing.T) {
	// Create temp state directory with seed-analysis.json
	tmpDir := t.TempDir()

	// Write seed-analysis.json
	seed := SeedAnalysis{
		Topic:    "Test multi-specialist",
		Summary:  "Found 3 distinct areas for analysis.",
		Findings: []SeedFinding{{ID: 1, Area: "auth", Description: "Auth module found", Source: "auth.go:10", ToolUsed: "Grep"}},
		KeyAreas: []string{"auth", "payments"},
	}
	seedData, _ := json.MarshalIndent(seed, "", "  ")
	if err := os.WriteFile(filepath.Join(tmpDir, "seed-analysis.json"), seedData, 0644); err != nil {
		t.Fatalf("write seed: %v", err)
	}

	cfg := AnalysisConfig{
		Topic:     "Test multi-specialist",
		Model:     "claude-sonnet-4-6",
		ContextID: "analyze-test123",
		StateDir:  tmpDir,
	}

	perspectives := []Perspective{
		{
			ID:           "auth-analysis",
			Name:         "Auth Analysis",
			Scope:        "Authentication flows",
			KeyQuestions: []string{"Is auth secure?", "Are tokens validated?"},
			Prompt: AnalystPrompt{
				Role:               "You are the AUTH ANALYST.",
				InvestigationScope: "Focus on auth flows",
				Tasks:              "1. Map auth entry points",
				OutputFormat:       "## Auth Findings\n...",
			},
		},
		{
			ID:           "perf-analysis",
			Name:         "Performance Analysis",
			Scope:        "Query performance",
			KeyQuestions: []string{"Are queries optimized?", "Are there N+1 problems?"},
			Prompt: AnalystPrompt{
				Role:               "You are the PERFORMANCE ANALYST.",
				InvestigationScope: "Focus on query performance",
				Tasks:              "1. Profile slow queries",
				OutputFormat:       "## Perf Findings\n...",
			},
		},
	}

	commands, err := BuildAllSpecialistCommands(cfg, perspectives)
	if err != nil {
		t.Fatalf("BuildAllSpecialistCommands failed: %v", err)
	}

	// Should produce one command per perspective
	if len(commands) != 2 {
		t.Fatalf("got %d commands, want 2", len(commands))
	}

	// Verify perspective directories were created
	for _, p := range perspectives {
		dir := PerspectiveDir(tmpDir, p.ID)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			t.Errorf("perspective directory not created: %s", dir)
		}
	}

	// Verify each command has distinct perspective ID and work dir
	if commands[0].PerspectiveID == commands[1].PerspectiveID {
		t.Error("commands must have distinct perspective IDs")
	}
	if commands[0].WorkDir == commands[1].WorkDir {
		t.Error("commands must have distinct work directories")
	}

	// Verify seed summary is injected
	for _, cmd := range commands {
		if !strings.Contains(cmd.SystemPrompt, "Found 3 distinct areas for analysis.") {
			t.Errorf("command for %s must contain seed summary in system prompt", cmd.PerspectiveID)
		}
	}
}

func TestBuildAllSpecialistCommands_MissingSeedAnalysis(t *testing.T) {
	tmpDir := t.TempDir()
	// No seed-analysis.json written

	cfg := AnalysisConfig{
		Topic:     "Test",
		Model:     "claude-sonnet-4-6",
		ContextID: "analyze-test",
		StateDir:  tmpDir,
	}

	perspectives := []Perspective{
		{
			ID:           "test",
			Name:         "Test",
			Scope:        "Test",
			KeyQuestions: []string{"Q1?", "Q2?"},
			Prompt: AnalystPrompt{
				Role:               "You are the TEST ANALYST.",
				InvestigationScope: "Test",
				Tasks:              "1. Test",
				OutputFormat:       "## Test",
			},
		},
	}

	_, err := BuildAllSpecialistCommands(cfg, perspectives)
	if err == nil {
		t.Fatal("expected error when seed-analysis.json is missing")
	}
	if !strings.Contains(err.Error(), "seed analysis") {
		t.Errorf("error should mention seed analysis: %v", err)
	}
}

func TestSpecialistFindingsSchema_ValidJSON(t *testing.T) {
	schema := SpecialistFindingsSchema()
	if !json.Valid([]byte(schema)) {
		t.Error("SpecialistFindingsSchema must be valid JSON")
	}
}

func TestSpecialistFindings_ReadWrite(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "findings.json")

	original := SpecialistFindings{
		Analyst: "security-analysis",
		Input:   "Analyze payment security",
		Findings: []SpecialistFinding{
			{
				Finding:  "SQL injection vulnerability in payment form",
				Evidence: "payment.go:handlePayment:42 — user input passed directly to query",
				Severity: "CRITICAL",
			},
			{
				Finding:  "Missing CSRF token validation",
				Evidence: "middleware.go:validateRequest:15 — no CSRF check",
				Severity: "HIGH",
			},
		},
	}

	// Write
	if err := WriteSpecialistFindings(path, original); err != nil {
		t.Fatalf("WriteSpecialistFindings failed: %v", err)
	}

	// Read back
	loaded, err := ReadSpecialistFindings(path)
	if err != nil {
		t.Fatalf("ReadSpecialistFindings failed: %v", err)
	}

	if loaded.Analyst != original.Analyst {
		t.Errorf("Analyst = %q, want %q", loaded.Analyst, original.Analyst)
	}
	if loaded.Input != original.Input {
		t.Errorf("Input = %q, want %q", loaded.Input, original.Input)
	}
	if len(loaded.Findings) != len(original.Findings) {
		t.Fatalf("Findings count = %d, want %d", len(loaded.Findings), len(original.Findings))
	}
	if loaded.Findings[0].Severity != "CRITICAL" {
		t.Errorf("Findings[0].Severity = %q, want CRITICAL", loaded.Findings[0].Severity)
	}
}

func TestFindingsPath(t *testing.T) {
	path := FindingsPath("/home/user/.prism/state/analyze-abc123", "security-analysis")
	expected := "/home/user/.prism/state/analyze-abc123/perspectives/security-analysis/findings.json"
	if path != expected {
		t.Errorf("FindingsPath = %q, want %q", path, expected)
	}
}

func TestPerspectiveDir(t *testing.T) {
	dir := PerspectiveDir("/home/user/.prism/state/analyze-abc123", "security-analysis")
	expected := "/home/user/.prism/state/analyze-abc123/perspectives/security-analysis"
	if dir != expected {
		t.Errorf("PerspectiveDir = %q, want %q", dir, expected)
	}
}

func TestTruncateForPrompt(t *testing.T) {
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"short", 10, "short"},
		{"exactly10!", 10, "exactly10!"},
		{"this is a longer string", 10, "this is..."},
		{"", 10, ""},
		{"한국어 테스트 문자열입니다", 8, "한국어 테..."},
		{"🎉🎊🎈🎁🎂🎃🎄🎅🎆🎇", 7, "🎉🎊🎈🎁..."},
	}

	for _, tt := range tests {
		got := TruncateForPrompt(tt.input, tt.maxLen)
		if got != tt.want {
			t.Errorf("TruncateForPrompt(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
		}
	}
}

func TestLoadOntologyScopeText_MissingFile(t *testing.T) {
	tmpDir := t.TempDir()
	text := LoadOntologyScopeText(tmpDir)
	if !strings.Contains(text, "N/A") {
		t.Errorf("missing ontology scope should return N/A fallback, got: %s", text)
	}
}

func TestLoadOntologyScopeText_ValidFile(t *testing.T) {
	tmpDir := t.TempDir()

	scope := map[string]interface{}{
		"sources": []map[string]interface{}{
			{
				"id":      1,
				"type":    "doc",
				"path":    "/docs/api",
				"domain":  "API",
				"summary": "API documentation",
				"status":  "available",
				"access": map[string]interface{}{
					"tools":        []string{"Read"},
					"instructions": "Use the Read tool with offset/limit to read files in the directory.",
				},
			},
			{
				"id":     2,
				"type":   "web",
				"url":    "https://example.com",
				"domain": "Reference",
				"status": "unavailable",
				"reason": "fetch failed",
			},
		},
		"citation_format": map[string]string{
			"doc": "source:section",
			"web": "url:section",
		},
	}

	data, _ := json.MarshalIndent(scope, "", "  ")
	if err := os.WriteFile(filepath.Join(tmpDir, "ontology-scope.json"), data, 0644); err != nil {
		t.Fatalf("write ontology-scope.json: %v", err)
	}

	text := LoadOntologyScopeText(tmpDir)

	// Should contain available doc source
	if !strings.Contains(text, "API documentation") {
		t.Error("ontology scope text should contain available doc source")
	}
	if !strings.Contains(text, "/docs/api") {
		t.Error("ontology scope text should contain doc path")
	}

	// Should NOT contain unavailable source
	if strings.Contains(text, "fetch failed") {
		t.Error("ontology scope text should NOT contain unavailable source details")
	}

	// Should contain citation format
	if !strings.Contains(text, "Cite findings as:") {
		t.Error("ontology scope text should contain citation format")
	}
}

func TestLoadOntologyScopeText_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "ontology-scope.json"), []byte("not json"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	text := LoadOntologyScopeText(tmpDir)
	if !strings.Contains(text, "N/A") {
		t.Error("invalid JSON should return N/A fallback")
	}
}

func TestLoadSpecialistContext(t *testing.T) {
	tmpDir := t.TempDir()

	// Write seed-analysis.json
	seed := SeedAnalysis{
		Topic:    "Test context loading",
		Summary:  "Context summary for specialists.",
		Findings: []SeedFinding{{ID: 1, Area: "test", Description: "Test finding", Source: "test.go:1", ToolUsed: "Grep"}},
		KeyAreas: []string{"test"},
	}
	seedData, _ := json.MarshalIndent(seed, "", "  ")
	if err := os.WriteFile(filepath.Join(tmpDir, "seed-analysis.json"), seedData, 0644); err != nil {
		t.Fatalf("write seed: %v", err)
	}

	cfg := AnalysisConfig{
		Topic:     "Test context loading",
		Model:     "claude-sonnet-4-6",
		ContextID: "analyze-ctx-test",
		StateDir:  tmpDir,
	}

	ctx, err := LoadSpecialistContext(cfg)
	if err != nil {
		t.Fatalf("LoadSpecialistContext failed: %v", err)
	}

	if ctx.Topic != "Test context loading" {
		t.Errorf("Topic = %q, want %q", ctx.Topic, "Test context loading")
	}
	if ctx.ContextID != "analyze-ctx-test" {
		t.Errorf("ContextID = %q, want %q", ctx.ContextID, "analyze-ctx-test")
	}
	if ctx.Model != "claude-sonnet-4-6" {
		t.Errorf("Model = %q, want %q", ctx.Model, "claude-sonnet-4-6")
	}
	if ctx.SeedSummary != "Context summary for specialists." {
		t.Errorf("SeedSummary = %q, want %q", ctx.SeedSummary, "Context summary for specialists.")
	}
	// OntologyScopeText should have N/A fallback (no ontology-scope.json)
	if !strings.Contains(ctx.OntologyScopeText, "N/A") {
		t.Error("OntologyScopeText should have N/A fallback")
	}
}

func TestBuildSpecialistCommand_FixedModelIgnoresPerspectiveModel(t *testing.T) {
	sctx := SpecialistContext{
		Topic:       "Test",
		ContextID:   "analyze-test",
		Model:       "claude-sonnet-4-6", // fixed model
		StateDir:    "/tmp/test",
		SeedSummary: "Summary",
	}

	perspective := Perspective{
		ID:           "test-persp",
		Name:         "Test",
		Scope:        "Test scope",
		Model:        "opus", // perspective says opus
		KeyQuestions: []string{"Q1?", "Q2?"},
		Prompt: AnalystPrompt{
			Role:               "You are the TEST ANALYST.",
			InvestigationScope: "Test scope",
			Tasks:              "1. Test",
			OutputFormat:       "## Test",
		},
	}

	cmd := BuildSpecialistCommand(sctx, perspective)

	// Must use the fixed model from context, NOT the perspective model
	if cmd.Model != "claude-sonnet-4-6" {
		t.Errorf("Model = %q, want fixed model %q (perspective model should be ignored)", cmd.Model, "claude-sonnet-4-6")
	}
}

func TestBuildSpecialistSystemPrompt_ContainsToolReferences(t *testing.T) {
	sctx := SpecialistContext{
		Topic:       "Test",
		ContextID:   "analyze-test",
		Model:       "claude-sonnet-4-6",
		StateDir:    "/tmp/test",
		SeedSummary: "Summary",
	}

	perspective := Perspective{
		ID:           "test-persp",
		Name:         "Test",
		Scope:        "Test scope",
		KeyQuestions: []string{"Q1?", "Q2?"},
		Prompt: AnalystPrompt{
			Role:               "You are the TEST ANALYST.",
			InvestigationScope: "Test scope",
			Tasks:              "1. Test",
			OutputFormat:       "## Test",
		},
	}

	prompt := buildSpecialistSystemPrompt(sctx, perspective)

	// Must reference available tools
	for _, tool := range []string{"Grep", "Read", "Bash", "Glob"} {
		if !strings.Contains(prompt, tool) {
			t.Errorf("prompt must reference %s tool", tool)
		}
	}
}

func TestBuildSpecialistSystemPrompt_ContainsOutputInstructions(t *testing.T) {
	sctx := SpecialistContext{
		Topic:       "Test analysis",
		ContextID:   "analyze-test",
		Model:       "claude-sonnet-4-6",
		StateDir:    "/tmp/test",
		SeedSummary: "Summary",
	}

	perspective := Perspective{
		ID:           "my-persp",
		Name:         "My Perspective",
		Scope:        "Test scope",
		KeyQuestions: []string{"Q1?", "Q2?"},
		Prompt: AnalystPrompt{
			Role:               "You are the MY ANALYST.",
			InvestigationScope: "Test scope",
			Tasks:              "1. Test",
			OutputFormat:       "## Test",
		},
	}

	prompt := buildSpecialistSystemPrompt(sctx, perspective)

	// Must specify the perspective ID in output instructions
	if !strings.Contains(prompt, `"my-persp"`) {
		t.Error("prompt must specify perspective ID in output instructions")
	}

	// Must mention findings structure fields
	for _, field := range []string{"analyst", "input", "findings", "finding", "evidence", "severity"} {
		if !strings.Contains(prompt, field) {
			t.Errorf("prompt must reference output field %q", field)
		}
	}
}

func TestLoadOntologyScopeText_MCPQuerySource(t *testing.T) {
	tmpDir := t.TempDir()

	scope := map[string]interface{}{
		"sources": []map[string]interface{}{
			{
				"id":          1,
				"type":        "mcp_query",
				"server_name": "grafana",
				"domain":      "monitoring",
				"summary":     "Grafana monitoring dashboards",
				"status":      "available",
				"access": map[string]interface{}{
					"tools":           []string{"mcp__grafana__query_prometheus"},
					"instructions":    "Query Prometheus via grafana",
					"capabilities":    "Query metrics and logs",
					"getting_started": "Start with list_datasources",
					"error_handling":  "Note errors and continue",
				},
			},
		},
	}

	data, _ := json.MarshalIndent(scope, "", "  ")
	if err := os.WriteFile(filepath.Join(tmpDir, "ontology-scope.json"), data, 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	text := LoadOntologyScopeText(tmpDir)

	// Should contain MCP query source details
	if !strings.Contains(text, "mcp-query: grafana") {
		t.Error("should contain mcp-query with server name")
	}
	if !strings.Contains(text, "Grafana monitoring dashboards") {
		t.Error("should contain summary")
	}
	if !strings.Contains(text, "Query metrics and logs") {
		t.Error("should contain capabilities")
	}
}
