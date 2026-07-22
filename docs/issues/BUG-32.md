# BUG-32 — Nesting a multi-return native/runtime function call as a single call argument corrupts the interpreter stack

**Severity:** HIGH

**Description:**  
Any native or runtime-package function that returns multiple values via the `data.List`
convention (e.g. `(value, error)` pairs) corrupts the interpreter stack when its call is
nested directly as a single argument to another function call, rather than first being
assigned to variables. This affects `json.Marshal`, `strings.Template`, `time.Parse`, and —
by the same mechanism — very likely every other native/runtime function using this
convention. User-defined Ego functions with multiple return values do **not** have this
problem when nested the same way.

**Reproducer 1** (`docs/LANGUAGE.md`'s own documented example, ~line 2955-2960):

```go
import "json"

func main() {
    a := map[string]interface{}{"name": "Tom", "age": 44}
    s := string(json.Marshal(a))
    fmt.Println(s)
}
```

**Actual output:**

```text
Error: at main(line 4), invalid function invocation: <nil>
```

**Expected output:**

```text
{"age":44,"name":"Tom"}
```

**Reproducer 2** (`@template` example, also drawn near-verbatim from `docs/LANGUAGE.md`):

```go
@template hello "Greetings, {{.Name}}"

func main() {
    fmt.Println(strings.Template(hello, { Name: "Tom"}))
}
```

**Actual output:** `Error: at main(line 4), invalid function invocation: <nil>`

**Reproducer 3:**

```go
func main() {
    fmt.Println(time.Parse("1/2/2006 15:04", "12/7/1960 15:30"))
}
```

**Actual output:** `Error: at main(line 2), invalid function invocation: <nil>`

**Notes:**  
All three work correctly once the return values are captured first, e.g.
`b, err := json.Marshal(a); s := string(b)`. Via `--log bytecode` disassembly: the outer
call (e.g. `Println`) compiles with only as many argument slots reserved as there are
syntactic arguments (`Call 1`), but the inner multi-return call pushes a
`StackMarker("results")` plus both list items (value, then error) per the documented
multi-return convention (see CLAUDE.md's "`callRuntimeFunction` dispatch mechanics"
section). The outer call only pops one value as its argument and then pops the *next* stack
item as the function pointer to invoke — which is the inner call's leftover error value —
producing "invalid function invocation: `<nil>`". By contrast, `fmt.Println(userFunc())` for
a user-defined `func userFunc() (int, error)` correctly spreads both values into `Println`,
confirming that only the native/runtime `data.List` multi-return path is affected.

**Resolution (July 2026):**  
Root cause confirmed: `internal/language/bytecode/callRuntimeFunction.go` and
`internal/language/bytecode/callNative.go` pushed a `StackMarker("results")` with **no
item count** below every native/runtime multi-return result, unconditionally, regardless of
how the result was about to be used. `checkForTupleOnStack` (`call.go`), which lets a
user-defined function's own multi-return marker (which *does* carry a count — see
`NewStackMarker(name, len(returnVariables))` in `return.go`) get recognized and fully
consumed when nested as a call argument, requires exactly one `values` entry on the marker
to do that recognition. Without a count, a native marker nested this way was neither
consumed as a tuple nor cleanly discarded — it, and the trailing return value(s) above it,
were simply left on the stack, corrupting whatever ran next.

Simply adding a count to the marker (making `checkForTupleOnStack` treat native results the
same as user-defined ones) was tried first but rejected: it fixes the crash, but changes the
*meaning* of `string(json.Marshal(a))` from "cast the primary return value" into "cast the
full `(value, error)` tuple," which — because `builtins.Cast` treats more than one supplied
value as an array to be JSON-array-encoded — produces `["<bytes>", <nil>]`-style output
instead of the plain JSON string `docs/LANGUAGE.md` has always documented for this exact
example.

The actual fix instead teaches `pushMultiReturnResult` (new shared helper in `call.go`, used
by both `callRuntimeFunction.go` and `callNative.go`) to look at the *next* bytecode
instruction before deciding what to push:

- If it is `StackCheck` (an explicit multi-value assignment, `a, err := json.Marshal(x)`) or
  `DropToMarker` (a bare statement call, `json.Marshal(x)`, whose result is entirely
  discarded) — push the full `StackMarker("results", N)` plus every value, exactly as
  before. `DropToMarker`'s existing abandoned-error check (gated by
  `ego.runtime.unchecked.errors`, on by default) depends on seeing every discarded value;
  this is also what lets `close(ch)` on an already-closed channel be caught with a bare
  `try { close(ch) } catch(e) { ... }` per `internal/builtins/close.go`'s documented usage.
- Otherwise (the call is nested inside a larger expression — a type cast, a call argument,
  an operand of an operator, and so on) — push **only the primary (first) return value**,
  silently discarding the rest. This matches the single-value usage `json.Marshal` and
  similar functions are documented to support, and never leaves anything behind to corrupt
  the next instruction.

This distinction was necessary because a first attempt at the fix (see above) satisfied the
crash but broke the double-close/`try`/`catch` interaction with `close()`, which relies on
the abandoned error surviving all the way to `DropToMarker`'s check.

New tests:

- `internal/language/bytecode/callRuntimeFunction_test.go` and
  `internal/language/bytecode/callNative_test.go` — each existing "multi-return push" test
  was split into a `..._MultiAssignment` variant (using a new `withNextOpcode(StackCheck)`
  test helper in `testhelpers_test.go` to simulate an explicit multi-value assignment, still
  asserting the full marker+values push) and a new `..._SingleValueContext` variant (the
  BUG-32 regression case: no `StackCheck`/`DropToMarker` follows, so only the primary value
  is pushed and the stack ends up empty).
- `tests/json/marshal.ego` — `"json: Marshal nested directly in string() (BUG-32)"`
  reproduces the exact `docs/LANGUAGE.md` example and asserts the plain JSON string result;
  `"json: Marshal nested in fmt.Println (BUG-32)"` covers the variadic-call nesting case.
- `tests/packages/time.ego` — `"packages: time.Parse nested in fmt.Println (BUG-32)"`
  reproduces the third bug-report example.
- The pre-existing `tests/defer/channel.ego` test `"defer: double close is catchable, not a
  crash"` (BUG-29) now doubles as a non-regression check that the `DropToMarker`
  abandoned-error path still works after this fix.

