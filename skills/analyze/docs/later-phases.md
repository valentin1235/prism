# Later Phases — Phase 2 through Phase 4

Read this file when entering Phase 2. Do NOT preload.

---

## Phase 2: Collect Findings & Spawn Verification Sessions

Phase 2 has two stages. Stage A collects findings from the finding sessions (Phase 1). Stage B shuts down those sessions and spawns new verification sessions for each analyst.

### Stage A: Collect Findings

#### Step 2A.1: Wait for Analyst Findings

Monitor analyst completion via `TaskList`. Each analyst (from Phase 1) will:
1. Write findings to `~/.prism/state/analyze-{short-id}/perspectives/{perspective-id}/findings.json`
2. Send findings to team-lead via `SendMessage`

The `SendMessage` from each analyst includes:
- context_id, perspective_id
- Findings with evidence

#### Step 2A.2: Drain Background Task Outputs

After each analyst completes (Claude Code bug workaround [#27431]):
- `TaskList` → find completed tasks → `TaskOutput` for each

#### Step 2A.3: Shutdown Finding Analysts

For each completed analyst:
- `SendMessage(type: "shutdown_request", recipient: "{perspective-id}-analyst", content: "Finding phase complete.")`

Wait for shutdown acknowledgment, then drain task outputs again.

#### Stage A Exit Gate

MUST NOT proceed until ALL checked:

- [ ] All analyst findings received via `SendMessage` → ERROR: "Stage A blocked: not all analysts sent findings. Run TaskList."
- [ ] All `findings.json` files written → ERROR: "Check ~/.prism/state/analyze-{short-id}/perspectives/{perspective-id}/findings.json for each analyst."
- [ ] All finding analysts shut down → ERROR: "Send shutdown_request to each analyst."
- [ ] All background task outputs drained → ERROR: "Run TaskList → TaskOutput for each completed task ([#27431])."

→ **NEXT ACTION: Proceed to Stage B — Spawn Verification Sessions.**

---

### Stage B: Spawn Verification Sessions

Each analyst gets a new session for Socratic verification. The new session reads the findings.json written by the finding session.

#### Step 2B.1: Spawn Verification Sessions

For each perspective in `perspectives.json`, spawn a new verification session.

Create verifier task via `TaskCreate`, pre-assign owner via `TaskUpdate(owner="{perspective-id}-verifier")`, then spawn.

MUST read prompt files before spawning. Files are relative to the SKILL.md directory.

**Prompt assembly order:** For each verifier:
1. Read archetype section from `prompts/core-archetypes.md` or `prompts/extended-archetypes.md` (same archetype as Phase 1 — use the same `agent_type` and `model` from the archetype table)
2. Read `prompts/verification-protocol.md`
3. Concatenate: `[worker preamble] + [archetype prompt] + [verification protocol]`
4. Replace placeholders (`{CONTEXT}`, `{ONTOLOGY_SCOPE}`, `{SHORT_ID}`, `{perspective-id}`, `{summary}`)
5. Spawn via `Task(...)`

> `{perspective-id}` is derived from the perspective's `id` field in `perspectives.json`. The orchestrator MUST replace it in both the archetype prompt and the verification protocol before spawning.

**Spawn pattern:**

```
Task(
  subagent_type="oh-my-claudecode:{agent_type}",
  name="{perspective-id}-verifier",
  team_name="analyze-{short-id}",
  model="{model}",
  run_in_background=true,
  prompt="{verification prompt with {CONTEXT}, {ONTOLOGY_SCOPE}, {SHORT_ID}, {perspective-id}, {summary} replaced}"
)
```

> Apply worker preamble from `../shared-v3/worker-preamble.md` with:
- `{TEAM_NAME}` = `"analyze-{short-id}"`
- `{WORKER_NAME}` = `"{perspective-id}-verifier"`
- `{WORK_ACTION}` = `"Read your findings from the path specified in your verification protocol. Run self-verification via MCP tools (prism_interview). Re-investigate with tools as needed to answer interview questions. Report verified findings via SendMessage to team-lead."`

MUST replace `{CONTEXT}` with a text summary derived from `context.json`: format as `Summary: {summary}\nKey Findings: {research_summary.key_findings joined}\nFiles Examined: {research_summary.files_examined joined}\nDimensions: {research_summary.dimensions}`.
MUST replace `{ONTOLOGY_SCOPE}` by reading `ontology-scope.json` and generating a text block per Phase B of ontology-scope-mapping.md (or "N/A" if not found).
MUST replace `{SHORT_ID}` with the session's `{short-id}`. Verifiers use the same session path as their finding counterpart: `analyze-{short-id}/perspectives/{perspective-id}`.
MUST replace `{summary}` with a short description of the case, derived from `context.json`'s `summary` field.

#### Step 2B.2: Wait for Verified Findings

Monitor verification session completion via `TaskList`. Each verifier will:
1. Read `~/.prism/state/analyze-{short-id}/perspectives/{perspective-id}/findings.json`
2. Run self-verification (prism_interview loop with integrated scoring)
3. Send verified findings to team-lead via `SendMessage`

The `SendMessage` from each verifier includes:
- context_id, perspective_id, rounds completed, weighted_total score, verdict (PASS/FORCE PASS)
- Verified findings (refined through Q&A)
- Key Q&A clarifications

#### Step 2B.3: Drain Background Task Outputs

After each verifier completes:
- `TaskList` → find completed tasks → `TaskOutput` for each

#### Step 2B.4: Shutdown Verifiers

For each completed verifier:
- `SendMessage(type: "shutdown_request", recipient: "{perspective-id}-verifier", content: "Verification complete.")`

Wait for shutdown acknowledgment, then drain task outputs again.

#### Step 2B.5: Persist Verified Results

For each analyst perspective:
- Write verified findings to `~/.prism/state/analyze-{short-id}/verified-findings-{perspective-id}.md`
- MCP session artifacts are at `~/.prism/state/analyze-{short-id}/perspectives/{perspective-id}/` (interview.json + findings.json)

#### Step 2B.6: Compile Verified Findings

After ALL verifiers are done:

1. Compile all verified findings into `~/.prism/state/analyze-{short-id}/analyst-findings.md`
2. Include verification scores summary table (per perspective: perspective_id, rounds, weighted_total, verdict)
3. Flag any FORCE PASS analysts for user attention

### Phase 2 Exit Gate

- [ ] All verifiers have completed and sent verified findings
- [ ] All background task outputs drained via `TaskOutput`
- [ ] All verifiers shut down
- [ ] All verified findings persisted
- [ ] Compiled findings written to `analyst-findings.md`

→ **NEXT ACTION: Proceed to Phase 3 — Synthesis & Report.**

---

## Phase 3: Synthesis & Report

### Step 3.1

Integrate all verified analyst findings. Read from `~/.prism/state/analyze-{short-id}/analyst-findings.md`.

### Step 3.2

Read `templates/report.md` and fill all sections with synthesized findings. Write the report in the language specified by `context.json.report_language`.

### Step 3.3

`AskUserQuestion`:
- "Is the analysis complete?"
- Options: "Complete" / "Need deeper investigation" / "Add recommendations" / "Share with team"

**Deeper investigation re-entry (max 2 loops):**

Before re-entry, increment `investigation_loops` counter in `~/.prism/state/analyze-{short-id}/context.json`. If counter ≥ 2, inform user: "Maximum investigation depth reached. Proceeding with current findings." and auto-select "Complete".

1. Write current findings to `~/.prism/state/analyze-{short-id}/analyst-findings.md`
2. Append iteration summary to `prior-iterations.md`
3. Identify gaps via `AskUserQuestion` (header: "Investigation Gaps"):
   - "Add new perspective" → spawn new analyst only (existing findings preserved)
   - "Re-examine with focus" → user specifies focus area → targeted follow-up tasks
4. New analyst runs → finding session first, then verification session (same Phase 2 two-stage flow)
5. Return to Phase 3 synthesis with expanded findings

→ **NEXT ACTION: Proceed to Phase 4 — Cleanup.**

---

## Phase 4: Cleanup

> Execute `../shared-v3/team-teardown.md`.
