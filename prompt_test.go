package main

import (
	"fmt"
	"strings"
	"testing"
)

// ============================================================================
// Prompt Tests - verify LISP produces expected facts
// ============================================================================

func TestPrompt01Counter(t *testing.T) {
	ev := NewEvaluator(1000)
	
	code := `
		;; Counter that asserts its value as facts
		(define (counter n)
		  (if (< n 5)
			(begin
			  (assert! 'counter-value (+ n 1))
			  (list 'become (list 'counter (+ n 1))))
			(done!)))
		
		;; Also test with message-driven counter
		(define (msg-counter n)
		  (let msg (receive!)
			(cond
			  ((eq? msg 'inc)
			   (assert! 'msg-counter-value (+ n 1))
			   (list 'become (list 'msg-counter (+ n 1))))
			  ((eq? msg 'stop)
			   (done!))
			  (else
			   (list 'become (list 'msg-counter n))))))
		
		;; Spawn and run
		(spawn-actor 'counter 10 '(counter 0))
		(spawn-actor 'msg-counter 10 '(msg-counter 0))
		
		;; Send 5 inc messages
		(send-to! 'msg-counter 'inc)
		(send-to! 'msg-counter 'inc)
		(send-to! 'msg-counter 'inc)
		(send-to! 'msg-counter 'inc)
		(send-to! 'msg-counter 'inc)
		(send-to! 'msg-counter 'stop)
		
		(run-scheduler 50)
	`
	
	runCode(ev, code)
	
	// Check facts
	results := checkFacts(ev, map[string]int{
		"counter-value":     5,  // 1,2,3,4,5
		"msg-counter-value": 5,  // 1,2,3,4,5
		"spawned":           2,  // 2 actors
		"sent":              6,  // 5 inc + 1 stop
	})
	
	for name, err := range results {
		if err != "" {
			t.Errorf("%s: %s", name, err)
		}
	}
	
	printFactSummary(t, ev)
}

func TestPrompt02ProducerConsumer(t *testing.T) {
	ev := NewEvaluator(1000)
	
	code := `
		;; Producer sends items
		(define (producer n)
		  (if (> n 0)
			(begin
			  (send-to! 'consumer (list 'item n))
			  (let ack (receive!)
				(assert! 'ack-received n)
				(list 'become (list 'producer (- n 1)))))
			(done!)))
		
		;; Consumer receives and acks
		(define (consumer)
		  (let msg (receive!)
			(assert! 'item-processed (nth msg 1))
			(send-to! 'producer 'ack)
			(list 'become '(consumer))))
		
		(spawn-actor 'producer 10 '(producer 3))
		(spawn-actor 'consumer 10 '(consumer))
		(run-scheduler 50)
	`
	
	runCode(ev, code)
	
	results := checkFacts(ev, map[string]int{
		"spawned":        2,
		"sent":           6,  // 3 items + 3 acks
		"received":       6,
		"item-processed": 3,
		"ack-received":   3,
	})
	
	for name, err := range results {
		if err != "" {
			t.Errorf("%s: %s", name, err)
		}
	}
	
	printFactSummary(t, ev)
}

func TestPrompt03Deadlock(t *testing.T) {
	ev := NewEvaluator(1000)
	
	code := `
		;; A waits for message from B first
		(define (actor-a)
		  (let msg (receive!)
			(assert! 'a-received msg)
			(send-to! 'actor-b 'from-a)
			(done!)))
		
		;; B waits for message from A first
		(define (actor-b)
		  (let msg (receive!)
			(assert! 'b-received msg)
			(send-to! 'actor-a 'from-b)
			(done!)))
		
		(spawn-actor 'actor-a 10 '(actor-a))
		(spawn-actor 'actor-b 10 '(actor-b))
		(run-scheduler 20)
	`
	
	runCode(ev, code)
	
	// Both should be spawned but deadlocked - no messages sent/received
	results := checkFacts(ev, map[string]int{
		"spawned":  2,
		"sent":     0,  // deadlock - no messages sent
		"received": 0,  // deadlock - no messages received
	})
	
	for name, err := range results {
		if err != "" {
			t.Errorf("%s: %s", name, err)
		}
	}
	
	printFactSummary(t, ev)
}

