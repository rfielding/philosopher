// ============================================================================
// Datalog Interpreter for BoundedLISP
// ============================================================================
//
// A minimal Datalog (subset of Prolog) embedded in BoundedLISP for:
//   - Collecting facts during simulation
//   - Defining rules/relationships
//   - Querying with unification
//   - Temporal reasoning over traces
//
// Datalog restrictions (guarantees termination):
//   - No function symbols in terms (only atoms, variables, numbers, strings)
//   - All variables in rule head must appear in body
//   - Stratified negation only

package main

import (
	"fmt"
	"strings"
)

// ============================================================================
// Datalog Types
// ============================================================================

// Term represents a Datalog term: variable, atom, number, or string
type Term struct {
	IsVar  bool
	Name   string  // variable name (if IsVar) or atom name
	Num    float64 // numeric value (if numeric term)
	Str    string  // string value (if string term)
	IsNum  bool
	IsStr  bool
	IsList bool
	List   []Term // compound term (for lists)
}

// Fact is a ground (no variables) predicate
type Fact struct {
	Predicate string
	Args      []Term
	Time      int64 // timestamp for temporal queries
}

// Rule is a Horn clause: head :- body
type Rule struct {
	Head      Fact   // may contain variables
	Body      []Goal // conjunction of goals
	Name      string // optional rule name
}

// Goal is a single goal in a rule body
type Goal struct {
	Predicate string
	Args      []Term
	Negated   bool   // for negation-as-failure
	IsBuiltin bool   // for built-in predicates like >, <, =
	Builtin   string // builtin operator
}

// Binding maps variables to terms
type Binding map[string]Term

// DatalogDB holds all facts and rules
type DatalogDB struct {
	Facts    []Fact
	Rules    []Rule
	TimeNow  int64 // current simulation time
	AutoTime bool  // auto-timestamp facts
}

// ============================================================================
// Term Construction
// ============================================================================

func Var(name string) Term {
	return Term{IsVar: true, Name: name}
}

func Atom(name string) Term {
	return Term{Name: name}
}

func NumTerm(n float64) Term {
	return Term{IsNum: true, Num: n}
}

func StrTerm(s string) Term {
	return Term{IsStr: true, Str: s}
}

func ListTerm(terms ...Term) Term {
	return Term{IsList: true, List: terms}
}

func (t Term) String() string {
	if t.IsVar {
		return "?" + t.Name
	}
	if t.IsNum {
		if t.Num == float64(int64(t.Num)) {
			return fmt.Sprintf("%d", int64(t.Num))
		}
		return fmt.Sprintf("%g", t.Num)
	}
	if t.IsStr {
		return fmt.Sprintf("%q", t.Str)
	}
	if t.IsList {
		parts := make([]string, len(t.List))
		for i, x := range t.List {
			parts[i] = x.String()
		}
		return "(" + strings.Join(parts, " ") + ")"
	}
	return t.Name
}

func (t Term) Equal(other Term) bool {
	if t.IsVar != other.IsVar {
		return false
	}
	if t.IsVar {
		return t.Name == other.Name
	}
	if t.IsNum != other.IsNum {
		return false
	}
	if t.IsNum {
		return t.Num == other.Num
	}
	if t.IsStr != other.IsStr {
		return false
	}
	if t.IsStr {
		return t.Str == other.Str
	}
	if t.IsList != other.IsList {
		return false
	}
	if t.IsList {
		if len(t.List) != len(other.List) {
			return false
		}
		for i := range t.List {
			if !t.List[i].Equal(other.List[i]) {
				return false
			}
		}
		return true
	}
	return t.Name == other.Name
}

// ============================================================================
// Unification
// ============================================================================

func (b Binding) Copy() Binding {
	newB := make(Binding)
	for k, v := range b {
		newB[k] = v
	}
	return newB
}

// Deref follows variable bindings to get the actual term
func (b Binding) Deref(t Term) Term {
	if !t.IsVar {
		if t.IsList {
			// Deref list elements
			newList := make([]Term, len(t.List))
			for i, elem := range t.List {
				newList[i] = b.Deref(elem)
			}
			return ListTerm(newList...)
		}
		return t
	}
	if bound, ok := b[t.Name]; ok {
		return b.Deref(bound)
	}
	return t
}

