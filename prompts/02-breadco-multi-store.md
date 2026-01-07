# BreadCo Multi-Location Expansion

Building on the basic bakery, I want to model a multi-location operation.

## Business Model

```
                    ┌─────────────┐
                    │  Warehouse  │
                    └──────┬──────┘
                           │
              ┌────────────┼────────────┐
              ▼            ▼            ▼
        ┌─────────┐  ┌─────────┐  ┌─────────┐
        │ Store A │  │ Store B │  │ Store C │
        │ Downtown│  │ Suburbs │  │ Airport │
        └────┬────┘  └────┬────┘  └────┬────┘
             │            │            │
        Customers    Customers    Customers
```

## Actors

### Production
- Bakes bread: base 30 + (day * 5) loaves/day
- Sends all to Warehouse

### Warehouse  
- Receives from Production
- Distributes to stores based on requests
- Has capacity limit of 200 loaves

### Stores (A, B, C)
Each store has different characteristics:

| Store | Location | Demand/Day | Price | Capacity |
|-------|----------|------------|-------|----------|
| A | Downtown | 15-25 | $4 | 50 |
| B | Suburbs | 8-15 | $3 | 40 |
| C | Airport | 5-12 | $5 | 30 |

### Customers
- Each store has 3-5 customers
- Each customer buys 1-6 loaves
- Customers retry once if sold out

## Message Protocol

```
Production -> Warehouse: (baked qty)
Store -> Warehouse: (request store-id qty)
Warehouse -> Store: (delivery qty) | (insufficient available)
Customer -> Store: (buy customer-id qty)
Store -> Customer: (purchase qty price) | (sold-out)
Customer -> Store: (retry customer-id qty)  ; second attempt
```

## Datalog Requirements

Track these facts during simulation:

```lisp
;; Sales events
(assert! 'sale store customer qty price time)

;; Inventory snapshots  
(assert! 'inventory store level time)

;; Unmet demand
(assert! 'unmet store customer wanted time)

;; Deliveries
(assert! 'delivery from to qty time)
```

Define these rules:

```lisp
;; Best performing store
(rule 'top-revenue
  '(top-revenue ?store ?total)
  ...)

;; Stores that ran out
(rule 'stockout
  '(stockout ?store ?time)
  '(inventory ?store 0 ?time))

;; Customer loyalty (bought from same store 3+ times)
(rule 'loyal-customer
  '(loyal ?customer ?store)
  ...)
```

## Invariants to Check

```lisp
;; Warehouse never negative
(never? '(inventory warehouse ?neg))  ; where neg < 0

;; All deliveries eventually fulfilled or rejected
(always? '(delivery-resolved ?request))

;; No store exceeds capacity
(never? '(over-capacity ?store))
```

## Metrics to Chart

After 14-day simulation, show:
1. Daily revenue by store (grouped bar chart)
2. Warehouse inventory over time (line)
3. Cumulative unmet demand (line)
4. Store utilization % (capacity used)

## Questions

1. Which store is most profitable?
2. Is warehouse capacity sufficient?
3. Should we add a 4th store or expand existing?
4. What's the optimal daily production rate?

Start by showing me:
1. State diagram for Warehouse actor
2. State diagram for Store actor (generic, parameterized)
3. Sequence diagram for a typical day
4. The Datalog rules for top-revenue and stockout
5. Initial LISP code structure
