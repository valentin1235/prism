# Early Phases — Reference Tables

This file provides reference tables for seed analysis dimension evaluation and archetype mapping. These tables are used by:
- **Orchestrator**: Phase 0 intake (SKILL.md § Phase 0)
- **Seed-analyst**: Active research and perspective generation (`prompts/seed-analyst.md`)

> **Note**: Phase 0 and seed analysis are NOT delegated to setup-agent for incident-v3. The orchestrator handles intake directly, and seed-analyst runs as a team member. See `prompts/seed-analyst.md` for the seed-analyst prompt.

---

## Dimension Evaluation

Evaluate the incident across 5 dimensions:

| Dimension | Values | Impact on Selection |
|-----------|--------|-------------------|
| Domain | infra / app / data / security / network | Maps to archetype categories |
| Failure type | crash / degradation / data loss / breach / misconfig | Determines analytical frameworks |
| Evidence available | logs / metrics / code diffs / traces | MUST NOT select perspectives without evidence |
| Complexity | single-cause / multi-factor | Simple: 2-3 perspectives. Complex: 4-5 |
| Recurrence | first-time / recurring | Recurring → add `systems` for pattern analysis |

---

## Characteristic → Archetype Mapping

| Incident Characteristics | Recommended Archetypes |
|-------------------------|----------------------|
| Security breach, unauthorized access | `security` + `timeline` + `systems` |
| Data corruption, stale reads, replication lag | `data-integrity` + `root-cause` + `systems` |
| Latency spike, OOM, resource exhaustion | `performance` + `root-cause` + `systems` |
| Post-deployment failure, config drift | `deployment` + `timeline` + `root-cause` |
| Network partition, DNS failure, LB issue | `network` + `systems` + `timeline` |
| Race condition, deadlock, distributed lock | `concurrency` + `root-cause` + `systems` |
| Third-party API failure, upstream outage | `dependency` + `impact` + `timeline` |
| User-facing degradation, confusing errors, UX breakage | `ux` + `impact` + `root-cause` |
| Novel / unclassifiable | `custom` + `root-cause` + relevant core |

Use this mapping as starting point, then refine based on specific evidence from active research.
