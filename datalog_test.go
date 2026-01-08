package main

import (
	"strings"
	"testing"
)

// ============================================================================
// Term Tests
// ============================================================================

func TestTermEquality(t *testing.T) {
	tests := []struct {
		name     string
		t1, t2   Term
		expected bool
	}{
		{"atoms equal", Atom("foo"), Atom("foo"), true},
		{"atoms not equal", Atom("foo"), Atom("bar"), false},
		{"numbers equal", NumTerm(42), NumTerm(42), true},
		{"numbers not equal", NumTerm(42), NumTerm(43), false},
		{"strings equal", StrTerm("hello"), StrTerm("hello"), true},
		{"strings not equal", StrTerm("hello"), StrTerm("world"), false},
		{"vars equal", Var("X"), Var("X"), true},
		{"vars not equal", Var("X"), Var("Y"), false},
		{"atom vs number", Atom("42"), NumTerm(42), false},
		{"list equal", ListTerm(Atom("a"), NumTerm(1)), ListTerm(Atom("a"), NumTerm(1)), true},
		{"list not equal length", ListTerm(Atom("a")), ListTerm(Atom("a"), Atom("b")), false},
		{"list not equal content", ListTerm(Atom("a")), ListTerm(Atom("b")), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.t1.Equal(tt.t2); got != tt.expected {
				t.Errorf("Equal() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestTermString(t *testing.T) {
	tests := []struct {
		term     Term
		expected string
	}{
		{Atom("foo"), "foo"},
		{Var("X"), "?X"},
		{NumTerm(42), "42"},
		{NumTerm(3.14), "3.14"},
		{StrTerm("hello"), `"hello"`},
		{ListTerm(Atom("a"), Atom("b")), "(a b)"},
	}

	for _, tt := range tests {
		if got := tt.term.String(); got != tt.expected {
			t.Errorf("String() = %v, want %v", got, tt.expected)
		}
	}
}

// ============================================================================
// Unification Tests
// ============================================================================

func TestUnifyAtoms(t *testing.T) {
	b := make(Binding)

	// Same atoms unify
	newB, ok := Unify(Atom("foo"), Atom("foo"), b)
	if !ok {
		t.Error("Same atoms should unify")
	}
	if len(newB) != 0 {
		t.Error("No bindings expected for atom unification")
	}

	// Different atoms don't unify
	_, ok = Unify(Atom("foo"), Atom("bar"), b)
	if ok {
		t.Error("Different atoms should not unify")
	}
}

func TestUnifyVariables(t *testing.T) {
	b := make(Binding)

	// Variable unifies with atom
	newB, ok := Unify(Var("X"), Atom("foo"), b)
	if !ok {
		t.Error("Variable should unify with atom")
	}
	if newB["X"].Name != "foo" {
		t.Errorf("Expected X=foo, got X=%v", newB["X"])
	}

	// Variable unifies with number
	newB, ok = Unify(Var("Y"), NumTerm(42), make(Binding))
	if !ok {
		t.Error("Variable should unify with number")
	}
	if !newB["Y"].IsNum || newB["Y"].Num != 42 {
		t.Errorf("Expected Y=42, got Y=%v", newB["Y"])
	}

	// Two variables unify
	newB, ok = Unify(Var("X"), Var("Y"), make(Binding))
	if !ok {
		t.Error("Two variables should unify")
	}
}

func TestUnifyWithBindings(t *testing.T) {
	b := Binding{"X": Atom("foo")}

	// Bound variable unifies with matching atom
	newB, ok := Unify(Var("X"), Atom("foo"), b)
	if !ok {
		t.Error("Bound variable should unify with matching value")
	}

	// Bound variable doesn't unify with different atom
	_, ok = Unify(Var("X"), Atom("bar"), b)
	if ok {
		t.Error("Bound variable should not unify with different value")
	}

	// New variable unifies and gets same binding
	newB, ok = Unify(Var("Y"), Var("X"), b)
	if !ok {
		t.Error("New variable should unify with bound variable")
	}
	deref := newB.Deref(Var("Y"))
	if deref.Name != "foo" {
		t.Errorf("Expected Y->foo, got %v", deref)
	}
}

func TestUnifyLists(t *testing.T) {
	b := make(Binding)

	// Same lists unify
	l1 := ListTerm(Atom("a"), NumTerm(1))
	l2 := ListTerm(Atom("a"), NumTerm(1))
	_, ok := Unify(l1, l2, b)
	if !ok {
		t.Error("Same lists should unify")
	}

	// Lists with variables
	l1 = ListTerm(Var("X"), NumTerm(1))
	l2 = ListTerm(Atom("a"), Var("Y"))
	newB, ok := Unify(l1, l2, b)
	if !ok {
		t.Error("Lists with variables should unify")
	}
	if newB["X"].Name != "a" {
		t.Errorf("Expected X=a, got %v", newB["X"])
	}
	if !newB["Y"].IsNum || newB["Y"].Num != 1 {
		t.Errorf("Expected Y=1, got %v", newB["Y"])
	}

	// Different length lists don't unify
	l1 = ListTerm(Atom("a"))
	l2 = ListTerm(Atom("a"), Atom("b"))
	_, ok = Unify(l1, l2, b)
	if ok {
		t.Error("Different length lists should not unify")
	}
}

// ============================================================================
// Fact Database Tests
// ============================================================================

func TestAssertAndQuery(t *testing.T) {
	db := NewDatalogDB()

	// Assert facts
	db.Assert("parent", Atom("tom"), Atom("bob"))
	db.Assert("parent", Atom("tom"), Atom("liz"))
	db.Assert("parent", Atom("bob"), Atom("ann"))

	// Query specific fact
	results := db.Query("parent", Atom("tom"), Atom("bob"))
	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}

	// Query with variable
	results = db.Query("parent", Atom("tom"), Var("X"))
	if len(results) != 2 {
		t.Errorf("Expected 2 results for tom's children, got %d", len(results))
	}

	// Query with two variables
	results = db.Query("parent", Var("P"), Var("C"))
	if len(results) != 3 {
		t.Errorf("Expected 3 parent-child pairs, got %d", len(results))
	}

	// Query non-existent fact
	results = db.Query("parent", Atom("bob"), Atom("tom"))
	if len(results) != 0 {
		t.Errorf("Expected 0 results, got %d", len(results))
	}
}

func TestRetract(t *testing.T) {
	db := NewDatalogDB()
	db.Assert("fact", Atom("a"))
	db.Assert("fact", Atom("b"))

	// Retract existing fact
	if !db.Retract("fact", Atom("a")) {
		t.Error("Retract should return true for existing fact")
	}

	results := db.Query("fact", Var("X"))
	if len(results) != 1 {
		t.Errorf("Expected 1 fact after retract, got %d", len(results))
	}

	// Retract non-existent fact
	if db.Retract("fact", Atom("c")) {
		t.Error("Retract should return false for non-existent fact")
	}
}

// ============================================================================
// Rule Tests
// ============================================================================

func TestSimpleRule(t *testing.T) {
	db := NewDatalogDB()

	// Facts
	db.Assert("parent", Atom("tom"), Atom("bob"))
	db.Assert("parent", Atom("bob"), Atom("ann"))

	// Rule: grandparent(X, Z) :- parent(X, Y), parent(Y, Z)
	db.AddRule("grandparent",
		Fact{Predicate: "grandparent", Args: []Term{Var("X"), Var("Z")}},
		Goal{Predicate: "parent", Args: []Term{Var("X"), Var("Y")}},
		Goal{Predicate: "parent", Args: []Term{Var("Y"), Var("Z")}},
	)

	results := db.Query("grandparent", Var("GP"), Var("GC"))
	if len(results) != 1 {
		t.Errorf("Expected 1 grandparent result, got %d", len(results))
	}
	if len(results) > 0 {
		gp := results[0].Deref(Var("GP"))
		gc := results[0].Deref(Var("GC"))
		if gp.Name != "tom" || gc.Name != "ann" {
			t.Errorf("Expected tom->ann, got %v->%v", gp, gc)
		}
	}
}

func TestTransitiveRule(t *testing.T) {
	db := NewDatalogDB()

	// Facts: a -> b -> c -> d
	db.Assert("edge", Atom("a"), Atom("b"))
	db.Assert("edge", Atom("b"), Atom("c"))
	db.Assert("edge", Atom("c"), Atom("d"))

	// Rule: path(X, Y) :- edge(X, Y)
	db.AddRule("path-base",
		Fact{Predicate: "path", Args: []Term{Var("X"), Var("Y")}},
		Goal{Predicate: "edge", Args: []Term{Var("X"), Var("Y")}},
	)

	// Rule: path(X, Z) :- edge(X, Y), path(Y, Z)
	db.AddRule("path-trans",
		Fact{Predicate: "path", Args: []Term{Var("X"), Var("Z")}},
		Goal{Predicate: "edge", Args: []Term{Var("X"), Var("Y")}},
		Goal{Predicate: "path", Args: []Term{Var("Y"), Var("Z")}},
	)

	// Should find paths: a->b, a->c, a->d, b->c, b->d, c->d
	results := db.Query("path", Var("From"), Var("To"))
	if len(results) != 6 {
		t.Errorf("Expected 6 paths, got %d", len(results))
	}

	// Specific query
	results = db.Query("path", Atom("a"), Atom("d"))
	if len(results) != 1 {
		t.Errorf("Expected path from a to d, got %d results", len(results))
	}
}

func TestNegation(t *testing.T) {
	db := NewDatalogDB()

	db.Assert("bird", Atom("tweety"))
	db.Assert("bird", Atom("penguin"))
	db.Assert("cannot-fly", Atom("penguin"))

	// Rule: can-fly(X) :- bird(X), not cannot-fly(X)
	db.AddRule("can-fly",
		Fact{Predicate: "can-fly", Args: []Term{Var("X")}},
		Goal{Predicate: "bird", Args: []Term{Var("X")}},
		Goal{Predicate: "cannot-fly", Args: []Term{Var("X")}, Negated: true},
	)

	results := db.Query("can-fly", Var("Y"))
	if len(results) != 1 {
		t.Errorf("Expected 1 flying bird, got %d", len(results))
	}
	if len(results) > 0 {
		// Deref to get the actual value
		val := results[0].Deref(Var("Y"))
		if val.IsVar {
			t.Errorf("Expected ground term, got variable %v", val)
		} else if val.Name != "tweety" {
			t.Errorf("Expected tweety, got %v", val)
		}
	}
}

// ============================================================================
// Builtin Tests
// ============================================================================

func TestBuiltinComparison(t *testing.T) {
	db := NewDatalogDB()

	db.Assert("value", Atom("x"), NumTerm(10))
	db.Assert("value", Atom("y"), NumTerm(20))
	db.Assert("value", Atom("z"), NumTerm(10))

	// Rule: larger(A, B) :- value(A, VA), value(B, VB), VA > VB
	db.AddRule("larger",
		Fact{Predicate: "larger", Args: []Term{Var("A"), Var("B")}},
		Goal{Predicate: "value", Args: []Term{Var("A"), Var("VA")}},
		Goal{Predicate: "value", Args: []Term{Var("B"), Var("VB")}},
		Goal{IsBuiltin: true, Builtin: ">", Args: []Term{Var("VA"), Var("VB")}},
	)

	results := db.Query("larger", Var("A"), Var("B"))
	// y > x, y > z
	if len(results) != 2 {
		t.Errorf("Expected 2 larger pairs, got %d", len(results))
	}

	// Rule: same-value(A, B) :- value(A, V), value(B, V), A != B
	db.AddRule("same-value",
		Fact{Predicate: "same-value", Args: []Term{Var("A"), Var("B")}},
		Goal{Predicate: "value", Args: []Term{Var("A"), Var("V")}},
		Goal{Predicate: "value", Args: []Term{Var("B"), Var("V")}},
		Goal{IsBuiltin: true, Builtin: "!=", Args: []Term{Var("A"), Var("B")}},
	)

	results = db.Query("same-value", Var("A"), Var("B"))
	// x and z have same value (both 10)
	if len(results) != 2 { // x-z and z-x
		t.Errorf("Expected 2 same-value pairs, got %d", len(results))
	}
}

// ============================================================================
// Temporal Tests
// ============================================================================

func TestTemporalAtTime(t *testing.T) {
	db := NewDatalogDB()

	db.AssertAtTime("event", 1, Atom("start"))
	db.AssertAtTime("event", 5, Atom("middle"))
	db.AssertAtTime("event", 10, Atom("end"))

	// Query events at specific time
	results := db.QueryGoals(Goal{
		Predicate: "at-time",
		Args:      []Term{Atom("event"), Var("E"), NumTerm(5)},
	})
	if len(results) != 1 {
		t.Errorf("Expected 1 event at time 5, got %d", len(results))
	}
	if len(results) > 0 && results[0]["E"].Name != "middle" {
		t.Errorf("Expected 'middle', got %v", results[0]["E"])
	}

	// Query time of specific event
	results = db.QueryGoals(Goal{
		Predicate: "at-time",
		Args:      []Term{Atom("event"), Atom("end"), Var("T")},
	})
	if len(results) != 1 {
		t.Errorf("Expected 1 result for 'end' event, got %d", len(results))
	}
	if len(results) > 0 && results[0]["T"].Num != 10 {
		t.Errorf("Expected time 10, got %v", results[0]["T"])
	}
}

func TestTemporalBefore(t *testing.T) {
	db := NewDatalogDB()

	db.AssertAtTime("event", 1, Atom("a"))
	db.AssertAtTime("event", 5, Atom("b"))
	db.AssertAtTime("event", 10, Atom("c"))

	// Events before time 6
	results := db.QueryGoals(Goal{
		Predicate: "before",
		Args:      []Term{Atom("event"), Var("E"), NumTerm(6)},
	})
	if len(results) != 2 {
		t.Errorf("Expected 2 events before time 6, got %d", len(results))
	}
}

func TestTemporalAfter(t *testing.T) {
	db := NewDatalogDB()

	db.AssertAtTime("event", 1, Atom("a"))
	db.AssertAtTime("event", 5, Atom("b"))
	db.AssertAtTime("event", 10, Atom("c"))

	// Events after time 4
	results := db.QueryGoals(Goal{
		Predicate: "after",
		Args:      []Term{Atom("event"), Var("E"), NumTerm(4)},
	})
	if len(results) != 2 {
		t.Errorf("Expected 2 events after time 4, got %d", len(results))
	}
}

func TestTemporalBetween(t *testing.T) {
	db := NewDatalogDB()

	db.AssertAtTime("event", 1, Atom("a"))
	db.AssertAtTime("event", 5, Atom("b"))
	db.AssertAtTime("event", 10, Atom("c"))

	// Events between time 2 and 8
	results := db.QueryGoals(Goal{
		Predicate: "between",
		Args:      []Term{Atom("event"), Var("E"), NumTerm(2), NumTerm(8)},
	})
	if len(results) != 1 {
		t.Errorf("Expected 1 event between 2 and 8, got %d", len(results))
	}
	if len(results) > 0 && results[0]["E"].Name != "b" {
		t.Errorf("Expected 'b', got %v", results[0]["E"])
	}
}

// ============================================================================
// CTL Operator Tests
// ============================================================================

func TestAlways(t *testing.T) {
	db := NewDatalogDB()

	// Invariant holds at all times
	db.AssertAtTime("valid", 1, Atom("ok"))
	db.AssertAtTime("valid", 2, Atom("ok"))
	db.AssertAtTime("valid", 3, Atom("ok"))

	// Simple check: valid(ok) exists
	if !db.Eventually(Goal{Predicate: "valid", Args: []Term{Atom("ok")}}) {
		t.Error("Eventually should find valid(ok)")
	}

	// Never has invalid state
	if !db.Never(Goal{Predicate: "valid", Args: []Term{Atom("bad")}}) {
		t.Error("Never should be true for non-existent valid(bad)")
	}

	// Add a bad state
	db.AssertAtTime("valid", 4, Atom("bad"))
	
	// Now bad exists
	if db.Never(Goal{Predicate: "valid", Args: []Term{Atom("bad")}}) {
		t.Error("Never should be false when bad fact exists")
	}
}

func TestEventually(t *testing.T) {
	db := NewDatalogDB()

	db.Assert("state", Atom("init"))

	if !db.Eventually(Goal{Predicate: "state", Args: []Term{Atom("init")}}) {
		t.Error("Eventually should find existing fact")
	}

	if db.Eventually(Goal{Predicate: "state", Args: []Term{Atom("done")}}) {
		t.Error("Eventually should not find non-existent fact")
	}
}

func TestNever(t *testing.T) {
	db := NewDatalogDB()

	db.Assert("good", Atom("thing"))

	if !db.Never(Goal{Predicate: "bad", Args: []Term{Var("X")}}) {
		t.Error("Never should be true for non-existent predicate")
	}

	db.Assert("bad", Atom("error"))
	if db.Never(Goal{Predicate: "bad", Args: []Term{Var("X")}}) {
		t.Error("Never should be false when bad fact exists")
	}
}

// ============================================================================
// CSP-Specific Tests
// ============================================================================

func TestCSPViolationDetection(t *testing.T) {
	db := NewDatalogDB()

	// Simulate actor execution trace
	// Good pattern: guard -> effect
	db.AssertAtTime("guard", 1, Atom("actor1"), Atom("receive"))
	db.AssertAtTime("effect", 2, Atom("actor1"), Atom("set"), Atom("x"))

	// Bad pattern: effect before guard
	db.AssertAtTime("effect", 10, Atom("actor2"), Atom("set"), Atom("y"))
	db.AssertAtTime("guard", 11, Atom("actor2"), Atom("receive"))

	// Rule: csp-violation(Actor) :- effect(Actor, _, _), not preceded-by-guard(Actor)
	// Simplified: check if there's an effect at time T with no guard at time < T

	db.AddRule("has-guard-before",
		Fact{Predicate: "has-guard-before", Args: []Term{Var("Actor"), Var("T")}},
		Goal{Predicate: "guard", Args: []Term{Var("Actor"), Var("_")}},
	)

	// Check actor1 has guard
	results := db.Query("guard", Atom("actor1"), Var("Type"))
	if len(results) != 1 {
		t.Errorf("Expected guard for actor1, got %d", len(results))
	}

	// Check actor2 has effect before guard
	effectResults := db.QueryGoals(Goal{
		Predicate: "before",
		Args:      []Term{Atom("effect"), Atom("actor2"), Var("Op"), Var("Var"), NumTerm(11)},
	})
	if len(effectResults) != 1 {
		t.Errorf("Expected effect before guard for actor2, got %d", len(effectResults))
	}
}

func TestMessageFlowTracking(t *testing.T) {
	db := NewDatalogDB()

	// Simulate message sends
	db.AssertAtTime("sent", 1, Atom("alice"), Atom("bob"), Atom("hello"))
	db.AssertAtTime("sent", 2, Atom("bob"), Atom("alice"), Atom("hi"))
	db.AssertAtTime("sent", 3, Atom("alice"), Atom("charlie"), Atom("request"))

	// Rule: communicated(A, B) :- sent(A, B, _)
	db.AddRule("communicated",
		Fact{Predicate: "communicated", Args: []Term{Var("A"), Var("B")}},
		Goal{Predicate: "sent", Args: []Term{Var("A"), Var("B"), Var("_")}},
	)

	// Rule: bidirectional(A, B) :- communicated(A, B), communicated(B, A)
	db.AddRule("bidirectional",
		Fact{Predicate: "bidirectional", Args: []Term{Var("A"), Var("B")}},
		Goal{Predicate: "communicated", Args: []Term{Var("A"), Var("B")}},
		Goal{Predicate: "communicated", Args: []Term{Var("B"), Var("A")}},
	)

	results := db.Query("bidirectional", Var("X"), Var("Y"))
	if len(results) != 2 { // alice-bob and bob-alice
		t.Errorf("Expected 2 bidirectional pairs, got %d", len(results))
	}
}

func TestDeadlockDetection(t *testing.T) {
	db := NewDatalogDB()

	// Simulate waiting states
	db.Assert("waiting-for", Atom("actor1"), Atom("actor2"))
	db.Assert("waiting-for", Atom("actor2"), Atom("actor1"))

	// Rule: deadlock(A, B) :- waiting-for(A, B), waiting-for(B, A)
	db.AddRule("deadlock",
		Fact{Predicate: "deadlock", Args: []Term{Var("A"), Var("B")}},
		Goal{Predicate: "waiting-for", Args: []Term{Var("A"), Var("B")}},
		Goal{Predicate: "waiting-for", Args: []Term{Var("B"), Var("A")}},
	)

	results := db.Query("deadlock", Var("A"), Var("B"))
	if len(results) != 2 { // symmetric
		t.Errorf("Expected deadlock detected, got %d results", len(results))
	}

	// No deadlock case
	db.Clear()
	db.Assert("waiting-for", Atom("actor1"), Atom("actor2"))
	db.Assert("waiting-for", Atom("actor2"), Atom("actor3"))

	db.AddRule("deadlock",
		Fact{Predicate: "deadlock", Args: []Term{Var("A"), Var("B")}},
		Goal{Predicate: "waiting-for", Args: []Term{Var("A"), Var("B")}},
		Goal{Predicate: "waiting-for", Args: []Term{Var("B"), Var("A")}},
	)

	results = db.Query("deadlock", Var("A"), Var("B"))
	if len(results) != 0 {
		t.Errorf("Expected no deadlock, got %d results", len(results))
	}
}

// ============================================================================
// LISP Integration Tests
// ============================================================================

func TestValueToTerm(t *testing.T) {
	tests := []struct {
		value    Value
		expected Term
	}{
		{Num(42), NumTerm(42)},
		{Str("hello"), StrTerm("hello")},
		{Sym("foo"), Atom("foo")},
		{Sym("?X"), Var("X")},
		{Bool(true), Atom("true")},
		{Bool(false), Atom("false")},
	}

	for _, tt := range tests {
		got := ValueToTerm(tt.value)
		if !got.Equal(tt.expected) {
			t.Errorf("ValueToTerm(%v) = %v, want %v", tt.value, got, tt.expected)
		}
	}
}

func TestTermToValue(t *testing.T) {
	tests := []struct {
		term     Term
		expected Value
	}{
		{NumTerm(42), Num(42)},
		{StrTerm("hello"), Str("hello")},
		{Atom("foo"), Sym("foo")},
		{Var("X"), Sym("?X")},
	}

	for _, tt := range tests {
		got := TermToValue(tt.term)
		if got.String() != tt.expected.String() {
			t.Errorf("TermToValue(%v) = %v, want %v", tt.term, got, tt.expected)
		}
	}
}

// ============================================================================
// Complex Integration Test
// ============================================================================

func TestBreadCoSimulation(t *testing.T) {
	db := NewDatalogDB()

	// Simulate BreadCo message trace
	db.AssertAtTime("sent", 1, Atom("production"), Atom("trucks"), ListTerm(Atom("bread"), NumTerm(10)))
	db.AssertAtTime("received", 2, Atom("trucks"), ListTerm(Atom("bread"), NumTerm(10)))
	db.AssertAtTime("sent", 3, Atom("trucks"), Atom("storefront"), ListTerm(Atom("delivery"), NumTerm(10)))
	db.AssertAtTime("received", 4, Atom("storefront"), ListTerm(Atom("delivery"), NumTerm(10)))
	db.AssertAtTime("state-change", 4, Atom("storefront"), Atom("inventory"), NumTerm(0), NumTerm(10))

	// Customers
	db.AssertAtTime("sent", 5, Atom("customer-alice"), Atom("storefront"), ListTerm(Atom("buy"), NumTerm(3)))
	db.AssertAtTime("received", 6, Atom("storefront"), ListTerm(Atom("buy"), NumTerm(3)))
	db.AssertAtTime("sent", 7, Atom("storefront"), Atom("customer-alice"), ListTerm(Atom("purchase"), NumTerm(3)))
	db.AssertAtTime("state-change", 7, Atom("storefront"), Atom("inventory"), NumTerm(10), NumTerm(7))

	// Rule: inventory-decreased(From, To) :- state-change(_, inventory, From, To), From > To
	db.AddRule("inventory-decreased",
		Fact{Predicate: "inventory-decreased", Args: []Term{Var("From"), Var("To")}},
		Goal{Predicate: "state-change", Args: []Term{Var("_"), Atom("inventory"), Var("From"), Var("To")}},
		Goal{IsBuiltin: true, Builtin: ">", Args: []Term{Var("From"), Var("To")}},
	)

	results := db.Query("inventory-decreased", Var("F"), Var("T"))
	if len(results) != 1 {
		t.Errorf("Expected 1 inventory decrease, got %d", len(results))
	}

	// Rule: message-delivered(From, To, Msg) :- sent(From, To, Msg), received(To, Msg)
	db.AddRule("message-delivered",
		Fact{Predicate: "message-delivered", Args: []Term{Var("From"), Var("To"), Var("Msg")}},
		Goal{Predicate: "sent", Args: []Term{Var("From"), Var("To"), Var("Msg")}},
		Goal{Predicate: "received", Args: []Term{Var("To"), Var("Msg")}},
	)

	results = db.Query("message-delivered", Var("F"), Var("T"), Var("M"))
	if len(results) < 2 {
		t.Errorf("Expected at least 2 delivered messages, got %d", len(results))
	}
}

// ============================================================================
// Benchmark
// ============================================================================

func BenchmarkQuery(b *testing.B) {
	db := NewDatalogDB()

	// Create 1000 facts
	for i := 0; i < 1000; i++ {
		db.Assert("item", NumTerm(float64(i)), Atom("value"))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		db.Query("item", Var("X"), Atom("value"))
	}
}

func BenchmarkTransitiveQuery(b *testing.B) {
	db := NewDatalogDB()

	// Create a chain of 100 edges
	for i := 0; i < 100; i++ {
		db.Assert("edge", NumTerm(float64(i)), NumTerm(float64(i+1)))
	}

	db.AddRule("path-base",
		Fact{Predicate: "path", Args: []Term{Var("X"), Var("Y")}},
		Goal{Predicate: "edge", Args: []Term{Var("X"), Var("Y")}},
	)
	db.AddRule("path-trans",
		Fact{Predicate: "path", Args: []Term{Var("X"), Var("Z")}},
		Goal{Predicate: "edge", Args: []Term{Var("X"), Var("Y")}},
		Goal{Predicate: "path", Args: []Term{Var("Y"), Var("Z")}},
	)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		db.Query("path", NumTerm(0), NumTerm(10))
	}
}

func TestCleanLispSection(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "clean code stays clean",
			input: `(define (foo x)
  (+ x 1))`,
			expected: `(define (foo x)
  (+ x 1))`,
		},
		{
			name: "strips markdown headers",
			input: `### Gateway Actor
(define (gateway-loop) ...)`,
			expected: `(define (gateway-loop) ...)`,
		},
		{
			name: "strips code fences",
			input: "```lisp\n(define (foo) ...)\n```",
			expected: `(define (foo) ...)`,
		},
		{
			name: "strips prose lines",
			input: `Here is the code:
(define (foo) ...)
This implements the actor.`,
			expected: `(define (foo) ...)`,
		},
		{
			name: "keeps LISP comments",
			input: `; This is a LISP comment
(define (foo) ...)`,
			expected: `; This is a LISP comment
(define (foo) ...)`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cleanLispSection(tt.input)
			if got != tt.expected {
				t.Errorf("cleanLispSection() =\n%q\nwant\n%q", got, tt.expected)
			}
		})
	}
}

