package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/heechul/prism-mcp/internal/engine"
	taskpkg "github.com/heechul/prism-mcp/internal/task"
)

// SynthesisContext holds all data needed to build the synthesis prompt.
// Built once from the analysis config, perspectives, collected findings,
// and collected verifications before invoking the claude CLI subprocess.
type SynthesisContext struct {
	// Topic is the original analysis topic description.
	Topic string

	// Model is the fixed model for this analysis run.
	Model string

	// StateDir is the root state directory for this analysis.
	StateDir string

	// ReportDir is the output directory for the final report.
	ReportDir string

	// Perspectives are the generated analysis perspectives.
	Perspectives []Perspective

	// CollectedFindings is the aggregated specialist output.
	CollectedFindings *CollectedFindings

	// CollectedVerifications is the aggregated interview/verification output.
	CollectedVerifications *CollectedVerifications

	// SeedSummary is the research summary from seed analysis.
	SeedSummary string

	// OntologyScopeText is the rendered ontology scope text block.
	OntologyScopeText string

	// ReportTemplate is the report template content to fill.
	ReportTemplate string

	// Language is the target language for report output (e.g. "ko", "en", "ja").
	// When empty, the report defaults to English.
	Language string

	// AnalysisDate is the formatted date of this analysis.
	AnalysisDate string
}

// LoadSynthesisContext reads all inputs needed for synthesis from the analysis
// config and state directory. Returns a fully populated SynthesisContext.
func LoadSynthesisContext(cfg AnalysisConfig, perspectives []Perspective) (SynthesisContext, error) {
	ctx := SynthesisContext{
		Topic:        cfg.Topic,
		Model:        cfg.Model,
		StateDir:     cfg.StateDir,
		ReportDir:    cfg.ReportDir,
		Perspectives: perspectives,
		Language:     cfg.Language,
		AnalysisDate: time.Now().Format("2006-01-02"),
	}

	// Read seed analysis summary
	seedPath := SeedAnalysisPath(cfg.StateDir)
	seedData, err := os.ReadFile(seedPath)
	if err != nil {
		return ctx, fmt.Errorf("read seed analysis: %w", err)
	}

	var seedMap map[string]interface{}
	if err := json.Unmarshal(seedData, &seedMap); err == nil {
		if summary, ok := seedMap["summary"].(string); ok {
			ctx.SeedSummary = summary
		}
	}
	if ctx.SeedSummary == "" {
		ctx.SeedSummary = "(seed analysis summary unavailable)"
	}

	// Load ontology scope text
	ctx.OntologyScopeText = LoadOntologyScopeText(cfg.StateDir)

	// Read collected findings from disk
	cf, err := ReadCollectedFindings(cfg.StateDir)
	if err != nil {
		return ctx, fmt.Errorf("read collected findings: %w", err)
	}
	ctx.CollectedFindings = &cf

	// Read collected verifications from disk
	cv, err := ReadCollectedVerifications(cfg.StateDir)
	if err != nil {
		// Verifications may not exist if all interviews failed — proceed with unverified
		log.Printf("Warning: could not read collected verifications: %v", err)
		ctx.CollectedVerifications = nil
	} else {
		ctx.CollectedVerifications = &cv
	}

	// Load report template
	templateContent, err := loadReportTemplate(cfg)
	if err != nil {
		return ctx, fmt.Errorf("load report template: %w", err)
	}
	ctx.ReportTemplate = templateContent

	return ctx, nil
}

// loadReportTemplate reads the report template from the configured path
// or falls back to the default template in the skills directory.
func loadReportTemplate(cfg AnalysisConfig) (string, error) {
	// Check for custom report template in config
	if cfg.ReportTemplate != "" {
		data, err := os.ReadFile(cfg.ReportTemplate)
		if err != nil {
			return "", fmt.Errorf("read custom report template %s: %w", cfg.ReportTemplate, err)
		}
		return string(data), nil
	}

	// Fall back to default template
	defaultPath, err := ResolveRepoAssetPath("skills/analyze/templates/report.md")
	if err != nil {
		return "", fmt.Errorf("resolve default report template: %w", err)
	}

	data, err := os.ReadFile(defaultPath)
	if err != nil {
		return "", fmt.Errorf("read default report template %s: %w", defaultPath, err)
	}

	return string(data), nil
}

