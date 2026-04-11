#!/usr/bin/env bash

prism_psm_brownfield_usage() {
  cat <<'EOF'
psm brownfield
psm brownfield scan
psm brownfield defaults
psm brownfield set 6,18,19
EOF
}

prism_psm_brownfield_skill_dispatch() {
  cat <<'EOF'
Resolve the shared Prism asset root deterministically:
  `PRISM_REPO_PATH` when it points to a Prism repo containing `skills/brownfield/SKILL.md`; otherwise fall back to `Glob(pattern="**/skills/brownfield/SKILL.md")`.
Read the resolved shared Prism brownfield skill.
Follow that shared skill exactly.
EOF
}

prism_psm_brownfield_skill_normalization() {
  cat <<'EOF'
Normalize the command prefix from the Claude Code form to Codex:
  `/prism:brownfield` -> `psm brownfield`
Reuse Prism's bundled MCP tools and shared skill assets.
Treat managed `~/.codex/skills/prism-brownfield` as a setup-refreshed mirror of the repo `skills/brownfield` source, not as an independently authored workflow.
Do not assume the command was launched from within `~/prism` or from the user's current working directory.
Do not reimplement the workflow locally in this Codex wrapper.
EOF
}

prism_psm_brownfield_command_entrypoint() {
  cat <<'EOF'
Use `PRISM_REPO_PATH` as the source of truth for shared Prism assets when it points to a Prism repo containing `skills/brownfield/SKILL.md`.
If that path is unavailable, fall back to `Glob(pattern="**/skills/brownfield/SKILL.md")` to locate the shared Prism brownfield skill.
Treat `psm brownfield`, `psm brownfield scan`, `psm brownfield defaults`, and `psm brownfield set <indices>` as exact command forms routed through that shared skill.
Treat the shared brownfield skill as the only workflow definition.
Read and follow that shared Prism skill from the resolved Prism asset root.
EOF
}

prism_psm_brownfield_asset_paths() {
  cat <<'EOF'
skills/brownfield/SKILL.md
EOF
}

prism_psm_brownfield_command_contract() {
  printf '%s\n' \
    "" \
    "For psm brownfield, resolve the shared brownfield workflow independently from the launch directory without redefining it in this Codex command layer:" \
    '- Treat the invocation as one of these exact shared-skill forms: `psm brownfield`, `psm brownfield scan`, `psm brownfield defaults`, or `psm brownfield set <indices>`.' \
    "- Use the shared Prism brownfield skill at \`${PRISM_REPO_PATH}/skills/brownfield/SKILL.md\` as the workflow source of truth." \
    "- Resolve the shared Prism brownfield skill deterministically from \`${PRISM_REPO_PATH}\`, the installed \`repo-root\` pointer, or the shared \`psm\` library location before considering any globbed matches." \
    "- If Codex does need to glob for \`skills/brownfield/SKILL.md\`, prefer the Prism-owned path under \`${PRISM_REPO_PATH}\` over matches from the user's target repository or working directory." \
    "- Treat installed \`~/.codex/skills/prism-brownfield\` entries as setup-refreshed mirrors of the shared repo skill, not as the authored workflow source." \
    "- Treat that shared skill as the only workflow definition; do not duplicate its phase logic, status text, or stop conditions in the Codex command layer." \
    "- Reuse Prism's bundled MCP tool routing and environment assumptions from the shared skill. Keep the user's original working directory as project context, but resolve shared Prism workflow assets from \`${PRISM_REPO_PATH}\`." \
    "- Invalid brownfield selections or MCP failures must fail the Codex run instead of being converted into a success summary." \
    "- Do not replace the workflow with ad hoc shell logic."
}

prism_psm_define_command_config "brownfield" "shared_skill_relative_path" "skills/brownfield/SKILL.md"
prism_psm_define_command_config "brownfield" "skill_title" "brownfield repository scan and default-management workflow"
prism_psm_define_command_config "brownfield" "skill_description" "Scan and manage brownfield repository defaults for interviews through the shared Prism brownfield skill."
prism_psm_define_command_config "brownfield" "command_description" "Run Prism brownfield repository setup"
prism_psm_define_command_config "brownfield" "usage_function" "prism_psm_brownfield_usage"
prism_psm_define_command_config "brownfield" "skill_dispatch_function" "prism_psm_brownfield_skill_dispatch"
prism_psm_define_command_config "brownfield" "skill_normalization_function" "prism_psm_brownfield_skill_normalization"
prism_psm_define_command_config "brownfield" "command_entrypoint_function" "prism_psm_brownfield_command_entrypoint"
prism_psm_define_command_config "brownfield" "asset_paths_function" "prism_psm_brownfield_asset_paths"
prism_psm_define_command_config "brownfield" "contract_function" "prism_psm_brownfield_command_contract"
