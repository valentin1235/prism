---
description: "Run Prism PRD policy analysis"
---

Use `PRISM_REPO_PATH` as the source of truth for shared Prism assets when it points to a Prism repo containing `skills/prd/SKILL.md`.
If that path is unavailable, fall back to `Glob(pattern="**/skills/prd/SKILL.md")` to locate the shared Prism PRD skill.
Resolve the shared PRD report template, post-processor prompt, and analyze handoff assets from that same Prism asset root rather than from the caller's working directory.
Preserve the shared PRD wrapper flow end to end: PRD path validation, shared state setup, analyze-config creation, `psm analyze --config ...` delegation, post-processing, `PM Decision Checklist` verification, and final report copy-back beside the PRD file.
Keep the existing invocation and artifact contract unchanged: write the analyze handoff config to `~/.prism/state/prd-{short-id}/analyze-config.json`, require the post-processor to write and return `~/.prism/state/prd-{short-id}/prd-policy-review-report.md`, then copy that report to `{PRD_DIR}/prd-policy-review-report.md` while also surfacing `~/.prism/state/analyze-{short-id}/` for raw analyze artifacts.
Read the resolved shared skill with the Read tool and follow its instructions exactly.
