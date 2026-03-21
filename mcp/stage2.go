package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SpecialistCommand holds all parameters needed to invoke a claude CLI subprocess
// for a single specialist finding session. The orchestrator uses this to construct
// the exec.Command call with proper isolation and structured output enforcement.
type SpecialistCommand struct {
	// PerspectiveID identifies which perspective this specialist analyzes.
	PerspectiveID string

	// SystemPrompt is the assembled specialist system prompt, combining:
	// - Role identity and investigation scope (from perspectives.json)
	// - Context (from seed analysis summary)
	// - Reference documents (from ontology scope)
	// - Tasks (from perspectives.json)
	// - Output format (from perspectives.json)
	// - Finding protocol (adapted for MCP server orchestration)
	SystemPrompt string

	// UserPrompt is the concise task instruction sent as the user message.
	UserPrompt string

	// Model is the fixed model identifier for this analysis run.
	Model string

	// WorkDir is the perspective-specific working directory under the task's state dir.
	// e.g., ~/.prism/state/analyze-{id}/perspectives/{perspective-id}/
	WorkDir string

	// OutputPath is the expected location of findings.json after the specialist completes.
	OutputPath string

	// MaxTurns is the maximum number of agentic turns for tool use.
	MaxTurns int

	// JSONSchema is the schema string for --json-schema structured output enforcement.
	JSONSchema string
}

// SpecialistContext holds the shared context needed to build specialist commands
// for all perspectives in a single analysis run. Built once from the analysis
// config and state files, then passed to BuildSpecialistCommand for each perspective.
type SpecialistContext struct {
	// Topic is the original analysis topic description.
	Topic string

	// ContextID is the analysis session identifier (e.g., "analyze-abc123def456").
	ContextID string

	// Model is the fixed model for all specialists in this run.
	Model string

	// StateDir is the root state directory for this analysis.
	StateDir string

	// SeedSummary is the research summary from seed-analysis.json,
	// injected as CONTEXT in the analyst prompt.
	SeedSummary string

	// OntologyScopeText is the rendered ontology scope text block
	// for injection into the Reference Documents section.
	// Empty string if no ontology scope is configured.
	OntologyScopeText string

	// DocPaths are the registered ontology document directories.
	DocPaths []string
}

// specialistFindingsJSONSchema enforces structured output from specialist subprocesses.
// Matches the findings.json format defined in finding-protocol.md.
const specialistFindingsJSONSchema = `{
  "type": "object",
  "required": ["analyst", "input", "findings"],
  "additionalProperties": false,
  "properties": {
    "analyst": {
      "type": "string",
      "description": "Perspective ID of the analyst producing these findings"
    },
    "input": {
      "type": "string",
      "description": "Original topic description from the analysis request"
    },
    "findings": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["finding", "evidence", "severity"],
        "additionalProperties": false,
        "properties": {
          "finding": {
            "type": "string",
            "description": "Description of the finding"
          },
          "evidence": {
            "type": "string",
            "description": "Evidence source: file:function:line — detail"
          },
          "severity": {
            "type": "string",
            "description": "Severity using the scale defined in output format"
          }
        }
      }
    }
  }
}`

// SpecialistFindingsSchema returns the JSON schema string for specialist findings
// structured output, used with claude CLI's --json-schema flag.
func SpecialistFindingsSchema() string {
	return specialistFindingsJSONSchema
}

// SpecialistFindings represents the parsed findings.json output from a specialist.
type SpecialistFindings struct {
	Analyst  string              `json:"analyst"`
	Input    string              `json:"input"`
	Findings []SpecialistFinding `json:"findings"`
}

// SpecialistFinding represents a single finding from a specialist analysis.
type SpecialistFinding struct {
	Finding  string `json:"finding"`
	Evidence string `json:"evidence"`
	Severity string `json:"severity"`
}

// ReadSpecialistFindings reads and parses findings.json from disk.
func ReadSpecialistFindings(path string) (SpecialistFindings, error) {
	var f SpecialistFindings
	data, err := os.ReadFile(path)
	if err != nil {
		return f, fmt.Errorf("read findings: %w", err)
	}
	if err := json.Unmarshal(data, &f); err != nil {
		return f, fmt.Errorf("parse findings: %w", err)
	}
	return f, nil
}

// WriteSpecialistFindings writes a SpecialistFindings to disk as formatted JSON.
func WriteSpecialistFindings(path string, f SpecialistFindings) error {
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal findings: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write findings: %w", err)
	}
	return nil
}

// FindingsPath returns the path to findings.json for a given perspective
// within the task's state directory.
func FindingsPath(stateDir, perspectiveID string) string {
	return filepath.Join(stateDir, "perspectives", perspectiveID, "findings.json")
}

