; ============================================================================
; BoundedLISP Prologue - Runtime Library
; ============================================================================

; ============================================================================
; List Helpers
; ============================================================================

(define (second lst) (nth lst 1))
(define (third lst) (nth lst 2))
(define (fourth lst) (nth lst 3))
(define (fifth lst) (nth lst 4))

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

(define (reverse lst)
  (fold (lambda (acc x) (cons x acc)) '() lst))

(define (member? x lst)
  (if (empty? lst)
      false
      (if (= x (first lst))
          true
          (member? x (rest lst)))))

(define (unique lst)
  (fold (lambda (acc x)
          (if (member? x acc) acc (append acc (list x))))
        '() lst))

(define (assoc key alist)
  (if (empty? alist)
      nil
      (if (= key (first (first alist)))
          (first alist)
          (assoc key (rest alist)))))

(define (range start end)
  (if (>= start end)
      '()
      (cons start (range (+ start 1) end))))

(define (zip a b)
  (if (or (empty? a) (empty? b))
      '()
      (cons (list (first a) (first b))
            (zip (rest a) (rest b)))))

(define (flatten lst)
  (if (empty? lst)
      '()
      (if (list? (first lst))
          (append (flatten (first lst)) (flatten (rest lst)))
          (cons (first lst) (flatten (rest lst))))))

(define (take n lst)
  (if (or (<= n 0) (empty? lst))
      '()
      (cons (first lst) (take (- n 1) (rest lst)))))

(define (drop n lst)
  (if (or (<= n 0) (empty? lst))
      lst
      (drop (- n 1) (rest lst))))

(define (sum-list lst)
  (fold (lambda (acc x) (+ acc x)) 0 lst))

(define (sort-numbers lst)
  (if (empty? lst)
      '()
      (let pivot (first lst)
        (let smaller (filter (lambda (x) (< x pivot)) (rest lst))
          (let larger (filter (lambda (x) (>= x pivot)) (rest lst))
            (append (sort-numbers smaller) 
                    (list pivot) 
                    (sort-numbers larger)))))))

; ============================================================================
; Actor Definition DSL
; ============================================================================

; Global registry for actor definitions (not instances)
(define *actor-defs* '())
(define *channel-defs* '())
(define *state-defs* '())
(define *transition-defs* '())
(define *invariants* '())

(define (reset-defs!)
  (set! *actor-defs* '())
  (set! *channel-defs* '())
  (set! *state-defs* '())
  (set! *transition-defs* '())
  (set! *invariants* '()))

; (defactor 'name '((var init) ...) '((stack name size) ...))
(define (defactor name vars stacks)
  (let def (list 'actor name vars stacks)
    (set! *actor-defs* (cons def *actor-defs*))
    def))

; (defchannel 'name 'from-actor 'to-actor capacity)
(define (defchannel name from to capacity)
  (let def (list 'channel name from to capacity)
    (set! *channel-defs* (cons def *channel-defs*))
    def))

; (defstate 'actor 'state-name '(predicate expression))
(define (defstate actor name predicate)
  (let def (list 'state actor name predicate)
    (set! *state-defs* (cons def *state-defs*))
    def))

; (deftransition 'name spec)
; spec is a list: (from to guard action mutations)
; from/to are state names or predicates
; guard is additional condition
; action is (send ch msg) or (recv ch var) or (chance ...)
; mutations is list of (set! var expr)
(define (deftransition name spec)
  (let def (cons name spec)
    (set! *transition-defs* (cons def *transition-defs*))
    def))

; (definvariant 'name '(predicate))
(define (definvariant name predicate)
  (let def (list 'invariant name predicate)
    (set! *invariants* (cons def *invariants*))
    def))

; ============================================================================
; State Helpers
; ============================================================================

; Get all states for an actor
(define (actor-states actor-name)
  (filter (lambda (s) (= (second s) actor-name)) *state-defs*))

; Get all transitions for an actor (by checking 'from' state ownership)
(define (actor-transitions actor-name)
  (let actor-state-names (map third (actor-states actor-name))
    (filter (lambda (t)
              (let from-state (second t)
                (member? from-state actor-state-names)))
            *transition-defs*)))

; Get channel endpoints for an actor
(define (actor-channels actor-name)
  (filter (lambda (c)
            (or (= (third c) actor-name)   ; from
                (= (fourth c) actor-name))) ; to
          *channel-defs*))

; ============================================================================
; Diagram Extraction - State Diagrams
; ============================================================================

(define (escape-mermaid s)
  (if (string? s) s (symbol->string s)))

(define (state->mermaid-id state-name)
  (string-append "s_" (escape-mermaid state-name)))

(define (predicate->label pred)
  (if (list? pred)
      (repr pred)
      (escape-mermaid pred)))

; Generate state diagram for one actor
(define (actor->state-diagram actor-name)
  (let states (actor-states actor-name)
    (let transitions (actor-transitions actor-name)
      (let header (string-append "stateDiagram-v2\n    %% Actor: " 
                                  (escape-mermaid actor-name) "\n")
        (let state-lines (map (lambda (s)
                                (let id (state->mermaid-id (third s))
                                  (let label (predicate->label (fourth s))
                                    (string-append "    " id " : " 
                                                   (escape-mermaid (third s))
                                                   " [" label "]"))))
                              states)
          (let trans-lines (map (lambda (t)
                                  (let name (first t)
                                    (let from (state->mermaid-id (second t))
                                      (let to (state->mermaid-id (third t))
                                        (let guard (fourth t)
                                          (let action (fifth t)
                                            (string-append "    " from " --> " to 
                                                           " : " (escape-mermaid name))))))))
                                transitions)
            (string-append header
                           (fold (lambda (acc line) 
                                   (string-append acc line "\n"))
                                 "" state-lines)
                           (fold (lambda (acc line)
                                   (string-append acc line "\n"))
                                 "" trans-lines))))))))

; Generate state diagrams for all actors
(define (all-state-diagrams)
  (let actor-names (unique (map second *actor-defs*))
    (fold (lambda (acc name)
            (string-append acc "\n" (actor->state-diagram name)))
          "" actor-names)))

; ============================================================================
; Diagram Extraction - Sequence Diagrams
; ============================================================================

; Build sequence diagram from channel definitions and transitions
(define (channels->sequence-diagram)
  (let header "sequenceDiagram\n"
    (let participants (unique (append 
                               (map third *channel-defs*)   ; from actors
                               (map fourth *channel-defs*))) ; to actors
      (let participant-lines (map (lambda (p)
                                    (string-append "    participant " 
                                                   (escape-mermaid p)))
                                  participants)
        ; Extract message flows from transitions that have send/recv
        (let message-lines (filter-map 
                            (lambda (t)
                              (let action (fifth t)
                                (if (and (list? action) 
                                         (= (first action) 'send))
                                    (let ch-name (second action)
                                      (let ch (assoc ch-name 
                                                (map (lambda (c) 
                                                      (cons (second c) c))
                                                     *channel-defs*))
                                        (if ch
                                            (string-append 
                                              "    " (escape-mermaid (third (rest ch)))
                                              " ->> " (escape-mermaid (fourth (rest ch)))
                                              " : " (escape-mermaid (third action)))
                                            nil)))
                                    nil)))
                            *transition-defs*)
          (string-append header
                         (fold (lambda (acc line)
                                 (string-append acc line "\n"))
                               "" participant-lines)
                         (fold (lambda (acc line)
                                 (string-append acc line "\n"))
                               "" message-lines)))))))

; Helper: filter and map in one pass
(define (filter-map f lst)
  (if (empty? lst)
      '()
      (let result (f (first lst))
        (if (nil? result)
            (filter-map f (rest lst))
            (cons result (filter-map f (rest lst)))))))

; ============================================================================
; Trace-based Diagram Generation
; ============================================================================

; Global trace buffer
(define *traces* '())

(define (clear-traces!)
  (set! *traces* '()))

(define (add-trace! timestamp from to message)
  (set! *traces* (cons (list timestamp from to message) *traces*)))

; Convert execution traces to sequence diagram
(define (traces->sequence)
  (let sorted (reverse *traces*)  ; oldest first
    (let participants (unique (append (map second sorted) (map third sorted)))
      (let header "sequenceDiagram\n"
        (let participant-lines (map (lambda (p)
                                      (string-append "    participant " 
                                                     (escape-mermaid p)))
                                    participants)
          (let message-lines (map (lambda (t)
                                    (string-append "    " 
                                                   (escape-mermaid (second t))
                                                   " ->> "
                                                   (escape-mermaid (third t))
                                                   " : "
                                                   (escape-mermaid (fourth t))))
                                  sorted)
            (string-append header
                           (fold (lambda (acc line)
                                   (string-append acc line "\n"))
                                 "" participant-lines)
                           (fold (lambda (acc line)
                                   (string-append acc line "\n"))
                                 "" message-lines))))))))

; Convert traces to flow diagram (aggregated message counts)
(define (traces->flow)
  (let sorted (reverse *traces*)
    (let edges (map (lambda (t) 
                      (list (second t) (third t) (fourth t))) 
                    sorted)
      ; Group by from-to pair and count
      (let grouped (fold (lambda (acc e)
                          (let key (list (first e) (second e))
                            (let existing (assoc key acc)
                              (if existing
                                  (map (lambda (a)
                                        (if (= (first a) key)
                                            (list key (+ (second a) 1))
                                            a))
                                       acc)
                                  (cons (list key 1) acc)))))
                        '() edges)
        (let header "flowchart LR\n"
          (let lines (map (lambda (g)
                           (let from (first (first g))
                             (let to (second (first g))
                               (let count (second g)
                                 (string-append "    " 
                                                (escape-mermaid from)
                                                " -->|" (number->string count) "| "
                                                (escape-mermaid to))))))
                         grouped)
            (string-append header
                           (fold (lambda (acc line)
                                   (string-append acc line "\n"))
                                 "" lines))))))))

; ============================================================================
; CTL Formula Constructors
; ============================================================================

; Atomic proposition
(define (prop name)
  (tag 'ctl-prop name))

; Path quantifiers with temporal operators
(define (EX phi)
  (tag 'ctl-EX phi))

(define (EF phi)
  (tag 'ctl-EF phi))

(define (EG phi)
  (tag 'ctl-EG phi))

(define (AX phi)
  (tag 'ctl-AX phi))

(define (AF phi)
  (tag 'ctl-AF phi))

(define (AG phi)
  (tag 'ctl-AG phi))

; Until operators
(define (EU phi psi)
  (tag 'ctl-EU (list phi psi)))

(define (AU phi psi)
  (tag 'ctl-AU (list phi psi)))

; Boolean combinators for CTL
(define (ctl-and . args)
  (tag 'ctl-and args))

(define (ctl-or . args)
  (tag 'ctl-or args))

(define (ctl-not phi)
  (tag 'ctl-not phi))

(define (ctl-implies phi psi)
  (tag 'ctl-implies (list phi psi)))

; Property registration
(define *properties* '())

(define (defproperty name formula)
  (let prop (list 'property name formula)
    (set! *properties* (cons prop *properties*))
    prop))

(define (get-properties)
  *properties*)

; ============================================================================
; Probability Distributions
; ============================================================================

; Exponential distribution: -ln(U)/rate
; u is uniform random in [0,1), rate is lambda
; Returns time until next event
(define (exponential u rate)
  (if (<= u 0)
      0
      (/ (- (ln u)) rate)))

; Normal distribution via Box-Muller (needs two uniforms)
(define (normal u1 u2 mean stddev)
  (let r (sqrt (* -2 (ln u1)))
    (let theta (* 2 3.14159265 u2)
      (+ mean (* stddev r (cos theta))))))

; Bernoulli: returns 1 with probability p, 0 otherwise
(define (bernoulli u p)
  (if (< u p) 1 0))

; Discrete uniform: random integer in [min, max]
(define (discrete-uniform u min max)
  (+ min (floor (* u (+ 1 (- max min))))))

; Continuous uniform in [min, max]
(define (uniform-range u min max)
  (+ min (* u (- max min))))

; Geometric distribution: number of trials until first success
(define (geometric u p)
  (ceil (/ (ln u) (ln (- 1 p)))))

; Poisson approximation via inverse CDF
(define (poisson u lambda)
  (let L (exp (- lambda))
    (let loop (lambda (k p)
                (if (> p L)
                    (loop (+ k 1) (* p u))
                    (- k 1)))
      (loop 0 1))))

; ============================================================================
; Queueing Theory Helpers (M/M/1)
; ============================================================================

; M/M/1 utilization: rho = arrival_rate / service_rate
(define (mm1-utilization arrival-rate service-rate)
  (/ arrival-rate service-rate))

; M/M/1 average number in system: L = rho / (1 - rho)
(define (mm1-avg-system-length arrival-rate service-rate)
  (let rho (mm1-utilization arrival-rate service-rate)
    (/ rho (- 1 rho))))

; M/M/1 average time in system: W = 1 / (service_rate - arrival_rate)
(define (mm1-avg-system-time arrival-rate service-rate)
  (/ 1 (- service-rate arrival-rate)))

; M/M/1 average queue length: Lq = rho^2 / (1 - rho)
(define (mm1-avg-queue-length arrival-rate service-rate)
  (let rho (mm1-utilization arrival-rate service-rate)
    (/ (* rho rho) (- 1 rho))))

; ============================================================================
; Metrics Collection
; ============================================================================

(define *metrics-actor* nil)

; Metrics state structure:
; ((counters . ((name . count) ...))
;  (gauges . ((name . value) ...))
;  (timings . ((name . (samples...)) ...)))

(define (make-empty-metrics-state)
  (list (list 'counters) (list 'gauges) (list 'timings)))

(define (metrics-get state key)
  (let entry (assoc key state)
    (if entry (rest entry) '())))

(define (update-counter state name delta)
  (let counters (metrics-get state 'counters)
    (let existing (assoc name counters)
      (let new-val (if existing (+ (second existing) delta) delta)
        (let new-counters (if existing
                              (map (lambda (c)
                                    (if (= (first c) name)
                                        (list name new-val)
                                        c))
                                   counters)
                              (cons (list name new-val) counters))
          (map (lambda (entry)
                (if (= (first entry) 'counters)
                    (cons 'counters new-counters)
                    entry))
               state))))))

(define (update-gauge state name value)
  (let gauges (metrics-get state 'gauges)
    (let existing (assoc name gauges)
      (let new-gauges (if existing
                          (map (lambda (g)
                                (if (= (first g) name)
                                    (list name value)
                                    g))
                               gauges)
                          (cons (list name value) gauges))
        (map (lambda (entry)
              (if (= (first entry) 'gauges)
                  (cons 'gauges new-gauges)
                  entry))
             state)))))

(define (add-timing state name sample)
  (let timings (metrics-get state 'timings)
    (let existing (assoc name timings)
      (let samples (if existing (second existing) '())
        (let new-samples (append samples (list sample))
          (let new-timings (if existing
                              (map (lambda (t)
                                    (if (= (first t) name)
                                        (list name new-samples)
                                        t))
                                   timings)
                              (cons (list name new-samples) timings))
            (map (lambda (entry)
                  (if (= (first entry) 'timings)
                      (cons 'timings new-timings)
                      entry))
                 state)))))))

; Metrics actor implementation
(define (metrics-loop state)
  (let msg (receive!)
    (let cmd (first msg)
      (let new-state
        (cond
          ((= cmd 'inc) 
           (update-counter state (second msg) (third msg)))
          ((= cmd 'gauge)
           (update-gauge state (second msg) (third msg)))
          ((= cmd 'timing)
           (add-timing state (second msg) (third msg)))
          ((= cmd 'get)
           (do
             (send-to! (second msg) (list 'metrics state))
             state))
          (true state))
        (list 'become (list 'metrics-loop new-state))))))

(define (start-metrics!)
  (reset-scheduler)
  (set! *metrics-actor* 
    (spawn-actor 'metrics 64 '(metrics-loop (make-empty-metrics-state)))))

; Convenience functions to send metrics
(define (metric-inc! name)
  (send-to! 'metrics (list 'inc name 1)))

(define (metric-inc-by! name delta)
  (send-to! 'metrics (list 'inc name delta)))

(define (metric-gauge! name value)
  (send-to! 'metrics (list 'gauge name value)))

(define (metric-timing! name sample)
  (send-to! 'metrics (list 'timing name sample)))

; Get current metrics snapshot (blocking)
(define (get-metrics)
  (send-to! 'metrics (list 'get (self)))
  (let response (receive!)
    (second response)))

; ============================================================================
; EFSM Visualization
; ============================================================================

; Generate EFSM diagram with guards and mutations shown
; Format: state[predicate] --> state[predicate]: guard / action / mutations

(define (efsm->mermaid actor-name)
  (let states (actor-states actor-name)
    (let transitions (actor-transitions actor-name)
      (let header (string-append "stateDiagram-v2\n    %% EFSM: " 
                                  (escape-mermaid actor-name) "\n")
        (let state-lines (map (lambda (s)
                                (let name (third s)
                                  (let pred (fourth s)
                                    (string-append "    " 
                                                   (state->mermaid-id name)
                                                   " : " (escape-mermaid name)
                                                   "\\n[" (repr pred) "]"))))
                              states)
          (let trans-lines (map (lambda (t)
                                  (let tname (first t)
                                    (let from (state->mermaid-id (second t))
                                      (let to (state->mermaid-id (third t))
                                        (let guard (if (fourth t) 
                                                      (repr (fourth t)) 
                                                      "")
                                          (let action (if (fifth t)
                                                         (repr (fifth t))
                                                         "")
                                            (let muts (if (> (length t) 5)
                                                         (repr (nth t 5))
                                                         "")
                                              (let label (string-append 
                                                          guard
                                                          (if (not (= guard "")) " / " "")
                                                          action
                                                          (if (not (= muts "")) " / " "")
                                                          muts)
                                                (string-append "    " from " --> " to 
                                                               " : " (escape-mermaid tname)
                                                               (if (not (= label ""))
                                                                   (string-append "\\n" label)
                                                                   ""))))))))))
                                transitions)
            (string-append header
                           (fold (lambda (acc line)
                                   (string-append acc line "\n"))
                                 "" state-lines)
                           (fold (lambda (acc line)
                                   (string-append acc line "\n"))
                                 "" trans-lines))))))))

; ============================================================================
; System Specification Helpers  
; ============================================================================

; Define a complete actor with states and transitions in one call
(define (defsystem name . specs)
  (reset-defs!)
  (map (lambda (spec)
        (let kind (first spec)
          (cond
            ((= kind 'actor) 
             (defactor (second spec) (third spec) (fourth spec)))
            ((= kind 'channel)
             (defchannel (second spec) (third spec) (fourth spec) (fifth spec)))
            ((= kind 'state)
             (defstate (second spec) (third spec) (fourth spec)))
            ((= kind 'transition)
             (deftransition (second spec) (rest (rest spec))))
            ((= kind 'invariant)
             (definvariant (second spec) (third spec)))
            (true nil))))
       specs)
  (list 'system name))

; Generate all diagrams for current system
(define (system-diagrams)
  (list
    (cons 'states (all-state-diagrams))
    (cons 'sequence (channels->sequence-diagram))))

; ============================================================================
; Printing / Debug Helpers
; ============================================================================

(define (print-diagram type)
  (cond
    ((= type 'states) (println (all-state-diagrams)))
    ((= type 'sequence) (println (channels->sequence-diagram)))
    ((= type 'traces) (println (traces->sequence)))
    ((= type 'flow) (println (traces->flow)))
    (true (println "Unknown diagram type"))))

(define (print-spec)
  (println "=== Actors ===")
  (map (lambda (a) (println "  " a)) *actor-defs*)
  (println "=== Channels ===")
  (map (lambda (c) (println "  " c)) *channel-defs*)
  (println "=== States ===")
  (map (lambda (s) (println "  " s)) *state-defs*)
  (println "=== Transitions ===")
  (map (lambda (t) (println "  " t)) *transition-defs*)
  (println "=== Invariants ===")
  (map (lambda (i) (println "  " i)) *invariants*)
  'ok)

; ============================================================================
; End of Prologue
; ============================================================================

(println "BoundedLISP prologue loaded.")
