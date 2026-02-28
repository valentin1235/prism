# Incident Analysis Report Template

Fill all sections. Write "N/A" if truly irrelevant — do NOT leave sections empty.

## Executive Summary
[2-3 sentences: what happened, why, what to do]

## Incident Overview
- **Incident**: {description}
- **Severity**: {SEV}
- **Duration**: {start} to {end/ongoing}
- **Status**: {Active/Mitigated/Resolved}
- **Affected Systems**: {list}
- **User Impact**: {summary}
- **Perspectives Used**: {list of active lenses}

## Timeline
| Time | Event | Evidence | Confidence |
|------|-------|----------|------------|

## Perspective Findings

{For EACH perspective (excluding DA):}

### {Lens Name}
{Full findings per archetype output format}

## Integrated Analysis
- **Convergence**: Where perspectives independently agreed
- **Divergence**: Where they disagreed + resolution
- **Emergent Insights**: Findings only visible when combining perspectives

## Devil's Advocate Findings
### Fallacy Check Results
| # | Analyst | Claim | Verdict | Fallacy / Issue | Severity | Detail |
|---|---------|-------|---------|-----------------|----------|--------|

### Cross-Analyst Contradictions
| Analyst A Claims | Analyst B Claims | Contradiction | Question to Resolve |
|-----------------|-----------------|---------------|-------------------|

### Perspective Critique
| Perspective | Appropriateness | What It Might Miss | Question for Analyst |
|-------------|----------------|-------------------|---------------------|

### Ontology Scope Critique
| Perspective | Mapped Docs | Missed Docs? | Evidence Gap |
|-------------|-------------|-------------|--------------|

### Unanswered Questions
- [Questions that MUST be answered before conclusions can be drawn]

### Aggregate Verdict
- BLOCKING: {count}
- MAJOR: {count}
- MINOR: {count}

## Tribunal Review (if activated)
- Trigger: {reason}
- Consensus: {X} unanimous, {Y} w/caveats, {Z} w/dissent

### Recommendation Verdicts
| Recommendation | DA | UX Critic | Eng Critic | Final |
|---------------|----|-----------|------------|-------|

### Items Requiring User Decision
### UX Recommendations (from UX Critic)
### Engineering Notes (from Eng Critic)

## Recommendations
| Action | Priority | UX Impact | Eng Effort | Tribunal Verdict |
|--------|----------|-----------|------------|-----------------|

### Immediate (This Week)
### Short-Term (This Month)
### Long-Term (This Quarter)
### Monitoring & Alerting

## Prevention Checklist
- [ ] Root cause permanently fixed
- [ ] Monitoring for early detection
- [ ] Runbook updated
- [ ] Post-incident review scheduled
- [ ] Similar risks elsewhere mitigated

## Appendix

### Perspectives and Rationale
[Why each perspective was selected]

### Ontology Scope Mapping
| Perspective | Mapped Ontology Docs | Reasoning |
|-------------|---------------------|-----------|

### Ontology Catalog
| # | Path | Domain | Summary |
|---|------|--------|---------|

### Raw Evidence Links
### Related Past Incidents