// Unify attempts to unify two terms, extending bindings
func Unify(t1, t2 Term, b Binding) (Binding, bool) {
	t1 = b.Deref(t1)
	t2 = b.Deref(t2)

	// Both variables
	if t1.IsVar && t2.IsVar {
		if t1.Name == t2.Name {
			return b, true
		}
		newB := b.Copy()
		newB[t1.Name] = t2
		return newB, true
	}

	// One variable
	if t1.IsVar {
		newB := b.Copy()
		newB[t1.Name] = t2
		return newB, true
	}
	if t2.IsVar {
		newB := b.Copy()
		newB[t2.Name] = t1
		return newB, true
	}

	// Both lists
	if t1.IsList && t2.IsList {
		if len(t1.List) != len(t2.List) {
			return nil, false
		}
		currentB := b
		var ok bool
		for i := range t1.List {
			currentB, ok = Unify(t1.List[i], t2.List[i], currentB)
			if !ok {
				return nil, false
			}
		}
		return currentB, true
	}

	// Both ground
	if t1.Equal(t2) {
		return b, true
	}

	return nil, false
}

// UnifyArgs unifies two argument lists
func UnifyArgs(args1, args2 []Term, b Binding) (Binding, bool) {
	if len(args1) != len(args2) {
		return nil, false
	}
	currentB := b
	var ok bool
	for i := range args1 {
		currentB, ok = Unify(args1[i], args2[i], currentB)
		if !ok {
			return nil, false
		}
	}
	return currentB, true
}

// ============================================================================
// Database Operations
// ============================================================================

func NewDatalogDB() *DatalogDB {
	return &DatalogDB{
		Facts:    make([]Fact, 0),
		Rules:    make([]Rule, 0),
		AutoTime: true,
	}
}

func (db *DatalogDB) Assert(pred string, args ...Term) {
	fact := Fact{
		Predicate: pred,
		Args:      args,
		Time:      db.TimeNow,
	}
	db.Facts = append(db.Facts, fact)
}

func (db *DatalogDB) AssertAtTime(pred string, time int64, args ...Term) {
	fact := Fact{
		Predicate: pred,
		Args:      args,
		Time:      time,
	}
	db.Facts = append(db.Facts, fact)
}

func (db *DatalogDB) Retract(pred string, args ...Term) bool {
	for i := len(db.Facts) - 1; i >= 0; i-- {
		f := db.Facts[i]
		if f.Predicate == pred && len(f.Args) == len(args) {
			match := true
			for j := range args {
				if !args[j].Equal(f.Args[j]) {
					match = false
					break
				}
			}
			if match {
				db.Facts = append(db.Facts[:i], db.Facts[i+1:]...)
				return true
			}
		}
	}
	return false
}

func (db *DatalogDB) AddRule(name string, head Fact, body ...Goal) {
	db.Rules = append(db.Rules, Rule{
		Name: name,
		Head: head,
		Body: body,
	})
}

func (db *DatalogDB) ClearFacts() {
	db.Facts = make([]Fact, 0)
}

func (db *DatalogDB) ClearRules() {
	db.Rules = make([]Rule, 0)
}

func (db *DatalogDB) Clear() {
	db.ClearFacts()
	db.ClearRules()
}

// ============================================================================
// Query Engine
// ============================================================================

// QueryResult is one solution to a query
type QueryResult struct {
	Bindings Binding
	Success  bool
}

// Query executes a query and returns all solutions
func (db *DatalogDB) Query(pred string, args ...Term) []Binding {
	goal := Goal{Predicate: pred, Args: args}
	return db.solve([]Goal{goal}, make(Binding), 0)
}

// QueryGoals executes a conjunction of goals
func (db *DatalogDB) QueryGoals(goals ...Goal) []Binding {
	return db.solve(goals, make(Binding), 0)
}

const maxDepth = 100 // prevent infinite recursion