// PerspectiveDir returns the perspective-specific directory path.
func PerspectiveDir(stateDir, perspectiveID string) string {
	return filepath.Join(stateDir, "perspectives", perspectiveID)
}

// LoadSpecialistContext reads the analysis config and state files to build
// the shared context needed for specialist command generation.
// Called once before building commands for all perspectives.
func LoadSpecialistContext(cfg AnalysisConfig) (SpecialistContext, error) {
	ctx := SpecialistContext{
		Topic:     cfg.Topic,
		ContextID: cfg.ContextID,
		Model:     cfg.Model,
		StateDir:  cfg.StateDir,
	}

	// Read seed analysis summary for CONTEXT injection
	seedPath := SeedAnalysisPath(cfg.StateDir)
	seedData, err := os.ReadFile(seedPath)
	if err != nil {
		return ctx, fmt.Errorf("read seed analysis for specialist context: %w", err)
	}

	var seed SeedAnalysis
	if err := json.Unmarshal(seedData, &seed); err != nil {
		return ctx, fmt.Errorf("parse seed analysis for specialist context: %w", err)
	}
	ctx.SeedSummary = seed.Research.Summary

	// Build ontology scope text block from ontology-scope.json if it exists
	ctx.OntologyScopeText = loadOntologyScopeText(cfg.StateDir)

	// Load registered doc paths
	ctx.DocPaths = LoadOntologyDocPaths()

	return ctx, nil
}

// loadOntologyScopeText reads ontology-scope.json from the state directory
// and renders it into the text block format defined in ontology-scope-schema.md.
// Returns empty string with fallback message if file doesn't exist.
func loadOntologyScopeText(stateDir string) string {
	scopePath := filepath.Join(stateDir, "ontology-scope.json")
	data, err := os.ReadFile(scopePath)
	if err != nil {
		return "N/A — ontology scope file not found. Analyze using available evidence only."
	}

	// Parse the ontology-scope.json structure
	var scope struct {
		Sources []struct {
			ID       int    `json:"id"`
			Type     string `json:"type"`
			Path     string `json:"path,omitempty"`
			URL      string `json:"url,omitempty"`
			Server   string `json:"server_name,omitempty"`
			Domain   string `json:"domain,omitempty"`
			Summary  string `json:"summary,omitempty"`
			Status   string `json:"status"`
			Reason   string `json:"reason,omitempty"`
			Access   struct {
				Tools         []string `json:"tools,omitempty"`
				Instructions  string   `json:"instructions,omitempty"`
				Capabilities  string   `json:"capabilities,omitempty"`
				GettingStarted string  `json:"getting_started,omitempty"`
				ErrorHandling string   `json:"error_handling,omitempty"`
				CachedSummary string   `json:"cached_summary,omitempty"`
			} `json:"access,omitempty"`
		} `json:"sources"`
		CitationFormat map[string]string `json:"citation_format,omitempty"`
	}

	if err := json.Unmarshal(data, &scope); err != nil {
		return "N/A — failed to parse ontology scope file. Analyze using available evidence only."
	}

	var sb strings.Builder
	sb.WriteString("Your reference documents and data sources:\n\n")

	for _, src := range scope.Sources {
		if src.Status != "available" {
			continue
		}

		switch src.Type {
		case "doc":
			sb.WriteString(fmt.Sprintf("- doc: %s (%s)\n", src.Summary, src.Status))
			if src.Path != "" {
				sb.WriteString(fmt.Sprintf("  Directories: %s\n", src.Path))
			}
			if src.Access.Instructions != "" {
				sb.WriteString(fmt.Sprintf("  Access: %s\n", src.Access.Instructions))
			}
			for _, tool := range src.Access.Tools {
				sb.WriteString(fmt.Sprintf("    %s\n", tool))
			}

		case "mcp_query":
			sb.WriteString(fmt.Sprintf("- mcp-query: %s: %s\n", src.Server, src.Summary))
			if len(src.Access.Tools) > 0 {
				sb.WriteString(fmt.Sprintf("  Tools (read-only): %s\n", strings.Join(src.Access.Tools, ", ")))
			}
			if src.Access.Instructions != "" {
				sb.WriteString(fmt.Sprintf("  Access: %s\n", src.Access.Instructions))
			}
			if src.Access.Capabilities != "" {
				sb.WriteString(fmt.Sprintf("  Capabilities: %s\n", src.Access.Capabilities))
			}
			if src.Access.GettingStarted != "" {
				sb.WriteString(fmt.Sprintf("  Getting started: %s\n", src.Access.GettingStarted))
			}
			if src.Access.ErrorHandling != "" {
				sb.WriteString(fmt.Sprintf("  Error handling: %s\n", src.Access.ErrorHandling))
			}

		case "web":
			sb.WriteString(fmt.Sprintf("- web: %s: %s — %s\n", src.URL, src.Domain, src.Summary))
			if src.Access.Instructions != "" {
				sb.WriteString(fmt.Sprintf("  Access: %s\n", src.Access.Instructions))
			}
			if src.Access.CachedSummary != "" {
				sb.WriteString(fmt.Sprintf("  %s\n", src.Access.CachedSummary))
			}

		case "file":
			sb.WriteString(fmt.Sprintf("- file: %s: %s — %s\n", src.Path, src.Domain, src.Summary))
			if src.Access.Instructions != "" {
				sb.WriteString(fmt.Sprintf("  Access: %s\n", src.Access.Instructions))
			}
			if src.Access.CachedSummary != "" {
				sb.WriteString(fmt.Sprintf("  %s\n", src.Access.CachedSummary))
			}
		}

		sb.WriteString("\n")
	}

	// Add citation format
	if len(scope.CitationFormat) > 0 {
		sb.WriteString("Explore these sources through your perspective's lens.\n")
		sb.WriteString("Cite findings as: ")
		formats := make([]string, 0, len(scope.CitationFormat))
		for _, v := range scope.CitationFormat {
			formats = append(formats, v)
		}
		sb.WriteString(strings.Join(formats, ", "))
		sb.WriteString(".\n")
	}

	return sb.String()
}

