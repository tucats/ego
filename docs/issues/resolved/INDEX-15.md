# INDEX-15 — Splicing default `--log` arguments invalidates previously recorded argument positions

**Affected function:** `processServerArguments`
**File:** `commands/start.go`
**Risk:** High — silently rewrites the wrong element of the server's command line
**Status: RESOLVED**

## INDEX-15: Description

The scan at the top of the function records the positions of the `--users` and
`--log-file` values. Later, if no `--log` option was given, two elements are
spliced in immediately after the program name:

```go
if loggingNamesArg == 0 {
    if defaultLoggingNames := settings.Get(defs.ServerDefaultLogSetting); defaultLoggingNames != "" {
        newArgs := make([]string, 3)
        newArgs[0] = args[0]
        newArgs[1] = "--log"
        newArgs[2] = defaultLoggingNames
        args = append(newArgs, args[1:]...)
    }
}
```

Every argument after `args[0]` shifts two places to the right, but
`userDatabaseArg` and `logNameArg` were not adjusted. The fixups that follow then
address the wrong elements:

```go
args[userDatabaseArg] = normalizeDBName(args[userDatabaseArg])
...
args[logNameArg], _ = filepath.Abs(args[logNameArg])
```

Instead of normalizing the user-database path and absolutizing the log file name,
these overwrite whatever arguments have moved into those slots — potentially the
injected `--log` flag or its value. Nothing reports the problem; the server is
simply launched with a corrupted command line, and because that line is saved to
the status file, the corruption persists across restarts.

This is reachable whenever a user starts the server with `--users` or
`--log-file` but without `--log`, and a default logging set is configured.

## INDEX-15: Fix

The recorded positions are shifted by the same amount as the arguments they name:

```go
const insertedArgs = 2

args = append(newArgs, args[1:]...)

if userDatabaseArg > 0 {
    userDatabaseArg += insertedArgs
}

if logNameArg > 0 {
    logNameArg += insertedArgs
}
```
