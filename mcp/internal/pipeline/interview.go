package pipeline

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// InterviewCommand holds all parameters needed to invoke a claude CLI subprocess
// for a single interview/verification session. The orchestrator uses this to construct
// the exec.Command call that autonomously verifies a specialist's findings.
//
// Unlike the old MCP tool-based interview (handleInterview in interview.go) where
// the main session drove a multi-round Q&A loop via prism_interview tool calls,
// this command produces a single autonomous CLI session that:
//  1. Reviews the specialist's findings against the original topic
//  2. Conducts self-verification by identifying weak points and investigating them
//  3. Produces a verified findings document with confidence assessment
type InterviewCommand struct {
	// PerspectiveID identifies which perspective's findings are being verified.
	PerspectiveID string

	// SystemPrompt is the assembled verification system prompt, combining:
	// - Verifier role identity
	// - Original analysis context (topic, seed summary)
	// - The specialist's findings to verify
	// - Verification criteria (assumption, relevance, constraints)
	// - Output format specification
	SystemPrompt string

	// UserPrompt is the concise task instruction sent as the user message.
	UserPrompt string

	// Model is the fixed model identifier for this analysis run.
	Model string

	// Adaptor is the explicit LLM runtime backend for this interview session.
	Adaptor string

	// WorkDir is the perspective-specific working directory.
	// e.g., ~/.prism/state/analyze-{id}/perspectives/{perspective-id}/
	WorkDir string

	// OutputPath is the expected location of verified-findings.json after completion.
	OutputPath string

	// JSONSchema is the schema string for --json-schema structured output enforcement.
	JSONSchema string
}

// VerifiedFindings represents the structured output from a verification session.
type VerifiedFindings struct {
	Analyst  string            `json:"analyst"`
	Topic    string            `json:"topic"`
	Verdict  string            `json:"verdict"`
	Score    VerificationScore `json:"score"`
	Findings []VerifiedFinding `json:"findings"`
	Summary  string            `json:"summary"`
}

// VerificationScore holds the scoring axes for interview verification.
type VerificationScore struct {
	Assumption    float64 `json:"assumption"`
	Relevance     float64 `json:"relevance"`
	Constraints   float64 `json:"constraints"`
	WeightedTotal float64 `json:"weighted_total"`
}

// VerifiedFinding represents a single finding after verification,
// with its original content plus verification status.
type VerifiedFinding struct {
	Finding      string `json:"finding"`
	Evidence     string `json:"evidence"`
	Severity     string `json:"severity"`
	Status       string `json:"status"`
	Verification string `json:"verification"`
}

// verifiedFindingsJSONSchema enforces structured output from interview subprocesses.
const verifiedFindingsJSONSchema = `{
  "type": "object",
  "required": ["analyst", "topic", "verdict", "score", "findings", "summary"],
  "additionalProperties": false,
  "properties": {
    "analyst": {
      "type": "string",
      "description": "Perspective ID of the analyst whose findings were verified"
    },
    "topic": {
      "type": "string",
      "description": "Original analysis topic"
    },
    "verdict": {
      "type": "string",
      "enum": ["pass", "pass_with_caveats", "fail"],
      "description": "Overall verification verdict"
    },
    "score": {
      "type": "object",
      "required": ["assumption", "relevance", "constraints", "weighted_total"],
      "additionalProperties": false,
      "properties": {
        "assumption": {
          "type": "number",
          "description": "Score 0.0-1.0: Are assumptions verified rather than taken as fact?"
        },
        "relevance": {
          "type": "number",
          "description": "Score 0.0-1.0: Do findings directly address the original topic?"
        },
        "constraints": {
          "type": "number",
          "description": "Score 0.0-1.0: Are technical and scope constraints specified?"
        },
        "weighted_total": {
          "type": "number",
          "description": "Weighted average: assumption*0.4 + relevance*0.4 + constraints*0.2"
        }
      }
    },
    "findings": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["finding", "evidence", "severity", "status", "verification"],
        "additionalProperties": false,
        "properties": {
          "finding": {
            "type": "string",
            "description": "The original finding description"
          },
          "evidence": {
            "type": "string",
            "description": "Evidence source: file:function:line - detail"
          },
          "severity": {
            "type": "string",
            "description": "Severity level from the original finding"
          },
          "status": {
            "type": "string",
            "enum": ["confirmed", "revised", "withdrawn"],
            "description": "Verification status of this finding"
          },
          "verification": {
            "type": "string",
            "description": "Explanation of verification result - what was checked and what was found"
          }
        }
      }
    },
    "summary": {
      "type": "string",
      "description": "Brief summary of verification outcome and key clarifications"
    }
  }
}`

