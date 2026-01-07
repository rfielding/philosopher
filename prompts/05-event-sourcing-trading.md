# Event Sourcing with Temporal Datalog

## Goal

Model an event-sourced system where ALL state changes are captured as Datalog facts, enabling powerful temporal queries and debugging.

## Domain: Trading System

A simplified stock trading system with:
- Order submission
- Matching engine
- Position tracking
- Risk checks
- Audit trail

## Architecture

```
┌──────────────────────────────────────────────────────────────────┐
│                      TRADING SYSTEM                               │
│                                                                   │
│  ┌─────────┐    ┌─────────┐    ┌─────────┐    ┌─────────┐       │
│  │ Client1 │    │ Client2 │    │ Client3 │    │ Client4 │       │
│  └────┬────┘    └────┬────┘    └────┬────┘    └────┬────┘       │
│       │              │              │              │              │
│       └──────────────┴──────────────┴──────────────┘              │
│                              │                                    │
│                              ▼                                    │
│                    ┌─────────────────┐                           │
│                    │    Gateway      │ ◄── Validation            │
│                    └────────┬────────┘                           │
│                             │                                     │
│              ┌──────────────┼──────────────┐                     │
│              ▼              ▼              ▼                      │
│       ┌──────────┐   ┌──────────┐   ┌──────────┐                │
│       │ RiskMgr  │   │ Matching │   │ Position │                │
│       │          │◄─►│  Engine  │◄─►│  Keeper  │                │
│       └──────────┘   └─────┬────┘   └──────────┘                │
│                            │                                      │
│                            ▼                                      │
│                    ┌──────────────┐                              │
│                    │  Market Data │ ◄── Price feeds              │
│                    └──────────────┘                              │
│                                                                   │
│  ════════════════════════════════════════════════════════════   │
│                    │ EVENT LOG (Datalog) │                       │
│  ════════════════════════════════════════════════════════════   │
└──────────────────────────────────────────────────────────────────┘
```

## Event Types

Every state change becomes a Datalog fact:

### Order Events
```lisp
;; Order lifecycle
(assert! 'order-submitted order-id client symbol side qty price time)
(assert! 'order-validated order-id status time)
(assert! 'order-accepted order-id time)
(assert! 'order-rejected order-id reason time)
(assert! 'order-filled order-id fill-qty fill-price time)
(assert! 'order-partial-fill order-id fill-qty remaining time)
(assert! 'order-cancelled order-id reason time)
(assert! 'order-expired order-id time)
```

### Trade Events
```lisp
;; Trade execution
(assert! 'trade-executed trade-id buy-order sell-order symbol qty price time)
(assert! 'trade-reported trade-id exchange time)
(assert! 'trade-settled trade-id time)
```

### Position Events
```lisp
;; Position changes
(assert! 'position-opened client symbol qty avg-price time)
(assert! 'position-increased client symbol add-qty new-avg time)
(assert! 'position-decreased client symbol reduce-qty time)
(assert! 'position-closed client symbol realized-pnl time)
```

### Risk Events
```lisp
;; Risk checks
(assert! 'risk-check order-id check-type result time)
(assert! 'risk-limit-breach client limit-type current-value limit time)
(assert! 'risk-override order-id approver reason time)
```

### Market Events
```lisp
;; Market data
(assert! 'price-update symbol bid ask last time)
(assert! 'volume-update symbol volume time)
(assert! 'market-open symbol time)
(assert! 'market-close symbol time)
(assert! 'trading-halt symbol reason time)
```

## Datalog Rules

### Order State Reconstruction

```lisp
;; Current order state (latest event wins)
(rule 'order-state
  '(order-state ?order-id ?state ?as-of)
  '(order-submitted ?order-id ?_ ?_ ?_ ?_ ?_ ?t1)
  '(latest-order-event ?order-id ?state ?as-of)
  '(>= ?as-of ?t1))

;; Pending orders
(rule 'pending-order
  '(pending-order ?order-id ?client ?symbol ?side ?qty ?price)
  '(order-accepted ?order-id ?_)
  '(order-submitted ?order-id ?client ?symbol ?side ?qty ?price ?_)
  '(not (order-filled ?order-id ?_ ?_ ?_))
  '(not (order-cancelled ?order-id ?_ ?_)))

;; Fill rate
(rule 'order-fill-rate
  '(fill-rate ?order-id ?rate)
  '(order-submitted ?order-id ?_ ?_ ?_ ?orig-qty ?_ ?_)
  '(total-filled ?order-id ?filled)
  '(= ?rate (/ ?filled ?orig-qty)))
```