func TestListFacts(t *testing.T) {
	ev := NewEvaluator(1000)
	
	// Assert some facts via LISP
	exprs := NewParser(`
		(assert! 'sale 'store1 100)
		(assert! 'sale 'store2 200)
		(assert! 'inventory 'store1 50)
	`).Parse()
	for _, expr := range exprs {
		ev.Eval(expr, ev.GlobalEnv)
	}
	
	// List all facts
	result := ev.Eval(NewParser(`(list-facts)`).Parse()[0], ev.GlobalEnv)
	if result.Type != TypeList || len(result.List) != 3 {
		t.Errorf("expected 3 facts, got %d: %v", len(result.List), result)
	}
	
	// List filtered by predicate
	result = ev.Eval(NewParser(`(list-facts 'sale)`).Parse()[0], ev.GlobalEnv)
	if result.Type != TypeList || len(result.List) != 2 {
		t.Errorf("expected 2 sale facts, got %d: %v", len(result.List), result)
	}
}

func TestFactCount(t *testing.T) {
	ev := NewEvaluator(1000)
	
	// Assert some facts via LISP
	exprs := NewParser(`
		(assert! 'sale 'store1 100)
		(assert! 'sale 'store2 200)
		(assert! 'inventory 'store1 50)
	`).Parse()
	for _, expr := range exprs {
		ev.Eval(expr, ev.GlobalEnv)
	}
	
	// Count all
	result := ev.Eval(NewParser(`(fact-count)`).Parse()[0], ev.GlobalEnv)
	if result.Number != 3 {
		t.Errorf("expected 3 total facts, got %v", result.Number)
	}
	
	// Count by predicate
	result = ev.Eval(NewParser(`(fact-count 'sale)`).Parse()[0], ev.GlobalEnv)
	if result.Number != 2 {
		t.Errorf("expected 2 sale facts, got %v", result.Number)
	}
	
	result = ev.Eval(NewParser(`(fact-count 'inventory)`).Parse()[0], ev.GlobalEnv)
	if result.Number != 1 {
		t.Errorf("expected 1 inventory fact, got %v", result.Number)
	}
}

