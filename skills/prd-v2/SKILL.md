---
name: prd-v2
description: Multi-perspective PRD policy conflict analysis with ontology-scoped analysis
version: 1.0.0
user-invocable: true
allowed-tools: Task, SendMessage, TeamCreate, TeamDelete, TaskCreate, TaskUpdate, TaskList, TaskGet, AskUserQuestion, Read, Glob, Grep, Bash, Write, WebFetch, WebSearch, mcp__ontology-docs__directory_tree, mcp__ontology-docs__list_directory, mcp__ontology-docs__read_file, mcp__ontology-docs__read_text_file, mcp__ontology-docs__read_multiple_files, mcp__ontology-docs__search_files
---

# Table of Contents

- [Prerequisite](#prerequisite)
- [Phase 0: Input](#phase-0-input)
- [Phase 1: PRD Analysis & Perspective Generation](#phase-1-prd-analysis--perspective-generation)
- [Phase 2: Team Setup & Task Assignment](#phase-2-team-setup--task-assignment)
- [Phase 3: Spawn & Execute Analysts](#phase-3-spawn--execute-analysts)
- [Phase 4: Monitoring](#phase-4-monitoring)
- [Phase 5: Report Synthesis](#phase-5-report-synthesis)
- [Phase 6: Team Teardown](#phase-6-team-teardown)

Analyst rules are defined inline. DA prompt is in `prompts/devil-advocate.md`. Read shared modules at execution time — do NOT preload into memory.

## Prerequisite

→ Read and execute `../shared/prerequisite-gate.md`. Set `{PROCEED_TO}` = "Phase 0".

---

## Phase 0: Input

### MUST: Minimum Required

| Item | Required | Method |
|------|----------|--------|
| PRD file path | YES | Argument or `AskUserQuestion` |
| Report output path | NO | Defaults to `prd-policy-review-report.md` in PRD file's directory |

```
/prd-v2 @path/to/prd.md
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

#### Perspective Quality Gate

→ Apply `../shared/perspective-quality-gate.md` with `{DOMAIN}` = "prd", `{EVIDENCE_SOURCE}` = "PRD content and ontology docs".

### 1.3 Ontology Scope Mapping

→ Read and execute `../shared/ontology-scope-mapping.md` with:
- `{AVAILABILITY_MODE}` = `required`
- `{CALLER_CONTEXT}` = `"PRD analysis"`

PRD analysis requires policy document references — MCP unavailability stops execution.

**Note:** This skill requires the `ontology-docs` MCP server to be configured. If not set up, run the `podo-plugin:install-docs` skill or see the plugin README for configuration instructions.

#### Phase 1.3 Exit Gate

Additional check beyond shared module exit gate:
- [ ] Pool Catalog is non-empty (required for PRD analysis)

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
- **Ontology scope mapping** (which specific docs to prioritize)
- Analysis rules (see "Analyst Behavior Rules" below)

### 2.3 TaskCreate — Devil's Advocate

Create DA task with `addBlockedBy` depending on ALL analyst tasks.

### 2.4 Pre-assign Owners

MUST: Pre-assign owners via `TaskUpdate(owner="{worker-name}")` before spawning. Prevents race conditions.

### Phase 2 Exit Gate

MUST NOT proceed until ALL verified:

- [ ] Team created
- [ ] All analyst tasks created with ontology scope in description
- [ ] DA task created with `addBlockedBy` on all analyst tasks
- [ ] All task owners pre-assigned

If ANY missing → fix before spawning. Error: "Cannot spawn: {item} not configured."

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

All analysts use `analyst` (opus) — PRD policy conflict analysis requires deep cross-referencing between PRD sections and ontology policy documents across every perspective. Sonnet-tier agents are insufficient for this level of reasoning.

### 3.2 Analyst Prompt Structure

→ Apply worker preamble from `../shared/worker-preamble.md` with:
- `{TEAM_NAME}` = `"prd-policy-review"`
- `{WORKER_NAME}` = `"{perspective-slug}-analyst"`
- `{WORK_ACTION}` = `"Use ontology-docs MCP tools to explore and read the ontology pool documents (see ONTOLOGY SCOPE below), then cross-reference PRD sections against docs to find policy conflicts/ambiguities"`

Then include the ontology scope block:

```
== ONTOLOGY SCOPE ==
{ONTOLOGY_SCOPE}
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

Read `prompts/devil-advocate.md` (relative to this SKILL.md) + `shared/da-evaluation-protocol.md`.

```
Task(
  subagent_type="oh-my-claudecode:critic",
  model="opus",
  team_name="prd-policy-review",
  name="devils-advocate",
  run_in_background=true,
  prompt="{worker preamble + DA prompt from prompts/devil-advocate.md}"
)
```

Placeholder replacements:
- `{ALL_ANALYST_FINDINGS}` → compiled findings from all analysts
- `{PRD_CONTEXT}` → PRD content and sibling files from Phase 1
- `{ONTOLOGY_SCOPE}` → full-scope ontology reference from Phase 1.3 (all docs, not perspective-filtered)

DA receives analyst findings for **evaluation** — NOT synthesis tasks. DA is a logic auditor only.

### 3.4 DA Challenge-Response Loop

Orchestrator-mediated loop, max 2 rounds:

1. **Round 1**: DA evaluates analyst findings → produces verdict table with BLOCKING/MAJOR/MINOR issues
2. **If NOT SUFFICIENT**: Orchestrator forwards DA challenges to relevant analysts (via `SendMessage`)
3. **Round 2**: Analysts respond → orchestrator sends responses to DA → DA re-evaluates → updated verdict
4. **Termination**:
   - All BLOCKING resolved → **SUFFICIENT** → proceed
   - BLOCKING persists after 2 rounds → **NEEDS TRIBUNAL** → record as open question for Phase 5
   - MAJOR unresolved after 2 rounds → record as acknowledged limitation

### 3.5 DA Exit Gate

MUST NOT proceed until:

- [ ] DA verdict is SUFFICIENT (or NEEDS TRIBUNAL items recorded as open questions)
- [ ] Challenge-response loop completed (max 2 rounds)
- [ ] DA evaluation report received with: fallacy check results, aggregate verdict, ontology scope critique, tribunal trigger assessment

### Phase 3 Exit Gate

MUST NOT proceed until ALL verified:

- [ ] All analyst tasks in `completed` status
- [ ] Every analyst cited ontology-docs filename and section as evidence
- [ ] DA evaluation complete (Step 3.5 exit gate passed)

If ANY analyst incomplete → check via `TaskList`, send status query. Error: "Cannot synthesize: {analyst} not completed."

## Phase 4: Monitoring

Lead receives messages automatically from teammates.

- Analyst completion report received → record content
- All analysts complete → verify DA task unblocked → spawn DA (Phase 3.3)
- DA challenge-response loop mediated by lead (Phase 3.4) → forward challenges to analysts, collect responses
- DA evaluation complete (Phase 3.5 exit gate) → proceed to Phase 5

### Clarity Enforcement

→ Apply `../shared/clarity-enforcement.md` with `{EVIDENCE_FORMAT}` = `"filename:section"`.

MUST: Periodically check `TaskList`. If any task stays `in_progress` for 5+ minutes without messages, send status check via `SendMessage`.

### Phase 4 Exit Gate

MUST NOT proceed until ALL verified:

- [ ] All analyst reports received
- [ ] DA evaluation complete (Phase 3.5 exit gate passed)
- [ ] No analyst stuck in `in_progress` for 5+ minutes without response

If ANY missing → investigate via `TaskList` and `SendMessage`. Error: "Cannot generate report: {item} pending."

## Phase 5: Report Synthesis

After DA evaluation complete, lead performs synthesis and writes the final report.

### Step 5.1: Lead Synthesis

Lead uses DA-verified analyst findings to perform (tasks previously in DA scope, now orchestrator responsibility):

1. **Merge duplicates**: Consolidate same issue reported from different perspectives
2. **Severity calibration**: Adjust over/under-rated severities based on DA verdict (evidence required for changes)
3. **Dev-level downgrade**: Identify items that don't need PM decision (devs can decide)
4. **Gap discovery**: Compare all analyst reports to find policy conflicts/ambiguities analysts missed
5. **PRD internal contradictions**: Find self-contradictions within the PRD itself
6. **TOP 10 PM decisions**: Rank the most urgent PM decision items based on DA-verified findings
7. **Ontology scope audit**: Review DA's ontology scope critique for missed docs or under-explored areas

### Step 5.2: Write Report

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
2. Ontology Scope Mapping
3. TOP 10 PM Decisions Required
4. PRD Internal Contradictions
5~{N+4}. Per-Perspective Analysis Results
{N+5}. Devil's Advocate Evaluation
{N+6}. Dev-Level Downgrades (No PM Decision Needed)
{N+7}. Recommendations
```

MUST: Write all section headers and content in {REPORT_LANGUAGE}.
If Korean: use the following header mappings: "Analysis Overview" → "분석 개요", "Ontology Scope Mapping" → "온톨로지 스코프 매핑", "TOP 10 PM Decisions Required" → "PM 필수 의사결정 TOP 10", "PRD Internal Contradictions" → "PRD 내부 자기 모순", "Per-Perspective Analysis Results" → "각 관점별 분석 결과", "Devil's Advocate Evaluation" → "Devil's Advocate 평가 결과", "Dev-Level Downgrades" → "개발 레벨 다운그레이드", "Recommendations" → "권고 사항".

### Ontology Scope Mapping Section

Include in report:
```markdown
## Ontology Scope Mapping

### Ontology Catalog
| # | Path | Domain | Summary |
|---|------|--------|---------|

### DA Ontology Audit
{DA ontology scope audit results — under-explored documents, missed sections}
```

### Report Rules

- TOP 10 items MUST include `Decision needed` as checklist (`- [ ]`)
- All issues MUST cite ontology-docs filename and section as evidence
- Dev-level downgrade section MUST explain why PM decision is unnecessary

## Phase 6: Team Teardown

→ Execute `../shared/team-teardown.md`. Then report file path to user.

## Verification Checklist

MUST verify before outputting report:

- [ ] All analyst tasks in `completed` status
- [ ] DA task in `completed` status with SUFFICIENT verdict (or NEEDS TRIBUNAL items recorded)
- [ ] DA challenge-response loop completed (max 2 rounds)
- [ ] Every TOP 10 item has `Decision needed` checklist
- [ ] Every issue has ontology-docs citation evidence
- [ ] PRD contradictions section exists (state "None found" if 0)
- [ ] Statistics summary numbers match actual issue counts
- [ ] Ontology scope mapping section included in report
- [ ] Report file successfully written

If any item fails, return to the relevant phase and fix. On report write failure, error: `"Report generation failed: {reason}. Analysis data preserved."`.
