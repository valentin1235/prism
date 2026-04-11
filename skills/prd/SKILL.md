---
name: prd
description: PRD policy conflict analysis — takes a prd.md file as input, runs multi-perspective analysis against codebase policy documents (ontology), and generates a PM-friendly report. Use this skill for "prd analysis", "PRD policy review", "review PRD", "check PRD conflicts", "PRD policy check", or any request mentioning a PRD/spec document with policy/conflict/ambiguity/review. Always use this skill when a user mentions a PRD or spec and asks about policy conflicts, ambiguities, or review.
version: 1.0.0
user-invocable: true
allowed-tools: Skill, Task, Read, Write, Bash, Glob, Grep, AskUserQuestion, ToolSearch
---

# PRD Policy Analysis (Wrapper for analyze)

Takes a PRD file as input, cross-references it against codebase policy documents (ontology) to find policy conflicts and ambiguities. Internally delegates the full multi-perspective analysis to `prism:analyze`, then post-processes the results into a PM-readable format.

All PRD skill assets must remain self-contained under this directory:
- `skills/prd/SKILL.md`
- `skills/prd/prompts/post-processor.md`
- `skills/prd/templates/report.md`
- `skills/prd/evals/`

Do not require `~/.codex` copies or repo-external prompt/template paths when invoking this skill from Claude against the repo source.

## Phase 0: Input

### Step 0.1: Get PRD File Path

Extract the PRD file path from `$ARGUMENTS`.

- Path provided → verify file exists via `Read`
- No path → `AskUserQuestion` (header: "PRD File", question: "Please provide the path to the PRD file to analyze.")
- File not found → ERROR: "PRD file not found: {path}"

### Step 0.2: Generate Session ID

```bash
uuidgen | tr '[:upper:]' '[:lower:]' | cut -c1-8
```

Generate ONCE, reuse throughout. Create state directories for both prd and analyze (shared session ID):

```bash
mkdir -p ~/.prism/state/prd-{short-id}
mkdir -p ~/.prism/state/analyze-{short-id}
```

### Step 0.3: Language Detection

1. If CLAUDE.md contains a `Language` directive → use that language
2. Otherwise → detect from user's input language in this session
3. Store as `{REPORT_LANGUAGE}`

### Phase 0 Exit Gate

- [ ] PRD file path confirmed and file existence verified
- [ ] `{short-id}` generated and `~/.prism/state/prd-{short-id}/` directory created
- [ ] `{REPORT_LANGUAGE}` determined

→ **NEXT: Phase 1 — Create config and invoke analyze**

---

## Phase 1: Config & Analyze Invocation

### Step 1.1: Read PRD

Read the full PRD file via `Read`. Identify the title, functional requirements (FR), and non-functional requirements (NFR).

If related files (handoff, constraints, etc.) exist in the same directory, read them as well.

### Step 1.2: Create Analyze Config

Write the following JSON to `~/.prism/state/prd-{short-id}/analyze-config.json`:

```json
{
  "topic": "PRD policy conflict analysis: {PRD title} — multi-perspective analysis of whether this PRD conflicts with or has ambiguities against existing codebase policies",
  "input_context": "{PRD file absolute path}",
  "seed_hints": "First, Read the PRD file at {PRD file absolute path}. Extract policy domains from a PM (product manager) perspective. Focus on business policy conflicts, rule contradictions, undefined edge cases, and ambiguous requirements — NOT engineering implementation details. Classify each functional requirement as either conflicting with existing policy documents or covering a new area not addressed by existing policies. Generated perspectives should focus on policy/business domains, not engineering/architecture domains.",
  "session_id": "{short-id}"
}
```

> Determine the absolute path of the directory containing this SKILL.md via `Glob("**/skills/prd/SKILL.md")`. Extract the parent directory and store it as `{SKILL_DIR}` for use in Step 2.1. Do not hardcode `~/prism` or any `~/.codex` path.

### Step 1.3: Invoke Analyze

```
Skill(skill="prism:analyze", args="--config ~/.prism/state/prd-{short-id}/analyze-config.json")
```

Wait for analyze to complete. If analyze fails or the user cancels mid-execution → ERROR: "Analyze skill failed or was cancelled. Check ~/.prism/state/ for partial results." and terminate.

Analyze internally handles:
- Seed analyst investigation of PRD and policy domains
- Multi-perspective generation and user approval
- Per-perspective analyst spawning (policy conflict analysis)
- Socratic verification of findings
- Report generation

### Step 1.4: Locate Analyze Output

The analyze state directory is already known: `~/.prism/state/analyze-{short-id}` (shared session ID).

Verify the following files exist:
- `~/.prism/state/analyze-{short-id}/analyst-findings.md` — verified analysis results
- `~/.prism/state/analyze-{short-id}/verification-log.json` — Socratic verification scores (may not exist — this is tolerated because the post-processor has a 3-tier fallback for confidence scores)

Store `~/.prism/state/analyze-{short-id}` as `{ANALYZE_STATE_DIR}`.

### Phase 1 Exit Gate

- [ ] `analyze-config.json` written
- [ ] `prism:analyze` skill invocation completed
- [ ] `{ANALYZE_STATE_DIR}` identified and `analyst-findings.md` exists

→ **NEXT: Phase 2 — Post-processing (PM report generation)**

---

## Phase 2: Post-Processing (PM Report Generation)

The output from analyze is a technical analysis report. A post-processor agent transforms it into a format that PMs can directly use.

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
- `{ANALYZE_STATE_DIR}` → analyze result directory path identified in Step 1.4
- `{PRD_FILE_PATH}` → PRD file absolute path
- `{PRD_STATE_DIR}` → `~/.prism/state/prd-{short-id}`
- `{REPORT_LANGUAGE}` → language determined in Phase 0.3
- `{SHORT_ID}` → session ID
- `{REPORT_TEMPLATE_PATH}` → `{SKILL_DIR}/templates/report.md` (absolute path, from Step 1.2)

### Step 2.2: Verify Output

After post-processor agent completes, verify report file exists:

```
~/.prism/state/prd-{short-id}/prd-policy-review-report.md
```

Require the post-processor handoff result to return this exact path:

```
~/.prism/state/prd-{short-id}/prd-policy-review-report.md
```

Return the report file path: `{PRD_STATE_DIR}/prd-policy-review-report.md`

If missing → ERROR: "Post-processor agent failed to generate report."

### Phase 2 Exit Gate

- [ ] Post-processor agent completed
- [ ] `prd-policy-review-report.md` exists
- [ ] Report contains "PM Decision Checklist" section (verify via `Grep`)

→ **NEXT: Phase 3 — Deliver report**

---

## Phase 3: Output

### Step 3.1: Copy Report to PRD Directory

Save a copy of the report to the PRD file's directory:

```bash
cp ~/.prism/state/prd-{short-id}/prd-policy-review-report.md {PRD_DIR}/prd-policy-review-report.md
```

### Step 3.2: Report to User

Inform the user of the results:

```
PRD policy analysis complete.

Report location:
- {PRD_DIR}/prd-policy-review-report.md
- ~/.prism/state/prd-{short-id}/prd-policy-review-report.md

Analyze raw results: {ANALYZE_STATE_DIR}/
```
