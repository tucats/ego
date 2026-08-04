# BUILTIN-NEW-2 — `NewInstanceOf` returned the existing channel instead of a new one

**Affected function:** `NewInstanceOf`
**File:** `builtins/new.go`
**Risk:** Low
**Status: RESOLVED**

## Original NEW-2 behavior

When the argument was a `*data.Channel`, `NewInstanceOf` returned the same
channel unchanged.  `$new(ch)` and `ch` therefore aliased the same underlying
channel object.

## Fix for NEW-2

`NewInstanceOf` now calls `data.NewChannel(typeValue.Cap())` to create a fresh,
independent channel with the same buffer capacity.  `Cap()` is a new method
added to `data.Channel` (alongside the `Len()` method added for
BUILTIN-LENGTH-1) that returns the `size` field set at construction:

```go
if typeValue, ok := args.Get(0).(*data.Channel); ok {
    return data.NewChannel(typeValue.Cap()), nil
}
```

**Tests:** `Test_NewInstanceOf_ChannelReturnsNewInstance`
