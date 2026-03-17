package main

import (
	"log"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	if err := initFilesystem(); err != nil {
		log.Printf("Warning: filesystem init failed: %v", err)
	}

	s := server.NewMCPServer(
		"prism-mcp",
		"1.0.0",
	)

	s.AddTool(
		mcp.NewTool("prism_interview",
			mcp.WithDescription("Socratic interviewer with integrated scoring. Reads findings.json, generates probing questions, and auto-scores after each answer. Returns {continue: true/false, score, question?}. Max 20 rounds. Caller must write findings.json before starting."),
			mcp.WithString("context_id", mcp.Required(), mcp.Description("Context identifier (e.g., incident-abc123, plan-def456, prd-ghi789)")),
			mcp.WithString("perspective_id", mcp.Required(), mcp.Description("Perspective identifier (e.g., timeline, root-cause, systems)")),
			mcp.WithString("topic", mcp.Description("Short title for the interview. Provide to start a new session.")),
			mcp.WithString("response", mcp.Description("Answer to the previous question. Required for follow-up rounds.")),
		),
		handleInterview,
	)

	s.AddTool(
		mcp.NewTool("prism_score",
			mcp.WithDescription("Ambiguity scorer. Evaluates clarity of findings + interview Q&A on 3 axes: Assumption (40%), Relevance (40%), Constraints (20%). Pass threshold: weighted_total > 0.8."),
			mcp.WithString("context_id", mcp.Required(), mcp.Description("Context identifier (e.g., incident-abc123, plan-def456, prd-ghi789)")),
			mcp.WithString("perspective_id", mcp.Required(), mcp.Description("Perspective identifier")),
		),
		handleScore,
	)

	// Filesystem tools (configured via ~/.prism/ontology-docs.json)
	if len(allowedDirs) > 0 {
		s.AddTool(
			mcp.NewTool("prism_docs_roots",
				mcp.WithDescription("Returns the list of configured documentation directories."),
			),
			handleListRoots,
		)

		s.AddTool(
			mcp.NewTool("prism_docs_list",
				mcp.WithDescription("List contents of a documentation directory. Only works within configured directories."),
				mcp.WithString("path", mcp.Required(), mcp.Description("Directory path to list")),
			),
			handleListDir,
		)

		s.AddTool(
			mcp.NewTool("prism_docs_read",
				mcp.WithDescription("Read a file from documentation directories. Max 500KB. Use head/tail for large files."),
				mcp.WithString("path", mcp.Required(), mcp.Description("File path to read")),
				mcp.WithNumber("head", mcp.Description("Return only the first N lines")),
				mcp.WithNumber("tail", mcp.Description("Return only the last N lines")),
			),
			handleReadFile,
		)

		s.AddTool(
			mcp.NewTool("prism_docs_search",
				mcp.WithDescription("Search for files by glob pattern within documentation directories. Skips hidden dirs and node_modules. Max 100 results."),
				mcp.WithString("path", mcp.Required(), mcp.Description("Directory to search in")),
				mcp.WithString("pattern", mcp.Required(), mcp.Description("Glob pattern to match filenames (e.g., *.md, *payment*)")),
			),
			handleSearchFiles,
		)

		log.Printf("Filesystem tools enabled: %d directories", len(allowedDirs))
	}

	log.Println("Prism MCP server starting on stdio...")
	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
