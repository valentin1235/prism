# Prism

Multi-perspective agent team analysis plugin for Claude Code.

## Skills

| Skill | Purpose | Invocation |
|-------|---------|------------|
| **incident** | Multi-perspective incident postmortem with devil's advocate | `/prism:incident` |
| **prd** | PRD policy conflict analysis against existing docs | `/prism:prd` |
| **plan** | Multi-perspective planning with committee debate | `/prism:plan` |

## Plan Skill

Analyzes input from multiple dynamically-generated perspectives, synthesizes via Devil's Advocate, and produces actionable execution plans through a 3-person committee debate with consensus enforcement.

### Input

```
/prism:plan path/to/prd.md
/prism:plan "Design a new payment system"
/prism:plan https://example.com/requirements
/prism:plan --hell path/to/prd.md    # Hell Mode: unanimous or infinite loop
```

| Input Type | Detection | Action |
|-----------|-----------|--------|
| File path | `.md`, `.txt`, etc. | Read the file |
| URL | `http://` or `https://` | WebFetch |
| Text prompt | Plain text | Parse as requirements |
| No argument | During conversation | Summarize context |
| `--hell` | Hell Mode flag | Unanimous consensus required |

### Architecture

```mermaid
graph TB
    subgraph "Phase 0: Input"
        A[Input Analysis] --> B[Language Detection]
        B --> C[Extract Context]
        C --> D{Gaps?}
        D -->|Yes| E[User Interview]
        E --> C
        D -->|No| F[Exit Gate: 5 items]
    end

    subgraph "Phase 1: Perspectives"
        F --> G[Seed Analysis]
        G --> H[Generate 3-6 Perspectives]
        H --> I[Quality Gate]
        I --> J[User Approval]
        J -->|Modify| H
        J -->|Proceed| K[Lock Roster]
    end

    subgraph "Phase 2: Team Formation"
        K --> L[TeamCreate + Artifact Dir]
        L --> M[Create Tasks: Analysts + DA + Committee]
        M --> N[Pre-assign Owners]
    end

    subgraph "Phase 3: Analysis"
        N --> O[Spawn Analysts in Parallel]
        O --> P[Monitor & Coordinate]
        P --> Q[Clarity Enforcement]
        Q --> R[Exit Gate: 4 items]
        R --> S[Write analyst-findings.md]
    end

    subgraph "Phase 4: Devil's Advocate"
        S --> T[Spawn DA]
        T --> U[Challenge & Synthesize]
        U --> V[Write da-synthesis.md]
    end

    subgraph "Phase 5: Committee Debate"
        V --> W[Spawn UX Critic + Eng Critic + Planner]
        W --> X[Collect Positions]
        X --> Y[Lead-Mediated Debate]
        Y --> Z{Consensus?}
    end

    subgraph "Consensus Resolution"
        Z -->|Strong 3/3| AA[Phase 6]
        Z -->|Working 2/3| AA
        Z -->|Partial 60%+| AA
        Z -->|No Consensus| AB[Feedback Loop]
        AB --> AC[Write committee-debate.md]
        AC --> AD[Gap Analysis]
        AD --> AE{User Choice}
        AE -->|Add Perspective| AF[New Tasks Phase 2.2-2.3]
        AF --> O
        AE -->|Force/Stop| AA
    end

    subgraph "Phase 6-7: Output"
        AA --> AG[Write plan.md]
        AG --> AH[Chat Summary]
        AH --> AI[Cleanup & TeamDelete]
    end

    style Z fill:#f96,stroke:#333
    style AB fill:#f66,stroke:#333
    style AA fill:#6f6,stroke:#333
```

### Committee Debate Protocol

```mermaid
sequenceDiagram
    participant L as Lead
    participant UX as UX Critic
    participant E as Eng Critic
    participant P as Planner

    Note over L: Compile Briefing Package

    L->>UX: Synthesis Package
    L->>E: Synthesis Package
    L->>P: Synthesis Package

    UX->>L: Initial Position (votes per element)
    E->>L: Initial Position (votes per element)
    P->>L: Initial Position (votes per element)

    Note over L: Identify Disagreements

    L->>UX: Eng raises {concern}
    L->>E: UX argues {point}
    UX->>L: Updated Position
    E->>L: Updated Position

    alt Deadlock between UX & Eng
        L->>P: Tie-break request
        P->>L: Resolution proposal
    end

    Note over L: Build Convergence Table
    Note over L: Consensus Check

    alt Strong (3/3) or Working (2/3)
        L->>L: Proceed to Phase 6
    else No Consensus
        L->>L: Feedback Loop
    end
```

