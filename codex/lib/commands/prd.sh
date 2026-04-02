#!/usr/bin/env bash

prism_psm_prd_usage() {
  cat <<'EOF'
psm prd /path/to/prd.md
EOF
}

prism_psm_prd_skill_dispatch() {
  cat <<'EOF'
Resolve the shared Prism asset root deterministically:
  `PRISM_REPO_PATH` when it points to a Prism repo containing `skills/prd/SKILL.md`; otherwise fall back to `Glob(pattern="**/skills/prd/SKILL.md")`.
Read the resolved shared Prism PRD skill.
Follow that shared skill exactly.
Preserve the shared PRD wrapper phases exactly: PRD path intake, shared session/state setup, analyze config creation, analyze delegation, post-processing, report verification, and final report delivery.
EOF
}

prism_psm_prd_skill_normalization() {
  cat <<'EOF'
Normalize the command prefix from the Claude Code form to Codex:
  `/prism:prd` -> `psm prd`
Normalize analyze delegation accordingly:
  `Skill(skill="prism:analyze", args="...")` -> `psm analyze ...`
Resolve the PRD input path from the user's launch directory, not from the shared Prism repo, and fail if the referenced file does not exist.
Reuse the shared session id across `~/.prism/state/prd-{short-id}` and `~/.prism/state/analyze-{short-id}` exactly as the shared skill specifies.
Preserve the shared analyze-config contract exactly, including `topic`, `input_context`, `seed_hints`, and `session_id`, and keep the generated config at `~/.prism/state/prd-{short-id}/analyze-config.json`.
Preserve the shared analyze handoff format exactly: invoke analyze as `psm analyze --config ~/.prism/state/prd-{short-id}/analyze-config.json` and treat that config file as the handoff artifact consumed by downstream Prism workflows.
Require the shared analyze output contract before post-processing: `~/.prism/state/analyze-{short-id}/analyst-findings.md` must exist, while a missing `verification-log.json` remains tolerated.
When the shared PRD skill resolves files relative to its own `SKILL.md`, bind `SKILL_DIR` to `PRISM_REPO_PATH/skills/prd` and keep the post-processor prompt at `PRISM_REPO_PATH/skills/prd/prompts/post-processor.md`.
Keep the PRD report template at `PRISM_REPO_PATH/skills/prd/templates/report.md` and the analyze handoff assets under `PRISM_REPO_PATH/skills/analyze/...`.
Preserve the shared post-processing and output gates exactly: wait for the post-processor result, require `~/.prism/state/prd-{short-id}/prd-policy-review-report.md`, require the post-processor to return that same report file path as its handoff result, verify the report contains `PM Decision Checklist`, then copy the final report beside the PRD file as `{PRD_DIR}/prd-policy-review-report.md`.
Preserve the final delivery format exactly: report the copied PRD-side path first, the state-directory report path second, and the raw analyze artifacts directory as `~/.prism/state/analyze-{short-id}/`.
Reuse Prism's bundled MCP tools, prompts, and templates from the shared skill directory.
Do not assume the command was launched from within `~/prism` or from the user's current working directory.
Do not reimplement or paraphrase the Prism PRD workflow in this Codex wrapper.
EOF
}

prism_psm_prd_command_entrypoint() {
  cat <<'EOF'
Use `PRISM_REPO_PATH` as the source of truth for shared Prism assets when it points to a Prism repo containing `skills/prd/SKILL.md`.
If that path is unavailable, fall back to `Glob(pattern="**/skills/prd/SKILL.md")` to locate the shared Prism PRD skill.
Resolve the shared PRD report template, post-processor prompt, and analyze handoff assets from that same Prism asset root rather than from the caller's working directory.
Preserve the shared PRD wrapper flow end to end: PRD path validation, shared state setup, analyze-config creation, `psm analyze --config ...` delegation, post-processing, `PM Decision Checklist` verification, and final report copy-back beside the PRD file.
Keep the existing invocation and artifact contract unchanged: write the analyze handoff config to `~/.prism/state/prd-{short-id}/analyze-config.json`, require the post-processor to write and return `~/.prism/state/prd-{short-id}/prd-policy-review-report.md`, then copy that report to `{PRD_DIR}/prd-policy-review-report.md` while also surfacing `~/.prism/state/analyze-{short-id}/` for raw analyze artifacts.
Read the resolved shared skill with the Read tool and follow its instructions exactly.
EOF
}

