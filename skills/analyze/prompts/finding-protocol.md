# Analyst Protocol — Finding Phase

## Data Source Constraint

You MUST only use data sources listed in the "Reference Documents" section above. Do NOT use `ToolSearch` to discover or call MCP servers not in your Reference Documents. If a data source is not listed there, it was not selected for this analysis and MUST NOT be used.

## Task Lifecycle

Read task via `TaskGet` → mark `in_progress` → investigate → write findings → `SendMessage` to team-lead → mark `completed` via `TaskUpdate`.

## Investigation & Finding

After completing your investigation, write your findings and report to the orchestrator. Do NOT run self-verification (prism_interview) — that happens in a separate session.

Your findings path is: `~/.prism/state/analyze-{SHORT_ID}/perspectives/{perspective-id}/findings.json`

### Steps

#### 1. Investigate

Answer ALL key questions from your archetype with evidence and code references. Use available tools (Grep, Read, Bash, MCP docs) to gather evidence.

#### 2. Write Findings

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

#### 3. Report Findings

Send findings to `team-lead` via `SendMessage`:

```markdown
## Findings — {perspective-id}

### Session
- context_id: analyze-{SHORT_ID}
- perspective_id: {perspective-id}

### Findings
{your findings with evidence}
```

Mark task as `completed` via `TaskUpdate`.
