# BoundedLISP Dialect Reference

BoundedLISP is a purpose-built dialect for protocol specification. It borrows from Scheme, Common Lisp, and Clojure, but has its own semantics. This document defines exactly what the language does.

## Truthiness (IMPORTANT)

BoundedLISP has **JavaScript-like** truthiness, not Scheme or CL:

| Value | Truthy? |
|-------|---------|
| `nil` | ❌ false |
| `false` | ❌ false |
| `'()` (empty list) | ❌ false |
| `0` | ❌ false |
| `""` (empty string) | ❌ false |
| Everything else | ✅ true |

**Contrast with other Lisps:**
- Scheme: Only `#f` is false. Empty list `'()` is **truthy**.
- Common Lisp: Only `nil` is false. `nil` = `'()`.
- BoundedLISP: `nil`, `false`, `'()`, `0`, `""` are all falsey.

## Boolean Literals

```lisp
true   ; boolean true
false  ; boolean false
nil    ; null/nothing (also falsey)
```

Not `#t`/`#f` (Scheme) or `t`/`nil` (CL).

## Let Bindings

### Simple let (single binding)
```lisp
(let name value body...)
```
Binds ONE name to ONE value, then evaluates body expressions.
```lisp
(let x 5 
  (+ x 1))  ; => 6

(let x 5
  (println x)
  (* x 2))  ; prints 5, returns 10
```

This is NOT standard Lisp syntax. It's closer to ML/Haskell.

### let* (sequential bindings)
```lisp
(let* ((name1 val1) (name2 val2) ...) body...)
```
Standard sequential binding like Scheme/CL.
```lisp
(let* ((x 5) (y (+ x 1)))
  (* x y))  ; => 30
```

## Define

### Simple value
```lisp
(define name value)
(define pi 3.14159)
```

### Function (Scheme-style shorthand)
```lisp
(define (name args...) body...)
(define (square x) (* x x))
(define (add a b) (+ a b))
```

### Multi-expression body
```lisp
(define (greet name)
  (println "Hello")
  (string-append "Hi " name))  ; last expression is return value
```

### ⚠️ SCOPING WARNING: Define is ALWAYS global

`define` writes to the global environment, even when used inside a function:

```lisp
(define (outer x)
  (define (inner y) (+ x y))  ; DANGER: inner is now GLOBAL!
  (inner 10))

(outer 5)   ; => 15
(inner 100) ; => 105 - inner leaked to global scope!
```

**For local helper functions, use `let` + `lambda`:**

```lisp
(define (outer x)
  (let inner (lambda (y) (+ x y))  ; inner is LOCAL
    (inner 10)))

(outer 5)   ; => 15
inner       ; => error: undefined
```

Closures DO capture their environment correctly - the issue is only that `define` pollutes the global namespace.

## Lambda

```lisp
(lambda (args...) body...)
((lambda (x) (* x x)) 5)  ; => 25
```

## Conditionals

### if
```lisp
(if condition then-expr else-expr)
(if (> x 0) "positive" "non-positive")
```

### cond
```lisp
(cond
  (condition1 result1)
  (condition2 result2)
  (true default))  ; use 'true' not 'else'
```

## Lists

```lisp
'(1 2 3)           ; quoted list
(list 1 2 3)       ; constructed list
(cons 1 '(2 3))    ; => (1 2 3)
(first '(1 2 3))   ; => 1
(rest '(1 2 3))    ; => (2 3)
(nth lst index)    ; 0-indexed
(length lst)
(append lst1 lst2)
(empty? lst)       ; true if nil or '()
```

## Comparison

```lisp
(= a b)    ; equality (works on numbers, symbols, strings, lists)
(!= a b)   ; not equal
(< a b)    ; less than
(<= a b)   ; less or equal
(> a b)    ; greater than
(>= a b)   ; greater or equal
```

Note: `=` does deep equality on lists.

## Arithmetic

```lisp
(+ a b ...)   ; variadic
(- a b ...)   ; variadic (first - rest)
(* a b ...)   ; variadic
(/ a b)       ; division
(mod a b)     ; modulo
```

## Boolean Logic

```lisp
(and a b ...)  ; short-circuits
(or a b ...)   ; short-circuits  
(not x)
```

## Type Predicates

```lisp
(number? x)
(symbol? x)
(string? x)
(list? x)
(nil? x)
```

## String Operations

```lisp
(string-append s1 s2 ...)
(symbol->string 'foo)   ; => "foo"
(string->symbol "foo")  ; => foo
(number->string 42)     ; => "42"
```

