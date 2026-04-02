package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type RuntimeConfig struct {
	Runtime RuntimeSection `yaml:"runtime"`
	LLM     LLMSection     `yaml:"llm"`
}

type RuntimeSection struct {
	Backend        string `yaml:"backend"`
	ClaudeCLIPath  string `yaml:"claude_cli_path,omitempty"`
	CodexCLIPath   string `yaml:"codex_cli_path,omitempty"`
	PermissionMode string `yaml:"permission_mode,omitempty"`
}

type LLMSection struct {
	Backend      string `yaml:"backend,omitempty"`
	DefaultModel string `yaml:"default_model,omitempty"`
}

func DefaultRuntimeConfig() RuntimeConfig {
	return RuntimeConfig{
		Runtime: RuntimeSection{
			Backend:        "claude",
			ClaudeCLIPath:  "claude",
			CodexCLIPath:   "codex",
			PermissionMode: "acceptEdits",
		},
		LLM: LLMSection{
			Backend:      "claude",
			DefaultModel: "default",
		},
	}
}

func ConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".prism/config.yaml"
	}
	return filepath.Join(home, ".prism", "config.yaml")
}

func LoadRuntimeConfig() RuntimeConfig {
	cfg := DefaultRuntimeConfig()
	data, err := os.ReadFile(ConfigPath())
	if err != nil {
		return cfg
	}

	var loaded RuntimeConfig
	if err := yaml.Unmarshal(data, &loaded); err != nil {
		return cfg
	}

	if v := strings.TrimSpace(loaded.Runtime.Backend); v != "" {
		cfg.Runtime.Backend = v
	}
	if v := strings.TrimSpace(loaded.Runtime.ClaudeCLIPath); v != "" {
		cfg.Runtime.ClaudeCLIPath = v
	}
	if v := strings.TrimSpace(loaded.Runtime.CodexCLIPath); v != "" {
		cfg.Runtime.CodexCLIPath = v
	}
	if v := strings.TrimSpace(loaded.Runtime.PermissionMode); v != "" {
		cfg.Runtime.PermissionMode = v
	}
	if v := strings.TrimSpace(loaded.LLM.Backend); v != "" {
		cfg.LLM.Backend = v
	}
	if v := strings.TrimSpace(loaded.LLM.DefaultModel); v != "" {
		cfg.LLM.DefaultModel = v
	}

	return cfg
}

func ResolveRuntimeBackend() string {
	for _, candidate := range []string{
		os.Getenv("PRISM_AGENT_RUNTIME"),
		os.Getenv("PRISM_LLM_BACKEND"),
		LoadRuntimeConfig().Runtime.Backend,
		inferRuntimeBackend(),
	} {
		switch strings.ToLower(strings.TrimSpace(candidate)) {
		case "claude", "codex":
			return strings.ToLower(strings.TrimSpace(candidate))
		}
	}
	return "claude"
}

func inferRuntimeBackend() string {
	if os.Getenv("CLAUDECODE") != "" || os.Getenv("CLAUDE_CODE_ENTRYPOINT") != "" {
		return "claude"
	}
	if os.Getenv("CODEX_HOME") != "" {
		return "codex"
	}
	if _, err := exec.LookPath("codex"); err == nil {
		return "codex"
	}
	return "claude"
}
