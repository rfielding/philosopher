# CSP Debugging Challenge

## Goal

This prompt tests the system's ability to detect and fix CSP violations. It contains intentionally broken code that violates the guard-before-effect pattern.

## The Rule

In CSP-style actors, every state must begin with a **guard** (receive! or send-to!) before any **effects** (set!, registry-set!, send-to! after state change).

```
CORRECT:
  (let msg (receive!)          ; GUARD first
    (set! counter (+ counter 1))  ; EFFECT after
    ...)

WRONG:
  (set! counter (+ counter 1))  ; EFFECT first - VIOLATION!
  (let msg (receive!)
    ...)
```

## Challenge: Find the Bugs

Here are 5 actors. Each has 1-3 CSP violations. Find them all.

### Actor 1: Counter (2 violations)

```lisp
(define (counter-loop count)
  (set! last-count count)           ; BUG: effect before guard
  (let msg (receive!)
    (cond
      ((eq? (car msg) 'inc)
       (set! count (+ count 1))
       (list 'become (list 'counter-loop count)))
      ((eq? (car msg) 'get)
       (send-to! (cadr msg) count)
       (registry-set! 'queries (+ (registry-get 'queries) 1))  ; BUG: effect after send
       (list 'become (list 'counter-loop count)))
      (else
       (list 'become (list 'counter-loop count))))))
```

### Actor 2: Buffer (3 violations)

```lisp
(define (buffer-loop items max-size)
  (registry-set! 'buffer-size (length items))  ; BUG: effect before guard
  (let msg (receive!)
    (cond
      ((eq? (car msg) 'put)
       (if (< (length items) max-size)
         (begin
           (set! items (append items (list (cadr msg))))
           (set! stats (+ stats 1))  ; BUG: stats not initialized via guard
           (list 'become (list 'buffer-loop items max-size)))
         (begin
           (send-to! (caddr msg) 'full)
           (list 'become (list 'buffer-loop items max-size)))))
      ((eq? (car msg) 'get)
       (if (> (length items) 0)
         (let item (car items)
           (set! items (cdr items))
           (send-to! (cadr msg) item)
           (registry-set! 'gets (+ (registry-get 'gets) 1))  ; BUG: after send
           (list 'become (list 'buffer-loop items max-size)))
         (begin
           (send-to! (cadr msg) 'empty)
           (list 'become (list 'buffer-loop items max-size))))))))
```

### Actor 3: Worker (1 violation)

```lisp
(define (worker-loop status)
  (let msg (receive!)
    (cond
      ((eq? (car msg) 'work)
       (let result (process-work (cadr msg))
         (set! status 'busy)
         (send-to! (caddr msg) result)
         (set! status 'idle)  ; BUG: effect after send-to!
         (list 'become (list 'worker-loop status))))
      ((eq? (car msg) 'status)
       (send-to! (cadr msg) status)
       (list 'become (list 'worker-loop status))))))
```

### Actor 4: Accumulator (2 violations)

```lisp
(define (accum-loop total count)
  (let msg (receive!)
    (cond
      ((eq? (car msg) 'add)
       (let value (cadr msg)
         (set! total (+ total value))
         (set! count (+ count 1))
         (registry-set! 'running-avg (/ total count))
         (list 'become (list 'accum-loop total count))))
      ((eq? (car msg) 'reset)
       (set! total 0)
       (set! count 0)
       (send-to! (cadr msg) 'reset-done)
       (set! last-reset (current-time))  ; BUG: effect after send
       (list 'become (list 'accum-loop total count)))
      ((eq? (car msg) 'get-avg)
       (set! query-count (+ query-count 1))  ; BUG: query-count undefined, effect before response
       (send-to! (cadr msg) (/ total count))
       (list 'become (list 'accum-loop total count))))))
```

### Actor 5: Router (2 violations)

