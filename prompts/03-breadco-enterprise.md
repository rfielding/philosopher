# BreadCo Enterprise Supply Chain Simulation

## Overview

Model a complete bakery supply chain with suppliers, production, logistics, retail, and customers. This tests the system's ability to handle:
- 15+ actors
- Complex message protocols
- Failure modes and recovery
- Extensive Datalog fact collection
- Multiple invariants
- Time-series analysis

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           SUPPLY CHAIN                                   │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐                               │
│  │ Flour    │  │ Yeast    │  │ Packaging│                               │
│  │ Supplier │  │ Supplier │  │ Supplier │                               │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘                               │
│       │             │             │                                      │
│       └─────────────┼─────────────┘                                      │
│                     ▼                                                    │
│              ┌─────────────┐                                             │
│              │  Purchasing │ ◄─── Budget constraints                     │
│              └──────┬──────┘                                             │
│                     │                                                    │
│                     ▼                                                    │
│              ┌─────────────┐                                             │
│              │  Inventory  │ ◄─── Raw materials storage                  │
│              └──────┬──────┘                                             │
│                     │                                                    │
│       ┌─────────────┼─────────────┐                                      │
│       ▼             ▼             ▼                                      │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐                                  │
│  │ Oven 1  │  │ Oven 2  │  │ Oven 3  │  ◄─── Production capacity        │
│  └────┬────┘  └────┬────┘  └────┬────┘                                  │
│       │            │            │                                        │
│       └────────────┼────────────┘                                        │
│                    ▼                                                     │
│             ┌─────────────┐                                              │
│             │  Packaging  │                                              │
│             └──────┬──────┘                                              │
│                    │                                                     │
│                    ▼                                                     │
│             ┌─────────────┐                                              │
│             │  Warehouse  │ ◄─── Finished goods, 500 capacity           │
│             └──────┬──────┘                                              │
│                    │                                                     │
│       ┌────────────┼────────────┬────────────┐                          │
│       ▼            ▼            ▼            ▼                          │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐                        │
│  │ Truck 1 │ │ Truck 2 │ │ Truck 3 │ │ Truck 4 │ ◄─── Delivery fleet   │
│  └────┬────┘ └────┬────┘ └────┬────┘ └────┬────┘                        │
│       │           │           │           │                              │
│       └───────────┴───────────┴───────────┘                              │
│                         │                                                │
│    ┌────────────────────┼────────────────────┐                          │
│    ▼                    ▼                    ▼                          │
│ ┌───────┐          ┌───────┐          ┌───────┐                         │
│ │Store A│          │Store B│          │Store C│                         │
│ │5 staff│          │3 staff│          │4 staff│                         │
│ └───┬───┘          └───┬───┘          └───┬───┘                         │
│     │                  │                  │                              │
│     ▼                  ▼                  ▼                              │
│ Customers          Customers          Customers                          │
│ (10/day)           (8/day)            (12/day)                          │
└─────────────────────────────────────────────────────────────────────────┘
```

## Actor Specifications

### Suppliers

| Actor | Product | Lead Time | Reliability | Price/Unit | Min Order |
|-------|---------|-----------|-------------|------------|-----------|
| FlourSupplier | Flour (kg) | 2 days | 95% | $0.50 | 100 |
| YeastSupplier | Yeast (packs) | 1 day | 90% | $2.00 | 20 |
| PackagingSupplier | Bags | 1 day | 99% | $0.10 | 500 |

### Production

| Actor | Capacity | Flour/Loaf | Yeast/Loaf | Time/Batch | Batch Size |
|-------|----------|------------|------------|------------|------------|
| Oven1 | 50/day | 0.5 kg | 0.1 pack | 2 hours | 25 |
| Oven2 | 50/day | 0.5 kg | 0.1 pack | 2 hours | 25 |
| Oven3 | 30/day | 0.5 kg | 0.1 pack | 3 hours | 15 |

### Logistics

| Actor | Capacity | Speed | Cost/Trip | Routes |
|-------|----------|-------|-----------|--------|
| Truck1 | 100 loaves | Fast | $20 | A, B |
| Truck2 | 100 loaves | Fast | $20 | B, C |
| Truck3 | 150 loaves | Slow | $15 | A, B, C |
| Truck4 | 80 loaves | Fast | $25 | A, C |

### Retail

| Store | Location | Rent/Day | Staff | Price | Peak Hours |
|-------|----------|----------|-------|-------|------------|
| StoreA | Downtown | $100 | 5 | $4.50 | 7-9, 12-14 |
| StoreB | Mall | $150 | 3 | $4.00 | 11-20 |
| StoreC | Station | $80 | 4 | $5.00 | 6-9, 17-19 |

## Message Protocol

### Supply Chain Messages

```
;; Purchasing
Purchasing -> Supplier: (order product qty urgency)
Supplier -> Purchasing: (confirm order-id eta) | (reject reason)
Supplier -> Inventory: (deliver order-id product qty)

