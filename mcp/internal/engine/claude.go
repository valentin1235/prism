package engine

// claude.go retains Prism's historical engine API while switching the
// underlying subprocess transport to Codex CLI exec mode.

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
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

// ClaudeOptions configures a Prism LLM subprocess invocation.
// The type name is preserved to avoid rippling changes through the pipeline.
type ClaudeOptions struct {
	Model           string
	SystemPrompt    string
	PermissionMode  string
	MaxTurns        int
	JSONSchema      string
	AllowedTools    []string
	DisallowedTools []string
	Cwd             string
	Env             map[string]string
	OnMessage       func(msgType, detail string)
}

// ResultMessage is retained for compatibility with existing tests and callers.
type ResultMessage struct {
	Type             string          `json:"type"`
	Subtype          string          `json:"subtype"`
	IsError          bool            `json:"is_error"`
	Result           string          `json:"result"`
	StructuredOutput json.RawMessage `json:"structured_output,omitempty"`
	DurationMs       int             `json:"duration_ms"`
	DurationAPIMs    int             `json:"duration_api_ms"`
	NumTurns         int             `json:"num_turns"`
	SessionID        string          `json:"session_id"`
	TotalCostUSD     float64         `json:"total_cost_usd"`
	StopReason       string          `json:"stop_reason"`
}

// AssistantStreamMsg remains for backwards compatibility.
type AssistantStreamMsg struct {
	Type    string `json:"type"`
	Message struct {
		Content []ContentBlock `json:"content"`
		Model   string         `json:"model"`
	} `json:"message"`
	SessionID string `json:"session_id"`
}

// ContentBlock remains for backwards compatibility.
type ContentBlock struct {
	Type     string          `json:"type"`
	Text     string          `json:"text,omitempty"`
	Thinking string          `json:"thinking,omitempty"`
	ID       string          `json:"id,omitempty"`
	Name     string          `json:"name,omitempty"`
	Input    json.RawMessage `json:"input,omitempty"`
}

// maxQueryRetries is the number of retry attempts for retryable errors.
const maxQueryRetries = 3

// Query streams JSONL messages from a codex exec subprocess.
func Query(ctx context.Context, prompt string, opts ClaudeOptions) (<-chan json.RawMessage, func(), error) {
	if resolveRuntimeBackend(opts) == "claude" {
		return queryClaude(ctx, prompt, opts)
	}

	outputPath, err := newTempFile("prism-codex-output-*.txt")
	if err != nil {
		return nil, nil, fmt.Errorf("create output tempfile: %w", err)
	}

	schemaPath := ""
	if opts.JSONSchema != "" {
		schemaPath, err = newTempFile("prism-codex-schema-*.json")
		if err != nil {
			_ = os.Remove(outputPath)
			return nil, nil, fmt.Errorf("create schema tempfile: %w", err)
		}
		if err := os.WriteFile(schemaPath, []byte(opts.JSONSchema), 0644); err != nil {
			_ = os.Remove(outputPath)
			_ = os.Remove(schemaPath)
			return nil, nil, fmt.Errorf("write schema tempfile: %w", err)
		}
	}

	args := buildCLIArgs(opts, outputPath, schemaPath)
	cmd := exec.CommandContext(ctx, resolveCLIPath(opts), args...)
	cmd.Dir = commandWorkingDir(opts)
	cmd.Env = buildCLIEnv(opts)
	cmd.Stderr = os.Stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		_ = os.Remove(outputPath)
		_ = os.Remove(schemaPath)
		return nil, nil, fmt.Errorf("stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		_ = os.Remove(outputPath)
		_ = os.Remove(schemaPath)
		return nil, nil, fmt.Errorf("stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = os.Remove(outputPath)
		_ = os.Remove(schemaPath)
		return nil, nil, fmt.Errorf("start codex CLI: %w", err)
	}

	composedPrompt := composePrompt(prompt, opts)
	go func() {
		_, _ = stdin.Write([]byte(composedPrompt))
		_ = stdin.Close()
	}()

	ch := make(chan json.RawMessage, 32)
	var sessionID string
	var lastAssistantText string

	var waitOnce sync.Once
	doWait := func() { waitOnce.Do(func() { _ = cmd.Wait() }) }

	var cleanupOnce sync.Once
	cleanup := func() {
		cleanupOnce.Do(func() {
			defer os.Remove(outputPath) //nolint:errcheck
			if schemaPath != "" {
				defer os.Remove(schemaPath) //nolint:errcheck
			}
			if cmd.Process == nil {
				return
			}
			_ = cmd.Process.Signal(syscall.SIGTERM)
			done := make(chan struct{})
			go func() {
				doWait()
				close(done)
			}()
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				_ = cmd.Process.Kill()
				<-done
			}
		})
	}

	go func() {
		defer close(ch)
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 1<<20), 10<<20)
		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}
			raw := make(json.RawMessage, len(line))
			copy(raw, line)

			if opts.OnMessage != nil {
				dispatchCallback(raw, opts.OnMessage)
			}

			if id := extractSessionID(raw); id != "" {
				sessionID = id
			}
			if text := extractCodexText(raw); text != "" {
				lastAssistantText = text
			}

			if translated, ok := translateCodexEvent(raw, sessionID); ok {
				select {
				case ch <- translated:
				case <-ctx.Done():
					return
				}
			}

			select {
			case <-ctx.Done():
				return
			default:
			}
		}

		var waitErr error
		waitOnce.Do(func() {
			waitErr = cmd.Wait()
		})
		finalContent := readOutputFile(outputPath)
		if finalContent == "" {
			finalContent = lastAssistantText
		}
		if result, ok := makeLegacyResultMessage(waitErr, sessionID, finalContent); ok {
			select {
			case ch <- result:
			case <-ctx.Done():
				return
			}
		}
	}()

	return ch, cleanup, nil
}

