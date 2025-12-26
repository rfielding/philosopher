; ============================================================================
; BoundedLISP Prologue - Standard Library
; ============================================================================

; ----------------------------------------------------------------------------
; List Accessors
; ----------------------------------------------------------------------------

(define (second lst) (nth lst 1))
(define (third lst) (nth lst 2))
(define (fourth lst) (nth lst 3))
(define (fifth lst) (nth lst 4))

; ----------------------------------------------------------------------------
; Higher-Order Functions
; ----------------------------------------------------------------------------

(define (map f lst)
  (if (empty? lst)
      '()
      (cons (f (first lst)) (map f (rest lst)))))

(define (filter pred lst)
  (if (empty? lst)
      '()
      (if (pred (first lst))
          (cons (first lst) (filter pred (rest lst)))
          (filter pred (rest lst)))))

(define (fold f init lst)
  (if (empty? lst)
      init
      (fold f (f init (first lst)) (rest lst))))

(define (reduce f lst)
  (if (empty? lst)
      nil
      (fold f (first lst) (rest lst))))

(define (for-each f lst)
  (if (empty? lst)
      nil
      (do (f (first lst))
          (for-each f (rest lst)))))

; ----------------------------------------------------------------------------
; List Utilities
; ----------------------------------------------------------------------------

; Helper for reverse - avoids self-reference issue in let-lambda
(define (reverse-acc in out)
  (if (empty? in)
      out
      (reverse-acc (rest in) (cons (first in) out))))

