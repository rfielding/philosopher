;; ============================================================================
;; Datalog Tests in BoundedLISP
;; ============================================================================
;; Run with: ./philosopher datalog-tests.lisp

(print "=== Datalog Integration Tests ===")
(print "")

;; ----------------------------------------------------------------------------
;; Basic Facts
;; ----------------------------------------------------------------------------

(print "--- Basic Facts ---")

(assert! 'parent 'tom 'bob)
(assert! 'parent 'tom 'liz)
(assert! 'parent 'bob 'ann)
(assert! 'parent 'bob 'pat)
(assert! 'parent 'liz 'jim)

;; Query: Who are tom's children?
(print "Tom's children:")
(print (query 'parent 'tom '?x))

;; Query: All parent-child pairs
(print "All parent-child pairs:")
(print (query 'parent '?p '?c))

;; ----------------------------------------------------------------------------
;; Rules
;; ----------------------------------------------------------------------------

(print "")
(print "--- Rules ---")

;; grandparent(X, Z) :- parent(X, Y), parent(Y, Z)
(rule 'grandparent
  '(grandparent ?x ?z)
  '(parent ?x ?y)
  '(parent ?y ?z))

(print "Grandparent relationships:")
(print (query 'grandparent '?gp '?gc))

;; ancestor(X, Y) :- parent(X, Y)
;; ancestor(X, Z) :- parent(X, Y), ancestor(Y, Z)
(rule 'ancestor-base
  '(ancestor ?x ?y)
  '(parent ?x ?y))

(rule 'ancestor-trans
  '(ancestor ?x ?z)
  '(parent ?x ?y)
  '(ancestor ?y ?z))

(print "Tom's descendants (ancestor query):")
(print (query 'ancestor 'tom '?d))

;; ----------------------------------------------------------------------------
;; Negation
;; ----------------------------------------------------------------------------

(print "")
(print "--- Negation ---")

(datalog-clear!)

(assert! 'bird 'tweety)
(assert! 'bird 'penguin)
(assert! 'bird 'ostrich)
(assert! 'cannot-fly 'penguin)
(assert! 'cannot-fly 'ostrich)

;; can-fly(X) :- bird(X), not cannot-fly(X)
(rule 'can-fly
  '(can-fly ?x)
  '(bird ?x)
  '(not (cannot-fly ?x)))

(print "Birds that can fly:")
(print (query 'can-fly '?x))

;; ----------------------------------------------------------------------------
;; Builtins
;; ----------------------------------------------------------------------------

(print "")
(print "--- Builtin Comparisons ---")

(datalog-clear!)
(datalog-clear-rules!)

(assert! 'score 'alice 85)
(assert! 'score 'bob 72)
(assert! 'score 'carol 91)
(assert! 'score 'dave 72)

;; passed(X) :- score(X, S), S >= 75
(rule 'passed
  '(passed ?x)
  '(score ?x ?s)
  '(>= ?s 75))

(print "Students who passed (>= 75):")
(print (query 'passed '?x))

;; same-score(A, B) :- score(A, S), score(B, S), A != B
(rule 'same-score
  '(same-score ?a ?b)
  '(score ?a ?s)
  '(score ?b ?s)
  '(!= ?a ?b))

(print "Students with same score:")
(print (query 'same-score '?a '?b))

;; ----------------------------------------------------------------------------
;; Temporal Queries
;; ----------------------------------------------------------------------------

(print "")
(print "--- Temporal Queries ---")

(datalog-clear!)
(datalog-clear-rules!)

;; Simulate events over time
(assert-at! 1 'event 'start)
(assert-at! 2 'event 'process)
(assert-at! 3 'event 'checkpoint)
(assert-at! 4 'event 'process)
(assert-at! 5 'event 'complete)

(print "All facts with timestamps:")
(print (datalog-facts))

;; Query events
(print "Events that occurred (via query):")
(print (query 'event '?e))

;; ----------------------------------------------------------------------------
;; CSP Trace Analysis
;; ----------------------------------------------------------------------------

(print "")
(print "--- CSP Trace Analysis ---")

(datalog-clear!)
(datalog-clear-rules!)

;; Simulate actor execution traces
(assert-at! 1 'guard 'actor1 'receive)
(assert-at! 2 'effect 'actor1 'set 'counter)
(assert-at! 3 'sent 'actor1 'actor2 '(message hello))

;; BAD: effect before guard
(assert-at! 10 'effect 'actor2 'set 'state)
(assert-at! 11 'guard 'actor2 'receive)

;; Rule: violation if effect occurs without preceding guard
(rule 'has-guard
  '(has-guard ?actor)
  '(guard ?actor ?_))

(print "Actors with guards:")
(print (query 'has-guard '?a))

(print "All guards:")
(print (query 'guard '?actor '?type))

(print "All effects:")
(print (query 'effect '?actor '?op '?var))

;; ----------------------------------------------------------------------------
;; Message Flow Analysis
;; ----------------------------------------------------------------------------

(print "")
(print "--- Message Flow Analysis ---")

(datalog-clear!)
(datalog-clear-rules!)

(assert! 'sent 'alice 'bob 'hello)
(assert! 'sent 'bob 'alice 'reply)
(assert! 'sent 'alice 'carol 'request)
(assert! 'sent 'carol 'alice 'response)
(assert! 'sent 'dave 'eve 'message)

;; communicated(A, B) :- sent(A, B, _)
(rule 'communicated
  '(communicated ?a ?b)
  '(sent ?a ?b ?_))

;; bidirectional(A, B) :- communicated(A, B), communicated(B, A)
(rule 'bidirectional
  '(bidirectional ?a ?b)
  '(communicated ?a ?b)
  '(communicated ?b ?a))

(print "Bidirectional communication:")
(print (query 'bidirectional '?a '?b))

;; ----------------------------------------------------------------------------
;; Deadlock Detection
;; ----------------------------------------------------------------------------

(print "")
(print "--- Deadlock Detection ---")

(datalog-clear!)
(datalog-clear-rules!)

;; Scenario 1: Deadlock
(assert! 'waiting-for 'actor1 'actor2)
(assert! 'waiting-for 'actor2 'actor1)

;; deadlock(A, B) :- waiting-for(A, B), waiting-for(B, A)
(rule 'deadlock
  '(deadlock ?a ?b)
  '(waiting-for ?a ?b)
  '(waiting-for ?b ?a))

(print "Deadlock check (should find):")
(print (query 'deadlock '?a '?b))

;; Scenario 2: No deadlock
(datalog-clear!)
(assert! 'waiting-for 'actor1 'actor2)
(assert! 'waiting-for 'actor2 'actor3)
(assert! 'waiting-for 'actor3 'resource)

(print "Deadlock check (should be empty):")
(print (query 'deadlock '?a '?b))

;; ----------------------------------------------------------------------------
;; CTL Operators
;; ----------------------------------------------------------------------------

(print "")
(print "--- CTL-style Checks ---")

(datalog-clear!)
(datalog-clear-rules!)

(assert! 'valid 'state1)
(assert! 'valid 'state2)
(assert! 'valid 'state3)

(print "Eventually valid:")
(print (eventually? '(valid ?x)))

(print "Never invalid:")
(print (never? '(invalid ?x)))

;; Add invalid state
(assert! 'invalid 'broken)

(print "Never invalid (after adding broken):")
(print (never? '(invalid ?x)))

;; ----------------------------------------------------------------------------
;; Summary
;; ----------------------------------------------------------------------------

(print "")
(print "=== All Datalog Tests Complete ===")