func queryClaude(ctx context.Context, prompt string, opts ClaudeOptions) (<-chan json.RawMessage, func(), error) {
	args := buildClaudeCLIArgs(opts)
	args = append(args, "--", prompt)

	cmd := exec.CommandContext(ctx, resolveCLIPath(opts), args...)
	cmd.Dir = commandWorkingDir(opts)
	cmd.Env = buildCLIEnv(opts)
	cmd.Stderr = os.Stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("start claude CLI: %w", err)
	}

	ch := make(chan json.RawMessage, 32)
	var waitOnce sync.Once
	doWait := func() { waitOnce.Do(func() { _ = cmd.Wait() }) }

	var cleanupOnce sync.Once
	cleanup := func() {
		cleanupOnce.Do(func() {
			if cmd.Process == nil {
				return
			}
			_ = cmd.Process.Signal(syscall.SIGTERM)
			done := make(chan struct{})
			go func() {
				doWait()
				close(done)
			}()
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				_ = cmd.Process.Kill()
				<-done
			}
		})
	}

	go func() {
		defer close(ch)
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 1<<20), 10<<20)
		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}
			raw := make(json.RawMessage, len(line))
			copy(raw, line)

			if opts.OnMessage != nil {
				dispatchCallback(raw, opts.OnMessage)
			}

			select {
			case ch <- raw:
			case <-ctx.Done():
				return
			}
		}
		doWait()
	}()

	return ch, cleanup, nil
}

// QuerySync runs codex exec and returns the final output-last-message content.
func QuerySync(ctx context.Context, prompt string, opts ClaudeOptions) (string, error) {
	var lastErr error

	for attempt := 1; attempt <= maxQueryRetries; attempt++ {
		if attempt > 1 {
			log.Printf("QuerySync: retry %d/%d (previous error: %v)", attempt, maxQueryRetries, lastErr)
		}

		result, err := querySyncOnce(ctx, prompt, opts)
		if err == nil {
			return result, nil
		}

		if !IsRetryableError(err) {
			return "", err
		}
		lastErr = err
	}

	return "", fmt.Errorf("QuerySync failed after %d attempts: %w", maxQueryRetries, lastErr)
}

