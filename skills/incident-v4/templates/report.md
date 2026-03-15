# Incident RCA Report

---

## Executive Summary

[2-3 sentences: what happened, root cause, severity, current status]

## Incident Overview

- **Incident**: {incident title}
- **Analysis Date**: {date}
- **Method**: {N}-perspective multi-perspective analysis + Socratic verification
- **Reference Docs**: {number of ontology documents} documents referenced
- **Perspectives Used**: {list of perspectives}

---

## Timeline

| Time | Event | Source |
|------|-------|--------|
| {time} | {event description} | {how this was determined} |

> If timeline data is insufficient, state: "Timeline could not be fully reconstructed from available evidence."

---

## Root Cause

### Primary Root Cause

**What**: {description of the root cause}

**Where**: {code path / system component}

**Why**: {explanation of why this caused the incident}

**Confidence**: {Verified / Partial / Unverified}

### Contributing Factors

| # | Factor | Relationship to Root Cause | Confidence |
|---|--------|---------------------------|------------|
| 1 | {factor} | {how it contributed} | {badge} |

### Trigger

{The immediate event that initiated the incident}

---

## UX Impact Analysis

### User Experience During Incident

{What users saw, experienced, or were unable to do}

### Affected User Flows

| # | Flow | Impact | Affected Users |
|---|------|--------|---------------|
| 1 | {user flow} | {what broke / degraded} | {scope: all users / segment / %} |

### Technical Cause → UX Effect Mapping

| Technical Cause | UX Effect | Severity |
|----------------|-----------|----------|
| {code/system issue} | {what user experienced} | {CRITICAL/HIGH/MEDIUM} |

---

## Per-Perspective Analysis Summary

### {Perspective Name}
- **Scope**: {what this perspective examined}
- **Findings**: {N} items (CRITICAL: {n}, HIGH: {n}, MEDIUM: {n})
- **Key Finding**: {1-2 sentence summary}
- **Verification Score**: {score} ({verdict})

---

## Cross-Perspective Analysis

- **Corroborated Root Causes**: Items where multiple perspectives independently identified the same cause
- **Conflicting Conclusions**: Items where perspectives reached different conclusions
- **Integrated Insights**: Issues only visible when combining perspectives

---

## Action Items

### Immediate Fixes (Prevent Recurrence)
- [ ] {specific action with code/system reference}

### Short-Term Improvements (Monitoring / Alerting / Testing)
- [ ] {specific improvement}

### Long-Term Considerations (Architecture / Process)
- [ ] {specific consideration}

---

## Confidence Summary (Socratic Verification)

| Perspective | Verification Rounds | Score | Verdict |
|-------------|-------------------|-------|---------|
| {name} | {rounds} | {score} | {verdict} |

---

## Appendix

### Referenced Documents
| # | Document Path | Related Perspective | Reference Count |
|---|--------------|-------------------|-----------------|

### Raw Evidence
{Key code snippets, log entries, or data points that support the root cause determination}
