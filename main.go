package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

// ============================================================================
// Value Types
// ============================================================================

type ValueType int

const (
	TypeNil ValueType = iota
	TypeSymbol
	TypeNumber
	TypeString
	TypeList
	TypeFunc
	TypeBuiltin
	TypeStack
	TypeQueue
	TypeBool
	TypeTailCall
	TypeBlocked
	TypeTagged
)

type Value struct {
	Type    ValueType
	Symbol  string
	Number  float64
	Str     string
	List    []Value
	Func    *Function
	Builtin func(*Evaluator, []Value, *Env) Value
	Stack   *BoundedStack
	Queue   *BoundedQueue
	Bool    bool
	Tail    *TailCall
	Blocked *BlockedOp
	Tagged  *TaggedValue
}

type TaggedValue struct {
	Tag   string
	Value Value
}

type Function struct {
	Params    []string
	RestParam string
	Body      Value
	Env       *Env
	IsTail    bool
}

type TailCall struct {
	Func Value
	Args []Value
}

type BlockReason int

const (
	BlockNone BlockReason = iota
	BlockStackFull
	BlockStackEmpty
	BlockQueueFull
	BlockQueueEmpty
	BlockCallStackFull
)

type BlockedOp struct {
	Reason   BlockReason
	Resource interface{}
}

// Value constructors
func Nil() Value                     { return Value{Type: TypeNil} }
func Sym(s string) Value             { return Value{Type: TypeSymbol, Symbol: s} }
func Num(n float64) Value            { return Value{Type: TypeNumber, Number: n} }
func Str(s string) Value             { return Value{Type: TypeString, Str: s} }
func Lst(items ...Value) Value       { return Value{Type: TypeList, List: items} }
func Bool(b bool) Value              { return Value{Type: TypeBool, Bool: b} }
func Blocked(r BlockReason) Value    { return Value{Type: TypeBlocked, Blocked: &BlockedOp{Reason: r}} }

func (v Value) IsNil() bool    { return v.Type == TypeNil }
func (v Value) IsList() bool   { return v.Type == TypeList }
func (v Value) IsSymbol() bool { return v.Type == TypeSymbol }
func (v Value) IsTruthy() bool {
	switch v.Type {
	case TypeNil:
		return false
	case TypeBool:
		return v.Bool
	case TypeList:
		return len(v.List) > 0
	case TypeNumber:
		return v.Number != 0
	case TypeString:
		return v.Str != ""
	default:
		return true
	}
}

func (v Value) String() string {
	switch v.Type {
	case TypeNil:
		return "nil"
	case TypeSymbol:
		return v.Symbol
	case TypeNumber:
		if v.Number == float64(int64(v.Number)) {
			return fmt.Sprintf("%d", int64(v.Number))
		}
		return fmt.Sprintf("%g", v.Number)
	case TypeString:
		return fmt.Sprintf("%q", v.Str)
	case TypeBool:
		if v.Bool {
			return "true"
		}
		return "false"
	case TypeList:
		parts := make([]string, len(v.List))
		for i, item := range v.List {
			parts[i] = item.String()
		}
		return "(" + strings.Join(parts, " ") + ")"
	case TypeFunc:
		return "<function>"
	case TypeBuiltin:
		return "<builtin>"
	case TypeStack:
		return fmt.Sprintf("<stack %d/%d>", len(v.Stack.Data), v.Stack.Capacity)
	case TypeQueue:
		return fmt.Sprintf("<queue %d/%d>", len(v.Queue.Data), v.Queue.Capacity)
	case TypeBlocked:
		return fmt.Sprintf("<blocked: %d>", v.Blocked.Reason)
	case TypeTagged:
		return fmt.Sprintf("#%s{%s}", v.Tagged.Tag, v.Tagged.Value.String())
	case TypeActor:
		return fmt.Sprintf("<actor:%s>", v.Symbol)
	default:
		return "<unknown>"
	}
}

// ============================================================================
// Bounded Data Structures
// ============================================================================

type BoundedStack struct {
	Capacity int
	Data     []Value
}

func NewStack(capacity int) *BoundedStack {
	return &BoundedStack{
		Capacity: capacity,
		Data:     make([]Value, 0, capacity),
	}
}

func (s *BoundedStack) IsFull() bool  { return len(s.Data) >= s.Capacity }
func (s *BoundedStack) IsEmpty() bool { return len(s.Data) == 0 }

func (s *BoundedStack) PushNow(v Value) bool {
	if s.IsFull() {
		return false
	}
	s.Data = append(s.Data, v)
	return true
}

func (s *BoundedStack) PopNow() (Value, bool) {
	if s.IsEmpty() {
		return Nil(), false
	}
	v := s.Data[len(s.Data)-1]
	s.Data = s.Data[:len(s.Data)-1]
	return v, true
}

func (s *BoundedStack) PeekNow() (Value, bool) {
	if s.IsEmpty() {
		return Nil(), false
	}
	return s.Data[len(s.Data)-1], true
}

func (s *BoundedStack) Read(index int) (Value, bool) {
	if index >= 0 && index < len(s.Data) {
		return s.Data[index], true
	}
	return Nil(), false
}

func (s *BoundedStack) Write(index int, v Value) bool {
	if index >= 0 && index < len(s.Data) {
		s.Data[index] = v
		return true
	}
	return false
}

type BoundedQueue struct {
	Capacity int
	Data     []Value
}

func NewQueue(capacity int) *BoundedQueue {
	return &BoundedQueue{
		Capacity: capacity,
		Data:     make([]Value, 0, capacity),
	}
}

func (q *BoundedQueue) IsFull() bool  { return len(q.Data) >= q.Capacity }
func (q *BoundedQueue) IsEmpty() bool { return len(q.Data) == 0 }

func (q *BoundedQueue) SendNow(v Value) bool {
	if q.IsFull() {
		return false
	}
	q.Data = append(q.Data, v)
	return true
}

func (q *BoundedQueue) RecvNow() (Value, bool) {
	if q.IsEmpty() {
		return Nil(), false
	}
	v := q.Data[0]
	q.Data = q.Data[1:]
	return v, true
}

func (q *BoundedQueue) PeekNow() (Value, bool) {
	if q.IsEmpty() {
		return Nil(), false
	}
	return q.Data[0], true
}

// ============================================================================
// Tokenizer
// ============================================================================

type TokenType int

const (
	TokLParen TokenType = iota
	TokRParen
	TokQuote
	TokSymbol
	TokNumber
	TokString
	TokEOF
)

type Token struct {
	Type   TokenType
	Text   string
	Number float64
}

type Tokenizer struct {
	input []rune
	pos   int
}

func NewTokenizer(input string) *Tokenizer {
	return &Tokenizer{input: []rune(input), pos: 0}
}

func (t *Tokenizer) peek() rune {
	if t.pos >= len(t.input) {
		return 0
	}
	return t.input[t.pos]
}

func (t *Tokenizer) advance() rune {
	if t.pos >= len(t.input) {
		return 0
	}
	r := t.input[t.pos]
	t.pos++
	return r
}

func (t *Tokenizer) skipWhitespace() {
	for t.pos < len(t.input) {
		c := t.peek()
		if c == ';' {
			for t.pos < len(t.input) && t.peek() != '\n' {
				t.advance()
			}
		} else if unicode.IsSpace(c) {
			t.advance()
		} else {
			break
		}
	}
}

func (t *Tokenizer) Next() Token {
	t.skipWhitespace()

	if t.pos >= len(t.input) {
		return Token{Type: TokEOF}
	}

	c := t.peek()

	switch c {
	case '(':
		t.advance()
		return Token{Type: TokLParen}
	case ')':
		t.advance()
		return Token{Type: TokRParen}
	case '\'':
		t.advance()
		return Token{Type: TokQuote}
	case '"':
		t.advance()
		var sb strings.Builder
		for t.pos < len(t.input) && t.peek() != '"' {
			if t.peek() == '\\' {
				t.advance()
				switch t.peek() {
				case 'n':
					sb.WriteRune('\n')
				case 't':
					sb.WriteRune('\t')
				case '"':
					sb.WriteRune('"')
				case '\\':
					sb.WriteRune('\\')
				default:
					sb.WriteRune(t.peek())
				}
				t.advance()
			} else {
				sb.WriteRune(t.advance())
			}
		}
		t.advance() // closing quote
		return Token{Type: TokString, Text: sb.String()}
	default:
		var sb strings.Builder
		for t.pos < len(t.input) {
			c := t.peek()
			if unicode.IsSpace(c) || c == '(' || c == ')' || c == '\'' || c == '"' {
				break
			}
			sb.WriteRune(t.advance())
		}
		text := sb.String()

		// Try parsing as number
		if n, err := strconv.ParseFloat(text, 64); err == nil {
			return Token{Type: TokNumber, Number: n, Text: text}
		}

		return Token{Type: TokSymbol, Text: text}
	}
}

// ============================================================================
// Parser
// ============================================================================

type Parser struct {
	tokenizer *Tokenizer
	current   Token
}

func NewParser(input string) *Parser {
	p := &Parser{tokenizer: NewTokenizer(input)}
	p.current = p.tokenizer.Next()
	return p
}

func (p *Parser) advance() Token {
	tok := p.current
	p.current = p.tokenizer.Next()
	return tok
}

func (p *Parser) Parse() []Value {
	var exprs []Value
	for p.current.Type != TokEOF {
		exprs = append(exprs, p.parseExpr())
	}
	return exprs
}

func (p *Parser) parseExpr() Value {
	switch p.current.Type {
	case TokLParen:
		p.advance()
		
		// Normal list
		var items []Value
		for p.current.Type != TokRParen && p.current.Type != TokEOF {
			items = append(items, p.parseExpr())
		}
		p.advance() // consume ')'
		return Lst(items...)

	case TokQuote:
		p.advance()
		// Quote wraps next expression: 'x -> (quote x)
		expr := p.parseExpr()
		return Lst(Sym("quote"), expr)

	case TokNumber:
		tok := p.advance()
		return Num(tok.Number)

	case TokString:
		tok := p.advance()
		return Str(tok.Text)

	case TokSymbol:
		tok := p.advance()
		switch tok.Text {
		case "true":
			return Bool(true)
		case "false":
			return Bool(false)
		case "nil":
			return Nil()
		default:
			return Sym(tok.Text)
		}

	default:
		p.advance()
		return Nil()
	}
}

// ============================================================================
// Environment
// ============================================================================

type Env struct {
	bindings map[string]Value
	parent   *Env
}

func NewEnv(parent *Env) *Env {
	return &Env{
		bindings: make(map[string]Value),
		parent:   parent,
	}
}

func (e *Env) Get(name string) (Value, bool) {
	if v, ok := e.bindings[name]; ok {
		return v, true
	}
	if e.parent != nil {
		return e.parent.Get(name)
	}
	return Nil(), false
}

func (e *Env) Set(name string, val Value) {
	e.bindings[name] = val
}

func (e *Env) SetLocal(name string, val Value) {
	if _, ok := e.bindings[name]; ok {
		e.bindings[name] = val
		return
	}
	if e.parent != nil {
		if _, ok := e.parent.Get(name); ok {
			e.parent.SetLocal(name, val)
			return
		}
	}
	e.bindings[name] = val
}

// ============================================================================
// Evaluator
// ============================================================================

type Evaluator struct {
	CallStack    *BoundedStack
	GlobalEnv    *Env
	Registry     map[string]Value
	GensymCount  int64
	Scheduler    *Scheduler
	DatalogDB    *DatalogDB  // Embedded Datalog for temporal reasoning
}

// ============================================================================
// Scheduler and Actors
// ============================================================================

type ActorState int

const (
	ActorRunnable ActorState = iota
	ActorBlocked
	ActorDone
)

type Actor struct {
	Name      string
	Mailbox   *BoundedQueue
	State     ActorState
	BlockedOn string         // Description of what we're blocked on
	Env       *Env           // Actor's local environment
	Code      Value          // Current code to execute (continuation)
	Result    Value          // Last result
	// CSP enforcement
	GuardSeen     bool
	CSPStrict     bool
	CSPViolations []string
}

type Scheduler struct {
	Actors       map[string]*Actor
	RunQueue     []string      // Names of runnable actors
	CurrentActor string        // Currently executing actor
	StepCount    int64
	MaxSteps     int64         // 0 = unlimited
	Trace        bool          // Print execution trace
	CSPEnforce   bool          // CSP enforcement mode
}

func NewScheduler() *Scheduler {
	return &Scheduler{
		Actors:   make(map[string]*Actor),
		RunQueue: make([]string, 0),
		MaxSteps: 0,
		Trace:    false,
	}
}


// ============================================================================
// CSP Enforcement Helpers
// ============================================================================

func (ev *Evaluator) markGuardSeen() {
	if ev.Scheduler == nil || ev.Scheduler.CurrentActor == "" {
		return
	}
	if actor := ev.Scheduler.GetActor(ev.Scheduler.CurrentActor); actor != nil {
		actor.GuardSeen = true
	}
}

func (ev *Evaluator) resetCSPState(actorName string) {
	if actor := ev.Scheduler.GetActor(actorName); actor != nil {
		actor.GuardSeen = false
	}
}

func (ev *Evaluator) checkCSPViolation(varName string) bool {
	if ev.Scheduler == nil || !ev.Scheduler.CSPEnforce || ev.Scheduler.CurrentActor == "" {
		return false
	}
	actor := ev.Scheduler.GetActor(ev.Scheduler.CurrentActor)
	if actor == nil || actor.GuardSeen {
		return false
	}
	violation := fmt.Sprintf("CSP violation: set! '%s' before guard in actor '%s'", varName, ev.Scheduler.CurrentActor)
	actor.CSPViolations = append(actor.CSPViolations, violation)
	if ev.Scheduler.Trace {
		fmt.Fprintln(os.Stderr, "  ⚠ "+violation)
	}
	return actor.CSPStrict
}

func (s *Scheduler) AddActor(name string, mailboxSize int, env *Env, code Value) *Actor {
	actor := &Actor{
		Name:    name,
		Mailbox: NewQueue(mailboxSize),
		State:   ActorRunnable,
		Env:     env,
		Code:    code,
		Result:  Nil(),
	}
	s.Actors[name] = actor
	s.RunQueue = append(s.RunQueue, name)
	return actor
}

func (s *Scheduler) GetActor(name string) *Actor {
	return s.Actors[name]
}

func (s *Scheduler) BlockActor(name string, reason string) {
	if actor, ok := s.Actors[name]; ok {
		actor.State = ActorBlocked
		actor.BlockedOn = reason
		// Remove from run queue
		newQueue := make([]string, 0, len(s.RunQueue))
		for _, n := range s.RunQueue {
			if n != name {
				newQueue = append(newQueue, n)
			}
		}
		s.RunQueue = newQueue
	}
}

func (s *Scheduler) UnblockActor(name string) {
	if actor, ok := s.Actors[name]; ok {
		if actor.State == ActorBlocked {
			actor.State = ActorRunnable
			actor.BlockedOn = ""
			s.RunQueue = append(s.RunQueue, name)
		}
	}
}

func (s *Scheduler) MarkDone(name string) {
	if actor, ok := s.Actors[name]; ok {
		actor.State = ActorDone
		// Remove from run queue
		newQueue := make([]string, 0, len(s.RunQueue))
		for _, n := range s.RunQueue {
			if n != name {
				newQueue = append(newQueue, n)
			}
		}
		s.RunQueue = newQueue
	}
}

func (s *Scheduler) IsDeadlocked() bool {
	// Deadlock if no actors are runnable and at least one is blocked
	if len(s.RunQueue) > 0 {
		return false
	}
	for _, actor := range s.Actors {
		if actor.State == ActorBlocked {
			return true
		}
	}
	return false
}

func (s *Scheduler) AllDone() bool {
	for _, actor := range s.Actors {
		if actor.State != ActorDone {
			return false
		}
	}
	return len(s.Actors) > 0
}

func (s *Scheduler) NextActor() *Actor {
	if len(s.RunQueue) == 0 {
		return nil
	}
	name := s.RunQueue[0]
	// Rotate queue (round-robin)
	s.RunQueue = append(s.RunQueue[1:], name)
	s.CurrentActor = name
	return s.Actors[name]
}

func (s *Scheduler) Status() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Step %d:\n", s.StepCount))
	for name, actor := range s.Actors {
		state := "runnable"
		extra := ""
		switch actor.State {
		case ActorBlocked:
			state = "blocked"
			extra = fmt.Sprintf(" on %s", actor.BlockedOn)
		case ActorDone:
			state = "done"
		}
		sb.WriteString(fmt.Sprintf("  %s: %s%s (mailbox: %d/%d)\n", 
			name, state, extra, len(actor.Mailbox.Data), actor.Mailbox.Capacity))
	}
	return sb.String()
}

func NewEvaluator(callStackDepth int) *Evaluator {
	ev := &Evaluator{
		CallStack:   NewStack(callStackDepth),
		GlobalEnv:   NewEnv(nil),
		Registry:    make(map[string]Value),
		GensymCount: 0,
		Scheduler:   NewScheduler(),
		DatalogDB:   NewDatalogDB(),
	}
	ev.setupBuiltins()
	return ev
}

