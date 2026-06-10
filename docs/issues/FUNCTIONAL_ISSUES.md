# Functional Issues

This document records known behavioral differences between Ego and Go, as well
as Ego-specific limitations uncovered during testing. It is intended as a living
reference for future development: each section describes the issue, the affected
code, and a concrete recommendation. A checklist at the bottom tracks
remediation progress.

## Structure of this document and naming conventions

There is a section (##) for each area of Ego that was evaluated. Within each
section, issues are grouped by priority (HIGH, MEDIUM, LOW). Each issue has a
unique identifier of the form `AREA-PN`, with three components to the issue
identifier:

- `AREA` the functional area such as "FUNC" for issues with functions,
- `P` is the priority (H for high, M for medium, L for low),
- `N` is a sequence number for that AREA and Priority.

So for example, the first high-priority issue for Functions (FUNC) is named `FUNC-H1`,
and the second low-priority issue around input/output (IO) would be named `IO-L2`.

Additionally, there is a section at the end of this document that shows the
completion state of all issues.

---

## Index of Issue Areas

1. [Functions (arguments, receivers, returns)](#functions)
1. [Flow Control (for, switch, defer)](#flow)
1. [JSON package](#json)
1. [Type system (JSON decode reconstruction)](#types)
1. [@transaction Scripting endpoint](#script)
1. [Remediation Checklist](#checklist)

---

## Functions<a name="functions"></a>

### FUNC High priority issues

#### FUNC-H1 — Calling a variadic function with zero variadic arguments fails

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

```go
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

**Resolution (May 2026):**  
Three changes to `compiler/function.go`:

1. **`generateFunctionBytecode` (lines 150–161):** The `ArgCheck` bytecode was
   emitted as `(len(parameters), -1)` for variadic functions, using the total
   parameter count as both the minimum and (sentinel) maximum. The minimum is
   now emitted as `len(parameters) - 1`, so only the fixed parameters are
   required and the variadic parameter contributes zero or more.

2. **`ParseFunctionDeclaration` (line 487):** The `hasVarArgs` return value from
   `parseParameterDeclaration` was silently discarded with `_`. It is now
   captured and used to set `funcDef.Variadic = hasVarArgs`, so the
   `Declaration.Variadic` flag is correctly propagated when a function is
   called via a `data.Function` wrapper (e.g. as a type method).

3. **`storeOrInvokeFunction` (line 323):** When building a `data.Declaration`
   for a function literal whose declaration was not already known, `Variadic`
   is now derived from the `parms` slice by checking whether the last
   parameter's kind is `VarArgsKind`.

The `ArgCheck` opcode handler (`bytecode/argcheck.go`) and the variable-argument
extraction instruction (`bytecode/stack.go:getVarArgsByteCode`) already handled
the empty-slice case correctly — no changes were needed there.

Tests updated in `tests/functions/variadics.ego`: the two tests that previously
expected errors on zero-argument calls now assert the correct return values
instead, and their names and comments reflect Go-compatible behavior.

---

#### FUNC-H2 — Closures stored during a loop are invalid after the loop ends

**Affected files:**

- `bytecode/bytecode.go` — `ByteCode` struct and accessor methods
- `bytecode/stack.go` — `pushByteCode` opcode handler
- `bytecode/callBytecodeFunction.go` — closure dispatch

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

```go
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

**Resolution (May 2026):**  
The root cause was two separate gaps:

1. **`ByteCode` had no field for the captured scope.** A compiled function
   literal is a single `*ByteCode` object. Every iteration of a loop reuses
   the same pointer — so simply setting a scope on the object would cause all
   iterations to overwrite each other.

2. **`callBytecodeFunction` always parented the closure's symbol table to
   `c.symbols` (the runtime scope at call time)**, not the scope that was
   active when the closure was defined.

Three changes fixed this:

- **`bytecode/bytecode.go`**: Added a `capturedScope *symbols.SymbolTable`
  field to `ByteCode` plus `Clone()`, `CaptureScope()`, and
  `GetCapturedScope()` accessors. `Clone()` returns a shallow copy of the
  struct; the instructions slice is shared (read-only after `Seal`) so the
  clone is cheap.

- **`bytecode/stack.go` — `pushByteCode`**: When the operand is a literal
  `*ByteCode`, the handler clones it and stamps `c.symbols` onto the clone's
  `capturedScope` field before pushing. Each push in a loop iteration
  produces a fresh clone with the scope at that moment. Non-literal bytecodes
  are pushed unchanged.

- **`bytecode/callBytecodeFunction.go`**: When dispatching a literal closure
  whose `capturedScope` is non-nil, the function's symbol table is created as
  a child of the captured scope (via `NewChildSymbolTable` +
  `callFramePushWithTable`) rather than of `c.symbols`. This ensures that
  variable lookup walks up through the captured scope chain, where the loop
  variable is still alive as a Go heap object even after `PopScope` removed
  it from the active parent chain.

The scope object is never freed by `PopScope` — Go's garbage collector keeps
it alive as long as any reference to it exists. Once a closure's
`capturedScope` holds that reference, the entire ancestor chain remains valid
and reachable for the lifetime of the closure, which matches Go's escape
analysis semantics.

Tests updated in `tests/functions/scope_advanced.ego`: the test previously
named `"functions: stored closure is invalid after loop"` is renamed to
`"functions: stored closure survives after loop"` and now asserts that
`captured()` returns `3` (the post-loop value of `i`) instead of asserting
that a runtime error is thrown.

---

### Functions Medium Priority Issues

#### FUNC-M1 — Named nested functions do not capture enclosing function scope

**Affected files:**

- `compiler/function.go` — compilation of named function declarations

**Description:**  
Ego allows named functions to be declared inside other named functions, which
Go does not. However, Ego's nested named functions behave differently from
Go's nested closures: they cannot see the parameters or local variables of the
enclosing named function. Only anonymous function literals (closures) capture
the enclosing scope.

```go
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

**Resolution (May 2026):**  
Option (b) was implemented: a compile-time error is now emitted when a nested
named function body references a parameter or local variable that belongs to an
enclosing named function. The error message guides the user to use a closure
instead: `"nested named function cannot access enclosing function variable; use
a closure"`.

Implementation:

- **`errors/messages.go`**: Added `ErrNestedFunctionScope` (`"nested.function.scope"`)
  with corresponding translations in all three language files.

- **`compiler/compiler.go`**: Added three fields to `Compiler`:
  - `functionLocalScopeStart int` — index in `c.scopes` where the current
    function's own body scopes begin; copied by `Clone`.
  - `ownParamNames map[string]bool` — this function's own parameter names;
    stored separately because `parseParameterDeclaration` adds them to the
    outer compiler's scope before the function body clone is created.
  - `forbiddenSymbols map[string]bool` — names from the immediately enclosing
    named function that produce a compile error if referenced inside this nested
    named function.
  - `Clone` propagates `forbiddenSymbols` and `ownParamNames` to clones so that
    the expression-eval clone in `compiler/expression.go` also enforces the
    boundary.

- **`compiler/function.go` — `generateFunctionBytecode`**: For each non-literal
  (named) function, before the parameter-assignment loop, the function builds a
  `forbiddenForNested` map from the outer function's own param names
  (`c.ownParamNames`) plus any body-scope variables above
  `c.functionLocalScopeStart` that are not globally accessible. Inner function
  own-params are excluded (they are known upfront via the `parameters` slice
  because `parseParameterDeclaration` already added them to `c.scopes`). After
  the clone, `cx.forbiddenSymbols`, `cx.ownParamNames`, and
  `cx.functionLocalScopeStart` are set on the new compiler.

- **`compiler/symbols.go` — `validateSymbol`**: The forbidden-symbols check now
  runs **before** the scope search, not after. Without this ordering, outer
  locals (still present in `cx.scopes` — no truncation) would be silently found
  by the search and accepted.

Closures (function literals, `isLiteral == true`) never receive a
`forbiddenSymbols` assignment, so they continue to see the full enclosing scope
chain, consistent with Go behavior.

Tests in `tests/functions/scope_advanced.ego` cover both directions:
`"functions: nested named funcs do not share scope"` confirms that inner's own
parameters and globals are accessible, while `"functions: closure captures named
func parameter"` confirms that a closure can still see an enclosing named
function's parameters.

---

#### FUNC-M2 — Value receiver method cannot be called on a pointer variable

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

```go
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

**Resolution (May 2026):**  
One change to `bytecode/this.go`:

- **`getThisByteCode`**: After popping the receiver from the "this" stack, if
  the value is `*any` (the runtime representation of an Ego pointer created with
  `&`), it is automatically dereferenced to the underlying value before being
  stored in the receiver variable. This is a pure runtime fix — no compiler
  changes were needed. The dynamic nature of Ego means the same runtime path
  handles both value receivers and pointer receivers, and whether the caller
  passed a pointer or a value variable is only known at runtime.

  For value receivers (`byValue = true`), the dereferenced `*data.Struct` is
  then passed to `$new` for copying, which already had a handler for
  `*data.Struct`. For pointer receivers (`byValue = false`), the dereferenced
  `*data.Struct` is a Go pointer, so field writes inside the method still
  propagate to the original struct. Both paths now work correctly.

Test updated in `tests/functions/receivers.ego`: the test previously named
`"functions: value receiver on pointer var errors"` is renamed to
`"functions: value receiver called on pointer var"` and now asserts that
`pp.Sum()` returns `10` instead of asserting that a runtime error is thrown.

---

#### FUNC-M3 — Dynamic mode silently accepts wrong-type arguments

**Affected files:**

- `compiler/function.go` — argument type checking
- `bytecode/coerce.go` — runtime coercion

**Description:**  
In Go, passing a value of the wrong type to a function parameter is always a
compile-time error. In Ego's dynamic mode (the default), type mismatches at
function call boundaries are not rejected. The argument retains its actual type
inside the function body, which can produce silently wrong results.

The most common case is passing a string where an integer is expected:

```go
func double(n int) int { return n * 2 }
double("5")   // No error in dynamic mode
// Inside double: n is still a string "5"
// "5" * 2 = "55" (string repetition — see FUNC-L1)
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

**Resolution (May 2026):**  
Two changes:

- **`data/types.go`**: Added `IsCoercible(*Type) bool`, which returns `true` for
  scalar types (bool, all integer widths, both float widths, and string). Complex
  types (struct, array, map, channel, function, pointer) return `false` and are
  never silently coerced.

- **`bytecode/arg.go` — `argByteCode`**: After all existing type-guard checks,
  if the expected parameter type is coercible and the argument's type does not
  already match, `data.Coerce` is called to convert the value before it is stored
  in the function's local symbol table. On failure (e.g., `"abc"` where `int` is
  expected), a descriptive error including argument position and original value is
  returned — and because it goes through the normal runtime error path, `try/catch`
  can catch it.

  As a result, `double("5")` now returns `10` instead of `"55"`. Passing a
  non-coercible value such as `double("abc")` raises a catchable runtime error
  rather than producing a silently wrong result.

The previously-used test `"functions: dynamic mode string to int no error"` was
replaced by `"functions: type mismatch is always catchable"` which covers both
the success case (coercible string) and the error case.

---

### Functions Low Priority Issues

#### FUNC-L1 — String multiplication is asymmetric

**Affected files:**

- `bytecode/mul.go` (or equivalent arithmetic opcode handler)

**Description:**  
Ego supported string repetition via the `*` operator when the string was the
**left** operand:

```go
"A" * 3   // → "AAA"   (string repetition, former behavior)
3 * "A"   // → ERROR   (numeric multiplication fails on string)
```

This was documented as intentional (similar to Python), but the asymmetry was
confusing and combined badly with FUNC-5: `double("5")` where `double` expects
`int` silently produced `"55"` (string repetition) rather than `10`.

Additionally, the function `double("5")` where `double` expects an `int`
produces `"55"` (string repetition) rather than `10` (arithmetic) in dynamic
mode — a consequence of FUNC-M3 combined with this behavior.

**Test file:** `tests/functions/arg_types.ego` — tests
`"functions: string times int is string repetition"` and
`"functions: int times string is an error"` document the current behavior.

**Recommendation:**  
The string-repetition shortcut was already listed as informational with no
required action. Given that FUNC-5 has been resolved (coercion at call
boundaries now produces correct results), the string-repetition shortcut became
unnecessary and its asymmetry was a source of confusion. Removing it simplifies
the operator model.

**Resolution (May 2026):**  
The string-repetition special case was removed from `bytecode/math.go`:

- The `multiplyByteCode` handler no longer checks for a `(string, numeric)` pair
  and no longer calls `strings.Repeat`. Both `"A" * 3` and `3 * "A"` now
  produce a runtime error (`"invalid or unsupported data type"`), consistent with
  Go's behavior. The `strings.Repeat` function remains available for the
  intentional use case.

The test `"functions: string times int is string repetition"` was removed from
`tests/functions/arg_types.ego`. The test `"functions: int times string is an
error"` was retained (its stale comment about the left-operand exception was
removed).

The unit test `"multiply strings"` was removed from `bytecode/math_test.go`.

---

#### FUNC-L2 — Named return with explicit return value is a compile error

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

```go
func clamp(x, lo, hi int) (result int) {
    if x < lo { return lo }   // Ego compile error
    ...
}
```

Users must assign to the named return variable and then use a bare `return`:

```go
if x < lo { result = lo; return }
```

Bare returns and defErred modifications to named returns both work correctly.

**Test file:** `tests/functions/named_returns.ego` covers the working cases
(bare return, zero values, early return paths). The explicit-value form is
excluded from tests because it does not compile.

**Recommendation:**  
In the return statement compiler (`compiler/return.go`), when the current
function has named return variables and an explicit return value expression
is provided, assign the expression to the named return variable (if there is
exactly one) or to all named return variables in order (if there are multiple)
and then proceed as a bare return. This is consistent with Go's semantics.

**Resolution (May 2026):**  
`compiler/return.go` — `compileReturn` restructured:

1. **Explicit-value path added**: when the function has named return variables
   and the return statement is not at a statement end, each expression is
   parsed, coerced via `c.coercions[i]`, and stored into the corresponding
   named return variable with `Store`. Too many or too few values produce
   `ErrReturnValueCount` / `ErrMissingReturnValues` respectively.

2. **`RunDefers` moved inside the named-return block**, placed *after* any
   explicit assignments. This matches Go semantics: the named variable receives
   the explicit value first; defErred functions then run and may read or modify
   it; the final value is loaded and returned. The previous bare-return
   behavior is unchanged — when no explicit values are present the assignment
   loop is skipped and `RunDefers` still fires before the loads.

3. **`ErrInvalidReturnValues` check removed** — the error that previously
   rejected any token after the `Return` instruction in the named-return branch
   is deleted.

Three new tests added to `tests/functions/named_returns.ego`:

- `"functions: single named return with explicit value"` — the `clamp` pattern
  from the issue description, exercising all three return paths in one function
- `"functions: two named returns with explicit values"` — multi-value explicit
  return via `split`
- `"functions: named return explicit value interacts with defer"` — verifies
  that a defErred closure observes the post-assignment value of the named
  variable (`f(5)` returns `12`, not `6`)

---

## Flow Control<a name="flow"></a>

### FLOW High priority issues

#### FLOW-H1 — `for range` over an array iterates an immutable snapshot; mutation fails

**Affected files:**

- `bytecode/range.go` — `rangeInitByteCode`, `rangeNextArray`

**Description:**  
In Go, a `for i, v := range a` loop iterates over the array `a` directly and it is
perfectly legal to modify elements through the index during the loop body:

```go
a := []int{1, 2, 3}
for i := range a {
    a[i] *= 10     // modifies original; result: [10, 20, 30]
}
```

In Ego, `rangeInitByteCode` called `actual.SetReadonly(true)` on the original array
before iterating. Because the symbol `a` in the Ego symbol table pointed to the same
`*data.Array` object, any `a[i] = val` inside the loop body reached
`array.Set()` → `if a.immutable > 0 { return ErrImmutableArray }` → runtime error.

A secondary bug existed: if the loop was exited early via `break`,
`rangeNextArray`'s cleanup (`SetReadonly(false)`) was never called, leaving the
array permanently immutable for the rest of the function scope.

**Test file:** `tests/flow/for_range_advanced.ego` — test
`"flow: range over array allows element mutation"` verifies the fixed behavior, and
`"flow: range mutation and direct index loop produce same result"` confirms both loop
forms produce identical results.

**Resolution (May 2026):**  
Two lines removed from `bytecode/range.go`:

1. **`rangeInitByteCode` (line 89):** Removed `actual.SetReadonly(true)` from the
   `*data.Array` case. Ego arrays are fixed-size — there is no structural-modification
   hazard to guard against. Element writes inside the loop body now succeed, matching
   Go's slice range semantics.

2. **`rangeNextArray` (line 195):** Removed the corresponding `actual.SetReadonly(false)`
   call from the loop-exhaustion branch. Since the array is never marked immutable at
   range start, there is nothing to restore. This also eliminates the secondary bug
   where a `break` out of a range loop left the array permanently immutable.

The `immutable` counting semaphore on `data.Array` and the `SetReadonly` method are
unchanged; they continue to serve other legitimate read-only use cases (`_`-prefixed
variables, server runtime arrays, etc.).

---

### FLOW Medium priority issues

#### FLOW-M1 — `for` init clause only accepts `:=`; `=` for existing variables fails — **Resolved**

**Affected files:**

- `compiler/for.go` — for-statement parsing

**Description:**  
In Go, the init clause of a C-style `for` loop accepts either `:=` (declare a new
variable) or `=` (assign to an existing variable):

```go
var i int
for i = 0; i < 10; i++ { ... }  // Go: valid; i retains its value after the loop
```

The LANGUAGE.md guide documents this pattern explicitly and provides a code example.
In Ego, the init clause only accepted `:=`; using `=` for a pre-declared variable
produced a compile error:

```go
var i int
for i = 0; i < 10; i++ {}   // Ego: ERROR "missing ':='"
```

**Resolution:**  
Fixed in `compiler/for.go` — `IsNext(DefineToken)` changed to
`AnyNext(DefineToken, AssignToken)` at line 93. The `assignmentTarget()` function
already handled both forms correctly; only the guard that rejected `=` needed removal.
Test added: `"flow: for-loop init clause with assignment to existing variable"` in
`tests/flow/while_loop.ego`.

---

#### FLOW-M2 — Multi-value `case` clause not supported — **Resolved**

**Affected files:**

- `compiler/switch.go` — `compileSwitchCase`

**Description:**  
In Go, a `switch` case clause can list multiple comma-separated values:

```go
switch day {
case "Sat", "Sun":
    fmt.Println("weekend")
default:
    fmt.Println("weekday")
}
```

In Ego, `compileSwitchCase` called `c.emitExpression()` once and then immediately
expected a `:` token. If a `,` followed the first value, the colon check failed
silently and the compiler subsequently reported a spurious `"missing 'case'"` error.

**Resolution:**  
Fixed in `compiler/switch.go` — `compileSwitchCase` now loops on `CommaToken` after
the first expression, emitting `Equal` + `Or` per additional value so that the case
matches when the switch expression equals any listed value. Works for both value
switches and conditional switches. Tests added: `"flow: switch case with multiple
comma-separated values"` and the existing `"flow: switch on string value"` test was
updated to use `case "Sat", "Sun":` directly.

---

#### FLOW-M4 — `defer namedFunc(arg)` evaluates arguments lazily, not eagerly

**Affected files:**

- `compiler/defer.go` — defer statement compilation
- `bytecode/defer.go` (or equivalent) — argument capture at defer registration time

**Description:**  
In Go, the arguments to any defErred function call — named function or closure —
are evaluated immediately when the `defer` statement executes:

```go
x := "first"
defer fmt.Println(x)   // captures "first" NOW
x = "second"
// Output: "first"
```

In Ego, the closure-with-immediate-invocation form `defer func(p T){...}(arg)`
correctly captures `arg` at registration time. However, `defer namedFunc(arg)` does
**not** capture `arg` eagerly; the argument is re-read from the symbol table when
the defErred function actually runs:

```go
x := "first"
defer setLog(x)   // Ego: x is read lazily when defer runs
x = "second"
setLog is called with "second", not "first"
```

This silent behavioral difference can produce hard-to-diagnose bugs when the
variable changes between the `defer` statement and the function's return.

**Workaround:**  
Use the closure form with an explicit argument to get Go-compatible eager capture:

```go
x := "first"
defer func(v string) { setLog(v) }(x)   // captures "first" eagerly
x = "second"
// setLog receives "first"
```

**Test file:** `tests/flow/defer_lifo.ego` — tests `"flow: named function defer evaluates
arg lazily"` and `"flow: closure arg captured at defer time (eager)"` document both
behaviors side by side.

**Recommendation:**  
When compiling `defer namedFunc(arg)`, evaluate and snapshot all argument expressions
at the point of the `defer` statement and store them in a local temporary, exactly
as is done for the closure-invocation form. This would make both forms consistent
with Go and with each other.

---

### FLOW Low priority issues

#### FLOW-L1 — Labeled `break` and `continue` ✓ FIXED

**Fixed in:** `compiler/compiler.go`, `compiler/for.go`, `compiler/statement.go`

**Description:**  
Labeled `break` and `continue` are now supported. A label is an identifier
immediately followed by `:` placed before a `for` statement (on the same line
or on the preceding line):

```go
outer:
for i := 0; i < 3; i++ {
    for j := 0; j < 3; j++ {
        if j == 1 { break outer }
    }
}
```

Both `break label` and `continue label` find the nearest enclosing loop with
that label and target it. Using an unknown label is a compile-time error.

**Test file:** `tests/flow/labeled_break.ego`

---

#### FLOW-L2 — `switch init; expr` semicolon-separated form not supported

**Affected files:**

- `compiler/switch.go` — `compileSwitchAssignedValue`

**Description:**  
In Go, a switch statement can separate the init statement from the switch expression
with a semicolon:

```go
switch x := compute(); x > 0 {   // init-statement ; condition
case true: ...
case false: ...
}
// or for a value switch:
switch x := getCode(); x {
case 1: ...
case 2: ...
}
```

In Ego, the `switchInit` production accepts either an assignment OR an expression,
but not both separated by a semicolon. The Go form `switch x := f(); x { ... }`
produces a compile error `"missing '{}'"`. The Ego-supported form is
`switch x := f() { ... }` where the init variable becomes the switch value
automatically.

**Workaround:**  
Use a preliminary assignment before the switch:

```go
x := compute()
switch x {
case 1: ...
}
// or the Ego init-only form (init is also the switch value):
switch x := compute() {
case 1: ...
}
```

**Test file:** `tests/flow/switch_advanced.ego` — comment at the top of the file
references this issue; `"flow: switch with init assignment"` demonstrates the
supported Ego init-only form.

**Recommendation:**  
In `compileSwitchAssignedValue`, after parsing the first clause, check for a
semicolon token and, if present, parse a second expression as the switch value.
This would make `switch x := f(); x { ... }` a valid Ego form. Low priority since
the Ego init-only form and the pre-assignment workaround cover most use cases.

**Resolution (May 2026):**  
One change to `compiler/switch.go` — `compileSwitchAssignedValue`:

After `emitExpression()` returns for the first (init) clause, the function now
checks for a `SemicolonToken`. If found, it takes one of two paths:

- **Named init** (`switch x := f(); expr`): stores the first expression result
  under the declared variable name via `DefineSymbol` + `CreateAndStore`, making
  `x` available in the second expression and in case bodies. Then generates a new
  synthetic name, emits the second expression, and stores it as the actual switch
  test value.

- **Anonymous init** (`switch f(); expr`): discards the first expression result
  with `Drop` (side effects still execute), then emits the second expression and
  stores it under the originally-generated synthetic name.

When no semicolon is present, the existing single-expression paths are unchanged.
The `hasScope` flag continues to control whether `PopScope` or `SymbolDelete` is
emitted at the end of the switch — both forms still clean up correctly because in
the named case both variables live inside the pushed scope.

Three new tests added to `tests/flow/switch_advanced.ego`:

- `"flow: switch semicolon form with same variable as switch value"` — verifies
  `switch code := getCode(); code { case 3: ... }` selects the correct branch.
- `"flow: switch semicolon form with boolean condition"` — verifies
  `switch n := getValue(); n > 0 { case true: ... }` with a derived boolean.
- `"flow: switch semicolon init var visible in case body"` — verifies that the
  init variable `code` is readable inside the matching case body.

---

## JSON package<a name="json"></a>

Issues discovered during a review of `runtime/json/` and `tests/json/` in May 2026.
The implementation wraps Go's `encoding/json` and the `jaxon` path-query library.

### JSON High priority issues

#### JSON-H1 — Unmarshal nested struct stores raw Go map instead of Ego struct

**Affected files:**

- `runtime/json/unmarshal.go` — `remapDecodedValue`, `case *data.Struct`

**Description:**
When `json.Unmarshal` writes to an Ego struct and the JSON contains a nested
object, the nested object is stored in the struct field as a raw
`map[string]interface{}` (a Go native map) rather than as the corresponding
Ego struct type that the field was initialized with.

```go
inner := {x: 0, y: 0}
outer := {label: "", pt: inner}
err := json.Unmarshal([]byte(`{"label":"origin","pt":{"x":1,"y":2}}`), &outer)
// outer.pt is now map[string]interface{}, not {x, y}
// reflect.Type(outer.pt) == interface{}   ← not the struct type
```

The root cause is that `remapDecodedValue` iterates over the decoded JSON map
and calls `target.Set(k, v)` with the raw Go value `v` for each key. It does
not check whether the field's declared type is a struct and does not recursively
convert the nested map to that struct type.

Go's `encoding/json` handles this by using reflection on typed struct fields.
Ego's implementation would need to inspect the type of each struct field (via
`target.FieldTypes()` or similar) and call `data.NewStructOfTypeFromMap` when
the field type is a struct kind.

**Test file:** `tests/json/unmarshal.ego` —
`"json: Unmarshal - nested struct stores raw map"` documents this behavior.

**Recommendation:**
In `remapDecodedValue` under `case *data.Struct`, for each key-value pair look
up the declared field type. If the field type is `StructKind` and the decoded
value is `map[string]any`, call `data.NewStructOfTypeFromMap(fieldType, m)` to
create a proper Ego struct before calling `target.Set(k, v)`. This is the same
pattern already applied to struct elements in the `case *data.Array` path.

**Resolution (May 2026):**
One change to `runtime/json/unmarshal.go` — `remapDecodedValue`, `case *data.Struct`:

Inside the `for k, v := range m` loop, before the `target.Set(k, v)` call, a
type-check was added:

```go
if mm, ok := v.(map[string]any); ok {
    if fieldType, fErr := target.Type().Field(k); fErr == nil && fieldType.Kind() == data.StructKind {
        v = data.NewStructOfTypeFromMap(fieldType, mm)
    }
}
```

`target.Type().Field(k)` (`data/types.go:1299`) returns the declared `*data.Type`
for field `k`. When its kind is `StructKind` and the decoded JSON value is a
`map[string]any`, `data.NewStructOfTypeFromMap` converts the raw map into a
properly-typed Ego struct before it is stored. Fields whose declared type is not
`StructKind` (scalars, arrays, maps) fall through unchanged. Unknown field names
produce a non-nil `fErr` and also fall through, letting the existing
`target.Set(k, v)` error path handle them (covered separately by JSON-M3).

This is the same pattern already used in the `case *data.Array` branch (lines
123–126 before this change).

Test in `tests/json/unmarshal.ego` renamed from
`"json: Unmarshal - nested struct stores raw map"` to
`"json: Unmarshal - nested struct field is correct type"` and updated to assert
that `outer.pt.x == 1` and `outer.pt.y == 2` after unmarshal rather than
asserting the buggy `interface{}` type.

---

#### JSON-H2 — Unmarshal type mismatch raises exception instead of returning error

**Affected files:**

- `runtime/json/unmarshal.go` — `remapDecodedValue`, `default` case

**Description:**
When the JSON value cannot be coerced to the destination type (for example,
the JSON string `"notanumber"` decoded into an `int` variable), the error does
not come back as the return value of `json.Unmarshal`. Instead it is raised as
a catchable exception that bypasses the normal return path.

```go
var i int
err := json.Unmarshal([]byte(`"notanumber"`), &i)
// err is nil -- the error did NOT come back here
// A try/catch block would catch the exception instead
```

The root cause is that the `default` case of `remapDecodedValue` returns
`(nil, err)` — not a `data.List` — when `data.Coerce` fails. Because
`json.Unmarshal` is declared with a single `error` return type and its wrapper
returns a non-list result with a non-nil Go error, the runtime takes the
special single-error-return path in `callRuntimeFunction`, which routes the
error as a catchable exception rather than the Ego-level return value.

In Go, `json.Unmarshal` always returns the error normally; callers use
`if err := json.Unmarshal(...); err != nil {}`.

**Test file:** `tests/json/unmarshal.ego` —
`"json: Unmarshal - type mismatch is catchable exception"` documents this
behavior.

**Recommendation:**
In the `default` case of `remapDecodedValue`, change the error return from
`return nil, err` to `return data.NewList(err), nil`. This ensures the error
flows through the `data.List` path and becomes the Ego-level return value of
`json.Unmarshal`, consistent with all other error paths in the function.

**Resolution (May 2026):**
One change to `runtime/json/unmarshal.go` — `remapDecodedValue`, `default` case:

The `data.Coerce` failure path was:

```go
if err != nil {
    return nil, err
}
```

Changed to:

```go
if err != nil {
    return data.NewList(errors.New(err).In("Unmarshal")), nil
}
```

Because `json.Unmarshal` is declared with a single `error` return type, returning
`(nil, err)` (a non-list result with a non-nil Go error) caused `callRuntimeFunction`
to take the special single-error-return path: the error became a catchable exception
rather than the Ego-level return value of `Unmarshal`. Wrapping it in `data.NewList`
forces the list path, where the Go second return is ignored and the error inside the
list becomes what the caller sees as the `error` return of `json.Unmarshal`.

Test in `tests/json/unmarshal.ego` renamed from
`"json: Unmarshal - type mismatch is catchable exception"` to
`"json: Unmarshal - type mismatch returns error"` and simplified: the `try/catch`
wrapper was removed. The test now calls `err := json.Unmarshal(...)` directly and
asserts `err != nil`, matching Go's `encoding/json` behavior.

---

### JSON Medium priority issues

#### JSON-M1 — Marshal of []byte produces "null" instead of base64 string

**Affected files:**

- `data/sanitize.go` — `Sanitize` function, `*data.Array` of `ByteKind`

**Description:**
Go's `encoding/json` encodes a `[]byte` value as a base64-encoded JSON string.
In Ego, `json.Marshal([]byte{65, 66, 67})` produces `"null"` instead of
`"QUJD"`.

The cause is that `data.Sanitize` — which converts Ego runtime values to Go
native types before passing them to `json.Marshal` — does not convert a
`*data.Array` whose element type is `ByteKind` into a native Go `[]byte` slice.
The sanitized value ends up as `nil`, which `json.Marshal` encodes as `null`.

```go
byt := []byte{65, 66, 67}
b, err := json.Marshal(byt)
// Expected:  b == []byte(`"QUJD"`)
// Actual:    b == []byte("null")
```

This also affects `json.WriteFile` when given a byte array value to write.

**Test file:** `tests/json/marshal.ego` —
`"json: Marshal - []byte produces null"` documents the current (incorrect)
behavior.

**Recommendation:**
In `data.Sanitize`, add a case for `*data.Array` values whose `Type().Kind()`
is `data.ByteKind`: call `array.GetBytes()` and return the resulting `[]byte`
slice. This will allow `json.Marshal` to receive a native `[]byte` and produce
the standard base64 encoding.

**Resolution (May 2026):**
One change to `data/sanitize.go` — `Sanitize`, `case *Array`:

Before returning `v.data`, a `ByteKind` check was added:

```go
if v.Type().Kind() == ByteKind {
    return v.GetBytes()
}
```

`GetBytes()` returns the internal `[]byte` backing the byte array. Passing that
native slice to `json.Marshal` triggers Go's standard base64 encoding. For all
other array element types the existing `v.data` path is unchanged.

Test in `tests/json/marshal.ego` renamed from
`"json: Marshal - []byte produces null"` to
`"json: Marshal - []byte produces base64 string"` and updated to assert
`string(b)` equals the JSON string `"QUJD"` (the base64 encoding of `[65,66,67]` = `"ABC"`).

---

#### JSON-M2 — Parse of empty JSON array with "." returns error

**Affected files:**

- External: `github.com/tucats/jaxon` — `GetItem` function

**Description:**
`json.Parse("[]", ".")` returns an error `"element not found: ."` rather than
a string representation of the empty array. By contrast, `json.Parse("{}", ".")`
returns `"{}"` without error.

```go
a, err := json.Parse("{}", ".")  // a == "{}", err == nil   ← correct
b, err := json.Parse("[]", ".")  // b == "",   err != nil   ← inconsistent
```

The inconsistency is in the `jaxon` library's handling of the root expression
`"."` applied to a JSON array vs. a JSON object. Jaxon returns the root object
representation for `{}` but fails for `[]`.

**Test file:** `tests/json/parse.ego` —
`"json: Parse - empty array root returns error"` documents this behavior.

**Recommendation:**
If this is a jaxon library limitation, a workaround could be applied in
`runtime/json/parse.go`: detect when `jaxon.GetItem` returns an "element not
found" error for a root expression `"."`, re-parse the input JSON, and if the
top-level value is an array return its string representation directly. This
avoids changing the external library.

**Resolution (May 2026):**
One change to `runtime/json/parse.go` — `parse`:

After `jaxon.GetItem` returns an error, a recovery block now checks whether
the expression is `"."` and the input is a valid JSON array:

```go
if err != nil && expression == "." {
    var root any
    if decodeErr := stdjson.Unmarshal([]byte(text), &root); decodeErr == nil {
        if _, ok := root.([]any); ok {
            if encoded, encErr := stdjson.Marshal(root); encErr == nil {
                return data.NewList(string(encoded), nil), nil
            }
        }
    }
}
```

`encoding/json` is imported as `stdjson` to avoid the naming collision with the
enclosing package. `json.Marshal([]any{})` returns `[]byte("[]")`, so
`json.Parse("[]", ".")` now returns `("[]", nil)`, consistent with
`json.Parse("{}", ".")` returning `("{}", nil)`.

Test in `tests/json/parse.ego` renamed from
`"json: Parse - empty array root returns error"` to
`"json: Parse - empty array root returns empty array string"` and updated to
assert `err == nil` and `result == "[]"`.

---

#### JSON-M3 — Unmarshal into struct with unknown fields returns error

**Affected files:**

- `runtime/json/unmarshal.go` — `remapDecodedValue`, `case *data.Struct`

**Description:**
In Go, `json.Unmarshal` ignores JSON keys that do not correspond to any field
in the destination struct. In Ego, such keys cause `json.Unmarshal` to return
an error: `"invalid field name for type: <key>"`.

Additionally, because Go's `map` iteration order is non-deterministic, some
fields may already be written to the struct before the unknown key is
encountered. This produces a partial update with an error, which is both
unexpected and non-deterministic.

```go
person := {name: ""}
err := json.Unmarshal([]byte(`{"name":"Alice","ghost":"x"}`), &person)
// err != nil: "invalid field name for type: ghost"
// person.name may or may not be "Alice" depending on iteration order
```

**Test file:** `tests/json/unmarshal.ego` —
`"json: Unmarshal - unknown fields return error"` documents this behavior.

**Recommendation:**
In `remapDecodedValue` under `case *data.Struct`, check whether the key exists
as a field of the struct before calling `target.Set`. If the key does not exist,
silently skip it (matching Go's default behavior). If strict unknown-field
rejection is desired it can be added as a separate option or configuration
setting.

**Resolution (May 2026):**
One change to `runtime/json/unmarshal.go` — `remapDecodedValue`, `case *data.Struct`:

The loop now captures the bool result of `target.Get(k)` and skips the field when
not found, rather than discarding the found indicator and letting `target.Set` fail:

```go
existing, found := target.Get(k)
if !found {
    continue
}
```

`data.Struct.Get` returns `(value, bool)` — `false` for any key that is not a
declared field. The `continue` discards unknown keys silently, matching Go's
`encoding/json` default behavior. Known fields are still updated as before.

Test in `tests/json/unmarshal.ego` renamed from
`"json: Unmarshal - unknown fields return error"` to
`"json: Unmarshal - unknown fields are silently ignored"` and updated to assert
`err == nil` and `person.name == "Alice"`.

---

## Type system<a name="types"></a>

Issues discovered during investigation of JSON unmarshal correctness in May 2026.
These affect how Ego's runtime reconstructs typed values from raw Go values produced
by the standard `encoding/json` decoder.

### TYPE High priority issues

#### TYPE-H1 — json.Unmarshal cannot reconstruct deeply nested or array-element types

**Affected files:**

- `runtime/json/unmarshal.go` — `remapDecodedValue`

**Description:**
Go's `encoding/json` decodes all JSON into a small set of native Go types:
`map[string]any`, `[]any`, `float64`, `string`, `bool`, and `nil`. Ego's
`remapDecodedValue` then attempts to convert that generic tree into typed Ego
values, guided by the destination pointer. The current implementation is a flat
`switch` on the destination's runtime type (`*data.Struct`, `*data.Map`,
`*data.Array`, scalar) with case-specific conversion logic at each level. This
approach has two related gaps:

**Gap 1 — Array elements are not converted when the element type is `interface{}`.**

The standard Ego idiom for an array of structs is `[]any{pt, pt}`, which gives the
array an element type of `InterfaceKind`. The existing `case *data.Array` code only
converts `map[string]any` elements to Ego structs when `target.Type().Kind() ==
data.StructKind`. For `[]any` the check is always false, so JSON array elements that
represent objects are stored as raw `map[string]any` rather than the struct values
the array already contained before the unmarshal call.

```go
pt := {x: 0, y: 0}
arr := []any{pt, pt}
err := json.Unmarshal([]byte(`[{"x":1,"y":2},{"x":3,"y":4}]`), &arr)
// arr[0] is map[string]interface{}, not {x:1, y:2}
```

The existing elements of `arr` before the call carry the struct type information
(they are `*data.Struct` values), but `SetSize` is called before they are examined,
and the conversion loop has no access to that type hint.

**Gap 2 — Nesting beyond one level is not handled.**

The fix for JSON-H1 converts a `map[string]any` value into a struct when the
containing struct's field type is `StructKind`. But if that nested struct itself has
a field that is an array of structs, or if a map value is a struct, those inner
conversions are not applied. Only one level of nesting is handled in any given
`case` branch.

**Root cause:**
The flat `switch`-based approach in `remapDecodedValue` is inherently limited to
one level of conversion per call. Arbitrarily deep nesting (arrays of structs
containing arrays of maps containing structs, etc.) requires a recursive traversal
of the decoded value tree guided by the destination object's type information at
every level.

**Recommendation:**
Redesign `remapDecodedValue` as a recursive function. The strategy:

1. Let `encoding/json` decode normally into a bare `any` (current behavior —
   no change here).
2. Replace the flat `switch` with a recursive `convertToEgoType(decoded any,
   targetType *data.Type) any` that walks both the decoded value tree and the
   declared type tree simultaneously:
   - If the target type is `StructKind` and the decoded value is `map[string]any`,
     create a new `*data.Struct` of that type and recursively convert each field
     value by looking up the field's declared type.
   - If the target type is `ArrayKind` and the decoded value is `[]any`, create a
     new `*data.Array` of the declared element type and recursively convert each
     element.
   - If the target type is `MapKind` and the decoded value is `map[string]any`,
     create a new `*data.Map` and recursively convert each value using the map's
     declared value type.
   - For scalar types, apply `data.Coerce` as today.
   - If the target type is `InterfaceKind`, apply a best-effort conversion: a
     `map[string]any` becomes an Ego `*data.Map`, a `[]any` becomes a `*data.Array`
     of `InterfaceType`, scalars are kept as-is. This preserves the current
     no-model behavior.
3. The entry point (`remapDecodedValue`) seeds the recursion with the declared type
   of the destination pointer's current value.

This approach handles arbitrary nesting depth, eliminates the per-type special
cases, and makes it straightforward to add future target types (e.g., typed
channels, user-defined types).

**Resolution (May 2026):**
`remapDecodedValue` was refactored and a new `reconstructValue(decoded any, model any) (any, error)`
helper was added in `runtime/json/unmarshal.go`.

**Design:** Instead of a `*data.Type`-based recursive target type, the helper takes the
*existing Ego value* at each position as the `model` and uses its concrete Go type (via a
`switch` on the model) to drive conversion. This sidesteps the need for an unexported
`ValueType()` accessor on `*data.Type` and works naturally with Ego's runtime representation:

- `model = *data.Struct` → creates a new `*data.Struct` of the same type, recurses into each field using `m.Get(fieldName)` as the per-field model.
- `model = *data.Array` → creates a new `*data.Array` with the same element type (`m.Type()`), uses the original element at each index (or the first element as a fallback for new slots) as the per-element model.
- `model = *data.Map` → creates a new `*data.Map` with the same key and value types; uses `InstanceOfType(m.ElementType())` as the per-value model (or `nil` when element type is `interface{}` to avoid spurious coercion).
- scalar / nil model → attempt `data.Coerce` to the model's type; fall back to best-effort (`map[string]any` → Ego map, `[]any` → Ego `[]any` array, scalars as-is).

**`remapDecodedValue` changes:**

- `case *data.Struct`: replaced the JSON-H1 single-level fix with `target.Get(k)` + `reconstructValue(v, existing)` for each field.
- `case *data.Array`: before `SetSize`, captures `origLen` and `elemModel` (first existing element). After resize, calls `reconstructValue(v, thisModel)` per element using the original element as the type hint.
- `case *data.Map`: uses `InstanceOfType(target.ElementType())` as `elemModel` (nil when interface); calls `reconstructValue(v, elemModel)` per value. The `default` case is unchanged (JSON-H2 tracked separately).

**Observable behavior change:** Unmarshaling a JSON array of objects into `[]any{}` (no struct model available) now produces Ego `*data.Map` elements instead of raw Go `map[string]any` values. The Ego type seen by callers changes from `interface{}` to `map[string]interface{}`. This is strictly better — callers can now index into the maps with normal Ego map syntax.

Test in `tests/json/unmarshal.ego`:

- `"json: Unmarshal - array of objects returns Go maps"` → renamed `"json: Unmarshal - array of objects into []any gives Ego maps"` and updated assertion from `interface{}` to `"map[string]interface{}"`.
- `"json: Unmarshal - array of structs restores element type"` added: verifies that a `[]any{pt, pt}` destination produces typed struct elements after unmarshal.

---

## @transaction Scripting endpoint<a name="script"></a>

Issues discovered during the creation of API tests for the `@transaction`
endpoint (`POST /dsns/{dsn}/tables/@transaction`) in May 2026. The handler
lives in `server/tables/scripting/handler.go`; individual opcodes are
implemented in sibling files (`rows.go`, `insert.go`, `drop.go`, etc.).

### SCRIPT High priority issues

#### SCRIPT-H1 — `readrows` opcode returns at most one row

**Affected files:**

- `server/tables/scripting/rows.go` — `doRows`, `fakeURL` construction

**Description:**
The `readrows` opcode is intended to return all rows that match the specified
filters — an unbounded SELECT. However, `doRows` constructs its internal URL
with a hard-coded `?limit=1` parameter, which was copied from `doSelect` (the
`select` opcode handler) where limiting to one row is the correct behavior:

```go
fakeURL, _ := url.Parse("http://localhost/tables/" + task.Table + "/rows?limit=1")
```

Because `parsing.FormSelectorDeleteQuery` honours the `limit` query parameter
from the URL, this means the `readrows` opcode silently returns at most one row
regardless of how many rows match. Transactions that use `readrows` to
aggregate result sets will receive incomplete data with no error or warning.

The `sql` opcode with a raw `SELECT` statement is unaffected — it bypasses
`fakeURL` entirely and calls the SQL layer directly.

**Test file:** `tools/apitest/tests/4-dsns/dsns-304-tx-readrows.json` — the
test inserts three rows and then calls `readrows`. It expects `count=3`; if
this bug is present the test will fail with `count=1`.

**Recommendation:**
Remove the `?limit=1` fragment from the `fakeURL` in `doRows`, or replace it
with a sufficiently large sentinel value (e.g., `?limit=10000000`). The
`select` opcode (`doSelect`) should retain `?limit=1` since it is a
point-lookup by design. A cleaner solution is to add an explicit `limit` field
to `defs.TXOperation` and pass it through; the `readrows` default would be
unlimited.

**Resolution (May 2026):**
`doRows()` in `server/tables/scripting/rows.go` was modified to remove the
unwanted `?limit=1` parameter on the "readrows" operation. This is distinct
from "select" which intentionally reads a single row to set dictionary variables
to the values of each column. The "readrows" operator wants to read the entire
rowset into memory, as it will be the return data for the @transaction call.

Along the way, fixed an issue where the escape operation "/{{value}}" was poorly
chosen, as it interfered with the injection of dictionary values at the apitest
level into URL paths. The escape was changed to "\{{value}}" to avoid this.
APITEST streams were updated accordingly.

Also, augmented the FullTableName function to be told the provider name, so the
name path could be formed properly according to the provider. Specifically,
if the provider is sqlite3 then we can't and shouldn't use dotted names.

---

### SCRIPT Medium priority issues

#### SCRIPT-M1 — `_rows_` is always 0 for error conditions after an `insert` opcode

**Affected files:**

- `server/tables/scripting/handler.go` — main dispatch loop, `insertOpcode` case

**Description:**
When processing each opcode the handler maintains two variables that are
exposed to user-defined error condition expressions: `_rows_` (rows affected by
the current operation) and `_all_rows_` (cumulative rows affected so far).

For every opcode except `insert`, the handler captures the return value of the
opcode handler into `count` before evaluating error conditions:

```go
count, httpStatus, operationErr = doSelect(...)   // count updated
...
evalSymbols.SetAlways("_rows_", count)
```

The `insertOpcode` case does not update `count` — it increments `rowsAffected`
directly and ignores the return value of `doInsert`:

```go
case insertOpcode:
    httpStatus, operationErr = doInsert(...)
    rowsAffected++
    // count is never set — still 0 from declaration
```

Because `evalSymbols.SetAlways("_rows_", count)` is evaluated after the
`switch`, `_rows_` is always `0` for `insert` operations regardless of the
actual row count. A user-defined error condition such as `EQ(_rows_, 0)` will
always trigger after an `insert`, even when a row was successfully inserted.

**Recommendation:**
Set `count = 1` immediately before or after `rowsAffected++` in the
`insertOpcode` branch, so that `_rows_` reflects the one row inserted:

```go
case insertOpcode:
    httpStatus, operationErr = doInsert(...)
    count = 1
    rowsAffected++
```

**Resolution (May 2026):**
One change to `server/tables/scripting/handler.go` — `insertOpcode` case:

Added `count = 1` between the `doInsert` call and `rowsAffected++`:

```go
case insertOpcode:
    httpStatus, operationErr = doInsert(session.ID, session.User, db, task, n+1, &dictionary)
    count = 1
    rowsAffected++
```

`doInsert` always inserts exactly one row (its return value is an HTTP status
code, not a row count). Setting `count = 1` unconditionally is correct because
the error-condition evaluation block is guarded by `operationErr == nil`, so
`_rows_` is only read from `count` when the insert actually succeeded.

Two API tests added to `tools/apitest/tests/4-dsns/`:

- `dsns-314-tx-ins-rowcount.json` — inserts a row with an `EQ(_rows_, 0)`
  error condition; before the fix this condition always triggered (returning
  409 Conflict); after the fix `_rows_` is 1 so the condition does not trigger
  and the transaction commits with HTTP 200 and `count=1`.

- `dsns-315-tx-ins-errcond.json` — inserts a row with an `EQ(_rows_, 1)`
  error condition; confirms that a user-defined condition that fires on a
  successful insert (`_rows_ == 1`) correctly rolls back the transaction and
  returns 409 Conflict with the custom error message.

The existing `dsns-314-tx-drop.json` was renumbered to `dsns-316-tx-drop.json`
to preserve alphabetical execution order.

---

#### SCRIPT-M2 — `drop` opcode does not flush the schema cache

**Affected files:**

- `server/tables/scripting/handler.go` — `needCacheFlush` logic
- `server/tables/scripting/drop.go` — `doDrop`

**Description:**
The server maintains an in-memory schema cache to avoid repeated database
introspection for table column lists and types. After a DDL change the cache
must be flushed so that subsequent operations see the updated schema.

The handler sets `needCacheFlush = true` only when `sqlOpcode` returns a
flush signal — specifically for `ALTER TABLE` statements executed via the `sql`
opcode. The `drop` opcode (`dropOpCode` / `doDrop`) does not set
`needCacheFlush`, meaning the schema cache retains an entry for a table that
no longer exists.

In a test sequence that drops and recreates a table (or in any transaction that
drops a table and then the same connection subsequently references that table),
a stale cache entry may produce `"column 'x' does not exist"` errors or
incorrect column type metadata.

**Recommendation:**
Either have `doDrop` return a flush-needed flag (parallel to the `doSQL`
signature) and propagate it to `needCacheFlush`, or unconditionally set
`needCacheFlush = true` in the `dropOpCode` case of the handler dispatch loop.

**Resolution (May 2026):**
Two changes, fixing both the `drop` opcode and the `sql` opcode `DROP TABLE`
path simultaneously:

**`server/tables/scripting/handler.go` — `dropOpCode` case:**

```go
case dropOpCode:
    httpStatus, operationErr = doDrop(session.ID, session.User, db, task, n+1, &dictionary)
    if operationErr == nil {
        needCacheFlush = true
    }
```

The guard `operationErr == nil` ensures that a failed drop (e.g. table does not
exist → 404) does not spuriously flush the cache before the transaction is rolled
back.

**`server/tables/scripting/sql.go` — `doSQL`, `cacheFlush` detection:**

```go
// Before: ALTER TABLE only
if len(tokens) > 2 && tokens[0] == "alter" && tokens[1] == "table" {
    cacheFlush = true
}
// After: ALTER TABLE and DROP TABLE
if len(tokens) > 2 && tokens[1] == "table" && (tokens[0] == "alter" || tokens[0] == "drop") {
    cacheFlush = true
}
```

`CREATE TABLE` is intentionally excluded: a brand-new table has no stale cache
entry to evict.

**API tests added** in `tools/apitest/tests/4-dsns/`:

*Subgroup A — `drop` opcode (table `sca{{SQLUUID}}`, tests 317–321):*

- `dsns-317-tx-sca-create.json` — creates `sca{{SQLUUID}}` with two columns (id, name) and inserts one row.
- `dsns-318-tx-sca-warm.json` — `GET .../sca{{SQLUUID}}/rows` warms the `SchemaCache` with the 2-column layout.
- `dsns-319-tx-sca-drop-recreate.json` — `@transaction` with `drop sca{{SQLUUID}}` + `sql CREATE TABLE sca{{SQLUUID}} (..., extra TEXT)` + `insert {id:2, name:"after", extra:"bonus"}`.
- `dsns-320-tx-sca-verify.json` — `GET .../sca{{SQLUUID}}/rows?columns=extra` must return **200** with `rows.0.extra == "bonus"`. Without the fix, the stale cache does not know about `extra` and `ReadRows` returns **400 "invalid column name: extra"`.
- `dsns-321-tx-sca-cleanup.json` — drops `sca{{SQLUUID}}`.

*Subgroup B — `sql` opcode `DROP TABLE` (table `scb{{SQLUUID}}`, tests 322–326):*

- `dsns-322-tx-scb-create.json` — same setup with `scb{{SQLUUID}}`.
- `dsns-323-tx-scb-warm.json` — warms the cache for `scb{{SQLUUID}}`.
- `dsns-324-tx-scb-sqldrop-recreate.json` — `@transaction` with `sql "DROP TABLE scb{{SQLUUID}}"` + `sql CREATE TABLE` (with `extra`) + `insert`.
- `dsns-325-tx-scb-verify.json` — `GET .../scb{{SQLUUID}}/rows?columns=extra` must return **200**; same failure mode without the fix.
- `dsns-326-tx-scb-cleanup.json` — drops `scb{{SQLUUID}}`.

The verify tests (`dsns-320`, `dsns-325`) are the regression anchors: they pass
only when the schema cache has been flushed and `ReadRows` re-queries the
database to obtain the updated column list for the recreated table.

---

### SCRIPT Low priority issues

#### SCRIPT-L1 — Empty transaction body returns plain text, not JSON

**Affected files:**

- `server/tables/scripting/handler.go` — empty-task early return (lines 51–59)

**Description:**
When a `POST /@transaction` request is received with an empty body (zero
operations), the handler writes a plain text message and returns HTTP 200:

```go
text := i18n.T("msg.table.tx.empty")
w.WriteHeader(http.StatusOK)
_, _ = w.Write([]byte(text))
```

No `Content-Type` response header is set, so the response defaults to
`text/plain; charset=utf-8`. Every other `@transaction` response carries a
structured `application/vnd.ego.*+json` content type. A client that inspects
the `Content-Type` header to decide how to parse the response will need a
special case for the empty-body path.

**Recommendation:**
Replace the plain text response with a `rowcount+json` response carrying
`count: 0`, consistent with how a non-empty transaction with zero affected rows
is encoded. This makes all `@transaction` success responses uniform and
removes the special-case parsing requirement from clients.

---

## Remediation Checklist<a name="checklist"></a>

Use this checklist to track progress as issues are resolved.

### HIGH items

- [x] **FUNC-H1** — Allow variadic functions to be called with zero variadic arguments; fixed by correcting the `ArgCheck` minimum count in `compiler/function.go` and propagating `Declaration.Variadic` from the parser
- [x] **FUNC-H2** — Extend closure capture to keep loop-body variables alive for the lifetime of any closures that reference them, even after the enclosing scope is popped; fixed by cloning literal `ByteCode` at push time, capturing `c.symbols` into the clone, and using the captured scope as the closure's parent table in `callBytecodeFunction`
- [x] **FLOW-H1** — `for range` over an array now allows element writes via `a[i]` inside the loop body, matching Go semantics; fixed by removing `SetReadonly` calls in `rangeInitByteCode` and `rangeNextArray`

### MEDIUM items

- [x] **FUNC-M1** — Nested named functions now produce a compile-time error when referencing the enclosing function's parameters or locals; closures continue to capture the enclosing scope normally
- [x] **FUNC-M2** — Auto-deref pointer when dispatching a value receiver method, consistent with Go's method set rules
- [x] **FUNC-M3** — Add a warning (or runtime error in a new mode) when a clearly incompatible type is silently accepted for a statically-typed parameter in dynamic mode
- [x] **FLOW-M1** — `for` init clause only accepts `:=`; using `=` for a pre-declared variable fails to compile, contradicting the LANGUAGE.md documentation
- [x] **FLOW-M2** — Multi-value `case` clause (`case v1, v2:`) is not supported; produces a spurious compile error, contradicting the LANGUAGE.md documentation
- [ ] **FLOW-M4** — `defer namedFunc(arg)` evaluates arguments lazily (at run time) rather than eagerly (at registration time), diverging from Go behavior

### LOW items

- [x] **FUNC-L1** — (Informational) String `*` int asymmetry is intentional; consider whether `int * string` should also produce repetition for symmetry
- [x] **FUNC-L2** — Allow explicit return value expressions inside named-return functions; assign the value to the named return variable before proceeding as a bare return
- [x] **FLOW-L1** — Labeled `break` and `continue` are now supported; `break label` and `continue label` target any enclosing labeled `for` loop
- [x] **FLOW-L2** — `switch init; expr` semicolon-separated form now supported; `switch x := f(); x { ... }` and `switch x := f(); x > 0 { ... }` are valid Ego forms

### JSON items

- [x] **JSON-H1** — Unmarshal nested struct: nested JSON objects are now converted to the declared Ego struct field type via `data.NewStructOfTypeFromMap` in `remapDecodedValue`
- [x] **JSON-H2** — Unmarshal type mismatch: coercion error now returned as the Ego-level `error` value by wrapping in `data.NewList`; callers use `if err := json.Unmarshal(...); err != nil` as in Go
- [x] **JSON-M1** — Marshal `[]byte` now produces a base64 JSON string; fixed by returning `array.GetBytes()` from `data.Sanitize` when the array element kind is `ByteKind`
- [x] **JSON-M2** — `json.Parse("[]", ".")` now returns `"[]"`; jaxon limitation worked around in `runtime/json/parse.go` by re-decoding the input when expression is `"."` and the root value is a JSON array
- [x] **JSON-M3** — Unmarshal into struct now silently skips unknown JSON fields; fixed by checking `target.Get(k)` found-bool before calling `target.Set` in `remapDecodedValue`

### TYPE items

- [x] **TYPE-H1** — `json.Unmarshal` now reconstructs arbitrarily nested types; `remapDecodedValue` uses the new recursive `reconstructValue(decoded, model)` helper that drives conversion from the existing Ego value at each position

### SCRIPT items

- [x] **SCRIPT-H1** — `readrows` opcode returns at most one row; `doRows` in `server/tables/scripting/rows.go` constructs its `fakeURL` with `?limit=1` (copied from `doSelect`), silently capping result sets to one row
- [x] **SCRIPT-M1** — `_rows_` is always 0 in error condition expressions after an `insert` opcode; fixed by adding `count = 1` in the `insertOpcode` branch of `handler.go` before `evalSymbols.SetAlways("_rows_", count)` is reached
- [x] **SCRIPT-M2** — `drop` opcode now sets `needCacheFlush = true` on success; `sql` opcode extended to also flush on `DROP TABLE` (previously only `ALTER TABLE` triggered a flush)
- [x] **SCRIPT-L1** — Empty transaction body returns plain text with no `Content-Type` header; all non-empty responses are structured JSON — the empty path returns 200 with msg set to "No transactions in task".
