# Clarity Enforcement Rules

Common quality enforcement rules for analysis output. The lead MUST reject and return analyst findings that match these patterns.

## Parameters

| Placeholder | Description | Examples |
|-------------|-------------|---------|
| `{EVIDENCE_FORMAT}` | Required evidence citation format | `"file:function:line"` / `"concrete evidence or reasoning"` / `"filename:section"` |

---

## Enforcement Table

| Pattern Found | Required Response |
|---------------|------------------|
| "probably", "might", "seems like" | "Cite specific evidence. What {EVIDENCE_FORMAT} supports this?" |
| Unsupported claims | "INCOMPLETE: Provide {EVIDENCE_FORMAT} as evidence." |
| Scope drift | "Stay within your perspective scope: {scope}. Redirect to {correct-analyst}." |
| Missing key question answers | "Key question unanswered: {question}. Address before completing." |
| Cross-analyst conflicts | Route to both analysts + DA for resolution |
| Unaddressed DA challenges | Forward to analyst, REQUIRE evidence-based response |
