; ============================================================================
; BoundedLISP Prologue - Actor-First Design (Consolidated)
; Actors are the source of truth. Diagrams and properties are derived.
; ============================================================================

; ============================================================================
; PART 1: Basic Helpers
; ============================================================================

(define (second lst) (nth lst 1))
(define (third lst) (nth lst 2))
(define (fourth lst) (nth lst 3))
(define (fifth lst) (nth lst 4))
(define (sixth lst) (nth lst 5))
(define (seventh lst) (nth lst 6))

(define (drop lst n)
  (if (or (empty? lst) (<= n 0))
      lst
      (drop (rest lst) (- n 1))))

(define (take lst n)
  (if (or (empty? lst) (<= n 0))
      '()
      (cons (first lst) (take (rest lst) (- n 1)))))

(define (any? pred lst)
  (if (empty? lst)
      false
      (if (pred (first lst))
          true
          (any? pred (rest lst)))))

(define (all? pred lst)
  (if (empty? lst)
      true
      (if (pred (first lst))
          (all? pred (rest lst))
          false)))

(define (map func lst)
  (if (empty? lst)
      '()
      (cons (func (first lst)) (map func (rest lst)))))

(define (filter pred lst)
  (if (empty? lst)
      '()
      (if (pred (first lst))
          (cons (first lst) (filter pred (rest lst)))
          (filter pred (rest lst)))))

