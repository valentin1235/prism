---
name: prd
description: Multi-perspective PRD policy conflict analysis with devil's advocate verification
version: 1.0.0
user-invocable: true
allowed-tools: Task, SendMessage, TeamCreate, TeamDelete, TaskCreate, TaskUpdate, TaskList, TaskGet, AskUserQuestion, Read, Glob, Grep, Bash, Write, ToolSearch, mcp__ontology-docs__directory_tree, mcp__ontology-docs__list_directory, mcp__ontology-docs__read_file, mcp__ontology-docs__read_text_file, mcp__ontology-docs__read_multiple_files, mcp__ontology-docs__search_files
---

# PRD Multi-Perspective Policy Analysis

Analyze a PRD against existing policy documents (via ontology-docs MCP) to find policy-level conflicts and ambiguities using a coordinated agent team.

## Input

### MUST: Minimum Required

| Item | Required | Method |
|------|----------|--------|
| PRD file path | YES | Argument or `AskUserQuestion` |
| Report output path | NO | Defaults to `prd-policy-review-report.md` in PRD file's directory |

```
/prd-team-analysis @path/to/prd.md
```

Reference docs are accessed via `ontology-docs` MCP — no path input needed.
If PRD file not found, error: `"PRD file not found: {path}"`.

## Phase 1: PRD Analysis & Perspective Generation

### 1.0 Language Detection

Detect report language from user's environment:
1. Check if CLAUDE.md contains `Language` directive → use that language
2. Otherwise, detect from user's input language in this session
3. Store as `{REPORT_LANGUAGE}` for Phase 5

Default: user's detected language.

### 1.1 Read PRD

Read full PRD via `Read`. Also read any sibling files (handoff, constraints) in the same directory.

### 1.2 Generate Perspectives

Analyze PRD functional requirements and cross-reference with ontology-docs domains to derive **N orthogonal policy analysis perspectives**.

Rules:
- Each perspective covers a **policy ontology unit** (e.g., ticket policy, payment policy, retention policy)
- **Orthogonality** between perspectives — no overlapping domains
- Minimum 3, maximum 6 perspectives

Perspective definition format:
```
ID: {slug}
Name: {perspective name}
Scope: {policy domain this perspective examines}
PRD sections: {FR-N, NFR-N, etc.}
```

## Phase 2: Team Setup & Task Assignment

### 2.1 TeamCreate

```json
{
  "team_name": "prd-policy-review",
  "description": "PRD multi-perspective policy analysis: {PRD title}"
}
```

### 2.2 TaskCreate — Per-Perspective Analysts

Create 1 task per perspective. MUST include in `description`:
- Summary of relevant PRD content (FR/NFR for this perspective)
- Analysis scope (policy domains to explore via ontology-docs MCP)
- Analysis rules (see "Analyst Behavior Rules" below)

### 2.3 TaskCreate — Devil's Advocate

Create DA task with `addBlockedBy` depending on ALL analyst tasks.

### 2.4 Pre-assign Owners

MUST: Pre-assign owners via `TaskUpdate(owner="{worker-name}")` before spawning. Prevents race conditions.

## Phase 3: Spawn & Execute Analysts

### 3.1 Spawn Analysts

Spawn all analysts **in parallel**.

```
Task(
  subagent_type="oh-my-claudecode:analyst",
  model="opus",
  team_name="prd-policy-review",
  name="{perspective-slug}-analyst",
  run_in_background=true,
  prompt="{worker preamble + analyst prompt}"
)
```

### 3.2 Analyst Prompt Structure

Worker preamble (shared by all analysts):
```
You are a TEAM WORKER in team "prd-policy-review". Your name is "{worker-name}".
You report to the team lead ("team-lead").

== WORK PROTOCOL ==
1. TaskList → find my assigned task → TaskUpdate(status="in_progress")
2. Use ontology-docs MCP tools (directory_tree, search_files, read_file, etc.) to explore and read relevant domain docs
3. Cross-reference PRD sections against docs to find policy conflicts/ambiguities
4. Report findings via SendMessage to team-lead
5. TaskUpdate(status="completed")
6. On shutdown_request → respond with shutdown_response(approve=true)
```

