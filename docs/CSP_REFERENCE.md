# CSP (Communicating Sequential Processes) Reference

## Overview

CSP is a formal language for describing patterns of interaction in concurrent systems, developed by Tony Hoare in 1978. It provides a mathematical foundation for reasoning about concurrent processes and their communications.

## Basic Concepts

### Processes
A process is a fundamental unit in CSP that can perform actions and communicate with other processes.

**Syntax:**
- `P` - Process name
- `STOP` - Deadlocked process (does nothing)
- `SKIP` - Successfully terminated process

### Events
Events represent atomic actions that processes can perform.

**Examples:**
- `coin` - Insert a coin
- `coffee` - Dispense coffee
- `button.press` - Press a button

### Basic Operators

#### Prefix (`→`)
```csp
a → P
```
Performs action `a` then behaves like process `P`.

**Example:**
```csp
coin → coffee → STOP
```

#### Choice (`□`)
```csp
P □ Q
```
External choice: the environment chooses between P and Q.

**Example:**
```csp
coffee □ tea
```

#### Internal Choice (`⊓`)
```csp
P ⊓ Q
```
Internal choice: the process chooses between P and Q.

#### Sequential Composition (`;`)
```csp
P ; Q
```
Do P, then Q when P terminates successfully.

#### Parallel Composition (`||`)
```csp
P || Q
```
P and Q run in parallel and must synchronize on shared events.

#### Interleaving (`|||`)
```csp
P ||| Q
```
P and Q run independently with no synchronization.

### Recursion
Processes can be defined recursively:

```csp
CLOCK = tick → CLOCK
```

### Process Definitions

**Example: Vending Machine**
```csp
VM = coin → (coffee → VM □ tea → VM)
```

**Example: Buffer**
```csp
BUFFER = in?x → out!x → BUFFER
```

## Communication

### Input (`?`)
```csp
channel?x
```
Receive value `x` on `channel`.

### Output (`!`)
```csp
channel!v
```
Send value `v` on `channel`.

## Common Patterns

### Mutual Exclusion
```csp
MUTEX = acquire → critical → release → MUTEX
```

### Producer-Consumer
```csp
PRODUCER = produce → buffer!item → PRODUCER
CONSUMER = buffer?item → consume → CONSUMER
SYSTEM = PRODUCER || CONSUMER
```

### Dining Philosophers
```csp
PHIL(i) = pickup.i → pickup.(i+1 mod n) → 
          eat → putdown.i → putdown.(i+1 mod n) → PHIL(i)
```

## Refinement

CSP supports refinement checking:
- **Traces Refinement**: P ⊑T Q (Q's traces are a subset of P's)
- **Failures Refinement**: P ⊑F Q
- **Failures-Divergences**: P ⊑FD Q

## Tools

- **FDR4**: Refinement checker for CSP
- **ProBE**: Process Behavior Explorer
- **PAT**: Process Analysis Toolkit

## References

- Hoare, C. A. R. (1985). "Communicating Sequential Processes"
- Roscoe, A. W. (2010). "Understanding Concurrent Systems"
- [CSP Formal Methods](https://www.cs.ox.ac.uk/projects/concurrency-theory/)

## Relevance to Philosopher

In the Philosopher project, CSP serves as the intermediate formal representation that:
1. Captures the structure and behavior from natural language requirements
2. Provides a formal foundation for verification
3. Can be translated to CTL for model checking
4. Enables diagram generation and visualization
