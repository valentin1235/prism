# Prism - Development Environment

> This CLAUDE.md is for local Prism development. In this repo, treat the checked-in
> `commands/` and `skills/` directories as the command source of truth.
> Do not require generated skill copies for slash-command discovery.
> Codex setup may refresh managed mirrors under `~/.codex/skills/prism-*`, but those are install artifacts, not the authored workflow source.

## /prism Commands (Dev Mode)

When the user types any of these commands, read the corresponding repo skill file
from `skills/` and follow it exactly:

| Input | Action |
|-------|--------|
| `/prism:analyze ...` | Read `skills/analyze/SKILL.md` and follow it |
| `/prism:brownfield ...` | Read `skills/brownfield/SKILL.md` and follow it |
| `/prism:incident ...` | Read `skills/incident/SKILL.md` and follow it |
| `/prism:prd ...` | Read `skills/prd/SKILL.md` and follow it |
| `/prism:setup ...` | Read `skills/setup/SKILL.md` and follow it |

## psm Commands (Dev Mode)

When the user types any of these commands, read the corresponding repo skill file
from `skills/` and follow it exactly:

| Input | Action |
|-------|--------|
| `psm analyze ...` | Read `skills/analyze/SKILL.md` and follow it |
| `psm brownfield ...` | Read `skills/brownfield/SKILL.md` and follow it |
| `psm incident ...` | Read `skills/incident/SKILL.md` and follow it |
| `psm prd ...` | Read `skills/prd/SKILL.md` and follow it |
| `psm setup ...` | Read `skills/setup/SKILL.md` and follow it |

**Important**:
- Treat `psm analyze` as a command, not as natural language.
- Treat `psm brownfield` as a command, not as natural language.
- Treat `psm incident` as a command, not as natural language.
- Treat `psm prd` as a command, not as natural language.
- Treat `psm setup` as a command, not as natural language.
- Treat `/prism:*` and `psm *` inputs as commands, not as natural language.
- For local Prism development, resolve slash commands from the checked-in `commands/`
  directory and resolve the workflows from the checked-in `skills/` directory.
- Reuse Prism's bundled MCP tools and skill assets; do not reimplement the workflow ad hoc.
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