// VerifiedFindingsSchema returns the JSON schema string for verified findings
// structured output, used with claude CLI's --json-schema flag.
func VerifiedFindingsSchema() string {
	return verifiedFindingsJSONSchema
}

// InterviewContext holds the shared context needed to build interview commands
// for all perspectives in a single analysis run. Built once from the analysis
// config and state files, then passed to BuildInterviewCommand for each perspective.
type InterviewContext struct {
	// Topic is the original analysis topic description.
	Topic string

	// ContextID is the analysis session identifier (e.g., "analyze-abc123def456").
	ContextID string

	// Model is the fixed model for all interviews in this run.
	Model string

	// Adaptor is the explicit LLM runtime backend for this analysis run.
	Adaptor string

	// StateDir is the root state directory for this analysis.
	StateDir string

	// WorkDir is the filesystem root Codex should investigate with Grep/Glob/Bash.
	WorkDir string

	// SeedSummary is the research summary from seed-analysis.json,
	// providing context for evaluating finding relevance.
	SeedSummary string

	// OntologyScopeText is the rendered ontology scope text block.
	// Passed to verifier so it can re-investigate evidence sources.
	OntologyScopeText string

	// AvailableMCPServersText is the rendered MCP server list for interviewers.
	// This is always rendered as a dedicated section below Reference Documents,
	// with an empty body when no default MCP servers are configured.
	AvailableMCPServersText string
}

// LoadInterviewContext reads the analysis config and state files to build
// the shared context needed for interview command generation.
// Reuses the same underlying data as LoadSpecialistContext.
func LoadInterviewContext(cfg AnalysisConfig) (InterviewContext, error) {
	ctx := InterviewContext{
		Topic:     cfg.Topic,
		ContextID: cfg.ContextID,
		Model:     cfg.Model,
		Adaptor:   cfg.Adaptor,
		StateDir:  cfg.StateDir,
		WorkDir:   ResolveAnalysisWorkDir(cfg),
	}

	// Read seed analysis summary for context
	seedPath := SeedAnalysisPath(cfg.StateDir)
	seedData, err := os.ReadFile(seedPath)
	if err != nil {
		return ctx, fmt.Errorf("read seed analysis for interview context: %w", err)
	}

	var seed SeedAnalysis
	if err := json.Unmarshal(seedData, &seed); err != nil {
		return ctx, fmt.Errorf("parse seed analysis for interview context: %w", err)
	}
	ctx.SeedSummary = seed.Summary

	// Build ontology scope text blocks — split docs and MCP into separate sections
	ctx.OntologyScopeText, ctx.AvailableMCPServersText = LoadSpecialistOntologyScopeSections(cfg.StateDir)

	return ctx, nil
}