### Hell Mode

```mermaid
graph LR
    A[Phase 3: Analysis] --> B[Phase 4: DA]
    B --> C[Phase 5: Committee]
    C --> D{3/3 Unanimous?}
    D -->|Yes| E[Phase 6: Output]
    D -->|No| F[Shutdown Committee]
    F --> G[Add Perspective]
    G --> A

    style D fill:#f00,color:#fff,stroke:#333
    style F fill:#f66,stroke:#333
    style E fill:#6f6,stroke:#333
```

Hell Mode (`--hell`) requires **3/3 unanimous consensus** on ALL plan elements. The feedback loop has **no iteration limit** — it cycles through Phase 3 → 4 → 5 until every element achieves Strong consensus, or the user manually stops.

Each iteration:
1. Shuts down old committee (prevents position entrenchment)
2. Appends iteration summary to `prior-iterations.md`
3. Creates new tasks, spawns new agents with cumulative context
4. New committee receives all prior analysis + debate history

### Artifact Persistence

All intermediate results are persisted to `.omc/state/plan-{short-id}/` to survive context compression:

```mermaid
graph LR
    subgraph Artifacts
        A[context.md]
        B[analyst-findings.md]
        C[da-synthesis.md]
        D[committee-debate.md]
        E[prior-iterations.md]
    end

    P0[Phase 0] -.->|write| A
    P3[Phase 3] -.->|write| B
    P4[Phase 4] -.->|write| C
    P5[Phase 5] -.->|write| D
    FL[Feedback Loop] -.->|append| E

    B -.->|read| P4
    C -.->|read| P5
    D -.->|read| P3
    E -.->|read| P3
    E -.->|read| P4
    E -.->|read| P5
    A -.->|read| P3
    A -.->|read| P4
    A -.->|read| P5
```

### Consensus Levels

| Level | Condition | Normal Mode | Hell Mode |
|-------|-----------|-------------|-----------|
| **Strong** | 3/3 agree | Phase 6 | Phase 6 |
| **Working** | 2/3, 1 dissent | Phase 6 | Feedback Loop |
| **Partial** | 60%+ elements | Phase 6 | Feedback Loop |
| **No Consensus** | <60% | Feedback Loop | Feedback Loop |

**Normal Mode**: max 2 feedback loops, then forced Phase 6.
**Hell Mode**: no limit until 3/3 unanimous.

### Agent Mapping

| Role | Agent Type | Model |
|------|-----------|-------|
| Analyst (complex) | `oh-my-claudecode:analyst` | opus |
| Analyst (standard) | `oh-my-claudecode:architect-medium` | sonnet |
| Devil's Advocate | `oh-my-claudecode:critic` | opus |
| UX Critic | `oh-my-claudecode:architect-medium` | sonnet |
| Engineering Critic | `oh-my-claudecode:architect` | opus |
| Planner | `oh-my-claudecode:planner` | opus |

### File Structure

```
skills/plan/
├── SKILL.md                        # Main skill definition (491 lines)
├── prompts/
│   ├── analyst.md                  # Dynamic analyst template
│   ├── devil-advocate.md           # DA synthesis prompt
│   └── committee/
│       ├── ux-critic.md            # UX Critic prompt
│       ├── engineering-critic.md   # Engineering Critic prompt
│       └── planner.md             # Planner prompt (tie-breaker)
└── templates/
    └── plan-output.md              # Final plan output template
```

## Incident Skill

Multi-perspective incident postmortem with 4 core + 9 extended archetypes, devil's advocate challenge, and optional tribunal review.

```
/prism:incident
```

## PRD Skill

Multi-perspective PRD policy conflict analysis against existing policy documents via podo-docs MCP.

```
/prism:prd path/to/prd.md
```

## Requirements

- Claude Code with Agent Teams enabled:
  ```json
  // ~/.claude/settings.json
  { "env": { "CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS": "1" } }
  ```

## License

See [LICENSE](./LICENSE).
