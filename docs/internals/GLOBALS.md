# Package-Level Const/Var/Type Access — Implementation Plan

**Status:** Phase 0 (groundwork) and Phase 1 (const folding) are landed and verified. Phases 2
(runtime global-reference cache for `var`/non-foldable-const) and 3 (type-assertion fallback +
`PERFORMANCE.md` Resolution writeup) are not yet started. This document is the design and risk
record for fixing `docs/internals/PERFORMANCE.md` Finding 17, written and reviewed before any code
changes, following the same process `docs/internals/SLOTS.md` used for Finding 7.

**Phase 1 results.** Re-profiling Finding 17's own repros with `ego run --profiling` after the
const-folding fix landed: `examples/mandelbrot2.ego`'s `mandelIterate:40` (the `MaxIter` const
comparison that originally motivated this document) dropped from 5.69s to 119ms (~48x) across
675,279 hits — now *cheaper* than its locals-only sibling line (`mandelIterate:35`, 248ms). The
isolated `deep_check.ego` repro (800-deep recursion) dropped from 8.86s to 267ms (~33x), converging
to parity with the non-recursive `shallow_check.ego` baseline (282ms) for the same total number of
comparisons. Const folding alone fully resolves this specific finding's own repro case, since
`MaxIter` is a simple untyped int const with no `var` component; Phase 2 remains valuable for the
more general `var`/non-foldable-const case the original request also asked about.

