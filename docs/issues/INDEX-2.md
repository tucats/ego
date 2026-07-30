# INDEX-2 — `SetAddress` accepts a mark one past the last instruction

**Affected function:** `(*ByteCode).SetAddress`
**File:** `language/bytecode/bytecode.go`
**Risk:** Medium — panics on a sealed bytecode; silently patches an unemitted
instruction slot otherwise
**Status: RESOLVED**

## INDEX-2: Description

The guard used an inclusive upper bound:

```go
if mark > b.nextAddress || mark < 0 {
    return errors.ErrInvalidBytecodeAddress
}

instruction := b.instructions[mark]
```

`mark == b.nextAddress` names the slot *after* the last emitted instruction, and
it passed the check. Two different failures follow:

- On a sealed bytecode, `Seal()` truncates `b.instructions` to `b.nextAddress`,
  so `b.instructions[mark]` is out of range and panics.
- On an unsealed bytecode the slot usually exists as spare capacity, so the
  write succeeds but patches an instruction that was never emitted — a silent
  no-op that hides the caller's bug.

The mark is a position the caller saved earlier and may no longer be valid; see
the `SetAddress(0, count)` call in `compiler/lvalue.go`, which targets a
bytecode object that can legitimately be empty.

## INDEX-2: Fix

The bound is now exclusive, and is also checked against the physical length of
the instruction slice:

```go
if mark < 0 || mark >= b.nextAddress || mark >= len(b.instructions) {
    return errors.ErrInvalidBytecodeAddress
}
```
