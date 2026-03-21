package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SpecialistOutcome classifies how a specialist session ended.
type SpecialistOutcome string

const (
	OutcomeSuccess      SpecialistOutcome = "success"
	OutcomeTimeout      SpecialistOutcome = "timeout"
	OutcomeCrashed      SpecialistOutcome = "crashed"
	OutcomeParseError   SpecialistOutcome = "parse_error"
	OutcomeEmptyOutput  SpecialistOutcome = "empty_output"
	OutcomeCancelled    SpecialistOutcome = "cancelled"
	OutcomeRetryFailed  SpecialistOutcome = "retry_exhausted"
)

// SpecialistResult tracks the outcome of a single specialist's analysis,
// including parsed findings on success or classified error on failure.
type SpecialistResult struct {
	PerspectiveID string            `json:"perspective_id"`
	Outcome       SpecialistOutcome `json:"outcome"`
	Findings      *SpecialistFindings `json:"findings,omitempty"`
	FindingsCount int               `json:"findings_count"`
	OutputPath    string            `json:"output_path,omitempty"`
	ErrorMessage  string            `json:"error_message,omitempty"`
	ErrorClass    string            `json:"error_class,omitempty"`
	// Skipped is true when the specialist was skipped after retry exhaustion.
	// A skipped specialist means the perspective is missing from the analysis.
	Skipped bool `json:"skipped"`
}

// CollectedFindings is the unified aggregation of all specialist outputs.
// It is written to collected-findings.json in the task state directory
// and consumed by the interview and synthesis stages.
type CollectedFindings struct {
	// TaskID is the analysis task identifier.
	TaskID string `json:"task_id"`

	// CollectedAt is the timestamp when collection completed.
	CollectedAt time.Time `json:"collected_at"`

	// Summary counts for quick status checks.
	TotalSpecialists int `json:"total_specialists"`
	Succeeded        int `json:"succeeded"`
	Failed           int `json:"failed"`
	TotalFindings    int `json:"total_findings"`

	// Per-specialist results with parsed findings or error details.
	Results []SpecialistResult `json:"results"`

	// AllFindings is the flattened list of all findings from all successful
	// specialists, annotated with the source perspective ID.
	AllFindings []AnnotatedFinding `json:"all_findings"`

	// FailedSpecialists lists the perspective IDs that failed, along with
	// their error classification. Used for degradation notices in reports.
	FailedSpecialists []FailedSpecialist `json:"failed_specialists,omitempty"`

	// SkippedPerspectives lists perspectives that were skipped after retry
	// exhaustion. These are a subset of FailedSpecialists where the outcome
	// is specifically OutcomeRetryFailed. Recorded separately so the synthesis
	// stage can explicitly note which perspectives are missing and why.
	SkippedPerspectives []SkippedPerspective `json:"skipped_perspectives,omitempty"`

	// Degraded is true when at least one specialist failed but the analysis
	// can continue with partial results.
	Degraded bool `json:"degraded"`
}

// AnnotatedFinding is a single finding with its source perspective ID attached.
// Used in the flattened AllFindings list for cross-perspective analysis.
type AnnotatedFinding struct {
	PerspectiveID string `json:"perspective_id"`
	Finding       string `json:"finding"`
	Evidence      string `json:"evidence"`
	Severity      string `json:"severity"`
}

// FailedSpecialist records a specialist that failed with classification.
type FailedSpecialist struct {
	PerspectiveID string            `json:"perspective_id"`
	Outcome       SpecialistOutcome `json:"outcome"`
	ErrorMessage  string            `json:"error_message"`
}

// SkippedPerspective records a perspective that was marked as skipped after
// retry exhaustion. It captures which perspective is missing from the analysis
// and the underlying failure reason for inclusion in degradation notices.
type SkippedPerspective struct {
	PerspectiveID string `json:"perspective_id"`
	Reason        string `json:"reason"` // human-readable failure reason
	ErrorMessage  string `json:"error_message"` // original error detail
}

