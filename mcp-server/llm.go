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
		prompt,
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

