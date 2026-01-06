; ============================================================================
; Example: Simple Order Protocol
; Customer places order, Merchant fulfills or rejects
; ============================================================================

; Load prologue first:
; cat prologue.lisp example-order.lisp | go run main.go -repl

; ============================================================================
; Define the System
; ============================================================================

; Actors with their variables
(defactor 'customer 
  '((balance 100) (pending nil) (status idle))
  '())

(defactor 'merchant
  '((inventory 5) (revenue 0))
  '())

(defactor 'bank
  '((escrow 0))
  '())

; Channels (synchronous rendezvous, but bounded buffer for diagram purposes)
(defchannel 'order-ch 'customer 'merchant 1)
(defchannel 'payment-ch 'customer 'bank 1)
(defchannel 'confirm-ch 'merchant 'customer 1)
(defchannel 'settle-ch 'bank 'merchant 1)

; ============================================================================
; Customer States
; ============================================================================

(defstate 'customer 'idle '(= status idle))
(defstate 'customer 'ordering '(= status ordering))
(defstate 'customer 'awaiting '(= status awaiting))
(defstate 'customer 'complete '(= status complete))

; ============================================================================
; Customer Transitions
; ============================================================================

(deftransition 'place-order
  '(idle ordering 
    (> balance 0)
    (send order-ch (list 'order 1))
    ((set! status ordering) (set! pending 1))))

(deftransition 'pay
  '(ordering awaiting
    pending
    (send payment-ch (list 'pay 10))
    ((set! balance (- balance 10)) (set! status awaiting))))

(deftransition 'receive-confirm
  '(awaiting complete
    true
    (recv confirm-ch msg)
    ((set! status complete) (set! pending nil))))

; ============================================================================
; Merchant States  
; ============================================================================

(defstate 'merchant 'ready '(> inventory 0))
(defstate 'merchant 'out-of-stock '(= inventory 0))
(defstate 'merchant 'processing '(= mstatus processing))

; ============================================================================
; Merchant Transitions
; ============================================================================

(deftransition 'receive-order
  '(ready processing
    true
    (recv order-ch msg)
    ((set! mstatus processing))))

(deftransition 'fulfill
  '(processing ready
    (> inventory 0)
    (send confirm-ch 'shipped)
    ((set! inventory (- inventory 1)) (set! mstatus nil))))

(deftransition 'reject
  '(processing ready
    (= inventory 0)
    (send confirm-ch 'rejected)
    ((set! mstatus nil))))

; ============================================================================
; Bank States
; ============================================================================

(defstate 'bank 'idle '(= escrow 0))
(defstate 'bank 'holding '(> escrow 0))

; ============================================================================
; Bank Transitions
; ============================================================================

(deftransition 'hold-payment
  '(idle holding
    true
    (recv payment-ch msg)
    ((set! escrow (+ escrow (second msg))))))

(deftransition 'release-to-merchant
  '(holding idle
    true
    (send settle-ch escrow)
    ((set! escrow 0))))

; ============================================================================
; Invariants (Conservation Laws)
; ============================================================================

(definvariant 'money-conserved
  '(= (+ customer.balance merchant.revenue bank.escrow) 100))

(definvariant 'inventory-bound
  '(>= merchant.inventory 0))

; ============================================================================
; CTL Properties
; ============================================================================

(defproperty 'eventually-complete
  (AG (ctl-implies (prop 'ordering) (AF (prop 'complete)))))

(defproperty 'no-negative-balance
  (AG (ctl-not (prop 'negative-balance))))

(defproperty 'orders-get-response
  (AG (ctl-implies (prop 'order-placed) 
                   (AF (ctl-or (prop 'shipped) (prop 'rejected))))))

; ============================================================================
; Generate Diagrams
; ============================================================================

(println "")
(println "=== STATE DIAGRAMS ===")
(println (all-state-diagrams))

(println "")
(println "=== SEQUENCE DIAGRAM ===")
(println (channels->sequence-diagram))

(println "")
(println "=== EFSM: Customer ===")
(println (efsm->mermaid 'customer))

(println "")
(println "=== EFSM: Merchant ===")
(println (efsm->mermaid 'merchant))

(println "")
(println "=== PROPERTIES ===")
(map (lambda (p) (println "  " p)) (get-properties))

(println "")
(println "=== INVARIANTS ===")
(map (lambda (i) (println "  " i)) *invariants*)
