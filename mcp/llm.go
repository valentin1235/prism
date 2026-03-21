package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// filterEnv returns os.Environ() with the specified keys removed.
func filterEnv(keys ...string) []string {
	env := os.Environ()
	filtered := make([]string, 0, len(env))
	for _, e := range env {
		skip := false
		for _, key := range keys {
			if strings.HasPrefix(e, key+"=") {
				skip = true
				break
			}
		}
		if !skip {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

// queryLLM calls the claude CLI as a subprocess, leveraging Max Plan authentication.
func queryLLM(ctx context.Context, prompt string) (string, error) {
	cmd := exec.CommandContext(ctx, "claude",
		"--print",
		"--model", "claude-sonnet-4-6",
		"--max-turns", "1",
		"--", prompt,
	)
	cmd.Env = filterEnv("CLAUDECODE")

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("claude CLI error: %s", string(exitErr.Stderr))
		}
		return "", fmt.Errorf("claude CLI exec error: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// queryLLMWithSystemPrompt calls the claude CLI with a separate system prompt.
// This allows proper separation of the system instructions from the user message,
// which is important for tools that wrap a specific agent prompt (e.g., devils-advocate.md).
//
// Note: system prompt (~8KB) and user prompt are passed as CLI arguments.
// Safe within macOS ARG_MAX (256KB) and Linux (2MB) for current usage.
// If seed-analysis.json grows beyond ~200KB, consider stdin-based approach.
func queryLLMWithSystemPrompt(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	cmd := exec.CommandContext(ctx, "claude",
		"--print",
		"--model", "claude-sonnet-4-6",
		"--max-turns", "1",
		"--system-prompt", systemPrompt,
		"--", userPrompt,
	)
	cmd.Env = filterEnv("CLAUDECODE")

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("claude CLI error: %s", string(exitErr.Stderr))
		}
		return "", fmt.Errorf("claude CLI exec error: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// queryLLMScoped calls the claude CLI as a subprocess with task-scoped isolation.
// Each subprocess gets its own working directory (the task's stateDir) to ensure
// no resource contention between parallel analysis tasks. The model is explicitly
// specified per-task rather than using a hardcoded default.
//
// This is the preferred function for pipeline stages where multiple tasks may
// run concurrently. Each invocation is fully stateless—no shared file handles,
// working directories, or environment state between subprocesses.
func queryLLMScoped(ctx context.Context, stateDir, model, prompt string) (string, error) {
	if model == "" {
		model = "claude-sonnet-4-6"
	}
	cmd := exec.CommandContext(ctx, "claude",
		"--print",
		"--model", model,
		"--max-turns", "1",
		"--", prompt,
	)
	cmd.Dir = stateDir
	cmd.Env = filterEnv("CLAUDECODE")

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("claude CLI error (dir=%s): %s", stateDir, string(exitErr.Stderr))
		}
		return "", fmt.Errorf("claude CLI exec error (dir=%s): %w", stateDir, err)
	}

	return strings.TrimSpace(string(output)), nil
}

// queryLLMScopedWithSystemPrompt calls the claude CLI with task-scoped isolation
// and a separate system prompt. Combines the isolation of queryLLMScoped with
// the system/user prompt separation of queryLLMWithSystemPrompt.
//
// Each subprocess runs in the task's stateDir with no shared state, making it
// safe for concurrent execution across multiple analysis tasks and parallel
// specialist/interview stages within a single task.
func queryLLMScopedWithSystemPrompt(ctx context.Context, stateDir, model, systemPrompt, userPrompt string) (string, error) {
	if model == "" {
		model = "claude-sonnet-4-6"
	}
	cmd := exec.CommandContext(ctx, "claude",
		"--print",
		"--model", model,
		"--max-turns", "1",
		"--system-prompt", systemPrompt,
		"--", userPrompt,
	)
	cmd.Dir = stateDir
	cmd.Env = filterEnv("CLAUDECODE")

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("claude CLI error (dir=%s): %s", stateDir, string(exitErr.Stderr))
		}
		return "", fmt.Errorf("claude CLI exec error (dir=%s): %w", stateDir, err)
	}

	return strings.TrimSpace(string(output)), nil
}

// queryLLMScopedWithSchema calls the claude CLI in --print mode with --json-schema
// for structured output enforcement. Single-turn, no tool access.
// Suitable for perspective generation where all input is provided inline.
//
// The --json-schema flag constrains the LLM response to conform to the given
// JSON schema. The output is the raw structured JSON.
func queryLLMScopedWithSchema(ctx context.Context, stateDir, model, jsonSchema, prompt string) (string, error) {
	if model == "" {
		model = "claude-sonnet-4-6"
	}
	cmd := exec.CommandContext(ctx, "claude",
		"--print",
		"--model", model,
		"--max-turns", "1",
		"--json-schema", jsonSchema,
		"--", prompt,
	)
	cmd.Dir = stateDir
	cmd.Env = filterEnv("CLAUDECODE")

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("claude CLI error (dir=%s): %s", stateDir, string(exitErr.Stderr))
		}
		return "", fmt.Errorf("claude CLI exec error (dir=%s): %w", stateDir, err)
	}

	return strings.TrimSpace(string(output)), nil
}