// CollectSpecialistResults aggregates parallel specialist outputs into a
// unified CollectedFindings structure. For each StageResult:
//   - Success: reads and parses findings.json from OutputPath
//   - Failure: classifies the error (timeout, crash, parse error, etc.)
//
// Single specialist failure does not block the collection — the result is
// marked as degraded with explicit notation of which specialists failed.
func CollectSpecialistResults(taskID string, stageResults []StageResult, perspectives []Perspective) CollectedFindings {
	now := time.Now().UTC()
	collected := CollectedFindings{
		TaskID:           taskID,
		CollectedAt:      now,
		TotalSpecialists: len(stageResults),
		Results:          make([]SpecialistResult, 0, len(stageResults)),
		AllFindings:      make([]AnnotatedFinding, 0),
	}

	for i, sr := range stageResults {
		perspID := sr.PerspectiveID
		if perspID == "" && i < len(perspectives) {
			perspID = perspectives[i].ID
		}

		result := processSpecialistResult(perspID, sr)
		collected.Results = append(collected.Results, result)

		if result.Outcome == OutcomeSuccess && result.Findings != nil {
			collected.Succeeded++
			collected.TotalFindings += result.FindingsCount

			// Flatten findings into AllFindings with perspective annotation
			for _, f := range result.Findings.Findings {
				collected.AllFindings = append(collected.AllFindings, AnnotatedFinding{
					PerspectiveID: perspID,
					Finding:       f.Finding,
					Evidence:      f.Evidence,
					Severity:      f.Severity,
				})
			}
		} else {
			collected.Failed++
			collected.FailedSpecialists = append(collected.FailedSpecialists, FailedSpecialist{
				PerspectiveID: perspID,
				Outcome:       result.Outcome,
				ErrorMessage:  result.ErrorMessage,
			})

			// Mark as skipped if retry was exhausted — this perspective is
			// missing from the analysis and needs explicit notation.
			if result.Outcome == OutcomeRetryFailed {
				result.Skipped = true
				collected.Results[len(collected.Results)-1] = result // update the stored result
				collected.SkippedPerspectives = append(collected.SkippedPerspectives, SkippedPerspective{
					PerspectiveID: perspID,
					Reason:        outcomeDescription(result.Outcome),
					ErrorMessage:  result.ErrorMessage,
				})
			}
		}
	}

	collected.Degraded = collected.Failed > 0 && collected.Succeeded > 0

	return collected
}

// processSpecialistResult processes a single StageResult into a SpecialistResult.
// On success, reads and parses the findings.json file.
// On failure, classifies the error into a specific outcome category.
func processSpecialistResult(perspectiveID string, sr StageResult) SpecialistResult {
	result := SpecialistResult{
		PerspectiveID: perspectiveID,
		OutputPath:    sr.OutputPath,
	}

	// Handle error case
	if sr.Err != nil {
		result.Outcome, result.ErrorClass = classifyError(sr.Err)
		result.ErrorMessage = sr.Err.Error()
		return result
	}

	// Handle missing output path
	if sr.OutputPath == "" {
		result.Outcome = OutcomeEmptyOutput
		result.ErrorClass = "no_output_path"
		result.ErrorMessage = "specialist completed but produced no output path"
		return result
	}

	// Try to read and parse findings
	findings, err := ReadSpecialistFindings(sr.OutputPath)
	if err != nil {
		result.Outcome = OutcomeParseError
		result.ErrorClass = "findings_read_error"
		result.ErrorMessage = fmt.Sprintf("failed to read findings from %s: %v", sr.OutputPath, err)
		return result
	}

	// Validate findings are non-empty
	if len(findings.Findings) == 0 {
		result.Outcome = OutcomeEmptyOutput
		result.ErrorClass = "empty_findings"
		result.ErrorMessage = fmt.Sprintf("specialist %s produced zero findings", perspectiveID)
		return result
	}

	// Success
	result.Outcome = OutcomeSuccess
	result.Findings = &findings
	result.FindingsCount = len(findings.Findings)
	return result
}