;; Production
Inventory -> Oven: (materials-ready batch-id flour yeast)
Inventory -> Oven: (materials-short batch-id missing)
Oven -> Packaging: (batch-complete batch-id qty quality)
Packaging -> Warehouse: (packaged batch-id qty)

;; Logistics
Store -> Warehouse: (request store-id qty priority)
Warehouse -> Dispatcher: (fulfill request-id store qty)
Dispatcher -> Truck: (assign delivery-id store qty)
Truck -> Store: (delivery delivery-id qty)
Truck -> Dispatcher: (completed delivery-id) | (failed delivery-id reason)

;; Retail
Customer -> Store: (buy customer-id qty)
Store -> Customer: (sale qty total) | (partial qty available) | (sold-out)
Customer -> Store: (complaint customer-id issue)
Store -> Manager: (escalate complaint-id)
```

## Failure Modes

Model these failure scenarios:

### Supply Failures
```lisp
;; Supplier can fail to deliver (10% chance for flour, 5% yeast)
(rule 'supply-failure
  '(supply-failure ?supplier ?order ?time)
  '(order-placed ?supplier ?order ?time)
  '(not (delivered ?order)))
```

### Production Failures  
```lisp
;; Oven can break down (2% chance per batch)
(rule 'oven-breakdown
  '(oven-down ?oven ?start ?duration)
  '(breakdown-event ?oven ?start)
  '(repair-time ?oven ?duration))

;; Quality issues (5% of batches)
(rule 'quality-reject
  '(rejected ?batch ?reason)
  '(quality-check ?batch fail ?reason))
```

### Logistics Failures
```lisp
;; Truck breakdown (1% per trip)
(rule 'truck-failure
  '(truck-down ?truck ?delivery ?time)
  '(in-transit ?truck ?delivery ?time)
  '(breakdown ?truck ?time))

;; Traffic delays (20% of deliveries)
(rule 'delayed-delivery  
  '(late ?delivery ?expected ?actual)
  '(scheduled ?delivery ?expected)
  '(completed ?delivery ?actual)
  '(> ?actual ?expected))
```

### Retail Failures
```lisp
;; Staff shortage
(rule 'understaffed
  '(understaffed ?store ?time)
  '(staff-present ?store ?count ?time)
  '(staff-required ?store ?min)
  '(< ?count ?min))

;; Spoilage (bread older than 2 days)
(rule 'spoiled
  '(spoiled ?store ?qty ?time)
  '(inventory-age ?store ?batch ?age ?time)
  '(> ?age 2))
```

## Datalog Fact Collection

### Core Facts (auto-collected)
```lisp
;; Every message send
(assert! 'sent from to msg-type payload time)

;; Every state change
(assert! 'state-change actor old-state new-state time)

;; Every guard event
(assert! 'guard actor guard-type time)

;; Every effect
(assert! 'effect actor operation target time)
```

### Business Facts
```lisp
;; Financial
(assert! 'revenue store amount time)
(assert! 'cost category amount time)
(assert! 'profit store amount time)

;; Inventory
(assert! 'stock location product qty time)
(assert! 'reorder-point product threshold)

;; Performance
(assert! 'cycle-time process start end)
(assert! 'utilization resource pct time)
(assert! 'throughput process qty time)

;; Customer
(assert! 'sale store customer qty price time)
(assert! 'lost-sale store customer qty reason time)
(assert! 'complaint store customer issue time)
```

## Datalog Rules

### Financial Analysis
```lisp
;; Daily profit by store
(rule 'daily-profit
  '(daily-profit ?store ?day ?profit)
  '(daily-revenue ?store ?day ?rev)
  '(daily-costs ?store ?day ?costs)
  '(= ?profit (- ?rev ?costs)))

;; Cost breakdown
(rule 'cost-breakdown
  '(cost-breakdown ?category ?total)
  ; aggregate costs by category
  ...)

;; Margin analysis
(rule 'margin
  '(margin ?store ?pct)
  '(total-revenue ?store ?rev)
  '(total-costs ?store ?costs)
  '(= ?pct (* 100 (/ (- ?rev ?costs) ?rev))))
```

### Operational Analysis
```lisp
;; Bottleneck detection
(rule 'bottleneck
  '(bottleneck ?resource ?utilization)
  '(utilization ?resource ?util ?_)
  '(> ?util 90))

;; Lead time analysis
(rule 'order-lead-time
  '(lead-time ?order ?days)
  '(order-placed ?_ ?order ?t1)
  '(order-received ?_ ?order ?t2)
  '(= ?days (- ?t2 ?t1)))

;; Stock velocity
(rule 'fast-moving
  '(fast-moving ?product)
  '(turnover ?product ?rate)
  '(> ?rate 5))  ; turns over 5x per period
```

### Quality Analysis
```lisp
;; Defect rate by oven
(rule 'defect-rate
  '(defect-rate ?oven ?rate)
  '(total-batches ?oven ?total)
  '(rejected-batches ?oven ?rejected)
  '(= ?rate (/ ?rejected ?total)))

;; Supplier reliability
(rule 'supplier-score
  '(supplier-score ?supplier ?score)
  '(orders-placed ?supplier ?total)
  '(orders-fulfilled ?supplier ?fulfilled)
  '(= ?score (* 100 (/ ?fulfilled ?total))))
```

### CSP Verification
```lisp
;; No negative inventory (critical!)
(rule 'inventory-violation
  '(inventory-violation ?location ?product ?time)
  '(stock ?location ?product ?qty ?time)
  '(< ?qty 0))

;; Deadlock detection
(rule 'deadlock
  '(deadlock ?a ?b)
  '(waiting-for ?a ?b)
  '(waiting-for ?b ?a))

;; Starvation (actor waiting > 100 ticks)
(rule 'starved
  '(starved ?actor ?since)
  '(blocked-since ?actor ?since)
  '(current-time ?now)
  '(> (- ?now ?since) 100))

;; Message loss (sent but never received after timeout)
(rule 'lost-message
  '(lost-message ?from ?to ?msg ?time)
  '(sent ?from ?to ?msg ?time)
  '(current-time ?now)
  '(> (- ?now ?time) 50)
  '(not (received ?to ?msg)))
```

## Invariants

```lisp
;; Safety
(never? '(inventory-violation ?_ ?_ ?_))
(never? '(deadlock ?_ ?_))
(never? '(spoiled-sold ?_ ?_ ?_))  ; never sell spoiled bread

;; Liveness  
(always? '(eventually-delivered ?order))
(always? '(truck-returns-home ?truck))

;; Business rules
(never? '(price-below-cost ?store ?sale))
(always? '(reorder-triggered-when-low ?product))
```

## Simulation Parameters

```lisp
;; Time
(define SIMULATION-DAYS 30)
(define TICKS-PER-DAY 100)
(define TOTAL-TICKS (* SIMULATION-DAYS TICKS-PER-DAY))

;; Initial conditions
(define INITIAL-FLOUR 500)      ; kg
(define INITIAL-YEAST 100)      ; packs
(define INITIAL-PACKAGING 2000) ; bags
(define INITIAL-CASH 10000)     ; dollars
(define WAREHOUSE-CAPACITY 500) ; loaves

;; Demand model
(define BASE-DEMAND-A 15)
(define BASE-DEMAND-B 10)
(define BASE-DEMAND-C 18)
(define DEMAND-VARIANCE 0.3)    ; +/- 30%

;; Pricing
(define PRICE-A 4.50)
(define PRICE-B 4.00)
(define PRICE-C 5.00)
(define COST-PER-LOAF 1.50)
```

## Charts Required

After 30-day simulation, generate:

### Financial
1. Daily revenue by store (stacked bar)
2. Cumulative profit (line)
3. Cost breakdown (pie via description)
4. Margin trend (line)

### Operations  
1. Production output vs capacity (line)
2. Warehouse inventory level (line with capacity line)
3. Delivery performance (on-time % bar)
4. Resource utilization (grouped bar)

### Quality
1. Defect rate by oven (bar)
2. Supplier reliability scores (bar)
3. Spoilage by store (bar)

### Customer
1. Sales volume by store (line)
2. Lost sales (line)
3. Customer complaints (if any)

## Questions to Answer

### Strategic
1. Should we add a 4th oven or optimize existing?
2. Which store should we expand first?
3. Is our truck fleet right-sized?
4. Should we switch flour suppliers?

### Operational
1. What's our production bottleneck?
2. Are we holding too much inventory?
3. What's causing most lost sales?
4. Which oven has quality issues?

### Financial
1. What's our true cost per loaf?
2. Which store is most profitable per sq ft?
3. What's our cash flow pattern?
4. Where should we cut costs?

## Starting Point

Begin by:

1. **State diagram for Purchasing actor** - show order lifecycle
2. **State diagram for Oven actor** - show production states including breakdown
3. **Sequence diagram for complete order-to-delivery flow**
4. **Initial actor definitions** for the 3 suppliers and purchasing
5. **Datalog rules** for bottleneck detection and supplier-score
6. **xychart** showing expected production capacity over 30 days

Remember:
- Keep mermaid labels SHORT (no := or >= symbols!)
- Use transition tables for precise semantics
- CSP compliant code (guard before effects)
- Collect facts during simulation for Datalog queries
