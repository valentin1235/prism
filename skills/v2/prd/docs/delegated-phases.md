# Delegated Phases — Original Steps Reference

These phases are delegated to the setup agent (`../shared/setup-agent.md`). This file preserves the original step definitions for reference.

---

## Phase 0: Input

### MUST: Minimum Required

| Item | Required | Method |
|------|----------|--------|
| PRD file path | YES | Argument or `AskUserQuestion` |
| Report output path | NO | Defaults to `prd-policy-review-report.md` in PRD file's directory |

```
/prd-v2 @path/to/prd.md
```

Reference docs are accessed via `ontology-docs` MCP — no path input needed.
If PRD file not found, error: `"PRD file not found: {path}"`.

---

## Phase 1: PRD Analysis & Perspective Generation

### 1.0 Language Detection

Detect report language from user's environment:
1. Check if CLAUDE.md contains `Language` directive → use that language
2. Otherwise, detect from user's input language in this session
3. Store as `{REPORT_LANGUAGE}` for Phase 5

Default: user's detected language.

### 1.1 Read PRD

Read full PRD via `Read`. Also read any sibling files (handoff, constraints) in the same directory.

### 1.2 Generate Perspectives

Analyze PRD functional requirements and cross-reference with ontology-docs domains to derive **N orthogonal policy analysis perspectives**.

Rules:
- Each perspective covers a **policy ontology unit** (e.g., ticket policy, payment policy, retention policy)
- **Orthogonality** between perspectives — no overlapping domains
- Minimum 3, maximum 6 perspectives

Perspective definition format:
```
ID: {slug}
Name: {perspective name}
Scope: {policy domain this perspective examines}
PRD sections: {FR-N, NFR-N, etc.}
```

#### Perspective Quality Gate

→ Apply `../shared/perspective-quality-gate.md` with `{DOMAIN}` = "prd", `{EVIDENCE_SOURCE}` = "PRD content and ontology docs".

### 1.3 Ontology Scope Mapping

→ Read and execute `../shared/ontology-scope-mapping.md` with:
- `{AVAILABILITY_MODE}` = `required`
- `{CALLER_CONTEXT}` = `"PRD analysis"`
- `{STATE_DIR}` = `.omc/state/prd-{short-id}`

PRD analysis requires policy document references — MCP unavailability stops execution.

**Note:** This skill requires the `ontology-docs` MCP server to be configured. If not set up, run the `podo-plugin:install-docs` skill or see the plugin README for configuration instructions.
