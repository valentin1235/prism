package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	prismconfig "github.com/heechul/prism-mcp/internal/config"
	"github.com/heechul/prism-mcp/internal/brownfield"
	"github.com/heechul/prism-mcp/internal/pipeline"
	taskpkg "github.com/heechul/prism-mcp/internal/task"
	"github.com/mark3labs/mcp-go/mcp"
)

// TaskStore is the package-level in-memory store for analysis tasks.
// Must be initialized by the caller (main) before server start.
var TaskStore *taskpkg.TaskStore

// PrismBaseDir is the resolved ~/.prism directory.
var PrismBaseDir string

func init() {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Printf("Warning: cannot resolve home dir: %v", err)
		return
	}
	PrismBaseDir = filepath.Join(home, ".prism")
}

// HandleAnalyze validates input parameters, creates a task entry,
// launches the analysis pipeline in a background goroutine, and
// immediately returns the task_id.
func HandleAnalyze(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// --- Extract and validate parameters ---

	topic, _ := request.Params.Arguments["topic"].(string)
	topic = strings.TrimSpace(topic)
	if topic == "" {
		return mcp.NewToolResultError("topic is required and must be non-empty"), nil
	}
	const maxTopicLen = 10_000
	if len([]rune(topic)) > maxTopicLen {
		return mcp.NewToolResultError(fmt.Sprintf("topic exceeds maximum length of %d characters", maxTopicLen)), nil
	}

	model, _ := request.Params.Arguments["model"].(string)
	model = strings.TrimSpace(model)
	if model == "" {
		model = "default"
	}
	if !isSupportedModelAlias(model) {
		return mcp.NewToolResultError(fmt.Sprintf("invalid model %q", model)), nil
	}

	adaptor, _ := request.Params.Arguments["adaptor"].(string)
	adaptor = strings.ToLower(strings.TrimSpace(adaptor))
	if adaptor == "" {
		adaptor = prismconfig.ResolveRuntimeBackend()
	}
	if adaptor != "codex" && adaptor != "claude" {
		return mcp.NewToolResultError(fmt.Sprintf("invalid adaptor %q", adaptor)), nil
	}

	inputContext, _ := request.Params.Arguments["input_context"].(string)
	inputContext = strings.TrimSpace(inputContext)

	// ontology_scope is a JSON string representing the scope mapping
	// (pre-resolved by SKILL.md before calling this tool)
	ontologyScope, _ := request.Params.Arguments["ontology_scope"].(string)
	ontologyScope = strings.TrimSpace(ontologyScope)

	seedHints, _ := request.Params.Arguments["seed_hints"].(string)
	seedHints = strings.TrimSpace(seedHints)

	reportTemplate, _ := request.Params.Arguments["report_template"].(string)
	reportTemplate = strings.TrimSpace(reportTemplate)

	sessionID, _ := request.Params.Arguments["session_id"].(string)
	sessionID = strings.TrimSpace(sessionID)

	language, _ := request.Params.Arguments["language"].(string)
	language = strings.TrimSpace(language)

	perspectiveInjection, _ := request.Params.Arguments["perspective_injection"].(string)
	perspectiveInjection = strings.TrimSpace(perspectiveInjection)

	// Validate input_context file exists if provided
	if inputContext != "" {
		if _, err := os.Stat(inputContext); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("input_context file not found: %s", inputContext)), nil
		}
	}

	// --- Resolve ontology scope: explicit param → brownfield defaults → error ---
	if ontologyScope == "" {
		resolved, err := resolveOntologyScopeFromBrownfield(PrismBaseDir)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		ontologyScope = resolved
	}

	// Validate report_template file exists if provided
	if reportTemplate != "" {
		if _, err := os.Stat(reportTemplate); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("report_template file not found: %s", reportTemplate)), nil
		}
	}

	// Validate perspective_injection file exists if provided
	// Full JSON parsing is deferred to runScopeStage to avoid double I/O.
	if perspectiveInjection != "" {
		if _, err := os.Stat(perspectiveInjection); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("perspective_injection file not found: %s", perspectiveInjection)), nil
		}
	}

	// --- Create state and report directories ---

	stateBase := filepath.Join(PrismBaseDir, "state")
	reportBase := filepath.Join(PrismBaseDir, "reports")

	// Create a task to get the generated ID
	// We use a temporary contextID first, then derive directories from task ID
	// When session_id is provided, task_id becomes "analyze-{session_id}"
	task := TaskStore.Create("", model, "", "", sessionID)

	// The task ID is "analyze-{12hex}", use it as the context ID and directory name
	contextID := task.ID
	stateDir := filepath.Join(stateBase, contextID)
	reportDir := filepath.Join(reportBase, contextID)

	// Update task fields now that we have the directories
	task.UpdateDirs(contextID, stateDir, reportDir)

	// Create directories
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		TaskStore.Remove(task.ID)
		return mcp.NewToolResultError(fmt.Sprintf("failed to create state directory: %v", err)), nil
	}
	if err := os.MkdirAll(reportDir, 0755); err != nil {
		TaskStore.Remove(task.ID)
		return mcp.NewToolResultError(fmt.Sprintf("failed to create report directory: %v", err)), nil
	}

	// --- Write config.json to state directory ---

	config := map[string]interface{}{
		"topic":      topic,
		"model":      model,
		"adaptor":    adaptor,
		"task_id":    task.ID,
		"context_id": contextID,
		"state_dir":  stateDir,
		"report_dir": reportDir,
	}
	if inputContext != "" {
		config["input_context"] = inputContext
	}
	if ontologyScope != "" {
		config["ontology_scope"] = ontologyScope
	}
	if seedHints != "" {
		config["seed_hints"] = seedHints
	}
	if reportTemplate != "" {
		config["report_template"] = reportTemplate
	}
	if language != "" {
		config["language"] = language
	}
	if perspectiveInjection != "" {
		config["perspective_injection"] = perspectiveInjection
	}

	configBytes, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		TaskStore.Remove(task.ID)
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal config: %v", err)), nil
	}
	configPath := filepath.Join(stateDir, "config.json")
	if err := os.WriteFile(configPath, configBytes, 0644); err != nil {
		TaskStore.Remove(task.ID)
		return mcp.NewToolResultError(fmt.Sprintf("failed to write config.json: %v", err)), nil
	}

	// --- Write ontology-scope.json to state directory ---
	// The ontology_scope parameter is a JSON string in canonical {"sources": [...]} format.
	// Write it as ontology-scope.json so LoadOntologyScopeText() can read it in all stages.
	if ontologyScope != "" {
		// Validate that the ontology_scope is valid JSON before writing
		if !json.Valid([]byte(ontologyScope)) {
			TaskStore.Remove(task.ID)
			return mcp.NewToolResultError("ontology_scope must be valid JSON"), nil
		}
		scopePath := filepath.Join(stateDir, "ontology-scope.json")
		if err := os.WriteFile(scopePath, []byte(ontologyScope), 0644); err != nil {
			TaskStore.Remove(task.ID)
			return mcp.NewToolResultError(fmt.Sprintf("failed to write ontology-scope.json: %v", err)), nil
		}
	}

	log.Printf("Analysis task %s created: topic=%q model=%s state=%s", task.ID, topic, model, stateDir)

	// --- Launch analysis pipeline in background goroutine ---
	// Create a cancellable context so that prism_cancel_task (and server shutdown)
	// can propagate cancellation to all in-flight subprocess work.
	pipelineCtx, pipelineCancel := context.WithCancel(context.Background())
	task.Ctx = pipelineCtx
	task.Cancel = pipelineCancel

	go pipeline.RunAnalysisPipeline(task)

	// --- Return task_id immediately ---

	snapshot := task.Snapshot()
	resultBytes, err := json.Marshal(snapshot)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal response: %v", err)), nil
	}

	return mcp.NewToolResultText(string(resultBytes)), nil
}

