# BUG-90 — nil rejected as a func or chan return value in strict mode

**Severity:** MEDIUM

**Description:**  
Returning `nil` from a function declared to return a `func` or `chan` type
failed under `--types strict`, even though `nil` is the zero value for both:

```ego
func f() func(int) int { return nil }   // Error: type mismatch: nil, func( int) int
func f() chan { return nil }            // Error: type mismatch: nil, chan
```

Strict-mode `requireMatch` (`internal/language/bytecode/coerce.go`) admitted a
`nil` value only when the target type was a pointer or the built-in `error`
type: `if v == nil && (t.IsPointer() || t.Kind() == data.ErrorKind)`. Map,
slice, and interface returns of `nil` happened to pass through a different path,
but `func` and `chan` returns had no such escape and were rejected. This is the
same class of gap as BUG-65 (the `error` type not treated as nil-compatible),
extended to the remaining nillable kinds.

**Fix:**  
Replaced the pointer/error-only test with an `isNillableKind` helper that
accepts `nil` for every type whose zero value is `nil` — pointer, function, map,
slice, channel, interface, and error:

```go
if v == nil && isNillableKind(t) {
    return c.push(v)
}
```

Regression coverage: `tests/functions/return_types.ego` — the "returns: error
type, nil and non-nil" and "returns: multiple returns mixing compound types"
tests (the latter returns `nil` for a `func(int) int` value on its error path),
under both `--types=strict` and `--types=dynamic`.