## Tagged Values (Sum Types)

```lisp
(tag 'type-name value)       ; create tagged value
(tag-is? val 'type-name)     ; check tag
(tag-type val)               ; get tag name
(tag-value val)              ; unwrap value
```

Example:
```lisp
(define result (tag 'ok 42))
(tag-is? result 'ok)    ; => true
(tag-value result)      ; => 42
```

## Mutation

```lisp
(set! name value)  ; mutate existing binding
```

Only works on already-defined variables.

## Printing

```lisp
(print x y ...)    ; print without newline
(println x y ...)  ; print with newline
```

## Bounded Data Structures

### Stack
```lisp
(make-stack capacity)
(push-now! stack value)  ; returns 'ok or 'full
(pop-now! stack)         ; returns value or 'empty
(stack-peek stack)       ; returns value or 'empty
(stack-empty? stack)
(stack-full? stack)
```

### Queue (Mailbox)
```lisp
(make-queue capacity)
(send-now! queue value)  ; returns 'ok or 'full
(recv-now! queue)        ; returns value or 'empty
(queue-empty? queue)
(queue-full? queue)
```

## Actor System

### Spawning
```lisp
(spawn-actor name mailbox-size initial-code)
```

### Messaging
```lisp
(send-to! actor-name message)  ; async send
(receive!)                      ; blocking receive
(self)                          ; current actor name
```

### State via Become
```lisp
(define (my-loop state)
  (let msg (receive!)
    ; process msg
    (if (should-continue?)
        (list 'become (list 'my-loop new-state))
        'done)))
```

Return values:
- `(list 'become code)` - continue with new code
- `'done` - actor terminates
- `'yield` - yield timeslice, restart body

## CTL Formulas

```lisp
(prop 'name)              ; atomic proposition
(ctl-and f1 f2 ...)       ; conjunction
(ctl-or f1 f2 ...)        ; disjunction
(ctl-not f)               ; negation
(ctl-implies p q)         ; implication

(EX f)   ; exists next
(AX f)   ; forall next
(EF f)   ; exists eventually
(AF f)   ; forall eventually
(EG f)   ; exists globally
(AG f)   ; forall globally
(EU p q) ; exists until
(AU p q) ; forall until
```

## NOT Supported

- Macros
- `quote` as special form (use `'`)
- `quasiquote` / `unquote`
- `call/cc` or continuations
- Multiple return values
- Dotted pairs
- Characters as distinct type
- Complex numbers
- Rational numbers

## Metrics Actor

BoundedLISP includes a built-in metrics collection actor for monitoring simulations.

### Starting Metrics Collection
```lisp
(start-metrics!)  ; spawns the 'metrics actor
```

### Sending Metrics
```lisp
(metric-inc! 'requests)           ; increment counter by 1
(metric-counter! 'bytes 1024)     ; increment counter by value
(metric-gauge! 'queue-depth 42)   ; set gauge to value
(metric-timing! 'latency 150)     ; record timing sample
(metric-event! 'error)            ; same as (metric-inc! 'error)
```

### Reading Metrics
```lisp
(get-metrics)  ; returns current metrics state
; => ((counters (requests 100) (bytes 10240)) 
;     (gauges (queue-depth 42)) 
;     (timings (latency (150 120 180))) 
;     (events))
```

### Important: Placement Matters

Due to how actors block on `receive!`, code BEFORE receive may execute multiple times. Place metric calls AFTER receive for accurate counts:

```lisp
; WRONG - may double-count
(define (my-actor)
  (metric-inc! 'iterations)  ; runs each time actor wakes!
  (let msg (receive!)
    ...))

; CORRECT - counts once per message
(define (my-actor)
  (let msg (receive!)
    (metric-inc! 'processed)  ; runs once per received message
    ...))
```

## Summary: Key Differences from Standard Lisps

| Feature | Scheme | Common Lisp | BoundedLISP |
|---------|--------|-------------|-------------|
| False values | `#f` only | `nil` only | `nil`, `false`, `'()`, `0`, `""` |
| Booleans | `#t`/`#f` | `t`/`nil` | `true`/`false` |
| Simple let | `(let ((x 1)) ...)` | `(let ((x 1)) ...)` | `(let x 1 ...)` |
| Define fn | `(define (f x) ...)` | `(defun f (x) ...)` | `(define (f x) ...)` |
| Empty list | `'()` (truthy!) | `nil`/`'()` | `'()` (falsey) |
| Else in cond | `else` | `t` | `true` |
