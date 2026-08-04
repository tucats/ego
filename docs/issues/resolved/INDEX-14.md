# INDEX-14 — `processServerArguments` records and reads option positions without bounds checks

**Affected function:** `processServerArguments`
**File:** `commands/start.go`
**Risk:** High — `ego server start --users` (flag with no value) panics
**Status: RESOLVED**

## INDEX-14: Description

The option scan recorded or read `i+1` for four different flags, none of them
checked against the length of `args`:

```go
if v == "--session-uuid" {
    logID = uuid.MustParse(args[i+1])
    ...
if v == "--log" || v == "-l" {
    loggingNamesArg = i + 1
}

if v == "--users" || v == "-u" {
    userDatabaseArg = i + 1
}

if v == "--log-file" {
    logNameArg = i + 1
}
```

`args` is the user's command line (and, on a restart, the saved server status
file), so any of these flags can appear as the final element. `--session-uuid`
panicked immediately on `args[i+1]`, and the other three stored an out-of-range
position that was dereferenced later in the same function:

```go
if userDatabaseArg > 0 {
    args[userDatabaseArg] = normalizeDBName(args[userDatabaseArg])
}
...
if logNameArg > 0 {
    args[logNameArg], _ = filepath.Abs(args[logNameArg])
}
```

Separately, `uuid.MustParse` panics on a malformed value. A bad UUID on the
command line is a user error, not a programming error, so `MustParse` is the
wrong choice here.

Finally, `args[0]` is read in four places (including `exec.LookPath(args[0])` and
a splice that rebuilds the list) with no check that the list is non-empty.

## INDEX-14: Fix

A single `hasValue` test gates all four options, so a flag with no value is
ignored and the corresponding default is appended by the existing fallback
branches:

```go
hasValue := i+1 < len(args)
```

`uuid.MustParse` was replaced with `uuid.Parse`; a malformed value leaves the
freshly generated `logID` in place. An empty argument list is now rejected with
`ErrInvalidArgumentList` before `args[0]` is touched.

See also INDEX-15, which concerns the same recorded positions going stale.