// BuildSpecialistCommand constructs a SpecialistCommand for a single perspective.
// The command includes the fully assembled system prompt following the
// analyst-prompt-structure.md template, with all placeholders filled.
//
// The system prompt structure:
//  1. Role identity (from perspective.prompt.role)
//  2. CONTEXT (seed analysis summary)
//  3. Reference Documents (ontology scope text)
//  4. Investigation scope (from perspective.prompt.investigation_scope)
//  5. TASKS (from perspective.prompt.tasks)
//  6. OUTPUT (from perspective.prompt.output_format)
//  7. Finding protocol (adapted for MCP server orchestration — no team coordination)
//
// Parameters:
//   - sctx: shared specialist context (built once via LoadSpecialistContext)
//   - perspective: the specific perspective to build a command for
func BuildSpecialistCommand(sctx SpecialistContext, perspective Perspective) SpecialistCommand {
	perspDir := PerspectiveDir(sctx.StateDir, perspective.ID)
	findingsPath := FindingsPath(sctx.StateDir, perspective.ID)

	systemPrompt := buildSpecialistSystemPrompt(sctx, perspective)

	userPrompt := fmt.Sprintf(
		"Investigate this topic from your assigned perspective and output your findings as structured JSON.\n\nTopic: %s\n\nYour perspective: %s\nScope: %s",
		sctx.Topic,
		perspective.Name,
		perspective.Scope,
	)

	return SpecialistCommand{
		PerspectiveID: perspective.ID,
		SystemPrompt:  systemPrompt,
		UserPrompt:    userPrompt,
		Model:         sctx.Model,
		WorkDir:       perspDir,
		OutputPath:    findingsPath,
		MaxTurns:      10,
		JSONSchema:    SpecialistFindingsSchema(),
	}
}

// buildSpecialistSystemPrompt assembles the full system prompt for a specialist,
// following the analyst-prompt-structure.md template with finding protocol appended.
func buildSpecialistSystemPrompt(sctx SpecialistContext, perspective Perspective) string {
	var sb strings.Builder

	// --- Section 1: Role Identity ---
	sb.WriteString(perspective.Prompt.Role)
	sb.WriteString("\n\n")

	// --- Section 2: CONTEXT ---
	sb.WriteString("CONTEXT:\n")
	sb.WriteString(sctx.SeedSummary)
	sb.WriteString("\n\n")

	// --- Section 3: Reference Documents ---
	sb.WriteString("### Reference Documents\n")
	if sctx.OntologyScopeText != "" {
		sb.WriteString(sctx.OntologyScopeText)
	} else {
		sb.WriteString("N/A — ontology scope file not found. Analyze using available evidence only.")
	}
	sb.WriteString("\n\n")

	// Add doc paths for targeted search
	if len(sctx.DocPaths) > 0 {
		sb.WriteString("## Analysis Target Directories\n\n")
		sb.WriteString("Focus your investigation on these registered document/code directories:\n\n")
		for _, p := range sctx.DocPaths {
			sb.WriteString("- ")
			sb.WriteString(p)
			sb.WriteString("\n")
		}
		sb.WriteString("\nSearch within these directories first. You may also search related areas outside these directories if the evidence trail leads there.\n\n")
	}

	// --- Section 4: Investigation Scope ---
	sb.WriteString(perspective.Prompt.InvestigationScope)
	sb.WriteString("\n\n")

	// --- Section 5: TASKS ---
	sb.WriteString("TASKS:\n")
	sb.WriteString(perspective.Prompt.Tasks)
	sb.WriteString("\n\n")

	// --- Section 6: OUTPUT ---
	sb.WriteString("OUTPUT:\n")
	sb.WriteString(perspective.Prompt.OutputFormat)
	sb.WriteString("\n\n")

	// --- Section 7: Finding Protocol (adapted for MCP server orchestration) ---
	sb.WriteString(buildFindingProtocol(sctx, perspective))

	return sb.String()
}

