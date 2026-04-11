package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"concurrency", fmt.Errorf("API concurrency limit reached"), true},
		{"rate limit", fmt.Errorf("rate limit exceeded"), true},
		{"timeout", fmt.Errorf("request timeout"), true},
		{"overloaded", fmt.Errorf("server overloaded"), true},
		{"temporarily", fmt.Errorf("service temporarily unavailable"), true},
		{"empty response", fmt.Errorf("empty response from CLI"), true},
		{"need retry", fmt.Errorf("need retry"), true},
		{"case insensitive", fmt.Errorf("API RATE LIMIT"), true},
		{"non-retryable", fmt.Errorf("invalid JSON schema"), false},
		{"permission denied", fmt.Errorf("permission denied"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsRetryableError(tt.err)
			if got != tt.want {
				t.Errorf("IsRetryableError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestBuildCLIArgs_UsesCodexExecPattern(t *testing.T) {
	t.Setenv("PRISM_AGENT_RUNTIME", "codex")
	args := buildCLIArgs(ClaudeOptions{
		Model:          "gpt-5-codex",
		PermissionMode: "acceptEdits",
		Cwd:            "/tmp/prism-state",
	}, "/tmp/out.txt", "/tmp/schema.json")

	got := strings.Join(args, " ")
	wantParts := []string{
		"exec",
		"--json",
		"--skip-git-repo-check",
		"-C /tmp/prism-state",
		"--output-last-message /tmp/out.txt",
		"--model gpt-5-codex",
		"--full-auto",
		"--output-schema /tmp/schema.json",
	}

	for _, part := range wantParts {
		if !strings.Contains(got, part) {
			t.Fatalf("command %q missing %q", got, part)
		}
	}
}

func TestResolveAdaptor_UsesExplicitRequestRuntime(t *testing.T) {
	t.Setenv("PRISM_AGENT_RUNTIME", "claude")

	got := ResolveAdaptor(LLMRequest{
		Env: map[string]string{
			"PRISM_AGENT_RUNTIME": "codex",
		},
	})

	if got.Name() != "codex" {
		t.Fatalf("ResolveAdaptor() = %q, want codex", got.Name())
	}
}

func TestResolveCLIPath_DelegatesThroughAdaptor(t *testing.T) {
	t.Setenv("PRISM_AGENT_RUNTIME", "codex")

	got := resolveCLIPath(ClaudeOptions{
		Env: map[string]string{
			"PRISM_CODEX_CLI_PATH": "/tmp/codex-cli",
		},
	})

	if got != "/tmp/codex-cli" {
		t.Fatalf("resolveCLIPath() = %q, want /tmp/codex-cli", got)
	}
}

func TestBuildCLIArgs_OmitsLegacyClaudeModelAliases(t *testing.T) {
	t.Setenv("PRISM_AGENT_RUNTIME", "codex")
	args := buildCLIArgs(ClaudeOptions{
		Model: "claude-sonnet-4-6",
		Cwd:   "/tmp/prism-state",
	}, "/tmp/out.txt", "")

	got := strings.Join(args, " ")
	if strings.Contains(got, "--model claude-sonnet-4-6") {
		t.Fatalf("command %q should not pass Claude model aliases to Codex CLI", got)
	}
}

func TestBuildPermissionArgs(t *testing.T) {
	tests := []struct {
		name string
		mode string
		want string
	}{
		{name: "default explicit", mode: "default", want: "--sandbox read-only"},
		{name: "default implicit", mode: "", want: "--sandbox read-only"},
		{name: "accept edits", mode: "acceptEdits", want: "--full-auto"},
		{name: "bypass", mode: "bypassPermissions", want: "--dangerously-bypass-approvals-and-sandbox"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := strings.Join(buildPermissionArgs(tt.mode), " ")
			if got != tt.want {
				t.Fatalf("buildPermissionArgs(%q) = %q, want %q", tt.mode, got, tt.want)
			}
		})
	}
}

func TestBuildCLIEnv_FiltersRecursiveRuntimeVars(t *testing.T) {
	t.Setenv("CLAUDECODE", "1")
	t.Setenv("CLAUDE_CODE_ENTRYPOINT", "1")
	t.Setenv("PRISM_AGENT_RUNTIME", "codex")
	t.Setenv("PRISM_LLM_BACKEND", "codex")
	t.Setenv("KEEP_ME", "yes")

	env := buildCLIEnv(ClaudeOptions{
		Env: map[string]string{"EXTRA": "value"},
	})
	joined := strings.Join(env, "\n")

	for _, forbidden := range []string{"CLAUDECODE=1", "CLAUDE_CODE_ENTRYPOINT=1", "PRISM_AGENT_RUNTIME=codex", "PRISM_LLM_BACKEND=codex"} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("filtered env still contains %q", forbidden)
		}
	}
	for _, required := range []string{"KEEP_ME=yes", "EXTRA=value"} {
		if !strings.Contains(joined, required) {
			t.Fatalf("filtered env missing %q", required)
		}
	}
}

