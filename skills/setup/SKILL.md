---
name: setup
description: Configure documentation directories for prism skills (ontology-docs.json)
version: 2.0.0
user-invocable: true
allowed-tools: Bash, Write, AskUserQuestion
---

# Prism Setup

Configure documentation directories that prism skills (incident, prd, plan) reference during analysis.

> **Note**: prism is now a built-in MCP server bundled with this plugin. No separate binary installation or MCP registration is needed.

## Step 1: Configure Documentation Directories

Loop with a single question per iteration. The user can type a path directly or skip:

```
AskUserQuestion(
  header: "Documentation Directories",
  question: "<current_list_or_empty>prism 스킬(incident, prd, plan)이 분석 시 참조할 문서 디렉토리 경로를 입력하세요:",
  options: [
    {label: "Enter path", description: "Type an absolute directory path"},
    {label: "Skip", description: "Save and finish"}
  ]
)
```

- `<current_list_or_empty>`: If directories already added, show `"Current directories:\n- /path/a\n- /path/b\n\n"`. If none yet, omit.
- **"Enter path"**: Extract path → validate with `test -d` → if not found, warn and loop back → if already in list, warn and loop back → if valid, add to list, immediately loop back with updated list.
- **"Skip"**: Exit loop. If any paths were added, write to `~/.prism/ontology-docs.json`. If none, inform user they can configure later.

Write config:
```json
{
  "directories": [
    "/path/to/project-a/docs",
    "/path/to/project-b/docs"
  ]
}
```

## Step 2: Verify

Confirm to the user:
- Documentation directories configured (if any) at `~/.prism/ontology-docs.json`
- prism is bundled as a built-in MCP server — no restart needed