// classifyError categorizes an error from a specialist subprocess into a
// specific SpecialistOutcome and human-readable error class string.
// This enables the synthesis stage to produce specific degradation notices.
func classifyError(err error) (SpecialistOutcome, string) {
	msg := err.Error()

	// Retry exhaustion check FIRST — the wrapped error may contain timeout/crash
	// strings from the underlying cause, but the primary classification should
	// be retry_exhausted when the ParallelExecutor has given up.
	if strings.Contains(msg, "all attempts failed") ||
		strings.Contains(msg, "no more retries") {
		return OutcomeRetryFailed, "retry_exhausted"
	}

	// Context cancellation / timeout
	if strings.Contains(msg, "context deadline exceeded") ||
		strings.Contains(msg, "context canceled") ||
		strings.Contains(msg, "timed out") ||
		strings.Contains(msg, "timeout") {
		if strings.Contains(msg, "deadline exceeded") || strings.Contains(msg, "timed out") || strings.Contains(msg, "timeout") {
			return OutcomeTimeout, "subprocess_timeout"
		}
		return OutcomeCancelled, "context_cancelled"
	}

	// Process crash signals
	if strings.Contains(msg, "signal:") ||
		strings.Contains(msg, "exit status") ||
		strings.Contains(msg, "killed") ||
		strings.Contains(msg, "broken pipe") {
		return OutcomeCrashed, "subprocess_crash"
	}

	// JSON parse errors
	if strings.Contains(msg, "unmarshal") ||
		strings.Contains(msg, "invalid character") ||
		strings.Contains(msg, "unexpected end of JSON") ||
		strings.Contains(msg, "parse") {
		return OutcomeParseError, "output_parse_error"
	}

	// Default: treat as crash
	return OutcomeCrashed, "unknown_error"
}

// CollectedFindingsPath returns the path to collected-findings.json
// within the task's state directory.
func CollectedFindingsPath(stateDir string) string {
	return filepath.Join(stateDir, "collected-findings.json")
}

// WriteCollectedFindings persists the collected findings to disk as formatted JSON.
// Written to ~/.prism/state/{context_id}/collected-findings.json for consumption
// by interview and synthesis stages.
func WriteCollectedFindings(stateDir string, cf CollectedFindings) error {
	path := CollectedFindingsPath(stateDir)
	data, err := json.MarshalIndent(cf, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal collected findings: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write collected findings: %w", err)
	}
	return nil
}

// ReadCollectedFindings reads collected-findings.json from disk.
func ReadCollectedFindings(stateDir string) (CollectedFindings, error) {
	var cf CollectedFindings
	path := CollectedFindingsPath(stateDir)
	data, err := os.ReadFile(path)
	if err != nil {
		return cf, fmt.Errorf("read collected findings: %w", err)
	}
	if err := json.Unmarshal(data, &cf); err != nil {
		return cf, fmt.Errorf("parse collected findings: %w", err)
	}
	return cf, nil
}

// SuccessfulPerspectiveIDs returns the perspective IDs that produced valid findings.
// Used to determine which perspectives proceed to the interview stage.
func (cf *CollectedFindings) SuccessfulPerspectiveIDs() []string {
	ids := make([]string, 0, cf.Succeeded)
	for _, r := range cf.Results {
		if r.Outcome == OutcomeSuccess {
			ids = append(ids, r.PerspectiveID)
		}
	}
	return ids
}

// DegradationNotice generates a human-readable summary of failed specialists
// for inclusion in the final report. Returns empty string if no degradation.
func (cf *CollectedFindings) DegradationNotice() string {
	if !cf.Degraded && cf.Failed == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("**Note:** %d of %d specialist analyses could not be completed:\n\n",
		cf.Failed, cf.TotalSpecialists))

	for _, fs := range cf.FailedSpecialists {
		reason := outcomeDescription(fs.Outcome)
		skippedLabel := ""
		if fs.Outcome == OutcomeRetryFailed {
			skippedLabel = " [SKIPPED]"
		}
		sb.WriteString(fmt.Sprintf("- **%s**: %s%s\n", fs.PerspectiveID, reason, skippedLabel))
	}

	sb.WriteString("\nFindings from the remaining specialists are included below. ")
	sb.WriteString("The analysis may have reduced coverage in the areas these specialists would have examined.\n")

	return sb.String()
}

// InterviewOutcome classifies how an interview/verification session ended.
type InterviewOutcome string

const (
	InterviewSuccess     InterviewOutcome = "success"
	InterviewTimeout     InterviewOutcome = "timeout"
	InterviewCrashed     InterviewOutcome = "crashed"
	InterviewParseError  InterviewOutcome = "parse_error"
	InterviewEmptyOutput InterviewOutcome = "empty_output"
	InterviewCancelled   InterviewOutcome = "cancelled"
	InterviewRetryFailed InterviewOutcome = "retry_exhausted"
)