func (db *DatalogDB) solve(goals []Goal, bindings Binding, depth int) []Binding {
	if depth > maxDepth {
		return nil
	}

	if len(goals) == 0 {
		return []Binding{bindings}
	}

	goal := goals[0]
	rest := goals[1:]
	var results []Binding

	// Handle negation
	if goal.Negated {
		positiveGoal := goal
		positiveGoal.Negated = false
		solutions := db.solve([]Goal{positiveGoal}, bindings, depth+1)
		if len(solutions) == 0 {
			// Negation succeeds
			results = append(results, db.solve(rest, bindings, depth+1)...)
		}
		return results
	}

	// Handle builtins
	if goal.IsBuiltin {
		if db.evalBuiltin(goal, bindings) {
			results = append(results, db.solve(rest, bindings, depth+1)...)
		}
		return results
	}

	// Handle temporal predicates
	switch goal.Predicate {
	case "at-time":
		return db.solveAtTime(goal, rest, bindings, depth)
	case "before":
		return db.solveBefore(goal, rest, bindings, depth)
	case "after":
		return db.solveAfter(goal, rest, bindings, depth)
	case "between":
		return db.solveBetween(goal, rest, bindings, depth)
	}

	// Match against facts
	for _, fact := range db.Facts {
		if fact.Predicate == goal.Predicate {
			if newB, ok := UnifyArgs(goal.Args, fact.Args, bindings); ok {
				results = append(results, db.solve(rest, newB, depth+1)...)
			}
		}
	}

	// Match against rules
	for _, rule := range db.Rules {
		if rule.Head.Predicate == goal.Predicate {
			// Rename variables to avoid conflicts
			renamedRule := db.renameVars(rule, depth)
			if newB, ok := UnifyArgs(goal.Args, renamedRule.Head.Args, bindings); ok {
				// Solve body with new bindings
				bodyResults := db.solve(renamedRule.Body, newB, depth+1)
				for _, bodyB := range bodyResults {
					results = append(results, db.solve(rest, bodyB, depth+1)...)
				}
			}
		}
	}

	return results
}

// renameVars creates fresh variable names to avoid capture
func (db *DatalogDB) renameVars(rule Rule, depth int) Rule {
	suffix := fmt.Sprintf("_%d", depth)
	varMap := make(map[string]string)

	renameTerm := func(t Term) Term {
		if t.IsVar {
			if newName, ok := varMap[t.Name]; ok {
				return Var(newName)
			}
			newName := t.Name + suffix
			varMap[t.Name] = newName
			return Var(newName)
		}
		if t.IsList {
			newList := make([]Term, len(t.List))
			for i, elem := range t.List {
				if elem.IsVar {
					if newName, ok := varMap[elem.Name]; ok {
						newList[i] = Var(newName)
					} else {
						newName := elem.Name + suffix
						varMap[elem.Name] = newName
						newList[i] = Var(newName)
					}
				} else {
					newList[i] = elem
				}
			}
			return ListTerm(newList...)
		}
		return t
	}

	newHead := Fact{
		Predicate: rule.Head.Predicate,
		Args:      make([]Term, len(rule.Head.Args)),
	}
	for i, arg := range rule.Head.Args {
		newHead.Args[i] = renameTerm(arg)
	}

	newBody := make([]Goal, len(rule.Body))
	for i, g := range rule.Body {
		newBody[i] = Goal{
			Predicate: g.Predicate,
			Args:      make([]Term, len(g.Args)),
			Negated:   g.Negated,
			IsBuiltin: g.IsBuiltin,
			Builtin:   g.Builtin,
		}
		for j, arg := range g.Args {
			newBody[i].Args[j] = renameTerm(arg)
		}
	}

	return Rule{Head: newHead, Body: newBody, Name: rule.Name}
}

// ============================================================================
// Builtin Predicates
// ============================================================================

