# Functional Issues

This document records known behavioral differences between Ego and Go, as well
as Ego-specific limitations uncovered during testing. It is intended as a living
reference for future development: each section describes the issue, the affected
code, and a concrete recommendation. A checklist at the bottom tracks
remediation progress.

## Structure of this document

There is a section (##) for each area of Ego that was evaluated. Within each
section, issues are grouped by priority (HIGH, MEDIUM, LOW). Each issue has a
unique identifier of the form `FUNC-N` where `N` is a sequence number. The
priority is part of the identifier suffix, e.g. `FUNC-1-HIGH`.

Additionally, there is a section at the end of this document that shows the
completion of all issues.

---

## Index of Issue Areas

1. [Variadic Functions (VARARG)](#vararg)
2. [Closure Scope and Lifetime (CLOSURE)](#closure)
3. [Receiver Methods (RECEIVER)](#receiver)
4. [Type Coercion and Operators (COERCE)](#coerce)
5. [Named Return Values (NAMED)](#named)
6. [Remediation Checklist](#checklist)

---

## Variadic Functions<a name="vararg"></a>

### HIGH

#### FUNC-1-HIGH — Calling a variadic function with zero variadic arguments fails

**Affected files:**

- `compiler/function.go` — argument count validation
- `bytecode/callframe.go` — argument unpacking at call time

**Description:**  
In Go, a function declared as `func f(args ...int)` may be called as `f()` —
passing zero variadic arguments is valid, and `args` inside the function body
is `nil` (or an empty slice). The same is true for functions with a fixed
parameter plus varargs: `func g(base int, args ...int)` may be called as
`g(10)` — the varargs receive an empty slice.

In Ego, both forms fail at runtime with `"incorrect function argument count"`:

```ego
func sum(args... int) int { ... }
sum()       // ERROR: incorrect function argument count

func sumFrom(base int, args... int) int { ... }
sumFrom(10) // ERROR: incorrect function argument count
```

This breaks common Go patterns such as logging helpers, optional-parameter
functions, and aggregation functions that are sometimes called with no
additional values.

The spread operator (`sum(nums...)`) works correctly.

**Test file:** `tests/functions/variadics.ego` — tests
`"functions: zero variadic args is an error"` and
`"functions: zero varargs after fixed is error"` document the current behavior.

**Recommendation:**  
In the argument-count validation pass, treat a variadic parameter as
contributing zero or more arguments (not one or more). When the call provides
exactly the number of fixed parameters and no varargs, pass an empty (nil)
slice for the variadic parameter rather than rejecting the call.

---

## Closure Scope and Lifetime<a name="closure"></a>

### HIGH

#### FUNC-2-HIGH — Closures stored during a loop are invalid after the loop ends

**Affected files:**

- `compiler/function.go` — closure compilation
- `symbols/` — symbol table lifetime and scope

**Description:**  
In Go, closures capture variables by reference and those references remain
valid for the lifetime of the closure (the garbage collector keeps the
captured variable alive as long as the closure lives). A common Go pattern is
to build a slice of closures inside a loop and call them later:

```go
var funcs []func() int
for i := 0; i < 3; i++ {
    i := i                          // capture-copy in Go
    funcs = append(funcs, func() int { return i })
}
// All closures are valid and callable after the loop ends
fmt.Println(funcs[0](), funcs[1](), funcs[2]()) // → 0 1 2
```

In Ego, the loop variable `i` (and any variable declared inside the loop
body, including an explicit copy `i_copy := i`) goes out of scope when the
loop exits. Any closure that references such a variable produces a runtime
error `"unknown identifier: i"` when called after the loop:

```ego
for i := 0; i < 3; i = i + 1 {
    i_copy := i
    funcs = append(funcs, func() int { return i_copy })
}
funcs[0]()  // ERROR: unknown identifier: i_copy
```

Even the standard Go workaround of taking a copy of the loop variable does
not help in Ego, because the copy variable is itself scoped to the loop
body and goes out of scope at the same time as `i`.

Closures called *immediately* within the loop (before the variable leaves
scope) work correctly.

**Test file:** `tests/functions/scope_advanced.ego` — test
`"functions: stored closure is invalid after loop"` documents this behavior.

**Recommendation:**  
Extend closure capture semantics so that variables referenced by a closure are
kept alive (allocated on the heap or in a persistent parent symbol table) for
the full lifetime of the closure, even after the enclosing scope is popped.
This is the standard "escape analysis" approach used by Go and other languages
with first-class functions.

---

### MEDIUM

#### FUNC-3-MEDIUM — Named nested functions do not capture enclosing function scope

**Affected files:**

- `compiler/function.go` — compilation of named function declarations

**Description:**  
Ego allows named functions to be declared inside other named functions, which
Go does not. However, Ego's nested named functions behave differently from
Go's nested closures: they cannot see the parameters or local variables of the
enclosing named function. Only anonymous function literals (closures) capture
the enclosing scope.

```ego
func outer(a int) int {
    func inner(b int) int {
        return a + b   // ERROR in Ego: unknown identifier: a
    }
    return inner(5)
}

// But closures work:
func outer(a int) int {
    adder := func(b int) int {
        return a + b   // OK: closure captures 'a'
    }
    return adder(5)
}
```

In Go, `func inner` inside `outer` is not legal syntax — you must use a
function literal. Ego's behavior of accepting but not capturing is therefore
surprising: user code that is syntactically valid in Ego but semantically
broken.

**Test file:** `tests/functions/scope_advanced.ego` — tests
`"functions: nested named funcs do not share scope"` and
`"functions: closure captures named func parameter"` document the distinction.

**Recommendation:**  
Either (a) make nested named function declarations capture the enclosing scope
(consistent with the expectations set by function literals), or (b) produce a
compile-time error when a nested named function references a variable from the
enclosing named function's scope. Option (b) is safer and maintains the current
semantics; option (a) is more consistent with user expectations.

---

## Receiver Methods<a name="receiver"></a>

### MEDIUM

#### FUNC-4-MEDIUM — Value receiver method cannot be called on a pointer variable

**Affected files:**

- `bytecode/` — method dispatch
- `data/` — type handling for pointer and value types

**Description:**  
In Go, if a method is declared with a value receiver (`func (p Pair) Sum() int`),
it can be called on both a value and a pointer to that value — Go automatically
dereferences the pointer:

```go
pp := &Pair{a: 3, b: 7}
pp.Sum()   // Go: valid, auto-dereferences to (*pp).Sum()
```

In Ego, calling a value receiver method on a pointer variable produces a
runtime error `"invalid or unsupported data type for this operation"`:

```ego
pp := &Pair{a: 3, b: 7}
pp.Sum()   // Ego ERROR: invalid or unsupported data type
```

Note: the reverse case (calling a pointer receiver method on a value variable)
works in Ego via auto-addressing. It is only value receiver + pointer variable
that fails.

**Test file:** `tests/functions/receivers.ego` — test
`"functions: value receiver on pointer var errors"` documents this behavior.

**Recommendation:**  
In the method dispatch path, detect when the receiver is a pointer type and the
method is declared for the base (non-pointer) type. In this case, auto-deref
the pointer before dispatching, consistent with Go's method set rules.

---

## Type Coercion and Operators<a name="coerce"></a>

### MEDIUM

#### FUNC-5-MEDIUM — Dynamic mode silently accepts wrong-type arguments

**Affected files:**

- `compiler/function.go` — argument type checking
- `bytecode/coerce.go` — runtime coercion

**Description:**  
In Go, passing a value of the wrong type to a function parameter is always a
compile-time error. In Ego's dynamic mode (the default), type mismatches at
function call boundaries are not rejected. The argument retains its actual type
inside the function body, which can produce silently wrong results.

The most common case is passing a string where an integer is expected:

```ego
func double(n int) int { return n * 2 }
double("5")   // No error in dynamic mode
// Inside double: n is still a string "5"
// "5" * 2 = "55" (string repetition — see FUNC-6)
```

The result is neither the expected integer `10` nor an error — it is the string
`"55"` silently coerced into the `int` result variable.

In strict mode (`ego test --types=strict`), the mismatch is caught as a runtime
type error.

**Test file:** `tests/functions/arg_types.ego` — test
`"functions: dynamic mode string to int no error"` documents this behavior.

**Recommendation:**  
In dynamic mode, add an optional warning (controllable by a setting) when a
value of a clearly incompatible type is silently accepted for a statically-typed
parameter. Alternatively, promote this check from strict-only to a standard
runtime check, and only bypass it in a more permissive "relaxed" mode.

---

### LOW

#### FUNC-6-LOW — String multiplication is asymmetric

**Affected files:**

- `bytecode/mul.go` (or equivalent arithmetic opcode handler)

**Description:**  
Ego supports string repetition via the `*` operator when the string is the
**left** operand:

```ego
"A" * 3   // → "AAA"   (string repetition)
3 * "A"   // → ERROR   (tries numeric multiplication, fails)
```

This is intentional behavior (similar to Python) and is documented here for
completeness. The asymmetry means that code like `n * sep` where `sep` is a
string separator will fail even when `"sep" * n` would succeed. Users must
ensure the string is always the left operand.

Additionally, the function `double("5")` where `double` expects an `int`
produces `"55"` (string repetition) rather than `10` (arithmetic) in dynamic
mode — a consequence of FUNC-5 combined with this behavior.

**Test file:** `tests/functions/arg_types.ego` — tests
`"functions: string times int is string repetition"` and
`"functions: int times string is an error"` document the current behavior.

**Recommendation:**  
No change required — the behavior is intentional. If operator symmetry is
desired in the future, `3 * "A"` could be defined to also produce `"AAA"`,
which would be consistent with most languages that support this idiom.
However, changing this would be a breaking change if any code relies on
`int * string` being an error.

---

## Named Return Values<a name="named"></a>

### LOW

#### FUNC-7-LOW — Named return with explicit return value is a compile error

**Affected files:**

- `compiler/return.go` — return statement compilation

**Description:**  
In Go, a function with named return variables can mix bare returns (`return`)
and explicit-value returns (`return someValue`):

```go
func clamp(x, lo, hi int) (result int) {
    if x < lo { return lo }   // explicit value with named return — valid in Go
    if x > hi { return hi }
    result = x
    return                    // bare return — valid in Go
}
```

In Ego, using an explicit return value expression inside a function declared
with named returns is a compile error:

```ego
func clamp(x, lo, hi int) (result int) {
    if x < lo { return lo }   // Ego compile error
    ...
}
```

Users must assign to the named return variable and then use a bare `return`:

```ego
if x < lo { result = lo; return }
```

Bare returns and deferred modifications to named returns both work correctly.

**Test file:** `tests/functions/named_returns.ego` covers the working cases
(bare return, zero values, early return paths). The explicit-value form is
excluded from tests because it does not compile.

**Recommendation:**  
In the return statement compiler (`compiler/return.go`), when the current
function has named return variables and an explicit return value expression
is provided, assign the expression to the named return variable (if there is
exactly one) or to all named return variables in order (if there are multiple)
and then proceed as a bare return. This is consistent with Go's semantics.

---

## Remediation Checklist<a name="checklist"></a>

Use this checklist to track progress as issues are resolved.

### HIGH items

- [ ] **FUNC-1-HIGH** — Allow variadic functions to be called with zero variadic arguments; pass an empty/nil slice for the varargs parameter rather than rejecting the call
- [ ] **FUNC-2-HIGH** — Extend closure capture to keep loop-body variables alive for the lifetime of any closures that reference them, even after the enclosing scope is popped

### MEDIUM items

- [ ] **FUNC-3-MEDIUM** — Nested named functions: either capture enclosing scope (option A) or emit a compile error when a nested named function references the enclosing function's locals/params (option B)
- [ ] **FUNC-4-MEDIUM** — Auto-deref pointer when dispatching a value receiver method, consistent with Go's method set rules
- [ ] **FUNC-5-MEDIUM** — Add a warning (or runtime error in a new mode) when a clearly incompatible type is silently accepted for a statically-typed parameter in dynamic mode

### LOW items

- [ ] **FUNC-6-LOW** — (Informational) String `*` int asymmetry is intentional; consider whether `int * string` should also produce repetition for symmetry
- [ ] **FUNC-7-LOW** — Allow explicit return value expressions inside named-return functions; assign the value to the named return variable before proceeding as a bare return