// InterviewResult tracks the outcome of a single interview/verification session,
// including parsed verified findings on success or classified error on failure.
type InterviewResult struct {
	PerspectiveID string            `json:"perspective_id"`
	Outcome       InterviewOutcome  `json:"outcome"`
	Verified      *VerifiedFindings `json:"verified,omitempty"`
	FindingsCount int               `json:"findings_count"`
	OutputPath    string            `json:"output_path,omitempty"`
	Verdict       string            `json:"verdict,omitempty"`
	Score         float64           `json:"score,omitempty"`
	ErrorMessage  string            `json:"error_message,omitempty"`
	ErrorClass    string            `json:"error_class,omitempty"`
	// Skipped is true when the interview was skipped after retry exhaustion.
	// A skipped interview means findings for this perspective are unverified.
	Skipped bool `json:"skipped"`
}

// CollectedVerifications is the unified aggregation of all interview outputs.
// Written to collected-verifications.json in the task state directory
// and consumed by the synthesis stage for report generation.
type CollectedVerifications struct {
	// TaskID is the analysis task identifier.
	TaskID string `json:"task_id"`

	// CollectedAt is the timestamp when collection completed.
	CollectedAt time.Time `json:"collected_at"`

	// Summary counts for quick status checks.
	TotalInterviews int `json:"total_interviews"`
	Succeeded       int `json:"succeeded"`
	Failed          int `json:"failed"`

	// Per-interview results with parsed verified findings or error details.
	Results []InterviewResult `json:"results"`

	// VerifiedFindings is the flattened list of all verified findings from all
	// successful interviews, annotated with the source perspective ID.
	VerifiedFindings []AnnotatedVerifiedFinding `json:"verified_findings"`

	// FailedInterviews lists the perspective IDs that failed, along with
	// their error classification. Used for degradation notices in reports.
	FailedInterviews []FailedInterview `json:"failed_interviews,omitempty"`

	// SkippedInterviews lists interviews that were skipped after retry
	// exhaustion. These perspectives have unverified findings only.
	SkippedInterviews []SkippedPerspective `json:"skipped_interviews,omitempty"`

	// Degraded is true when at least one interview failed but the analysis
	// can continue with unverified findings from the specialist stage.
	Degraded bool `json:"degraded"`

	// AverageScore is the weighted average verification score across all
	// successful interviews. Zero if no interviews succeeded.
	AverageScore float64 `json:"average_score"`
}

// AnnotatedVerifiedFinding is a single verified finding with its source
// perspective ID and verification status attached.
type AnnotatedVerifiedFinding struct {
	PerspectiveID string  `json:"perspective_id"`
	Finding       string  `json:"finding"`
	Evidence      string  `json:"evidence"`
	Severity      string  `json:"severity"`
	Status        string  `json:"status"`
	Verification  string  `json:"verification"`
	Score         float64 `json:"score,omitempty"`
}

// FailedInterview records an interview that failed with classification.
type FailedInterview struct {
	PerspectiveID string           `json:"perspective_id"`
	Outcome       InterviewOutcome `json:"outcome"`
	ErrorMessage  string           `json:"error_message"`
}

