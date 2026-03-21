package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// --- Seed Analysis JSON schema for --json-schema flag ---

// seedAnalysisJSONSchema enforces structured output from the seed analyst subprocess.
// Mirrors the SeedAnalysis struct in seed_merge.go.
const seedAnalysisJSONSchema = `{
  "type": "object",
  "required": ["topic", "da_passed", "research"],
  "additionalProperties": false,
  "properties": {
    "topic": {
      "type": "string",
      "description": "Copy of the original topic description"
    },
    "da_passed": {
      "type": "boolean",
      "description": "Always true when output by seed analyst (DA review handled externally)"
    },
    "research": {
      "type": "object",
      "required": ["summary", "findings", "key_areas", "files_examined"],
      "additionalProperties": false,
      "properties": {
        "summary": {
          "type": "string",
          "description": "High-level summary of investigated areas for the perspective generator"
        },
        "findings": {
          "type": "array",
          "items": {
            "type": "object",
            "required": ["id", "area", "description", "source", "tool_used"],
            "additionalProperties": false,
            "properties": {
              "id": {
                "type": "integer",
                "description": "Sequential finding ID starting from 1"
              },
              "area": {
                "type": "string",
                "description": "Name of the code area, module, or system"
              },
              "description": {
                "type": "string",
                "description": "What this area does and how it relates to the topic"
              },
              "source": {
                "type": "string",
                "description": "Evidence source: file:function:line or tool:query"
              },
              "tool_used": {
                "type": "string",
                "description": "Tool that found this: Grep, Read, Bash, or Glob"
              }
            }
          }
        },
        "key_areas": {
          "type": "array",
          "items": { "type": "string" },
          "description": "Main domains/areas discovered during research"
        },
        "files_examined": {
          "type": "array",
          "items": { "type": "string" },
          "description": "Files examined with what was found: file:line — detail"
        },
        "mcp_queries": {
          "type": "array",
          "items": { "type": "string" },
          "description": "MCP queries performed (empty when no MCP tools available)"
        }
      }
    }
  }
}`

// SeedAnalysisSchema returns the JSON schema for seed analysis structured output.
func SeedAnalysisSchema() string {
	return seedAnalysisJSONSchema
}

// --- Stage 1 Prompt Templates ---

// BuildSeedAnalystPrompt constructs the system prompt for the seed analysis subprocess.
// The subprocess runs as a claude CLI with tool access (Grep, Read, Glob, Bash)
// but WITHOUT prism MCP (no circular dependency).
//
// DA review is handled separately by the MCP server after seed analysis completes.
// The subprocess focuses purely on breadth-first research.
//
// Parameters:
//   - topic: user-provided analysis topic/description
//   - contextID: analysis session ID (e.g., "analyze-abc123def456")
//   - seedHints: optional additional guidance for the seed analyst
//   - ontologyScope: JSON string of ontology-scope.json (pre-resolved), or empty
//   - docPaths: list of ontology document root paths for the analyst to search
func BuildSeedAnalystPrompt(topic, contextID, seedHints, ontologyScope string, docPaths []string) string {
	var sb strings.Builder

	sb.WriteString(`You are the SEED ANALYST performing breadth-first research for a multi-perspective analysis.

Your job: actively investigate the given topic using available tools and map the landscape of related code areas, systems, and modules that will inform perspective generation. You focus ONLY on breadth of discovery — perspective selection and deep analysis are handled by separate stages.

CRITICAL: Breadth over depth. Your goal is to discover as many distinct, relevant code areas as possible — NOT to trace a single code path to its root cause. When you find a relevant area, note it and move on to discover the next area. Do NOT follow one trail deeply at the expense of missing other related areas.

TOPIC:
`)
	sb.WriteString(topic)
	sb.WriteString("\n")

	if seedHints != "" {
		sb.WriteString("\nADDITIONAL GUIDANCE:\n")
		sb.WriteString(seedHints)
		sb.WriteString("\n")
	}

	// Provide document root paths for targeted search
	if len(docPaths) > 0 {
		sb.WriteString("\n## Analysis Target Directories\n\n")
		sb.WriteString("Focus your investigation on these registered document/code directories:\n\n")
		for _, p := range docPaths {
			sb.WriteString("- ")
			sb.WriteString(p)
			sb.WriteString("\n")
		}
		sb.WriteString("\nSearch within these directories first. You may also search related areas outside these directories if the evidence trail leads there.\n")
	}

	if ontologyScope != "" {
		sb.WriteString("\n## Reference Documents\n")
		sb.WriteString(ontologyScope)
		sb.WriteString("\n")
	}

	sb.WriteString(`
---

## STEP 1: Active Research

MUST actively investigate using available tools. Do NOT rely solely on the description.

### Research Protocol

1. Start with the topic — extract concrete identifiers (file paths, service names, error messages, policy names, feature names, etc.)
2. Use Grep to search the codebase for each identifier — note file:line references
3. Use Read to examine relevant source files and understand each area's role
4. Note the area and pivot to search for other distinct areas
5. Use Bash (e.g., git log --oneline --since="7 days ago") to check for recent changes in affected areas if relevant
6. Record ALL discovered areas with evidence sources

**Time limit:** Prioritize breadth of discovery. If research exceeds 3 minutes of tool calls, proceed to Step 2 with findings so far.

---

## STEP 2: Research Summary

Synthesize your discoveries into a structured summary that will help the perspective generator determine the best analysis angles.

---

## OUTPUT

After completing your research, output a JSON object with EXACTLY this structure:

- topic: Copy the original topic description exactly
- da_passed: Set to true (DA review is handled externally)
- research.summary: High-level summary to orient the perspective generator
- research.findings: Array of findings, each with:
  - id: Sequential integer starting from 1
  - area: Name of the code area, module, or system
  - description: What this area does and how it relates to the topic
  - source: Evidence source (file:function:line or tool:query)
  - tool_used: Which tool found this (Grep, Read, Bash, or Glob)
- research.key_areas: List the main domains/areas discovered during research
- research.files_examined: Files examined with what was found (file:line — detail)
- research.mcp_queries: Empty array (no MCP tools in this context)

Every finding MUST have a concrete source — no unsourced claims.
`)

	return sb.String()
}

