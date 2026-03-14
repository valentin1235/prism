# PRD Policy Analysis Report

This template produces a report for PMs (product managers). Minimize technical jargon and focus on policy conflicts and actionable decision items.

---

## Executive Summary

[2-3 sentences: what PRD was analyzed, key findings, immediate actions PM should take]

## Analysis Overview

- **Target**: {PRD title}
- **Analysis Date**: {date}
- **Method**: {N}-perspective multi-perspective analysis + Socratic verification
- **Reference Docs**: {number of ontology documents} policy documents referenced
- **Perspectives Used**: {list of perspectives}

---

## PM Decision Checklist

Items that PM must decide before proceeding with this PRD. Sorted by priority.

| # | Item | Severity | Confidence | Decision |
|---|------|----------|------------|----------|
| 1 | {decision item} | {CRITICAL/HIGH/MEDIUM} | {Verified/Partial/Unverified} | - [ ] |
| 2 | ... | ... | ... | - [ ] |

> **Severity Criteria**
> - **CRITICAL**: Direct conflict with existing policy. Cannot proceed without resolution
> - **HIGH**: Multiple interpretations possible. Clear definition needed
> - **MEDIUM**: Undefined edge case. Policy supplement recommended

> **Confidence Criteria** (based on Socratic verification results)
> - ✅ **Verified**: Code/document evidence confirmed (score >= 0.7)
> - ⚠️ **Partial**: Some evidence confirmed, additional verification recommended (0.4 <= score < 0.7)
> - ❌ **Unverified**: Insufficient evidence, must verify (score < 0.4)

---

## Policy Conflict Details

Detailed description of each conflict/ambiguity found.

### [{Severity}-{N}] {Conflict Title}

**PRD states**: {what the PRD defines}

**Existing policy states**: {what existing policy documents define} (source: {document:section})

**Conflict/Ambiguity**: {explanation of why this is a problem, written for PM comprehension}

**PM decisions needed**:
- [ ] {specific decision item 1}
- [ ] {specific decision item 2}

**Confidence**: {✅ Verified | ⚠️ Partial | ❌ Unverified}

---

## PRD Internal Ambiguities

Ambiguous or self-contradictory parts found within the PRD itself. These are internal PRD issues, not conflicts with existing policies.

| # | Location | Ambiguous Content | Why It's a Problem | Suggestion |
|---|----------|-------------------|-------------------|------------|

---

## Per-Perspective Analysis Summary

Summary of key findings from each analysis perspective.

### {Perspective Name}
- **Scope**: {policy domain this perspective examined}
- **Findings**: {N} items (CRITICAL: {n}, HIGH: {n}, MEDIUM: {n})
- **Key Finding**: {1-2 sentence summary}
- **Verification Score**: {score} ({verdict})

---

## Cross-Perspective Analysis

- **Common Findings**: Items where multiple perspectives independently identified the same conflict
- **Cross-Perspective Conflicts**: Items where perspectives reached different conclusions
- **Integrated Insights**: Issues only visible when combining perspectives

---

## Confidence Summary (Socratic Verification)

| Perspective | Verification Rounds | Score | Verdict |
|-------------|-------------------|-------|---------|

### Ambiguities Resolved During Verification
{Items clarified through Socratic Q&A}

### Unresolved Ambiguities
{Items not resolved even through verification — requires additional PM confirmation}

---

## Recommendations

### Immediate Actions (Required before PRD revision)
1. {specific recommendation for CRITICAL items}

### Short-Term Actions (Recommended before PRD finalization)
1. {specific recommendation for HIGH items}

### Long-Term Considerations (Policy supplement level)
1. {recommendation for MEDIUM items or policy gaps}

---

## Appendix

### Referenced Policy Documents
| # | Document Path | Related Perspective | Reference Count |
|---|--------------|-------------------|-----------------|

### Ontology Scope Mapping
| Perspective | Mapped Policy Documents | Selection Rationale |
|-------------|----------------------|-------------------|
