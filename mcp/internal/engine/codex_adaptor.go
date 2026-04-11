package engine

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

func (CodexAdaptor) Query(ctx context.Context, prompt string, opts LLMRequest) (<-chan json.RawMessage, func(), error) {
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

	args := CodexAdaptor{}.BuildCLIArgs(opts, outputPath, schemaPath)
	cmd := exec.CommandContext(ctx, CodexAdaptor{}.CLIPath(opts), args...)
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

func (CodexAdaptor) QuerySync(ctx context.Context, prompt string, opts LLMRequest) (string, error) {
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

	args := CodexAdaptor{}.BuildCLIArgs(opts, outputPath, schemaPath)
	cmd := exec.CommandContext(ctx, CodexAdaptor{}.CLIPath(opts), args...)
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

func (CodexAdaptor) BuildCLIArgs(req LLMRequest, outputPath, schemaPath string) []string {
	args := []string{
		"exec",
		"--json",
		"--skip-git-repo-check",
		"-C", commandWorkingDir(req),
		"--output-last-message", outputPath,
	}

	if model := normalizeModelForBackend(req.Model, "codex"); model != "" {
		args = append(args, "--model", model)
	}

	args = append(args, buildPermissionArgs(req.PermissionMode)...)

	if schemaPath != "" {
		args = append(args, "--output-schema", schemaPath)
	}

	return args
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
			return "tool", truncateDetail("Bash: "+command), true
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
