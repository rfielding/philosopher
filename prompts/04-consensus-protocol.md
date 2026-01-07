# Distributed Consensus Protocol Verification

## Goal

Model and verify a simplified Raft-like consensus protocol using BoundedLISP actors and Datalog for invariant checking.

## The Problem

5 servers need to agree on a sequence of commands despite:
- Network partitions (messages can be lost)
- Server crashes (and restarts)
- Timing differences

## Protocol Overview

```
┌─────────────────────────────────────────────────────────┐
│                    CLUSTER                               │
│                                                          │
│     ┌────────┐         ┌────────┐         ┌────────┐    │
│     │Server 1│◄───────►│Server 2│◄───────►│Server 3│    │
│     │        │         │ LEADER │         │        │    │
│     └────┬───┘         └────┬───┘         └────┬───┘    │
│          │                  │                  │         │
│          │    ┌─────────────┴─────────────┐   │         │
│          │    │                           │   │         │
│          ▼    ▼                           ▼   ▼         │
│     ┌────────┐                         ┌────────┐       │
│     │Server 4│◄───────────────────────►│Server 5│       │
│     └────────┘                         └────────┘       │
│                                                          │
└─────────────────────────────────────────────────────────┘
          │
          ▼
     ┌─────────┐
     │ Clients │  (submit commands)
     └─────────┘
```

## Server States

Each server can be in one of three states:

| State | Description |
|-------|-------------|
| Follower | Default state, accepts leader's commands |
| Candidate | Requesting votes to become leader |
| Leader | Coordinates replication, handles client requests |

## Server Variables

```lisp
;; Per-server state
(define server-state
  '((current-term 0)        ; election term number
    (voted-for nil)         ; who we voted for this term
    (log ())                ; command log entries
    (commit-index 0)        ; highest committed entry
    (last-applied 0)        ; last applied to state machine
    (role follower)))       ; follower | candidate | leader

;; Leader-only state (when role = leader)
(define leader-state
  '((next-index ())         ; per-follower: next log index to send
    (match-index ())))      ; per-follower: highest replicated index
```

## Message Protocol

### Election Messages

```
;; Candidate requests vote
Candidate -> All: (request-vote term candidate-id last-log-index last-log-term)

;; Server responds to vote request
Server -> Candidate: (vote-response term vote-granted)
```

### Replication Messages

```
;; Leader sends log entries (or heartbeat if entries empty)
Leader -> Follower: (append-entries term leader-id prev-log-index prev-log-term entries leader-commit)

;; Follower acknowledges
Follower -> Leader: (append-response term success match-index)
```

### Client Messages

```
;; Client submits command
Client -> Leader: (command client-id cmd-id data)

;; Leader responds when committed
Leader -> Client: (command-response cmd-id success result)

;; Redirect if not leader
Server -> Client: (not-leader leader-hint)
```

## State Machine Rules

### Election Rules

```
;; Start election when election timeout expires
Follower -> Candidate:
  - Increment currentTerm
  - Vote for self
  - Reset election timer
  - Send RequestVote to all servers

;; Win election with majority votes
Candidate -> Leader:
  - Received votes from majority
  - Send initial empty AppendEntries (heartbeat)

;; Discover higher term
Any -> Follower:
  - Received message with term > currentTerm
  - Update currentTerm, clear votedFor
```

### Replication Rules

```
;; Leader receives client command
Leader:
  - Append entry to local log
  - Send AppendEntries to all followers
  - When majority acknowledge: commit entry
  - Apply to state machine
  - Respond to client

;; Follower receives AppendEntries  
Follower:
  - If term < currentTerm: reject
  - If log doesn't contain entry at prevLogIndex with prevLogTerm: reject
  - Append new entries
  - Update commitIndex
  - Respond success
```

## Datalog Facts

### State Facts
```lisp
;; Server state at time t
(assert! 'server-state server term role time)
(assert! 'voted-for server candidate term time)
(assert! 'log-entry server index term command time)
(assert! 'commit-index server index time)
```

### Message Facts
```lisp
;; Vote requests and responses
(assert! 'vote-requested candidate term time)
(assert! 'vote-granted voter candidate term time)
(assert! 'vote-denied voter candidate term reason time)

;; Replication
(assert! 'append-sent leader follower term entries time)
(assert! 'append-ack follower leader term success time)
(assert! 'entry-committed index term time)
```

### Failure Facts
```lisp
;; Crashes and recoveries
(assert! 'crashed server time)
(assert! 'recovered server time)

;; Network partitions
(assert! 'partition-start servers time)
(assert! 'partition-end time)
(assert! 'message-lost from to msg time)
```

## Datalog Rules

### Safety Properties

```lisp
;; Election Safety: at most one leader per term
(rule 'multiple-leaders
  '(multiple-leaders ?term ?leader1 ?leader2)
  '(server-state ?leader1 ?term leader ?t1)
  '(server-state ?leader2 ?term leader ?t2)
  '(!= ?leader1 ?leader2))

;; Leader Completeness: committed entries appear in future leaders' logs
(rule 'missing-committed-entry
  '(missing-committed ?leader ?index ?term)
  '(entry-committed ?index ?term ?_)
  '(server-state ?leader ?_ leader ?t)
  '(not (log-entry ?leader ?index ?term ?_ ?_)))

;; Log Matching: if two logs contain entry with same index and term,
;; logs are identical up to that index
(rule 'log-mismatch
  '(log-mismatch ?s1 ?s2 ?index)
  '(log-entry ?s1 ?index ?term ?cmd1 ?_)
  '(log-entry ?s2 ?index ?term ?cmd2 ?_)
  '(!= ?cmd1 ?cmd2))
```