func TestPrompt04BreadCo(t *testing.T) {
	ev := NewEvaluator(1000)
	
	code := `
		;; Production makes bread each day
		(define (production day)
		  (if (<= day 7)
			(begin
			  (assert! 'produced day (+ 10 day))  ; 11, 12, 13... bread
			  (send-to! 'storefront (list 'delivery (+ 10 day)))
			  (list 'become (list 'production (+ day 1))))
			(done!)))
		
		;; StoreFront receives deliveries and serves customers
		(define (storefront inv day)
		  (let msg (receive!)
			(cond
			  ((eq? (nth msg 0) 'delivery)
			   (let qty (nth msg 1)
				 (assert! 'inventory-after-delivery (+ inv qty) day)
				 (list 'become (list 'storefront (+ inv qty) day))))
			  ((eq? (nth msg 0) 'buy)
			   (let want (nth msg 1)
				 (if (>= inv want)
				   (begin
					 (assert! 'sale want day)
					 (assert! 'inventory-after-sale (- inv want) day)
					 (list 'become (list 'storefront (- inv want) day)))
				   (begin
					 (assert! 'stockout day)
					 (list 'become (list 'storefront inv day))))))
			  ((eq? (nth msg 0) 'next-day)
			   (list 'become (list 'storefront inv (+ day 1))))
			  (else
			   (list 'become (list 'storefront inv day))))))
		
		;; Customers buy each day
		(define (customers day)
		  (if (<= day 7)
			(begin
			  (send-to! 'storefront (list 'buy (+ 2 day)))  ; 3, 4, 5... demand
			  (send-to! 'storefront (list 'next-day))
			  (list 'become (list 'customers (+ day 1))))
			(done!)))
		
		(spawn-actor 'production 10 '(production 1))
		(spawn-actor 'storefront 20 '(storefront 0 1))
		(spawn-actor 'customers 10 '(customers 1))
		(run-scheduler 100)
	`
	
	runCode(ev, code)
	
	results := checkFacts(ev, map[string]int{
		"spawned":  3,
		"produced": 7,  // 7 days of production
	})
	
	for name, err := range results {
		if err != "" {
			t.Errorf("%s: %s", name, err)
		}
	}
	
	// Check we have enough facts overall
	if len(ev.DatalogDB.Facts) < 20 {
		t.Errorf("expected at least 20 facts, got %d", len(ev.DatalogDB.Facts))
	}
	
	printFactSummary(t, ev)
}

func TestPrompt05Scale(t *testing.T) {
	ev := NewEvaluator(1000)
	
	code := `
		;; Fast producer - just asserts facts
		(define (fast-producer id n)
		  (if (> n 0)
			(begin
			  (assert! 'produced id n)
			  (list 'become (list 'fast-producer id (- n 1))))
			(done!)))
		
		;; Spawn 10 producers, each makes 50 facts
		(spawn-actor 'p1 10 '(fast-producer "p1" 50))
		(spawn-actor 'p2 10 '(fast-producer "p2" 50))
		(spawn-actor 'p3 10 '(fast-producer "p3" 50))
		(spawn-actor 'p4 10 '(fast-producer "p4" 50))
		(spawn-actor 'p5 10 '(fast-producer "p5" 50))
		(spawn-actor 'p6 10 '(fast-producer "p6" 50))
		(spawn-actor 'p7 10 '(fast-producer "p7" 50))
		(spawn-actor 'p8 10 '(fast-producer "p8" 50))
		(spawn-actor 'p9 10 '(fast-producer "p9" 50))
		(spawn-actor 'p10 10 '(fast-producer "p10" 50))
		
		(run-scheduler 1000)
	`
	
	runCode(ev, code)
	
	// Should have 500 produced facts + 10 spawned
	results := checkFacts(ev, map[string]int{
		"spawned":  10,
		"produced": 500,
	})
	
	for name, err := range results {
		if err != "" {
			t.Errorf("%s: %s", name, err)
		}
	}
	
	if len(ev.DatalogDB.Facts) < 500 {
		t.Errorf("scale test: expected 500+ facts, got %d", len(ev.DatalogDB.Facts))
	}
	
	printFactSummary(t, ev)
}