// BuildSynthesisSystemPrompt constructs the full system prompt for the
// synthesis claude CLI subprocess. Includes all collected data and the
// report template to fill.
func BuildSynthesisSystemPrompt(sctx SynthesisContext) string {
	var sb strings.Builder

	// --- Section 1: Synthesizer Role Identity ---
	sb.WriteString("You are the REPORT SYNTHESIZER for a multi-perspective analysis.\n\n")
	sb.WriteString("Your task is to produce a comprehensive analysis report by synthesizing findings ")
	sb.WriteString("from multiple specialist analysts and their Socratic verification results.\n\n")
	sb.WriteString("You MUST fill the provided report template completely. Do NOT leave any section empty — ")
	sb.WriteString("write 'N/A' only if a section is genuinely irrelevant.\n\n")

	// --- Section 2: Analysis Context ---
	sb.WriteString("## Analysis Context\n\n")
	sb.WriteString(fmt.Sprintf("**Topic:** %s\n\n", sctx.Topic))
	sb.WriteString(fmt.Sprintf("**Analysis Date:** %s\n\n", sctx.AnalysisDate))
	sb.WriteString(fmt.Sprintf("**Number of Perspectives:** %d\n\n", len(sctx.Perspectives)))

	sb.WriteString("**Seed Analysis Summary:**\n")
	sb.WriteString(sctx.SeedSummary)
	sb.WriteString("\n\n")

	// --- Section 3: Perspectives Used ---
	sb.WriteString("## Perspectives Used\n\n")
	for _, p := range sctx.Perspectives {
		sb.WriteString(fmt.Sprintf("### %s (ID: %s)\n", p.Name, p.ID))
		sb.WriteString(fmt.Sprintf("- **Scope:** %s\n", p.Scope))
		sb.WriteString(fmt.Sprintf("- **Rationale:** %s\n", p.Rationale))
		sb.WriteString("- **Key Questions:**\n")
		for _, q := range p.KeyQuestions {
			sb.WriteString(fmt.Sprintf("  - %s\n", q))
		}
		sb.WriteString("\n")
	}

	// --- Section 4: Specialist Findings ---
	sb.WriteString("## Specialist Findings Data\n\n")
	if sctx.CollectedFindings != nil {
		sb.WriteString(fmt.Sprintf("Total specialists: %d succeeded, %d failed, %d total findings\n\n",
			sctx.CollectedFindings.Succeeded,
			sctx.CollectedFindings.Failed,
			sctx.CollectedFindings.TotalFindings,
		))

		// Per-specialist findings
		for _, r := range sctx.CollectedFindings.Results {
			if r.Outcome == OutcomeSuccess && r.Findings != nil {
				sb.WriteString(fmt.Sprintf("### Findings from: %s\n\n", r.PerspectiveID))
				for i, f := range r.Findings.Findings {
					sb.WriteString(fmt.Sprintf("**Finding %d:** %s\n", i+1, f.Finding))
					sb.WriteString(fmt.Sprintf("- Evidence: %s\n", f.Evidence))
					sb.WriteString(fmt.Sprintf("- Severity: %s\n\n", f.Severity))
				}
			}
		}

		// Degradation notice for failed specialists
		if sctx.CollectedFindings.PartialFailure {
			sb.WriteString(sctx.CollectedFindings.DegradationNotice())
			sb.WriteString("\n")
		}
	}

	// --- Section 5: Verification Results ---
	sb.WriteString("## Verification Results Data\n\n")
	if sctx.CollectedVerifications != nil {
		sb.WriteString(fmt.Sprintf("Total interviews: %d succeeded, %d failed, avg score: %.2f\n\n",
			sctx.CollectedVerifications.Succeeded,
			sctx.CollectedVerifications.Failed,
			sctx.CollectedVerifications.AverageScore,
		))

		// Per-interview verification scores
		sb.WriteString("### Verification Scores\n\n")
		sb.WriteString("| Analyst | Verdict | Assumption | Relevance | Constraints | Weighted Total |\n")
		sb.WriteString("|---------|---------|------------|-----------|-------------|----------------|\n")
		for _, r := range sctx.CollectedVerifications.Results {
			if r.Outcome == InterviewSuccess && r.Verified != nil {
				sb.WriteString(fmt.Sprintf("| %s | %s | %.2f | %.2f | %.2f | %.2f |\n",
					r.PerspectiveID,
					r.Verdict,
					r.Verified.Score.Assumption,
					r.Verified.Score.Relevance,
					r.Verified.Score.Constraints,
					r.Verified.Score.WeightedTotal,
				))
			}
		}
		sb.WriteString("\n")

		// Verified findings with status
		sb.WriteString("### Verified Findings Detail\n\n")
		for _, r := range sctx.CollectedVerifications.Results {
			if r.Outcome == InterviewSuccess && r.Verified != nil {
				sb.WriteString(fmt.Sprintf("#### %s (Verdict: %s, Score: %.2f)\n\n",
					r.PerspectiveID, r.Verdict, r.Score))
				sb.WriteString(fmt.Sprintf("**Summary:** %s\n\n", r.Verified.Summary))

				for i, f := range r.Verified.Findings {
					sb.WriteString(fmt.Sprintf("**Finding %d [%s]:** %s\n", i+1, f.Status, f.Finding))
					sb.WriteString(fmt.Sprintf("- Evidence: %s\n", f.Evidence))
					sb.WriteString(fmt.Sprintf("- Verification: %s\n\n", f.Verification))
				}
			}
		}

		// Degradation notice for failed interviews
		if sctx.CollectedVerifications.PartialFailure {
			sb.WriteString(sctx.CollectedVerifications.InterviewDegradationNotice())
			sb.WriteString("\n")
		}
	} else {
		sb.WriteString("No verification data available. All interviews may have failed.\n")
		sb.WriteString("Use unverified specialist findings for the report, noting this limitation.\n\n")
	}

	// --- Section 5b: Missing Perspectives ---
	missingSection := MissingPerspectivesReport(sctx.CollectedFindings, sctx.CollectedVerifications)
	if missingSection != "" {
		sb.WriteString(missingSection)
	}

	// --- Section 6: Ontology Scope ---
	sb.WriteString("## Ontology Scope\n\n")
	sb.WriteString(sctx.OntologyScopeText)
	sb.WriteString("\n\n")

	// --- Section 7: Report Template ---
	sb.WriteString("---\n\n")
	sb.WriteString("# REPORT TEMPLATE — Fill this template completely\n\n")
	sb.WriteString(sctx.ReportTemplate)
	sb.WriteString("\n\n")

	// --- Section 8: Synthesis Instructions ---
	sb.WriteString("---\n\n")
	sb.WriteString("# Synthesis Instructions\n\n")
	sb.WriteString("1. Fill EVERY section of the template above using the provided data.\n")
	sb.WriteString("2. In **Perspective Findings**, include each specialist's findings organized by perspective.\n")
	sb.WriteString("3. In **Integrated Analysis**, identify convergences (where perspectives agree), ")
	sb.WriteString("divergences (where they disagree), and emergent insights (only visible by combining perspectives).\n")
	sb.WriteString("4. In **Socratic Verification Summary**, include the verification scores table, ")
	sb.WriteString("key clarifications from the verification process, and any unresolved ambiguities.\n")
	sb.WriteString("5. In **Recommendations**, provide actionable items with priority, impact, effort, ")
	sb.WriteString("and whether the recommendation was verified through Socratic questioning.\n")
	sb.WriteString("6. Mark any findings from perspectives that had 'pass_with_caveats' or 'fail' verdicts ")
	sb.WriteString("with appropriate caveats.\n")
	sb.WriteString("7. If any specialists or interviews failed (degraded mode), explicitly note the reduced ")
	sb.WriteString("coverage in the Executive Summary and relevant sections.\n")
	sb.WriteString("8. If a 'Missing Perspectives' data section is provided above, include a **Missing Perspectives** ")
	sb.WriteString("section in the report listing each skipped specialist, the stage where it failed, the reason, ")
	sb.WriteString("and the impact on analysis coverage. Place it between Perspective Findings and Integrated Analysis.\n")
	sb.WriteString("9. Output ONLY the filled report in Markdown format. No extra commentary before or after.\n")

	// --- Section 9: Language Override ---
	if sctx.Language != "" {
		sb.WriteString("\n---\n\n")
		sb.WriteString("# Language Requirement\n\n")
		sb.WriteString(fmt.Sprintf("IMPORTANT: The ENTIRE report MUST be written in **%s**.\n", sctx.Language))
		sb.WriteString("Translate all section headings, analysis content, findings, recommendations, and summaries ")
		sb.WriteString(fmt.Sprintf("into %s. Only preserve proper nouns, technical terms, code identifiers, and file paths in their original form.\n", sctx.Language))
	}

	return sb.String()
}

