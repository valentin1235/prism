package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"time"

	"github.com/heechul/prism-mcp/internal/engine"
	"github.com/heechul/prism-mcp/internal/parallel"
	taskpkg "github.com/heechul/prism-mcp/internal/task"
)

// RunAnalysisPipeline executes the 4-stage analysis pipeline in a background goroutine.
// Each stage transition updates the task's thread-safe in-memory state so that
// prism_task_status callers can observe progress in real time.
//
// Pipeline stages:
//  1. Scope: seed analysis -> DA review -> perspective generation
//  2. Specialist: parallel finding sessions (one per perspective)
//  3. Interview: parallel verification sessions (one per perspective)
//  4. Synthesis: report generation from verified findings
func RunAnalysisPipeline(task *taskpkg.AnalysisTask) {
	// Ensure cancel is called when pipeline exits to release resources.
	defer func() {
		if task.Cancel != nil {
			task.Cancel()
		}
	}()

	task.SetStatus(taskpkg.TaskStatusRunning)
	log.Printf("[%s] Pipeline started", task.ID)

	// --- Read config ---
	stateDir := task.GetStateDir()

	cfg, err := ReadAnalysisConfig(stateDir)
	if err != nil {
		task.FailStage(taskpkg.StageScope, fmt.Sprintf("config read error: %v", err))
		task.SetError(fmt.Sprintf("failed to read config: %v", err))
		log.Printf("[%s] Pipeline failed: %v", task.ID, err)
		return
	}

	// ============================
	// Stage 1: Scope
	// ============================
	task.StartStage(taskpkg.StageScope, "starting seed analysis")
	log.Printf("[%s] Stage scope: started", task.ID)

	perspectives, err := runScopeStage(task, cfg)
	if err != nil {
		task.FailStage(taskpkg.StageScope, fmt.Sprintf("scope failed: %v", err))
		task.SetError(fmt.Sprintf("scope stage failed: %v", err))
		log.Printf("[%s] Stage scope: FAILED — %v", task.ID, err)
		return
	}
	if err := writeLegacyContextArtifact(task.GetStateDir(), cfg); err != nil {
		task.FailStage(taskpkg.StageScope, fmt.Sprintf("context artifact failed: %v", err))
		task.SetError(fmt.Sprintf("scope compatibility artifacts failed: %v", err))
		log.Printf("[%s] Stage scope: FAILED writing context artifact — %v", task.ID, err)
		return
	}

	task.CompleteStage(taskpkg.StageScope, fmt.Sprintf("%d perspectives generated", len(perspectives)))
	log.Printf("[%s] Stage scope: completed with %d perspectives", task.ID, len(perspectives))

	// ============================
	// Stage 2: Specialist (parallel)
	// ============================
	numPerspectives := len(perspectives)
	task.StartStage(taskpkg.StageSpecialist, fmt.Sprintf("launching %d specialists", numPerspectives))
	task.SetStageParallel(taskpkg.StageSpecialist, numPerspectives)
	log.Printf("[%s] Stage specialist: started with %d perspectives", task.ID, numPerspectives)

	specialistResults := runSpecialistStage(task, cfg, perspectives)

	// Collect and aggregate specialist results
	collected := CollectSpecialistResults(task.ID, specialistResults, perspectives)

	// Persist collected findings for downstream stages
	collectStateDir := task.GetStateDir()

	if err := WriteCollectedFindings(collectStateDir, collected); err != nil {
		log.Printf("[%s] Warning: failed to persist collected findings: %v", task.ID, err)
		// Non-fatal — downstream stages can still use in-memory collected findings
	}

	// All specialists failed -> abort
	if collected.Succeeded == 0 {
		task.FailStage(taskpkg.StageSpecialist, fmt.Sprintf("all %d specialists failed", collected.Failed))
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
	task.CompleteStage(taskpkg.StageSpecialist, detail)
	log.Printf("[%s] Stage specialist: completed — %s", task.ID, detail)

	// ============================
	// Stage 3: Interview (parallel)
	// ============================
	// Only interview perspectives that produced findings
	interviewCount := collected.Succeeded
	task.StartStage(taskpkg.StageInterview, fmt.Sprintf("launching %d interviews", interviewCount))
	task.SetStageParallel(taskpkg.StageInterview, interviewCount)
	log.Printf("[%s] Stage interview: started with %d verifiers", task.ID, interviewCount)

	interviewResults := runInterviewStage(task, cfg, perspectives, specialistResults)

	// Collect and aggregate interview results
	collectedVerifications := CollectInterviewResults(task.ID, interviewResults, perspectives)

	// Persist collected verifications for synthesis stage
	interviewStateDir := task.GetStateDir()

	if err := WriteCollectedVerifications(interviewStateDir, collectedVerifications); err != nil {
		log.Printf("[%s] Warning: failed to persist collected verifications: %v", task.ID, err)
		// Non-fatal — downstream stages can still use in-memory results
	}
	if err := writeLegacyVerificationArtifacts(interviewStateDir, perspectives, collectedVerifications); err != nil {
		task.FailStage(taskpkg.StageInterview, fmt.Sprintf("legacy verification artifacts failed: %v", err))
		task.SetError(fmt.Sprintf("interview compatibility artifacts failed: %v", err))
		log.Printf("[%s] Stage interview: FAILED writing compatibility artifacts — %v", task.ID, err)
		return
	}

	// All interviews failed -> still proceed with unverified findings (degraded)
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
	task.CompleteStage(taskpkg.StageInterview, intDetail)
	log.Printf("[%s] Stage interview: completed — %s", task.ID, intDetail)

	// ============================
	// Stage 4: Synthesis
	// ============================
	task.StartStage(taskpkg.StageSynthesis, "generating report")
	log.Printf("[%s] Stage synthesis: started", task.ID)

	reportPath, err := runSynthesisStage(task, cfg, perspectives, interviewResults)
	if err != nil {
		task.FailStage(taskpkg.StageSynthesis, fmt.Sprintf("synthesis failed: %v", err))
		task.SetError(fmt.Sprintf("synthesis stage failed: %v", err))
		log.Printf("[%s] Stage synthesis: FAILED — %v", task.ID, err)
		return
	}

	if err := writeLegacyStateReportCopy(task.GetStateDir(), reportPath); err != nil {
		task.FailStage(taskpkg.StageSynthesis, fmt.Sprintf("state report copy failed: %v", err))
		task.SetError(fmt.Sprintf("synthesis compatibility artifacts failed: %v", err))
		log.Printf("[%s] Stage synthesis: FAILED writing state report copy — %v", task.ID, err)
		return
	}
	task.CompleteStage(taskpkg.StageSynthesis, fmt.Sprintf("report at %s", reportPath))
	task.SetReportPath(reportPath)
	log.Printf("[%s] Pipeline completed — report: %s", task.ID, reportPath)
}

// runScopeStage executes the scope stage: seed analysis -> DA review -> perspective generation.
// Returns the generated perspectives or an error.
func runScopeStage(task *taskpkg.AnalysisTask, cfg AnalysisConfig) ([]Perspective, error) {
	stateDir := task.GetStateDir()

	// Sub-step 1: Seed analysis
	task.UpdateStageDetail(taskpkg.StageScope, "running seed analysis")
	if err := RunSeedAnalysis(task, cfg); err != nil {
		return nil, fmt.Errorf("seed analysis: %w", err)
	}

	// Sub-step 2: DA review loop (up to 1 round)
	task.UpdateStageDetail(taskpkg.StageScope, "seed complete, running DA review")
	if err := RunDAReviewLoop(task, cfg); err != nil {
		return nil, fmt.Errorf("DA review: %w", err)
	}

	// Sub-step 3: Perspective generation
	task.UpdateStageDetail(taskpkg.StageScope, "DA review complete, generating perspectives")
	if err := RunPerspectiveGeneration(task, cfg); err != nil {
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
		task.UpdateStageDetail(taskpkg.StageScope, "merging injected perspectives")
		injected, err := LoadInjectedPerspectives(cfg.PerspectiveInjection)
		if err != nil {
			// Non-fatal: log warning and continue with generated perspectives only
			log.Printf("[%s] Warning: failed to load injected perspectives from %s: %v — continuing without injection",
				task.ID, cfg.PerspectiveInjection, err)
		} else if len(injected) > 0 {
			originalLen := len(pf.Perspectives)
			pf.Perspectives = MergeInjectedPerspectives(pf.Perspectives, injected)
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
// pre-built commands are passed to each RunSpecialistSession.
// Updates task parallel progress counters as specialists complete.
func runSpecialistStage(task *taskpkg.AnalysisTask, cfg AnalysisConfig, perspectives []Perspective) []StageResult {
	taskID := task.GetID()

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

	jobs := make([]parallel.ParallelJob, len(commands))
	for i, c := range commands {
		cmd := c // capture for closure
		jobs[i] = parallel.ParallelJob{
			PerspectiveID: cmd.PerspectiveID,
			Fn: func(ctx context.Context) (string, error) {
				err := RunSpecialistSession(ctx, task, cmd)
				if err != nil {
					return "", err
				}
				return cmd.OutputPath, nil
			},
		}
	}

	executor := &parallel.ParallelExecutor{
		Concurrency: parallel.DefaultConcurrencyLimit,
		// RetryLimit uses default (5, matching Python _MAX_RETRIES)
		JobTimeout: parallel.DefaultJobTimeout,
		OnJobComplete: func(perspectiveID string, success bool, attempts int) {
			if success {
				task.IncrStageCompleted(taskpkg.StageSpecialist)
				log.Printf("[%s] Specialist %s completed (attempts: %d)", taskID, perspectiveID, attempts)
			} else {
				task.IncrStageFailed(taskpkg.StageSpecialist)
				log.Printf("[%s] Specialist %s failed after %d attempts", taskID, perspectiveID, attempts)
			}
		},
	}

	pr := executor.Execute(task.Ctx, jobs)
	return JobResultsToStageResults(pr.Results)
}

// runInterviewStage executes parallel verification sessions for perspectives
// that produced findings. Uses ParallelExecutor for concurrency limiting.
// Updates task parallel progress counters.
func runInterviewStage(task *taskpkg.AnalysisTask, cfg AnalysisConfig, perspectives []Perspective, specialistResults []StageResult) []StageResult {
	taskID := task.GetID()

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

	jobs := make([]parallel.ParallelJob, len(commands))
	for i, cmd := range commands {
		cmd := cmd // capture for closure
		jobs[i] = parallel.ParallelJob{
			PerspectiveID: cmd.PerspectiveID,
			Fn: func(ctx context.Context) (string, error) {
				err := runInterviewSession(ctx, task, cmd)
				if err != nil {
					return "", err
				}
				return cmd.OutputPath, nil
			},
		}
	}

	executor := &parallel.ParallelExecutor{
		Concurrency: parallel.DefaultConcurrencyLimit,
		// RetryLimit uses default (5, matching Python _MAX_RETRIES)
		JobTimeout: parallel.DefaultJobTimeout,
		OnJobComplete: func(perspectiveID string, success bool, attempts int) {
			if success {
				task.IncrStageCompleted(taskpkg.StageInterview)
				log.Printf("[%s] Interview %s completed (attempts: %d)", taskID, perspectiveID, attempts)
			} else {
				task.IncrStageFailed(taskpkg.StageInterview)
				log.Printf("[%s] Interview %s failed after %d attempts", taskID, perspectiveID, attempts)
			}
		},
	}

	pr := executor.Execute(task.Ctx, jobs)
	return JobResultsToStageResults(pr.Results)
}

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
func runInterviewSession(ctx context.Context, task *taskpkg.AnalysisTask, cmd InterviewCommand) error {
	log.Printf("[%s] Interview %s: starting CLI subprocess (model=%s, maxTurns=%d, workDir=%s)",
		task.ID, cmd.PerspectiveID, cmd.Model, cmd.MaxTurns, cmd.WorkDir)

	// Run claude CLI with tool access and structured output.
	// The ctx already carries a per-job timeout from the ParallelExecutor.
	rawOutput, err := engine.QueryLLMScopedWithToolsAndSchema(
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
	jsonStr, err := engine.ExtractJSON(rawOutput)
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

	log.Printf("[%s] Interview %s: completed with verdict=%s, score=%.2f, %d findings -> %s",
		task.ID, cmd.PerspectiveID, verified.Verdict, verified.Score.WeightedTotal,
		len(verified.Findings), cmd.OutputPath)

	return nil
}

// runSynthesisStage generates the final report from verified findings.
// Returns the path to the generated report file.
func runSynthesisStage(task *taskpkg.AnalysisTask, cfg AnalysisConfig, perspectives []Perspective, interviewResults []StageResult) (string, error) {
	reportDir := task.GetReportDir()

	reportPath := filepath.Join(reportDir, "report.md")

	if err := runReportGeneration(task, cfg, perspectives, reportPath); err != nil {
		return "", fmt.Errorf("report generation: %w", err)
	}

	return reportPath, nil
}

// runReportGeneration runs the synthesis/report generation via a single claude CLI subprocess.
// It loads all collected data from prior stages (findings, verifications), builds a comprehensive
// synthesis prompt with the report template, and invokes a single claude CLI to produce
// the final analysis report. The report is validated for required sections and written to disk.
// Implemented via RunSynthesisSession.
func runReportGeneration(task *taskpkg.AnalysisTask, cfg AnalysisConfig, perspectives []Perspective, reportPath string) error {
	// Synthesis timeout: 30 minutes
	ctx, cancel := context.WithTimeout(task.Ctx, 30*time.Minute)
	defer cancel()
	return RunSynthesisSession(ctx, task, cfg, perspectives, reportPath)
}
