package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// --- Perspectives JSON schema for structured output (--json-schema flag) ---
// These structs mirror the perspectives.json format defined in
// skills/analyze/prompts/perspective-generator.md and are used both for:
// 1. Parsing the perspective generator's structured JSON output
// 2. Generating the JSON schema string passed to claude CLI via --json-schema

// AnalystPrompt contains the dynamically generated prompt sections for a specialist.
// Matches perspectives.json → perspectives[].prompt as defined in
// analyst-prompt-structure.md.
type AnalystPrompt struct {
	// Role identity, e.g., "You are the POLICY CONFLICT ANALYST."
	Role string `json:"role"`
	// Detailed scope description for this case
	InvestigationScope string `json:"investigation_scope"`
	// Numbered list of concrete investigation tasks (3-6), grounded in seed findings
	Tasks string `json:"tasks"`
	// Markdown template with evidence fields for reporting
	OutputFormat string `json:"output_format"`
}

// Perspective represents a single analysis perspective with its tailored analyst prompt.
type Perspective struct {
	// Unique kebab-case identifier, e.g., "policy-conflict-analysis"
	ID string `json:"id"`
	// Human-readable name, e.g., "Policy Conflict Analysis"
	Name string `json:"name"`
	// What this perspective examines — specific to THIS case
	Scope string `json:"scope"`
	// 2-4 questions grounded in seed analyst findings
	KeyQuestions []string `json:"key_questions"`
	// Model choice (ignored by orchestrator — fixed model used across all stages)
	Model string `json:"model"`
	// Tailored analyst prompt sections
	Prompt AnalystPrompt `json:"prompt"`
	// Why THIS topic demands this perspective — cites seed analyst findings
	Rationale string `json:"rationale"`
}

// PerspectiveQualityGate documents which quality checks the perspective set passed.
type PerspectiveQualityGate struct {
	AllOrthogonal    bool `json:"all_orthogonal"`
	AllEvidenceBacked bool `json:"all_evidence_backed"`
	AllSpecific      bool `json:"all_specific"`
	AllActionable    bool `json:"all_actionable"`
	MinPerspectivesMet bool `json:"min_perspectives_met"`
}

// PerspectivesOutput is the top-level structure for perspectives.json.
// This is the complete output of the perspective generator stage.
type PerspectivesOutput struct {
	Perspectives     []Perspective          `json:"perspectives"`
	QualityGate      PerspectiveQualityGate `json:"quality_gate"`
	SelectionSummary string                 `json:"selection_summary"`
}

// perspectivesJSONSchema is the JSON schema for PerspectivesOutput,
// used with claude CLI's --json-schema flag to enforce structured output.
// Generated as a raw JSON string to avoid runtime reflection overhead.
const perspectivesJSONSchema = `{
  "type": "object",
  "required": ["perspectives", "quality_gate", "selection_summary"],
  "additionalProperties": false,
  "properties": {
    "perspectives": {
      "type": "array",
      "minItems": 2,
      "items": {
        "type": "object",
        "required": ["id", "name", "scope", "key_questions", "model", "prompt", "rationale"],
        "additionalProperties": false,
        "properties": {
          "id": {
            "type": "string",
            "description": "Unique kebab-case identifier for this perspective"
          },
          "name": {
            "type": "string",
            "description": "Human-readable perspective name"
          },
          "scope": {
            "type": "string",
            "description": "What this perspective examines, specific to this case"
          },
          "key_questions": {
            "type": "array",
            "minItems": 2,
            "maxItems": 4,
            "items": { "type": "string" },
            "description": "Questions grounded in seed analyst findings"
          },
          "model": {
            "type": "string",
            "enum": ["opus", "sonnet"],
            "description": "Model complexity tier (ignored by orchestrator - fixed model used)"
          },
          "prompt": {
            "type": "object",
            "required": ["role", "investigation_scope", "tasks", "output_format"],
            "additionalProperties": false,
            "properties": {
              "role": {
                "type": "string",
                "description": "Role identity starting with 'You are the...'"
              },
              "investigation_scope": {
                "type": "string",
                "description": "Detailed scope description for this case"
              },
              "tasks": {
                "type": "string",
                "description": "Numbered list of 3-6 concrete investigation tasks"
              },
              "output_format": {
                "type": "string",
                "description": "Markdown template with evidence fields"
              }
            }
          },
          "rationale": {
            "type": "string",
            "description": "Why this topic demands this perspective, citing seed findings"
          }
        }
      }
    },
    "quality_gate": {
      "type": "object",
      "required": ["all_orthogonal", "all_evidence_backed", "all_specific", "all_actionable", "min_perspectives_met"],
      "additionalProperties": false,
      "properties": {
        "all_orthogonal": { "type": "boolean" },
        "all_evidence_backed": { "type": "boolean" },
        "all_specific": { "type": "boolean" },
        "all_actionable": { "type": "boolean" },
        "min_perspectives_met": { "type": "boolean" }
      }
    },
    "selection_summary": {
      "type": "string",
      "description": "Brief explanation of why these perspectives were chosen"
    }
  }
}`

