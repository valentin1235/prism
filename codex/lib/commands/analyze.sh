#!/usr/bin/env bash

prism_psm_analyze_usage() {
  cat <<'EOF'
psm analyze
psm analyze <topic>
psm analyze --config /path/to/config.json
EOF
}

prism_psm_analyze_skill_dispatch() {
  cat <<'EOF'
Use `Glob(pattern="**/skills/analyze/SKILL.md")` to locate the shared Prism analyze skill.
Treat `PRISM_REPO_PATH/skills/analyze/SKILL.md` as the canonical shared skill path when it is available.
If multiple matches exist, prefer the Prism-owned path under `PRISM_REPO_PATH` over matches from the user's target repository or working directory.
Read the first match.
Follow that shared skill exactly.
EOF
}

prism_psm_analyze_skill_normalization() {
  cat <<'EOF'
Normalize the command prefix from the Claude Code form to Codex:
  `/prism:analyze` -> `psm analyze`
Normalize shared-skill references accordingly:
  `Use \`prism:brownfield\`` -> `Use \`psm brownfield\``
  `Wrapper skills (e.g., \`/prd\`)` -> `Wrapper skills (e.g., \`psm prd\`)`
Treat the installed Prism MCP server plus `PRISM_REPO_PATH` as the source of truth for shared analyze assets.
Resolve the shared Prism asset root deterministically in this order:
  `PRISM_REPO_PATH` when it points to a Prism repo containing the required shared analyze assets.
  The installed `repo-root` pointer shipped with the shared `psm` integration layer.
  A Prism repo root inferred relative to the shared `psm` library.
Never resolve shared Prism analyze assets from the user's working directory.
Reuse shared analyze assets directly from the Prism repository whenever Codex can consume them without translation.
Pass shared analyze config paths such as `report_template` through unchanged when they already point at Prism assets.
Do not reimplement or paraphrase the Prism analyze workflow in this Codex wrapper.
EOF
}

prism_psm_analyze_command_entrypoint() {
  cat <<'EOF'
Use `PRISM_REPO_PATH` as the source of truth for shared Prism assets when it points to a Prism repo containing `skills/analyze/SKILL.md`.
If that path is unavailable, fall back to `Glob(pattern="**/skills/analyze/SKILL.md")` to locate the shared Prism analyze skill.
Treat the shared analyze skill as the only workflow definition.
Pass shared analyze config paths such as `report_template` through unchanged when they already point at Prism assets.
Read the resolved shared skill with the Read tool and follow its instructions exactly.
EOF
}

prism_psm_analyze_asset_paths() {
  cat <<'EOF'
skills/analyze/SKILL.md
skills/analyze/prompts/seed-analyst.md
skills/analyze/prompts/perspective-generator.md
skills/analyze/prompts/finding-protocol.md
skills/analyze/prompts/verification-protocol.md
skills/analyze/templates/report.md
EOF
}

prism_psm_analyze_command_contract() {
  printf '%s\n' \
    "" \
    "For psm analyze, use the shared Prism assets from that repository directly when Codex can consume them as-is:" \
    "- ${PRISM_REPO_PATH}/skills/analyze/SKILL.md" \
    "- Treat that shared skill as the only workflow definition; do not duplicate its phase logic in the Codex command layer." \
    "- Reuse Prism-owned prompt and template assets from ${PRISM_REPO_PATH}/skills/analyze/ when the shared skill references them." \
    "If analyze config already provides a Prism asset path such as report_template, pass it through unchanged."
}

prism_psm_define_command_config "analyze" "shared_skill_relative_path" "skills/analyze/SKILL.md"
prism_psm_define_command_config "analyze" "skill_title" "general-purpose multi-perspective analysis flow"
prism_psm_define_command_config "analyze" "skill_description" "Run Prism multi-perspective analysis through the shared analyze skill and Codex runtime adapter."
prism_psm_define_command_config "analyze" "command_description" "Run Prism multi-perspective analysis"
prism_psm_define_command_config "analyze" "usage_function" "prism_psm_analyze_usage"
prism_psm_define_command_config "analyze" "skill_dispatch_function" "prism_psm_analyze_skill_dispatch"
prism_psm_define_command_config "analyze" "skill_normalization_function" "prism_psm_analyze_skill_normalization"
prism_psm_define_command_config "analyze" "command_entrypoint_function" "prism_psm_analyze_command_entrypoint"
prism_psm_define_command_config "analyze" "asset_paths_function" "prism_psm_analyze_asset_paths"
prism_psm_define_command_config "analyze" "contract_function" "prism_psm_analyze_command_contract"
prism_psm_define_command_config "analyze" "prepare_function" "prism_psm_prepare_analyze_args"
prism_psm_define_command_config "analyze" "prompt_function" "prism_psm_analyze_bridge_prompt"
