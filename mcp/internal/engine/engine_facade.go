package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
)

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

// Query streams vendor-native JSONL messages via the resolved adaptor.
func Query(ctx context.Context, prompt string, opts ClaudeOptions) (<-chan json.RawMessage, func(), error) {
	return ResolveAdaptor(opts).Query(ctx, prompt, opts)
}

// QuerySync runs the resolved adaptor and returns the final response content.
func QuerySync(ctx context.Context, prompt string, opts ClaudeOptions) (string, error) {
	adaptor := ResolveAdaptor(opts)
	backend := adaptor.Name()
	var lastErr error

	for attempt := 1; attempt <= maxQueryRetries; attempt++ {
		if attempt > 1 {
			log.Printf("QuerySync[%s]: retry %d/%d (previous error: %v)", backend, attempt, maxQueryRetries, lastErr)
		}

		result, err := adaptor.QuerySync(ctx, prompt, opts)
		if err == nil {
			return result, nil
		}

		if !IsRetryableError(err) {
			return "", fmt.Errorf("backend=%s: %w", backend, err)
		}
		lastErr = err
	}

	return "", fmt.Errorf("backend=%s: QuerySync failed after %d attempts: %w", backend, maxQueryRetries, lastErr)
}

func buildCLIArgs(opts ClaudeOptions, outputPath, schemaPath string) []string {
	return ResolveAdaptor(opts).BuildCLIArgs(opts, outputPath, schemaPath)
}

func resolveCLIPath(opts ClaudeOptions) string {
	return ResolveAdaptor(opts).CLIPath(opts)
}
