package engine

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	prismconfig "github.com/heechul/prism-mcp/internal/config"
)

type toolCapabilitySpec struct {
	Name        string
	Aliases     []string
	Description string
}

var codexToolCapabilitySpecs = []toolCapabilitySpec{
	{
		Name:        "Glob",
		Aliases:     []string{"glob"},
		Description: "discover candidate files and directories by path pattern before reading them",
	},
	{
		Name:        "Grep",
		Aliases:     []string{"grep", "rg", "ripgrep"},
		Description: "search repository text for identifiers, symbols, or error strings before deeper inspection",
	},
	{
		Name:        "Read",
		Aliases:     []string{"read", "open"},
		Description: "inspect specific files once Glob or Grep has identified relevant targets",
	},
	{
		Name:        "Bash",
		Aliases:     []string{"bash", "shell", "command"},
		Description: "run tightly scoped terminal commands when file tools are insufficient",
	},
}

var codexToolCapabilityLookup = buildToolCapabilityLookup()

func buildCLIEnv(opts ClaudeOptions) []string {
	env := FilterEnv("CLAUDECODE", "PRISM_AGENT_RUNTIME", "PRISM_LLM_BACKEND", "CLAUDE_CODE_ENTRYPOINT")
	for k, v := range opts.Env {
		env = append(env, k+"="+v)
	}
	return env
}

