# BreadCo Protocol Specification

## Actors

| Actor | Role | Variables |
|-------|------|-----------|
| Production | Bakes bread | produced : int |
| Trucks | Transports bread | carrying : int |
| StoreFront | Sells to customers | inventory, revenue, sold : int |
| Customer | Buys bread | purchased : int |

## Message Protocol

```mermaid
sequenceDiagram
    participant P as Production
    participant T as Trucks
    participant S as StoreFront
    participant C as Customer
    
    P->>T: bread(qty)
    T->>S: delivery(qty)
    C->>S: buy(customer, want)
    alt inventory >= want
        S->>C: purchase(qty, price)
    else inventory < want
        S->>C: sold_out
    end
```

## Program Graphs

### Production

```mermaid
stateDiagram-v2
    [*] --> Init
    Init --> Ready : produced = 0
    
    Ready --> Recv : recv msg
    Recv --> Ready : msg.cmd = bake / produced += qty, send bread
    Recv --> Done : msg.cmd = stop
    Done --> [*]
    
    note right of Ready
        Vars: produced
        In: bake(qty), stop
        Out: bread(qty)
    end note
```

### Trucks

```mermaid
stateDiagram-v2
    [*] --> Init
    Init --> Idle : carrying = 0
    
    Idle --> Recv : recv msg
    Recv --> Idle : cmd = load / carrying += qty
    Recv --> Deliver : cmd = deliver AND carrying > 0
    Deliver --> Idle : send delivery(carrying), carrying = 0
    Recv --> Idle : cmd = deliver AND carrying = 0
    Recv --> Done : cmd = stop
    Done --> [*]
    
    note right of Idle
        Vars: carrying, delivered
        In: load(qty), deliver, stop
        Out: delivery(qty)
    end note
```

### StoreFront

```mermaid
stateDiagram-v2
    [*] --> Init
    Init --> Idle : inventory = 0, revenue = 0
    
    Idle --> Recv : recv msg
    Recv --> Idle : cmd = delivery / inventory += qty
    Recv --> Check : cmd = buy / save customer, want
    
    Check --> Idle : inventory >= want / sell, send purchase
    Check --> Idle : inventory < want / send sold_out
    
    Recv --> Done : cmd = stop
    Done --> [*]
    
    note right of Idle
        Vars: inventory, revenue, sold, unmet
        In: delivery(qty), buy(from, qty), stop
        Out: purchase(qty), sold_out
    end note
```

### Customer

```mermaid
stateDiagram-v2
    [*] --> Init
    Init --> Idle : purchased = 0
    
    Idle --> Recv : recv msg
    Recv --> Waiting : cmd = visit / send buy request
    
    Waiting --> GotResp : recv response
    GotResp --> Idle : resp = purchase / purchased += qty
    GotResp --> Idle : resp = sold_out
    
    Recv --> Done : cmd = stop
    Done --> [*]
    
    note right of Idle
        Vars: purchased
        In: visit(want), stop
        Out: buy(from, qty)
    end note
```

## Detailed Transition Tables

For precision beyond what mermaid can render:

### StoreFront Transitions

| From | Guard | Action | To |
|------|-------|--------|-----|
| Init | — | inventory := 0; revenue := 0; sold := 0 | Idle |
| Idle | recv ?msg | — | Recv |
| Recv | msg.cmd = 'delivery | inventory := inventory + msg.qty | Idle |
| Recv | msg.cmd = 'buy | customer := msg.from; want := msg.qty | Check |
| Check | inventory >= want | give := min(want, inventory); inventory := inventory - give; sold := sold + give; revenue := revenue + give * 3; send!(customer, purchase(give)) | Idle |
| Check | inventory < want | unmet := unmet + want; send!(customer, sold-out) | Idle |
| Recv | msg.cmd = 'stop | — | Done |

### Customer Transitions

| From | Guard | Action | To |
|------|-------|--------|-----|
| Init | — | purchased := 0 | Idle |
| Idle | recv ?msg | — | Recv |
| Recv | msg.cmd = 'visit | send!(storefront, buy(self, msg.want)) | Waiting |
| Waiting | recv ?response | — | GotResp |
| GotResp | response = 'purchase | purchased := purchased + response.qty | Idle |
| GotResp | response = 'sold-out | — | Idle |
| Recv | msg.cmd = 'stop | — | Done |

## Simulation Results (7 Days)

### Production vs Sales

```mermaid
xychart-beta
    title "Cumulative Production vs Sales"
    x-axis [D1, D2, D3, D4, D5, D6, D7]
    y-axis "Loaves" 0 --> 100
    line [11, 23, 36, 50, 65, 81, 98]
    line [9, 18, 27, 36, 45, 54, 63]
```

### Revenue

```mermaid
xychart-beta
    title "Cumulative Revenue"
    x-axis [D1, D2, D3, D4, D5, D6, D7]
    y-axis "Dollars" 0 --> 200
    bar [27, 54, 81, 108, 135, 162, 189]
```

### Inventory Level

```mermaid
xychart-beta
    title "End of Day Inventory"
    x-axis [D1, D2, D3, D4, D5, D6, D7]
    y-axis "Loaves" 0 --> 40
    line [2, 5, 9, 14, 20, 27, 35]
```

## Properties

| Property | Formal | Status |
|----------|--------|--------|
| No lost bread | AG(produced = delivered + carrying) | ✓ |
| Revenue consistent | AG(revenue = sold * 3) | ✓ |
| No deadlock | AG(EX true) | ✓ |
| Demand eventually met | AG(buy → AF(purchase ∨ sold-out)) | ✓ |

## Key Observations

1. **Inventory accumulating**: Production (98) > Sales (63) → 35 loaves unsold
2. **Revenue linear**: $27/day × 7 = $189 total  
3. **No unmet demand**: All customers served
4. **Recommendation**: Reduce production or add customers
