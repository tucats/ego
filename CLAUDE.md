# Ego — Claude Code Guide

## Project Overview

`ego` is a Go implementation of the _Ego_ scripting language — an emulated-Go language that closely mirrors Go syntax. It can run programs interactively (REPL-like), execute `.ego` source files, or operate as a REST server with Ego programs as service endpoints. It also includes a web dashboard and a built-in SQL table/database layer.

Module path: `github.com/tucats/ego`

Current version: stored in `tools/buildver.txt` (format `MAJOR.MINOR-BUILD`, e.g. `1.8-1518`).

---

## Building

**Normal build** (current platform):

```sh
./tools/build
```

**With version increment:**

```sh
./tools/build -i
```

**Cross-compile for all platforms:**

```sh
./tools/build --all
```

Output lands in `builds/{macos,linux,windows}/{applesilicon,x86,arm64}/ego[.exe]`. These cross-compile outputs are **not** installed into `~/bin` automatically — use `--bin` for that.

**Install to `~/bin` after build:**

```sh
./tools/build --bin
```

After a build, always use `./ego` (the freshly compiled binary in the project root, or `~/bin/ego` after `--bin`) to test. Never copy from `builds/` manually — that directory holds cross-compiled artifacts.

**Plain `go build`** works but injects `"developer build"` as the version string (no `-ldflags` injection).

The build script runs `go generate ./...` before compiling to regenerate any auto-generated source.

---

## Testing

### Go unit tests

```sh
go test ./...
```

Run with race detector:

```sh
go test -race ./...
```

Coverage report (opens in browser):

```sh
./tools/coverage.sh                                # all packages
./tools/coverage.sh ./internal/language/compiler/  # one package
```

### Ego language tests

Higher-level tests written in the Ego language itself, under `tests/`:

```sh
ego test tests/
ego test tests/flow/      # subset by subdirectory
ego test path/to/file.ego # single file
```

Tests use directives `@test`, `@assert`, `@fail`, `@error`, `@pass`. See `tests/README.md` for the full spec.

### Timeout guard for goroutine tests

When debugging tests that use Ego goroutines, `sync.WaitGroup`, channels, or other synchronization primitives, a misbehaving program can hang indefinitely. Use the `--timeout` option (placed immediately after `ego`, before the subcommand) to cap execution time:

```sh
ego --timeout 5s test tests/defer/waitgroups.ego
ego --timeout 10s run myprogram.ego
```

The value is a standard Go duration string (`5s`, `500ms`, `2m`, etc.). If the timeout elapses, Ego exits with OS code 99 and logs:

```text
INTERNAL: Ego execution time exceeded 5s; terminating program execution
```

Without `--timeout`, behavior is unchanged. Use this liberally when working on goroutine or channel code to avoid silent hangs during research.

### Running a bare code fragment with `ego run`

