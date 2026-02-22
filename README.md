# Prism — Multi-Perspective Agent Team Analysis

## Prerequisites

### 1. ontology-docs MCP Setup (Required)

Register an `ontology-docs` MCP server pointing to your project's documentation directory.

Add to `~/.claude/settings.json` (global) or `.claude/settings.json` (project):

```json
{
  "mcpServers": {
    "ontology-docs": {
      "type": "stdio",
      "command": "npx",
      "args": [
        "-y",
        "@anthropic/mcp-filesystem",
        "/path/to/your/docs"
      ]
    }
  }
}
```

Replace `/path/to/your/docs` with the absolute path to your project's documentation directory.

### 2. Agent Teams (Required)

Enable experimental agent teams:

Add to `~/.claude/settings.json`:

```json
{
  "env": {
    "CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS": "1"
  }
}
```

## Skills

- `/prism:incident` — Multi-perspective incident postmortem
- `/prism:prd` — PRD policy conflict analysis

## Documentation Structure

The `ontology-docs` MCP should point to a directory containing your project's
domain/policy documentation. The plugin auto-discovers the directory structure
at runtime and assigns relevant docs to each analysis perspective.
