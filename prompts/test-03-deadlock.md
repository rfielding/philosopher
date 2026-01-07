# Test 3: Deadlock Detection

Two actors waiting for each other = deadlock.

## Expected State Diagram

```mermaid
stateDiagram-v2
    [*] --> Waiting
    
    state "Actor A" as A {
        [*] --> A_wait: waiting for B
    }
    
    state "Actor B" as B {
        [*] --> B_wait: waiting for A
    }
```

## Expected Facts

| Fact |
|------|
| waiting-for A B |
| waiting-for B A |

## Derived Fact (from rule)

| Fact |
|------|
| deadlock A B |

## Expected Property

AG(not deadlock)

Result: **false** (deadlock exists!)

## Pass Criteria

- [ ] Diagram renders
- [ ] 2 waiting-for facts shown
- [ ] Deadlock detected
- [ ] Property correctly returns false