// CollectInterviewResults aggregates parallel interview/verification outputs into
// a unified CollectedVerifications structure. For each StageResult:
//   - Success: reads and parses verified-findings.json from OutputPath
//   - Failure: classifies the error (timeout, crash, parse error, etc.)
//
// Single interview failure does not block the collection — the result is
// marked as degraded with explicit notation of which interviews failed.
// Failed interviews mean the synthesis stage will use unverified specialist
// findings for those perspectives.
func CollectInterviewResults(taskID string, stageResults []StageResult, perspectives []Perspective) CollectedVerifications {
	now := time.Now().UTC()
	collected := CollectedVerifications{
		TaskID:           taskID,
		CollectedAt:      now,
		TotalInterviews:  len(stageResults),
		Results:          make([]InterviewResult, 0, len(stageResults)),
		VerifiedFindings: make([]AnnotatedVerifiedFinding, 0),
	}

	var totalScore float64
	var scoreCount int

	for i, sr := range stageResults {
		perspID := sr.PerspectiveID
		if perspID == "" && i < len(perspectives) {
			perspID = perspectives[i].ID
		}

		result := processInterviewResult(perspID, sr)
		collected.Results = append(collected.Results, result)

		if result.Outcome == InterviewSuccess && result.Verified != nil {
			collected.Succeeded++

			// Accumulate score for averaging
			totalScore += result.Score
			scoreCount++

			// Flatten verified findings with perspective annotation
			for _, f := range result.Verified.Findings {
				collected.VerifiedFindings = append(collected.VerifiedFindings, AnnotatedVerifiedFinding{
					PerspectiveID: perspID,
					Finding:       f.Finding,
					Evidence:      f.Evidence,
					Severity:      f.Severity,
					Status:        f.Status,
					Verification:  f.Verification,
					Score:         result.Score,
				})
			}
		} else {
			collected.Failed++
			collected.FailedInterviews = append(collected.FailedInterviews, FailedInterview{
				PerspectiveID: perspID,
				Outcome:       result.Outcome,
				ErrorMessage:  result.ErrorMessage,
			})

			// Mark as skipped if retry was exhausted — this interview's perspective
			// will only have unverified findings in the final report.
			if result.Outcome == InterviewRetryFailed {
				result.Skipped = true
				collected.Results[len(collected.Results)-1] = result
				collected.SkippedInterviews = append(collected.SkippedInterviews, SkippedPerspective{
					PerspectiveID: perspID,
					Reason:        interviewOutcomeDescription(result.Outcome),
					ErrorMessage:  result.ErrorMessage,
				})
			}
		}
	}

	collected.Degraded = collected.Failed > 0 && collected.Succeeded > 0

	if scoreCount > 0 {
		collected.AverageScore = totalScore / float64(scoreCount)
	}

	return collected
}

// processInterviewResult processes a single StageResult into an InterviewResult.
// On success, reads and parses the verified-findings.json file.
// On failure, classifies the error into a specific outcome category.
func processInterviewResult(perspectiveID string, sr StageResult) InterviewResult {
	result := InterviewResult{
		PerspectiveID: perspectiveID,
		OutputPath:    sr.OutputPath,
	}

	// Handle error case
	if sr.Err != nil {
		result.Outcome, result.ErrorClass = classifyInterviewError(sr.Err)
		result.ErrorMessage = sr.Err.Error()
		return result
	}

	// Handle missing output path
	if sr.OutputPath == "" {
		result.Outcome = InterviewEmptyOutput
		result.ErrorClass = "no_output_path"
		result.ErrorMessage = "interview completed but produced no output path"
		return result
	}

	// Try to read and parse verified findings
	verified, err := ReadVerifiedFindings(sr.OutputPath)
	if err != nil {
		result.Outcome = InterviewParseError
		result.ErrorClass = "verified_findings_read_error"
		result.ErrorMessage = fmt.Sprintf("failed to read verified findings from %s: %v", sr.OutputPath, err)
		return result
	}

	// Validate: must have at least one finding
	if len(verified.Findings) == 0 {
		result.Outcome = InterviewEmptyOutput
		result.ErrorClass = "empty_verified_findings"
		result.ErrorMessage = fmt.Sprintf("interview %s produced zero verified findings", perspectiveID)
		return result
	}

	// Success
	result.Outcome = InterviewSuccess
	result.Verified = &verified
	result.FindingsCount = len(verified.Findings)
	result.Verdict = verified.Verdict
	result.Score = verified.Score.WeightedTotal
	return result
}

