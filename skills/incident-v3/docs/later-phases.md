# Later Phases — Phase 2 through Phase 4

Read this file when entering Phase 2. Do NOT preload.

---

## Phase 2: Socratic Verification Loop

This phase runs a per-analyst verification pipeline: Analyst → Socratic DA → Ambiguity Scorer. The orchestrator manages the loop.

### Step 2.1: Collect Analyst Findings

As each analyst completes (monitor via `TaskList`), collect their findings from `SendMessage`.

**Immediately persist** each analyst's findings to:
`.omc/state/incident-{short-id}/raw-findings-{analyst-id}.md`

Apply clarity enforcement (`../shared-v3/clarity-enforcement.md` with `{EVIDENCE_FORMAT}` = `"file:function:line"`) before proceeding. Max 2 rework cycles per analyst.

### Step 2.2: Spawn Sidecar DA (per analyst)

For each analyst that passes clarity enforcement, spawn a Socratic DA sidecar.

Read `prompts/devil-advocate.md` (relative to the main SKILL.md).

```
Task(
  subagent_type="oh-my-claudecode:critic",
  name="da-{analyst-id}",
  team_name="incident-analysis-{short-id}",
  model="opus",
  run_in_background=true,
  prompt="{DA prompt with placeholders replaced}"
)
```

> Apply worker preamble with:
- `{WORKER_NAME}` = `"da-{analyst-id}"`
- `{WORK_ACTION}` = `"Ask Socratic questions to reduce ambiguity in the analyst's findings. Do NOT reference ontology documents. Work only with what the analyst provides."`

Placeholder replacements:
- `{ANALYST_NAME}` → the paired analyst's name
- `{ANALYST_FINDINGS}` → the analyst's findings
- `{PRIOR_QA}` → empty string for Round 1
- `{ROUND_NUMBER}` → "1"
- `{INCIDENT_CONTEXT}` → Phase 0 details

**Spawn DAs in parallel** as analysts complete — do not wait for all analysts.

### Step 2.3: Socratic Q&A Loop

The orchestrator mediates the conversation between DA and analyst:

**Round 1:**
1. DA receives analyst findings, produces 2-4 questions targeting ambiguity
2. Orchestrator receives DA questions via `SendMessage`
3. Orchestrator forwards questions to the paired analyst via `SendMessage`
4. Analyst responds with clarifications
5. **Persist Q&A**: Write to `.omc/state/incident-{short-id}/da-qa-{analyst-id}-round-1.md`

**Round N:**
6. Orchestrator compiles all prior Q&A into `{PRIOR_QA}`
7. Sends updated prompt to DA (or spawns new DA instance with accumulated context)
8. DA either asks more questions or declares COMPLETE
9. **Persist Q&A**: Write to `.omc/state/incident-{short-id}/da-qa-{analyst-id}-round-{N}.md`

**DA declares COMPLETE** → proceed to Step 2.4 for this analyst.

**Max rounds: 3** per analyst (prevent infinite loops). After 3 rounds, proceed to scoring regardless.

### Step 2.4: Ambiguity Scoring (Single Shared Scorer)

Spawn ONE shared scorer agent for the entire team. As each analyst's DA completes, feed that analyst's findings to the scorer sequentially.

Read `prompts/ambiguity-scorer.md` (relative to the main SKILL.md).

**Spawn once (after the first DA completes):**
```
Task(
  subagent_type="oh-my-claudecode:analyst",
  name="shared-scorer",
  team_name="incident-analysis-{short-id}",
  model="sonnet",
  run_in_background=true,
  prompt="{scorer prompt with placeholders replaced}"
)
```

**For each analyst**, send scoring request to the shared scorer via `SendMessage`:
- `{ANALYST_NAME}` → analyst name
- `{ANALYST_FINDINGS}` → analyst's findings (post-Q&A updated version)
- `{DA_QA_HISTORY}` → compiled Q&A from all rounds (read from persisted files)
- `{INCIDENT_CONTEXT}` → Phase 0 details

**Persist each score**: Write scorer's JSON response to `.omc/state/incident-{short-id}/ambiguity-{analyst-id}.json`

### Step 2.5: Threshold Check

Parse the scorer's JSON output:

