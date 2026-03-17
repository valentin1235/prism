package main

import (
	"context"
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