// queryLLMScopedWithToolsAndSchema calls the claude CLI with tool access and
// --json-schema for structured output. Multi-turn mode allows the agent to use
// tools (Grep, Read, Glob, Bash) for active research. The --print flag captures
// the final structured output to stdout.
//
// Parameters:
//   - stateDir: working directory for task isolation
//   - model: LLM model identifier
//   - jsonSchema: JSON schema string for structured output enforcement
//   - systemPrompt: system-level instructions (empty string to omit)
//   - userPrompt: user message / task description
//   - maxTurns: maximum agentic turns (tool use iterations)
func queryLLMScopedWithToolsAndSchema(ctx context.Context, stateDir, model, jsonSchema, systemPrompt, userPrompt string, maxTurns int) (string, error) {
	if model == "" {
		model = "claude-sonnet-4-6"
	}
	if maxTurns <= 0 {
		maxTurns = 10
	}

	args := []string{
		"--print",
		"--model", model,
		"--max-turns", fmt.Sprintf("%d", maxTurns),
		"--json-schema", jsonSchema,
	}
	if systemPrompt != "" {
		args = append(args, "--system-prompt", systemPrompt)
	}
	args = append(args, "--", userPrompt)

	cmd := exec.CommandContext(ctx, "claude", args...)
	cmd.Dir = stateDir
	cmd.Env = filterEnv("CLAUDECODE")

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("claude CLI error (dir=%s): %s", stateDir, string(exitErr.Stderr))
		}
		return "", fmt.Errorf("claude CLI exec error (dir=%s): %w", stateDir, err)
	}

	return strings.TrimSpace(string(output)), nil
}

// extractJSON extracts a valid JSON object from output that may contain
// surrounding text (markdown fences, explanatory text, etc.).
// With --json-schema, output should be clean JSON, but this provides robustness
// for edge cases where the CLI wraps the output.
func extractJSON(s string) (string, error) {
	s = strings.TrimSpace(s)

	// Fast path: output is already valid JSON
	if json.Valid([]byte(s)) {
		return s, nil
	}

	// Strip markdown code fences if present (```json\n...\n```)
	if strings.HasPrefix(s, "```") {
		lines := strings.Split(s, "\n")
		if len(lines) > 2 {
			inner := strings.Join(lines[1:len(lines)-1], "\n")
			inner = strings.TrimSpace(inner)
			if json.Valid([]byte(inner)) {
				return inner, nil
			}
		}
	}

	// Find first '{' and its matching '}' with proper nesting
	start := strings.Index(s, "{")
	if start < 0 {
		return "", fmt.Errorf("no JSON object found in output (len=%d)", len(s))
	}

	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(s); i++ {
		ch := s[i]
		if escaped {
			escaped = false
			continue
		}
		if ch == '\\' && inString {
			escaped = true
			continue
		}
		if ch == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		switch ch {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				candidate := s[start : i+1]
				if json.Valid([]byte(candidate)) {
					return candidate, nil
				}
			}
		}
	}

	return "", fmt.Errorf("no valid JSON object found in output (len=%d, first 200 chars: %.200s)", len(s), s)
}