Analyst behavior rules (include in prompt):
```
== ANALYSIS RULES ==
- Report ONLY planning-level policy conflicts/ambiguities. Exclude dev implementation details.
- Format: "PRD says X, but docs say Y" — always cite both sides.
- Severity: CRITICAL (policy conflict, feature blocking) / HIGH (ambiguous, multiple interpretations) / MEDIUM (undefined, edge case)
- MUST cite ontology-docs filename and section as evidence.

== REPORT FORMAT ==
Per issue:
### [{SEVERITY}-{N}] {Title}
- **PRD states**: {what PRD defines}
- **Existing policy**: {what ontology-docs says, filename:section}
- **Conflict/Ambiguity**: {why this is a problem}
- **Decision needed**: checklist of items PM must decide
```

### 3.3 Spawn Devil's Advocate

After all analysts complete (DA task blockedBy resolved), spawn DA.

```
Task(
  subagent_type="oh-my-claudecode:critic",
  model="opus",
  team_name="prd-policy-review",
  name="devils-advocate",
  run_in_background=true,
  prompt="{worker preamble + DA prompt}"
)
```

DA prompt core:
```
== DA MISSION ==
Synthesize all analyst reports:
1. Merge duplicates: consolidate same issue reported from different perspectives
2. Severity calibration: adjust over/under-rated severities (evidence required)
3. Dev-level downgrade: identify items that don't need PM decision (devs can decide)
4. Gap discovery: find policy conflicts/ambiguities analysts missed
5. PRD internal contradictions: find self-contradictions within the PRD itself
6. TOP 10 PM decisions: rank the most urgent PM decision items

== CHALLENGE CRITERIA ==
- Filter: "Is this really a planning-level decision, or can devs just decide?"
- REJECT issues without evidence
- When upgrading or downgrading severity, MUST state the reason

== REPORT FORMAT ==
1. Statistics summary (original count, post-merge count, upgrades/downgrades, dev-level downgrades, new discoveries)
2. TOP 10 PM decisions (ranked with rationale)
3. PRD internal contradictions list
4. Dev-level downgrade list
```

## Phase 4: Monitoring

Lead receives messages automatically from teammates.

- Analyst completion report received → record content
- All analysts complete → verify DA task unblocked → spawn DA
- DA completion report received → proceed to Phase 5

MUST: Periodically check `TaskList`. If any task stays `in_progress` for 5+ minutes without messages, send status check via `SendMessage`.

## Phase 5: Report Synthesis

After DA report received, lead synthesizes the final .md report via `Write`.

MUST: Write report to **PRD file's directory** as `prd-policy-review-report.md`.
MUST: Report content in **{REPORT_LANGUAGE}** (detected in Phase 1.0).

### Report Structure

```markdown
# {REPORT_TITLE: "PRD Policy Analysis Final Report"}

**Target**: {PRD title}
**Analysis Date**: {date}
**Method**: {N}-agent multi-perspective team ({N-1} perspective analysts + Devil's Advocate)
**Reference Docs**: ontology-docs MCP ({file count} files referenced)

---

## TOC
1. Analysis Overview
2. TOP 10 PM Decisions Required
3. PRD Internal Contradictions
4~{N+3}. Per-Perspective Analysis Results
{N+4}. Devil's Advocate Verification
{N+5}. Dev-Level Downgrades (No PM Decision Needed)
{N+6}. Recommendations
```

MUST: Write all section headers and content in {REPORT_LANGUAGE}.
If Korean: use the following header mappings: "Analysis Overview" → "분석 개요", "TOP 10 PM Decisions Required" → "PM 필수 의사결정 TOP 10", "PRD Internal Contradictions" → "PRD 내부 자기 모순", "Per-Perspective Analysis Results" → "각 관점별 분석 결과", "Devil's Advocate Verification" → "Devil's Advocate 검증 결과", "Dev-Level Downgrades" → "개발 레벨 다운그레이드", "Recommendations" → "권고 사항".

### Report Rules

- TOP 10 items MUST include `Decision needed` as checklist (`- [ ]`)
- All issues MUST cite ontology-docs filename and section as evidence
- Dev-level downgrade section MUST explain why PM decision is unnecessary

## Phase 6: Team Teardown

```
1. SendMessage(type="shutdown_request") to each worker
2. Await shutdown_response(approve=true)
3. Call TeamDelete
4. Report file path to user
```

## Verification Checklist

MUST verify before outputting report:

- [ ] All analyst tasks in `completed` status
- [ ] DA task in `completed` status
- [ ] Every TOP 10 item has `Decision needed` checklist
- [ ] Every issue has ontology-docs citation evidence
- [ ] PRD contradictions section exists (state "None found" if 0)
- [ ] Statistics summary numbers match actual issue counts
- [ ] Report file successfully written

If any item fails, return to the relevant phase and fix. On report write failure, error: `"Report generation failed: {reason}. Analysis data preserved."`.
