package pipeline

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type legacyContextArtifact struct {
	Summary            string                `json:"summary"`
	ResearchSummary    legacyResearchSummary `json:"research_summary"`
	ReportLanguage     string                `json:"report_language"`
	InvestigationLoops int                   `json:"investigation_loops"`
}

type legacyResearchSummary struct {
	KeyFindings []string `json:"key_findings"`
	KeyAreas    []string `json:"key_areas"`
}

type legacyInterviewArtifact struct {
	PerspectiveID string            `json:"perspective_id"`
	Analyst       string            `json:"analyst"`
	Verdict       string            `json:"verdict"`
	WeightedTotal float64           `json:"weighted_total"`
	Score         VerificationScore `json:"score"`
	Findings      []VerifiedFinding `json:"findings"`
	Summary       string            `json:"summary"`
	Rounds        int               `json:"rounds"`
}

type legacyVerificationLog struct {
	TaskID       string                       `json:"task_id,omitempty"`
	GeneratedAt  time.Time                    `json:"generated_at"`
	Perspectives []legacyVerificationLogEntry `json:"perspectives"`
}

type legacyVerificationLogEntry struct {
	PerspectiveID string            `json:"perspective_id"`
	Analyst       string            `json:"analyst"`
	Verdict       string            `json:"verdict"`
	WeightedTotal float64           `json:"weighted_total"`
	Score         VerificationScore `json:"score"`
	FindingsCount int               `json:"findings_count"`
	Status        string            `json:"status"`
}

func writeLegacyContextArtifact(stateDir string, cfg AnalysisConfig) error {
	seed, err := ReadSeedAnalysis(SeedAnalysisPath(stateDir))
	if err != nil {
		return fmt.Errorf("read seed analysis for context artifact: %w", err)
	}

	investigationLoops := 0
	if historyData, err := os.ReadFile(DAHistoryPath(stateDir)); err == nil {
		var history DAReviewHistory
		if json.Unmarshal(historyData, &history) == nil && history.TotalRounds > 0 {
			investigationLoops = history.TotalRounds
		}
	}

	keyFindings := make([]string, 0, len(seed.Findings))
	for _, finding := range seed.Findings {
		keyFindings = append(keyFindings, fmt.Sprintf("%s: %s", finding.Area, finding.Description))
	}

	ctx := legacyContextArtifact{
		Summary: seed.Summary,
		ResearchSummary: legacyResearchSummary{
			KeyFindings: keyFindings,
			KeyAreas:    append([]string(nil), seed.KeyAreas...),
		},
		ReportLanguage:     normalizedReportLanguage(cfg.Language),
		InvestigationLoops: investigationLoops,
	}

	return writeJSONFile(filepath.Join(stateDir, "context.json"), ctx)
}

func writeLegacyVerificationArtifacts(stateDir string, perspectives []Perspective, collected CollectedVerifications) error {
	for _, result := range collected.Results {
		if result.Outcome != InterviewSuccess || result.Verified == nil {
			continue
		}

		if err := writeLegacyInterviewArtifact(stateDir, result); err != nil {
			return err
		}
		if err := writeLegacyVerifiedFindingsMarkdown(stateDir, perspectives, result); err != nil {
			return err
		}
	}

	if err := writeLegacyVerificationLog(stateDir, collected); err != nil {
		return err
	}

	if err := writeLegacyAnalystFindingsMarkdown(stateDir, perspectives, collected); err != nil {
		return err
	}

	return nil
}

func writeLegacyInterviewArtifact(stateDir string, result InterviewResult) error {
	if result.Verified == nil {
		return nil
	}

	artifact := legacyInterviewArtifact{
		PerspectiveID: result.PerspectiveID,
		Analyst:       firstNonEmpty(result.Verified.Analyst, result.PerspectiveID),
		Verdict:       result.Verified.Verdict,
		WeightedTotal: result.Verified.Score.WeightedTotal,
		Score:         result.Verified.Score,
		Findings:      append([]VerifiedFinding(nil), result.Verified.Findings...),
		Summary:       result.Verified.Summary,
		Rounds:        1,
	}

	return writeJSONFile(filepath.Join(stateDir, "perspectives", result.PerspectiveID, "interview.json"), artifact)
}

func writeLegacyVerifiedFindingsMarkdown(stateDir string, perspectives []Perspective, result InterviewResult) error {
	if result.Verified == nil {
		return nil
	}

	perspectiveName := result.PerspectiveID
	if perspective, ok := findPerspectiveByID(perspectives, result.PerspectiveID); ok && strings.TrimSpace(perspective.Name) != "" {
		perspectiveName = perspective.Name
	}

	var sb strings.Builder
	sb.WriteString("# Verified Findings\n\n")
	sb.WriteString(fmt.Sprintf("- Perspective: %s\n", perspectiveName))
	sb.WriteString(fmt.Sprintf("- Perspective ID: %s\n", result.PerspectiveID))
	sb.WriteString(fmt.Sprintf("- Verdict: %s\n", strings.ToUpper(result.Verified.Verdict)))
	sb.WriteString(fmt.Sprintf("- Weighted Total: %.2f\n\n", result.Verified.Score.WeightedTotal))

	sb.WriteString("## Verification Summary\n\n")
	sb.WriteString(result.Verified.Summary)
	sb.WriteString("\n\n")

	sb.WriteString("## Findings\n\n")
	for idx, finding := range result.Verified.Findings {
		sb.WriteString(fmt.Sprintf("### Finding %d\n", idx+1))
		sb.WriteString(fmt.Sprintf("- Status: %s\n", finding.Status))
		sb.WriteString(fmt.Sprintf("- Severity: %s\n", finding.Severity))
		sb.WriteString(fmt.Sprintf("- Finding: %s\n", finding.Finding))
		sb.WriteString(fmt.Sprintf("- Evidence: %s\n", finding.Evidence))
		sb.WriteString(fmt.Sprintf("- Verification: %s\n\n", finding.Verification))
	}

	return os.WriteFile(filepath.Join(stateDir, fmt.Sprintf("verified-findings-%s.md", result.PerspectiveID)), []byte(sb.String()), 0644)
}

