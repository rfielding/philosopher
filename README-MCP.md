# MCP Support for Philosopher

## Usage

```bash
# Build
go mod tidy
go build -o philosopher

# Test
go test -v

# Run modes
./philosopher              # Web UI (default)
./philosopher -repl        # Interactive REPL
./philosopher file.lisp    # Run file
./philosopher -mcp         # MCP server (stdio)
./philosopher -mcp-sse 3000  # MCP server (HTTP/SSE)
```

## Claude Desktop Config

Add to `~/.config/claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "philosopher": {
      "command": "/path/to/philosopher",
      "args": ["-mcp"]
    }
  }
}
```

## MCP Tools

| Tool | Description |
|------|-------------|
| `eval_lisp` | Evaluate BoundedLISP code |
| `run_simulation` | Run scheduler for N steps |
| `spawn_actor` | Create actor with name, mailbox, initial state |
| `send_message` | Send message to actor |
| `get_metrics` | Get registry metrics |
| `get_actors` | Get actor state(s) |
| `reset` | Clear all actors |
| `csp_status` | Get CSP violations |
| `csp_enforce` | Enable/disable CSP enforcement |

## Embedded Datalog

Temporal Datalog is embedded for fact collection and queries.

### Facts

```lisp
;; Assert facts
(assert! 'parent 'tom 'bob)
(assert! 'owns 'alice 100)

;; Assert with timestamp
(assert-at! 5 'event 'start)

;; Retract
(retract! 'owns 'alice 100)
```

### Rules

```lisp
;; grandparent(X, Z) :- parent(X, Y), parent(Y, Z)
(rule 'grandparent
  '(grandparent ?x ?z)
  '(parent ?x ?y)
  '(parent ?y ?z))

;; With negation
(rule 'can-fly
  '(can-fly ?x)
  '(bird ?x)
  '(not (cannot-fly ?x)))

;; With builtins
(rule 'passed
  '(passed ?x)
  '(score ?x ?s)
  '(>= ?s 75))
```

### Queries

```lisp
;; Simple query
(query 'parent 'tom '?x)       ; Tom's children

;; Conjunction
(query-all '(parent ?x ?y) '(parent ?y ?z))

;; CTL operators
(eventually? '(state done))    ; EF
(never? '(error ?x))           ; AG(not ...)
(always? '(valid ?x))          ; AG
```

### Temporal Queries

```lisp
;; Events at specific time
(query 'at-time 'event '?e 5)

;; Events before/after time
(query 'before 'event '?e 10)
(query 'after 'event '?e 3)

;; Events in time range
(query 'between 'event '?e 2 8)
```

### CSP Verification via Datalog

```lisp
;; Define CSP violation rule
(rule 'csp-violation
  '(csp-violation ?actor ?var ?time)
  '(effect ?actor set ?var)
  '(not (guard-before ?actor)))

;; Query violations
(query 'csp-violation '?who '?what '?when)

;; Deadlock detection
(rule 'deadlock
  '(deadlock ?a ?b)
  '(waiting-for ?a ?b)
  '(waiting-for ?b ?a))

(query 'deadlock '?a '?b)
```

## Files

| File | Description |
|------|-------------|
| `main.go` | Full BoundedLISP + MCP + CSP |
| `datalog.go` | Datalog interpreter (~500 lines) |
| `datalog_builtins.go` | LISP integration |
| `datalog_test.go` | 25 Go tests |
| `datalog-tests.lisp` | LISP integration tests |
| `breadco.lisp` | Multi-actor simulation example |

## Test Results

```
=== Test Count ===
25 tests (37 including subtests)

=== All Pass ===
TestTermEquality, TestUnifyAtoms, TestUnifyVariables,
TestUnifyLists, TestAssertAndQuery, TestRetract,
TestSimpleRule, TestTransitiveRule, TestNegation,
TestBuiltinComparison, TestTemporalAtTime, TestTemporalBefore,
TestTemporalAfter, TestTemporalBetween, TestAlways,
TestEventually, TestNever, TestCSPViolationDetection,
TestMessageFlowTracking, TestDeadlockDetection,
TestValueToTerm, TestTermToValue, TestBreadCoSimulation
```
