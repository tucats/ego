# CODE-M5 — Language extensions enabled in sandboxed symbol table

**Affected file:** `server/admin/run.go:355` — `getOrCreateSymbolTable()`

```go
comp := compiler.New("dashboard").
    SetExtensionsEnabled(true).
    SetRoot(consoleTable)
```

**Description:**  
Persistent console symbol tables are initialized with the compiler's extension
mode enabled. Extensions add language features beyond the standard Ego/Go
subset (for example, `panic` as a statement token is guarded by
`ExtensionsEnabledSetting` in `compiler/statement.go:72`). Because the symbol
table is re-used across multiple requests in console mode, any effect of
extension-enabled compilation persists into subsequent executions.

If any extension exposes lower-level primitives, bypasses type or sandbox
checks, or widens the set of callable native functions in ways not anticipated
by the sandbox model, every dashboard user who has a console session is
exposed to that wider attack surface. The risk is currently unquantified
because the full set of behaviors gated on `ExtensionsEnabledSetting` has not
been audited for sandbox compatibility.

**Recommendation:**  
Audit every code path that checks `ExtensionsEnabledSetting` or
`SetExtensionsEnabled` and confirm that none of the extension-only behaviors
conflict with the `Sandboxed(true)` constraints. If any do, disable extensions
for sandboxed contexts, or guard the individual extension features with an
additional sandbox check.

**Status:** Resolved — audited, no conflict with the sandbox model.

**Audit (July 2026).** Every compile-time behavior gated on the compiler's
`extensionsEnabled` flag (`SetExtensionsEnabled` / `ExtensionsEnabledSetting`)
was enumerated and checked against what the sandbox actually enforces:

| Extension-gated behavior | Site | New capability? |
| --- | --- | --- |
| `if/then/else` expression | `compiler/expr_atom.go:42` | No — control-flow sugar |
| `panic` as expression/statement token | `compiler/expr_atom.go:226`, `compiler/call.go:16` | No — `panic()` is an always-available builtin; this only lexes the keyword form |
| post-`++`/`--` in an expression | `compiler/expr_atom.go:348` | No — arithmetic sugar |
| multi-rune char literal to `[]int32` | `compiler/expr_atom.go:491` | No — constant array literal |
| `?expr : fallback` optional / short try | `compiler/expr_atom.go:950`, `compiler/expr_condiitional.go:24` | No — error-handling sugar |
| `x.(type)` outside a type switch | `compiler/type.go:61` | No — introspection sugar |
| `assert(expr)` | `compiler/statement.go:165` | No — aborts on false |
| `call expr()` | `compiler/statement.go:177` | No — discards a return value |
| `print` | `compiler/statement.go:209` | No — writes the same stdout `fmt.Print` already does |
| `throw` | `compiler/statement.go:225` | No — raises a catchable error |
| `try`/`catch` | `compiler/statement.go:231` | No — structured error handling |
| extended reserved words (`try`, `catch`, `throw`, `print`, `call`, `exit`) | `tokenizer/reserved.go:462`; `IsReserved(ext)` in `var.go`, `lvalue.go`, `for.go`, `statement.go:108` | No — makes *more* words reserved (more restrictive, not less) |
| `sizeof`, `typeof` builtins | `builtins/functions.go:330` (only two `Extension: true` entries) | No — pure introspection; touch no fs/net/exec |

The sandbox boundary is enforced entirely at **runtime**, inside the native
functions, keyed off symbol-table flags and config settings that the
compile-time extensions flag never reads or writes:

- **exec** — `internal/runtime/exec/run.go:27` blocks a subprocess when
  `!ExecPermittedSetting || sandBoxedExec(s)` (the `SandboxedExecSymbolName`
  flag). `Context.Sandboxed(true)` (`bytecode/context.go:436`) forces that flag
  true unconditionally.
- **file I/O** — `internal/runtime/io` and `internal/runtime/os` confine every
  path through `util.SandboxJoin` (the `SandboxedIOSymbolName` flag plus
  `SandboxPathSetting`); see CODE-M4.
- **network** — `internal/runtime/rest/security.go` applies the same sandbox
  flag.

None of the extension features import a package, add a native function that
reaches the filesystem/network/subprocess, write to any sandbox flag or
`SandboxPathSetting`, or call a runtime function in a way that skips its check
— `throw`/`try`/`?:`/`call`/`panic` all wrap ordinary calls that still hit the
same runtime gates, and *catching* a `ErrNoPrivilegeForOperation` after the
denied operation has already done nothing grants no access. `recover()`
intercepts only `UserPanic` from the `panic()` builtin, never a sandbox denial
(which is a normal catchable error, not a panic).

The one genuine capability that looks extension-shaped — `exit`, which compiles
to `os.Exit(n)` (`compiler/exit.go`) — is gated on a **separate** `exitEnabled`
flag (`compiler/statement.go:191`), set only in interactive CLI mode
(`commands/run.go:423`) and never by the dashboard/service sandbox paths. With
extensions on but `exitEnabled` off (the sandboxed dashboard case), `exit` is a
compile error. This coupling must be preserved: do not fold `exit` in under the
`extensionsEnabled` flag.

Conclusion: enabling extensions in the sandboxed console
(`server/admin/run.go` via `Sandboxed(!session.Admin)`) does not widen the
sandbox attack surface. The `exit`/`exitEnabled` separation is the invariant
that keeps it that way and must be preserved.

**Hardening applied.** Although the audit found no vulnerability, the mechanism
by which extensions were enabled *was* leaky and has been fixed. The
package-level `compiler.CompileString` (the dashboard's per-request compile
path) previously enabled extensions by mutating the process-global
`ExtensionsEnabledSetting` for the duration of the call — so that the
`import` sub-compiler's `New()` would observe it — and restoring it via
`defer`. Because that global is shared by every goroutine, a concurrent
compile in another session could observe the flipped value, and the flag was
forced on regardless of the caller's intent. `CompileString` now takes an
explicit `extensions bool` (each of its three callers passes it directly) and
sets the flag only on its own compiler instance; `compileImport`
(`compiler/import.go`) inherits `c.flags.extensionsEnabled` from its parent
rather than re-reading the global. No process-global state is mutated during a
compile, so the flag can no longer leak across sessions. Library packages that
use extension syntax (`math`'s `throw`, `http`'s `try/catch`) still compile
because the dashboard passes `extensions = true` and imports inherit it.

