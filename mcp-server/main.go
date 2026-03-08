package main

import (
	"log"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	s := server.NewMCPServer(
		"prism-mcp",
		"1.0.0",
		server.WithToolCapabilities(true),
	)

	s.AddTool(
		mcp.NewTool("prism_interview",
			mcp.WithDescription("Socratic interviewer. Reads ~/.prism/state/{context_id}/perspectives/{perspective_id}/findings.json as context and generates probing questions. Max 30 rounds. Caller must write findings.json before starting."),
			mcp.WithString("context_id", mcp.Required(), mcp.Description("Context identifier (e.g., incident-abc123, plan-def456, prd-ghi789)")),
			mcp.WithString("perspective_id", mcp.Required(), mcp.Description("Perspective identifier (e.g., timeline, root-cause, systems)")),
			mcp.WithString("topic", mcp.Description("Short title for the interview. Provide to start a new session.")),
			mcp.WithString("response", mcp.Description("Answer to the previous question. Required for follow-up rounds.")),
		),
		handleInterview,
	)

	s.AddTool(
		mcp.NewTool("prism_score",
			mcp.WithDescription("Ambiguity scorer. Evaluates clarity of findings + interview Q&A on 3 axes: Goal Clarity (40%), Constraints (30%), Acceptance Criteria (30%). Pass threshold: weighted_total > 0.8."),
			mcp.WithString("context_id", mcp.Required(), mcp.Description("Context identifier (e.g., incident-abc123, plan-def456, prd-ghi789)")),
			mcp.WithString("perspective_id", mcp.Required(), mcp.Description("Perspective identifier")),
		),
		handleScore,
	)

	log.Println("Prism MCP server starting on stdio...")
	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
