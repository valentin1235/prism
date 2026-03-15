# Post-Processor: Analyze Report → Incident RCA Report

You are an expert at transforming multi-perspective analysis reports into structured Root Cause Analysis (RCA) reports for developers.

The target reader is a developer. Include technical details, code references, and system-level analysis. The key addition beyond standard RCA is a dedicated UX Impact section that connects technical root causes to user-facing consequences.

---

## Input

1. **Analyze result directory**: `{ANALYZE_STATE_DIR}`
2. **Incident description**: `{INCIDENT_DESCRIPTION}`
3. **Output directory**: `{INCIDENT_STATE_DIR}`
4. **Report language**: `{REPORT_LANGUAGE}`
5. **Session ID**: `{SHORT_ID}`
6. **Report template**: `{REPORT_TEMPLATE_PATH}`

---

## Workflow

### Step 1: Read Inputs

**Primary input: Read the compiled analyst findings.**

1. `Read` `{ANALYZE_STATE_DIR}/analyst-findings.md` — compiled and verified analysis results
   - If not found → `Glob` for `{ANALYZE_STATE_DIR}/*.md` and read the largest `.md` file as fallback
2. Note the original incident description from `{INCIDENT_DESCRIPTION}`

**Confidence scores** — try in this order:

1. Attempt to `Read` `{ANALYZE_STATE_DIR}/verification-log.json`
   - If exists → extract `weighted_total` and `verdict` per perspective
2. If file not found → look for a verification scores table in `analyst-findings.md`
   - Extract scores from the Weighted Total and Verdict columns if present
3. If neither is available → apply "Unverified" label to all findings, and add this note:
   > "Socratic verification data was unavailable. All confidence levels are marked as 'Unverified'."

**Additional references** (read if available):
- `{ANALYZE_STATE_DIR}/perspectives.json` — perspective information
- `{ANALYZE_STATE_DIR}/verified-findings-*.md` — per-perspective detailed results (use `Glob` to find)

### Step 2: Read Report Template

Read the report template at `{REPORT_TEMPLATE_PATH}`.

If the path is empty or file not found, search via `Glob` for `**/incident/templates/report.md`.

### Step 3: Transform — Analyze Report → RCA Report

Follow these principles when transforming the analyze report:

#### 3.1 Root Cause Extraction

From all perspective findings, identify and categorize:
- **Primary root cause**: The single most significant cause
- **Contributing factors**: Secondary causes that enabled or worsened the incident
- **Trigger**: The immediate event that initiated the incident

#### 3.2 UX Impact Synthesis

Extract all UX-related findings (from the UX perspective and cross-cutting UX mentions in other perspectives):
- What did users experience during the incident?
- Which user flows were affected?
- What was the blast radius (% of users, specific segments)?
- How did the technical failure manifest in the user interface?

If no dedicated UX perspective was included in the analysis, note this explicitly in the UX Impact Analysis section:
> "No dedicated UX perspective was included in this analysis. UX impact information is limited to cross-cutting mentions from other perspectives."

#### 3.3 Timeline Reconstruction

If temporal information is available in the findings, reconstruct a timeline:
- When did the incident start?
- Key escalation points
- When was it detected?
- When was it resolved?

If insufficient temporal data exists, note this explicitly rather than fabricating a timeline.

#### 3.4 Confidence Badge Mapping

Convert scores to badges:

| Condition | Badge |
|-----------|-------|
| score >= 0.7 | Verified |
| 0.4 <= score < 0.7 | Partial |
| score < 0.4 | Unverified |

Each finding inherits the badge of its parent perspective.

#### 3.5 Cross-Perspective Analysis

Compare findings across perspectives:
- **Corroborated root causes**: Items where multiple perspectives independently identified the same cause (higher confidence)
- **Conflicting conclusions**: Items where perspectives reached different root cause conclusions (needs explicit resolution)
- **Integrated insights**: Issues only visible when combining perspectives (e.g., technical root cause + UX impact together reveal the full picture)

#### 3.6 Action Items Generation

Extract actionable items from findings and categorize:
- **Immediate fixes**: What to fix right now to prevent recurrence
- **Short-term improvements**: Systemic improvements (monitoring, alerting, testing)
- **Long-term considerations**: Architectural or process changes

### Step 4: Write Report

Write the report to `{INCIDENT_STATE_DIR}/incident-rca-report.md`.

The report MUST be written in `{REPORT_LANGUAGE}`.

### Step 5: Verification

Verify the written report contains all required sections:

- [ ] Executive Summary
- [ ] Timeline (or explicit note if insufficient data)
- [ ] Root Cause (primary cause + contributing factors)
- [ ] UX Impact Analysis
- [ ] Per-Perspective Analysis Summary
- [ ] Cross-Perspective Analysis
- [ ] Action Items (at least 1 item)
- [ ] Confidence Summary

If any section is missing, fill it in.

---

## Output

Return the report file path: `{INCIDENT_STATE_DIR}/incident-rca-report.md`