prism_psm_prd_asset_paths() {
  cat <<'EOF'
skills/prd/SKILL.md
skills/prd/prompts/post-processor.md
skills/prd/templates/report.md
EOF
  prism_psm_analyze_asset_paths
}

prism_psm_prd_command_contract() {
  printf '%s\n' \
    "" \
    "For psm prd, preserve full shared-skill execution parity while resolving shared assets independently from the launch directory:" \
    "- Use the shared Prism PRD skill at \`${PRISM_REPO_PATH}/skills/prd/SKILL.md\` as the workflow source of truth." \
    "- Treat that shared skill as the PRD workflow entrypoint for Codex, including PRD intake, session setup, analyze-config generation, analyze delegation, post-processing, report verification, and final report delivery." \
    "- When the shared PRD skill refers to files relative to its own \`SKILL.md\`, resolve them from \`${PRISM_REPO_PATH}/skills/prd\`." \
    "- Required shared PRD support files for this invocation are:" \
    "  \`${PRISM_REPO_PATH}/skills/prd/prompts/post-processor.md\`" \
    "  \`${PRISM_REPO_PATH}/skills/prd/templates/report.md\`" \
    "  \`${PRISM_REPO_PATH}/skills/analyze/SKILL.md\`" \
    "  \`${PRISM_REPO_PATH}/skills/analyze/templates/report.md\`" \
    "- Resolve shared PRD prompts, templates, and analyze handoff assets from \`${PRISM_REPO_PATH}\`, not from the user's working directory." \
    "- The wrapper has already exported \`PRISM_REPO_PATH=${PRISM_REPO_PATH}\` and \`PRISM_TARGET_CWD=${PRISM_TARGET_CWD}\` before launching Codex; keep using that split between shared Prism assets and the user's project context." \
    "- Do not assume the command was launched from within \`~/prism\`; the original working directory is only the user project context." \
    "- Preserve the shared analyze delegation contract exactly when the PRD flow hands off to \`psm analyze\`." \
    "- Reuse Prism's bundled MCP, prompt, and template assets from the shared skill tree without reimplementing the PRD workflow."
}

prism_psm_define_command_config "prd" "shared_skill_relative_path" "skills/prd/SKILL.md"
prism_psm_define_command_config "prd" "skill_title" "PRD policy analysis flow"
prism_psm_define_command_config "prd" "skill_description" "PRD policy conflict analysis for Codex. Thin wrapper over the shared Prism PRD skill with Codex command normalization."
prism_psm_define_command_config "prd" "skill_version" "1.0.0"
prism_psm_define_command_config "prd" "command_description" "Run Prism PRD policy analysis"
prism_psm_define_command_config "prd" "usage_function" "prism_psm_prd_usage"
prism_psm_define_command_config "prd" "skill_dispatch_function" "prism_psm_prd_skill_dispatch"
prism_psm_define_command_config "prd" "skill_normalization_function" "prism_psm_prd_skill_normalization"
prism_psm_define_command_config "prd" "command_entrypoint_function" "prism_psm_prd_command_entrypoint"
prism_psm_define_command_config "prd" "asset_paths_function" "prism_psm_prd_asset_paths"
prism_psm_define_command_config "prd" "contract_function" "prism_psm_prd_command_contract"
prism_psm_define_command_config "prd" "prompt_function" "prism_psm_prd_bridge_prompt"
