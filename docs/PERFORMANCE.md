# Ego Interpreter Performance Audit

This document reports the findings of a CPU-profiling audit of the Ego bytecode interpreter
and runtime, triggered by the observation that a plain 50,000,000-iteration `for` loop (see
`examples/panic.ego`) takes on the order of 95 seconds to run. It is a survey of *where the
interpreter actually spends its time*, backed by real `pprof` profiles rather than
speculation, together with concrete recommendations ranging from one-line fixes to
significant design changes. Findings are ordered roughly by impact-for-effort, not by where
they appear in the call graph.

---

## 1. Methodology

Ego's `run` command has a hidden, developer-only `--pprof <file>` flag
(`internal/commands/run.go`) that wraps program execution in Go's native
`runtime/pprof` CPU profiler. Three synthetic workloads were profiled with it, each isolating
a different part of the interpreter:

| Workload | Program | Purpose |
| - | - | - |
| **Tight loop** | `count := 0; for i := 0; i < 20000000; i++ { count = count + 1 }` | Isolates loop/scope overhead and scalar arithmetic, with no function calls at all. |
| **Function calls** | Same loop, but each iteration calls a trivial `func add1(x int) int { return x + 1 }` | Adds function-call overhead (argument binding, scope push/pop, return) on top of the loop cost. |
| **Native package calls** | Same loop, calling `math.Sqrt(2.0)` each iteration | Isolates the cost of calling into a native-pass-through Go function via reflection. |

```sh
ego run --pprof /tmp/cpu.prof program.ego
go tool pprof -top -cum ./ego /tmp/cpu.prof
```

Timings below are wall-clock (`time ego run ...`) on the development machine used for this
audit; absolute numbers will vary by hardware, but the *proportions* — which is what matters
for prioritizing work — should generalize.

| Workload | Iterations | Wall time | CPU samples | % of samples inside `RunFromAddress`'s own subtree |
| - | - | - | - | - |
| Tight loop | 20,000,000 | 37.2s | 44.8s (120%\*) | 40.0% |
| Function calls | 3,000,000 | 17.1s | 23.4s (137%\*) | 23.3% |
| Native calls | 1,000,000 | 2.9s | 3.3s (113%\*) | 39.5% |