func querySyncOnce(ctx context.Context, prompt string, opts ClaudeOptions) (string, error) {
	if resolveRuntimeBackend(opts) == "claude" {
		return querySyncClaude(ctx, prompt, opts)
	}

	outputPath, err := newTempFile("prism-codex-output-*.txt")
	if err != nil {
		return "", fmt.Errorf("create output tempfile: %w", err)
	}
	defer os.Remove(outputPath) //nolint:errcheck

	schemaPath := ""
	if opts.JSONSchema != "" {
		schemaPath, err = newTempFile("prism-codex-schema-*.json")
		if err != nil {
			return "", fmt.Errorf("create schema tempfile: %w", err)
		}
		defer os.Remove(schemaPath) //nolint:errcheck
		if err := os.WriteFile(schemaPath, []byte(opts.JSONSchema), 0644); err != nil {
			return "", fmt.Errorf("write schema tempfile: %w", err)
		}
	}

	args := buildCLIArgs(opts, outputPath, schemaPath)
	cmd := exec.CommandContext(ctx, resolveCLIPath(opts), args...)
	cmd.Dir = commandWorkingDir(opts)
	cmd.Env = buildCLIEnv(opts)
	cmd.Stderr = os.Stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", fmt.Errorf("stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return "", fmt.Errorf("stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		return "", fmt.Errorf("start codex CLI: %w", err)
	}

	composedPrompt := composePrompt(prompt, opts)
	go func() {
		_, _ = stdin.Write([]byte(composedPrompt))
		_ = stdin.Close()
	}()

	var lastAssistantText string
	msgCount := 0

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 1<<20), 10<<20)
	for scanner.Scan() {
		msgCount++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		raw := make(json.RawMessage, len(line))
		copy(raw, line)

		if opts.OnMessage != nil {
			dispatchCallback(raw, opts.OnMessage)
		}

		if text := extractCodexText(raw); text != "" {
			lastAssistantText = text
		}
	}
	if err := scanner.Err(); err != nil {
		_ = terminateProcess(cmd)
		return "", fmt.Errorf("scan codex output: %w", err)
	}

	if err := cmd.Wait(); err != nil {
		content := readOutputFile(outputPath)
		if content == "" {
			content = lastAssistantText
		}
		if content == "" {
			content = err.Error()
		}
		return "", fmt.Errorf("codex exec failed after %d messages: %s", msgCount, strings.TrimSpace(content))
	}

	content := readOutputFile(outputPath)
	if content != "" {
		return content, nil
	}
	if lastAssistantText != "" {
		log.Printf("querySyncOnce: output-last-message missing, returning streamed text (%d msgs, %d chars)",
			msgCount, len(lastAssistantText))
		return lastAssistantText, nil
	}

	return "", fmt.Errorf("codex CLI produced no output (received %d msgs)", msgCount)
}

func querySyncClaude(ctx context.Context, prompt string, opts ClaudeOptions) (string, error) {
	ch, cleanup, err := queryClaude(ctx, prompt, opts)
	if err != nil {
		return "", err
	}
	defer cleanup()

	var resultMsg *ResultMessage
	var structuredOutput string
	var lastAssistantText string
	msgCount := 0

	for raw := range ch {
		msgCount++
		var base struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(raw, &base) != nil {
			log.Printf("QuerySync: failed to parse NDJSON line %d (len=%d)", msgCount, len(raw))
			continue
		}
		switch base.Type {
		case "assistant":
			var msg AssistantStreamMsg
			if json.Unmarshal(raw, &msg) == nil {
				for _, block := range msg.Message.Content {
					if block.Type == "tool_use" && block.Name == "StructuredOutput" {
						structuredOutput = string(block.Input)
					}
					if block.Type == "text" && block.Text != "" {
						lastAssistantText = block.Text
					}
				}
			}
		case "result":
			var rm ResultMessage
			if json.Unmarshal(raw, &rm) == nil {
				resultMsg = &rm
			}
		}
	}

	if resultMsg == nil {
		if lastAssistantText != "" {
			log.Printf("QuerySync: no result message but got assistant text (%d msgs received, %d chars), returning it",
				msgCount, len(lastAssistantText))
			return lastAssistantText, nil
		}
		return "", fmt.Errorf("claude CLI produced no result message (received %d msgs)", msgCount)
	}
	if resultMsg.IsError {
		return "", fmt.Errorf("claude error (turns=%d, stop=%s): %s",
			resultMsg.NumTurns, resultMsg.StopReason, resultMsg.Result)
	}
	if structuredOutput != "" {
		return structuredOutput, nil
	}
	if len(resultMsg.StructuredOutput) > 0 && string(resultMsg.StructuredOutput) != "null" {
		return string(resultMsg.StructuredOutput), nil
	}
	return resultMsg.Result, nil
}

