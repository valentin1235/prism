---
name: prd-v3
description: PRD policy conflict analysis — takes a prd.md file as input, runs multi-perspective analysis against codebase policy documents (ontology), and generates a PM-friendly report. Use this skill for "prd analysis", "PRD policy review", "review PRD", "check PRD conflicts", "PRD policy check", or any request mentioning a PRD/spec document with policy/conflict/ambiguity/review. Always use this skill when a user mentions a PRD or spec and asks about policy conflicts, ambiguities, or review.
version: 1.0.0
user-invocable: true
allowed-tools: Skill, Task, Read, Write, Bash, Glob, Grep, AskUserQuestion, ToolSearch
---

# PRD Policy Analysis (Wrapper for analyze)

Takes a PRD file as input, cross-references it against codebase policy documents (ontology) to find policy conflicts and ambiguities. Internally delegates the full multi-perspective analysis to `prism:analyze`, then post-processes the results into a PM-readable format.

## Prerequisite

> Read and execute `../shared-v3/prerequisite-gate.md`. Set `{PROCEED_TO}` = "Phase 0".

---

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

Generate ONCE, reuse throughout. Create state directory:

```bash
mkdir -p ~/.prism/state/prd-{short-id}
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
  "report_template": "{absolute path to this skill}/templates/report.md",
  "seed_hints": "First, Read the PRD file at {PRD file absolute path}. Extract policy domains from a PM (product manager) perspective. Focus on business policy conflicts, rule contradictions, undefined edge cases, and ambiguous requirements — NOT engineering implementation details. Classify each functional requirement as either conflicting with existing policy documents or covering a new area not addressed by existing policies.",
  "ontology_mode": "required"
}
```

> `{absolute path to this skill}` is the absolute path of the directory containing this SKILL.md. Determine via `Bash` by extracting the directory from this SKILL.md's file path.

### Step 1.3: Snapshot Before Analyze

Take a snapshot of existing analyze directories **before** invoking analyze:

```bash
ls -d ~/.prism/state/analyze-* 2>/dev/null > ~/.prism/state/prd-{short-id}/analyze-dirs-before.txt || touch ~/.prism/state/prd-{short-id}/analyze-dirs-before.txt
```

### Step 1.4: Invoke Analyze

```
Skill(skill="prism:analyze", args="--config ~/.prism/state/prd-{short-id}/analyze-config.json")
```

Wait for analyze to complete. Analyze internally handles:
- Seed analyst investigation of PRD and policy domains
- Multi-perspective generation and user approval
- Per-perspective analyst spawning (policy conflict analysis)
- Socratic verification of findings
- Report generation

### Step 1.5: Locate Analyze Output

After analyze completes, compare directories before and after to find the newly created analyze directory:

```bash
comm -13 <(sort ~/.prism/state/prd-{short-id}/analyze-dirs-before.txt) <(ls -d ~/.prism/state/analyze-* 2>/dev/null | sort)
```

There should be exactly 1 new directory. If 0 → ERROR: "analyze did not create a state directory." If 2+ → select the most recent one.

Verify the following files exist in that directory:
- `analyst-findings.md` — verified analysis results
- `verification-log.json` — Socratic verification scores (may not exist)

Store this path as `{ANALYZE_STATE_DIR}`.

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
  subagent_type="oh-my-claudecode:analyst",
  model="opus",
  prompt="{post-processor prompt with placeholders replaced}"
)
```

**CRITICAL: Do NOT add `run_in_background=true`.** Must wait for post-processing results.

Placeholder replacements:
- `{ANALYZE_STATE_DIR}` → analyze result directory path identified in Step 1.5
- `{PRD_FILE_PATH}` → PRD file absolute path
- `{PRD_STATE_DIR}` → `~/.prism/state/prd-{short-id}`
- `{REPORT_LANGUAGE}` → language determined in Phase 0.3
- `{SHORT_ID}` → session ID

### Step 2.2: Verify Output

After post-processor agent completes, verify report file exists:

```
~/.prism/state/prd-{short-id}/prd-policy-review-report.md
```

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
