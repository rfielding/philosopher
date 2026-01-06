;; BreadCo Multi-Actor Simulation
;; Demonstrates CSP-compliant actor patterns with metrics tracking

;; ============================================================================
;; Metrics Infrastructure
;; ============================================================================

(registry-set! 'bread-produced 0)
(registry-set! 'bread-delivered 0)
(registry-set! 'bread-sold 0)
(registry-set! 'revenue 0)
(registry-set! 'inventory 0)
(registry-set! 'unmet-demand 0)
(registry-set! 'day 0)

;; Time series for charts
(registry-set! 'ts-produced '())
(registry-set! 'ts-sold '())
(registry-set! 'ts-revenue '())
(registry-set! 'ts-inventory '())

(define (snapshot-metrics)
  (registry-set! 'ts-produced 
    (append (registry-get 'ts-produced) (list (registry-get 'bread-produced))))
  (registry-set! 'ts-sold 
    (append (registry-get 'ts-sold) (list (registry-get 'bread-sold))))
  (registry-set! 'ts-revenue 
    (append (registry-get 'ts-revenue) (list (registry-get 'revenue))))
  (registry-set! 'ts-inventory 
    (append (registry-get 'ts-inventory) (list (registry-get 'inventory)))))

;; ============================================================================
;; Production Actor - Bakes bread, sends to trucks
;; ============================================================================

(define (production-init)
  (let msg (receive!)                          ; Guard: wait for day start
    (let day (registry-get 'day)
      (let quantity (+ 10 day)                 ; Ramp up production
        (registry-set! 'bread-produced 
          (+ (registry-get 'bread-produced) quantity))
        (send-to! 'trucks (list 'bread quantity))
        (list 'become '(production-init))))))

;; ============================================================================
;; Trucks Actor - Receives from production, delivers to storefront
;; ============================================================================

(define (trucks-init)
  (let msg (receive!)                          ; Guard: wait for bread
    (if (= (first msg) 'bread)
      (let quantity (nth msg 1)
        (registry-set! 'bread-delivered 
          (+ (registry-get 'bread-delivered) quantity))
        (send-to! 'storefront (list 'delivery quantity))
        (list 'become '(trucks-init)))
      (list 'become '(trucks-init)))))

;; ============================================================================
;; StoreFront Actor - Receives deliveries, sells to customers
;; ============================================================================

(define (storefront-init)
  (let msg (receive!)                          ; Guard: wait for message
    (let cmd (first msg)
      (if (= cmd 'delivery)
        ;; Handle delivery
        (let quantity (nth msg 1)
          (registry-set! 'inventory 
            (+ (registry-get 'inventory) quantity))
          (list 'become '(storefront-init)))
        (if (= cmd 'buy)
          ;; Handle purchase
          (let customer (nth msg 1)
            (let want (nth msg 2)
              (let have (registry-get 'inventory)
                (if (>= have want)
                  ;; Can fulfill
                  (let give (min want have)
                    (registry-set! 'inventory (- have give))
                    (registry-set! 'bread-sold 
                      (+ (registry-get 'bread-sold) give))
                    (registry-set! 'revenue 
                      (+ (registry-get 'revenue) (* give 3)))
                    (send-to! customer (list 'purchase give))
                    (list 'become '(storefront-init)))
                  ;; Out of stock
                  (let unmet want
                    (registry-set! 'unmet-demand 
                      (+ (registry-get 'unmet-demand) unmet))
                    (send-to! customer '(sold-out))
                    (list 'become '(storefront-init)))))))
          (list 'become '(storefront-init)))))))

;; ============================================================================
;; Customer Actors
;; ============================================================================

(define (customer-alice)
  (let msg (receive!)                          ; Guard
    (if (= (first msg) 'go-shopping)
      (let want (+ 2 (rand 3))                 ; Want 2-4 loaves
        (send-to! 'storefront (list 'buy 'customer-alice want))
        (list 'become '(customer-wait-alice)))
      (list 'become '(customer-alice)))))

(define (customer-wait-alice)
  (let msg (receive!)                          ; Guard: wait for response
    (list 'become '(customer-alice))))

(define (customer-bob)
  (let msg (receive!)
    (if (= (first msg) 'go-shopping)
      (let want (+ 1 (rand 4))                 ; Want 1-4 loaves
        (send-to! 'storefront (list 'buy 'customer-bob want))
        (list 'become '(customer-wait-bob)))
      (list 'become '(customer-bob)))))

(define (customer-wait-bob)
  (let msg (receive!)
    (list 'become '(customer-bob))))

(define (customer-carol)
  (let msg (receive!)
    (if (= (first msg) 'go-shopping)
      (let want (+ 2 (rand 5))                 ; Want 2-6 loaves
        (send-to! 'storefront (list 'buy 'customer-carol want))
        (list 'become '(customer-wait-carol)))
      (list 'become '(customer-carol)))))

(define (customer-wait-carol)
  (let msg (receive!)
    (list 'become '(customer-carol))))

;; ============================================================================
;; Day Controller - Orchestrates simulation
;; ============================================================================

(define (day-controller)
  (let msg (receive!)                          ; Guard
    (if (= msg 'tick)
      (let d (+ 1 (registry-get 'day))
        (registry-set! 'day d)
        ;; Start production
        (send-to! 'production 'start-day)
        ;; Send customers shopping
        (send-to! 'customer-alice '(go-shopping))
        (send-to! 'customer-bob '(go-shopping))
        (send-to! 'customer-carol '(go-shopping))
        ;; Snapshot end of day
        (snapshot-metrics)
        (list 'become '(day-controller)))
      (list 'become '(day-controller)))))

;; ============================================================================
;; Spawn Actors
;; ============================================================================

(spawn-actor 'production 16 '(production-init))
(spawn-actor 'trucks 16 '(trucks-init))
(spawn-actor 'storefront 32 '(storefront-init))
(spawn-actor 'customer-alice 8 '(customer-alice))
(spawn-actor 'customer-bob 8 '(customer-bob))
(spawn-actor 'customer-carol 8 '(customer-carol))
(spawn-actor 'day-controller 16 '(day-controller))

;; ============================================================================
;; Run Simulation (7 days)
;; ============================================================================

(define (run-days n)
  (if (<= n 0)
    'done
    (let _ (send-to! 'day-controller 'tick)
      (let _ (run-scheduler 100)
        (run-days (- n 1))))))

(run-days 7)

;; Print results
(print "=== BreadCo 7-Day Results ===")
(print (concat "Produced: " (registry-get 'bread-produced)))
(print (concat "Sold: " (registry-get 'bread-sold)))
(print (concat "Revenue: $" (registry-get 'revenue)))
(print (concat "Inventory: " (registry-get 'inventory)))
(print (concat "Unmet Demand: " (registry-get 'unmet-demand)))