func buildCLIArgs(opts ClaudeOptions, outputPath, schemaPath string) []string {
	if resolveRuntimeBackend(opts) == "claude" {
		return buildClaudeCLIArgs(opts)
	}
	return buildCodexCLIArgs(opts, outputPath, schemaPath)
}

func buildCodexCLIArgs(opts ClaudeOptions, outputPath, schemaPath string) []string {
	args := []string{
		"exec",
		"--json",
		"--skip-git-repo-check",
		"-C", commandWorkingDir(opts),
		"--output-last-message", outputPath,
	}

	if model := normalizeModel(opts.Model); model != "" {
		args = append(args, "--model", model)
	}

	args = append(args, buildPermissionArgs(opts.PermissionMode)...)

	if schemaPath != "" {
		args = append(args, "--output-schema", schemaPath)
	}

	return args
}

func buildClaudeCLIArgs(opts ClaudeOptions) []string {
	model := normalizeModelForBackend(opts.Model, "claude")
	if model == "" {
		model = "claude-sonnet-4-6"
	}

	args := []string{
		"--print",
		"--output-format", "stream-json",
		"--verbose",
		"--model", model,
	}

	if opts.PermissionMode != "" {
		args = append(args, "--permission-mode", opts.PermissionMode)
	}
	if opts.MaxTurns > 0 {
		args = append(args, "--max-turns", fmt.Sprintf("%d", opts.MaxTurns))
	}
	if opts.JSONSchema != "" {
		args = append(args, "--json-schema", opts.JSONSchema)
	}
	if opts.SystemPrompt != "" {
		args = append(args, "--system-prompt", opts.SystemPrompt)
	}
	if len(opts.AllowedTools) > 0 {
		args = append(args, "--allowedTools", strings.Join(opts.AllowedTools, ","))
	}
	if len(opts.DisallowedTools) > 0 {
		args = append(args, "--disallowedTools", strings.Join(opts.DisallowedTools, ","))
	}

	return args
}

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

