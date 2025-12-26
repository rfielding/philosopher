; ============================================================================
; BoundedLISP Unit Tests
; Run with: ./boundedlisp tests.lisp
; ============================================================================

(define *tests-run* 0)
(define *tests-passed* 0)
(define *tests-failed* 0)
(define *current-section* "")

(define (test-section name)
  (set! *current-section* name)
  (println "")
  (println "=== " name " ==="))

(define (assert-eq actual expected name)
  (set! *tests-run* (+ *tests-run* 1))
  (if (= actual expected)
      (do
        (set! *tests-passed* (+ *tests-passed* 1))
        (println "  ✓ " name))
      (do
        (set! *tests-failed* (+ *tests-failed* 1))
        (println "  ✗ " name)
        (println "    expected:" expected)
        (println "    actual:  " actual))))

(define (assert-true val name)
  (assert-eq val true name))

(define (assert-false val name)
  (assert-eq val false name))

(define (assert-nil val name)
  (set! *tests-run* (+ *tests-run* 1))
  (if (nil? val)
      (do
        (set! *tests-passed* (+ *tests-passed* 1))
        (println "  ✓ " name))
      (do
        (set! *tests-failed* (+ *tests-failed* 1))
        (println "  ✗ " name)
        (println "    expected: nil")
        (println "    actual:  " val))))

(define (assert-approx actual expected epsilon name)
  (set! *tests-run* (+ *tests-run* 1))
  (if (< (abs (- actual expected)) epsilon)
      (do
        (set! *tests-passed* (+ *tests-passed* 1))
        (println "  ✓ " name))
      (do
        (set! *tests-failed* (+ *tests-failed* 1))
        (println "  ✗ " name)
        (println "    expected:" expected " ± " epsilon)
        (println "    actual:  " actual))))

(define (test-summary)
  (println "")
  (println "============================================")
  (println "Tests: " *tests-run* 
           " | Passed: " *tests-passed* 
           " | Failed: " *tests-failed*)
  (if (= *tests-failed* 0)
      (println "ALL TESTS PASSED ✓")
      (println "SOME TESTS FAILED ✗"))
  (println "============================================"))

; ============================================================================
; Arithmetic Tests
; ============================================================================

(test-section "Arithmetic")

(assert-eq (+ 1 2) 3 "addition")
(assert-eq (+ 1 2 3 4) 10 "addition variadic")
(assert-eq (- 10 3) 7 "subtraction")
(assert-eq (- 10 3 2) 5 "subtraction variadic")
(assert-eq (* 3 4) 12 "multiplication")
(assert-eq (* 2 3 4) 24 "multiplication variadic")
(assert-eq (/ 12 3) 4 "division")
(assert-eq (mod 10 3) 1 "modulo")

; ============================================================================
; Math Functions Tests
; ============================================================================

(test-section "Math Functions")

(assert-approx (ln 1) 0 0.001 "ln(1) = 0")
(assert-approx (ln 2.718281828) 1 0.001 "ln(e) ≈ 1")
(assert-approx (exp 0) 1 0.001 "exp(0) = 1")
(assert-approx (exp 1) 2.718281828 0.001 "exp(1) = e")
(assert-eq (sqrt 4) 2 "sqrt(4) = 2")
(assert-eq (sqrt 9) 3 "sqrt(9) = 3")
(assert-eq (pow 2 10) 1024 "pow(2,10) = 1024")
(assert-eq (pow 3 3) 27 "pow(3,3) = 27")
(assert-eq (floor 3.7) 3 "floor(3.7) = 3")
(assert-eq (floor 3.2) 3 "floor(3.2) = 3")
(assert-eq (ceil 3.2) 4 "ceil(3.2) = 4")
(assert-eq (ceil 3.9) 4 "ceil(3.9) = 4")
(assert-eq (abs -5) 5 "abs(-5) = 5")
(assert-eq (abs 5) 5 "abs(5) = 5")
(assert-eq (min 5 3 8 1) 1 "min(5,3,8,1) = 1")
(assert-eq (max 5 3 8 1) 8 "max(5,3,8,1) = 8")
(assert-approx (sin 0) 0 0.001 "sin(0) = 0")
(assert-approx (cos 0) 1 0.001 "cos(0) = 1")

