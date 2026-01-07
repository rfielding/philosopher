# Philosopher Test Prompts

These are **unit tests** - each has expected visual outputs and pass/fail criteria.

## Test Suite

| Test | What it tests | Expected Facts | Key Property |
|------|--------------|----------------|--------------|
| 01 | Basic counter | 5 | AG(count >= 0) |
| 02 | Message passing | 12 | sent implies received |
| 03 | Deadlock detection | 2 + derived | AG(not deadlock) = false |
| 04 | BreadCo simulation | ~50 | AG(inventory >= 0) |
| 05 | Scale (1000 facts) | 1000 | performance < 1s |
| 06 | CSP violations | 4 + derived | guard-before-effect |

## Running Tests

1. Start server: `make server`
2. Select gpt-4
3. Paste prompt
4. Check pass criteria

## What to Verify

Each prompt specifies:

1. **Diagrams** - must render without syntax errors
2. **Facts** - show actual facts in tables (not LISP)
3. **Properties** - CTL formulas with expected results
4. **Pass Criteria** - checkboxes for each requirement

## If a Test Fails

- Mermaid error → check for := or >= in labels
- No facts shown → actor not collecting via assert!
- Wrong property result → check rule definition
- Performance issue → too many facts, need optimization

## Note

Users see: diagrams, tables, properties, charts

Users don't see: LISP code (it's an internal protocol)
