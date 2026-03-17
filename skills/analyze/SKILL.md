---
name: analyze
description: Runs multi-perspective agent team analysis with ontology-scoped investigation and MCP-based Socratic verification. General-purpose analysis engine — any topic can be seeded for multi-perspective analysis against ontology documents. Supports config-based customization for wrapper skills (e.g., PRD analysis).
version: 5.0.1
user-invocable: true
allowed-tools: Task, SendMessage, TeamCreate, TeamDelete, TaskCreate, TaskUpdate, TaskList, TaskGet, TaskOutput, Read, Glob, Grep, Bash, Write, WebFetch, WebSearch, ToolSearch, ListMcpResourcesTool, mcp__prism-mcp__prism_docs_roots, mcp__prism-mcp__prism_docs_list, mcp__prism-mcp__prism_docs_read, mcp__prism-mcp__prism_docs_search, mcp__prism-mcp__prism_interview, mcp__prism-mcp__prism_da_review
---

# Multi-Perspective Analysis

General-purpose analysis engine. Any topic is seeded, researched, and analyzed from dynamically generated perspectives with Socratic verification.

Prompt templates and report template are in subdirectories relative to this file. Read them at spawn time — do NOT preload into memory.

Later phases (Phase 2+) are in `docs/later-phases.md`. Read that file ONLY when entering Phase 2.

## Config-Based Customization

Wrapper skills (e.g., `/prd`) can customize analyze behavior by providing a config file. The config path is passed via `$ARGUMENTS` as `--config <path>`.

### Config Schema

```json
{
  "topic": "What to analyze (overrides description if provided)",
  "input_context": "Path to input file (e.g., PRD file path)",
  "report_template": "Path to custom report template (overrides default)",
  "seed_hints": "Additional guidance for seed analyst (e.g., 'Focus on policy domain extraction')",
  "ontology_mode": "required|optional (default: optional)",
  "session_id": "Pre-generated session ID (optional — if provided, reuse instead of generating new one)"
}
```

If no config is provided, analyze runs with defaults (topic from user input, default report template, optional ontology).

## Artifact Persistence

Persist phase outputs to `~/.prism/state/analyze-{short-id}/` (created in Phase 0, Step 0.2). On deeper investigation re-entry, agents MUST `Read` artifact files — do NOT rely solely on prompt context.

| File | Written | Read By |
|------|---------|---------|
| `config.json` | Orchestrator (Phase 0, if config provided) | Orchestrator (all phases) |
| `seed-analysis.json` | Seed Analyst (Phase 0.5, with DA self-loop) | Perspective Generator (Phase 0.55), Orchestrator |
| `perspectives.json` | Perspective Generator (Phase 0.55), updated by Orchestrator (Phase 0.6) | Orchestrator (Phase 0.6, 0.8, 1, 3) |
| `context.json` | Orchestrator (Phase 0.8) | Orchestrator (Phase 1 `{CONTEXT}` injection, Phase 3 re-entry) |
| `~/.prism/state/analyze-{short-id}/perspectives/{perspective-id}/findings.json` | Analyst (Phase 1 — Finding Session) | Analyst (Phase 2 — Verification Session), MCP prism_interview |
| `verified-findings-{perspective-id}.md` | Orchestrator (Phase 2 Stage B) | Phase 3 synthesis |
| `analyst-findings.md` | Orchestrator (Phase 2 exit) | Phase 3 synthesis |
| `verification-log.json` | Orchestrator (Phase 2 Step 2B.6) | Phase 3 synthesis (Socratic Verification Summary section) |
| `prior-iterations.md` | Each re-entry (append) | All agents (cumulative) |
| `ontology-scope.json` | Orchestrator (Phase 0.3) | Analysts (via `{ONTOLOGY_SCOPE}` injection) |
| `perspective_injection.json` | Wrapper skill (optional, before analyze invocation) | Merge script (Step 0.55.5) |

## Prerequisite: Agent Team Mode (HARD GATE)

> Read and execute `protocols/prerequisite-gate.md`. Set `{PROCEED_TO}` = "Phase 0".

## Team Size

2 min analysts, no hard max (typically 3-5; complex cases may need more). Verification runs via MCP `prism_interview` tool, not sidecar agents.

