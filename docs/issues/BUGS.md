# Ego Language Bug Report

This document records bugs discovered through systematic testing of the Ego language
(version 1.8). Each bug has a reproducer program, actual output, expected output, and
a severity classification.

> **Relationship to `FUNCTIONAL_ISSUES.md`:** That document tracks known behavioral
> differences and their resolutions. This document records newly discovered bugs not
> yet captured there. `FLOW-M4` (defer lazy argument evaluation) is already tracked
> in FUNCTIONAL_ISSUES.md and is included here only for cross-reference.

---

## Severity Classification

| Level | Meaning |
| :---- | :------ |
| **CRITICAL** | Incorrect behavior with no workaround; likely data corruption or crashes |
| **HIGH** | Core language feature behaves incorrectly; program logic is wrong |
| **MEDIUM** | Spec or documented behavior violated; workaround usually exists |
| **LOW** | Minor inconsistency, missing feature, or documentation error |

---

## Summary Table

| ID | Severity | Title |
| :--- | :------- | :---- |
| BUG-01 | HIGH ✓ | `for v := range ch` over channel yields indices, not values |
| BUG-02 | HIGH ✓ | `go func() {}()` closures cannot read outer-scope variables |
| BUG-03 | HIGH ✓ | Type assertions `v.(T)` always succeed regardless of actual type |
| BUG-04 | HIGH ✓ | `recover()` in deferred function of a value-returning function causes caller error |
| BUG-05 | HIGH | Calling a function stored in an `any` variable fails |
| BUG-06 | HIGH ✓ | `++`/`--` not permitted on struct fields or array elements |
| BUG-07 | MEDIUM ✓ | Two-value channel receive `v, ok := <-ch` not supported |
| BUG-08 | MEDIUM ✓ | `delete(struct, key)` fails on dynamic structs despite spec |
| BUG-09 | MEDIUM ✓ | Import alias (`import alias "pkg"`) not recognized at use site |
| BUG-10 | MEDIUM | `json.Unmarshal(b)` single-argument form rejected |
| BUG-11 | MEDIUM | `fmt.Printf()` two-value return `n, err := fmt.Printf(...)` fails |
| BUG-12 | MEDIUM | Writing to a nil map succeeds; should error |
| BUG-13 | MEDIUM | `typeof()` result incompatible with `switch` case matching |
| BUG-14 | MEDIUM ✓ | Typed array element type not enforced in dynamic mode |
| BUG-15 | MEDIUM | `append()` to typed array silently accepts wrong-type elements |
| BUG-16 | MEDIUM | `defer namedFunc(arg)` evaluates args lazily (cross-ref: FLOW-M4) |
| BUG-17 | LOW | `var (...)` group declaration not supported |
| BUG-18 | LOW | `type()` documented but actual builtin is `typeof()` |
| BUG-19 | LOW | `for v := range string` yields single-char strings, not int32 runes |
| BUG-20 | LOW | `iota` not supported in `const` blocks |
| BUG-21 | LOW ✓ | `@compile` test directive cannot pass computed values back to the enclosing test |

---

## Bug Details

---

### BUG-01 — `for v := range ch` over channel yields indices, not values

**Severity:** HIGH

**Description:**  
When ranging over a channel with a single loop variable, the variable receives
successive integer indices (0, 1, 2, …) instead of the values stored in the channel.
The channel values are discarded. This makes the standard Go idiom of consuming a
channel with `for v := range ch` produce completely wrong results.

**Reproducer:**

```go
import "fmt"

func producer(ch chan) {
    ch <- 100
    ch <- 200
    ch <- 300
    close(ch)
}

func main() {
    ch := make(chan, 5)
    go producer(ch)

    for v := range ch {
        fmt.Println("received:", v)
    }
}
```

**Actual output:**

```text
received: 0
received: 1
received: 2
```

**Expected output:**

```text
received: 100
received: 200
received: 300
```

**Notes:**  
Direct receive (`v := <-ch`) returns the correct values. Only the `for range`
form is broken. The bug appears to be that the range loop treats the channel
like an array and emits the iteration count rather than the received value.

**Resolution (June 2026):**  
`bytecode/range.go` — `rangeNextChannel`: the function now distinguishes between
the single-variable and two-variable loop forms by checking whether `r.valueName`
is present:

- **Two-variable form** (`for i, v := range ch`): `r.indexName` receives the loop
  counter (`r.index`) and `r.valueName` receives the channel value (`datum`) —
  behavior unchanged.
- **Single-variable form** (`for v := range ch`): the compiler emits
  `RangeInit ["v", ""]`, placing the sole variable name in `r.indexName` and
  leaving `r.valueName` empty. The function now routes `datum` to `r.indexName`
  in this case. The loop counter is meaningless for channel receives and is
  no longer exposed.

Three new unit tests were added to `bytecode/range_test.go` (Section 8):

- `Test_rangeNextChannel_SingleVarReceivesValue` — the direct regression test;
  verifies that the single-variable form delivers channel values, not indices.
- `Test_rangeNextChannel_SingleVarDiscarded` — verifies the discard form
  (`for _ := range ch`) consumes values without writing to any symbol.
- `Test_rangeNextChannel_TwoVarCounterAndValue` — verifies the two-variable form
  still delivers the counter to `i` and the value to `v` after the fix.

Two new Ego language tests were added to `tests/flow/rangechannels.ego`:

- `"flow: single-variable channel range receives values, not indices"` — the
  end-to-end regression test.
- `"flow: two-variable channel range counter and value are correct"` — ensures
  the two-variable form continues to work.

---

### BUG-02 — `go func() {}()` closures cannot read outer-scope variables

**Severity:** HIGH

