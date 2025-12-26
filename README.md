# philosopher
A Philosophy Calculator to turn an English chat into CSP that can be turned into CTL and diagrams for requirements.

A philosophy calculator for temporal logic and protocol design.

## What Is This?

Most temporal logic tools (NuSMV, SPIN, TLA+) require you to already know what you want to specify. They're verification tools, not thinking tools.

BoundedLISP is different. It's a **conversational specification environment** where you:

1. **Sketch** half-formed ideas on a whiteboard (state machines, message flows, LaTeX math)
2. **Discuss** with an AI to refine your thinking  
3. **Formalize** sketches into executable actor specifications
4. **Verify** properties using CTL model checking
5. **Iterate** until the specification captures your intent

Think of it as pair-programming for protocol design. You and the AI are in a conference room with a whiteboard, working toward a formal specification that you might not have been able to write directly.

## Why Temporal Logic?

Temporal logic lets you express properties like:
- "Every request **eventually** gets a response" — `AG(request → AF(response))`
- "The system **never** deadlocks" — `AG(EX(true))`  
- "Once started, the protocol **always** terminates" — `AF(done)`

These are the properties that matter for distributed systems, protocols, and concurrent programs. But temporal logic is rarely accessible outside academic papers and expert tools.

## Quick Start

```bash
# Run tests
cat prologue.lisp tests.lisp | go run main.go -repl

# Start interactive server
export ANTHROPIC_API_KEY=sk-ant-...
go run main.go

# Open http://localhost:8080 in browser
```

## The Language

**BoundedLISP is its own dialect** - not Scheme, not Common Lisp. Key differences:

| Feature | Scheme | Common Lisp | BoundedLISP |
|---------|--------|-------------|-------------|
| False values | `#f` only | `nil` only | `nil`, `false`, `'()`, `0`, `""` |
| Booleans | `#t`/`#f` | `t`/`nil` | `true`/`false` |
| Simple let | `(let ((x 1)) ...)` | `(let ((x 1)) ...)` | `(let x 1 ...)` |
| Else in cond | `else` | `t` | `true` |

See [DIALECT.md](DIALECT.md) for the complete language reference.

## Compared to Other Tools

| Tool | Approach | Learning Curve | Conversation? |
|------|----------|----------------|---------------|
| **NuSMV** | SMV input language | Steep | No |
| **SPIN** | Promela + LTL | Steep | No |
| **TLA+** | Mathematical notation | Very steep | No |
| **Alloy** | Relational logic | Moderate | No |
| **LogiCola** | Educational exercises | Low | No |
| **BoundedLISP** | Actors + CTL | Low | **Yes** |

The key difference: you don't need to know temporal logic to start. Describe what you want, sketch on the whiteboard, and let the conversation lead you to a formal specification.

## Usage Modes

### Web UI (default)
```bash
go run main.go
```
Opens a web interface with:
- **Chat panel** - describe your system, ask questions
- **Whiteboard** - sketch ideas before formalizing (LaTeX, diagrams)
- **Specification** - rendered markdown with diagrams
- **LISP** - the executable code

### Console + Server
When the server is running, you can also type in the terminal. Useful for quick queries without switching to the browser.

### REPL Only
```bash
go run main.go -repl
```
Pure LISP interpreter, no server.

### File Execution
```bash
go run main.go myspec.lisp
```

## The Actor Model

Actors are the source of truth. Each actor:
- Has a mailbox (bounded queue)
- Processes messages sequentially
- Uses `become` to carry state forward

```lisp
(define (server-loop request-count)
  (let msg (receive!)
    (send-to! (first msg) 'ack)
    (list 'become (list 'server-loop (+ request-count 1)))))

(spawn-actor 'server 16 '(server-loop 0))
```

## Key Primitives

| Function | Purpose |
|----------|---------|
| `spawn-actor` | Create an actor with mailbox |
| `send-to!` | Send message to actor |
| `receive!` | Block until message arrives |
| `(list 'become ...)` | Continue with new state |
| `'done` | Actor terminates |

## Properties (CTL)

Specify what must be true:

```lisp
; Every request eventually gets a response
(defproperty 'responsive
  (AG (ctl-implies (prop 'request) (AF (prop 'response)))))

; No deadlocks - always can make progress
(defproperty 'no-deadlock
  (AG (EX (prop 'true))))
```

## Probability Distributions

For modeling stochastic systems:

```lisp
(exponential u rate)        ; Inter-arrival times
(normal u1 u2 mean stddev)  ; Gaussian
(bernoulli u p)             ; Success/failure
(discrete-uniform u min max) ; Random integer
```

## Workflow

1. **Sketch** - Draw rough ideas on the whiteboard
   - "Client sends request, server responds or times out"
   - State machine sketches
   - Message sequence ideas

2. **Discuss** - Refine with the AI
   - "What if the server crashes mid-request?"
   - "Add retry logic"

3. **Formalize** - Convert to LISP
   - AI generates actor code
   - Properties are defined

4. **Verify** - Check properties
   - Model checking runs
   - Counterexamples shown if failed

5. **Iterate** - Refine and repeat

## Files

| File | Purpose |
|------|---------|
| `main.go` | Interpreter, scheduler, web server |
| `prologue.lisp` | Runtime library (actors, CTL, distributions) |
| `tests.lisp` | 158 unit tests |
| `DIALECT.md` | Complete language reference |

## Environment Variables

| Variable | Purpose |
|----------|---------|
| `ANTHROPIC_API_KEY` | Claude API key |
| `OPENAI_API_KEY` | GPT-4 API key (alternative) |
| `KRIPKE_PORT` | Server port (default: 8080) |

## Running Tests

```bash
make test
# or
cat prologue.lisp tests.lisp | go run main.go -repl
```

## Why "Philosophy Calculator"?

Temporal logic—reasoning about what *must* happen, what *might* happen, what happens *eventually* or *always*—has been studied by philosophers and logicians for decades. But the tools to actually *compute* with these ideas have remained locked in academic silos.

A calculator democratized arithmetic. A spreadsheet democratized financial modeling. BoundedLISP aims to democratize temporal reasoning about systems.

You shouldn't need a PhD to ask "will my protocol eventually terminate?" or "can this system deadlock?" You should be able to sketch your intuition, have a conversation, and arrive at a formal answer.

## License

MIT
