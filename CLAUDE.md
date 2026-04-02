## Prism Commands (Codex Dev Mode)

When the user types `psm analyze` or `psm analyze ...`, use `Glob(pattern="**/codex/skills/analyze/SKILL.md")` to locate the Prism Codex analyze skill file, then read the first match and follow its instructions exactly.

When the user types `psm brownfield` or `psm brownfield ...`, use `Glob(pattern="**/codex/skills/brownfield/SKILL.md")` to locate the Prism Codex brownfield skill file, then read the first match and follow its instructions exactly.

When the user types `psm incident` or `psm incident ...`, use `Glob(pattern="**/codex/skills/incident/SKILL.md")` to locate the Prism Codex incident skill file, then read the first match and follow its instructions exactly.

When the user types `psm prd` or `psm prd ...`, use `Glob(pattern="**/codex/skills/prd/SKILL.md")` to locate the Prism Codex PRD skill file, then read the first match and follow its instructions exactly.

When the user types `psm setup`, use `Glob(pattern="**/codex/skills/setup/SKILL.md")` to locate the Prism Codex setup skill file, then read the first match and follow its instructions exactly.

Important:
- Treat `psm analyze` as a command, not as natural language.
- Treat `psm brownfield` as a command, not as natural language.
- Treat `psm incident` as a command, not as natural language.
- Treat `psm prd` as a command, not as natural language.
- Treat `psm setup` as a command, not as natural language.
- Reuse Prism's bundled MCP tools and Codex skill assets; do not reimplement the workflow ad hoc.
- Do NOT use the Skill tool. Read the file directly and execute it.

<!-- ooo:START -->
<!-- ooo:VERSION:0.26.0 -->
# Ouroboros — Specification-First AI Development

> Before telling AI what to build, define what should be built.
> As Socrates asked 2,500 years ago — "What do you truly know?"
> Ouroboros turns that question into an evolutionary AI workflow engine.

Most AI coding fails at the input, not the output. Ouroboros fixes this by
**exposing hidden assumptions before any code is written**.

1. **Socratic Clarity** — Question until ambiguity ≤ 0.2
2. **Ontological Precision** — Solve the root problem, not symptoms
3. **Evolutionary Loops** — Each evaluation cycle feeds back into better specs

```
Interview → Seed → Execute → Evaluate
    ↑                           ↓
    └─── Evolutionary Loop ─────┘
```

## ooo Commands

Each command loads its agent/MCP on-demand. Details in each skill file.

| Command | Loads |
|---------|-------|
| `ooo` | — |
| `ooo interview` | `ouroboros:socratic-interviewer` |
| `ooo seed` | `ouroboros:seed-architect` |
| `ooo run` | MCP required |
| `ooo evolve` | MCP: `evolve_step` |
| `ooo evaluate` | `ouroboros:evaluator` |
| `ooo unstuck` | `ouroboros:{persona}` |
| `ooo status` | MCP: `session_status` |
| `ooo setup` | — |
| `ooo help` | — |

## Agents

Loaded on-demand — not preloaded.

**Core**: socratic-interviewer, ontologist, seed-architect, evaluator,
wonder, reflect, advocate, contrarian, judge
**Support**: hacker, simplifier, researcher, architect
<!-- ooo:END -->
