package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

// taskStore is the package-level in-memory store for analysis tasks.
// Initialized in main() before server start.
var taskStore *TaskStore

// prismBaseDir is the resolved ~/.prism directory.
var prismBaseDir string

func init() {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Printf("Warning: cannot resolve home dir: %v", err)
		return
	}
	prismBaseDir = filepath.Join(home, ".prism")
}

// handleAnalyze validates input parameters, creates a task entry,
// launches the analysis pipeline in a background goroutine, and
// immediately returns the task_id.
func handleAnalyze(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
		model = "claude-sonnet-4-6" // default model
	}
	if !strings.HasPrefix(model, "claude-") {
		return mcp.NewToolResultError(fmt.Sprintf("invalid model %q: must start with 'claude-'", model)), nil
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

	stateBase := filepath.Join(prismBaseDir, "state")
	reportBase := filepath.Join(prismBaseDir, "reports")

	// Create a task to get the generated ID
	// We use a temporary contextID first, then derive directories from task ID
	// When session_id is provided, task_id becomes "analyze-{session_id}"
	task := taskStore.Create("", model, "", "", sessionID)

	// The task ID is "analyze-{12hex}", use it as the context ID and directory name
	contextID := task.ID
	stateDir := filepath.Join(stateBase, contextID)
	reportDir := filepath.Join(reportBase, contextID)

	// Update task fields now that we have the directories
	task.UpdateDirs(contextID, stateDir, reportDir)

	// Create directories
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		taskStore.Remove(task.ID)
		return mcp.NewToolResultError(fmt.Sprintf("failed to create state directory: %v", err)), nil
	}
	if err := os.MkdirAll(reportDir, 0755); err != nil {
		taskStore.Remove(task.ID)
		return mcp.NewToolResultError(fmt.Sprintf("failed to create report directory: %v", err)), nil
	}

	// --- Write config.json to state directory ---

	config := map[string]interface{}{
		"topic":       topic,
		"model":       model,
		"task_id":     task.ID,
		"context_id":  contextID,
		"state_dir":   stateDir,
		"report_dir":  reportDir,
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
		taskStore.Remove(task.ID)
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal config: %v", err)), nil
	}
	configPath := filepath.Join(stateDir, "config.json")
	if err := os.WriteFile(configPath, configBytes, 0644); err != nil {
		taskStore.Remove(task.ID)
		return mcp.NewToolResultError(fmt.Sprintf("failed to write config.json: %v", err)), nil
	}

	// --- Write ontology-scope.json to state directory ---
	// The ontology_scope parameter is a JSON string in canonical {"sources": [...]} format.
	// Write it as ontology-scope.json so loadOntologyScopeText() can read it in all stages.
	if ontologyScope != "" {
		// Validate that the ontology_scope is valid JSON before writing
		if !json.Valid([]byte(ontologyScope)) {
			taskStore.Remove(task.ID)
			return mcp.NewToolResultError("ontology_scope must be valid JSON"), nil
		}
		scopePath := filepath.Join(stateDir, "ontology-scope.json")
		if err := os.WriteFile(scopePath, []byte(ontologyScope), 0644); err != nil {
			taskStore.Remove(task.ID)
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

	go runAnalysisPipeline(task)

	// --- Return task_id immediately ---

	snapshot := task.Snapshot()
	resultBytes, err := json.Marshal(snapshot)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal response: %v", err)), nil
	}

	return mcp.NewToolResultText(string(resultBytes)), nil
}