// PerspectivesSchema returns the JSON schema string for use with
// claude CLI's --json-schema flag to enforce structured output from
// the perspective generator subprocess.
func PerspectivesSchema() string {
	return perspectivesJSONSchema
}

// ReadPerspectives reads and parses perspectives.json from disk.
func ReadPerspectives(path string) (PerspectivesOutput, error) {
	var p PerspectivesOutput
	data, err := os.ReadFile(path)
	if err != nil {
		return p, fmt.Errorf("read perspectives: %w", err)
	}
	if err := json.Unmarshal(data, &p); err != nil {
		return p, fmt.Errorf("parse perspectives: %w", err)
	}
	return p, nil
}

// WritePerspectives writes a PerspectivesOutput to disk as formatted JSON.
func WritePerspectives(path string, p PerspectivesOutput) error {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal perspectives: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write perspectives: %w", err)
	}
	return nil
}

// ValidatePerspectives performs basic validation on a parsed PerspectivesOutput.
// Returns nil if valid, or an error describing the first validation failure.
func ValidatePerspectives(p PerspectivesOutput) error {
	if len(p.Perspectives) < 2 {
		return fmt.Errorf("minimum 2 perspectives required, got %d", len(p.Perspectives))
	}

	ids := make(map[string]bool, len(p.Perspectives))
	for i, persp := range p.Perspectives {
		if persp.ID == "" {
			return fmt.Errorf("perspective[%d]: id is required", i)
		}
		if ids[persp.ID] {
			return fmt.Errorf("perspective[%d]: duplicate id %q", i, persp.ID)
		}
		ids[persp.ID] = true

		if persp.Name == "" {
			return fmt.Errorf("perspective[%d] (%s): name is required", i, persp.ID)
		}
		if persp.Scope == "" {
			return fmt.Errorf("perspective[%d] (%s): scope is required", i, persp.ID)
		}
		if len(persp.KeyQuestions) < 2 || len(persp.KeyQuestions) > 4 {
			return fmt.Errorf("perspective[%d] (%s): key_questions must have 2-4 items, got %d", i, persp.ID, len(persp.KeyQuestions))
		}
		if persp.Prompt.Role == "" {
			return fmt.Errorf("perspective[%d] (%s): prompt.role is required", i, persp.ID)
		}
		if persp.Prompt.InvestigationScope == "" {
			return fmt.Errorf("perspective[%d] (%s): prompt.investigation_scope is required", i, persp.ID)
		}
		if persp.Prompt.Tasks == "" {
			return fmt.Errorf("perspective[%d] (%s): prompt.tasks is required", i, persp.ID)
		}
		if persp.Prompt.OutputFormat == "" {
			return fmt.Errorf("perspective[%d] (%s): prompt.output_format is required", i, persp.ID)
		}
		if persp.Rationale == "" {
			return fmt.Errorf("perspective[%d] (%s): rationale is required", i, persp.ID)
		}
	}

	if p.SelectionSummary == "" {
		return fmt.Errorf("selection_summary is required")
	}

	return nil
}