// BuildSynthesisUserPrompt constructs the user message for the synthesis subprocess.
func BuildSynthesisUserPrompt(sctx SynthesisContext) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Generate the analysis report for topic: %s\n\n", sctx.Topic))
	sb.WriteString(fmt.Sprintf("This analysis used %d perspectives. ", len(sctx.Perspectives)))

	if sctx.CollectedFindings != nil {
		sb.WriteString(fmt.Sprintf("%d specialist analyses completed with %d total findings. ",
			sctx.CollectedFindings.Succeeded, sctx.CollectedFindings.TotalFindings))
	}
	if sctx.CollectedVerifications != nil {
		sb.WriteString(fmt.Sprintf("%d verification interviews completed with avg score %.2f. ",
			sctx.CollectedVerifications.Succeeded, sctx.CollectedVerifications.AverageScore))
	}
	sb.WriteString("\n\nFill the report template completely using the data provided in the system prompt. ")
	if sctx.Language != "" {
		sb.WriteString(fmt.Sprintf("Write the entire report in %s. ", sctx.Language))
	}
	sb.WriteString("Output only the Markdown report.")

	return sb.String()
}

// RunSynthesisSession executes the synthesis/report generation via a single claude CLI subprocess.
// It loads all collected data (findings, verifications, perspectives), builds a comprehensive
// synthesis prompt, invokes the claude CLI, validates the report has required sections,
// and writes the final report to the report directory.
//
// Unlike specialist and interview stages which run in parallel, synthesis is a single
// sequential subprocess that consumes all prior stage outputs.
func RunSynthesisSession(ctx context.Context, task *taskpkg.AnalysisTask, cfg AnalysisConfig, perspectives []Perspective, reportPath string) error {
	stateDir := task.GetStateDir()

	// Load all synthesis context from disk
	sctx, err := LoadSynthesisContext(cfg, perspectives)
	if err != nil {
		return fmt.Errorf("load synthesis context: %w", err)
	}

	log.Printf("[%s] Synthesis: context loaded — %d perspectives, findings=%v, verifications=%v",
		task.ID, len(perspectives),
		sctx.CollectedFindings != nil,
		sctx.CollectedVerifications != nil)

	// Build prompts
	systemPrompt := BuildSynthesisSystemPrompt(sctx)
	userPrompt := BuildSynthesisUserPrompt(sctx)

	// Update progress detail
	task.UpdateStageDetail(taskpkg.StageSynthesis, "invoking synthesis subprocess")

	// Run single claude CLI subprocess for synthesis
	// Uses --print mode with system prompt, single turn, no tool access needed
	// (all data is provided inline in the system prompt)
	rawReport, err := engine.QueryLLMScopedWithSystemPrompt(
		ctx,
		stateDir,
		cfg.Model,
		systemPrompt,
		userPrompt,
	)
	if err != nil {
		return fmt.Errorf("synthesis subprocess: %w", err)
	}

	if strings.TrimSpace(rawReport) == "" {
		return fmt.Errorf("synthesis subprocess produced empty output")
	}

	log.Printf("[%s] Synthesis: subprocess completed, output length=%d", task.ID, len(rawReport))

	// Update progress
	task.UpdateStageDetail(taskpkg.StageSynthesis, "validating report sections")

	// Validate that the report contains required sections.
	// Derive expected sections from the template when available;
	// fall back to defaultReportSections for the standard analyze template.
	expectedSections := extractTemplateSections(sctx.ReportTemplate)
	if len(expectedSections) == 0 {
		expectedSections = defaultReportSections
	}
	missing := validateReportSections(rawReport, expectedSections)
	if len(missing) > 0 {
		log.Printf("[%s] Synthesis: WARNING — report missing sections: %v", task.ID, missing)
		// Non-fatal: still write the report but log the warning
		// The report is useful even if some sections are missing
	}

	// Ensure report directory exists
	reportDir := filepath.Dir(reportPath)
	if err := os.MkdirAll(reportDir, 0755); err != nil {
		return fmt.Errorf("create report directory: %w", err)
	}

	// Write the final report
	if err := os.WriteFile(reportPath, []byte(rawReport), 0644); err != nil {
		return fmt.Errorf("write report: %w", err)
	}

	log.Printf("[%s] Synthesis: report written to %s (%d bytes)", task.ID, reportPath, len(rawReport))

	return nil
}