func writeLegacyVerificationLog(stateDir string, collected CollectedVerifications) error {
	logArtifact := legacyVerificationLog{
		TaskID:       collected.TaskID,
		GeneratedAt:  time.Now().UTC(),
		Perspectives: make([]legacyVerificationLogEntry, 0, len(collected.Results)),
	}

	for _, result := range collected.Results {
		if result.Verified == nil {
			continue
		}
		status := "completed"
		if result.Outcome != InterviewSuccess {
			status = string(result.Outcome)
		}
		logArtifact.Perspectives = append(logArtifact.Perspectives, legacyVerificationLogEntry{
			PerspectiveID: result.PerspectiveID,
			Analyst:       firstNonEmpty(result.Verified.Analyst, result.PerspectiveID),
			Verdict:       result.Verified.Verdict,
			WeightedTotal: result.Verified.Score.WeightedTotal,
			Score:         result.Verified.Score,
			FindingsCount: len(result.Verified.Findings),
			Status:        status,
		})
	}

	sort.Slice(logArtifact.Perspectives, func(i, j int) bool {
		return logArtifact.Perspectives[i].PerspectiveID < logArtifact.Perspectives[j].PerspectiveID
	})

	return writeJSONFile(filepath.Join(stateDir, "verification-log.json"), logArtifact)
}

func writeLegacyAnalystFindingsMarkdown(stateDir string, perspectives []Perspective, collected CollectedVerifications) error {
	var sb strings.Builder
	sb.WriteString("# Analyst Findings\n\n")
	sb.WriteString("Compiled and verified analysis results adapted for legacy Prism consumers.\n\n")

	sb.WriteString("## Verification Scores Summary\n\n")
	sb.WriteString("| Perspective | Weighted Total | Verdict | Findings |\n")
	sb.WriteString("|-------------|----------------|---------|----------|\n")

	for _, result := range collected.Results {
		if result.Verified == nil {
			continue
		}
		perspectiveLabel := result.PerspectiveID
		if perspective, ok := findPerspectiveByID(perspectives, result.PerspectiveID); ok && strings.TrimSpace(perspective.Name) != "" {
			perspectiveLabel = perspective.Name
		}
		sb.WriteString(fmt.Sprintf("| %s | %.2f | %s | %d |\n",
			perspectiveLabel,
			result.Verified.Score.WeightedTotal,
			strings.ToUpper(result.Verified.Verdict),
			len(result.Verified.Findings),
		))
	}

	for _, result := range collected.Results {
		if result.Verified == nil {
			continue
		}
		perspectiveLabel := result.PerspectiveID
		if perspective, ok := findPerspectiveByID(perspectives, result.PerspectiveID); ok && strings.TrimSpace(perspective.Name) != "" {
			perspectiveLabel = perspective.Name
		}
		sb.WriteString(fmt.Sprintf("\n## %s\n\n", perspectiveLabel))
		sb.WriteString(fmt.Sprintf("Perspective ID: `%s`\n\n", result.PerspectiveID))
		sb.WriteString(fmt.Sprintf("Verdict: **%s**\n\n", strings.ToUpper(result.Verified.Verdict)))
		sb.WriteString(result.Verified.Summary)
		sb.WriteString("\n\n")
		for idx, finding := range result.Verified.Findings {
			sb.WriteString(fmt.Sprintf("### Finding %d\n", idx+1))
			sb.WriteString(fmt.Sprintf("- Status: %s\n", finding.Status))
			sb.WriteString(fmt.Sprintf("- Severity: %s\n", finding.Severity))
			sb.WriteString(fmt.Sprintf("- Finding: %s\n", finding.Finding))
			sb.WriteString(fmt.Sprintf("- Evidence: %s\n", finding.Evidence))
			sb.WriteString(fmt.Sprintf("- Verification: %s\n\n", finding.Verification))
		}
	}

	return os.WriteFile(filepath.Join(stateDir, "analyst-findings.md"), []byte(sb.String()), 0644)
}

func writeLegacyStateReportCopy(stateDir, reportPath string) error {
	data, err := os.ReadFile(reportPath)
	if err != nil {
		return fmt.Errorf("read report for state copy: %w", err)
	}
	if err := os.WriteFile(filepath.Join(stateDir, "report.md"), data, 0644); err != nil {
		return fmt.Errorf("write state report copy: %w", err)
	}
	return nil
}

func findPerspectiveByID(perspectives []Perspective, perspectiveID string) (Perspective, bool) {
	for _, perspective := range perspectives {
		if perspective.ID == perspectiveID {
			return perspective, true
		}
	}
	return Perspective{}, false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func normalizedReportLanguage(language string) string {
	if strings.TrimSpace(language) == "" {
		return "en"
	}
	return language
}

func writeJSONFile(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", filepath.Base(path), err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write %s: %w", filepath.Base(path), err)
	}
	return nil
}
