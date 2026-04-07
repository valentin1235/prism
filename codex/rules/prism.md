# Prism for Codex

Use Prism commands when the user is asking to manage brownfield repository defaults, run general Prism analysis, or run incident RCA analysis.

## CRITICAL: Skill Routing

When the user types a registered `psm` command, you MUST route to the matching Prism Codex skill.
Do NOT interpret `psm` commands as natural language when they match a registered Prism command.
Treat installed `~/.codex/skills/prism-*` entries as setup-managed mirrors refreshed from the shared repo `skills/` source, not as independently authored workflows.

| User Input | Codex Skill |
|-----------|-------------|
| `psm analyze` | `prism-analyze` |
| `psm analyze <topic>` | `prism-analyze` |
| `psm analyze --config /path/to/config.json` | `prism-analyze` |
| `psm brownfield` | `prism-brownfield` |
| `psm brownfield scan` | `prism-brownfield` |
| `psm brownfield defaults` | `prism-brownfield` |
| `psm brownfield set 6,18,19` | `prism-brownfield` |
| `psm incident` | `prism-incident` |
| `psm incident <description>` | `prism-incident` |
| `psm prd /path/to/prd.md` | `prism-prd` |
| `psm setup` | `prism-setup` |

## Scope Boundary

This initial Codex registration only covers `psm analyze`, `psm brownfield`, `psm incident`, `psm prd`, and `psm setup`.
Do not assume other `psm` commands are installed unless separate Prism Codex skills are present.
In particular, `psm analyze-workspace` and `psm test-analyze` are excluded from this first milestone and must not be treated as registered Codex commands.
Treat `codex/lib/command-registry.tsv` as the executable closed set for this milestone.
Treat `codex/lib/command-ontology.tsv` as the broader command ontology: commands outside the closed set may appear there only when their rows are explicitly marked `non-acceptance-bearing` and `unregistered`, so they do not expand required implementation scope.

## Analyze Asset Root

For `psm analyze`, treat the shared Prism asset root as deterministic:
1. `PRISM_REPO_PATH` when it points to a Prism repo containing the required shared analyze assets.
2. The installed `repo-root` pointer shipped with the shared `psm` integration layer.
3. A Prism repo root inferred relative to the shared `psm` library.

Do not resolve shared Prism analyze assets from the user's working directory.

## Natural Language Mapping

For natural-language requests, map to the same skill when the user is clearly asking to:
- run Prism multi-perspective analysis
- scan Prism brownfield repositories
- show Prism default repositories
- set Prism brownfield defaults
- run Prism incident RCA analysis
- analyze an outage or incident with Prism
- run Prism PRD policy analysis
- run Prism setup

If the request is unrelated to registered Prism commands, handle it normally.