(define (reverse lst)
  (reverse-acc lst '()))

(define (member? x lst)
  (if (empty? lst)
      false
      (if (= x (first lst))
          true
          (member? x (rest lst)))))

(define (unique lst)
  (if (empty? lst)
      '()
      (let x (first lst)
        (if (member? x (rest lst))
            (unique (rest lst))
            (cons x (unique (rest lst)))))))

(define (assoc key lst)
  (if (empty? lst)
      nil
      (if (= key (first (first lst)))
          (first lst)
          (assoc key (rest lst)))))

(define (assoc-set key val lst)
  (if (empty? lst)
      (list (list key val))
      (if (= key (first (first lst)))
          (cons (list key val) (rest lst))
          (cons (first lst) (assoc-set key val (rest lst))))))

(define (range start end)
  (if (>= start end)
      '()
      (cons start (range (+ start 1) end))))

(define (take n lst)
  (if (or (<= n 0) (empty? lst))
      '()
      (cons (first lst) (take (- n 1) (rest lst)))))

(define (drop n lst)
  (if (or (<= n 0) (empty? lst))
      lst
      (drop (- n 1) (rest lst))))

(define (last lst)
  (if (empty? lst)
      nil
      (if (empty? (rest lst))
          (first lst)
          (last (rest lst)))))

(define (flatten lst)
  (if (empty? lst)
      '()
      (if (list? (first lst))
          (append (flatten (first lst)) (flatten (rest lst)))
          (cons (first lst) (flatten (rest lst))))))

; ----------------------------------------------------------------------------
; Sorting
; ----------------------------------------------------------------------------

(define (insert-sorted x lst)
  (if (empty? lst)
      (list x)
      (if (<= x (first lst))
          (cons x lst)
          (cons (first lst) (insert-sorted x (rest lst))))))

(define (sort-numbers lst)
  (if (empty? lst)
      '()
      (insert-sorted (first lst) (sort-numbers (rest lst)))))

(define (sum-list lst)
  (if (empty? lst)
      0
      (+ (first lst) (sum-list (rest lst)))))

; ----------------------------------------------------------------------------
; Distribution Functions (for simulation)
; ----------------------------------------------------------------------------

; Exponential distribution: given uniform U in [0,1] and rate, return sample
; Formula: -ln(U) / rate
(define (exponential u rate)
  (/ (- 0 (ln u)) rate))

; Discrete uniform: given U in [0,1], return integer in [low, high]
(define (discrete-uniform u low high)
  (+ low (floor (* u (+ 1 (- high low))))))

; Bernoulli: given U in [0,1] and probability p, return 1 with prob p, else 0
(define (bernoulli u p)
  (if (< u p) 1 0))

; Uniform in range [low, high]
(define (uniform-range u low high)
  (+ low (* u (- high low))))

; M/M/1 queue theory
(define (mm1-utilization lambda mu)
  (/ lambda mu))

(define (mm1-avg-system-length lambda mu)
  (let rho (/ lambda mu)
    (/ rho (- 1 rho))))

(define (mm1-avg-wait lambda mu)
  (/ 1 (- mu lambda)))

; ----------------------------------------------------------------------------
; CTL Formula Constructors
; ----------------------------------------------------------------------------

(define (prop name)
  (tag 'ctl-prop name))

(define (EX phi)
  (tag 'ctl-EX phi))

(define (AX phi)
  (tag 'ctl-AX phi))

(define (EF phi)
  (tag 'ctl-EF phi))

(define (AF phi)
  (tag 'ctl-AF phi))

(define (EG phi)
  (tag 'ctl-EG phi))

(define (AG phi)
  (tag 'ctl-AG phi))

(define (EU phi psi)
  (tag 'ctl-EU (list phi psi)))

(define (AU phi psi)
  (tag 'ctl-AU (list phi psi)))

(define (ctl-and phi psi)
  (tag 'ctl-and (list phi psi)))

(define (ctl-or phi psi)
  (tag 'ctl-or (list phi psi)))

(define (ctl-not phi)
  (tag 'ctl-not phi))

(define (ctl-implies phi psi)
  (tag 'ctl-implies (list phi psi)))

; ----------------------------------------------------------------------------
; Trace Recording and Visualization
; ----------------------------------------------------------------------------

(define *traces* '())

(define (record-trace! tick from to msg)
  (set! *traces* (cons (list tick from to msg) *traces*)))

(define (clear-traces!)
  (set! *traces* '()))

(define (traces->sequence)
  (let sorted (reverse *traces*)
    (let lines (map (lambda (t)
                      (string-append "    "
                                     (symbol->string (second t))
                                     "->>"
                                     (symbol->string (third t))
                                     ": "
                                     (symbol->string (fourth t))))
                    sorted)
      (let header "sequenceDiagram\n"
        (fold (lambda (acc line)
                (string-append acc line "\n"))
              header
              lines)))))

(define (traces->flow)
  (let pairs (map (lambda (t) (list (second t) (third t))) *traces*)
    (let unique-pairs (unique pairs)
      (let lines (map (lambda (p)
                        (string-append "    "
                                       (symbol->string (first p))
                                       " --> "
                                       (symbol->string (second p))))
                      unique-pairs)
        (let header "graph LR\n"
          (fold (lambda (acc line)
                  (string-append acc line "\n"))
                header
                lines))))))

; ----------------------------------------------------------------------------
; Metrics System
; ----------------------------------------------------------------------------

; Metrics state structure: ((counters ...) (gauges ...) (timings ...))
(define (make-empty-metrics-state)
  '((counters) (gauges) (timings)))

(define (metrics-get state key)
  (let entry (assoc key state)
    (if (nil? entry)
        '()
        (rest entry))))

(define (metrics-set state key val)
  (if (empty? state)
      (list (cons key val))
      (if (= key (first (first state)))
          (cons (cons key val) (rest state))
          (cons (first state) (metrics-set (rest state) key val)))))

(define (update-counter state name delta)
  (let counters (metrics-get state 'counters)
    (let current (assoc name counters)
      (let new-val (if (nil? current) delta (+ (second current) delta))
        (let new-counters (assoc-set name new-val counters)
          (metrics-set state 'counters new-counters))))))

(define (update-gauge state name value)
  (let gauges (metrics-get state 'gauges)
    (let new-gauges (assoc-set name value gauges)
      (metrics-set state 'gauges new-gauges))))

(define (add-timing state name value)
  (let timings (metrics-get state 'timings)
    (let current (assoc name timings)
      (let samples (if (nil? current) '() (second current))
        (let new-samples (append samples (list value))
          (let new-timings (assoc-set name new-samples timings)
            (metrics-set state 'timings new-timings)))))))

; Global metrics registry key
(define *metrics-key* 'global-metrics)

; Store metrics in registry for simplicity
(define (init-metrics!)
  (registry-set! *metrics-key* (make-empty-metrics-state)))

; Metrics collector actor - stores state in registry
(define (metrics-collector-loop)
  (let msg (receive!)
    (let cmd (first msg)
      (let state (registry-get *metrics-key*)
        (cond
          ((= cmd 'inc)
           (let name (second msg)
             (let delta (if (> (length msg) 2) (third msg) 1)
               (do
                 (registry-set! *metrics-key* (update-counter state name delta))
                 (list 'become '(metrics-collector-loop))))))
          ((= cmd 'gauge)
           (let name (second msg)
             (let value (third msg)
               (do
                 (registry-set! *metrics-key* (update-gauge state name value))
                 (list 'become '(metrics-collector-loop))))))
          ((= cmd 'timing)
           (let name (second msg)
             (let value (third msg)
               (do
                 (registry-set! *metrics-key* (add-timing state name value))
                 (list 'become '(metrics-collector-loop))))))
          ((= cmd 'get)
           (let sender (second msg)
             (do
               (send-to! sender (list 'metrics state))
               (list 'become '(metrics-collector-loop)))))
          (true
           (list 'become '(metrics-collector-loop))))))))

(define (start-metrics!)
  (init-metrics!)
  (spawn-actor 'metrics-collector 64 '(metrics-collector-loop)))

; Helper functions to send metrics (can be called from any actor)
(define (metric-inc! name)
  (send-to! 'metrics-collector (list 'inc name 1)))

(define (metric-add! name delta)
  (send-to! 'metrics-collector (list 'inc name delta)))

(define (metric-gauge! name value)
  (send-to! 'metrics-collector (list 'gauge name value)))

(define (metric-timing! name value)
  (send-to! 'metrics-collector (list 'timing name value)))

; Synchronous get-metrics - just reads from registry
(define (get-metrics)
  (registry-get *metrics-key*))

; ----------------------------------------------------------------------------
; Property Storage
; ----------------------------------------------------------------------------

(define *properties* '())

(define (defproperty name formula)
  (set! *properties* (cons (list name formula) *properties*))
  name)

(define (get-property name)
  (let entry (assoc name *properties*)
    (if (nil? entry)
        nil
        (second entry))))

(define (list-properties)
  (map first *properties*))

; ----------------------------------------------------------------------------
; Grammar Storage (placeholder for DSL)
; ----------------------------------------------------------------------------

(define *grammars* '())

(define (defgrammar name . rules)
  (set! *grammars* (cons (list name rules) *grammars*))
  name)

(define (get-grammar name)
  (let entry (assoc name *grammars*)
    (if (nil? entry)
        nil
        (second entry))))

; Placeholder diagram generators
(define (grammar->state-diagram name)
  "stateDiagram-v2\n    [*] --> Start\n")

(define (grammar->sequence name)
  "sequenceDiagram\n    participant A\n")

(define (grammar->flowchart name)
  "graph LR\n    A --> B\n")

; ----------------------------------------------------------------------------
; Utility Macros (via functions)
; ----------------------------------------------------------------------------

(define (when cond . body)
  (if cond
      (eval (cons 'do body))
      nil))

(define (unless cond . body)
  (if cond
      nil
      (eval (cons 'do body))))

(println "; Prologue loaded")