func resolveCLIPath(opts ClaudeOptions) string {
	backend := resolveRuntimeBackend(opts)
	if backend == "claude" {
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

func normalizeModel(model string) string {
	return normalizeModelForBackend(model, resolveRuntimeBackend(ClaudeOptions{}))
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

	if maxTurns := opts.MaxTurns; maxTurns > 0 {
		parts = append(parts, fmt.Sprintf("## Execution Budget\nKeep the work within at most %d tool-assisted turns if possible.", maxTurns))
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

func dispatchCallback(raw json.RawMessage, cb func(string, string)) {
	if msgType, detail, ok := summarizeCodexEvent(raw); ok {
		cb(msgType, detail)
		return
	}

	if text := extractCodexText(raw); text != "" {
		if len(text) > 120 {
			cb("text", text[:120]+"...")
		} else {
			cb("text", text)
		}
		return
	}

	var event map[string]any
	if json.Unmarshal(raw, &event) != nil {
		return
	}
	if eventType, _ := event["type"].(string); eventType != "" {
		cb("event", eventType)
	}
}

func summarizeCodexEvent(raw json.RawMessage) (string, string, bool) {
	var event map[string]any
	if json.Unmarshal(raw, &event) != nil {
		return "", "", false
	}
	if eventType, _ := event["type"].(string); eventType != "item.completed" {
		return "", "", false
	}
	item, _ := event["item"].(map[string]any)
	itemType, _ := item["type"].(string)
	switch itemType {
	case "reasoning", "todo_list":
		if text := extractTextValue(item); text != "" {
			return "thinking", truncateDetail(text), true
		}
	case "command_execution":
		if command := extractTextValue(map[string]any{"command": item["command"]}); command != "" {
			return "tool", truncateDetail("Bash: " + command), true
		}
	case "mcp_tool_call":
		name, _ := item["name"].(string)
		if name == "" {
			name = "mcp_tool"
		}
		detail := name
		if inputText := extractTextValue(item["input"]); inputText != "" {
			detail += ": " + inputText
		}
		return "tool", truncateDetail(detail), true
	case "agent_message":
		if text := extractCodexText(raw); text != "" {
			return "text", truncateDetail(text), true
		}
	}
	return "", "", false
}

func truncateDetail(text string) string {
	text = strings.TrimSpace(text)
	if len(text) > 120 {
		return text[:120] + "..."
	}
	return text
}

func extractCodexText(raw json.RawMessage) string {
	var event map[string]any
	if json.Unmarshal(raw, &event) != nil {
		return ""
	}

	if item, ok := event["item"]; ok {
		if text := extractTextValue(item); text != "" {
			return text
		}
	}

	for _, key := range []string{"output_text", "content", "message", "summary", "details"} {
		if value, ok := event[key]; ok {
			if text := extractTextValue(value); text != "" {
				return text
			}
		}
	}

	return ""
}

func extractSessionID(raw json.RawMessage) string {
	var event map[string]any
	if json.Unmarshal(raw, &event) != nil {
		return ""
	}
	if eventType, _ := event["type"].(string); eventType == "thread.started" {
		if threadID, _ := event["thread_id"].(string); threadID != "" {
			return threadID
		}
	}
	for _, key := range []string{"session_id", "thread_id", "native_session_id", "run_id"} {
		if value, _ := event[key].(string); value != "" {
			return value
		}
	}
	if item, _ := event["item"].(map[string]any); item != nil {
		for _, key := range []string{"session_id", "thread_id"} {
			if value, _ := item[key].(string); value != "" {
				return value
			}
		}
	}
	return ""
}

func translateCodexEvent(raw json.RawMessage, sessionID string) (json.RawMessage, bool) {
	var event map[string]any
	if json.Unmarshal(raw, &event) != nil {
		return nil, false
	}
	if eventType, _ := event["type"].(string); eventType != "item.completed" {
		return nil, false
	}
	item, _ := event["item"].(map[string]any)
	itemType, _ := item["type"].(string)
	if itemType == "" {
		return nil, false
	}

	switch itemType {
	case "agent_message", "reasoning", "todo_list":
		text := extractTextValue(item)
		if text == "" {
			return nil, false
		}
		msg := AssistantStreamMsg{
			Type:      "assistant",
			SessionID: sessionID,
		}
		block := ContentBlock{Type: "text", Text: text}
		if itemType == "reasoning" || itemType == "todo_list" {
			block.Type = "thinking"
			block.Thinking = text
			block.Text = ""
		}
		msg.Message.Content = []ContentBlock{block}
		if marshaled, err := json.Marshal(msg); err == nil {
			return marshaled, true
		}
	}

	return nil, false
}

func makeLegacyResultMessage(waitErr error, sessionID, content string) (json.RawMessage, bool) {
	result := ResultMessage{
		Type:      "result",
		Subtype:   "success",
		Result:    strings.TrimSpace(content),
		SessionID: sessionID,
	}
	if waitErr != nil {
		result.Subtype = "error"
		result.IsError = true
		if result.Result == "" {
			result.Result = waitErr.Error()
		}
	}
	if result.Result == "" && !result.IsError {
		return nil, false
	}
	marshaled, err := json.Marshal(result)
	if err != nil {
		return nil, false
	}
	return marshaled, true
}

func extractTextValue(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			if text := extractTextValue(item); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n")
	case map[string]any:
		for _, key := range []string{"text", "message", "output_text", "content", "summary", "details", "command"} {
			if child, ok := v[key]; ok {
				if text := extractTextValue(child); text != "" {
					return text
				}
			}
		}
		parts := make([]string, 0, len(v))
		for _, child := range v {
			if text := extractTextValue(child); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n")
	default:
		return ""
	}
}