func isSupportedModelAlias(model string) bool {
	model = strings.TrimSpace(model)
	if model == "" || model == "default" {
		return true
	}
	return strings.HasPrefix(model, "claude-") || strings.HasPrefix(model, "gpt-") || strings.HasPrefix(model, "o")
}

// HandleTaskStatus retrieves the current status and progress of an analysis task
// by its task_id. Returns stage-level progress for running tasks and report_path
// or error for terminal tasks.
func HandleTaskStatus(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	taskID, _ := request.Params.Arguments["task_id"].(string)
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return mcp.NewToolResultError("task_id is required"), nil
	}

	task := TaskStore.Get(taskID)
	if task == nil {
		return mcp.NewToolResultError(fmt.Sprintf("task not found: %s", taskID)), nil
	}

	// Enforce max poll iterations for non-terminal tasks.
	// After taskpkg.MaxPollIterations (120) polls at 30-second intervals (60 minutes),
	// auto-cancel the task to prevent infinite polling.
	snapshot := task.Snapshot()
	if !snapshot.Status.IsTerminal() {
		pollCount := task.IncrPollCount()
		if pollCount > taskpkg.MaxPollIterations {
			log.Printf("[%s] Poll limit exceeded (%d > %d) — cancelling task",
				taskID, pollCount, taskpkg.MaxPollIterations)
			if task.Cancel != nil {
				task.Cancel()
			}
			task.SetError(fmt.Sprintf("poll limit exceeded: %d polls (max %d) — task timed out after prolonged execution", pollCount, taskpkg.MaxPollIterations))
			// Re-take snapshot after failure
			snapshot = task.Snapshot()
		}
	}

	resultBytes, err := json.Marshal(snapshot)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal status: %v", err)), nil
	}

	return mcp.NewToolResultText(string(resultBytes)), nil
}

