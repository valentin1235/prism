# Later Phases — Phase 2 through Phase 4

Read this file when entering Phase 2. Do NOT preload.

---

## Phase 2: Collect Verified Findings

Each analyst runs self-verification via MCP tools (prism_interview) autonomously. The orchestrator only collects verified results.

### Architecture

```
analyst-1: investigate → write findings.json → prism_interview loop (integrated scoring) → SendMessage(verified)
analyst-2: investigate → write findings.json → prism_interview loop (integrated scoring) → SendMessage(verified)
...
orchestrator: wait for all analysts → collect verified findings → compile
```

### Step 2.1: Wait for Analysts

Monitor analyst completion via `TaskList`. Each analyst will:
1. Write findings to `~/.prism/state/incident-{short-id}/perspectives/{perspective-id}/findings.json`
2. Run self-verification (prism_interview loop with integrated scoring)
3. Send verified findings to team-lead via `SendMessage`

The `SendMessage` from each analyst includes:
- context_id, perspective_id, rounds completed, weighted_total score, verdict (PASS/FORCE PASS)
- Verified findings (refined through Q&A)
- Key Q&A clarifications

### Step 2.2: Drain Background Tasks

After each analyst completes (Claude Code bug workaround [#27431]):
- `TaskList` → find completed tasks → `TaskOutput` for each

### Step 2.3: Persist Verified Results

For each analyst:
- Write verified findings to `~/.prism/state/incident-{short-id}/verified-findings-{perspective-id}.md`
- MCP session artifacts are at `~/.prism/state/incident-{short-id}/perspectives/{perspective-id}/` (interview.json + findings.json)

### Step 2.4: Compile Verified Findings

After ALL analysts are verified:

1. Compile all verified findings into `~/.prism/state/incident-{short-id}/analyst-findings.md`
2. Include ambiguity scores summary table (per perspective: perspective_id, goal, constraints, criteria, weighted_total, verdict)
3. Flag any FORCE PASS analysts for user attention

### Phase 2 Exit Gate

- [ ] All analysts have completed and sent verified findings
- [ ] All background task outputs drained via `TaskOutput`
- [ ] All verified findings persisted
- [ ] Compiled findings written to `analyst-findings.md`

→ **NEXT ACTION: Proceed to Phase 3 — Synthesis & Report.**

---

## Phase 3: Synthesis & Report

### Step 3.1

Integrate all verified analyst findings. Read from `~/.prism/state/incident-{short-id}/analyst-findings.md`.

### Step 3.2

Read `templates/report.md` and fill all sections with synthesized findings.

### Step 3.3

`AskUserQuestion`:
- "Is the analysis complete?"
- Options: "Complete" / "Need deeper investigation" / "Add recommendations" / "Share with team"

**Deeper investigation re-entry (max 2 loops):**

Before re-entry, increment `investigation_loops` counter in `~/.prism/state/incident-{short-id}/context.md`. If counter ≥ 2, inform user: "Maximum investigation depth reached. Proceeding with current findings." and auto-select "Complete".

1. Write current findings to `~/.prism/state/incident-{short-id}/analyst-findings.md`
2. Append iteration summary to `prior-iterations.md`
3. Identify gaps via `AskUserQuestion` (header: "Investigation Gaps"):
   - "Add new perspective" → spawn new analyst only (existing findings preserved)
   - "Re-examine with focus" → user specifies focus area → targeted follow-up tasks
4. New analyst runs → full MCP Socratic verification (prism_interview)
5. Return to Phase 3 synthesis with expanded findings

→ **NEXT ACTION: Proceed to Phase 4 — Cleanup.**

---

## Phase 4: Cleanup

> Execute `../shared-v3/team-teardown.md`.
