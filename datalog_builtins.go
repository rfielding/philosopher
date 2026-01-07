// ============================================================================
// Datalog LISP Builtins
// ============================================================================
//
// Add to main.go after the Datalog types.
//
// LISP Interface:
//   (assert! pred arg1 arg2 ...)      ; Add fact
//   (assert-at! time pred args...)    ; Add fact with timestamp
//   (retract! pred arg1 arg2 ...)     ; Remove fact
//   (rule name (head) (body...))      ; Define rule
//   (query pred arg1 ?x ...)          ; Query, returns list of bindings
//   (query-all (goal1) (goal2) ...)   ; Conjunction query
//   (always? (goal))                  ; Check AG
//   (eventually? (goal))              ; Check EF
//   (never? (goal))                   ; Check AG(not ...)
//   (datalog-clear!)                  ; Clear all facts
//   (datalog-clear-rules!)            ; Clear all rules
//   (datalog-time! n)                 ; Set current time
//   (datalog-time)                    ; Get current time
//   (datalog-facts)                   ; List all facts
//   (datalog-rules)                   ; List all rules

package main

// RegisterDatalogBuiltins adds Datalog functions to the evaluator
func RegisterDatalogBuiltins(ev *Evaluator) {
	env := ev.GlobalEnv

	// Initialize Datalog DB in evaluator if not present
	if ev.DatalogDB == nil {
		ev.DatalogDB = NewDatalogDB()
	}

	// (assert! pred arg1 arg2 ...)
	env.Set("assert!", Value{Type: TypeBuiltin, Builtin: func(ev *Evaluator, args []Value, env *Env) Value {
		if len(args) < 1 {
			return Sym("error:assert-needs-predicate")
		}
		pred := args[0].Symbol
		terms := make([]Term, len(args)-1)
		for i, a := range args[1:] {
			terms[i] = ValueToTerm(a)
		}
		ev.DatalogDB.Assert(pred, terms...)
		return Sym("ok")
	}})

	// (assert-at! time pred args...)
	env.Set("assert-at!", Value{Type: TypeBuiltin, Builtin: func(ev *Evaluator, args []Value, env *Env) Value {
		if len(args) < 2 {
			return Sym("error:assert-at-needs-time-and-pred")
		}
		time := int64(args[0].Number)
		pred := args[1].Symbol
		terms := make([]Term, len(args)-2)
		for i, a := range args[2:] {
			terms[i] = ValueToTerm(a)
		}
		ev.DatalogDB.AssertAtTime(pred, time, terms...)
		return Sym("ok")
	}})

	// (retract! pred arg1 arg2 ...)
	env.Set("retract!", Value{Type: TypeBuiltin, Builtin: func(ev *Evaluator, args []Value, env *Env) Value {
		if len(args) < 1 {
			return Sym("error:retract-needs-predicate")
		}
		pred := args[0].Symbol
		terms := make([]Term, len(args)-1)
		for i, a := range args[1:] {
			terms[i] = ValueToTerm(a)
		}
		if ev.DatalogDB.Retract(pred, terms...) {
			return Sym("ok")
		}
		return Sym("not-found")
	}})

	// (rule name (head-pred head-args...) (body-goal1) (body-goal2) ...)
	env.Set("rule", Value{Type: TypeBuiltin, Builtin: func(ev *Evaluator, args []Value, env *Env) Value {
		if len(args) < 2 {
			return Sym("error:rule-needs-name-and-head")
		}

		name := args[0].Symbol

		// Parse head: (pred arg1 arg2 ...)
		if args[1].Type != TypeList || len(args[1].List) < 1 {
			return Sym("error:rule-head-must-be-list")
		}
		headList := args[1].List
		headPred := headList[0].Symbol
		headArgs := make([]Term, len(headList)-1)
		for i, a := range headList[1:] {
			headArgs[i] = ValueToTerm(a)
		}
		head := Fact{Predicate: headPred, Args: headArgs}

		// Parse body goals
		body := make([]Goal, 0, len(args)-2)
		for _, bodyArg := range args[2:] {
			if bodyArg.Type != TypeList || len(bodyArg.List) < 1 {
				continue
			}
			goalList := bodyArg.List
			goalPred := goalList[0].Symbol

			// Check for negation: (not (pred args...))
			if goalPred == "not" && len(goalList) > 1 {
				innerGoal := parseGoal(goalList[1])
				innerGoal.Negated = true
				body = append(body, innerGoal)
				continue
			}

			// Check for builtins: (> a b), (< a b), (= a b), (!= a b)
			if isBuiltinOp(goalPred) {
				goalArgs := make([]Term, len(goalList)-1)
				for i, a := range goalList[1:] {
					goalArgs[i] = ValueToTerm(a)
				}
				body = append(body, Goal{
					IsBuiltin: true,
					Builtin:   goalPred,
					Args:      goalArgs,
				})
				continue
			}

			// Regular goal
			body = append(body, parseGoal(bodyArg))
		}

		ev.DatalogDB.AddRule(name, head, body...)
		return Sym("ok")
	}})

	// (query pred arg1 ?x ...)
	env.Set("query", Value{Type: TypeBuiltin, Builtin: func(ev *Evaluator, args []Value, env *Env) Value {
		if len(args) < 1 {
			return Lst()
		}
		pred := args[0].Symbol
		terms := make([]Term, len(args)-1)
		for i, a := range args[1:] {
			terms[i] = ValueToTerm(a)
		}

		results := ev.DatalogDB.Query(pred, terms...)
		return bindingsToLisp(results)
	}})

	// (query-all (goal1) (goal2) ...) - conjunction query
	env.Set("query-all", Value{Type: TypeBuiltin, Builtin: func(ev *Evaluator, args []Value, env *Env) Value {
		goals := make([]Goal, 0, len(args))
		for _, arg := range args {
			if arg.Type == TypeList && len(arg.List) > 0 {
				goals = append(goals, parseGoal(arg))
			}
		}

		results := ev.DatalogDB.QueryGoals(goals...)
		return bindingsToLisp(results)
	}})

	// (always? (goal))
	env.Set("always?", Value{Type: TypeBuiltin, Builtin: func(ev *Evaluator, args []Value, env *Env) Value {
		if len(args) < 1 || args[0].Type != TypeList {
			return Bool(false)
		}
		goal := parseGoal(args[0])
		return Bool(ev.DatalogDB.Always(goal))
	}})

	// (eventually? (goal))
	env.Set("eventually?", Value{Type: TypeBuiltin, Builtin: func(ev *Evaluator, args []Value, env *Env) Value {
		if len(args) < 1 || args[0].Type != TypeList {
			return Bool(false)
		}
		goal := parseGoal(args[0])
		return Bool(ev.DatalogDB.Eventually(goal))
	}})

	// (never? (goal))
	env.Set("never?", Value{Type: TypeBuiltin, Builtin: func(ev *Evaluator, args []Value, env *Env) Value {
		if len(args) < 1 || args[0].Type != TypeList {
			return Bool(true)
		}
		goal := parseGoal(args[0])
		return Bool(ev.DatalogDB.Never(goal))
	}})

	// (datalog-clear!)
	env.Set("datalog-clear!", Value{Type: TypeBuiltin, Builtin: func(ev *Evaluator, args []Value, env *Env) Value {
		ev.DatalogDB.ClearFacts()
		return Sym("ok")
	}})

	// (datalog-clear-rules!)
	env.Set("datalog-clear-rules!", Value{Type: TypeBuiltin, Builtin: func(ev *Evaluator, args []Value, env *Env) Value {
		ev.DatalogDB.ClearRules()
		return Sym("ok")
	}})

	// (datalog-time! n)
	env.Set("datalog-time!", Value{Type: TypeBuiltin, Builtin: func(ev *Evaluator, args []Value, env *Env) Value {
		if len(args) > 0 && args[0].Type == TypeNumber {
			ev.DatalogDB.TimeNow = int64(args[0].Number)
		}
		return Num(float64(ev.DatalogDB.TimeNow))
	}})

	// (datalog-time)
	env.Set("datalog-time", Value{Type: TypeBuiltin, Builtin: func(ev *Evaluator, args []Value, env *Env) Value {
		return Num(float64(ev.DatalogDB.TimeNow))
	}})

	// (datalog-facts) - list all facts
	env.Set("datalog-facts", Value{Type: TypeBuiltin, Builtin: func(ev *Evaluator, args []Value, env *Env) Value {
		facts := make([]Value, len(ev.DatalogDB.Facts))
		for i, f := range ev.DatalogDB.Facts {
			factTerms := make([]Value, len(f.Args)+2)
			factTerms[0] = Sym(f.Predicate)
			for j, t := range f.Args {
				factTerms[j+1] = TermToValue(t)
			}
			factTerms[len(factTerms)-1] = Lst(Sym("@"), Num(float64(f.Time)))
			facts[i] = Lst(factTerms...)
		}
		return Lst(facts...)
	}})

	// (datalog-rules) - list all rules
	env.Set("datalog-rules", Value{Type: TypeBuiltin, Builtin: func(ev *Evaluator, args []Value, env *Env) Value {
		rules := make([]Value, len(ev.DatalogDB.Rules))
		for i, r := range ev.DatalogDB.Rules {
			rules[i] = Sym(r.Name)
		}
		return Lst(rules...)
	}})
}

