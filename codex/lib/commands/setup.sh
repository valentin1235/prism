#!/usr/bin/env bash

prism_psm_setup_usage() {
  cat <<'EOF'
psm setup
psm setup scan
psm setup defaults
psm setup set 6,18,19
EOF
}

prism_psm_setup_skill_dispatch() {
  cat <<'EOF'
Resolve the shared Prism asset root deterministically:
  `PRISM_REPO_PATH` when it points to a Prism repo containing `skills/setup/SKILL.md`; otherwise fall back to `Glob(pattern="**/skills/setup/SKILL.md")`.
Read the resolved shared Prism setup skill.
Follow that shared skill exactly.
EOF
}

prism_psm_setup_skill_normalization() {
  cat <<'EOF'
Normalize the command prefix from the Claude Code form to Codex where referenced:
  `/prism:setup` -> `psm setup`
  `/prism:brownfield` -> `psm brownfield`
Reuse Prism's bundled MCP tools and shared skill content exactly.
When the shared setup skill configures Prism runtime, `psm setup` must run the shared `scripts/setup.sh --runtime codex` flow so the managed Codex installation artifacts and `~/.prism/config.yaml` stay aligned.
Treat managed `~/.codex/skills/prism-*` entries as setup-refreshed mirrors of the repo `skills/` source, not as independently authored workflow definitions.
Do not assume the command was launched from within `~/prism` or from the user's current working directory.
Do not reimplement or paraphrase the Prism setup workflow in this Codex wrapper.
EOF
}

prism_psm_setup_command_entrypoint() {
  cat <<'EOF'
Use `PRISM_REPO_PATH` as the source of truth for shared Prism assets when it points to a Prism repo containing `skills/setup/SKILL.md`.
If that path is unavailable, fall back to `Glob(pattern="**/skills/setup/SKILL.md")` to locate the shared Prism setup skill.
Treat `psm setup`, `psm setup scan`, `psm setup defaults`, and `psm setup set <indices>` as exact command forms routed through that shared skill.
When the shared setup flow configures runtime, `psm setup` must run `scripts/setup.sh --runtime codex` so the Codex install is refreshed and `~/.prism/config.yaml` is set to the `codex` backend before continuing.
Read the resolved shared skill with the Read tool and follow its instructions exactly.
EOF
}

prism_psm_setup_asset_paths() {
  cat <<'EOF'
skills/setup/SKILL.md
skills/brownfield/SKILL.md
scripts/install-codex.sh
scripts/configure-runtime.sh
scripts/setup.sh
EOF
}

prism_psm_setup_command_contract() {
  printf '%s\n' \
    "" \
    "For psm setup, preserve full shared-skill execution parity while resolving the shared setup workflow independently from the launch directory:" \
    "- Use the shared Prism setup skill at \`${PRISM_REPO_PATH}/skills/setup/SKILL.md\` as the workflow source of truth." \
    "- Resolve the shared Prism setup skill deterministically from \`${PRISM_REPO_PATH}\`, the installed \`repo-root\` pointer, or the shared \`psm\` library location before considering any globbed matches." \
    "- If Codex does need to glob for \`skills/setup/SKILL.md\`, prefer the Prism-owned path under \`${PRISM_REPO_PATH}\` over matches from the user's target repository or working directory." \
    "- Preserve the shared setup flow exactly, including the brownfield scan, default-selection prompt, MCP tool usage, and final confirmation messaging." \
    "- Preserve the shared runtime-configuration step exactly: in Codex, \`psm setup\` must run the shared \`scripts/setup.sh --runtime codex\` flow so the managed Codex install is refreshed and \`~/.prism/config.yaml\` is written with \`runtime.backend: codex\` before continuing." \
    "- During that runtime-configuration step, treat the repo \`skills/\` directory as the single authored source of truth and refresh managed \`~/.codex/skills/prism-*\` copies from it." \
    "- Preserve the default no-argument flow exactly: scan first, render the scan result, then prompt for default selection." \
    "- Preserve the shared-skill subcommand behavior exactly: \`scan\` means scan only, \`defaults\` means show current defaults, and \`set <indices>\` means update defaults directly with the provided comma-separated indices." \
    "- Preserve the shared skill's user-facing status text and stop conditions: empty scans should surface \`No GitHub repositories found in your home directory.\`, clearing defaults should surface the shared greenfield-mode confirmation, and successful default updates should confirm the selected repository names." \
    "- Invalid brownfield selections or MCP failures must fail the Codex run instead of being converted into a success summary." \
    "- Reuse Prism's bundled MCP tools and shared skill content exactly. Do not reimplement or paraphrase the setup workflow in this wrapper."
}

prism_psm_define_command_config "setup" "shared_skill_relative_path" "skills/setup/SKILL.md"
prism_psm_define_command_config "setup" "skill_title" "setup flow"
prism_psm_define_command_config "setup" "skill_description" "Run Prism setup from Codex through the shared brownfield scan and default-selection workflow."
prism_psm_define_command_config "setup" "command_description" "Run Prism setup workflow"
prism_psm_define_command_config "setup" "usage_function" "prism_psm_setup_usage"
prism_psm_define_command_config "setup" "skill_dispatch_function" "prism_psm_setup_skill_dispatch"
prism_psm_define_command_config "setup" "skill_normalization_function" "prism_psm_setup_skill_normalization"
prism_psm_define_command_config "setup" "command_entrypoint_function" "prism_psm_setup_command_entrypoint"
prism_psm_define_command_config "setup" "asset_paths_function" "prism_psm_setup_asset_paths"
prism_psm_define_command_config "setup" "contract_function" "prism_psm_setup_command_contract"