func (db *DatalogDB) evalBuiltin(goal Goal, b Binding) bool {
	if len(goal.Args) < 2 {
		return false
	}

	left := b.Deref(goal.Args[0])
	right := b.Deref(goal.Args[1])

	// Both must be ground for comparison
	if left.IsVar || right.IsVar {
		return false
	}

	switch goal.Builtin {
	case "=":
		return left.Equal(right)
	case "!=", "<>":
		return !left.Equal(right)
	case ">":
		if left.IsNum && right.IsNum {
			return left.Num > right.Num
		}
	case "<":
		if left.IsNum && right.IsNum {
			return left.Num < right.Num
		}
	case ">=":
		if left.IsNum && right.IsNum {
			return left.Num >= right.Num
		}
	case "<=":
		if left.IsNum && right.IsNum {
			return left.Num <= right.Num
		}
	}
	return false
}

// ============================================================================
// Temporal Queries
// ============================================================================

func (db *DatalogDB) solveAtTime(goal Goal, rest []Goal, bindings Binding, depth int) []Binding {
	// at-time(Pred, Args..., Time)
	if len(goal.Args) < 2 {
		return nil
	}

	predTerm := bindings.Deref(goal.Args[0])
	timeTerm := bindings.Deref(goal.Args[len(goal.Args)-1])
	queryArgs := goal.Args[1 : len(goal.Args)-1]

	var results []Binding

	for _, fact := range db.Facts {
		if predTerm.IsVar || fact.Predicate == predTerm.Name {
			if newB, ok := UnifyArgs(queryArgs, fact.Args, bindings); ok {
				// Unify time
				factTime := NumTerm(float64(fact.Time))
				if timeB, ok := Unify(timeTerm, factTime, newB); ok {
					// Also bind predicate if it was a variable
					if predTerm.IsVar {
						timeB[predTerm.Name] = Atom(fact.Predicate)
					}
					results = append(results, db.solve(rest, timeB, depth+1)...)
				}
			}
		}
	}
	return results
}

func (db *DatalogDB) solveBefore(goal Goal, rest []Goal, bindings Binding, depth int) []Binding {
	// before(Pred, Args..., Time) - fact occurred before Time
	if len(goal.Args) < 2 {
		return nil
	}

	predTerm := bindings.Deref(goal.Args[0])
	timeTerm := bindings.Deref(goal.Args[len(goal.Args)-1])
	queryArgs := goal.Args[1 : len(goal.Args)-1]

	if timeTerm.IsVar || !timeTerm.IsNum {
		return nil
	}
	maxTime := int64(timeTerm.Num)

	var results []Binding

	for _, fact := range db.Facts {
		if fact.Time < maxTime {
			if predTerm.IsVar || fact.Predicate == predTerm.Name {
				if newB, ok := UnifyArgs(queryArgs, fact.Args, bindings); ok {
					if predTerm.IsVar {
						newB[predTerm.Name] = Atom(fact.Predicate)
					}
					results = append(results, db.solve(rest, newB, depth+1)...)
				}
			}
		}
	}
	return results
}

func (db *DatalogDB) solveAfter(goal Goal, rest []Goal, bindings Binding, depth int) []Binding {
	// after(Pred, Args..., Time) - fact occurred after Time
	if len(goal.Args) < 2 {
		return nil
	}

	predTerm := bindings.Deref(goal.Args[0])
	timeTerm := bindings.Deref(goal.Args[len(goal.Args)-1])
	queryArgs := goal.Args[1 : len(goal.Args)-1]

	if timeTerm.IsVar || !timeTerm.IsNum {
		return nil
	}
	minTime := int64(timeTerm.Num)

	var results []Binding

	for _, fact := range db.Facts {
		if fact.Time > minTime {
			if predTerm.IsVar || fact.Predicate == predTerm.Name {
				if newB, ok := UnifyArgs(queryArgs, fact.Args, bindings); ok {
					if predTerm.IsVar {
						newB[predTerm.Name] = Atom(fact.Predicate)
					}
					results = append(results, db.solve(rest, newB, depth+1)...)
				}
			}
		}
	}
	return results
}

