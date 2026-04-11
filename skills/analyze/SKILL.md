---
name: analyze
description: Runs multi-perspective analysis via MCP server orchestration. Thin wrapper that collects user input then delegates all processing to prism_analyze MCP tool. Ontology scope is automatically resolved from brownfield default repositories. General-purpose analysis engine — any topic can be analyzed.
version: 7.2.0
user-invocable: true
allowed-tools: Read, AskUserQuestion, ToolSearch, mcp__prism__prism_analyze, mcp__prism__prism_task_status, mcp__prism__prism_analyze_result, mcp__prism__prism_cancel_task
---

# Multi-Perspective Analysis

General-purpose analysis engine. Any topic is seeded, researched, and analyzed from dynamically generated perspectives with Socratic verification.

All analysis processing is orchestrated by the MCP server internally. This skill is a thin wrapper that:
1. Collects user input (topic description)
2. Calls `prism_analyze` to start the analysis pipeline (ontology scope is auto-resolved from brownfield defaults by the MCP server)
3. Polls `prism_task_status` for progress updates
4. Retrieves results via `prism_analyze_result` when complete
5. Presents the final report to the user

## Ontology Scope Resolution (Automatic)

The MCP server automatically resolves ontology scope using this priority:
1. **Explicit `ontology_scope` parameter** — if provided via config, used as-is
2. **Brownfield default repositories** — all repos with `is_default=1` in `prism.db` are merged into scope sources
3. **Error** — if neither is available, the server returns an error asking the user to set up brownfield defaults first

No user interaction is required for ontology scope mapping. Use `prism:brownfield` to configure default repositories before running analysis.

## Config-Based Customization

Wrapper skills (e.g., `/prd`) can customize analyze behavior by providing a config file. The config path is passed via `$ARGUMENTS` as `--config <path>`.

When this skill runs from Codex, pass `adaptor: "codex"` in the `prism_analyze` call. When it runs from Claude Code, pass `adaptor: "claude"`. Do not rely on process-global runtime defaults when the caller knows the host runtime.

### Config Schema

```json
{
  "topic": "What to analyze (overrides description if provided)",
  "input_context": "Path to input file (e.g., PRD file path)",
  "report_template": "Path to custom report template (overrides default)",
  "seed_hints": "Additional guidance for seed analyst (e.g., 'Focus on policy domain extraction')",
  "session_id": "Pre-generated session ID (optional — task_id becomes analyze-{session_id})",
  "model": "Claude model override (default: claude-sonnet-4-6)"
}
```

If no config is provided, analyze runs with defaults (topic from user input, default report template, auto-resolved ontology scope from brownfield defaults).

---

## Phase 1: Problem Intake

Main session handles intake — collecting the analysis topic from the user.

### Step 1.1: Parse Arguments & Config

Check if `$ARGUMENTS` contains `--config <path>`:
- **Config provided**: Read the config file. Use `config.topic` as description if present, otherwise fall back to remaining arguments.
- **No config**: Use `$ARGUMENTS` as the description directly. If empty, ask via `AskUserQuestion` (header: "Analysis"): "Please describe what you'd like to analyze."

Store the resolved description and config values.

### Phase 1 Exit Gate

- [ ] Description collected

→ **NEXT ACTION: Proceed to Phase 2 — Start Analysis.**

---

## Phase 2: Select Adaptor

### Step 2.1: Select Who You Are

Before calling `prism_analyze`, explicitly select the host runtime identity:

```
SELECT who you are: codex | claude
```

Selection rule:
- Running in Codex → choose `codex`
- Running in Claude Code → choose `claude`

Store the selected value as `{ADAPTOR}`. Do not infer it later from model names or server-side defaults once this step has been completed.

### Phase 2 Exit Gate

- [ ] `{ADAPTOR}` selected as `codex` or `claude`

→ **NEXT ACTION: Proceed to Phase 3 — Start Analysis via MCP.**

---

## Phase 3: Start Analysis via MCP

### Step 3.1: Call prism_analyze

```
mcp__prism__prism_analyze(
  topic: "{resolved description}",
  adaptor: "{ADAPTOR}",
  session_id: "{config.session_id if provided, otherwise omit}",
  model: "{config.model if provided, otherwise omit to use server default}",
  input_context: "{config.input_context if provided}",
  ontology_scope: "{config.ontology_scope if provided, otherwise omit — server auto-resolves from brownfield defaults}",
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

### Phase 3 Exit Gate

- [ ] `task_id` received from `prism_analyze`

→ **NEXT ACTION: Proceed to Phase 4 — Poll for Progress.**

---

## Phase 4: Progress Polling

### Step 4.1: Poll Status

Poll `prism_task_status` every 30 seconds until status is `completed` or `failed`:

```
mcp__prism__prism_task_status(task_id: "{task_id}")
```

Response includes:
- `status`: "running" | "completed" | "failed"
- `stage`: current stage name (e.g., "scope", "specialists", "interview", "synthesis")
- `progress`: human-readable progress description
- `details`: stage-specific details (e.g., specialist count, completed count)

### Step 4.2: Display Progress

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

### Step 4.3: Handle Cancellation

If the user requests cancellation during polling, call `prism_cancel_task(task_id)` and report the result.

### Step 4.4: Handle Failure

If status is `failed`:
- Display the error message to the user
- Suggest re-running with the same topic

### Phase 4 Exit Gate

- [ ] Status is `completed` or `failed`

→ If completed: **Proceed to Phase 5 — Retrieve & Present Results.**
→ If failed: **Stop and report error.**

---

## Phase 5: Retrieve & Present Results

### Step 5.1: Get Analysis Result

```
mcp__prism__prism_analyze_result(task_id: "{task_id}")
```

Response includes:
- `report_path`: absolute path to the final report file
- `summary`: executive summary extracted from the report

### Step 5.2: Present Report

1. Present the `summary` returned by `prism_analyze_result` to the user
2. Tell the user the full report location: `report_path`

### Phase 5 Exit Gate

- [ ] Report path and summary retrieved
- [ ] Summary presented to user
- [ ] Full report path communicated

---

## Pipeline Summary

```
Phase 1 [intake — collect topic from user]
→ Phase 2 [select adaptor — choose codex or claude]
→ Phase 3 [prism_analyze call — starts MCP server pipeline, scope auto-resolved]
→ Phase 4 [poll prism_task_status — display progress]
→ Phase 5 [prism_analyze_result — present report]
```

The MCP server internally executes the full 4-stage pipeline:
- Stage 1: Scope (seed analysis + DA review + perspective generation)
- Stage 2a: Specialists (parallel finding sessions)
- Stage 2b: Interview (parallel Socratic verification)
- Stage 3: Synthesis (report generation)

All intermediate artifacts are stored at `~/.prism/state/analyze-{id}/`.
Final report is saved to `~/.prism/reports/`.