// BuildPerspectiveGeneratorPrompt constructs the system prompt for the perspective
// generation subprocess. This subprocess receives seed-analysis.json content inline
// and outputs structured perspectives.json via --json-schema.
//
// The subprocess runs as a single-turn claude CLI call (--print mode with --json-schema).
// No tool access needed — all input is provided in the prompt.
//
// Parameters:
//   - topic: user-provided analysis topic/description
//   - seedAnalysisJSON: the full content of seed-analysis.json
func BuildPerspectiveGeneratorPrompt(topic, seedAnalysisJSON string) string {
	var sb strings.Builder

	sb.WriteString(`You are the PERSPECTIVE GENERATOR for a multi-perspective analysis engine.

Your job: read the seed analyst's research findings, then generate the optimal set of analysis perspectives WITH tailored analyst prompts for each. You make the strategic decision of WHICH lenses to apply AND HOW each analyst should investigate.

DESCRIPTION:
`)
	sb.WriteString(topic)
	sb.WriteString("\n")

	sb.WriteString(`
---

## STEP 1: Seed Analysis

The seed analyst has completed breadth-first research. Here are their findings:

`)
	sb.WriteString("```json\n")
	sb.WriteString(seedAnalysisJSON)
	sb.WriteString("\n```\n")

	sb.WriteString(`
---

## STEP 2: Analyst Prompt Structure

Every analyst prompt you generate MUST follow this structure:

### Required Sections (in order)

` + "```" + `
You are the {ROLE_NAME}.

CONTEXT:
{CONTEXT}

### Reference Documents
{ONTOLOGY_SCOPE}

{INVESTIGATION_SCOPE}

TASKS:
{TASKS}

OUTPUT:
{OUTPUT_FORMAT}
` + "```" + `

| Section | Description | Your Responsibility |
|---------|-------------|---------------------|
| role | Analyst's role identity (e.g., "POLICY CONFLICT ANALYST") | Create based on perspective |
| investigation_scope | What this analyst should focus on — specific to this case | Write specific scope description |
| tasks | Numbered list of concrete investigation tasks | Create 3-6 tasks grounded in seed findings |
| output_format | Markdown structure for reporting findings | Create appropriate format (tables, sections, checklists) |

Note: CONTEXT and ONTOLOGY_SCOPE are placeholders filled by the orchestrator at spawn time.

### Rules for Task Generation

1. **Evidence-grounded**: Each task MUST relate to specific findings from the seed analysis
2. **Tool-oriented**: Tasks should reference what tools to use (Grep, Read, etc.)
3. **Specific**: "Analyze the payment policy conflicts in ticket-related flows" NOT "Analyze policies"
4. **Completeness**: Tasks should cover the full scope of the perspective
5. **Code-first**: Where applicable, tasks should require citing code paths (file:function:line)

### Rules for Output Format Generation

1. **Structured**: Use tables, headers, or checklists — not free-form prose
2. **Evidence-required**: Every finding slot must include an evidence field
3. **Severity-rated**: Include severity classification appropriate to the topic
4. **Consistent**: All analysts in the same session should use comparable severity scales

---

## STEP 3: Generate Perspectives

Based on the seed analyst's findings, determine the optimal set of analysis perspectives. There are NO predefined perspective templates — you create perspectives tailored to THIS specific topic.

### Perspective Generation Process

1. **Identify key areas** from seed research findings
2. **Determine analysis angles** — what orthogonal lenses would produce the most valuable insights for this topic?
3. **For each perspective**, create:
   - Identity (id, name, scope)
   - Key investigation questions grounded in seed findings
   - Model selection (opus for deep reasoning, sonnet for standard)
   - Full analyst prompt content following the prompt structure above

### Perspective Quality Gate

Each perspective MUST pass ALL checks before inclusion:
1. **Orthogonal** — does NOT overlap analysis scope with other selected perspectives
2. **Evidence-backed** — seed analyst research found evidence this perspective can analyze
3. **Specific** — selected because THIS topic demands it, not generically useful
4. **Actionable** — will produce concrete findings/recommendations, not just observations

If a perspective fails any check → replace or drop it.

Prefer fewer targeted perspectives over broad coverage — each perspective runs through verification, so more perspectives = more verification rounds. Recommend only what the evidence justifies.

### Perspective Count

- Minimum: 2 perspectives
- Typical: 3-5 perspectives
- No hard maximum, but each must pass the quality gate

---

## OUTPUT

Generate a JSON object with this structure:

- perspectives: Array of perspective objects, each with:
  - id: Unique kebab-case identifier (e.g., "policy-conflict-analysis")
  - name: Human-readable perspective name
  - scope: What this perspective examines — specific to THIS case
  - key_questions: 2-4 questions grounded in seed analyst findings
  - model: "opus" for deep cross-referencing/complex reasoning, "sonnet" for standard investigation
  - prompt: Object with role, investigation_scope, tasks, output_format fields
  - rationale: Why THIS topic demands this perspective — cite seed analyst findings
- quality_gate: Object documenting which checks passed (all_orthogonal, all_evidence_backed, all_specific, all_actionable, min_perspectives_met)
- selection_summary: Brief explanation of why these perspectives were chosen

### Field Rules
- perspectives[].id: Unique kebab-case identifier
- perspectives[].scope: MUST be specific to this case, not generic
- perspectives[].key_questions: 2-4 questions, each grounded in seed analyst findings
- perspectives[].prompt.role: Single sentence starting with "You are the..."
- perspectives[].prompt.investigation_scope: Detailed scope description
- perspectives[].prompt.tasks: Numbered list (3-6 tasks), each grounded in specific seed findings
- perspectives[].prompt.output_format: Markdown template with evidence fields
- perspectives[].rationale: MUST cite specific findings from seed-analysis.json
- quality_gate: All booleans must be true for a valid perspective set
- selection_summary: Explain the reasoning
`)

	return sb.String()
}

