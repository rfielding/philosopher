# Philosopher

**A Philosophy Calculator for Formal Verification**

Philosopher is a tool that transforms natural English descriptions of concurrent systems into CSP (Communicating Sequential Processes), enabling formal verification, model checking, diagram generation, and mathematical proofs.

## Table of Contents

- [What is Philosopher?](#what-is-philosopher)
- [What is CSP?](#what-is-csp)
- [Key Features](#key-features)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [Usage Guide](#usage-guide)
  - [Converting English to CSP](#converting-english-to-csp)
  - [Model Checking](#model-checking)
  - [Generating Diagrams](#generating-diagrams)
  - [Proving Properties](#proving-properties)
- [Examples](#examples)
- [Use Cases](#use-cases)
- [Contributing](#contributing)
- [License](#license)

## What is Philosopher?

Philosopher bridges the gap between natural language descriptions and formal mathematical models. It allows you to:

1. **Describe systems in English** - Write natural descriptions of concurrent processes and their interactions
2. **Generate formal CSP** - Automatically convert these descriptions into precise CSP notation
3. **Verify correctness** - Use model checking to verify that your system meets its requirements
4. **Visualize behavior** - Generate diagrams showing process interactions and state transitions
5. **Prove properties** - Mathematically prove that your system has desired properties (safety, liveness, deadlock-freedom)

## What is CSP?

**CSP (Communicating Sequential Processes)** is a formal language developed by Tony Hoare for describing patterns of interaction in concurrent systems. It's particularly useful for:

- Modeling concurrent and distributed systems
- Describing how processes communicate and synchronize
- Reasoning about system behavior mathematically
- Detecting deadlocks, livelocks, and race conditions
- Verifying that systems meet their specifications

CSP represents systems as processes that communicate through channels, making it ideal for analyzing protocols, concurrent algorithms, and distributed systems.

**CTL (Computation Tree Logic)** is a temporal logic used for specifying properties about the behavior of systems over time, often used in conjunction with CSP for model checking.

## Key Features

### 🗣️ Natural Language Input
Write system descriptions in plain English. Philosopher understands descriptions of:
- Processes and their behaviors
- Communication channels and events
- Synchronization requirements
- Temporal properties and constraints

### 📝 CSP Generation
Automatically generates formal CSP notation including:
- Process definitions
- Channel declarations
- Parallel composition operators
- Choice and sequential operators
- Hiding and renaming

### ✅ Model Checking
Verify system properties such as:
- **Deadlock freedom** - Ensure processes never get stuck
- **Livelock freedom** - Verify processes make progress
- **Safety properties** - Check that bad things never happen
- **Liveness properties** - Ensure good things eventually happen
- **Refinement** - Verify that implementations match specifications

### 📊 Diagram Generation
Create visual representations including:
- Process interaction diagrams
- State transition diagrams
- Communication flow charts
- Event traces

### 🔬 Proof Support
Generate and verify mathematical proofs for:
- Process equivalence
- Refinement relationships
- Temporal properties
- Behavioral properties

## Installation

```bash
# Clone the repository
git clone https://github.com/rfielding/philosopher.git
cd philosopher

# Install dependencies (adjust based on your implementation)
# npm install  # For Node.js implementation
# pip install -r requirements.txt  # For Python implementation
# go get ./...  # For Go implementation

# Run philosopher
# ./philosopher  # Adjust based on your build system
```

## Quick Start

Here's a simple example to get you started:

```bash
# Example: Describe a simple producer-consumer system
echo "A producer creates items and sends them to a consumer. 
The consumer receives items and processes them. 
They communicate through a buffer channel." | philosopher --output csp
```

This generates CSP like:
```csp
PRODUCER = produce -> buffer!item -> PRODUCER

CONSUMER = buffer?item -> consume -> CONSUMER

SYSTEM = PRODUCER [| {buffer} |] CONSUMER
```

## Usage Guide

### Converting English to CSP

Philosopher interprets natural language descriptions and converts them to formal CSP notation.

**Example 1: Simple Process**
```
Input: "A traffic light alternates between red, yellow, and green states."

Output CSP:
TRAFFIC_LIGHT = red -> yellow -> green -> TRAFFIC_LIGHT
```

**Example 2: Communicating Processes**
```
Input: "A sender transmits a message. A receiver acknowledges receipt."

Output CSP:
SENDER = send!msg -> ack?ok -> SENDER
RECEIVER = send?msg -> ack!ok -> RECEIVER
SYSTEM = SENDER [| {send, ack} |] RECEIVER
```

**Example 3: Choice and Alternatives**
```
Input: "A user can either login or register, then proceed to the main menu."

Output CSP:
USER = (login -> MAIN_MENU) [] (register -> MAIN_MENU)
MAIN_MENU = menu -> USER
```

### Model Checking

Verify properties of your system:

```bash
# Check for deadlocks
philosopher --input system.csp --check deadlock-free

# Verify a safety property
philosopher --input system.csp --check "safe: never(error_state)"

# Verify a liveness property  
philosopher --input system.csp --check "live: eventually(completed)"

# Check refinement
philosopher --input spec.csp impl.csp --check "impl refines spec"
```

**Common Properties to Check:**

1. **Deadlock Freedom**: `--check deadlock-free`
   - Ensures the system never reaches a state where no progress is possible

2. **Livelock Freedom**: `--check livelock-free`
   - Verifies that the system doesn't loop infinitely without making progress

3. **Determinism**: `--check deterministic`
   - Checks if the system behaves deterministically

4. **Trace Refinement**: `--check "T1 traces T2"`
   - Verifies that implementation traces are contained in specification traces

### Generating Diagrams

Create visual representations of your systems:

```bash
# Generate a process interaction diagram
philosopher --input system.csp --diagram interaction --output system.png

# Generate a state transition diagram
philosopher --input system.csp --diagram states --output states.svg

# Generate an event trace diagram
philosopher --input system.csp --diagram trace --output trace.pdf

# Generate all diagrams
philosopher --input system.csp --diagram all --output-dir ./diagrams/
```

**Diagram Types:**

- **Interaction Diagrams**: Show how processes communicate
- **State Diagrams**: Display state transitions and events
- **Trace Diagrams**: Illustrate execution sequences
- **Dependency Graphs**: Show process dependencies

### Proving Properties

Generate and verify mathematical proofs:

```bash
# Prove deadlock freedom
philosopher --input system.csp --prove deadlock-free

# Prove a custom property
philosopher --input system.csp --prove "property: always(safe_state)"

# Generate a proof outline
philosopher --input system.csp --prove refinement --format outline

# Interactive proof assistant
philosopher --input system.csp --prove --interactive
```

## Examples

### Example 1: Dining Philosophers

Classic concurrency problem:

```
Input: "Five philosophers sit at a round table. Between each pair is a fork.
A philosopher must pick up both adjacent forks to eat. After eating, they put
down both forks and think."

Generated CSP:
PHIL(i) = think -> pickupLeft.i -> pickupRight.i -> eat -> 
          putdownLeft.i -> putdownRight.i -> PHIL(i)

FORK(i) = pickupLeft.i -> putdownLeft.i -> FORK(i)
        [] pickupRight.i -> putdownRight.i -> FORK(i)

SYSTEM = ||| i:{0..4} @ (PHIL(i) [| {pickupLeft.i, pickupRight.i, 
                                      putdownLeft.i, putdownRight.i} |] 
                          FORK(i))
```

Check for deadlock:
```bash
philosopher --input dining_philosophers.csp --check deadlock-free
# Result: DEADLOCK DETECTED (all philosophers pick up left fork)
```

### Example 2: Client-Server Protocol

```
Input: "A client sends a request and waits for a response. The server
receives requests, processes them, and sends responses."

Generated CSP:
CLIENT = request!msg -> response?data -> CLIENT

SERVER = request?msg -> process -> response!data -> SERVER

PROTOCOL = CLIENT [| {request, response} |] SERVER
```

Verify correctness:
```bash
philosopher --input protocol.csp --check deadlock-free
# Result: DEADLOCK FREE

philosopher --input protocol.csp --check "live: eventually(response)"
# Result: VERIFIED
```

### Example 3: Mutex Algorithm

```
Input: "Two processes compete for a critical section. Only one can enter
at a time. They use a lock to coordinate access."

Generated CSP:
PROCESS(i) = request.i -> enter.i -> critical.i -> 
             exit.i -> release.i -> PROCESS(i)

LOCK = request?i -> enter!i -> exit?i -> release?i -> LOCK

SYSTEM = (PROCESS(0) ||| PROCESS(1)) [| {request, enter, exit, release} |] LOCK
```

Prove mutual exclusion:
```bash
philosopher --input mutex.csp --prove "mutex: never(critical.0 && critical.1)"
# Result: PROVEN
```

## Use Cases

### 1. Protocol Design and Verification
- Design communication protocols
- Verify protocol correctness
- Detect race conditions and deadlocks
- Ensure message ordering properties

### 2. Concurrent Algorithm Analysis
- Model concurrent algorithms
- Verify mutual exclusion properties
- Check for deadlocks and livelocks
- Prove correctness properties

### 3. Distributed Systems
- Model distributed consensus algorithms
- Verify eventual consistency
- Analyze failure scenarios
- Check safety and liveness properties

### 4. Hardware Design
- Model hardware components and their interactions
- Verify synchronization protocols
- Detect timing issues
- Ensure correct behavior under all scenarios

### 5. Requirements Engineering
- Formalize system requirements
- Verify requirement consistency
- Generate test cases from specifications
- Document system behavior precisely

### 6. Education and Research
- Teach formal methods and concurrency
- Research new verification techniques
- Prototype new concurrent systems
- Experiment with process algebras

## Contributing

We welcome contributions! Here's how you can help:

1. **Report Issues**: Found a bug? Report it on our issue tracker
2. **Suggest Features**: Have ideas for improvements? We'd love to hear them
3. **Submit PRs**: Fix bugs or add features
4. **Improve Documentation**: Help make our docs clearer
5. **Share Examples**: Add interesting use cases and examples

```bash
# Fork the repository
# Create a feature branch
git checkout -b feature/amazing-feature

# Make your changes
# Commit with clear messages
git commit -m "Add amazing feature"

# Push and create a pull request
git push origin feature/amazing-feature
```

## License

[Add appropriate license information]

## Contact and Support

- **GitHub**: [rfielding/philosopher](https://github.com/rfielding/philosopher)
- **Issues**: [Report bugs or request features](https://github.com/rfielding/philosopher/issues)
- **Discussions**: [Join the conversation](https://github.com/rfielding/philosopher/discussions)

## Additional Resources

- [CSP Tutorial](https://www.cs.ox.ac.uk/projects/concurrency-theory/)
- [Model Checking Handbook](https://www.springer.com/gp/book/9781402063554)
- [Tony Hoare's CSP Book](https://www.cs.ox.ac.uk/people/bill.roscoe/publications/68b.pdf)
- [FDR Model Checker](https://www.cs.ox.ac.uk/projects/fdr/)

---

**Note**: Philosopher is a tool for formal verification and model checking. While it helps identify issues in system designs, always combine formal methods with other verification techniques for critical systems.