func TestResolveCLIPath_PrefersConfiguredEnvironment(t *testing.T) {
	t.Setenv("PRISM_AGENT_RUNTIME", "codex")
	t.Setenv("PRISM_CODEX_CLI_PATH", "/env/prism-codex")
	if got := resolveCLIPath(ClaudeOptions{}); got != "/env/prism-codex" {
		t.Fatalf("resolveCLIPath() = %q, want %q", got, "/env/prism-codex")
	}

	got := resolveCLIPath(ClaudeOptions{
		Env: map[string]string{
			"PRISM_CODEX_CLI_PATH": "/opts/prism-codex",
		},
	})
	if got != "/opts/prism-codex" {
		t.Fatalf("resolveCLIPath(opts.Env) = %q, want %q", got, "/opts/prism-codex")
	}
}

func TestBuildCLIArgs_UsesClaudePatternWhenConfigured(t *testing.T) {
	t.Setenv("PRISM_AGENT_RUNTIME", "claude")
	args := buildCLIArgs(ClaudeOptions{
		Model:          "claude-sonnet-4-6",
		PermissionMode: "acceptEdits",
		SystemPrompt:   "Follow the protocol.",
		JSONSchema:     `{"type":"object"}`,
		AllowedTools:   []string{"Read", "Grep"},
		Cwd:            "/tmp/prism-state",
	}, "", "")

	got := strings.Join(args, " ")
	for _, part := range []string{
		"--print",
		"--output-format stream-json",
		"--verbose",
		"--model claude-sonnet-4-6",
		"--permission-mode acceptEdits",
		"--json-schema {\"type\":\"object\"}",
		"--system-prompt Follow the protocol.",
		"--allowedTools Read,Grep",
	} {
		if !strings.Contains(got, part) {
			t.Fatalf("command %q missing %q", got, part)
		}
	}
}

func TestResolveCLIPath_PrefersClaudeConfigWhenConfigured(t *testing.T) {
	t.Setenv("PRISM_AGENT_RUNTIME", "claude")
	t.Setenv("PRISM_CLAUDE_CLI_PATH", "/env/prism-claude")
	if got := resolveCLIPath(ClaudeOptions{}); got != "/env/prism-claude" {
		t.Fatalf("resolveCLIPath() = %q, want %q", got, "/env/prism-claude")
	}
}

func TestComposePrompt_IncludesSystemToolsAndBudget(t *testing.T) {
	got := composePrompt("Investigate the failure.", ClaudeOptions{
		SystemPrompt:    "Follow the shared protocol exactly.",
		AllowedTools:    []string{"Read", "grep", "Bash"},
		DisallowedTools: []string{"MCP", "WebFetch"},
		MaxTurns:        3,
	})

	for _, needle := range []string{
		"## System Instructions\nFollow the shared protocol exactly.",
		"## Tooling Guidance",
		"Honor this routing contract as if it were CLI-enforced.",
		"- Read: inspect specific files once Glob or Grep has identified relevant targets",
		"- Grep: search repository text for identifiers, symbols, or error strings before deeper inspection",
		"- Bash: run tightly scoped terminal commands when file tools are insufficient",
		"Do NOT use these tools or capability routes:",
		"- MCP",
		"- WebFetch",
		"## Execution Budget\nKeep the work within at most 3 tool-assisted turns if possible.",
		"Investigate the failure.",
	} {
		if !strings.Contains(got, needle) {
			t.Fatalf("composePrompt() missing %q in:\n%s", needle, got)
		}
	}
}

