---
description: "Run Prism brownfield repository setup"
---

Use `PRISM_REPO_PATH` as the source of truth for shared Prism assets when it points to a Prism repo containing `skills/brownfield/SKILL.md`.
If that path is unavailable, fall back to `Glob(pattern="**/skills/brownfield/SKILL.md")` to locate the shared Prism brownfield skill.
Treat `psm brownfield`, `psm brownfield scan`, `psm brownfield defaults`, and `psm brownfield set <indices>` as exact command forms routed through that shared skill.
Preserve the default no-argument flow exactly: scan first, render the scan result, then prompt for default selection.
Read the resolved shared skill with the Read tool and follow its instructions exactly.
