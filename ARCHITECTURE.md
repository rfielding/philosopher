# Philosopher - Tool-Based Architecture

## Overview

The key insight: **LLMs write English, tools do the deterministic work**.

```
┌──────────────────────────────────────────────────────────┐
│  User: "Model a bakery with inventory management"        │
└──────────────────────────────────────────────────────────┘
                          │
                          ▼
┌──────────────────────────────────────────────────────────┐
│  LLM (cheap model) outputs English + tool placeholders   │
│                                                          │
│  "Here's the StoreFront actor that manages inventory."   │
│                                                          │
│  {{state_diagram actor="storefront"}}                    │
│                                                          │
│  "This property ensures we never go negative:"           │
│                                                          │
│  {{property formula="AG(inventory >= 0)"}}               │
│                                                          │
│  "Here are the sales collected during simulation:"       │
│                                                          │
│  {{facts_table predicate="sale" limit=5}}                │
└──────────────────────────────────────────────────────────┘
                          │
                          ▼
┌──────────────────────────────────────────────────────────┐
│  Deterministic Tool Substitution (no LLM needed)         │
│                                                          │
│  {{state_diagram}} → query actor registry → mermaid      │
│  {{property}}      → run CTL check → result box          │
│  {{facts_table}}   → query Datalog → HTML table          │
│  {{tla_spec}}      → translate actor → TLA+ code         │
│  {{alloy_spec}}    → translate actor → Alloy code        │
└──────────────────────────────────────────────────────────┘
                          │
                          ▼
┌──────────────────────────────────────────────────────────┐
│  Final output to user:                                   │
│  - Rendered mermaid diagrams                             │
│  - Property check results                                │
│  - Fact tables                                           │
│  - Optional: TLA+, Alloy, SPIN specifications            │
│                                                          │
│  User NEVER sees LISP (it's an internal protocol)        │
└──────────────────────────────────────────────────────────┘
```

## Available Tools

### Visualization Tools

| Tool | Input | Output |
|------|-------|--------|
| `state_diagram` | `actor="name"` | mermaid stateDiagram-v2 |
| `sequence_diagram` | `actors="a,b,c"` | mermaid sequenceDiagram |
| `metrics_chart` | `metrics="x,y" title="..."` | mermaid xychart |

### Verification Tools

| Tool | Input | Output |
|------|-------|--------|
| `property` | `formula="AG(...)" name="..."` | pass/fail box |
| `facts_table` | `predicate="sale" limit=10` | markdown table |

### Definition Tools (via MCP)

| Tool | Input | Output |
|------|-------|--------|
| `define_actor` | states, transitions | internal actor |
| `define_rule` | head, body | Datalog rule |
| `run_simulation` | steps, seed | execute actors |

### Export Tools

| Tool | Input | Output |
|------|-------|--------|
| `tla_spec` | `actor="name"` | TLA+ specification |
| `alloy_spec` | `actor="name"` | Alloy specification |

## Example Flow

### User Prompt
```
Model a producer-consumer system where:
- Producer makes 10 items/tick
- Consumer consumes 5 items/tick
- Verify no deadlock occurs
```

### LLM Output (raw)
```markdown
## Producer-Consumer Model

I'll create two actors that communicate via messages.

{{define_actor name="producer" ... }}
{{define_actor name="consumer" ... }}

Here's the state machine for the producer:

{{state_diagram actor="producer"}}

And the message flow between them:

{{sequence_diagram actors="producer,consumer"}}

## Verification

{{property name="No Deadlock" formula="AG(not deadlock)"}}
{{property name="Bounded Queue" formula="AG(queue_size <= 100)"}}

## Simulation Results

After 50 ticks:

{{facts_table predicate="produced" limit=5}}
{{facts_table predicate="consumed" limit=5}}

{{metrics_chart title="50-Tick Simulation" metrics="produced,consumed,queue"}}
```

### After Tool Substitution

The `{{...}}` placeholders become actual rendered content:
- Mermaid diagrams render in the UI
- Property boxes show ✓ or ✗
- Tables show real data from simulation
- Charts show actual metrics

## Why This Architecture?

1. **LLM doesn't write code** - just English + tool calls
2. **Deterministic outputs** - diagrams/tables are generated from real data
3. **Can't hallucinate syntax** - tools validate inputs
4. **Cheap models work** - GPT-3.5-turbo can emit placeholders
5. **Swappable backends** - want SPIN instead of TLA+? Add a tool
6. **Auditable** - users can inspect what tools generated

## LISP as Internal Protocol

The BoundedLISP is an **undocumented communication protocol** between:
- The tool implementations
- The actor runtime
- The Datalog engine

Users may eventually read/edit LISP when requirements get complex, but that's a power-user feature, not the primary interface.

## MCP Integration

When running as MCP server:
1. Tools are exposed as MCP tool definitions
2. LLM calls tools directly (not placeholders)
3. Results returned as tool responses
4. UI renders results

```bash
./philosopher --mcp  # stdio mode for Claude Desktop
```
