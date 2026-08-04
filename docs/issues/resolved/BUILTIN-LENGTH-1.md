# BUILTIN-LENGTH-1 — `Length` returned `math.MaxInt32` for any non-empty channel

**Affected function:** `Length`
**File:** `builtins/length.go`
**Risk:** Low
**Status: RESOLVED**

## Original LENGTH-1 behavior

The channel case returned `math.MaxInt32` for any open channel and `0` only
when the channel was both closed and empty.  An open channel with zero buffered
items returned `math.MaxInt32` instead of `0`, diverging from Go's `len()`.

## Fix for LENGTH-1

The channel case now calls `arg.Len()` — a new method added to `data.Channel`
that delegates to Go's built-in `len(c.channel)`:

```go
case *data.Channel:
    return arg.Len(), nil
```

The `math` import, which was only needed for `math.MaxInt32`, was also removed.
A companion `Cap()` method was added to `data.Channel` to support
BUILTIN-NEW-2.

**Tests:** `Test_Length_ChannelEmptyOpenReturnsZero`,
`Test_Length_ChannelWithItemsReturnsCount`,
`Test_Length_ChannelEmptyAfterCloseReturnsZero`
