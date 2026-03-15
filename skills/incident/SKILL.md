---
name: incident
description: Incident root cause analysis with UX impact — takes an incident description (text + optional screenshots) as input, runs multi-perspective analysis including UX impact perspective via analyze, then post-processes into a developer-facing RCA report. Use this skill for "incident analysis", "incident postmortem", "RCA", "root cause analysis", "incident review", "장애 분석", "인시던트 분석", "장애 리뷰", "포스트모템", or any request about analyzing an incident or outage.
version: 1.0.0
user-invocable: true
allowed-tools: Skill, Task, Read, Write, Bash, Glob, Grep, AskUserQuestion, ToolSearch
---

# Incident RCA Analysis (Wrapper for analyze)

Takes an incident description (text + optional screenshots) as input, runs multi-perspective analysis via `prism:analyze` with UX impact perspective guidance, then post-processes the results into a developer-facing RCA report.

## Phase 0: Input

### Step 0.1: Get Incident Description

Extract the incident description from `$ARGUMENTS`.

- Description provided → store as `{INCIDENT_DESCRIPTION}`
- No description → `AskUserQuestion` (header: "Incident", question: "Please describe the incident to analyze. You can include text description and optional screenshot paths.")
- If the description references image/screenshot paths → verify files exist via `Read`. Store paths as `{SCREENSHOT_PATHS}` (comma-separated). If no images, set `{SCREENSHOT_PATHS}` to empty string.

### Step 0.2: Generate Session ID

```bash
uuidgen | tr '[:upper:]' '[:lower:]' | cut -c1-8
```

Generate ONCE, reuse throughout. Create state directories for both incident and analyze (shared session ID):

```bash
mkdir -p ~/.prism/state/incident-{short-id}
mkdir -p ~/.prism/state/analyze-{short-id}
```

### Step 0.3: Language Detection

1. If CLAUDE.md contains a `Language` directive → use that language
2. Otherwise → detect from user's input language in this session
3. Store as `{REPORT_LANGUAGE}`

### Phase 0 Exit Gate

- [ ] Incident description collected and stored as `{INCIDENT_DESCRIPTION}`
- [ ] `{SCREENSHOT_PATHS}` determined (may be empty)
- [ ] `{short-id}` generated and `~/.prism/state/incident-{short-id}/` directory created
- [ ] `{REPORT_LANGUAGE}` determined

→ **NEXT: Phase 1 — Create config and invoke analyze**

---

## Phase 1: Config & Analyze Invocation

### Step 1.1: Prepare Context

Combine the incident description and any screenshot references into a single context block.

If `{SCREENSHOT_PATHS}` is not empty, append to the description:
```
Referenced screenshots: {SCREENSHOT_PATHS}
```

### Step 1.2: Create Analyze Config

Write the following JSON to `~/.prism/state/incident-{short-id}/analyze-config.json`:

```json
{
  "topic": "Incident root cause analysis: {first 80 chars of INCIDENT_DESCRIPTION} — multi-perspective analysis of root cause, contributing factors, and user-facing UX impact",
  "input_context": "{INCIDENT_DESCRIPTION with screenshot paths if any}",
  "report_template": "{SKILL_DIR}/templates/report.md",
  "seed_hints": "This is an incident/outage analysis. Investigate root cause, contributing factors, and timeline. Perspectives should cover technical root cause, system architecture implications, and operational gaps. Use available tools (Grep, Read, Bash, MCP) to trace the incident through the codebase.",
  "ontology_mode": "optional",
  "session_id": "{short-id}"
}
```

> Determine the absolute path of the directory containing this SKILL.md via `Bash`. Store it as `{SKILL_DIR}` for use in Step 2.1.

### Step 1.3: Write Perspective Injection

Copy the UX impact perspective to the analyze state directory. This file will be merged into `perspectives.json` by analyze's merge script after perspective generation.

```bash
cp {SKILL_DIR}/perspectives/ux-impact.json ~/.prism/state/analyze-{short-id}/perspective_injection.json
```

### Step 1.4: Invoke Analyze

```
Skill(skill="prism:analyze", args="--config ~/.prism/state/incident-{short-id}/analyze-config.json")
```

Wait for analyze to complete. If analyze fails or the user cancels mid-execution → ERROR: "Analyze skill failed or was cancelled. Check ~/.prism/state/ for partial results." and terminate.

Analyze internally handles:
- Seed analyst investigation of incident and related code areas
- Multi-perspective generation + merging injected UX perspective (from perspective_injection.json)
- Per-perspective analyst spawning
- Socratic verification of findings
- Report generation

### Step 1.5: Locate Analyze Output

The analyze state directory is already known: `~/.prism/state/analyze-{short-id}` (shared session ID).

Verify the following files exist:
- `~/.prism/state/analyze-{short-id}/analyst-findings.md` — verified analysis results
- `~/.prism/state/analyze-{short-id}/verification-log.json` — Socratic verification scores (may not exist — this is tolerated because the post-processor has a 3-tier fallback for confidence scores)

Store `~/.prism/state/analyze-{short-id}` as `{ANALYZE_STATE_DIR}`.

### Phase 1 Exit Gate

- [ ] `analyze-config.json` written
- [ ] `prism:analyze` skill invocation completed
- [ ] `{ANALYZE_STATE_DIR}` identified and `analyst-findings.md` exists

→ **NEXT: Phase 2 — Post-processing (RCA report generation)**

---

## Phase 2: Post-Processing (RCA Report Generation)

The output from analyze is a multi-perspective analysis report. A post-processor agent transforms it into a developer-facing RCA report with UX impact analysis.

### Step 2.1: Spawn Post-Processor Agent

Read `prompts/post-processor.md` (relative to this SKILL.md).

```
Task(
  subagent_type="prism:finder",
  model="opus",
  prompt="{post-processor prompt with placeholders replaced}"
)
```

**CRITICAL: Do NOT add `run_in_background=true`.** Must wait for post-processing results.

Placeholder replacements:
- `{ANALYZE_STATE_DIR}` → analyze result directory path identified in Step 1.5
- `{INCIDENT_DESCRIPTION}` → original incident description
- `{INCIDENT_STATE_DIR}` → `~/.prism/state/incident-{short-id}`
- `{REPORT_LANGUAGE}` → language determined in Phase 0.3
- `{SHORT_ID}` → session ID
- `{REPORT_TEMPLATE_PATH}` → `{SKILL_DIR}/templates/report.md` (absolute path, from Step 1.2)

### Step 2.2: Verify Output

After post-processor agent completes, verify report file exists:

```
~/.prism/state/incident-{short-id}/incident-rca-report.md
```

If missing → ERROR: "Post-processor agent failed to generate report."

### Phase 2 Exit Gate

- [ ] Post-processor agent completed
- [ ] `incident-rca-report.md` exists
- [ ] Report contains "Root Cause" section (verify via `Grep`)

→ **NEXT: Phase 3 — Deliver report**

---

## Phase 3: Output

### Step 3.1: Report to User

Inform the user of the results:

```
Incident RCA analysis complete.

Report location:
- ~/.prism/state/incident-{short-id}/incident-rca-report.md

Analyze raw results: {ANALYZE_STATE_DIR}/
```