```lisp
(define (router-loop routes stats)
  (set! stats (assoc-set stats 'checks (+ (assoc-get stats 'checks 0) 1)))  ; BUG: before guard
  (let msg (receive!)
    (let dest (car msg)
      (let payload (cadr msg)
        (if (assoc-has? routes dest)
          (begin
            (send-to! (assoc-get routes dest) payload)
            (set! stats (assoc-set stats 'routed (+ (assoc-get stats 'routed 0) 1)))  ; BUG: after send
            (list 'become (list 'router-loop routes stats)))
          (begin
            (set! stats (assoc-set stats 'failed (+ (assoc-get stats 'failed 0) 1)))
            (list 'become (list 'router-loop routes stats))))))))
```

## Your Tasks

### Task 1: Enable CSP Enforcement

```lisp
(csp-enforce! true)
(csp-strict! true)
```

### Task 2: Run Each Actor

Spawn each actor and send it messages. The CSP checker should flag violations.

### Task 3: Use Datalog to Find Violations

```lisp
;; Define rule to find effects before guards
(rule 'effect-before-guard
  '(violation ?actor ?var ?time)
  '(effect ?actor set ?var ?time)
  '(not (guard-before ?actor ?time)))

;; Query all violations
(query 'violation '?actor '?var '?time)
```

### Task 4: Fix Each Actor

Show the corrected code for all 5 actors.

### Task 5: Verify Fixes

Run again with CSP enforcement - should have zero violations.

## Expected Violations Summary

| Actor | Violation | Line | Issue |
|-------|-----------|------|-------|
| Counter | set! last-count | 2 | effect before receive! |
| Counter | registry-set! queries | 9 | effect after send-to! |
| Buffer | registry-set! buffer-size | 2 | effect before receive! |
| Buffer | set! stats | 8 | undefined var, wrong order |
| Buffer | registry-set! gets | 17 | effect after send-to! |
| Worker | set! status 'idle | 8 | effect after send-to! |
| Accum | set! last-reset | 12 | effect after send-to! |
| Accum | set! query-count | 14 | undefined, before response |
| Router | set! stats (checks) | 2 | effect before receive! |
| Router | set! stats (routed) | 10 | effect after send-to! |

**Total: 10 violations across 5 actors**

## Corrected Patterns

### Pattern: Pre-guard Initialization

```lisp
;; WRONG
(define (bad-loop state)
  (set! initialized true)  ; effect before guard!
  (let msg (receive!)
    ...))

;; CORRECT - initialize via spawn or first message
(define (good-loop state)
  (let msg (receive!)
    (if (eq? (car msg) 'init)
      (list 'become (list 'good-loop (cadr msg)))  ; state via become
      ...)))
```

### Pattern: Metrics After Response

```lisp
;; WRONG
(define (bad-respond value requester)
  (send-to! requester value)
  (registry-set! 'responses (+ (registry-get 'responses) 1))  ; after send!
  ...)

;; CORRECT - metrics before send, or in next guard cycle
(define (good-respond value requester)
  (registry-set! 'responses (+ (registry-get 'responses) 1))  ; before send
  (send-to! requester value)
  ...)
```

### Pattern: State Updates Around Send

```lisp
;; WRONG  
(define (bad-process msg)
  (set! status 'processing)
  (send-to! target (compute msg))
  (set! status 'done)  ; after send!
  ...)

;; CORRECT - all effects before send
(define (good-process msg)
  (let result (compute msg)
    (set! status 'done)  ; before send
    (set! last-result result)
    (send-to! target result)
    ...))
```

## Questions

1. Why does CSP require guard-before-effect?
2. What bugs does this pattern prevent?
3. How do we handle "effect after response" metrics?
4. Can Datalog detect all violation patterns?
5. What's the runtime cost of CSP enforcement?

## Deliverables

1. **State diagram** showing correct guard/effect ordering
2. **Corrected code** for all 5 actors
3. **Datalog rules** that detect each violation type
4. **Test harness** that exercises each actor and verifies no violations
5. **Explanation** of why each fix works
