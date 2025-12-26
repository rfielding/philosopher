# Philosopher

A Philosophy Calculator to turn an English chat into CSP that can be turned into CTL and diagrams for requirements.

## Overview

Philosopher bridges the gap between natural language requirements and formal verification. It converts conversational English into formal CSP (Communicating Sequential Processes) specifications, which can then be transformed into CTL (Computational Tree Logic) for model checking, diagram generation, and formal proofs.

## Quick Start

(Coming soon - project is currently in the planning phase)

## Documentation

Comprehensive documentation is available in the `docs/` directory:

- **[CONTEXT.md](docs/CONTEXT.md)** - Project overview, key concepts, and architecture
- **[CSP_REFERENCE.md](docs/CSP_REFERENCE.md)** - Complete CSP language reference with examples
- **[CTL_REFERENCE.md](docs/CTL_REFERENCE.md)** - CTL temporal logic reference and patterns
- **[EXAMPLES.md](docs/EXAMPLES.md)** - Practical examples showing how to use Philosopher
- **[DEVELOPMENT.md](docs/DEVELOPMENT.md)** - Development context, roadmap, and technical details

## What Can You Do With Philosopher?

1. **Natural Language → Formal Specifications**: Write requirements in English, get CSP
2. **Model Checking**: Verify system properties automatically
3. **Diagram Generation**: Visualize system behavior
4. **Formal Proofs**: Prove correctness properties
5. **Requirements Analysis**: Catch specification errors early

## Example

```
Input: "A vending machine accepts coins. After receiving a coin, 
        the user can choose either coffee or tea."

Output (CSP): VM = coin → (coffee → VM □ tea → VM)

Output (CTL): AG(coin → AF(coffee ∨ tea))
              "After every coin, a drink is eventually dispensed"

Output (Diagram): [Visual state machine showing the flow]
```

See [EXAMPLES.md](docs/EXAMPLES.md) for many more examples.

## Project Status

🚧 **In Development** - This project is in the initial planning phase. Documentation and context are being organized to facilitate development.

## Contributing

This project is in early stages. More information about contributing will be added as the project develops.

## References

- Tony Hoare - *Communicating Sequential Processes* (1985)
- Clarke & Emerson - *Design and Synthesis Using Branching Time Temporal Logic* (1981)
- Baier & Katoen - *Principles of Model Checking* (2008)

## License

(To be determined)