// classifyInterviewError categorizes an error from an interview subprocess into a
// specific InterviewOutcome and human-readable error class string.
func classifyInterviewError(err error) (InterviewOutcome, string) {
	msg := err.Error()

	// Retry exhaustion check FIRST — the wrapped error may contain timeout/crash
	// strings from the underlying cause, but the primary classification should
	// be retry_exhausted when the ParallelExecutor has given up.
	if strings.Contains(msg, "all attempts failed") ||
		strings.Contains(msg, "no more retries") {
		return InterviewRetryFailed, "retry_exhausted"
	}

	// Context cancellation / timeout
	if strings.Contains(msg, "context deadline exceeded") ||
		strings.Contains(msg, "context canceled") ||
		strings.Contains(msg, "timed out") ||
		strings.Contains(msg, "timeout") {
		if strings.Contains(msg, "deadline exceeded") || strings.Contains(msg, "timed out") || strings.Contains(msg, "timeout") {
			return InterviewTimeout, "subprocess_timeout"
		}
		return InterviewCancelled, "context_cancelled"
	}

	// Process crash signals
	if strings.Contains(msg, "signal:") ||
		strings.Contains(msg, "exit status") ||
		strings.Contains(msg, "killed") ||
		strings.Contains(msg, "broken pipe") {
		return InterviewCrashed, "subprocess_crash"
	}

	// JSON parse errors
	if strings.Contains(msg, "unmarshal") ||
		strings.Contains(msg, "invalid character") ||
		strings.Contains(msg, "unexpected end of JSON") ||
		strings.Contains(msg, "parse") {
		return InterviewParseError, "output_parse_error"
	}

	// Default: treat as crash
	return InterviewCrashed, "unknown_error"
}

// CollectedVerificationsPath returns the path to collected-verifications.json
// within the task's state directory.
func CollectedVerificationsPath(stateDir string) string {
	return filepath.Join(stateDir, "collected-verifications.json")
}

// WriteCollectedVerifications persists the collected verifications to disk as formatted JSON.
// Written to ~/.prism/state/{context_id}/collected-verifications.json for consumption
// by the synthesis stage.
func WriteCollectedVerifications(stateDir string, cv CollectedVerifications) error {
	path := CollectedVerificationsPath(stateDir)
	data, err := json.MarshalIndent(cv, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal collected verifications: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write collected verifications: %w", err)
	}
	return nil
}

// ReadCollectedVerifications reads collected-verifications.json from disk.
func ReadCollectedVerifications(stateDir string) (CollectedVerifications, error) {
	var cv CollectedVerifications
	path := CollectedVerificationsPath(stateDir)
	data, err := os.ReadFile(path)
	if err != nil {
		return cv, fmt.Errorf("read collected verifications: %w", err)
	}
	if err := json.Unmarshal(data, &cv); err != nil {
		return cv, fmt.Errorf("parse collected verifications: %w", err)
	}
	return cv, nil
}

// SuccessfulInterviewIDs returns the perspective IDs that produced valid verified findings.
func (cv *CollectedVerifications) SuccessfulInterviewIDs() []string {
	ids := make([]string, 0, cv.Succeeded)
	for _, r := range cv.Results {
		if r.Outcome == InterviewSuccess {
			ids = append(ids, r.PerspectiveID)
		}
	}
	return ids
}

// InterviewDegradationNotice generates a human-readable summary of failed interviews
// for inclusion in the final report. Returns empty string if no degradation.
func (cv *CollectedVerifications) InterviewDegradationNotice() string {
	if !cv.Degraded && cv.Failed == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("**Note:** %d of %d verification interviews could not be completed:\n\n",
		cv.Failed, cv.TotalInterviews))

	for _, fi := range cv.FailedInterviews {
		reason := interviewOutcomeDescription(fi.Outcome)
		skippedLabel := ""
		if fi.Outcome == InterviewRetryFailed {
			skippedLabel = " [SKIPPED]"
		}
		sb.WriteString(fmt.Sprintf("- **%s**: %s%s\n", fi.PerspectiveID, reason, skippedLabel))
	}

	sb.WriteString("\nFindings from these perspectives are included without verification. ")
	sb.WriteString("Exercise additional caution with unverified findings.\n")

	return sb.String()
}

// ConfirmedFindings returns only verified findings with status "confirmed".
func (cv *CollectedVerifications) ConfirmedFindings() []AnnotatedVerifiedFinding {
	confirmed := make([]AnnotatedVerifiedFinding, 0)
	for _, f := range cv.VerifiedFindings {
		if f.Status == "confirmed" {
			confirmed = append(confirmed, f)
		}
	}
	return confirmed
}