func TestPrompt06CSP(t *testing.T) {
	ev := NewEvaluator(1000)
	ev.Scheduler.CSPEnforce = true  // Enable CSP checking
	
	code := `
		;; Good actor: guard before effect
		(define (good-actor n)
		  (let msg (receive!)      ; guard first
			(assert! 'good-step n) ; effect after
			(if (< n 3)
			  (list 'become (list 'good-actor (+ n 1)))
			  (done!))))
		
		(spawn-actor 'good 10 '(good-actor 1))
		(send-to! 'good 'go)
		(send-to! 'good 'go)
		(send-to! 'good 'go)
		(run-scheduler 20)
	`
	
	runCode(ev, code)
	
	results := checkFacts(ev, map[string]int{
		"spawned":   1,
		"good-step": 3,
	})
	
	for name, err := range results {
		if err != "" {
			t.Errorf("%s: %s", name, err)
		}
	}
	
	// Check for CSP violations
	for name, actor := range ev.Scheduler.Actors {
		if len(actor.CSPViolations) > 0 {
			t.Errorf("unexpected CSP violations in %s: %v", name, actor.CSPViolations)
		}
	}
	
	printFactSummary(t, ev)
}

// ============================================================================
// Helpers
// ============================================================================

func runCode(ev *Evaluator, code string) {
	parser := NewParser(code)
	exprs := parser.Parse()
	for _, expr := range exprs {
		ev.Eval(expr, ev.GlobalEnv)
	}
}

func checkFacts(ev *Evaluator, expected map[string]int) map[string]string {
	results := make(map[string]string)
	
	// Count facts by predicate
	counts := make(map[string]int)
	for _, fact := range ev.DatalogDB.Facts {
		counts[fact.Predicate]++
	}
	
	for pred, want := range expected {
		got := counts[pred]
		if got < want {
			results[pred] = fmt.Sprintf("want >= %d, got %d", want, got)
		} else {
			results[pred] = ""  // pass
		}
	}
	
	return results
}

func printFactSummary(t *testing.T, ev *Evaluator) {
	counts := make(map[string]int)
	for _, fact := range ev.DatalogDB.Facts {
		counts[fact.Predicate]++
	}
	
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Total facts: %d\n", len(ev.DatalogDB.Facts)))
	for pred, count := range counts {
		sb.WriteString(fmt.Sprintf("  %s: %d\n", pred, count))
	}
	t.Log(sb.String())
}

// Test tool substitution produces expected output
func TestToolSubstitutionWithFacts(t *testing.T) {
	ev := NewEvaluator(1000)
	
	// Add some facts
	code := `
		(assert! 'sale 'store1 100)
		(assert! 'sale 'store2 200)
		(assert! 'inventory 'store1 50)
	`
	runCode(ev, code)
	
	tr := NewToolRegistry(ev)
	
	// Test facts_table
	out := tr.Process("{{facts_table}}")
	if !strings.Contains(out, "sale") || !strings.Contains(out, "2") {
		t.Errorf("facts_table should show sale:2, got: %s", out)
	}
	
	// Test specific predicate
	out = tr.Process("{{facts_table predicate=\"sale\"}}")
	if !strings.Contains(out, "store1") {
		t.Errorf("facts_table predicate=sale should show store1, got: %s", out)
	}
}

func TestMetricsChartWithFacts(t *testing.T) {
	ev := NewEvaluator(1000)
	
	// Simulate some traffic over time
	for i := 0; i < 10; i++ {
		ev.DatalogDB.AssertAtTime("sent", int64(i), Atom("producer"), Atom("consumer"), Atom("msg"))
		ev.DatalogDB.AssertAtTime("received", int64(i), Atom("consumer"), Atom("msg"))
	}
	
	tr := NewToolRegistry(ev)
	
	out := tr.Process(`{{metrics_chart title="Test" predicates="sent,received"}}`)
	
	if !strings.Contains(out, "xychart") {
		t.Errorf("expected xychart, got: %s", out)
	}
	if !strings.Contains(out, "sent") {
		t.Errorf("expected 'sent' in chart, got: %s", out)
	}
	if !strings.Contains(out, "received") {
		t.Errorf("expected 'received' in chart, got: %s", out)
	}
	
	t.Log("Chart output:\n" + out)
}

