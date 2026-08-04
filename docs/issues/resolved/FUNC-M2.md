# FUNC-M2 — Value receiver method cannot be called on a pointer variable

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