// --- Ontology Document Path Resolution ---

// ontologyDocsConfig represents the ~/.prism/ontology-docs.json structure.
type ontologyDocsConfig struct {
	Directories []string `json:"directories"`
}

// LoadOntologyDocPaths reads ~/.prism/ontology-docs.json and returns the list of
// registered document root directories. Returns an empty slice if the file
// doesn't exist or can't be parsed.
func LoadOntologyDocPaths() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	configPath := filepath.Join(home, ".prism", "ontology-docs.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil
	}
	var config ontologyDocsConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil
	}
	return config.Directories
}

// --- Stage 1 Config Loader ---

// Stage1Config holds the parameters needed to build Stage 1 prompts,
// extracted from the task's config.json.
type Stage1Config struct {
	Topic         string
	ContextID     string
	Model         string
	StateDir      string
	SeedHints     string
	OntologyScope string
	DocPaths      []string
}

// LoadStage1Config reads config.json from the task's state directory
// and resolves ontology doc paths for the seed analyst.
func LoadStage1Config(task *AnalysisTask) (Stage1Config, error) {
	configPath := filepath.Join(task.StateDir, "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return Stage1Config{}, fmt.Errorf("read config.json: %w", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return Stage1Config{}, fmt.Errorf("parse config.json: %w", err)
	}

	sc := Stage1Config{
		Topic:     stringFromMap(config, "topic"),
		ContextID: stringFromMap(config, "context_id"),
		Model:     stringFromMap(config, "model"),
		StateDir:  stringFromMap(config, "state_dir"),
		SeedHints: stringFromMap(config, "seed_hints"),
	}

	// Load ontology scope if present
	sc.OntologyScope = stringFromMap(config, "ontology_scope")

	// Resolve ontology doc paths from ~/.prism/ontology-docs.json
	sc.DocPaths = LoadOntologyDocPaths()

	return sc, nil
}

// stringFromMap extracts a string value from a map, returning "" if missing or wrong type.
func stringFromMap(m map[string]interface{}, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

// --- Seed Analysis Output Path ---

// SeedAnalysisPath returns the path to seed-analysis.json for a given state directory.
func SeedAnalysisPath(stateDir string) string {
	return filepath.Join(stateDir, "seed-analysis.json")
}

// PerspectivesPath returns the path to perspectives.json for a given state directory.
func PerspectivesPath(stateDir string) string {
	return filepath.Join(stateDir, "perspectives.json")
}