// AnalyzeResultResponse is the JSON structure returned by prism_analyze_result.
type AnalyzeResultResponse struct {
	TaskID     string `json:"task_id"`
	Status     string `json:"status"`
	ReportPath string `json:"report_path"`
	Summary    string `json:"summary"`
}

// HandleAnalyzeResult returns the report file path and a summary for a completed analysis task.
// Returns an error if the task is not found, still running, or failed.
func HandleAnalyzeResult(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	taskID, _ := request.Params.Arguments["task_id"].(string)
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return mcp.NewToolResultError("task_id is required"), nil
	}

	snapshot, found := TaskStore.Snapshot(taskID)
	if !found {
		return mcp.NewToolResultError(fmt.Sprintf("task not found: %s", taskID)), nil
	}

	switch snapshot.Status {
	case taskpkg.TaskStatusQueued, taskpkg.TaskStatusRunning:
		return mcp.NewToolResultError(fmt.Sprintf("task %s is still %s — use prism_task_status to poll progress", taskID, snapshot.Status)), nil
	case taskpkg.TaskStatusFailed:
		errMsg := snapshot.Error
		if errMsg == "" {
			errMsg = "unknown error"
		}
		return mcp.NewToolResultError(fmt.Sprintf("task %s failed: %s", taskID, errMsg)), nil
	case taskpkg.TaskStatusCompleted:
		// Continue to extract result below
	default:
		return mcp.NewToolResultError(fmt.Sprintf("task %s has unexpected status: %s", taskID, snapshot.Status)), nil
	}

	reportPath := snapshot.ReportPath
	if reportPath == "" {
		return mcp.NewToolResultError(fmt.Sprintf("task %s completed but no report path recorded", taskID)), nil
	}

	// Read the report file and extract a summary
	summary, err := ExtractReportSummary(reportPath)
	if err != nil {
		log.Printf("[%s] Warning: could not extract report summary: %v", taskID, err)
		summary = "(summary extraction failed — report file may be unreadable)"
	}

	resp := AnalyzeResultResponse{
		TaskID:     taskID,
		Status:     string(taskpkg.TaskStatusCompleted),
		ReportPath: reportPath,
		Summary:    summary,
	}

	resultBytes, err := json.Marshal(resp)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal result: %v", err)), nil
	}

	return mcp.NewToolResultText(string(resultBytes)), nil
}

// ExtractReportSummary reads a report file and extracts the Executive Summary section.
// Falls back to the first N lines if no Executive Summary header is found.
func ExtractReportSummary(reportPath string) (string, error) {
	data, err := os.ReadFile(reportPath)
	if err != nil {
		return "", fmt.Errorf("read report: %w", err)
	}

	content := string(data)
	if content == "" {
		return "", fmt.Errorf("report file is empty")
	}

	// Try to extract "Executive Summary" section
	summary := ExtractSection(content, "Executive Summary")
	if summary != "" {
		// Truncate if too long (max ~2000 chars for MCP response)
		if len(summary) > 2000 {
			summary = summary[:2000] + "\n\n... (truncated — see full report)"
		}
		return summary, nil
	}

	// Fallback: return the first 1500 characters as preview
	preview := content
	if len(preview) > 1500 {
		preview = preview[:1500] + "\n\n... (truncated — see full report)"
	}
	return preview, nil
}