// MissingPerspectivesReport generates a structured report of all perspectives that
// are absent or incomplete in the final analysis. This combines:
//   - Failed specialists (perspectives with no findings at all)
//   - Failed interviews (perspectives with unverified findings)
//
// The report is formatted as a markdown section ready for inclusion in the final report.
// Returns empty string if all perspectives completed successfully.
func MissingPerspectivesReport(cf *CollectedFindings, cv *CollectedVerifications) string {
	// Collect all missing/degraded entries
	type missingEntry struct {
		PerspectiveID string
		Stage         string // "Specialist Analysis" or "Socratic Verification"
		Reason        string
		Impact        string
	}

	var entries []missingEntry

	// Failed specialists — no findings at all for these perspectives
	if cf != nil {
		for _, fs := range cf.FailedSpecialists {
			entries = append(entries, missingEntry{
				PerspectiveID: fs.PerspectiveID,
				Stage:         "Specialist Analysis",
				Reason:        outcomeDescription(fs.Outcome),
				Impact:        "No findings produced — perspective entirely absent from analysis",
			})
		}
	}

	// Failed interviews — findings exist but are unverified
	if cv != nil {
		for _, fi := range cv.FailedInterviews {
			// Skip if this perspective already failed at specialist stage
			// (no point reporting interview failure for a perspective with no findings)
			alreadyFailed := false
			if cf != nil {
				for _, fs := range cf.FailedSpecialists {
					if fs.PerspectiveID == fi.PerspectiveID {
						alreadyFailed = true
						break
					}
				}
			}
			if alreadyFailed {
				continue
			}
			entries = append(entries, missingEntry{
				PerspectiveID: fi.PerspectiveID,
				Stage:         "Socratic Verification",
				Reason:        interviewOutcomeDescription(fi.Outcome),
				Impact:        "Findings included but unverified — treat with additional caution",
			})
		}
	}

	if len(entries) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## Missing Perspectives\n\n")
	sb.WriteString(fmt.Sprintf("**%d perspective(s)** could not be fully completed:\n\n", len(entries)))
	sb.WriteString("| Perspective | Failed Stage | Reason | Impact |\n")
	sb.WriteString("|-------------|-------------|--------|--------|\n")
	for _, e := range entries {
		sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n",
			e.PerspectiveID, e.Stage, e.Reason, e.Impact))
	}
	sb.WriteString("\n")

	return sb.String()
}

// interviewOutcomeDescription returns a human-readable description of an interview outcome.
func interviewOutcomeDescription(o InterviewOutcome) string {
	switch o {
	case InterviewTimeout:
		return "verification timed out before completion"
	case InterviewCrashed:
		return "verification subprocess terminated unexpectedly"
	case InterviewParseError:
		return "verification output could not be parsed"
	case InterviewEmptyOutput:
		return "verification produced no findings"
	case InterviewCancelled:
		return "verification was cancelled"
	case InterviewRetryFailed:
		return "verification failed after retry"
	default:
		return "verification failed for unknown reason"
	}
}

// HasMissingPerspectives returns true if there are any failed specialists or
// failed interviews that would result in a non-empty Missing Perspectives section.
func HasMissingPerspectives(cf *CollectedFindings, cv *CollectedVerifications) bool {
	if cf != nil && len(cf.FailedSpecialists) > 0 {
		return true
	}
	if cv != nil && len(cv.FailedInterviews) > 0 {
		// Check if any interview failures are for perspectives not already failed at specialist stage
		if cf == nil {
			return true
		}
		failedSpecIDs := make(map[string]bool)
		for _, fs := range cf.FailedSpecialists {
			failedSpecIDs[fs.PerspectiveID] = true
		}
		for _, fi := range cv.FailedInterviews {
			if !failedSpecIDs[fi.PerspectiveID] {
				return true
			}
		}
	}
	return false
}

// outcomeDescription returns a human-readable description of a specialist outcome.
func outcomeDescription(o SpecialistOutcome) string {
	switch o {
	case OutcomeTimeout:
		return "analysis timed out before completion"
	case OutcomeCrashed:
		return "analysis subprocess terminated unexpectedly"
	case OutcomeParseError:
		return "analysis output could not be parsed"
	case OutcomeEmptyOutput:
		return "analysis produced no findings"
	case OutcomeCancelled:
		return "analysis was cancelled"
	case OutcomeRetryFailed:
		return "analysis failed after retry"
	default:
		return "analysis failed for unknown reason"
	}
}