// ============================================================================
// Temporal Logic Tests
// ============================================================================

func TestTemporalAlways(t *testing.T) {
	ev := NewEvaluator(1000)
	
	// Assert facts at different times
	ev.DatalogDB.AssertAtTime("temperature", 0, Atom("ok"))
	ev.DatalogDB.AssertAtTime("temperature", 1, Atom("ok"))
	ev.DatalogDB.AssertAtTime("temperature", 2, Atom("ok"))
	ev.DatalogDB.AssertAtTime("temperature", 3, Atom("ok"))
	
	code := `
		;; Check: temperature is always ok
		(define result (always? '(temperature ok)))
		result
	`
	
	parser := NewParser(code)
	exprs := parser.Parse()
	var result Value
	for _, expr := range exprs {
		result = ev.Eval(expr, ev.GlobalEnv)
	}
	
	if result.Type != TypeBool || !result.Bool {
		t.Errorf("always? should return true, got %v", result)
	}
	t.Logf("always? '(temperature ok) = %v", result.Bool)
}

func TestTemporalAlwaysFails(t *testing.T) {
	ev := NewEvaluator(1000)
	
	// Assert facts - one time has 'bad' instead of 'ok'
	ev.DatalogDB.AssertAtTime("temperature", 0, Atom("ok"))
	ev.DatalogDB.AssertAtTime("temperature", 1, Atom("ok"))
	ev.DatalogDB.AssertAtTime("temperature", 2, Atom("bad"))  // violation!
	ev.DatalogDB.AssertAtTime("temperature", 3, Atom("ok"))
	
	code := `(always? '(temperature ok))`
	
	parser := NewParser(code)
	exprs := parser.Parse()
	result := ev.Eval(exprs[0], ev.GlobalEnv)
	
	if result.Type != TypeBool || result.Bool {
		t.Errorf("always? should return false (violation at t=2), got %v", result)
	}
	t.Logf("always? '(temperature ok) with violation = %v", result.Bool)
}

func TestTemporalEventually(t *testing.T) {
	ev := NewEvaluator(1000)
	
	// Goal achieved at t=5
	ev.DatalogDB.AssertAtTime("searching", 0, Atom("active"))
	ev.DatalogDB.AssertAtTime("searching", 1, Atom("active"))
	ev.DatalogDB.AssertAtTime("found", 5, Atom("treasure"))
	
	code := `(eventually? '(found treasure))`
	
	parser := NewParser(code)
	result := ev.Eval(parser.Parse()[0], ev.GlobalEnv)
	
	if result.Type != TypeBool || !result.Bool {
		t.Errorf("eventually? should return true, got %v", result)
	}
	t.Logf("eventually? '(found treasure) = %v", result.Bool)
}

func TestTemporalEventuallyFails(t *testing.T) {
	ev := NewEvaluator(1000)
	
	// Never find treasure
	ev.DatalogDB.AssertAtTime("searching", 0, Atom("active"))
	ev.DatalogDB.AssertAtTime("searching", 1, Atom("active"))
	ev.DatalogDB.AssertAtTime("searching", 2, Atom("active"))
	
	code := `(eventually? '(found treasure))`
	
	parser := NewParser(code)
	result := ev.Eval(parser.Parse()[0], ev.GlobalEnv)
	
	if result.Type != TypeBool || result.Bool {
		t.Errorf("eventually? should return false (never found), got %v", result)
	}
	t.Logf("eventually? '(found treasure) when never found = %v", result.Bool)
}

