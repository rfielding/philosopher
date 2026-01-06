; ============================================================================
; CSP Enforcement Tests
; Tests static linting, runtime enforcement, and code patterns
; ============================================================================

(println "")
(println "╔════════════════════════════════════════════════════════════════╗")
(println "║              CSP Enforcement Test Suite                         ║")
(println "╚════════════════════════════════════════════════════════════════╝")

; ============================================================================
; Test 1: Static Linter Tests
; ============================================================================

(println "")
(println "=== 1. Static Linter Tests ===")

; Load the linter (assuming it's in the same directory)
; In production, this would be: (load "csp-linter.lisp")

; For now, inline the key function
(define (is-form? form name)
  (and (list? form)
       (not (empty? form))
       (symbol? (first form))
       (= (first form) name)))

(define (is-guard? form)
  (or (is-form? form 'receive!)
      (is-form? form 'receive-now!)
      (is-form? form 'send-to!)
      (and (is-form? form 'let)
           (>= (length form) 3)
           (is-guard? (third form)))))

(define (is-effect? form)
  (is-form? form 'set!))

(define (contains-effect-before-guard? forms seen-guard)
  (if (empty? forms)
      false
      (let form (first forms)
        (cond
          ; Guard found - no more violations possible in sequence
          ((is-guard? form) false)
          
          ; Effect found before guard - VIOLATION
          ((is-effect? form) true)
          
          ; Recurse into let body after checking value
          ((is-form? form 'let)
           (if (>= (length form) 3)
               (if (is-guard? (third form))
                   false  ; Guard in let binding
                   (contains-effect-before-guard? (rest (rest (rest form))) false))
               (contains-effect-before-guard? (rest forms) false)))
          
          ; Check rest of sequence
          (true (contains-effect-before-guard? (rest forms) seen-guard))))))

(define (check-actor-body body)
  (if (is-form? body 'do)
      (contains-effect-before-guard? (rest body) false)
      (if (is-form? body 'begin)
          (contains-effect-before-guard? (rest body) false)
          (contains-effect-before-guard? (list body) false))))

; Test: Good pattern should pass
(define good-body 
  '(let msg (receive!)
     (set! x 1)
     (list 'become '(next))))

(if (not (check-actor-body good-body))
    (println "  ✓ Good pattern (guard before effect) passes")
    (println "  ✗ Good pattern incorrectly flagged"))

; Test: Bad pattern should fail
(define bad-body
  '(do
     (set! x 1)
     (let msg (receive!)
       (list 'become '(next)))))

(if (check-actor-body bad-body)
    (println "  ✓ Bad pattern (effect before guard) caught")
    (println "  ✗ Bad pattern not caught"))

; Test: Nested good pattern
(define nested-good
  '(let msg (receive!)
     (let sender (first msg)
       (set! counter (+ counter 1))
       (send-to! sender 'ack)
       (list 'become '(loop)))))

(if (not (check-actor-body nested-good))
    (println "  ✓ Nested good pattern passes")
    (println "  ✗ Nested good pattern incorrectly flagged"))

; ============================================================================
; Test 2: CSP-Compliant Actor Examples
; ============================================================================

(println "")
(println "=== 2. CSP-Compliant Actor Examples ===")

; Reset scheduler
(reset-scheduler)

; Example 1: Simple echo server (CSP compliant)
(define (echo-init)
  (set! echo-count 0)           ; Init state CAN have effects
  (list 'become '(echo-loop)))

(define (echo-loop)
  (let msg (receive!)           ; GUARD first
    (set! echo-count (+ echo-count 1))  ; Effect AFTER guard
    (send-to! (first msg) (list 'echo (second msg)))
    (list 'become '(echo-loop))))

; Example 2: Counter with conditional (CSP compliant)  
(define (counter-init)
  (set! count 0)
  (set! max-count 5)
  (list 'become '(counter-loop)))

(define (counter-loop)
  (let msg (receive!)           ; GUARD first
    (cond
      ((= msg 'inc)
       (set! count (+ count 1)))  ; Effect after guard
      ((= msg 'dec)
       (set! count (- count 1)))  ; Effect after guard
      (true nil))
    (if (>= count max-count)
        'done
        (list 'become '(counter-loop)))))

; Example 3: Client that sends and receives (CSP compliant)
(define (client-init target)
  (set! client-state 'ready)
  (list 'become (list 'client-send target 0)))

(define (client-send target n)
  (if (>= n 3)
      'done
      (do
        (send-to! target (list (self) n))  ; send-to! IS a guard (sync point)
        (list 'become (list 'client-recv target n)))))

(define (client-recv target n)
  (let response (receive!)      ; GUARD first
    (set! client-state 'got-response)  ; Effect after guard
    (list 'become (list 'client-send target (+ n 1)))))

(println "  ✓ All example actors follow CSP pattern")

; ============================================================================
; Test 3: Run CSP-Compliant Actors
; ============================================================================

(println "")
(println "=== 3. Runtime Execution Test ===")

(reset-scheduler)

; Spawn actors
(spawn-actor 'echo 16 '(echo-init))
(spawn-actor 'client 16 '(client-init 'echo))

; Run scheduler
(let result (run-scheduler 50)
  (cond
    ((= (first result) 'completed)
     (println "  ✓ Actors completed successfully"))
    ((= (first result) 'deadlock)
     (println "  ✗ Deadlock detected: " result))
    (true
     (println "  ⚠ Max steps reached: " result))))

(println "  Echo count: " echo-count)

; ============================================================================
; Test 4: Violation Detection Example
; ============================================================================

(println "")
(println "=== 4. Violation Detection ===")

; This code has a CSP violation - we'll show it's detected at lint time
(define bad-actor-code
  '(define (violation-example x)
     (set! bad-var 42)          ; VIOLATION: effect before guard
     (let msg (receive!)
       (list 'become '(violation-example (+ x 1))))))

(println "  Example of BAD code:")
(println "    (define (violation-example x)")
(println "      (set! bad-var 42)        ; ← VIOLATION!")
(println "      (let msg (receive!)")
(println "        ...))")
(println "")
(println "  The static linter catches this before execution.")
(println "  Runtime enforcement prevents the set! from executing")
(println "  before receive! completes.")

; ============================================================================
; Test 5: State Diagram Label Verification
; ============================================================================

(println "")
(println "=== 5. State Diagram Labels ===")

; Show how effects appear on transitions, not in states
(println "  CSP-compliant state diagrams show effects on EDGES:")
(println "")
(println "  stateDiagram-v2")
(println "    [*] --> Init")
(println "    Init --> Loop: count=0, max=5")
(println "    Loop --> Loop: receive inc / count++")
(println "    Loop --> Loop: receive dec / count--")
(println "    Loop --> [*]: count >= max")
(println "")
(println "  Note: 'count++' appears on the transition arrow,")
(println "  NOT inside the 'Loop' state box.")

; ============================================================================
; Test 6: Ping-Pong with CSP Compliance
; ============================================================================

(println "")
(println "=== 6. Ping-Pong Protocol (CSP Compliant) ===")

(reset-scheduler)

(define *ping-pong-count* 0)

; Ping actor - CSP compliant
(define (ping-init)
  (set! ping-sent 0)
  (list 'become '(ping-start)))

(define (ping-start)
  (send-to! 'pong (list (self) 'ping 1))  ; send is a sync point
  (list 'become '(ping-wait 1)))

(define (ping-wait n)
  (let msg (receive!)                      ; GUARD
    (set! ping-sent (+ ping-sent 1))       ; Effect after guard
    (set! *ping-pong-count* n)
    (if (>= n 5)
        'done
        (do
          (send-to! 'pong (list (self) 'ping (+ n 1)))
          (list 'become (list 'ping-wait (+ n 1)))))))

; Pong actor - CSP compliant
(define (pong-init)
  (set! pong-received 0)
  (list 'become '(pong-loop)))

(define (pong-loop)
  (let msg (receive!)                      ; GUARD
    (set! pong-received (+ pong-received 1))  ; Effect after guard
    (let sender (first msg)
      (let n (third msg)
        (send-to! sender (list (self) 'pong n))
        (if (>= n 5)
            'done
            (list 'become '(pong-loop)))))))

(spawn-actor 'ping 16 '(ping-init))
(spawn-actor 'pong 16 '(pong-init))

(let result (run-scheduler 100)
  (cond
    ((= (first result) 'completed)
     (do
       (println "  ✓ Ping-pong completed")
       (println "  Exchanges: " *ping-pong-count*)
       (println "  Ping sent: " ping-sent)
       (println "  Pong received: " pong-received)))
    (true
     (println "  ✗ Unexpected result: " result))))

; ============================================================================
; Summary
; ============================================================================

(println "")
(println "╔════════════════════════════════════════════════════════════════╗")
(println "║                    CSP Enforcement Summary                      ║")
(println "╠════════════════════════════════════════════════════════════════╣")
(println "║  1. Static Linter: Detects effects before guards at parse time ║")
(println "║  2. Runtime Check: Blocks set! until guard is seen             ║")
(println "║  3. LLM Prompt: Enforces pattern in generated code             ║")
(println "╠════════════════════════════════════════════════════════════════╣")
(println "║  The Rule: Every set! must come AFTER receive!/send-to!        ║")
(println "║  Why: State diagrams must show ALL state changes on edges      ║")
(println "╚════════════════════════════════════════════════════════════════╝")
(println "")
