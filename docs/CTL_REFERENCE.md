# CTL (Computational Tree Logic) Reference

## Overview

Computational Tree Logic (CTL) is a branching-time temporal logic used in formal verification and model checking. It allows us to specify and verify properties about the possible execution paths of a system.

## Temporal Logic Basics

CTL extends propositional logic with temporal operators that reason about time and possible futures. Unlike linear temporal logic (LTL), CTL reasons about a tree of possible execution paths rather than individual paths.

## Syntax

### State Formulas
A state formula describes properties that hold at a particular state.

**Atomic Propositions:**
- `p, q, r` - Boolean properties of states

**Boolean Operators:**
- `¬φ` - NOT φ
- `φ ∧ ψ` - φ AND ψ
- `φ ∨ ψ` - φ OR ψ
- `φ → ψ` - φ implies ψ

### Path Quantifiers
- `A` - For all paths (universal)
- `E` - There exists a path (existential)

### Temporal Operators
- `X φ` - neXt: φ holds in the next state
- `F φ` - Future: φ holds in some future state
- `G φ` - Globally: φ holds in all future states
- `φ U ψ` - Until: φ holds until ψ becomes true

## CTL Operators

CTL formulas combine path quantifiers with temporal operators:

### `AX φ` (All neXt)
φ holds in all immediate successor states
```
Example: AX(ready) - "After any action, the system is ready"
```

### `EX φ` (Exists neXt)
φ holds in at least one immediate successor state
```
Example: EX(error) - "An error state is immediately reachable"
```

### `AF φ` (All Future)
φ inevitably holds in the future on all paths
```
Example: AF(terminated) - "The system will eventually terminate"
```

### `EF φ` (Exists Future)
φ holds in some future state on at least one path
```
Example: EF(goal) - "It's possible to reach the goal state"
```

### `AG φ` (All Globally)
φ holds in all states on all paths
```
Example: AG(safe) - "The system is always safe"
```

### `EG φ` (Exists Globally)
φ holds in all states on at least one infinite path
```
Example: EG(running) - "The system can run forever"
```

### `A[φ U ψ]` (All Until)
On all paths, φ holds until ψ becomes true
```
Example: A[requesting U granted] - "All requests eventually get granted"
```

### `E[φ U ψ]` (Exists Until)
On some path, φ holds until ψ becomes true
```
Example: E[trying U success] - "Success is achievable while trying"
```

## Common Properties

### Safety Properties
"Nothing bad ever happens"
```ctl
AG(¬error)          - "No errors occur"
AG(critical → mutex) - "Critical sections are mutually exclusive"
```

### Liveness Properties
"Something good eventually happens"
```ctl
AF(response)                    - "Every request gets a response"
AG(request → AF(acknowledge))   - "Every request is eventually acknowledged"
```

### Fairness Properties
```ctl
AG(AF(scheduled)) - "Every process is scheduled infinitely often"
```

### Reachability Properties
```ctl
EF(goal)    - "The goal state is reachable"
AG(EF(home)) - "From any state, we can return home"
```

### Deadlock Freedom
```ctl
AG(EX(true)) - "From every state, some next state exists"
```

## Examples

### Traffic Light Controller
```ctl
AG(red → AX(¬green))           - "Red is never immediately followed by green"
AG(yellow → AX(red))            - "Yellow is always followed by red"
AG(request → AF(green))         - "Every request eventually gets a green light"
```

### Mutual Exclusion Protocol
```ctl
AG(¬(critical₁ ∧ critical₂))   - "Never both in critical section"
AG(requesting₁ → AF(critical₁)) - "Every request eventually succeeds"
AG(critical₁ → AF(¬critical₁)) - "Critical section is eventually exited"
```

### Producer-Consumer
```ctl
AG(full → AX(¬produce))         - "Can't produce when buffer is full"
AG(empty → AX(¬consume))        - "Can't consume when buffer is empty"
AG(EF(¬full) ∧ EF(¬empty))     - "Buffer is neither always full nor always empty"
```

## Model Checking CTL

Model checking algorithms verify whether a system model satisfies a CTL formula:

1. **Input**: 
   - Kripke structure (state transition system)
   - CTL formula φ

2. **Output**:
   - True/False: whether the system satisfies φ
   - Counterexample: if φ is violated

3. **Complexity**:
   - Time: O(|M| × |φ|)
   - Where |M| is model size, |φ| is formula length

## Tools

- **NuSMV**: Symbolic model checker
- **SPIN**: Explicit-state model checker
- **UPPAAL**: Timed automata verification
- **PAT**: Process Analysis Toolkit
- **PRISM**: Probabilistic model checker

## CTL vs CTL*

**CTL**: Path quantifiers and temporal operators are paired
- `AF(p)`, `EG(p)` ✓
- `F(p)` alone ✗

**CTL***: More expressive, allows arbitrary nesting
- `A(FG(p))` - eventually always p
- `A(GF(p) → GF(q))` - if p infinitely often, then q infinitely often

## References

- Clarke, E. M., & Emerson, E. A. (1981). "Design and Synthesis of Synchronization Skeletons Using Branching Time Temporal Logic"
- Emerson, E. A. (1990). "Temporal and Modal Logic"
- Baier, C., & Katoen, J. P. (2008). "Principles of Model Checking"

## Relevance to Philosopher

In the Philosopher project, CTL is used for:
1. **Specification**: Expressing temporal properties of systems
2. **Verification**: Model checking CSP specifications against CTL properties
3. **Proof Generation**: Generating formal proofs of system correctness
4. **Requirements**: Formalizing natural language requirements as CTL formulas
5. **Analysis**: Checking safety, liveness, and fairness properties
