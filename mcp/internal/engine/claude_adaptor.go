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

func (ClaudeAdaptor) Query(ctx context.Context, prompt string, opts LLMRequest) (<-chan json.RawMessage, func(), error) {
	args := ClaudeAdaptor{}.BuildCLIArgs(opts, "", "")
	args = append(args, "--", prompt)

	cmd := exec.CommandContext(ctx, ClaudeAdaptor{}.CLIPath(opts), args...)
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

func (ClaudeAdaptor) QuerySync(ctx context.Context, prompt string, opts LLMRequest) (string, error) {
	ch, cleanup, err := ClaudeAdaptor{}.Query(ctx, prompt, opts)
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

func (ClaudeAdaptor) BuildCLIArgs(req LLMRequest, outputPath, schemaPath string) []string {
	model := normalizeModelForBackend(req.Model, "claude")
	if model == "" {
		model = "claude-sonnet-4-6"
	}

	args := []string{
		"--print",
		"--output-format", "stream-json",
		"--verbose",
		"--model", model,
	}

	if req.PermissionMode != "" {
		args = append(args, "--permission-mode", req.PermissionMode)
	}
	// --max-turns is not in Claude Code 2.1.92 help output (hidden/deprecated).
	// Timeout via context controls duration instead.
	if req.JSONSchema != "" {
		args = append(args, "--json-schema", req.JSONSchema)
	}
	if req.SystemPrompt != "" {
		args = append(args, "--system-prompt", req.SystemPrompt)
	}
	if len(req.AllowedTools) > 0 {
		args = append(args, "--allowedTools", strings.Join(req.AllowedTools, ","))
	}
	if len(req.DisallowedTools) > 0 {
		args = append(args, "--disallowedTools", strings.Join(req.DisallowedTools, ","))
	}

	return args
}