### Position Calculation

```lisp
;; Current position from events
(rule 'current-position
  '(current-position ?client ?symbol ?qty ?avg-price)
  '(position-events ?client ?symbol ?events)
  '(aggregate-position ?events ?qty ?avg-price))

;; Unrealized P&L
(rule 'unrealized-pnl
  '(unrealized-pnl ?client ?symbol ?pnl)
  '(current-position ?client ?symbol ?qty ?avg)
  '(latest-price ?symbol ?current)
  '(= ?pnl (* ?qty (- ?current ?avg))))

;; Total exposure
(rule 'total-exposure
  '(exposure ?client ?total)
  '(all-positions ?client ?positions)
  '(sum-exposure ?positions ?total))
```

### Risk Analysis

```lisp
;; Concentration risk
(rule 'concentration-risk
  '(concentration-risk ?client ?symbol ?pct)
  '(current-position ?client ?symbol ?qty ?_)
  '(latest-price ?symbol ?price)
  '(exposure ?client ?total)
  '(= ?pct (/ (* ?qty ?price) ?total))
  '(> ?pct 0.25))  ; >25% in one symbol

;; Velocity check (too many orders too fast)
(rule 'order-velocity-breach
  '(velocity-breach ?client ?count ?window)
  '(orders-in-window ?client ?window ?count)
  '(velocity-limit ?limit)
  '(> ?count ?limit))

;; Loss limit breach
(rule 'loss-limit-breach
  '(loss-breach ?client ?loss ?limit)
  '(realized-pnl-today ?client ?loss)
  '(daily-loss-limit ?client ?limit)
  '(< ?loss (- 0 ?limit)))  ; loss is negative
```

### Temporal Queries

```lisp
;; Position at specific time (time travel!)
(rule 'position-at-time
  '(position-at ?client ?symbol ?qty ?time)
  '(position-events-before ?client ?symbol ?time ?events)
  '(replay-position ?events ?qty))

;; Orders between times
(rule 'orders-between
  '(orders-in-range ?client ?t1 ?t2 ?orders)
  '(all-orders ?client ?orders)
  '(filter-by-time ?orders ?t1 ?t2))

;; First occurrence of condition
(rule 'first-breach
  '(first-breach ?client ?limit-type ?time)
  '(risk-limit-breach ?client ?limit-type ?_ ?_ ?time)
  '(not (earlier-breach ?client ?limit-type ?time)))

;; Events leading to trade
(rule 'trade-history
  '(trade-history ?trade-id ?events)
  '(trade-executed ?trade-id ?buy ?sell ?_ ?_ ?_ ?_)
  '(order-events ?buy ?buy-events)
  '(order-events ?sell ?sell-events)
  '(merge-events ?buy-events ?sell-events ?events))
```

### Audit Queries

```lisp
;; Who changed what when
(rule 'audit-trail
  '(audit ?entity ?change-type ?before ?after ?time)
  '(state-change ?entity ?before ?after ?time))

;; Suspicious patterns
(rule 'wash-trade-suspect
  '(wash-trade ?client ?buy-order ?sell-order)
  '(order-submitted ?buy-order ?client ?symbol buy ?_ ?_ ?t1)
  '(order-submitted ?sell-order ?client ?symbol sell ?_ ?_ ?t2)
  '(trade-executed ?_ ?buy-order ?sell-order ?_ ?_ ?_ ?_)
  '(< (abs (- ?t2 ?t1)) 10))  ; orders within 10 ticks

;; Order modification frequency
(rule 'excessive-modifications
  '(excessive-mods ?client ?order-id ?count)
  '(modification-count ?order-id ?count)
  '(> ?count 5))
```

## Simulation Data

### Symbols
```lisp
(define SYMBOLS '(AAPL GOOGL MSFT AMZN TSLA))
```

