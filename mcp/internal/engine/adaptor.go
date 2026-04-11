package engine

import (
	"context"
	"encoding/json"
)

// LLMRequest is the vendor-neutral request contract for Prism subprocess LLM calls.
type LLMRequest struct {
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

// ClaudeOptions is retained as a type alias so existing callers/tests keep compiling
// while the engine uses vendor-neutral adaptor routing internally.
type ClaudeOptions = LLMRequest

// LLMAdaptor owns vendor-specific subprocess invocation and response parsing.
type LLMAdaptor interface {
	Name() string
	Query(ctx context.Context, prompt string, req LLMRequest) (<-chan json.RawMessage, func(), error)
	QuerySync(ctx context.Context, prompt string, req LLMRequest) (string, error)
	BuildCLIArgs(req LLMRequest, outputPath, schemaPath string) []string
	CLIPath(req LLMRequest) string
}

// CodexAdaptor handles Codex CLI execution.
type CodexAdaptor struct{}

// ClaudeAdaptor handles Claude CLI execution.
type ClaudeAdaptor struct{}

func (CodexAdaptor) Name() string  { return "codex" }
func (ClaudeAdaptor) Name() string { return "claude" }

// ResolveAdaptor picks the concrete vendor adaptor for this request.
func ResolveAdaptor(req LLMRequest) LLMAdaptor {
	if resolveRuntimeBackend(req) == "claude" {
		return ClaudeAdaptor{}
	}
	return CodexAdaptor{}
}
