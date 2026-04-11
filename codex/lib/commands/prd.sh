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
EOF
}

prism_psm_prd_skill_normalization() {
  cat <<'EOF'
Normalize the command prefix from the Claude Code form to Codex:
  `/prism:prd` -> `psm prd`
Normalize analyze delegation accordingly:
  `Skill(skill="prism:analyze", args="...")` -> `psm analyze ...`
Resolve the PRD input path from the user's launch directory, not from the shared Prism repo, and fail if the referenced file does not exist.
Treat the shared PRD skill as the only workflow definition.
When the shared PRD skill resolves files relative to its own `SKILL.md`, bind `SKILL_DIR` to `PRISM_REPO_PATH/skills/prd`.
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
Treat the shared PRD skill as the only workflow definition.
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
    "For psm prd, preserve shared-skill execution parity while resolving shared assets independently from the launch directory:" \
    "- Use the shared Prism PRD skill at \`${PRISM_REPO_PATH}/skills/prd/SKILL.md\` as the workflow source of truth." \
    "- Treat that shared skill as the only workflow definition; do not duplicate its phase logic in the Codex command layer." \
    "- When the shared PRD skill refers to files relative to its own \`SKILL.md\`, resolve them from \`${PRISM_REPO_PATH}/skills/prd\`." \
    "- Required shared PRD support files for this invocation remain:" \
    "  \`${PRISM_REPO_PATH}/skills/prd/prompts/post-processor.md\`" \
    "  \`${PRISM_REPO_PATH}/skills/prd/templates/report.md\`" \
    "  \`${PRISM_REPO_PATH}/skills/analyze/SKILL.md\`" \
    "- Resolve shared PRD prompts, templates, and analyze handoff assets from \`${PRISM_REPO_PATH}\`, not from the user's working directory." \
    "- The wrapper has already exported \`PRISM_REPO_PATH=${PRISM_REPO_PATH}\` and \`PRISM_TARGET_CWD=${PRISM_TARGET_CWD}\` before launching Codex; keep using that split between shared Prism assets and the user's project context." \
    "- Do not assume the command was launched from within \`~/prism\`; the original working directory is only the user project context." \
    "- Preserve the shared analyze delegation contract exactly when the PRD flow hands off to \`psm analyze\`." \
    "- Reuse Prism's bundled MCP, prompt, and template assets from the shared skill tree without reimplementing the PRD workflow."
}

prism_psm_define_command_config "prd" "shared_skill_relative_path" "skills/prd/SKILL.md"
prism_psm_define_command_config "prd" "skill_title" "PRD policy analysis flow"
prism_psm_define_command_config "prd" "skill_description" "Run Prism PRD policy analysis through the shared PRD skill and Codex runtime adapter."
prism_psm_define_command_config "prd" "command_description" "Run Prism PRD policy analysis"
prism_psm_define_command_config "prd" "usage_function" "prism_psm_prd_usage"
prism_psm_define_command_config "prd" "skill_dispatch_function" "prism_psm_prd_skill_dispatch"
prism_psm_define_command_config "prd" "skill_normalization_function" "prism_psm_prd_skill_normalization"
prism_psm_define_command_config "prd" "command_entrypoint_function" "prism_psm_prd_command_entrypoint"
prism_psm_define_command_config "prd" "asset_paths_function" "prism_psm_prd_asset_paths"
prism_psm_define_command_config "prd" "contract_function" "prism_psm_prd_command_contract"
prism_psm_define_command_config "prd" "prompt_function" "prism_psm_prd_bridge_prompt"