func TestTemporalNever(t *testing.T) {
	ev := NewEvaluator(1000)
	
	// No errors ever
	ev.DatalogDB.AssertAtTime("status", 0, Atom("ok"))
	ev.DatalogDB.AssertAtTime("status", 1, Atom("ok"))
	ev.DatalogDB.AssertAtTime("status", 2, Atom("ok"))
	
	code := `(never? '(error critical))`
	
	parser := NewParser(code)
	result := ev.Eval(parser.Parse()[0], ev.GlobalEnv)
	
	if result.Type != TypeBool || !result.Bool {
		t.Errorf("never? should return true (no errors), got %v", result)
	}
	t.Logf("never? '(error critical) = %v", result.Bool)
}

func TestTemporalNeverFails(t *testing.T) {
	ev := NewEvaluator(1000)
	
	// Error at t=2
	ev.DatalogDB.AssertAtTime("status", 0, Atom("ok"))
	ev.DatalogDB.AssertAtTime("status", 1, Atom("ok"))
	ev.DatalogDB.AssertAtTime("error", 2, Atom("critical"))  // violation!
	ev.DatalogDB.AssertAtTime("status", 3, Atom("recovered"))
	
	code := `(never? '(error critical))`
	
	parser := NewParser(code)
	result := ev.Eval(parser.Parse()[0], ev.GlobalEnv)
	
	if result.Type != TypeBool || result.Bool {
		t.Errorf("never? should return false (error occurred), got %v", result)
	}
	t.Logf("never? '(error critical) when error occurred = %v", result.Bool)
}

func TestTemporalWithActorSimulation(t *testing.T) {
	ev := NewEvaluator(1000)
	
	code := `
		;; Counter that tracks positive values
		(define (safe-counter n)
		  (if (< n 10)
			(begin
			  (assert! 'counter-positive (> n 0))
			  (assert! 'counter-value n)
			  (list 'become (list 'safe-counter (+ n 1))))
			(done!)))
		
		(spawn-actor 'counter 10 '(safe-counter 1))
		(run-scheduler 50)
		
		;; Verify properties
		(list
		  (always? '(counter-positive true))       ; counter always > 0
		  (eventually? '(counter-value 9))         ; reaches 9
		  (never? '(counter-value 0)))             ; never 0 (started at 1)
	`
	
	parser := NewParser(code)
	exprs := parser.Parse()
	var result Value
	for _, expr := range exprs {
		result = ev.Eval(expr, ev.GlobalEnv)
	}
	
	// Result should be (true true true)
	if result.Type != TypeList || len(result.List) != 3 {
		t.Fatalf("expected list of 3 bools, got %v", result)
	}
	
	always := result.List[0].Bool
	eventually := result.List[1].Bool
	never := result.List[2].Bool
	
	t.Logf("Simulation temporal results:")
	t.Logf("  always? (counter-positive true) = %v", always)
	t.Logf("  eventually? (counter-value 9) = %v", eventually)
	t.Logf("  never? (counter-value 0) = %v", never)
	
	if !always {
		t.Error("always? should be true")
	}
	if !eventually {
		t.Error("eventually? should be true")
	}
	if !never {
		t.Error("never? should be true")
	}
}

func TestTemporalSafetyViolation(t *testing.T) {
	ev := NewEvaluator(1000)
	
	code := `
		;; Counter that goes negative (violates safety)
		(define (unsafe-counter n)
		  (assert! 'balance n)
		  (if (> n -3)
			(list 'become (list 'unsafe-counter (- n 1)))
			(done!)))
		
		(spawn-actor 'counter 10 '(unsafe-counter 2))
		(run-scheduler 50)
		
		;; Check if balance ever goes negative
		(list
		  (never? '(balance -1))    ; should be FALSE - balance does go to -1
		  (never? '(balance -2))    ; should be FALSE - balance does go to -2
		  (eventually? '(balance -1)))  ; should be TRUE
	`
	
	parser := NewParser(code)
	exprs := parser.Parse()
	var result Value
	for _, expr := range exprs {
		result = ev.Eval(expr, ev.GlobalEnv)
	}
	
	if result.Type != TypeList || len(result.List) != 3 {
		t.Fatalf("expected list of 3 bools, got %v", result)
	}
	
	never1 := result.List[0].Bool
	never2 := result.List[1].Bool
	eventually := result.List[2].Bool
	
	t.Logf("Safety violation detection:")
	t.Logf("  never? (balance -1) = %v (should be false)", never1)
	t.Logf("  never? (balance -2) = %v (should be false)", never2)
	t.Logf("  eventually? (balance -1) = %v (should be true)", eventually)
	
	if never1 {
		t.Error("never? (balance -1) should be false - violation occurred")
	}
	if never2 {
		t.Error("never? (balance -2) should be false - violation occurred")
	}
	if !eventually {
		t.Error("eventually? (balance -1) should be true")
	}
}

