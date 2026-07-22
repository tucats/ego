# JSON-H2 — Unmarshal type mismatch raises exception instead of returning error

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

