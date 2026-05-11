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

```ego
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

```ego
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

**Resolution (May 2026):**  
`compiler/return.go` — `compileReturn` restructured:

1. **Explicit-value path added**: when the function has named return variables
   and the return statement is not at a statement end, each expression is
   parsed, coerced via `c.coercions[i]`, and stored into the corresponding
   named return variable with `Store`. Too many or too few values produce
   `ErrReturnValueCount` / `ErrMissingReturnValues` respectively.

2. **`RunDefers` moved inside the named-return block**, placed *after* any
   explicit assignments. This matches Go semantics: the named variable receives
   the explicit value first; deferred functions then run and may read or modify
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
  that a deferred closure observes the post-assignment value of the named
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

#### FLOW-M1 — `for` init clause only accepts `:=`; `=` for existing variables fails

**Affected files:**

- `compiler/for.go` — for-statement parsing
- `docs/LANGUAGE.md` — documents the `=` form as valid (incorrectly)

**Description:**  
In Go, the init clause of a C-style `for` loop accepts either `:=` (declare a new
variable) or `=` (assign to an existing variable):

```go
var i int
for i = 0; i < 10; i++ { ... }  // Go: valid; i retains its value after the loop
```

The LANGUAGE.md guide documents this pattern explicitly and provides a code example.
In Ego, the init clause only accepts `:=`; using `=` for a pre-declared variable
produces a compile error:

```ego
var i int
for i = 0; i < 10; i++ {}   // Ego: ERROR "missing ':='"
```

Because `:=` always creates a new loop-scoped variable, there is no way to make
the loop counter visible in the enclosing scope after the loop ends, except by
capturing it manually into another variable inside the loop body:

```ego
last := 0
for i := 0; i < 10; i++ { last = i }
// last == 9 after the loop
```

**Test file:** `tests/flow/while_loop.ego` — test
`"flow: for-loop variable accessible after loop via outer scope"` demonstrates the
workaround and references this issue. The LANGUAGE.md code example for `=` in a for
init clause is incorrect.

**Recommendation:**  
Either (a) extend the for-statement parser to accept `=` in the init clause for
pre-declared variables, updating their value in the enclosing scope after the loop,
consistent with Go and with the existing LANGUAGE.md documentation; or (b) remove
the `=` example from LANGUAGE.md and document `:=` as the only supported form.

---

#### FLOW-M2 — Multi-value `case` clause not supported

**Affected files:**

- `compiler/switch.go` — `compileSwitchCase`
- `docs/LANGUAGE.md` — documents `case v1, v2:` as valid (incorrectly)

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

The LANGUAGE.md guide documents this syntax explicitly:

```
case <value2>, <value3>:
    <statements>
```

In Ego, `compileSwitchCase` calls `c.emitExpression()` once and then immediately
expects a `:` token. If a `,` follows the first value, the colon check fails
silently (returns `nil` error, leaving `,` in the token stream) and the compiler
subsequently reports a spurious `"missing 'case'"` error:

```ego
switch x {
case 1, 2, 3:       // ERROR: missing 'case'
    matched = true
}
```

**Workaround:**  
List each value as a separate `case` clause, or use a conditional switch:

```ego
switch x {
case 1:
    matched = true
case 2:
    matched = true
case 3:
    matched = true
}
// or
switch {
case x == 1 || x == 2 || x == 3:
    matched = true
}
```

**Test file:** `tests/flow/switch_advanced.ego` — comment at the top of the file
references this issue; test `"flow: switch on string value"` uses one-value-per-case
as the workaround.

**Recommendation:**  
In `compileSwitchCase`, after emitting the first expression, loop to consume
additional comma-separated expressions and emit a corresponding `Equal` + `Or`
sequence so that the case matches if any value equals the switch expression.
Also update LANGUAGE.md to correctly mark this syntax as unsupported until the
fix lands.

---

#### FLOW-M4 — `defer namedFunc(arg)` evaluates arguments lazily, not eagerly

**Affected files:**

- `compiler/defer.go` — defer statement compilation
- `bytecode/defer.go` (or equivalent) — argument capture at defer registration time

**Description:**  
In Go, the arguments to any deferred function call — named function or closure —
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
the deferred function actually runs:

```ego
x := "first"
defer setLog(x)   // Ego: x is read lazily when defer runs
x = "second"
setLog is called with "second", not "first"
```

This silent behavioral difference can produce hard-to-diagnose bugs when the
variable changes between the `defer` statement and the function's return.

**Workaround:**  
Use the closure form with an explicit argument to get Go-compatible eager capture:

```ego
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

#### FLOW-L1 — Labeled `break` and `continue` not supported

**Affected files:**

- `compiler/for.go` — break/continue compilation
- `tokenizer/` — label parsing

**Description:**  
In Go, `break` and `continue` can name an outer loop label to exit or skip multiple
nesting levels:

```go
outer:
for i := 0; i < 3; i++ {
    for j := 0; j < 3; j++ {
        if j == 1 { break outer }
    }
}
```

In Ego, only bare `break` and `continue` (no label) are supported. The SYNTAX.md
differences table already notes this explicitly. There is no workaround within the
language; a boolean flag variable is the typical substitute.

**Test file:** No specific test exists since the feature is absent. The lack of
labeled loop control is documented in `docs/SYNTAX.md` (differences table, row
"Labeled break/continue").

**Recommendation:**  
If Go compatibility is desired, implement label-aware `break` and `continue` in the
for-statement compiler. This requires the tokenizer to recognize labels (an
`IDENTIFIER :` at the start of a statement that is a loop), the compiler to register
loop depths with their labels, and the break/continue bytecodes to carry an optional
depth operand.

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

```ego
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
- [ ] **FLOW-M1** — `for` init clause only accepts `:=`; using `=` for a pre-declared variable fails to compile, contradicting the LANGUAGE.md documentation
- [ ] **FLOW-M2** — Multi-value `case` clause (`case v1, v2:`) is not supported; produces a spurious compile error, contradicting the LANGUAGE.md documentation
- [ ] **FLOW-M4** — `defer namedFunc(arg)` evaluates arguments lazily (at run time) rather than eagerly (at registration time), diverging from Go behavior

### LOW items

- [x] **FUNC-L1** — (Informational) String `*` int asymmetry is intentional; consider whether `int * string` should also produce repetition for symmetry
- [x] **FUNC-L2** — Allow explicit return value expressions inside named-return functions; assign the value to the named return variable before proceeding as a bare return
- [ ] **FLOW-L1** — Labeled `break` and `continue` are not supported; bare `break`/`continue` only; workaround is a boolean flag variable
- [ ] **FLOW-L2** — `switch init; expr` semicolon-separated form not supported; use pre-assignment or the Ego `switch x := f() { ... }` init-only form instead