// BuildInterviewCommand constructs an InterviewCommand for a single perspective's
// verification session. Takes the specialist's findings from disk and builds
// an autonomous verification prompt.
//
// The verification subprocess will:
//  1. Review each finding against the original topic for relevance
//  2. Check assumptions — are they verified with evidence or merely assumed?
//  3. Re-investigate weak points using tools (Grep, Read, Glob, Bash)
//  4. Mark each finding as confirmed/revised/withdrawn with explanation
//  5. Score the overall findings quality
//  6. Output structured verified findings JSON
//
// Parameters:
//   - ictx: shared interview context (built once via LoadInterviewContext)
//   - perspective: the perspective whose findings are being verified
//   - findings: the parsed specialist findings to verify
func BuildInterviewCommand(ictx InterviewContext, perspective Perspective, findings SpecialistFindings) InterviewCommand {
	perspDir := PerspectiveDir(ictx.StateDir, perspective.ID)
	outputPath := filepath.Join(perspDir, "verified-findings.json")

	systemPrompt := buildInterviewSystemPrompt(ictx, perspective, findings)

	userPrompt := fmt.Sprintf(
		"Verify the findings from the %s analyst for this topic.\n\nTopic: %s\n\nPerspective: %s\nScope: %s\n\nThe analyst produced %d findings. Review each one, re-investigate weak points, and output your verified assessment as structured JSON.",
		perspective.Name,
		ictx.Topic,
		perspective.Name,
		perspective.Scope,
		len(findings.Findings),
	)

	return InterviewCommand{
		PerspectiveID: perspective.ID,
		SystemPrompt:  systemPrompt,
		UserPrompt:    userPrompt,
		Model:         ictx.Model,
		Adaptor:       ictx.Adaptor,
		WorkDir:       ictx.WorkDir,
		OutputPath:    outputPath,
		JSONSchema:    VerifiedFindingsSchema(),
	}
}

// buildInterviewSystemPrompt assembles the full system prompt for a verification session.
// Adapted from verification-protocol.md for autonomous MCP server orchestration:
// - No prism_interview MCP tool calls (the subprocess IS the interview)
// - No SendMessage/TaskGet/TaskUpdate (MCP server manages lifecycle)
// - All verification logic self-contained in a single subprocess
func buildInterviewSystemPrompt(ictx InterviewContext, perspective Perspective, findings SpecialistFindings) string {
	var sb strings.Builder

	// --- Section 1: Verifier Role Identity ---
	sb.WriteString(fmt.Sprintf(
		"You are the VERIFICATION INTERVIEWER for the %s analyst's findings.\n\n",
		perspective.Name,
	))
	sb.WriteString("Your role is to critically examine each finding through Socratic questioning, ")
	sb.WriteString("verify evidence claims, identify assumptions treated as facts, ")
	sb.WriteString("and assess whether findings actually address the original analysis topic.\n\n")

	// --- Section 2: Original Analysis Context ---
	sb.WriteString("## Original Analysis Context\n\n")
	sb.WriteString(fmt.Sprintf("**Topic:** %s\n\n", ictx.Topic))
	sb.WriteString("**Seed Analysis Summary:**\n")
	sb.WriteString(ictx.SeedSummary)
	sb.WriteString("\n\n")

	// --- Section 3: Analyst Perspective Context ---
	sb.WriteString("## Analyst Perspective\n\n")
	sb.WriteString(fmt.Sprintf("**Name:** %s\n", perspective.Name))
	sb.WriteString(fmt.Sprintf("**Scope:** %s\n", perspective.Scope))
	sb.WriteString(fmt.Sprintf("**Role:** %s\n\n", perspective.Prompt.Role))

	// Key questions the analyst was asked to address
	sb.WriteString("**Key Questions the analyst was tasked with:**\n")
	for i, q := range perspective.KeyQuestions {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, q))
	}
	sb.WriteString("\n")

	// --- Section 4: Findings to Verify ---
	sb.WriteString("## Findings to Verify\n\n")
	sb.WriteString(renderFindingsForVerification(findings))
	sb.WriteString("\n")

	// --- Section 5: Reference Documents (for re-investigation) ---
	sb.WriteString("## Reference Documents\n\n")
	if ictx.OntologyScopeText != "" {
		sb.WriteString(ictx.OntologyScopeText)
	} else {
		sb.WriteString("N/A — ontology scope file not found. Verify using available evidence only.")
	}
	sb.WriteString("\n\n")

	// --- Section 6: Available MCP Servers ---
	sb.WriteString("## Available MCP Servers\n\n")
	sb.WriteString(ictx.AvailableMCPServersText)
	sb.WriteString("\n\n")

	// --- Section 7: Data Source Constraint ---
	sb.WriteString("## Data Source Constraint\n\n")
	sb.WriteString("You MUST only use data sources listed in the \"Reference Documents\" and \"Available MCP Servers\" sections above. ")
	sb.WriteString("Do NOT use `ToolSearch` to discover or call MCP servers not listed in those sections. ")
	sb.WriteString("If a data source is not listed there, it was not selected for this analysis and MUST NOT be used.\n\n")

	// --- Section 8: Verification Protocol ---
	sb.WriteString(buildVerificationProtocol())

	return sb.String()
}