func (ev *Evaluator) setupBuiltins() {
	env := ev.GlobalEnv

	// Arithmetic
	env.Set("+", Value{Type: TypeBuiltin, Builtin: builtinAdd})
	env.Set("-", Value{Type: TypeBuiltin, Builtin: builtinSub})
	env.Set("*", Value{Type: TypeBuiltin, Builtin: builtinMul})
	env.Set("/", Value{Type: TypeBuiltin, Builtin: builtinDiv})
	env.Set("mod", Value{Type: TypeBuiltin, Builtin: builtinMod})

	// Math functions
	env.Set("ln", Value{Type: TypeBuiltin, Builtin: builtinLn})
	env.Set("log", Value{Type: TypeBuiltin, Builtin: builtinLn}) // alias
	env.Set("exp", Value{Type: TypeBuiltin, Builtin: builtinExp})
	env.Set("sqrt", Value{Type: TypeBuiltin, Builtin: builtinSqrt})
	env.Set("pow", Value{Type: TypeBuiltin, Builtin: builtinPow})
	env.Set("sin", Value{Type: TypeBuiltin, Builtin: builtinSin})
	env.Set("cos", Value{Type: TypeBuiltin, Builtin: builtinCos})
	env.Set("floor", Value{Type: TypeBuiltin, Builtin: builtinFloor})
	env.Set("ceil", Value{Type: TypeBuiltin, Builtin: builtinCeil})
	env.Set("abs", Value{Type: TypeBuiltin, Builtin: builtinAbs})
	env.Set("min", Value{Type: TypeBuiltin, Builtin: builtinMin})
	env.Set("max", Value{Type: TypeBuiltin, Builtin: builtinMax})

	// Comparison
	env.Set("=", Value{Type: TypeBuiltin, Builtin: builtinEq})
	env.Set("eq?", Value{Type: TypeBuiltin, Builtin: builtinEq})     // alias
	env.Set("equals", Value{Type: TypeBuiltin, Builtin: builtinEq})  // alias
	env.Set("!=", Value{Type: TypeBuiltin, Builtin: builtinNeq})
	env.Set("<", Value{Type: TypeBuiltin, Builtin: builtinLt})
	env.Set("<=", Value{Type: TypeBuiltin, Builtin: builtinLte})
	env.Set(">", Value{Type: TypeBuiltin, Builtin: builtinGt})
	env.Set(">=", Value{Type: TypeBuiltin, Builtin: builtinGte})

	// Logic
	env.Set("and", Value{Type: TypeBuiltin, Builtin: builtinAnd})
	env.Set("or", Value{Type: TypeBuiltin, Builtin: builtinOr})
	env.Set("not", Value{Type: TypeBuiltin, Builtin: builtinNot})

	// List operations
	env.Set("first", Value{Type: TypeBuiltin, Builtin: builtinFirst})
	env.Set("rest", Value{Type: TypeBuiltin, Builtin: builtinRest})
	env.Set("car", Value{Type: TypeBuiltin, Builtin: builtinFirst})  // alias
	env.Set("cdr", Value{Type: TypeBuiltin, Builtin: builtinRest})   // alias
	env.Set("cons", Value{Type: TypeBuiltin, Builtin: builtinCons})
	env.Set("append", Value{Type: TypeBuiltin, Builtin: builtinAppend})
	env.Set("list", Value{Type: TypeBuiltin, Builtin: builtinList})
	env.Set("empty?", Value{Type: TypeBuiltin, Builtin: builtinEmpty})
	env.Set("length", Value{Type: TypeBuiltin, Builtin: builtinLength})
	env.Set("nth", Value{Type: TypeBuiltin, Builtin: builtinNth})

	// Type checks
	env.Set("list?", Value{Type: TypeBuiltin, Builtin: builtinIsList})
	env.Set("number?", Value{Type: TypeBuiltin, Builtin: builtinIsNumber})
	env.Set("symbol?", Value{Type: TypeBuiltin, Builtin: builtinIsSymbol})
	env.Set("string?", Value{Type: TypeBuiltin, Builtin: builtinIsString})
	env.Set("nil?", Value{Type: TypeBuiltin, Builtin: builtinIsNil})

	// Evaluation
	env.Set("eval", Value{Type: TypeBuiltin, Builtin: builtinEval})

	// Bounded structures
	env.Set("make-stack", Value{Type: TypeBuiltin, Builtin: builtinMakeStack})
	env.Set("make-queue", Value{Type: TypeBuiltin, Builtin: builtinMakeQueue})

	// Stack operations (blocking and non-blocking)
	env.Set("push!", Value{Type: TypeBuiltin, Builtin: builtinPush})
	env.Set("pop!", Value{Type: TypeBuiltin, Builtin: builtinPop})
	env.Set("push-now!", Value{Type: TypeBuiltin, Builtin: builtinPushNow})
	env.Set("pop-now!", Value{Type: TypeBuiltin, Builtin: builtinPopNow})
	env.Set("stack-peek", Value{Type: TypeBuiltin, Builtin: builtinStackPeek})
	env.Set("stack-peek-now", Value{Type: TypeBuiltin, Builtin: builtinStackPeekNow})
	env.Set("stack-read", Value{Type: TypeBuiltin, Builtin: builtinStackRead})
	env.Set("stack-write!", Value{Type: TypeBuiltin, Builtin: builtinStackWrite})
	env.Set("stack-full?", Value{Type: TypeBuiltin, Builtin: builtinStackFull})
	env.Set("stack-empty?", Value{Type: TypeBuiltin, Builtin: builtinStackEmpty})

	// Queue operations (blocking and non-blocking)
	env.Set("send!", Value{Type: TypeBuiltin, Builtin: builtinSend})
	env.Set("recv!", Value{Type: TypeBuiltin, Builtin: builtinRecv})
	env.Set("send-now!", Value{Type: TypeBuiltin, Builtin: builtinSendNow})
	env.Set("recv-now!", Value{Type: TypeBuiltin, Builtin: builtinRecvNow})
	env.Set("queue-peek", Value{Type: TypeBuiltin, Builtin: builtinQueuePeek})
	env.Set("queue-peek-now", Value{Type: TypeBuiltin, Builtin: builtinQueuePeekNow})
	env.Set("queue-full?", Value{Type: TypeBuiltin, Builtin: builtinQueueFull})
	env.Set("queue-empty?", Value{Type: TypeBuiltin, Builtin: builtinQueueEmpty})

	// I/O
	env.Set("print", Value{Type: TypeBuiltin, Builtin: builtinPrint})
	env.Set("println", Value{Type: TypeBuiltin, Builtin: builtinPrintln})
	env.Set("repr", Value{Type: TypeBuiltin, Builtin: builtinRepr})

	// String operations
	env.Set("string-append", Value{Type: TypeBuiltin, Builtin: builtinStringAppend})
	env.Set("symbol->string", Value{Type: TypeBuiltin, Builtin: builtinSymbolToString})
	env.Set("string->symbol", Value{Type: TypeBuiltin, Builtin: builtinStringToSymbol})
	env.Set("number->string", Value{Type: TypeBuiltin, Builtin: builtinNumberToString})

	// Registry
	env.Set("registry-set!", Value{Type: TypeBuiltin, Builtin: builtinRegistrySet})
	env.Set("registry-get", Value{Type: TypeBuiltin, Builtin: builtinRegistryGet})
	env.Set("registry-keys", Value{Type: TypeBuiltin, Builtin: builtinRegistryKeys})
	env.Set("registry-has?", Value{Type: TypeBuiltin, Builtin: builtinRegistryHas})
	env.Set("registry-delete!", Value{Type: TypeBuiltin, Builtin: builtinRegistryDelete})

	// Type tagging
	env.Set("tag", Value{Type: TypeBuiltin, Builtin: builtinTag})
	env.Set("tag-type", Value{Type: TypeBuiltin, Builtin: builtinTagType})
	env.Set("tag-value", Value{Type: TypeBuiltin, Builtin: builtinTagValue})
	env.Set("tagged?", Value{Type: TypeBuiltin, Builtin: builtinIsTagged})
	env.Set("tag-is?", Value{Type: TypeBuiltin, Builtin: builtinTagIs})

	// Symbol generation
	env.Set("gensym", Value{Type: TypeBuiltin, Builtin: builtinGensym})

	// Scheduler and actor management
	env.Set("spawn-actor", Value{Type: TypeBuiltin, Builtin: builtinSpawnActor})
	env.Set("self", Value{Type: TypeBuiltin, Builtin: builtinSelf})
	env.Set("send-to!", Value{Type: TypeBuiltin, Builtin: builtinSendTo})
	env.Set("receive!", Value{Type: TypeBuiltin, Builtin: builtinReceive})
	env.Set("receive-now!", Value{Type: TypeBuiltin, Builtin: builtinReceiveNow})
	env.Set("mailbox-empty?", Value{Type: TypeBuiltin, Builtin: builtinMailboxEmpty})
	env.Set("mailbox-full?", Value{Type: TypeBuiltin, Builtin: builtinMailboxFull})
	env.Set("yield!", Value{Type: TypeBuiltin, Builtin: builtinYield})
	env.Set("done!", Value{Type: TypeBuiltin, Builtin: builtinDone})
	env.Set("run-scheduler", Value{Type: TypeBuiltin, Builtin: builtinRunScheduler})
	env.Set("scheduler-status", Value{Type: TypeBuiltin, Builtin: builtinSchedulerStatus})
	env.Set("set-trace!", Value{Type: TypeBuiltin, Builtin: builtinSetTrace})
	env.Set("actor-state", Value{Type: TypeBuiltin, Builtin: builtinActorState})
	env.Set("list-actors-sched", Value{Type: TypeBuiltin, Builtin: builtinListActorsSched})
	env.Set("reset-scheduler", Value{Type: TypeBuiltin, Builtin: builtinResetScheduler})

	// CSP enforcement builtins
	env.Set("csp-enforce!", Value{Type: TypeBuiltin, Builtin: func(ev *Evaluator, args []Value, env *Env) Value {
		if len(args) > 0 {
			ev.Scheduler.CSPEnforce = args[0].IsTruthy()
		}
		return Bool(ev.Scheduler.CSPEnforce)
	}})
	env.Set("csp-strict!", Value{Type: TypeBuiltin, Builtin: func(ev *Evaluator, args []Value, env *Env) Value {
		if len(args) < 1 {
			return Bool(false)
		}
		var name string
		if args[0].Type == TypeSymbol {
			name = args[0].Symbol
		} else if args[0].Type == TypeString {
			name = args[0].Str
		}
		strict := len(args) > 1 && args[1].IsTruthy()
		if actor := ev.Scheduler.GetActor(name); actor != nil {
			actor.CSPStrict = strict
			return Bool(true)
		}
		return Bool(false)
	}})
	env.Set("csp-violations", Value{Type: TypeBuiltin, Builtin: func(ev *Evaluator, args []Value, env *Env) Value {
		var result []Value
		if len(args) > 0 {
			// Get violations for specific actor
			var name string
			if args[0].Type == TypeSymbol {
				name = args[0].Symbol
			} else if args[0].Type == TypeString {
				name = args[0].Str
			}
			if actor := ev.Scheduler.GetActor(name); actor != nil {
				for _, v := range actor.CSPViolations {
					result = append(result, Str(v))
				}
			}
		} else {
			// Get all violations
			for actorName, actor := range ev.Scheduler.Actors {
				for _, v := range actor.CSPViolations {
					result = append(result, Lst(Sym(actorName), Str(v)))
				}
			}
		}
		return Lst(result...)
	}})
	env.Set("csp-clear-violations!", Value{Type: TypeBuiltin, Builtin: func(ev *Evaluator, args []Value, env *Env) Value {
		if len(args) > 0 {
			var name string
			if args[0].Type == TypeSymbol {
				name = args[0].Symbol
			} else if args[0].Type == TypeString {
				name = args[0].Str
			}
			if actor := ev.Scheduler.GetActor(name); actor != nil {
				actor.CSPViolations = nil
			}
		} else {
			for _, actor := range ev.Scheduler.Actors {
				actor.CSPViolations = nil
			}
		}
		return Sym("ok")
	}})

	// Register Datalog builtins
	RegisterDatalogBuiltins(ev)
}

func (ev *Evaluator) Eval(expr Value, env *Env) Value {
	if env == nil {
		env = ev.GlobalEnv
	}

	// Trampoline loop for tail calls
	for {
		result := ev.evalStep(expr, env)

		if result.Type == TypeTailCall {
			tc := result.Tail
			if tc.Func.Type == TypeFunc {
				fn := tc.Func.Func
				env = NewEnv(fn.Env)
				
				// Bind regular parameters
				for i, param := range fn.Params {
					if i < len(tc.Args) {
						env.Set(param, tc.Args[i])
					} else {
						env.Set(param, Nil())
					}
				}
				
				// Bind rest parameter if present
				if fn.RestParam != "" {
					restArgs := make([]Value, 0)
					if len(tc.Args) > len(fn.Params) {
						restArgs = tc.Args[len(fn.Params):]
					}
					env.Set(fn.RestParam, Lst(restArgs...))
				}
				
				expr = fn.Body
			} else {
				// Not a function, just call normally
				args := make([]Value, len(tc.Args))
				for i, arg := range tc.Args {
					args[i] = ev.Eval(arg, env)
				}
				return ev.apply(tc.Func, args, env)
			}
		} else {
			return result
		}
	}
}

func (ev *Evaluator) evalStep(expr Value, env *Env) Value {
	switch expr.Type {
	case TypeNil, TypeNumber, TypeString, TypeBool, TypeFunc, TypeBuiltin, TypeStack, TypeQueue:
		return expr

	case TypeSymbol:
		if v, ok := env.Get(expr.Symbol); ok {
			return v
		}
		fmt.Fprintf(os.Stderr, "Undefined symbol: %s\n", expr.Symbol)
		return Nil()

	case TypeList:
		if len(expr.List) == 0 {
			return expr
		}

		head := expr.List[0]

		// Special forms
		if head.IsSymbol() {
			switch head.Symbol {
			case "quote": // Quote - return argument unevaluated
				if len(expr.List) > 1 {
					return expr.List[1]
				}
				return Nil()

			case "if":
				if len(expr.List) < 3 {
					return Nil()
				}
				cond := ev.Eval(expr.List[1], env)
				if cond.IsTruthy() {
					return ev.Eval(expr.List[2], env)
				} else if len(expr.List) > 3 {
					return ev.Eval(expr.List[3], env)
				}
				return Nil()

			case "cond":
				for i := 1; i < len(expr.List); i++ {
					clause := expr.List[i]
					if !clause.IsList() || len(clause.List) < 2 {
						continue
					}
					test := clause.List[0]
					if test.IsSymbol() && test.Symbol == "else" {
						return ev.Eval(clause.List[1], env)
					}
					if ev.Eval(test, env).IsTruthy() {
						return ev.Eval(clause.List[1], env)
					}
				}
				return Nil()

			case "let":
				if len(expr.List) < 3 {
					return Nil()
				}
				name := expr.List[1]
				val := ev.Eval(expr.List[2], env)
				// Propagate blocked status
				if val.Type == TypeBlocked {
					return val
				}
				newEnv := NewEnv(env)
				newEnv.Set(name.Symbol, val)
				if len(expr.List) == 4 {
					// Single body expression
					return ev.Eval(expr.List[3], newEnv)
				} else if len(expr.List) > 4 {
					// Multiple body expressions - wrap in begin
					bodyExprs := make([]Value, len(expr.List)-3+1)
					bodyExprs[0] = Sym("begin")
					copy(bodyExprs[1:], expr.List[3:])
					return ev.Eval(Lst(bodyExprs...), newEnv)
				}
				return val

			case "let*":
				if len(expr.List) < 3 {
					return Nil()
				}
				bindings := expr.List[1]
				newEnv := NewEnv(env)
				if bindings.IsList() {
					for _, binding := range bindings.List {
						if binding.IsList() && len(binding.List) >= 2 {
							name := binding.List[0].Symbol
							val := ev.Eval(binding.List[1], newEnv)
							newEnv.Set(name, val)
						}
					}
				}
				if len(expr.List) == 3 {
					return ev.Eval(expr.List[2], newEnv)
				} else {
					// Multiple body expressions - wrap in begin
					bodyExprs := make([]Value, len(expr.List)-2+1)
					bodyExprs[0] = Sym("begin")
					copy(bodyExprs[1:], expr.List[2:])
					return ev.Eval(Lst(bodyExprs...), newEnv)
				}

			case "set!":
				if len(expr.List) < 3 {
					return Nil()
				}
				name := expr.List[1].Symbol
                // CSP enforcement: check for violation before guard
				if ev.checkCSPViolation(name) {
					return Nil() // Block in strict mode
				}
				val := ev.Eval(expr.List[2], env)
				// Try to set in existing scope, fall back to global
				if _, found := env.Get(name); found {
					env.SetLocal(name, val)
				} else {
					ev.GlobalEnv.Set(name, val)
				}
				return val

			case "define":
				if len(expr.List) < 3 {
					return Nil()
				}
				// (define name value) or (define (name args...) body...)
				if expr.List[1].IsList() {
					// Function shorthand
					sig := expr.List[1].List
					name := sig[0].Symbol
					params := make([]string, 0)
					restParam := ""
					sigParams := sig[1:] // Parameters part of signature
					for i := 0; i < len(sigParams); i++ {
						p := sigParams[i]
						if p.IsSymbol() && p.Symbol == "." {
							// Rest parameter: next symbol is the rest param name
							if i+1 < len(sigParams) && sigParams[i+1].IsSymbol() {
								restParam = sigParams[i+1].Symbol
							}
							break
						}
						if p.IsSymbol() {
							params = append(params, p.Symbol)
						}
					}
					// Handle multi-expression body: wrap in implicit begin
					var body Value
					if len(expr.List) == 3 {
						body = expr.List[2]
					} else {
						// Multiple body expressions - wrap in begin
						bodyExprs := make([]Value, len(expr.List)-2+1)
						bodyExprs[0] = Sym("begin")
						copy(bodyExprs[1:], expr.List[2:])
						body = Lst(bodyExprs...)
					}
					fn := &Function{
						Params:    params,
						RestParam: restParam,
						Body:      body,
						Env:       env,
					}
					val := Value{Type: TypeFunc, Func: fn}
					ev.GlobalEnv.Set(name, val)
					return val
				} else {
					name := expr.List[1].Symbol
                // CSP enforcement: check for violation before guard
				if ev.checkCSPViolation(name) {
					return Nil() // Block in strict mode
				}
					val := ev.Eval(expr.List[2], env)
					ev.GlobalEnv.Set(name, val)
					return val
				}

			case "lambda", "fn":
				if len(expr.List) < 3 {
					return Nil()
				}
				params := make([]string, 0)
				restParam := ""
				if expr.List[1].IsList() {
					paramList := expr.List[1].List
					for i := 0; i < len(paramList); i++ {
						p := paramList[i]
						if p.IsSymbol() && p.Symbol == "." {
							// Rest parameter: next symbol is the rest param name
							if i+1 < len(paramList) && paramList[i+1].IsSymbol() {
								restParam = paramList[i+1].Symbol
							}
							break
						}
						if p.IsSymbol() {
							params = append(params, p.Symbol)
						}
					}
				}
				// Handle multi-expression body
				var body Value
				if len(expr.List) == 3 {
					body = expr.List[2]
				} else {
					bodyExprs := make([]Value, len(expr.List)-2+1)
					bodyExprs[0] = Sym("begin")
					copy(bodyExprs[1:], expr.List[2:])
					body = Lst(bodyExprs...)
				}
				return Value{
					Type: TypeFunc,
					Func: &Function{
						Params:    params,
						RestParam: restParam,
						Body:      body,
						Env:       env,
					},
				}

			case "tail":
				// Tail call - evaluate args but return TailCall marker
				if len(expr.List) < 2 {
					return Nil()
				}
				fn := ev.Eval(expr.List[1], env)
				args := make([]Value, len(expr.List)-2)
				for i, arg := range expr.List[2:] {
					args[i] = ev.Eval(arg, env)
				}
				return Value{
					Type: TypeTailCall,
					Tail: &TailCall{Func: fn, Args: args},
				}

			case "do", "begin":
				var result Value = Nil()
				for _, e := range expr.List[1:] {
					result = ev.Eval(e, env)
					// Propagate blocked status
					if result.Type == TypeBlocked {
						return result
					}
				}
				return result

			case "match":
				if len(expr.List) < 2 {
					return Nil()
				}
				target := ev.Eval(expr.List[1], env)
				for i := 2; i < len(expr.List); i++ {
					clause := expr.List[i]
					if !clause.IsList() || len(clause.List) < 2 {
						continue
					}
					pattern := clause.List[0]
					body := clause.List[1]
					if bindings, ok := ev.match(pattern, target, env); ok {
						newEnv := NewEnv(env)
						for k, v := range bindings {
							newEnv.Set(k, v)
						}
						return ev.Eval(body, newEnv)
					}
				}
				return Nil()
			}
		}

		// Function call
		fn := ev.Eval(head, env)
		args := make([]Value, len(expr.List)-1)
		for i, arg := range expr.List[1:] {
			args[i] = ev.Eval(arg, env)
		}
		return ev.apply(fn, args, env)
	}

	return Nil()
}

