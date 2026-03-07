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

{For EACH perspective:}

### {Lens Name}
{Full findings per archetype output format}

## Integrated Analysis
- **Convergence**: Where perspectives independently agreed
- **Divergence**: Where they disagreed + resolution
- **Emergent Insights**: Findings only visible when combining perspectives

## Socratic Verification Summary

### Per-Analyst Ambiguity Scores
| Analyst | Evidence Clarity | Causal Chain Clarity | Recommendation Clarity | Ambiguity Score | Verdict |
|---------|-----------------|---------------------|----------------------|----------------|---------|

### Key Clarifications from DA Q&A
{For each analyst, list the most important ambiguities that were resolved through Socratic questioning}

#### {Analyst Name}
| Round | Question | Answer | Impact on Findings |
|-------|----------|--------|--------------------|

### Unresolved Ambiguities
{Any FORCE PASS items or known unknowns that could not be resolved}

| Analyst | Ambiguity | Reason Unresolved | Impact on Conclusions |
|---------|-----------|-------------------|-----------------------|

## Tribunal Review

{If tribunal was NOT requested:}
> Tribunal not requested by user. All findings carry Socratic-verified status.

{If tribunal WAS requested, fill below:}
- Trigger: User requested
- Consensus: {X} unanimous, {Y} w/caveats, {Z} w/dissent

### Recommendation Verdicts
| Recommendation | UX Critic | Eng Critic | Final |
|---------------|-----------|------------|-------|

### Items Requiring User Decision
### UX Recommendations (from UX Critic)
### Engineering Notes (from Eng Critic)

## Recommendations
| Action | Priority | UX Impact | Eng Effort | Verified? |
|--------|----------|-----------|------------|-----------|

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

### Ambiguity Score Details
{Full JSON scores from each scorer, for reference}

### Ontology Scope Mapping
| Perspective | Mapped Ontology Docs | Reasoning |
|-------------|---------------------|-----------|

### Ontology Catalog
| # | Source | Type | Path/URL | Domain | Summary |
|---|--------|------|----------|--------|---------|

### Raw Evidence Links
### Related Past Incidents
