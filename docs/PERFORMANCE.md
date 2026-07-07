# Ego Interpreter Performance Audit

This document reports the findings of a CPU-profiling audit of the Ego bytecode interpreter
and runtime, triggered by the observation that a plain 50,000,000-iteration `for` loop (see
`examples/panic.ego`) takes on the order of 95 seconds to run. It is a survey of *where the
interpreter actually spends its time*, backed by real `pprof` profiles rather than
speculation, together with concrete recommendations ranging from one-line fixes to
significant design changes. Findings are ordered roughly by impact-for-effort, not by where
they appear in the call graph.

## Contents

- [1. Methodology](#1-methodology)
- [2. Finding 1 — `uuid.New()` on every scope creation (highest impact, lowest risk)](#2-finding-1--uuidnew-on-every-scope-creation-highest-impact-lowest-risk)
- [3. Finding 2 — `InstanceOfType`'s linear scan for every scalar type coercion](#3-finding-2--instanceoftypes-linear-scan-for-every-scalar-type-coercion)
- [4. Finding 3 — `ui.Log` call sites build their argument map before checking if logging is active](#4-finding-3--uilog-call-sites-build-their-argument-map-before-checking-if-logging-is-active)
- [5. Finding 4 (design-level) — per-iteration loop scopes: correct, but costly, and often unnecessary](#5-finding-4-design-level--per-iteration-loop-scopes-correct-but-costly-and-often-unnecessary)
- [6. Finding 5 (systemic) — garbage-collector and scheduler overhead dominates wall-clock time](#6-finding-5-systemic--garbage-collector-and-scheduler-overhead-dominates-wall-clock-time)
- [7. Finding 6 (measured, and smaller than expected) — reflection-based native calls](#7-finding-6-measured-and-smaller-than-expected--reflection-based-native-calls)
- [8. Finding 7 (architectural, high-effort, high-ceiling) — name-based symbol resolution](#8-finding-7-architectural-high-effort-high-ceiling--name-based-symbol-resolution)
- [9. Finding 8 (design-level) — per-execution basic-block scopes: same idea as Finding 4, one level lower](#9-finding-8-design-level--per-execution-basic-block-scopes-same-idea-as-finding-4-one-level-lower)
- [10. New workload — `examples/mandelbrot.ego` (a real program, not a synthetic loop)](#10-new-workload--examplesmandelbrotego-a-real-program-not-a-synthetic-loop)
- [11. Finding 9 — `data.String()` runs a full reflective `fmt.Sprintf` even when the value is already a `string` (highest impact of this study)](#11-finding-9--datastring-runs-a-full-reflective-fmtsprintf-even-when-the-value-is-already-a-string-highest-impact-of-this-study)
- [12. Finding 10 — `atLineByteCode` writes `__line`/`__module` into the symbol table on every statement, for a feature almost nothing reads](#12-finding-10--atlinebytecode-writes-__line__module-into-the-symbol-table-on-every-statement-for-a-feature-almost-nothing-reads)
- [13. Finding 11 (corroborating, extends Finding 4) — loop bodies that declare locals still pay per-iteration scope allocation, and `IsConstant` walks the full parent chain to check a brand-new name](#13-finding-11-corroborating-extends-finding-4--loop-bodies-that-declare-locals-still-pay-per-iteration-scope-allocation-and-isconstant-walks-the-full-parent-chain-to-check-a-brand-new-name)
- [14. New workload — `examples/mandelbrot2.ego` (deep recursion, up to 800 stack levels)](#14-new-workload--examplesmandelbrot2ego-deep-recursion-up-to-800-stack-levels)
- [15. Finding 12 — `SetParent`'s cycle-detection walk is dead code on the hot `NewChildSymbolTable` path (largest single cost in this workload)](#15-finding-12--setparents-cycle-detection-walk-is-dead-code-on-the-hot-newchildsymboltable-path-largest-single-cost-in-this-workload)
- [16. Finding 13 — the `__extensions` flag is redundantly round-tripped through an O(depth) symbol-table walk on every single call and return](#16-finding-13--the-__extensions-flag-is-redundantly-round-tripped-through-an-odepth-symbol-table-walk-on-every-single-call-and-return)
- [17. Finding 14 (architectural) — `FindNextScope` and `callFramePop`'s package-clone check both walk the full ancestor chain on every operation](#17-finding-14-architectural--findnextscope-and-callframepops-package-clone-check-both-walk-the-full-ancestor-chain-on-every-operation)
- [18. Summary table](#18-summary-table)

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
  exited via early `break` (confirming the map's read lock still releases), chained
  pointer-receiver calls inside an elided block, a function body with no locals called 100
  times, a function using `defer` (confirming its scope is never elided), doubly-nested `if`
  with no locals at either level, and a `conditionalFor` loop with an elided body.

**A second, unrelated bug found while writing these tests, fixed separately from this task:**
the pointer-receiver test originally chained calls that returned the receiver as its own
declared pointer type (`b.add("a").add("b")`, each `add` returning `*Builder`). Under
`--types strict` this failed with `type mismatch: Builder …, *Builder …` — root-caused to
`getThisByteCode` (`internal/language/bytecode/this.go`) discarding Ego's pointer-type marker
when it dereferences a pointer receiver for field-write propagation, so the receiver's runtime
type no longer matched its declared type once returned. Confirmed pre-existing and unrelated
to Finding 8 by reproducing it against the unmodified compiler; the test was temporarily
rewritten to call the receiver method for its mutation side effect only (no chaining/return)
so this Finding's test suite did not depend on it. Documented and subsequently fixed as
[BUG-64](ISSUES.md#BUG-64) (see its Resolution for the full two-part fix, which also covers a
second call shape — a pointer-receiver method called on a plain, non-`&` value — and a related
pre-existing bug in `loadIndexByteCode` it surfaced along the way). With BUG-64 resolved, the
pointer-receiver test here was restored to its original chained-call form.

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

## 10. New workload — `examples/mandelbrot.ego` (a real program, not a synthetic loop)

Every workload in Sections 1-9 was a small synthetic program built to isolate one specific
cost (a bare loop, a loop with a function call, a loop with a native call). This section
profiles `examples/mandelbrot.ego`, a self-contained, non-trivial example program, to check
whether the findings and fixes above generalize to something shaped like real Ego code.

The program computes a 200×200-pixel Mandelbrot set (`width`/`height` constants) with up to
1,000 escape-time iterations per pixel (`maxItr`). `Mandelbrot(cx, cy)` is an ordinary
(non-recursive) function containing a `for` loop; `GenerateSet()` calls it once per pixel from
a nested `for` loop (40,000 calls total). Unlike the tight/function-call/native-call workloads,
it exercises floating-point arithmetic throughout, a function body with several named local
scalars per call (`x`, `y`, `x2`, `y2`, `i`), and a loop body that itself declares new locals
on every iteration (`x2, y2 := x*x, y*y`) — a shape none of the earlier synthetic workloads
happened to cover. It does **not** use recursion, arrays, or matrices, despite an earlier,
inaccurate description of it — see the actual source for the exact shape being profiled.

```sh
./tools/build
./ego run --pprof /tmp/cpu.prof examples/mandelbrot.ego
go tool pprof -top -cum ./ego /tmp/cpu.prof
```

| Metric | Value |
| - | - |
| Grid / iterations | 200×200 pixels, up to 1,000 iterations/pixel |
| Reported total iterations | 9,929,677 (`GenerateSet`'s return value) |
| Wall clock (`time ego run ...`) | 27.98s-28.36s (two runs) |
| CPU samples captured | 27.09s (96.28% of wall clock) |

This run was taken against the **current** `master` — i.e., *after* all of Findings 1-4 and 8
were implemented and merged — so every cost identified below is cost that survived those five
fixes, not cost they already addressed. The findings below are new, not previously documented.

---

## 11. Finding 9 — `data.String()` runs a full reflective `fmt.Sprintf` even when the value is already a `string` (highest impact of this study)

**Impact:** **15.06% of total profiled time** (4.08s of 27.09s) is spent inside
`data.String()`, of which **3.96s — effectively all of it — is a single line**:
`return fmt.Sprintf("%v", v)`. This is the largest single item found anywhere in this
profile, and proportionally larger than any individual fix in Findings 1-4/8's original
tight-loop audit.

### Evidence - Finding 9

```text
ROUTINE ======================== github.com/tucats/ego/internal/language/data.String
     170ms      4.08s (flat, cum) 15.06% of Total
      90ms       90ms     75:func String(v any) string {
         .          .     80:   v = UnwrapConstant(v)
      10ms       10ms     87:   if t, ok := v.(time.Time); ok {
         .          .     88:       return t.Format(time.RFC822Z)
      50ms      3.96s     91:   return fmt.Sprintf("%v", v)     <-- 97% of this function's own cost
```

`fmt.Sprintf` has exactly one caller anywhere in this profile — `data.String` — confirmed by
`pprof -peek`:

```text
ROUTINE ==== fmt.Sprintf
     0.25s  0.92%  0.92%      3.91s 14.43%
                                             3.91s   100% |   github.com/tucats/ego/internal/language/data.String
```

And the callers of `data.String` are exactly the bytecode instructions that resolve a
variable's name every time they run — `loadByteCode` (62.0% of `data.String`'s calls),
`storeByteCode` (24.5%), `symbolCreateIfByteCode` (9.8%), `symbolCreateByteCode` (3.7%):

```text
ROUTINE ======================== github.com/tucats/ego/internal/language/bytecode.loadByteCode
     280ms      6.30s (flat, cum) 23.26% of Total
      30ms      2.56s     11:   name := data.String(i)          <-- 40% of loadByteCode's own total cost
         .      3.40s     16:   v, found := c.get(name)
```

```text
ROUTINE ======================== github.com/tucats/ego/internal/language/bytecode.storeByteCode
     170ms      2.45s (flat, cum)  9.04% of Total
      10ms      1.01s    173:   name = data.String(i)           <-- 41% of storeByteCode's own total cost
```

`ego run --disassemble` confirms the operand passed to every one of these instructions is
already a plain Go `string` literal baked in at compile time — e.g. `Load "Second"`,
`Load "e"`, `Load "a"`, `Load "$new"` — never a number, struct, or anything else that would
actually need general-purpose formatting.

### Root cause - Finding 9

`data.String(v any) string` (`internal/language/data/accessor.go:75-92`) is a general-purpose
"stringify anything" helper. It correctly special-cases `nil` and `time.Time`, but has **no
special case for the single most common input type: `string` itself.** Every other type falls
through to `fmt.Sprintf("%v", v)`, and so does `string` — even though `fmt.Sprintf("%v", s)`
for a `string` `s` always returns `s` unchanged. Every call pays the full cost of `fmt`'s
machinery — acquiring a pooled `pp` printer (`fmt.newPrinter`, `sync.Pool.Get`), reflecting
over the boxed argument (`doPrintf`, `printArg`), building the result through a byte buffer
(`runtime.slicebytetostring`), and returning the printer to the pool (`fmt.(*pp).free`,
`sync.Pool.Put`) — purely to hand back the exact string it was given.

Because `data.String` is the standard way every `Load`/`Store`/`SymbolCreate`/
`SymbolCreateIf` bytecode instruction converts its name operand from `any` to `string`, and
because those instructions run on essentially every variable reference and every declaration
in every Ego program, this cost scales with total statements executed — the same shape of
problem as Findings 1 and 2, just not previously visible because the earlier synthetic
workloads used only one or two named variables total and so did not stress name resolution as
heavily as a program with a dozen or more distinct locals in play.

### Recommendation - Finding 9

Add a direct `string` fast path to `data.String`, immediately after `UnwrapConstant`:

```go
func String(v any) string {
    if v == nil {
        return ""
    }

    v = UnwrapConstant(v)

    if v == nil {
        return ""
    }

    if s, ok := v.(string); ok {
        return s
    }

    if t, ok := v.(time.Time); ok {
        return t.Format(time.RFC822Z)
    }

    return fmt.Sprintf("%v", v)
}
```

This is behavior-preserving by construction (`fmt.Sprintf("%v", s) == s` for any `string s`),
requires no compiler analysis or bytecode changes, and touches exactly one function — the
same low-risk shape as Finding 1's `uuid.New()` fix. Given that the profile shows the
overwhelming majority of real-world calls into this function are already-`string` bytecode
operands, this single change is projected to eliminate close to the full measured 15% of
wall-clock time in this workload, with likely secondary reduction in GC pressure too (`fmt`'s
printer pool and intermediate byte buffer are both real allocations happening millions of
times over).

### Resolution (July 2026) — Finding 9

Implemented the recommendation above exactly as specified: a `string` type-switch case was
added to `data.String` (`internal/language/data/accessor.go`), immediately after
`UnwrapConstant` and before the existing `time.Time` case, returning the string unchanged with
no call into `fmt`.

**Files modified:**

- `internal/language/data/accessor.go` — added the `string` fast-path case to `String`, with a
  comment explaining why it is safe (`fmt.Sprintf("%v", s) == s` for any string `s`).
- `internal/language/data/accessor_test.go` (new) — `TestString` (nine cases: nil, empty
  string, plain string, a string that itself contains `fmt`-verb-like text such as `"%v %d"`
  — to confirm the fast path does not accidentally re-interpret its input as a format string,
  an `Immutable`-wrapped string — to confirm `UnwrapConstant` still runs first, plus `int`,
  `bool` in both states, and `float64`, all still routed through the unchanged `fmt.Sprintf`
  fallback) and `TestString_Time` (confirms the `time.Time`/RFC822Z case, order-independent of
  the new string case, is unaffected). No test file previously existed for this function.

**Bug found while doing this work:** none — this was a pure additive fast path with no
adjacent defect uncovered, matching Finding 2's experience more than Finding 1's.

**Correctness verification:** `go build ./...`, `go vet ./...`, and the full `go test ./...`
suite are clean, including the two new test functions above. All 1,193 tests in
`ego test tests/` continue to pass (0 failures).

**Re-profiling results:** `examples/mandelbrot.ego` (Section 10) was re-profiled with the
identical `--pprof` methodology, after rebuilding with `./tools/build`.

| Metric | Before Finding 9 | After Finding 9 | Change |
| - | - | - | - |
| Wall clock (`time ego run ...`, avg of 2 runs) | 28.17s | 21.17s | **-24.9%** |
| `data.String` (cum) | 4.08s / 15.06% | 0.04s / 0.17% | **-99%** |
| `fmt.Sprintf` (cum) | 3.91s / 14.43% | *(no longer appears in the profile at all)* | **-100%** |
| `loadByteCode` (cum) | 6.30s / 23.26% | 0.53s / 2.25% | **-92%** |
| `storeByteCode` (cum) | 2.45s / 9.04% | 0.26s / 1.10% | **-89%** |
| `symbolCreateIfByteCode` (cum) | 1.88s / 6.94% | 0.21s / 0.89% | **-89%** |
| `RunFromAddress` (cum — "true interpreter work") | 18.27s / 67.44% | 2.58s / 10.96% | **-86%** (see caveat below) |

```text
(mandelbrot, after Finding 9 — data.String no longer shows fmt.Sprintf in its subtree)
ROUTINE ======================== github.com/tucats/ego/internal/language/data.String
      40ms       40ms (flat, cum)  0.17% of Total
      30ms       30ms     75:func String(v any) string {
      10ms       10ms     80:   v = UnwrapConstant(v)
```

Every prediction from the Recommendation held: `fmt.Sprintf` is completely gone from the
profile (not merely reduced), `data.String` itself is now essentially free (0.17% vs. 15.06%),
and the bytecode instructions that were its heaviest callers (`loadByteCode`, `storeByteCode`,
`symbolCreateIfByteCode`) all dropped by roughly an order of magnitude in absolute cumulative
seconds — not just in their share of a smaller total.

**A caveat on the `RunFromAddress` and other secondary numbers.** Several costs *not* directly
touched by this fix also fell by more than call-count alone would predict — e.g.
`runtime.mapaccess2_faststr` (3.58s / 13.22% → 0.52s / 2.21%) and `symbols.IsConstant`
(1.18s / 4.36% → 0.19s / 0.81%), even though the program executes the exact same 9,929,677
iterations, calling these functions the same number of times, before and after. The most
likely explanation, consistent with Finding 5's systemic thesis, is a secondary cache-locality
effect: removing millions of `fmt.Sprintf` calls also removes their pooled-printer
acquisition, reflection-driven argument boxing, and intermediate byte-buffer allocation — a
large amount of allocation churn and cache pollution that was surrounding *every other* hot
operation in the interpreter loop, not just the `data.String` call sites themselves. With that
churn gone, the surviving hot paths (map lookups, scope pushes) appear to run measurably
faster per call too, not merely less often. This is a plausible, but not rigorously isolated,
explanation — it was not independently verified with a separate micro-benchmark — and is
recorded honestly here rather than overclaimed.

The profile's overall *shape* also changed as a direct consequence: with the dominant
CPU-bound cost gone, OS-level thread/scheduler overhead (`runtime.pthread_cond_wait` 21.10%,
`runtime.madvise` 15.03%, `runtime.pthread_cond_signal` 11.68%, `runtime.pthread_kill` 11.17%,
`runtime.kevent` 9.47%, `runtime.usleep` 6.79% — together roughly 75% of all samples) is now
overwhelmingly the largest category of remaining cost, and total CPU samples exceeded wall
clock (111.26%, vs. 96.28% before) — concurrent GC background-mark workers doing real work in
parallel with the main goroutine, rather than the same amount of GC work being paid for
synchronously inside `RunFromAddress`'s own call tree (which is why `RunFromAddress`'s
*cumulative* share fell so much more sharply than the wall clock did: much of what used to be
counted inside it, via `data.String`'s allocations, is simply gone, not relocated).

**Honest assessment:** the wall-clock improvement (-24.9%) is smaller than the near-total
elimination of the specific cost this fix targeted (`data.String` -99%, `fmt.Sprintf` -100%),
for exactly the reason Finding 5 predicts: GC/scheduler overhead was already present
underneath this cost and now makes up a larger share of what's left. That said, this is
comfortably the second-largest single-fix wall-clock improvement recorded in this report
(after Finding 4's -48/-49%), delivered by a three-line, fully mechanical change with no
compiler involvement and no behavioral risk — confirming the Section 13/14 assessment that
this was the correct next item to fix ahead of Finding 7's much higher-effort, higher-risk
architectural redesign. Findings 10 and 11 remain open, and — per the caveat above — the
remaining GC/scheduler overhead visible in this new profile is exactly the kind of cost
Finding 7 (and, to a lesser extent, Finding 11) would address, not something a further
`data.String`-shaped fix could reach.

---

## 12. Finding 10 — `atLineByteCode` writes `__line`/`__module` into the symbol table on every statement, for a feature almost nothing reads

**Impact:** `atLineByteCode` totals **4.91% (1.33s)** of this profile. Of that, **57%
(0.76s)** is inside `SymbolTable.SetAlways`, and essentially all of `SetAlways`'s own cost in
this profile (98.7%) comes from this one call site. Including the interface-boxing needed to
call it (`runtime.convTstring`, 0.22s) and its own downstream map-access cost, roughly 3-4% of
total wall-clock time in this workload goes toward bookkeeping that is read back almost never.

### Evidence - Finding 10

```text
ROUTINE ======================== github.com/tucats/ego/internal/language/bytecode.atLineByteCode
      90ms      1.33s (flat, cum)  4.91% of Total
                                             0.76s 57.14% |   symbols.(*SymbolTable).SetAlways
                                             0.22s 16.54% |   runtime.convTstring
                                             0.14s 10.53% |   data.Int
```

```text
ROUTINE ==== symbols.(*SymbolTable).SetAlways
     0.15s  0.55%  0.55%      0.77s  2.84%
                                             0.76s 98.70% |   bytecode.atLineByteCode
```

### Root cause - Finding 10

The compiler emits an `AtLine` bytecode instruction ahead of every source statement (for error
location tracking, the debugger, and the source tracer). `atLineByteCode`
(`internal/language/bytecode/flow.go:220+`) unconditionally does, on every one of those
instructions:

```go
c.symbols.SetAlways(defs.LineVariable, c.line)
c.symbols.SetAlways(defs.ModuleVariable, c.bc.name)
```

`defs.LineVariable` (`__line`) and `defs.ModuleVariable` (`__module`) are written into the
current symbol table's `map[string]*SymbolAttribute` — a map lookup, an interface-box of an
`int` and a `string`, and (when the table is shared) a lock check — on every single statement
execution. A repository-wide grep shows the **only** reader of either symbol is
`internal/runtime/errors/new.go`, which looks them up solely to attach source-location context
to a newly constructed Ego runtime error. `c.line` and `c.bc.name` are already ordinary fields
on the `Context` struct — the symbol-table copies exist purely so that the (rare) error-
construction path has somewhere to read them from.

### Recommendation - Finding 10

Have `internal/runtime/errors/new.go`'s error-construction path read `c.line`/`c.bc.name`
directly from the executing `Context` (or an equivalent lightweight channel) instead of from
the symbol table, and drop the two `SetAlways` calls from `atLineByteCode`'s hot path
entirely. If `__line`/`__module` must remain independently readable as ordinary Ego symbols
(e.g. for `util.symbols()` introspection or direct user-code lookups), a narrower alternative
is to special-case `Load` for exactly these two names to read `c.line`/`c.bc.name` directly,
still without writing them into the map on every statement. Either approach removes a
per-statement cost that scales with total statements executed, independent of and additive
with Finding 9's fix.

---

## 13. Finding 11 (corroborating, extends Finding 4) — loop bodies that declare locals still pay per-iteration scope allocation, and `IsConstant` walks the full parent chain to check a brand-new name

**Impact:** `pushScopeByteCode` (0.83s / 3.06%) + `NewChildSymbolTable` (0.73s / 2.69%) +
`popScopeByteCode` (0.31s / 1.14%) ≈ **~7% combined** — smaller than the 12%+ these functions
cost in the pre-Finding-4 tight-loop workloads (Findings 1/2 already made each individual
scope creation much cheaper), but still a real, repeated cost: roughly 9.9 million scope
allocations, matching `Mandelbrot`'s reported total-iteration count almost exactly. Separately,
inside the bytecode that creates each new loop-local, `SymbolTable.IsConstant`'s call costs
**1.04s (3.8% of total)** on its own, inside `symbolCreateIfByteCode` alone.

### Root cause - Finding 11

`Mandelbrot`'s hot inner loop declares new locals at the loop body's top level every
iteration:

```go
for i = 0; i < maxItr; i++ {
    x2, y2 := x*x, y*y
    if x2+y2 > 4.0 {
        break
    }
    ...
}
```

Finding 4's `loopBodyNeedsFreshScopePerIteration` predicate (`internal/language/compiler/for.go`)
treats any top-level `:=`/`var` in a loop body as disqualifying for scope-sharing — not
because of closure-capture risk (there is none here), but because reusing one scope across
iterations would make the second iteration's `x2, y2 := ...` collide with the first's, since
the `SymbolCreate`/`SymbolCreateIf` opcodes used for `:=` are built around "this name must not
already exist in this scope." Loops shaped like this one — declaring simple scalar locals with
no closures at all — are common, and Finding 4 explicitly, deliberately excludes them from its
optimization.

A second, distinct cost showed up investigating this: `symbolCreateIfByteCode` (the opcode
used for `x2, y2 := ...`, a multi-variable declaration) calls `c.isConstant(n)` before
creating each new local, and `IsConstant` (`internal/language/symbols/get.go:243`) walks the
**entire parent scope chain to the root**, not just the current scope, on every call. For a
loop-local variable that is, by construction, brand new in a freshly pushed scope, this walk
can never find a match — it exists purely to answer "no" as expensively as possible. It is
worth separately confirming whether this full-chain walk is semantically required here: `:=`
in Go (and, so far as this investigation could confirm, in Ego) always declares a fresh local
that *shadows* — rather than checks against — any same-named variable or constant in an
enclosing scope, which would suggest a local-only existence check ought to be sufficient for
this particular call site. This needs a more careful semantic check than this profiling study
performed before changing it, since getting it wrong would be a correctness regression, not
just a missed optimization.

### Recommendation - Finding 11

Not designed in detail here — flagged for future work, since it is more delicate than
Finding 4's original all-or-nothing scope decision:

1. For loop bodies that pass a *relaxed* version of Finding 4's predicate (no
   closure/`go`/`defer`, regardless of top-level declarations), consider keeping a single
   shared scope for the whole loop (as Finding 4 already does for closure-free, declaration-free
   bodies) and compiling the loop body's `:=`/`var` declarations to *overwrite* the previous
   iteration's slot instead of allocating an entirely new `SymbolTable` each time. This needs
   care around any case where a declaration's reachability differs between iterations (e.g. a
   `:=` inside a conditionally-taken branch of the loop body) and around whether a slot's
   type/readonly metadata needs to be reset on redeclaration.
2. Independently, investigate whether `symbolCreateIfByteCode`'s (and `symbolCreateByteCode`'s)
   `c.isConstant(n)` check needs to walk the parent chain at all, or whether a `GetLocal`-style
   current-scope-only check is sufficient for the "is this new declaration name already a
   constant *in this scope*" question `:=` actually needs answered.

### Also corroborates Finding 7

This workload independently reinforces the open, architectural Finding 7 (name-based symbol
resolution). The earlier tight-loop/function-call/native-call workloads used essentially one
named variable throughout; `Mandelbrot` uses over a dozen distinct scalar locals across its two
functions (`x`, `y`, `x2`, `y2`, `i`, `cx`, `cy`, `dx`, `dy`, `minX`, `minY`, `maxX`, `maxY`,
`totalIterations`). With more distinct names in play, `runtime.mapaccess2_faststr` becomes the
single largest interpreter-owned cost category in this profile after Finding 9's `data.String`
issue — **13.22% (3.58s) cumulative**, split across `symbols.Get` (53.1% of those calls),
`symbols.IsConstant` (19.3%), `symbols.Set` (17.9%), and `symbols.SetAlways` (8.1%). This does
not change Finding 7's cost/effort/risk assessment (still "Open," still "Very High" effort),
but is worth recording as independent evidence, from a second and differently-shaped workload,
that the bottleneck it describes generalizes beyond single-variable synthetic loops.

**Update, post-Finding-9-fix:** after Finding 9 was implemented (see its Resolution section),
this same `runtime.mapaccess2_faststr` cost fell further still, from 3.58s/13.22% to
0.52s/2.21% — a larger drop than call-count alone would predict, for the reasons discussed in
Finding 9's Resolution (likely reduced cache pollution from `fmt.Sprintf`'s removed
allocations). The figures above are left as originally measured (pre-Finding-9) since they are
still the correct evidence for Finding 7's *existence*; they should not be read as Finding 7's
current, post-Finding-9 cost.

---

## 14. New workload — `examples/mandelbrot2.ego` (deep recursion, up to 800 stack levels)

`examples/mandelbrot.ego` (Section 10) computes the same Mandelbrot set iteratively, with no
recursion. `examples/mandelbrot2.ego` computes it with **pure recursion** instead: each pixel's
escape-time calculation is one call to `mandelIterate(zr, zi, cr, ci, iter)`, which either hits
a base case (escape or `iter >= MaxIter`) or tail-calls itself with `iter+1` — meaning a single
pixel that never escapes recurses **800 levels deep** (`MaxIter = 800`), 3,200 times over (an
80×40 grid). This workload was profiled specifically to answer a question the earlier,
iteration-only workloads could not: **does the interpreter's function-call machinery have costs
that scale with call-stack depth, not just call count?**

```go
func mandelIterate(zr, zi, cr, ci float64, iter int) int {
    if zr*zr+zi*zi > 4.0 {
        return iter
    }
    if iter >= MaxIter {
        return MaxIter
    }
    nextZr := zr*zr - zi*zi + cr
    nextZi := 2*zr*zi + ci
    return mandelIterate(nextZr, nextZi, cr, ci, iter+1)
}
```

```sh
./ego run --pprof /tmp/cpu.prof examples/mandelbrot2.ego
go tool pprof -top -cum ./ego /tmp/cpu.prof
```

| Metric | Value |
| - | - |
| Grid | 80×40 pixels, up to 800 recursive calls/pixel |
| Wall clock (`time ego run ...`) | ~34.1s (three runs: 34.11s, 34.12s, 34.65s user+sys) |
| CPU samples captured | 31.98s of 34.29s duration (93.25%) |

**The answer is an emphatic yes**, and the effect is far larger than anything found in the
iterative workloads: **four functions, all of which walk the symbol table's ancestor chain
from the current scope to (or toward) the root, together account for 70.76% of all CPU-flat-time
in this profile** — 22.63s of 31.98s, in functions with no other purpose:

| Function | Flat time | Flat % | Triggered by |
| - | - | - | - |
| `symbols.(*SymbolTable).SetParent` | 10.18s | 31.83% | Every function call (`NewChildSymbolTable`) |
| `symbols.(*SymbolTable).FindNextScope` | 8.85s | 27.67% | Every non-local variable read/declaration (`Get`, `IsConstant`) |
| `symbols.(*SymbolTable).IsRoot` | 2.34s | 7.32% | Every function return (`Root`, called from `callFramePop`) |
| `symbols.(*SymbolTable).IsClone` | 1.26s | 3.94% | Every function return (`updatePackageFromLocalSymbols`, called from `callFramePop`) |

Each of these four is an **O(depth) operation**, and each fires at least once per function
call or return. Because `mandelIterate`'s scope chain is exactly as deep as its current
recursion depth (every call's own scope is chained onto its caller's — see Finding 12's root
cause for why), the *total* cost of walking that chain across one complete top-to-bottom
recursive descent to depth *N* is O(1 + 2 + ... + N) = **O(N²)**, not O(N). This is a
fundamentally different — and much more severe — cost shape than anything the earlier,
non-recursive workloads exposed: a loop with 10,000,000 flat iterations and a function that
recurses 800 levels deep can cost the interpreter comparably, because it isn't the *number* of
calls that dominates here, it's the *depth* each one pays for.

Three new findings below (12, 13, 14) each target one distinct piece of this. Two (12, 13) are
mechanical, low-risk, high-confidence fixes with a clear "this call is provably unnecessary"
argument, in the same spirit as Findings 1, 2, and 9. The third (14) is the harder,
architectural piece — the part of this cost that is legitimately doing correctness-required
work, just doing it the slow way.

---

## 15. Finding 12 — `SetParent`'s cycle-detection walk is dead code on the hot `NewChildSymbolTable` path (largest single cost in this workload)

**Impact:** **31.83% of total profiled time** (10.18s flat of 31.98s) is spent inside
`SymbolTable.SetParent`, and essentially all of it (10.18s of 10.60s cum) is a single loop
whose only job is to detect a cycle that **cannot exist** at this call site. This is, by flat
time, the single largest cost found anywhere in this report — larger than Finding 9's
`data.String`/`fmt.Sprintf` cost was in the iterative workload.

### Evidence - Finding 12

```text
ROUTINE ======================== github.com/tucats/ego/internal/language/symbols.(*SymbolTable).SetParent
    10.18s     10.60s (flat, cum) 33.15% of Total
         .          .    402:func (s *SymbolTable) SetParent(p *SymbolTable) *SymbolTable {
         .          .    418:   // Chase the parent chain from the new parent to make sure this symbol table
         .          .    419:   // is not already in the loop.
         .          .    420:   chain := p
     2.34s      2.46s    422:       if chain == s {
     7.84s      8.04s    426:       chain = chain.parent
         .          .    427:   }
```

`SetParent` has exactly two callers in the whole codebase: `NewChildSymbolTable`
(`internal/language/symbols/tables.go:184`) and one compile-time call in
`internal/language/compiler/compiler.go:474`. Every function call and every scope push in a
running Ego program goes through the first one — `NewChildSymbolTable` constructs a brand-new
`SymbolTable{...}` value and, on the very next line, calls `symbols.SetParent(parent)` before
that new table has been returned to, or become reachable from, anywhere else in the program.

### Root cause - Finding 12

```go
func NewChildSymbolTable(name string, parent *SymbolTable) *SymbolTable {
    symbols := SymbolTable{ ... }   // brand new, not yet reachable from anywhere
    ...
    symbols.SetParent(parent)       // <-- walks parent's ENTIRE existing chain
    ...
    return &symbols
}
```

`SetParent`'s loop walks from the *new parent* (`p`) up through its entire existing ancestor
chain, checking whether the *table being reparented* (`s`) already appears in it — a genuine
and reasonable defensive check *in general*, since `SetParent` is a public method that could,
in principle, be called to reattach an already-referenced table into a position that creates a
cycle. But at the `NewChildSymbolTable` call site, `s` (the freshly constructed `symbols`
value) has existed for exactly one line of code and has not been assigned to any field,
returned, or stored anywhere — it is *impossible* for it to already appear in `parent`'s chain,
because that chain was entirely built before this line ever ran. The check can never fire here;
it exists purely to protect the *other*, rare, compile-time call site.

Because this new child table's parent (`c.symbols`, the caller's own scope) is chained
directly onto the caller's own parent, and so on up through every enclosing call frame, the
length of this walk is exactly the current call-stack depth. For `mandelIterate`'s deepest
calls (depth ~800), each new call pays a ~800-entry walk just to prove something that was
already structurally guaranteed by construction — and it pays a proportionally shorter walk at
every shallower depth too, which is why the *total* cost across one recursive descent is O(N²).

### Recommendation - Finding 12

Do not change `SetParent` itself — the compiler.go:474 call site is rare (compile-time, not
per-call) and may legitimately want the safety check. Instead, give `NewChildSymbolTable` a
way to set `parent`/`isRoot` directly, without the cycle check or the `Lock()`/`Unlock()` pair
`SetParent` also does (also unnecessary here, since `symbols` is a stack-local value no other
goroutine can see yet):

```go
symbols.parent = parent
symbols.isRoot = (parent == nil)
```

replacing the `symbols.SetParent(parent)` call in `NewChildSymbolTable`. This is a
mechanically verifiable, behavior-preserving change with the same low-risk shape as Finding 1
(`uuid.New()`) and Finding 9 (`data.String`'s `fmt.Sprintf` fallback): a defensive check that is
correct in general but provably dead weight at the one call site responsible for virtually all
of its cost. Given the profile shows this is the single largest cost in the recursive
workload — and that its cost is structurally tied to call-stack depth, not just call count —
this is the highest-priority item surfaced by this study.

### Resolution (July 2026) — Finding 12

Implemented the recommendation above exactly as specified: `NewChildSymbolTable`
(`internal/language/symbols/tables.go`) now sets `symbols.parent` and `symbols.isRoot`
directly, with a comment explaining why `SetParent`'s cycle check is unreachable here.
`SetParent` itself was left completely unchanged — its one other caller
(`internal/language/compiler/compiler.go:474`, a rare, compile-time reparenting call) still
gets the full cycle-detection walk and locking.

**Files modified:**

- `internal/language/symbols/tables.go` — replaced `symbols.SetParent(parent)` with direct
  `symbols.parent = parent; symbols.isRoot = (parent == nil)` assignments in
  `NewChildSymbolTable`, plus a comment recording why this is safe.

**Bug found while doing this work:** none — this was a pure call-site substitution with no
adjacent defect uncovered, matching Finding 2's and Finding 9's experience.

**Correctness verification:** `go build ./...`, `go vet ./...`, and the full `go test ./...`
suite are clean, including the existing `SetParent` cycle-detection tests in
`internal/language/symbols/symbols_test.go` and `tables_test.go` (which call `SetParent`
directly and are unaffected, since `SetParent` itself was not touched). All 1,193 tests in
`ego test tests/` continue to pass (0 failures). Symbol table trace logging
(`ego --log symbols run ...`) was manually re-inspected: the `"Setting parent of table ..."`
message (`SetParent`'s own log line) no longer appears for ordinary scope pushes and function
calls, since those no longer go through `SetParent` — but it still appears, correctly, for the
handful of package-registration tables that go through the compiler's direct `SetParent` call
(confirmed with a small test program: 15 `"push symbol table"` events during execution, 0 of
which now log `"Setting parent"`, versus 6 `"Setting parent"` events at program-load time for
package/root table registration, unchanged from before this fix).

**Re-profiling results:** `examples/mandelbrot2.ego` (Section 14) was re-profiled with the
identical `--pprof` methodology, after rebuilding with `./tools/build`.

| Metric | Before Finding 12 | After Finding 12 | Change |
| - | - | - | - |
| Wall clock (`time ego run ...`, avg of 2 runs) | 34.1s | 20.26s | **-40.6%** |
| `SetParent` (flat) | 10.18s / 31.83% | *(no longer appears in the profile at all)* | **-100%** |
| Total CPU samples captured | 31.98s / 34.29s (93.25%) | 19.17s / 20.47s (93.66%) | -40.0% |

```text
(mandelbrot2, after Finding 12 — SetParent no longer appears anywhere in the profile)
$ go tool pprof -top ./ego cpu.prof | grep -i setparent
(no output)
```

Every prediction from the Recommendation held: `SetParent` is completely gone from the
profile — not merely reduced — and the wall-clock improvement (-40.6%) tracks its targeted
cost's elimination even more closely than Finding 9's did on the iterative workload, consistent
with Finding 5's general thesis that removing an allocation-adjacent, synchronously-executed
cost (here, a lock acquire/release plus an O(depth) chain walk on every single scope push)
tends to reduce downstream GC/scheduler pressure roughly in proportion, not just the function's
own share.

**A secondary effect, consistent with Finding 9's Resolution.** Several other costs *not*
directly touched by this fix also fell in absolute terms, not just in their share of a smaller
total — e.g. `FindNextScope` (8.85s flat → 7.59s flat, -14%) and `symbols.Get` (5.16s cum →
4.16s cum, -19%) — even though `mandelIterate` executes the exact same number of calls, at the
exact same depths, before and after. As with Finding 9, the most likely explanation is reduced
cache pollution: removing millions of lock acquisitions and chain walks from the hot call/push
path leaves less allocation and memory-access churn surrounding the *other* hot operations in
the interpreter loop. This was not independently isolated with a micro-benchmark and is
recorded honestly as a plausible, not rigorously proven, secondary effect.

**Findings 13 and 14 remain fully open after this fix**, and now represent a *larger* share of
a *smaller* total: `FindNextScope` alone is now 40.95% of all profiled time in this workload
(up from 28.52% before, because the pool it's measured against shrank by 40%, while its own
absolute cost fell only 14%). The combined cost of `FindNextScope`, `Root`/`IsRoot` (Finding 13),
and `updatePackageFromLocalSymbols`/`IsClone` (Finding 14) is now **59.5%** of total profiled
time (11.40s of 19.17s) — reinforcing that Findings 13 and 14 are the next highest-value targets
in this workload, exactly as the Summary table (Section 18) already recommended.

**Honest assessment:** at -40.6%, this is the single largest wall-clock improvement measured on
any *workload-specific* baseline in this report (Finding 4's -48/-49% was larger, but measured
on the original synthetic tight-loop/function-call workloads, not the Mandelbrot programs) —
delivered by a two-line, mechanically verifiable change with no compiler involvement, no
behavioral risk, and no adjacent bug. It confirms the Section 14 assessment that deep recursion
exposes cost shapes (O(depth) work paid on every call, compounding to O(depth²) per descent)
that the earlier, non-recursive workloads could never have surfaced, and that this class of
fix — "a chain-walk that's structurally guaranteed to find nothing, done anyway" — is both
unusually cheap to fix and unusually expensive to leave in place once a program recurses more
than a few levels deep.

---

## 16. Finding 13 — the `__extensions` flag is redundantly round-tripped through an O(depth) symbol-table walk on every single call and return

**Impact:** **~15.4% of total profiled time** (4.93s: 2.46s on call entry + 2.47s on call
return) is spent reading and writing one boolean flag — whether Ego language extensions are
enabled — through the symbol table on every function call and return, even though the *same*
value is already tracked directly, and correctly, as a field on the executing `Context`.

### Evidence - Finding 13

Read side — `callByteCode`, which runs at the top of *every* `Call` instruction:

```text
ROUTINE ======================== github.com/tucats/ego/internal/language/bytecode.callByteCode
         0      6.11s (flat, cum) 19.11% of Total
         .      2.46s     71:   if v, found := c.symbols.Get(defs.ExtensionsVariable); found {
         .      3.60s    189:       return callBytecodeFunction(c, function, args)
```

Write side — `callFramePop`, which runs at the end of *every* function return:

```text
ROUTINE ======================== github.com/tucats/ego/internal/language/bytecode.(*Context).callFramePop
     340ms      5.05s (flat, cum) 15.79% of Total
         .      2.50s    154:   for st := c.symbols; st != nil; st = st.Parent() {
         .          .    155:       updatePackageFromLocalSymbols(c, st)
         .          .    156:   }
         .          .    176:   c.extensions = callFrame.extensions
         .      2.47s    177:   c.symbols.Root().SetAlways(defs.ExtensionsVariable, c.extensions)
```

Line 176 already does the correct, O(1) thing — copies the saved flag from the call frame back
onto the Context. Line 177, immediately after, then pays an O(depth) `Root()` walk (itself
built on the same O(depth) `IsRoot()` loop shown in Section 14's table) to write that exact
same value into the symbol table — apparently so that `callByteCode`'s line 71, on the *next*
call, can read it back out again, at the cost of another O(depth) walk through `Get`'s
`FindNextScope` fallback.

### Root cause - Finding 13

`c.extensions` is already the Context's live, authoritative record of whether extensions are
enabled — it is read directly (no symbol table involved) at half a dozen other call sites
throughout the `bytecode` package (`member.go:167`, `math.go:55/384/400/570`,
`symbols.go:240/291/316`). `callByteCode`'s line 71 is the outlier: instead of reading
`c.extensions` directly, it re-derives the identical value via `c.symbols.Get(defs.ExtensionsVariable)`,
which — since this name is never local to a function's own scope — always falls through to
`FindNextScope`'s O(depth) walk. `callFramePop`'s line 177 exists, as far as this
investigation could determine, only to keep that symbol-table copy fresh for line 71 to read.

This value **does** need to live in the symbol table for other reasons — `internal/builtins/length.go`,
`internal/builtins/functions.go`, and the compiler's directive handling
(`internal/language/compiler/directives.go:599`) all read or write
`defs.ExtensionsVariable` through a `*symbols.SymbolTable`, in places that do not have a
`*bytecode.Context` available. In particular, an `@extensions` directive embedded in Ego source
compiles to a `StoreGlobal defs.ExtensionsVariable` instruction (`directives.go:599`), meaning
extensions really can be toggled **mid-program**, at runtime — so simply deleting the
symbol-table copy is not safe without also making sure a `Context`'s cached `c.extensions`
still picks up that toggle.

### Recommendation - Finding 13

1. Change `callByteCode`'s line 71 to read `c.extensions` directly instead of calling
   `c.symbols.Get(defs.ExtensionsVariable)`. This alone removes the read-side O(depth) walk
   for the overwhelmingly common case (extensions setting unchanged since the `Context` was
   created).
2. Update `storeGlobalByteCode` (`internal/language/bytecode/store.go:426`, which implements
   the `StoreGlobal` opcode `@extensions` compiles to) to also set `c.extensions` when the name
   being stored is `defs.ExtensionsVariable`, so a mid-program `@extensions` toggle still takes
   effect immediately for the executing `Context`, without needing `callByteCode` to re-read
   the symbol table on every subsequent call.
3. With (1) and (2) in place, `callFramePop`'s line 177 write-back becomes unnecessary for the
   `bytecode.Context` code path and can likely be removed too — but this needs a check first
   that nothing else expects the *root symbol table's* copy of `__extensions` to be refreshed
   on every return specifically (as opposed to only when the Context is created or the value is
   explicitly toggled), since `internal/builtins/length.go` and `internal/builtins/functions.go`
   read it from an arbitrary `*symbols.SymbolTable`, not from a `Context`.

This is a smaller, more surgical fix than Finding 12 and touches slightly more call sites, but
each piece is independently easy to verify, and the underlying claim — "this value is already
correctly tracked elsewhere; the symbol-table round-trip on the hot path is redundant" — is the
same shape of argument as Findings 1, 2, 9, and 12.

### Resolution (July 2026) — Finding 13

Implemented recommendations (1) and (2) above. Recommendation (3) — removing `callFramePop`'s
write-back entirely — was investigated and **deliberately not implemented**, because the check
called for in (3) turned up a real correctness dependency on it.

**Why the write-back had to stay.** `callFramePop`'s comment ("Restore the setting for
extensions, both in the context and in the global table") describes real, load-bearing
behavior, not just cache-freshening: an `@extensions` directive can appear *inside* a function
body, not only at file scope (`directives.go` emits a `StoreGlobal defs.ExtensionsVariable` for
it wherever it appears), so a callee can toggle extensions mid-execution. `c.extensions` is
correctly scoped to unwind that toggle on return (`callFrame.extensions`, saved at call time, is
copied back at line 176) — but `internal/builtins/length.go` and
`internal/builtins/functions.go` read `defs.ExtensionsVariable` from an arbitrary
`*symbols.SymbolTable`, not from a `Context`, and have no way to observe `c.extensions` at all.
Without line 177's write-back, a callee's mid-function toggle would correctly un-apply to
`c.extensions` on return, but would **leak** into the symbol table for any code reachable only
through those two builtins-package call sites, until something else happened to overwrite it.
Removing the write-back would trade a real, if narrow, correctness risk for a second O(depth)
walk's worth of savings — not a trade worth making without a broader redesign of how
`defs.ExtensionsVariable` is tracked (e.g., giving those two builtins a way to reach the owning
`Context` instead of an arbitrary symbol table). That redesign is out of scope for this fix and
is not recommended as a follow-up unless Finding 14's broader architectural work is undertaken
anyway.

**Files modified:**

- `internal/language/bytecode/call.go` — `callByteCode` now reads `c.extensions` directly
  instead of calling `c.symbols.Get(defs.ExtensionsVariable)`, with a comment explaining why
  this is safe and cross-referencing how `c.extensions` is kept in sync.
- `internal/language/bytecode/store.go` — `storeGlobalByteCode` now also updates `c.extensions`
  whenever the name being stored is `defs.ExtensionsVariable`, so a mid-program `@extensions`
  toggle (which compiles to exactly this opcode) still takes effect immediately for
  `callByteCode`'s new direct read.
- `internal/language/bytecode/callframe.go` — unchanged; `callFramePop`'s write-back (line 177)
  was kept, per the correctness finding above.

**Bug found while doing this work:** none — both changes are additive/substitutive with no
adjacent defect uncovered. The correctness dependency discovered while investigating
recommendation (3) was pre-existing design intent (as documented by the code's own comment), not
a bug — it just meant one part of the original three-part recommendation should not be carried
out as proposed.

**Correctness verification:** `go build ./...`, `go vet ./...`, and the full `go test ./...`
suite are clean. All 1,193 tests in `ego test tests/` continue to pass (0 failures), including
`tests/flow/panic_recover.ego`, which uses a file-scoped `@extensions true` directive. Manually
verified a mid-function `@extensions true` directive (toggling extensions inside a function
body, then calling a variadic function both inside and after that function returns) continues
to produce correct argument-count behavior in both positions.

**Re-profiling results:** `examples/mandelbrot2.ego` (Section 14) was re-profiled with the
identical `--pprof` methodology, comparing against the post-Finding-12 baseline (Finding 12 was
already implemented and re-profiled first).

| Metric | After Finding 12 only | After Finding 12 + 13 | Change |
| - | - | - | - |
| Wall clock (`time ego run ...`, avg of 2 runs) | 20.26s | 17.27s | **-14.8%** |
| `callByteCode` (cum) | 2.07s / 10.80% | 0.21s / 1.29% | **-90%** |
| `c.symbols.Get(defs.ExtensionsVariable)` call site (call.go:71) | 2.46s (pre-Finding-12 measurement; see Finding 13's own Evidence) | *(line removed — no longer exists)* | **-100%** |
| `symbols.Get` (cum) | 4.16s / 21.70% | 2.37s / 14.58% | -43% |
| `Root`/`IsRoot` (cum, write-side — unchanged as designed) | 2.54s / 13.25% | 2.20s / 13.54% | ~flat |

`callByteCode`'s own cumulative cost dropped by 90% — from 2.07s to 0.21s — confirming the read
side is now essentially free, exactly as predicted. `Root`/`IsRoot` (the write-side, in
`callFramePop`) stayed essentially flat in absolute terms, as expected, since that call site was
deliberately left unmodified.

**Honest assessment:** the wall-clock improvement (-14.8%) is smaller than Finding 12's
(-40.6%), consistent with this fix removing only *half* of the two-sided round-trip Finding 13
originally identified (the read side; ~7.7% of the pre-Finding-12 profile) rather than the full
~15.4%. This is the expected, correctly-scoped outcome once the correctness investigation ruled
out removing the write side too — a smaller but still real and safe win, delivered without
taking on the risk the full original recommendation would have required. Findings 13's
remaining, unaddressed cost (the write-side `Root()`/`IsRoot()` walk in `callFramePop`) is now
effectively folded into Finding 14's territory: it is architecturally the same shape of problem
(an O(depth) ancestor-chain walk on every return), and any future fix to it should be considered
alongside Finding 14's broader `FindNextScope`/`callFramePop` redesign, not as a standalone
follow-up to this fix.

---

## 17. Finding 14 (architectural) — `FindNextScope` and `callFramePop`'s package-clone check both walk the full ancestor chain on every operation

**Impact:** **28.52% of total profiled time** (9.12s cum) in `FindNextScope` itself, split
between `Get` (50.66% of calls into it) resolving non-local names like the package-level
`MaxIter` constant, and `IsConstant` (49.34%) checking whether a brand-new `:=` declaration
(`nextZr`, `nextZi`) collides with an existing constant. A further **7.82%** (2.50s cum) is
`callFramePop`'s own, separate full-chain walk checking whether any ancestor scope is a
modified package clone that needs writing back. Unlike Findings 12 and 13, this cost is not a
provably-dead check — it is doing correctness-required work, just paying an O(depth) cost to do
it, every single time, with no memoization.

### Evidence - Finding 14

```text
ROUTINE ======================== github.com/tucats/ego/internal/language/symbols.(*SymbolTable).FindNextScope
     8.85s      9.12s (flat, cum) 28.52% of Total
         .          .    249:   p := s.parent
         .          .    250:   lastBoundaryParent := p
      20ms       20ms    252:   for p != nil {
     4.58s      4.68s    254:       if p.forPackage != "" {
         .          .    255:           return p
         .          .    256:       }
     2.44s      2.54s    258:       if p.boundary && p.parent != nil {
         .          .    259:           lastBoundaryParent = p.parent
         .          .    260:       }
     1.80s      1.87s    262:       p = p.parent
         .          .    263:   }
```

```text
ROUTINE ======================== github.com/tucats/ego/internal/language/bytecode.(*Context).callFramePop
      10ms      40ms    154:   for st := c.symbols; st != nil; st = st.Parent() {
     290ms      2.50s    155:       updatePackageFromLocalSymbols(c, st)
         .          .    156:   }
```

### Root cause - Finding 14

**`FindNextScope`.** Every function-call scope is marked `boundary = true` (so that ordinary
lexical lookups stop at the caller's own locals rather than leaking into them — correct,
intentional scoping). But its own scope's *parent* pointer is chained onto the *caller's*
scope (`callFramePush` uses `c.symbols` — the caller — as the new table's parent), which means
the chain of ancestor `SymbolTable`s is isomorphic to the dynamic call stack, not to lexical
nesting. So whenever code inside a deeply-recursed function references a name that isn't
local (here: the package-level `MaxIter` constant, or checking whether a brand-new local
shadows an existing constant), resolving "what's the next *lexically* visible scope past all
these call-stack boundaries" requires walking past every intervening call frame — one by one —
to reach the package/global scope, even though, for a single connected run of recursive calls
to the *same* function, the answer is always the *same* scope.

**`callFramePop`'s clone-check loop.** Separately, `callFramePop` walks `c.symbols` all the way
to `nil` via `st.Parent()` (not just to `callFrame.symbols`, the *one* scope actually being
restored to), calling `updatePackageFromLocalSymbols(c, st)` at every level, to check whether
any ancestor is a modified clone of an imported package whose exported values need writing
back. For an ordinary function-local scope, `updatePackageFromLocalSymbols`'s own first line
(`if !st.IsClone() || !st.IsModified() { return }`) exits immediately — but the *loop* still
visits every ancestor to find that out, on every single return, regardless of how many
(typically one) scopes this particular pop operation is actually discarding. It was not
possible to determine, within this profiling study, why the loop's bound is "everything up to
nil" rather than "everything from `c.symbols` down to `callFrame.symbols`" (the exact set of
scopes being torn down by this one pop) — this needs a closer semantic read of the "call frames
we are popping off" comment before changing it, since getting the bound wrong could silently
break package-value write-back rather than just cost more CPU.

### Recommendation - Finding 14

The three-item sketch originally written here (cache `FindNextScope` per-table; inherit that
cache across call frames; bound `callFramePop`'s clone-check loop) has since been followed up
with a deeper read of the compiler and runtime code each item touches, specifically to answer
the "needs a closer semantic read before changing" caveats the sketch left open. That deeper
read turned up real, concrete obstacles for two of the three items — not hypothetical
caution, but specific call sites that would break if the original wording were implemented
literally. What follows is a revised, phased implementation plan that keeps the two items that
held up, redesigns the one that needed it to stay safe, and explicitly declines the one that
turned out to be substantially riskier than first scoped.

#### Phase 1 — bound `callFramePop`'s clone-check loop (low risk, mechanical)

**What.** Change the loop in `callFramePop` (`internal/language/bytecode/callframe.go:154`)
from walking `c.symbols` all the way to `nil` to stopping as soon as it reaches
`callFrame.symbols` (inclusive), falling through to `nil` only if that table is never reached:

```go
for st := c.symbols; st != nil; st = st.Parent() {
    updatePackageFromLocalSymbols(c, st)

    if st == callFrame.symbols {
        break
    }
}
```

**Why this is safe.** Two things had to be confirmed before trusting this bound, and both
checked out:

- *The loop really can be discarding more than one scope at once*, which is why it can't simply
  stop at the first ancestor. `internal/language/compiler/return.go` emits only `RunDefers` and
  `Return` for a `return` statement — it does **not** emit a matching `PopScope` for every
  enclosing `if`/block the `return` happens to be nested inside. So `c.symbols` at the moment
  `callFramePop` runs can legitimately be several un-popped block scopes deeper than
  `callFrame.symbols` (the scope that was active when the call was *made*), confirming the
  existing comment's "call frame**s** we are popping off" is deliberate, plural phrasing, not
  a typo. The fix has to walk from `c.symbols` down through all of those, which the revised loop
  still does — it just stops once it reaches the boundary of what this pop operation actually
  discards, instead of continuing into the *caller's* own still-active ancestors, which were
  already checked when each of them was pushed and will be checked again, correctly, on their
  own eventual pop.
- *The bound degrades safely when it can't be met.* For a function literal with a captured scope
  (a closure — `callBytecodeFunction`'s `function.capturedScope != nil` branch), the pushed
  table's parent is the scope where the closure was *defined*, not `c.symbols` at call time —
  so `callFrame.symbols` (saved as the caller's `c.symbols` at push time) is generally **not**
  reachable by walking `.Parent()` from inside the closure's own execution; the two chains
  diverge immediately. In that case the revised loop simply walks to `nil`, exactly as today's
  unbounded loop already does — so the fix is a strict improvement for the common case (ordinary
  named-function calls, which is 100% of the recursive Mandelbrot workload) and a no-op, not a
  regression, for the closure case.

**Also verified:** `callFramePop` is called from three places —
`internal/language/bytecode/return.go` (normal return), `catch.go` (try/catch unwind), and
`panic.go` (panic unwind, in a loop that pops one frame per iteration even when unwinding many
frames in a row). All three call it exactly once per discarded call frame with a fresh
`callFrame` value each time, so the fix applies uniformly with no special-casing needed for any
of the three.

#### Phase 2 — cache the program's root table on `Context` (low risk, mechanical)

**What.** Add a `rootSymbols *symbols.SymbolTable` field to `Context`, populated once in
`NewContext` (`internal/language/bytecode/context.go`), which already computes `s.Root()` once
at line 330 to read the initial extensions setting — that same result is simply worth keeping
instead of discarding. Change the two hot-path `.Root()` call sites to use `c.rootSymbols`
directly instead of re-walking:

- `callframe.go:177` — `c.symbols.Root().SetAlways(defs.ExtensionsVariable, c.extensions)`
  (the write-side cost intentionally left in place by Finding 13's Resolution).
- `store.go:455`/`457` — `storeGlobalByteCode`'s two `c.symbols.Root().SetAlways(...)` calls
  (Finding 13 added a third `.Root()`-adjacent read to this function's territory; while here,
  `context.go:476`'s `SetGlobal` and `symbols.go:376`'s `syncPackageSymbols` are lower-frequency
  call sites that can pick up the same cached field for consistency, though they are not
  contributing measurably to the profiled cost).

**Why this is safe.** A table's root is architecturally invariant for the life of a running
`Context` — confirmed by checking every place a table's ancestry can change after construction.
`SetParent` has exactly two callers: `NewChildSymbolTable` (always on a freshly-constructed,
not-yet-reachable table — see Finding 12) and one call in
`internal/language/compiler/compiler.go:474` (`SetRoot`), which reparents the *compiler's own*
working table during compiler setup, before any bytecode from that compilation has executed —
never during a running program's execution. Also confirmed that every child `Context` created
during execution — deferred-statement contexts (`internal/language/bytecode/defer.go`,
`NewContext(deferTask.symbols, ...)`) and goroutine contexts
(`internal/language/bytecode/goroutine.go`, built from `parentSymbols.SharedParent()`) — is
always constructed from a descendant of the *same* original chain as the context that spawned
it, so every `Context` alive during one program run shares one root. Re-deriving it once per
`Context` (cheap, and far less frequent than once per call frame) rather than once per
`callFramePop` is a straightforward win with no case found where it doesn't hold.

**Caveat carried forward.** This phase rests on the same "ancestry doesn't change during
execution" invariant Finding 12 already relied on for its own, narrower claim (that a
freshly-constructed table's parent can't yet be reachable from anywhere). Phase 2 leans on the
same invariant more broadly — that *no* live table's ancestry changes during execution, not
just a brand-new one's. The check above didn't find a counter-example, but this is flagged
explicitly as the load-bearing assumption to keep in mind if a future change ever needs to
reparent a table that's already in use.

#### Phase 3 — memoize `FindNextScope`'s answer per boundary table (moderate risk, needs care)

**What.** Add a lazily-populated cache to `SymbolTable` — conceptually
`nextScope *SymbolTable` plus a populated flag (or a sentinel/`sync.Once`-style pattern) —
filled in the first time `FindNextScope()` computes an answer for a given table instance, and
returned directly on every subsequent call against that same instance. This does not change
*what* gets computed, only how many times: `mandelIterate` calls into `FindNextScope` multiple
times per single invocation today (once resolving `MaxIter` via `Get`, once each for the
`nextZr`/`nextZi` declarations via `IsConstant`), and per-table memoization turns the second and
third of those into an O(1) cache hit instead of a repeat O(depth) walk, without requiring any
reasoning about how one frame's answer relates to another's.

**Obstacle found — mutation after construction.** The original sketch assumed a table's
`.boundary`/`.parent` are effectively fixed once `FindNextScope` might first be called against
it. That assumption does not hold universally; two confirmed counter-examples:

- `internal/language/bytecode/defer.go` (lines 116 and 159) calls `deferTask.symbols.Boundary(false)`
  on a table that was captured earlier — potentially the function's own call-boundary scope —
  and may already have had `FindNextScope()` (and, under this phase, a cached answer) computed
  against it during the function's normal execution, before the deferred call runs with the
  `.boundary` flag flipped.
- `internal/language/compiler/compiler.go:474` calls `.SetParent()` on an existing,
  compiler-owned table (see Phase 2) — rare and compile-time-only today, but a public method
  with no documented "only call this before first use" contract.

**How the plan addresses it:** rather than documenting "don't call `.Boundary()`/`.SetParent()`
on a table after it's been used" as a convention every future call site has to remember, make
`.Boundary(flag)` and `.SetParent(p)` themselves clear the cache field whenever they run. This
makes correctness structural — the cache can never be observed in a stale state relative to the
fields it was computed from, regardless of whether a given mutation happens before or after the
table's first use, and regardless of whether a future contributor adding a new call site knows
about the cache at all.

**Obstacle found — concurrent access.** `internal/language/bytecode/goroutine.go` attaches a
spawned goroutine's own function table to `parentSymbols.SharedParent()` — the nearest ancestor
already marked `shared`. That means a table reachable from more than one goroutine's own chain
is a real, not theoretical, possibility for any program that starts a `go` statement from inside
a function whose enclosing scope is later read by both the original and the spawned goroutine.
A bare, unsynchronized cache field would be a genuine data race in that case. The cache field
needs to follow the same discipline the table already uses for its other mutable state — guarded
by the existing `shared`/`RWMutex` pair (checking `shared.Load()` before deciding whether to
lock, exactly as `Get`/`Set`/`IsConstant` already do), not a plain, ad hoc field.

**Expected impact.** Partial, not asymptotic: this removes *repeat* O(depth) walks within a
single call frame's lifetime, but each *new* frame (i.e., each new recursive call) still starts
with an empty cache and pays one full O(depth) walk on its first non-local lookup — so total
cost across a complete recursive descent remains O(depth²), just with a smaller constant factor.
Profiling (Section 14) suggests that factor is roughly 2-3× for the Mandelbrot workload specifically
(one `Get` plus two `IsConstant` calls per frame, collapsing to effectively one real walk instead
of three) — a real, worthwhile reduction, but not a change in asymptotic shape.

#### Phase 4 — inherit the cached scope across call frames (not recommended without further design)

**What was originally proposed:** since a newly-pushed boundary frame's "next visible scope" is
*often* identical to its caller's own answer, inherit the caller's cached pointer directly at
`callFramePush` time instead of recomputing it, turning the whole chain's resolution into O(1)
per call regardless of depth — the only change in this report that would address Finding 14's
cost *asymptotically* rather than by a constant factor.

**Why this turned out to be substantially riskier than first scoped.** `FindNextScope`'s own
logic contains an early-return specifically for crossing into a different package's symbol
table (`if p.forPackage != "" { return p }`), and that branch exists for a concrete, common
reason: `internal/language/bytecode/package.go`'s `inPackageByteCode` (the `InPackage` opcode,
emitted by the compiler — `internal/language/compiler/function.go:168` — for every function
compiled with a non-empty `activePackageName`) pushes a **proxy** scope
(`symbols.NewChildProxy`, `internal/language/symbols/copy.go`) onto the chain on every call to
a function that belongs to an imported package. A proxy table's own parent is whatever the
caller's `c.symbols` was at that moment — not the real package table it proxies — so "the next
visible scope past this boundary" is a function of *which packages were crossed to get here*,
not just "how deep is the call stack." Two calls at the exact same depth, one entirely within
`package main` and one that passed through an imported package along the way, can have
genuinely different correct answers. Separately, closures with a `capturedScope`
(`callBytecodeFunction`) attach to whatever scope they were *defined* in, which can be an
arbitrary, unrelated table with no depth relationship to the immediate caller at all — so
"inherit from the caller" isn't even well-defined for that case, only "inherit from wherever the
closure was captured," a different and more complex question.

**Consequence:** a naive "always inherit the caller's cached value" implementation would
silently return the *wrong* scope the first time a program imports and calls into any package —
which describes nearly every non-trivial Ego program, including every test in `tests/` that
imports `fmt`. That is a correctness bug, not a missed optimization, and the profiled
Mandelbrot workloads (single-file, `package main`, no imports in the hot recursive path) could
not have surfaced it — they are exactly the case where the naive version would happen to be
correct.

**Recommendation:** do not implement Phase 4 as originally scoped. If it is pursued at all in
the future, it needs a provably-safe precondition before inheriting anything — at minimum,
confirming the immediate parent is itself a boundary table with a valid, still-fresh cache,
is not a proxy (`IsProxy()`), and that no `InPackage` instruction has executed since the parent
was pushed — and even then the win would be confined to same-package, non-closure call chains,
which is not representative of Ego programs in general. Given the complexity-to-benefit ratio
relative to Phases 1-3, this is left as future architectural work, in the same category as
Finding 7, rather than part of this implementation plan.

#### Suggested rollout

Phases 1-3 are recommended as a single implementation pass — each is independently mechanical,
each has a concrete, checked correctness argument rather than an assumption, and each is
independently revertible if re-profiling doesn't confirm the predicted effect. Phase 4 is
explicitly not recommended without a dedicated follow-up design effort, gated on how much of
Finding 14's cost remains after Phases 1-3 land — if the constant-factor reduction from Phase 3
turns out to be enough in practice, the asymptotic fix's much higher risk may not be worth
taking on at all.

### Resolution (July 2026) — Finding 14, Phases 1-3

Implemented Phases 1-3 together, exactly as planned above, plus the two lower-frequency
call-site updates Phase 2 flagged as "worth picking up for consistency" (`Context.SetGlobal`
and `syncPackageSymbols`).

**Files modified:**

- `internal/language/bytecode/callframe.go` (Phase 1) — `callFramePop`'s clone-check loop now
  breaks as soon as it reaches `callFrame.symbols`, with a comment explaining both why the loop
  can legitimately discard more than one scope (no `PopScope` is emitted for enclosing blocks at
  `return`) and why the bound is still safe for closures (the loop simply falls through to `nil`
  when the two chains diverge, exactly as before).
- `internal/language/bytecode/context.go` (Phase 2) — added a `rootSymbols *symbols.SymbolTable`
  field to `Context`, populated once in `NewContext` from the `s.Root()` call that already
  existed there for the extensions lookup. `SetGlobal` also updated to use it.
- `internal/language/bytecode/callframe.go`, `store.go`, `symbols.go` (Phase 2) — the four
  hot/warm `.Root()` call sites (`callFramePop`'s extensions write-back, `storeGlobalByteCode`'s
  two writes, `syncPackageSymbols`'s package lookup) now read `c.rootSymbols` directly instead
  of re-deriving it.
- `internal/language/symbols/tables.go` (Phase 3) — added `nextScope`/`nextScopeCached` fields
  to `SymbolTable`; `FindNextScope` now checks and populates this cache around its existing walk
  (control flow and logging behavior otherwise unchanged — the cache is populated at the same
  two return points the function already had, via a small `setCachedNextScope` helper, so the
  existing `symbols.boundary.skip` log line still fires only where it always did); `Boundary`
  and `SetParent` now clear the cache whenever called, since either can change what the walk
  would find, by calling a shared `invalidateNextScopeCache` helper.

**Bugs found while doing this work — one shipped, one caught before shipping.**

*A real deadlock shipped in the first version of this fix and hung the REST server.* The first
version of `cachedNextScope`/`setCachedNextScope`/`invalidateNextScopeCache` followed the
table's existing `shared`/`RWMutex` discipline literally — acquiring `RLock`/`Lock` around the
cache read/write, matching how `Get`/`Set` guard the symbol map itself. That is wrong for this
specific cache, and the mistake was not caught by any of this work's own testing (`go test
-race`, `ego test tests/`, a hand-written 20-goroutine stress test) because none of it exercised
a *shared* boundary table with an *unpopulated* cache — the scenario that actually triggers the
bug. It was found only after the user reported `curl -kL https://localhost/services/factor/10`
hanging indefinitely against a running server with `--log symbols` enabled, and provided the
resulting log. The log showed execution stall immediately after a `FindNextScope`
boundary-skip resolution inside a request handler, with no further progress — a classic deadlock
signature, not a slow computation. Root cause: `symbols.(*SymbolTable).Get` and `IsConstant`
both acquire `s.RLock()` via a deferred `RUnlock()` held for the rest of their call, then invoke
`s.FindNextScope()` *while still holding that read lock*. The buggy cache-population path
called `s.Lock()` — a write lock on the same table — to store the computed answer. Go's
`sync.RWMutex` is not reentrant, so a write-lock attempt while the same goroutine already holds
a read lock on the same mutex blocks forever. This can only happen when `s.shared` is true,
which a plain `ego run`/`ego test` process essentially never sets on a boundary table (per
`internal/language/symbols/tables.go`'s `SerializeTableAccess` default of `false`), but the REST
server explicitly does — `internal/commands/run.go:702` marks the server's root table shared so
package tables (like `time`, visible in the reported hang's log) can be reused safely across
concurrent requests. That is exactly why this bug was invisible to every test run during
development and only surfaced against a running server.

**Fix:** rather than making the cache lock-safe, the cache is now skipped entirely for shared
tables — not merely lock-protected, but never read or written at all when `s.shared.Load()` is
true. This is correct because it costs nothing on the case Phase 3 actually targets: shared
tables are long-lived, cross-request package tables, not the per-call function scopes that
accumulate O(depth) `FindNextScope` cost under recursion. With the cache reads/writes now
unconditional early-returns for shared tables (no locking at all, in either direction), there is
no lock to acquire and therefore nothing to deadlock on; for non-shared tables — which by this
codebase's own model are never accessed by more than one goroutine — no locking was ever needed
either. `SetParent`'s cache invalidation, which already held the table's write lock for its own
`.parent`/`.isRoot` writes (pre-existing, unrelated to this fix), now simply calls the same
`invalidateNextScopeCache` helper directly, since that helper no longer locks and so can no
longer conflict with a lock `SetParent` already holds.

*Caught before shipping, separately:* an early draft of `SetParent`'s cache invalidation called
`invalidateNextScopeCache()` (before it was simplified to skip shared tables) from inside the
block already holding `SetParent`'s own `s.Lock()` — a second, independent way to hit the same
class of self-deadlock. This one was caught by re-reading the diff before building, not by a
failing test, and fixed at the time by inlining the field clears under the already-held lock.
That workaround is no longer needed now that the cache helpers never lock at all.

**Verification of the fix:** `go build ./...`, `go vet ./...`, full `go test ./...`, and `go
test -race ./internal/...` all clean. Reproduced the exact reported failure against a real
server: `./ego stop server` (the hung instance from the original report), rebuilt, `./ego --log
symbols start server`, then the identical `curl -kL https://localhost/services/factor/10 -v` —
completed in 8.5ms with the correct result (`[1, 2, 5, 10]`), confirmed against the server's own
structured log (`[1] 200 GET /services/factor/10 ... elapsed 8.524417ms`). Followed with 20
concurrent requests to the same service (all `200`, no hang) and the full `tools/apitest.sh`
suite against the running server (122/122 passed).

**Correctness verification:**

- `go build ./...`, `go vet ./...` clean.
- `go test ./...` clean, including `go test -race ./internal/...` — run specifically because
  Phase 3 adds new mutable state to a type that is deliberately shared across goroutines in some
  programs (see the Phase 3 write-up's concurrency obstacle).
- All 1,193 tests in `ego test tests/` pass, including `tests/flow/goroutine.ego`,
  `tests/flow/defer.ego`, `tests/flow/defer_lifo.ego`, `tests/flow/defer_scope.ego`,
  `tests/flow/panic_recover.ego`, `tests/errors/panic_recover.ego`, and `tests/packages/`
  (which exercises the `InPackage`/proxy-table code path Phase 4 was declined over — confirming
  Phases 1-3 don't disturb it, since none of them touch `forPackage` or proxy-table handling).
- A hand-written 20-goroutine stress test (each incrementing a mutex-protected package-level
  counter 1,000 times through a shared closure) was run five times and produced the correct
  `counter = 20000` result on every run, with no `-race` complaints when the equivalent Go-level
  concurrency was exercised via the package test suite.

**Re-profiling results — recursive workload (`examples/mandelbrot2.ego`, Section 14):**

| Metric | After Findings 12+13 | After Phases 1-3 | Change |
| - | - | - | - |
| Wall clock (`time ego run ...`, avg of 3 runs) | 17.27s | 5.71s | **-66.9%** |
| Total CPU samples captured | 16.25s / 17.45s (93.15%) | 5.52s / 5.85s (94.38%) | -66.0% |
| `callFramePop` (cum) | 4.64s / 28.55% | *(0 samples — not present at all)* | **-100%** |
| `updatePackageFromLocalSymbols` (cum) | 2.23s / 11.63% | *(0 samples)* | **-100%** |
| `Root`/`IsRoot` (cum) | 2.55s / 13.30% | *(0 samples)* | **-100%** |
| `IsConstant` (cum) | 4.16s / 25.60% | 0.02s / 0.36% | **-99.5%** |
| `IsClone` (flat) | 1.32s / 6.89% | *(0 samples)* | **-100%** |
| `FindNextScope` (cum) | 6.23s / 38.34% | 2.41s / 43.66% | -61%, now 100% attributable to `Get`'s `MaxIter` lookup |

```text
(mandelbrot2, after Phases 1-3 — FindNextScope's remaining cost is entirely from Get)
                                             2.41s   100% |   github.com/tucats/ego/internal/language/symbols.(*SymbolTable).Get
     2.40s 43.48% 43.48%      2.41s 43.66%                | github.com/tucats/ego/internal/language/symbols.(*SymbolTable).FindNextScope
                                             0.01s  0.41% |   runtime.asyncPreempt
```

**Why the improvement is much larger than Phase 3 alone would predict.** Phase 3's own
write-up characterized itself as "partial, not asymptotic" — and that held exactly true:
`FindNextScope`'s cost from `IsConstant` (two calls per frame, `nextZr` then `nextZi`) dropped
to noise, because the second call now hits the Phase 3 cache, but its cost from `Get` (the
`iter >= MaxIter` / `return MaxIter` lookups, which execute once per non-escaped call — `MaxIter`
is referenced at every level of the recursion, not just the terminal one) is essentially
unchanged in shape, since there is no repeat call within a single frame for Phase 3 to
memoize. The much larger overall win came from Phases 1 and 2, which turned out to be
*complete* eliminations, not reductions, for this specific workload: `mandelIterate`'s own
function body has almost no internal scope nesting (its two `if` bodies contain no
declarations, so Finding 8 already elides their scopes), which means `callFrame.symbols` sits
only one or two steps above `c.symbols` at return time — so Phase 1's bounded loop now
terminates almost immediately instead of walking the remaining call stack, and Phase 2 removed
the `Root()`/`IsRoot()` walk entirely by never walking at all. Two of the three O(depth) costs
Finding 14 identified turned out to be fully O(1)-able for this workload once bounded/cached
correctly; only the third (`Get`'s single-per-frame `MaxIter` lookup) remains genuinely
O(depth) per call, and is now the dominant cost by a wide margin, exactly where Phase 4's
declined, higher-risk fix would have applied if pursued.

**Re-profiling results — iterative workload (`examples/mandelbrot.ego`, Section 10):**

| Metric | After Finding 9 | After Phases 1-3 | Change |
| - | - | - | - |
| Wall clock (`time ego run ...`, avg of 2 runs) | 21.17s | 20.89s | -1.3% |
| `Root`/`IsRoot`/`callFramePop`/`FindNextScope` | present (small, per Finding 9's Resolution) | *(none appear in top 300 — below noise threshold)* | further reduced |

The iterative workload's own hot loop (`Mandelbrot()`, called 40,000 times, each a single,
shallow, non-recursive call) never built the deep call-stack chains Finding 14 targets, so it
was never going to show a large effect here — this small, expected improvement is entirely
Phase 2 finishing off the `Root()`/`IsRoot()` write-side cost that Finding 13's Resolution
explicitly measured and left in place (see that section's "folded into Finding 14's scope"
note).

**Honest assessment.** This is the largest single wall-clock improvement measured anywhere in
this report on a workload-specific baseline — larger than Finding 4's -48/-49% and Finding 12's
own -40.6%, though, as with those, only fairly compared against the specific workload and prior
baseline it was measured on, not as a universal multiplier. The size of the win is specific to
`mandelIterate`'s shape (minimal internal scope nesting, one non-local constant reference per
call) rather than a general property of recursion — a recursive function with deeper internal
block nesting would see less benefit from Phase 1, and one referencing more distinct non-local
names per call would see more benefit from Phase 3. The `Get`/`MaxIter` cost that Phase 3
couldn't reach is now, by a wide margin, the largest remaining cost in the recursive workload —
and is exactly Phase 4's territory, reinforcing the original plan's judgment that Phase 4 should
be evaluated *after* seeing how much of Finding 14's cost survives Phases 1-3, not before. With
Phases 1-3 landed, that remaining cost is real but substantially smaller in absolute terms
(2.41s of a 5.52s pool) than the combined ~15s it was part of before this round of fixes — worth
a future look, but no longer the dominant story it was.

---

## 18. Summary table

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
| 9 | `data.String()` has no `string` fast path, always calls `fmt.Sprintf` | **15.06% of total** in the Mandelbrot workload (97% of that inside one `fmt.Sprintf` line) | Low | Low | Implementation bug (wrong tool for the job, same shape as Finding 1) | **Fixed** (July 2026) — see Resolution above |
| 10 | `atLineByteCode` writes `__line`/`__module` into the symbol table every statement | ~3-4% of total in the Mandelbrot workload; consumed almost solely by rare error-construction path | Low-Medium | Low-Medium | Implementation inefficiency (eager work for a rarely-read value) | Open |
| 11 | Loop bodies with top-level `:=`/`var` still pay per-iteration scope cost; `IsConstant` full-chain walk on new declarations | ~7% combined (scope alloc) + ~3.8% (`IsConstant` walk) in the Mandelbrot workload | Medium-High | Medium-High (touches `:=` creation semantics; needs correctness care) | Extends Finding 4 to a case it deliberately excluded; corroborates Finding 7 | Open — needs design work before implementation |
| 12 | `SetParent`'s cycle-detection walk is dead code on the hot `NewChildSymbolTable` path | **31.83% of total** in the recursive Mandelbrot workload — largest single flat cost in this report | Low | Low | Implementation bug (defensive check, provably unreachable at this call site) | **Fixed** (July 2026) — see Resolution above |
| 13 | `__extensions` flag round-trips through an O(depth) symbol-table walk on every call/return | **~15.4% of total** in the recursive Mandelbrot workload (2.46% read + 2.47% write, both O(depth)) | Low-Medium | Low-Medium (one rare runtime-toggle path to preserve) | Implementation inefficiency (duplicates a value already tracked on `Context`) | **Partially fixed** (July 2026) — read side only; write side kept for correctness — see Resolution |
| 14 | `FindNextScope` and `callFramePop`'s clone-check both walk the full ancestor chain per operation | **-66.9% wall clock** on the recursive Mandelbrot workload (17.27s → 5.71s) from Phases 1-3 | High (Phases 1-3 done; Phase 4 still High) | Medium-High for Phase 4 (needs design work) | Architectural — same category as Finding 7, specific to deep recursion | **Phases 1-3 fixed** (July 2026) — see Resolution above; Phase 4 declined, not implemented |

**Suggested order of work:** ~~1~~ → ~~2~~ → ~~3~~ → ~~4~~ → ~~8~~ → ~~9~~ → ~~12~~ → ~~13
(partial)~~ → ~~14 (Phases 1-3)~~ → **10** → **11** → **14 (Phase 4, only if warranted)**, in
that order, re-profiling after each change using the same `--pprof` workloads described in
Section 1 (and, from this point on, also the Mandelbrot workloads in Sections 10 and 14) to
confirm the expected compounding effect on Finding 5 before deciding whether Finding 7 is ever
worth pursuing. Findings 1-4, 8, 9, and 12 are fully done; Finding 13 is done on its read side
only, by design; Finding 14 is done for Phases 1-3, with Phase 4 explicitly declined (see its
Resolution section). Re-profiling after each (see their Resolution sections) confirms each
predicted direct cost was eliminated (`uuid.New` gone entirely; `IsType` no longer appears in
the profile on this path at all; `ui.Log` argument construction is now uniformly guarded across
248 inspected call sites; `pushScopeByteCode`/`NewChildSymbolTable` no longer appear at all in a
loop with no closures or top-level declarations, nor in an if/else nested inside one;
`fmt.Sprintf` no longer appears anywhere in the Mandelbrot profile; `SetParent` no longer
appears anywhere in the recursive Mandelbrot profile; `callByteCode`'s cumulative cost fell 90%
once its extensions check stopped walking the symbol table; `callFramePop`,
`updatePackageFromLocalSymbols`, `Root`/`IsRoot`, and `IsClone` no longer appear at all in the
recursive Mandelbrot profile). Finding 4 delivered the largest single-fix wall-clock improvement
on the original synthetic workloads (-48% to -49% on top of Findings 1+2+3, depending on
workload) — a cumulative **63-72% reduction from the original baseline** (95.6s → 27.25s on the
50,000,000-iteration loop). Finding 8 adds a further **-30%** on top of that for code shaped
like conditionals inside loops, a pattern Finding 4 could not reach on its own. Finding 9 is the
largest single-fix wall-clock improvement measured on the iterative Mandelbrot workload
specifically: **-24.9%** (28.17s → 21.17s), for a three-line, fully mechanical change. On the
recursive Mandelbrot workload, Finding 12 delivered **-40.6%** (34.1s → 20.26s), Finding 13's
read-side fix added **-14.8%** on top of that (20.26s → 17.27s), and Finding 14's Phases 1-3
added a further **-66.9%** on top of *that* (17.27s → 5.71s) — a cumulative **83.3% reduction**
from the three fixes together (34.1s → 5.71s), the largest cumulative improvement measured on
any single workload in this report. `runtime.kevent` and `runtime.madvise` (thread scheduling
and memory management) remain present in every profile but are no longer anywhere near the
dominant cost they were before Findings 4, 8, 9, 12, 13, and 14 on the workloads each one
targeted.

**Profiling a deeply recursive program (Section 14) surfaced the largest, most severe cost
shape in this entire report, and Findings 12-14 have now captured nearly all of it.** Findings
12, 13, and 14 together originally accounted for over **70% of total flat CPU time** in that
workload. Finding 12 is fully fixed. Finding 13 is fixed on its read side, which was the piece
that could be removed outright; its write side turned out, on investigation, to be load-bearing
— correctly unwinding a callee's mid-function `@extensions` toggle for code that can only
observe that flag through the symbol table, not through `Context` — so it was deliberately left
in place, and Finding 14's Phase 2 then eliminated it anyway by caching the root table pointer
directly on `Context`, sidestepping the walk entirely rather than removing the write. Finding
14's Phases 1-3 turned out to eliminate two of its three targeted costs completely
(`callFramePop`'s clone-check loop and the `Root()`/`IsRoot()` walk both dropped to zero
measured samples on the recursive workload) and substantially reduced the third
(`FindNextScope`, down 61% in absolute cost) — leaving only the single-per-frame `Get("MaxIter")`
lookup as a genuinely unaddressed O(depth) cost, which is exactly Phase 4's declined territory.
Phase 4 remains not recommended without further design work, per its own write-up, and this
result doesn't change that calculus — it just narrows what Phase 4 would need to fix, and by how
much, if it's ever revisited. Findings 10 and 11 remain open, smaller, independent items from
the iterative Mandelbrot workload, not superseded by any of this.