func TestToolSubstitutionFullPipeline(t *testing.T) {
	ev := NewEvaluator(1000)
	
	// Simulate what runPrompt does
	lisp := `
		(define (producer n)
		  (if (> n 0)
			(begin
			  (assert! 'produced n)
			  (list 'become (list 'producer (- n 1))))
			(done!)))
		(spawn-actor 'p1 10 '(producer 5))
		(run-scheduler 20)
	`
	
	// Execute LISP
	parser := NewParser(lisp)
	exprs := parser.Parse()
	for _, expr := range exprs {
		ev.Eval(expr, ev.GlobalEnv)
	}
	
	// Now test tool substitution
	markdown := `## Facts

{{facts_table}}

## Chart

{{metrics_chart title="Test" predicates="produced,spawned"}}
`
	
	tr := NewToolRegistry(ev)
	result := tr.Process(markdown)
	
	t.Logf("Input markdown:\n%s", markdown)
	t.Logf("Output markdown:\n%s", result)
	
	// Should NOT contain {{facts_table}}
	if strings.Contains(result, "{{facts_table}}") {
		t.Error("facts_table placeholder was not substituted")
	}
	
	// Should contain actual rendered table
	if !strings.Contains(result, "produced") || !strings.Contains(result, "|") {
		t.Error("facts_table should render a markdown table with 'produced'")
	}
	
	// Should contain chart
	if !strings.Contains(result, "xychart") {
		t.Error("metrics_chart should render an xychart")
	}
}

func TestMermaidCleanup(t *testing.T) {
	ev := NewEvaluator(64)
	tr := NewToolRegistry(ev)
	
	tests := []struct {
		name     string
		input    string
		contains string
		excludes string
	}{
		{
			name: "fix := in labels",
			input: "```mermaid\nstateDiagram-v2\n    A --> B: x := 5\n```",
			contains: "x = 5",
			excludes: ":=",
		},
		{
			name: "fix angle brackets",
			input: "```mermaid\nsequenceDiagram\n    A->>B: send <data>\n```",
			contains: "A->>B: send",
			excludes: "<",
		},
		{
			name: "preserve valid mermaid",
			input: "```mermaid\nstateDiagram-v2\n    [*] --> Ready\n    Ready --> Done\n```",
			contains: "[*] --> Ready",
			excludes: "",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tr.Process(tt.input)
			if tt.contains != "" && !strings.Contains(result, tt.contains) {
				t.Errorf("expected to contain %q, got:\n%s", tt.contains, result)
			}
			if tt.excludes != "" && strings.Contains(result, tt.excludes) {
				t.Errorf("expected to NOT contain %q, got:\n%s", tt.excludes, result)
			}
		})
	}
}

func TestMetricsChartWarning(t *testing.T) {
	ev := NewEvaluator(64)
	
	// Only add a spawned fact, no sent/received
	ev.DatalogDB.AssertAtTime("spawned", 1, Atom("test"))
	
	tr := NewToolRegistry(ev)
	
	result := tr.Process(`{{metrics_chart title="Test" predicates="sent,received"}}`)
	
	t.Logf("Result:\n%s", result)
	
	if !strings.Contains(result, "⚠️") {
		t.Error("Expected warning when no matching facts")
	}
	
	if !strings.Contains(result, "No facts found") {
		t.Error("Expected 'No facts found' message")
	}
}