### Liveness Properties

```lisp
;; Eventually elect a leader (in stable network)
(rule 'no-leader
  '(no-leader ?term)
  '(server-state ?_ ?term ?_ ?_)
  '(not (server-state ?_ ?term leader ?_)))

;; Commands eventually commit (leader exists, majority connected)
(rule 'uncommitted-command
  '(stuck-command ?cmd-id ?since)
  '(command-received ?_ ?cmd-id ?since)
  '(current-time ?now)
  '(> (- ?now ?since) 200)
  '(not (command-committed ?cmd-id)))
```

### Debugging Rules

```lisp
;; Split vote (no majority)
(rule 'split-vote
  '(split-vote ?term)
  '(election-started ?term ?_)
  '(not (leader-elected ?term)))

;; Stale leader (partitioned leader doesn't know it's stale)
(rule 'stale-leader
  '(stale-leader ?server ?old-term ?current-term)
  '(server-state ?server ?old-term leader ?_)
  '(server-state ?_ ?current-term leader ?_)
  '(> ?current-term ?old-term))

;; Log divergence point
(rule 'divergence-point
  '(divergence ?s1 ?s2 ?index)
  '(log-entry ?s1 ?index ?term1 ?_ ?_)
  '(log-entry ?s2 ?index ?term2 ?_ ?_)
  '(!= ?term1 ?term2))
```

## Invariants

```lisp
;; CRITICAL: Never two leaders in same term
(never? '(multiple-leaders ?_ ?_ ?_))

;; CRITICAL: Committed entries never lost
(never? '(missing-committed ?_ ?_ ?_))

;; CRITICAL: Logs never contradict
(never? '(log-mismatch ?_ ?_ ?_))

;; Liveness: eventually make progress (in stable conditions)
(eventually? '(entry-committed ?_ ?_ ?_))
```

## Test Scenarios

### Scenario 1: Normal Operation
```lisp
;; 5 servers, no failures, 10 client commands
(define scenario-normal
  '((servers 5)
    (clients 2)
    (commands 10)
    (failures none)
    (duration 500)))
```

### Scenario 2: Leader Crash
```lisp
;; Leader crashes after committing 3 entries
(define scenario-leader-crash
  '((servers 5)
    (commands 10)
    (failures ((crash leader after-commit 3)))
    (expected (new-leader-elected)
              (no-committed-entries-lost))))
```

### Scenario 3: Network Partition
```lisp
;; Partition isolates leader from majority
(define scenario-partition
  '((servers 5)
    (partition ((1 2) (3 4 5)))  ; servers 1,2 isolated
    (duration 200)
    (expected (new-leader-in-majority)
              (old-leader-steps-down))))
```

### Scenario 4: Cascading Failures
```lisp
;; Multiple failures in sequence
(define scenario-chaos
  '((servers 5)
    (failures ((crash 1 at 50)
               (crash 2 at 100)
               (recover 1 at 150)
               (partition ((3) (4 5)) at 200)
               (heal-partition at 300)))
    (commands 20)
    (expected (all-commands-eventually-commit)
              (no-invariant-violations))))
```

## Metrics

Track these during simulation:

```lisp
;; Elections
(registry-set! 'elections-started 0)
(registry-set! 'elections-completed 0)
(registry-set! 'split-votes 0)

;; Replication
(registry-set! 'entries-committed 0)
(registry-set! 'entries-replicated 0)
(registry-set! 'replication-round-trips 0)

;; Performance
(registry-set! 'commit-latency-sum 0)
(registry-set! 'commit-latency-count 0)

;; Failures
(registry-set! 'messages-lost 0)
(registry-set! 'leader-changes 0)
```

## Expected Output

### State Diagrams

1. **Server state machine** - Follower/Candidate/Leader transitions
2. **Log replication flow** - Leader to follower sync

### Sequence Diagrams

1. **Successful election** - Vote request/response flow
2. **Log replication** - Command commit process
3. **Leader failure recovery** - Crash and re-election

### Charts

1. **Commit latency over time** - How fast commands commit
2. **Leader tenure** - Time between leader changes
3. **Log convergence** - How quickly logs sync after partition heals

## Questions

1. How many messages needed to commit one command?
2. What's minimum time to elect new leader after crash?
3. Can we have two leaders that both think they're valid?
4. What happens to in-flight commands when leader crashes?
5. How does the system behave under 50% message loss?

## Starting Point

Show me:

1. **State diagram for Server actor** (Follower/Candidate/Leader)
2. **Transition table** with all state transitions and conditions
3. **Datalog rule for multiple-leaders** violation detection
4. **Initial LISP code** for Server actor with election timeout
5. **Sequence diagram** for successful election (5 servers)
