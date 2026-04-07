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

func TestCodexPathHelpersHonorEnvironment(t *testing.T) {
	t.Setenv("CODEX_HOME", "/tmp/custom-codex")
	t.Setenv("PRISM_CODEX_SKILLS_ROOT", "/tmp/custom-codex-skills")
	t.Setenv("PRISM_CODEX_RULES_ROOT", "/tmp/custom-codex-rules")

	if got := CodexHomePath(); got != "/tmp/custom-codex" {
		t.Fatalf("CodexHomePath() = %q, want /tmp/custom-codex", got)
	}
	if got := CodexSkillsPath(); got != "/tmp/custom-codex-skills" {
		t.Fatalf("CodexSkillsPath() = %q, want /tmp/custom-codex-skills", got)
	}
	if got := CodexRulesPath(); got != "/tmp/custom-codex-rules" {
		t.Fatalf("CodexRulesPath() = %q, want /tmp/custom-codex-rules", got)
	}
	if got := CodexRepoRootPointerPath(); got != "/tmp/custom-codex/lib/prism/repo-root" {
		t.Fatalf("CodexRepoRootPointerPath() = %q, want /tmp/custom-codex/lib/prism/repo-root", got)
	}
}
