# INDEX-4 — `Patch` trusts the caller's window and panics inside `make()`

**Affected function:** `(*ByteCode).Patch`
**File:** `language/bytecode/optimizer.go`
**Risk:** Medium — a bad pattern-match window crashes the optimizer instead of
being rejected
**Status: RESOLVED**

## INDEX-4: Description

`Patch(start, deleteSize, insert)` performed no validation of the window it was
given:

```go
tailStart := start + deleteSize
tail := make([]instruction, b.nextAddress-tailStart)
copy(tail, b.instructions[tailStart:b.nextAddress])
...
instructions = append(instructions, b.instructions[:start]...)
```

When `start + deleteSize` exceeds `b.nextAddress`, `b.nextAddress - tailStart`
is negative and `make()` panics with "makeslice: len out of range". The slice
expressions on the following lines are equally unguarded, as is
`b.instructions[:start]` for a `start` past the end.

`start` and `deleteSize` describe a region the caller located by matching an
optimization pattern against the instruction stream, and `Patch` is exported,
so the window is not trustworthy input.

## INDEX-4: Fix

A window that does not lie entirely within the emitted instruction stream is
rejected as a no-op and logged to the optimizer log:

```go
if start < 0 || deleteSize < 0 || start+deleteSize > b.nextAddress || b.nextAddress > len(b.instructions) {
    ui.Log(ui.OptimizerLogger, "optimizer.patch.range", ui.A{
        "start": start,
        "size":  deleteSize})

    return
}
```

A new `optimizer.patch.range` message was added to all four language files.
