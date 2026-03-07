# Prerequisite: Agent Team Mode (HARD GATE)

**This gate MUST be checked before ALL other phases. Do NOT skip.**

Placeholder: `{PROCEED_TO}` — the phase to proceed to on success (e.g., "Phase 0", "Input").

## Step 1: Check Settings

Read `~/.claude/settings.json` using the `Read` tool and verify:

```
env.CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS === "1"
```

## Step 2: Decision

| Condition | Action |
|-----------|--------|
| `"CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS": "1"` exists | → Proceed to {PROCEED_TO} |
| Value is `"0"` or key is missing | → **STOP immediately**, show message below |
| `~/.claude/settings.json` file does not exist | → **STOP immediately**, show message below |

## On Failure: Show This Message and STOP

If the setting is not satisfied, output the following message to the user and **terminate skill execution entirely**:

```
Agent Team Mode is not enabled.

This plugin (prism) requires Agent Team Mode because it uses multi-agent team
features (TeamCreate, TaskList, SendMessage, etc.).

How to enable:

1. Open ~/.claude/settings.json (create it if it doesn't exist)
2. Add the following to the "env" section:

   {
     "env": {
       "CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS": "1"
     }
   }

   If you already have an "env" section, just add this key inside it:

   "CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS": "1"

3. Restart Claude Code
4. Run this skill again after restarting
```

**HARD STOP**: Do NOT proceed to {PROCEED_TO} or any subsequent phase if this gate fails. Output the message above and terminate immediately.