func (ev *Evaluator) apply(fn Value, args []Value, env *Env) Value {
	switch fn.Type {
	case TypeBuiltin:
		return fn.Builtin(ev, args, env)

	case TypeFunc:
		f := fn.Func
		newEnv := NewEnv(f.Env)
		
		// Bind regular parameters
		for i, param := range f.Params {
			if i < len(args) {
				newEnv.Set(param, args[i])
			} else {
				newEnv.Set(param, Nil())
			}
		}
		
		// Bind rest parameter if present
		if f.RestParam != "" {
			restArgs := make([]Value, 0)
			if len(args) > len(f.Params) {
				restArgs = args[len(f.Params):]
			}
			newEnv.Set(f.RestParam, Lst(restArgs...))
		}

		// Check call stack bounds
		if !ev.CallStack.PushNow(Lst(args...)) {
			return Blocked(BlockCallStackFull)
		}

		result := ev.Eval(f.Body, newEnv)
		ev.CallStack.PopNow()
		return result
	}

	return Nil()
}

func (ev *Evaluator) match(pattern, target Value, env *Env) (map[string]Value, bool) {
	bindings := make(map[string]Value)

	// Wildcard
	if pattern.IsSymbol() && pattern.Symbol == "_" {
		return bindings, true
	}

	// Pattern variable ?name
	if pattern.IsSymbol() && len(pattern.Symbol) > 0 && pattern.Symbol[0] == '?' {
		bindings[pattern.Symbol[1:]] = target
		return bindings, true
	}

	// Quoted symbol matches symbol
	if pattern.IsList() && len(pattern.List) == 2 &&
		pattern.List[0].IsSymbol() && pattern.List[0].Symbol == "'" {
		if target.IsSymbol() && target.Symbol == pattern.List[1].Symbol {
			return bindings, true
		}
		return nil, false
	}

	// Literal match
	if pattern.Type == target.Type {
		switch pattern.Type {
		case TypeNil:
			return bindings, true
		case TypeNumber:
			if pattern.Number == target.Number {
				return bindings, true
			}
		case TypeString:
			if pattern.Str == target.Str {
				return bindings, true
			}
		case TypeSymbol:
			if pattern.Symbol == target.Symbol {
				return bindings, true
			}
		case TypeBool:
			if pattern.Bool == target.Bool {
				return bindings, true
			}
		case TypeList:
			if len(pattern.List) != len(target.List) {
				return nil, false
			}
			for i := range pattern.List {
				sub, ok := ev.match(pattern.List[i], target.List[i], env)
				if !ok {
					return nil, false
				}
				for k, v := range sub {
					bindings[k] = v
				}
			}
			return bindings, true
		}
	}

	return nil, false
}

// ============================================================================
// Builtins
// ============================================================================

func builtinAdd(ev *Evaluator, args []Value, env *Env) Value {
	sum := 0.0
	for _, a := range args {
		sum += a.Number
	}
	return Num(sum)
}

func builtinSub(ev *Evaluator, args []Value, env *Env) Value {
	if len(args) == 0 {
		return Num(0)
	}
	if len(args) == 1 {
		return Num(-args[0].Number)
	}
	result := args[0].Number
	for _, a := range args[1:] {
		result -= a.Number
	}
	return Num(result)
}

func builtinMul(ev *Evaluator, args []Value, env *Env) Value {
	product := 1.0
	for _, a := range args {
		product *= a.Number
	}
	return Num(product)
}

func builtinDiv(ev *Evaluator, args []Value, env *Env) Value {
	if len(args) < 2 {
		return Num(0)
	}
	return Num(args[0].Number / args[1].Number)
}

func builtinMod(ev *Evaluator, args []Value, env *Env) Value {
	if len(args) < 2 {
		return Num(0)
	}
	return Num(float64(int64(args[0].Number) % int64(args[1].Number)))
}

// Math functions
func builtinLn(ev *Evaluator, args []Value, env *Env) Value {
	if len(args) < 1 || args[0].Type != TypeNumber {
		return Num(0)
	}
	return Num(math.Log(args[0].Number))
}

func builtinExp(ev *Evaluator, args []Value, env *Env) Value {
	if len(args) < 1 || args[0].Type != TypeNumber {
		return Num(0)
	}
	return Num(math.Exp(args[0].Number))
}

func builtinSqrt(ev *Evaluator, args []Value, env *Env) Value {
	if len(args) < 1 || args[0].Type != TypeNumber {
		return Num(0)
	}
	return Num(math.Sqrt(args[0].Number))
}

func builtinPow(ev *Evaluator, args []Value, env *Env) Value {
	if len(args) < 2 {
		return Num(0)
	}
	return Num(math.Pow(args[0].Number, args[1].Number))
}

func builtinSin(ev *Evaluator, args []Value, env *Env) Value {
	if len(args) < 1 || args[0].Type != TypeNumber {
		return Num(0)
	}
	return Num(math.Sin(args[0].Number))
}

func builtinCos(ev *Evaluator, args []Value, env *Env) Value {
	if len(args) < 1 || args[0].Type != TypeNumber {
		return Num(0)
	}
	return Num(math.Cos(args[0].Number))
}

func builtinFloor(ev *Evaluator, args []Value, env *Env) Value {
	if len(args) < 1 || args[0].Type != TypeNumber {
		return Num(0)
	}
	return Num(math.Floor(args[0].Number))
}

func builtinCeil(ev *Evaluator, args []Value, env *Env) Value {
	if len(args) < 1 || args[0].Type != TypeNumber {
		return Num(0)
	}
	return Num(math.Ceil(args[0].Number))
}

func builtinAbs(ev *Evaluator, args []Value, env *Env) Value {
	if len(args) < 1 || args[0].Type != TypeNumber {
		return Num(0)
	}
	return Num(math.Abs(args[0].Number))
}

func builtinMin(ev *Evaluator, args []Value, env *Env) Value {
	if len(args) < 1 {
		return Num(0)
	}
	min := args[0].Number
	for _, a := range args[1:] {
		if a.Type == TypeNumber && a.Number < min {
			min = a.Number
		}
	}
	return Num(min)
}

func builtinMax(ev *Evaluator, args []Value, env *Env) Value {
	if len(args) < 1 {
		return Num(0)
	}
	max := args[0].Number
	for _, a := range args[1:] {
		if a.Type == TypeNumber && a.Number > max {
			max = a.Number
		}
	}
	return Num(max)
}

func builtinEq(ev *Evaluator, args []Value, env *Env) Value {
	if len(args) < 2 {
		return Bool(true)
	}
	return Bool(valuesEqual(args[0], args[1]))
}

func valuesEqual(a, b Value) bool {
	if a.Type != b.Type {
		return false
	}
	switch a.Type {
	case TypeNumber:
		return a.Number == b.Number
	case TypeString:
		return a.Str == b.Str
	case TypeSymbol:
		return a.Symbol == b.Symbol
	case TypeBool:
		return a.Bool == b.Bool
	case TypeNil:
		return true
	case TypeList:
		if len(a.List) != len(b.List) {
			return false
		}
		for i := range a.List {
			if !valuesEqual(a.List[i], b.List[i]) {
				return false
			}
		}
		return true
	}
	return false
}

func builtinNeq(ev *Evaluator, args []Value, env *Env) Value {
	return Bool(!builtinEq(ev, args, env).Bool)
}

func builtinLt(ev *Evaluator, args []Value, env *Env) Value {
	if len(args) < 2 {
		return Bool(false)
	}
	return Bool(args[0].Number < args[1].Number)
}

func builtinLte(ev *Evaluator, args []Value, env *Env) Value {
	if len(args) < 2 {
		return Bool(false)
	}
	return Bool(args[0].Number <= args[1].Number)
}

func builtinGt(ev *Evaluator, args []Value, env *Env) Value {
	if len(args) < 2 {
		return Bool(false)
	}
	return Bool(args[0].Number > args[1].Number)
}

func builtinGte(ev *Evaluator, args []Value, env *Env) Value {
	if len(args) < 2 {
		return Bool(false)
	}
	return Bool(args[0].Number >= args[1].Number)
}

func builtinAnd(ev *Evaluator, args []Value, env *Env) Value {
	for _, a := range args {
		if !a.IsTruthy() {
			return Bool(false)
		}
	}
	return Bool(true)
}

func builtinOr(ev *Evaluator, args []Value, env *Env) Value {
	for _, a := range args {
		if a.IsTruthy() {
			return Bool(true)
		}
	}
	return Bool(false)
}

func builtinNot(ev *Evaluator, args []Value, env *Env) Value {
	if len(args) == 0 {
		return Bool(true)
	}
	return Bool(!args[0].IsTruthy())
}

func builtinFirst(ev *Evaluator, args []Value, env *Env) Value {
	if len(args) == 0 || !args[0].IsList() || len(args[0].List) == 0 {
		return Nil()
	}
	return args[0].List[0]
}

func builtinRest(ev *Evaluator, args []Value, env *Env) Value {
	if len(args) == 0 || !args[0].IsList() || len(args[0].List) == 0 {
		return Lst()
	}
	return Lst(args[0].List[1:]...)
}

func builtinCons(ev *Evaluator, args []Value, env *Env) Value {
	if len(args) < 2 {
		return Lst()
	}
	if args[1].IsList() {
		return Lst(append([]Value{args[0]}, args[1].List...)...)
	}
	return Lst(args[0], args[1])
}

func builtinAppend(ev *Evaluator, args []Value, env *Env) Value {
	var result []Value
	for _, a := range args {
		if a.IsList() {
			result = append(result, a.List...)
		} else {
			result = append(result, a)
		}
	}
	return Lst(result...)
}

func builtinList(ev *Evaluator, args []Value, env *Env) Value {
	return Lst(args...)
}

func builtinEmpty(ev *Evaluator, args []Value, env *Env) Value {
	if len(args) == 0 {
		return Bool(true)
	}
	if args[0].IsList() {
		return Bool(len(args[0].List) == 0)
	}
	return Bool(true)
}

func builtinLength(ev *Evaluator, args []Value, env *Env) Value {
	if len(args) == 0 || !args[0].IsList() {
		return Num(0)
	}
	return Num(float64(len(args[0].List)))
}

func builtinNth(ev *Evaluator, args []Value, env *Env) Value {
	if len(args) < 2 || !args[0].IsList() {
		return Nil()
	}
	idx := int(args[1].Number)
	if idx >= 0 && idx < len(args[0].List) {
		return args[0].List[idx]
	}
	return Nil()
}

func builtinIsList(ev *Evaluator, args []Value, env *Env) Value {
	if len(args) == 0 {
		return Bool(false)
	}
	return Bool(args[0].Type == TypeList)
}

func builtinIsNumber(ev *Evaluator, args []Value, env *Env) Value {
	if len(args) == 0 {
		return Bool(false)
	}
	return Bool(args[0].Type == TypeNumber)
}

func builtinIsSymbol(ev *Evaluator, args []Value, env *Env) Value {
	if len(args) == 0 {
		return Bool(false)
	}
	return Bool(args[0].Type == TypeSymbol)
}

func builtinIsString(ev *Evaluator, args []Value, env *Env) Value {
	if len(args) == 0 {
		return Bool(false)
	}
	return Bool(args[0].Type == TypeString)
}

func builtinIsNil(ev *Evaluator, args []Value, env *Env) Value {
	if len(args) == 0 {
		return Bool(true)
	}
	return Bool(args[0].Type == TypeNil)
}

// eval - evaluate a data structure as code
func builtinEval(ev *Evaluator, args []Value, env *Env) Value {
	if len(args) == 0 {
		return Nil()
	}
	// Evaluate the argument in the global environment
	return ev.Eval(args[0], ev.GlobalEnv)
}

func builtinMakeStack(ev *Evaluator, args []Value, env *Env) Value {
	capacity := 16
	if len(args) > 0 {
		capacity = int(args[0].Number)
	}
	return Value{Type: TypeStack, Stack: NewStack(capacity)}
}

func builtinMakeQueue(ev *Evaluator, args []Value, env *Env) Value {
	capacity := 16
	if len(args) > 0 {
		capacity = int(args[0].Number)
	}
	return Value{Type: TypeQueue, Queue: NewQueue(capacity)}
}

// Stack operations
func builtinPush(ev *Evaluator, args []Value, env *Env) Value {
	if len(args) < 2 || args[0].Type != TypeStack {
		return Nil()
	}
	stack := args[0].Stack
	if stack.IsFull() {
		return Blocked(BlockStackFull)
	}
	stack.PushNow(args[1])
	return Sym("ok")
}

func builtinPop(ev *Evaluator, args []Value, env *Env) Value {
	if len(args) < 1 || args[0].Type != TypeStack {
		return Nil()
	}
	stack := args[0].Stack
	if stack.IsEmpty() {
		return Blocked(BlockStackEmpty)
	}
	v, _ := stack.PopNow()
	return v
}

func builtinPushNow(ev *Evaluator, args []Value, env *Env) Value {
	if len(args) < 2 || args[0].Type != TypeStack {
		return Nil()
	}
	if args[0].Stack.PushNow(args[1]) {
		return Sym("ok")
	}
	return Sym("full")
}

func builtinPopNow(ev *Evaluator, args []Value, env *Env) Value {
	if len(args) < 1 || args[0].Type != TypeStack {
		return Nil()
	}
	v, ok := args[0].Stack.PopNow()
	if ok {
		return v
	}
	return Sym("empty")
}

func builtinStackPeek(ev *Evaluator, args []Value, env *Env) Value {
	if len(args) < 1 || args[0].Type != TypeStack {
		return Nil()
	}
	stack := args[0].Stack
	if stack.IsEmpty() {
		return Blocked(BlockStackEmpty)
	}
	v, _ := stack.PeekNow()
	return v
}

func builtinStackPeekNow(ev *Evaluator, args []Value, env *Env) Value {
	if len(args) < 1 || args[0].Type != TypeStack {
		return Nil()
	}
	v, ok := args[0].Stack.PeekNow()
	if ok {
		return v
	}
	return Sym("empty")
}

func builtinStackRead(ev *Evaluator, args []Value, env *Env) Value {
	if len(args) < 2 || args[0].Type != TypeStack {
		return Nil()
	}
	v, ok := args[0].Stack.Read(int(args[1].Number))
	if ok {
		return v
	}
	return Nil()
}

func builtinStackWrite(ev *Evaluator, args []Value, env *Env) Value {
	if len(args) < 3 || args[0].Type != TypeStack {
		return Nil()
	}
	if args[0].Stack.Write(int(args[1].Number), args[2]) {
		return Sym("ok")
	}
	return Sym("error")
}

func builtinStackFull(ev *Evaluator, args []Value, env *Env) Value {
	if len(args) < 1 || args[0].Type != TypeStack {
		return Bool(false)
	}
	return Bool(args[0].Stack.IsFull())
}

func builtinStackEmpty(ev *Evaluator, args []Value, env *Env) Value {
	if len(args) < 1 || args[0].Type != TypeStack {
		return Bool(true)
	}
	return Bool(args[0].Stack.IsEmpty())
}

// Queue operations
func builtinSend(ev *Evaluator, args []Value, env *Env) Value {
	if len(args) < 2 || args[0].Type != TypeQueue {
		return Nil()
	}
	queue := args[0].Queue
	if queue.IsFull() {
		return Blocked(BlockQueueFull)
	}
	queue.SendNow(args[1])
	return Sym("ok")
}

func builtinRecv(ev *Evaluator, args []Value, env *Env) Value {
	if len(args) < 1 || args[0].Type != TypeQueue {
		return Nil()
	}
	queue := args[0].Queue
	if queue.IsEmpty() {
		return Blocked(BlockQueueEmpty)
	}
	v, _ := queue.RecvNow()
	return v
}

func builtinSendNow(ev *Evaluator, args []Value, env *Env) Value {
	if len(args) < 2 || args[0].Type != TypeQueue {
		return Nil()
	}
	if args[0].Queue.SendNow(args[1]) {
		return Sym("ok")
	}
	return Sym("full")
}

func builtinRecvNow(ev *Evaluator, args []Value, env *Env) Value {
	if len(args) < 1 || args[0].Type != TypeQueue {
		return Nil()
	}
	v, ok := args[0].Queue.RecvNow()
	if ok {
		return v
	}
	return Sym("empty")
}

func builtinQueuePeek(ev *Evaluator, args []Value, env *Env) Value {
	if len(args) < 1 || args[0].Type != TypeQueue {
		return Nil()
	}
	queue := args[0].Queue
	if queue.IsEmpty() {
		return Blocked(BlockQueueEmpty)
	}
	v, _ := queue.PeekNow()
	return v
}

func builtinQueuePeekNow(ev *Evaluator, args []Value, env *Env) Value {
	if len(args) < 1 || args[0].Type != TypeQueue {
		return Nil()
	}
	v, ok := args[0].Queue.PeekNow()
	if ok {
		return v
	}
	return Sym("empty")
}

func builtinQueueFull(ev *Evaluator, args []Value, env *Env) Value {
	if len(args) < 1 || args[0].Type != TypeQueue {
		return Bool(false)
	}
	return Bool(args[0].Queue.IsFull())
}

func builtinQueueEmpty(ev *Evaluator, args []Value, env *Env) Value {
	if len(args) < 1 || args[0].Type != TypeQueue {
		return Bool(true)
	}
	return Bool(args[0].Queue.IsEmpty())
}

// I/O
func builtinPrint(ev *Evaluator, args []Value, env *Env) Value {
	parts := make([]string, len(args))
	for i, a := range args {
		if a.Type == TypeString {
			parts[i] = a.Str
		} else {
			parts[i] = a.String()
		}
	}
	fmt.Print(strings.Join(parts, " "))
	return Nil()
}

func builtinPrintln(ev *Evaluator, args []Value, env *Env) Value {
	builtinPrint(ev, args, env)
	fmt.Println()
	return Nil()
}

func builtinRepr(ev *Evaluator, args []Value, env *Env) Value {
	if len(args) == 0 {
		return Str("")
	}
	return Str(args[0].String())
}

// ============================================================================
// String Operations
// ============================================================================

func builtinStringAppend(ev *Evaluator, args []Value, env *Env) Value {
	var sb strings.Builder
	for _, arg := range args {
		switch arg.Type {
		case TypeString:
			sb.WriteString(arg.Str)
		case TypeSymbol:
			sb.WriteString(arg.Symbol)
		case TypeNumber:
			if arg.Number == float64(int64(arg.Number)) {
				sb.WriteString(fmt.Sprintf("%d", int64(arg.Number)))
			} else {
				sb.WriteString(fmt.Sprintf("%g", arg.Number))
			}
		default:
			sb.WriteString(arg.String())
		}
	}
	return Str(sb.String())
}

func builtinSymbolToString(ev *Evaluator, args []Value, env *Env) Value {
	if len(args) == 0 {
		return Str("")
	}
	if args[0].Type == TypeSymbol {
		return Str(args[0].Symbol)
	}
	return Str(args[0].String())
}

func builtinStringToSymbol(ev *Evaluator, args []Value, env *Env) Value {
	if len(args) == 0 {
		return Sym("")
	}
	if args[0].Type == TypeString {
		return Sym(args[0].Str)
	}
	if args[0].Type == TypeSymbol {
		return args[0]
	}
	return Sym(args[0].String())
}

