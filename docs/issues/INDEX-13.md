# INDEX-13 — Server restart reads option values past the end of the argument list

**Affected function:** `Restart`
**File:** `commands/restart.go`
**Risk:** Medium — panics on a saved command line whose last element is a flag
**Status: RESOLVED**

## INDEX-13: Description

Two option scans read `args[i+1]` without establishing that a value follows the
flag. `args` is the command line read back from the server status file, so any
flag can legitimately appear last.

**1. No check at all:**

```go
for i, v := range args {
    if v == "--session-uuid" {
        args[i+1] = logID.String()
```

**2. An off-by-one check, which is worse than none because it looks correct:**

```go
if v == "--log-file" && i+1 <= len(args) {
    logFile = args[i+1]
```

`i+1 <= len(args)` admits `i+1 == len(args)`, which is exactly the out-of-range
index the test was meant to exclude. The valid-index test for a slice of length
`n` is `i+1 < n`.

## INDEX-13: Fix

Both scans use a strict comparison. A trailing `--session-uuid` is now treated as
"not present", so the option is appended with its value by the existing
`if !found` branch below:

```go
if v == "--session-uuid" && i+1 < len(args) {
if v == "--log-file" && i+1 < len(args) {
```
