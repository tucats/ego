# CALL-12 — Sandbox flags set by `Context.Sandboxed()` invisible to runtime functions called from top-level code

**Affected function:** `callRuntimeFunction`  
**File:** `bytecode/callRuntimeFunction.go`  
**Risk:** High — a sandboxed-execution security check (e.g. `rest.UseToken()`'s
refusal to attach the ambient logon token while sandboxed) can silently fail
to trigger, depending on how deeply nested the call site is relative to where
sandboxing was enabled; affects the real dashboard "run code" sandboxed
session path (`server/admin/run.go`), not just CLI test tooling  
**Discovered by:** manual testing while reviewing the `rest` package's
`UseToken()` (added in the prior `rest` package session) against the `sql`
package's analogous `AsStruct()` gap  
**Status: RESOLVED**

## CALL-12: Description

`Context.Sandboxed(flag)` records the sandbox state in two places: the
context's own atomic fields (`c.sandboxedIO`, `c.sandboxedExec` — checked
directly by the `Declaration.Sandboxed` blanket-block mechanism) and, per its
doc comment, "also in the symbol table so that it can be accessed by runtime
functions as needed" — via `c.symbols.SetAlways(defs.SandboxedIOSymbolName,
flag)` (and the `Exec` counterpart). `NewContext()` does the same at
construction time. This second path is what packages like `rest`
(`sandboxedIO(s)`) and `io` (`SandBoxedIO(s)`) use for their own
finer-grained, in-function sandboxing checks, since a runtime function only
receives a `*symbols.SymbolTable`, not the `*Context`.

`callRuntimeFunction` builds a fresh child symbol table for every call:

```go
if fullScope {
    parentTable = c.symbols
} else {
    parentTable = c.symbols.FindNextScope()
}

functionSymbols := symbols.NewChildSymbolTable("builtin "+name, parentTable)
```

`FindNextScope()` (`symbols/tables.go`) always **skips the table it's called
on** — for the common case where that table isn't itself a scope boundary, it
returns `s.parent` directly. Combined with `SetAlways` having written the
sandbox flags directly onto `c.symbols` itself, this means: for any runtime
function call made with **no intervening nested scope** between it and the
table `Sandboxed()`/`NewContext()` last wrote to (i.e. a call in a bare
top-level statement, not inside any `if`/`for`/`try`/function body that
pushed its own child scope first), `parentTable` skips right past the exact
table holding the current, correct flag value and lands on its parent
instead — which never had it set. `functionSymbols.Get(...)` then finds
nothing there, silently returning the zero value (`false`,
never-sandboxed) regardless of the context's real state.

A call made one level deeper — e.g. inside a `try { ... }` block, which pushes
its own child scope before executing — does not skip the table with the flag
(that intervening scope's `FindNextScope()` correctly walks back to it), so
the same check succeeds. This inconsistency was reproduced directly:

```go
conn := rest.New("")

// Bare top-level statement: sandboxing silently NOT detected (bug).
_, err := conn.UseToken(true)
fmt.Println(err)   // <nil> -- wrong; should be blocked when sandboxed

// Same call one scope deeper: sandboxing correctly detected.
try {
    _, err := conn.UseToken(true)
    fmt.Println(err)   // in UseToken, no privilege for operation -- correct
} catch {}
```

Wrapping a whole program in `package main` / `func main() { ... }` also
"fixes" the symptom, because then no top-level statement is bare — every
statement already executes one scope deeper than the table `Sandboxed()`
wrote to. This masked the bug for any conventionally-structured `.ego`
program; it only reproduces for a bare, unwrapped fragment (as used by `ego
run < file.ego`'s stdin path, `ego test`'s `@test` blocks, and — critically —
the dashboard's `executeAdminEgo`/`executeAdminDebug` sandboxed "run code"
session handlers in `server/admin/run.go`, which compile and run user
submissions directly against a session-level table with no synthetic
function wrapper).

## CALL-12: Fix

`callRuntimeFunction` now stamps both sandbox fields directly onto every
call's freshly created `functionSymbols` table, read straight from the
context's own authoritative atomic fields, immediately after that table is
created:

```go
functionSymbols := symbols.NewChildSymbolTable("builtin "+name, parentTable)

functionSymbols.SetAlways(defs.SandboxedIOSymbolName, c.sandboxedIO.Load())
functionSymbols.SetAlways(defs.SandboxedExecSymbolName, c.sandboxedExec.Load())
```

Since `symbols.Get()` checks the local table before walking to its parent,
this guarantees every runtime function call sees the correct, current value
regardless of scope depth or nesting, without needing to change
`FindNextScope()`'s general (and otherwise legitimate) scope-skipping
behavior, which is relied on for its normal purpose elsewhere. Also fixed the
same-shaped bug this investigation started from: `rest.UseToken()` was
declared with a single (non-error) return and returned a bare `(nil, err)` on
its own sandboxing-rejection path — see the `rest` package's `UseToken` fix
in this same change for the accompanying (value, error) convention fix.

Regression tests: `Test_callRuntimeFunction_SandboxFlagsVisibleInFunctionSymbols`
(`bytecode/callRuntimeFunction_test.go`) calls a synthetic runtime function
with `fullScope=false` against a two-level (root + child) test context —
reproducing the exact shape that hid the flag — and asserts both symbols are
visible with the correct value in both the sandboxed and non-sandboxed case.