**Origin:** `docs/internals/PERFORMANCE.md`,
[Finding 17](PERFORMANCE.md#21-finding-17--a-package-level-constglobal-referenced-from-deep-recursion-costs-odepth-per-reference-not-o1)
("a package-level const/global referenced from deep recursion costs O(depth) per reference, not
O(1)"), the documented remainder of [Finding 7](PERFORMANCE.md#8-finding-7-architectural-high-effort-high-ceiling--name-based-symbol-resolution)
(name-based symbol resolution) after `SLOTS.md`'s register system resolved the *local*-variable
half of that problem.

---

## Contents

- [1. Motivation](#1-motivation)
- [2. Goals and non-goals](#2-goals-and-non-goals)
- [3. Relevant existing architecture](#3-relevant-existing-architecture)
- [4. Design overview — two independent mechanisms](#4-design-overview--two-independent-mechanisms)
- [5. Tier 1: const folding](#5-tier-1-const-folding)
- [6. Tier 2: runtime global-reference cache](#6-tier-2-runtime-global-reference-cache)
- [7. Phasing plan](#7-phasing-plan)
- [8. File-by-file touch list](#8-file-by-file-touch-list)
- [9. Testing and verification strategy](#9-testing-and-verification-strategy)
- [10. Open questions / decisions needed before implementation](#10-open-questions--decisions-needed-before-implementation)

---

## 1. Motivation

Finding 17 measured a ~22x per-hit cost for referencing a package-level `const` from 800-level
recursion versus the same reference at shallow depth, using `examples/mandelbrot2.ego`'s recursive
`mandelIterate` function and its `MaxIter` constant. Root cause: `symbols.Get()`
(`internal/language/symbols/get.go:60`) does a map probe at the current scope and, on a miss,
recurses into `FindNextScope()` (`internal/language/symbols/tables.go:296`), which — for a
function-call boundary table — walks up through every intervening call-frame boundary before
reaching the package-level table where the name actually lives. This repeats once per call frame
between the reference and that table, so a full recursive descent pays O(depth²) total, not
O(depth).

`SLOTS.md` (Finding 7's resolution) already solved the *local*-variable half of name-based
resolution: a function's own parameters and `:=` locals compile to integer register slots
(`LoadRegister`/`StoreRegister`), completely bypassing `Get()`. `SLOTS.md` explicitly scoped
package-level `const`/`var` out as a non-goal. Finding 17 is exactly that remainder — and Finding
14's own Resolution section (three landed phases fixing the *other* O(depth) costs riding
alongside this one) states plainly that, after those fixes, 100% of the *remaining*
`FindNextScope` cost in the recursive Mandelbrot workload is attributable to this one const
lookup.

Finding 14 Phase 4 — the one change that would have made `FindNextScope` O(1) per call in
general, by having a new call frame inherit its caller's cached "next scope" answer — was
deliberately **rejected** as unsafe: it breaks as soon as a call crosses an `InPackage` package-proxy
boundary or involves a closure with a captured scope, both of which make "the next visible scope"
depend on *which path* was taken, not just call depth. Any fix here needs its own, narrower safety
argument, not a resurrection of that declined idea.

---

## 2. Goals and non-goals

**Goals:**

- Eliminate the runtime lookup entirely for package-level `const` references that are foldable
  within a single compilation unit (Finding 17's own repro, `MaxIter`) — reduce to zero lookup, not
  just O(1).

- Provide a general, safe caching mechanism for everything that can't be folded: `var`,
  cross-file/cross-package `const`, and the one type-assertion (`x.(T)`) fallback that still does a
  name-based lookup — reduce those to O(1) after a one-time miss per call site.

- Reuse the compiler's existing centralized emission choke points (`emitLoadName`/`emitStoreName`/
  `emitAddressOfName`/`emitDeRefName`, `internal/language/compiler/slots.go`) rather than building a
  parallel path.

- Preserve every existing safety property `SLOTS.md` and Finding 14 established: conservative,
  err-toward-not-eligible logic; no change to readonly/type-coercion semantics; no new opcodes
  unless truly necessary (there are none in this design).

**Non-goals (this pass):**

- Cross-file or cross-package constant *folding*. A const declared in another file of the same
  package, or in an imported package, is not visible to the referencing `Compiler` instance's
  `c.constants`/`c.constantValues` map — those references go through Tier 2 (the runtime cache),
  not Tier 1 (compile-time folding). This is a scope boundary, not a bug.

- Resurrecting Finding 14 Phase 4's declined approach (caching/inheriting the *next-scope pointer*
  itself across call frames). This design caches something different and narrower — see Section
  6.3's safety argument for exactly why that distinction matters.

- A dedicated design pass for `ego.runtime.deep.scope`/`SerializeTableAccess` interactions beyond
  what's explicitly tested in Section 9 — flagged as a guard, not designed away.

---

## 3. Relevant existing architecture

### 3.1 Storage is provably stable

Every package (main's own top-level code, and each imported package) gets exactly one persistent
`*symbols.SymbolTable` that lives for the entire process lifetime, created once
(`internal/commands/run.go`'s `initializeSymbols` for main; `internal/language/compiler/import.go:395`
for imports, cached thereafter via `internal/packages`). Storage is bin-indexed
(`values []*[]any`, `internal/language/symbols/values.go`) specifically so a slot's address stays
valid for the table's whole lifetime — a name's `(table, slot)` pair, once established, never
changes for the life of the process.

### 3.2 The O(depth) mechanism

`symbols.Get()` (`get.go:60-119`): map probe at the current table; on a miss,
`s.FindNextScope()` then recurse into that table's own `Get()`. Every function-call activation
pushes a fresh `boundary=true` table whose parent is the caller's own table
(`pushScopeByteCode`, `internal/language/bytecode/symbols.go`), so the ancestor chain of symbol
tables is isomorphic to the *dynamic call stack*, not lexical nesting. `FindNextScope()`
(`tables.go:296-341`) walks up through boundary tables until it reaches the package/global scope.
Finding 14 Phase 3 already caches this walk's answer per table instance
(`cachedNextScope`/`setCachedNextScope`, `tables.go:343-376`) — but every *new* recursive call
starts with a brand-new boundary table whose cache is empty, so it still pays one full O(depth)
walk on its first non-local lookup per call. Finding 14's Resolution section states the total cost
across a complete recursive descent remains O(depth²), just with a smaller constant factor.

### 3.3 Why `cachedNextScope` skips shared tables — and why that doesn't block this design

Verified directly against `tables.go`: `Get()`/`IsConstant()` hold a read lock on a table before
calling `FindNextScope()`; if `FindNextScope()` tried to populate that same table's own cache
fields, it would need a write lock while the read lock is still held — a self-deadlock, since
`sync.RWMutex` is not reentrant. So `cachedNextScope`/`setCachedNextScope` are unconditionally
no-ops whenever `s.shared.Load()` is true. This is a *different* mechanism from the one this
document adds (Section 6): the new cache lives on the `*bytecode.ByteCode` object, not on any
`SymbolTable`, so it never needs to lock the table it caches a reference to — this specific
deadlock hazard does not apply here.

### 3.4 `IsShared()` is not a safe "is this a singleton" signal

Confirmed by reading `NewSymbolTable`/`NewChildSymbolTable`: both call
`symbols.shared.Store(SerializeTableAccess)` at creation. `SerializeTableAccess`
(`EGO_SERIALIZE_SYMBOLTABLES`, read by both `ego run`, `internal/commands/run.go:109`, and
`ego server`, `internal/commands/server.go:78`) is a debug/testing hook, not a production feature —
per the user, it was a workaround for symbol-table serialization bugs that are believed mostly or
entirely fixed now, not something end users are expected to reach for. Still, when it is set,
**every** newly-created table, including ordinary transient per-call boundary tables, starts out
`shared == true` — so `IsShared()` cannot be used as this design's eligibility signal (Section 6.2)
regardless of how the flag is currently used in practice: relying on it would make the new cache's
correctness depend on this debug hook staying unused forever, which isn't a bet worth making when a
new, purpose-built field costs almost nothing extra and removes the ambiguity outright.

### 3.5 Package proxy tables — the existing asymmetry

`symbols.NewChildProxy` (`internal/language/symbols/copy.go:20-38`) creates a lightweight table
that shares the real package table's `symbols`/`values` maps but has its own `parent` pointer.
Inserted via the `InPackage` bytecode instruction on every call into a function belonging to a
named, non-`main` package (`internal/language/compiler/function.go`, when
`c.activePackageName != ""`). This already gives imported-package code an O(1) shortcut to its own
package's names (the proxy sits directly above the function's own locals table). **`package main`'s
own top-level code — Finding 17's actual repro — gets no such proxy**, since
`c.activePackageName == ""` throughout main. This design (Tier 2, Section 6) naturally covers both
cases without needing to special-case either, since its eligibility test is about the *destination*
table's identity, not the *path* taken to reach it.

### 3.6 No existing constant-folding for named consts (a real, confirmed gap)

`compileConst` (`internal/language/compiler/constant.go`) tracks constant *names only*, in
`c.constants []string`, used solely to validate that a later constant's RHS doesn't reference a
non-constant name — the actual literal value is discarded after validation, pushed onto the
runtime stack and consumed by the `Constant` opcode at *execution* time. The peephole optimizer
(`internal/language/bytecode/optimizer.go`) already folds adjacent literal/arithmetic sequences
(`tryConstantArithmetic`) but has no rule recognizing "a `Load` of a name already proven to be a
compile-time constant" and rewriting it to `Push <value>`. `type` declarations, by contrast,
already get this treatment via `c.types map[string]*data.Type` — this design brings `const` up to
the same standard `type` already has.

### 3.7 `compileConst`'s existing validation already proves RHS purity

`constant.go:187-191` rejects any RHS whose compiled bytecode contains a `Load` of a name not
already in `c.constants`. Since a `Load` of a *function* name would itself fail this exact check
(a function name is never itself in `c.constants`), the only opcodes that can appear in a
successfully-validated const's bytecode are: literal `Push`, arithmetic/logical ops on those, and
`Load` of previously-validated constants. **Every const that compiles successfully today is
therefore already side-effect-free and fully evaluable at compile time** — there is no
"sometimes foldable" case within one compilation unit. The only unfoldable case is
cross-file/cross-package (Section 2, non-goals), which the language's declare-before-use rule
already structurally excludes from appearing in a same-unit expression.

### 3.8 The register system's centralized choke points (recap)

`internal/language/compiler/slots.go:243-292` — `emitLoadName`, `emitStoreName`,
`emitAddressOfName`, `emitDeRefName` — are the *only* places in the compiler that decide between a
register-based opcode and the name-based fallback, for every identifier read/write/address-of/deref
site. `resolveRegister` (`slots.go:156-176`) walks the current function's own lexical scope stack
(bounded by `c.funcRegisters.scopeStart`) and its parameter map; it has zero knowledge of
consts/package-level vars by design. This is exactly where Tier 1's new check belongs.

### 3.9 `executeFragment` — a proven compile-time-execution primitive

`(*bytecode.ByteCode).executeFragment` (`internal/language/bytecode/optimizer.go:721-740`) builds a
throwaway `ByteCode`, a fresh `symbols.NewSymbolTable`, a `bytecode.NewContext`, runs it, and pops
the result — used today by the peephole optimizer's own constant-folding rule as its fallback for
non-numeric types. This is the direct precedent for Tier 1's const-value evaluation (Section 5.2).

### 3.10 The statement profiler's storage pattern — the model for Tier 2

`internal/language/bytecode/profile.go` (documented in `docs/internals/CLAUDE.md`'s "Built-in
Statement Profiler" section) attaches per-instruction data directly to `*ByteCode`, indexed by
instruction offset, via a bare `unsafe.Pointer` field accessed only through `sync/atomic`'s free
functions — because `go vet`'s copylocks check flags both a directly-embedded `sync.Mutex` and a
generic `atomic.Pointer[T]` as unsafe to duplicate in a struct (`ByteCode`) that's copied by value
elsewhere (`Clone()`, `NeedsCoerce`'s value receiver, `restoreByteCode`'s struct assignment). Tier 2
(Section 6) reuses this exact pattern for its own per-instruction cache.

---

## 4. Design overview — two independent mechanisms

| | Tier 1 — const folding | Tier 2 — global reference cache |
| --- | --- | --- |
| When | Compile time | Runtime, self-discovering, first-hit populates |
| Scope | Same-compilation-unit, provably-pure `const` | `var`, non-foldable `const`, the type-assertion fallback |
| Mechanism | `c.constants` gains values; extend `emitLoadName` | New per-instruction cache on `*bytecode.ByteCode`, keyed by PC |
| Cost after fix | Zero (literal `Push`) | O(1) after one-time miss |
| New opcode? | No — reuses `Push` | No — reuses existing `Load`/`Store`/`AddressOf`/`DeRef`/`UnWrap` handlers |
| New compiler eligibility predicate? | Yes (Section 5.3) | No — see 6.1, this is deliberate |

They are kept separate because they operate at different times and have different safety
arguments — not merged into one mechanism.

---

## 5. Tier 1: const folding

### 5.1 Data structure change

Add `c.constantValues map[string]any` alongside the existing `c.constants []string`
(`internal/language/compiler/compiler.go`) — keep both rather than converting `c.constants` to a
value-carrying map outright, since its ordered-list-membership use in `constant.go`'s `util.InList`
check is unrelated to value storage and minimizing that diff reduces risk. Both must be propagated
identically to wherever `c.constants` already is: `Clone()`, and the parent/child compiler
relationship used when compiling an imported package's source.

### 5.2 Evaluating the RHS to a concrete value

In `compileConst`, immediately after the existing purity-validation loop succeeds and before/
alongside emitting the `Constant` opcode:

1. Build a throwaway fragment from the compiled RHS's own bytecode plus, if a type conversion
   suffix was emitted (`Push type; Swap; Call 1`, for a typed/`iota`-based const), that suffix too
   — so typed enum consts fold, not just untyped literals.

2. Seed a scratch `symbols.SymbolTable` with every entry of `c.constantValues` collected so far
   (readonly), so a `Load` of an earlier same-block constant resolves correctly inside the fragment.

3. Run it via the same pattern `executeFragment` already uses (Section 3.9), implemented as a
   small, self-contained helper in the `compiler` package (not a call into `optimizer.go`'s
   unexported method — see Open Question 2) — e.g.
   `(c *Compiler) foldConstantValue(vx *bytecode.ByteCode, t *data.Type) (any, bool)`.

4. On success, store the popped value in `c.constantValues[nameSpelling]`. On any failure
   (should not happen given 3.7's purity argument, but handled defensively), simply don't populate
   the map — the const still compiles exactly as it does today.

### 5.3 The shadowing hazard — the one correctness risk in this tier

A package-level `const MaxIter = 1000` can legally be shadowed by a function-local
`MaxIter := 5` — ordinary lexical scoping, with nothing restricting it the way
`ego.compiler.type.shadowing` restricts *type*-name shadowing specifically. Naively folding every
reference to a name present in `c.constantValues` would silently produce the **wrong value** inside
a shadowing scope — worse than a missed optimization, an actual correctness bug.
`resolveRegister` (checked first, unchanged) already handles this for *register-eligible*
shadowing locals, but not for a non-register-eligible local or a `var` sharing the const's name.

**Resolution:** maintain `c.nonConstLocalNames map[string]bool`, populated at the existing call
sites where a name-based (non-register) local/parameter declaration already happens — the points
where `allocateRegister`/`allocateParamRegister` return `(-1, false)` and the caller proceeds with
the pre-existing name-based path (`:=` handling, `var` declarations, parameter binding) — one line
recording the name at each such site. `emitLoadName`'s new fold check is gated on
`!c.nonConstLocalNames[name]`. This is deliberately coarse: one shadowing declaration anywhere in
the compilation unit disables folding for that name everywhere, not just in the lexically-shadowed
region — matching the same conservative, whole-unit, err-toward-not-eligible discipline the
register-eligibility predicate itself already uses. Shadowing a package const with a same-named
local is rare in idiomatic code, so the lost optimization is expected to be negligible (see Open
Question 3 for a more precise alternative, deliberately not chosen for the first cut).

`emitLoadName`'s new check order:

1. `resolveRegister(name)` → `LoadRegister` (unchanged).

2. Else, if `!c.nonConstLocalNames[name]` and `value, ok := c.constantValues[name]; ok` →
   `b.Emit(bytecode.Push, value)` (new).

3. Else, existing fallback: `b.Emit(bytecode.Load, name)` (unchanged).

`emitStoreName`/`emitAddressOfName`/`emitDeRefName` are **not** touched — consts are never
assignment targets or addressable in Go/Ego.

### 5.4 Interaction with the `Constant` opcode

The `Constant` opcode (`internal/language/bytecode/symbols.go`) is still emitted at the declaration
site exactly as today — it's what makes the name resolvable via ordinary name-based lookup for
everything Tier 1's fold doesn't reach (debugger introspection, `util.Symbols`, cross-file/
cross-package references). Tier 1 is strictly additive: it changes what *reference* sites compile
to, never what the declaration site does.

### 5.5 Gating

New setting mirroring `ego.compiler.registers`'s own convention (e.g. `ego.compiler.constfold`),
with a matching `@compile constfold=true|false` directive override, so a test or cautious rollout
can disable it independently of the general optimizer level and of Tier 2.

---

## 6. Tier 2: runtime global-reference cache

### 6.1 Why no compiler eligibility predicate is needed here

Every other optimization in this codebase (register eligibility, Findings 4/8/11's scope-elision
predicates) is a compile-time, conservative, whole-function scan because the compiler needs to
*decide* something ahead of time. Tier 2 doesn't need this: whether a given instruction resolves to
a cacheable global is a runtime fact, discovered the first time that instruction executes via the
existing, unchanged `Get()`/`Set()` walk. If the resolved table qualifies (6.2), cache it; if not,
nothing changes from today. This sidesteps entirely the question of how the compiler would know
whether a given name is a forward-declared or cross-package const — it doesn't need to; the runtime
finds out once and remembers.

### 6.2 The eligibility test: a new explicit field

Add `globalSingleton bool` + `SetGlobalSingleton()`/`IsGlobalSingleton()` to `symbols.SymbolTable`
(alongside `isRoot`), set at exactly two production call sites, synchronously before any concurrent
access is possible:

- `internal/commands/run.go`'s `initializeSymbols`, right after
  `symbols.NewSymbolTable(name).Shared(true)` (main's own persistent top-level table).

- `internal/language/compiler/import.go`, right after `importSymbols := symbols.NewChildSymbolTable(...)`
  (each imported package's persistent table).

Not `IsShared()` — see Section 3.4 for why that's unsound under `EGO_SERIALIZE_SYMBOLTABLES`. Not
`IsRoot()`/`Package() != ""` — the persistent import table has `isRoot == false` (its parent is
`c.rootTable`, not nil) and `forPackage == ""` (confirmed: the only production writer of
`forPackage` doesn't exist — see Open Question 1), so neither reliably identifies it.

### 6.3 The safety argument

**Claim:** once a `(ByteCode, instruction-offset)` pair has cached table `T`, it is safe to call
`T.Get(name)`/`T.Set(name, v)`/`T.GetAddress(name)` directly for every future execution of that
instruction — regardless of which clone of that `ByteCode` is executing, which closure captured it,
or which call path reached it — **provided the cache is only populated when `T.IsGlobalSingleton()`
is true at the moment of caching.**

- `T` is one of the program's small, fixed set of persistent tables, created exactly once and never
  recreated or reparented (Section 3.1). Once "this name is found in `T`'s own `symbols` map" is
  true, it stays true for the rest of the process's life — a package-level name is never deleted or
  moved to a different table once declared, which is exactly why calling `T.Get(name)` directly
  reproduces the identical answer `c.symbols.Get(name)` would eventually reach after its walk.

- Finding 14 Phase 4's rejected idea cached something genuinely path-dependent — the *next-scope
  pointer itself*, which legitimately differs across calls that cross different package-proxy
  boundaries or capture different closures. This design never caches a path-dependent value: it
  only caches an answer *after* confirming it landed on a table whose identity is invariant to
  every possible path. If a given instruction's resolution ever depends on calling context (a
  closure capturing an enclosing function's own local, a supported feature), that resolution lands
  in some other, non-singleton table; `IsGlobalSingleton()` is false, and the cache is simply never
  populated for that instruction — it keeps paying the full walk, exactly as today. A given `name`
  at a given lexical position either is always shadowed-by-closure-capture or never is — that's a
  static property of the source, not something that varies call-to-call.

- Closures cloned per-loop-iteration (the same "same `ByteCode` identity, multiple runtime
  instances" concern `profile.go` already handles for its own per-instruction storage, Section
  3.10): a clone's captured scope differs per iteration, but a package-level global reference
  inside its body still resolves, via `FindNextScope`, past whichever captured scope to the *same*
  one persistent table regardless of which clone is running — so sharing the cache array across
  clones (automatic, since `Clone()` value-copies the new `unsafe.Pointer` field exactly as it
  already does for `profile`) is correct *by construction* here, not merely tolerable the way it is
  for the profiler's cosmetic merge-at-report-time tolerance.

- Concurrent goroutines: reading `(T, N)` from the cache still requires going through `T`'s own
  `IsShared()`-gated locking discipline exactly as today — the cache only bypasses the O(depth)
  *walk to find* T, never T's own concurrency safety.

### 6.4 Structure

**Simplification found during Phase 0 implementation, superseding an earlier draft of this
section:** the original draft planned new slot-indexed accessors (`GetAtSlot`/`AddressOfAtSlot`/
`SetAtSlot`) mirroring `getValue`/`addressOfValue`/`setValue`. Reading `Get()`/`Set()`/`GetAddress()`
(`internal/language/symbols/get.go`/`set.go`) closely while wiring Phase 0 showed this is
unnecessary: **each of those methods already skips the entire parent-walk/`FindNextScope` branch
whenever the name is found in `s.symbols` on the *first* map probe** — `if !found && !s.IsRoot() {
...FindNextScope...}` never executes when `found` is true. So the O(depth) cost was never "the map
probe at the correct table"; it was always and only "walking through intervening tables to reach
it." That means Tier 2 doesn't need to reimplement storage access at all — it only needs to
remember *which table* to call the existing, unchanged `Get`/`Set`/`GetAddress` on directly, skipping
straight past the walk. This also removes the `SetAtSlot`-vs-`setValue` parity risk the original
Open Question 5 flagged, since no new write path is introduced — `Set()` itself runs verbatim,
just invoked on the already-known table instead of on `c.symbols`.

New file `internal/language/bytecode/globalcache.go`, modeled on `profile.go`'s storage pattern:

```go
type globalRefStorage struct {
    tables []unsafe.Pointer // one *symbols.SymbolTable per instruction offset, atomic
}
```

`*ByteCode` gains `globalRefs unsafe.Pointer` (same bare-`unsafe.Pointer`-not-`atomic.Pointer[T]`
rationale as `profile` — see Section 3.10). `ensureGlobalRefStorage()`/a small
`cachedTable(pc int) *symbols.SymbolTable` / `cacheTable(pc int, t *symbols.SymbolTable)` pair mirror
`ensureProfileSlot`'s lazy, `CompareAndSwapPointer`-guarded allocation (sized to
`len(b.instructions)`), but store a `*symbols.SymbolTable` pointer per PC rather than a struct with
counters — there's nothing to accumulate here, just one pointer to remember or not.

Each of `loadByteCode` (`load.go`), `storeByteCode` (`store.go`), `addressOfByteCode`/
`deRefByteCode` (`types.go`), and `unwrapByteCode`'s symbol-table fallback branch (`types.go`) gains
a fast-path check at the top: on a cache hit (`cachedTable(pc)` returns non-nil), call
`table.Get(name)`/`table.Set(name, v)`/`table.GetAddress(name)` **directly on that cached table**
instead of `c.symbols.Get(name)`/etc. — identical semantics (same coercion/readonly/underscore-
prefix handling, since it's the literal same method), just invoked on the table already known to
hold the name instead of the current call frame's table, skipping the walk entirely. On a miss,
call `c.symbols.Get(name)`/etc. exactly as today; if the name was found and
`resolvedTable.IsGlobalSingleton()`, call `cacheTable(pc, resolvedTable)` for next time (a benign
race if two goroutines populate the same correct answer concurrently).

One unified cache serves `var`, non-foldable `const`, and the type-assertion fallback — the same
shape of problem at the same handful of opcode-handler call sites; three parallel caches would
triplicate the same safety argument and atomics boilerplate for no benefit.

### 6.5 Gating

A separate setting (e.g. `ego.runtime.globalcache`), independent of Tier 1's flag and of the
register system, so each can be disabled independently during rollout/testing.

---

## 7. Phasing plan

1. **Phase 0 — groundwork.** `globalSingleton` field + `SetGlobalSingleton`/`IsGlobalSingleton`
   accessors on `SymbolTable`; wire the two production call sites. No behavior change (done — see
   Section 6.4's note on why no further accessors are needed here). Also: empirically confirm
   (cheap, one-off instrumentation) that `FindNextScope`'s
   `p.forPackage != ""` branch never actually fires in a real run — a full grep audit of every
   production `SetPackage` call site found none outside test files, so this branch appears dead,
   but it's worth a real confirmation before anything leans on that (Open Question 1).

2. **Phase 1 — const folding.** Section 5, in full. Re-profile `deep_check.ego`/
   `examples/mandelbrot2.ego` with `ego run --profiling` (the tool that found this bug) to confirm
   the `MaxIter` line drops to parity with the locals-only comparison line.

3. **Phase 2 — the runtime global cache.** Section 6, in full, covering both `package main`'s own
   top-level code (Finding 17's literal repro, no proxy shortcut today) and the
   already-working-acceptably imported-package proxy path (the mechanism naturally covers both — no
   reason to artificially restrict scope, per Open Question 4's discussion). Dedicated `-race`
   stress test: closures created in a loop, each referencing a package-level `var`, executed
   concurrently across goroutines, to validate the Section 6.3 safety argument empirically, not
   just on paper. Also test under `EGO_SERIALIZE_SYMBOLTABLES=true` to confirm the new field-based
   gate (not `IsShared()`) behaves correctly in that mode.

4. **Phase 3 — type-assertion fallback + wrap-up.** Extend `unwrapByteCode`'s `Get()` fallback to
   use the Phase 2 cache (small, mechanical, reuses everything Phase 2 built). Full re-profile pass
   per every other finding's existing convention (`--pprof` and `--profiling`/`--profile-file`).
   Write Finding 17's "Resolution" section in `docs/internals/PERFORMANCE.md`, following Finding
   7's/Finding 14's own template.

---

## 8. File-by-file touch list

| File | Phase | Change |
| --- | --- | --- |
| `internal/language/symbols/tables.go` | 0 | `globalSingleton` field on `SymbolTable` |
| `internal/language/symbols/root.go` | 0 | `SetGlobalSingleton`/`IsGlobalSingleton` methods (alongside `IsRoot`) |
| `internal/commands/run.go` | 0 | `SetGlobalSingleton()` call in `initializeSymbols` |
| `internal/language/compiler/import.go` | 0 | `SetGlobalSingleton()` call after `importSymbols` creation |
| `internal/language/compiler/compiler.go` | 1 | `constantValues map[string]any` field; propagate in `Clone()`/parent-child linkage |
| `internal/language/compiler/constant.go` | 1 | `foldConstantValue` helper; populate `c.constantValues` |
| `internal/language/compiler/slots.go` | 1 | `emitLoadName` new fold-check branch |
| `internal/language/compiler/assignment.go`, `var.go`, `function.go` | 1 | Record names into `c.nonConstLocalNames` at existing name-based-decl fallback points |
| `internal/defs/config.go` + i18n language files | 1/2 | New settings (`constfold`, `globalcache`) |
| `internal/language/compiler/directives.go` | 1/2 | `@compile` flag overrides mirroring `typeShadowing=` |
| `internal/language/bytecode/bytecode.go` | 2 | `globalRefs unsafe.Pointer` field on `ByteCode` |
| `internal/language/bytecode/globalcache.go` (new) | 2 | `globalRefSlot`/`globalRefStorage`/`ensureGlobalRefSlot` |
| `internal/language/bytecode/load.go`, `store.go`, `types.go` | 2/3 | Fast-path cache check in the five opcode handlers |
| `docs/internals/PERFORMANCE.md` | 3 | Finding 17 "Resolution" section |

---

## 9. Testing and verification strategy

Full regression bar for any interpreter-level change: `go build ./...`, `go vet ./...`,
`go test ./...`, `go test -race ./...`, the full `ego test tests/` suite (1,573+ cases), and
`tools/apitest.sh` against a live server (REST service bodies are ordinary Ego functions).

**New targeted tests:**

- Phase 0: unit tests for `SetGlobalSingleton`/`IsGlobalSingleton`; the empirical
  `forPackage`-dead-code confirmation run (throwaway instrumentation, not shipped).

- Phase 1: const-folding correctness — literal consts, `iota`-based typed enum consts, same-block
  const-referencing-const chains, and explicitly the shadowing hazard from Section 5.3 (a local
  `:=`/`var` reusing a package-const's name, both inside a register-eligible function and inside
  one disqualified from register eligibility, asserting the *local* value wins) as new `.ego` test
  cases plus Go unit tests.

- Phase 2: the closures-in-a-loop-referencing-a-global stress test under `-race`; a
  same-depth-different-path test (one call entirely within `main`, one that crosses an `InPackage`
  proxy, both referencing package-level `var`s) confirming no cross-contamination between cache
  entries; an `EGO_SERIALIZE_SYMBOLTABLES=true` run confirming the new field-based gate behaves
  correctly under that mode.

- Both tiers: re-run `deep_check.ego`/`shallow_check.ego` (Finding 17's own minimal repro) and
  `examples/mandelbrot2.ego` under `ego run --profiling`, confirming the previously-anomalous
  `MaxIter` line converges toward the locals-only line's per-hit cost.

- Re-profile every other Finding in `PERFORMANCE.md` per its own existing methodology, to catch any
  regression this change might introduce elsewhere.

---

## 10. Open questions / decisions needed before implementation

1. **`forPackage` dead-code confirmation.** Confirmed via grep audit of all production call sites
   (Section 6.2/Phase 0) that `SetPackage` has no production caller, so `FindNextScope`'s
   `forPackage != ""` branch is unreachable today — but not yet confirmed via runtime
   instrumentation across a real test/example/server run. Recommend doing the cheap instrumented
   confirmation in Phase 0. Note this design does not actually depend on this fact (a new field is
   used instead, Section 6.2), so this is lower-stakes than it might first appear, but worth closing
   out since it's referenced in the reasoning for why other candidate signals were rejected.

2. **Shared `bytecode.EvaluateConstantFragment` helper vs. a small duplicated helper.** Whether to
   export a shared helper for both the peephole optimizer's existing `executeFragment` and the new
   compiler-side const-folding, vs. keeping a small duplicated helper inside the `compiler` package.
   Recommend the latter for Phase 1, to avoid an unrelated cross-package refactor of `optimizer.go`
   as a side effect of this work.

3. **Coarse vs. precise const-shadowing exclusion (Section 5.3).** The coarse,
   whole-compilation-unit exclusion (one shadowing declaration anywhere disables folding for that
   name everywhere) versus a more precise per-scope-tracked alternative that only excludes actually-
   shadowed lexical regions. Recommend shipping the coarse version first; revisit only if profiling
   shows real-world constants going unfolded due to unrelated same-named locals elsewhere in the
   same file.

4. **Scope of Phase 2: `package main` only, or also the already-working imported-package proxy
   path?** Recommend covering both — the eligibility check is about the *destination* table, not
   the *path*, so restricting scope would mean deliberately disabling something the mechanism
   already gets for free. If a more conservative rollout is preferred regardless, gate the new
   setting's default to off initially rather than restricting it by code.

5. **Calling `T.Get`/`T.Set`/`T.GetAddress` directly on the cached table, bypassing `c.symbols`
   entirely.** Since these are the exact same methods, behavior should be identical — but confirm
   there's no other caller-context-dependent side effect (e.g. `ui.SymbolLogger` trace lines report
   `s.Name`/`s.id`, which will now show the cached table's identity instead of the current call
   frame's; this looks like a strictly more accurate log line, not a behavior change, but worth a
   quick visual check during implementation).

6. **Default-on vs. opt-in for the new settings at initial merge.** Correction found during
   Phase 1 implementation: `ego.compiler.registers` actually defaults to **off** (`registers :=
   false` in `New()`), only enabled when the optimizer level is set above 2 or via explicit
   config — not "shipped enabled by default" as this question originally assumed. `constfold`
   (Tier 1) instead follows `typeShadowing`'s pattern (default **on**, `constFold := true` unless
   explicitly overridden), since — unlike registers — it changes no observable semantics: every
   const that compiles successfully is already provably pure (Section 3.7), and the shadowing
   guard (Section 5.3) closes the one correctness risk. Tier 2 (the runtime cache, not yet
   implemented) still needs its own explicit decision when it's built — a project risk-tolerance
   call, not a technical one, but with less precedent to lean on than this question first assumed.