func builtinNumberToString(ev *Evaluator, args []Value, env *Env) Value {
	if len(args) == 0 {
		return Str("0")
	}
	if args[0].Type == TypeNumber {
		if args[0].Number == float64(int64(args[0].Number)) {
			return Str(fmt.Sprintf("%d", int64(args[0].Number)))
		}
		return Str(fmt.Sprintf("%g", args[0].Number))
	}
	return Str(args[0].String())
}

// ============================================================================
// Registry Builtins
// ============================================================================

func builtinRegistrySet(ev *Evaluator, args []Value, env *Env) Value {
	if len(args) < 2 {
		return Nil()
	}
	var name string
	if args[0].Type == TypeSymbol {
		name = args[0].Symbol
	} else if args[0].Type == TypeString {
		name = args[0].Str
	} else {
		return Nil()
	}
	ev.Registry[name] = args[1]
	return args[1]
}

func builtinRegistryGet(ev *Evaluator, args []Value, env *Env) Value {
	if len(args) < 1 {
		return Nil()
	}
	var name string
	if args[0].Type == TypeSymbol {
		name = args[0].Symbol
	} else if args[0].Type == TypeString {
		name = args[0].Str
	} else {
		return Nil()
	}
	if v, ok := ev.Registry[name]; ok {
		return v
	}
	return Nil()
}

func builtinRegistryKeys(ev *Evaluator, args []Value, env *Env) Value {
	keys := make([]Value, 0, len(ev.Registry))
	for k := range ev.Registry {
		keys = append(keys, Sym(k))
	}
	return Lst(keys...)
}

func builtinRegistryHas(ev *Evaluator, args []Value, env *Env) Value {
	if len(args) < 1 {
		return Bool(false)
	}
	var name string
	if args[0].Type == TypeSymbol {
		name = args[0].Symbol
	} else if args[0].Type == TypeString {
		name = args[0].Str
	} else {
		return Bool(false)
	}
	_, ok := ev.Registry[name]
	return Bool(ok)
}

func builtinRegistryDelete(ev *Evaluator, args []Value, env *Env) Value {
	if len(args) < 1 {
		return Bool(false)
	}
	var name string
	if args[0].Type == TypeSymbol {
		name = args[0].Symbol
	} else if args[0].Type == TypeString {
		name = args[0].Str
	} else {
		return Bool(false)
	}
	if _, ok := ev.Registry[name]; ok {
		delete(ev.Registry, name)
		return Bool(true)
	}
	return Bool(false)
}

// ============================================================================
// Type Tagging Builtins
// ============================================================================

func builtinTag(ev *Evaluator, args []Value, env *Env) Value {
	if len(args) < 2 {
		return Nil()
	}
	var tagName string
	if args[0].Type == TypeSymbol {
		tagName = args[0].Symbol
	} else if args[0].Type == TypeString {
		tagName = args[0].Str
	} else {
		return Nil()
	}
	return Value{
		Type: TypeTagged,
		Tagged: &TaggedValue{
			Tag:   tagName,
			Value: args[1],
		},
	}
}

func builtinTagType(ev *Evaluator, args []Value, env *Env) Value {
	if len(args) < 1 || args[0].Type != TypeTagged {
		return Nil()
	}
	return Sym(args[0].Tagged.Tag)
}

func builtinTagValue(ev *Evaluator, args []Value, env *Env) Value {
	if len(args) < 1 || args[0].Type != TypeTagged {
		return Nil()
	}
	return args[0].Tagged.Value
}

func builtinIsTagged(ev *Evaluator, args []Value, env *Env) Value {
	if len(args) < 1 {
		return Bool(false)
	}
	return Bool(args[0].Type == TypeTagged)
}

func builtinTagIs(ev *Evaluator, args []Value, env *Env) Value {
	if len(args) < 2 || args[0].Type != TypeTagged {
		return Bool(false)
	}
	var tagName string
	if args[1].Type == TypeSymbol {
		tagName = args[1].Symbol
	} else if args[1].Type == TypeString {
		tagName = args[1].Str
	} else {
		return Bool(false)
	}
	return Bool(args[0].Tagged.Tag == tagName)
}

// ============================================================================
// Symbol Generation
// ============================================================================

func builtinGensym(ev *Evaluator, args []Value, env *Env) Value {
	prefix := "g"
	if len(args) > 0 {
		if args[0].Type == TypeSymbol {
			prefix = args[0].Symbol
		} else if args[0].Type == TypeString {
			prefix = args[0].Str
		}
	}
	ev.GensymCount++
	return Sym(fmt.Sprintf("%s-%d", prefix, ev.GensymCount))
}

// ============================================================================
// Scheduler Builtins
// ============================================================================

// TypeActor for actor references
const TypeActor ValueType = 100

type ActorRef struct {
	Name string
}

func ActorVal(name string) Value {
	return Value{Type: TypeActor, Symbol: name}
}

func (v Value) IsActor() bool {
	return v.Type == TypeActor
}

// (spawn-actor name mailbox-size body)
// Creates a new actor with the given name, mailbox size, and initial code
func builtinSpawnActor(ev *Evaluator, args []Value, env *Env) Value {
	if len(args) < 3 {
		fmt.Fprintln(os.Stderr, "spawn-actor: need name, mailbox-size, body")
		return Nil()
	}
	
	var name string
	if args[0].Type == TypeSymbol {
		name = args[0].Symbol
	} else if args[0].Type == TypeString {
		name = args[0].Str
	} else {
		fmt.Fprintln(os.Stderr, "spawn-actor: name must be symbol or string")
		return Nil()
	}
	
	mailboxSize := 16
	if args[1].Type == TypeNumber {
		mailboxSize = int(args[1].Number)
	}
	
	// Create actor's own environment (inherits from global)
	actorEnv := NewEnv(ev.GlobalEnv)
	
	// The body is a thunk (code to execute)
	body := args[2]
	
	ev.Scheduler.AddActor(name, mailboxSize, actorEnv, body)
	
	// AUTO-TRACE: log the spawn as a fact
	ev.DatalogDB.AssertAtTime("spawned", ev.Scheduler.StepCount, Atom(name))
	
	return ActorVal(name)
}

// (self) - returns current actor's name
func builtinSelf(ev *Evaluator, args []Value, env *Env) Value {
	if ev.Scheduler.CurrentActor == "" {
		return Nil()
	}
	return Sym(ev.Scheduler.CurrentActor)
}

// (send-to! actor-name message)
// Sends a message to the named actor's mailbox
// Blocks if mailbox is full
// AUTO-TRACES: asserts (sent from to msg time) fact
func builtinSendTo(ev *Evaluator, args []Value, env *Env) Value {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "send-to!: need actor-name and message")
		return Nil()
	}
	
	var targetName string
	if args[0].Type == TypeSymbol {
		targetName = args[0].Symbol
	} else if args[0].Type == TypeString {
		targetName = args[0].Str
	} else if args[0].Type == TypeActor {
		targetName = args[0].Symbol
	} else {
		fmt.Fprintln(os.Stderr, "send-to!: target must be symbol, string, or actor ref")
		return Nil()
	}
	
	target := ev.Scheduler.GetActor(targetName)
    ev.markGuardSeen() // CSP: send is a synchronization point
	if target == nil {
		fmt.Fprintf(os.Stderr, "send-to!: unknown actor %s\n", targetName)
		return Nil()
	}
	
	message := args[1]
	
	if target.Mailbox.SendNow(message) {
		// AUTO-TRACE: log the send as a fact
		sender := ev.Scheduler.CurrentActor
		if sender == "" {
			sender = "external"
		}
		ev.DatalogDB.AssertAtTime("sent", ev.Scheduler.StepCount,
			Atom(sender), Atom(targetName), ValueToTerm(message))
		
		// Message sent successfully
		// If target was blocked on receive, unblock it
		if target.State == ActorBlocked && strings.HasPrefix(target.BlockedOn, "recv") {
			ev.Scheduler.UnblockActor(targetName)
		}
		return Sym("ok")
	} else {
		// Mailbox full, block sender
		if ev.Scheduler.CurrentActor != "" {
			ev.Scheduler.BlockActor(ev.Scheduler.CurrentActor, 
				fmt.Sprintf("send-to %s (full)", targetName))
		}
		return Blocked(BlockQueueFull)
	}
}

// (receive!) - receive from own mailbox, blocks if empty
// AUTO-TRACES: asserts (received actor msg time) fact
func builtinReceive(ev *Evaluator, args []Value, env *Env) Value {
    ev.markGuardSeen() // CSP: mark guard seen
	if ev.Scheduler.CurrentActor == "" {
		fmt.Fprintln(os.Stderr, "receive!: no current actor")
		return Nil()
	}
	
	actor := ev.Scheduler.GetActor(ev.Scheduler.CurrentActor)
	if actor == nil {
		return Nil()
	}
	
	if msg, ok := actor.Mailbox.RecvNow(); ok {
		// AUTO-TRACE: log the receive as a fact
		ev.DatalogDB.AssertAtTime("received", ev.Scheduler.StepCount,
			Atom(ev.Scheduler.CurrentActor), ValueToTerm(msg))
		return msg
	} else {
		// Mailbox empty, block
		ev.Scheduler.BlockActor(ev.Scheduler.CurrentActor, "recv (empty)")
		return Blocked(BlockQueueEmpty)
	}
}

// (receive-now!) - non-blocking receive, returns 'empty if nothing
func builtinReceiveNow(ev *Evaluator, args []Value, env *Env) Value {
	if ev.Scheduler.CurrentActor == "" {
		fmt.Fprintln(os.Stderr, "receive-now!: no current actor")
		return Sym("empty")
	}
	
	actor := ev.Scheduler.GetActor(ev.Scheduler.CurrentActor)
	if actor == nil {
		return Sym("empty")
	}
	
	if msg, ok := actor.Mailbox.RecvNow(); ok {
		return msg
	}
	return Sym("empty")
}

// (mailbox-empty?) - check if own mailbox is empty
func builtinMailboxEmpty(ev *Evaluator, args []Value, env *Env) Value {
	if ev.Scheduler.CurrentActor == "" {
		return Bool(true)
	}
	actor := ev.Scheduler.GetActor(ev.Scheduler.CurrentActor)
	if actor == nil {
		return Bool(true)
	}
	return Bool(actor.Mailbox.IsEmpty())
}

// (mailbox-full? actor-name) - check if actor's mailbox is full
func builtinMailboxFull(ev *Evaluator, args []Value, env *Env) Value {
	var targetName string
	if len(args) > 0 {
		if args[0].Type == TypeSymbol {
			targetName = args[0].Symbol
		} else if args[0].Type == TypeString {
			targetName = args[0].Str
		}
	} else if ev.Scheduler.CurrentActor != "" {
		targetName = ev.Scheduler.CurrentActor
	} else {
		return Bool(false)
	}
	
	actor := ev.Scheduler.GetActor(targetName)
	if actor == nil {
		return Bool(false)
	}
	return Bool(actor.Mailbox.IsFull())
}

// (yield!) - voluntarily give up execution
func builtinYield(ev *Evaluator, args []Value, env *Env) Value {
	// This is a marker - the scheduler will handle it
	return Sym("yield")
}

// (done!) - mark current actor as finished
func builtinDone(ev *Evaluator, args []Value, env *Env) Value {
	if ev.Scheduler.CurrentActor != "" {
		ev.Scheduler.MarkDone(ev.Scheduler.CurrentActor)
	}
	return Sym("done")
}

// (run-scheduler max-steps) - run the scheduler
func builtinRunScheduler(ev *Evaluator, args []Value, env *Env) Value {
	maxSteps := int64(10000)
	if len(args) > 0 && args[0].Type == TypeNumber {
		maxSteps = int64(args[0].Number)
	}
	
	ev.Scheduler.MaxSteps = maxSteps
	ev.Scheduler.StepCount = 0
	
	for ev.Scheduler.StepCount < maxSteps {
		// Check termination conditions
		if ev.Scheduler.AllDone() {
			return Lst(Sym("completed"), Num(float64(ev.Scheduler.StepCount)))
		}
		if ev.Scheduler.IsDeadlocked() {
			// Return deadlock info
			blocked := make([]Value, 0)
			for name, actor := range ev.Scheduler.Actors {
				if actor.State == ActorBlocked {
					blocked = append(blocked, Lst(Sym(name), Str(actor.BlockedOn)))
				}
			}
			return Lst(Sym("deadlock"), Num(float64(ev.Scheduler.StepCount)), Lst(blocked...))
		}
		
		// Get next actor
		actor := ev.Scheduler.NextActor()
		if actor == nil {
			// No runnable actors but not deadlocked - all must be done
			return Lst(Sym("completed"), Num(float64(ev.Scheduler.StepCount)))
		}
        
		ev.resetCSPState(actor.Name) // CSP: reset for new step
		
		if ev.Scheduler.Trace {
			fmt.Printf("[%d] Running %s\n", ev.Scheduler.StepCount, actor.Name)
		}
		
		// Execute one step of actor's code
		if ev.Scheduler.Trace {
			fmt.Printf("    code: %s\n", actor.Code.String())
		}
		result := ev.Eval(actor.Code, actor.Env)
		actor.Result = result
		ev.Scheduler.StepCount++
		
		if ev.Scheduler.Trace {
			fmt.Printf("    result: %s\n", result.String())
		}
		
		// Check result
		if result.Type == TypeBlocked {
			// Already blocked by the operation
			if ev.Scheduler.Trace {
				fmt.Printf("    %s blocked: %s\n", actor.Name, actor.BlockedOn)
			}
		} else if result.Type == TypeSymbol && result.Symbol == "yield" {
			// Yielded voluntarily - stays runnable, re-run same code
			if ev.Scheduler.Trace {
				fmt.Printf("    %s yielded\n", actor.Name)
			}
		} else if result.Type == TypeSymbol && result.Symbol == "done" {
			// Actor finished
			ev.Scheduler.MarkDone(actor.Name)
			if ev.Scheduler.Trace {
				fmt.Printf("    %s done\n", actor.Name)
			}
		} else if result.IsList() && len(result.List) >= 2 {
			// Check for (next-state new-code) or (become new-code)
			if result.List[0].IsSymbol() && result.List[0].Symbol == "become" {
				// AUTO-TRACE: log state change
				oldState := extractStateName(actor.Code)
				newState := extractStateName(result.List[1])
				if oldState != newState {
					ev.DatalogDB.AssertAtTime("state-change", ev.Scheduler.StepCount,
						Atom(actor.Name), Atom(oldState), Atom(newState))
				}
				
				// Change actor's code
				actor.Code = result.List[1]
				if ev.Scheduler.Trace {
					fmt.Printf("    %s become %s\n", actor.Name, result.List[1].String())
				}
			} else if result.List[0].IsSymbol() && result.List[0].Symbol == "continue" {
				// Update code and keep running
				actor.Code = result.List[1]
			}
		}
		
		// Try to unblock actors whose conditions may have changed
		ev.tryUnblockActors()
	}
	
	return Lst(Sym("max-steps"), Num(float64(ev.Scheduler.StepCount)))
}

// extractStateName gets the function name from a code expression
// (counter-loop 5) → "counter-loop"
// (idle) → "idle"
// 'done → "done"
func extractStateName(code Value) string {
	if code.Type == TypeSymbol {
		return code.Symbol
	}
	if code.IsList() && len(code.List) > 0 {
		if code.List[0].Type == TypeSymbol {
			return code.List[0].Symbol
		}
	}
	return "unknown"
}

// Try to unblock actors that can now proceed
func (ev *Evaluator) tryUnblockActors() {
	for name, actor := range ev.Scheduler.Actors {
		if actor.State != ActorBlocked {
			continue
		}
		
		if strings.HasPrefix(actor.BlockedOn, "recv") {
			// Blocked on receive - check if mailbox now has messages
			if !actor.Mailbox.IsEmpty() {
				ev.Scheduler.UnblockActor(name)
			}
		} else if strings.HasPrefix(actor.BlockedOn, "send-to ") {
			// Blocked on send - check if target mailbox has space
			parts := strings.Split(actor.BlockedOn, " ")
			if len(parts) >= 2 {
				targetName := parts[1]
				target := ev.Scheduler.GetActor(targetName)
    ev.markGuardSeen() // CSP: send is a synchronization point
				if target != nil && !target.Mailbox.IsFull() {
					ev.Scheduler.UnblockActor(name)
				}
			}
		}
	}
}

// (scheduler-status) - print scheduler state
func builtinSchedulerStatus(ev *Evaluator, args []Value, env *Env) Value {
	fmt.Print(ev.Scheduler.Status())
	return Nil()
}

// (set-trace! bool) - enable/disable execution tracing
func builtinSetTrace(ev *Evaluator, args []Value, env *Env) Value {
	if len(args) > 0 {
		ev.Scheduler.Trace = args[0].IsTruthy()
	}
	return Bool(ev.Scheduler.Trace)
}

// (actor-state name) - get actor's current state
func builtinActorState(ev *Evaluator, args []Value, env *Env) Value {
	if len(args) == 0 {
		return Nil()
	}
	var name string
	if args[0].Type == TypeSymbol {
		name = args[0].Symbol
	} else if args[0].Type == TypeString {
		name = args[0].Str
	} else {
		return Nil()
	}
	
	actor := ev.Scheduler.GetActor(name)
	if actor == nil {
		return Nil()
	}
	
	state := "unknown"
	switch actor.State {
	case ActorRunnable:
		state = "runnable"
	case ActorBlocked:
		state = "blocked"
	case ActorDone:
		state = "done"
	}
	
	return Lst(
		Sym(state),
		Str(actor.BlockedOn),
		Num(float64(len(actor.Mailbox.Data))),
		Num(float64(actor.Mailbox.Capacity)),
	)
}

// (list-actors-sched) - list all actors in scheduler
func builtinListActorsSched(ev *Evaluator, args []Value, env *Env) Value {
	names := make([]Value, 0, len(ev.Scheduler.Actors))
	for name := range ev.Scheduler.Actors {
		names = append(names, Sym(name))
	}
	return Lst(names...)
}

// (reset-scheduler) - clear all actors and reset scheduler state
func builtinResetScheduler(ev *Evaluator, args []Value, env *Env) Value {
	ev.Scheduler = NewScheduler()
	return Sym("ok")
}

// ============================================================================
// REPL and File Execution
// ============================================================================

func countParens(s string) (int, int) {
	open := 0
	close := 0
	inString := false
	escaped := false
	for _, c := range s {
		if escaped {
			escaped = false
			continue
		}
		if c == '\\' && inString {
			escaped = true
			continue
		}
		if c == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		if c == '(' {
			open++
		} else if c == ')' {
			close++
		}
	}
	return open, close
}

func runREPL(ev *Evaluator) {
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("BoundedLISP - Type (exit) to quit")
	fmt.Print("> ")

	var accum strings.Builder
	openCount := 0
	closeCount := 0

	for scanner.Scan() {
		line := scanner.Text()
		
		if strings.TrimSpace(line) == "(exit)" && openCount == closeCount {
			break
		}

		accum.WriteString(line)
		accum.WriteString("\n")
		
		o, c := countParens(line)
		openCount += o
		closeCount += c

		// If parens are balanced and we have something, evaluate
		if openCount > 0 && openCount == closeCount {
			input := accum.String()
			accum.Reset()
			openCount = 0
			closeCount = 0

			parser := NewParser(input)
			exprs := parser.Parse()

			for _, expr := range exprs {
				result := ev.Eval(expr, nil)
				if result.Type != TypeNil {
					fmt.Println(result.String())
				}
			}
			fmt.Print("> ")
		} else if openCount > closeCount {
			// Need more input
			fmt.Print("  ")
		} else {
			// Unbalanced or empty line
			fmt.Print("> ")
		}
	}
}