`ego run <file>.ego` normally requires a full `package main` / `func main()` wrapper. To quickly run a bare fragment (top-level statements, no package or function wrapper — the same style accepted by `ego test`'s `@test` blocks or the REPL) pipe it into `ego run` via stdin instead of passing a filename:

```sh
ego run --pprof /tmp/cpu.prof < fragment.ego
echo 'x := 1 + 2; fmt.Println(x)' | ego run
```

This is useful for quick one-off benchmarking/profiling snippets or scratch experiments where writing a full `package main` wrapper would just add noise.

---

## Key Package Structure

> **Layout note:** all Go packages except `main.go` itself now live under
> `internal/`. Older docs, comments, or memories that reference top-level
> paths like `compiler/`, `bytecode/`, `runtime/`, `errors/`, `defs/`,
> `server/`, `commands/`, or `app-cli/` are describing a **pre-refactor**
> layout — those directories no longer exist at the repository root. Use the
> table below (paths rooted at `internal/`) for anything written from this
> point forward. When in doubt, `find internal -maxdepth 2 -type d` is the
> fastest way to confirm current structure before trusting a remembered path.

| Directory | Purpose |
|: -------- |: ------ |
| `main.go` | Entry point (repository root); injects `BuildVersion` and `BuildTime` via linker flags |
| `internal/language/compiler/` | Ego language compiler |
| `internal/language/bytecode/` | Pseudo-instruction set and executor |
| `internal/language/tokenizer/` | Lexer/tokenizer |
| `internal/language/tokens/` | Token type definitions |
| `internal/language/symbols/` | Symbol table |
| `internal/language/data/` | Runtime data types |
| `internal/language/expressions/` | Expression evaluation support |
| `internal/language/debugger/` | Interactive debugger |
| `internal/runtime/` | Built-in runtime functions, one subpackage per Ego-visible package (e.g. `runtime/time`, `runtime/strconv`, `runtime/sql`, `runtime/tables`) |
| `internal/builtins/` | Built-in (always-available) function implementations |
| `internal/packages/` | Package loading and management |
| `internal/cli/` | CLI framework, split into `cli/app`, `cli/cli`, `cli/config`, `cli/parser`, `cli/settings`, `cli/tables`, `cli/ui` (formerly the top-level `app-cli/`) |
| `internal/grammar/` | CLI verb/option grammar definitions (e.g. `create-table`, `set`, `show`, `logon`) — see `internal/grammar/GRAMMAR.md` |
| `internal/commands/` | CLI sub-command handlers |
| `internal/server/` | REST server (admin, auth, cluster, dsns, oauth, services, tables, assets) |
| `internal/router/` | HTTP routing, rate limiting, and request logging middleware |
| `internal/resources/` | Struct-reflection-based DDL/CRUD framework used by the tables layer |
| `internal/dsns/` | Data source name (DSN) definitions and connection handling |
| `internal/caches/` | In-memory expiring key/value cache used for server admin session state |
| `internal/errors/` | Error types and handling |
| `internal/defs/` | Shared constants and definitions |
| `internal/i18n/` | Internationalization / string lookup (`internal/i18n/languages/` holds `messages_en.txt` etc.) |
| `internal/util/` | Miscellaneous utilities: `util/fork` (subprocess spawn), `util/formats`, `util/javascript` (JS/CSS minifier for dashboard assets), `util/profiling`, `util/validate` |
| `tests/` | Ego-language unit tests |
| `tools/` | Build, test, and utility scripts |
| `docs/` | User-facing documentation |
| `docs/issues/` | Bug and behavioral-issue tracking documents (`BUGS.md`, `FUNCTIONAL_ISSUES.md`, `BYTECODE_ISSUES.md`, etc.) |

---

## Commit Conventions

Commits follow the conventional-commits style as seen in recent history:

- `feat:` — new features
- `fix:` — bug fixes
- `chore:` — maintenance, comments, refactoring, tests
- Messages are lowercase after the prefix

---

## Logging

Enable diagnostic loggers with `--log <class,...>` before any sub-command:

```sh
ego --log trace,symbols run myprogram.ego
```

Available classes: `AUTH`, `BYTECODE`, `CLI`, `COMPILER`, `DB`, `REST`, `SERVER`, `SYMBOLS`, `TABLES`, `TRACE`, `USER`.

### Server log files

When running as a server, ego writes structured log files to the project (working) directory. File names follow the pattern:

```text
ego-server_YYYY-MM-DD-NNNNNN.log
```

The file is **newline-delimited JSON** (one JSON object per line, not a JSON array). Parse with:

```python
import json
entries = [json.loads(line) for line in open("ego-server_....log") if line.strip()]
```

Each entry has these fields:

| Field | Description |
| :---- | :---------- |
| `time` | Timestamp (`"2026-05-03 09:55:20"`) |
| `id` | Server instance UUID (stable for the life of the process) |
| `seq` | Global sequence number — monotonically increasing across all sessions |
| `session` | Per-request session number (matches the session ID in REST logs) |
| `class` | Logger class (`"server"`, `"rest"`, `"admin"`, etc.) |
| `msg` | Localization key (e.g. `"log.server.request"`) — look up in `internal/i18n/languages/messages_en.txt` under the `[log]` section to read the human-readable template |
| `args` | Object containing named arguments substituted into the message template |

Common `msg` values and their `args`:

- `log.rest.request.payload` — `args.body` is the raw request JSON sent to an endpoint
- `log.rest.response.payload` — `args.body` is the raw response JSON returned to the caller
- `log.server.request` — `args.path`, `args.method`, `args.status`, `args.elapsed`, `args.user`, `args.length`, `args.type`
- `log.admin.run.session.created` — `args.id` is the new code-session UUID
- `log.server.auth.failed` — login attempt rejected

The `elapsed` field in `log.server.request` is the **total** server-side time for the request (auth + routing + handler + response). The `elapsed` field inside the `log.rest.response.payload` body (for `/admin/run`) is just the Ego execution time, which is typically much shorter.

---

## Configuration / Preferences

Preferences stored in `~/.ego/` as JSON profile files. Set via:

```sh
ego config set ego.runtime.path=/home/tom/ego
```

Notable settings:

- `ego.compiler.extensions` — allow language extensions (default `false`)
- `ego.compiler.import` — auto-import all built-in packages (default `false`)
- `ego.compiler.types` — `dynamic` (default) or `static` type checking
- `ego.runtime.path` — where Ego looks for its `lib/` directory

---

## REST Server / Dashboard

### Starting and stopping the server

Start as a detached background server (HTTPS, uses TLS certs from `~/.ego/`):

```sh
ego server start
```

Start without TLS — plain HTTP, no certificate required (useful for local testing):

```sh
ego server start -k
```

Stop, restart:

```sh
ego server stop
ego server restart
```

The server writes its PID to `~/.ego/ego-server.pid` and its log to
`ego-server.log` in the project directory (configurable via
`ego.server.default.log.file`). Use `ego show config` to inspect all active
settings, including `ego.logon.server` (the URL apitest will connect to) and
`ego.server.insecure` (true when running with `-k`).

To run the server in-process (blocking — useful for attaching a debugger):

```sh
ego server
```

### Default server URL for API tests

The `apitest` tool defaults to `SCHEME=https` and `HOST=<hostname>.local`
(macOS mDNS name), with no `PORT` — meaning port 443. Override any of these
with `-x KEY=VALUE` on the command line:

```sh
# Run tests against a local HTTP server on port 4040
tools/apitest.sh -x SCHEME=http -x PORT=4040
```

### Web applications

#### Dashboard

A web dashboard is available. The full dashboard specification is in `docs/DASHBOARD.md`.

The Status tab of the dashboard calls `GET /admin/resources` (a single endpoint returning `defs.StatusResponse`) rather than the older two-call approach using `/admin/memory` and `/admin/caches` separately. The handler is `internal/server/admin/status.go:GetResourcesHandler`. The Content-Type for this response is `defs.ResourcesMediaType`.

Dashboard JS and CSS live under `lib/assets/dashboard/`. The Status tab uses an 8-column stat-grid HTML table structure (`label | value | sep | label | value | sep | label | value`) for both the Metrics and Cache Status sections; CSS `min-width` on `.stat-lbl` and `.stat-val` keeps columns visually aligned.

---

## Adding Error Messages

Errors are defined in `internal/errors/messages.go` as package-level variables:

```go
var ErrMyThing = Message("my.thing.key")
```

The key maps to a localized string in the `[error]` section of each language file under `internal/i18n/languages/`. **Error messages must not contain `{{substitution}}` parameters** — variable context is attached at the call site with `.Context(value)`, which appends `: <value>` to the rendered message:

```go
return errors.ErrMyThing.Context(filename)  // → "my error message: /path/to/file"
```

After adding the variable to `internal/errors/messages.go`, add the corresponding key to all three localization files (`messages_en.txt`, `messages_fr.txt`, `messages_es.txt`) inside the `[error]` section, keeping entries in alphabetical order within their group.

### Never use fmt.Errorf with literal text

**Do not use `fmt.Errorf("literal text")` or `errors.New("literal text")` in non-test code.** Every new error that surfaces to the caller must be defined in `internal/errors/messages.go` with a localization key. Use `errors.New(errors.ErrXxx)` (creating a clone with no context) or `errors.New(errors.ErrXxx).Context(...)` (attaching runtime detail).

Pattern for wrapping a Go error with context:
```go
// Before (wrong — literal text, not localized):
return fmt.Errorf("fetching JWKS from %s: %w", url, err)

// After (correct — localized base message, dynamic detail in context):
return errors.New(errors.ErrJWKSFetch).Context(fmt.Sprintf("%s: %v", url, err))
```

Pattern when the called function already returns an Ego error (no re-wrapping needed):
```go
if err := fetchSomething(); err != nil {
    return err  // already an Ego error — pass through directly
}
```

### Replacing wrapping fmt.Errorf calls

When converting a chain like `fmt.Errorf("X: %w", err)` where the callee (`err`) is already converted to return Ego errors, just pass the error through:
```go
// Before:
if err := doThing(); err != nil { return fmt.Errorf("doing thing: %w", err) }

// After (callee already returns Ego errors):
if err := doThing(); err != nil { return err }
```

### Tests that check error message text

When a test asserts on the string content of an error (e.g., `strings.Contains(err.Error(), "exceeds")`), update it to use `errors.Equals(err, errors.ErrXxx)` instead. The Ego error system formats errors via i18n lookups, so literal English substring checks are fragile once error constants are used.

```go
// Before:
if !strings.Contains(err.Error(), "exceeds") { t.Errorf(...) }

// After:
if !errors.Equals(err, errors.ErrJWKSSizeLimit) { t.Errorf(...) }
```

---

## Adding Configuration Settings

Settings (preferences) are defined as string constants in `internal/defs/config.go` and must be added to the `ValidSettings` map in the same file to be user-settable.

**Every new setting constant must also have a matching localization entry** in all three language files (`messages_en.txt`, `messages_fr.txt`, `messages_es.txt`). The entry goes in the `[config.ego]` section. The section prefix `ego.` is handled by the block header, so only the remainder of the key is used. For example, a constant with value `"ego.server.read.timeout"` gets the entry key `server.read.timeout`.

Keep entries in approximate alphabetical order within their prefix group (e.g., all `server.*` entries together). The description should explain what values are accepted (e.g., "Must be a Go duration string") and what the default is.

---

## API Tests (`tools/apitest/`)

Tests are JSON files under `tools/apitest/tests/`. Files execute in alphabetical order within each group directory. Test groups are subdirectories with numeric prefixes (e.g., `4-dsns/`).

### File structure

```json
{
    "description": "human-readable description",
    "request": {
        "method": "PUT|GET|POST|DELETE",
        "endpoint": "/path/with/{{VARIABLE}}",
        "headers": { "Content-Type": ["application/json"], "Authorization": ["Bearer {{API_TOKEN}}"] },
        "body": <any JSON value>,
        "parameters": { "key": "value" }
    },
    "response": {
        "status": 200,
        "headers": { "Content-Type": ["substring-to-match"] },
        "save": { "MY_VAR": "dot.path.into.response" }
    },
    "tests": [
        { "name": "label", "query": "dot.path", "value": "expected string" },
        { "name": "label", "query": "dot.path", "op": "contains", "value": "substring" }
    ]
}
```

### Key conventions

- **Variables**: `{{VAR}}` is substituted from `dictionary.json`. `"$hash"` generates a random hex string per run — use this for unique names. Standard variables: `API_TOKEN`, `SERVER_ID`, `DSNUUID`, `SQLUUID`, `USERUUID`, `ROOT`.
- **Content-Type matching**: The `headers` check is a substring match. Use `"rowcount+json"`, `"rows+json"`, `"error+json"` rather than the full `application/vnd.ego.*` type.
- **Response field paths**: Use dot notation. `server.api` → `"1"`, `server.id` → `"{{SERVER_ID}}"`, `count` → row/affected count, `rows.N.column` → Nth row value, `msg` → error message.
- **Saved values**: `"save"` in `response` captures response fields into variables for use in later tests via `{{MY_VAR}}`.
- **Default comparison** is exact string match; use `"op": "contains"` for partial matches (error messages, etc.).
- **Number comparison**: Integer and float JSON values are compared as strings. Integers compare as `"42"`, TEXT columns as `"Alice"`. Avoid asserting on float columns — JSON encoding of `float64` may omit the decimal for whole numbers (`72.0` → `"72"`).

### Running the API test suite

`tools/apitest.sh` changes to `$(ego path)/tools/apitest/` and runs `go run .`
against the `tests/` directory. A `dictionary.json` in each test directory is
loaded automatically — no `-d` flag needed for the standard run.

Run all tests:

```sh
tools/apitest.sh
```

Run a subset. The `1-logon` group **must always be included first** — it hits
`GET /services/up` to capture `SERVER_ID` and `POST /services/admin/logon` to
capture `API_TOKEN`; without these, every subsequent test that uses
`{{SERVER_ID}}` or `{{API_TOKEN}}` will fail or be skipped. If the server is
not reachable, `logon-01-up.json` (which has `"abort": true`) terminates the
suite immediately rather than spewing false failures.

```sh
tools/apitest.sh tests/1-logon tests/4-dsns
```

Useful flags (passed through to the `go run .` invocation):

| Flag | Purpose |
| ---- | ------- |
| `-x KEY=VALUE` | Override a dictionary entry (e.g. `-x SCHEME=http -x PORT=4040`) |
| `-v` / `--verbose` | Print each request URL as it runs |
| `-r` / `--rest` | Dump full request and response bodies (debug failures) |
| `-f PATTERN` / `--filter PATTERN` | Run only tests whose description matches the pattern |

### `abort` field

A test file may include `"abort": true` at the top level. If that test fails,
the entire suite stops immediately. Use this on liveness probes (like
`logon-01-up.json`) to avoid running hundreds of tests against a server that
isn't running.

### `@sql` endpoint

`PUT /dsns/{dsn}/tables/@sql` — body is a JSON array of SQL strings. Admin-only.

- All statements execute as a single transaction.
- A SELECT may only appear as the **last** statement; a SELECT in any other position returns 400.
- Last statement is non-SELECT → `rowcount+json` response (`count` = rows affected by last statement).
- Last statement is SELECT → `rows+json` response (`rows`, `count` = number of rows returned).
- SQL errors return 500 with `error+json`; the `msg` field starts with `"Error in SQL execute; "`.
- Table names derived from `{{SQLUUID}}` must be prefixed with a letter (e.g., `t{{SQLUUID}}`) since `$hash` can start with a digit.
- Index names must be unique in the database; use a name tied to the table UUID (e.g., `idx_t{{SQLUUID}}`).

### `@transaction` endpoint and the schema cache

`POST /dsns/{dsn}/tables/@transaction` — body is a JSON array of `defs.TXOperation` objects. Handler is `internal/server/tables/scripting/handler.go`.

Key behaviors:

- All operations share a per-transaction symbol table. Earlier operations can produce values (via `select`) that later operations consume via `\{{symbol}}` substitution.
- `_rows_` — rows affected by the current operation; `_all_rows_` — cumulative rows for the transaction. Both are available in user-defined `errors` condition expressions.
- The `insert` opcode always counts as `_rows_ == 1` on success (one row per call).
- **Schema cache:** `internal/server/tables/tables.go` maintains a `caches.SchemaCache` keyed by `user/dsn/tableName`. It is populated by `GET .../rows` (and other REST row-access endpoints) and must be flushed after DDL. The `@transaction` handler calls `caches.Purge(caches.SchemaCache)` after a successful commit when `needCacheFlush` is true. `needCacheFlush` is set by:
  - `sql` opcode containing `ALTER TABLE` or `DROP TABLE`
  - `drop` opcode (on success)

  Without a flush, a stale schema entry causes `GET .../rows?columns=newCol` to return 400 "invalid column name" even after the column has been added or the table has been recreated.

---

## Runtime Function Conventions

Runtime packages under `internal/runtime/` expose Go functions to Ego programs. Two patterns co-exist:

### Native pass-through

The Ego runtime calls the real Go function directly with no wrapper code. In `types.go` the function is registered with `IsNative: true` and `Value: theGoFunc`. The runtime reflection layer handles argument unpacking and return-value packing automatically. Examples: `time.Now`, `time.Sleep`, `time.Parse`, `math.Sqrt`.

### Ego-implemented wrappers

When Ego-specific behavior is needed (extra arguments, custom error handling, units not in the Go standard, etc.) the package supplies its own Go function with the signature:

```go
func myFunc(s *symbols.SymbolTable, args data.List) (any, error)
```

Key conventions:

- **Receiver (method calls)**: the object the method is called on is stored in the symbol table under `defs.ThisVariable` (`"__this"`). Retrieve it with `s.Get(defs.ThisVariable)` and helpers like `data.GetNativeDuration` / `data.GetNativeTime`.
- **Multi-return results**: Go's `(T, error)` tuples are bundled into a `data.List` (`data.NewList(value, err)`) because the Ego dispatch layer expects a single `any` return. The Ego compiler unpacks the list at the call site.
- **Errors**: always return the error as both the second element of the result list _and_ as the naked `error` return of the wrapper function.
- **Extended duration parsing** (`util.ParseDuration`): extends `time.ParseDuration` with a `"d"` suffix for days. Strings without `"d"` are passed straight to the standard library. Spaces between units are permitted.
- **Extended duration formatting** (`util.FormatDuration`): with `extendedFormat=true`, outputs `"1d 6h 30m"` style (spaces, days explicit). Sub-second durations always fall back to Go's default formatter even in extended mode.

### `callRuntimeFunction` dispatch mechanics — catchable vs uncatchable errors

Understanding `internal/language/bytecode/callRuntimeFunction.go` is essential when writing or reviewing runtime wrappers:

**When the result IS a `data.List`** (line 61–73 of `callRuntimeFunction`):

- A `StackMarker("results")` is pushed first, then all list items in reverse order (index `Len()-1` first, index 0 on top).
- The Go second return (`error`) is **completely ignored** — `callRuntimeFunction` returns `nil` to the run loop.
- Errors must be **inside the list** to be visible to Ego code.
- This is the correct path for all multi-return and error-returning wrappers.

**When the result is NOT a `data.List`** (lines 75–87):

- If `err != nil`: `c.runtimeError(err)` is called → **uncatchable runtime abort** that bypasses `try/catch` and does NOT appear as a returned value.
- If `err == nil`: the result is pushed onto the stack as a single value.
- Exception: a function with exactly one `error`-typed return in its declaration takes a special path in `functionReturnedValueAndError` — `err` is returned to the run loop (catchable via `try/catch`) but **does not** become the Ego-level return value.

**Correct return pattern for error-returning wrappers:**

```go
// Success (single value + no error):
return data.NewList(value, nil), nil

// Success (error-only return):
return data.NewList(nil), nil

// Error (single value + error):
return data.NewList(nil, myErr), nil   // nil for Go second return — it IS ignored for lists

// Error (error-only return):
return data.NewList(myErr), nil        // 1-item list, nil Go return
```

**Common bugs caused by getting this wrong:**

1. `return value, err` (no `data.List`) — if `err != nil`, triggers an uncatchable runtime abort. Ego code cannot catch it with `try/catch` or read it as a return value.
2. `return data.NewList(nil, err), err` for a 1-return (error-only) function — creates a 2-item list; the top item is `nil`, so `e := f()` assigns nil. The actual error is an unread list item (detected as a warning by the "unread errors" extension).
3. Mismatching the number of list items to the declared `Returns` count — can leave stale items on the stack, corrupting subsequent operations.

### Native pass-through limitations

Native pass-through (`IsNative: true`) works for scalar and struct types but **does not work** for functions that accept Go slice types (`[]int`, `[]float64`, etc.) because the Ego runtime represents arrays as `*data.Array`, not native Go slices. All sort functions and any function taking a typed slice need Ego wrappers.

### Invoking an Ego function callback from Go

When a runtime wrapper needs to call back into Ego code (e.g., `sort.Slice`, `sort.Search`), the pattern is:

```go
callbackSymbols := symbols.NewChildSymbolTable("label", s)
if fn.Name() == "" { fn.SetName(defs.Anon) }
ctx := bytecode.NewContext(callbackSymbols, fn)
// Inside Go's sort.Slice / sort.Search / etc.:
callbackSymbols.SetAlways(defs.ArgumentListVariable,
    data.NewArrayFromInterfaces(data.IntType, arg1, arg2))
if err := ctx.Run(); err != nil { ... }
result := data.BoolOrFalse(ctx.Result())
```

The function's declaration entry in `types.go` must set `Scope: true` so the Ego closure can access variables from its enclosing scope.

### `data.Array` internals relevant to sort

- `array.BaseArray() []any` — returns the internal `[]any` slice for non-byte arrays; sorting it in-place (via `sort.Slice`/`sort.SliceStable`) modifies the array. For byte arrays it returns a **new** `[]any` copy — do not use for in-place byte sorting.
- `array.GetBytes() []byte` — for byte arrays returns the actual internal `[]byte`; sorting this in-place works correctly.
- `array.Sort()` — uses Go's unstable sort internally. Use `sort.SliceStable` on `BaseArray()` (or `GetBytes()` for bytes) when a stable sort is needed.
- `array.Type().Kind()` — returns a `data.XxxKind` constant for dispatch in type-switch comparators.

### Declaration type for sort comparator functions

Go's `sort.Slice` / `sort.SliceStable` comparator is `func(i, j int) bool`. The Ego declaration must match:

```go
Type: data.FunctionType(&data.Function{
    Declaration: &data.Declaration{
        Parameters: []data.Parameter{
            {Name: "i", Type: data.IntType},
            {Name: "j", Type: data.IntType},
        },
        Returns: []*data.Type{data.BoolType},
    },
}),
```

Do **not** add a `data` array parameter — the implementation only passes `(i, j)` and the array is captured via closure.

### Testing wrappers

Tests live in the same package (e.g., `internal/runtime/time/time_test.go`), giving direct access to unexported helpers. Set up a method receiver in tests like this:

```go
st := symbols.NewSymbolTable("test")
st.SetAlways(defs.ThisVariable, myDurationOrTimeValue)
result, err := myWrapperFunc(st, data.NewList(arg1, arg2))
```

Result lists are cast to `data.List` and inspected with `.Get(0)` / `.Get(1)`.

---

## Error Handling — Ego Language

### `@error` and `@fail` directives

- **`@error "msg"`** — emits a `Signal` bytecode: pops the value from the stack, wraps it as a runtime error with location info, and returns it as a catchable error. `try/catch` can handle it. `e.Is(errors.New("msg"))` returns `true` inside the catch block.
- **`@error`** (no argument) — throws `errors.ErrPanic` (key `"panic"`) as a catchable error.
- **`@fail "msg"`** — unconditionally fails the test; uses `Panic` bytecode, which is NOT catchable by `try/catch`.

### `Signal` vs `Panic` vs `UserPanic` bytecode

- **`Signal`**: returns a catchable error (the error propagates through the call stack and can be caught by `try/catch`). Used by `@error`.
- **`Panic`**: sets `c.running = false` (stops execution entirely). NOT caught by `try/catch`. Used by `@fail` and truly unrecoverable situations.
- **`UserPanic`**: emitted by the Ego `panic()` built-in. Starts a recoverable unwind: sets `c.panicActive = true`, returns `ErrPanicActive`, which bypasses `try/catch` and triggers `unwindPanic()` in the run loop. A deferred `recover()` can intercept it; if nothing recovers it, it prints a fatal message and stops.

### `panic()` / `recover()` — implementation notes

`panic()` and `recover()` are standard built-ins (no extension flag required). Key design facts:

- **Single-context execution**: all Ego function calls run in ONE `bytecode.Context` with a frame stack embedded in the runtime stack. Deferred functions run in separate child contexts (`NewContext(s, cb)`) but via a pointer chain.
- **`panicContext` field**: child contexts created during panic unwinding have `cx.panicContext = c` (the panicking parent). `recover()` walks this chain via `findPanickingContext()` to find and clear the panic state.
- **`tryDepth` in CallFrame**: the length of `c.tryStack` is saved in each `CallFrame` and restored on pop, so stale try entries created inside a panicking function are discarded during unwind.
- **Deferred functions run once**: `unwindPanic()` clears `c.deferStack` after successful recovery so the `RunDefers` opcode (compiled by `Return`) does not re-execute them.
- **`RunDefers` must only be emitted once per function**: `compileFunctionDefinition` in `internal/language/compiler/function.go` calls `compileRequiredBlock(true)`, which emits `RunDefers + PopScope` at the end of the block. Do NOT emit a second `RunDefers` after this call — doing so causes deferred functions to execute twice, triggering bugs like `sync: negative WaitGroup counter` and `close of closed channel`.
- **Frame pop after recovery**: after `recover()` clears the panic, `unwindPanic()` calls `callFramePop()` to return execution to the caller of the panicking function. The run loop then continues normally in the caller.
- **`recover()` as an expression**: handled in `internal/language/compiler/expr_atom.go` by recognizing `RecoverToken` and emitting the `Recover` opcode.
- **`recover()` as a statement**: handled in `internal/language/compiler/statement.go` — emits `Recover` + `Drop` to discard the return value.
- **`panic` and `recover` are reserved words**: both are in `tokenizer.ReservedWords` (not `ExtendedReservedWords`), so they are always lexed as reserved tokens regardless of the extensions setting.

### `try/catch` — catch variable scoping

`catch(e)` creates a **new** local variable `e` scoped to the catch block only. It does NOT assign to an outer variable of the same name. To preserve the error beyond the catch block, declare the variable in outer scope and assign inside:

```ego
var captured error
try {
    _ = 5 / 0
} catch(e) {
    captured = e   // explicit assignment needed to escape the scope
}
// 'captured' holds the error; 'e' is gone
```

If you do not need to use the error value, use anonymous `catch {}` to avoid "variable created but never used" errors in strict mode.

### Error method declarations — use plain types, not pointer types

When declaring method parameters in `data.ErrorType.DefineFunction(...)`, use plain types:

- `data.StringType` (not `data.PointerType(data.StringType)`)
- `data.IntType` (not `data.PointerType(data.IntType)`)
- `data.InterfaceType` (not `data.PointerType(data.InterfaceType)`)

Using `PointerType` wrappers causes strict-mode failures: "incorrect function argument type: argument 1: string".

### Known limitations

- **`Unwrap()` with no context returns `""`**, not `nil`.
- **Optional operator `?pkg.Member : fallback`** does NOT catch package member access errors in that syntactic position. The error propagates instead. Stick to `?expr : fallback` where expr is a computation, not a bare package-member lookup.

### `errors.New` and `Is()` matching

`errors.New("key")` looks up the key in the i18n table. `Is()` compares by the underlying key string, not the translated text. Two errors created with the same key always match via `Is()`, regardless of attached context.

---

## Writing Ego Tests (`tests/`)

Tests live under `tests/` in `.ego` files, organized by subdirectory. Run with:

```sh
ego test tests/           # all tests
ego test tests/math/      # one subdirectory
ego test tests/math/trig.ego  # single file
```

### Test file structure

```ego
@test "package: description of test"

{
    // helper functions and local vars go here
    @assert someExpr
}
```

A single file may contain multiple `@test` blocks. Variables declared outside a `{}` block are shared across blocks in the same file — use unique names to avoid collisions.

### Auto-imported packages — do not add explicit imports in `tests/*.ego`

`ego test` (`internal/commands/test.go`) always calls `comp.AutoImport(true, symbolTable)` — the "import all" variant — for every test compilation, regardless of the `ego.compiler.import` setting (which defaults to `false` and only affects `ego run`). This means `.ego` files under `tests/` can reference the following packages with **no `import` statement**:

```text
base64, cipher, errors, exec, filepath, fmt, io, json, math, os,
profile, reflect, rest, sort, sql, strconv, strings, tables, time, util, uuid
```

For example, `tests/errors/throw.ego` and `tests/math/aggregate.ego` call `errors.New(...)` with no `import "errors"` — this is not an oversight, the package is already in scope. Don't add an import for anything in this list when writing a new test file; it's just noise.

This does **not** apply to library source under `lib/packages/*.ego` (e.g. `lib/packages/math/primes.ego`, `lib/packages/http/http.ego`). Those are compiled via the normal import path and must explicitly `import` any package they use, except the small `requiredPackages` set (`os`, `cipher`, `profile`) which are always injected everywhere. It also doesn't apply to `ego run` on a plain script, which respects `ego.compiler.import` (default `false`) unless overridden with `--import` or the equivalent config setting.

### Directives

| Directive | Behavior |
| --------- | -------- |
| `@test "name"` | Starts a named test; creates the `testing` package |
| `@assert expr` | Fails (fatally) if `expr` is not `true`; uses expression source text as the error message |
| `@fail "msg"` | Unconditionally fails (fatal, not catchable) |
| `@error "msg"` | Generates a non-fatal runtime error (catchable in `try/catch`) |
| `@pass` | Records a pass if no errors have occurred |
| `@optimizer on` | Enables the optimizer for subsequently compiled code |
| `@optimizer off` | Disables the optimizer for subsequently compiled code |
| `@compile [flags] { code } [catch [(e)] {...}]` | Compiles `code` in an isolated sub-compiler; see below |
| `@capture var := { code }` | Runs `code` and captures its console output into `var`; see below |

`ego test` always runs with the optimizer off (it incurs a startup cost that outweighs any benefit for single-use test programs). Use `@optimizer on` in a test file only when testing optimizer-specific behavior. The `@optimizer` state persists until changed again or until compilation ends.

**`@assert` takes exactly one boolean expression — no second "message" argument.** The expression source text becomes the error message automatically.

**`@test` name constraints:** The descriptive string must be no longer than 48 characters and must contain only ASCII characters — no em-dashes, smart quotes, or other Unicode. Use a plain hyphen or comma instead of an em-dash.

### `@compile` — writing tests that exercise compile errors

`@compile [flags] { code }` compiles `code` in an isolated sub-compiler and splices the result inline on success. Flags (any combination, any order, all optional): `block` (bare — `code` is a statement block, not a full program), `unused=true|false`, `unknown=true|false`, `optimize=off|false|low|high|0|1|2`, `bytecode`/`disasm` (bare — print a disassembly of `code`'s bytecode once it compiles, regardless of `--log bytecode`), `eof="marker"` (delimit `code` with a text marker instead of `{ }`, so a test can exercise intentionally mismatched braces).

**As of July 2026, if `code` fails to compile and no `catch` clause is present, the compile error is signalled (raised) at runtime instead of being silently discarded.** If you are deliberately compiling broken code and don't care about the error, you must write an explicit empty catch block:

```ego
@compile block {
    pring "Hello"     // typo — invalid verb
} catch {}            // required to swallow the error silently
```

Without the empty `catch {}`, the above raises a catchable runtime error (which will fail the test unless wrapped in an outer `try`/`catch`, or crash `ego run`). To inspect the error instead of swallowing it, use `catch(e) { ... }` as usual — see the existing examples in `tests/directives/compile.ego`.

**`ByteCode.Disasm(force bool, ranges ...int)`** is what the `bytecode`/`disasm` flag calls. By default (`force=false`) it only prints when the `ui.ByteCodeLogger` class is active (`--log bytecode`) — this is the fast path used everywhere else in the compiler and is called constantly, so it must stay cheap when the logger is off. `force=true` bypasses that check by using `ui.WriteLog` instead of `ui.Log`, **not** by toggling `ui.Active(ui.ByteCodeLogger, ...)` — that flag is process-global and shared by every goroutine, so flipping it would race with any other goroutine concurrently compiling or disassembling code. If you add another caller that wants unconditional output, pass `force=true`; do not reach for the toggle-the-global-flag pattern even though `internal/language/bytecode/optimizer.go`'s `Patch` function still does exactly that (pre-existing, not yet fixed — don't copy it).

### Common gotchas

1. **`< -value` tokenizes as `<-`** (channel receive operator) even with whitespace between `<` and `-`. Wrap negative literals in parentheses: use `@assert x < (-1.5)` not `@assert x < -1.5`.

2. **Float comparisons need a tolerance helper.** Define `near` inside each test block (it is locally scoped):

   ```ego
   func near(a, b float64) bool {
       return math.Abs(a-b) < 1e-9
   }
   ```

3. **Package extensions live in `lib/packages/{pkg}/*.ego`**, not in `internal/runtime/`. For `math`, the extra constants and functions (`Pi`, `E`, `Phi`, `Sqrt2`, `SqrtE`, `SqrtPi`, `Ln2`, `Ln10`, `Log2E`, `Log10E`, `MaxInt`, `MinInt`, fixed-width int bounds, float bounds, `Factor`, `Primes`) are defined in `lib/packages/math/`.

4. **`var f func() int` is valid Ego syntax** (fixed BUG-70, July 2026) — both the parameterless form and a parameterized one, in either Go's named (`func(a, b int) int`) or type-only unnamed (`func(int, int) int`) spelling, all now work as a `var` declaration's type, a struct/interface field type, or a type-assertion target (`x.(func(int, int) int)`):

   ```ego
   var f func(int, int) int
   f = func(a, b int) int { return a + b }   // correct, and has always been

   g := func() int { return 0 }               // also still fine
   ```

5. **Type aliases create distinct types in strict mode.** In strict mode, `type FuncType func() int` and `func() int` are not interchangeable. Appending a `func() int` literal to a `[]FuncType` slice fails with `"wrong array value type"`. Either avoid the alias or cast explicitly. Tests that must pass in both modes should avoid typed-slice patterns for function types.

6. **Channels use no type argument in `make`.** Ego syntax is `make(chan, N)`, not `make(chan int, N)`. There is no per-channel element type in Ego channels.

### Writing tests that pass in both `--types strict` and `--types dynamic`

The `ego test` command accepts `--types strict|relaxed|dynamic`. Tests should ideally pass in both `strict` and `dynamic` modes. The key difference: in strict mode, `==` between values of different types (e.g., `int8` vs `int`) is a type error.

**Rule: never compare a narrow-type cast result directly to an untyped integer literal.** Wrap the result in `int()` to normalize for comparison:

```ego
@assert int(int32(byte(42))) == 42   // correct: both sides are int
@assert int32(byte(42)) == 42        // FAILS in strict: int32 vs int
```

The same applies to `byte` array indexing — `arr[i]` returns `byte`, so wrap it:

```ego
@assert int(b[0]) == 65    // correct
@assert b[0] == 65         // FAILS in strict: byte vs int
```

For `float32`, compare to an explicit `float32()` literal rather than a plain float:

```ego
@assert float32(x) == float32(1.0)   // correct
@assert float32(x) == 1.0            // FAILS in strict: float32 vs float64
```

### `ego.runtime.precision.error` setting

The `ego.runtime.precision.error` preference (default `false`) controls whether float-to-integer conversions that lose a fractional part throw a runtime error. When this setting is `true`, `int64(3.9)` errors instead of silently truncating to `3`.

- `coerceToInt` (used for plain `int()` casts) does **not** check fractional precision regardless of the setting — it only checks overflow.
- `coerceToInt64`, `coerceToInt32`, etc. **do** check fractional precision when the setting is enabled.

Tests that cast fractional floats to integers should use `int()` rather than `int64()` to remain independent of the precision-error setting.

---

## Known Ego vs Go Behavioral Differences

These are confirmed differences between Ego and Go discovered during testing.
All are documented with test cases; open items are tracked in `docs/FUNCTIONAL_ISSUES.md`.

### `defer` statement

- **`defer` runs while the function scope is still live.** The `RunDefers` opcode executes at the end of the function body block, before `PopScope`, so deferred closures can still access local variables. This is correct and matches Go.
- **Named return variables are modifiable by deferred closures** — this now works correctly (single and multiple named returns both supported).
- **Named return with explicit value expression (`return someValue`) now works.** The compiler assigns the expression to the named return variable(s) and proceeds as a bare return. Deferred functions run after the assignment and can observe or modify the value. (Fixed May 2026, FUNC-7.)

### Functions and closures

- **Variadic functions accept zero variadic arguments.** `func f(args... int)` called as `f()` is valid; `args` is an empty slice and range loops simply do not execute. This matches Go. (Fixed May 2026, FUNC-1.)
- **Closures stored during a loop survive after the loop ends.** `pushByteCode` clones each literal `*ByteCode` and stamps the current symbol table onto the clone as `capturedScope`; `callBytecodeFunction` uses that captured scope as the closure's parent table, keeping the captured variables alive on the Go heap. Matches Go's escape-analysis behavior. (Fixed May 2026, FUNC-2.)
- **Nested named functions cannot reference the enclosing named function's parameters or locals** — doing so is now a compile-time error: `"nested named function cannot access enclosing function variable; use a closure"`. Only anonymous function literals (closures) capture the enclosing scope. (Fixed May 2026, FUNC-3.)
- **Value receiver methods can be called on a pointer variable.** `getThisByteCode` auto-dereferences `*any` pointers before dispatching, consistent with Go's method set rules. The reverse (pointer receiver on value variable) also works via auto-addressing. (Fixed May 2026, FUNC-4.)

### Type coercion

- **Dynamic mode now coerces scalar arguments at call boundaries.** Passing `"5"` to an `int` parameter coerces the value to `5` via `data.Coerce`. Non-coercible mismatches (e.g. `"abc"` for `int`) raise a catchable runtime error. Strict mode rejects mismatches earlier, at type-check time. (Fixed May 2026, FUNC-5.)
- **String multiplication (`*`) no longer produces repetition.** The `"A" * 3` → `"AAA"` special case was removed from `internal/language/bytecode/math.go`. Both `"A" * 3` and `3 * "A"` now produce a runtime error, consistent with Go. Use `strings.Repeat` for intentional repetition. (Fixed May 2026, FUNC-6.)

### `nil` through `interface{}` parameters

- **`nil` passed to an `interface{}` parameter does not compare equal to `nil` inside the function.** The value survives the call but `v == nil` evaluates to `false` inside the body. Do not rely on nil-interface checks for `interface{}` parameters.

### Flow control

- **`for range` over an array allows element mutation.** Previously `rangeInitByteCode` set the array immutable; a `break` inside the loop would leave it permanently immutable. Both calls to `SetReadonly` were removed — Ego arrays are fixed-size so there is no structural-modification hazard. (Fixed May 2026, FLOW-H1.)
- **`for` init clause accepts `=` as well as `:=`.** Pre-declared variables can now be used as the loop index: `var i int; for i = 0; i < 10; i++ { ... }`. (Fixed May 2026, FLOW-M1.)
- **Multi-value `case` clauses are supported.** `case "Sat", "Sun":` compiles into an `Equal`+`Or` chain, matching Go's semantics. (Fixed May 2026, FLOW-M2.)
- **`defer namedFunc(arg)` evaluates arguments lazily, not eagerly.** Unlike `defer func(v T){ namedFunc(v) }(arg)` (which captures at registration time), `defer namedFunc(arg)` re-reads `arg` from the symbol table when the deferred function runs. Workaround: use the closure form. (Open, FLOW-M4.)
- **Labeled `break` and `continue` are supported.** Place a label (`outer:`) before a `for` statement; `break outer` or `continue outer` targets that loop. Unknown labels produce a compile-time error. Only `for` loops may be labeled in Ego (matching Go's rules). (Fixed May 2026, FLOW-L1.)
- **`switch init; expr` semicolon form is supported.** Both `switch x := f(); x { ... }` (named init) and `switch f(); expr { ... }` (anonymous init) now compile. The init variable is in scope in case bodies. (Fixed May 2026, FLOW-L2.)

---

## Type Conversion (Cast) Behavior

Ego's type cast syntax `T(value)` calls `builtins.Cast`, which routes to `data.Coerce` for scalar types. Key behaviors:

- **`string(nil)`** → `""` (empty string).
- **`bool(nil)`** → `false`; **`bool("")`** → `false`; **`bool("1")`/`bool("t")`** → `true`; **`bool("0")`/`bool("f")`** → `false`. Matches `strconv.ParseBool` for single-char forms.
- **`int("")`** → `0` (empty string returns zero, not an error). Same for all integer types. Float types (`float32`, `float64`) do NOT have this exception — `float64("")` returns an error.
- **Float-to-integer truncation** is toward zero (Go semantics): `int(-3.9) == -3`, not `-4`.
- **`[]int("ABC")`** → `[]int{65, 66, 67}` (string to rune array). **`string([]int{65,66,67})`** → `"ABC"` (round-trips).
- **`[]byte("ABC")`** → byte slice of UTF-8 bytes. **`string([]byte{...})`** → string.

---

## Type Assertions and Function Type Specs (`internal/language/compiler/unwrap.go`, `type.go`, `typeCompiler.go`)

Fixed BUG-60/BUG-70/BUG-71 (July 2026); these notes cover the resulting architecture and its remaining known gap, so future work here doesn't have to re-derive them.

### `x.(T)` — two different operand shapes on the `UnWrap` bytecode

`compileUnwrap` tries two forms for the assertion target, in order:

1. **A single IDENTIFIER token** immediately followed by `)` — covers a plain type name (user-defined type, primitive like `int`, `any`, or the `type` keyword used by `switch v := x.(type)`). The raw token is passed as the `UnWrap` operand; `unwrapByteCode` (`internal/language/bytecode/types.go`) resolves it **by name** at runtime, first against `data.TypeDeclarations`, then the symbol table.
2. **Anything else** is retried via `c.parseType("", true)` — the same parser type-cast expressions (`T(value)`) use. This is what makes a compound target (pointer, slice, map, struct, `interface{}` literal, or function type like `func() int`) work as an assertion target. Since a compound type has no name to look up, the resolved `*data.Type` itself is passed as the `UnWrap` operand; `unwrapByteCode` checks for a `*data.Type` operand and uses it directly, bypassing the by-name lookup entirely.

Do not collapse these into a single path — the by-name lookup is what lets a user-defined type registered only in the runtime symbol table (not `c.types` at compile time) resolve correctly, and changing it risks re-breaking that.

**Known gap (BUG-71, not yet fixed):** the single-value form (`v := x.(T)`) only gets its bool-check-and-drop cleanup when it is the direct RHS of a `:=`/`=` assignment **statement** (wired up in `assignment.go` via `c.flags.hasUnwrap`). Used inline in a larger expression — `x.(int) + 1`, or calling the result immediately, `x.(func())()` — it leaves a stray boolean on the stack and corrupts whatever runs next. When writing or generating Ego code (tests, examples, docs), always assign a single-value assertion to a variable on its own line first; never chain it inline.

### `data.TypeOf()` can't type-switch on `*bytecode.ByteCode` — duck-type instead

`internal/language/data` cannot import `internal/language/bytecode` (bytecode already imports data — that would be a cycle). A compiled Ego function/closure value is a `*bytecode.ByteCode` at runtime, so `data.TypeOf()` used to fall through to its `default: return InterfaceType` case for every function value, misreporting it as `interface{}` (this also broke `reflect.TypeOf(f)`). Fixed by structurally detecting any value exposing `Declaration() *Declaration` — which `*bytecode.ByteCode` has — via `i.(interface{ Declaration() *Declaration })`, and building a `FunctionKind` type from it. If you ever need to recognize another `bytecode` package type from within `data`, use this same duck-typing pattern rather than trying to import `bytecode`.

### `parseTypeSpec` (var declarations) vs. `parseType` (everything else)

Two separate, non-shared type parsers exist. `parseType` (`typeCompiler.go`) is the general one — used by type casts, type assertions, struct/interface field types — and handles `func`, `struct`, `interface{...}`, `map`, arrays, pointers, primitives, and user types. `parseTypeSpec` (`type.go`) is a narrower parser used **only** by `var` declarations (`compileVar`), and historically only covered pointer/array/map/primitives/user-types — it had no `func` case at all until the BUG-70 fix added one that delegates to the same `ParseFunctionDeclaration` call `parseType` uses. **If a type form ever fails specifically in a `var` declaration while working fine in a cast or assertion, suspect `parseTypeSpec` is simply missing that case** — check it before assuming the bug is in the shared `parseType`/`ParseFunctionDeclaration` machinery. (`struct{...}` used directly as a `var`'s type is a known, separate, still-open gap: `var x struct { A int }` fails with an unrelated "unexpected token" parse error, not yet investigated.)

### `parseParameterDeclaration`'s `defineSymbols` flag — don't let a type spec's parameter names get "unused variable" errors

`parseParameterDeclaration(defineSymbols bool)` parses a function's parameter list and is reachable from two fundamentally different contexts: a **real function body** (`compileFunctionDefinition`, always passes `true` — a parameter the body never references should still be flagged unused) and a **bare type spec with no body at all** (every path through `ParseFunctionDeclaration`: struct/interface fields, type casts, type assertions, `parseTypeSpec`'s `func` case — all pass `false`). Passing `true` from a type-spec-only caller makes named parameters like `var f func(a, b int) int` falsely report `a`/`b` as unused, since nothing will ever reference them. When adding a new caller that parses a function type with no body, always pass `false`.

Unnamed parameter lists (`func(int, int) int`, Go's type-only form) are detected by `isUnnamedParameterList`, a read-only `Peek`-only lookahead in `function.go` — it never mutates tokenizer state, so it's safe to call speculatively before choosing which real parser to run.

---

## Style

- Use **American English** spellings in all comments and documentation (e.g., "color" not "colour", "recognize" not "recognise", "behavior" not "behaviour").

---

## Caches Package (`internal/caches/`)

In-memory key/value store with per-class expiration, used for server admin session state. Key types: `caches.SymbolTableCache` (default 1 h), `caches.DebugSessionCache` (default 15 m). API:

```go
caches.SetExpiration(caches.SymbolTableCache, "1h")
caches.Add(caches.SymbolTableCache, uuid, &myStruct{...})
if v, found := caches.Find(caches.SymbolTableCache, uuid); found {
    entry := v.(*myStruct)
    ...
}
caches.Delete(caches.DebugSessionCache, uuid)
```

Cache stores `any`; always type-assert on retrieval. Expiration and eviction are managed by the cache package itself — no manual cleanup goroutines needed.

---

## `data.Channel` — Go Channel Wrapper

`data.Channel` wraps `chan any` with a `sync.RWMutex` and `isOpen` flag. Key patterns:

- **`Receive()`** must use two-value receive internally (`datum, ok := <-c.channel`) so that a close-while-blocked wakeup returns `ErrChannelNotOpen` rather than `(nil, nil)`.
- **`Send()`** is protected by `defer recover()` against a closed-channel panic — that pattern is adequate.
- **`IsEmpty()`** returns `!c.isOpen && len(c.channel) == 0`; it is not atomic with a subsequent `Receive()` call (TOCTOU), so loop termination should rely on `Receive()` returning an error, not on `IsEmpty()` alone.

---

## `strconv` Package Notes

The Ego `strconv` package (`internal/runtime/strconv/`) wraps Go's standard library, plus two Ego extensions: `Itor` (int→Roman numeral) and `Rtoi` (Roman→int). Tests live in `tests/strconv/`.

### Known quirks and fixed bugs

- **`FormatInt` type fix (fixed)**: The declaration originally used `data.IntType` for the first parameter, but Go's `strconv.FormatInt` takes `int64`. This caused a `reflect: Call using int as type int64` panic at runtime. Fixed by changing to `data.Int64Type`. Callers must pass `int64(n)` explicitly.

- **`Itor`/`Rtoi` two-value return (fixed)**: These functions now return `(value, error)` via `data.List`, consistent with `Atoi`. Callers use `r, err := strconv.Itor(n)` and `n, err := strconv.Rtoi(s)`. The wrappers return `data.NewList(value, nil)` on success and `data.NewList(nil, err)` on failure, with the Go-level error always `nil`.

- **`FormatFloat` negative zero**: IEEE 754 negative zero (`-0.0` in source) is not preserved by Ego's arithmetic — it formats as `"0.0"` rather than `"-0.0"`.

### Added functions (native passthroughs)

The following Go `strconv` functions were added as native passthroughs. All live in `internal/runtime/strconv/types.go` and have corresponding tests in `tests/strconv/`:

- `CanBackquote(s string) bool`
- `FormatUint(i uint64, base int) string` — requires `uint64()` cast at call site
- `IsGraphic(r rune) bool` — declared with `data.Int32Type` (rune = int32)
- `IsPrint(r rune) bool` — declared with `data.Int32Type`
- `ParseBool(s string) (bool, error)`
- `ParseFloat(s string, bitSize int) (float64, error)`
- `ParseInt(s string, base int, bitSize int) (int64, error)` — returns `int64`
- `ParseUint(s string, base int, bitSize int) (uint64, error)` — returns `uint64`
- `QuoteRune(r rune) string` — declared with `data.Int32Type`
- `QuoteRuneToASCII(r rune) string` — declared with `data.Int32Type`
- `QuoteRuneToGraphic(r rune) string` — declared with `data.Int32Type`
- `QuoteToASCII(s string) string`
- `QuoteToGraphic(s string) string`

**Note on rune parameters**: Go's `rune` is a type alias for `int32`. Declare rune parameters with `data.Int32Type` and pass `int32(n)` at call sites in Ego tests.

**Note on `QuoteRune` test assertions**: `QuoteRune` returns a string like `'A'`. The lexer fix (see below) means `"'A'"` now correctly evaluates to the 3-char string `'A'`, so direct comparison works. The older concat workaround (`q := "'"` then `q + "A" + q`) still works too.

### `uint64` native passthrough support

`internal/language/bytecode/callNative.go:convertToNative` was updated to handle `data.UInt64Kind` (maps to Go's `uint64` via `data.UInt64`). This enables `FormatUint` and `ParseUint` as native passthroughs. `convertFromNative` already handled `uint64` via its `default` branch. Functions that take or return `uint64` should use `data.UInt64Type` in declarations.

### Character literal syntax in Ego

In Ego, `'x'` is a **character literal** of type `int32` with the Unicode code point of `x`:

```ego
a := 'A'          // int32(65)
b := "'A'"        // string of length 3: the characters ', A, '   (FIXED — see below)
c := "hello"      // string
```

**Lexer fix (applied)**: An earlier bug caused `"'a'"` (a double-quoted string containing single-quoted content) to be tokenized as a character literal `int32(97)` rather than the 3-char string `'a'`. This was fixed. Double-quoted strings containing single quotes now parse correctly. The old workaround — building the string via concatenation (`q := "'"` then `q + "a" + q`) — still works and is harmless, but is no longer required.

`string(byte(39))` returns `"39"` (decimal), NOT `"'"`. Use `"'"` directly for a literal single-quote string.

---

## Clustering (`internal/server/cluster/`)

The cluster package implements multi-node Ego server operation. Nodes that share the same
`--cluster <name>` flag and system database file form a named cluster.

### System table

The `cluster` table in the system database has one row per known node:

| Column | Type | Description |
| ------ | ---- | ----------- |
| `name` | TEXT | Cluster name from `--cluster` flag |
| `node_id` | TEXT PK | Server instance UUID |
| `host` | TEXT | Hostname or IP |
| `port` | INTEGER | TCP port |
| `scheme` | TEXT | `http` or `https` |
| `joined_at` | TEXT | RFC3339 join timestamp |
| `last_seen` | TEXT | RFC3339 last health-check timestamp |
| `state` | TEXT | `active` or `removed` |

### Package responsibilities

| File | Responsibility |
| ---- | -------------- |
| `cluster.go` | `Initialize()` — register node, open system DB, register OnPurge hook; `Shutdown()` — mark removed |
| `membership.go` | CRUD on the cluster table (`ListMembers`, `ListActiveMembers`, `upsertMember`, `RemoveMember`, `UpdateLastSeen`) |
| `health.go` | Background goroutine pinging peers every 30 s; evicts after 3 consecutive failures |
| `invalidate.go` | `BroadcastCacheFlush(cacheID)` — posts to every active peer; `SendCacheFlush(peer, cacheID)` — one peer HTTP POST |
| `auth.go` | `clusterToken()` — HMAC-SHA256 of `ego.server.token.key`; `ValidateClusterToken(r)` — constant-time check; `ClusterAuthHeader()` — exported for peer calls |
| `handlers.go` | REST handlers registered in `internal/commands/server.go`: GET /services/cluster (admin auth), POST /services/cluster/flush (cluster token), POST /services/cluster/shutdown (cluster token OR admin), POST /services/cluster/remove (cluster token OR admin) |

### Cache invalidation broadcast pattern

`caches.Purge(id)` calls `caches.OnPurge(id)` (registered by `cluster.Initialize` as `BroadcastCacheFlush`).
This means **every `caches.Purge()` call in the codebase automatically triggers a cluster broadcast in cluster mode**.
No per-call site changes are needed when adding new write operations.

To add a new write operation to the broadcast, just call `caches.Purge(caches.XxxCache)` as usual — the broadcast happens automatically.

### HMAC token derivation

```go
mac := hmac.New(sha256.New, []byte(settings.Get(defs.ServerTokenKeySetting)))
mac.Write([]byte("cluster:" + ClusterName))
token := "cluster-" + hex.EncodeToString(mac.Sum(nil))
```

All nodes in a cluster share the same `ego.server.token.key`, so any node can compute and verify the token independently. The `"cluster-"` prefix ensures it cannot be mistaken for a user session token.

### Health check goroutine

Started as `go cluster.StartHealthChecker()` from `internal/commands/server.go:RunServer()`. It:

- Sleeps for `ego.cluster.ping.interval` (default 30 s) between rounds
- GETs `/services/up` on each active peer with a `ego.cluster.ping.timeout` (default 5 s) timeout
- Tracks consecutive failures per `nodeID` in a local map
- Calls `evictPeer()` (→ `RemoveMember`) after `maxConsecutiveFailures = 3` misses (90 s total)
- Updates its own `last_seen` after each round so peers do not evict it

### Standalone mode

`cluster.Initialize(c)` returns immediately when the `--cluster` flag is absent. All cluster
functions are no-ops when `ClusterName == ""` or `systemDB == nil`. No configuration change is
needed for standalone servers.

---

## OAuth2 Authorization Server (`internal/server/oauth/authserver/`)

When `ego.server.oauth.as.enabled` is `true`, Ego registers a set of standard OIDC/OAuth2 endpoints at server startup and acts as an Authorization Server (AS) for development and testing environments. Phase 2 (Ego as a Resource Server accepting JWTs from an external IdP) is not yet implemented.

### Configuration settings

| Setting | Default | Description |
| ------- | ------- | ----------- |
| `ego.server.oauth.as.enabled` | `false` | Enable the AS role |
| `ego.server.oauth.as.issuer` | *(required)* | Base URL of this server; used as the `iss` JWT claim |
| `ego.server.oauth.as.key.file` | `{EGO_PATH}/lib/oauth/oauth-signing.pem` | EC private signing key; auto-generated on first start; chmod 0600 |
| `ego.server.oauth.as.clients` | `{EGO_PATH}/lib/oauth/oauth-clients.json` | Registered OAuth2 clients; chmod 0600 |
| `ego.server.oauth.as.token.expiration` | `1h` | Access token lifetime |
| `ego.server.oauth.as.refresh.expiration` | `24h` | Refresh token lifetime |
| `ego.server.oauth.as.code.expiration` | `5m` | Authorization code lifetime |

### Registered endpoints

| Method | Path | Handler | Notes |
| ------ | ---- | ------- | ----- |
| GET | `/.well-known/openid-configuration` | `DiscoveryHandler` | OIDC discovery doc (no auth) |
| GET | `/.well-known/jwks.json` | `JWKSHandler` | Public signing key (no auth) |
| GET | `/oauth2/authorize` | `AuthorizeGetHandler` | Renders login form (no auth) |
| POST | `/oauth2/authorize` | `AuthorizePostHandler` | Processes login, issues code (no auth) |
| POST | `/oauth2/token` | `TokenHandler` | All grant types (client auth) |
| GET | `/oauth2/userinfo` | `UserinfoHandler` | Identity claims (Bearer token) |
| POST | `/oauth2/revoke` | `RevokeHandler` | Revokes token via JTI blacklist (client auth) |

### Architecture notes

- **Signing**: ES256 (ECDSA P-256). The private key is loaded from `ego.server.oauth.as.key.file`; if absent it is auto-generated and saved. The public key is published as a JWKS document. The signing key is stored in the package-level `signingKey` variable in `keys.go`.
- **Client registry**: JSON array at `ego.server.oauth.as.clients`. Plaintext `client_secret` values are bcrypt-hashed in memory on load and never kept as plaintext. See `OAuthClient` struct in `clients.go`.
- **Caches**: Authorization codes are stored in `caches.OAuthCodeCache` (default 5-minute TTL, single-use — deleted on read by `consumeCode()`). Refresh tokens are stored in `caches.OAuthRefreshCache` (default 24-hour TTL, with rotation on use).
- **Scope mapping**: Ego user permissions are mapped to OAuth2 scopes: `ego.root`→`ego:admin`, table write permissions→`ego:write`, `ego.logon`→`ego:read`, `ego.code`→`ego:code`. All users also receive `openid`, `profile`, `email`.
- **Route registration**: `authserver.RegisterRoutes(r)` is called in `internal/commands/server.go:defineNativeAdminHandlers()` when `OAuthASEnabledSetting` is true. It initializes the signing key, client registry, and discovery document before registering the seven routes.
- **PKCE**: S256 method is supported (not `plain`). PKCE is optional — clients that include a `code_challenge` must supply a matching `code_verifier` at the token endpoint.
- **Token revocation**: `revoke.go` extracts the JWT's `jti` claim and calls `tokens.Blacklist(jti)` to persist the revocation.

### Client registry format

```json
[
  {
    "client_id": "myapp",
    "client_secret": "supersecret",
    "redirect_uris": ["https://myapp.example.com/callback"],
    "grant_types": ["authorization_code", "client_credentials", "refresh_token"],
    "scopes": ["openid", "profile", "ego:read"]
  }
]
```

Plaintext `client_secret` values are bcrypt-hashed and cleared on first load. You may pre-hash them and use `client_secret_hash` instead.

### Quick start

```sh
ego set config ego.server.oauth.as.enabled=true
ego set config ego.server.oauth.as.issuer=http://localhost:4040
ego server -k -p 4040

# Verify discovery
curl http://localhost:4040/.well-known/openid-configuration

# Get a client_credentials token
curl -X POST http://localhost:4040/oauth2/token \
  -d "grant_type=client_credentials&client_id=myapp&client_secret=supersecret&scope=openid ego:read"
```

### Running AS and RS simultaneously using profiles

When testing AS and RS on the same machine, use named configuration profiles
(`--profile` / `-p`) to keep the two sets of settings isolated:

```sh
# Configure and start the AS in the "oauth" profile
ego -p oauth set config ego.server.oauth.as.enabled=true
ego -p oauth set config ego.server.oauth.as.issuer=http://localhost:4040
ego -p oauth server start -k --port 4040

# Configure and start the RS in the default profile
ego config set ego.server.oauth.provider=http://localhost:4040
ego config set ego.server.oauth.mode=hybrid
ego server start -k --port 8080
```

See `docs/OAUTH.md` (section "Testing AS and RS Together Using Profiles") for
the full walkthrough, including how to run `apitest` against each server.

### Full design document

See `docs/OAUTH.md` for the full design, implementation checklist, and Phase 2 (RS mode) plans.

---

## Database Dialect Notes

Ego supports two database backends — **SQLite** (`modernc.org/sqlite`, provider `"sqlite"`) and **PostgreSQL** (`lib/pq`, provider `"postgres"`). The provider string `"sqlite3"` is a deprecated alias; `"postgresql"` is a deprecated alias for Postgres. Both are normalized at open time. A full dialect audit is in `docs/DATABASES.md`.

### SQL Identifier and Value Quoting

- **Identifier quoting** (table names, column names): always use `egostrings.SQLIdentifier(name)`. This wraps the name in ANSI SQL double-quotes and doubles any internal double-quote (`"` → `""`). Do NOT use `strconv.Quote()` — it uses Go backslash escaping, which is invalid in SQL identifiers.
- **String literal values**: embed as `$N` positional parameters and pass them as `...any` arguments to `db.Query()` or `db.Exec()`. Never interpolate user-supplied strings directly into SQL. Both `modernc.org/sqlite` and `lib/pq` accept `$N` positional placeholders.
- `egostrings.SQLIdentifier` lives in `internal/util/strings/quotes.go` (package name `egostrings`) alongside `SingleQuote`.

### Type Portability

- In the `internal/resources/` framework, `SQLStringType = "TEXT"`. TEXT is accepted by both SQLite and PostgreSQL. Do not use `"char varying"` — SQLite accepts it via type affinity but it appears as a different string during schema introspection.
- SQLite uses `TEXT / INTEGER / REAL` affinity names; PostgreSQL uses `CHAR VARYING / INT / DOUBLE PRECISION / TIMESTAMP WITH TIME ZONE`. When generating DDL for user-facing tables (via `internal/server/tables/parsing/generators.go`), use `MapColumnType(native, provider)` which dispatches to a dialect-specific type map.
- `ALTER TABLE … ADD COLUMN` type names should use `TEXT` (not `char varying`) so the migration works identically on both databases.

### Provider-Specific Branching

- `db.Provider == defs.SqliteProvider` is the canonical provider check. The deprecated `defs.DeprecatedSqliteProvider` alias must also be normalized before comparison; the `internal/server/tables/database` open path does this.
- SQLite has no schema concept. `createSchemaIfNeeded()` (in `internal/server/tables/tables.go`) returns early when the provider is SQLite. Do not issue `CREATE SCHEMA` statements against SQLite.
- `PRAGMA index_list(tbl)`, `PRAGMA index_info(idx)`, `PRAGMA table_info(tbl)` are SQLite-only for column metadata. For PostgreSQL, use `information_schema.columns` and `pg_index`/`pg_attribute` (or `pg_class`/`pg_namespace` for OID lookups — preferred over `::regclass` which folds names to lowercase).
- To list tables: SQLite uses `select name from sqlite_schema where (type='table' or type='view')` with no schema filter; PostgreSQL uses `SELECT table_name FROM information_schema.tables WHERE table_schema = $1`.

### Extracting Schema and Table Names

- `parsing.FullName(provider, user, name)` returns a provider-appropriate fully-qualified name: `"schema"."table"` for PostgreSQL, `"table"` for SQLite.
- `parsing.TableNameParts(provider, user, name)` returns a `[]string` of unquoted bare name parts (strips double-quotes). The last element is always the unquoted table name. Use this to extract names before passing them as SQL parameters.
- `egostrings.SQLIdentifier(parts[len(parts)-1])` re-wraps the bare name for use in SQL statements (e.g., PRAGMA arguments).

### SQLite PRAGMA Quoting

PRAGMA calls that take a table or index name accept SQL identifier quoting:

```go
tableOnly := egostrings.SQLIdentifier(parts[len(parts)-1])
q := fmt.Sprintf("PRAGMA index_list(%s)", tableOnly)   // → PRAGMA index_list("mytable")
```

Never split on `"."` to extract the table name from a fully-qualified string — use `TableNameParts()` instead.

### `internal/resources/` Framework DDL

The resource framework (`internal/resources/`) uses Go struct reflection to generate DDL. Key generated SQL patterns use `egostrings.SQLIdentifier()` for all identifiers:

```go
createTableSQL() → create table "tablename" ("col" TEXT primary key, ...)
insertSQL()      → insert into "tablename"("col", ...) values($1, ...)
updateSQL()      → update "tablename" set "col" = $1, ...
deleteRowSQL()   → delete from "tablename" ...
readRowSQL()     → select "col", ... from "tablename"
```

Nullable columns emit `NULL` (not `nullable` — the latter is not a valid SQL keyword).

---

## Important Notes

- The `ego` binary must know `EGO_PATH` (or `ego.runtime.path` in config) to find its `lib/` directory; without it some features degrade gracefully.
- Race conditions in the symbol table have been an active area of fixes — be careful with concurrent symbol table access.
- The `builds/` directory contains pre-built binaries for all platforms; do not edit these by hand.
- `tools/buildver.txt` is the single source of truth for the version number; the build script reads and optionally increments it.
