//go:build integration
// +build integration

package engine

import (
	"context"
	"testing"
	"time"
)

func TestLiveCodexScopedToolsAndSchemaCompletes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live Codex test in short mode")
	}

	t.Setenv("PRISM_CODEX_CLI_PATH", resolveCLIPath(ClaudeOptions{}))

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	result, err := QueryLLMScopedWithToolsAndSchema(
		ctx,
		t.TempDir(),
		"claude-sonnet-4-6",
		`{"type":"object","properties":{"summary":{"type":"string"}},"required":["summary"],"additionalProperties":false}`,
		"You may inspect files if needed, but keep the work minimal and return one short summary field.",
		"Return structured JSON with summary='ok'.",
		0,
	)
	if err != nil {
		t.Fatalf("QueryLLMScopedWithToolsAndSchema() error = %v", err)
	}
	if result == "" {
		t.Fatal("QueryLLMScopedWithToolsAndSchema() returned empty output")
	}
}

func TestLiveCodexScopedSchemaCompletes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live Codex schema-only test in short mode")
	}

	t.Setenv("PRISM_AGENT_RUNTIME", "codex")
	t.Setenv("PRISM_LLM_BACKEND", "codex")
	t.Setenv("PRISM_CODEX_CLI_PATH", resolveCLIPath(ClaudeOptions{}))

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	result, err := QueryLLMScopedWithSchema(
		ctx,
		t.TempDir(),
		"claude-sonnet-4-6",
		`{"type":"object","properties":{"status":{"type":"string"}},"required":["status"],"additionalProperties":false}`,
		"Return JSON with status='ok'. Do not use any tools.",
	)
	if err != nil {
		t.Fatalf("QueryLLMScopedWithSchema() error = %v", err)
	}
	if result == "" {
		t.Fatal("QueryLLMScopedWithSchema() returned empty output")
	}
}
