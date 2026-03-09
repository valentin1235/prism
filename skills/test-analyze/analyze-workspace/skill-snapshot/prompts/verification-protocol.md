# Analyst Protocol

## Data Source Constraint

You MUST only use data sources listed in the "Reference Documents" section above. Do NOT use `ToolSearch` to discover or call MCP servers not in your Reference Documents. If a data source is not listed there, it was not selected for this analysis and MUST NOT be used.

## Task Lifecycle

Read task via `TaskGet` → mark `in_progress` → investigate → self-verify (below) → `SendMessage` to team-lead → mark `completed` via `TaskUpdate`.

## Self-Verification (MCP)

After completing your investigation, run self-verification using MCP tools before reporting to team-lead.

Your session path is: `analyze-{SHORT_ID}/perspectives/{perspective-id}`

### Steps

#### 1. Write Findings

Write your findings to `~/.prism/state/analyze-{SHORT_ID}/perspectives/{perspective-id}/findings.json`:

```json
{
  "analyst": "{perspective-id}",
  "findings": [
    {
      "finding": "description",
      "evidence": "file:function:line — detail",
      "severity": "critical|high|medium|low"
    }
  ]
}
```

#### 2. Start Interview

```
mcp__prism-mcp__prism_interview(
  context_id="analyze-{SHORT_ID}",
  perspective_id="{perspective-id}",
  topic="{perspective-id} findings verification — {summary}"
)
→ returns { context_id, perspective_id, round, question }
```

#### 3. Answer + Integrated Score Loop

The interview tool has integrated scoring — each answer submission automatically scores and returns `continue: true/false`.

For each question from the interviewer:

1. **Answer the question** — re-investigate using tools (Grep, Read, Bash) if needed to provide evidence-backed answers
2. **Submit answer:**
```
mcp__prism-mcp__prism_interview(
  context_id="analyze-{SHORT_ID}",
  perspective_id="{perspective-id}",
  response="{your answer}"
)
→ returns { context_id, perspective_id, round, continue, score, question?, reason? }
```
3. **Check response:**
   - `continue: false` + `reason: "pass"` → **PASS** — proceed to step 4
   - `continue: false` + `reason: "interview_complete"` → **PASS** — proceed to step 4
   - `continue: false` + `reason: "max_rounds"` → **FORCE PASS** — proceed to step 4 with caveat
   - `continue: true` → answer the returned `question`, repeat loop

#### 4. Report Verified Findings

Send verified findings to `team-lead` via `SendMessage`:

```markdown
## Verified Findings — {perspective-id}

### Session
- context_id: analyze-{SHORT_ID}
- perspective_id: {perspective-id}
- rounds: {N}
- score: {weighted_total}
- verdict: PASS | FORCE PASS (score {X} after {N} rounds)

### Findings
{your findings, refined through Q&A}

### Key Q&A Clarifications
{important clarifications from the interview that strengthened your analysis}
```

Mark task as `completed` via `TaskUpdate`.