// handleTaskStatus retrieves the current status and progress of an analysis task
// by its task_id. Returns stage-level progress for running tasks and report_path
// or error for terminal tasks.
func handleTaskStatus(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	taskID, _ := request.Params.Arguments["task_id"].(string)
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return mcp.NewToolResultError("task_id is required"), nil
	}

	task := taskStore.Get(taskID)
	if task == nil {
		return mcp.NewToolResultError(fmt.Sprintf("task not found: %s", taskID)), nil
	}

	// Enforce max poll iterations for non-terminal tasks.
	// After MaxPollIterations (120) polls at 30-second intervals (60 minutes),
	// auto-cancel the task to prevent infinite polling.
	snapshot := task.Snapshot()
	if !snapshot.Status.IsTerminal() {
		pollCount := task.IncrPollCount()
		if pollCount > MaxPollIterations {
			log.Printf("[%s] Poll limit exceeded (%d > %d) — cancelling task",
				taskID, pollCount, MaxPollIterations)
			if task.Cancel != nil {
				task.Cancel()
			}
			task.SetError(fmt.Sprintf("poll limit exceeded: %d polls (max %d) — task timed out after prolonged execution", pollCount, MaxPollIterations))
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

// handleAnalyzeResult returns the report file path and a summary for a completed analysis task.
// Returns an error if the task is not found, still running, or failed.
func handleAnalyzeResult(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	taskID, _ := request.Params.Arguments["task_id"].(string)
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return mcp.NewToolResultError("task_id is required"), nil
	}

	snapshot, found := taskStore.Snapshot(taskID)
	if !found {
		return mcp.NewToolResultError(fmt.Sprintf("task not found: %s", taskID)), nil
	}

	switch snapshot.Status {
	case TaskStatusQueued, TaskStatusRunning:
		return mcp.NewToolResultError(fmt.Sprintf("task %s is still %s — use prism_task_status to poll progress", taskID, snapshot.Status)), nil
	case TaskStatusFailed:
		errMsg := snapshot.Error
		if errMsg == "" {
			errMsg = "unknown error"
		}
		return mcp.NewToolResultError(fmt.Sprintf("task %s failed: %s", taskID, errMsg)), nil
	case TaskStatusCompleted:
		// Continue to extract result below
	default:
		return mcp.NewToolResultError(fmt.Sprintf("task %s has unexpected status: %s", taskID, snapshot.Status)), nil
	}

	reportPath := snapshot.ReportPath
	if reportPath == "" {
		return mcp.NewToolResultError(fmt.Sprintf("task %s completed but no report path recorded", taskID)), nil
	}

	// Read the report file and extract a summary
	summary, err := extractReportSummary(reportPath)
	if err != nil {
		log.Printf("[%s] Warning: could not extract report summary: %v", taskID, err)
		summary = "(summary extraction failed — report file may be unreadable)"
	}

	resp := AnalyzeResultResponse{
		TaskID:     taskID,
		Status:     string(TaskStatusCompleted),
		ReportPath: reportPath,
		Summary:    summary,
	}

	resultBytes, err := json.Marshal(resp)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal result: %v", err)), nil
	}

	return mcp.NewToolResultText(string(resultBytes)), nil
}

