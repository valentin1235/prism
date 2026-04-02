---
description: "Run Prism multi-perspective analysis"
---

Use `PRISM_REPO_PATH` as the source of truth for shared Prism assets when it points to a Prism repo containing `skills/analyze/SKILL.md`.
If that path is unavailable, fall back to `Glob(pattern="**/skills/analyze/SKILL.md")` to locate the shared Prism analyze skill.
Preserve the full shared analyze decision flow encoded in that skill, including config intake, argument fallback, `prism_analyze` dispatch, polling, cancellation, and result retrieval.
Pass shared analyze config paths such as `report_template` through unchanged when they already point at Prism assets.
Read the resolved shared skill with the Read tool and follow its instructions exactly.
