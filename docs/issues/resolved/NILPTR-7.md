# NILPTR-7 — Unchecked type assertion on the service response `_body` field

**Affected functions:** `callServices` response formatting, `ChildService`
response formatting
**Files:** `server/services/service.go`, `server/services/child.go`
**Risk:** Medium — a missing or retyped `_body` field panics while formatting a
service response
**Status: RESOLVED**

## NILPTR-7: Description

Both files read the response body out of the Ego response struct with a
single-value type assertion:

```go
bodyValue := response.GetAlways("_body")
body := bodyValue.(*data.Array)
b = body.GetBytes()
```

In Go, the one-value form of a type assertion panics when the value is not of the
asserted type. The two-value ("comma ok") form — `v, ok := x.(T)` — sets `ok` to
false instead, and also covers the nil case, because a nil interface never
satisfies a concrete type.

Two things can put something other than a `*data.Array` in `bodyValue`:

- `Struct.GetAlways` returns a **nil interface** for a field that is not present.
  It does not error and does not panic; the nil comes straight back.
- `_body` lives in an Ego struct that the service's own Ego code holds a
  reference to. Ego's default type-enforcement mode is `dynamic`, in which a
  value may change type on assignment, so a service is not structurally
  prevented from leaving something else in that field.

The strongest evidence that this was an oversight rather than a considered
invariant is the surrounding code. Every other field read in the same function
already uses a safe accessor:

```go
// headers: checked assertions at every level
headerV := response.GetAlways(headersField)
if s, ok := headerV.(*data.Struct); ok {
    mv := s.GetAlways(headersField)
    if m, ok := mv.(*data.Map); ok {
        ...
        if array, ok := arrayV.(*data.Array); ok {

// _status and _size: data.Int tolerates any type
status, _ := data.Int(response.GetAlways("_status"))
size, _ := data.Int(response.GetAlways("_size"))
```

And `getWriterBody` in `runtime/http/writer.go` reads the very same `_body` field
with the checked form:

```go
value := s.GetAlways("_body")
if writer, ok := value.(*data.Array); ok {
    return writer, nil
}
```

`_body` in these two response-formatting paths was the only place that did not.

## NILPTR-7: Fix

Both sites use the comma-ok form and log the unexpected type:

```go
bodyValue := response.GetAlways("_body")
if body, ok := bodyValue.(*data.Array); ok {
    b = body.GetBytes()
} else {
    ui.Log(ui.ServicesLogger, "services.body.invalid", ui.A{
        "session": session.ID,
        "type":    data.TypeOf(bodyValue).String()})
}
```

Falling through with an empty `b` is the right behavior in both files. In
`child.go` the block immediately below already treats an empty body as "use the
captured print buffer instead", so the recovery path was there waiting to be used.

A new `services.body.invalid` message was added to all four language files.