\* Percentages over 100% mean multiple OS threads were executing concurrently during
profiling (mostly the Go runtime's own background GC workers — see Finding 6).

The single biggest, cross-cutting takeaway, expanded on in Finding 6: **in every workload,
well under half of total CPU time was spent inside the interpreter's own instruction-dispatch
loop.** The majority went to Go's garbage collector and scheduler, as a direct consequence of
how many small heap objects the interpreter allocates per loop iteration and per function
call. Findings 1–4 are, not coincidentally, all about reducing that allocation rate.

---

## 2. Finding 1 — `uuid.New()` on every scope creation (highest impact, lowest risk)

**Impact:** in both profiled workloads, generating a random UUID accounted for roughly
**half of the total cost of creating a new symbol-table scope** — and a new scope is created
on every loop iteration, every function call, and every `if`/block entry.

### Evidence - Finding 1

```text
(tight loop, 20M iterations — /tmp/cpu.prof)
ROUTINE ==== github.com/tucats/ego/internal/language/symbols.NewChildSymbolTable
      70ms      4.38s (flat, cum)  9.77% of Total
         .          .    153:   symbols := SymbolTable{
         .          .    154:      Name:    name,
         .      180ms    155:      symbols: map[string]*SymbolAttribute{},
         .      2.42s    156:      id:      uuid.New(),        <-- 55% of this function's own cost
         .          .    157:   }
         .          .    158:   symbols.shared.Store(SerializeTableAccess)
         .          .    159:
         .      780ms    160:   symbols.SetParent(parent)
         .          .    161:   ...
         .      800ms    170:   symbols.initializeValues()
```

```text
(function-call loop, 3M calls — /tmp/cpu_call.prof)
ROUTINE ==== github.com/tucats/ego/internal/language/symbols.NewChildSymbolTable
      10ms      1.88s (flat, cum)  8.04% of Total
         .          .    156:      id:      uuid.New(),        <-- 1.04s of 1.88s = 55%
```

`google/uuid.New()` is backed by `crypto/rand`, which on this platform ultimately makes a
real syscall (`ARC4Random` → `syscall.syscall`) for every single call. That is an expensive
thing to do on a hot path that runs millions of times per second.

### Root cause - Finding 1

`internal/language/symbols/tables.go` (`NewChildSymbolTable`, line 156; `NewSymbolTable`,
line 141) and `internal/language/symbols/copy.go` (`NewChildProxy`, line 28; `Clone`, line
111) each stamp a fresh `uuid.New()` onto every table's `id uuid.UUID` field.

**The ID is used purely for human-readable diagnostics** — every call site that reads it
(`internal/runtime/runtime.go`, `internal/runtime/util/symbols.go`,
`internal/language/bytecode/defer.go`, `internal/language/symbols/format.go`) only ever
passes it to `ui.Log`/`Format` for trace output. It is never compared, hashed as a map key, or
used for anything security-sensitive. `symbols.(*SymbolTable).ID()` even documents this: it
exists so callers "can identify a specific table in log output."

`Clone` (`copy.go:111`) is doubly wasteful: it calls `NewChildSymbolTable` (which already
generates one UUID) and then immediately overwrites `.id` with a **second** `uuid.New()`
call, discarding the first.

### Recommendation - Finding 1

Replace the cryptographically-random UUID with a cheap, process-unique identifier. Two
options, in order of preference:

1. **Simplest and fastest:** change `id` to a `uint64` populated from an
   `atomic.Uint64` counter (`sync/atomic`'s `Add`), formatted as needed only when actually
   logged. This is a two-line change per call site, is allocation-free, and remains unique for
   the life of the process — everything the current usage actually needs. The only cost is
   updating `ID()`'s return type and the handful of call sites that currently expect
   `uuid.UUID`/`uuid.Nil` (grep shows this is a small, contained set — see
   `tables_test.go`, `format.go`, `runtime.go`, `util/symbols.go`, `defer.go`).
2. **Lower-disruption alternative:** keep the `uuid.UUID` type (no signature/test changes)
   but stop using `uuid.New()`'s crypto-random generator. `google/uuid` supports plugging in
   a custom `io.Reader` via `uuid.SetRand()`; pointing it at a fast, non-cryptographic PRNG
   (seeded once at startup) removes the syscall while keeping every existing call site and
   type unchanged. This is the lower-risk option if a wider signature change is undesirable.

Either fix is small, mechanical, and safe — there is no correctness dependency anywhere on
these IDs being unpredictable or cryptographically unique, only *distinct enough for a human
reading a trace log to tell two tables apart*.

Also fix `Clone`'s redundant second `uuid.New()` call while making this change.

### Resolution (July 2026) — Finding 1

Implemented option 1 above (the simplest/fastest one): `SymbolTable.id` is now a plain
`uint64`, populated from a package-level `atomic.Uint64` counter, rather than a
`google/uuid.UUID`. Per the design direction given for this fix, no attempt was made to keep
the value "UUID-shaped" — a small, clearly-legible integer is exactly what the logging use
case wants, and is arguably *more* readable in a log line than a UUID ever was.

**Files modified:**

- `internal/language/symbols/tables.go` — removed the `github.com/google/uuid` import; added
  `nextTableID atomic.Uint64` and a `newTableID()` helper (`nextTableID.Add(1)`, documented as
  never returning 0 so 0 remains available as the "no table" sentinel); changed the `id`
  field from `uuid.UUID` to `uint64`; updated `NewSymbolTable` and `NewChildSymbolTable` to
  call `newTableID()` instead of `uuid.New()`; changed `ID()`'s return type to `uint64` and
  its nil-receiver sentinel from `uuid.Nil` to `0`.
- `internal/language/symbols/copy.go` — removed the now-unused `uuid` import;
  `NewChildProxy` calls `newTableID()` instead of `uuid.New()`. See "Bug found" below for the
  change to `Clone`.
- `internal/language/symbols/format.go` — two `fmt.Sprintf` call sites and one `ui.Log` call
  site that rendered `s.id.String()` (or `parent.ID().String()`) now use the `uint64` value
  directly with a `%d` verb (or no formatting at all for the `ui.A` map case, since `ui.Log`
  formats any value type).
- `internal/language/symbols/get.go` — five `ui.Log`/`ui.WriteLog` call sites changed from
  `s.id.String()` to `s.id` (same reasoning).
- `internal/runtime/util/symbols.go` — the one call site that exposes a table's ID through
  Ego's `util.symbols()` builtin (`UtilSymbolTableType`'s `"id"` field is declared as
  `data.StringType`, so unlike the internal log call sites, this one genuinely needs a
  string) now uses `strconv.FormatUint(p.ID(), 10)` instead of `p.ID().String()`. This is the
  only externally-visible (Ego-program-visible) change: a symbol table's reported `"id"` is
  now a decimal string like `"42"` instead of a UUID string. Nothing in `docs/API.md` or
  `docs/DASHBOARD.md` documents this field as UUID-formatted, and it is not used as a lookup
  key anywhere, so this is not considered a breaking change.
- `internal/runtime/runtime.go` and `internal/language/bytecode/defer.go` — no changes
  needed; both pass `s.ID()` straight into a `ui.A` map value, which accepts any type.
- `internal/language/symbols/tables_test.go` and `values_test.go` — removed the `uuid`
  import; replaced `uuid.New()` in test fixture construction with `newTableID()`, and the two
  `symbols.id == uuid.Nil` assertions with `symbols.id == 0`.

**Bug found and fixed while doing this work:** `SymbolTable.Clone` (`copy.go`) called
`NewChildSymbolTable(...)` — which, both before and after this change, already assigns a
fresh, unique ID to the new table — and then immediately **overwrote** that ID with a second,
independently generated one a few lines later, silently discarding the first. This was
always a wasted ID generation (harmless before this fix beyond the wasted `uuid.New()` call;
still harmless after, since an atomic increment is nearly free, but there is no reason to
keep pointless code once it's been noticed). The redundant assignment was deleted; `Clone`
now keeps the ID `NewChildSymbolTable` already gave the new table.

**Re-profiling results:** the same three workloads from Section 1 were re-profiled after the
change, using the identical programs and the same `--pprof` methodology.

| Metric (tight loop, 20M iterations) | Before | After | Change |
| - | - | - | - |
| Wall clock | 37.2s | 31.8s | **-14%** |
| `uuid.New` (self, anywhere in profile) | 2.4s+ (part of `NewChildSymbolTable`) | *(eliminated — does not appear)* | -100% |
| `NewChildSymbolTable` (cum) | 4.38s / 9.77% | 0.48s / 1.12% | **-89%** |
| `pushScopeByteCode` (cum) | 5.60s / 12.49% | 0.79s / 1.84% | **-85%** |
| `runtime.madvise` (flat) | 10.61s / 23.67% | 0.75s / 1.75% | **-93%** |
| `runtime.mallocgc` (cum) | 3.54s / 7.90% | 0.93s / 2.17% | **-74%** |
| `RunFromAddress` (cum — "true interpreter work") | 17.94s / 40.0% | 3.83s / 8.9% | **-79%** (absolute seconds) |
| `runtime.kevent` (flat) | 11.89s / 26.53% | 12.38s / 28.82% | ~flat |

| Metric (function calls, 3M calls) | Before | After | Change |
| - | - | - | - |
| Wall clock | 17.1s | 14.5s | **-15%** |
| `uuid.New` (cum) | 1.04s / 4.45% | *(eliminated)* | -100% |
| `NewChildSymbolTable` (cum) | 1.88s / 8.04% | 1.05s / 5.11% | **-44%** |
| `pushScopeByteCode` (cum) | 1.79s / 7.65% | 1.11s / 5.40% | **-38%** |

**The example that started this audit:** the plain 50,000,000-iteration loop from
`examples/panic.ego`'s scenario 2 (run standalone, without the goroutine, so it runs to
completion) dropped from **95.6s to 80.4s wall clock — a 16% reduction** — for the exact
program that motivated this investigation.

**Honest assessment of the result:** every cost this fix specifically targeted dropped
dramatically and exactly as predicted — `uuid.New()` is completely gone from the profile,
`NewChildSymbolTable`'s and `pushScopeByteCode`'s own costs fell by 85-89% on the tight loop,
and `runtime.madvise` (memory returned to the OS, a direct proxy for allocation churn) fell
by 93%. However, the **overall wall-clock improvement is a more modest 14-16%**, because —
exactly as Finding 5 predicted — this was never the only cost, and `runtime.kevent` (OS-level
thread park/wake overhead, not memory-reclamation) barely moved. That means the GC/scheduler
tax described in Finding 5 has shifted character: before this fix it was substantially about
returning freed memory to the OS (`madvise`); after this fix, with less garbage produced per
scope but still a full scope allocated every single loop iteration, it is now dominated by
plain goroutine/thread scheduling overhead from how often the collector still needs to run
at all. This is exactly the reasoning behind Finding 4 (skip per-iteration scope creation
when a loop body cannot observe it) and Finding 2 (stop the `TypeDeclarations` linear scan) —
both remain necessary to meaningfully reduce the *rate* of scope/value allocation itself, not
just the cost of each one, before the next large wall-clock improvement should be expected.

**Test status:** `go build ./...`, `go vet ./...`, and the full `go test ./...` suite are all
clean after this change, and all 1,142 tests in `ego test tests/` continue to pass. Symbol
table trace logging (`ego --log symbols run ...`) was manually inspected and now shows table
identities as small, clearly distinct integers (e.g. `root (0)`, `math (135)`, `main (175)`)
in place of UUID strings, confirmed against the "id clear in the symbol table logging"
requirement for this fix.

---

## 3. Finding 2 — `InstanceOfType`'s linear scan for every scalar type coercion

**Impact:** `data.Coerce` (called on essentially every arithmetic operation, comparison, and
typed assignment in Ego's default dynamic-typing mode) spent **7.7%** of total tight-loop CPU
time, of which two-thirds was inside `InstanceOfType`, and *that* function's own cost was
dominated by a **linear scan with a non-trivial comparison function**.

### Evidence - Finding 2

```text
ROUTINE ==== github.com/tucats/ego/internal/language/data.Coerce
     0.58s  1.29%  1.29%      3.46s  7.72%
                                             |   data.Coerce
                                             2.45s 70.81% |     data.InstanceOfType
                                             0.27s  7.80% |     data.coerceToInt64
                                             0.16s  4.62% |     data.coerceToInt

ROUTINE ==== github.com/tucats/ego/internal/language/data.InstanceOfType
     0.90s  2.01%  2.01%      2.45s  5.47%
                                             1.54s 62.86% |   data.(*Type).IsType   <-- linear scan cost
```

### Root cause - Finding 2

`data.Coerce` (`internal/language/data/coerce.go:32-34`) does this unconditionally whenever
its `model` argument is a `*Type` — which is the common case for scalar coercions like
`data.Int64(x)`:

```go
if t, ok := model.(*Type); ok {
    model = InstanceOfType(t)
}
```

`InstanceOfType`'s `default` branch (`internal/language/data/instance.go:85-94`), reached for
every scalar kind (`int`, `int64`, `float64`, `bool`, `string`, ...), does this:

```go
for _, typeDef := range TypeDeclarations {
    if typeDef.Kind.IsType(t) {
        return typeDef.Model
    }
}
```

`TypeDeclarations` (`internal/language/data/declarations.go`) is a **39-entry slice** that
does double duty as both "the compiler's table of recognized type-token spellings" and "the
runtime's table of zero-value models" — so every scalar coercion linearly scans up to 39
entries, calling `(*Type).IsType()` (which itself calls `UnwrapUserType()` twice per
comparison) on each one, just to find the zero value of a kind the caller almost always
**already knows** (`t.kind` is sitting right there).

### Recommendation - Finding 2

Replace the linear scan in the scalar branch with a direct dispatch on `t.kind` — a
`switch`/map keyed by `Kind`, using the exact same package-level model variables
(`intModel`, `int64Model`, `float64Model`, ...) that `TypeDeclarations` already points at.
This is a pure performance change with no behavioral difference: same models, same
fallback-to-nil for unrecognized kinds, just O(1) instead of O(n) with expensive comparisons.
`TypeDeclarations` itself does not need to change — it can stay as the compiler's
token-spelling table — only `InstanceOfType`'s lookup strategy needs to change.

As a secondary, smaller improvement: `data.Coerce`'s callers that only need "the zero value
for this *Kind*" (as opposed to genuinely needing to run the full `InstanceOfType` machinery
for structs/arrays/maps) could dispatch directly on `t.kind` before ever calling
`InstanceOfType`, skipping a function call entirely for the scalar fast path.

### Resolution (July 2026) — Finding 2

Implemented the primary recommendation above: `InstanceOfType`'s `default` branch now
dispatches directly on `t.kind` via a `switch`, using the exact same package-level Model
variables (`intModel`, `int64Model`, `boolModel`, ...) the old linear scan would eventually
have read out of `TypeDeclarations`. The secondary `data.Coerce`-level improvement (skipping
`InstanceOfType` entirely at the call site) was intentionally **not** done — it would touch a
more central, more frequently-called function for comparatively little extra gain now that
`InstanceOfType` itself is already fast, and Finding 2 was specifically about the scan, not
about `Coerce`'s calling convention.

**Files modified:**

- `internal/language/data/instance.go` — the `default:` branch of `InstanceOfType`'s outer
  `switch t.kind` now tries a direct, second-level `switch t.kind` first (one `case` per
  scalar `Kind` that actually has a `TypeDeclarations` entry: `Bool`, `Byte`, `Int8`, `Int16`,
  `UInt16`, `Int32`, `UInt32`, `Int`, `UInt`, `Int64`, `UInt64`, `Float32`, `Float64`,
  `String`, `Chan`, `Error`), each returning the matching Model variable directly — O(1),
  with no scan and no call into `IsType()` at all. The *original* linear scan is kept
  underneath, as a deliberately defensive fallback for any `Kind` not covered by the new
  switch (see "Correctness verification" below for why this is safe to treat as dead code in
  practice, while still costing nothing to keep as a safety net).
- `internal/language/data/declarations.go` — added `errorModel = &errors.Error{}` alongside
  the other package-level Model variables (previously the `TypeDeclarations` table's `"error"`
  entry inlined `&errors.Error{}` directly, with no named variable to reuse); the table's
  `error` entry now references `errorModel` too, so there is exactly one shared zero-value
  instance for the error type, read by both the table and the new switch, instead of two
  independently-allocated ones.
- `internal/language/data/instance_test.go` — added two new test functions (see below).

**Bug found while doing this work:** none. Unlike Finding 1 (where `Clone`'s redundant second
`uuid.New()` call was an incidental bug), this change was a pure algorithmic substitution with
no adjacent defect uncovered.

**Correctness verification:** because this touches a function called from nearly every
arithmetic/comparison/assignment operation in dynamic mode, it was verified more rigorously
than a typical change, with two new tests in `instance_test.go`:

- `TestInstanceOfType_ScalarFastPathMatchesTableScan` iterates every one of the 39 entries in
  `TypeDeclarations` and, for each one, independently reproduces the *old* linear-scan lookup
  by hand (a second, separate loop written directly in the test, not calling any production
  code) to use as a reference oracle. It then asserts that `InstanceOfType` — now running the
  *new* switch-based code — returns exactly what the old scan would have found, for every
  single entry in the table (skipping only the handful of entries for kinds `InstanceOfType`
  was already dispatching to a different case *before* ever reaching the scan — `Array`,
  `Map`, `Pointer`, `Type`, `Interface`, `Struct` — since the scan was already dead code for
  those and remains so). This is the strongest form of "no behavior change" verification
  available short of formal proof: it doesn't just spot-check a few kinds, it mechanically
  confirms agreement between old and new for the complete, actual table.
- `TestInstanceOfType_AllScalarKindsCovered` separately confirms each `Kind` handled by the
  new switch returns a correctly-typed, non-nil zero value, independent of whatever
  `TypeDeclarations` happens to contain — a belt-and-suspenders check on the switch itself.

Both new tests pass, alongside the pre-existing `TestInstanceOfType` (spot checks for int,
bool, float64, string, byte, int32, interface, struct, map, array) and the full existing
suite.

**Re-profiling results:** the same workloads and methodology from Section 1 (and from
Finding 1's own resolution) were used again, this time comparing against the **post-Finding-1**
baseline, since Finding 1 was already implemented and re-profiled first.

| Metric (tight loop, 20M iterations) | After Finding 1 only | After Finding 1 + 2 | Change |
| - | - | - | - |
| Wall clock | 31.8s | 27.3s | **-14%** (-27% from original baseline) |
| `InstanceOfType` (cum) | 590ms\* | 70ms / 0.19% | **-88%** |
| `data.(*Type).IsType` (cum) | 370ms\* | *(no longer appears in profile at all)* | effectively eliminated on this path |
| `data.Coerce` (cum) | 760ms / 1.77% | 350ms / 0.95% | **-54%** |

\* Estimated from the post-Finding-1 profile's `data.Coerce` breakdown (`InstanceOfType` was
1.37s / 1.59% of `Coerce`'s 0.76s+... — see the raw `-list` output captured during this
change for the exact figures; not independently called out as its own line item in Finding
1's resolution table).

| Metric (function calls, 3M calls) | After Finding 1 only | After Finding 1 + 2 | Change |
| - | - | - | - |
| Wall clock | 14.5s | 13.0s | **-10%** (-24% from original baseline) |
| `data.Coerce` (cum) | 610ms / 2.61% (pre-Finding-1 baseline) | 150ms / 0.79% | **-75% from original baseline** |

**The example that started this audit:** the standalone 50,000,000-iteration loop (see
Finding 1's own resolution for the same measurement taken after Finding 1 alone) now runs in
**70.0s**, down from **80.4s** after Finding 1 alone, and **95.6s** originally — a cumulative
**27% reduction** in wall-clock time from the two fixes together.

**Honest assessment:** exactly like Finding 1, this delivered a very large reduction in the
*specific* cost it targeted (`InstanceOfType` -88%, `IsType` calls on this path eliminated
entirely) but a more modest overall wall-clock gain (-10% to -14%), for the same reason
Finding 5 describes: GC/scheduler overhead (`runtime.kevent`, `runtime.madvise`) remains the
largest single cost in every profile taken so far, and neither this fix nor Finding 1
addresses it directly — both simply reduce how much CPU work happens *between* garbage
collections. `runtime.madvise` did drop further with this change (13.87% vs. Finding 1's
1.75%... — see raw profiles; both fixes reduce allocation-driven memory churn from different
angles), reinforcing that Finding 4 (stop allocating a whole new scope on every loop
iteration when nothing needs it) is still the most direct remaining lever on the dominant
cost in this report.

**Test status:** `go build ./...`, `go vet ./...`, and the full `go test ./...` suite are
clean after this change (including the two new tests above), and all 1,142 tests in
`ego test tests/` continue to pass.

---

## 4. Finding 3 — `ui.Log` call sites build their argument map before checking if logging is active

**Impact:** proportional to call-site frequency; confirmed to cost **~730ms (1.6% of total)**
at a single call site (`pushScopeByteCode`) in the tight-loop profile, and a further ~90ms at
`callBytecodeFunction`'s equivalent call for the function-call profile. Widespread — see
below — so the aggregate cost across the whole interpreter is almost certainly larger than
what these two call sites alone show.

### Evidence - Finding 3

```text
ROUTINE ==== github.com/tucats/ego/internal/language/bytecode.pushScopeByteCode
      80ms      5.60s (flat, cum) 12.49% of Total
         .          .     77:   newName := "block " + strconv.Itoa(c.blockDepth)
         .          .    120:   newTable := symbols.NewChildSymbolTable(newName, parent)...
         .          .    126:   ui.Log(ui.SymbolLogger, "symbols.push.table.boundary", ui.A{
      40ms      730ms    127:      "thread": c.threadID,
         .       90ms    128:      "name":   c.symbols.Name,
         .       10ms    129:      "parent": oldName,
         .          .    130:      "flag":   isBoundary})
```

### Root cause - Finding 3

`ui.Log(class int, format string, args A)` (`internal/cli/ui/messaging.go:264`) takes its
`args ui.A` (a `map[string]any`) **by value**, and only checks `loggers[class].active.Load()`
*inside* `Log`, after the caller has already constructed the map literal. Building a
multi-entry `map[string]any` is a real heap allocation plus several map inserts — paid on
every call, regardless of whether `SymbolLogger` (or whichever class) is even turned on,
which for a normal `ego run` it never is.

This pattern is **inconsistent** across the codebase: some call sites already guard against
exactly this (e.g. `internal/language/bytecode/catch.go` and `panic.go`, both fixed during
recent bug work, do `if ui.IsActive(ui.TraceLogger) { ui.Log(...) }`), and `symbols.Get`
correctly guards its own diagnostic log line the same way (`internal/language/symbols/get.go:89`).
But a rough count across `internal/language/bytecode`, `internal/language/symbols`, and
`internal/language/compiler` found **85 `ui.Log` call sites, only ~7 of which are guarded**
with a preceding `IsActive` check. Not all of the other 78 build expensive argument maps (a
`nil` args value, or a one-field map, costs relatively little), but every one of them is a
candidate worth checking, and every one on a genuinely hot path (opcode dispatch functions
especially) is a real, easy win.

### Recommendation - Finding 3

1. **Immediate, no-risk fix:** wrap the specific hot-path call sites identified above
   (`pushScopeByteCode`, `callBytecodeFunction`, and their siblings `popScopeByteCode`,
   `callFramePush`/`callFramePop`) in `if ui.IsActive(ui.SymbolLogger) { ... }`, matching the
   pattern already established in `catch.go`/`panic.go`/`get.go`.
2. **Systemic fix:** sweep every `ui.Log` call site in the three packages above (a
   mechanical, low-risk refactor) and add the same guard wherever the constructed `ui.A` has
   more than a trivial number of fields, or where any field requires computation (string
   concatenation, formatting, method calls) rather than a bare variable read.
3. Also worth fixing while touching this code: `pushScopeByteCode` computes
   `newName := "block " + strconv.Itoa(c.blockDepth)` (a string concatenation, `internal/language/bytecode/symbols.go:77`)
   and `callBytecodeFunction` computes `"function "+function.name`
   (`internal/language/bytecode/callBytecodeFunction.go:102`) **unconditionally on every
   scope push**, purely to give the new table a human-readable debug name. Consider computing
   these lazily (only when something actually reads `.Name`, e.g. inside the same
   `IsActive`-guarded logging block) rather than on every call.

### Resolution (July 2026) — Finding 3

Per direction from the project owner, this was done as a **full systemic sweep**, not just the
hot-path spot fixes originally proposed — deliberately going beyond what the tight-loop/
function-call profiles alone would have justified, in order to leave the codebase
consistent: every `ui.Log` call site should follow the same "check `IsActive` before doing
any work" shape, so a future contributor adding a new one has an unambiguous pattern to
copy, rather than having to guess whether a given call site was "hot enough to bother."

**Scope.** The sweep covered every package that is part of compiling or executing Ego
bytecode: `internal/language/{bytecode,data,symbols,compiler,tokenizer}`, `internal/builtins`,
and `internal/runtime/*` (every native-package implementation Ego programs can call into,
including I/O-bound ones like `rest`/`sql` — included for the consistency reason above, even
though their per-call-site win is smaller since network/database latency already dominates
their cost). Deliberately **excluded**: `internal/server/*`, `internal/router`,
`internal/cli/*`, `internal/commands`, `internal/dsns`, `internal/caches`,
`internal/resources` — these run at HTTP-request or CLI-command frequency, not
per-bytecode-instruction frequency, so guarding them would not show up in any profile of
"running an Ego program" and was out of scope for this performance audit. One package,
`internal/language/tokens`, was initially assumed in-scope (it sounded like lexer token
support) but turned out on inspection to be JWT authentication-token validation code — a
naming collision with the lexer's "tokens," not part of the interpreter at all — so it was
excluded for the same reason as the server packages.

**Method.** Within scope, every `ui.Log` call was inspected individually — not just the ones
with an obviously expensive-looking argument. Each fell into one of four buckets:

1. **`nil` argument, no setup** (89 of the 248 call sites found in-scope) — left untouched, per
   the explicit carve-out for this case: there is no map to construct and nothing to defer, so
   an `IsActive` guard would only add a branch for no benefit.
2. **Already correctly guarded** — either directly (`if ui.IsActive(class) { ui.Log(...) }`,
   the pattern already established by the BUG-35/BUG-45 fixes in `catch.go`/`panic.go`) or
   *transitively*, where the enclosing function already has an early return or an outer `if`
   on the same condition before ever reaching the log call (found in `trace.go`, `flow.go`'s
   `traceLine`, `disassembler.go`'s `Disasm`, `format.go`'s `Log`, and `methods.go`'s
   `logRequest`/`logResponse` — all confirmed by tracing the actual control flow, not just
   proximity of an `IsActive` check in the source text).
3. **Needed a guard, no other setup to move** — the common case: wrap the call (or, where the
   `if` body contained *only* the log call, fold `&& ui.IsActive(class)` into the existing
   condition instead of adding a new nested `if`, which reads more naturally at several call
   sites, e.g. `internal/language/data/accessor.go`'s `OrZero` family).
4. **Needed a guard AND had real setup to move inside it** — the interesting case the project
   owner specifically asked to watch for: a preceding line computes something (a string
   concatenation, a `.String()`/`.Format()` call, a second field lookup) that is used **only**
   by the log call. Found and fixed at several sites, e.g.:
   - `internal/language/bytecode/this.go`'s `getThisByteCode` — `data.Format(v)` (a recursive
     value formatter) was called on every receiver-method invocation purely to log it.
   - `internal/language/bytecode/callBytecodeFunction.go` — `parentName` (derived from
     `parentTable.Name`) was computed on every function call for logging only; `parentTable`
     itself was **not** moved, since it is also used for real work in the closure-with-
     captured-scope branch a few lines later (see the code's own comment, quoted in the
     Root Cause section above) — moving it would have changed behavior, so only the
     log-exclusive `parentName` derivation was guarded.
   - `internal/runtime/rest/client.go` — `egostrings.TruncateMiddle(token, 10)` (building a
     truncated, safe-to-log form of a bearer token) ran on every authenticated REST call
     whether or not `RestLogger` was active.
   - `internal/runtime/profile/initialization.go` — a token-truncation call at server startup,
     same shape as the client.go case.

   Where the same value was genuinely needed for both logging and later functional use (e.g.
   `create.go`'s `typeString`, used in both a log call and the subsequent error's `.Context()`;
   `symbols.go`'s `newName`, which becomes the new table's actual `.Name` field, not just a log
   label), the computation was deliberately **left in place** and only the `ui.Log` call itself
   was guarded — moving genuinely-dual-purpose work inside a logging-only guard would be a
   correctness risk for no performance benefit, since that work has to happen regardless of
   whether logging is active.

**Bugs found while doing this work:** none — this was a mechanical, behavior-preserving sweep
by construction (every change is "skip constructing something that would have been silently
discarded by an inactive logger anyway"), and each site was checked individually rather than
pattern-matched, specifically to avoid introducing one.

**Scale:** 248 `ui.Log` call sites were inspected across the in-scope packages; 89 needed no
change (nil argument); of the remaining 159, the large majority needed a new or corrected
guard, and roughly two dozen had genuine setup work that was identified and, where safe,
moved inside the guard alongside the log call itself.

**Re-profiling results:** the same workloads and methodology from Section 1 were used again,
this time comparing against the **post-Finding-2** baseline (the state after both prior
fixes), since Findings 1 and 2 were already implemented and re-profiled in that order.

| Workload | After Finding 1+2 | After Finding 1+2+3 | Change (this fix alone) | Change (cumulative from original) |
| - | - | - | - | - |
| Tight loop (20M iterations) | 27.3s | 20.95s | **-23%** | **-44%** |
| Function calls (3M calls) | 13.0s | 8.46s | **-35%** | **-51%** |
| 50,000,000-iteration loop (the example that started this audit) | 70.0s | 53.2s | **-24%** | **-44%** |

This is, by a wide margin, the largest single-fix improvement of the three implemented so
far — notably larger than either Finding 1 or Finding 2 individually, and larger than their
combined effect. The function-call workload benefited the most (-35% on top of the prior two
fixes) because every function call passes through `callBytecodeFunction` and `callFramePush`/
`callFramePop`, each of which had at least one unguarded (or setup-heavy) log call on the
per-call path; a plain arithmetic loop only pays the `pushScopeByteCode` cost once per
iteration, so it benefited somewhat less proportionally, though still substantially.

```text
(tight loop, after Finding 1+2+3)
     0.61s  2.53%  2.53%      9.40s 38.99%  github.com/tucats/ego/internal/language/bytecode.(*Context).RunFromAddress
     8.83s 36.62% 39.15%      8.83s 36.62%  runtime.kevent
     0.04s  0.17% 48.03%      1.32s  5.47%  github.com/tucats/ego/internal/language/bytecode.pushScopeByteCode
     0.04s  0.17% 54.54%      1.08s  4.48%  github.com/tucats/ego/internal/language/symbols.NewChildSymbolTable
```

**Honest assessment:** unlike Findings 1 and 2 — where the targeted cost dropped by 74-97%
but overall wall-clock only improved 10-16% — this fix moved the wall-clock number by roughly
the same proportion as the workload-specific costs it targeted. The likely reason: because
this fix touched dozens of call sites rather than one or two functions, and several of those
sites sit on the function-call path specifically (which the earlier two fixes touched only
indirectly, via the scope-creation machinery those calls also trigger), its aggregate effect
on total allocation rate was larger relative to its own direct, single-site cost than either
prior fix — consistent with Finding 5's thesis that allocation rate, not any one hot function,
is the dominant lever on total run time. `runtime.kevent` (thread scheduling overhead) remains
the largest single line item in every profile taken across all three fixes, reinforcing that
Finding 4 (stop allocating a scope at all for loop bodies that don't need one) is still the
most direct remaining way to move that number.

**Test status:** `go build ./...`, `go vet ./...`, and the full `go test ./...` suite are
clean after this change, and all 1,142 tests in `ego test tests/` continue to pass. Symbol
table trace logging was re-verified manually (`ego --log symbols,compiler,bytecode,optimizer
run ...`) to confirm log output is byte-for-byte unaffected when the relevant logger classes
are active — over 2,000 log lines were produced for a small three-iteration test program,
with no missing entries compared to before the sweep.

---

## 5. Finding 4 (design-level) — per-iteration loop scopes: correct, but costly, and often unnecessary

**This one is *not* a bug** — it is exactly the kind of "slow but present to support a
language feature" case worth documenting rather than "fixing" outright.

### What it costs

`pushScopeByteCode` (which, per Finding 1 + 2, mostly means `NewChildSymbolTable` +
`uuid.New()`) accounted for **12.49% of total tight-loop CPU time** and **7.65%–10.64% of
total function-call/native-call CPU time**. A `for` loop's body scope is re-entered — meaning
a brand new child `SymbolTable` (with its own `map[string]*SymbolAttribute`) is allocated —
**on every single iteration**, not once for the whole loop.

### Why it's there

This is intentional, and exists to correctly implement Go 1.22+'s per-iteration loop variable
semantics (see `internal/language/compiler/for.go`, the long comment block starting "Fix
BUG-30: per-iteration loop variables"). Before that fix, a closure created inside a loop body
that captured the loop variable would see whatever value the variable held when the loop
*finished*, not the value at the time the closure was created — a well-known class of bug in
pre-1.22 Go. The fix makes each iteration copy the loop's outer control variable into a
**fresh, per-iteration scope**, so a closure captures that iteration's own copy.

The mechanism: the loop body's own `PushScope` bytecode instruction is (already, structurally)
re-executed every time the loop branches back to the top of the body — so BUG-30's fix
piggybacks on that existing re-entry rather than introducing new bytecode, and just splices a
three-instruction "copy the outer variable into a new same-named variable in this fresh
scope" prologue into the body.

### The opportunity

The fresh-scope-per-iteration cost is only *necessary* when the loop body could actually
observe cross-iteration variable identity — i.e., when it contains a closure (`func literal`,
`go` statement, or `defer`) that might capture the loop variable and outlive the iteration.
The overwhelming majority of loop bodies do not do this (`for i := 0; i < n; i++ { sum +=
a[i] }` has no way to observe whether `i` is a fresh variable each iteration or not). For
those, the correct, much cheaper behavior is the pre-fix one: reuse a single scope for the
whole loop.

**Recommendation:** teach the compiler to detect, at the point it compiles a loop body,
whether that body contains any construct that can capture a variable by reference across
iterations (a function literal, a `go` statement, or a `defer` of anything other than a
direct call with already-evaluated arguments). If none is present, skip emitting both the
per-iteration `PushScope`/`PopScope` pair and the BUG-30 copy-in prologue, falling back to a
single scope push for the entire loop (exactly as a `for` loop with no scope-sensitive
constructs behaved before BUG-30). This preserves 100% of the corrected semantics for the
loops that need it, while removing the dominant cost of this profile for the many loops that
don't. This is a compiler-level (not runtime-level) change, and is more involved than
Findings 1–3, but plausibly the single biggest win available without a deeper architectural
change, *because* it would also proportionally shrink the cost of Finding 1 and Finding 2
(fewer scope creations means fewer `uuid.New()` calls and — indirectly — fewer coercions
triggered by re-entering a scope's variable initialization).

### Resolution (July 2026) — Finding 4

Implemented the recommendation above. The compiler now decides, once, at the point it
compiles each `for` loop's body, whether the body can ever observe cross-iteration variable
identity. If not, it pushes a single scope for the whole loop (outside the region the loop
branches back to) instead of letting the loop body's own `PushScope`/`PopScope` re-execute
every iteration, and skips the BUG-30 copy-in/copy-out prologue/epilogue entirely, since there
is no per-iteration scope for them to splice into.

**The predicate.** `loopBodyNeedsFreshScopePerIteration` (new, in `for.go`) is a deliberately
conservative token-level scan of the loop body — not a full semantic analysis — run once at
compile time, before the body is compiled. It forces the old (always-fresh) behavior whenever
the body contains, anywhere:

- a function literal, a `go` statement, or a `defer` (the exact BUG-30 closure-capture case), or
- a `:=` or `var` declaration directly at the body's own top-level (brace-depth 1) scope — reusing
  one scope for the whole loop would make the second iteration's declaration collide with the
  first's (`symbols.ErrSymbolExists`).

Being overly cautious here only costs a missed optimization opportunity, never correctness, so
two safe-but-imprecise cases are deliberately left undetected-as-safe rather than chased for
full precision: a declaration nested inside an `if`/`switch`/etc. block (actually scoped to
that nested block, not the loop body, but the depth-1-only check does not special-case it out)
and a nested `for`/named-init `switch`'s own `:=` sitting at depth 1 before *its own* opening
brace (actually scoped to the nested construct). Both are simply treated as disqualifying.

**Files modified:**

- `internal/language/compiler/for.go` — added `loopBodyNeedsFreshScopePerIteration` and a
  large comment block explaining the design; `compileForBody` gained a `perIterationScope
  bool` parameter (when `false`, it emits no `PushScope`/`PopScope` of its own, and the caller
  must not pass a prologue/epilogue — enforced with an internal-compiler-error check);
  `simpleFor`, `rangeFor`, and `iterationFor` each now compute the predicate once, before the
  loop's repeat point, and either hoist a single `PushScope`/matching `PopScope` pair around
  the whole loop (skipping the prologue/epilogue) or fall back to the unchanged, always-fresh
  per-iteration path. `conditionalFor` was deliberately left unmodified — it has its own
  existing "was the loop body actually empty" check that inspects the last emitted
  instruction for a `PopScope`, and reworking that check to stay correct under a
  sometimes-hoisted `PopScope` was judged not worth the risk for one of four loop forms.
- `internal/language/compiler/for_finding4_test.go` (new) — `TestLoopBodyNeedsFreshScopePerIteration`
  (10 cases directly exercising the predicate: empty body, arithmetic-only, plain reassignment,
  top-level `:=`/`var`, a declaration nested inside an `if`, a function literal, `go`, `defer`,
  a nested `for` loop, and malformed input) and `TestFinding4LocalDeclarationsAcrossIterations`
  (4 cases confirming a body-local `:=` declaration still computes correctly across iterations
  for the classic, range, and simple `for{}` forms).
- `tests/flow/for_shared_scope.ego` (new) — 8 `@test` blocks: local declarations across
  iterations for all three optimized loop forms, a loop with both a local declaration *and* a
  closure together (both disqualifying conditions active at once), nested `for` loops each
  declaring their own local, and a range loop with a local declaration plus a `break`.
- `docs/ISSUES.md` — new `BUG-63` entry (see below).

**Bug found while doing this work, but *not* fixed as part of this task:** while writing the
Ego-level regression test for `simpleFor`, a test using a bare `n++` statement produced a
wrong result. Root-caused to `docs/ISSUES.md` **BUG-63**: `compileAssignment`'s auto-increment
handling for a simple variable (`internal/language/compiler/assignment.go`) builds its own
inline `Load, Push 1, Add/Sub, Dup, Store` sequence and returns without ever appending the
`storeLValue` fragment's `DropToMarker` instruction, permanently leaking a "let" stack marker
onto the runtime stack every time a bare `x++`/`x--` *statement* executes (as opposed to `i++`
used inside a classic `for` loop's increment clause, which is compiled through a different,
unaffected code path in `for.go`). Confirmed, by stashing every Finding 4 change and
re-running the reproducer against the unmodified compiler, that this bug predates and is
completely unrelated to Finding 4. Documented in `docs/ISSUES.md` as `BUG-63` rather than
fixed here, to keep this change scoped and independently reviewable; the new Finding 4 tests
use `n = n + 1` instead of a bare `n++` statement to avoid it.

**Correctness verification:** beyond the new unit and Ego-level tests described above, the
full pre-existing BUG-30 regression suite — `internal/language/compiler/for_loopvar_test.go`
(15 cases) and `tests/flow/for_loopvar.ego` (12 `@test` blocks) — continues to pass unmodified,
confirming closures still correctly capture distinct per-iteration values in every loop form
this change touches.

**Re-profiling results:** the same workloads and methodology from Section 1 were used again,
this time comparing against the **post-Finding-3** baseline (the state after all three prior
fixes), since Findings 1-3 were already implemented and re-profiled in that order.

| Workload | After Finding 1+2+3 | After Finding 1+2+3+4 | Change (this fix alone) | Change (cumulative from original) |
| - | - | - | - | - |
| Tight loop (20M iterations) | 20.95s | 10.83s | **-48%** | **-71%** |
| Function calls (3M calls) | 8.46s | 6.35s | **-25%** | **-63%** |
| 50,000,000-iteration loop (the example that started this audit) | 53.2s | 27.25s | **-49%** | **-72%** |

This is the largest single-fix improvement of the four implemented so far, and lines up almost
exactly with the prediction in "What it costs" above: `pushScopeByteCode` and
`NewChildSymbolTable` — 12.49% of tight-loop time and 7.65-10.64% of function-call time before
this change — **no longer appear anywhere in the tight-loop profile at all** (neither function
shows up even with `-nodefraction=0`, confirming they are now called at most once for the
entire 20,000,000-iteration loop, not once per iteration). The function-call workload's smaller
percentage improvement is expected and correct: its loop body (`count = add1(count)`) has no
closures or top-level declarations, so the *loop's own* scope is eliminated exactly like the
tight loop — but every call to `add1` still pushes its own, separate, per-call function scope
(`callBytecodeFunction`/`callFramePush`), which Finding 4 does not touch and was never intended
to. That remaining, expected cost shows up directly in the post-fix profile:
`pushScopeByteCode` still accounts for 3.82% (0.32s) of the function-call workload's total, all
of it now attributable to the function-call machinery rather than the loop.

```text
(tight loop, after Finding 1+2+3+4 — pushScopeByteCode / NewChildSymbolTable no longer appear)
     0.55s  5.51%  5.51%      8.64s 86.49%  github.com/tucats/ego/internal/language/bytecode.(*Context).RunFromAddress
     0.64s  6.41% 42.84%      0.64s  6.41%  runtime.madvise
     0.18s  1.80% 75.98%      0.18s  1.80%  runtime.kevent
```

**Honest assessment:** unlike Findings 1-3, where the wall-clock improvement was noticeably
smaller than the targeted cost's own reduction (a symptom of GC/scheduler overhead dominating,
per Finding 5), this fix's wall-clock improvement (-48% to -49% on the two loop-only workloads)
tracks its targeted cost's elimination almost one-to-one. This is consistent with Finding 5's
thesis in reverse: because `pushScopeByteCode` and `NewChildSymbolTable` were themselves a
direct *cause* of allocation-driven GC/scheduler pressure (a brand-new `SymbolTable` plus its
backing `map[string]*SymbolAttribute`, 20,000,000 times over), removing the allocation removes
its downstream GC cost too, rather than just moving CPU time from one line item to another.
`runtime.kevent` and `runtime.madvise` remain present in every profile (thread scheduling and
memory management are not eliminated, just triggered far less often), but neither is anywhere
near the dominant cost it was in the Finding 3 profile.

**Test status:** `go build ./...`, `go vet ./...`, and the full `go test ./...` suite are clean
after this change. All 1,150 tests in `ego test tests/` pass (the pre-existing 1,142 plus the 8
new `tests/flow/for_shared_scope.ego` cases). The new Go unit tests in `for_finding4_test.go`
pass, alongside the full pre-existing `for_loopvar_test.go` suite (BUG-30 regression coverage).

---

## 6. Finding 5 (systemic) — garbage-collector and scheduler overhead dominates wall-clock time

**This is the umbrella finding that ties 1–4 together, and the single most important number
in this report.**

In every workload profiled, **more CPU time was spent in Go's own garbage collector and
goroutine scheduler than in the Ego interpreter's own instruction-dispatch loop**:

| Workload | % of samples in `RunFromAddress`'s subtree | % of samples in GC/scheduler (`runtime.kevent`, `runtime.madvise`, `gcStart`/`gcDrain`, `park_m`/`findRunnable`, ...) |
| - | - | - |
| Tight loop | 40.0% | ~50%+ |
| Function calls | 23.3% | ~58%+ |
| Native calls | 39.5% | ~54%+ |

```text
(tight loop)
    11.89s 26.53% 26.53%     11.89s 26.53%  runtime.kevent
    10.61s 23.67% 50.20%     10.61s 23.67%  runtime.madvise
         0     0%  1.49%     11.91s 26.57%  runtime.startTheWorldWithSema
         0     0%  1.49%     11.90s 26.55%  runtime.gcStart.func4
```

`runtime.kevent`/`runtime.madvise` are OS-level syscalls the Go runtime makes as part of
stop-the-world/start-the-world GC pauses and returning freed memory pages to the OS. Their
sheer volume here is a direct symptom of **allocation rate**: every loop iteration and every
function call allocates at least one new `SymbolTable` (with its own backing map), forcing
the garbage collector to run far more often than the actual amount of "real work" being done
would otherwise require.

A quick, no-code-change experiment confirms the diagnosis directionally: raising `GOGC` (less
frequent, larger GC cycles) reduced *system* time substantially (3.52s → 0.80s in one run) and
gave a modest (~5%) wall-clock improvement — real, but not a substitute for reducing the
underlying allocation rate, which is what Findings 1, 2, and 4 target directly. Tuning
`GOGC`/`GOMEMLIMIT` as a runtime default for `ego run` may be worth a small, separate
follow-up experiment, but should be treated as a secondary knob, not the fix.

**Bottom line:** Findings 1 (remove `uuid.New()`), 2 (remove the `TypeDeclarations` linear
scan), and 4 (skip per-iteration scopes when safe to do so) all reduce the same thing —
small-object allocation rate in the hottest part of the interpreter — and should be expected
to compound: each one not only saves its own direct cost, but also reduces how often the
garbage collector needs to run at all, which by this data is currently *more than half* of
the total cost of running an Ego program.

---

## 7. Finding 6 (measured, and smaller than expected) — reflection-based native calls

CLAUDE.md documents that "native pass-through" functions (functions registered with
`IsNative: true`, e.g. `math.Sqrt`, `time.Now`) are invoked via Go's `reflect.Value.Call`,
which has a well-known reputation for being slow (argument boxing, `[]reflect.Value`
allocation, runtime type checks). This was expected to show up clearly in the "native calls"
profile (1,000,000 calls to `math.Sqrt`).

**It did not.** `reflect.Value.call` accounted for only **0.91%** of total samples in that
profile — a real cost, but a minor one, and dwarfed by the *same* scope/allocation overhead
already described in Findings 1, 2, and 5 (`pushScopeByteCode` alone was 10.6% of this
profile, over ten times the reflection cost). This is worth stating explicitly, in the
interest of not chasing a plausible-sounding but not-actually-dominant cost: **reflection
overhead for native calls is real but is not, on this evidence, a priority relative to the
scope/allocation issues above.** It may be worth revisiting with a more call-heavy (rather
than loop-heavy) workload, or one that calls native functions taking/returning more complex
argument shapes, before investing engineering effort here.

---

## 8. Finding 7 (architectural, high-effort, high-ceiling) — name-based symbol resolution

Every variable read or write (`symbols.Get`/`Set`) resolves the variable by **string name**
through a `map[string]*SymbolAttribute` at the current scope, walking up the parent chain on
a miss:

```text
ROUTINE ==== github.com/tucats/ego/internal/language/symbols.(*SymbolTable).Get
     160ms      1.20s (flat, cum)  2.68% of Total
         .      550ms     72:   attr, found := s.symbols[name]     <-- map lookup, per scope in the chain
         .       30ms     85:      return next.Get(name)          <-- recurse to parent on miss
```

This costs 1.9%–2.7% of total samples on its own in these profiles (on top of the scope
*creation* costs already covered above), and — because it is a map lookup, not an array
index — the cost scales with hashing the name string, not with anything about the variable
itself.

This is a known, general pattern in interpreter design: resolving a local variable by name at
every single reference is asking more of the runtime than necessary, because the *set* of
local variable names in a given function/block is already fully known to the compiler at
compile time. Compiled/bytecode-VM languages typically resolve locals to a **slot index**
(an integer offset into the current call frame) during compilation, turning "hash a string,
probe a map, possibly recurse to a parent table" into "index into an array" — a difference of
roughly an order of magnitude in raw cost, with no chain-walking needed at all for anything
declared in the same function.

**This is the highest-ceiling, highest-effort item in this report.** It would require:

- A compile-time "slot allocation" pass (assigning each local variable in a function/block a
  fixed slot number, tracking which names are still needed by closures and must remain
  name-addressable for capture).
- Changing the `Load`/`Store`/`CreateAndStore`/`StoreAlways` bytecode operand from a name
  string to a slot index (or a hybrid — slot number plus a name for the (rarer) cases that
  still need dynamic/reflective lookup, e.g. `@global`, debugger variable inspection, or
  runtime `errors`/`recover` machinery that reads named-return variables by name).
- Preserving today's fully-dynamic behavior (`ego.compiler.types=dynamic`, the default) where
  a variable's very existence can depend on runtime control flow — slot allocation is
  straightforward when a variable's declaration is unconditionally reached, and needs care
  for variables declared inside conditionally-executed blocks.

This is explicitly the kind of "big new design feature" the audit was asked to flag even
though it goes well beyond a bug fix — a genuine VM-level redesign, not a tweak. It is not
recommended as a next step (Findings 1–4 are far cheaper and, per the profiling here, address
a comparable or larger fraction of total cost), but it is the natural ceiling on how fast a
tree-walking-by-name symbol table can ultimately get, and worth having on record for anyone
considering Ego's longer-term performance trajectory (up to and including a JIT, which would
depend on exactly this kind of compile-time-resolved storage to be worthwhile at all).

---

## 9. Finding 8 (design-level) — per-execution basic-block scopes: same idea as Finding 4, one level lower

**This one is also *not* a bug** — same category as Finding 4: a runtime scope that exists to
correctly support a language feature (block-local `:=`/`var`/`const`/`type` isolation), paid
for on every execution even when that feature is never actually used.

### What it costs - Finding 8

Finding 4 stopped a `for` loop from re-allocating a fresh scope on every *iteration* when the
loop's own body never needed one. It did not touch the scopes pushed by constructs *nested
inside* that body. An `if`/`else` whose branches declare nothing still pushed and popped a
scope every single time either branch ran — including every iteration of a loop whose own
body scope Finding 4 had already eliminated:

```go
for i := 0; i < 20000000; i++ {
    if i % 2 == 0 {
        sum = sum + i
    } else {
        sum = sum - i
    }
}
```

Before this fix, `pushScopeByteCode`/`NewChildSymbolTable` reappear in this profile (despite
Finding 4 already having eliminated them from an *unconditional*-body version of the same
loop), because the `if`/`else` bodies are a completely separate pair of `PushScope`/`PopScope`
instructions that Finding 4 never analyzed.

### Why it's there - Finding 8

Same reason as Finding 4: a block's `PushScope`/`PopScope` pair exists to give any `:=`/`var`
declarations inside it an isolated home that is discarded when the block exits, so a second
execution of the same block (because it's inside a loop, or its enclosing function is called
again) does not collide with the first. This is necessary machinery for blocks that declare
something — but the *overwhelming majority* of `if`/`else` bodies, and a great many function
bodies, declare nothing at their own top level at all.

### The opportunity / Recommendation

Identical shape to Finding 4's, applied to any brace-delimited block, not just loop bodies:
detect, once, at the point the compiler is about to compile a block's body, whether that body
declares anything (`:=`, `var`, `const`, `type`, `import`, or `package`) at its own top level,
or contains a closure-capturing construct (function literal, `go`, `defer`). If none of those
is present, skip the block's `PushScope`/`PopScope` pair entirely — unlike Finding 4, there is
no "loop variable" forcing at least one scope to remain, so a qualifying block needs **zero**
scopes, not one shared scope.

### Resolution (July 2026) — Finding 8

Implemented the recommendation above.

**The predicate.** `blockBodyNeedsOwnScope` (new, in `internal/language/compiler/block.go`) is
a deliberately conservative token-level scan of the block body — not a full semantic analysis,
matching Finding 4's own predicate in spirit — run once at compile time, before the block is
compiled. It forces the old (always-scoped) behavior whenever the block contains, anywhere, a
function literal/`go`/`defer`, or, directly at its own top level (brace-depth 0 relative to
itself), a `:=`, `var`, `const`, `type`, `import`, or `package` declaration.

**Where it's applied.** `compileBlock`/`compileRequiredBlock` gained a `mayElideScope bool`
parameter; when true and the predicate finds nothing disqualifying, the runtime
`PushScope`/`PopScope` instructions are skipped (compile-time bookkeeping — `blockDepth`,
`PushSymbolScope`/`PopSymbolScope`, and therefore unused-variable detection — always still
happens, exactly as Finding 4's `compileForBody` already established as the right pattern).
Enabled at: `if`/`else` bodies (`if.go`), a bare `{ ... }` statement (`statement.go`), the
`@compile` directive's catch block (`directives.go`), a `for cond { ... }` loop's body
(`conditionalFor`, `for.go` — previously excluded by Finding 4 for an unrelated reason, fixed
here, see below), a function's own body (`function.go` — the scope layer *inside* the
call-boundary/parameter scope, which is untouched), and a `try`/`catch` statement's `catch`
body only (`try.go`).

**Two call sites were deliberately left unmodified**, each for a specific, verified reason
rather than general caution:

- **`try`'s own body** (not its `catch` body) must always keep its scope. When an error occurs
  partway through the try body, control jumps directly to the catch handler *without* running
  the try body's own normal-exit `PopScope`, leaving that scope deliberately open — the catch
  clause stores its error variable into that still-open scope, and an explicit extra
  `PopScope` (see `compileTry`) closes it once the catch body finishes. Eliding the try body's
  own scope would remove the table that mechanism depends on. The separate, nested `catch`
  body scope has no such dependency and was confirmed independently elidable.
- **`switch` `case`/`default` bodies** are not covered at all. Investigating this call site
  surfaced a genuine, pre-existing bug — now folded into
  [BUG-61](ISSUES.md#BUG-61) — where `continue` inside a case body already skips scope/symbol
  cleanup the switch statement depends on. That is silently masked today whenever the case
  body pushes its own (always-present) scope, because `continue`'s skip *also* leaves that
  scope's own `PopScope` unexecuted, which coincidentally shifts later same-switch iterations
  into a fresh table where the stale name is shadowed rather than collided with. Eliding this
  scope removes that accidental masking and makes the bug reliably reproducible instead of
  merely latent. Scope elision was therefore withheld from this one call site until BUG-61 has
  a real fix, rather than bundling an unrelated correctness fix into a performance change.

**A structural side effect required a small, independent fix.** `conditionalFor`
(`for.go`) had its own "was the loop body actually empty" check that inspected whether the
*last emitted instruction* was a `PopScope` — a check that would have silently stopped working
(or worse, started rejecting legitimate non-empty loop bodies) the moment a qualifying body's
`PopScope` was elided. It was replaced with a check based purely on `statementCount` (a
compile-time counter of real statements compiled), which was already being tracked alongside
the old check and needed no scope bytecode to exist at all — strictly more robust, and simpler
than what it replaced.

**Files modified:**

- `internal/language/compiler/block.go` — added `blockBodyNeedsOwnScope` and a large comment
  block explaining the design; `compileBlock`/`compileRequiredBlock` gained the
  `mayElideScope bool` parameter.
- `internal/language/compiler/if.go`, `statement.go`, `directives.go`, `function.go`,
  `try.go` — enabled elision at the call sites described above.
- `internal/language/compiler/for.go` — enabled elision for `conditionalFor`'s body; replaced
  the `PopScope`-sniffing empty-body check with a `statementCount`-only comparison.
- `internal/language/compiler/switch.go` — investigated and explicitly left unmodified (see
  above); the surfaced bug is documented, not fixed, in `docs/ISSUES.md` BUG-61.
- `internal/language/compiler/block_finding8_test.go` (new) —
  `TestBlockBodyNeedsOwnScope` (11 cases directly exercising the predicate: no declarations,
  plain assignment, top-level `:=`/`var`/`const`/`type`, a declaration nested inside an `if`
  at depth 2, a function literal, `go`, `defer`, a nested block with its own declaration not
  disqualifying the outer block, and malformed input) and
  `TestFinding8ConstInElidedBlockDoesNotLeak` (a same-program regression test for the `const`
  bug described next).
- `tests/flow/basic_block_shared_scope.ego` (new) — 19 `@test` blocks covering: plain
  if/else with no locals, an if body with a local repeated across 50 loop iterations, `const`
  declared inside an elided block (both a bare block and a function body, plus the exact
  two-test-file shape that originally surfaced the leak), `break`/`continue` inside elided
  blocks at 1-2 nesting levels (including 500-iteration and 200-call stress cases to catch
  slow scope-depth drift), `continue` inside a `try`/`catch`'s `catch` body (run 300×8 times),
  a `catch` body with its own local, a range-over-map loop nested inside an elided block
  exited via early `break` (confirming the map's read lock still releases), repeated
  pointer-receiver calls inside an elided block, a function body with no locals called 100
  times, a function using `defer` (confirming its scope is never elided), doubly-nested `if`
  with no locals at either level, and a `conditionalFor` loop with an elided body.

**A second, unrelated bug found while writing these tests, also *not* fixed as part of this
task:** the pointer-receiver test originally chained calls that returned the receiver as its
own declared pointer type (`b.add("a").add("b")`, each `add` returning `*Builder`). Under
`--types strict` this failed with `type mismatch: Builder …, *Builder …` — root-caused to
`getThisByteCode` (`internal/language/bytecode/this.go`) discarding Ego's pointer-type marker
when it dereferences a pointer receiver for field-write propagation, so the receiver's runtime
type no longer matches its declared type once returned. Confirmed pre-existing and unrelated
to Finding 8 by reproducing it against the unmodified compiler. Documented as
[BUG-64](ISSUES.md#BUG-64) rather than fixed here; the test was rewritten to call the receiver
method for its mutation side effect only (no chaining/return), which exercises the same
elided-block/receiver-stack mechanics this Finding's test suite cares about without depending
on the unrelated bug being fixed.

**Bug found while doing this work, but *not* fixed as part of this task:** implementing scope
elision for `switch` `case`/`default` bodies surfaced a genuine, pre-existing defect in how
`break`/`continue` interact with a switch statement's own scope/symbol cleanup — see the
"Reproducer 2" addition to `docs/ISSUES.md` BUG-61 for the full root-cause writeup and a
directly `ego run`-reproducible case (a `continue` inside a named-init `switch`'s case body,
in a loop, causes a later unrelated variable of the same name to fail with
`symbol already exists`). Confirmed pre-existing and unrelated to this change by reproducing
it against the unmodified compiler. Scope elision was withheld from switch case/default bodies
specifically because of this finding, keeping this change scoped to a genuine, uncontroversial
performance win rather than bundling in an unrelated correctness fix.

**A second, more subtle correctness bug was found and fixed during development, not merely
documented:** an early version of `blockBodyNeedsOwnScope` checked only for `:=` and `var`,
missing `const` (and `type`/`import`/`package`) entirely. A block containing only a `const`
declaration has no `:=`/`var` token at all, so it was wrongly treated as declaration-free and
elided. Since `ego test` runs every test file in one process against a single shared root
symbol table, a leaked immutable constant from one test file collided with an unrelated,
later variable of the same name in a completely different test file
(`item is read-only`) — caught by the full `ego test tests/` regression run before this change
was considered complete, root-caused, and fixed by adding all five remaining
declaration-introducing keywords to the predicate.

**Correctness verification:** beyond the new unit and Ego-level tests described above, the
full existing suite continues to pass unmodified: `go test ./...`, `go test -race ./...`
(1170+ Go tests across the repository), and `ego test tests/` (1174 `@test` blocks, up from
1155 before this change).

**Re-profiling results:** two workloads not previously improved by Findings 1-4, since neither
has an unconditional loop body:

| Workload | Before Finding 8 | After Finding 8 | Change |
| - | - | - | - |
| If/else in a tight loop (20M iterations, `if i%2==0 { sum += i } else { sum -= i }`) | 23.32s | 16.42s | **-30%** |
| Function calls with a no-locals body (3M calls to `func add1(x int) int { return x + 1 }`) | 6.70s | 6.00s | **-10%** |

(Wall-clock `time ego run`, averaged over 2 runs each; `ego run --pprof` profiles taken
alongside confirm `pushScopeByteCode`/`NewChildSymbolTable`/`popScopeByteCode` no longer
appear anywhere in the if/else workload's profile at all, matching Finding 4's own evidence
pattern for the equivalent loop-body case.) The smaller function-call improvement is expected
and correct, for the same reason Finding 4's function-call workload showed a smaller gain than
its tight-loop workload: `add1`'s own body scope is eliminated, but each call still pushes its
own, separate call-boundary/parameter scope, which this change does not touch and was never
intended to.

**Honest assessment:** the if/else workload's -30% is a genuine, standalone win on top of
everything Findings 1-4 already delivered — this is exactly the "basic block nested inside an
already-optimized loop" case Finding 4 explicitly did not cover. The function-call workload's
smaller -10% is consistent with (and roughly comparable to) Finding 4's own function-call
result, for the identical reason: the per-call boundary scope (Finding 7's territory, not this
one) remains the dominant remaining cost for call-heavy code.

---

## 10. Summary table

| # | Finding | Impact (measured) | Effort | Risk | Type | Status |
| - | - | - | - | - | - | - |
| 1 | `uuid.New()` per scope creation | ~50% of scope-creation cost; scope creation is 8-12% of total | Low | Low | Implementation bug (wrong tool for the job) | **Fixed** (July 2026) — see Resolution above |
| 2 | `InstanceOfType` linear scan | ~5.5% of total, ~63% of that is the scan itself | Low | Low | Implementation bug (should be O(1)) | **Fixed** (July 2026) — see Resolution above |
| 3 | Eager `ui.Log` argument construction | ~1.6%+ confirmed at 2 call sites; 248 sites swept codebase-wide | Low (hot spots) / Medium (full sweep) | Low | Implementation inconsistency | **Fixed** (July 2026) — see Resolution above |
| 4 | Per-iteration loop scopes | 8-12% of total | Medium-High | Medium (compiler analysis) | Working as designed (BUG-30); optimizable | **Fixed** (July 2026) — see Resolution above |
| 5 | GC/scheduler overhead | >50% of total wall time in every workload | N/A (symptom) | N/A | Systemic consequence of 1, 2, 4 | Partially addressed by fixing 1, 2, and 3 — see Resolution sections above |
| 6 | Reflection-based native calls | ~0.9% measured — smaller than expected | N/A | N/A | Investigated, deprioritized by evidence | Not planned |
| 7 | Name-based symbol resolution | ~2-3% of total, on top of scope-creation costs | Very High | High | Architectural / VM redesign | Open |
| 8 | Per-execution basic-block scopes (if/else, function bodies, etc.) | -30% on an if/else-in-a-loop workload; -10% on a no-locals function-call workload | Medium-High | Medium (compiler analysis; surfaced BUG-61) | Same category as Finding 4, one level lower | **Fixed** (July 2026) — see Resolution above |

**Suggested order of work:** ~~1~~ → ~~2~~ → ~~3~~ → ~~4~~ → ~~8~~, in that order, re-profiling
after each change using the same `--pprof` workloads described in Section 1 to confirm the
expected compounding effect on Finding 5 before deciding whether Finding 7 is ever worth
pursuing. Findings 1-4 and 8 are all done; re-profiling after each (see their Resolution
sections) confirms each predicted direct cost was eliminated (`uuid.New` gone entirely;
`IsType` no longer appears in the profile on this path at all; `ui.Log` argument construction
is now uniformly guarded across 248 inspected call sites; `pushScopeByteCode`/
`NewChildSymbolTable` no longer appear at all in a loop with no closures or top-level
declarations, nor in an if/else nested inside one). Finding 4 delivered the largest single-fix
wall-clock improvement so far (-48% to -49% on top of Findings 1+2+3, depending on workload) —
a cumulative **63-72% reduction from the original baseline** (95.6s → 27.25s on the
50,000,000-iteration loop). Finding 8 adds a further **-30%** on top of that for code shaped
like conditionals inside loops, a pattern Finding 4 could not reach on its own. `runtime.kevent`
and `runtime.madvise` (thread scheduling and memory management) remain present in every
profile but are no longer anywhere near the dominant cost they were before Findings 4 and 8,
leaving Finding 7's architectural symbol-resolution redesign as the only remaining lever of
comparable size — at very substantially higher effort and risk than any of the fixes completed
so far.