### Clients
```lisp
(define CLIENTS
  '((client-1 (balance 100000) (risk-limit 10000) (velocity-limit 100))
    (client-2 (balance 500000) (risk-limit 50000) (velocity-limit 200))
    (client-3 (balance 50000)  (risk-limit 5000)  (velocity-limit 50))
    (client-4 (balance 1000000) (risk-limit 100000) (velocity-limit 500))))
```

### Market Simulation
```lisp
;; Price random walk
(define (next-price symbol current-price)
  (let change (* current-price (- (random) 0.5) 0.02)  ; +/- 1%
    (max 1 (+ current-price change))))

;; Order generation
(define (generate-order client)
  (let symbol (random-choice SYMBOLS)
    (let side (random-choice '(buy sell))
      (let qty (+ 1 (random 100))
        (let price (get-current-price symbol)
          (list 'submit-order client symbol side qty price))))))
```

### Simulation Parameters
```lisp
(define SIMULATION-TICKS 10000)
(define ORDERS-PER-TICK 5)
(define PRICE-UPDATE-FREQUENCY 10)
(define EXPECTED-EVENTS (* SIMULATION-TICKS ORDERS-PER-TICK 3))  ; ~150,000 events
```

## Expected Event Volume

After simulation:
- ~50,000 order events
- ~20,000 trade events
- ~30,000 position events
- ~10,000 risk events
- ~40,000 market events
- **Total: ~150,000 Datalog facts**

## Queries to Run

### Operational Queries
```lisp
;; What's client-1's current position in AAPL?
(query 'current-position 'client-1 'AAPL '?qty '?avg)

;; All pending orders
(query 'pending-order '?id '?client '?symbol '?side '?qty '?price)

;; Orders filled in last 100 ticks
(query-all 
  '(order-filled ?id ?qty ?price ?time)
  '(> ?time (- (current-time) 100)))
```

### Risk Queries
```lisp
;; Any risk breaches?
(query 'risk-limit-breach '?client '?type '?value '?limit '?time)

;; Concentration risks
(query 'concentration-risk '?client '?symbol '?pct)

;; Velocity breaches today
(query 'velocity-breach '?client '?count '?window)
```

### Temporal Queries
```lisp
;; What was client-2's GOOGL position at tick 5000?
(query 'position-at 'client-2 'GOOGL '?qty 5000)

;; When did client-3 first breach loss limit?
(query 'first-breach 'client-3 'daily-loss '?time)

;; Order history for specific trade
(query 'trade-history 'trade-12345 '?events)
```

### Audit Queries
```lisp
;; Wash trade suspects
(query 'wash-trade '?client '?buy '?sell)

;; Most modified orders
(query 'excessive-mods '?client '?order '?count)

;; All changes to client-1's positions
(query 'audit 'client-1-position '?change '?before '?after '?time)
```

### Analytics Queries
```lisp
;; Best performing client by P&L
(query 'realized-pnl-today '?client '?pnl)

;; Most traded symbol
(query 'trade-volume '?symbol '?volume)

;; Average fill rate
(query 'avg-fill-rate '?rate)
```

## Invariants

```lisp
;; No negative cash (after accounting for positions)
(never? '(negative-balance ?client ?balance))

;; No position without corresponding trades
(never? '(orphan-position ?client ?symbol))

;; All trades have two sides
(never? '(one-sided-trade ?trade-id))

;; Timestamps are monotonic
(never? '(time-travel ?event1 ?event2 ?backwards))
```

## Performance Questions

1. How long to query position-at for arbitrary time?
2. Can we reconstruct full order book at time T?
3. How many facts before queries slow down?
4. Can we do incremental rule evaluation?

## Charts

After simulation:

1. **Price chart per symbol** (line)
2. **Order volume over time** (bar)
3. **Position changes for top client** (line)
4. **Risk breach frequency** (bar by type)
5. **Trade execution latency** (histogram)

## Starting Point

Show me:

1. **Actor diagram** for the trading system (Gateway, MatchingEngine, etc.)
2. **State diagram for Order lifecycle** (submitted -> accepted -> filled/cancelled)
3. **Sequence diagram** for a successful buy order that fills
4. **LISP code for Gateway actor** that validates and routes orders
5. **Datalog rules** for current-position and wash-trade detection
6. **How to efficiently query position-at-time** given 150k facts
