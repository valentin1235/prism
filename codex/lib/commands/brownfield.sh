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
Use `Glob(pattern="**/skills/brownfield/SKILL.md")` to locate the shared Prism brownfield skill.
Read the first match.
Follow that shared skill exactly.
EOF
}

prism_psm_brownfield_skill_normalization() {
  cat <<'EOF'
Normalize the command prefix from the Claude Code form to Codex:
  `/prism:brownfield` -> `psm brownfield`
Reuse Prism's bundled MCP tools and shared skill assets.
Do not reimplement the workflow locally in this Codex wrapper.
EOF
}

prism_psm_brownfield_command_entrypoint() {
  cat <<'EOF'
Use `PRISM_REPO_PATH` as the source of truth for shared Prism assets when it points to a Prism repo containing `skills/brownfield/SKILL.md`.
If that path is unavailable, fall back to `Glob(pattern="**/skills/brownfield/SKILL.md")` to locate the shared Prism brownfield skill.
Treat `psm brownfield`, `psm brownfield scan`, `psm brownfield defaults`, and `psm brownfield set <indices>` as exact command forms routed through that shared skill.
Preserve the default no-argument flow exactly: scan first, render the scan result, then prompt for default selection.
Read the resolved shared skill with the Read tool and follow its instructions exactly.
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
    "For psm brownfield, preserve full Claude-skill execution parity for arguments, defaults, and environment assumptions:" \
    '- Treat the invocation as one of these exact shared-skill forms: `psm brownfield`, `psm brownfield scan`, `psm brownfield defaults`, or `psm brownfield set <indices>`.' \
    "- Use the shared Prism brownfield skill at \`${PRISM_REPO_PATH}/skills/brownfield/SKILL.md\` as the workflow source of truth." \
    "- Preserve the default no-argument flow exactly: scan first, render the scan result, then prompt for default selection." \
    '- Preserve the shared-skill subcommand behavior exactly: `scan` means scan only, `defaults` means show current defaults, and `set <indices>` means update defaults directly with the provided comma-separated indices.' \
    "- Preserve the shared skill's user-facing status text and stop conditions: empty scans should surface \`No GitHub repositories found in your home directory.\`, clearing defaults should surface the shared greenfield-mode confirmation, and successful default updates should confirm the selected repository names." \
    "- Invalid brownfield selections or MCP failures must fail the Codex run instead of being converted into a success summary." \
    "- Reuse Prism's bundled MCP tool routing and environment assumptions from the shared skill. Do not replace the workflow with ad hoc shell logic."
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
