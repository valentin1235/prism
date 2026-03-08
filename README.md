# Prism

Multi-perspective agent team analysis plugin for [Claude Code](https://docs.anthropic.com/en/docs/claude-code).

Prism spawns a coordinated team of specialized AI agents — each analyzing from a different perspective — then cross-validates findings through a Devil's Advocate before producing a final report.

## Philosophy

How has humanity solved its hardest problems? Diverse minds in a room, arguing until only the defensible ideas survive. Prism is that room — but the minds are AI specialists, and nobody gets to leave until the weak ideas are dead.

## Skills

| Skill | Command | Description |
|-------|---------|-------------|
| **incident** | `/prism:incident` | Incident postmortem with 3-6 perspective agents + Devil's Advocate + optional Tribunal |
| **prd** | `/prism:prd` | PRD policy conflict analysis against your reference docs via ontology-docs MCP |
| **plan** | `/prism:plan` | Multi-perspective planning with committee debate + consensus enforcement |
| **analyze** | `/prism:analyze` | General-purpose multi-perspective analysis with MCP-based Socratic verification + ambiguity scoring |

## Prerequisites

Before installing Prism, make sure you have:

1. **Claude Code** installed and working
2. **oh-my-claudecode** plugin installed (Prism uses its agent types for team members)

## Installation

### Step 1: Install the plugin

Inside Claude Code, register the Prism marketplace and install the plugin:

```
/plugin marketplace add valentin1235/prism
/plugin install prism@prism-plugins
```

Or from the terminal CLI:

```bash
claude plugin marketplace add valentin1235/prism
claude plugin install prism@prism-plugins
```

The plugin will be automatically enabled after installation. You can verify with `/plugin` (Installed tab) or:

```bash
claude plugin list
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

### Step 3: Install oh-my-claudecode (agent pack)

Prism does not have its own built-in agents. It currently uses [oh-my-claudecode](https://github.com/anthropics-community/oh-my-claudecode) as a general-purpose agent pack, which provides the specialized agent types needed for team analysis (`architect`, `architect-medium`, `analyst`, `critic`, etc.). Install it if you haven't already:

```
/plugin marketplace add Yeachan-Heo/oh-my-claudecode
/plugin install oh-my-claudecode@omc
```

Or from the terminal CLI:

```bash
claude plugin marketplace add Yeachan-Heo/oh-my-claudecode
claude plugin install oh-my-claudecode@omc
```

### Step 4: Configure ontology-docs MCP (optional)

All skills can reference your internal documentation through the `ontology-docs` MCP server. This is optional but recommended for accurate policy/codebase analysis.

Use the `claude mcp add` CLI command to register the server with **user scope**. Replace `/path/to/your/docs` with the absolute path to your documentation directory.

```bash
claude mcp add --transport stdio --scope user ontology-docs \
  -- npx -y @modelcontextprotocol/server-filesystem /path/to/your/docs
```

> **The server name must be exactly `ontology-docs`.** Prism skills internally reference `mcp__ontology-docs__*` tools by this name. Using a different name will cause the skills to fail.

> `--scope user` is recommended so the MCP server is available across all projects. With `local` or `project` scope, the server will only be accessible within that specific project.

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

After completing all installation steps, your `~/.claude/settings.json` should contain at least these entries. The `env` section must be added manually (Step 2), while `enabledPlugins` are added automatically by `claude plugin install`:

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

| Skill | Command | Workflow | Output |
|-------|---------|---------|--------|
| **incident** | `/prism:incident` | Intake → Seed Analysis → 3-6 Perspective Agents → Devil's Advocate → optional Tribunal → Report | Postmortem report |
| **prd** | `/prism:prd path/to/prd.md` | Read PRD → 3-6 Policy Analysts (via ontology-docs MCP) → Devil's Advocate → Report | `prd-policy-review-report.md` |
| **plan** | `/prism:plan path/to/prd.md` | Input Analysis → 3-6 Analysts → Devil's Advocate → Committee Debate (UX + Eng + Planner) → Consensus Loop → Plan | `plan.md` |
| **analyze** | `/prism:analyze` | Intake → Seed Analysis → Perspective Generation → Parallel Analysts → Socratic Verification (per analyst) → Synthesis | Analysis report |

All skills share the same core pattern: **spawn multi-perspective agents → cross-validate → synthesize**. See [How It Works](#how-it-works) for detailed flow diagrams.

### analyze: Socratic Verification & Ambiguity Scoring

The analyze skill adds MCP-based Socratic verification on top of the shared pattern. Each analyst self-verifies through a `prism_interview` loop before reporting:

```
Analyst → write findings.json → prism_interview(start) → question
       → answer (with evidence) → auto-score → next question
       → ... repeat until PASS or max rounds
```

**Ambiguity Score:**

| Dimension | Weight | Measures |
|-----------|--------|----------|
| Evidence Clarity | 0.4 | Findings backed by concrete evidence (file, line, log)? |
| Causal Chain Clarity | 0.35 | Cause-effect chain logically sound and complete? |
| Recommendation Clarity | 0.25 | Recommendations specific and actionable? |

Verdict: **PASS** (score meets threshold) or **FORCE PASS** (max rounds reached, flagged for user attention).

---

## Inspired By

Prism's multi-perspective analysis and Socratic verification approach was inspired by [Ouroboros](https://github.com/Q00/ouroboros).

---

## How It Works

<details>
<summary><b>Analyze with Socratic Verification</b> (<code>/prism:analyze</code>)</summary>

```mermaid
graph TD
    A[User Input] --> B{Prerequisite Gate}
    B -->|Not enabled| X[Show setup guide & STOP]
    B -->|Enabled| C[Problem Intake]

    C --> D["Seed Analysis (Opus)"]
    D -->|"severity, dimensions, research"| E["Perspective Generation (Opus)"]
    E -->|"2-5 lenses from 14 archetypes"| F{User Approval}

    F -->|Modify| E
    F -->|Approve| G[Ontology Scope Mapping]
    G --> H[Spawn Analysts in Parallel]

    H --> I1[Analyst 1: Investigate]
    H --> I2[Analyst 2: Investigate]
    H --> IN[Analyst N: Investigate]

    I1 --> J1["prism_interview Loop"]
    I2 --> J2["prism_interview Loop"]
    IN --> JN["prism_interview Loop"]

    J1 -->|"score ≥ threshold → PASS"| K[Collect Verified Findings]
    J2 -->|"score ≥ threshold → PASS"| K
    JN -->|"score ≥ threshold → PASS"| K

    K --> L[Synthesis & Report]
    L --> M{Complete?}
    M -->|Yes| N[Cleanup]
    M -->|"Deeper Investigation (max 2)"| H

    style D fill:#7c3aed,color:#fff
    style E fill:#7c3aed,color:#fff
    style I1 fill:#4a9eff,color:#fff
    style I2 fill:#4a9eff,color:#fff
    style IN fill:#4a9eff,color:#fff
    style J1 fill:#ef4444,color:#fff
    style J2 fill:#ef4444,color:#fff
    style JN fill:#ef4444,color:#fff
    style F fill:#f59e0b,color:#fff
    style X fill:#dc2626,color:#fff
    style L fill:#16a34a,color:#fff
```

</details>

<details>
<summary><b>Incident Postmortem</b> (<code>/prism:incident</code>)</summary>

```mermaid
graph TD
    A[User Input] --> B{Prerequisite Gate}
    B -->|Not enabled| X[Show setup guide & STOP]
    B -->|Enabled| C[Problem Intake]
    C -->|Severity, Evidence, Context| D{SEV1 or Active?}

    D -->|Yes| E1[Fast Track: 4 Core + DA]
    D -->|No| E2[Perspective Generation]
    E2 -->|Select 3-5 archetypes| E1

    E1 --> F[Team Formation]

    F --> G1[Timeline Analyst]
    F --> G2[Root Cause Analyst]
    F --> G3[Systems Analyst]
    F --> GN[+ Extended Analysts]

    G1 --> H[Devil's Advocate]
    G2 --> H
    G3 --> H
    GN --> H

    H --> I{Tribunal needed?}
    I -->|Yes| J1[UX Critic]
    I -->|Yes| J2[Engineering Critic]
    I -->|No| K

    J1 --> K[Final Report]
    J2 --> K

    K --> L[Team Teardown]

    style G1 fill:#4a9eff,color:#fff
    style G2 fill:#4a9eff,color:#fff
    style G3 fill:#4a9eff,color:#fff
    style GN fill:#4a9eff,color:#fff
    style H fill:#ef4444,color:#fff
    style J1 fill:#f59e0b,color:#fff
    style J2 fill:#f59e0b,color:#fff
    style X fill:#dc2626,color:#fff
```

</details>

<details>
<summary><b>PRD Policy Analysis</b> (<code>/prism:prd</code>)</summary>

```mermaid
graph TD
    A["/prism:prd path/to/prd.md"] --> B{Prerequisite Gate}
    B -->|Not enabled| X[Show setup guide & STOP]
    B -->|Enabled| C[Read PRD & Sibling Files]
    C --> D[Generate 3-6 Policy Perspectives]

    D --> E[Team Formation]

    E --> F1[Policy Analyst 1]
    E --> F2[Policy Analyst 2]
    E --> F3[Policy Analyst 3]
    E --> FN[Policy Analyst N]

    F1 -->|ontology-docs MCP| G[All Analysts Complete]
    F2 -->|ontology-docs MCP| G
    F3 -->|ontology-docs MCP| G
    FN -->|ontology-docs MCP| G

    G --> H[Devil's Advocate]
    H -->|Merge, Calibrate, Rank| I["Final Report (prd-policy-review-report.md)"]

    I --> J[Team Teardown]

    style F1 fill:#4a9eff,color:#fff
    style F2 fill:#4a9eff,color:#fff
    style F3 fill:#4a9eff,color:#fff
    style FN fill:#4a9eff,color:#fff
    style H fill:#ef4444,color:#fff
    style X fill:#dc2626,color:#fff
```

</details>

<details>
<summary><b>Plan with Committee Debate</b> (<code>/prism:plan</code>)</summary>

```mermaid
graph TD
    A[Input Analysis & Context] --> B[Generate 3-6 Perspectives]
    B --> C[Team Formation]

    C --> D1[Analyst 1]
    C --> D2[Analyst 2]
    C --> D3[Analyst 3]
    C --> DN[Analyst N]

    D1 --> E[Devil's Advocate Synthesis]
    D2 --> E
    D3 --> E
    DN --> E

    E --> F["Committee Debate (UX + Eng + Planner)"]
    F --> G{Consensus?}

    G -->|"3/3 Unanimous"| H["Write plan.md & Cleanup"]
    G -->|"2/3 Working (Normal only)"| H
    G -->|"No Consensus"| I[Shutdown Committee]
    I --> J[Add Perspective + Gap Analysis]

    J -->|"Normal: max 2 loops"| C
    J -->|"Hell: unlimited loops"| C

    style D1 fill:#4a9eff,color:#fff
    style D2 fill:#4a9eff,color:#fff
    style D3 fill:#4a9eff,color:#fff
    style DN fill:#4a9eff,color:#fff
    style E fill:#ef4444,color:#fff
    style F fill:#f59e0b,color:#fff
    style G fill:#d97706,color:#fff,stroke:#333
    style H fill:#16a34a,color:#fff,stroke:#333
    style I fill:#dc2626,color:#fff,stroke:#333
```

</details>

### plan: Consensus Modes

| Consensus Level | Condition | Normal Mode | Hell Mode (`--hell`) |
|-------|-----------|-------------|-----------|
| **Strong** | 3/3 agree | Output | Output |
| **Working** | 2/3 agree | Output | Feedback Loop |
| **No Consensus** | <60% | Feedback Loop | Feedback Loop |

Normal Mode: max 2 feedback loops, then forced output. Hell Mode: unlimited loops until unanimous.

### Agent Mapping

| Role | Agent Type | Model |
|------|-----------|-------|
| Analyst (complex) | `oh-my-claudecode:analyst` | opus |
| Analyst (standard) | `oh-my-claudecode:architect-medium` | sonnet |
| Devil's Advocate | `oh-my-claudecode:critic` | opus |
| UX / Engineering Critic | `oh-my-claudecode:architect` / `architect-medium` | opus / sonnet |
| Planner | `oh-my-claudecode:planner` | opus |

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
