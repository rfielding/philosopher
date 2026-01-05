; ============================================================================
; CSP Static Linter - Verifies effects occur only in transitions, not on state entrance
; ============================================================================
;
; CSP RULE: In proper CSP, a state is a stable configuration waiting for events.
; Variable modifications (set!) must happen AFTER a blocking guard (receive!, send-to!).
;
; WRONG (effect on state entrance):
;   (define (my-state x)
;     (set! counter (+ counter 1))  ; ← VIOLATION: before any guard
;     (let msg (receive!)
;       (become (next-state))))
;
; RIGHT (effect in transition):
;   (define (my-state x)
;     (let msg (receive!)           ; ← guard first
;       (set! counter (+ counter 1)) ; ← effect after guard
;       (become (next-state))))
;
; ============================================================================

; ----------------------------------------------------------------------------
; AST Utilities
; ----------------------------------------------------------------------------

; Check if a form is a specific special form
(define (is-form? form name)
  (and (list? form)
       (not (empty? form))
       (symbol? (first form))
       (= (first form) name)))

; Check if form is a blocking operation (guard)
(define (is-guard? form)
  (or (is-form? form 'receive!)
      (is-form? form 'receive-now!)
      (is-form? form 'send-to!)
      (is-form? form 'send!)
      (is-form? form 'recv!)
      (is-form? form 'pop!)
      (is-form? form 'push!)
      ; Also treat 'let with receive as a guard pattern
      (and (is-form? form 'let)
           (>= (length form) 3)
           (is-guard? (third form)))))

; Check if form is an effect (state mutation)
(define (is-effect? form)
  (or (is-form? form 'set!)
      (is-form? form 'define)))

; Check if form is a state transition
(define (is-transition? form)
  (or (is-form? form 'become)
      (and (is-form? form 'list)
           (>= (length form) 2)
           (= (second form) (quote 'become)))))

; ----------------------------------------------------------------------------
; Violation Detection
; ----------------------------------------------------------------------------

; A violation is: (location description form)
(define (make-violation loc desc form)
  (list loc desc form))

(define (violation-location v) (first v))
(define (violation-description v) (second v))
(define (violation-form v) (third v))

; Walk an AST and collect effects that appear before any guard
; Returns: (list has-seen-guard violations)
(define (analyze-for-early-effects form path)
  (cond
    ; Nil or atom - no effects, no guards
    ((nil? form) (list false '()))
    ((not (list? form)) (list false '()))
    ((empty? form) (list false '()))
    
    ; Guard found - mark that we've seen one
    ((is-guard? form)
     (list true '()))
    
    ; Effect found - violation if no guard seen yet
    ((is-effect? form)
     (list false (list (make-violation path "set! before guard" form))))
    
    ; let binding - check value, then body
    ((is-form? form 'let)
     (let* ((binding-result (analyze-for-early-effects (third form) (append path '(let-value))))
            (saw-guard (first binding-result))
            (violations (second binding-result)))
       (if saw-guard
           ; Guard in binding - body effects are OK
           (list true violations)
           ; No guard yet - check body for early effects
           (let body-forms (rest (rest (rest form)))
             (analyze-sequence body-forms (append path '(let-body)) saw-guard violations)))))
    
    ; do/begin - check sequence
    ((or (is-form? form 'do) (is-form? form 'begin))
     (analyze-sequence (rest form) path false '()))
    
    ; if - check condition, then both branches
    ((is-form? form 'if)
     (let* ((cond-result (analyze-for-early-effects (second form) (append path '(if-cond))))
            (saw-guard (first cond-result))
            (violations (second cond-result)))
       (if saw-guard
           ; Guard in condition - branches are OK
           (list true violations)
           ; Check both branches
           (let* ((then-result (analyze-for-early-effects (third form) (append path '(if-then))))
                  (then-guard (first then-result))
                  (then-violations (second then-result))
                  (else-result (if (>= (length form) 4)
                                   (analyze-for-early-effects (fourth form) (append path '(if-else)))
                                   (list false '())))
                  (else-guard (first else-result))
                  (else-violations (second else-result)))
             ; Need guard in BOTH branches for it to count
             (list (and then-guard else-guard)
                   (append violations then-violations else-violations))))))
    
    ; cond - check all clauses
    ((is-form? form 'cond)
     (analyze-cond-clauses (rest form) path false '()))
    
    ; match - check all clauses
    ((is-form? form 'match)
     (let* ((target-result (analyze-for-early-effects (second form) (append path '(match-target))))
            (saw-guard (first target-result))
            (violations (second target-result)))
       (if saw-guard
           (list true violations)
           (analyze-match-clauses (rest (rest form)) path saw-guard violations))))
    
    ; lambda/fn - defines a new scope, analyze the body
    ((or (is-form? form 'lambda) (is-form? form 'fn))
     (let body-forms (rest (rest form))
       (analyze-sequence body-forms (append path '(lambda-body)) false '())))
    
    ; define with function body - analyze the function
    ((and (is-form? form 'define)
          (>= (length form) 3)
          (list? (second form)))
     (let body-forms (rest (rest form))
       (analyze-sequence body-forms (append path (list 'define (first (second form)))) false '())))
    
    ; Other list forms - could be function calls, check args
    (true
     (analyze-sequence (rest form) path false '()))))

; Analyze a sequence of forms
(define (analyze-sequence forms path saw-guard violations)
  (if (empty? forms)
      (list saw-guard violations)
      (let* ((result (analyze-for-early-effects (first forms) path))
             (new-guard (or saw-guard (first result)))
             (new-violations (if saw-guard
                                 violations  ; Already saw guard, no new violations
                                 (append violations (second result)))))
        (analyze-sequence (rest forms) path new-guard new-violations))))

; Analyze cond clauses
(define (analyze-cond-clauses clauses path saw-guard violations)
  (if (empty? clauses)
      (list saw-guard violations)
      (let* ((clause (first clauses))
             (test-result (analyze-for-early-effects (first clause) (append path '(cond-test))))
             (body-result (analyze-for-early-effects (second clause) (append path '(cond-body))))
             (clause-guard (or (first test-result) (first body-result)))
             (clause-violations (append (second test-result) (second body-result))))
        (analyze-cond-clauses (rest clauses) path 
                              (and saw-guard clause-guard)  ; Need guard in ALL clauses
                              (if saw-guard violations (append violations clause-violations))))))

; Analyze match clauses
(define (analyze-match-clauses clauses path saw-guard violations)
  (if (empty? clauses)
      (list saw-guard violations)
      (let* ((clause (first clauses))
             (body-result (analyze-for-early-effects (second clause) (append path '(match-body))))
             (clause-guard (first body-result))
             (clause-violations (second body-result)))
        (analyze-match-clauses (rest clauses) path
                               (and saw-guard clause-guard)
                               (if saw-guard violations (append violations clause-violations))))))

; ----------------------------------------------------------------------------
; Public API
; ----------------------------------------------------------------------------

; Lint a single form (typically a define)
(define (lint-form form)
  (let result (analyze-for-early-effects form '())
    (second result)))

; Lint a list of forms (a file)
(define (lint-forms forms)
  (if (empty? forms)
      '()
      (append (lint-form (first forms))
              (lint-forms (rest forms)))))

; Check if a form represents an actor state function
; Heuristic: defines a function that uses become or receive
(define (is-actor-state? form)
  (and (is-form? form 'define)
       (>= (length form) 3)
       (list? (second form))
       (or (contains-symbol? form 'become)
           (contains-symbol? form 'receive!)
           (contains-symbol? form 'send-to!))))

(define (contains-symbol? form sym)
  (cond
    ((nil? form) false)
    ((symbol? form) (= form sym))
    ((not (list? form)) false)
    ((empty? form) false)
    (true (or (contains-symbol? (first form) sym)
              (contains-symbol? (rest form) sym)))))

; Lint only actor state functions
(define (lint-actor-states forms)
  (let actor-forms (filter is-actor-state? forms)
    (lint-forms actor-forms)))

; Format violations for display
(define (format-violation v)
  (string-append "  ✗ " (violation-description v) "\n"
                 "    at: " (repr (violation-location v)) "\n"
                 "    form: " (repr (violation-form v))))

(define (format-violations violations)
  (if (empty? violations)
      "  ✓ No CSP violations found"
      (let formatted (map format-violation violations)
        (fold-left string-append "" formatted))))

; Main linting function with nice output
(define (csp-lint forms)
  (println "")
  (println "=== CSP Linter: Checking for effects before guards ===")
  (let violations (lint-actor-states forms)
    (println (format-violations violations))
    (println "")
    (if (empty? violations)
        (do (println "All actor states follow CSP pattern ✓")
            true)
        (do (println (string-append "Found " (number->string (length violations)) " violation(s) ✗"))
            false))))

; ----------------------------------------------------------------------------
; Fold helper (not in base language)
; ----------------------------------------------------------------------------

(define (fold-left f init lst)
  (if (empty? lst)
      init
      (fold-left f (f init (first lst)) (rest lst))))

; ----------------------------------------------------------------------------
; Self-test
; ----------------------------------------------------------------------------

(define (test-csp-linter)
  (println "")
  (println "=== CSP Linter Self-Test ===")
  
  ; Test 1: Good pattern - effect after guard
  (let good-form '(define (good-state x)
                    (let msg (receive!)
                      (set! counter (+ counter 1))
                      (list 'become '(good-state (+ x 1)))))
    (let violations (lint-form good-form)
      (if (empty? violations)
          (println "  ✓ Good pattern passes")
          (println "  ✗ Good pattern should pass"))))
  
  ; Test 2: Bad pattern - effect before guard
  (let bad-form '(define (bad-state x)
                   (set! counter (+ counter 1))
                   (let msg (receive!)
                     (list 'become '(bad-state (+ x 1)))))
    (let violations (lint-form bad-form)
      (if (not (empty? violations))
          (println "  ✓ Bad pattern caught")
          (println "  ✗ Bad pattern should be caught"))))
  
  ; Test 3: Effect in both branches
  (let branch-form '(define (branch-state x)
                      (if (> x 0)
                          (set! a 1)
                          (set! b 2))
                      (let msg (receive!)
                        (list 'become '(next))))
    (let violations (lint-form branch-form)
      (if (not (empty? violations))
          (println "  ✓ Branch effects caught")
          (println "  ✗ Branch effects should be caught"))))
  
  ; Test 4: Guard in let, effect in body
  (let let-guard-form '(define (let-guard-state)
                         (let msg (receive!)
                           (set! x (first msg))
                           (list 'become '(next))))
    (let violations (lint-form let-guard-form)
      (if (empty? violations)
          (println "  ✓ Let-guard pattern passes")
          (println "  ✗ Let-guard pattern should pass"))))
  
  (println ""))

; Run self-test when loaded
(test-csp-linter)

(println "; CSP Linter loaded. Use (csp-lint forms) to check code.")