| Condition | Action |
|-----------|--------|
| `ambiguity ≤ 0.2` | **PASS** — mark analyst as verified. Write final findings to `.omc/state/incident-{short-id}/verified-findings-{analyst-id}.md` |
| `ambiguity > 0.2` AND loop count < 3 | **RETRY** — send `improvement_hint` from scorer to DA. Return to Step 2.3 with accumulated Q&A. Increment loop count. |
| `ambiguity > 0.2` AND loop count ≥ 3 | **FORCE PASS** — mark as verified with caveat. Note in findings: "Ambiguity score {X} exceeds threshold after 3 rounds. Lowest dimension: {dimension}." |

### Step 2.6: Compile Verified Findings

After ALL analysts are verified (PASS or FORCE PASS):

1. Compile all verified findings into `.omc/state/incident-{short-id}/analyst-findings.md`
2. Include ambiguity scores summary table
3. Flag any FORCE PASS analysts for user attention

### Phase 2 Exit Gate

MUST NOT proceed until ALL verified:

- [ ] All analysts have completed findings
- [ ] All DA Q&A rounds persisted to files
- [ ] All ambiguity scores computed and persisted
- [ ] All analysts verified (PASS or FORCE PASS)
- [ ] Compiled findings written to `analyst-findings.md`

→ **NEXT ACTION: Proceed to Phase 2.5 — Tribunal Decision.**

---

## Phase 2.5: Tribunal Decision

Tribunal is now a user decision, not an automatic trigger.

### Step 2.5.1: Present Summary

Show the user a summary of verified findings:
- Per-analyst ambiguity scores
- Any FORCE PASS analysts (highlighted)
- Key findings overview

### Step 2.5.2: Ask User

```
AskUserQuestion(
  header: "Tribunal",
  question: "All analyst findings have been verified through Socratic Q&A. Would you like to send them to a Tribunal for additional review?",
  options: [
    "Skip — proceed to report",
    "Request Tribunal"
  ]
)
```

### Step 2.5.3: If Tribunal Requested

1. Compile findings package (~10-15K tokens):
   - **Incident Summary**: 2-3 sentence recap
   - **Key Findings by Perspective**: top 3 findings per analyst with ambiguity scores
   - **Recommendations**: all proposed recommendations
   - **FORCE PASS items**: analysts that didn't meet the threshold
2. Shut down completed analysts and DAs
3. Read `prompts/tribunal.md` for critic prompts
4. Replace placeholders:
   - `{FINDINGS_PACKAGE}` → compiled findings
   - `{TRIGGER_REASON}` → "User requested tribunal review"
   - `{INCIDENT_CONTEXT}` → Phase 0 details
5. Spawn UX Critic (Sonnet) + Engineering Critic (Opus) in parallel
6. Collect reviews, run consensus round:

| Level | Condition | Label |
|-------|-----------|-------|
| Strong | 2/2 APPROVE | `[Unanimous]` |
| Caveat | 1 APPROVE, 1 CONDITIONAL | `[Approved w/caveat]` |
| Split | 1+ REJECT | `[No consensus]` → user decision |

Split → share rationale, 1 final round only. Still split → present to user via `AskUserQuestion`.

7. Compile verdict, shut down critics, proceed to Phase 3.

### If Skip

Proceed directly to Phase 3.

→ **NEXT ACTION: Proceed to Phase 3 — Synthesis & Report.**

---

## Phase 3: Synthesis & Report

### Step 3.1

Integrate all verified analyst findings. Read from `.omc/state/incident-{short-id}/analyst-findings.md`.

### Step 3.2

Read `templates/report.md` and fill all sections with synthesized findings.

### Step 3.3

`AskUserQuestion`:
- "Is the analysis complete?"
- Options: "Complete" / "Need deeper investigation" / "Add recommendations" / "Share with team"

**Deeper investigation re-entry (max 2 loops):**

Before re-entry, increment `investigation_loops` counter in `.omc/state/incident-{short-id}/context.md`. If counter ≥ 2, inform user: "Maximum investigation depth reached. Proceeding with current findings." and auto-select "Complete".

1. Write current findings to `.omc/state/incident-{short-id}/analyst-findings.md`
2. Append iteration summary to `prior-iterations.md`
3. Identify gaps via `AskUserQuestion` (header: "Investigation Gaps"):
   - "Add new perspective" → spawn new analyst only (existing findings preserved)
   - "Re-examine with focus" → user specifies focus area → targeted follow-up tasks
4. New analyst runs → full Socratic DA + Scorer verification
5. Return to Phase 3 synthesis with expanded findings

→ **NEXT ACTION: Proceed to Phase 4 — Cleanup.**

---

## Phase 4: Cleanup

> Execute `../shared-v3/team-teardown.md`.
