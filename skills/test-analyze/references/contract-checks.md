# Producer/Consumer Contract Checks

Complete list of JSON contracts between prism:analyze phases. Each contract specifies which file produces data and which phase/file consumes it.

## Contract Map

```
Phase 0.5 (seed-analyst)
  └─ seed-analysis.json
       ├─→ Phase 0.55 (perspective-generator) reads: topic, research (findings, key_areas)
       └─→ Phase 0.8 (orchestrator) reads: research → context.json

Phase 0.55 (perspective-generator)
  └─ perspectives.json
       ├─→ Phase 0.6 (orchestrator) adds: approved, user_modifications
       ├─→ Phase 1 (orchestrator) reads: id, model, agent_type, key_questions, prompt
       └─→ Phase 2B (orchestrator) reads: id, model, agent_type, prompt

Phase 0.8 (orchestrator)
  └─ context.json
       ├─→ Phase 1 (orchestrator) reads: summary → {CONTEXT}
       ├─→ Phase 2B (orchestrator) reads: summary → {CONTEXT}, {TOPIC_SUMMARY}
       └─→ Phase 3 (orchestrator) reads: report_language

Phase 1 (analysts)
  └─ perspectives/{id}/findings.json
       └─→ Phase 2B (verifiers) reads: analyst, findings[]

Phase 2B (verifiers)
  ├─ perspectives/{id}/interview.json (MCP artifact)
  ├─ verified-findings-{id}.md
  └─ analyst-findings.md
       └─→ Phase 3 (orchestrator) reads: all findings + scores
```

## Verification Procedure

For each arrow (→) in the map above:

1. **Read producer file** — parse JSON, extract the listed fields
2. **Read consumer file** — parse JSON or markdown
3. **Check field presence** — every field the consumer expects must exist in the producer
4. **Check value consistency** — values must logically match (not just exist)

### Detailed Checks

#### 1. seed-analysis.json → perspectives.json

**Producer fields:**
```json
{
  "topic": "description",
  "research": {
    "summary": "...",
    "findings": [{"area": "...", "description": "...", "source": "..."}],
    "key_areas": ["area1", "area2"]
  }
}
```

**Consumer expectations:**
- `perspectives[].scope` should reference topics from `research.findings` and `research.key_areas`
- Perspective count: minimum 2, typically 3-5
- Perspectives should cover key areas identified in seed research
- `quality_gate` fields must all be `true` (all_orthogonal, all_evidence_backed, all_specific, all_actionable, min_perspectives_met)

#### 2. perspectives.json → findings.json (per perspective)

**Producer fields:**
```json
{
  "perspectives": [
    {"id": "security", "key_questions": ["Q1", "Q2"]}
  ]
}
```

**Consumer expectations:**
- `findings.json` `analyst` field == `perspectives[].id`
- Findings should address topics in `key_questions` (semantic check)
- One findings.json per perspective (file count == perspective count)

#### 3. seed-analysis.json → context.json

**Producer fields:**
```json
{
  "research": {
    "findings": [{"area": "...", "description": "..."}],
    "files_examined": ["path1", "path2"]
  }
}
```

**Consumer expectations:**
- `context.json.research_summary.key_findings` derived from `research.findings[].area` and `research.findings[].description`
- `context.json.research_summary.files_examined` derived from `research.files_examined`
- `context.json.research_summary.key_areas` reflects seed key_areas

#### 4. context.json → report.md

**Producer fields:**
```json
{
  "report_language": "ko",
  "investigation_loops": 0
}
```

**Consumer expectations:**
- Report written in the language specified by `report_language`
- If Korean, majority of report text should be Korean characters

#### 5. findings.json → interview.json

**Producer fields:**
```json
{
  "analyst": "security",
  "findings": [{"finding": "...", "evidence": "...", "severity": "high"}]
}
```

**Consumer expectations:**
- interview.json `perspective_id` matches findings.json `analyst`
- Interview questions should challenge the specific findings
- Findings without real code evidence should receive lower scores

#### 6. perspectives.json → verified-findings-{id}.md

**Producer fields:**
```json
{
  "perspectives": [{"id": "security"}, {"id": "timeline"}]
}
```

**Consumer expectations:**
- One `verified-findings-{id}.md` file per perspective
- Filename `{id}` matches `perspectives[].id` exactly