// ExtractSection extracts content between a markdown heading and the next heading of equal or higher level.
func ExtractSection(content string, sectionName string) string {
	lines := strings.Split(content, "\n")
	lowerTarget := strings.ToLower(sectionName)

	startIdx := -1
	headingLevel := 0

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			// Count heading level
			level := 0
			for _, ch := range trimmed {
				if ch == '#' {
					level++
				} else {
					break
				}
			}

			headerText := strings.ToLower(strings.TrimSpace(strings.TrimLeft(trimmed, "#")))
			if startIdx == -1 {
				// Looking for the target section
				if strings.Contains(headerText, lowerTarget) {
					startIdx = i + 1
					headingLevel = level
				}
			} else {
				// Found the start — look for next heading at same or higher level
				if level <= headingLevel {
					// Extract content between start and this heading
					section := strings.TrimSpace(strings.Join(lines[startIdx:i], "\n"))
					return section
				}
			}
		}
	}

	// If we found the start but hit EOF, return everything after
	if startIdx != -1 && startIdx < len(lines) {
		section := strings.TrimSpace(strings.Join(lines[startIdx:], "\n"))
		return section
	}

	return ""
}

// HandleCancelTask cancels a running analysis task by triggering context cancellation.
// This propagates to all in-flight subprocess work (specialists, interviews, synthesis).
func HandleCancelTask(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	taskID, _ := request.Params.Arguments["task_id"].(string)
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return mcp.NewToolResultError("task_id is required"), nil
	}

	task := TaskStore.Get(taskID)
	if task == nil {
		return mcp.NewToolResultError(fmt.Sprintf("task not found: %s", taskID)), nil
	}

	// Check if already in a terminal state
	snapshot := task.Snapshot()
	if snapshot.Status.IsTerminal() {
		resp := map[string]string{
			"task_id": taskID,
			"status":  string(snapshot.Status),
			"message": fmt.Sprintf("task already %s — nothing to cancel", snapshot.Status),
		}
		resultBytes, err := json.Marshal(resp)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to marshal response: %v", err)), nil
		}
		return mcp.NewToolResultText(string(resultBytes)), nil
	}

	// Cancel the pipeline context to stop all in-flight work
	if task.Cancel != nil {
		task.Cancel()
	}

	task.SetError("cancelled by user via prism_cancel_task")
	log.Printf("[%s] Task cancelled by user", taskID)

	// Return updated snapshot
	snapshot = task.Snapshot()
	resultBytes, err := json.Marshal(snapshot)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal response: %v", err)), nil
	}

	return mcp.NewToolResultText(string(resultBytes)), nil
}

// resolveOntologyScopeFromBrownfield opens the brownfield store at the given
// base directory, queries default repos (is_default=1), and builds an ontology
// scope JSON string from their paths. Returns an error if the store cannot be
// opened or no defaults are configured.
func resolveOntologyScopeFromBrownfield(baseDir string) (string, error) {
	dbPath := filepath.Join(baseDir, "prism.db")
	if _, err := os.Stat(dbPath); err != nil {
		return "", fmt.Errorf("brownfield store를 먼저 설정해주세요: prism.db not found at %s", dbPath)
	}

	store, err := brownfield.OpenStoreAt(dbPath)
	if err != nil {
		return "", fmt.Errorf("brownfield store를 먼저 설정해주세요: %v", err)
	}
	defer store.Close()

	defaults, err := store.DefaultRepos()
	if err != nil {
		return "", fmt.Errorf("brownfield store를 먼저 설정해주세요: %v", err)
	}

	if len(defaults) == 0 {
		return "", fmt.Errorf("ontology_scope이 지정되지 않았고 brownfield default repository도 설정되지 않았습니다. prism:brownfield defaults를 먼저 설정해주세요")
	}

	paths := make([]string, len(defaults))
	for i, r := range defaults {
		paths[i] = r.Path
	}

	scope, err := pipeline.BuildOntologyScopeFromPaths(paths)
	if err != nil {
		return "", fmt.Errorf("failed to build ontology scope: %v", err)
	}
	log.Printf("Resolved ontology scope from %d brownfield default repo(s)", len(defaults))
	return scope, nil
}
