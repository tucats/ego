# BUG-04 — `recover()` in deferred function of a value-returning function causes caller error

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