// defaultReportSections lists the section headers that must appear in a valid default report.
// Custom report templates (e.g., incident RCA) may use different section names,
// so validation extracts required sections from the template when available.
var defaultReportSections = []string{
	"Executive Summary",
	"Analysis Overview",
	"Perspective Findings",
	"Integrated Analysis",
	"Socratic Verification Summary",
	"Recommendations",
	"Appendix",
}

// validateReportSections checks that the report contains all expected section headers.
// Returns a list of missing section names. Empty list means all sections present.
func validateReportSections(report string, expectedSections []string) []string {
	lower := strings.ToLower(report)
	var missing []string
	for _, section := range expectedSections {
		if !strings.Contains(lower, strings.ToLower(section)) {
			missing = append(missing, section)
		}
	}
	return missing
}

// extractTemplateSections parses a report template and extracts H2 section
// headers (## lines) as the expected sections for validation. Returns nil
// if no H2 headers are found (caller should fall back to defaults).
func extractTemplateSections(template string) []string {
	if template == "" {
		return nil
	}
	var sections []string
	for _, line := range strings.Split(template, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") && !strings.HasPrefix(trimmed, "### ") {
			section := strings.TrimSpace(strings.TrimPrefix(trimmed, "## "))
			if section != "" {
				sections = append(sections, section)
			}
		}
	}
	return sections
}
