# BreadCo Simulation - Getting Started

I'm building a bakery supply chain simulation using BoundedLISP with actors.

## Current System

**Actors:**
- Production: Bakes bread, sends to StoreFront
- StoreFront: Receives bread, sells to customers
- Customer: Buys bread

**Message Flow:**
```
Production --bread(qty)--> StoreFront
Customer --buy(qty)--> StoreFront
StoreFront --purchase(qty)--> Customer
StoreFront --sold-out--> Customer
```

## Requirements

1. Production bakes 10 loaves per day
2. StoreFront sells at $3/loaf
3. Customer wants 1-5 loaves randomly
4. Track: inventory, revenue, unmet demand

## What I Need

Show me:
1. State diagram for StoreFront (valid mermaid - no := or >= in labels!)
2. The BoundedLISP actor code (CSP compliant - guard before effects)
3. How to track metrics with the registry

Start simple - just these 3 actors for now.
