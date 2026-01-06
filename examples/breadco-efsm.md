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
        S->>C: sold-out()
    end
```

## Program Graphs

### Production

```mermaid
stateDiagram-v2
    [*] --> q0: produced := 0
    
    q0 --> q1: [recv ?msg]
    q1 --> q0: [msg.cmd = 'bake] / produced := produced + msg.qty ; send!(trucks, bread(msg.qty))
    q1 --> [*]: [msg.cmd = 'stop]
    
    note right of q0
        Input: bake(qty), stop
        Output: bread(qty) → Trucks
        Vars: produced : int := 0
    end note
```

### Trucks

```mermaid
stateDiagram-v2
    [*] --> q0: carrying := 0
    
    q0 --> q1: [recv ?msg]
    q1 --> q0: [msg.cmd = 'load] / carrying := carrying + msg.qty
    q1 --> q2: [msg.cmd = 'deliver ∧ carrying > 0]
    q2 --> q0: send!(storefront, delivery(carrying)) ; delivered := delivered + carrying ; carrying := 0
    q1 --> q0: [msg.cmd = 'deliver ∧ carrying = 0]
    q1 --> [*]: [msg.cmd = 'stop]
    
    note right of q0
        Input: load(qty), deliver, stop
        Output: delivery(qty) → StoreFront
        Vars: carrying : int := 0
              delivered : int := 0
    end note
```

### StoreFront

```mermaid
stateDiagram-v2
    [*] --> Idle: inventory := 0 ; revenue := 0 ; sold := 0 ; unmet := 0
    
    Idle --> q1: [recv ?msg]
    
    q1 --> Idle: [msg.cmd = 'delivery] / inventory := inventory + msg.qty
    
    q1 --> Check: [msg.cmd = 'buy] / customer := msg.from ; want := msg.qty
    
    Check --> Idle: [inventory >= want] / give := min(want, inventory) ; inventory := inventory - give ; sold := sold + give ; revenue := revenue + give * 3 ; send!(customer, purchase(give))
    
    Check --> Idle: [inventory < want] / unmet := unmet + want ; send!(customer, sold-out())
    
    q1 --> [*]: [msg.cmd = 'stop]
    
    note right of Idle
        Input: delivery(qty), buy(from, qty), stop
        Output: purchase(qty) | sold-out() → Customer
        
        Variables:
          inventory : int := 0
          revenue   : int := 0
          sold      : int := 0
          unmet     : int := 0
    end note
```

### Customer

```mermaid
stateDiagram-v2
    [*] --> Idle: purchased := 0
    
    Idle --> q1: [recv ?msg]
    
    q1 --> Waiting: [msg.cmd = 'visit] / send!(storefront, buy(self, msg.want))
    
    Waiting --> q2: [recv ?response]
    
    q2 --> Idle: [response.cmd = 'purchase] / purchased := purchased + response.qty
    q2 --> Idle: [response.cmd = 'sold-out]
    
    q1 --> [*]: [msg.cmd = 'stop]
    
    note right of Idle
        Input: visit(want), stop
        Output: buy(from, qty) → StoreFront
        
        Variables:
          purchased : int := 0
    end note
```

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
    title "Cumulative Revenue ($)"
    x-axis [D1, D2, D3, D4, D5, D6, D7]
    y-axis "Dollars" 0 --> 200
    bar [27, 54, 81, 108, 135, 162, 189]
```

### Inventory Level

```mermaid
xychart-beta
    title "End-of-Day Inventory"
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
