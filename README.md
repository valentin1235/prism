# Prism

Multi-perspective agent team analysis plugin for [Claude Code](https://docs.anthropic.com/en/docs/claude-code).

Prism spawns a coordinated team of specialized AI agents — each analyzing from a different perspective — then cross-validates findings through a Devil's Advocate before producing a final report.

## Skills

| Skill | Command | Description |
|-------|---------|-------------|
| **incident** | `/prism:incident` | Incident postmortem with 3-6 perspective agents + Devil's Advocate + optional Tribunal |
| **prd** | `/prism:prd` | PRD policy conflict analysis against your reference docs via ontology-docs MCP |

## Prerequisites

Before installing Prism, make sure you have:

1. **Claude Code** installed and working
2. **oh-my-claudecode** plugin installed (Prism uses its agent types for team members)

## Installation

### Step 1: Install the plugin

```bash
claude plugin add prism-plugins/prism
```

Or clone manually:

```bash
git clone https://github.com/valentin1235/prism.git ~/.claude/plugins/prism
```

Then enable it in `~/.claude/settings.json`:

```json
{
  "enabledPlugins": {
    "prism@prism-plugins": true
  }
}
```

### Step 2: Enable Agent Team Mode

Prism uses multi-agent team features (TeamCreate, TaskList, SendMessage, etc.) which require Agent Team Mode to be enabled.

Open `~/.claude/settings.json` and add `CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS` to the `env` section:

```json
{
  "env": {
    "CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS": "1"
  }
}
```

If you already have an `env` section with other keys, just add the new key inside it:

```json
{
  "env": {
    "EXISTING_KEY": "existing_value",
    "CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS": "1"
  }
}
```

**Restart Claude Code after making this change.**

> Without this setting, Prism skills will refuse to run and show a setup guide instead.

### Step 3: Install oh-my-claudecode (dependency)

Prism agents use oh-my-claudecode agent types (`architect`, `architect-medium`, `analyst`, `critic`, etc.). Install it if you haven't already:

```bash
claude plugin add omc/oh-my-claudecode
```

And enable it:

```json
{
  "enabledPlugins": {
    "oh-my-claudecode@omc": true,
    "prism@prism-plugins": true
  }
}
```

### Step 4: Configure ontology-docs MCP (optional)

Both skills can reference your internal documentation through the `ontology-docs` MCP server. This is optional but recommended for accurate policy/codebase analysis.

Use the `claude mcp add` CLI command to register the server. Replace `/path/to/your/docs` with the absolute path to your documentation directory.

**User scope** (available across all your projects):

```bash
claude mcp add --transport stdio --scope user ontology-docs \
  -- npx -y @modelcontextprotocol/server-filesystem /path/to/your/docs
```

**Local scope** (current project only, default):

```bash
claude mcp add --transport stdio ontology-docs \
  -- npx -y @modelcontextprotocol/server-filesystem /path/to/your/docs
```

**Project scope** (shared with your team via `.mcp.json`):

```bash
claude mcp add --transport stdio --scope project ontology-docs \
  -- npx -y @modelcontextprotocol/server-filesystem /path/to/your/docs
```

Verify it was added:

```bash
claude mcp list
```

> For more details on MCP configuration, see the [official Claude Code MCP docs](https://code.claude.com/docs/en/mcp).

### Step 5: Verify installation

Restart Claude Code, then type:

```
/prism:incident
```

If everything is configured correctly, the skill will start the incident intake process. If Agent Team Mode is not enabled, it will show you the setup instructions.

## Full settings.json Example

Here's a complete example with all required settings:

```json
{
  "env": {
    "CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS": "1"
  },
  "enabledPlugins": {
    "oh-my-claudecode@omc": true,
    "prism@prism-plugins": true
  }
}
```

## Usage

### Incident Postmortem

```
/prism:incident
```

The skill will guide you through:

1. **Problem Intake** — Describe the incident, severity, and evidence
2. **Perspective Generation** — AI recommends 3-5 analysis perspectives based on your incident
3. **Team Formation** — Spawns specialized agents (Timeline, Root Cause, Systems, Impact, etc.)
4. **Analysis Execution** — Agents analyze in parallel, cross-validate findings
5. **Tribunal** (conditional) — UX + Engineering critics review recommendations if needed
6. **Report** — Structured postmortem report with findings and recommendations

**Available perspectives:**

| Core | Extended |
|------|----------|
| Timeline | Security & Threat |
| Root Cause | Data Integrity |
| Systems & Architecture | Performance & Capacity |
| Impact | Deployment & Change |
| | Network & Connectivity |
| | Concurrency & Race |
| | External Dependency |
| | User Experience |

### PRD Policy Analysis

```
/prism:prd path/to/your/prd.md
```

The skill will:

1. **Read & Analyze PRD** — Parse functional requirements, detect policy domains
2. **Generate Perspectives** — Create 3-6 orthogonal policy analysis perspectives
3. **Spawn Analysts** — Each analyst examines PRD against reference docs for their domain
4. **Devil's Advocate** — Merges duplicates, calibrates severity, finds gaps, ranks TOP 10 PM decisions
5. **Report** — Final policy analysis report written to the PRD's directory

**Output:** `prd-policy-review-report.md` in the same directory as the PRD file.

## How It Works

```
User Input
    |
    v
[Prerequisite Gate] -- checks CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS
    |
    v
[Problem Intake / PRD Analysis]
    |
    v
[Perspective Generation] -- selects 3-6 orthogonal analysis angles
    |
    v
[Team Formation] -- TeamCreate + spawn agents in parallel
    |
    v
[Parallel Analysis] -- each agent analyzes from its perspective
    |
    v
[Devil's Advocate] -- cross-validates, challenges, ranks findings
    |
    v
[Tribunal] -- (incident only, conditional) UX + Engineering critics
    |
    v
[Final Report] -- synthesized, evidence-cited report
    |
    v
[Team Teardown] -- shutdown agents + cleanup
```

## Troubleshooting

### "Agent Team Mode is not enabled"

Add `"CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS": "1"` to the `env` section of `~/.claude/settings.json` and restart Claude Code. See [Step 2](#step-2-enable-agent-team-mode).

### "ontology-docs MCP not configured"

The skill tried to access reference docs but the MCP server isn't set up. See [Step 4](#step-4-configure-ontology-docs-mcp-optional).

### Agents not spawning / TeamCreate fails

Make sure `oh-my-claudecode` plugin is installed and enabled. Prism's agents depend on oh-my-claudecode agent types. See [Step 3](#step-3-install-oh-my-claudecode-dependency).

### Skill not showing in autocomplete

Make sure `"prism@prism-plugins": true` is in your `enabledPlugins` and restart Claude Code.

## License

MIT
