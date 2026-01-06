;;; ============================================================================
;;; BreadCo - Multi-Party Protocol Simulation
;;; ============================================================================
;;;
;;; Actors:
;;;   Production  - Bakes bread, sends to Trucks
;;;   Trucks      - Receives from Production, delivers to StoreFront
;;;   StoreFront  - Receives deliveries, sells to Customers
;;;   Customers   - Buys bread, pays money
;;;
;;; Message flows:
;;;   Production -> Trucks: (bread quantity)
;;;   Trucks -> StoreFront: (delivery quantity)
;;;   StoreFront -> Customers: (bread quantity)
;;;   Customers -> StoreFront: (payment amount)
;;;
;;; Metrics tracked via registry:
;;;   bread-produced, bread-delivered, bread-sold, revenue, inventory

;;; ============================================================================
;;; Metrics Infrastructure
;;; ============================================================================

;; Initialize all metrics
(define (init-metrics)
  (registry-set! 'bread-produced 0)
  (registry-set! 'bread-delivered 0)
  (registry-set! 'bread-sold 0)
  (registry-set! 'revenue 0)
  (registry-set! 'inventory 0)
  (registry-set! 'customer-visits 0)
  (registry-set! 'unmet-demand 0)
  ;; Time series data (lists that grow)
  (registry-set! 'ts-produced '())
  (registry-set! 'ts-delivered '())
  (registry-set! 'ts-sold '())
  (registry-set! 'ts-revenue '())
  (registry-set! 'ts-inventory '())
  (registry-set! 'tick 0))

;; Increment a metric
(define (metric-inc! name amount)
  (registry-set! name (+ (registry-get name) amount)))

;; Record time series snapshot
(define (record-snapshot!)
  (registry-set! 'ts-produced 
    (append (registry-get 'ts-produced) (list (registry-get 'bread-produced))))
  (registry-set! 'ts-delivered 
    (append (registry-get 'ts-delivered) (list (registry-get 'bread-delivered))))
  (registry-set! 'ts-sold 
    (append (registry-get 'ts-sold) (list (registry-get 'bread-sold))))
  (registry-set! 'ts-revenue 
    (append (registry-get 'ts-revenue) (list (registry-get 'revenue))))
  (registry-set! 'ts-inventory 
    (append (registry-get 'ts-inventory) (list (registry-get 'inventory))))
  (registry-set! 'tick (+ (registry-get 'tick) 1)))

;;; ============================================================================
;;; Production Actor
;;; ============================================================================
;;; Bakes bread in batches, sends to trucks

(define (production-init)
  ;; No state modification before guard in loop states
  (list 'become '(production-loop)))

(define (production-loop)
  (let msg (receive!)                          ; Guard first
    (let cmd (first msg)
      (if (= cmd 'bake)
        (let quantity (nth msg 1)
          (metric-inc! 'bread-produced quantity)
          (send-to! 'trucks (list 'load quantity))
          (list 'become '(production-loop)))
        (if (= cmd 'stop)
          'done
          (list 'become '(production-loop)))))))

;;; ============================================================================
;;; Trucks Actor  
;;; ============================================================================
;;; Receives bread from production, delivers to storefront

(define (trucks-init)
  (list 'become '(trucks-loop 0)))  ; carrying = 0

(define (trucks-loop carrying)
  (let msg (receive!)                          ; Guard first
    (let cmd (first msg)
      (if (= cmd 'load)
        (let quantity (nth msg 1)
          (let new-carrying (+ carrying quantity)
            (list 'become (list 'trucks-loop new-carrying))))
        (if (= cmd 'deliver)
          (if (> carrying 0)
            (begin
              (metric-inc! 'bread-delivered carrying)
              (send-to! 'storefront (list 'delivery carrying))
              (list 'become '(trucks-loop 0)))
            (list 'become (list 'trucks-loop carrying)))
          (if (= cmd 'stop)
            'done
            (list 'become (list 'trucks-loop carrying))))))))

;;; ============================================================================
;;; StoreFront Actor
;;; ============================================================================
;;; Receives deliveries, sells to customers

(define (storefront-init)
  (list 'become '(storefront-loop)))

(define (storefront-loop)
  (let msg (receive!)                          ; Guard first
    (let cmd (first msg)
      (if (= cmd 'delivery)
        (let quantity (nth msg 1)
          (metric-inc! 'inventory quantity)
          (list 'become '(storefront-loop)))
        (if (= cmd 'buy)
          (let customer (nth msg 1)
            (let want (nth msg 2)
              (let have (registry-get 'inventory)
                (let sold (min want have)
                  (if (> sold 0)
                    (begin
                      (metric-inc! 'inventory (- 0 sold))
                      (metric-inc! 'bread-sold sold)
                      (send-to! customer (list 'purchase sold))
                      (list 'become '(storefront-loop)))
                    (begin
                      (metric-inc! 'unmet-demand want)
                      (send-to! customer (list 'sold-out))
                      (list 'become '(storefront-loop))))))))
          (if (= cmd 'stop)
            'done
            (list 'become '(storefront-loop))))))))

;;; ============================================================================
;;; Customer Actor
;;; ============================================================================
;;; Visits store, buys bread, pays

(define (customer-init name)
  (list 'become (list 'customer-loop name 0)))  ; purchased = 0

(define (customer-loop name purchased)
  (let msg (receive!)                          ; Guard first
    (let cmd (first msg)
      (if (= cmd 'visit)
        (let want (nth msg 1)
          (metric-inc! 'customer-visits 1)
          (send-to! 'storefront (list 'buy name want))
          (list 'become (list 'customer-waiting name purchased want)))
        (if (= cmd 'stop)
          'done
          (list 'become (list 'customer-loop name purchased)))))))

(define (customer-waiting name purchased want)
  (let msg (receive!)                          ; Guard first
    (let cmd (first msg)
      (if (= cmd 'purchase)
        (let got (nth msg 1)
          (let price (* got 3)                 ; $3 per bread
            (metric-inc! 'revenue price)
            (list 'become (list 'customer-loop name (+ purchased got)))))
        (if (= cmd 'sold-out)
          (list 'become (list 'customer-loop name purchased))
          (list 'become (list 'customer-waiting name purchased want)))))))

;;; ============================================================================
;;; Simulation Driver
;;; ============================================================================

(define (spawn-breadco)
  (reset-scheduler)
  (init-metrics)
  
  ;; Spawn all actors
  (spawn-actor 'production 16 '(production-init))
  (spawn-actor 'trucks 16 '(trucks-init))
  (spawn-actor 'storefront 32 '(storefront-loop))
  (spawn-actor 'customer-alice 8 '(customer-init customer-alice))
  (spawn-actor 'customer-bob 8 '(customer-init customer-bob))
  (spawn-actor 'customer-carol 8 '(customer-init customer-carol))
  
  'ready)

;; Run one "day" of simulation
(define (sim-day production-amount deliveries customer-demand)
  ;; Morning: bake bread
  (send-to! 'production (list 'bake production-amount))
  (run-scheduler 10)
  
  ;; Midday: truck delivers
  (send-to! 'trucks (list 'deliver))
  (run-scheduler 10)
  
  ;; Afternoon: customers visit
  (send-to! 'customer-alice (list 'visit (nth customer-demand 0)))
  (send-to! 'customer-bob (list 'visit (nth customer-demand 1)))
  (send-to! 'customer-carol (list 'visit (nth customer-demand 2)))
  (run-scheduler 30)
  
  ;; Record metrics
  (record-snapshot!))

;; Run full simulation
(define (run-simulation days)
  (spawn-breadco)
  
  ;; Run each day with varying production and demand
  (define (loop day)
    (if (<= day days)
      (begin
        ;; Vary production: 10 + day (ramping up)
        ;; Vary demand: each customer wants 2-4 breads
        (sim-day (+ 10 day) 1 (list 3 2 4))
        (loop (+ day 1)))
      'done))
  (loop 1)
  
  ;; Return final stats
  (list
    (list 'days days)
    (list 'produced (registry-get 'bread-produced))
    (list 'delivered (registry-get 'bread-delivered))
    (list 'sold (registry-get 'bread-sold))
    (list 'revenue (registry-get 'revenue))
    (list 'inventory (registry-get 'inventory))
    (list 'unmet-demand (registry-get 'unmet-demand))))

;;; ============================================================================
;;; Chart Generation
;;; ============================================================================

;; Generate xychart for a time series
(define (make-xychart title y-label data-name)
  (let data (registry-get data-name)
    (let n (length data)
      (let x-labels (make-day-labels n)
        (string-append
          "xychart-beta\n"
          "    title \"" title "\"\n"
          "    x-axis " (list->chart-array x-labels) "\n"
          "    y-axis \"" y-label "\"\n"
          "    line " (list->chart-array data) "\n")))))

;; Generate bar chart
(define (make-barchart title y-label data-name)
  (let data (registry-get data-name)
    (let n (length data)
      (let x-labels (make-day-labels n)
        (string-append
          "xychart-beta\n"
          "    title \"" title "\"\n"
          "    x-axis " (list->chart-array x-labels) "\n"
          "    y-axis \"" y-label "\"\n"
          "    bar " (list->chart-array data) "\n")))))

;; Helper: make day labels
(define (make-day-labels n)
  (define (loop i acc)
    (if (> i n)
      acc
      (loop (+ i 1) (append acc (list (string-append "D" (number->string i)))))))
  (loop 1 '()))

;; Helper: convert list to chart array format
(define (list->chart-array lst)
  (if (empty? lst)
    "[]"
    (let items (map-to-strings lst)
      (string-append "[" (join-strings items ", ") "]"))))

;; Helper: map values to strings  
(define (map-to-strings lst)
  (if (empty? lst)
    '()
    (cons (value->string (first lst)) (map-to-strings (rest lst)))))

(define (value->string v)
  (if (number? v)
    (number->string v)
    (if (string? v)
      (string-append "\"" v "\"")
      (repr v))))

;; Helper: join strings
(define (join-strings lst sep)
  (if (empty? lst)
    ""
    (if (empty? (rest lst))
      (first lst)
      (string-append (first lst) sep (join-strings (rest lst) sep)))))

;;; ============================================================================
;;; Generate All Charts
;;; ============================================================================

(define (print-all-charts)
  (println "")
  (println "### Cumulative Production")
  (println "```mermaid")
  (println (make-xychart "Bread Produced (Cumulative)" "Loaves" 'ts-produced))
  (println "```")
  (println "")
  (println "### Cumulative Sales")
  (println "```mermaid")
  (println (make-xychart "Bread Sold (Cumulative)" "Loaves" 'ts-sold))
  (println "```")
  (println "")
  (println "### Revenue Over Time")
  (println "```mermaid")
  (println (make-barchart "Revenue" "Dollars ($)" 'ts-revenue))
  (println "```")
  (println "")
  (println "### Inventory Levels")
  (println "```mermaid")
  (println (make-xychart "End-of-Day Inventory" "Loaves" 'ts-inventory))
  (println "```"))

;;; ============================================================================
;;; Run It
;;; ============================================================================

(println "=== BreadCo Simulation ===")
(println "")
(println (run-simulation 7))
(println "")
(print-all-charts)
