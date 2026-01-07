# Test 4: BreadCo 7-Day Simulation

Production -> StoreFront -> Customers

## Expected State Diagram (StoreFront)

```mermaid
stateDiagram-v2
    [*] --> Idle
    Idle --> Idle: delivery, inv '= inv + qty
    Idle --> Sell: buy request
    Sell --> Idle: have enough, sell
    Sell --> Idle: sold out, reject
```

## Expected Sequence (Day 1)

```mermaid
sequenceDiagram
    Production->>StoreFront: bread 11
    Customer->>StoreFront: buy 3
    StoreFront->>Customer: sold 3
```

## Expected Facts (sample 10 of ~50)

| Day | Event | Value |
|-----|-------|-------|
| 1 | produced | 11 |
| 1 | delivered | 11 |
| 1 | sale | 3 |
| 1 | inventory | 8 |
| 2 | produced | 12 |
| 2 | delivered | 12 |
| 2 | sale | 4 |
| 2 | inventory | 16 |
| 3 | produced | 13 |
| 3 | inventory | 24 |

Total facts: ~50

## Expected Chart

```mermaid
xychart-beta
    title "BreadCo 7-Day Metrics"
    x-axis [D1, D2, D3, D4, D5, D6, D7]
    y-axis "Units" 0 --> 100
    line "Cumulative Produced" [11, 23, 36, 50, 65, 81, 98]
    line "Cumulative Sold" [3, 8, 14, 21, 29, 38, 48]
    bar "Inventory" [8, 15, 22, 29, 36, 43, 50]
```

## Expected Properties

1. AG(inventory >= 0): **true**
2. AG(sold implies inventory decreased): **true**
3. EF(inventory > 20): **true**

## Pass Criteria

- [ ] State diagram renders
- [ ] Sequence diagram renders
- [ ] ~50 facts collected, 10 shown
- [ ] xychart renders with simulation data
- [ ] All 3 properties verified