// LoadAvailableMCPServersText reads ontology-scope.json from the state directory
// and renders only MCP server name and description lines for analyst prompts.
func LoadAvailableMCPServersText(stateDir string) string {
	scopePath := filepath.Join(stateDir, "ontology-scope.json")
	data, err := os.ReadFile(scopePath)
	if err != nil {
		return ""
	}

	var scope struct {
		Sources []struct {
			Type    string `json:"type"`
			Server  string `json:"server_name,omitempty"`
			Summary string `json:"summary,omitempty"`
			Status  string `json:"status"`
		} `json:"sources"`
	}

	if err := json.Unmarshal(data, &scope); err != nil {
		return ""
	}

	var sb strings.Builder
	for _, src := range scope.Sources {
		if src.Type != "mcp_query" || src.Status != "available" || src.Server == "" {
			continue
		}
		if src.Summary == "" {
			sb.WriteString(fmt.Sprintf("- %s\n", src.Server))
			continue
		}
		sb.WriteString(fmt.Sprintf("- %s: %s\n", src.Server, src.Summary))
	}

	return sb.String()
}

// buildVerificationProtocol generates the verification instructions section.
// Adapted from verification-protocol.md for autonomous subprocess execution.
func buildVerificationProtocol() string {
	var sb strings.Builder

	sb.WriteString("---\n\n")
	sb.WriteString("# Verification Protocol\n\n")

	sb.WriteString("## Step 1: Review Each Finding\n\n")
	sb.WriteString("For each finding, evaluate:\n\n")
	sb.WriteString("1. **Assumption Check (weight: 40%):** Is this finding based on verified evidence, ")
	sb.WriteString("or does it assume something without verification? Are there unvalidated hypotheses ")
	sb.WriteString("treated as confirmed facts?\n\n")
	sb.WriteString("2. **Relevance Check (weight: 40%):** Does this finding directly address the original ")
	sb.WriteString("analysis topic? Is it actually related to what was asked, or did the analyst find ")
	sb.WriteString("real but unrelated issues? If the topic mentions a concept that doesn't exist in ")
	sb.WriteString("the codebase, did the analyst acknowledge this gap?\n\n")
	sb.WriteString("3. **Constraints Check (weight: 20%):** Are technical, resource, and scope ")
	sb.WriteString("constraints properly specified?\n\n")

	sb.WriteString("## Step 2: Re-investigate Weak Points\n\n")
	sb.WriteString("For any finding where evidence seems weak, assumptions are unverified, or relevance ")
	sb.WriteString("is questionable:\n\n")
	sb.WriteString("- Use tools (Grep, Read, Glob, Bash) to independently verify the evidence claims\n")
	sb.WriteString("- Check if the cited file paths, function names, and line numbers actually exist\n")
	sb.WriteString("- Verify that the evidence supports the finding's conclusion\n")
	sb.WriteString("- Look for counter-evidence that might contradict the finding\n\n")

	sb.WriteString("## Step 3: Classify Each Finding\n\n")
	sb.WriteString("Mark each finding with a status:\n\n")
	sb.WriteString("- **confirmed:** Evidence verified, finding is accurate and relevant\n")
	sb.WriteString("- **revised:** Finding has merit but needs qualification or correction ")
	sb.WriteString("(describe what changed in the verification field)\n")
	sb.WriteString("- **withdrawn:** Evidence does not support the finding, or finding is irrelevant ")
	sb.WriteString("to the original topic\n\n")

	sb.WriteString("## Step 4: Score and Summarize\n\n")
	sb.WriteString("Calculate scores on each axis (0.0 = completely ambiguous, 1.0 = perfectly clear):\n\n")
	sb.WriteString("- **assumption:** Score across all findings\n")
	sb.WriteString("- **relevance:** Score across all findings\n")
	sb.WriteString("- **constraints:** Score across all findings\n")
	sb.WriteString("- **weighted_total:** assumption*0.4 + relevance*0.4 + constraints*0.2\n\n")
	sb.WriteString("Set verdict:\n")
	sb.WriteString("- **pass:** weighted_total > 0.8\n")
	sb.WriteString("- **pass_with_caveats:** weighted_total > 0.6 but <= 0.8\n")
	sb.WriteString("- **fail:** weighted_total <= 0.6\n\n")

	sb.WriteString("## Output\n\n")
	sb.WriteString("Output your verified findings as a JSON object. Every finding from the original ")
	sb.WriteString("must appear in your output with its verification status. Do NOT add new findings ")
	sb.WriteString("— only verify existing ones.\n")

	return sb.String()
}

