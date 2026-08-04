# BUG-40 — `uuid.Parse` on invalid input crashes the program instead of returning a catchable error

**Severity:** HIGH

**Description:**  
`docs/LANGUAGE.md`'s `uuid.Parse` documentation states that on a parse failure, "the `id`
will be `nil`, and the `err` will describe the error" — a normal, catchable two-value
return. In practice, an invalid UUID string aborts the whole program with an uncatchable
runtime error; `err` is never assigned/inspectable at all.

**Reproducer:**

```go
import "uuid"

func main() {
    id, err := uuid.Parse("not-a-uuid")
    fmt.Println(id, err)
}
```

**Actual output:**

```text
Error: at main(line 3), invalid UUID length: 10
Error: terminated with errors
```

**Expected output:**

```text
<nil> invalid UUID length: 10
```

**Notes:**  
Root cause in `internal/runtime/uuid/uuid.go:23-34` (`parseUUID`):

```go
u, err := uuid.Parse(s)
if err != nil {
    return nil, errors.New(err)   // NOT wrapped in data.NewList(...)
}
```

This is exactly the anti-pattern documented in this repo's own CLAUDE.md
("`callRuntimeFunction` dispatch mechanics — catchable vs uncatchable errors"): a
non-`data.List` result combined with a non-nil Go error triggers `c.runtimeError(err)`, an
uncatchable abort, instead of the documented catchable two-value return.

**Resolution (July 2026):**  
`internal/runtime/uuid/uuid.go` — `parseUUID`: the function's declaration
(`internal/runtime/uuid/types.go`) already listed two `Returns` entries
(`UUIDTypeDef, data.ErrorType`), but the implementation returned a bare
`(nil, error)` pair instead of wrapping the pair in a `data.List`. Per
`callRuntimeFunction`'s dispatch rules, a non-`data.List` result combined with
a non-nil Go `error` is treated as an uncatchable abort rather than a normal
two-value return. The fix wraps both the success and failure paths in
`data.NewList(...)`, with the wrapper-level Go `error` always `nil` (the
dispatcher ignores it for list results):

```go
u, err := uuid.Parse(s)
if err != nil {
    return data.NewList(nil, errors.New(err)), nil
}

result := data.NewStruct(UUIDTypeDef).SetNative(u)

return data.NewList(result, nil), nil
```

`uuid.Parse("not-a-uuid")` now returns `(nil, err)` exactly as documented,
with `err.Error() == "invalid UUID length: 10"`, and no longer aborts the
program.

New tests:

- `internal/runtime/uuid/uuid_test.go` — `TestParseUUID_ValidInput` confirms
  the success path still returns a 2-element list with a populated UUID and a
  `nil` error; `TestParseUUID_InvalidInput` is the direct regression test,
  asserting the wrapper itself returns a `nil` Go error and that the returned
  `data.List` carries `(nil, error)` rather than the process aborting.
- `tests/packages/uuid.ego` — `"packages: uuid.Parse invalid input is
  catchable"` exercises the fix end-to-end at the language level: it asserts
  `id == nil`, `err != nil`, and the exact error text. Prior to the fix, this
  test would have aborted the entire test file with an uncatchable runtime
  error before reaching any `@assert`.

