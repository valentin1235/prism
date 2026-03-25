package main

import (
	"log"

	"github.com/heechul/prism-mcp/internal/brownfield"
	"github.com/heechul/prism-mcp/internal/handler"
	taskpkg "github.com/heechul/prism-mcp/internal/task"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	if err := handler.InitFilesystem(); err != nil {
		log.Printf("Warning: filesystem init failed: %v", err)
	}

	// Initialize the task store for analysis orchestration
	handler.TaskStore = taskpkg.NewTaskStore()

	s := server.NewMCPServer(
		"prism",
		"1.0.1",
	)

	s.AddTool(
		mcp.NewTool("prism_interview",
			mcp.WithDescription("Socratic interviewer with integrated scoring. Reads findings.json, generates probing questions, and auto-scores after each answer. Returns {continue: true/false, score, question?}. Max 20 rounds. Caller must write findings.json before starting."),
			mcp.WithString("context_id", mcp.Required(), mcp.Description("Context identifier (e.g., incident-abc123, plan-def456, prd-ghi789)")),
			mcp.WithString("perspective_id", mcp.Required(), mcp.Description("Perspective identifier (e.g., timeline, root-cause, systems)")),
			mcp.WithString("topic", mcp.Description("Short title for the interview. Provide to start a new session.")),
			mcp.WithString("response", mcp.Description("Answer to the previous question. Required for follow-up rounds.")),
		),
		handler.HandleInterview,
	)

	s.AddTool(
		mcp.NewTool("prism_da_review",
			mcp.WithDescription("Devil's Advocate review of seed analysis. Reads seed-analysis.json, critiques perspective bias and codebase coverage gaps using the 4-phase DA protocol. Returns gap count — any gap (>=1) triggers fail. Hard-stops after 3 rounds."),
			mcp.WithString("seed_analysis_path", mcp.Required(), mcp.Description("Absolute path to seed-analysis.json file to review")),
			mcp.WithNumber("round", mcp.Description("Current loop round (1-based). Defaults to 1. Hard-stops after round 3.")),
			mcp.WithString("context", mcp.Description("Optional additional context for the DA review (e.g., specific areas of concern)")),
		),
		handler.HandleDAReview,
	)

	s.AddTool(
		mcp.NewTool("prism_score",
			mcp.WithDescription("Ambiguity scorer. Evaluates clarity of findings + interview Q&A on 3 axes: Assumption (40%), Relevance (40%), Constraints (20%). Pass threshold: weighted_total > 0.8."),
			mcp.WithString("context_id", mcp.Required(), mcp.Description("Context identifier (e.g., incident-abc123, plan-def456, prd-ghi789)")),
			mcp.WithString("perspective_id", mcp.Required(), mcp.Description("Perspective identifier")),
		),
		handler.HandleScore,
	)

	// Analysis orchestration tools
	s.AddTool(
		mcp.NewTool("prism_analyze",
			mcp.WithDescription("Start a new multi-perspective analysis. Launches a 4-stage pipeline (Scope → Specialists → Interview → Synthesis) as a background task. Returns immediately with a task_id for status polling via prism_task_status."),
			mcp.WithString("topic", mcp.Required(), mcp.Description("What to analyze — the central question or subject")),
			mcp.WithString("model", mcp.Description("Claude model to use for all stages. Default: claude-sonnet-4-6")),
			mcp.WithString("input_context", mcp.Description("Absolute path to input file providing additional context for the analysis")),
			mcp.WithString("ontology_scope", mcp.Description("JSON string of ontology scope in canonical {\"sources\": [...]} format (pre-resolved by caller). Written to state dir as ontology-scope.json.")),
			mcp.WithString("seed_hints", mcp.Description("Additional guidance for the seed analyst stage")),
			mcp.WithString("report_template", mcp.Description("Absolute path to a custom report template file")),
			mcp.WithString("session_id", mcp.Description("Optional session identifier. When provided, task_id becomes analyze-{session_id} for deterministic tracking")),
			mcp.WithString("language", mcp.Description("Language for the final report output (e.g. \"ko\", \"en\", \"ja\"). When omitted, the report is written in English by default")),
			mcp.WithString("perspective_injection", mcp.Description("Absolute path to a JSON file containing additional perspectives to merge into the generated perspective set after stage1. The file must be a JSON array of perspective objects matching the standard perspective schema.")),
		),
		handler.HandleAnalyze,
	)

	s.AddTool(
		mcp.NewTool("prism_task_status",
			mcp.WithDescription("Query the status and progress of an analysis task by task_id. Returns current stage progress for running tasks, report_path for completed tasks, or error details for failed tasks. Use this to poll after prism_analyze returns a task_id."),
			mcp.WithString("task_id", mcp.Required(), mcp.Description("The task identifier returned by prism_analyze")),
		),
		handler.HandleTaskStatus,
	)

	s.AddTool(
		mcp.NewTool("prism_cancel_task",
			mcp.WithDescription("Cancel a running analysis task. Propagates cancellation to all in-flight subprocess work (specialists, interviews, synthesis). Returns the updated task snapshot. No-op if the task is already completed or failed."),
			mcp.WithString("task_id", mcp.Required(), mcp.Description("The task identifier returned by prism_analyze")),
		),
		handler.HandleCancelTask,
	)

	s.AddTool(
		mcp.NewTool("prism_analyze_result",
			mcp.WithDescription("Retrieve the final result of a completed analysis task. Returns the report file path and an executive summary extracted from the report. Only works for completed tasks — returns an error for running, queued, or failed tasks."),
			mcp.WithString("task_id", mcp.Required(), mcp.Description("The task identifier returned by prism_analyze")),
		),
		handler.HandleAnalyzeResult,
	)

	// Filesystem tools (configured via ~/.prism/ontology-docs.json)
	if len(handler.AllowedDirs) > 0 {
		s.AddTool(
			mcp.NewTool("prism_docs_roots",
				mcp.WithDescription("Returns the list of configured documentation directories."),
			),
			handler.HandleListRoots,
		)

		s.AddTool(
			mcp.NewTool("prism_docs_list",
				mcp.WithDescription("List contents of a documentation directory. Only works within configured directories."),
				mcp.WithString("path", mcp.Required(), mcp.Description("Directory path to list")),
			),
			handler.HandleListDir,
		)

		s.AddTool(
			mcp.NewTool("prism_docs_read",
				mcp.WithDescription("Read a file from documentation directories. Max 500KB. Use head/tail for large files."),
				mcp.WithString("path", mcp.Required(), mcp.Description("File path to read")),
				mcp.WithNumber("head", mcp.Description("Return only the first N lines")),
				mcp.WithNumber("tail", mcp.Description("Return only the last N lines")),
			),
			handler.HandleReadFile,
		)

		s.AddTool(
			mcp.NewTool("prism_docs_search",
				mcp.WithDescription("Search for files by glob pattern within documentation directories. Skips hidden dirs and node_modules. Max 100 results."),
				mcp.WithString("path", mcp.Required(), mcp.Description("Directory to search in")),
				mcp.WithString("pattern", mcp.Required(), mcp.Description("Glob pattern to match filenames (e.g., *.md, *payment*)")),
			),
			handler.HandleSearchFiles,
		)

		log.Printf("Filesystem tools enabled: %d directories", len(handler.AllowedDirs))
	}

	// Brownfield repository registry
	if err := brownfield.InitStore(); err != nil {
		log.Printf("Warning: brownfield store init failed: %v", err)
	} else {
		s.AddTool(
			mcp.NewTool("prism_brownfield",
				mcp.WithDescription("Brownfield repository registry. Scans home directory for GitHub repos, registers them in SQLite, manages default selections, and generates README-based descriptions. Actions: scan, register, query, set_default, set_defaults, generate_desc."),
				mcp.WithString("action", mcp.Description("Action to perform: scan, register, query, set_default, set_defaults, generate_desc. Auto-detected from parameters when omitted.")),
				mcp.WithString("path", mcp.Description("Absolute filesystem path of the repository (for register, set_default, generate_desc)")),
				mcp.WithString("name", mcp.Description("Human-readable name. Defaults to directory name (for register)")),
				mcp.WithString("desc", mcp.Description("One-line description. If omitted, auto-generated from README (for register)")),
				mcp.WithBoolean("is_default", mcp.Description("For set_default: true to mark as default, false to unmark")),
				mcp.WithBoolean("default_only", mcp.Description("For query: return only repos marked as default")),
				mcp.WithString("scan_root", mcp.Description("Root directory for scan. Defaults to home directory")),
				mcp.WithString("indices", mcp.Description("Comma-separated rowid list for set_defaults (e.g., '6,18,19')")),
				mcp.WithNumber("offset", mcp.Description("Number of rows to skip for pagination (for query)")),
				mcp.WithNumber("limit", mcp.Description("Maximum rows to return (for query). 0 = no limit")),
			),
			brownfield.HandleBrownfield,
		)
		log.Println("Brownfield tool enabled")
	}

	log.Println("Prism MCP server starting on stdio...")
	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