func runFile(ev *Evaluator, filename string) {
	// LLM: we need to not start the web server with a web file, and consume the whole input as one prompt; not a thousand one line prompts.
	content, err := os.ReadFile(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
		os.Exit(1)
	}

	parser := NewParser(string(content))
	exprs := parser.Parse()

	for _, expr := range exprs {
		result := ev.Eval(expr, nil)
		if result.Type == TypeBlocked {
			fmt.Fprintf(os.Stderr, "Blocked: %v\n", result.Blocked.Reason)
		}
	}
}

func main() {
	ev := NewEvaluator(64) // 64 frame call stack limit

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "-mcp":
			runMCPServer()
			return
		case "-mcp-sse":
			port := "3000"
			if len(os.Args) > 2 {
				port = os.Args[2]
			}
			runMCPSSEServer(port)
			return
		case "-repl":
			runREPL(ev)
			return
		default:
			// File mode - run a .lisp file
			runFile(ev, os.Args[1])
			return
		}
	}

	// Default: web server mode
	port := os.Getenv("KRIPKE_PORT")
	if port == "" {
		port = "8080"
	}
	runServer(ev, port)
}

// ============================================================================
// Web Server for Requirements Chat
// ============================================================================

type Session struct {
	ID           string
	Messages     []ChatMessage
	Versions     []DocVersion
	CurrentDoc   string
	CreatedAt    time.Time
	InputTokens  int
	OutputTokens int
	Evaluator    *Evaluator  // For LISP eval and Datalog facts
	mu           sync.Mutex
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type DocVersion struct {
	Version   int       `json:"version"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
	Summary   string    `json:"summary"`
}

var (
	sessions   = make(map[string]*Session)
	sessionsMu sync.RWMutex
)

func getOrCreateSession(id string) *Session {
	sessionsMu.Lock()
	defer sessionsMu.Unlock()
	
	if sess, ok := sessions[id]; ok {
		return sess
	}
	
	sess := &Session{
		ID:         id,
		Messages:   []ChatMessage{},
		Versions:   []DocVersion{},
		CurrentDoc: "",
		CreatedAt:  time.Now(),
		Evaluator:  NewEvaluator(1000),  // Per-session evaluator
	}
	sessions[id] = sess
	return sess
}

func runServer(ev *Evaluator, port string) {
	// Load LISP modules
	loadLispModules(ev)
	
	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/chat", handleChat)
	http.HandleFunc("/versions", handleVersions)
	http.HandleFunc("/version/", handleGetVersion)
	http.HandleFunc("/eval", handleEval(ev))
	http.HandleFunc("/properties", handleProperties(ev))
	http.HandleFunc("/diagram", handleDiagram(ev))
	http.HandleFunc("/facts", handleFacts)  // Debug: show session facts
	
	// Check for API keys
	hasAnthropic := os.Getenv("ANTHROPIC_API_KEY") != ""
	hasOpenAI := os.Getenv("OPENAI_API_KEY") != ""
	hasGemini := os.Getenv("GEMINI_API_KEY") != ""
	
	fmt.Println("╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║            BoundedLISP - Philosophy Calculator              ║")
	fmt.Println("╠════════════════════════════════════════════════════════════╣")
	fmt.Printf("║  Web UI: http://localhost:%-33s║\n", port)
	fmt.Println("╠════════════════════════════════════════════════════════════╣")
	if hasAnthropic {
		fmt.Println("║  ✓ ANTHROPIC_API_KEY set                                   ║")
	} else {
		fmt.Println("║  ✗ ANTHROPIC_API_KEY not set                               ║")
	}
	if hasOpenAI {
		fmt.Println("║  ✓ OPENAI_API_KEY set                                      ║")
	} else {
		fmt.Println("║  ✗ OPENAI_API_KEY not set                                  ║")
	}
	if hasGemini {
		fmt.Println("║  ✓ GEMINI_API_KEY set                                      ║")
	} else {
		fmt.Println("║  ✗ GEMINI_API_KEY not set                                  ║")
	}
	fmt.Println("╠════════════════════════════════════════════════════════════╣")
	fmt.Println("║  Type here for quick queries, or use the web UI            ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")
	fmt.Println()
	
	// Start HTTP server in background
	go func() {
		if err := http.ListenAndServe(":"+port, nil); err != nil {
			fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
			os.Exit(1)
		}
	}()
	
	// Console input loop
	runConsoleChat(hasAnthropic, hasOpenAI, hasGemini)
}

// Console chat - type queries while server is running
func runConsoleChat(hasAnthropic, hasOpenAI, hasGemini bool) {
	if !hasAnthropic && !hasOpenAI && !hasGemini {
		// No API keys, just block forever
		select {}
	}
	
	reader := bufio.NewReader(os.Stdin)
	sess := getOrCreateSession("console")
	
	provider := "anthropic"
	if !hasAnthropic && hasOpenAI {
		provider = "openai"
	} else if !hasAnthropic && !hasOpenAI && hasGemini {
		provider = "gemini"
	}
	
	for {
		fmt.Print("\n> ")
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		
		if line == "exit" || line == "quit" {
			fmt.Println("Goodbye!")
			os.Exit(0)
		}
		
		// Get API key
		var apiKey string
		if provider == "openai" {
			apiKey = os.Getenv("OPENAI_API_KEY")
		} else if provider == "gemini" {
			apiKey = os.Getenv("GEMINI_API_KEY")
		} else {
			apiKey = os.Getenv("ANTHROPIC_API_KEY")
		}
		
		sess.mu.Lock()
		sess.Messages = append(sess.Messages, ChatMessage{Role: "user", Content: line})
		
		var response string
		var inTok, outTok int
		if provider == "openai" {
			response, inTok, outTok, err = callOpenAI(apiKey, sess.Messages)
		} else if provider == "gemini" {
			response, inTok, outTok, err = callGemini(apiKey, sess.Messages)
		} else {
			response, inTok, outTok, err = callAnthropic(apiKey, sess.Messages)
		}
		
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			sess.mu.Unlock()
			continue
		}
		
		sess.InputTokens += inTok
		sess.OutputTokens += outTok
		sess.Messages = append(sess.Messages, ChatMessage{Role: "assistant", Content: response})
		totalTokens := sess.InputTokens + sess.OutputTokens
		sess.mu.Unlock()
		
		// Parse and display
		chat, _, _ := parseStructuredResponse(response)
		fmt.Println()
		fmt.Println(chat)
		fmt.Printf("\n[tokens: %d in + %d out = %d total]\n", sess.InputTokens, sess.OutputTokens, totalTokens)
	}
}

func loadLispModules(ev *Evaluator) {
	// Try to load standard modules
	modules := []string{"prologue.lisp"}
	for _, mod := range modules {
		if content, err := os.ReadFile(mod); err == nil {
			parser := NewParser(string(content))
			for _, expr := range parser.Parse() {
				ev.Eval(expr, nil)
			}
		}
	}
}

const systemPrompt = `You are a requirements engineer helping users specify multi-party protocols.

## CRITICAL: What Users See

Users see DIAGRAMS and TABLES, not code. Never show LISP or Datalog to users.

## Tool Placeholders

Use these in your MARKDOWN section - they render automatically:

{{facts_table}}                                    - summary of all facts by predicate  
{{facts_table predicate="sale"}}                   - facts for specific predicate
{{property formula="AG(inv >= 0)"}}                - verification result
{{metrics_chart title="X" predicates="sent,received"}}  - cumulative chart over time

## Output Format

ALWAYS use THREE sections:

===CHAT===
Brief response (1-3 sentences).

===MARKDOWN===
Diagrams via mermaid fences, tool placeholders, English explanations.
NEVER include LISP code blocks here.

===LISP===
Internal code only - users don't see this section.

## ═══════════════════════════════════════════════════════════════════════════
## VALID BOUNDEDLISP CONSTRUCTS (use ONLY these)
## ═══════════════════════════════════════════════════════════════════════════

### Core Forms
(define name value)           ; global definition
(define (fn x y) body)        ; function definition
(let x 5 (+ x 1))             ; single binding
(let* ((x 5) (y 6)) body)     ; multiple bindings
(lambda (x) body)             ; anonymous function
(if test then else)           ; conditional
(cond (test1 expr1) ...)      ; multi-branch

### Lists
(list 1 2 3)                  ; create list
(car lst) (cdr lst)           ; first / rest
(cons x lst)                  ; prepend
(nth lst i)                   ; index
(empty? lst)                  ; check empty

### Actors - CRITICAL PATTERN

Actor loops MUST use (list 'become ...) to continue, NOT recursion!

CORRECT pattern:
(define (my-actor state)
  (let msg (receive!)                    ; ALWAYS receive first
    (cond
      ((eq? (nth msg 0) 'do-x)           ; use eq? for comparison
       (send-to! 'other (list 'response))
       (list 'become (list 'my-actor (+ state 1))))  ; MUST use become
      (else
       (list 'become (list 'my-actor state))))))

WRONG patterns (do NOT use):
  (loop)                    ; NO - don't use recursion, use (list 'become ...)
  (recur)                   ; NO - use (list 'become ...)

REQUIRED to run simulation:
(spawn-actor 'name 10 '(fn args))   ; spawn each actor
(run-scheduler 100)                  ; run the simulation!

### Auto-Tracing (happens automatically!)
Every spawn/send/receive creates facts:
  (spawned actor-name time)
  (sent from to msg time)
  (received actor msg time)
  (state-change actor old-state new-state time)

### Facts (Datalog) - Simple patterns only!
(assert! 'predicate 'arg1 'arg2)      ; add custom fact
(query 'predicate '?x '?y)            ; query facts

Datalog does NOT support: not, negation, aggregation, arithmetic in rules

### Temporal
(never? '(bad-state ?x))      ; AG(not ...)
(eventually? '(goal ?x))      ; EF(...)
(always? '(invariant ?x))     ; AG(...)

### NOT VALID (these don't exist - never use them):
- state-machine, transition, next-state, initial-state
- loop, recur (use (list 'become ...) instead)
- rule with 'not' or negation (Datalog doesn't support negation)
- define-rule (use rule)

## Mermaid Syntax

FORBIDDEN in labels (causes parse errors):
- := (use '= instead)
- >= <= != (use gte, lte, neq instead)

## Example Response

===CHAT===
I've modeled a producer-consumer simulation with message tracing.

===MARKDOWN===
## Actor State Machines

` + "```mermaid" + `
stateDiagram-v2
    [*] --> Producing
    Producing --> Producing: produce, send to consumer
    Producing --> [*]: done after 5 items
` + "```" + `

## Message Traffic Over Time

{{metrics_chart title="Producer-Consumer Traffic" predicates="sent,received"}}

## Collected Facts (auto-traced)

{{facts_table}}

===LISP===
;; Producer sends 5 items to consumer
(define (producer n)
  (if (> n 0)
    (begin
      (send-to! 'consumer (list 'item n))
      (list 'become (list 'producer (- n 1))))
    (done!)))

;; Consumer receives and acknowledges
(define (consumer)
  (let msg (receive!)
    (assert! 'processed (nth msg 1))
    (list 'become '(consumer))))

;; Spawn actors and run simulation
(spawn-actor 'producer 10 '(producer 5))
(spawn-actor 'consumer 10 '(consumer))
(run-scheduler 50)
`

func handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(indexHTML))
}

func handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	
	var req struct {
		SessionID string `json:"session_id"`
		Message   string `json:"message"`
		Provider  string `json:"provider"` // "anthropic" or "openai"
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	
	// Get API key from environment
	var apiKey string
	if req.Provider == "openai" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	} else if req.Provider == "gemini" {
		apiKey = os.Getenv("GEMINI_API_KEY")
	} else {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}
	
	if apiKey == "" {
		http.Error(w, "API key not set in environment. Set ANTHROPIC_API_KEY, OPENAI_API_KEY, or GEMINI_API_KEY", http.StatusBadRequest)
		return
	}
	
	sess := getOrCreateSession(req.SessionID)
	sess.mu.Lock()
	defer sess.mu.Unlock()
	
	// Add user message
	sess.Messages = append(sess.Messages, ChatMessage{Role: "user", Content: req.Message})
	
	// Call LLM
	var response string
	var inTok, outTok int
	var err error
	
	if req.Provider == "openai" {
		response, inTok, outTok, err = callOpenAI(apiKey, sess.Messages)
	} else if req.Provider == "gemini" {
		response, inTok, outTok, err = callGemini(apiKey, sess.Messages)
	} else {
		response, inTok, outTok, err = callAnthropic(apiKey, sess.Messages)
	}
	
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	// Track usage
	sess.InputTokens += inTok
	sess.OutputTokens += outTok
	
	// Add assistant message
	sess.Messages = append(sess.Messages, ChatMessage{Role: "assistant", Content: response})
	
	// Parse the structured response
	chatResponse, markdown, lisp := parseStructuredResponse(response)
	
	// Execute LISP code to populate DatalogDB with facts
	if lisp != "" {
		parser := NewParser(lisp)
		exprs := parser.Parse()
		for _, expr := range exprs {
			sess.Evaluator.Eval(expr, sess.Evaluator.GlobalEnv)
		}
	}
	
	// Process tool placeholders in markdown (AFTER executing LISP so facts exist)
	toolRegistry := NewToolRegistry(sess.Evaluator)
	markdown = toolRegistry.Process(markdown)
	
	// Store LISP as current doc
	if lisp != "" {
		if lisp != sess.CurrentDoc {
			sess.Versions = append(sess.Versions, DocVersion{
				Version:   len(sess.Versions) + 1,
				Content:   lisp,
				Timestamp: time.Now(),
				Summary:   "Update",
			})
			sess.CurrentDoc = lisp
		}
	}
	
	json.NewEncoder(w).Encode(map[string]interface{}{
		"chat_response": chatResponse,
		"markdown":      markdown,
		"current_doc":   sess.CurrentDoc,
		"version":       len(sess.Versions),
		"usage": map[string]int{
			"input_tokens":  sess.InputTokens,
			"output_tokens": sess.OutputTokens,
			"total_tokens":  sess.InputTokens + sess.OutputTokens,
		},
	})
}

// Parse structured response with ===CHAT===, ===MARKDOWN===, ===LISP=== sections
func parseStructuredResponse(response string) (chat, markdown, lisp string) {
	// Default: treat entire response as chat
	chat = response
	
	// Try to find sections
	chatIdx := strings.Index(response, "===CHAT===")
	mdIdx := strings.Index(response, "===MARKDOWN===")
	lispIdx := strings.Index(response, "===LISP===")
	
	if chatIdx >= 0 && mdIdx >= 0 {
		// Extract chat section
		chatStart := chatIdx + len("===CHAT===")
		chatEnd := mdIdx
		chat = strings.TrimSpace(response[chatStart:chatEnd])
		
		// Extract markdown section
		mdStart := mdIdx + len("===MARKDOWN===")
		mdEnd := len(response)
		if lispIdx > mdIdx {
			mdEnd = lispIdx
		}
		markdown = strings.TrimSpace(response[mdStart:mdEnd])
		
		// Extract LISP section if present
		if lispIdx >= 0 {
			lispStart := lispIdx + len("===LISP===")
			lisp = strings.TrimSpace(response[lispStart:])
			lisp = cleanLispSection(lisp)
		}
	} else {
		// Fallback: try to extract LISP from code blocks
		if spec := extractSpec(response); spec != "" {
			lisp = spec
		}
		// Use full response as both chat and markdown
		markdown = response
	}
	
	return
}

// cleanLispSection removes markdown artifacts that LLMs sometimes add to LISP sections
func cleanLispSection(lisp string) string {
	lines := strings.Split(lisp, "\n")
	var cleanLines []string
	inCodeBlock := false
	
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		// Skip markdown headers
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		
		// Skip prose lines (heuristic: starts with capital letter and has spaces, not a LISP form)
		if len(trimmed) > 0 && trimmed[0] >= 'A' && trimmed[0] <= 'Z' && 
		   strings.Contains(trimmed, " ") && !strings.HasPrefix(trimmed, "(") {
			continue
		}
		
		// Handle code fences
		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
			continue
		}
		
		// Skip empty lines at the start
		if len(cleanLines) == 0 && trimmed == "" {
			continue
		}
		
		// Keep the line (either inside code block or looks like LISP)
		if trimmed == "" || strings.HasPrefix(trimmed, "(") || strings.HasPrefix(trimmed, ";") ||
		   strings.HasPrefix(trimmed, "'") || inCodeBlock {
			cleanLines = append(cleanLines, line)
		}
	}
	
	return strings.TrimSpace(strings.Join(cleanLines, "\n"))
}

func callAnthropic(apiKey string, messages []ChatMessage) (string, int, int, error) {
	if apiKey == "" {
		return "", 0, 0, fmt.Errorf("API key required")
	}
	
	// Build messages array
	msgs := make([]map[string]string, len(messages))
	for i, m := range messages {
		msgs[i] = map[string]string{"role": m.Role, "content": m.Content}
	}
	
	reqBody := map[string]interface{}{
		"model":      "claude-sonnet-4-20250514",
		"max_tokens": 4096,
		"system":     systemPrompt,
		"messages":   msgs,
	}
	
	body, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", 0, 0, err
	}
	defer resp.Body.Close()
	
	respBody, _ := io.ReadAll(resp.Body)
	
	if resp.StatusCode != 200 {
		return "", 0, 0, fmt.Errorf("API error: %s", string(respBody))
	}
	
	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	json.Unmarshal(respBody, &result)
	
	if len(result.Content) > 0 {
		return result.Content[0].Text, result.Usage.InputTokens, result.Usage.OutputTokens, nil
	}
	return "", 0, 0, fmt.Errorf("empty response")
}

func callOpenAI(apiKey string, messages []ChatMessage) (string, int, int, error) {
	if apiKey == "" {
		return "", 0, 0, fmt.Errorf("API key required")
	}
	
	// Build messages with system prompt
	msgs := []map[string]string{{"role": "system", "content": systemPrompt}}
	for _, m := range messages {
		msgs = append(msgs, map[string]string{"role": m.Role, "content": m.Content})
	}
	
	reqBody := map[string]interface{}{
		"model":      "gpt-4o",
		"max_tokens": 4096,
		"messages":   msgs,
	}
	
	body, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", 0, 0, err
	}
	defer resp.Body.Close()
	
	respBody, _ := io.ReadAll(resp.Body)
	
	if resp.StatusCode != 200 {
		return "", 0, 0, fmt.Errorf("API error: %s", string(respBody))
	}
	
	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	json.Unmarshal(respBody, &result)
	
	if len(result.Choices) > 0 {
		return result.Choices[0].Message.Content, result.Usage.PromptTokens, result.Usage.CompletionTokens, nil
	}
	return "", 0, 0, fmt.Errorf("empty response")
}

func callGemini(apiKey string, messages []ChatMessage) (string, int, int, error) {
	if apiKey == "" {
		return "", 0, 0, fmt.Errorf("API key required")
	}
	
	// Build contents array for Gemini
	// Gemini uses "user" and "model" roles, and system instruction is separate
	contents := make([]map[string]interface{}, 0)
	for _, m := range messages {
		role := m.Role
		if role == "assistant" {
			role = "model"
		}
		contents = append(contents, map[string]interface{}{
			"role": role,
			"parts": []map[string]string{
				{"text": m.Content},
			},
		})
	}
	
	reqBody := map[string]interface{}{
		"contents": contents,
		"systemInstruction": map[string]interface{}{
			"parts": []map[string]string{
				{"text": systemPrompt},
			},
		},
		"generationConfig": map[string]interface{}{
			"maxOutputTokens": 4096,
			"temperature":     0.7,
		},
	}
	
	// Use gemini-2.0-flash model
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash:generateContent?key=%s", apiKey)
	
	body, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", 0, 0, err
	}
	defer resp.Body.Close()
	
	respBody, _ := io.ReadAll(resp.Body)
	
	if resp.StatusCode != 200 {
		return "", 0, 0, fmt.Errorf("API error: %s", string(respBody))
	}
	
	var result struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
		UsageMetadata struct {
			PromptTokenCount     int `json:"promptTokenCount"`
			CandidatesTokenCount int `json:"candidatesTokenCount"`
		} `json:"usageMetadata"`
	}
	json.Unmarshal(respBody, &result)
	
	if len(result.Candidates) > 0 && len(result.Candidates[0].Content.Parts) > 0 {
		return result.Candidates[0].Content.Parts[0].Text, 
			result.UsageMetadata.PromptTokenCount, 
			result.UsageMetadata.CandidatesTokenCount, nil
	}
	return "", 0, 0, fmt.Errorf("empty response")
}

func extractSpec(response string) string {
	// Look for markdown code blocks with lisp
	lines := strings.Split(response, "\n")
	var spec strings.Builder
	inBlock := false
	
	for _, line := range lines {
		if strings.HasPrefix(line, "```lisp") || strings.HasPrefix(line, "```scheme") {
			inBlock = true
			continue
		}
		if inBlock && strings.HasPrefix(line, "```") {
			inBlock = false
			spec.WriteString("\n")
			continue
		}
		if inBlock {
			spec.WriteString(line)
			spec.WriteString("\n")
		}
	}
	
	return strings.TrimSpace(spec.String())
}

func extractSummary(spec string) string {
	// Extract grammar name as summary
	if idx := strings.Index(spec, "defgrammar"); idx >= 0 {
		rest := spec[idx:]
		if start := strings.Index(rest, "'"); start >= 0 {
			end := strings.IndexAny(rest[start+1:], " \n\t)")
			if end > 0 {
				return rest[start+1 : start+1+end]
			}
		}
	}
	return "Draft"
}

func handleVersions(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("session_id")
	sess := getOrCreateSession(sessionID)
	
	sess.mu.Lock()
	defer sess.mu.Unlock()
	
	json.NewEncoder(w).Encode(sess.Versions)
}

func handleGetVersion(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 {
		http.Error(w, "version number required", http.StatusBadRequest)
		return
	}
	
	versionNum, err := strconv.Atoi(parts[2])
	if err != nil {
		http.Error(w, "invalid version", http.StatusBadRequest)
		return
	}
	
	sessionID := r.URL.Query().Get("session_id")
	sess := getOrCreateSession(sessionID)
	
	sess.mu.Lock()
	defer sess.mu.Unlock()
	
	if versionNum < 1 || versionNum > len(sess.Versions) {
		http.Error(w, "version not found", http.StatusNotFound)
		return
	}
	
	json.NewEncoder(w).Encode(sess.Versions[versionNum-1])
}

func handleFacts(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("session_id")
	sess := getOrCreateSession(sessionID)
	
	sess.mu.Lock()
	defer sess.mu.Unlock()
	
	w.Header().Set("Content-Type", "application/json")
	
	// Collect facts by predicate
	factsByPred := make(map[string][]map[string]interface{})
	for _, fact := range sess.Evaluator.DatalogDB.Facts {
		args := make([]string, len(fact.Args))
		for i, arg := range fact.Args {
			args[i] = arg.String()
		}
		factsByPred[fact.Predicate] = append(factsByPred[fact.Predicate], map[string]interface{}{
			"args": args,
			"time": fact.Time,
		})
	}
	
	json.NewEncoder(w).Encode(map[string]interface{}{
		"total_facts": len(sess.Evaluator.DatalogDB.Facts),
		"by_predicate": factsByPred,
		"rules": len(sess.Evaluator.DatalogDB.Rules),
	})
}

func handleEval(ev *Evaluator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Code string `json:"code"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		
		// Capture stdout/stderr for error detection
		var output strings.Builder
		var errors []string
		
		// Parse the code
		parser := NewParser(req.Code)
		exprs := parser.Parse()
		
		var results []string
		for _, expr := range exprs {
			result := ev.Eval(expr, nil)
			resultStr := result.String()
			results = append(results, resultStr)
			
			// Check for error indicators
			if strings.HasPrefix(resultStr, "Error:") || 
			   strings.HasPrefix(resultStr, "Undefined symbol:") ||
			   strings.HasPrefix(resultStr, "Parse error:") ||
			   strings.Contains(resultStr, "not a function") ||
			   strings.Contains(resultStr, "wrong number of arguments") ||
			   strings.Contains(resultStr, "expected") {
				errors = append(errors, resultStr)
			}
			output.WriteString(resultStr)
			output.WriteString("\n")
		}
		
		json.NewEncoder(w).Encode(map[string]interface{}{
			"results": results,
			"output":  output.String(),
			"errors":  errors,
			"success": len(errors) == 0,
		})
	}
}

func handleProperties(ev *Evaluator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Evaluate (properties->display) to get list of (name latex) pairs
		parser := NewParser("(properties->display)")
		exprs := parser.Parse()
		
		type Property struct {
			Name  string `json:"name"`
			LaTeX string `json:"latex"`
		}
		
		var properties []Property
		
		if len(exprs) > 0 {
			result := ev.Eval(exprs[0], nil)
			// Result should be a list of (name latex) pairs
			if result.Type == TypeList {
				for _, item := range result.List {
					if item.Type == TypeList && len(item.List) >= 2 {
						name := ""
						latex := ""
						if item.List[0].Type == TypeSymbol {
							name = item.List[0].Symbol
						}
						if item.List[1].Type == TypeString {
							latex = item.List[1].Str
						}
						if name != "" && latex != "" {
							properties = append(properties, Property{Name: name, LaTeX: latex})
						}
					}
				}
			}
		}
		
		json.NewEncoder(w).Encode(map[string]interface{}{
			"properties": properties,
		})
	}
}

func handleDiagram(ev *Evaluator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// POST: AI-powered sketch interpretation
		if r.Method == "POST" {
			var req struct {
				Sketch   string `json:"sketch"`
				Provider string `json:"provider"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			
			// Get API key
			var apiKey string
			if req.Provider == "openai" {
				apiKey = os.Getenv("OPENAI_API_KEY")
			} else if req.Provider == "gemini" {
				apiKey = os.Getenv("GEMINI_API_KEY")
			} else {
				apiKey = os.Getenv("ANTHROPIC_API_KEY")
			}
			
			if apiKey == "" {
				json.NewEncoder(w).Encode(map[string]string{"error": "No API key configured"})
				return
			}
			
			// Ask LLM to interpret sketch and generate mermaid diagrams
			prompt := `Interpret this whiteboard sketch and generate Mermaid diagrams.

The sketch may contain multiple sections:
- Message flows like "A -> B: message" → generate sequenceDiagram
- State transitions like "Idle --> Waiting" → generate stateDiagram-v2  
- Natural language notes/commands → apply them to nearby diagrams (e.g. "color X red", "make vertical")

Generate ALL relevant diagrams. Separate multiple diagrams with ===DIAGRAM=== on its own line.

Respond with ONLY mermaid code, no explanations, no markdown fences.

Sketch:
` + req.Sketch
			
			messages := []ChatMessage{{Role: "user", Content: prompt}}
			
			var response string
			var err error
			if req.Provider == "openai" {
				response, _, _, err = callOpenAI(apiKey, messages)
			} else if req.Provider == "gemini" {
				response, _, _, err = callGemini(apiKey, messages)
			} else {
				response, _, _, err = callAnthropic(apiKey, messages)
			}
			
			if err != nil {
				json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}
			
			// Clean up response and split into diagrams
			response = strings.TrimSpace(response)
			response = strings.ReplaceAll(response, "```mermaid", "")
			response = strings.ReplaceAll(response, "```", "")
			
			// Split by delimiter
			parts := strings.Split(response, "===DIAGRAM===")
			var diagrams []string
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if p != "" {
					diagrams = append(diagrams, p)
				}
			}
			
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"diagrams": diagrams,
				"mermaid":  strings.Join(diagrams, "\n"),  // backward compat
			})
			return
		}
		
		// GET: Grammar-based diagram generation (legacy)
		grammarName := r.URL.Query().Get("grammar")
		diagramType := r.URL.Query().Get("type")
		if diagramType == "" {
			diagramType = "state"
		}
		
		var code string
		switch diagramType {
		case "state":
			code = fmt.Sprintf("(grammar->state-diagram '%s)", grammarName)
		case "sequence":
			code = fmt.Sprintf("(grammar->sequence '%s)", grammarName)
		case "flowchart":
			code = fmt.Sprintf("(grammar->flowchart '%s)", grammarName)
		default:
			http.Error(w, "unknown diagram type", http.StatusBadRequest)
			return
		}
		
		parser := NewParser(code)
		exprs := parser.Parse()
		
		var result string
		for _, expr := range exprs {
			r := ev.Eval(expr, nil)
			if r.Type == TypeString {
				result = r.Str
			}
		}
		
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(result))
	}
}

const indexHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>BoundedLISP - Philosophy Calculator</title>
    <script src="https://cdn.jsdelivr.net/npm/mermaid/dist/mermaid.min.js"></script>
    <script src="https://cdn.jsdelivr.net/npm/marked/marked.min.js"></script>
    <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/katex@0.16.9/dist/katex.min.css">
    <script src="https://cdn.jsdelivr.net/npm/katex@0.16.9/dist/katex.min.js"></script>
    <style>
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body { 
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background: #0d1117; color: #c9d1d9; 
            display: flex; height: 100vh;
        }
        
        /* Three-panel layout */
        .panel {
            display: flex; flex-direction: column;
            border-right: 1px solid #30363d;
        }
        .panel:last-child { border-right: none; }
        .panel-header {
            padding: 0.6rem 1rem; background: #161b22; border-bottom: 1px solid #30363d;
            display: flex; align-items: center; gap: 0.75rem;
            font-weight: 600; font-size: 0.9rem;
        }
        .panel-header .title { color: #58a6ff; }
        .panel-header .spacer { flex: 1; }
        .panel-header select, .panel-header button {
            padding: 0.35rem 0.6rem; border: 1px solid #30363d; border-radius: 4px;
            background: #21262d; color: #c9d1d9; font-size: 0.8rem; cursor: pointer;
        }
        .panel-header button:hover { background: #30363d; }
        .panel-header button.primary { background: #238636; border-color: #238636; }
        .panel-header button.primary:hover { background: #2ea043; }
        .panel-header .usage { 
            font-size: 0.75rem; color: #8b949e; padding: 0.25rem 0.5rem;
            background: #21262d; border-radius: 4px; margin-right: 0.5rem;
        }
        .panel-header .usage.warn { color: #d29922; background: #3d2e00; }
        .panel-header .usage.danger { color: #f85149; background: #3d0000; }
        
        /* Chat Panel */
        .chat-panel { width: 35%; min-width: 320px; }
        .messages { flex: 1; overflow-y: auto; padding: 0.75rem; }
        .message { margin: 0.5rem 0; padding: 0.6rem 0.8rem; border-radius: 6px; font-size: 0.9rem; line-height: 1.5; }
        .message.user { background: #1f6feb22; margin-left: 8%; border: 1px solid #1f6feb44; }
        .message.assistant { background: #21262d; }
        .message p { margin: 0.4rem 0; }
        .message pre { background: #161b22; padding: 0.5rem; border-radius: 4px; overflow-x: auto; margin: 0.5rem 0; font-size: 0.8rem; }
        .message code { font-family: 'Fira Code', monospace; font-size: 0.8rem; }
        .message ul, .message ol { margin: 0.4rem 0 0.4rem 1.25rem; font-size: 0.85rem; }
        .input-area { padding: 0.75rem; background: #161b22; border-top: 1px solid #30363d; display: flex; gap: 0.5rem; }
        .input-area textarea { 
            flex: 1; padding: 0.6rem; border: 1px solid #30363d; border-radius: 4px;
            background: #0d1117; color: #c9d1d9; resize: none; font-family: inherit; font-size: 0.9rem;
        }
        .input-area textarea:focus { outline: none; border-color: #58a6ff; }
        .input-area button {
            padding: 0.5rem 1rem; background: #238636; border: none;
            border-radius: 4px; color: #fff; cursor: pointer; font-size: 0.85rem;
        }
        .input-area button:hover { background: #2ea043; }
        
        /* Whiteboard (in Spec Panel tab) */
        .whiteboard-area { flex: 1; display: flex; flex-direction: column; overflow: hidden; }
        .whiteboard-input {
            flex: 1; display: flex; flex-direction: column;
        }
        .whiteboard-input textarea {
            flex: 1; padding: 1rem; border: none; background: #0d1117; color: #c9d1d9;
            font-family: 'Fira Code', monospace; font-size: 0.9rem; resize: none; line-height: 1.5;
        }
        .whiteboard-input textarea:focus { outline: none; }
        .whiteboard-preview {
            height: 45%; border-top: 1px solid #30363d; overflow-y: auto;
            padding: 1rem; background: #161b22;
        }
        .whiteboard-preview .mermaid { background: transparent; }
        .whiteboard-preview .katex-display { margin: 0.5rem 0; }
        .whiteboard-preview .error { color: #f85149; font-size: 0.85rem; }
        .whiteboard-controls {
            padding: 0.5rem 0.75rem; background: #161b22; border-bottom: 1px solid #30363d;
            display: flex; gap: 0.5rem; align-items: center;
        }
        .whiteboard-controls button {
            padding: 0.35rem 0.6rem; border: 1px solid #30363d; border-radius: 4px;
            background: #21262d; color: #c9d1d9; font-size: 0.8rem; cursor: pointer;
        }
        .whiteboard-controls button:hover { background: #30363d; }
        .whiteboard-controls button.primary { background: #238636; border-color: #238636; }
        .whiteboard-controls button.primary:hover { background: #2ea043; }
        .whiteboard-controls .spacer { flex: 1; }
        .tab-content { display: none; flex: 1; flex-direction: column; overflow: hidden; }
        .tab-content.active { display: flex; }
        
        /* Spec Panel */
        .spec-panel { flex: 1; min-width: 400px; }
        .spec-tabs { 
            display: flex; background: #161b22; border-bottom: 1px solid #30363d;
        }
        .spec-tab { 
            padding: 0.6rem 1.25rem; cursor: pointer; 
            border-bottom: 2px solid transparent;
            font-size: 0.85rem; color: #8b949e;
        }
        .spec-tab:hover { color: #c9d1d9; }
        .spec-tab.active { color: #58a6ff; border-bottom-color: #58a6ff; }
        .spec-content { flex: 1; overflow-y: auto; padding: 1.25rem; }
        
        /* Markdown styles */
        .markdown { color: #c9d1d9; line-height: 1.6; }
        .markdown h1 { color: #58a6ff; font-size: 1.5rem; margin: 1.25rem 0 0.75rem; padding-bottom: 0.25em; border-bottom: 1px solid #30363d; }
        .markdown h2 { color: #58a6ff; font-size: 1.2rem; margin: 1.25rem 0 0.5rem; padding-bottom: 0.25em; border-bottom: 1px solid #30363d; }
        .markdown h3 { color: #8b949e; font-size: 1rem; margin: 1rem 0 0.4rem; }
        .markdown p { margin: 0.6rem 0; }
        .markdown pre { background: #161b22; padding: 0.75rem; border-radius: 4px; overflow-x: auto; margin: 0.75rem 0; border: 1px solid #30363d; }
        .markdown code { font-family: 'Fira Code', monospace; font-size: 0.85rem; color: #79c0ff; }
        .markdown pre code { color: #c9d1d9; }
        .markdown ul, .markdown ol { margin: 0.6rem 0 0.6rem 1.5rem; }
        .markdown li { margin: 0.25rem 0; }
        .markdown table { border-collapse: collapse; width: 100%; margin: 0.75rem 0; font-size: 0.9rem; }
        .markdown th, .markdown td { border: 1px solid #30363d; padding: 0.4rem 0.6rem; text-align: left; }
        .markdown th { background: #161b22; color: #58a6ff; }
        .markdown .mermaid { background: #161b22; padding: 0.75rem; border-radius: 4px; margin: 0.75rem 0; }
        
        .code-view pre { 
            background: #161b22; color: #c9d1d9; padding: 1rem; 
            border-radius: 4px; margin: 0; font-size: 0.85rem;
            white-space: pre-wrap; word-wrap: break-word; border: 1px solid #30363d;
        }
        
        .properties-view { padding: 1rem; }
        .property-item {
            background: #161b22; border: 1px solid #30363d; border-radius: 6px;
            padding: 0.75rem 1rem; margin-bottom: 0.75rem;
        }
        .property-name {
            color: #58a6ff; font-weight: 600; font-size: 0.9rem;
            margin-bottom: 0.5rem; font-family: 'Fira Code', monospace;
        }
        .property-formula {
            color: #c9d1d9; font-size: 1.1rem; padding: 0.5rem;
            background: #0d1117; border-radius: 4px; text-align: center;
            overflow-x: auto;
        }
        .property-formula .katex { font-size: 1.1rem; }
        
        .empty-state { display: flex; align-items: center; justify-content: center; height: 100%; color: #8b949e; font-style: italic; }
    </style>
</head>
<body>
    <!-- Chat Panel -->
    <div class="panel chat-panel">
        <div class="panel-header">
            <span class="title">💬 Chat</span>
            <span class="spacer"></span>
            <span id="usage" class="usage" title="Session token usage"></span>
            <select id="provider">
                <option value="openai">GPT-4</option>
                <option value="anthropic">Claude</option>
                <option value="gemini">Gemini</option>
            </select>
        </div>
        <div class="messages" id="messages">
            <div class="message assistant">
                <p>Let's design a protocol together.</p>
                <p>Use the <strong>Whiteboard</strong> tab to sketch ideas. Message flows like <code>A -> B: msg</code> and state transitions like <code>X --> Y</code> will render as diagrams. Click <strong>Formalize</strong> when ready.</p>
            </div>
        </div>
        <div class="input-area">
            <textarea id="input" rows="2" placeholder="Describe or ask..."></textarea>
            <button onclick="sendMessage()">Send</button>
        </div>
    </div>
    
    <!-- Specification Panel (with Whiteboard tab) -->
    <div class="panel spec-panel">
        <div class="panel-header">
            <span class="title">📋 Specification</span>
        </div>
        <div class="spec-tabs">
            <div class="spec-tab active" data-tab="markdown" onclick="showTab('markdown')">Document</div>
            <div class="spec-tab" data-tab="code" onclick="showTab('code')">LISP</div>
            <div class="spec-tab" data-tab="properties" onclick="showTab('properties')">Properties</div>
            <div class="spec-tab" data-tab="whiteboard" onclick="showTab('whiteboard')">Whiteboard</div>
        </div>
        <div class="tab-content active" id="tab-markdown">
            <div class="spec-content markdown" id="specContent">
                <div class="empty-state">Formal specification will appear here...</div>
            </div>
        </div>
        <div class="tab-content" id="tab-code">
            <div class="spec-content code-view" id="codeContent">
                <div class="empty-state">LISP code will appear here...</div>
            </div>
        </div>
        <div class="tab-content" id="tab-properties">
            <div class="spec-content properties-view" id="propertiesContent">
                <div class="empty-state">CTL properties will appear here...</div>
            </div>
        </div>
        <div class="tab-content" id="tab-whiteboard">
            <div class="whiteboard-controls">
                <button onclick="clearWhiteboard()">Clear</button>
                <button onclick="aiPreview()" title="AI interprets sketch + commands">✨ AI</button>
                <span class="spacer"></span>
                <button id="newSpecBtn" style="display:none" onclick="newSpec()">New</button>
                <button class="primary" id="formalizeBtn" onclick="formalizeWhiteboard()">Formalize →</button>
            </div>
            <div class="whiteboard-area">
                <div class="whiteboard-input">
                    <textarea id="whiteboard" placeholder="Sketch multiple diagrams...

MESSAGES (sequence diagram):
  A -> B: hello
  B -> A: hi there

some notes or commands here...

STATES (state diagram):
  Idle --> Waiting
  Waiting --> Done

Click '✨ AI' for smart interpretation."></textarea>
                </div>
                <div class="whiteboard-preview" id="preview">
                    <div class="empty-state">Live preview appears here...</div>
                </div>
            </div>
        </div>
    </div>

    <script>
        mermaid.initialize({ 
            startOnLoad: false, 
            theme: 'dark',
            themeVariables: {
                primaryColor: '#1f6feb', primaryTextColor: '#c9d1d9', primaryBorderColor: '#30363d',
                lineColor: '#8b949e', secondaryColor: '#21262d', tertiaryColor: '#161b22',
                background: '#0d1117', mainBkg: '#161b22', nodeBorder: '#30363d',
                clusterBkg: '#21262d', titleColor: '#58a6ff', edgeLabelBackground: '#21262d'
            }
        });
        marked.setOptions({ gfm: true, breaks: true });
        
        const NL = String.fromCharCode(10);  // Newline for mermaid
        const FENCE = String.fromCharCode(96,96,96);  // Triple backtick for markdown code blocks
        let sessionId = 'session-' + Date.now();
        let currentTab = 'markdown';
        let currentDoc = '';
        let currentMarkdown = '';
        
        // Whiteboard preview with debounce
        let previewTimeout;
        document.getElementById('whiteboard').addEventListener('input', () => {
            clearTimeout(previewTimeout);
            previewTimeout = setTimeout(updatePreview, 300);
        });
        
        function updatePreview() {
            const text = document.getElementById('whiteboard').value;
            const preview = document.getElementById('preview');
            
            if (!text.trim()) {
                preview.innerHTML = '<div class="empty-state">Live preview appears here...</div>';
                return;
            }
            
            // Parse into sections
            const sections = parseWhiteboardSections(text);
            let html = '';
            let diagramCount = 0;
            
            sections.forEach((section, idx) => {
                if (section.type === 'sequence') {
                    diagramCount++;
                    const code = 'sequenceDiagram' + NL + section.lines.map(l => {
                        const m = l.match(/^(\w+)\s*->\s*(\w+)\s*:\s*(.+?)\.?$/);
                        if (m) {
                            let msg = m[3].replace(/\.+$/, '').trim().replace(/[^a-zA-Z0-9 ,.!?'-]/g, '');
                            return '    ' + m[1] + '->>' + m[2] + ': ' + msg;
                        }
                        return '';
                    }).filter(l => l).join(NL);
                    html += '<div class="mermaid" id="diagram-' + idx + '">' + code + '</div>';
                } else if (section.type === 'state') {
                    diagramCount++;
                    const code = 'stateDiagram-v2' + NL + section.lines.map(l => {
                        var withAction = l.match(/\s*(\w+)\s*-->\s*(\w+)\s*:\s*(\w+)\s*/)
                        if( !withAction ) {
                            withAction = l.match(/\s*(\w+)\s*-->\s*(\w+)\s*/)
                        }
                        if(withAction) {
                            if (withAction.length === 4) {
                                return '    ' + withAction[1] + ' --> ' + withAction[2] + ' : ' + withAction[3];
                            } else {
                                return '    ' + withAction[1] + ' --> ' + withAction[2];
                            }
                        } else {
                            console.log("could not parse: " + l);
                        }
                        return '';
                    }).filter(l => l).join(NL);
                    html += '<div class="mermaid" id="diagram-' + idx + '">' + code + '</div>';
                } else if (section.type === 'flow') {
                    diagramCount++;
                    const code = 'graph LR' + NL + section.lines.map(l => {
                        const m = l.match(/^(\w+)\s*->\s*(\w+)$/);
                        if (m) return '    ' + m[1] + ' --> ' + m[2];
                        return '';
                    }).filter(l => l).join(NL);
                    html += '<div class="mermaid" id="diagram-' + idx + '">' + code + '</div>';
                } else if (section.type === 'latex') {
                    html += '<div class="latex-section">' + renderLatex(section.lines.join(NL)) + '</div>';
                } else if (section.type === 'text') {
                    // Unknown text - show as note (could be commands for AI)
                    const content = section.lines.join(NL).trim();
                    if (content) {
                        html += '<div class="note-section" style="color:#8b949e;font-size:0.8rem;padding:0.5rem;border-left:2px solid #30363d;margin:0.5rem 0;">' + 
                            '<em>📝 ' + escapeHtml(content) + '</em></div>';
                    }
                }
            });
            
            if (!html) {
                html = '<pre style="color:#c9d1d9;font-size:0.85rem;white-space:pre-wrap;">' + escapeHtml(text) + '</pre>';
            }
            
            preview.innerHTML = html;
            
            // Run mermaid
            if (diagramCount > 0) {
                setTimeout(async () => {
                    try { 
                        await mermaid.run(); 
                    } catch(e) { 
                        console.log('Mermaid error:', e);
                    }
                }, 50);
            }
        }
        
        function parseWhiteboardSections(text) {
            const lines = text.split(NL);
            const sections = [];
            let currentSection = null;
            
            const msgPattern = /^(\w+)\s*->\s*(\w+)\s*:\s*.+$/;
            const statePattern = /^(\w+)\s*--.*-->\s*(\w+)|^(\w+)\s*-->\s*(\w+)/;
            const flowPattern = /^(\w+)\s*->\s*(\w+)$/;
            const latexPattern = /\$[^$]+\$/;
            
            function getLineType(line) {
                const trimmed = line.trim();
                if (!trimmed) return 'empty';
                if (msgPattern.test(trimmed)) return 'sequence';
                if (statePattern.test(trimmed)) return 'state';
                if (flowPattern.test(trimmed)) return 'flow';
                if (latexPattern.test(trimmed)) return 'latex';
                return 'text';
            }
            
            function pushSection() {
                if (currentSection && currentSection.lines.length > 0) {
                    sections.push(currentSection);
                }
            }
            
            lines.forEach(line => {
                const type = getLineType(line);
                
                if (type === 'empty') {
                    // Empty lines can end a section
                    if (currentSection && currentSection.type !== 'text') {
                        pushSection();
                        currentSection = null;
                    }
                    return;
                }
                
                if (!currentSection || currentSection.type !== type) {
                    pushSection();
                    currentSection = { type: type, lines: [] };
                }
                
                currentSection.lines.push(line.trim());
            });
            
            pushSection();
            return sections;
        }
        
        function extractMermaid(text) {
            // Legacy single-diagram extraction (kept for compatibility)
            const sections = parseWhiteboardSections(text);
            const diagrams = sections.filter(s => ['sequence', 'state', 'flow'].includes(s.type));
            if (diagrams.length === 0) return null;
            
            // Return first diagram only
            const first = diagrams[0];
            if (first.type === 'sequence') {
                return 'sequenceDiagram' + NL + first.lines.map(l => {
                    const m = l.match(/^(\w+)\s*->\s*(\w+)\s*:\s*(.+?)\.?$/);
                    if (m) return '    ' + m[1] + '->>' + m[2] + ': ' + m[3].replace(/\.+$/, '');
                    return '';
                }).filter(l => l).join(NL);
            }
            return null;
        }
        
        function renderLatex(text) {
            // Replace $...$ with rendered KaTeX
            return text.replace(/\$([^$]+)\$/g, (match, latex) => {
                try {
                    return katex.renderToString(latex, { throwOnError: false });
                } catch (e) {
                    return '<span class="error">' + escapeHtml(match) + '</span>';
                }
            });
        }
        
        function clearWhiteboard() {
            document.getElementById('whiteboard').value = '';
            document.getElementById('preview').innerHTML = '<div class="empty-state">Live preview appears here...</div>';
        }
        
        async function aiPreview() {
            const sketch = document.getElementById('whiteboard').value.trim();
            if (!sketch) return;
            
            const preview = document.getElementById('preview');
            preview.innerHTML = '<div class="empty-state">✨ AI interpreting...</div>';
            
            const provider = document.getElementById('provider').value;
            
            try {
                const resp = await fetch('/diagram', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ sketch, provider })
                });
                
                if (!resp.ok) throw new Error(await resp.text());
                
                const data = await resp.json();
                
                if (data.diagrams && data.diagrams.length > 0) {
                    let html = '';
                    data.diagrams.forEach((diagram, idx) => {
                        html += '<div class="mermaid" id="ai-diagram-' + idx + '">' + diagram + '</div>';
                    });
                    preview.innerHTML = html;
                    setTimeout(() => {
                        try { mermaid.run(); } catch(e) { console.log('Mermaid error:', e); }
                    }, 50);
                } else if (data.mermaid) {
                    // Backward compatibility
                    preview.innerHTML = '<div class="mermaid" id="ai-preview-mermaid">' + data.mermaid + '</div>';
                    setTimeout(() => {
                        try { mermaid.run(); } catch(e) { console.log('Mermaid error:', e); }
                    }, 50);
                } else if (data.error) {
                    preview.innerHTML = '<div class="error" style="color:#f85149;padding:1rem;">' + data.error + '</div>';
                } else {
                    preview.innerHTML = '<div class="empty-state">No diagrams generated</div>';
                }
            } catch (err) {
                preview.innerHTML = '<div class="error" style="color:#f85149;padding:1rem;">Error: ' + err.message + '</div>';
            }
        }
        
        function formalizeWhiteboard() {
            const sketch = document.getElementById('whiteboard').value.trim();
            if (!sketch) return;
            
            // Detect what kind of sketch this is
            let sketchType = 'general notes';
            if (sketch.includes('->') && sketch.includes(':')) {
                sketchType = 'message sequence diagram';
            } else if (sketch.includes('-->')) {
                sketchType = 'state machine';
            } else if (sketch.includes('$')) {
                sketchType = 'mathematical formulas';
            } else if (sketch.toLowerCase().includes('actor') || sketch.toLowerCase().includes('server') || sketch.toLowerCase().includes('client')) {
                sketchType = 'actor descriptions';
            }
            
            let prompt = '';
            
            // If we have a previous spec, ask for incremental update
            if (currentMarkdown && currentDoc) {
                prompt = 'I updated my whiteboard sketch (looks like ' + sketchType + '):\n\n' +
                    sketch + '\n\n' +
                    'Here is the current specification you generated:\n\n' +
                    '--- CURRENT LISP ---\n' + currentDoc + '\n--- END LISP ---\n\n' +
                    'Please update the specification to reflect my changes. Keep what still applies, modify what needs changing.';
            } else {
                // First time - full generation
                prompt = 'I sketched this on the whiteboard (looks like ' + sketchType + '):\n\n' +
                    sketch + '\n\n' +
                    'Please:\n' +
                    '1. Interpret what I am trying to express\n' +
                    '2. Create proper mermaid diagrams for it (sequence diagrams for message flows, state diagrams for state machines)\n' +
                    '3. Define actors and their behaviors in BoundedLISP\n' +
                    '4. Suggest relevant CTL properties to verify';
            }
            
            document.getElementById('input').value = prompt;
            sendMessage();
        }
        
        function updateFormalizeButton() {
            const btn = document.getElementById('formalizeBtn');
            const newBtn = document.getElementById('newSpecBtn');
            if (currentDoc) {
                btn.textContent = 'Update →';
                btn.title = 'Update existing specification with whiteboard changes';
                newBtn.style.display = 'inline-block';
            } else {
                btn.textContent = 'Formalize →';
                btn.title = 'Generate specification from whiteboard sketch';
                newBtn.style.display = 'none';
            }
        }
        
        function newSpec() {
            // Clear current spec and start fresh
            currentDoc = '';
            currentMarkdown = '';
            updateSpecPanel();
            updateFormalizeButton();
            document.getElementById('whiteboard').focus();
        }
        
        // Execute LISP code and return results
        async function executeCode(code) {
            try {
                const resp = await fetch('/eval', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ code })
                });
                if (!resp.ok) {
                    return { success: false, errors: [await resp.text()], output: '' };
                }
                return await resp.json();
            } catch (err) {
                return { success: false, errors: [err.message], output: '' };
            }
        }
        
        // Retry counter for auto-fix loop prevention
        let autoFixRetries = 0;
        const MAX_AUTO_FIX_RETRIES = 3;
        
        async function sendMessage(message, isAutoFix = false) {
            const input = document.getElementById('input');
            if (!message) {
                message = input.value.trim();
            }
            if (!message) return;
            
            const provider = document.getElementById('provider').value;
            
            // Only show user message if not an auto-fix retry
            if (!isAutoFix) {
                addMessage('user', message);
                input.value = '';
                autoFixRetries = 0; // Reset retry counter on new user message
            } else {
                // Show auto-fix attempt in chat
                addMessage('user', '🔧 *Auto-fix attempt ' + autoFixRetries + '/' + MAX_AUTO_FIX_RETRIES + '*\n\n' + message);
            }
            
            const loading = document.createElement('div');
            loading.className = 'message assistant';
            loading.innerHTML = '<p><em>' + (isAutoFix ? 'Attempting fix...' : 'Thinking...') + '</em></p>';
            loading.id = 'loading';
            document.getElementById('messages').appendChild(loading);
            loading.scrollIntoView({ behavior: 'smooth' });
            
            try {
                const resp = await fetch('/chat', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ session_id: sessionId, message, provider })
                });
                
                document.getElementById('loading')?.remove();
                if (!resp.ok) throw new Error(await resp.text());
                
                const data = await resp.json();
                addMessage('assistant', data.chat_response || 'Updated.');
                
                // Update usage display
                if (data.usage) {
                    updateUsage(data.usage);
                }
                
                if (data.markdown) {
                    currentMarkdown = data.markdown;
                    if (currentTab === 'markdown') updateSpecPanel();
                }
                if (data.current_doc) {
                    currentDoc = data.current_doc;
                    if (currentTab === 'code') updateSpecPanel();
                    updateFormalizeButton();
                    
                    // Auto-execute the generated code
                    const execResult = await executeCode(currentDoc);
                    
                    if (!execResult.success && execResult.errors && execResult.errors.length > 0) {
                        // Code has errors - attempt auto-fix
                        autoFixRetries++;
                        
                        if (autoFixRetries <= MAX_AUTO_FIX_RETRIES) {
                            const errorMsg = execResult.errors.join('\n');
                            const fixPrompt = 
                                '⚠️ The code you generated has errors when executed:\n\n' +
                                FENCE + '\n' + errorMsg + '\n' + FENCE + '\n\n' +
                                'Here is the code that failed:\n\n' +
                                FENCE + 'lisp\n' + currentDoc + '\n' + FENCE + '\n\n' +
                                'Please fix the errors and provide corrected code.';
                            
                            // Small delay before retry
                            await new Promise(r => setTimeout(r, 500));
                            
                            // Recursive call with auto-fix message
                            await sendMessage(fixPrompt, true);
                        } else {
                            // Max retries exceeded - report to user
                            addMessage('assistant', 
                                '❌ **Auto-fix failed after ' + MAX_AUTO_FIX_RETRIES + ' attempts.**\n\n' +
                                'The code continues to have errors:\n\n' +
                                FENCE + '\n' + execResult.errors.join('\n') + '\n' + FENCE + '\n\n' +
                                'Please review the code in the LISP tab and describe what you are trying to accomplish so I can help fix it.');
                            autoFixRetries = 0;
                        }
                    } else {
                        // Code executed successfully
                        autoFixRetries = 0;
                        // Refresh properties panel in case new ones were defined
                        updatePropertiesPanel();
                        if (execResult.output && execResult.output.trim()) {
                            // Show execution output if there's meaningful output
                            const outputLines = execResult.output.trim().split('\n').filter(l => l.trim());
                            if (outputLines.length > 0 && outputLines.some(l => !l.startsWith(';'))) {
                                addMessage('assistant', '✅ Code executed successfully.');
                            }
                        }
                    }
                }
            } catch (err) {
                document.getElementById('loading')?.remove();
                addMessage('assistant', '❌ ' + err.message);
                autoFixRetries = 0;
            }
        }
        
        function addMessage(role, content) {
            const div = document.createElement('div');
            div.className = 'message ' + role;
            div.innerHTML = marked.parse(content);
            document.getElementById('messages').appendChild(div);
            div.scrollIntoView({ behavior: 'smooth' });
        }
        
        function updateSpecPanel() {
            const markdownContainer = document.getElementById('specContent');
            const codeContainer = document.getElementById('codeContent');
            
            // Update markdown tab content
            if (!currentMarkdown) {
                markdownContainer.innerHTML = '<div class="empty-state">Formal specification will appear here...</div>';
            } else {
                markdownContainer.innerHTML = marked.parse(currentMarkdown);
                setTimeout(() => {
                    markdownContainer.querySelectorAll('pre code.language-mermaid').forEach((el, i) => {
                        const wrapper = document.createElement('div');
                        wrapper.className = 'mermaid';
                        wrapper.id = 'mermaid-spec-' + Date.now() + '-' + i;
                        wrapper.textContent = el.textContent;
                        el.parentElement.replaceWith(wrapper);
                    });
                    mermaid.run();
                }, 50);
            }
            
            // Update code tab content
            if (!currentDoc) {
                codeContainer.innerHTML = '<div class="empty-state">LISP code will appear here...</div>';
            } else {
                codeContainer.innerHTML = '<pre><code>' + escapeHtml(currentDoc) + '</code></pre>';
            }
        }
        
        function showTab(tab) {
            currentTab = tab;
            // Update tab button active state
            document.querySelectorAll('.spec-tab').forEach(t => {
                t.classList.toggle('active', t.dataset.tab === tab);
            });
            // Update tab content visibility
            document.querySelectorAll('.tab-content').forEach(c => {
                c.classList.toggle('active', c.id === 'tab-' + tab);
            });
            // Refresh content if needed
            if (tab === 'markdown' || tab === 'code') {
                updateSpecPanel();
            }
            // Fetch and render properties
            if (tab === 'properties') {
                updatePropertiesPanel();
            }
            // Focus whiteboard if switching to it
            if (tab === 'whiteboard') {
                setTimeout(() => document.getElementById('whiteboard').focus(), 100);
            }
        }
        
        async function updatePropertiesPanel() {
            const container = document.getElementById('propertiesContent');
            try {
                const resp = await fetch('/properties');
                const data = await resp.json();
                
                if (!data.properties || data.properties.length === 0) {
                    container.innerHTML = '<div class="empty-state">No CTL properties defined yet.</div>';
                    return;
                }
                
                let html = '';
                for (const prop of data.properties) {
                    html += '<div class="property-item">';
                    html += '<div class="property-name">' + escapeHtml(prop.name) + '</div>';
                    html += '<div class="property-formula">';
                    try {
                        html += katex.renderToString(prop.latex, { 
                            throwOnError: false,
                            displayMode: true 
                        });
                    } catch (e) {
                        html += escapeHtml(prop.latex);
                    }
                    html += '</div></div>';
                }
                container.innerHTML = html;
            } catch (err) {
                container.innerHTML = '<div class="empty-state">Error loading properties: ' + escapeHtml(err.message) + '</div>';
            }
        }
        
        function updateUsage(usage) {
            const el = document.getElementById('usage');
            const total = usage.total_tokens || 0;
            const k = (total / 1000).toFixed(1);
            el.textContent = k + 'k tokens';
            
            // Color code based on usage (rough heuristics)
            el.className = 'usage';
            if (total > 50000) {
                el.className = 'usage danger';
                el.textContent = k + 'k ⚠️';
            } else if (total > 25000) {
                el.className = 'usage warn';
            }
        }
        
        function escapeHtml(text) {
            return text.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
        }
        
        document.getElementById('input').addEventListener('keydown', e => {
            if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); sendMessage(); }
        });
    </script>
</body>
</html>`

// ============================================================================
// MCP Server (Model Context Protocol)
// ============================================================================
//
// Run:  ./philosopher -mcp          (stdio for Claude Desktop)
//       ./philosopher -mcp-sse 3000 (HTTP/SSE for web clients)
//
// Claude Desktop config (~/.config/claude/claude_desktop_config.json):
//   { "mcpServers": { "philosopher": { "command": "/path/to/philosopher", "args": ["-mcp"] } } }

var mcpEvaluator *Evaluator

func runMCPServer() {
	mcpEvaluator = NewEvaluator(64)
	loadLispModules(mcpEvaluator)

	fmt.Fprintln(os.Stderr, "BoundedLISP MCP Server (stdio)")
	reader := bufio.NewReader(os.Stdin)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}

		var req map[string]interface{}
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			mcpSendError(-32700, "Parse error", nil)
			continue
		}

		method, _ := req["method"].(string)
		id := req["id"]
		params, _ := req["params"].(map[string]interface{})

		switch method {
		case "initialize":
			mcpSendResult(id, map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"capabilities":    map[string]interface{}{"tools": map[string]bool{"listChanged": false}},
				"serverInfo":      map[string]string{"name": "BoundedLISP", "version": "1.0.0"},
			})
		case "initialized":
			// no response
		case "tools/list":
			mcpSendResult(id, map[string]interface{}{"tools": mcpToolDefs()})
		case "tools/call":
			mcpSendResult(id, mcpCallTool(params))
		default:
			mcpSendError(-32601, "Method not found: "+method, id)
		}
	}
}

func runMCPSSEServer(port string) {
	mcpEvaluator = NewEvaluator(64)
	loadLispModules(mcpEvaluator)

	fmt.Printf("BoundedLISP MCP Server (SSE) on :%s\n", port)

	http.HandleFunc("/sse", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		flusher, _ := w.(http.Flusher)
		fmt.Fprintf(w, "event: endpoint\ndata: /message\n\n")
		flusher.Flush()
		<-r.Context().Done()
	})

	http.HandleFunc("/message", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "POST required", 405)
			return
		}
		var req map[string]interface{}
		json.NewDecoder(r.Body).Decode(&req)
		method, _ := req["method"].(string)
		id := req["id"]
		params, _ := req["params"].(map[string]interface{})

		var result interface{}
		switch method {
		case "initialize":
			result = map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"capabilities":    map[string]interface{}{"tools": map[string]bool{"listChanged": false}},
				"serverInfo":      map[string]string{"name": "BoundedLISP", "version": "1.0.0"},
			}
		case "tools/list":
			result = map[string]interface{}{"tools": mcpToolDefs()}
		case "tools/call":
			result = mcpCallTool(params)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"jsonrpc": "2.0", "id": id, "result": result})
	})

	http.ListenAndServe(":"+port, nil)
}

func mcpSendResult(id, result interface{}) {
	out, _ := json.Marshal(map[string]interface{}{"jsonrpc": "2.0", "id": id, "result": result})
	fmt.Println(string(out))
}

func mcpSendError(code int, msg string, id interface{}) {
	out, _ := json.Marshal(map[string]interface{}{"jsonrpc": "2.0", "id": id, "error": map[string]interface{}{"code": code, "message": msg}})
	fmt.Println(string(out))
}

func mcpToolDefs() []map[string]interface{} {
	return []map[string]interface{}{
		{"name": "eval_lisp", "description": "Evaluate BoundedLISP code",
			"inputSchema": map[string]interface{}{"type": "object", "properties": map[string]interface{}{"code": map[string]string{"type": "string"}}, "required": []string{"code"}}},
		{"name": "run_simulation", "description": "Run scheduler for N steps",
			"inputSchema": map[string]interface{}{"type": "object", "properties": map[string]interface{}{"max_steps": map[string]interface{}{"type": "number"}}}},
		{"name": "spawn_actor", "description": "Create actor",
			"inputSchema": map[string]interface{}{"type": "object", "properties": map[string]interface{}{"name": map[string]string{"type": "string"}, "mailbox_size": map[string]interface{}{"type": "number"}, "initial_state": map[string]string{"type": "string"}}, "required": []string{"name", "initial_state"}}},
		{"name": "send_message", "description": "Send message to actor",
			"inputSchema": map[string]interface{}{"type": "object", "properties": map[string]interface{}{"actor": map[string]string{"type": "string"}, "message": map[string]string{"type": "string"}}, "required": []string{"actor", "message"}}},
		{"name": "get_metrics", "description": "Get registry metrics",
			"inputSchema": map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}},
		{"name": "get_actors", "description": "Get actor states",
			"inputSchema": map[string]interface{}{"type": "object", "properties": map[string]interface{}{"name": map[string]string{"type": "string"}}}},
		{"name": "reset", "description": "Reset scheduler",
			"inputSchema": map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}},
		{"name": "csp_status", "description": "Get CSP violations",
			"inputSchema": map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}},
		{"name": "csp_enforce", "description": "Enable CSP enforcement",
			"inputSchema": map[string]interface{}{"type": "object", "properties": map[string]interface{}{"enabled": map[string]string{"type": "boolean"}, "strict": map[string]string{"type": "boolean"}}}},
	}
}

func mcpCallTool(params map[string]interface{}) map[string]interface{} {
	name, _ := params["name"].(string)
	args, _ := params["arguments"].(map[string]interface{})

	var result interface{}
	var isErr bool

	switch name {
	case "eval_lisp":
		code, _ := args["code"].(string)
		if code == "" {
			result, isErr = "code required", true
		} else {
			var results []string
			for _, expr := range NewParser(code).Parse() {
				results = append(results, mcpEvaluator.Eval(expr, nil).String())
			}
			result = map[string]interface{}{"results": results}
		}

	case "run_simulation":
		steps := 1000
		if s, ok := args["max_steps"].(float64); ok {
			steps = int(s)
		}
		for _, expr := range NewParser(fmt.Sprintf("(run-scheduler %d)", steps)).Parse() {
			mcpEvaluator.Eval(expr, nil)
		}
		states := make(map[string]string)
		for n, a := range mcpEvaluator.Scheduler.Actors {
			s := "runnable"
			if a.State == ActorBlocked {
				s = "blocked:" + a.BlockedOn
			} else if a.State == ActorDone {
				s = "done"
			}
			states[n] = s
		}
		result = map[string]interface{}{"steps": mcpEvaluator.Scheduler.StepCount, "actors": states}

	case "spawn_actor":
		n, _ := args["name"].(string)
		ms := 16
		if m, ok := args["mailbox_size"].(float64); ok {
			ms = int(m)
		}
		init, _ := args["initial_state"].(string)
		for _, expr := range NewParser(fmt.Sprintf("(spawn-actor '%s %d '%s)", n, ms, init)).Parse() {
			mcpEvaluator.Eval(expr, nil)
		}
		result = map[string]interface{}{"spawned": n}

	case "send_message":
		actor, _ := args["actor"].(string)
		msg, _ := args["message"].(string)
		for _, expr := range NewParser(fmt.Sprintf("(send-to! '%s %s)", actor, msg)).Parse() {
			mcpEvaluator.Eval(expr, nil)
		}
		result = map[string]interface{}{"sent": actor, "message": msg}

	case "get_metrics":
		m := make(map[string]interface{})
		for k, v := range mcpEvaluator.Registry {
			m[k] = mcpFormatValue(v)
		}
		result = m

	case "get_actors":
		n, _ := args["name"].(string)
		if n != "" {
			if a := mcpEvaluator.Scheduler.GetActor(n); a != nil {
				result = mcpActorInfo(a)
			} else {
				result, isErr = "not found", true
			}
		} else {
			m := make(map[string]interface{})
			for n, a := range mcpEvaluator.Scheduler.Actors {
				m[n] = mcpActorInfo(a)
			}
			result = m
		}

	case "reset":
		mcpEvaluator.Scheduler = NewScheduler()
		result = map[string]bool{"reset": true}

	case "csp_status":
		v := make(map[string][]string)
		for n, a := range mcpEvaluator.Scheduler.Actors {
			if len(a.CSPViolations) > 0 {
				v[n] = a.CSPViolations
			}
		}
		result = map[string]interface{}{"enforce": mcpEvaluator.Scheduler.CSPEnforce, "violations": v}

	case "csp_enforce":
		if e, ok := args["enabled"].(bool); ok {
			mcpEvaluator.Scheduler.CSPEnforce = e
		}
		if s, ok := args["strict"].(bool); ok && s {
			for _, a := range mcpEvaluator.Scheduler.Actors {
				a.CSPStrict = true
			}
		}
		result = map[string]bool{"ok": true}

	default:
		result, isErr = "unknown tool: "+name, true
	}

	if isErr {
		return map[string]interface{}{"content": []map[string]interface{}{{"type": "text", "text": fmt.Sprintf("Error: %v", result)}}, "isError": true}
	}
	txt, _ := json.MarshalIndent(result, "", "  ")
	return map[string]interface{}{"content": []map[string]interface{}{{"type": "text", "text": string(txt)}}}
}

func mcpFormatValue(v Value) interface{} {
	switch v.Type {
	case TypeNumber:
		return v.Number
	case TypeString:
		return v.Str
	case TypeBool:
		return v.Bool
	case TypeList:
		items := make([]interface{}, len(v.List))
		for i, x := range v.List {
			items[i] = mcpFormatValue(x)
		}
		return items
	default:
		return v.String()
	}
}

func mcpActorInfo(a *Actor) map[string]interface{} {
	s := "runnable"
	if a.State == ActorBlocked {
		s = "blocked"
	} else if a.State == ActorDone {
		s = "done"
	}
	return map[string]interface{}{
		"state": s, "blocked_on": a.BlockedOn,
		"mailbox": len(a.Mailbox.Data), "capacity": a.Mailbox.Capacity,
		"csp_violations": a.CSPViolations,
	}
}

// ============================================================================
// Datalog Interpreter for BoundedLISP
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

// Always checks if a goal holds for all times in the trace (AG)
func (db *DatalogDB) Always(goal Goal) bool {
	// Get all unique times from facts
	times := make(map[int64]bool)
	for _, f := range db.Facts {
		times[f.Time] = true
	}

	if len(times) == 0 {
		return true // vacuously true
	}

	for t := range times {
		// Create at-time wrapped goal: (at-time pred args... time)
		atTimeArgs := append([]Term{Atom(goal.Predicate)}, goal.Args...)
		atTimeArgs = append(atTimeArgs, NumTerm(float64(t)))
		atTimeGoal := Goal{
			Predicate: "at-time",
			Args:      atTimeArgs,
		}
		results := db.solve([]Goal{atTimeGoal}, make(Binding), 0)
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

	// (list-facts) - list all facts, optionally filtered by predicate
	// (list-facts) - all facts
	// (list-facts 'sale) - only 'sale' facts
	env.Set("list-facts", Value{Type: TypeBuiltin, Builtin: func(ev *Evaluator, args []Value, env *Env) Value {
		var predFilter string
		if len(args) > 0 && args[0].Type == TypeSymbol {
			predFilter = args[0].Symbol
		}
		
		var result []Value
		for _, fact := range ev.DatalogDB.Facts {
			if predFilter != "" && fact.Predicate != predFilter {
				continue
			}
			// Convert fact to list: (predicate arg1 arg2 ...)
			factList := make([]Value, len(fact.Args)+1)
			factList[0] = Sym(fact.Predicate)
			for i, arg := range fact.Args {
				factList[i+1] = TermToValue(arg)
			}
			result = append(result, Lst(factList...))
		}
		return Lst(result...)
	}})

	// (fact-count) - count total facts
	// (fact-count 'sale) - count facts with predicate
	env.Set("fact-count", Value{Type: TypeBuiltin, Builtin: func(ev *Evaluator, args []Value, env *Env) Value {
		var predFilter string
		if len(args) > 0 && args[0].Type == TypeSymbol {
			predFilter = args[0].Symbol
		}
		
		count := 0
		for _, fact := range ev.DatalogDB.Facts {
			if predFilter == "" || fact.Predicate == predFilter {
				count++
			}
		}
		return Num(float64(count))
	}})

	// (sum-facts 'predicate field-index) - sum numeric values at field position
	// (sum-facts 'sent 2) - sum the 3rd field (0-indexed) of all 'sent' facts
	env.Set("sum-facts", Value{Type: TypeBuiltin, Builtin: func(ev *Evaluator, args []Value, env *Env) Value {
		if len(args) < 2 {
			return Num(0)
		}
		predFilter := ""
		if args[0].Type == TypeSymbol {
			predFilter = args[0].Symbol
		}
		fieldIdx := 0
		if args[1].Type == TypeNumber {
			fieldIdx = int(args[1].Number)
		}
		
		sum := 0.0
		for _, fact := range ev.DatalogDB.Facts {
			if predFilter != "" && fact.Predicate != predFilter {
				continue
			}
			if fieldIdx < len(fact.Args) && fact.Args[fieldIdx].IsNum {
				sum += fact.Args[fieldIdx].Num
			}
		}
		return Num(sum)
	}})

	// (max-facts 'predicate field-index) - max numeric value at field position
	env.Set("max-facts", Value{Type: TypeBuiltin, Builtin: func(ev *Evaluator, args []Value, env *Env) Value {
		if len(args) < 2 {
			return Num(0)
		}
		predFilter := ""
		if args[0].Type == TypeSymbol {
			predFilter = args[0].Symbol
		}
		fieldIdx := 0
		if args[1].Type == TypeNumber {
			fieldIdx = int(args[1].Number)
		}
		
		max := 0.0
		first := true
		for _, fact := range ev.DatalogDB.Facts {
			if predFilter != "" && fact.Predicate != predFilter {
				continue
			}
			if fieldIdx < len(fact.Args) && fact.Args[fieldIdx].IsNum {
				if first || fact.Args[fieldIdx].Num > max {
					max = fact.Args[fieldIdx].Num
					first = false
				}
			}
		}
		return Num(max)
	}})

	// (timeseries 'predicate value-index) - get [(time value) ...] for charts
	// Returns list of (time value) pairs sorted by time
	env.Set("timeseries", Value{Type: TypeBuiltin, Builtin: func(ev *Evaluator, args []Value, env *Env) Value {
		if len(args) < 2 {
			return Lst()
		}
		predFilter := ""
		if args[0].Type == TypeSymbol {
			predFilter = args[0].Symbol
		}
		valueIdx := 0
		if args[1].Type == TypeNumber {
			valueIdx = int(args[1].Number)
		}
		
		// Collect (time, value) pairs
		type point struct {
			time  int64
			value float64
		}
		points := []point{}
		
		for _, fact := range ev.DatalogDB.Facts {
			if predFilter != "" && fact.Predicate != predFilter {
				continue
			}
			if valueIdx < len(fact.Args) && fact.Args[valueIdx].IsNum {
				points = append(points, point{time: fact.Time, value: fact.Args[valueIdx].Num})
			}
		}
		
		// Sort by time
		sort.Slice(points, func(i, j int) bool {
			return points[i].time < points[j].time
		})
		
		// Convert to list of (time value) pairs
		result := make([]Value, len(points))
		for i, p := range points {
			result[i] = Lst(Num(float64(p.time)), Num(p.value))
		}
		return Lst(result...)
	}})

	// (group-count 'predicate field-index) - count by group
	// Returns ((group1 count1) (group2 count2) ...)
	env.Set("group-count", Value{Type: TypeBuiltin, Builtin: func(ev *Evaluator, args []Value, env *Env) Value {
		if len(args) < 2 {
			return Lst()
		}
		predFilter := ""
		if args[0].Type == TypeSymbol {
			predFilter = args[0].Symbol
		}
		fieldIdx := 0
		if args[1].Type == TypeNumber {
			fieldIdx = int(args[1].Number)
		}
		
		counts := make(map[string]int)
		for _, fact := range ev.DatalogDB.Facts {
			if predFilter != "" && fact.Predicate != predFilter {
				continue
			}
			if fieldIdx < len(fact.Args) {
				key := fact.Args[fieldIdx].String()
				counts[key]++
			}
		}
		
		result := make([]Value, 0, len(counts))
		for k, v := range counts {
			result = append(result, Lst(Str(k), Num(float64(v))))
		}
		return Lst(result...)
	}})

	// (group-sum 'predicate group-field-index value-field-index) - sum by group
	env.Set("group-sum", Value{Type: TypeBuiltin, Builtin: func(ev *Evaluator, args []Value, env *Env) Value {
		if len(args) < 3 {
			return Lst()
		}
		predFilter := ""
		if args[0].Type == TypeSymbol {
			predFilter = args[0].Symbol
		}
		groupIdx := 0
		if args[1].Type == TypeNumber {
			groupIdx = int(args[1].Number)
		}
		valueIdx := 0
		if args[2].Type == TypeNumber {
			valueIdx = int(args[2].Number)
		}
		
		sums := make(map[string]float64)
		for _, fact := range ev.DatalogDB.Facts {
			if predFilter != "" && fact.Predicate != predFilter {
				continue
			}
			if groupIdx < len(fact.Args) && valueIdx < len(fact.Args) {
				key := fact.Args[groupIdx].String()
				if fact.Args[valueIdx].IsNum {
					sums[key] += fact.Args[valueIdx].Num
				}
			}
		}
		
		result := make([]Value, 0, len(sums))
		for k, v := range sums {
			result = append(result, Lst(Str(k), Num(v)))
		}
		return Lst(result...)
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
