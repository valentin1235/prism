---
name: analyze
description: Runs multi-perspective analysis via MCP server orchestration. Thin wrapper that handles user interaction (ontology scope mapping) then delegates all processing to prism_analyze MCP tool. General-purpose analysis engine — any topic can be analyzed against ontology documents.
version: 6.0.0
user-invocable: true
allowed-tools: Read, Glob, Grep, Bash, Write, ToolSearch, AskUserQuestion, mcp__prism-mcp__prism_analyze, mcp__prism-mcp__prism_task_status, mcp__prism-mcp__prism_analyze_result, mcp__prism-mcp__prism_docs_roots, mcp__prism-mcp__prism_docs_list, mcp__prism-mcp__prism_docs_read, mcp__prism-mcp__prism_docs_search
---

# Multi-Perspective Analysis

General-purpose analysis engine. Any topic is seeded, researched, and analyzed from dynamically generated perspectives with Socratic verification.

All analysis processing is orchestrated by the MCP server internally. This skill is a thin wrapper that:
1. Collects user input and resolves ontology scope (user interaction required)
2. Calls `prism_analyze` to start the analysis pipeline
3. Polls `prism_task_status` for progress updates
4. Retrieves results via `prism_analyze_result` when complete
5. Presents the final report to the user

## Config-Based Customization

Wrapper skills (e.g., `/prd`) can customize analyze behavior by providing a config file. The config path is passed via `$ARGUMENTS` as `--config <path>`.

### Config Schema

```json
{
  "topic": "What to analyze (overrides description if provided)",
  "input_context": "Path to input file (e.g., PRD file path)",
  "report_template": "Path to custom report template (overrides default)",
  "seed_hints": "Additional guidance for seed analyst (e.g., 'Focus on policy domain extraction')",
  "ontology_mode": "required|optional (default: optional)"
}
```

If no config is provided, analyze runs with defaults (topic from user input, default report template, optional ontology).

---

## Phase 1: Problem Intake & Ontology Scope

Main session handles intake and ontology scope mapping — these require user interaction.

### Step 1.1: Parse Arguments & Config

Check if `$ARGUMENTS` contains `--config <path>`:
- **Config provided**: Read the config file. Use `config.topic` as description if present, otherwise fall back to remaining arguments.
- **No config**: Use `$ARGUMENTS` as the description directly. If empty, ask via `AskUserQuestion` (header: "Analysis"): "Please describe what you'd like to analyze."

Store the resolved description and config values.

### Step 1.2: Ontology Scope Mapping

> Read and execute `protocols/ontology-scope-mapping.md` with:
- `{AVAILABILITY_MODE}` = config's `ontology_mode` if present, otherwise `optional`
- `{CALLER_CONTEXT}` = `"analysis"`
- `{STATE_DIR}` = (temporary — ontology scope is passed to MCP tool, not written to disk)

Resolve ontology scope to a JSON string mapping perspective IDs to document paths. If `ONTOLOGY_AVAILABLE=false` → pass `null` as `ontology_scope`.

### Phase 1 Exit Gate

- [ ] Description collected
- [ ] Ontology scope resolved (JSON string or null)

→ **NEXT ACTION: Proceed to Phase 2 — Start Analysis.**

---

## Phase 2: Start Analysis via MCP

### Step 2.1: Call prism_analyze

```
mcp__prism-mcp__prism_analyze(
  topic: "{resolved description}",
  model: "{model from config or default}",
  input_context: "{config.input_context if provided}",
  ontology_scope: "{ontology scope JSON string or omit if null}",
  seed_hints: "{config.seed_hints if provided}",
  report_template: "{config.report_template if provided}"
)
```

The MCP server returns immediately with:
```json
{
  "task_id": "analyze-xxxxxxxx",
  "status": "running",
  "message": "Analysis started"
}
```

Store the `task_id` for polling.

### Phase 2 Exit Gate

- [ ] `task_id` received from `prism_analyze`

→ **NEXT ACTION: Proceed to Phase 3 — Poll for Progress.**

---

## Phase 3: Progress Polling

### Step 3.1: Poll Status

Poll `prism_task_status` every 30 seconds until status is `completed` or `failed`:

```
mcp__prism-mcp__prism_task_status(task_id: "{task_id}")
```

Response includes:
- `status`: "running" | "completed" | "failed"
- `stage`: current stage name (e.g., "scope", "specialists", "interview", "synthesis")
- `progress`: human-readable progress description
- `details`: stage-specific details (e.g., specialist count, completed count)

### Step 3.2: Display Progress

On each poll, display the current stage and progress to the user. Format as a brief status update:

```
🔍 Analysis in progress...
  Stage: {stage} — {progress}
```

If status changes to a new stage, announce it:
```
✅ {previous_stage} complete
🔍 Starting {new_stage}...
```

### Step 3.3: Handle Failure

If status is `failed`:
- Display the error message to the user
- Suggest re-running with the same topic

### Phase 3 Exit Gate

- [ ] Status is `completed` or `failed`

→ If completed: **Proceed to Phase 4 — Retrieve & Present Results.**
→ If failed: **Stop and report error.**

---

## Phase 4: Retrieve & Present Results

### Step 4.1: Get Analysis Result

```
mcp__prism-mcp__prism_analyze_result(task_id: "{task_id}")
```

Response includes:
- `report_path`: absolute path to the final report file
- `summary`: executive summary extracted from the report

### Step 4.2: Present Report

1. Read the report file at `report_path`
2. Present the executive summary to the user
3. Tell the user the full report location: `report_path`

### Phase 4 Exit Gate

- [ ] Report path and summary retrieved
- [ ] Summary presented to user
- [ ] Full report path communicated

---

## Pipeline Summary

```
Phase 1 [intake + ontology scope — user interaction in main session]
→ Phase 2 [prism_analyze call — starts MCP server pipeline]
→ Phase 3 [poll prism_task_status — display progress]
→ Phase 4 [prism_analyze_result — present report]
```

The MCP server internally executes the full 4-stage pipeline:
- Stage 1: Scope (seed analysis + DA review + perspective generation)
- Stage 2a: Specialists (parallel finding sessions)
- Stage 2b: Interview (parallel Socratic verification)
- Stage 3: Synthesis (report generation)

All intermediate artifacts are stored at `~/.prism/state/analyze-{id}/`.
Final report is saved to `~/.prism/reports/`.
