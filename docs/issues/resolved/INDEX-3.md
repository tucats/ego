# INDEX-3 — `Remove` has no bounds check and its negative-offset arithmetic is inverted

**Affected function:** `(*ByteCode).Remove`
**File:** `language/bytecode/bytecode.go`
**Risk:** Medium — every documented use of the negative form panics
**Status: RESOLVED**

## INDEX-3: Description

```go
func (b *ByteCode) Remove(address int) {
    if address >= 0 {
        b.instructions = append(b.instructions[:address], b.instructions[address+1:]...)
    } else {
        offset := b.nextAddress - address
        b.instructions = append(b.instructions[:offset], b.instructions[offset+1:]...)
    }
    ...
```

Two defects:

1. **No bounds check.** Any `address` at or past the end of the instruction
   stream panics on `b.instructions[address+1:]`.

2. **Inverted offset arithmetic.** The doc comment says a negative address is
   "the offset from the end of the bytecode", but `b.nextAddress - address` with
   a negative `address` moves *forward* past the end rather than back from it.
   `Remove(-1)`, which should remove the last instruction, computes
   `nextAddress + 1` and panics.

The sibling `Delete` method already treats an invalid position as a no-op, so
`Remove` was also inconsistent with the established convention in the same file.

## INDEX-3: Fix

The negative offset is added rather than subtracted, and a position that does
not name an emitted instruction is a no-op, matching `Delete`:

```go
position := address
if position < 0 {
    position = b.nextAddress + address
}

if position < 0 || position >= b.nextAddress || position >= len(b.instructions) {
    return
}
```
