package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// scoreSession evaluates the clarity of findings + interview Q&A.
// Returns the raw LLM scoring text.
func scoreSession(ctx context.Context, session *InterviewSession) (string, error) {
	if len(session.Rounds) == 0 {
		return "", fmt.Errorf("no interview rounds completed yet")
	}

	var history strings.Builder
	for i, qa := range session.Rounds {
		history.WriteString(fmt.Sprintf("Q%d: %s\nA%d: %s\n\n", i+1, qa.Question, i+1, qa.Answer))
	}

	findings := loadFindings(session.ContextID, session.PerspectiveID)
	findingsSection := ""
	if findings != "" {
		findingsSection = fmt.Sprintf("\nAnalyst findings:\n---\n%s\n---\n", findings)
	}

	prompt := fmt.Sprintf(`You are an ambiguity scorer evaluating how well-defined a set of requirements are.

Topic: %s
%s
Interview transcript:
%s

Score on these 3 axes (0.0 = completely ambiguous, 1.0 = perfectly clear):

1. **Assumption** (weight: 40%%): Are assumptions from input context verified rather than taken as fact? Are there unvalidated hypotheses treated as confirmed?
2. **Relevance** (weight: 40%%): Do the findings directly address the original topic/problem? Are the findings actually related to what was asked, or did the analyst find real but unrelated issues? If the topic mentions a concept that doesn't exist in the codebase, did the analyst acknowledge this gap?
3. **Constraints** (weight: 20%%): Are technical, resource, and scope constraints specified?

Respond in EXACTLY this format (no other text):
assumption: <score>
relevance: <score>
constraints: <score>
weighted_total: <weighted average>
pass: <true if weighted_total > 0.8, false otherwise>
summary: <one-line assessment in same language as topic>`, session.Topic, findingsSection, history.String())

	return queryLLM(ctx, prompt)
}

func handleScore(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.Params.Arguments
	contextID, _ := args["context_id"].(string)
	perspectiveID, _ := args["perspective_id"].(string)

	if contextID == "" || perspectiveID == "" {
		return mcp.NewToolResultError("context_id and perspective_id are required"), nil
	}

	session, err := loadSession(contextID, perspectiveID)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	result, err := scoreSession(ctx, session)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("scoring failed: %v", err)), nil
	}

	return mcp.NewToolResultText(result), nil
}