// Helper functions

func isBuiltinOp(s string) bool {
	switch s {
	case "=", "!=", "<>", ">", "<", ">=", "<=":
		return true
	}
	return false
}

func parseGoal(v Value) Goal {
	if v.Type != TypeList || len(v.List) < 1 {
		return Goal{}
	}

	pred := v.List[0].Symbol

	// Check for negation
	if pred == "not" && len(v.List) > 1 {
		inner := parseGoal(v.List[1])
		inner.Negated = true
		return inner
	}

	// Check for builtin
	if isBuiltinOp(pred) {
		args := make([]Term, len(v.List)-1)
		for i, a := range v.List[1:] {
			args[i] = ValueToTerm(a)
		}
		return Goal{IsBuiltin: true, Builtin: pred, Args: args}
	}

	// Regular goal
	args := make([]Term, len(v.List)-1)
	for i, a := range v.List[1:] {
		args[i] = ValueToTerm(a)
	}
	return Goal{Predicate: pred, Args: args}
}

func bindingsToLisp(results []Binding) Value {
	if len(results) == 0 {
		return Lst()
	}

	rows := make([]Value, len(results))
	for i, b := range results {
		pairs := make([]Value, 0, len(b))
		for k, v := range b {
			pairs = append(pairs, Lst(Sym(k), TermToValue(v)))
		}
		rows[i] = Lst(pairs...)
	}
	return Lst(rows...)
}