(define (reverse lst)
  (reverse-helper lst '()))

(define (reverse-helper lst acc)
  (if (empty? lst)
      acc
      (reverse-helper (rest lst) (cons (first lst) acc))))

(define (append a b)
  (if (empty? a)
      b
      (cons (first a) (append (rest a) b))))

(define (member? item lst)
  (if (empty? lst)
      false
      (if (= item (first lst))
          true
          (member? item (rest lst)))))

(define (assoc key lst)
  (if (empty? lst)
      nil
      (if (= (first (first lst)) key)
          (first lst)
          (assoc key (rest lst)))))

(define (unique lst)
  (unique-helper lst '()))

(define (unique-helper lst acc)
  (if (empty? lst)
      acc
      (if (member? (first lst) acc)
          (unique-helper (rest lst) acc)
          (unique-helper (rest lst) (cons (first lst) acc)))))

(define (equal? a b)
  (if (and (list? a) (list? b))
      (if (= (length a) (length b))
          (all-equal? a b)
          false)
      (= a b)))

(define (all-equal? a b)
  (if (empty? a)
      true
      (if (equal? (first a) (first b))
          (all-equal? (rest a) (rest b))
          false)))

(define (flatten lst)
  (if (empty? lst)
      '()
      (if (list? (first lst))
          (append (flatten (first lst)) (flatten (rest lst)))
          (cons (first lst) (flatten (rest lst))))))

; ============================================================================
; PART 2: Global State
; ============================================================================

(define *actors* '())           ; Actor templates
(define *instances* '())        ; Running actor instances (name . type)
(define *traces* '())           ; Message traces: (tick from to msg-type)
(define *tick* 0)               ; Simulation clock
(define *properties* '())       ; Named CTL properties

; ============================================================================
; PART 3: Actor Definition & Spawning
; ============================================================================

; Actors use the "become" pattern for state:
;
; (define (my-actor-loop state)
;   (let msg (receive!)
;     ... process ...
;     (if continue?
;         (list 'become (list 'my-actor-loop new-state))
;         'done)))
;
; (spawn-actor 'name 16 '(my-actor-loop initial))

; (defactor 'name :body expr) - register actor template
(define (defactor name . args)
  (let spec (parse-actor-spec args)
    (let actor (list 'actor name spec)
      (do
        (set! *actors* (cons actor *actors*))
        actor))))

(define (parse-actor-spec args)
  (parse-actor-spec-loop args '()))

(define (parse-actor-spec-loop args acc)
  (if (empty? args)
      acc
      (let key (first args)
        (let val (second args)
          (parse-actor-spec-loop 
            (rest (rest args))
            (cons (list key val) acc))))))

(define (actor-spec-get spec key default)
  (let entry (assoc key spec)
    (if (nil? entry)
        default
        (second entry))))

(define (get-actor name)
  (find-actor name *actors*))

(define (find-actor name actors)
  (if (empty? actors)
      nil
      (let a (first actors)
        (if (= (second a) name)
            a
            (find-actor name (rest actors))))))

; (spawn 'template-name 'instance-name [mailbox-size])
(define (spawn actor-name instance-name . args)
  (let template (get-actor actor-name)
    (if (nil? template)
        (do (println "Error: unknown actor" actor-name) nil)
        (let spec (third template)
          (let mailbox-size (if (empty? args) 16 (first args))
            (let body (actor-spec-get spec ':body '(done))
              (do
                (spawn-actor instance-name mailbox-size body)
                (set! *instances* (cons (list instance-name actor-name) *instances*))
                instance-name)))))))

; ============================================================================
; PART 4: Message Tracing
; ============================================================================

; Traced send wrapper
(define (send! target msg-type . payload)
  (let from (self)
    (do
      (set! *traces* (cons (list *tick* from target msg-type) *traces*))
      (send-to! target (cons msg-type payload)))))

(define (clear-traces!)
  (set! *traces* '()))

(define (get-traces)
  (reverse *traces*))

(define (trace-message-types)
  (unique (map fourth *traces*)))

(define (trace-actor-pairs)
  (unique (map (lambda (t) (list (second t) (third t))) *traces*)))

(define (trace-actors)
  (unique (append (map second *traces*) (map third *traces*))))

; ============================================================================
; PART 5: Kripke Structure from Traces
; ============================================================================

; A Kripke structure: (kripke name states initial transitions labels)
; Built from message traces rather than grammars.

; Build Kripke from collected traces
; States are unique (from, msg-type) pairs representing "actor X just sent/received Y"
(define (traces->kripke name)
  (let traces (get-traces)
    (if (empty? traces)
        nil
        (let states (extract-trace-states traces)
          (let initial (list (first states))
            (let trans (extract-trace-transitions traces)
              (let labels (make-trace-labels states)
                (tag 'kripke (list name states initial trans labels)))))))))

(define (extract-trace-states traces)
  (unique (map (lambda (t) 
                 (list (second t) (fourth t)))  ; (actor msg-type)
               traces)))

(define (extract-trace-transitions traces)
  (extract-trans-pairs traces))

(define (extract-trans-pairs traces)
  (if (or (empty? traces) (empty? (rest traces)))
      '()
      (let t1 (first traces)
        (let t2 (second traces)
          (cons (list (list (second t1) (fourth t1))   ; from state
                      (list (second t2) (fourth t2))   ; to state  
                      (fourth t2))                      ; label
                (extract-trans-pairs (rest traces)))))))

(define (make-trace-labels states)
  (map (lambda (s)
         (list s (list (list 'actor (first s))
                       (list 'msg (second s)))))
       states))

; Kripke accessors
(define (kripke? k)
  (tag-is? k 'kripke))

(define (kripke-name k)
  (first (tag-value k)))

(define (kripke-states k)
  (second (tag-value k)))

(define (kripke-initial k)
  (third (tag-value k)))

(define (kripke-transitions k)
  (fourth (tag-value k)))

(define (kripke-labels k)
  (fifth (tag-value k)))

(define (kripke-successors k state)
  (let trans (kripke-transitions k)
    (map second (filter (lambda (t) (equal? (first t) state)) trans))))

(define (kripke-props-at k state)
  (let labels (kripke-labels k)
    (let entry (assoc state labels)
      (if (nil? entry)
          '()
          (second entry)))))

; ============================================================================
; PART 6: CTL Formula Construction
; ============================================================================

(define (prop name) 
  (tag 'ctl-prop name))

(define (ctl-and . args)
  (tag 'ctl-and args))

(define (ctl-or . args)
  (tag 'ctl-or args))

(define (ctl-not formula)
  (tag 'ctl-not formula))

(define (ctl-implies p q)
  (tag 'ctl-implies (list p q)))

(define (AX formula) (tag 'ctl-AX formula))
(define (AF formula) (tag 'ctl-AF formula))
(define (AG formula) (tag 'ctl-AG formula))
(define (AU p q) (tag 'ctl-AU (list p q)))
(define (EX formula) (tag 'ctl-EX formula))
(define (EF formula) (tag 'ctl-EF formula))
(define (EG formula) (tag 'ctl-EG formula))
(define (EU p q) (tag 'ctl-EU (list p q)))

; ============================================================================
; PART 7: CTL Model Checking
; ============================================================================

(define (holds-at? k state prop)
  (let props (kripke-props-at k state)
    (any? (lambda (p) (equal? p prop)) props)))

; Evaluate CTL formula at state
(define (ctl-eval k state formula)
  (if (not (tag-is? formula 'ctl-prop))
      (let formula-type (tag-type formula)
        (cond
          ((= formula-type 'ctl-prop)
           (holds-at? k state (tag-value formula)))
          
          ((= formula-type 'ctl-not)
           (not (ctl-eval k state (tag-value formula))))
          
          ((= formula-type 'ctl-and)
           (all? (lambda (f) (ctl-eval k state f)) (tag-value formula)))
          
          ((= formula-type 'ctl-or)
           (any? (lambda (f) (ctl-eval k state f)) (tag-value formula)))
          
          ((= formula-type 'ctl-implies)
           (let args (tag-value formula)
             (or (not (ctl-eval k state (first args)))
                 (ctl-eval k state (second args)))))
          
          ((= formula-type 'ctl-EX)
           (any? (lambda (s) (ctl-eval k s (tag-value formula))) 
                 (kripke-successors k state)))
          
          ((= formula-type 'ctl-AX)
           (let succs (kripke-successors k state)
             (if (empty? succs)
                 true
                 (all? (lambda (s) (ctl-eval k s (tag-value formula))) succs))))
          
          ((= formula-type 'ctl-EF)
           (ctl-EF-eval k state (tag-value formula) '()))
          
          ((= formula-type 'ctl-AF)
           (ctl-AF-eval k state (tag-value formula) '()))
          
          ((= formula-type 'ctl-EG)
           (ctl-EG-eval k state (tag-value formula) '()))
          
          ((= formula-type 'ctl-AG)
           (ctl-AG-eval k state (tag-value formula) '()))
          
          (true
           (if (symbol? formula)
               (holds-at? k state formula)
               false))))
      ; Direct prop check
      (holds-at? k state (tag-value formula))))

; EF φ - exists path where φ eventually holds
(define (ctl-EF-eval k state formula visited)
  (if (member? state visited)
      false
      (if (ctl-eval k state formula)
          true
          (any? (lambda (s) (ctl-EF-eval k s formula (cons state visited)))
                (kripke-successors k state)))))

; AF φ - on all paths, φ eventually holds
(define (ctl-AF-eval k state formula visited)
  (if (member? state visited)
      false
      (if (ctl-eval k state formula)
          true
          (let succs (kripke-successors k state)
            (if (empty? succs)
                false
                (all? (lambda (s) (ctl-AF-eval k s formula (cons state visited))) 
                      succs))))))

; EG φ - exists path where φ always holds
(define (ctl-EG-eval k state formula visited)
  (if (not (ctl-eval k state formula))
      false
      (if (member? state visited)
          true
          (let succs (kripke-successors k state)
            (if (empty? succs)
                true
                (any? (lambda (s) (ctl-EG-eval k s formula (cons state visited)))
                      succs))))))

; AG φ - on all paths, φ always holds
(define (ctl-AG-eval k state formula visited)
  (if (not (ctl-eval k state formula))
      false
      (if (member? state visited)
          true
          (let succs (kripke-successors k state)
            (if (empty? succs)
                true
                (all? (lambda (s) (ctl-AG-eval k s formula (cons state visited)))
                      succs))))))

; ============================================================================
; PART 8: Property Definition & Checking
; ============================================================================

(define (defproperty name formula)
  (let prop-entry (list name formula)
    (do
      (set! *properties* (cons prop-entry *properties*))
      prop-entry)))

(define (get-property name)
  (assoc name *properties*))

; Check property on traces
(define (check-traces property-name)
  (let k (traces->kripke 'trace-model)
    (if (nil? k)
        (do (println "No traces collected") nil)
        (let prop-data (get-property property-name)
          (if (nil? prop-data)
              (do (println "Property not found:" property-name) nil)
              (let formula (second prop-data)
                (let initial (first (kripke-initial k))
                  (let result (ctl-eval k initial formula)
                    (do
                      (println "Property:" property-name)
                      (println "Result:" (if result "SATISFIED" "VIOLATED"))
                      result)))))))))

; ============================================================================
; PART 9: Visualization (Mermaid from Traces)
; ============================================================================

; Sequence diagram
(define (traces->sequence)
  (let traces (get-traces)
    (if (empty? traces)
        ""
        (let actors (trace-actors)
          (string-append
            "sequenceDiagram\n"
            (sequence-participants actors)
            (sequence-messages traces))))))

(define (sequence-participants actors)
  (if (empty? actors)
      ""
      (string-append
        "    participant " (symbol->string (first actors)) "\n"
        (sequence-participants (rest actors)))))

(define (sequence-messages traces)
  (if (empty? traces)
      ""
      (let t (first traces)
        (string-append
          "    " (symbol->string (second t))
          " ->> " (symbol->string (third t))
          " : " (symbol->string (fourth t)) "\n"
          (sequence-messages (rest traces))))))

; Flow diagram (message patterns)
(define (traces->flow)
  (let traces (get-traces)
    (if (empty? traces)
        ""
        (let pairs (trace-actor-pairs)
          (string-append
            "flowchart LR\n"
            (flow-actor-nodes (trace-actors))
            (flow-edges traces))))))

(define (flow-actor-nodes actors)
  (if (empty? actors)
      ""
      (string-append
        "    " (symbol->string (first actors)) 
        "[" (symbol->string (first actors)) "]\n"
        (flow-actor-nodes (rest actors)))))

(define (flow-edges traces)
  (let unique-flows (unique (map (lambda (t) 
                                   (list (second t) (third t) (fourth t))) 
                                 traces))
    (flow-edge-lines unique-flows)))

(define (flow-edge-lines flows)
  (if (empty? flows)
      ""
      (let f (first flows)
        (string-append
          "    " (symbol->string (first f))
          " -->|" (symbol->string (third f)) "| "
          (symbol->string (second f)) "\n"
          (flow-edge-lines (rest flows))))))

; State diagram from Kripke
(define (kripke->mermaid k)
  (if (not (kripke? k))
      "Error: not a Kripke structure"
      (string-append
        "stateDiagram-v2\n"
        (mermaid-initial (kripke-initial k))
        (mermaid-transitions (kripke-transitions k))
        (mermaid-terminals (kripke-states k) (kripke-transitions k)))))

(define (mermaid-initial initial)
  (if (empty? initial)
      ""
      (string-append "    [*] --> " (state->string (first initial)) "\n")))

(define (mermaid-transitions trans)
  (if (empty? trans)
      ""
      (let t (first trans)
        (string-append
          "    " (state->string (first t))
          " --> " (state->string (second t))
          " : " (symbol->string (third t)) "\n"
          (mermaid-transitions (rest trans))))))

(define (mermaid-terminals states transitions)
  (if (empty? states)
      ""
      (let s (first states)
        (let outgoing (filter (lambda (t) (equal? (first t) s)) transitions)
          (string-append
            (if (empty? outgoing)
                (string-append "    " (state->string s) " --> [*]\n")
                "")
            (mermaid-terminals (rest states) transitions))))))

(define (state->string s)
  (if (list? s)
      (string-append (symbol->string (first s)) "_" (symbol->string (second s)))
      (symbol->string s)))

; ============================================================================
; PART 10: Simulation Helpers
; ============================================================================

(define (reset-simulation!)
  (do
    (set! *instances* '())
    (set! *traces* '())
    (set! *tick* 0)))

(define (advance-tick!)
  (set! *tick* (+ *tick* 1)))

(define (current-tick)
  *tick*)

; ============================================================================
; PART 11: Metrics Actor
; ============================================================================

; Global metrics storage (for queries outside actor system)
(define *metrics* '())

; Metrics actor loop - receives metric events, accumulates statistics
; Messages:
;   (counter name delta)      - increment counter by delta
;   (gauge name value)        - set gauge to value  
;   (timing name duration)    - record a timing
;   (event name)              - increment event count
;   (query sender)            - send current metrics to sender
;   (reset)                   - clear all metrics
;
; State: ((counters . alist) (gauges . alist) (timings . alist) (events . alist))

(define (metrics-loop state)
  (let msg (receive!)
    (let cmd (first msg)
      (let new-state
        (cond
          ((= cmd 'counter)
           (let name (second msg)
             (let delta (third msg)
               (update-counter state name delta))))
          
          ((= cmd 'gauge)
           (let name (second msg)
             (let value (third msg)
               (update-gauge state name value))))
          
          ((= cmd 'timing)
           (let name (second msg)
             (let duration (third msg)
               (add-timing state name duration))))
          
          ((= cmd 'event)
           (let name (second msg)
             (update-counter state name 1)))
          
          ((= cmd 'query)
           (let sender (second msg)
             (do
               (send-to! sender (list 'metrics (compute-stats state)))
               state)))
          
          ((= cmd 'reset)
           (make-empty-metrics-state))
          
          (true state))
        (do
          (set! *metrics* new-state)
          (list 'become (list 'metrics-loop (list 'quote new-state))))))))

(define (make-empty-metrics-state)
  (list (list 'counters) (list 'gauges) (list 'timings) (list 'events)))

(define (metrics-get state key)
  (let entry (assoc key state)
    (if (nil? entry)
        '()
        (rest entry))))

(define (metrics-set state key value)
  (let without (filter (lambda (e) (not (= (first e) key))) state)
    (cons (cons key value) without)))

(define (update-counter state name delta)
  (let counters (metrics-get state 'counters)
    (let current (assoc name counters)
      (let new-val (+ (if (nil? current) 0 (second current)) delta)
        (let new-counters (cons (list name new-val) 
                                (filter (lambda (e) (not (= (first e) name))) counters))
          (metrics-set state 'counters new-counters))))))

(define (update-gauge state name value)
  (let gauges (metrics-get state 'gauges)
    (let new-gauges (cons (list name value)
                          (filter (lambda (e) (not (= (first e) name))) gauges))
      (metrics-set state 'gauges new-gauges))))

(define (add-timing state name duration)
  (let timings (metrics-get state 'timings)
    (let current (assoc name timings)
      (let samples (if (nil? current) '() (second current))
        (let new-samples (cons duration samples)
          (let new-timings (cons (list name new-samples)
                                 (filter (lambda (e) (not (= (first e) name))) timings))
            (metrics-set state 'timings new-timings)))))))

(define (compute-stats state)
  (list
    (list 'counters (metrics-get state 'counters))
    (list 'gauges (metrics-get state 'gauges))
    (list 'timings (map (lambda (t)
                          (list (first t) (timing-stats (second t))))
                        (metrics-get state 'timings)))))

(define (timing-stats samples)
  (if (empty? samples)
      '()
      (let n (length samples)
        (let total (sum-list samples)
          (let mean (/ total n)
            (let sorted (sort-numbers samples)
              (list (list 'count n)
                    (list 'mean mean)
                    (list 'min (first sorted))
                    (list 'max (first (reverse sorted)))
                    (list 'p50 (percentile sorted 50))
                    (list 'p99 (percentile sorted 99)))))))))

(define (sum-list lst)
  (if (empty? lst)
      0
      (+ (first lst) (sum-list (rest lst)))))

(define (sort-numbers lst)
  (if (empty? lst)
      '()
      (let pivot (first lst)
        (let rest-lst (rest lst)
          (let smaller (filter (lambda (x) (< x pivot)) rest-lst)
            (let larger (filter (lambda (x) (>= x pivot)) rest-lst)
              (append (sort-numbers smaller) 
                      (cons pivot (sort-numbers larger)))))))))

(define (percentile sorted p)
  (if (empty? sorted)
      0
      (let n (length sorted)
        (let idx (floor (* (/ p 100) n))
          (let safe-idx (if (>= idx n) (- n 1) idx)
            (nth sorted safe-idx))))))

; Convenience functions for sending metrics
(define (metric-counter! name delta)
  (send-to! 'metrics (list 'counter name delta)))

(define (metric-inc! name)
  (send-to! 'metrics (list 'counter name 1)))

(define (metric-gauge! name value)
  (send-to! 'metrics (list 'gauge name value)))

(define (metric-timing! name duration)
  (send-to! 'metrics (list 'timing name duration)))

(define (metric-event! name)
  (send-to! 'metrics (list 'event name)))

; Spawn the metrics actor
(define (start-metrics!)
  (spawn-actor 'metrics 256 (list 'metrics-loop (list 'quote (make-empty-metrics-state)))))

; Get current metrics (from global, not via message)
(define (get-metrics)
  *metrics*)

; ============================================================================
(println "BoundedLISP Prologue loaded.")
; ============================================================================
; Probability Distribution Transformations
; Transform uniform [0,1] pre-rolls into various distributions
; ============================================================================

; ----------------------------------------------------------------------------
; Core Distributions
; ----------------------------------------------------------------------------

; Exponential distribution: models inter-arrival times, service times
; Given uniform U in (0,1], returns exponential with rate λ
; Mean = 1/rate
(define (exponential u rate)
  (/ (- 0 (ln u)) rate))

; Alias for clarity
(define (exponential-from-uniform u rate)
  (exponential u rate))

; Poisson process: given a rate λ and uniform u, get next arrival time
; This is equivalent to exponential(λ)
(define (poisson-arrival u rate)
  (exponential u rate))

; Geometric distribution: number of failures before first success
; p = probability of success on each trial
; Returns count (0, 1, 2, ...)
(define (geometric u p)
  (floor (/ (ln u) (ln (- 1 p)))))

; Bernoulli trial: success (1) or failure (0) with probability p
(define (bernoulli u p)
  (if (< u p) 1 0))

; Uniform in range [a, b]
(define (uniform-range u a b)
  (+ a (* u (- b a))))

; ----------------------------------------------------------------------------
; Normal/Gaussian Distribution (Box-Muller transform)
; Requires TWO uniform values
; ----------------------------------------------------------------------------

; Standard normal (mean=0, stddev=1)
; Uses Box-Muller transform: given U1, U2 uniform
; Z0 = sqrt(-2*ln(U1)) * cos(2*π*U2)
; Z1 = sqrt(-2*ln(U1)) * sin(2*π*U2)
(define *pi* 3.141592653589793)

(define (normal-z0 u1 u2)
  (* (sqrt (* -2 (ln u1))) 
     (cos (* 2 *pi* u2))))

(define (normal-z1 u1 u2)
  (* (sqrt (* -2 (ln u1))) 
     (sin (* 2 *pi* u2))))

; General normal with mean μ and stddev σ
(define (normal u1 u2 mean stddev)
  (+ mean (* stddev (normal-z0 u1 u2))))

; ----------------------------------------------------------------------------
; Discrete Distributions
; ----------------------------------------------------------------------------

; Discrete uniform: pick integer in [min, max] inclusive
(define (discrete-uniform u min-val max-val)
  (+ min-val (floor (* u (+ 1 (- max-val min-val))))))

; Categorical/multinomial: pick from weighted options
; weights is a list of (weight . value) pairs
; Returns the value whose cumulative weight bracket contains u
(define (categorical u weights)
  (let total (sum-weights weights)
    (categorical-pick u weights 0 total)))

(define (sum-weights weights)
  (if (empty? weights)
      0
      (+ (first (first weights)) (sum-weights (rest weights)))))

(define (categorical-pick u weights cumulative total)
  (if (empty? weights)
      nil
      (let w (first weights)
        (let new-cumulative (+ cumulative (/ (first w) total))
          (if (< u new-cumulative)
              (second w)
              (categorical-pick u (rest weights) new-cumulative total))))))

; Weighted coin flip helper
(define (weighted-choice u p val-true val-false)
  (if (< u p) val-true val-false))

; ----------------------------------------------------------------------------
; Queueing Theory Helpers
; ----------------------------------------------------------------------------

; M/M/1 queue parameters
; λ = arrival rate (arrivals per time unit)
; μ = service rate (services per time unit)  
; ρ = λ/μ = utilization (must be < 1 for stability)

; Generate next inter-arrival time
(define (mm1-arrival-time u lambda)
  (exponential u lambda))

; Generate next service time  
(define (mm1-service-time u mu)
  (exponential u mu))

; Theoretical M/M/1 metrics for comparison
(define (mm1-utilization lambda mu)
  (/ lambda mu))

(define (mm1-avg-queue-length lambda mu)
  (let rho (/ lambda mu)
    (/ (* rho rho) (- 1 rho))))

(define (mm1-avg-system-length lambda mu)
  (let rho (/ lambda mu)
    (/ rho (- 1 rho))))

(define (mm1-avg-wait-time lambda mu)
  (let rho (/ lambda mu)
    (/ rho (* mu (- 1 rho)))))

(define (mm1-avg-system-time lambda mu)
  (/ 1 (- mu lambda)))

; ----------------------------------------------------------------------------
; Dice Pool Management
; ----------------------------------------------------------------------------

; A dice pool is a bounded queue of pre-rolled uniform values
; Create with: (make-dice-pool size rolls)

(define (make-dice-pool size initial-rolls)
  (let pool (make-queue size)
    (do
      (fill-dice-pool pool initial-rolls)
      pool)))

(define (fill-dice-pool pool rolls)
  (if (empty? rolls)
      pool
      (do
        (enqueue-now! pool (first rolls))
        (fill-dice-pool pool (rest rolls)))))

; Get next die from pool (non-blocking, returns 'empty if exhausted)
(define (next-die! pool)
  (dequeue-now! pool))

; Check if pool has dice remaining
(define (dice-remaining? pool)
  (not (queue-empty? pool)))
