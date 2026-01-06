; ============================================================================
; Example: Worker Pool
; Workers receive tasks, process them, report results
; ============================================================================

; The text format we generate looks like:
;
; actor worker:
;   vars: x = 0, capacity = 8
;   stacks: ctx = 4
;
;   [idle | x = 0]
;     --> [working | x > 0] : arrive / recv! tasks msg / x := x + 1
;   [working | x > 0]
;     --> [working | x > 0] : arrive / recv! tasks msg / x := x + 1
;     --> [working | x > 0] : process / choice(0.7: x := x-1, send! results done)
;     --> [idle | x = 0] : drain / x = 0
;   [full | x = capacity]
;     --> [working | x > 0] : process / choice(0.7: x := x-1, send! results done)
;
; This is complete - predicates, channel ops, mutations all shown.
; LLM can read it, user can read it, and we can parse it back to Lisp.

; ============================================================================
; Define the Worker actor
; ============================================================================

(defactor 
  '(actor worker
    (vars (x 0) (capacity 8))
    (stacks (ctx 4))
    
    ; Guards - named predicates (can overlap!)
    (guard idle (= x 0))
    (guard working (> x 0))
    (guard full (= x capacity))
    
    ; Transitions: from → to with channel op and mutations
    (trans arrive
      (from idle)
      (to working)
      (recv! tasks msg)
      (set! x (+ x 1)))
    
    (trans arrive-more
      (from working)
      (to working)
      (recv! tasks msg)
      (set! x (+ x 1)))
    
    (trans process
      (from working)
      (to working)
      (choice (0.7 (set! x (- x 1)) (send! results 'done))
              (0.3)))
    
    (trans reject
      (from full)
      (to full)
      (send! overflow 'full))))

; ============================================================================
; Define the Client actor
; ============================================================================

(defactor
  '(actor client
    (vars (pending 0) (acked 0))
    (stacks)
    
    (guard ready (< pending 10))
    (guard waiting (> pending 0))
    (guard done (and (= pending 0) (> acked 0)))
    
    (trans submit
      (from ready)
      (to waiting)
      (send! tasks 'work)
      (set! pending (+ pending 1)))
    
    (trans receive-ack
      (from waiting)
      (to waiting)
      (recv! results msg)
      (set! pending (- pending 1))
      (set! acked (+ acked 1)))))

; ============================================================================
; Show the diagrams
; ============================================================================

(println "")
(println "=== TEXT FORMAT (round-trip friendly) ===")
(show-text)

(println "")
(println "=== MERMAID STATE DIAGRAMS ===")
(show-states)

(println "")
(println "=== MERMAID SEQUENCE DIAGRAM ===")
(show-sequence)

; ============================================================================
; The key insight:
;
; The TEXT format above is what the LLM generates and what the user reviews.
; It maps directly to/from Lisp. Nothing is hidden.
;
; Guards show predicates:     [working | x > 0]
; Transitions show actions:   --> [idle | x = 0] : drain / x = 0
; Channel ops are explicit:   recv! tasks msg
; Mutations are explicit:     x := x + 1
;
; This is the contract between human understanding and formal spec.
; ============================================================================
