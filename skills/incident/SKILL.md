---
name: incident
description: Incident root cause analysis with UX impact — takes an incident description (text + optional screenshots) as input, runs multi-perspective analysis including UX impact perspective via prism_analyze MCP pipeline, producing a developer-facing RCA report. Use this skill for "incident analysis", "incident postmortem", "RCA", "root cause analysis", "incident review", "장애 분석", "인시던트 분석", "장애 리뷰", "포스트모템", or any request about analyzing an incident or outage.
version: 2.0.0
user-invocable: true
allowed-tools: Read, Glob, Grep, Bash, Write, ToolSearch, AskUserQuestion, WebFetch, WebSearch, mcp__prism__prism_analyze, mcp__prism__prism_task_status, mcp__prism__prism_analyze_result, mcp__prism__prism_cancel_task, mcp__prism__prism_docs_roots, mcp__prism__prism_docs_list, mcp__prism__prism_docs_read, mcp__prism__prism_docs_search
---

# Incident RCA Analysis (Thin Wrapper for prism_analyze)

Takes an incident description (text + optional screenshots) as input, runs multi-perspective analysis via `prism_analyze` MCP pipeline with UX impact perspective injection and language-aware synthesis, producing a developer-facing RCA report directly from the MCP server.

This skill is a thin wrapper that:
1. Collects incident input and resolves ontology scope (user interaction required)
2. Calls `prism_analyze` with incident-specific config including `perspective_injection` and `language`
3. Polls `prism_task_status` for progress updates
4. Retrieves results via `prism_analyze_result` when complete
5. Presents the final RCA report to the user

---

## Phase 0: Incident Input Collection

This phase performs **only** input collection — no ontology resolution, no MCP calls.

### Step 0.1: Get Incident Description

Extract the incident description from `$ARGUMENTS`.

- Description provided → store as `{INCIDENT_DESCRIPTION}`
- No description → `AskUserQuestion` (header: "Incident", question: "Please describe the incident to analyze. You can include text description and optional screenshot paths.")

### Step 0.2: Screenshot Text Extraction

If the description references image/screenshot paths:
1. Verify each file exists via `Read`
2. For each screenshot, read the file content using the `Read` tool (which supports multimodal image reading)
3. Incorporate the visual content description as text into `{INCIDENT_DESCRIPTION}`

Store the enriched description (original text + inlined screenshot descriptions) as `{INCIDENT_DESCRIPTION}`.

If no screenshots are referenced, skip this step.

### Step 0.3: Language Detection

1. If CLAUDE.md contains a `Language` directive → use that language
2. Otherwise → detect from user's input language in this session
3. Store as `{REPORT_LANGUAGE}` (e.g., "ko", "en", "ja")

### Phase 0 Exit Gate

- [ ] `{INCIDENT_DESCRIPTION}` collected (with screenshot content inlined if any)
- [ ] `{REPORT_LANGUAGE}` determined

→ **NEXT ACTION: Proceed to Phase 1 — Resolve Scope.**

---

## Phase 1: Resolve SKILL_DIR & Ontology Scope

### Step 1.1: Resolve SKILL_DIR

Determine the absolute path of the directory containing this SKILL.md:

Use `Glob` to find the skill directory:

```
Glob(pattern="**/skills/incident/SKILL.md")
```

Extract the directory path from the first match (remove `/SKILL.md` suffix). Store as `{SKILL_DIR}`.

If Glob returns no results, fall back to `~/prism/skills/incident` as default path and verify it exists via `Bash(ls {path}/SKILL.md)`.

### Step 1.2: Ontology Scope Mapping

> Read and execute `../analyze/protocols/ontology-scope-mapping.md` (relative to `{SKILL_DIR}`) with:
- `{AVAILABILITY_MODE}` = `optional`
- `{CALLER_CONTEXT}` = `"incident analysis"`
- `{STATE_DIR}` = Generate a short-id via `Bash(uuidgen | tr '[:upper:]' '[:lower:]' | cut -c1-8)`, then `mkdir -p ~/.prism/state/analyze-{short-id}` and use that path. Store `{short-id}` for use as `session_id`.

Resolve ontology scope to a JSON string in canonical `{"sources": [...]}` format. If `ONTOLOGY_AVAILABLE=false` → pass `null` as `ontology_scope`.

### Phase 1 Exit Gate

- [ ] `{SKILL_DIR}` resolved
- [ ] Ontology scope resolved (JSON string or null)
- [ ] `{short-id}` generated and `~/.prism/state/analyze-{short-id}/` directory created

