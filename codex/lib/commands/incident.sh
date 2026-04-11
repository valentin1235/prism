#!/usr/bin/env bash

prism_psm_incident_usage() {
  cat <<'EOF'
psm incident
psm incident <incident description>
EOF
}

prism_psm_incident_skill_dispatch() {
  cat <<'EOF'
Use `Glob(pattern="**/skills/incident/SKILL.md")` to locate the shared Prism incident skill.
Treat `PRISM_REPO_PATH/skills/incident/SKILL.md` as the canonical shared skill path when it is available.
If multiple matches exist, prefer the Prism-owned path under `PRISM_REPO_PATH` over matches from the user's target repository or working directory.
Read the first match.
Follow that shared skill exactly.
EOF
}

prism_psm_incident_skill_normalization() {
  cat <<'EOF'
Normalize the command prefix from the Claude Code form to Codex:
  `/prism:incident` -> `psm incident`
Normalize analyze delegation accordingly:
  any Codex-side analyze invocation must use `psm analyze`
Treat the installed Prism MCP server plus `PRISM_REPO_PATH` as the source of truth for the shared incident workflow and assets.
Resolve the shared Prism asset root deterministically in this order:
  `PRISM_REPO_PATH` when it points to a Prism repo containing `skills/incident/SKILL.md`.
  The installed `repo-root` pointer shipped with the shared `psm` integration layer.
  A Prism repo root inferred relative to the shared `psm` library.
Never resolve shared incident skill assets from the user's working directory.
Do not assume the command was launched from within `~/prism` or from the user's current working directory.
Treat the shared incident skill as the only workflow definition.
Reuse Prism's bundled MCP tools, prompts, perspectives, and templates from the shared skill directory.
Do not reimplement or paraphrase the Prism incident workflow in this Codex wrapper.
EOF
}

prism_psm_incident_command_entrypoint() {
  cat <<'EOF'
Use `PRISM_REPO_PATH` as the source of truth for shared Prism assets when it points to a Prism repo containing `skills/incident/SKILL.md`.
If that path is unavailable, fall back to `Glob(pattern="**/skills/incident/SKILL.md")` to locate the shared Prism incident skill.
Resolve the shared incident report template and perspective injection assets from that same Prism asset root rather than from the caller's working directory.
Treat the shared incident skill as the only workflow definition.
Read the resolved shared skill with the Read tool and follow its instructions exactly.
EOF
}

prism_psm_incident_asset_paths() {
  cat <<'EOF'
skills/incident/SKILL.md
skills/incident/templates/report.md
skills/incident/perspectives/ux-impact.json
EOF
  prism_psm_analyze_asset_paths
}

prism_psm_incident_command_contract() {
  printf '%s\n' \
    "" \
    "For psm incident, dispatch to the shared Prism incident workflow entrypoint:" \
    "- Use the shared Prism incident skill at \`${PRISM_REPO_PATH}/skills/incident/SKILL.md\` as the workflow source of truth." \
    "- Treat that shared skill as the only workflow definition; do not duplicate its phase logic in the Codex command layer." \
    "- Reuse Prism's bundled incident assets from the shared skill directory, especially \`${PRISM_REPO_PATH}/skills/incident/templates/report.md\` and \`${PRISM_REPO_PATH}/skills/incident/perspectives/ux-impact.json\`." \
    "- Keep Codex-side delegation aligned with the shared skill: any analyze-style dispatch from the incident flow must use \`psm analyze\` semantics, not Claude command names." \
    "- Preserve the existing MCP invocation and artifact contract; do not replace the incident workflow with a generic summarize-and-exit wrapper."
}

prism_psm_define_command_config "incident" "shared_skill_relative_path" "skills/incident/SKILL.md"
prism_psm_define_command_config "incident" "skill_title" "incident root cause analysis flow"
prism_psm_define_command_config "incident" "skill_description" "Run Prism incident RCA analysis through the shared incident skill and Codex runtime adapter."
prism_psm_define_command_config "incident" "command_description" "Run Prism incident RCA analysis"
prism_psm_define_command_config "incident" "usage_function" "prism_psm_incident_usage"
prism_psm_define_command_config "incident" "skill_dispatch_function" "prism_psm_incident_skill_dispatch"
prism_psm_define_command_config "incident" "skill_normalization_function" "prism_psm_incident_skill_normalization"
prism_psm_define_command_config "incident" "command_entrypoint_function" "prism_psm_incident_command_entrypoint"
prism_psm_define_command_config "incident" "asset_paths_function" "prism_psm_incident_asset_paths"
prism_psm_define_command_config "incident" "contract_function" "prism_psm_incident_command_contract"
prism_psm_define_command_config "incident" "prepare_function" "prism_psm_prepare_incident_args"
prism_psm_define_command_config "incident" "prompt_function" "prism_psm_incident_bridge_prompt"