// extractReportSummary reads a report file and extracts the Executive Summary section.
// Falls back to the first N lines if no Executive Summary header is found.
func extractReportSummary(reportPath string) (string, error) {
	data, err := os.ReadFile(reportPath)
	if err != nil {
		return "", fmt.Errorf("read report: %w", err)
	}

	content := string(data)
	if content == "" {
		return "", fmt.Errorf("report file is empty")
	}

	// Try to extract "Executive Summary" section
	summary := extractSection(content, "Executive Summary")
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

// extractSection extracts content between a markdown heading and the next heading of equal or higher level.
func extractSection(content string, sectionName string) string {
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

// handleCancelTask cancels a running analysis task by triggering context cancellation.
// This propagates to all in-flight subprocess work (specialists, interviews, synthesis).
func handleCancelTask(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	taskID, _ := request.Params.Arguments["task_id"].(string)
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return mcp.NewToolResultError("task_id is required"), nil
	}

	task := taskStore.Get(taskID)
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

// AnalysisConfig holds the configuration read from config.json in the state directory.
type AnalysisConfig struct {
	Topic          string `json:"topic"`
	Model          string `json:"model"`
	TaskID         string `json:"task_id"`
	ContextID      string `json:"context_id"`
	StateDir       string `json:"state_dir"`
	ReportDir      string `json:"report_dir"`
	InputContext   string `json:"input_context,omitempty"`
	OntologyScope  string `json:"ontology_scope,omitempty"`
	SeedHints      string `json:"seed_hints,omitempty"`
	ReportTemplate         string `json:"report_template,omitempty"`
	Language               string `json:"language,omitempty"`
	PerspectiveInjection   string `json:"perspective_injection,omitempty"`
}

// readAnalysisConfig reads config.json from the task's state directory.
func readAnalysisConfig(stateDir string) (AnalysisConfig, error) {
	var cfg AnalysisConfig
	data, err := os.ReadFile(filepath.Join(stateDir, "config.json"))
	if err != nil {
		return cfg, fmt.Errorf("read config.json: %w", err)
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config.json: %w", err)
	}
	return cfg, nil
}

// StageResult holds the outcome of a single parallel sub-task (specialist or interview).
type StageResult struct {
	PerspectiveID string // which perspective this result belongs to
	OutputPath    string // path to the output file (findings.json or verified-findings.json)
	Err           error  // nil on success
}

// runAnalysisPipeline executes the 4-stage analysis pipeline in a background goroutine.
// Each stage transition updates the task's thread-safe in-memory state so that
// prism_task_status callers can observe progress in real time.
//
// Pipeline stages:
//  1. Scope: seed analysis → DA review → perspective generation
//  2. Specialist: parallel finding sessions (one per perspective)
//  3. Interview: parallel verification sessions (one per perspective)
//  4. Synthesis: report generation from verified findings
func runAnalysisPipeline(task *AnalysisTask) {
	// Ensure cancel is called when pipeline exits to release resources.
	defer func() {
		if task.Cancel != nil {
			task.Cancel()
		}
	}()

	task.SetStatus(TaskStatusRunning)
	log.Printf("[%s] Pipeline started", task.ID)

	// --- Read config ---
	task.mu.RLock()
	stateDir := task.StateDir
	task.mu.RUnlock()

	cfg, err := readAnalysisConfig(stateDir)
	if err != nil {
		task.FailStage(StageScope, fmt.Sprintf("config read error: %v", err))
		task.SetError(fmt.Sprintf("failed to read config: %v", err))
		log.Printf("[%s] Pipeline failed: %v", task.ID, err)
		return
	}

	// ============================
	// Stage 1: Scope
	// ============================
	task.StartStage(StageScope, "starting seed analysis")
	log.Printf("[%s] Stage scope: started", task.ID)

	perspectives, err := runScopeStage(task, cfg)
	if err != nil {
		task.FailStage(StageScope, fmt.Sprintf("scope failed: %v", err))
		task.SetError(fmt.Sprintf("scope stage failed: %v", err))
		log.Printf("[%s] Stage scope: FAILED — %v", task.ID, err)
		return
	}

	task.CompleteStage(StageScope, fmt.Sprintf("%d perspectives generated", len(perspectives)))
	log.Printf("[%s] Stage scope: completed with %d perspectives", task.ID, len(perspectives))

	// ============================
	// Stage 2: Specialist (parallel)
	// ============================
	numPerspectives := len(perspectives)
	task.StartStage(StageSpecialist, fmt.Sprintf("launching %d specialists", numPerspectives))
	task.SetStageParallel(StageSpecialist, numPerspectives)
	log.Printf("[%s] Stage specialist: started with %d perspectives", task.ID, numPerspectives)

	specialistResults := runSpecialistStage(task, cfg, perspectives)

	// Collect and aggregate specialist results
	collected := CollectSpecialistResults(task.ID, specialistResults, perspectives)

	// Persist collected findings for downstream stages
	task.mu.RLock()
	collectStateDir := task.StateDir
	task.mu.RUnlock()

	if err := WriteCollectedFindings(collectStateDir, collected); err != nil {
		log.Printf("[%s] Warning: failed to persist collected findings: %v", task.ID, err)
		// Non-fatal — downstream stages can still use in-memory collected findings
	}

	// All specialists failed → abort
	if collected.Succeeded == 0 {
		task.FailStage(StageSpecialist, fmt.Sprintf("all %d specialists failed", collected.Failed))
		task.SetError("all specialist analyses failed")
		log.Printf("[%s] Stage specialist: FAILED — all %d failed", task.ID, collected.Failed)
		return
	}

	detail := fmt.Sprintf("%d/%d succeeded, %d findings collected",
		collected.Succeeded, numPerspectives, collected.TotalFindings)
	if collected.PartialFailure {
		detail += fmt.Sprintf(" (%d failed — partial)", collected.Failed)
		log.Printf("[%s] Stage specialist: partial failure — %s", task.ID, collected.DegradationNotice())
	}
	task.CompleteStage(StageSpecialist, detail)
	log.Printf("[%s] Stage specialist: completed — %s", task.ID, detail)

	// ============================
	// Stage 3: Interview (parallel)
	// ============================
	// Only interview perspectives that produced findings
	interviewCount := collected.Succeeded
	task.StartStage(StageInterview, fmt.Sprintf("launching %d interviews", interviewCount))
	task.SetStageParallel(StageInterview, interviewCount)
	log.Printf("[%s] Stage interview: started with %d verifiers", task.ID, interviewCount)

	interviewResults := runInterviewStage(task, cfg, perspectives, specialistResults)

	// Collect and aggregate interview results
	collectedVerifications := CollectInterviewResults(task.ID, interviewResults, perspectives)

	// Persist collected verifications for synthesis stage
	task.mu.RLock()
	interviewStateDir := task.StateDir
	task.mu.RUnlock()

	if err := WriteCollectedVerifications(interviewStateDir, collectedVerifications); err != nil {
		log.Printf("[%s] Warning: failed to persist collected verifications: %v", task.ID, err)
		// Non-fatal — downstream stages can still use in-memory results
	}

	// All interviews failed → still proceed with unverified findings (degraded)
	intDetail := fmt.Sprintf("%d/%d verified", collectedVerifications.Succeeded, interviewCount)
	if collectedVerifications.Failed > 0 {
		intDetail += fmt.Sprintf(" (%d failed — unverified findings used)", collectedVerifications.Failed)
	}
	if collectedVerifications.AverageScore > 0 {
		intDetail += fmt.Sprintf(", avg score: %.2f", collectedVerifications.AverageScore)
	}
	if collectedVerifications.PartialFailure {
		log.Printf("[%s] Stage interview: partial failure — %s", task.ID, collectedVerifications.InterviewDegradationNotice())
	}
	task.CompleteStage(StageInterview, intDetail)
	log.Printf("[%s] Stage interview: completed — %s", task.ID, intDetail)

	// ============================
	// Stage 4: Synthesis
	// ============================
	task.StartStage(StageSynthesis, "generating report")
	log.Printf("[%s] Stage synthesis: started", task.ID)

	reportPath, err := runSynthesisStage(task, cfg, perspectives, interviewResults)
	if err != nil {
		task.FailStage(StageSynthesis, fmt.Sprintf("synthesis failed: %v", err))
		task.SetError(fmt.Sprintf("synthesis stage failed: %v", err))
		log.Printf("[%s] Stage synthesis: FAILED — %v", task.ID, err)
		return
	}

	task.CompleteStage(StageSynthesis, fmt.Sprintf("report at %s", reportPath))
	task.SetReportPath(reportPath)
	log.Printf("[%s] Pipeline completed — report: %s", task.ID, reportPath)
}

// runScopeStage executes the scope stage: seed analysis → DA review → perspective generation.
// Returns the generated perspectives or an error.
func runScopeStage(task *AnalysisTask, cfg AnalysisConfig) ([]Perspective, error) {
	task.mu.RLock()
	stateDir := task.StateDir
	task.mu.RUnlock()

	// Sub-step 1: Seed analysis
	task.UpdateStageDetail(StageScope, "running seed analysis")
	if err := runSeedAnalysis(task, cfg); err != nil {
		return nil, fmt.Errorf("seed analysis: %w", err)
	}

	// Sub-step 2: DA review loop (up to 3 rounds)
	task.UpdateStageDetail(StageScope, "seed complete, running DA review")
	if err := runDAReviewLoop(task, cfg); err != nil {
		return nil, fmt.Errorf("DA review: %w", err)
	}

	// Sub-step 3: Perspective generation
	task.UpdateStageDetail(StageScope, "DA review complete, generating perspectives")
	if err := runPerspectiveGeneration(task, cfg); err != nil {
		return nil, fmt.Errorf("perspective generation: %w", err)
	}

	// Read the generated perspectives
	perspPath := filepath.Join(stateDir, "perspectives.json")
	pf, err := ReadPerspectives(perspPath)
	if err != nil {
		return nil, fmt.Errorf("read generated perspectives: %w", err)
	}

	if len(pf.Perspectives) == 0 {
		return nil, fmt.Errorf("no perspectives generated")
	}

	// Sub-step 4: Merge injected perspectives (if perspective_injection provided)
	if cfg.PerspectiveInjection != "" {
		task.UpdateStageDetail(StageScope, "merging injected perspectives")
		injected, err := loadInjectedPerspectives(cfg.PerspectiveInjection)
		if err != nil {
			// Non-fatal: log warning and continue with generated perspectives only
			log.Printf("[%s] Warning: failed to load injected perspectives from %s: %v — continuing without injection",
				task.ID, cfg.PerspectiveInjection, err)
		} else if len(injected) > 0 {
			originalLen := len(pf.Perspectives)
			pf.Perspectives = mergeInjectedPerspectives(pf.Perspectives, injected)
			actuallyAdded := len(pf.Perspectives) - originalLen
			// Persist the merged perspective set back to disk
			if err := WritePerspectives(perspPath, pf); err != nil {
				return nil, fmt.Errorf("write merged perspectives: %w", err)
			}
			log.Printf("[%s] Merged %d/%d injected perspectives (skipped %d duplicates) — total: %d",
				task.ID, actuallyAdded, len(injected), len(injected)-actuallyAdded, len(pf.Perspectives))
		}
	}

	return pf.Perspectives, nil
}

// runSpecialistStage executes parallel finding sessions for each perspective.
// Each specialist runs as a separate claude CLI subprocess with concurrency
// limited by the ParallelExecutor (default: 4 concurrent subprocesses).
// LoadSpecialistContext is called once via BuildAllSpecialistCommands, then
// pre-built commands are passed to each runSpecialistSession.
// Updates task parallel progress counters as specialists complete.
func runSpecialistStage(task *AnalysisTask, cfg AnalysisConfig, perspectives []Perspective) []StageResult {
	task.mu.RLock()
	taskID := task.ID
	task.mu.RUnlock()

	// Build all specialist commands once — loads shared context (seed summary,
	// ontology scope, doc paths) a single time instead of per-perspective.
	commands, err := BuildAllSpecialistCommands(cfg, perspectives)
	if err != nil {
		// Return an error result for each perspective
		results := make([]StageResult, len(perspectives))
		for i := range perspectives {
			results[i] = StageResult{Err: fmt.Errorf("build specialist commands: %w", err)}
		}
		return results
	}

	jobs := make([]ParallelJob, len(commands))
	for i, c := range commands {
		cmd := c // capture for closure
		jobs[i] = ParallelJob{
			PerspectiveID: cmd.PerspectiveID,
			Fn: func(ctx context.Context) StageResult {
				err := runSpecialistSession(ctx, task, cmd)
				if err != nil {
					return StageResult{Err: err}
				}
				return StageResult{
					OutputPath: cmd.OutputPath,
				}
			},
		}
	}

	executor := &ParallelExecutor{
		Concurrency: DefaultConcurrencyLimit,
		RetryLimit:  2, // 1 initial + 1 retry
		JobTimeout:  DefaultJobTimeout,
		OnJobComplete: func(perspectiveID string, success bool, attempts int) {
			if success {
				task.IncrStageCompleted(StageSpecialist)
				log.Printf("[%s] Specialist %s completed (attempts: %d)", taskID, perspectiveID, attempts)
			} else {
				task.IncrStageFailed(StageSpecialist)
				log.Printf("[%s] Specialist %s failed after %d attempts", taskID, perspectiveID, attempts)
			}
		},
	}

	pr := executor.Execute(task.Ctx, jobs)
	return pr.Results
}

// runInterviewStage executes parallel verification sessions for perspectives
// that produced findings. Uses ParallelExecutor for concurrency limiting.
// Updates task parallel progress counters.
func runInterviewStage(task *AnalysisTask, cfg AnalysisConfig, perspectives []Perspective, specialistResults []StageResult) []StageResult {
	task.mu.RLock()
	taskID := task.ID
	task.mu.RUnlock()

	// Build all interview commands upfront — loads shared context once,
	// reads specialist findings from disk for each perspective, and skips
	// perspectives without valid findings.
	commands, err := BuildAllInterviewCommands(cfg, perspectives, specialistResults)
	if err != nil {
		log.Printf("[%s] Interview stage: failed to build commands: %v", taskID, err)
		// Return failure results for all successful specialist results
		results := make([]StageResult, 0)
		for _, r := range specialistResults {
			if r.Err == nil {
				results = append(results, StageResult{Err: err})
			}
		}
		return results
	}

	jobs := make([]ParallelJob, len(commands))
	for i, cmd := range commands {
		cmd := cmd // capture for closure
		jobs[i] = ParallelJob{
			PerspectiveID: cmd.PerspectiveID,
			Fn: func(ctx context.Context) StageResult {
				err := runInterviewSession(ctx, task, cmd)
				if err != nil {
					return StageResult{Err: err}
				}
				return StageResult{
					OutputPath: cmd.OutputPath,
				}
			},
		}
	}

	executor := &ParallelExecutor{
		Concurrency: DefaultConcurrencyLimit,
		RetryLimit:  2, // 1 initial + 1 retry
		JobTimeout:  DefaultJobTimeout,
		OnJobComplete: func(perspectiveID string, success bool, attempts int) {
			if success {
				task.IncrStageCompleted(StageInterview)
				log.Printf("[%s] Interview %s completed (attempts: %d)", taskID, perspectiveID, attempts)
			} else {
				task.IncrStageFailed(StageInterview)
				log.Printf("[%s] Interview %s failed after %d attempts", taskID, perspectiveID, attempts)
			}
		},
	}

	pr := executor.Execute(task.Ctx, jobs)
	return pr.Results
}

// runSynthesisStage generates the final report from verified findings.
// Returns the path to the generated report file.
func runSynthesisStage(task *AnalysisTask, cfg AnalysisConfig, perspectives []Perspective, interviewResults []StageResult) (string, error) {
	task.mu.RLock()
	reportDir := task.ReportDir
	task.mu.RUnlock()

	reportPath := filepath.Join(reportDir, "report.md")

	if err := runReportGeneration(task, cfg, perspectives, reportPath); err != nil {
		return "", fmt.Errorf("report generation: %w", err)
	}

	return reportPath, nil
}

// --- Stub functions for CLI subprocess calls (to be implemented in subsequent ACs) ---
// Stage 1 functions (runSeedAnalysis, runDAReviewLoop, runPerspectiveGeneration)
// are implemented in stage1_exec.go.

// runSpecialistSession is implemented in stage2_exec.go

// runInterviewSession runs a single verification/interview session via claude CLI subprocess.
// It takes a pre-built InterviewCommand (constructed via BuildAllInterviewCommands)
// to avoid redundant LoadInterviewContext calls per perspective.
//
// The subprocess runs in the perspective-specific working directory with no shared state,
// making it safe for concurrent execution via goroutines.
//
// The ctx parameter carries the per-job timeout set by the ParallelExecutor. This function
// does NOT create its own timeout — the executor manages timeouts centrally to ensure
// consistent behavior across all parallel jobs.
func runInterviewSession(ctx context.Context, task *AnalysisTask, cmd InterviewCommand) error {
	log.Printf("[%s] Interview %s: starting CLI subprocess (model=%s, maxTurns=%d, workDir=%s)",
		task.ID, cmd.PerspectiveID, cmd.Model, cmd.MaxTurns, cmd.WorkDir)

	// Run claude CLI with tool access and structured output.
	// The ctx already carries a per-job timeout from the ParallelExecutor.
	rawOutput, err := queryLLMScopedWithToolsAndSchema(
		ctx,
		cmd.WorkDir,
		cmd.Model,
		cmd.JSONSchema,
		cmd.SystemPrompt,
		cmd.UserPrompt,
		cmd.MaxTurns,
	)
	if err != nil {
		return fmt.Errorf("interview %s subprocess: %w", cmd.PerspectiveID, err)
	}

	// Extract JSON from potentially wrapped output
	jsonStr, err := extractJSON(rawOutput)
	if err != nil {
		return fmt.Errorf("extract interview %s JSON: %w (raw length: %d)", cmd.PerspectiveID, err, len(rawOutput))
	}

	// Parse into VerifiedFindings struct
	var verified VerifiedFindings
	if err := json.Unmarshal([]byte(jsonStr), &verified); err != nil {
		return fmt.Errorf("parse interview %s verified findings: %w", cmd.PerspectiveID, err)
	}

	// Validate: must have at least one finding
	if len(verified.Findings) == 0 {
		return fmt.Errorf("interview %s produced no verified findings", cmd.PerspectiveID)
	}

	// Ensure analyst field matches perspective ID
	if verified.Analyst == "" {
		verified.Analyst = cmd.PerspectiveID
	}

	// Write verified-findings.json to the perspective directory
	if err := WriteVerifiedFindings(cmd.OutputPath, verified); err != nil {
		return fmt.Errorf("write interview %s verified findings: %w", cmd.PerspectiveID, err)
	}

	log.Printf("[%s] Interview %s: completed with verdict=%s, score=%.2f, %d findings → %s",
		task.ID, cmd.PerspectiveID, verified.Verdict, verified.Score.WeightedTotal,
		len(verified.Findings), cmd.OutputPath)

	return nil
}

// runReportGeneration runs the synthesis/report generation via a single claude CLI subprocess.
// It loads all collected data from prior stages (findings, verifications), builds a comprehensive
// synthesis prompt with the report template, and invokes a single claude CLI to produce
// the final analysis report. The report is validated for required sections and written to disk.
// Implemented in stage4_exec.go via runSynthesisSession.
func runReportGeneration(task *AnalysisTask, cfg AnalysisConfig, perspectives []Perspective, reportPath string) error {
	// Synthesis timeout: 30 minutes
	ctx, cancel := context.WithTimeout(task.Ctx, 30*time.Minute)
	defer cancel()
	return runSynthesisSession(ctx, task, cfg, perspectives, reportPath)
}

// loadInjectedPerspectives reads a JSON file containing an array of Perspective objects
// to be merged into the generated perspective set after stage1.
func loadInjectedPerspectives(filePath string) ([]Perspective, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read perspective injection file: %w", err)
	}

	var perspectives []Perspective
	if err := json.Unmarshal(data, &perspectives); err != nil {
		return nil, fmt.Errorf("parse perspective injection file: %w", err)
	}

	return perspectives, nil
}

// mergeInjectedPerspectives appends injected perspectives to the generated set,
// skipping any that have duplicate IDs with already-generated perspectives.
// Injected perspectives are appended at the end to preserve the generated ordering.
func mergeInjectedPerspectives(generated, injected []Perspective) []Perspective {
	// Build a set of existing IDs for dedup
	existingIDs := make(map[string]bool, len(generated))
	for _, p := range generated {
		existingIDs[p.ID] = true
	}

	merged := make([]Perspective, len(generated))
	copy(merged, generated)

	for _, p := range injected {
		if existingIDs[p.ID] {
			log.Printf("Perspective injection: skipping duplicate id %q (already in generated set)", p.ID)
			continue
		}
		merged = append(merged, p)
		existingIDs[p.ID] = true
	}

	return merged
}
