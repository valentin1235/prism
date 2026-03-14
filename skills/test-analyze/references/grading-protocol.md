# Grading Protocol

How to grade each test case after the prism:analyze orchestrator completes.

## Step 1: Locate Artifacts

All artifacts are in `~/.prism/state/{test-run-id}/{eval-id}/`. Expected files:

```
seed-analysis.json          (Phase 0.5)
perspectives.json           (Phase 0.55, updated in 0.6)
context.json                (Phase 0.8)
perspectives/{id}/findings.json     (Phase 1, per analyst)
perspectives/{id}/interview.json    (Phase 2B, per verifier)
verified-findings-{id}.md           (Phase 2B.5)
analyst-findings.md                 (Phase 2B.6)
report.md                           (Phase 3)
```

If any file is missing, the corresponding assertions FAIL.

## Step 2: Grade Workflow Gates

For each `workflow_gate` assertion, check:

| Assertion Pattern | Pass Condition |
|-------------------|----------------|
| "uuidgen + state dir" | State directory exists with files |
| "seed-analyst architect+opus" | seed-analysis.json exists and contains valid JSON |
| "shutdown + drain" | findings.json exists for all perspectives AND verified-findings files exist (proving session split happened) |
| "ontology scope" | Either ontology-scope.json exists OR skip was configured |
| "approved=true" | perspectives.json contains `"approved": true` |

## Step 3: Grade Field Contracts

For each `field_contract` assertion, read the relevant JSON and verify fields:

### seed-analysis.json
```
Required: topic,
          research.summary, research.findings[], research.key_areas,
          research.files_examined
```

### perspectives.json
```
Required: perspectives[].id, perspectives[].name, perspectives[].scope,
          perspectives[].key_questions, perspectives[].model, perspectives[].agent_type,
          perspectives[].prompt.role, perspectives[].prompt.tasks,
          perspectives[].prompt.output_format,
          perspectives[].rationale
          quality_gate.all_orthogonal,
          quality_gate.all_evidence_backed,
          quality_gate.all_specific,
          quality_gate.all_actionable,
          quality_gate.min_perspectives_met
          selection_summary
```

### context.json
```
Required: summary, research_summary.key_findings, research_summary.files_examined,
          research_summary.key_areas, report_language, investigation_loops
```

### findings.json (per perspective)
```
Required: analyst, findings[].finding, findings[].evidence, findings[].severity
```

## Step 4: Grade Data Flow (Producer/Consumer)

For each `data_flow` assertion and each `contract_check`:

1. Read the producer file
2. Read the consumer file
3. Verify the consumer's values are derived from the producer

**Concrete checks:**

### seed-analysis → perspectives
- perspectives.json's perspective selection should be grounded in seed-analysis.json's `research.findings` and `research.key_areas`
- Security-related findings should result in a security-focused perspective
- Perspective count: minimum 2, typically 3-5

### perspectives → findings
- Each findings.json `analyst` field must match a perspectives.json `id`
- Number of findings.json files must equal number of perspectives

### seed-analysis → context
- context.json `research_summary.key_findings` should contain content from seed-analysis.json `research.findings`
- context.json `research_summary.files_examined` should match seed-analysis.json `research.files_examined`

### context → report
- If `report_language` is "ko" or "Korean", report.md should be primarily in Korean

### findings → interview
- interview.json (if exists) should reference the same `perspective_id` as findings.json `analyst`

## Step 5: Grade Verification Scores

Read interview results. Two sources:
1. `perspectives/{id}/interview.json` — raw interview data
2. `analyst-findings.md` — compiled scores table

For each perspective, extract:
- `weighted_total` (or equivalent score field)
- `verdict` (PASS / FORCE PASS)
- `rounds` completed

Then check against `score_expectations` in the eval:

| Expectation | Pass Condition |
|-------------|----------------|
| `expect: "high"` | weighted_total >= 7.0 |
| `expect: "low"` | weighted_total < 6.0 |
| `expect: "mixed"` | weighted_total is NOT a perfect 10.0 (some findings lack evidence) |
| `expect: "low_to_mixed"` | weighted_total <= max_expected_score |

If no interview.json exists for a perspective (MCP call failed or wasn't made), the score assertion FAILS.

## Step 6: Compile Results

Count passes per category:
- **workflow**: workflow_gate assertions
- **contract**: field_contract assertions + contract_checks
- **score**: verification score expectations

Calculate overall pass rate and write to grading.json.
