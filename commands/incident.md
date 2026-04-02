---
description: "Run Prism incident RCA analysis"
---

Use `PRISM_REPO_PATH` as the source of truth for shared Prism assets when it points to a Prism repo containing `skills/incident/SKILL.md`.
If that path is unavailable, fall back to `Glob(pattern="**/skills/incident/SKILL.md")` to locate the shared Prism incident skill.
Resolve the shared incident report template and perspective injection assets from that same Prism asset root rather than from the caller's working directory.
Preserve the shared incident wrapper flow end to end: incident intake, screenshot extraction, report-language selection, state setup, `prism_analyze` dispatch, polling, and final `prism_analyze_result` retrieval.
Read the resolved shared skill with the Read tool and follow its instructions exactly.
