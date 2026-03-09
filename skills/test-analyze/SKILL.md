---
name: test-analyze
description: >
  Live test runner for prism:analyze skill. Executes full multi-agent workflow against real codebases,
  verifies JSON producer/consumer contracts between phases, checks real prism_interview verification scores,
  and validates that findings without code evidence receive low scores. Use this skill when the user says
  "test analyze", "analyze 테스트", "analyze 스킬 테스트", "live test analyze", or wants to verify the
  prism:analyze skill works correctly end-to-end.
---

# test-analyze: Live Test Runner for prism:analyze

Run live end-to-end tests of the `prism:analyze` skill with real code analysis, real MCP verification, and automated grading.

## What This Skill Tests

Three categories of assertions, each verified against real execution artifacts:

1. **Workflow Gates** (16 existing assertions) — Phase ordering, agent spawn patterns, tool calls
2. **Producer/Consumer Contracts** — JSON files written by one phase are correctly read by the next
3. **Verification Scores** — Real `prism_interview` MCP scores; findings without code evidence get low scores

## Execution

### Phase T0: Setup

1. Read test cases from `evals/evals.json` (relative to this SKILL.md)
2. Generate a test run ID: `test-{short-id}` via `uuidgen | cut -c1-8`
3. Create workspace: `~/.prism/state/{test-run-id}/`
4. Show user: "Running {N} test cases. Estimated time: ~5-8 min per test case."

### Phase T1: Execute Test Cases

For each test case in `evals/evals.json`:

#### T1.1: Spawn Analyze Orchestrator

Spawn a background agent that executes the full `prism:analyze` workflow. The agent must:

- Read `../analyze/SKILL.md` and follow it exactly
- Use the test case's `prompt` as the analysis request
- Use the test case's `target_codebase` as the code to analyze
- Use state directory: `~/.prism/state/{test-run-id}/{eval-id}/`
- Auto-approve Phase 0.6 (skip AskUserQuestion — add instruction: "Auto-approve all perspectives without asking the user")
- Skip Phase 0.7 ontology if `skip_ontology: true`
- Execute Phase 2B verification using real `prism_interview` + `prism_score` MCP calls
- Write all artifacts (seed-analysis.json, perspectives.json, context.json, findings.json, etc.)

**Spawn pattern:**

```
Agent(
  name="{eval-id}-orchestrator",
  run_in_background=true,
  mode="bypassPermissions",
  prompt="You are the orchestrator for prism:analyze. Read {skill_path}/SKILL.md and execute the FULL workflow.

TASK: {prompt}
TARGET CODEBASE: {target_codebase}
STATE DIR: ~/.prism/state/{test-run-id}/{eval-id}/

CRITICAL RULES:
- Follow SKILL.md phases exactly (Phase 0 → 4)
- Use TeamCreate + Task to spawn real sub-agents for each role
- Phase 0.6: Auto-approve perspectives (do NOT call AskUserQuestion)
- Phase 0.7: {ontology_instruction}
- Phase 2B: MUST call prism_interview and prism_score MCP tools for real verification
- Write ALL JSON artifacts to the state directory
- After Phase 3, write the final report to the state directory as report.md
"
)
```

If `parallel: true` in the test config, spawn all test cases simultaneously. Otherwise sequential.

#### T1.2: Wait for Completion

Monitor via agent completion notifications. Record timing (tokens, duration) for each.

### Phase T2: Collect & Grade Artifacts

After all orchestrators complete, grade each test case. Read `references/grading-protocol.md` for detailed grading instructions.

For each test case:

#### T2.1: Verify Workflow Gates

Check that each workflow assertion from the test case's `assertions` array is satisfied by reading the execution artifacts:

| Assertion Type | How to Verify |
|----------------|---------------|
| `workflow_gate` | Check that the expected tool call / file / directory exists |
| `field_contract` | Read the JSON file and verify all expected fields exist with correct types |
| `data_flow` | Read two JSON files and verify the consumer file references data from the producer |

#### T2.2: Verify Producer/Consumer Contracts

Read `references/contract-checks.md` for the full list. For each contract:

1. Read the **producer** JSON file
2. Read the **consumer** JSON file or prompt
3. Verify that every field the consumer expects exists in the producer output
4. Verify field values are consistent (not just present but matching)

Key contracts to verify:

| Producer | Consumer | Fields |
|----------|----------|--------|
| seed-analysis.json | perspectives.json | dimensions.domain, failure_type, complexity, research.findings |
| perspectives.json | Phase 1 spawn | id, model, agent_type, key_questions |
| perspectives.json | context.json | summary derived from research |
| context.json | Phase 1 {CONTEXT} | summary, research_summary.*, dimensions |
| context.json | Phase 3 report | report_language |
| findings.json | Phase 2B verifier | analyst, findings[].finding, findings[].evidence |

#### T2.3: Verify Verification Scores

Read each perspective's interview results from the state directory:
- `~/.prism/state/{test-run-id}/{eval-id}/perspectives/{perspective-id}/interview.json`

For each perspective, extract:
- `rounds`: number of interview rounds
- `weighted_total`: final verification score
- `verdict`: PASS or FORCE PASS

**Score expectations** (from test case `score_expectations`):
- If `expect_low_score: true` for a perspective, verify `weighted_total < 6.0`
- If `expect_high_score: true`, verify `weighted_total >= 7.0`
- If no expectation, just record the score

#### T2.4: Write Grading Results

Results directory: `{skill-dir}/test-results/{test-run-id}/`

For each test case, write to `{skill-dir}/test-results/{test-run-id}/{eval-id}/grading.json`:

```json
{
  "eval_id": "{eval-id}",
  "eval_name": "{eval-name}",
  "pass_rate": 0.0-1.0,
  "expectations": [
    {"text": "assertion text", "passed": true/false, "evidence": "brief evidence", "category": "workflow|contract|score"}
  ],
  "verification_scores": [
    {"perspective_id": "...", "rounds": 3, "weighted_total": 7.5, "verdict": "PASS"}
  ],
  "timing": {
    "total_tokens": 0,
    "duration_ms": 0
  }
}
```

### Phase T3: Aggregate & Report

1. Compile all grading results into a summary table
2. Calculate pass rates per category (workflow, contract, score)
3. Highlight any failures with evidence
4. Present to user:

```markdown
## test-analyze Results

| Test Case | Workflow | Contract | Score | Overall |
|-----------|----------|----------|-------|---------|
| auth-bypass | 16/16 | 6/6 | 2/3 | 24/25 (96%) |

### Verification Scores
| Test Case | Perspective | Score | Verdict | Expected |
|-----------|-------------|-------|---------|----------|
| auth-bypass | security | 8.2 | PASS | high |
| auth-bypass | premium-content | 3.1 | FORCE PASS | low |

### Failed Assertions
- [contract] auth-bypass: findings.json missing evidence field for finding #3
```

### Phase T4: Cleanup

- Kill any remaining background agents
- Optionally clean up state directory `~/.prism/state/{test-run-id}/` (ask user)
- Write aggregate summary to `{skill-dir}/test-results/{test-run-id}/summary.json`