---

## Phase 0: Problem Intake

Orchestrator handles intake directly — NOT delegated.

### Step 0.1: Parse Arguments & Config

Check if `$ARGUMENTS` contains `--config <path>`:
- **Config provided**: Read the config file. Copy it to `~/.prism/state/analyze-{short-id}/config.json` (after state dir creation in Step 0.2). Use `config.topic` as description if present, otherwise fall back to remaining arguments.
- **No config**: Use `$ARGUMENTS` as the description directly. If empty, ask via `AskUserQuestion` (header: "Analysis"): "Please describe what you'd like to analyze."

Store the resolved description and config values for use in subsequent phases.

### Step 0.2: Generate Session ID and State Directory

If `config.session_id` is provided, use it as `{short-id}`. Otherwise, generate `{short-id}`: `Bash(uuidgen | tr '[:upper:]' '[:lower:]' | cut -c1-8)`. Either way, generate/resolve ONCE and reuse throughout all phases.

> **Naming note:** `{short-id}` in path templates (e.g., `analyze-{short-id}/`) and `{SHORT_ID}` in prompt placeholders refer to the same value. Use `{short-id}` when constructing paths, `{SHORT_ID}` when replacing placeholders in agent prompts.

Create state directory (if it doesn't already exist): `Bash(mkdir -p ~/.prism/state/analyze-{short-id})`

If config was provided in Step 0.1, copy it to state directory now.

### Phase 0 Exit Gate

MUST NOT proceed until ALL checked:

- [ ] Description collected → ERROR: "Phase 0 blocked: description is empty. Re-run Step 0.1."
- [ ] `{short-id}` generated and state directory created → ERROR: "Phase 0 blocked: run `uuidgen | tr '[:upper:]' '[:lower:]' | cut -c1-8` and `mkdir -p ~/.prism/state/analyze-{short-id}`"
- [ ] Config copied to state directory (if provided) → ERROR: "Copy config to ~/.prism/state/analyze-{short-id}/config.json"

→ **NEXT ACTION: Proceed to Phase 0.3 — Ontology Scope Mapping.**

---

## Phase 0.3: Ontology Scope Mapping

> Read and execute `protocols/ontology-scope-mapping.md` with:
- `{AVAILABILITY_MODE}` = config's `ontology_mode` if present, otherwise `optional`
- `{CALLER_CONTEXT}` = `"analysis"`
- `{STATE_DIR}` = `~/.prism/state/analyze-{short-id}`

If `ONTOLOGY_AVAILABLE=false` → all analysts get `{ONTOLOGY_SCOPE}` = "N/A — ontology scope not available. Analyze using available evidence only."

→ **NEXT ACTION: Proceed to Phase 0.5 Step 0.5.1 — Create team.**

---

## Phase 0.5: Team Creation & Seed Analysis

### Step 0.5.1: Create Team

```
TeamCreate(team_name: "analyze-{short-id}", description: "Analysis: {summary}")
```

> Replace `{summary}` with a short (≤10 word) summary derived from the description (Phase 0.1). `context.json` is not yet available at this point.

### Step 0.5.2: Spawn Seed Analyst

Read `prompts/seed-analyst.md` (relative to this SKILL.md).

Create seed-analyst task via `TaskCreate`, pre-assign owner via `TaskUpdate(owner="seed-analyst")`, then spawn:

```
Task(
  subagent_type="prism:finder",
  name="seed-analyst",
  team_name="analyze-{short-id}",
  model="opus",
  run_in_background=true,
  prompt="{worker preamble + seed-analyst prompt with placeholders replaced}"
)
```

> Apply worker preamble from `protocols/worker-preamble.md` with:
- `{TEAM_NAME}` = `"analyze-{short-id}"`
- `{WORKER_NAME}` = `"seed-analyst"`
- `{WORK_ACTION}` = `"Actively investigate using available tools (Grep, Read, Bash, MCP). Map related code areas and systems. Write discoveries to seed-analysis.json. Report via SendMessage."`

Placeholder replacements in seed-analyst prompt:
- `{DESCRIPTION}` → Phase 0 description
- `{SHORT_ID}` → Phase 0 short-id
- `{SEED_HINTS}` → config's `seed_hints` value if present, otherwise empty string

### Step 0.5.3: Receive Seed Analyst Results

Wait for seed-analyst to send results via `SendMessage`. The message contains a JSON object with:
- `topic`: original description
- `da_passed`: boolean indicating whether DA review passed (no CRITICAL/MAJOR findings)
- `research`: summary, findings (with area, description, source, and tool_used), key_areas, files_examined, mcp_queries

The seed analyst internally runs a DA review self-loop (up to 3 rounds) using `prism_da_review` before reporting. Findings are incrementally updated — existing findings are preserved, new findings appended. The `da_passed` flag reflects the final DA review outcome. **DA critique details are NOT forwarded to the perspective generator** — only the enriched seed analysis is used downstream.

The seed analyst also writes this JSON to `~/.prism/state/analyze-{short-id}/seed-analysis.json`.

### Step 0.5.4: Shutdown Seed Analyst

After receiving results: `SendMessage(type: "shutdown_request", recipient: "seed-analyst", content: "Seed analysis complete.")`.

### Step 0.5.5: Drain Background Task Output

**CRITICAL ([#27431](https://github.com/anthropics/claude-code/issues/27431)):** Before any MCP tool call, drain all completed background tasks: `TaskList` → `TaskOutput` for each completed task.

### Phase 0.5 Exit Gate

MUST NOT proceed until:

- [ ] Team created → ERROR: "Phase 0.5 blocked: TeamCreate failed. Check team name format 'analyze-{short-id}'."
- [ ] Seed-analyst results received → ERROR: "Phase 0.5 blocked: no SendMessage from seed-analyst. Run TaskList to check task status."
- [ ] `seed-analysis.json` written with `da_passed` field → ERROR: "Phase 0.5 blocked: file missing at ~/.prism/state/analyze-{short-id}/seed-analysis.json or missing da_passed field"
- [ ] Seed-analyst shut down → ERROR: "Send shutdown_request to seed-analyst via SendMessage."
- [ ] All background task outputs drained → ERROR: "Run TaskList → TaskOutput for each completed task ([#27431])."

→ **NEXT ACTION: Proceed to Phase 0.55 — Perspective Generation.**

---

## Phase 0.55: Perspective Generation

### Step 0.55.1: Spawn Perspective Generator

Read `prompts/perspective-generator.md` (relative to this SKILL.md).

Create task via `TaskCreate`, pre-assign owner via `TaskUpdate(owner="perspective-generator")`, then spawn:

```
Task(
  subagent_type="prism:finder",
  name="perspective-generator",
  team_name="analyze-{short-id}",
  model="opus",
  run_in_background=true,
  prompt="{worker preamble + perspective-generator prompt with placeholders replaced}"
)
```

> Apply worker preamble from `protocols/worker-preamble.md` with:
- `{TEAM_NAME}` = `"analyze-{short-id}"`
- `{WORKER_NAME}` = `"perspective-generator"`
- `{WORK_ACTION}` = `"Read seed-analysis.json and analyst-prompt-structure.md. Generate perspective candidates with tailored analyst prompts. Write perspectives.json. Report via SendMessage."`

Placeholder replacements:
- `{SHORT_ID}` → Phase 0 short-id
- `{DESCRIPTION}` → Phase 0 description

### Step 0.55.2: Receive Perspective Generator Results

Wait for perspective-generator to send results via `SendMessage`. The message contains a JSON object with:
- `perspectives`: array of perspective candidates (id, name, scope, key_questions, model, agent_type, prompt, rationale)
- `quality_gate`: which checks were verified
- `selection_summary`: reasoning for the selection

The perspective generator also writes this JSON to `~/.prism/state/analyze-{short-id}/perspectives.json`.

### Step 0.55.3: Shutdown Perspective Generator

After receiving results: `SendMessage(type: "shutdown_request", recipient: "perspective-generator", content: "Perspective generation complete.")`.

### Step 0.55.4: Drain Background Task Output

Same as Step 0.5.5 — drain all completed background task outputs via `TaskList` → `TaskOutput`.

### Step 0.55.5: Merge Injected Perspectives

After perspective generator completes and `perspectives.json` is written, run the merge script:

```bash
bash {SKILL_DIR}/scripts/merge-perspective-injection.sh ~/.prism/state/analyze-{short-id}
```

> Determine `{SKILL_DIR}` as the absolute path of the directory containing this SKILL.md via `Bash`.

This script is deterministic and safe:
- If `perspective_injection.json` does not exist → no-op (silent exit)
- If it exists → appends injected perspectives to `perspectives.json`'s `perspectives` array

### Phase 0.55 Exit Gate

MUST NOT proceed until:

- [ ] Perspective generator results received → ERROR: "Phase 0.55 blocked: no SendMessage from perspective-generator. Run TaskList."
- [ ] `perspectives.json` written → ERROR: "File missing at ~/.prism/state/analyze-{short-id}/perspectives.json"
- [ ] Merge script executed → ERROR: "Run Step 0.55.5: bash {SKILL_DIR}/scripts/merge-perspective-injection.sh ~/.prism/state/analyze-{short-id}"
- [ ] Perspective generator shut down → ERROR: "Send shutdown_request to perspective-generator."
- [ ] All background task outputs drained → ERROR: "Run TaskList → TaskOutput for each completed task."

→ **NEXT ACTION: Proceed to Phase 0.6 — Perspective Approval.**

---

## Phase 0.6: Perspective Approval

### Step 0.6.1: Present Perspectives

Read `~/.prism/state/analyze-{short-id}/perspectives.json` and present to user.

`AskUserQuestion` (header: "Perspectives", question: "I recommend these {N} perspectives for analysis. How to proceed?", options: "Proceed" / "Add perspective" / "Remove perspective" / "Modify perspective")

Include seed-analyst's research summary (from `seed-analysis.json`) for user context. Show each perspective's name, scope, model, and a brief summary of the generated prompt tasks.

### Step 0.6.2: Iterate Until Approved

Repeat until user selects "Proceed". Warn if <2 perspectives.

### Step 0.6.3: Update Perspectives

Update `~/.prism/state/analyze-{short-id}/perspectives.json` in-place — add approval metadata and apply any user modifications:

```json
{
  "perspectives": [...],
  "quality_gate": {...},
  "selection_summary": "...",
  "approved": true,
  "user_modifications": ["description of changes, if any"]
}
```

The `perspectives` array, `quality_gate`, and `selection_summary` fields are preserved from Phase 0.55. The orchestrator adds `approved` and `user_modifications` (empty array if no changes).

### Phase 0.6 Exit Gate

MUST NOT proceed until:

- [ ] User selected "Proceed" → ERROR: "Phase 0.6 blocked: user has not approved perspectives. Re-run Step 0.6.1."
- [ ] `perspectives.json` updated with `approved: true` → ERROR: "Phase 0.6 blocked: perspectives.json missing 'approved: true'. Run Step 0.6.3."

→ **NEXT ACTION: Proceed to Phase 0.8 — Write context file.**

---

## Phase 0.8: Context & State Files

### Step 0.8.1: Write Context File

Write `~/.prism/state/analyze-{short-id}/context.json`:

```json
{
  "summary": "Topic description and key context",
  "research_summary": {
    "key_findings": ["area1: description1", "area2: description2"],
    "files_examined": ["path1", "path2"],
    "key_areas": ["area1", "area2"]
  },
  "report_language": "detected from user's input language",
  "investigation_loops": 0,
  "config": "{config object if provided, null otherwise}"
}
```

### Phase 0.8 Exit Gate

MUST NOT proceed until ALL checked:

- [ ] `perspectives.json` updated with approved=true → ERROR: "Phase 0.8 blocked: perspectives.json missing 'approved: true'. Re-run Phase 0.6."
- [ ] `context.json` written with structured summary → ERROR: "Write context.json to ~/.prism/state/analyze-{short-id}/context.json per Step 0.8.1."
- [ ] Ontology scope mapping complete or explicitly skipped → ERROR: "Check for ontology-scope.json or set ONTOLOGY_AVAILABLE=false."

→ **NEXT ACTION: Proceed to Phase 1 — Spawn analysts.**

---

## Phase 1: Spawn Analysts (Finding Phase)

Team already exists from Phase 0.5. Spawn all analyst agents in parallel. Each analyst investigates and writes findings only — verification happens in separate sessions (Phase 2).

### Step 1.1: Spawn Analysts

Read `perspectives.json` to get each perspective's dynamically generated prompt.

**Prompt assembly order:** For each analyst:
1. Read the perspective's `prompt` object from `perspectives.json`
2. Read `prompts/finding-protocol.md`
3. Assemble the full prompt using the prompt structure:
   ```
   [worker preamble]

   {prompt.role}

   CONTEXT:
   {CONTEXT}

   ### Reference Documents
   {ONTOLOGY_SCOPE}

   {prompt.investigation_scope}

   TASKS:
   {prompt.tasks}

   OUTPUT:
   {prompt.output_format}

   [finding protocol]
   ```
4. Replace placeholders (`{CONTEXT}`, `{ONTOLOGY_SCOPE}`, `{SHORT_ID}`, `{perspective-id}`, `{KEY_QUESTIONS}`, `{ORIGINAL_INPUT}`)
5. Spawn via `Task(...)`

**Spawn pattern:**

```
Task(
  subagent_type="prism:finder",
  name="{perspective-id}-analyst",
  team_name="analyze-{short-id}",
  model="{model}",
  run_in_background=true,
  prompt="{assembled analyst prompt}"
)
```

> Apply worker preamble with `{WORK_ACTION}` = `"Investigate from your assigned perspective. Answer ALL key questions with evidence and code references. Write findings to findings.json. Report findings via SendMessage to team-lead. Do NOT run self-verification — that happens in a separate session."`

MUST replace `{CONTEXT}` with a text summary derived from `context.json`: format as `Summary: {summary}\nKey Findings: {research_summary.key_findings joined}\nFiles Examined: {research_summary.files_examined joined}\nKey Areas: {research_summary.key_areas joined}`.
MUST replace `{ONTOLOGY_SCOPE}` by reading `ontology-scope.json` and generating a text block per Phase B of ontology-scope-mapping.md (or "N/A" if not found).
MUST replace `{SHORT_ID}` with the session's `{short-id}`. Analysts construct their own session path: `analyze-{short-id}/perspectives/{perspective-id}`.
MUST replace `{KEY_QUESTIONS}` from `perspectives.json` for the matching perspective's `key_questions` array, formatted as a numbered list.
MUST replace `{perspective-id}` with the perspective's `id` field from `perspectives.json`. This value appears in findings paths and SendMessage output in finding-protocol.md.
MUST replace `{ORIGINAL_INPUT}` with `context.json`'s `summary` field. This is written into findings.json so the verification interviewer can evaluate relevance to the original topic.

`{model}` and `{agent_type}` come from each perspective's fields in `perspectives.json` — these were dynamically determined by the perspective generator.

### Phase 1 Exit Gate

MUST NOT proceed until:

- [ ] All analyst tasks created and owners pre-assigned → ERROR: "Phase 1 blocked: run TaskCreate + TaskUpdate(owner=...) for each perspective."
- [ ] All analysts spawned in parallel → ERROR: "Spawn all analysts via Task(..., run_in_background=true). Check TaskList for missing spawns."

→ **NEXT ACTION: Read `docs/later-phases.md` and proceed to Phase 2 — Collect Findings & Spawn Verification Sessions.**

---

## Gate Summary

```
Prerequisite → Phase 0 [intake, config, session ID]
→ Phase 0.3 [ontology]
→ Phase 0.5 [TeamCreate + seed-analyst (research findings → DA self-loop → seed-analysis.json with da_passed) + drain]
→ Phase 0.55 [perspective-generator (seed-analysis.json → perspectives.json with dynamic prompts) + drain]
→ Phase 0.6 [perspective approval (user reviews perspectives.json → update with approved)]
→ Phase 0.8 [context + state files]
→ Phase 1 [spawn analysts — finding only, using dynamic prompts]
→ Phase 2 [collect findings → shutdown → spawn verification sessions → collect verified findings] ← docs/later-phases.md
→ Phase 3 [report — uses config report_template if provided, otherwise default] ← docs/later-phases.md
→ Phase 4 [cleanup] ← docs/later-phases.md
```

Every gate specifies exact missing items. Fix before proceeding.