// renderFindingsForVerification formats specialist findings into a readable block
// for inclusion in the verification system prompt.
func renderFindingsForVerification(findings SpecialistFindings) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Analyst: %s\n", findings.Analyst))
	sb.WriteString(fmt.Sprintf("Total findings: %d\n\n", len(findings.Findings)))

	for i, f := range findings.Findings {
		sb.WriteString(fmt.Sprintf("### Finding %d\n", i+1))
		sb.WriteString(fmt.Sprintf("- **Finding:** %s\n", f.Finding))
		sb.WriteString(fmt.Sprintf("- **Evidence:** %s\n", f.Evidence))
		sb.WriteString(fmt.Sprintf("- **Severity:** %s\n\n", f.Severity))
	}

	return sb.String()
}

// BuildAllInterviewCommands generates InterviewCommand structs for all perspectives
// that have findings. Reads findings from disk for each perspective.
// Skips perspectives without findings (returns nil entry in the result).
// Returns an error if the shared context cannot be loaded.
func BuildAllInterviewCommands(cfg AnalysisConfig, perspectives []Perspective, specialistResults []StageResult) ([]InterviewCommand, error) {
	ictx, err := LoadInterviewContext(cfg)
	if err != nil {
		return nil, fmt.Errorf("load interview context: %w", err)
	}

	commands := make([]InterviewCommand, 0, len(perspectives))
	for i, p := range perspectives {
		// Skip perspectives with failed specialist results
		if i >= len(specialistResults) || specialistResults[i].Err != nil {
			continue
		}

		// Read findings from disk
		findingsPath := FindingsPath(ictx.StateDir, p.ID)
		findings, err := ReadSpecialistFindings(findingsPath)
		if err != nil {
			// Log warning but skip — this perspective had no valid findings
			continue
		}

		if len(findings.Findings) == 0 {
			// No findings to verify — skip
			continue
		}

		cmd := BuildInterviewCommand(ictx, p, findings)
		commands = append(commands, cmd)
	}

	return commands, nil
}

// ReadVerifiedFindings reads and parses verified-findings.json from disk.
func ReadVerifiedFindings(path string) (VerifiedFindings, error) {
	var v VerifiedFindings
	data, err := os.ReadFile(path)
	if err != nil {
		return v, fmt.Errorf("read verified findings: %w", err)
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return v, fmt.Errorf("parse verified findings: %w", err)
	}
	return v, nil
}

// WriteVerifiedFindings writes a VerifiedFindings to disk as formatted JSON.
func WriteVerifiedFindings(path string, v VerifiedFindings) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal verified findings: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write verified findings: %w", err)
	}
	return nil
}

// VerifiedFindingsPath returns the path to verified-findings.json for a perspective.
func VerifiedFindingsPath(stateDir, perspectiveID string) string {
	return filepath.Join(stateDir, "perspectives", perspectiveID, "verified-findings.json")
}