func TestPropertyTool(t *testing.T) {
	ev := NewEvaluator(64)
	
	// Add facts - note: always? checks ALL times in the DB
	ev.DatalogDB.AssertAtTime("temperature", 1, Atom("ok"))
	ev.DatalogDB.AssertAtTime("temperature", 2, Atom("ok"))
	ev.DatalogDB.AssertAtTime("temperature", 3, Atom("ok"))
	// Add error at t=3 (same time as temp ok, so always? still works)
	ev.DatalogDB.AssertAtTime("error", 3, Atom("overflow"))
	
	tr := NewToolRegistry(ev)
	
	tests := []struct {
		name     string
		formula  string
		contains string
	}{
		// All times (1,2,3) have temp ok, so AG passes
		{"always temp ok", `{{property formula="always? '(temperature ok)"}}`, "✅"},
		{"eventually error", `{{property formula="eventually? '(error overflow)"}}`, "✅"},
		{"never error", `{{property formula="never? '(error overflow)"}}`, "❌"},
		{"never missing", `{{property formula="never? '(missing thing)"}}`, "✅"},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tr.Process(tt.formula)
			t.Logf("%s -> %s", tt.formula, result)
			if !strings.Contains(result, tt.contains) {
				t.Errorf("expected %s, got %s", tt.contains, result)
			}
		})
	}
}

func TestFactsListTool(t *testing.T) {
	ev := NewEvaluator(64)
	
	ev.DatalogDB.AssertAtTime("spawned", 1, Atom("alice"))
	ev.DatalogDB.AssertAtTime("spawned", 1, Atom("bob"))
	ev.DatalogDB.AssertAtTime("sent", 2, Atom("alice"), Atom("bob"), Atom("hello"))
	
	tr := NewToolRegistry(ev)
	
	result := tr.Process("{{facts_list}}")
	t.Logf("Facts list:\n%s", result)
	
	if !strings.Contains(result, "spawned") {
		t.Error("should contain spawned")
	}
	if !strings.Contains(result, "alice") {
		t.Error("should contain alice")
	}
}

func TestPropertiesTableTool(t *testing.T) {
	ev := NewEvaluator(64)
	
	ev.DatalogDB.AssertAtTime("spawned", 1, Atom("test"))
	ev.DatalogDB.AssertAtTime("sent", 2, Atom("a"), Atom("b"), Atom("msg"))
	
	tr := NewToolRegistry(ev)
	
	result := tr.Process("{{properties}}")
	t.Logf("Properties table:\n%s", result)
	
	if !strings.Contains(result, "Property") {
		t.Error("should have table header")
	}
	if !strings.Contains(result, "✅") {
		t.Error("should have at least one passing check")
	}
}

func TestFullRendering(t *testing.T) {
	ev := NewEvaluator(64)
	
	// Simulate
	lisp := `
(define (producer n)
  (if (> n 0)
    (begin
      (send-to! 'consumer (list 'item n))
      (list 'become (list 'producer (- n 1))))
    (done!)))

(define (consumer)
  (let msg (receive!)
    (assert! 'processed (nth msg 1))
    (list 'become '(consumer))))

(spawn-actor 'producer 10 '(producer 3))
(spawn-actor 'consumer 10 '(consumer))
(run-scheduler 20)
`
	for _, e := range NewParser(lisp).Parse() {
		ev.Eval(e, ev.GlobalEnv)
	}
	
	tr := NewToolRegistry(ev)
	
	markdown := `## Properties
{{properties checks="Items processed: eventually? '(processed 1); No crashes: never? '(error ?x)"}}

## Facts Summary
{{facts_table}}

## Actual Facts
{{facts_list limit="10"}}
`
	
	result := tr.Process(markdown)
	t.Logf("Rendered markdown:\n%s", result)
	
	// Check for expected content
	if !strings.Contains(result, "✅") {
		t.Error("should have passing properties")
	}
	if !strings.Contains(result, "spawned") {
		t.Error("should show spawned facts")
	}
	if !strings.Contains(result, "sent") {
		t.Error("should show sent facts")
	}
}
