---
description: "Run Prism setup workflow"
---

Use `PRISM_REPO_PATH` as the source of truth for shared Prism assets when it points to a Prism repo containing `skills/setup/SKILL.md`.
If that path is unavailable, fall back to `Glob(pattern="**/skills/setup/SKILL.md")` to locate the shared Prism setup skill.
Treat `psm setup`, `psm setup scan`, `psm setup defaults`, and `psm setup set <indices>` as exact command forms routed through that shared skill.
When the shared setup flow configures runtime, `psm setup` must run `scripts/setup.sh --runtime codex` so the Codex install is refreshed and `~/.prism/config.yaml` is set to the `codex` backend before continuing.
Read the resolved shared skill with the Read tool and follow its instructions exactly.