// buildFindingProtocol generates the finding protocol section for a specialist.
// This is adapted from finding-protocol.md for MCP server orchestration:
// - No team coordination (TaskGet, TaskUpdate, SendMessage) — the MCP server manages lifecycle
// - Key questions injected from perspective
// - Data source constraint enforced
// - Output path specified with concrete values (no placeholders)
func buildFindingProtocol(sctx SpecialistContext, perspective Perspective) string {
	var sb strings.Builder

	sb.WriteString("---\n\n")
	sb.WriteString("# Finding Protocol\n\n")

	// Key questions from perspective
	sb.WriteString("## Perspective-Specific Questions\n\n")
	for i, q := range perspective.KeyQuestions {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, q))
	}
	sb.WriteString("\nAnswer these questions in addition to your investigation tasks. They are grounded in the seed analyst's research findings and target this specific case.\n\n")

	// Data source constraint
	sb.WriteString("## Data Source Constraint\n\n")
	sb.WriteString("You MUST only use data sources listed in the \"Reference Documents\" section above. ")
	sb.WriteString("Do NOT use `ToolSearch` to discover or call MCP servers not in your Reference Documents. ")
	sb.WriteString("If a data source is not listed there, it was not selected for this analysis and MUST NOT be used.\n\n")

	// Investigation steps
	sb.WriteString("## Investigation & Findings\n\n")
	sb.WriteString("### 1. Investigate\n\n")
	sb.WriteString("Answer ALL key questions and complete ALL tasks from your prompt with evidence and code references. ")
	sb.WriteString("Use available tools (Grep, Read, Bash, Glob) to gather evidence.\n\n")

	// Output instructions
	sb.WriteString("### 2. Output Findings\n\n")
	sb.WriteString("Output your findings as a JSON object with this structure:\n\n")
	sb.WriteString(fmt.Sprintf("- analyst: \"%s\"\n", perspective.ID))
	sb.WriteString(fmt.Sprintf("- input: Copy the original topic description exactly: \"%s\"\n", truncateForPrompt(sctx.Topic, 200)))
	sb.WriteString("- findings: Array of finding objects, each with:\n")
	sb.WriteString("  - finding: Description of the finding\n")
	sb.WriteString("  - evidence: Evidence source (file:function:line — detail)\n")
	sb.WriteString("  - severity: Use the severity scale defined in your output format above\n\n")

	sb.WriteString("Every finding MUST have concrete evidence — no unsourced claims.\n")
	sb.WriteString("Do NOT run self-verification (prism_interview) — that happens in a separate session.\n")

	return sb.String()
}

// BuildAllSpecialistCommands generates SpecialistCommand structs for all perspectives.
// Creates perspective directories and returns one command per perspective.
// Returns an error if the shared context cannot be loaded.
func BuildAllSpecialistCommands(cfg AnalysisConfig, perspectives []Perspective) ([]SpecialistCommand, error) {
	sctx, err := LoadSpecialistContext(cfg)
	if err != nil {
		return nil, fmt.Errorf("load specialist context: %w", err)
	}

	commands := make([]SpecialistCommand, 0, len(perspectives))
	for _, p := range perspectives {
		// Ensure perspective directory exists
		perspDir := PerspectiveDir(sctx.StateDir, p.ID)
		if err := os.MkdirAll(perspDir, 0755); err != nil {
			return nil, fmt.Errorf("create perspective directory for %s: %w", p.ID, err)
		}

		cmd := BuildSpecialistCommand(sctx, p)
		commands = append(commands, cmd)
	}

	return commands, nil
}

// truncateForPrompt truncates a string to maxLen characters for prompt inclusion,
// appending "..." if truncated. Used for topic descriptions in protocol templates.
func truncateForPrompt(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
