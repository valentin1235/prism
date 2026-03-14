# Analyst Prompt Structure

This document defines the required structure for dynamically generated analyst prompts. The perspective generator MUST follow this structure when creating prompts for each analyst.

## Required Sections

Every analyst prompt MUST contain these sections in order:

```
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
```

### Section Descriptions

| Section | Description | Generator Responsibility |
|---------|-------------|------------------------|
| `ROLE_NAME` | Analyst's role identity (e.g., "POLICY CONFLICT ANALYST", "ROOT CAUSE ANALYST") | Generator creates based on perspective |
| `CONTEXT` | Injected by orchestrator at spawn time from `context.json` | Placeholder — orchestrator fills |
| `ONTOLOGY_SCOPE` | Injected by orchestrator at spawn time from `ontology-scope.json` | Placeholder — orchestrator fills |
| `INVESTIGATION_SCOPE` | What this analyst should focus on — specific to this case | Generator writes specific scope description |
| `TASKS` | Numbered list of concrete investigation tasks | Generator creates 3-6 tasks grounded in seed findings |
| `OUTPUT_FORMAT` | Markdown structure for reporting findings | Generator creates appropriate format (tables, sections, checklists) |

### Rules for Task Generation

1. **Evidence-grounded**: Each task MUST relate to specific findings from seed-analysis.json
2. **Tool-oriented**: Tasks should reference what tools to use (Grep, Read, MCP docs, etc.)
3. **Specific**: "Analyze the payment policy conflicts in ticket-related flows" NOT "Analyze policies"
4. **Completeness**: Tasks should cover the full scope of the perspective
5. **Code-first**: Where applicable, tasks should require citing code paths (file:function:line)

### Rules for Output Format Generation

1. **Structured**: Use tables, headers, or checklists — not free-form prose
2. **Evidence-required**: Every finding slot must include an evidence field
3. **Severity-rated**: Include severity classification appropriate to the topic
4. **Consistent**: All analysts in the same session should use comparable severity scales

## Integration Points

The orchestrator concatenates the generated prompt with:
- **Before**: Worker preamble (team coordination protocol)
- **After**: Finding protocol (`finding-protocol.md`) or Verification protocol (`verification-protocol.md`)

The generator should NOT include team coordination or finding/verification lifecycle instructions — those are handled by the protocols.

### Placeholder Mapping

| Template Placeholder | Filled From | Filled By |
|---------------------|-------------|-----------|
| `{ROLE_NAME}` | `perspectives.json → prompt.role` | Perspective Generator |
| `{INVESTIGATION_SCOPE}` | `perspectives.json → prompt.investigation_scope` | Perspective Generator |
| `{TASKS}` | `perspectives.json → prompt.tasks` | Perspective Generator |
| `{OUTPUT_FORMAT}` | `perspectives.json → prompt.output_format` | Perspective Generator |
| `{CONTEXT}` | `context.json → summary + research_summary` | Orchestrator (at spawn) |
| `{ONTOLOGY_SCOPE}` | `ontology-scope.json` | Orchestrator (at spawn) |
| `{SHORT_ID}` | Session short ID | Orchestrator (at spawn) |
| `{perspective-id}` | `perspectives.json → id` | Orchestrator (at spawn) |
| `{KEY_QUESTIONS}` | `perspectives.json → key_questions` | Orchestrator (Phase 1 only) |
| `{ORIGINAL_INPUT}` | `context.json → summary` | Orchestrator (Phase 1 only) |
| `{TOPIC_SUMMARY}` | `context.json → summary` | Orchestrator (Phase 2B only) |
