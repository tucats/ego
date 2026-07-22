# DEBUGGER-COMMANDS-1 — errors.Equal (full string) used instead of errors.Equals (key-only)

**File:** `debugger/commands.go`  
**Function:** `debuggerPrompt`, the `"print"` case  
**Risk:** Medium — `print` command output may be silently discarded when the
inner execution stops with a contextualized `ErrStop`  
**Status: RESOLVED**

## DEBUGGER-COMMANDS-1: Original behavior

After running the print expression in a child context, the code checked:

```go
if err == nil || errors.Equal(err, errors.ErrStop) {
    output := strings.TrimSuffix(printCtx.GetOutput(), "\n")
    sessionContext.println(output)
} else {
    sessionContext.say("msg.debug.error", ...)
}
```

`errors.Equal` (no `s`) compares the fully formatted `.Error()` strings of
both sides.  `errors.ErrStop` carries a bare message such as `"stop"`.  When
the child context exits normally via `ErrStop`, the error value stored in
`err` is often a contextualized form — e.g. `ErrStop.Context("line 5")` —
whose `.Error()` string is `"stop: line 5"`.  That does not match the bare
`"stop"` produced by `errors.ErrStop.Error()`, so `errors.Equal` returns
`false`.

The result is that the print output is discarded and a spurious error message
is shown to the user instead.

`errors.Equals` (with `s`) delegates to `(*Error).Is()`, which compares only
the underlying i18n key and ignores any `.Context(...)` suffix.  It correctly
identifies any flavour of `ErrStop` as a match.

## DEBUGGER-COMMANDS-1: Fix

Changed the call at the affected line from `errors.Equal` to `errors.Equals`:

```go
if err == nil || errors.Equals(err, errors.ErrStop) {
```
