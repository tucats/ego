# BUG-06 — `++`/`--` not permitted on struct fields or array elements

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
  `StoreIndex` + `DropToMarker`) to write back and clean up. Any other lvalue
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