func commandWorkingDir(opts ClaudeOptions) string {
	if strings.TrimSpace(opts.Cwd) != "" {
		return opts.Cwd
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return cwd
}

func buildPermissionArgs(mode string) []string {
	switch strings.TrimSpace(mode) {
	case "", "default":
		return []string{"--sandbox", "read-only"}
	case "acceptEdits":
		return []string{"--full-auto"}
	case "bypassPermissions":
		return []string{"--dangerously-bypass-approvals-and-sandbox"}
	default:
		return []string{"--sandbox", "read-only"}
	}
}

func (ClaudeAdaptor) CLIPath(opts LLMRequest) string {
	for _, candidate := range []string{
		strings.TrimSpace(opts.Env["PRISM_CLAUDE_CLI_PATH"]),
		strings.TrimSpace(os.Getenv("PRISM_CLAUDE_CLI_PATH")),
		strings.TrimSpace(prismconfig.LoadRuntimeConfig().Runtime.ClaudeCLIPath),
	} {
		if candidate != "" {
			return expandPath(candidate)
		}
	}
	return "claude"
}

func (CodexAdaptor) CLIPath(opts LLMRequest) string {
	for _, candidate := range []string{
		strings.TrimSpace(opts.Env["PRISM_CODEX_CLI_PATH"]),
		strings.TrimSpace(opts.Env["PSM_CODEX_CLI_PATH"]),
		strings.TrimSpace(opts.Env["OUROBOROS_CODEX_CLI_PATH"]),
		strings.TrimSpace(os.Getenv("PRISM_CODEX_CLI_PATH")),
		strings.TrimSpace(os.Getenv("PSM_CODEX_CLI_PATH")),
		strings.TrimSpace(os.Getenv("OUROBOROS_CODEX_CLI_PATH")),
		strings.TrimSpace(prismconfig.LoadRuntimeConfig().Runtime.CodexCLIPath),
	} {
		if candidate != "" {
			return expandPath(candidate)
		}
	}
	return "codex"
}

func expandPath(path string) string {
	if path == "" {
		return path
	}
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

func normalizeModelForBackend(model, backend string) string {
	candidate := strings.TrimSpace(model)
	if candidate == "" || candidate == "default" {
		return ""
	}
	if backend == "codex" && strings.HasPrefix(candidate, "claude-") {
		return ""
	}
	return candidate
}

func resolveRuntimeBackend(opts ClaudeOptions) string {
	for _, candidate := range []string{
		strings.TrimSpace(opts.Env["PRISM_AGENT_RUNTIME"]),
		strings.TrimSpace(os.Getenv("PRISM_AGENT_RUNTIME")),
		prismconfig.ResolveRuntimeBackend(),
	} {
		switch strings.ToLower(candidate) {
		case "claude", "codex":
			return strings.ToLower(candidate)
		}
	}
	return "claude"
}

func composePrompt(prompt string, opts ClaudeOptions) string {
	parts := make([]string, 0, 4)

	if systemPrompt := strings.TrimSpace(opts.SystemPrompt); systemPrompt != "" {
		parts = append(parts, "## System Instructions\n"+systemPrompt)
	}

	if toolingGuidance := buildToolingGuidance(opts); toolingGuidance != "" {
		parts = append(parts, toolingGuidance)
	}

	if userPrompt := strings.TrimSpace(prompt); userPrompt != "" {
		parts = append(parts, userPrompt)
	}

	return strings.Join(parts, "\n\n")
}

func buildToolingGuidance(opts ClaudeOptions) string {
	allowed := canonicalToolNames(opts.AllowedTools)
	disallowed := canonicalToolNames(opts.DisallowedTools)
	if opts.AllowedTools == nil && opts.DisallowedTools == nil {
		return ""
	}

	if len(allowed) == 0 {
		return "## Tooling Guidance\nDo NOT use any tools, shell commands, or MCP calls. Respond with plain text only from the provided context."
	}

	var sb strings.Builder
	sb.WriteString("## Tooling Guidance\n")
	sb.WriteString("Honor this routing contract as if it were CLI-enforced. If a needed capability is not listed, report the constraint instead of substituting another tool or MCP server.\n")
	sb.WriteString("Prefer to solve the task using the following tool set when possible:\n")
	for _, tool := range allowed {
		sb.WriteString("- ")
		sb.WriteString(tool)
		if spec, ok := codexToolCapabilityLookup[strings.ToLower(tool)]; ok && spec.Description != "" {
			sb.WriteString(": ")
			sb.WriteString(spec.Description)
		}
		sb.WriteString("\n")
	}
	if len(disallowed) > 0 {
		sb.WriteString("Do NOT use these tools or capability routes:\n")
		for _, tool := range disallowed {
			sb.WriteString("- ")
			sb.WriteString(tool)
			sb.WriteString("\n")
		}
	}
	return strings.TrimSpace(sb.String())
}

func canonicalToolNames(tools []string) []string {
	if len(tools) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(tools))
	canonical := make([]string, 0, len(tools))
	for _, tool := range tools {
		trimmed := strings.TrimSpace(tool)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if spec, ok := codexToolCapabilityLookup[key]; ok {
			trimmed = spec.Name
			key = strings.ToLower(trimmed)
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		canonical = append(canonical, trimmed)
	}
	return canonical
}

func buildToolCapabilityLookup() map[string]toolCapabilitySpec {
	lookup := make(map[string]toolCapabilitySpec)
	for _, spec := range codexToolCapabilitySpecs {
		lookup[strings.ToLower(spec.Name)] = spec
		for _, alias := range spec.Aliases {
			lookup[strings.ToLower(alias)] = spec
		}
	}
	return lookup
}

func newTempFile(pattern string) (string, error) {
	f, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", err
	}
	name := f.Name()
	if err := f.Close(); err != nil {
		_ = os.Remove(name)
		return "", err
	}
	return name, nil
}

func readOutputFile(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func terminateProcess(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}

	_ = cmd.Process.Signal(syscall.SIGTERM)
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case err := <-done:
		return err
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		return <-done
	}
}

// Retryable error classification retained from the Claude-based implementation.
var retryableErrorPatterns = []string{
	"concurrency",
	"rate",
	"timeout",
	"overloaded",
	"temporarily",
	"empty response",
	"need retry",
}

// IsRetryableError checks whether an error message indicates a transient condition.
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	for _, pattern := range retryableErrorPatterns {
		if strings.Contains(msg, pattern) {
			return true
		}
	}
	return false
}
