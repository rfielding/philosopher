# Documentation Index

This directory contains comprehensive context and reference documentation for the Philosopher project.

## Files

### Core Documentation

1. **[CONTEXT.md](CONTEXT.md)**
   - Project overview and purpose
   - Key concepts (CSP, CTL, Model Checking)
   - Use cases and target audience
   - Planned architecture
   - Project status

2. **[CSP_REFERENCE.md](CSP_REFERENCE.md)**
   - Complete CSP language reference
   - Basic operators and syntax
   - Communication patterns
   - Common CSP patterns
   - Refinement concepts
   - Tool ecosystem

3. **[CTL_REFERENCE.md](CTL_REFERENCE.md)**
   - Computational Tree Logic reference
   - Temporal operators explained
   - Path quantifiers (A, E)
   - Common properties (safety, liveness, fairness)
   - Model checking algorithms
   - Practical examples

4. **[EXAMPLES.md](EXAMPLES.md)**
   - Practical usage examples
   - Complete workflows from English to CSP to CTL
   - Common patterns and idioms
   - Tips for writing requirements
   - Expected output formats

5. **[DEVELOPMENT.md](DEVELOPMENT.md)**
   - Development history and context
   - Planned project structure
   - Technology stack considerations
   - Development phases and roadmap
   - Technical challenges and solutions
   - Testing strategy
   - Contributing guidelines

## How to Use This Documentation

### For New Users
1. Start with [CONTEXT.md](CONTEXT.md) to understand what Philosopher does
2. Look at [EXAMPLES.md](EXAMPLES.md) to see it in action
3. Refer to [CSP_REFERENCE.md](CSP_REFERENCE.md) and [CTL_REFERENCE.md](CTL_REFERENCE.md) as needed

### For Developers
1. Read [CONTEXT.md](CONTEXT.md) for project goals
2. Study [DEVELOPMENT.md](DEVELOPMENT.md) for technical details
3. Review [CSP_REFERENCE.md](CSP_REFERENCE.md) and [CTL_REFERENCE.md](CTL_REFERENCE.md) for implementation details
4. Use [EXAMPLES.md](EXAMPLES.md) for test cases

### For Contributors
1. Understand the project vision in [CONTEXT.md](CONTEXT.md)
2. Check [DEVELOPMENT.md](DEVELOPMENT.md) for roadmap and guidelines
3. Study the reference documents to understand the formal methods
4. Use [EXAMPLES.md](EXAMPLES.md) to understand expected behavior

## Document Maintenance

These documents should be kept up-to-date as the project evolves:

- **CONTEXT.md**: Update when project goals or architecture change
- **CSP_REFERENCE.md**: Add new patterns as they're implemented
- **CTL_REFERENCE.md**: Expand as more verification patterns are supported
- **EXAMPLES.md**: Add new examples as features are implemented
- **DEVELOPMENT.md**: Update roadmap as phases are completed

## Relationship Between Documents

```
CONTEXT.md (Start here)
    ├── What is Philosopher?
    ├── Why does it exist?
    └── How does it work?
         ├──> CSP_REFERENCE.md (Deep dive: CSP)
         ├──> CTL_REFERENCE.md (Deep dive: CTL)
         └──> EXAMPLES.md (See it in action)
              └──> DEVELOPMENT.md (Build it yourself)
```

## External Resources

For deeper understanding of the formal methods used in Philosopher:

- **CSP**: [Oxford CSP Page](https://www.cs.ox.ac.uk/projects/concurrency-theory/)
- **CTL**: Model checking textbooks (Baier & Katoen)
- **Tools**: FDR4, NuSMV, SPIN documentation

## Questions or Feedback

If you have questions about the documentation or suggestions for improvement, please open an issue on the GitHub repository.
