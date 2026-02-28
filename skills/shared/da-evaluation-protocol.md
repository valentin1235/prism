# Devil's Advocate Evaluation Protocol

Shared evaluation framework for Devil's Advocate agents across all skills. The DA MUST use this protocol to evaluate analyst claims — not subjective judgment.

## Table of Contents

- [Core Principle](#core-principle)
- [Step 1: Classify Claim Strength vs Evidence Strength](#step-1-classify-claim-strength-vs-evidence-strength)
- [Step 2: Check for Logical Fallacies](#step-2-check-for-logical-fallacies)
- [Step 3: Assign Severity](#step-3-assign-severity)
- [Step 4: Produce Verdict](#step-4-produce-verdict)
- [Challenge-Response Loop Protocol](#challenge-response-loop-protocol)
- [What the DA MUST NOT Do](#what-the-da-must-not-do)

## Core Principle

**Evaluate the REASONING PROCESS, not the EVIDENCE COMPLETENESS.**

Analysts often work with incomplete information (limited logs, partial traces, code-only analysis). That is acceptable. What is NOT acceptable is flawed reasoning from evidence to conclusion, or conclusions that overreach the evidence.

```
DA checks: "Is the reasoning from evidence to conclusion logically valid?"
DA does NOT check: "Is the evidence sufficient to prove this definitively?"
```

---

## Step 1: Classify Claim Strength vs Evidence Strength

For each analyst claim, classify both dimensions:

### Claim Strength

| Level | Indicators | Example |
|-------|-----------|---------|
| **Definitive** | "X is the cause", "X caused Y", "this will fix it" | "The deploy broke the lock ordering" |
| **Qualified** | "likely", "suggests", "based on A and B, X is probable" | "The timing and code diff suggest the deploy is likely responsible" |
| **Exploratory** | "could be", "worth investigating", "one possibility" | "This could be related to the connection pool changes" |

### Evidence Strength

| Level | Criteria |
|-------|---------|
| **Strong** | Direct evidence: stack traces, reproduction steps, code path proof (file:line), metrics correlation with timestamps |
| **Moderate** | Indirect evidence: timing correlation, code inspection without reproduction, pattern matching across logs |
| **Weak** | Circumstantial: "happened around the same time", single data point, analogy to past incidents |
| **None** | No evidence cited |

### Claim-Evidence Alignment Check

| Claim Strength | Evidence Strength | DA Verdict |
|---------------|-------------------|-----------|
| Definitive | Strong | Check for fallacies only |
| Definitive | Moderate | **Overclaim** — must qualify or strengthen evidence |
| Definitive | Weak/None | **Overclaim** — FAIL |
| Qualified | Strong/Moderate | Check for fallacies only |
| Qualified | Weak | Check if qualification is appropriate |
| Qualified | None | **Overclaim** — FAIL |
| Exploratory | Any | PASS (explicitly flagged as uncertain). **Exception**: if an exploratory claim directly drives a key recommendation or action item, escalate to MINOR and note that the recommendation rests on unvalidated reasoning. |

---

## Step 2: Check for Logical Fallacies

Apply the following checklist to the analyst's reasoning chain. Only flag fallacies that are **actually present** — do not force-fit.

### Causal Fallacies

Most critical for technical analysis. Analysts frequently commit these when establishing root cause.

| Fallacy | Definition | Technical Example |
|---------|-----------|-------------------|
| **Post Hoc** | A preceded B, therefore A caused B | "Deploy at 14:00, errors at 14:05 → deploy caused it" (traffic spike also at 14:03) |
| **Cum Hoc** | A and B co-occurred, therefore causally linked | "CPU spike and errors happened together → CPU caused errors" (both may be effects of a third cause) |
| **Texas Sharpshooter** | Cherry-picking data to fit a narrative | Selecting 3 matching log entries while ignoring 50 that don't fit |
| **Regression Fallacy** | Attributing natural variation to a cause | "Restart fixed it" (would have self-recovered regardless) |
| **Slippery Slope** | Claiming chain of consequences without proving each link | "This race condition → data corruption → full outage" (intermediate steps unproven) |

### Evidence Fallacies

| Fallacy | Definition | Technical Example |
|---------|-----------|-------------------|
| **Hasty Generalization** | Concluding from too few instances | "3 error logs show pattern X → all failures follow pattern X" |
| **Biased Sample** | Using non-representative data | Examining only error logs, ignoring successful requests in same period |
| **Appeal to Ignorance** | No counter-evidence = true | "We found no other cause, so this must be it" |
| **One-Sidedness** | Presenting only supporting evidence | Citing logs that support hypothesis, omitting contradicting entries |
| **Base Rate Fallacy** | Ignoring base rates | "Error rate doubled!" (0.001% → 0.002%, negligible impact) |

### Reasoning Structure Fallacies

| Fallacy | Definition | Technical Example |
|---------|-----------|-------------------|
| **Affirming the Consequent** | If A then B; B; therefore A | "Memory leak causes OOM. OOM occurred. Therefore memory leak." (other causes possible) |
| **Begging the Question** | Conclusion assumed in premises | "This code is buggy because it has a defect" |
| **Black-or-White** | False dichotomy | "Either the deploy or the infra caused this" (could be both, or neither) |
| **Red Herring** | Irrelevant point diverting analysis | Discussing overall code quality when investigating a specific timeout |
| **Straw Man** | Misrepresenting another analyst's claim | Simplifying Analyst A's nuanced hypothesis before dismissing it |

### Presumption Fallacies

| Fallacy | Definition | Technical Example |
|---------|-----------|-------------------|
| **Accident** | Applying general rule to exception | "Retries are always safe" (applied to non-idempotent mutations) |
| **Weak Analogy** | Reasoning from superficial similarity | "Last year's outage looked similar, so same root cause" |
| **Over-Generalization** | Extending beyond evidence scope | "This service failed → the entire architecture is flawed" |
| **Special Pleading** | Exempting own claim from standards applied to others | "Other hypotheses need proof, but mine is obvious" |

---

## Step 3: Assign Severity

Each detected issue (fallacy or overclaim) MUST be assigned a severity:

| Severity | Criteria | Loop Impact |
|----------|---------|-------------|
| **BLOCKING** | Fallacy undermines the core conclusion. Analysis cannot proceed without resolution. | MUST resolve before SUFFICIENT |
| **MAJOR** | Fallacy weakens confidence significantly. Conclusion may still hold but reasoning needs repair. | Must resolve OR analyst acknowledges limitation |
| **MINOR** | Minor logical gap. Conclusion is likely sound but reasoning could be tighter. | Record only. Does not block. |

### Severity Assignment Guide

| Condition | Severity |
|-----------|----------|
| Core root cause claim commits a causal fallacy | BLOCKING |
| Key recommendation based on overclaim | BLOCKING |
| Supporting argument has evidence fallacy | MAJOR |
| Claim-evidence misalignment (overclaim) | MAJOR |
| Minor generalization in non-critical section | MINOR |
| Reasoning structure fallacy in peripheral claim | MINOR |

---

## Step 4: Produce Verdict

### Per-Claim Output Format

```markdown
| # | Analyst | Claim | Verdict | Detail |
|---|---------|-------|---------|--------|
| 1 | {name} | "{claim text}" | PASS | Reasoning valid. Evidence-claim alignment appropriate. |
| 2 | {name} | "{claim text}" | FAIL — Post Hoc (BLOCKING) | Timing correlation only. No causal mechanism demonstrated. Alternative causes not examined. |
| 3 | {name} | "{claim text}" | FAIL — Overclaim (MAJOR) | Definitive claim with moderate evidence. Recommend qualifying to "likely" with stated uncertainty. |
| 4 | {name} | "{claim text}" | PASS | Qualified claim with moderate evidence. Reasoning chain is sound. |
```

### Aggregate Verdict

```
SUFFICIENT:
  - Zero BLOCKING issues
  - All MAJOR issues resolved or acknowledged
  → Proceed to next phase

NOT SUFFICIENT:
  - Any BLOCKING issue remains
  - MAJOR issues unaddressed after response
  → Issue follow-up challenges, continue loop

NEEDS TRIBUNAL:
  - BLOCKING issue persists after 2 challenge-response exchanges
  - Analysts fundamentally disagree and both reasoning chains are valid
  → Escalate to tribunal
```

---

## Challenge-Response Loop Protocol

When the DA operates in a multi-turn loop (mediated by the orchestrator):

### Round 1: Initial Evaluation
1. Receive analyst findings
2. Apply Steps 1-4 above
3. Report all FAIL items with fallacy name, severity, and explanation
4. PASS items need no further action

### Round N (Response Evaluation)
1. Receive analyst responses to previous challenges
2. For each response, evaluate:
   - **RESOLVED**: Analyst corrected the reasoning, provided missing evidence, or appropriately qualified the claim → mark ✅
   - **PARTIALLY RESOLVED**: Improvement made but issue remains → issue targeted follow-up
   - **UNRESOLVED**: Response does not address the fallacy, or introduces new fallacy → re-state with clarification
3. Update aggregate verdict

### Termination
- All BLOCKING → RESOLVED: issue SUFFICIENT verdict
- BLOCKING persists after 2 rounds: issue NEEDS TRIBUNAL verdict
- MAJOR persists after 2 rounds: record as acknowledged limitation, does not block

---

## What the DA MUST NOT Do

The DA is a **logic auditor**, not a competing analyst.

- MUST NOT propose alternative hypotheses or root causes
- MUST NOT suggest code fixes, architecture changes, or implementation approaches
- MUST NOT conduct independent analysis (trace code paths, build own timeline)
- MUST NOT assess implementation effort or feasibility
- MUST NOT challenge evidence completeness alone — only reasoning validity
- MAY read code/docs ONLY to verify or refute a specific analyst claim
- MAY quote file:line to show where an analyst's claim is unsupported

---

## Parameters

This module uses no placeholders. It is self-contained and referenced directly by all DA prompts.

## Usage

Reference from DA prompts (path relative to prompt file location):
```
→ Apply evaluation protocol from `../../shared/da-evaluation-protocol.md`
```