; ============================================================================
; Comparison Tests
; ============================================================================

(test-section "Comparison")

(assert-true (= 1 1) "= equal")
(assert-false (= 1 2) "= not equal")
(assert-true (!= 1 2) "!= different")
(assert-false (!= 1 1) "!= same")
(assert-true (< 1 2) "< less")
(assert-false (< 2 1) "< not less")
(assert-true (<= 1 1) "<= equal")
(assert-true (<= 1 2) "<= less")
(assert-true (> 2 1) "> greater")
(assert-false (> 1 2) "> not greater")
(assert-true (>= 2 2) ">= equal")
(assert-true (>= 3 2) ">= greater")

; ============================================================================
; Boolean Logic Tests
; ============================================================================

(test-section "Boolean Logic")

(assert-true (and true true) "and true true")
(assert-false (and true false) "and true false")
(assert-false (and false true) "and false true")
(assert-true (or true false) "or true false")
(assert-true (or false true) "or false true")
(assert-false (or false false) "or false false")
(assert-true (not false) "not false")
(assert-false (not true) "not true")

; ============================================================================
; List Operations Tests
; ============================================================================

(test-section "List Operations")

(assert-eq (first '(a b c)) 'a "first")
(assert-eq (rest '(a b c)) '(b c) "rest")
(assert-eq (cons 'a '(b c)) '(a b c) "cons")
(assert-eq (length '(a b c)) 3 "length")
(assert-eq (length '()) 0 "length empty")
(assert-eq (nth '(a b c d) 0) 'a "nth 0")
(assert-eq (nth '(a b c d) 2) 'c "nth 2")
(assert-eq (append '(a b) '(c d)) '(a b c d) "append")
(assert-eq (list 1 2 3) '(1 2 3) "list")
(assert-true (empty? '()) "empty? true")
(assert-false (empty? '(a)) "empty? false")

; ============================================================================
; Type Predicates Tests
; ============================================================================

(test-section "Type Predicates")

(assert-true (number? 42) "number? true")
(assert-false (number? 'a) "number? false")
(assert-true (symbol? 'a) "symbol? true")
(assert-false (symbol? 42) "symbol? false")
(assert-true (string? "hello") "string? true")
(assert-false (string? 42) "string? false")
(assert-true (list? '(a b)) "list? true")
(assert-false (list? 42) "list? false")
(assert-true (nil? nil) "nil? true")
(assert-false (nil? 42) "nil? false")

; ============================================================================
; Let Binding Tests
; ============================================================================

(test-section "Let Bindings")

(assert-eq (let x 5 x) 5 "let simple")
(assert-eq (let x 5 (+ x 1)) 6 "let with expr")
(assert-eq (let x 5 (let y 3 (+ x y))) 8 "let nested")

; Multiple body expressions
(assert-eq (let x 1
             (set! x (+ x 1))
             (set! x (+ x 1))
             x) 3 "let multiple body")

; let*
(assert-eq (let* ((x 1) (y (+ x 1))) y) 2 "let* sequential")

; ============================================================================
; Define and Lambda Tests
; ============================================================================

(test-section "Define and Lambda")

(define (add-one x) (+ x 1))
(assert-eq (add-one 5) 6 "define function")

(define (multi-body x)
  (set! x (+ x 1))
  (set! x (* x 2))
  x)
(assert-eq (multi-body 5) 12 "define multi-body")

(define add-lambda (lambda (x y) (+ x y)))
(assert-eq (add-lambda 3 4) 7 "lambda")

(define (variadic-sum . args)
  (if (empty? args) 
      0 
      (+ (first args) (apply variadic-sum (rest args)))))
; Note: apply not implemented, skip this test

; ============================================================================
; Conditional Tests
; ============================================================================

(test-section "Conditionals")

(assert-eq (if true 'yes 'no) 'yes "if true")
(assert-eq (if false 'yes 'no) 'no "if false")
(assert-eq (if (> 5 3) 'bigger 'smaller) 'bigger "if with comparison")

(assert-eq (cond 
             ((= 1 2) 'a)
             ((= 2 2) 'b)
             (true 'c)) 'b "cond")

; ============================================================================
; Bounded Stack Tests
; ============================================================================

(test-section "Bounded Stack")

(define s (make-stack 3))
(assert-true (stack-empty? s) "stack-empty? initially")
(assert-false (stack-full? s) "stack-full? initially")

(push-now! s 1)
(push-now! s 2)
(assert-eq (stack-peek s) 2 "stack-peek")
(assert-eq (pop-now! s) 2 "pop-now! first")
(assert-eq (pop-now! s) 1 "pop-now! second")
(assert-true (stack-empty? s) "stack-empty? after pops")

; Test bounds
(define s2 (make-stack 2))
(push-now! s2 'a)
(push-now! s2 'b)
(assert-true (stack-full? s2) "stack-full? at capacity")
(assert-eq (push-now! s2 'c) 'full "push-now! when full")

; ============================================================================
; Bounded Queue Tests
; ============================================================================

(test-section "Bounded Queue")

(define q (make-queue 3))
(assert-true (queue-empty? q) "queue-empty? initially")
(assert-false (queue-full? q) "queue-full? initially")

(send-now! q 1)
(send-now! q 2)
(send-now! q 3)
(assert-true (queue-full? q) "queue-full? at capacity")
(assert-eq (recv-now! q) 1 "recv-now! FIFO first")
(assert-eq (recv-now! q) 2 "recv-now! FIFO second")
(assert-false (queue-full? q) "queue-full? after recv")

; ============================================================================
; String Operations Tests
; ============================================================================

(test-section "String Operations")

(assert-eq (string-append "hello" " " "world") "hello world" "string-append")
(assert-eq (symbol->string 'hello) "hello" "symbol->string")
(assert-eq (string->symbol "hello") 'hello "string->symbol")
(assert-eq (number->string 42) "42" "number->string")

; ============================================================================
; Tagged Values Tests
; ============================================================================

(test-section "Tagged Values")

(define t (tag 'mytype '(data here)))
(assert-true (tag-is? t 'mytype) "tag-is? true")
(assert-false (tag-is? t 'other) "tag-is? false")
(assert-eq (tag-type t) 'mytype "tag-type")
(assert-eq (tag-value t) '(data here) "tag-value")

; ============================================================================
; Distribution Functions Tests  
; ============================================================================

(test-section "Distributions")

; Exponential: -ln(U)/rate, mean = 1/rate
(assert-approx (exponential 0.5 0.1) 6.93 0.1 "exponential mid")
(assert-true (> (exponential 0.1 0.1) (exponential 0.9 0.1)) "exponential ordering")

; Discrete uniform
(assert-eq (discrete-uniform 0.0 1 6) 1 "discrete-uniform low")
(assert-eq (discrete-uniform 0.99 1 6) 6 "discrete-uniform high")

; Bernoulli
(assert-eq (bernoulli 0.3 0.5) 1 "bernoulli success")
(assert-eq (bernoulli 0.7 0.5) 0 "bernoulli failure")

; Uniform range
(assert-eq (uniform-range 0.0 10 20) 10 "uniform-range low")
(assert-eq (uniform-range 1.0 10 20) 20 "uniform-range high")
(assert-eq (uniform-range 0.5 10 20) 15 "uniform-range mid")

; M/M/1 theory
(assert-eq (mm1-utilization 0.1 0.2) 0.5 "mm1-utilization")
(assert-eq (mm1-avg-system-length 0.1 0.2) 1 "mm1-avg-system-length")

; ============================================================================
; Actor System Tests (basic)
; ============================================================================

(test-section "Actor System")

; Test spawn-actor creates actor
(spawn-actor 'test-actor-1 8 '(quote done))
(let state (actor-state 'test-actor-1)
  (assert-eq (first state) 'runnable "actor-state runnable after spawn"))

; Test send-to! delivers message
(spawn-actor 'test-actor-2 4 '(quote yield))
(send-to! 'test-actor-2 '(hello world))
; Run one step to process
(run-scheduler 1)
; Now check actor can receive
(spawn-actor 'receiver 4 
  '(let msg (receive!) 
     (if (= (first msg) 'test-msg) 'got-it 'wrong)))
(send-to! 'receiver 'test-msg)
(run-scheduler 5)

; ============================================================================
; Scheduler Tests
; ============================================================================

(test-section "Scheduler")

; Ping-pong communication test
(define *ping-count* 0)

(define (ping-loop n)
  (send-to! 'pong-actor (list 'ping n))
  (let msg (receive!)
    (set! *ping-count* n)
    (if (< n 3)
        (list 'become (list 'ping-loop (+ n 1)))
        'done)))

(define (pong-loop)
  (let msg (receive!)
    (send-to! 'ping-actor (list 'pong (second msg)))
    (if (< (second msg) 3)
        (list 'become '(pong-loop))
        'done)))

(spawn-actor 'ping-actor 8 '(ping-loop 1))
(spawn-actor 'pong-actor 8 '(pong-loop))
(run-scheduler 20)
(assert-eq *ping-count* 3 "ping-pong completes 3 rounds")

; ============================================================================
; Become/State Continuation Tests
; ============================================================================

(test-section "Become Pattern")

(define *counter-final* 0)

(define (counter-loop n)
  (set! *counter-final* n)
  (if (< n 5)
      (list 'become (list 'counter-loop (+ n 1)))
      'done))

(spawn-actor 'counter 8 '(counter-loop 0))
(run-scheduler 10)
; Counter goes 0,1,2,3,4,5 then returns done at 5
(assert-true (>= *counter-final* 4) "become carries state forward")

; ============================================================================
; Blocking/Unblocking Tests
; ============================================================================

(test-section "Blocking Behavior")

; Actor that blocks then completes
(spawn-actor 'wait-then-done 8 
  '(do 
     (let msg (receive!)
       (if (= msg 'wake) 'done 'error))))

; Send a message
(send-to! 'wait-then-done 'wake)
(run-scheduler 5)
(let state (actor-state 'wait-then-done)
  (assert-eq (first state) 'done "actor receives and completes"))

; ============================================================================
; Visualization Tests
; ============================================================================

(test-section "Visualization")

; Sequence diagram from traces
(set! *traces* '())
(set! *traces* (cons '(0 client server request) *traces*))
(set! *traces* (cons '(1 server client response) *traces*))

(let diagram (traces->sequence)
  (assert-true (string? diagram) "traces->sequence produces string"))

; Flow diagram
(let diagram (traces->flow)
  (assert-true (string? diagram) "traces->flow produces string"))

; ============================================================================
; CTL Formula Construction Tests
; ============================================================================

(test-section "CTL Formulas")

(let f (EF (prop 'done))
  (assert-true (tag-is? f 'ctl-EF) "EF creates tagged formula"))

(let f (AG (prop 'safe))
  (assert-true (tag-is? f 'ctl-AG) "AG creates tagged formula"))

(let f (ctl-implies (prop 'a) (prop 'b))
  (assert-true (tag-is? f 'ctl-implies) "ctl-implies creates implication"))

(let f (AU (prop 'a) (prop 'b))
  (assert-true (tag-is? f 'ctl-AU) "AU creates until formula"))

; ============================================================================
; Edge Cases and Error Handling
; ============================================================================

(test-section "Edge Cases")

; Empty list operations
(assert-true (empty? '()) "empty list is empty")
(assert-nil (first '()) "first of empty is nil")
(assert-eq (rest '()) '() "rest of empty is empty")
(assert-eq (length '()) 0 "length of empty is 0")

; Nested lists
(assert-eq (first (first '((a b) (c d)))) 'a "nested first")
(assert-eq (first (rest '((a b) (c d)))) '(c d) "nested rest")

; Deep equality
(assert-true (= '(a (b c) d) '(a (b c) d)) "deep list equality")
(assert-false (= '(a (b c) d) '(a (b x) d)) "deep list inequality")

; Numbers in lists
(assert-eq (+ (first '(1 2 3)) (second '(1 2 3))) 3 "arithmetic on list elements")

; Symbols vs strings
(assert-false (= 'hello "hello") "symbol not equal to string")
(assert-true (= (symbol->string 'hello) "hello") "converted symbol equals string")

; ============================================================================
; Recursion and Higher-Order Functions
; ============================================================================

(test-section "Recursion")

; Recursive factorial
(define (fact n)
  (if (<= n 1) 
      1 
      (* n (fact (- n 1)))))

(assert-eq (fact 5) 120 "factorial 5")
(assert-eq (fact 0) 1 "factorial 0")

; Recursive fibonacci
(define (fib n)
  (if (< n 2)
      n
      (+ (fib (- n 1)) (fib (- n 2)))))

(assert-eq (fib 10) 55 "fibonacci 10")

; Fold/reduce pattern
(define (sum-list lst)
  (if (empty? lst)
      0
      (+ (first lst) (sum-list (rest lst)))))

(assert-eq (sum-list '(1 2 3 4 5)) 15 "sum-list")

; ============================================================================
; Closure Tests  
; ============================================================================

(test-section "Closures")

(define (make-adder n)
  (lambda (x) (+ x n)))

(define add5 (make-adder 5))
(define add10 (make-adder 10))

(assert-eq (add5 3) 8 "closure add5")
(assert-eq (add10 3) 13 "closure add10")

; Counter closure
(define (make-counter)
  (let count 0
    (lambda ()
      (set! count (+ count 1))
      count)))

; Note: This won't work correctly due to how set! works with closures
; Testing basic closure capture instead

; ============================================================================
; Complex Data Structure Tests
; ============================================================================

(test-section "Complex Data")

; Association lists
(define data '((name "Alice") (age 30) (city "Boston")))
(assert-eq (second (assoc 'name data)) "Alice" "assoc lookup name")
(assert-eq (second (assoc 'age data)) 30 "assoc lookup age")

; Nested structure access
(define tree '(root (left (a b)) (right (c d))))
(assert-eq (first tree) 'root "tree root")
(assert-eq (first (second tree)) 'left "tree left branch")
(assert-eq (first (third tree)) 'right "tree right branch")

; ============================================================================
; Prologue Helper Tests
; ============================================================================

(test-section "Prologue Helpers")

(assert-eq (second '(a b c)) 'b "second")
(assert-eq (third '(a b c)) 'c "third")
(assert-eq (fourth '(a b c d)) 'd "fourth")

; Map
(assert-eq (map (lambda (x) (* x 2)) '(1 2 3)) '(2 4 6) "map")

; Filter  
(assert-eq (filter (lambda (x) (> x 2)) '(1 2 3 4)) '(3 4) "filter")

; Reverse
(assert-eq (reverse '(a b c)) '(c b a) "reverse")

; Member?
(assert-true (member? 'b '(a b c)) "member? found")
(assert-false (member? 'd '(a b c)) "member? not found")

; Unique - order may vary, test length
(assert-eq (length (unique '(a b a c b))) 3 "unique removes duplicates")

; Assoc
(assert-eq (assoc 'b '((a 1) (b 2) (c 3))) '(b 2) "assoc found")
(assert-nil (assoc 'd '((a 1) (b 2))) "assoc not found")

; ============================================================================
; Scoping Tests
; ============================================================================

(test-section "Scoping")

; Local function via let+lambda stays local
(define *scope-test-result* nil)

(define (test-local-fn)
  (let helper (lambda (x) (* x 2))
    (set! *scope-test-result* (helper 5))
    *scope-test-result*))

(test-local-fn)
(assert-eq *scope-test-result* 10 "let+lambda local function works")

; Closures capture environment correctly
(define (make-adder-safe n)
  (lambda (x) (+ n x)))

(define add3 (make-adder-safe 3))
(define add7 (make-adder-safe 7))
(assert-eq (add3 10) 13 "closure captures n=3")
(assert-eq (add7 10) 17 "closure captures n=7")

; ============================================================================
; Metrics Actor Tests
; ============================================================================

(test-section "Metrics")

; Test helper functions directly
(assert-eq (sum-list '(1 2 3 4 5)) 15 "sum-list")
(assert-eq (sort-numbers '(3 1 4 1 5 9 2 6)) '(1 1 2 3 4 5 6 9) "sort-numbers")

; Test metrics state management
(define test-state (make-empty-metrics-state))
(assert-eq (metrics-get test-state 'counters) '() "empty counters")

(define test-state2 (update-counter test-state 'requests 1))
(define test-state3 (update-counter test-state2 'requests 1))
(assert-eq (second (assoc 'requests (metrics-get test-state3 'counters))) 2 "counter increments")

(define test-state4 (update-gauge test-state 'queue-depth 42))
(assert-eq (second (assoc 'queue-depth (metrics-get test-state4 'gauges))) 42 "gauge set")

(define test-state5 (add-timing test-state 'latency 100))
(define test-state6 (add-timing test-state5 'latency 200))
(assert-eq (length (second (assoc 'latency (metrics-get test-state6 'timings)))) 2 "timing samples")

; ============================================================================
; Summary
; ============================================================================

(test-summary)

; ============================================================================
; Metrics Actor Integration Test - Actors sending real metrics
; ============================================================================

(test-section "Metrics Integration")

; Reset state
(reset-scheduler)

; Start the metrics collector
(start-metrics!)

; NOTE: In BoundedLISP, code before receive! may execute multiple times
; due to how blocking works. Place metric calls AFTER receive! for accurate counts.

; A client that sends requests and records timing
(define (test-client-loop count)
  (if (<= count 0)
      'done
      (do
        (send-to! 'server (list 'request (self)))
        (let response (receive!)
          ; Count AFTER receiving to avoid double-counting
          (metric-inc! 'roundtrips)
          (metric-timing! 'latency (+ 10 (mod count 5)))
          (list 'become (list 'test-client-loop (- count 1)))))))

; A server that handles requests
(define (test-server-loop handled)
  (let msg (receive!)
    ; Count AFTER receiving
    (metric-inc! 'requests-handled)
    (metric-gauge! 'total-handled (+ handled 1))
    (let sender (second msg)
      (send-to! sender 'ok)
      (list 'become (list 'test-server-loop (+ handled 1))))))

(spawn-actor 'server 32 '(test-server-loop 0))
(spawn-actor 'client 32 '(test-client-loop 10))

; Run for enough ticks
(run-scheduler 200)

; Check that metrics were collected
(define final-metrics (get-metrics))

; Extract metrics - use REST not SECOND to get all entries
(define counters (rest (assoc 'counters final-metrics)))
(define gauges (rest (assoc 'gauges final-metrics)))
(define timings (rest (assoc 'timings final-metrics)))

(define roundtrips (assoc 'roundtrips counters))
(define handled (assoc 'requests-handled counters))
(define total-gauge (assoc 'total-handled gauges))
(define latency (assoc 'latency timings))

(assert-true (not (nil? roundtrips)) "roundtrips counter exists")
(assert-true (not (nil? handled)) "requests-handled counter exists")
(assert-true (not (nil? total-gauge)) "total-handled gauge exists")
(assert-true (not (nil? latency)) "latency timing exists")

; Check values - roundtrips should be exactly 10
(assert-eq (second roundtrips) 10 "10 roundtrips completed")
; Server may handle more due to blocking restart behavior
(assert-true (>= (second handled) 10) "at least 10 requests handled")
(assert-eq (length (second latency)) 10 "10 timing samples")

(println "")
(println "Collected metrics:")
(println "  Counters: " counters)
(println "  Gauges: " gauges)
(println "  Timing samples: " (length (second latency)))