**Description:**  
An anonymous function literal launched as a goroutine (`go func() { ... }()`)
cannot access variables from the enclosing scope. Any reference to an outer
variable produces a runtime error `"unknown identifier: <name>"`. This breaks
the idiomatic Go pattern of launching goroutines as closures.

The bug occurs when the goroutine runs asynchronously in a separate OS thread.
When the goroutine happens to run on the same thread (because the main goroutine
blocks on an unbuffered channel receive), it can sometimes access the parent
scope — but this is unreliable and race-dependent.

**Reproducer:**

```go
import "fmt"

func main() {
    x := 42
    go func() {
        fmt.Println("x:", x)   // should print 42
    }()
    for i := 0; i < 1000000; i++ {}   // spin to let goroutine run
    fmt.Println("done")
}
```

**Actual output:**

```text
Error: at go func ()(line 6), unknown identifier: x
```

**Expected output:**

```text
x: 42
done
```

**Workaround:**  
Pass all captured variables as explicit arguments to the anonymous function:

```go
go func(val int) {
    fmt.Println("x:", val)
}(x)
```

**Notes:**  
This also affects `sync.WaitGroup` usage where the goroutine captures `wg`
from the outer scope — `wg.Wait()` hangs because the goroutines cannot call
`wg.Done()` through the captured reference.

**Resolution (June 2026):**

**Root cause:** `pushByteCode` in `bytecode/stack.go` unconditionally overwrote
`capturedScope` with `c.symbols` every time a literal `*ByteCode` was pushed.
The sequence for `go func() { ... }()` is:

1. The parent context executes `Push <literal>` — clones the raw compiled
   bytecode and stamps `c.symbols` (the parent's local scope, containing `x`)
   as `capturedScope`. This is correct.
2. `goByteCode` pops the clone as `fx`.  `fx.capturedScope` now points at the
   parent's local scope.
3. `GoRoutine` constructs a tiny call script `Push fx; Call N` that runs in a
   minimal goroutine context whose `c.symbols` is a child of the global root
   (knows nothing about `x`).
4. When `Push fx` executes in the goroutine, `pushByteCode` cloned `fx` (the
   clone inherits `capturedScope` from `fx`) then **overwrote**
   `capturedScope = c.symbols` — the goroutine's root-child scope.
5. The closure's `callBytecodeFunction` then created a function scope as a
   child of the goroutine's root-child scope instead of the parent's local
   scope, so `x` was never reachable.

The user hint was confirmed: anonymous functions are (and must be) compiled with
`PushScope` (no boundary flag) while named functions use
`PushScope BoundaryScope`. This was already correct in the compiler
(`compiler/function.go` lines 146–149). The bug was purely in `pushByteCode`.

**Fixes applied:**

`bytecode/stack.go` — `pushByteCode`: added a guard `if clone.capturedScope == nil` so
the current scope is only stamped onto a clone when none has been captured yet.
An already-captured scope (the goroutine re-push case) is left intact, preserving
the parent's local variables in the chain. The loop-closure case (FUNC-H2) is
unaffected: the original compiled literal always starts with `capturedScope == nil`,
so each loop iteration still captures its per-iteration scope.

`bytecode/goroutine.go` — `GoRoutine`: when `fx` is a literal closure with a non-nil
`capturedScope`, calls `capturedScope.Shared(true)` before the goroutine runs.
`Shared(true)` propagates up the entire ancestor chain, so every symbol table
the closure will traverse is protected by read/write locks. This prevents data
races when the parent thread and the goroutine access the same tables concurrently.

**Tests added:**

- `bytecode/stack_test.go` — two new Go unit tests:
  - `Test_pushByteCode_PreservesCapturedScope` — direct regression test simulating
    the parent → goroutine double-push sequence; verifies the second push does
    not overwrite the scope captured by the first.
  - `Test_pushByteCode_LoopIterationsCaptureDifferentScopes` — verifies the
    loop-closure case continues to produce a distinct per-iteration scope.

- `tests/flow/go_func_literal.ego` — four new Ego language tests:
  - `"flow: goroutine closure reads outer-scope variable"` — direct BUG-02
    regression test; a goroutine closure must be able to read `x` from the
    outer scope.
  - `"flow: goroutine closure captures multiple outer-scope variables"` — same
    scenario with three captured variables.
  - `"flow: goroutine closure with explicit arguments still works"` — verifies
    the original argument-passing form is unaffected.
  - `"flow: named-function goroutine unaffected by BUG-02 fix"` — verifies named
    function goroutines are not broken by the change.

---

### BUG-03 — Type assertions `v.(T)` always succeed regardless of actual type

**Severity:** HIGH

**Description:**  
A type assertion `v.(T)` on an `any`-typed variable always succeeds and returns
the underlying value regardless of whether the value's actual type matches `T`.
The type name `T` is ignored. The two-value form `val, ok := v.(T)` also always
returns `ok = true`.

In Go, `v.(string)` on a value containing `42` (int) panics. In Ego, it silently
returns `42` and reports `typeof` as `"string"`.

**Reproducer:**

```go
import "fmt"

func main() {
    var v any = 42

    s := v.(string)             // should panic or error
    fmt.Println("s:", s)        // prints: 42  (WRONG)
    fmt.Println("type:", typeof(s)) // prints: string  (WRONG - the int was not converted)

    s2, ok := v.(string)        // should return ("", false)
    fmt.Println("ok:", ok, "s2:", s2)  // prints: true 42  (WRONG)
}
```

**Actual output:**

```text
s: 42
type: string
ok: true s2: 42
```

**Expected output:**

```text
Error: interface conversion: interface{} is int, not string
```

or in a two-value assertion: `ok: false s2:`

**Notes:**  
Correct type assertion (`v.(int)`) does work when the type matches. The bug is
that incorrect assertions don't fail — the assertion target type is ignored and
the stored value is returned as-is.

**Resolution (June 2026):**

**Design decision:** Type assertions validate the actual stored type in **all**
type-strictness modes (strict, relaxed, and dynamic). A type assertion is not a
coercion — it asks "does this interface value actually hold a T?" rather than
"can this value be converted to T?" Cast functions (`int()`, `string()`,
`float64()`, etc.) already provide coercion. Making assertions succeed by
coercion eliminates the only runtime mechanism for asking the first question;
the `v, ok := x.(T)` form only has value if `ok` can be `false`.

**Root cause:** `unwrapByteCode` in `bytecode/types.go` had two separate code
paths based on `c.typeStrictness`:

- **Strict mode**: checked `actualType.IsType(newType)`; returned `(nil, false)`
  on mismatch. Correct.
- **Non-strict modes**: called `data.Coerce(value, newType.InstanceOf(...))`,
  unconditionally converting any value to any type and reporting success,
  ignoring the actual stored type entirely. Broken.

**Secondary bug:** the `"any"` alias for `interface{}` was not recognized. The
type lookup loop compared `td.Kind.Name()` (which returns `"interface{}"`)
against the string `"any"`, so `v.(any)` always returned `ErrInvalidType` instead
of succeeding. Fixed by normalizing `"any"` to `data.InterfaceTypeName`
(`"interface{}"`) before the loop.

**Fixes applied:**

`bytecode/types.go` — `unwrapByteCode`: the two-path `if/else` was replaced
with a unified check:

- If `newType.Kind() == data.InterfaceKind` (target is `any`): always succeeds,
  since every value satisfies the empty interface.
- If `!actualType.IsType(newType)`: push `(nil, false)` and return. Applies in
  all three type-strictness modes.
- If types match: push `(value, true)`.

`bytecode/types.go` — `unwrapByteCode` (type lookup): added normalization of
the string `"any"` to `data.InterfaceTypeName` before the `TypeDeclarations`
loop.

**Tests added:**

- `bytecode/types_test.go` — nine new Go unit tests (section 2b) covering the
  BUG-03 regression in all three modes, plus the `"any"` alias fix:
  - `Test_unwrapByteCode_BUG03_Relaxed_WrongType`
  - `Test_unwrapByteCode_BUG03_Dynamic_WrongType`
  - `Test_unwrapByteCode_BUG03_Relaxed_CorrectType`
  - `Test_unwrapByteCode_BUG03_Dynamic_CorrectType`
  - `Test_unwrapByteCode_BUG03_IntToString`
  - `Test_unwrapByteCode_BUG03_FloatToInt`
  - `Test_unwrapByteCode_BUG03_AnyAlias`
  - `Test_unwrapByteCode_BUG03_InterfaceAlias`
  - `Test_unwrapByteCode_BUG03_AllModesConsistent`

- `tests/types/type_assertions.ego` — twelve new Ego language tests covering
  correct-type and wrong-type assertions in both the single-value and two-value
  forms, all scalar types, the `any` alias, and the multi-assert type-guard
  pattern.

---

### BUG-04 — `recover()` in deferred function of a value-returning function causes caller error

**Severity:** HIGH

**Description:**  
When a function that has a return value panics and a deferred function calls
`recover()`, the panic is correctly caught. However, the caller then receives
the error `"function did not return the expected number of values"` instead of
getting the function's zero/named return value.

`recover()` works correctly for `void` (no return value) functions. The bug is
specific to functions that declare a return type.

**Reproducer:**

```go
@extensions true
import "fmt"

func panicIfZero(b int) int {
    defer func() {
        if r := recover(); r != nil {
            fmt.Println("recovered:", r)
        }
    }()
    if b == 0 {
        panic("zero!")
    }
    return b * 2
}

func main() {
    r1 := panicIfZero(5)
    fmt.Println("panicIfZero(5):", r1)   // OK: prints 10

    r2 := panicIfZero(0)
    fmt.Println("panicIfZero(0):", r2)   // ERROR
}
```

**Actual output:**

```text
panicIfZero(5): 10
recovered: zero!
Error: at main(line 19), function did not return the expected number of values
```

**Expected output:**

```text
panicIfZero(5): 10
recovered: zero!
panicIfZero(0): 0
```

**Notes:**  
The `recover()` call itself works (prints "recovered: zero!"). The issue is that
after recovery the function does not push a return value onto the result stack,
so the caller's multi-return unpack fails. Named return variables should be
returned at their current (zero) value after recovery.

**Resolution (June 2026):**

**Root cause:** `unwindPanic` (in `bytecode/panic.go`) popped the call frame
without pushing any return value onto the caller's stack. The caller's next
instruction (`CreateAndStore` for single-value, `StackCheck` for multi-value)
then consumed the stack marker that had been placed before the function call,
producing "function did not return the expected number of values".

**Design decisions:**

- **Unnamed returns** → push `nil` (unambiguously signals "no useful value was
  set before the panic"). This is consistent with the user's Q3 preference over
  zero-value synthesis.
- **Named returns** → push the variable's *current* symbol-table value, which
  the compiler initialized to zero at function entry; the deferred function that
  called `recover()` may have explicitly set it before returning. This matches
  Go's semantics: named return variables retain whatever value they held at the
  point of recovery.
- **Void functions** → unchanged (no values pushed).
- **All three type-strictness modes** → identical behavior.

**Fixes applied:**

`bytecode/bytecode.go`: Added `returnVarNames []string` field to `ByteCode`
with `SetReturnVarNames`/`GetReturnVarNames` accessors. This records the
source-code names of any named return variables so `unwindPanic` can look them
up in the panicking function's symbol table at recovery time without changing
`data.Declaration`.

`compiler/function.go` — `generateFunctionBytecode`: After `c.returnVariables`
is fully parsed, copies the names into a `[]string` and calls
`b.SetReturnVarNames(names)`.

`bytecode/panic.go` — `unwindPanic`, recovery block: after clearing the defer
stack and resetting `c.stackPointer = c.framePointer`, synthesizes return
values before calling `callFramePop`:

- Single return: sets `c.result` and `c.resultSet = true` — `callFramePop`
  pushes the result onto the caller's stack via the existing single-result path.
- Multiple returns: pushes `NewStackMarker(c.bc.name, N)` followed by N values
  in reverse declaration order — exactly matching what `compileReturn` emits in
  the normal (non-panic) path. The `StackMarker` is required because
  `stackCheckByteCode` scans the stack looking for any marker to confirm the
  expected number of return values are present.

**Tests added:**

`tests/flow/panic_recover.ego` — 12 new Ego language tests in five groups:

1. **Unnamed single return** — BUG-04 primary regression; caller gets `nil`.
2. **Named single return** — zero value when defer doesn't modify; set value
   when defer does; pre-panic value preserved when defer doesn't touch it.
3. **Nested scope** — named return reachable when panic occurs inside an `if`
   block; reachable when panic occurs inside a `for` loop.
4. **Multiple returns** — unnamed gives `(nil, nil)`; named deferred-set gives
   correct values in correct order; partial deferred set; pre-panic values
   preserved.
5. **Void functions** — unchanged behavior confirmed.

---

### BUG-05 — Calling a function stored in an `any` variable fails

**Severity:** HIGH

**Description:**  
The LANGUAGE.md documents passing a function as an `any` parameter and calling
it inside the receiving function. In practice this fails with
`"invalid function invocation"` — calling a variable typed as `any` that holds
a function value is not supported.

**Reproducer:**

```go
import "fmt"

func show(fn any, name string) {
    fn("The name is ", name)   // documented in LANGUAGE.md
}

func main() {
    p := fmt.Println
    show(p, "tom")   // Error: invalid function invocation
}
```

**Actual output:**

```text
Error: at show(line 4), invalid function invocation: {{false Println(...) ...}}
```

**Expected output:**

```text
The name is  tom
```

**Workaround:**  
There is no clean workaround within the `any` parameter type. Pass the function
as its concrete type if known, or restructure to avoid passing functions as `any`.

**Notes:**  
Calling a function stored in a concrete-typed variable (not `any`) works
correctly. The bug is specific to function values wrapped in the `any`/interface
type. This makes the documented pattern in LANGUAGE.md non-functional.

**Resolution:**

Added code to the Call bytecode handler to detect when the item being used as the
target of the call is wrapped as a data.Interface (the Ego version of an `any` value),
the value is unwrapped before proceeding to determine who the value is meant to be
used (i.e. the target can be bytecode, a built-in function, a type, etc.)

---

### BUG-06 — `++`/`--` not permitted on struct fields or array elements

**Severity:** HIGH

**Description:**  
The `++` and `--` operators only work on simple (unqualified) variable names.
Applying them to struct field access (`s.field++`) or array element access
(`a[i]++`) produces a compile-time error: `"invalid use of auto increment/decrement
operation"`. In Go, both forms are valid.

**Reproducer:**

```go
import "fmt"

type Foo struct { x int }

func main() {
    a := []int{1, 2, 3}
    a[0]++        // compile error

    f := Foo{x: 10}
    f.x++         // compile error
}
```

**Actual error:**

```text
Error: at line 7:4, invalid use of auto increment/decrement operation
```

**Expected behavior:**  
`a[0]++` increments `a[0]` from 1 to 2.  
`f.x++` increments `f.x` from 10 to 11.

**Workaround:**  
Use explicit assignment: `a[0] = a[0] + 1` and `f.x = f.x + 1`.

**Resolution (June 2026):**

**Root cause:** `compileAssignment` in `compiler/assignment.go` checked whether
the lvalue store bytecode had more than two instructions and, if so, immediately
returned `ErrInvalidAuto`. A simple variable like `x` produces exactly two
instructions (`Store "x"` + `DropToMarker`), but any qualified lvalue — an array
subscript `a[0]` or a struct field `s.field` — produces at least four instructions
(`Load base`, index expression, `StoreIndex`, `DropToMarker`), which always tripped
the guard.

The same pattern existed in the for-loop increment compiler (`compiler/for.go`)
where the increment clause of a `for init; cond; incr` loop assumed instruction 0
of the lvalue was always `Store "name"`, which is only true for simple variables.

**Design:**

A qualified lvalue's store bytecode has this structure:

```text
[0]       Load "base"     — load the container (array or struct variable)
[1..n-3]  <index exprs>   — push each intermediate index
[n-2]     StoreIndex      — write back (patched from LoadIndex by patchStore)
[n-1]     DropToMarker    — clean up the "let" stack marker
```

For `a[0]++` the load half can be reconstructed by copying instructions 0 through
`n-3` from the store lvalue and appending `LoadIndex` (with the same operand as
`StoreIndex`). This reads the current value of `a[0]` onto the stack. After applying
`Push 1` + `Add`/`Sub`, the full `storeLValue` is appended to write the result back
and clean up the marker.

**Fixes applied:**

`compiler/assignment.go` — `compileAssignment`: replaced the blanket
`if storeLValue.Mark() > 2 { return ErrInvalidAuto }` guard with a two-branch
path:

- **Simple variable** (`storeLValue.Mark() == 2` with `Store` as instruction 0):
  existing `Load / Push 1 / Add|Sub / Dup / Store` sequence, behavior unchanged.
- **Qualified lvalue** (second-to-last instruction is `StoreIndex`): emits the
  load-path instructions from `storeLValue`, a `LoadIndex` to read the current
  value, `Push 1` + `Add`/`Sub`, then appends the full `storeLValue` (which runs
  `StoreIndex` + `DropToMarker`) to write back and clean up.  Any other lvalue
  shape (e.g. pointer dereference) falls through to `ErrInvalidAuto`.

`compiler/for.go` — for-loop increment clause: the same structural change was
applied so that `for a[0] := 0; a[0] < n; a[0]++` works in the increment position.

**Tests added:**

`compiler/assignment_test.go` — four new compile-time cases in
`TestCompiler_compileAssignment` (one per form: array `++`, array `--`, struct
`++`, struct `--`), verifying that compilation now succeeds where it previously
returned `ErrInvalidAuto`.

`compiler/run_test.go` — seven new runtime cases in `TestArbitraryCodeFragments`
that build arrays and dynamic structs, apply `++`/`--`, and verify the resulting
values.

`tests/flow/qualified_increment.ego` — ten Ego language tests covering:

- `"flow: array element increment (BUG-06)"` — basic `a[0]++`, checks value and
  sibling elements.
- `"flow: array element decrement (BUG-06)"` — `a[2]--`.
- `"flow: array element increment, variable index (BUG-06)"` — `a[i]++` where
  `i` is a variable.
- `"flow: array element decrement, variable index (BUG-06)"` — `a[idx]--`.
- `"flow: multiple array element increments (BUG-06)"` — three consecutive `a[0]++`
  must be cumulative.
- `"flow: struct field increment (BUG-06)"` — `s.x++` on a dynamic struct.
- `"flow: struct field decrement (BUG-06)"` — `s.count--`.
- `"flow: multiple struct field increments (BUG-06)"` — five consecutive `s.n++`.
- `"flow: struct other fields unaffected by increment (BUG-06)"` — increments one
  field and confirms the other is unchanged.
- `"flow: for loop with array element increment (BUG-06)"` — uses `a[0]++` as the
  increment clause of a `for` loop.

---

### BUG-07 — Two-value channel receive `v, ok := <-ch` not supported

**Severity:** MEDIUM

**Description:**  
Go's standard idiom for detecting a closed channel is the two-value receive
form: `v, ok := <-ch`. In Ego, this produces `"incorrect number of return values"`.
Only the single-value form `v := <-ch` is supported.

**Reproducer:**

```go
import "fmt"

func main() {
    ch := make(chan, 2)
    ch <- 10
    close(ch)

    v, ok := <-ch   // ERROR: incorrect number of return values
    fmt.Println(v, ok)
}
```

**Actual output:**

```text
Error: at main(line 8), incorrect number of return values
```

**Expected output:**

```text
10 true
```

**Notes:**  
A second receive from the closed-and-drained channel should give `(<nil>, false)`.
Without this form, there is no reliable way in Ego to detect when a channel has
been closed.

**Resolution (June 2026):**

**Root cause:** A multi-target assignment such as `v, ok := <-ch` compiles its
left-hand side via `assignmentTargetList`, which always begins the generated
store bytecode with `StackCheck 2` — a check that exactly two values are present
above a stack marker before unpacking them to `v` and `ok`. For an ordinary
multi-return call (`a, b := f()`) the callee pushes both values itself. For a
channel receive, however, the only thing on the stack before this point was the
single channel object pushed by evaluating `<-ch`'s operand — one value, not two
— so `StackCheck 2` always failed with `"incorrect number of return values"`.
The single-value form `v := <-ch` was unaffected because it goes through a
different code path (`StoreChan`) that doesn't use `StackCheck`.

**Design:** A new `ReceiveChannel` bytecode instruction performs the actual
channel receive and leaves the stack in the shape `StackCheck 2` expects:

```text
[0]  StackMarker("receive")
[1]  ok bool      — true if a value was received, false if the channel
                     is closed and drained
[2]  datum any    — the received value, or nil when ok is false
```

`compileAssignment` detects the specific combination of a channel-receive RHS
(`<-` was consumed) together with a multi-target LHS (`c.flags.multipleTargets`)
and emits `ReceiveChannel` instead of leaving the plain `Load "ch"` result on the
stack. The existing `storeLValue` bytecode (already built by
`assignmentTargetList` for any multi-target assignment) is unchanged and unpacks
the two pushed values to `v` and `ok` exactly as it would for a function's
multi-value return.

**Fixes applied:**

`bytecode/opcodes.go` — added the `ReceiveChannel` opcode: a new constant, an
entry in `opcodeNames`, and a dispatch-table registration pointing at
`receiveChannelByteCode`.

`bytecode/store.go` — added `receiveChannelByteCode`: pops the channel value
(erroring with `ErrInvalidChannel` if the popped value isn't a `*data.Channel`),
calls `Receive()`, and pushes `[StackMarker("receive"), ok, datum]`. A closed,
drained channel (`Receive()` returning `ErrChannelNotOpen`) is translated to
`ok = false, datum = nil` rather than propagated as an error, matching Go's
"second value is the success flag" convention.

`compiler/assignment.go` — `compileAssignment`: captures whether the RHS began
with `<-` (`isChannelReceive`) before parsing the expression. When
`isChannelReceive && c.flags.multipleTargets` is true, emits `ReceiveChannel`
in place of the plain expression result and appends `storeLValue` as usual. The
single-value channel-receive path (`v := <-ch`) is untouched since
`multipleTargets` is false in that case.

**Tests added:**

`bytecode/store_test.go` — five new Go unit tests (Section 7):
`Test_receiveChannelByteCode_SuccessfulReceive`,
`Test_receiveChannelByteCode_ClosedChannel`,
`Test_receiveChannelByteCode_NonChannelValue`,
`Test_receiveChannelByteCode_StackMarker`, and
`Test_receiveChannelByteCode_StackLayout` — covering the success case, the
closed-channel case, error handling for non-channel and stack-marker inputs,
and the exact push order/count on the stack.

`compiler/assignment_test.go` — one new compile-time case verifying
`v, ok := <-ch` compiles without error.

`compiler/run_test.go` — `TestBUG07TwoValueChannelReceive`, a standalone Go
test (separate from `TestArbitraryCodeFragments`, which lacks builtins like
`make`) that injects a pre-built `*data.Channel` directly into the symbol table
and exercises three scenarios: a value is waiting (`ok == true`), the received
value itself is correct, and a closed channel yields `ok == false`. Because this
test compiles a fragment using a channel that was never declared with `make()`
in the Ego source, it pins `defs.UnknownVarSetting` to `false` for its duration
so its outcome doesn't depend on a developer's persisted `~/.ego/` profile
settings or on what other tests in the package happen to run first.

`tests/flow/two_value_receive.ego` — five Ego language tests: receiving a
buffered value (`ok == true`, correct value), draining multiple buffered values
in order, detecting a closed channel (`ok == false`, `v == nil`), draining all
buffered items before reporting closure, and receiving correctly when the
sender is a separate goroutine.

---

### BUG-08 — `delete(struct, key)` fails on dynamic structs

**Severity:** MEDIUM

**Description:**  
LANGUAGE.md documents `delete()` as: `"Remove the named field from a map, or a
delete a dynamic struct member"`. In practice, calling `delete(s, "field")` on
a dynamic (empty-literal) struct fails with `"invalid or unsupported data type
for this operation: argument 1: struct"`.

**Reproducer:**

```go
import "fmt"

func main() {
    s := {}
    s.x = 1
    s.y = 2
    delete(s, "x")    // should remove field x
    fmt.Println(s)    // expected: {y: 2}
}
```

**Actual output:**

```text
Error: at delete(line 7), invalid or unsupported data type for this operation: argument 1: struct
```

**Expected output:**

```text
{y: 2}
```

**Notes:**  
`delete()` on a `map` type works correctly. Only the struct variant is broken.

**Resolution:**

The `delete()` function now works with dynamic structs (Ego structs created using
an empty struct constant). If the struct is not a dyanmic struct (which is an Ego
extension), most structs are static like Go) then a read-only error is generated.
If the field name does not exist, an error is generated.

Along the way, noted that field order was not tracked for dynamic structs. That is,
the order in which the fields are declared should be used as the order to print
the fields when printing a formatted version of the struct. This was added, along
with code to remove a field name from the field order list when the field was
deleted.

Ego unit tests added.

---

### BUG-09 — Import alias (`import alias "pkg"`) not recognized at use site

**Severity:** MEDIUM

**Description:**  
LANGUAGE.md documents an import alias syntax: `import str "strings"`, after which
the package should be accessible as `str.ToUpper(...)`. In practice, using the
alias name produces `"unknown symbol: str"` — the alias is ignored and the
package is not accessible under that name or its original name.

**Reproducer:**

```go
import str "strings"
import "fmt"

func main() {
    result := str.ToUpper("hello")   // unknown symbol: str
    fmt.Println(result)
}
```

**Actual output:**

```text
Error: at line 5:8, unknown symbol: str
```

**Expected output:**

```text
HELLO
```

**Workaround:**  
Import the package under its canonical name and use it without aliasing.

---

### BUG-10 — `json.Unmarshal(b)` single-argument form rejected

**Severity:** MEDIUM

**Description:**  
LANGUAGE.md documents that `json.Unmarshal` can be called with just one argument
(the JSON byte array), in which case the decoded value is returned directly:
`r := json.Unmarshal(s)`. In practice this fails with
`"incorrect function argument count: 1"`.

**Reproducer:**

```go
import "fmt"
import "json"

func main() {
    b := json.Marshal({name: "Tom", age: 44})
    r := json.Unmarshal(b)     // single-arg form documented to return decoded value
    fmt.Println(r)
}
```

**Actual output:**

```text
Error: incorrect function argument count: 1
```

**Expected output:**

```text
{age: 44, name: "Tom"}
```

**Workaround:**  
Always use the two-argument form: `err := json.Unmarshal(b, &target)`.

**Resolution:**

The single-argument version was a legacy from a much older version of Ego
that didn't yet properly support pointers, so the Go-compliant version
was not possible. There's no reason to retain this legacy variable, so
the fix is to delete it from the documentation and rquire that the Unmarshal()
function be called properly.

---

### BUG-11 — `fmt.Printf()` two-value return fails

**Severity:** MEDIUM

**Description:**  
LANGUAGE.md states that `fmt.Printf` returns `(length int, error)`. When callers
use the two-value assignment form `n, err := fmt.Printf(...)`, Ego reports
`"incorrect number of return values"`. The single-value form `n := fmt.Printf(...)`
works and returns the character count.

**Reproducer:**

```go
import "fmt"

func main() {
    n, err := fmt.Printf("hello %d\n", 42)
    fmt.Println("n:", n, "err:", err)
}
```

**Actual output:**

```text
hello 42
Error: at main(line 4), incorrect number of return values
```

**Expected output:**

```text
hello 42
n: 8 err: <nil>
```

**Notes:**  
`fmt.Sscanf` correctly supports the two-value return form. The bug is specific to
`fmt.Printf` and `fmt.Println` (which also returns `(int, error)` in Go but is not
documented to do so in Ego).

---

### BUG-12 — Writing to a nil map succeeds; should error

**Severity:** MEDIUM

**Description:**  
In Go, assigning to an entry in a nil map panics: `"assignment to entry in nil map"`.
In Ego, a nil map (declared with `var m map[string]int`) silently accepts writes
and retains the written values. This silently corrupts program state instead of
alerting the programmer.

**Reproducer:**

```go
import "fmt"

func main() {
    var m map[string]int   // nil map

    m["key"] = 42          // should error; Go would panic here
    fmt.Println(m["key"])  // prints 42 (unexpected: write to nil map succeeded)
}
```

**Actual output:**

```text
42
```

**Expected output:**

```text
Error: assignment to entry in nil map
```

(or a catchable Ego runtime error equivalent)

**Notes:**  
Reading from a nil map (`v := m["key"]`) correctly returns `nil`. Only writes
are mishandled.

---

### BUG-13 — `typeof()` result incompatible with `switch` case matching

**Severity:** MEDIUM

**Description:**  
`typeof(v)` returns a type object (not a string). While `typeof(v) == "int"` works
because of coercion in equality comparison, using `typeof(v)` as the switch
expression with string case values fails with `"invalid integer value: int"`.

**Reproducer:**

```go
import "fmt"

func main() {
    n := 42
    t := typeof(n)          // returns type object, displays as "int"
    fmt.Println(t == "int") // true — coercion works in ==

    switch t {
    case "int":             // ERROR: invalid integer value: int
        fmt.Println("it's int")
    }
}
```

**Actual output:**

```text
true
Error: invalid integer value: int
```

**Expected output:**

```text
true
it's int
```

**Workaround:**  
Convert to string before switching: `switch string(typeof(n)) { case "int": ... }`

**Notes:**  
The inconsistency — `==` coerces `typeof()` result to string, but `switch` does
not — is what makes this a bug. Either the `==` coercion should be removed (making
`typeof(n) == "int"` also fail), or `switch` should apply the same coercion.

---

### BUG-14 — Typed array element type not enforced in dynamic mode

**Severity:** MEDIUM

**Description:**  
A typed array (`[]int{1, 2, 3}`) should reject assignments of incompatible types
to its elements. LANGUAGE.md explicitly states this: `"a[1] = 1325.0 // Failed,
must be of type int"`. In practice, `[]int` elements accept strings, floats, and
other types silently in dynamic mode.

**Reproducer:**

```go
import "fmt"

func main() {
    a := []int{1, 2, 3}
    a[0] = "hello"           // should error: wrong type for []int
    fmt.Println("a:", a)     // prints: ["hello", 2, 3]  (array corrupted)
}
```

**Actual output:**

```text
a: ["hello", 2, 3]
```

**Expected output:**

```text
Error: wrong type for element of []int: string
```

(or a coercion to int with an error for non-numeric strings)

**Notes:**  
The LANGUAGE.md says `a[1] = 1325.0` on a `[]int` should fail, but in practice
`1325.0` is silently truncated to `1325`. Even `a[0] = "string"` silently
converts the array to a mixed-type array. Element type enforcement is entirely
absent.

***Resolution:***
Its a little more complicated than that. The Ego-correct behavior depends on the
current mode checking:

- Strict requires that the type match
- Relaxed will try to coerce the type to fit if possible
- Dynamic will just convert the array type to []any

Changes where made in StoreIndex to evaluate if type coercion of the value
or the array are possible. A function `MakeAny()` was added to *data.Array
elements that converts the type from a specific array type to []any.

---

### BUG-15 — `append()` to typed array silently accepts wrong-type elements

**Severity:** MEDIUM

**Description:**  
`append()` does not enforce the element type of a typed array. Appending a
`string` to a `[]int` silently succeeds and corrupts the array's type contract.

**Reproducer:**

```go
import "fmt"

func main() {
    a := []int{1, 2, 3}
    a = append(a, "hello")    // should error: wrong type for []int
    fmt.Println("a:", a)      // prints: [1, 2, 3, "hello"]
}
```

**Actual output:**

```text
a: [1, 2, 3, "hello"]
```

**Expected output:**

```text
Error: wrong type for element of []int: string
```

---

### BUG-16 — `defer namedFunc(arg)` evaluates arguments lazily (cross-ref: FLOW-M4)

**Severity:** MEDIUM

**Description:**  
Already documented as `FLOW-M4` in `FUNCTIONAL_ISSUES.md`. Included here for
completeness. In Go, the arguments of a deferred function call are evaluated
immediately when the `defer` statement executes. In Ego, they are evaluated
lazily when the deferred function runs (at function return time), so the final
value of the variable is seen instead of the value at defer time.

**Reproducer:**

```go
import "fmt"

func main() {
    x := 1
    defer fmt.Println("defer x (should be 1):", x)
    x = 2
    defer fmt.Println("defer x (should be 2):", x)
    x = 3
    fmt.Println("main, x =", x)
}
```

**Actual output (LIFO, all see final x):**

```text
main, x = 3
defer x (should be 2): 3
defer x (should be 1): 3
```

**Expected output:**

```text
main, x = 3
defer x (should be 2): 2
defer x (should be 1): 1
```

**Workaround:**  
Use a closure that receives the value as an argument:

```go
defer func(v int) { fmt.Println("defer x:", v) }(x)
```

---

### BUG-17 — `var (...)` group declaration not supported

**Severity:** LOW

**Description:**  
The `const (...)` group form works, but the equivalent `var (...)` group form
produces `"invalid type specification"`. This is a common Go pattern for
declaring multiple variables at the top of a function.

**Reproducer:**

```go
import "fmt"

func main() {
    var (
        p = 3.14
        q = "pi"
    )
    fmt.Println(p, q)
}
```

**Actual output:**

```text
Error: at line 5:5, invalid type specification
```

**Expected output:**

```text
3.14 pi
```

---

### BUG-18 — `type()` documented but actual builtin is `typeof()`

**Severity:** LOW

**Description:**  
LANGUAGE.md (Functions section for `func` statement) states: `"If the function
body needs to know the actual type of the value passed, the type() function would
be used."` The actual builtin is named `typeof()`. Calling `type(v)` fails with
`"in type, invalid value: <v>"`.

**Test:**

```go
import "fmt"
func main() {
    n := 42
    t1 := type(n)     // Error: in type, invalid value: 42
    t2 := typeof(n)   // OK: returns "int"
    fmt.Println(t1, t2)
}
```

**Notes:**  
This is a documentation bug, not an implementation bug. `typeof()` works correctly.
The LANGUAGE.md should be updated to use `typeof()`.

---

### BUG-19 — `for v := range string` yields single-char strings, not int32 runes

**Severity:** LOW

**Description:**  
In Go, ranging over a string yields `(byte_index int, rune int32)` — the rune
value is an integer representing the Unicode code point. In Ego, the second
variable receives a single-character `string`, not an `int32`. This behavioral
difference can cause porting issues for Go developers and silently produces
different types from what Go documentation describes.

**Test:**

```go
import "fmt"
func main() {
    for i, c := range "ABC" {
        fmt.Printf("i=%d, c=%v, type=%T\n", i, c, c)
    }
}
```

**Actual output:**

```text
i=0, c=A, type=string
i=1, c=B, type=string
i=2, c=C, type=string
```

**Expected Go output:**

```text
i=0, c=65, type=int32
i=1, c=66, type=int32
i=2, c=67, type=int32
```

**Notes:**  
The behavior may be intentional in Ego (strings as character sequences rather
than byte arrays). However, it should be documented explicitly as an Ego vs. Go
difference, similar to how other differences are called out in LANGUAGE.md.

---

### BUG-20 — `iota` not supported in `const` blocks

**Severity:** LOW

**Description:**  
Go's `iota` identifier provides automatically-incrementing constants in a `const`
block. Ego does not support `iota` — any reference to it in a `const` block
produces `"invalid constant expression file"`. This is not documented as an
unsupported feature.

**Reproducer:**

```go
const (
    Apple  = iota
    Banana
    Cherry
)
```

**Actual output:**

```text
Error: at line 2:9, invalid constant expression file
```

**Notes:**  
Since `iota` is not mentioned in LANGUAGE.md, it may be intentionally unsupported.
The error message is confusing and unhelpful — it should say something like
`"iota is not supported in Ego const blocks"`.

---

### BUG-21 — `@compile` test directive cannot pass computed values back to the enclosing test

**Severity:** LOW

**Description:**  
The `@compile { ... } catch(e) { ... }` directive (see
`internal/language/compiler/directives.go:compileBlockDirective` and
`tests/directives/compile.ego`) lets a test compile a snippet of code at
runtime and inspect any compile error in `catch(e)` instead of aborting the
whole test. It has two forms, and both have a scoping gap:

1. **Full-program form** (`@compile { ... }`, no `block` keyword): the content
   must be a complete, standalone program with its own prolog/`main()`. By
   design, only symbols declared directly at that sub-program's *top level*
   (`subCompiler.s.Names()`) are copied back into the enclosing test via
   `CreateAndStore`. Anything computed inside a function body in that
   sub-program (including `main()`) lives in that function's own call-time
   scope and is discarded once the synthetic program finishes running — there
   is no mechanism to read it back out from the enclosing `@test` block.

2. **Block form** (`@compile block { ... }`): this form is supposed to share
   the caller's scope chain, and plain assignment to a pre-declared outer
   variable does work for simple cases (e.g. the existing
   `tests/directives/compile.ego` example `x = 33`). However, if the block
   *also* contains an `import` statement, a subsequent assignment to a
   pre-declared outer variable silently fails to propagate — no error is
   raised, the outer variable is just left unchanged. This looks like a
   narrower, distinct defect from (1), possibly related to how compiling an
   `import` statement inside block mode interacts with the parent scope
   chain, but it has not been root-caused.

**Reproducer (full-program form, item 1):**

```go
@test "demo: @compile full-program form cannot pass data out"
{
    @compile {
        func foo() {
            x := 33   // computed here, but unreachable from outside
        }
    } catch(e) {
        @fail "unexpected compile error"
    }
    // No way to observe x's value (33) from here.
}
```

**Reproducer (block form + import, item 2):**

```go
@test "demo: @compile block + import drops an outer assignment"
{
    var result string

    @compile block {
        import alias "strings"
        result = alias.ToUpper("hi")
    } catch(e) {
        @fail "unexpected compile error"
    }

    @assert result == "HI"   // FAILS: result is still ""
}
```

**Actual output:**  
No compile error in either case; the code runs successfully, but the computed
value never reaches the enclosing test. In the block-form reproducer,
`result` is `""` instead of `"HI"`.

**Expected output:**  
Some supported way to read a value computed inside an `@compile`-compiled
block or program back into the enclosing test, or (at minimum) clear
documentation that `@compile` is pass/fail-only and values must be checked
with assertions *inside* the compiled block itself.

**Notes:**  
Workaround: write the assertions inside the `@compile`/`@compile block` body
itself (or just check the `failed`/`catch` flag) rather than relying on an
outer variable being updated. This is a test-infrastructure limitation, not a
defect in user-facing Ego programs — flagged during work on BUG-09, filed for
later investigation.

---

## Testing Methodology

All bugs were found by writing small Ego programs to `/tmp/test_*.ego` and running
them with:

```sh
./ego run /tmp/test_xxx.ego
./ego --timeout 5s run /tmp/test_xxx.ego   # for tests involving goroutines
```

Tests covered: arithmetic, type conversions, string operations, maps, structs,
closures, goroutines, channels, defer/recover, error handling, built-in functions,
packages (math, sort, strings, strconv, errors, json, fmt), operators, and scoping.