func TestToolSubstitution(t *testing.T) {
	ev := NewEvaluator(1000)
	
	// Assert some facts via LISP
	exprs := NewParser(`
		(assert! 'sale 'store1 100)
		(assert! 'sale 'store2 200)
	`).Parse()
	for _, expr := range exprs {
		ev.Eval(expr, ev.GlobalEnv)
	}
	
	tr := NewToolRegistry(ev)
	
	// Test facts_table substitution
	input := "Here are the sales:\n\n{{facts_table predicate=\"sale\"}}"
	output := tr.Process(input)
	
	if !strings.Contains(output, "sale") {
		t.Errorf("expected output to contain facts table, got: %s", output)
	}
	if strings.Contains(output, "{{") {
		t.Errorf("expected placeholders to be replaced, got: %s", output)
	}
	
	// Test facts summary (no predicate)
	input2 := "{{facts_table}}"
	output2 := tr.Process(input2)
	
	if !strings.Contains(output2, "sale") || !strings.Contains(output2, "2") {
		t.Errorf("expected summary with sale count, got: %s", output2)
	}
}

func TestLispExecutionPopulatesFacts(t *testing.T) {
	ev := NewEvaluator(1000)
	
	// Simulate what happens when LISP section is executed
	lisp := `
		(assert! 'sale 'store1 100)
		(assert! 'sale 'store2 200)
		(assert! 'inventory 'store1 50)
	`
	
	parser := NewParser(lisp)
	exprs := parser.Parse()
	for _, expr := range exprs {
		ev.Eval(expr, ev.GlobalEnv)
	}
	
	// Now facts should exist
	if len(ev.DatalogDB.Facts) != 3 {
		t.Errorf("expected 3 facts after LISP execution, got %d", len(ev.DatalogDB.Facts))
	}
	
	// Tool substitution should now show facts
	tr := NewToolRegistry(ev)
	output := tr.Process("{{facts_table}}")
	
	if strings.Contains(output, "No facts") || strings.Contains(output, "*No facts") {
		t.Errorf("expected facts in output, got: %s", output)
	}
	
	if !strings.Contains(output, "sale") {
		t.Errorf("expected 'sale' in facts summary, got: %s", output)
	}
}
