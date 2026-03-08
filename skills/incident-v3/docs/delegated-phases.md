# Early Phases — Reference Tables

This file provides reference tables for seed analysis dimension evaluation and archetype mapping. These tables are used by:
- **Orchestrator**: Phase 0 intake (SKILL.md § Phase 0)
- **Seed-analyst**: Active research and dimension evaluation (`prompts/seed-analyst.md`)
- **Perspective-generator**: Archetype mapping and perspective selection (`prompts/perspective-generator.md`)

> **Note**: Phase 0 and seed analysis are NOT delegated to setup-agent for incident-v3. The orchestrator handles intake directly, and seed-analyst runs as a team member. See `prompts/seed-analyst.md` for the seed-analyst prompt.

---

## Dimension Evaluation

Evaluate the incident across 5 dimensions:

| Dimension | Values |
|-----------|--------|
| Domain | infra / app / data / security / network |
| Failure type | crash / degradation / data loss / breach / misconfig |
| Evidence available | logs / metrics / code diffs / traces |
| Complexity | single-cause / multi-factor |
| Recurrence | first-time / recurring |

> Selection rules (evidence-backed, complexity scaling, recurring → systems) are enforced by the perspective-generator. See `prompts/perspective-generator.md` § STEP 3.

---

## Characteristic → Archetype Mapping

> Canonical table lives in `prompts/perspective-generator.md` § STEP 2. Duplicated here for quick reference only — perspective-generator is the source of truth.

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

Use this mapping as a starting point. Perspective-generator refines based on seed analyst evidence and mandatory rules.
