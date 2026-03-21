> **DEPRECATED (v6.0):** This protocol is no longer used at runtime. Finding phase logic has been
> reimplemented in the Go MCP server (`mcp/stage2_exec.go`). Retained as design reference only.

# Analyst Protocol — Finding Phase

## Perspective-Specific Questions

{KEY_QUESTIONS}

Answer these questions in addition to your investigation tasks. They are grounded in the seed analyst's research findings and target this specific case.

## Data Source Constraint

You MUST only use data sources listed in the "Reference Documents" section above. Do NOT use `ToolSearch` to discover or call MCP servers not in your Reference Documents. If a data source is not listed there, it was not selected for this analysis and MUST NOT be used.

## Task Lifecycle

Read task via `TaskGet` → mark `in_progress` → investigate → write findings → `SendMessage` to team-lead → mark `completed` via `TaskUpdate`.

## Investigation & Finding

After completing your investigation, write your findings and report to the orchestrator. Do NOT run self-verification (prism_interview) — that happens in a separate session.

Your findings path is: `~/.prism/state/analyze-{SHORT_ID}/perspectives/{perspective-id}/findings.json`

### Steps

#### 1. Investigate

Answer ALL key questions and complete ALL tasks from your prompt with evidence and code references. Use available tools (Grep, Read, Bash, MCP docs) to gather evidence.

#### 2. Write Findings

Write your findings to `~/.prism/state/analyze-{SHORT_ID}/perspectives/{perspective-id}/findings.json`:

```json
{
  "analyst": "{perspective-id}",
  "input": "{ORIGINAL_INPUT}",
  "findings": [
    {
      "finding": "description",
      "evidence": "file:function:line — detail",
      "severity": "use the severity scale defined in your output format"
    }
  ]
}
```

`{ORIGINAL_INPUT}` is the original topic description from context.json. Copy it exactly — the verification interviewer uses this to evaluate whether your findings address the original topic.

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