func (db *DatalogDB) solveBetween(goal Goal, rest []Goal, bindings Binding, depth int) []Binding {
	// between(Pred, Args..., T1, T2) - fact occurred between T1 and T2
	if len(goal.Args) < 3 {
		return nil
	}

	predTerm := bindings.Deref(goal.Args[0])
	t1Term := bindings.Deref(goal.Args[len(goal.Args)-2])
	t2Term := bindings.Deref(goal.Args[len(goal.Args)-1])
	queryArgs := goal.Args[1 : len(goal.Args)-2]

	if t1Term.IsVar || !t1Term.IsNum || t2Term.IsVar || !t2Term.IsNum {
		return nil
	}
	minTime := int64(t1Term.Num)
	maxTime := int64(t2Term.Num)

	var results []Binding

	for _, fact := range db.Facts {
		if fact.Time >= minTime && fact.Time <= maxTime {
			if predTerm.IsVar || fact.Predicate == predTerm.Name {
				if newB, ok := UnifyArgs(queryArgs, fact.Args, bindings); ok {
					if predTerm.IsVar {
						newB[predTerm.Name] = Atom(fact.Predicate)
					}
					results = append(results, db.solve(rest, newB, depth+1)...)
				}
			}
		}
	}
	return results
}

// ============================================================================
// Temporal Operators (CTL-style)
// ============================================================================

// Always checks if a goal holds for all times in the trace
func (db *DatalogDB) Always(goal Goal) bool {
	// Get all unique times
	times := make(map[int64]bool)
	for _, f := range db.Facts {
		times[f.Time] = true
	}

	for t := range times {
		// Check if goal holds at time t
		db.TimeNow = t
		results := db.solve([]Goal{goal}, make(Binding), 0)
		if len(results) == 0 {
			return false
		}
	}
	return true
}

// Eventually checks if a goal holds at some time
func (db *DatalogDB) Eventually(goal Goal) bool {
	results := db.solve([]Goal{goal}, make(Binding), 0)
	return len(results) > 0
}

// Never checks if a goal never holds
func (db *DatalogDB) Never(goal Goal) bool {
	return !db.Eventually(goal)
}

// LeadsTo checks if whenever goal1 holds, goal2 eventually holds after
func (db *DatalogDB) LeadsTo(goal1, goal2 Goal) bool {
	// Find all times where goal1 holds
	results1 := db.solve([]Goal{goal1}, make(Binding), 0)
	
	for _, b := range results1 {
		// Get the time when goal1 held (need to extract from facts)
		// This is a simplified version - checks if goal2 ever holds
		results2 := db.solve([]Goal{goal2}, b, 0)
		if len(results2) == 0 {
			return false
		}
	}
	return true
}

// ============================================================================
// LISP Integration Helpers
// ============================================================================

// ValueToTerm converts a LISP Value to a Datalog Term
func ValueToTerm(v Value) Term {
	switch v.Type {
	case TypeNumber:
		return NumTerm(v.Number)
	case TypeString:
		return StrTerm(v.Str)
	case TypeSymbol:
		if len(v.Symbol) > 0 && v.Symbol[0] == '?' {
			return Var(v.Symbol[1:])
		}
		return Atom(v.Symbol)
	case TypeBool:
		if v.Bool {
			return Atom("true")
		}
		return Atom("false")
	case TypeList:
		terms := make([]Term, len(v.List))
		for i, elem := range v.List {
			terms[i] = ValueToTerm(elem)
		}
		return ListTerm(terms...)
	default:
		return Atom(v.String())
	}
}

// TermToValue converts a Datalog Term to a LISP Value
func TermToValue(t Term) Value {
	if t.IsVar {
		return Sym("?" + t.Name)
	}
	if t.IsNum {
		return Num(t.Num)
	}
	if t.IsStr {
		return Str(t.Str)
	}
	if t.IsList {
		vals := make([]Value, len(t.List))
		for i, elem := range t.List {
			vals[i] = TermToValue(elem)
		}
		return Lst(vals...)
	}
	return Sym(t.Name)
}

// BindingsToValue converts bindings to a LISP association list
func BindingsToValue(b Binding) Value {
	pairs := make([]Value, 0, len(b))
	for k, v := range b {
		pairs = append(pairs, Lst(Sym(k), TermToValue(v)))
	}
	return Lst(pairs...)
}