→ **NEXT ACTION: Proceed to Phase 2 — Start Analysis.**

---

## Phase 2: Start Analysis via MCP

### Step 2.1: Call prism_analyze

```
mcp__prism__prism_analyze(
  topic: "Incident root cause analysis: {first 80 chars of INCIDENT_DESCRIPTION} — multi-perspective analysis of root cause, contributing factors, and user-facing UX impact\n\n{full INCIDENT_DESCRIPTION with inlined screenshot descriptions}",
  session_id: "{short-id}",
  ontology_scope: "{ontology scope JSON string or omit if null}",
  seed_hints: "This is an incident/outage analysis. Research directions: (1) Identify the trigger — the immediate event or change that initiated the incident (recent deploys, config changes, dependency updates). (2) Trace the root cause chain — follow error propagation from the trigger through the system to understand why it caused failure. (3) Map contributing factors — discover code areas with missing error handling, absent monitoring/alerting, inadequate fallbacks, or insufficient test coverage that allowed the incident to escalate. (4) Reconstruct timeline evidence — look for logs, metrics, deployment timestamps, and commit history that establish when the incident started, escalated, was detected, and resolved. (5) Discover user-facing impact paths — trace how the technical failure propagated to user-facing components (API responses, UI rendering, data consistency). Perspectives should cover technical root cause, system architecture implications, operational gaps, and error handling resilience. Use available tools (Grep, Read, Bash, MCP) to trace the incident through the codebase. Prioritize breadth: discover as many distinct affected code areas and systems as possible.",
  report_template: "{SKILL_DIR}/templates/report.md",
  perspective_injection: "{SKILL_DIR}/perspectives/ux-impact.json",
  language: "{REPORT_LANGUAGE}"
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
mcp__prism__prism_task_status(task_id: "{task_id}")
```

Response includes:
- `status`: "running" | "completed" | "failed"
- `stage`: current stage name (e.g., "scope", "specialists", "interview", "synthesis")
- `progress`: human-readable progress description
- `details`: stage-specific details (e.g., specialist count, completed count)

### Step 3.2: Display Progress

On each poll, display the current stage and progress to the user. Format as a brief status update:

```
🔍 Incident analysis in progress...
  Stage: {stage} — {progress}
```

If status changes to a new stage, announce it:
```
✅ {previous_stage} complete
🔍 Starting {new_stage}...
```

### Step 3.3: Handle Cancellation

If the user requests cancellation during polling, call `prism_cancel_task(task_id)` and report the result.

### Step 3.4: Handle Failure

If status is `failed`:
- Display the error message to the user
- Suggest re-running with the same incident description

### Phase 3 Exit Gate

- [ ] Status is `completed` or `failed`

→ If completed: **Proceed to Phase 4 — Retrieve & Present Results.**
→ If failed: **Stop and report error.**

---

## Phase 4: Retrieve & Present Results

### Step 4.1: Get Analysis Result

```
mcp__prism__prism_analyze_result(task_id: "{task_id}")
```

Response includes:
- `report_path`: absolute path to the final RCA report file
- `summary`: executive summary extracted from the report

### Step 4.2: Present Report

1. Read the report file at `report_path`
2. Present the executive summary to the user
3. Tell the user the full report location: `report_path`
4. Mention that raw analysis artifacts are at `~/.prism/state/analyze-{short-id}/`

### Phase 4 Exit Gate

- [ ] Report path and summary retrieved
- [ ] Summary presented to user
- [ ] Full report path communicated

---

## Pipeline Summary

```
Phase 0 [incident input collection — description, screenshot extraction, language detection]
→ Phase 1 [SKILL_DIR + ontology scope — user interaction in main session]
→ Phase 2 [prism_analyze call with perspective_injection + language — starts MCP server pipeline]
→ Phase 3 [poll prism_task_status — display progress]
→ Phase 4 [prism_analyze_result — present RCA report]
```

The MCP server internally executes the full 4-stage pipeline:
- Stage 1: Scope (seed analysis + DA review + perspective generation + UX perspective injection merge)
- Stage 2a: Specialists (parallel finding sessions including UX impact analyst)
- Stage 2b: Interview (parallel Socratic verification)
- Stage 3: Synthesis (RCA report generation in specified language using report template)

All intermediate artifacts are stored at `~/.prism/state/analyze-{short-id}/`.
Final report is saved to `~/.prism/reports/`.
