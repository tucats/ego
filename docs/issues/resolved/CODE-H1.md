# CODE-H1 — Exec sandbox bypass when exec is globally permitted

**Affected files:**

- `runtime/exec/run.go:22` — `run()`
- `runtime/exec/output.go:18` — `output()`
- `runtime/exec/command.go:19` — `command()`

**Description:**  
Every execution context created by `RunCodeHandler` calls `.Sandboxed(true)`,
which sets `SandboxedExecSymbolName = true` in the context's symbol table.
The intent is to prevent user-submitted code from spawning OS subprocesses. In
practice the guard in all three exec functions reads:

```go
if !settings.GetBool(defs.ExecPermittedSetting) || !sandBoxedExec(s) {
    return nil, errors.ErrNoPrivilegeForOperation.In("Run")
}
```

`sandBoxedExec(s)` returns `true` when `SandboxedExecSymbolName` is `true`
(i.e. when the context is sandboxed), so `!sandBoxedExec(s)` evaluates to
`false`. When an administrator has also set `ExecPermittedSetting = true`, the
combined condition is `false || false = false` — the check passes and exec is
allowed even inside a sandboxed admin/run context.

The default value of `ExecPermittedSetting` is `false` (see
`runtime/profile/initialization.go:92`), so the endpoint is safe out of the
box. However, `.Sandboxed(true)` provides a false sense of protection: a
single administrator setting `ego.runtime.exec = true` silently re-enables
subprocess execution for all user-submitted dashboard code.

**Recommendation:**  
Make sandboxed execution contexts unconditionally block exec, regardless of the
global setting. One clear approach is to rename the symbol to reflect its actual
semantics (e.g., `SandboxedExecAllowed`) and then invert the guard so that a
sandboxed context explicitly overrides the global permission:

```go
// Block exec when the context is sandboxed, even if globally permitted.
if sandboxedCtx || !settings.GetBool(defs.ExecPermittedSetting) {
    return nil, errors.ErrNoPrivilegeForOperation.In("Run")
}
```

Alternatively, introduce a separate `sandboxedCtx` atomic bool on the
`bytecode.Context` (distinct from `sandboxedExec`) that is set by `.Sandboxed(true)`
and checked unconditionally by all exec functions before the global setting.

**Resolution (April 2026):**  
`Sandboxed()` in `bytecode/context.go` now sets `sandboxedExec` to `false`
(exec blocked) when `flag` is `true`, and restores it from `ExecPermittedSetting`
when `flag` is `false`. This ensures that calling `.Sandboxed(true)` on an
admin/run execution context unconditionally disables subprocess exec regardless
of the global setting. The existing exec guard in `runtime/exec/run.go`,
`output.go`, and `command.go` is unchanged; `!sandBoxedExec(s)` now correctly
evaluates to `true` for any sandboxed context.

