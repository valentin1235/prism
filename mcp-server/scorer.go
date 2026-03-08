package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

func handleScore(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	contextID, _ := args["context_id"].(string)
	perspectiveID, _ := args["perspective_id"].(string)

	if contextID == "" || perspectiveID == "" {
		return mcp.NewToolResultError("context_id and perspective_id are required"), nil
	}

	session, err := loadSession(contextID, perspectiveID)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if len(session.Rounds) == 0 {
		return mcp.NewToolResultError("no interview rounds completed yet"), nil
	}

	var history strings.Builder
	for i, qa := range session.Rounds {
		history.WriteString(fmt.Sprintf("Q%d: %s\nA%d: %s\n\n", i+1, qa.Question, i+1, qa.Answer))
	}

	findings := loadFindings(contextID, perspectiveID)
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

1. **Goal Clarity** (weight: 40%%): Is the objective well-defined? Are success outcomes clear?
2. **Constraints** (weight: 30%%): Are technical, resource, and scope constraints specified?
3. **Acceptance Criteria** (weight: 30%%): Are measurable success criteria defined?

Respond in EXACTLY this format (no other text):
goal: <score>
constraints: <score>
criteria: <score>
weighted_total: <weighted average>
pass: <true if weighted_total > 0.8, false otherwise>
summary: <one-line assessment in same language as topic>`, session.Topic, findingsSection, history.String())

	result, err := queryLLM(ctx, prompt)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("scoring failed: %v", err)), nil
	}

	return mcp.NewToolResultText(result), nil
}
