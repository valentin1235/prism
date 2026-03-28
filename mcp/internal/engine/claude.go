package engine

// claude.go — Go abstraction mirroring Python claude_agent_sdk.
//
// Python SDK pattern:
//   async for message in query(prompt=prompt, options=ClaudeAgentOptions(...)):
//       process(message)
//
// Go equivalent:
//   ch, cleanup, _ := Query(ctx, prompt, ClaudeOptions{...})
//   defer cleanup()
//   for msg := range ch { process(msg) }
//
// Convenience:
//   result, _ := QuerySync(ctx, prompt, ClaudeOptions{...})

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

// ---------------------------------------------------------------------------
// Options (mirrors ClaudeAgentOptions)
// ---------------------------------------------------------------------------

// ClaudeOptions configures a Claude CLI invocation.
type ClaudeOptions struct {
	Model           string            // Model ID (default: "claude-sonnet-4-6")
	SystemPrompt    string            // System-level instructions
	PermissionMode  string            // "default", "acceptEdits", "bypassPermissions"
	MaxTurns        int               // Max agentic turns (0 = no limit, controlled by timeout)
	JSONSchema      string            // JSON schema for structured output
	AllowedTools    []string          // Tool whitelist (e.g. ["Read", "Grep", "Glob"])
	DisallowedTools []string          // Tool blocklist (e.g. ["Write", "Edit"])
	Cwd             string            // Working directory for isolation
	Env             map[string]string // Extra env vars to set
	OnMessage       func(msgType, detail string) // Optional progress callback
}

// ---------------------------------------------------------------------------
// Stream message types
// ---------------------------------------------------------------------------

// ResultMessage is the final message from a claude CLI invocation.
type ResultMessage struct {
	Type             string          `json:"type"`
	Subtype          string          `json:"subtype"` // "success" or "error"
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

// AssistantStreamMsg represents an assistant message in stream-json output.
type AssistantStreamMsg struct {
	Type    string `json:"type"`
	Message struct {
		Content []ContentBlock `json:"content"`
		Model   string         `json:"model"`
	} `json:"message"`
	SessionID string `json:"session_id"`
}

// ContentBlock is a single block within an assistant message.
type ContentBlock struct {
	Type     string          `json:"type"` // "text", "thinking", "tool_use", "tool_result"
	Text     string          `json:"text,omitempty"`
	Thinking string          `json:"thinking,omitempty"`
	ID       string          `json:"id,omitempty"`
	Name     string          `json:"name,omitempty"`
	Input    json.RawMessage `json:"input,omitempty"`
}

// ---------------------------------------------------------------------------
// Query — streaming API (mirrors Python async generator)
// ---------------------------------------------------------------------------

// Query streams NDJSON messages from the claude CLI.
// Callers receive raw JSON lines on the returned channel and must call
// cleanup when done (even on error/cancel).
func Query(ctx context.Context, prompt string, opts ClaudeOptions) (<-chan json.RawMessage, func(), error) {
	args := buildCLIArgs(opts)
	args = append(args, "--", prompt)

	cmd := exec.CommandContext(ctx, "claude", args...)
	if opts.Cwd != "" {
		cmd.Dir = opts.Cwd
	}
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
	cleanup := func() {
		if cmd.Process == nil {
			return
		}
		// Graceful shutdown: SIGTERM → wait → SIGKILL (mirrors Python terminate_process)
		_ = cmd.Process.Signal(syscall.SIGTERM)
		done := make(chan struct{})
		go func() {
			cmd.Wait() //nolint:errcheck
			close(done)
		}()
		select {
		case <-done:
			return
		case <-time.After(5 * time.Second):
			cmd.Process.Kill() //nolint:errcheck
			<-done
		}
	}

	go func() {
		defer close(ch)
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 1<<20), 10<<20) // 10 MB max line buffer
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
		cmd.Wait() //nolint:errcheck
	}()

	return ch, cleanup, nil
}

// ---------------------------------------------------------------------------
// QuerySync — convenience wrapper (returns final result string)
// ---------------------------------------------------------------------------

// QuerySync runs the claude CLI and blocks until it completes,
// returning the final result text. For --json-schema calls, returns
// structured_output if available, otherwise result text.
func QuerySync(ctx context.Context, prompt string, opts ClaudeOptions) (string, error) {
	ch, cleanup, err := Query(ctx, prompt, opts)
	if err != nil {
		return "", err
	}
	defer cleanup()

	var resultMsg *ResultMessage
	var structuredOutput string // captured from StructuredOutput tool_use
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
			// With --json-schema, the model calls a StructuredOutput tool.
			// The actual JSON is in the tool_use input, not in the result message.
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
			// Subprocess was likely killed by context timeout but did produce text.
			// Return the last assistant text as the result.
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

	log.Printf("Claude CLI completed: turns=%d duration=%dms cost=$%.4f",
		resultMsg.NumTurns, resultMsg.DurationMs, resultMsg.TotalCostUSD)

	// Priority: StructuredOutput tool call > structured_output field > result text
	if structuredOutput != "" {
		return structuredOutput, nil
	}
	if len(resultMsg.StructuredOutput) > 0 && string(resultMsg.StructuredOutput) != "null" {
		return string(resultMsg.StructuredOutput), nil
	}
	return resultMsg.Result, nil
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

func buildCLIArgs(opts ClaudeOptions) []string {
	model := opts.Model
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
	env := FilterEnv("CLAUDECODE")
	for k, v := range opts.Env {
		env = append(env, k+"="+v)
	}
	return env
}

// ---------------------------------------------------------------------------
// Retryable error classification (mirrors Python _RETRYABLE_ERROR_PATTERNS)
// ---------------------------------------------------------------------------

// retryableErrorPatterns are substrings that indicate a transient error
// worth retrying. Matches Python claude_code_adapter._RETRYABLE_ERROR_PATTERNS.
var retryableErrorPatterns = []string{
	"concurrency",
	"rate",
	"timeout",
	"overloaded",
	"temporarily",
	"empty response",
	"need retry",
}

// IsRetryableError checks whether an error message indicates a transient
// condition that should be retried with backoff.
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
	var base struct {
		Type string `json:"type"`
	}
	if json.Unmarshal(raw, &base) != nil {
		return
	}
	switch base.Type {
	case "assistant":
		var msg AssistantStreamMsg
		if json.Unmarshal(raw, &msg) == nil {
			for _, block := range msg.Message.Content {
				switch block.Type {
				case "tool_use":
					cb("tool", block.Name)
				case "text":
					if len(block.Text) > 120 {
						cb("text", block.Text[:120]+"...")
					} else if block.Text != "" {
						cb("text", block.Text)
					}
				}
			}
		}
	case "result":
		var rm ResultMessage
		if json.Unmarshal(raw, &rm) == nil {
			if rm.IsError {
				cb("error", rm.Result)
			} else {
				cb("result", fmt.Sprintf("turns=%d cost=$%.4f", rm.NumTurns, rm.TotalCostUSD))
			}
		}
	}
}