func TestComposePrompt_NoToolsBlocksCommandsAndMCP(t *testing.T) {
	got := composePrompt("Summarize the report.", ClaudeOptions{
		AllowedTools: []string{},
	})

	want := "## Tooling Guidance\nDo NOT use any tools, shell commands, or MCP calls. Respond with plain text only from the provided context."
	if !strings.Contains(got, want) {
		t.Fatalf("composePrompt() missing no-tools contract %q in:\n%s", want, got)
	}
}

func TestTranslateCodexEvent_ProjectsAssistantMessages(t *testing.T) {
	raw := json.RawMessage(`{"type":"item.completed","item":{"type":"agent_message","content":[{"type":"output_text","text":"done"}]}}`)
	translated, ok := translateCodexEvent(raw, "thread-123")
	if !ok {
		t.Fatal("expected translated event")
	}

	var msg AssistantStreamMsg
	if err := json.Unmarshal(translated, &msg); err != nil {
		t.Fatalf("unmarshal translated event: %v", err)
	}
	if msg.Type != "assistant" {
		t.Fatalf("Type = %q, want assistant", msg.Type)
	}
	if msg.SessionID != "thread-123" {
		t.Fatalf("SessionID = %q, want thread-123", msg.SessionID)
	}
	if len(msg.Message.Content) != 1 || msg.Message.Content[0].Text != "done" {
		t.Fatalf("unexpected content: %+v", msg.Message.Content)
	}
}

func TestMakeLegacyResultMessage_ProjectsTerminalResult(t *testing.T) {
	raw, ok := makeLegacyResultMessage(nil, "thread-123", "{\"ok\":true}")
	if !ok {
		t.Fatal("expected result message")
	}

	var msg ResultMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		t.Fatalf("unmarshal result message: %v", err)
	}
	if msg.Type != "result" || msg.Subtype != "success" || msg.IsError {
		t.Fatalf("unexpected result envelope: %+v", msg)
	}
	if msg.SessionID != "thread-123" {
		t.Fatalf("SessionID = %q, want thread-123", msg.SessionID)
	}
	if msg.Result != "{\"ok\":true}" {
		t.Fatalf("Result = %q, want structured output content", msg.Result)
	}
}

