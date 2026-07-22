# TYPES-1: Dead nil guard in `deRefByteCode` leaves `*c3` vulnerable to panic

| | |
| :-- | :-- |
| **Affected function** | `deRefByteCode` in `bytecode/types.go` |
| **Risk** | MEDIUM — dereferencing an Ego variable that holds a nil pointer (`*any(nil)`) panics the runtime instead of returning ErrNilPointerReference |
| **Discovering test** | `Test_deRefByteCode_TypedNilInnerPointer_TYPES1` |
| **Status** | RESOLVED |

## TYPES-1: Original behavior

`deRefByteCode` walks two levels of indirection to dereference an Ego pointer
variable:

```go
addr     = GetAddress("p")    // *any → symbol's value slot
content  = addr.(*any)        // outer pointer (always non-nil here)
c2       = *content           // the Ego pointer value stored in the slot
c3, ok   = c2.(*any)          // inner pointer (the Ego pointer target)
*c3                           // final dereference ← potential panic
```

Inside the `c3` branch there is a nil check meant to guard against `*c3`
panicking:

```go
if data.IsNil(content) {           // ← TYPES-1 bug: checks 'content', not 'c3'
    return c.runtimeError(errors.ErrNilPointerReference)
}
return c.push(*c3)
```

`content` is the outer `*any` pointer, which was already verified non-nil by
an earlier guard (line ~362).  Because `content` can never be nil at this
point, the check is dead code and `*c3` is reached unconditionally.

If the symbol holds a **typed nil** `*any` value — for example, an Ego pointer
variable that has been declared but not assigned — the type assertion
`c2.(*any)` succeeds with `c3 = nil`.  The dead guard passes (content is non-nil),
and `*c3` panics.

## TYPES-1: Fix

The nil guard was moved to before the first dereference and corrected to check
`c3` (the inner pointer) instead of `content` (the outer pointer):

```go
if c3, ok := c2.(*any); ok {
    if c3 == nil {   // ← corrected: was data.IsNil(content)
        return c.runtimeError(errors.ErrNilPointerReference)
    }
    xc3 := *c3      // safe — c3 is guaranteed non-nil
    ...
}
```

`Test_deRefByteCode_TypedNilInnerPointer_TYPES1` stores a typed-nil `*any` in
a symbol and confirms that `deRefByteCode` returns `ErrNilPointerReference`
instead of panicking.
