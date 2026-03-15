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
1. Read the perspective's `prompt` object from `perspectives.json` (same dynamic prompt used in Phase 1)
2. Read `prompts/verification-protocol.md`
3. Assemble: `[worker preamble] + [perspective prompt] + [verification protocol]`
4. Replace placeholders (`{CONTEXT}`, `{ONTOLOGY_SCOPE}`, `{SHORT_ID}`, `{perspective-id}`, `{TOPIC_SUMMARY}`)
5. Spawn via `Task(...)`

> **Note:** Unlike Phase 1 (which appends `finding-protocol.md`), Phase 2B appends `verification-protocol.md`. Therefore `{KEY_QUESTIONS}` and `{ORIGINAL_INPUT}` are intentionally omitted — they exist only in `finding-protocol.md`.

> `{perspective-id}` is derived from the perspective's `id` field in `perspectives.json`. The orchestrator MUST replace it in both the perspective prompt and the verification protocol before spawning.

**Spawn pattern:**

```
Task(
  subagent_type="prism:finder",
  name="{perspective-id}-verifier",
  team_name="analyze-{short-id}",
  model="{model}",
  run_in_background=true,
  prompt="{verification prompt with placeholders replaced}"
)
```

> Apply worker preamble from `protocols/worker-preamble.md` with:
- `{TEAM_NAME}` = `"analyze-{short-id}"`
- `{WORKER_NAME}` = `"{perspective-id}-verifier"`
- `{WORK_ACTION}` = `"Read your findings from the path specified in your verification protocol. Run self-verification via MCP tools (prism_interview). Re-investigate with tools as needed to answer interview questions. Report verified findings via SendMessage to team-lead."`

`{model}` and `{agent_type}` come from each perspective's fields in `perspectives.json` — same values used in Phase 1.

MUST replace `{CONTEXT}` with a text summary derived from `context.json`: format as `Summary: {summary}\nKey Findings: {research_summary.key_findings joined}\nFiles Examined: {research_summary.files_examined joined}\nKey Areas: {research_summary.key_areas joined}`.
MUST replace `{ONTOLOGY_SCOPE}` by reading `ontology-scope.json` and generating a text block per Phase B of ontology-scope-mapping.md (or "N/A" if not found).
MUST replace `{SHORT_ID}` with the session's `{short-id}`. Verifiers use the same session path as their finding counterpart: `analyze-{short-id}/perspectives/{perspective-id}`.
MUST replace `{perspective-id}` with the perspective's `id` field from `perspectives.json`. This value appears in the findings path, the `prism_interview` call, and throughout the verification protocol.
MUST replace `{TOPIC_SUMMARY}` with a short description of the topic, derived from `context.json`'s `summary` field.

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
4. Write verification log to `~/.prism/state/analyze-{short-id}/verification-log.json`:
   ```json
   {
     "verification_executed": true,
     "verifiers_spawned": ["perspective-id-1", "perspective-id-2"],
     "scores": [
       {"perspective_id": "...", "rounds": 3, "weighted_total": 0.82, "verdict": "PASS"},
       {"perspective_id": "...", "rounds": 5, "weighted_total": 0.65, "verdict": "FORCE PASS"}
     ],
     "key_clarifications": [
       {"perspective_id": "...", "round": 2, "question": "...", "answer": "...", "impact": "..."}
     ]
   }
   ```
   This file is the authoritative source for Phase 3 report generation — verification scores MUST appear in the final report.

### Phase 2 Exit Gate

- [ ] All verifiers have completed and sent verified findings
- [ ] All background task outputs drained via `TaskOutput`
- [ ] All verifiers shut down
- [ ] All verified findings persisted
- [ ] Compiled findings written to `analyst-findings.md`
- [ ] `verification-log.json` written with per-analyst scores and verdicts → ERROR: "Phase 2 blocked: verification-log.json missing. Write it per Step 2B.6 item 4."

→ **NEXT ACTION: Proceed to Phase 3 — Synthesis & Report.**

---

## Phase 3: Synthesis & Report

### Step 3.1: Integrate Findings and Verification Data

Integrate all verified analyst findings. Read BOTH files:
1. `~/.prism/state/analyze-{short-id}/analyst-findings.md` — compiled findings
2. `~/.prism/state/analyze-{short-id}/verification-log.json` — verification scores and clarifications

The verification data MUST be included in the final report's "Socratic Verification Summary" section. This is a core differentiator of the analysis — omitting verification scores defeats the purpose of the Socratic verification phase.

### Step 3.2: Select Report Template

Check for config-provided report template:
1. Read `~/.prism/state/analyze-{short-id}/config.json` (if exists)
2. If `config.report_template` is set → Read the template at that path
3. If no config or no `report_template` → Read `templates/report.md` (default template, relative to SKILL.md)

Fill the selected template with synthesized findings. Write the report in the language specified by `context.json.report_language`.

### Step 3.2.1: Report Template Compliance Check

Before presenting to user, verify the generated report contains ALL required sections from the template:

- [ ] Executive Summary
- [ ] Analysis Overview (topic, date, method, perspectives, reference docs)
- [ ] Perspective Findings (one subsection per analyst)
- [ ] Integrated Analysis (convergence, divergence, emergent insights)
- [ ] Socratic Verification Summary (per-analyst scores table, key clarifications, unresolved ambiguities) → ERROR: "Report missing verification scores. Read verification-log.json and populate the Socratic Verification Summary section."
- [ ] Recommendations (with priority/impact/effort table + immediate/short/long-term)
- [ ] Appendix

If any section is missing or empty, fix it before proceeding. The "Socratic Verification Summary" section is the most commonly omitted — explicitly check for the per-analyst scores table.

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

> Execute `protocols/team-teardown.md`.