// ============================================================================
// Auto-tracing for Actor System
// ============================================================================

// TraceEvent records an event in Datalog during simulation
func (ev *Evaluator) TraceEvent(pred string, args ...Term) {
	if ev.DatalogDB == nil {
		return
	}
	ev.DatalogDB.Assert(pred, args...)
}

// TraceSend records a message send
func (ev *Evaluator) TraceSend(from, to string, msg Value) {
	if ev.DatalogDB == nil {
		return
	}
	ev.DatalogDB.Assert("sent",
		Atom(from),
		Atom(to),
		ValueToTerm(msg),
	)
}

// TraceReceive records a message receive
func (ev *Evaluator) TraceReceive(actor string, msg Value) {
	if ev.DatalogDB == nil {
		return
	}
	ev.DatalogDB.Assert("received",
		Atom(actor),
		ValueToTerm(msg),
	)
}

// TraceStateChange records a state variable change
func (ev *Evaluator) TraceStateChange(actor, varName string, oldVal, newVal Value) {
	if ev.DatalogDB == nil {
		return
	}
	ev.DatalogDB.Assert("state-change",
		Atom(actor),
		Atom(varName),
		ValueToTerm(oldVal),
		ValueToTerm(newVal),
	)
}

// TraceGuard records a guard (receive/send) event
func (ev *Evaluator) TraceGuard(actor, guardType string) {
	if ev.DatalogDB == nil {
		return
	}
	ev.DatalogDB.Assert("guard",
		Atom(actor),
		Atom(guardType),
	)
}

// TraceEffect records an effect (set!) event
func (ev *Evaluator) TraceEffect(actor, op, varName string) {
	if ev.DatalogDB == nil {
		return
	}
	ev.DatalogDB.Assert("effect",
		Atom(actor),
		Atom(op),
		Atom(varName),
	)
}