func TestQuerySync_UsesCodexExecOutputForPipelineStructuredResults(t *testing.T) {
	tmpDir := t.TempDir()
	workDir := filepath.Join(tmpDir, "workdir")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("mkdir workdir: %v", err)
	}
	argsPath := filepath.Join(tmpDir, "args.txt")
	stdinPath := filepath.Join(tmpDir, "stdin.txt")
	outputPathFile := filepath.Join(tmpDir, "output-path.txt")
	schemaPathFile := filepath.Join(tmpDir, "schema-path.txt")
	schemaContentsFile := filepath.Join(tmpDir, "schema-contents.json")

	fakeCodexPath := filepath.Join(tmpDir, "codex")
	fakeCodex := `#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$@" > "` + argsPath + `"
cat > "` + stdinPath + `"

output_path=""
schema_path=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    --output-last-message)
      output_path="$2"
      shift 2
      ;;
    --output-schema)
      schema_path="$2"
      shift 2
      ;;
    *)
      shift
      ;;
  esac
done

printf '%s' "$output_path" > "` + outputPathFile + `"
printf '%s' "$schema_path" > "` + schemaPathFile + `"
if [ -n "$schema_path" ]; then
  cp "$schema_path" "` + schemaContentsFile + `"
fi
printf '{"type":"item.completed","item":{"type":"agent_message","content":[{"type":"output_text","text":"streamed fallback"}]}}'"\n"
printf '{"ok":true,"items":[1,2]}' > "$output_path"
`
	if err := os.WriteFile(fakeCodexPath, []byte(fakeCodex), 0o755); err != nil {
		t.Fatalf("write fake codex: %v", err)
	}

	result, err := QuerySync(context.Background(), "Analyze this Prism task.", ClaudeOptions{
		SystemPrompt:    "Follow the shared Prism workflow.",
		AllowedTools:    []string{"Read", "Glob", "Bash"},
		DisallowedTools: []string{"MCP", "ToolSearch"},
		PermissionMode:  "acceptEdits",
		Cwd:             workDir,
		JSONSchema:      `{"type":"object"}`,
		Env: map[string]string{
			"PRISM_CODEX_CLI_PATH": fakeCodexPath,
		},
	})
	if err != nil {
		t.Fatalf("QuerySync() error = %v", err)
	}
	if result != `{"ok":true,"items":[1,2]}` {
		t.Fatalf("QuerySync() = %q, want output-last-message content", result)
	}

	argsOutput, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatalf("read args: %v", err)
	}
	argsText := string(argsOutput)
	for _, needle := range []string{
		"exec",
		"--json",
		"--skip-git-repo-check",
		"-C",
		workDir,
		"--output-last-message",
		"--output-schema",
		"--full-auto",
	} {
		if !strings.Contains(argsText, needle) {
			t.Fatalf("expected %q in codex args\n%s", needle, argsText)
		}
	}

	stdinOutput, err := os.ReadFile(stdinPath)
	if err != nil {
		t.Fatalf("read stdin: %v", err)
	}
	stdinText := string(stdinOutput)
	for _, needle := range []string{
		"## System Instructions\nFollow the shared Prism workflow.",
		"## Tooling Guidance",
		"Honor this routing contract as if it were CLI-enforced.",
		"- Read: inspect specific files once Glob or Grep has identified relevant targets",
		"- Glob: discover candidate files and directories by path pattern before reading them",
		"- Bash: run tightly scoped terminal commands when file tools are insufficient",
		"Do NOT use these tools or capability routes:",
		"- MCP",
		"- ToolSearch",
		"Analyze this Prism task.",
	} {
		if !strings.Contains(stdinText, needle) {
			t.Fatalf("expected %q in composed prompt\n%s", needle, stdinText)
		}
	}

	outputPathBytes, err := os.ReadFile(outputPathFile)
	if err != nil {
		t.Fatalf("read output path: %v", err)
	}
	if strings.TrimSpace(string(outputPathBytes)) == "" {
		t.Fatal("expected QuerySync to pass an output-last-message path to codex exec")
	}

	schemaPathBytes, err := os.ReadFile(schemaPathFile)
	if err != nil {
		t.Fatalf("read schema path: %v", err)
	}
	schemaPath := strings.TrimSpace(string(schemaPathBytes))
	if schemaPath == "" {
		t.Fatal("expected QuerySync to pass an output schema path to codex exec")
	}

	schemaContent, err := os.ReadFile(schemaContentsFile)
	if err != nil {
		t.Fatalf("read schema snapshot: %v", err)
	}
	if string(schemaContent) != `{"type":"object"}` {
		t.Fatalf("schema file = %q, want exact JSON schema", string(schemaContent))
	}
}

func TestQueryLLMScopedWithSystemPrompt_UsesNoToolsContract(t *testing.T) {
	tmpDir := t.TempDir()
	workDir := filepath.Join(tmpDir, "workdir")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("mkdir workdir: %v", err)
	}
	stdinPath := filepath.Join(tmpDir, "stdin.txt")
	fakeCodexPath := filepath.Join(tmpDir, "codex")
	fakeCodex := `#!/usr/bin/env bash
set -euo pipefail
cat > "` + stdinPath + `"

output_path=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    --output-last-message)
      output_path="$2"
      shift 2
      ;;
    *)
      shift
      ;;
  esac
done

printf 'summary only' > "$output_path"
`
	if err := os.WriteFile(fakeCodexPath, []byte(fakeCodex), 0o755); err != nil {
		t.Fatalf("write fake codex: %v", err)
	}
	t.Setenv("PRISM_CODEX_CLI_PATH", fakeCodexPath)

	result, err := QueryLLMScopedWithSystemPrompt(
		context.Background(),
		workDir,
		"gpt-5-codex",
		"Follow the DA protocol.",
		"Review this seed analysis.",
	)
	if err != nil {
		t.Fatalf("QueryLLMScopedWithSystemPrompt() error = %v", err)
	}
	if result != "summary only" {
		t.Fatalf("QueryLLMScopedWithSystemPrompt() = %q, want summary only", result)
	}

	stdinOutput, err := os.ReadFile(stdinPath)
	if err != nil {
		t.Fatalf("read stdin: %v", err)
	}
	stdinText := string(stdinOutput)
	want := "Do NOT use any tools, shell commands, or MCP calls. Respond with plain text only from the provided context."
	if !strings.Contains(stdinText, want) {
		t.Fatalf("expected no-tools contract in composed prompt\n%s", stdinText)
	}
}

