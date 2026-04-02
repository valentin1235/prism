package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadRuntimeConfig_ReadsConfigFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	configDir := filepath.Join(home, ".prism")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}

	data := []byte(`runtime:
  backend: codex
  claude_cli_path: /custom/claude
  codex_cli_path: /custom/codex
llm:
  backend: codex
  default_model: gpt-5.4
`)
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), data, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg := LoadRuntimeConfig()
	if cfg.Runtime.Backend != "codex" {
		t.Fatalf("backend = %q, want codex", cfg.Runtime.Backend)
	}
	if cfg.Runtime.CodexCLIPath != "/custom/codex" {
		t.Fatalf("codex_cli_path = %q, want /custom/codex", cfg.Runtime.CodexCLIPath)
	}
	if cfg.LLM.DefaultModel != "gpt-5.4" {
		t.Fatalf("default_model = %q, want gpt-5.4", cfg.LLM.DefaultModel)
	}
}

func TestResolveRuntimeBackend_PrefersEnvironment(t *testing.T) {
	t.Setenv("PRISM_AGENT_RUNTIME", "codex")
	if got := ResolveRuntimeBackend(); got != "codex" {
		t.Fatalf("ResolveRuntimeBackend() = %q, want codex", got)
	}
}