func TestQuerySync_FallsBackToStreamedAssistantTextWhenOutputFileMissing(t *testing.T) {
	tmpDir := t.TempDir()
	workDir := filepath.Join(tmpDir, "workdir")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("mkdir workdir: %v", err)
	}
	fakeCodexPath := filepath.Join(tmpDir, "codex")
	fakeCodex := `#!/usr/bin/env bash
set -euo pipefail
cat >/dev/null
printf '{"type":"item.completed","item":{"type":"agent_message","content":[{"type":"output_text","text":"assistant fallback text"}]}}'"\n"
`
	if err := os.WriteFile(fakeCodexPath, []byte(fakeCodex), 0o755); err != nil {
		t.Fatalf("write fake codex: %v", err)
	}

	result, err := QuerySync(context.Background(), "Inspect the analysis result.", ClaudeOptions{
		Cwd: workDir,
		Env: map[string]string{
			"PRISM_CODEX_CLI_PATH": fakeCodexPath,
		},
	})
	if err != nil {
		t.Fatalf("QuerySync() error = %v", err)
	}
	if result != "assistant fallback text" {
		t.Fatalf("QuerySync() = %q, want streamed assistant fallback", result)
	}
}

func TestQuery_StreamsLegacyAssistantAndResultMessagesFromCodexExec(t *testing.T) {
	tmpDir := t.TempDir()
	workDir := filepath.Join(tmpDir, "workdir")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("mkdir workdir: %v", err)
	}

	fakeCodexPath := filepath.Join(tmpDir, "codex")
	fakeCodex := `#!/usr/bin/env bash
set -euo pipefail
cat >/dev/null

output_path=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    --output-last-message)
      output_path="$2"
      shift 2
      ;;
    *)
      shift
      ;;
  esac
done

printf '{"type":"thread.started","thread_id":"thread-abc"}'"\n"
printf '{"type":"item.completed","item":{"type":"agent_message","content":[{"type":"output_text","text":"stream update"}]}}'"\n"
printf '{"ok":true}' > "$output_path"
`
	if err := os.WriteFile(fakeCodexPath, []byte(fakeCodex), 0o755); err != nil {
		t.Fatalf("write fake codex: %v", err)
	}

	ch, cleanup, err := Query(context.Background(), "Run the specialist flow.", ClaudeOptions{
		Cwd: workDir,
		Env: map[string]string{
			"PRISM_CODEX_CLI_PATH": fakeCodexPath,
		},
	})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	defer cleanup()

	var got []json.RawMessage
	for msg := range ch {
		got = append(got, msg)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 streamed messages, got %d", len(got))
	}

	var assistant AssistantStreamMsg
	if err := json.Unmarshal(got[0], &assistant); err != nil {
		t.Fatalf("unmarshal assistant message: %v", err)
	}
	if assistant.Type != "assistant" {
		t.Fatalf("assistant Type = %q, want assistant", assistant.Type)
	}
	if assistant.SessionID != "thread-abc" {
		t.Fatalf("assistant SessionID = %q, want thread-abc", assistant.SessionID)
	}
	if len(assistant.Message.Content) != 1 || assistant.Message.Content[0].Text != "stream update" {
		t.Fatalf("unexpected assistant content: %+v", assistant.Message.Content)
	}

	var result ResultMessage
	if err := json.Unmarshal(got[1], &result); err != nil {
		t.Fatalf("unmarshal result message: %v", err)
	}
	if result.Type != "result" || result.Subtype != "success" || result.IsError {
		t.Fatalf("unexpected result envelope: %+v", result)
	}
	if result.SessionID != "thread-abc" {
		t.Fatalf("result SessionID = %q, want thread-abc", result.SessionID)
	}
	if result.Result != `{"ok":true}` {
		t.Fatalf("result.Result = %q, want output-last-message content", result.Result)
	}
}
