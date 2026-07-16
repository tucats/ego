# Slot-Based Local Variable Access — Implementation Plan

**Status:** in progress — Phase 0 landed; Phase 1 landed and functional
(predicate, feature flag, opcodes, and emission). Slotted locals, iteration
counters, shadowing, `&x`, and `++`/compound-assign all compile to slot opcodes;
range vars, params, receivers, and named returns remain name-based (Phase 2).
All 1568 Ego tests and the Go suite pass with slots on and off; a hot arithmetic
loop runs ~20% faster with slots on. Remaining: introspection (Q3), apitest,
formal re-profiling; then Phase 2 breadth.

## Implementation progress log

- **Design decisions (from Section 11):** Q1 → Option B is the target (slots
  subsume Findings 4/8/11 scope-push elision for eligible functions), staged in
  after Phase 1's Option-A cut. Q2 → refined closure rule: a literal disqualifies
  its enclosing function only if the literal's body references an enclosing
  declared name. Q3 → full introspection support, including the debugger. Q4 →
  the legacy `Explode`/`explodeByteCode` opcode has been **deleted**.
- **Gating (decided during implementation):** a feature flag
  `ego.compiler.slots` (default **true**) acts as a kill-switch, cached in
  `c.flags.slots`; independent of the peephole optimizer level so `ego test`
  exercises slots. A per-block override `slots=true|false` is available on the
  `@compile` directive, mirroring `typeShadowing=`.
- **Phase 0 (complete, verified):** slot bank storage + accessors on
  `SymbolTable` (`internal/language/symbols/slots.go`); 7 new opcodes
  (`AllocateLocal`, `LoadSlot`, `StoreSlot`, `CreateAndStoreSlot`,
  `SymbolOptCreateSlot`, `SymbolDeleteSlot`, `AddressOfSlot`) with
  implementations in `internal/language/bytecode/slots.go`; `checkType`
  refactored to share `checkTypeCore` with the slot path; inert `scope.slots`
  bookkeeping field on the compiler.
- **Phase 1 (in progress):** eligibility predicate
  `functionBodyIsSlotEligible` (`internal/language/compiler/slots.go`) with the
  Q2 closure refinement, covered by `slots_test.go`. **Remaining:** per-name
  slot assignment threaded through `DefineSymbol`, the all-or-nothing conversion
  of variable-access emission sites (params, `:=`/`var`, reads, `&x`, named
  returns) to the slot opcodes for eligible functions, `AllocateLocal`
  emission/back-patch, the `LocalNames` introspection table, and `.ego`
  regression tests + re-profiling.
**Origin:** `docs/PERFORMANCE.md`, [Finding 7](PERFORMANCE.md#8-finding-7-architectural-high-effort-high-ceiling--name-based-symbol-resolution) ("name-based symbol resolution"), corroborated by Findings 9, 11, 13, and 14.

---

## Contents

- [1. Motivation](#1-motivation)
- [2. Goals and non-goals](#2-goals-and-non-goals)
- [3. Relevant existing architecture](#3-relevant-existing-architecture)
- [4. Design overview](#4-design-overview)
- [5. Compile-time eligibility and slot assignment](#5-compile-time-eligibility-and-slot-assignment)
- [6. New bytecode instructions](#6-new-bytecode-instructions)
- [7. Runtime changes](#7-runtime-changes)
- [8. Interaction with existing features](#8-interaction-with-existing-features)
- [9. Phasing plan](#9-phasing-plan)
- [10. Testing and verification strategy](#10-testing-and-verification-strategy)
- [11. Open questions / decisions needed before implementation](#11-open-questions--decisions-needed-before-implementation)
- [12. File-by-file touch list (anticipated)](#12-file-by-file-touch-list-anticipated)

---

## 1. Motivation

Every variable read or write in Ego today resolves the variable **by string name**
through a `map[string]*SymbolAttribute` at the current scope, recursing to parent
scopes on a miss (`internal/language/symbols/get.go`, `set.go`). Findings 9, 11, 13,
and 14 all independently corroborate this as a real, generalizable cost once a
program uses more than one or two distinct variable names — `runtime.mapaccess2_faststr`
is the largest interpreter-owned cost category in the Mandelbrot profiles after
Finding 9's fix landed.

Finding 7 itself was intentionally left as "not recommended as a next step" at the
time it was written, because Findings 1-4/8/9/11/13/14 were all cheaper and
addressed a comparable-or-larger fraction of measured cost. With those now landed,
this is the next-largest remaining architectural lever, and the one explicitly
flagged as the ceiling on how fast a tree-walking, name-based interpreter can get.

This document plans that work. It is a design and risk document, not an
implementation — no code changes are part of this pass.

---

## 2. Goals and non-goals

**Goals:**

- Eliminate the map lookup (and, where possible, the scope-object/table allocation)
  for **well-defined local-scope variable accesses** — parameters and `:=`/`var`
  locals whose storage location can be fully determined at compile time.
- Do this via new, distinctly-named bytecode instructions (`AllocateLocal`,
  `LoadSlot`, `StoreSlot`, and slot equivalents of the other name-based
  create/store opcodes) rather than overloading `Load`/`Store`/etc. with a second
  operand shape. A slot-based instruction stream must be visually and
  mechanically distinguishable from a name-based one.
- Preserve 100% of today's dynamic-mode semantics (`ego.compiler.types=dynamic`,
  the default) for any code shape the compiler cannot prove safe for slots —
  falling back to the existing, unchanged name-based path, exactly as Findings
  4/8/11 already do for scope elision.
- Preserve every escape hatch that depends on name-based access today: the
  debugger, `@global`, package proxy write-back, panic/recover formatting,
  reflection (if any exists over locals — see [Section 11](#11-open-questions--decisions-needed-before-implementation)).

**Non-goals (for this plan; may become later, separately-designed work):**

- Slotting closures' captured variables ("upvalues"). See
  [Section 5](#5-compile-time-eligibility-and-slot-assignment) — a function
  containing a closure/`go`/`defer` is disqualified from slot treatment in this
  plan's scope, mirroring the same conservative posture Findings 4/8/11 already
  established at the block/loop level.
- Slotting package-level or global variables. Only a single function activation's
  own parameters and block-locals are in scope for this design.
- A general register-allocation pass that reuses slot numbers between
  non-overlapping sibling blocks to minimize array size. V1 gives every distinct
  declared name in a function its own slot for the life of the call; packing is a
  possible later refinement, not required for the win this targets.

---

## 3. Relevant existing architecture

This section is a factual recap of the mechanics this design must build on or
interoperate with — not proposed changes.

### 3.1 `SymbolTable` already separates "value storage" from "name lookup"

`internal/language/symbols/tables.go` and `values.go`: a `SymbolTable` already
stores its values in `values []*[]any` — a growable array of fixed-size bins,
indexed by an integer `SymbolAttribute.slot`, specifically so that
`addressOfValue(slot)` pointers remain stable even as the table grows. **The
map (`symbols map[string]*SymbolAttribute`) exists purely to translate a name to
that integer slot** — the array-of-slots mechanism this plan wants already
exists, one level down; what's missing is a way to skip the map and know the
slot number *at compile time* instead of *at first use, per table instance*.

### 3.2 Scopes are dynamic, per-call, per-block runtime objects

A new `*symbols.SymbolTable` is created by `PushScope` on: every function call
(`pushScopeByteCode`, `callFramePush`), every loop iteration or whole-loop
scope (Findings 4/11), and every remaining unelided block (Finding 8). Its
`parent` pointer chains to the enclosing scope; `boundary=true` marks a
function-call scope so name lookups stop there and jump to `FindNextScope()`
(package/global scope), not the caller's locals.

### 3.3 Closures capture a live `*SymbolTable` pointer, not a name

`internal/language/bytecode/bytecode.go`: a function literal's `capturedScope`
field is a pointer to the `*symbols.SymbolTable` active at the point the
closure was created (`callBytecodeFunction.go`). Lifetime is automatic — Go's
GC keeps the table (and its parent chain) alive for as long as any closure
still references it, with no explicit pooling or "this scope has ended" event.
Any slot design must not break this for functions containing closures — this
plan's answer is to not attempt it in v1 (see [Section 5](#5-compile-time-eligibility-and-slot-assignment)).

### 3.4 Package/global name resolution walks past call-stack boundaries

Finding 14: `FindNextScope()` walks past the current function's own boundary
scope to reach package/global tables. Package member access also goes through
proxy tables (`symbols.NewChildProxy`) inserted by the `InPackage` opcode. None
of this is affected by this plan — any name not local to the current function
keeps going through the existing name-based path unchanged.

### 3.5 The compiler already tracks per-scope variable bookkeeping, just not slot numbers

`internal/language/compiler/symbols.go`: `c.scopes []scope`, each with a
`usage map[string]*errors.Error`, already exists — pushed/popped in lockstep
with `PushScope`/`PopScope` at compile time, and used today only for
"declared but never used" / "unknown symbol" diagnostics (`DefineSymbol`,
`ReferenceSymbol`, `validateSymbol`). This is the natural place to also record
a slot number per name, following the same "scan/decide once at compile time"
pattern Findings 4/8/11 already established (`loopBodyNeedsFreshScopePerIteration`,
`blockBodyNeedsOwnScope`, `loopBodyIdempotentDeclEligible`).

### 3.6 Bytecode instructions that read or write a *named* local today

From `internal/language/bytecode/opcodes.go`, the instructions whose operand
is (or can be) a plain variable name and that would need a slot-based
equivalent for a fully slotted function: `Load`, `Store`, `StoreAlways`
(non-array-operand form), `CreateAndStore`, `SymbolCreate`, `SymbolOptCreate`,
`SymbolDelete`, `AddressOf`, `Increment` (used by some auto-increment paths),
and the name-operand form of `StoreViaPointer`. `GetThis`/`SetThis` and
`GetVarArgs` bind to fixed, compiler-known synthetic names (`__this`,
`__args`) rather than user-chosen ones — these can be treated as fixed,
reserved slot numbers rather than needing general dynamic-name support.

`explodeByteCode` (`Explode` opcode) creates variables at runtime from a map's
*runtime* keys — a genuinely dynamic-name construct. It currently has **no
compiler call site at all** (confirmed by search — dead code, or reachable
only via hand-built `*ByteCode` outside the compiler package). This needs
confirming, not assuming, before implementation (see
[Section 11](#11-open-questions--decisions-needed-before-implementation)); if
truly unreachable from compiled Ego source, it's a non-issue. If some path
does reach it, any scope where it can run must be disqualified from slot
treatment (or, more precisely, this design already excludes runtime-dynamic
name creation from the "well-defined" locals it targets).

---

## 4. Design overview

**One slot bank per eligible function activation.** When the compiler proves a
function body eligible (Section 5), it emits `AllocateLocal <n>` once, at
function entry, where `n` is the total number of distinct parameter/local
names declared anywhere in the function body (including nested, non-closure
blocks). At runtime this allocates a single `[]any` of exactly size `n` —
no growth, no bins, since `n` is fixed and known — and every slot starts as
`UndefinedValue{}` exactly like today's `Create()`.

**New instructions read/write that array by integer index directly** —
`LoadSlot <n>` / `StoreSlot <n>` / slot equivalents of `CreateAndStore`,
`SymbolCreate`/`SymbolOptCreate`, `SymbolDelete`, and `AddressOf` — with no
map involved and no `data.String(i)` conversion (the operand is already an
`int`).

**Everything else about the call is unchanged.** The function's boundary
`*symbols.SymbolTable` is still pushed/popped exactly as today (so `this`,
`FindNextScope`, package-proxy write-back, and the try/defer/panic machinery
all keep working verbatim) — the slot bank is carried as a new field *on that
table* (see [Section 7](#7-runtime-changes)), not as a replacement for it.
This is the intentionally conservative shape of this plan: it is additive to
Findings 4/8/11, not a replacement for them, and it composes cleanly with
each. A more ambitious version that also eliminates the boundary-scope table
push itself is discussed as a later option in
[Section 11](#11-open-questions--decisions-needed-before-implementation), since
it changes the scope of this project materially and should be an explicit
decision, not an assumption baked into the plan.

**Any reference to a name that is *not* one of this function's own
slotted locals** (a package-level constant, an imported package member, a
global, `@global`, an outer function's variable reached only because
`ego.runtime.deep.scope` disables boundary isolation) compiles to the
existing `Load`/`Store`/etc. exactly as today. The compiler only emits a slot
instruction when it has proven, at the point of compiling that specific
identifier reference, that the name resolves to a slot in the *current*
function's own bank.

---

## 5. Compile-time eligibility and slot assignment

### 5.1 Eligibility predicate (function granularity, conservative)

A function body (named or literal) is **slot-eligible** only if a one-time,
conservative token-level scan of its own body (not nested function bodies,
which are compiled and judged independently) — in the same style as
`loopBodyNeedsFreshScopePerIteration` / `blockBodyNeedsOwnScope` /
`loopBodyIdempotentDeclEligible` — finds **none** of the following, anywhere:

- A function literal, a `go` statement, or a `defer` statement. This is what
  lets this plan skip the closure/"upvalue" capture problem entirely (Section
  3.3): a slot-eligible function's locals are, by construction, never
  referenced from any scope that could outlive the function's own call frame.
- Any construct that creates a local by a name not known at compile time
  (confirm during implementation whether any such construct is reachable —
  see Section 3.6 and Section 11).
- (Package-body top-level code is out of scope entirely — only the bodies of
  individual functions compiled within a package are candidates; the
  package's own export/proxy table is untouched.)

Being overly cautious here only costs a missed optimization, never
correctness — exactly the design stance Findings 4/8/11 already took, and for
the same reason: the profiled evidence (tight loops, Mandelbrot, function-call
workloads) shows the overwhelming majority of hot-path functions contain no
closures at all, so a function-granularity, whole-or-nothing predicate still
captures the large majority of the available win without needing to solve
per-variable escape analysis up front.

### 5.2 Per-name slot assignment

Extend the existing `c.scopes []scope` bookkeeping (Section 3.5): when
compiling a slot-eligible function, each `DefineSymbol` call (parameter,
`:=`, `var`, named return, receiver) also assigns the next free integer slot
number for that name, recorded alongside the existing `usage` entry. Nested
non-eliminated blocks (see Section 5.3) still push/pop `c.scopes` entries at
compile time exactly as today (this is compile-time bookkeeping only, not a
runtime table push) — a name declared in a nested block gets its own slot
number, distinct from any outer or sibling block's use of the same name,
exactly matching today's shadowing semantics but resolved once, at compile
time, instead of via a runtime map+chain walk.

`AllocateLocal <n>` is emitted once, after the whole function body has been
scanned/compiled and the final count of distinct slots is known (or with a
back-patch, similar to how branch targets are already back-patched elsewhere
in the compiler — implementation detail to confirm).

**`:=` redeclaration becomes a compile-time question.** Today, `SymbolCreate`
errors at runtime if the name already exists in the current table
(`ErrSymbolExists`); Finding 11 had to introduce `SymbolOptCreate` specifically
to make loop-body redeclaration idempotent across iterations. With
compile-time slot assignment, "has this name already been declared at this
exact lexical nesting" is something the compiler already tracks in `c.scopes`
— so the erroring-vs-idempotent decision (and Finding 11's whole
`loopBodyIdempotentDeclEligible` analysis) collapses into a simpler,
free-of-charge byproduct of slot assignment for any slot-eligible loop body:
the *same* slot number is reused across iterations by construction, and no
runtime existence check is needed at all. This is a secondary, incidental
simplification this design enables — not a required part of it, and not a
license to remove Finding 11's existing logic, since non-eligible functions
still need it exactly as today.

### 5.3 Nested block scopes within an eligible function

Open design decision — see
[Section 11, Q1](#11-open-questions--decisions-needed-before-implementation).
Two options:

- **(A) Minimal scope (recommended default for v1):** nested blocks
  (`if`/`for`/`switch`/`try` bodies) that Findings 4/8/11 haven't already
  elided still push/pop their own child `*symbols.SymbolTable` at runtime,
  exactly as today — but that child table has no slot bank of its own; slot
  instructions inside it reference the *function's* bank via a pointer/lookup
  to the nearest ancestor table that owns one. This keeps this plan strictly
  additive and low-risk: it changes nothing about scope-push counts, only
  about the cost of each variable access within whatever scopes already exist.
- **(B) Maximal scope:** since slot numbers alone already provide the
  uniqueness and shadowing guarantees a runtime scope boundary exists to
  provide, a fully slot-eligible function could skip pushing *any* additional
  child table for its own nested blocks — subsuming Findings 4/8/11 for these
  functions rather than composing with them, and eliminating scope-push
  overhead entirely (not just per-access cost) for the functions this
  predicate accepts.

Option B is a strictly bigger win but a materially bigger change (it touches
every remaining `PushScope`/`PopScope` call site's eligibility, not just
`Load`/`Store`), and interacts with `try`'s own documented scope-retention
requirement (Section 8) and the still-open BUG-61 (`switch`/`continue`
scope-cleanup bug) in ways that need their own careful analysis, not an
assumption made in passing here.

---

## 6. New bytecode instructions

| Instruction | Operand | Semantics |
| - | - | - |
| `AllocateLocal` | `int` (slot count `n`) | Allocates `make([]any, n)`, each initialized to `UndefinedValue{}`, and attaches it to the current (just-pushed) `*symbols.SymbolTable` as its slot bank. |
| `LoadSlot` | `int` (slot index) | Pushes `bank[index]` (after `data.UnwrapConstant`, matching `loadByteCode`). Runtime error if no bank is attached to the reachable table chain (should be impossible if the compiler is correct — see Section 10 for how this is tested). |
| `StoreSlot` | `int` (slot index) | Pops a value, applies the same `checkType`/`copyStructForValueSemantics` treatment `storeByteCode` applies today, writes `bank[index] = value`. |
| `CreateAndStoreSlot` | `int` | Slot equivalent of `CreateAndStore` — combines "was this the first `:=` for this slot" bookkeeping (already resolved at compile time, so this can usually just be a plain store) with the same constant/readonly special-casing `createAndStoreByteCode` does today. |
| `SymbolOptCreateSlot` | `int` | Only needed if Option A of Section 5.3 requires a runtime existence check for some case the compiler couldn't fully resolve; likely unnecessary in the common case per Section 5.2's "becomes a compile-time question" analysis — confirm during implementation whether this opcode is needed at all or whether `StoreSlot` alone suffices for every slot-eligible `:=`. |
| `SymbolDeleteSlot` | `int` | Resets `bank[index] = UndefinedValue{}` — slot equivalent of `SymbolDelete` for the (rare, currently name-known-at-compile-time-only) call sites in `expr_atom.go`/`switch.go`. |
| `AddressOfSlot` | `int` | Returns `&bank[index]` (a `*any`) — directly analogous to `addressOfValue`, and simpler, since the array never grows after `AllocateLocal` (no bin math needed). |

Readonly/constant handling (today's `_`-name-prefix convention, checked at
runtime by string-prefix-testing the name) becomes a **compile-time-known
boolean per slot** instead — the compiler already knows, from the declaration
site, whether a given slot is a `_`-prefixed constant, so `StoreSlot`'s
generated bytecode can simply be the constant-aware or plain variant as
appropriate, rather than re-deriving it from the name at runtime on every
store. This is a secondary simplification worth confirming during design
review, not a required part of the opcode semantics above.

All new opcodes need: a dispatch table entry (`opcodes.go`), a
human-readable name string (`opcodes.go`), an implementation (new file,
e.g. `internal/language/bytecode/slots.go`), and disassembler support
(`disassembler.go` — should be no different in kind from any existing
int-operand instruction). The existing peephole optimizer
(`internal/language/bytecode/optimizer.go`/`optimizations.go`) has patterns
keyed on specific opcodes (e.g. merging sequential `PopScope`s) — these need a
pass to confirm none of them assume every `Load`/`Store` in a function is
name-based, and to consider whether any *new* slot-specific peephole patterns
are worth adding later (not part of this plan's initial scope).

---

## 7. Runtime changes

- **`internal/language/symbols/tables.go`:** add a `locals []any` (or a small
  wrapper struct, if debug metadata needs to travel with it — see Section 8.2)
  field to `SymbolTable`, populated by a new method (e.g. `AllocateLocals(n
  int)`), read/written by unexported helpers analogous to `getValue`/`setValue`
  but with no bin math (`locals[index]` directly). Needs a way for a *child*
  table (Section 5.3, Option A) to find the nearest ancestor's bank — a new
  `LocalsBank() *SymbolTable` walk, likely cacheable the same way
  `FindNextScope` is (Finding 14, Phase 3) if profiling shows it matters,
  though for Option A the common case is zero additional child tables per
  Finding 8, so this may not be a hot path at all.
- **`internal/language/bytecode/context.go` / `callframe.go`:** no changes
  anticipated beyond what's already needed to call the new
  `AllocateLocals`/slot accessor methods — `PushScope`/`callFramePush` are
  otherwise unchanged (Section 4).
- **New file, e.g. `internal/language/bytecode/slots.go`:** the seven opcode
  implementations from Section 6.

---

## 8. Interaction with existing features

- **Debugger (`internal/language/debugger/`) and any other name-based
  introspection** (`show symbols`, error-context formatting, `recover()`
  reading a named return by name, the "unread errors" extension): these need
  a name↔slot mapping to keep working for a slot-eligible frame, since the
  runtime slot bank itself carries no names. Plan: the compiler additionally
  emits a small debug-info table (e.g. `LocalNames []string`, indexed by slot
  number) attached to the `*ByteCode` object, alongside the `n` passed to
  `AllocateLocal` — any tool that needs a name for a given frame's local zips
  `bc.LocalNames` with `c.symbols.locals` on demand. This is a cold path
  (debugger commands, error formatting, not the hot instruction-dispatch
  loop), so its cost is irrelevant to this project's goals. Each of the
  specific call sites listed above needs to be located and updated
  individually during implementation — this plan identifies the *mechanism*
  (name/slot debug table) but has not yet enumerated every consumer; see
  [Section 11, Q3](#11-open-questions--decisions-needed-before-implementation).
- **`try`'s own body scope retention** (Finding 8's Resolution notes this
  scope is deliberately never elided, because the `catch` clause depends on
  it staying open across an error jump). A `try` body inside a slot-eligible
  function is compiled exactly as today under Section 5.3 Option A (its own
  child table, no bank of its own) — unaffected by this design. Under Option
  B this would need its own explicit analysis; flagged, not solved, here.
- **`switch`/`continue` (BUG-61, still open):** Section 5.3 Option A leaves
  `switch` case/default bodies exactly as today, so this design neither fixes
  nor worsens that pre-existing bug. Option B would need to account for it
  explicitly before touching `switch`.
- **Goroutines / shared tables:** a slot-eligible function's bank belongs
  solely to one call activation on one goroutine (no closures allowed in, by
  construction — Section 5.1), so none of Finding 14's shared-table/locking
  concerns apply to the bank itself. A slot-eligible function can still be
  the *target* of a `go` statement (a named-function call, not a literal) —
  that's an ordinary fresh call with its own fresh bank, no different from
  any other call.
- **Package proxy write-back (`updatePackageFromLocalSymbols`):** only
  triggers for tables with `forPackage != ""` / `isClone`, which ordinary
  function-local tables never are — unaffected.
- **AddressOf / pointer semantics:** `&localVar` for a slotted local returns
  `&bank[index]` (`*any`), directly compatible with the existing
  `storeViaPointerByteCode`'s `*any` case — no new pointer-target type needed.
- **Struct-copy-on-store (BUG-26/BUG-43) and type-checking (`c.checkType`):**
  `StoreSlot`'s implementation must call the same
  `copyStructForValueSemantics`/`checkType` logic `storeByteCode` already
  applies — these are value-semantics rules, not name-lookup mechanics, and
  are completely orthogonal to how the destination slot was addressed.
- **Named returns and the receiver (`this`):** ordinary declared names from
  the compiler's point of view (Section 3.6) — eligible for slots like any
  other local, *unless* disqualified by Section 5.1 (a `defer func(){...}()`
  that mutates a named return is exactly the closure case that already
  disqualifies the whole function).
- **`ego.runtime.deep.scope` setting** (disables function-scope boundary
  isolation, used by `ego test`): `pushScopeByteCode` already branches on this
  today; a slot-eligible function's `AllocateLocal`/bank mechanism is
  independent of whether the *boundary* flag is set on its table, so this
  setting should need no special-casing — worth a dedicated test case rather
  than an assumption (see Section 10).

---

## 9. Phasing plan

1. **Phase 0 — groundwork and instrumentation.** Add the new opcodes
   (dispatch + disassembler + no-op-safe implementations), the `locals`
   field and accessor methods on `SymbolTable`, and the compiler-side slot
   bookkeeping extension to `c.scopes` — but do not yet change what any real
   function compiles to. Land this inert scaffolding first so Phase 1's diff
   is reviewable as "turn the new mechanism on for one case," not
   "everything at once."
2. **Phase 1 — MVP eligibility.** Implement `Section 5.1`'s predicate and
   `Section 5.2`'s slot assignment for the narrowest useful case: a function
   with no closures/`go`/`defer` anywhere, using Section 5.3 Option A (no
   change to scope-push counts). Re-profile the exact workloads Findings
   7/9/11/13/14 already used (tight loop, function-call loop, Mandelbrot
   iterative and recursive) using the existing `--pprof` methodology
   (`docs/PERFORMANCE.md` Section 1) for a like-for-like comparison.
3. **Phase 2 — breadth.** Extend the predicate to cover named returns,
   receivers, and any remaining common patterns Phase 1's re-profiling shows
   are still falling back to the name-based path in real (not synthetic)
   programs — driven by evidence, the same way Finding 8 followed Finding 4
   and Finding 11 followed Finding 4 again.
4. **Phase 3 (not recommended without further design, evaluated only after
   Phases 1-2 land) — Section 5.3 Option B** (eliminate nested-block scope
   pushes entirely for eligible functions) and/or **per-variable closure
   capture analysis** (extending eligibility to functions containing a
   closure that doesn't actually capture every local). Both are flagged, in
   the same spirit as Finding 14's declined Phase 4, as needing a dedicated
   design pass gated on how much headroom remains after the cheaper phases —
   not committed to as part of this plan.

---

## 10. Testing and verification strategy

Following the precedent set by every fix in `docs/PERFORMANCE.md`:

- New Go unit tests directly exercising the eligibility predicate (in the
  style of `TestLoopBodyNeedsFreshScopePerIteration` /
  `TestBlockBodyNeedsOwnScope` / `TestLoopBodyIdempotentDeclEligible`) —
  positive and negative cases for every disqualifying construct in Section
  5.1, plus malformed/edge-case input.
- New Go unit tests for the opcode implementations themselves
  (`internal/language/bytecode/slots_test.go`), mirroring the existing
  per-opcode test files in that package.
- New `.ego`-level regression tests under `tests/flow/` (or a new
  `tests/flow/slots.ego`) covering: a slot-eligible function called many
  times with distinct argument values (confirming no cross-call bleed
  between activations), nested `if`/`for`/`try` bodies inside an eligible
  function each declaring their own same-named locals (shadowing), `&x` on a
  slotted local, a `_`-prefixed constant inside an eligible function, a named
  return mutated via a plain (non-deferred) code path, and at least one
  disqualifying case per Section 5.1 confirmed to still fall back to
  identical behavior as today (ideally by running the same test body through
  both a stashed pre-change compiler and the new one, as Finding 4's own
  verification did).
- Full existing suite must stay green and is the real correctness bar here,
  given how much of the interpreter this touches: `go build ./...`,
  `go vet ./...`, `go test ./...`, `go test -race ./...` (this touches
  concurrent-adjacent code even though slot banks themselves are
  single-goroutine-only by construction — the *decision* of whether a
  function is eligible must not itself race), and the full `ego test tests/`
  suite (1,558+ cases as of Finding 11's Resolution).
- Re-run `tools/apitest.sh` against a live server, since REST service bodies
  are ordinary Ego functions and this change touches how every function call
  executes.
- Re-profile with the existing `--pprof` methodology and compare against the
  post-Finding-14 baseline numbers already recorded in `docs/PERFORMANCE.md`,
  the same way every prior finding's Resolution section did — this plan
  should get its own Resolution section(s) in `PERFORMANCE.md` once
  implemented, not a separate results doc.

---

## 11. Open questions / decisions needed before implementation

These are the points where this plan made a specific, stated choice but a
different one is plausible, or where investigation during implementation
could change the shape of the work. Flagging explicitly rather than
deciding silently, per the brief for this document.

1. **Section 5.3: Option A (additive, no scope-push changes) vs. Option B
   (slots subsume Findings 4/8/11's scope-push elision for eligible
   functions).** This plan recommends starting with Option A because it is
   strictly smaller and composes with, rather than replaces, already-shipped
   work — but Option B is where the *rest* of the win in Findings 4/8/11's
   territory lives for slot-eligible functions (no scope allocation at all,
   not just cheaper access within whatever scopes exist). Worth a decision
   before Phase 1 starts, since it affects how the eligibility predicate and
   `PushScope`/`PopScope` emission are threaded together from the start, even
   if Option B's actual implementation is deferred to Phase 3. **The user
   selects option B for Q1**

2. **How aggressively should Phase 1's eligibility predicate treat
   closures?** This plan's default (Section 5.1) disqualifies the *entire
   enclosing function* if a closure/`go`/`defer` appears anywhere in its
   body, even if the closure captures nothing from the outer function at
   all (a closure that only uses its own parameters, e.g. a `sort.Slice`
   comparator literal, still disqualifies its enclosing function under this
   rule). A cheap refinement worth considering even for Phase 1: only
   disqualify if the closure's own body actually references a name declared
   in the enclosing function (a bounded, local check, not general escape
   analysis) — reducing false disqualifications without taking on the harder
   "upvalue" problem. Worth deciding before implementation since it changes
   the predicate's shape, not just its precision. **The user would like
   the additional refinement of disqualifying if the closure's own body
   references a name declared in teh enclosing function).**

3. **Full enumeration of name-based introspection consumers.** Section 8
   names debugger, `recover()`/named-return access, and the "unread errors"
   extension as needing the `LocalNames` debug-table treatment, based on
   what this investigation found — but this was not an exhaustive audit of
   every place in the codebase that calls `SymbolTable.Names()`/`Get()`
   expecting to enumerate a live function's *user* locals (as opposed to
   package/debug/admin uses, which are unaffected). This needs a dedicated
   `grep`-and-verify pass at the start of implementation, not an assumption
   that Section 8's list is complete. **The user would like the additional
   introspection support, especially to handle the debugger**.

4. **Confirm `explodeByteCode`/`Explode` is genuinely unreachable from
   compiled Ego source** (Section 3.6) before relying on that in the
   eligibility predicate design. If some path does reach it (e.g. a REST
   service framework building bytecode by hand rather than through the
   compiler), the predicate needs an explicit rule for it. **The user
   has determined that this is legacy code and can be deleted**.
   
5. **Slot array sizing for pathologically large functions.** V1 gives every
   distinct name its own slot with no reuse across non-overlapping sibling
   blocks (Section 2, non-goals). This is very unlikely to matter in
   practice, but worth a sanity check against the largest real function
   bodies in `lib/packages/` during Phase 1 rather than assuming it's a
   non-issue.

---

## 12. File-by-file touch list (anticipated)

Based on the architecture review in Section 3 — subject to revision once
implementation begins and the open questions in Section 11 are resolved.

| File | Anticipated change |
| - | - |
| `internal/language/bytecode/opcodes.go` | 7 new opcode constants, name strings, dispatch entries |
| `internal/language/bytecode/slots.go` (new) | Opcode implementations |
| `internal/language/bytecode/disassembler.go` | Operand formatting for new int-operand opcodes (likely no change needed if it already handles generic int operands generically — confirm) |
| `internal/language/symbols/tables.go` | `locals []any` field, `AllocateLocals`, bank-lookup helper |
| `internal/language/symbols/values.go` (or new file) | Slot bank get/set/address helpers (no bin math) |
| `internal/language/compiler/symbols.go` | Extend `scope` struct with slot-number bookkeeping; new eligibility-predicate function (in the style of `blockBodyNeedsOwnScope` et al.) |
| `internal/language/compiler/function.go` | Emit `AllocateLocal`; route parameter/named-return/receiver binding through slot ops when eligible |
| `internal/language/compiler/expr_atom.go`, `lvalue.go`, `assignment.go` | Emit `LoadSlot`/`StoreSlot`/etc. instead of name-based ops for identifiers resolved to a local slot |
| `internal/language/compiler/for.go`, `if.go`, `try.go`, `switch.go` | Confirm Section 5.3 Option A requires no changes here (nested blocks keep pushing tables as today); revisit if Option B is pursued later |
| `internal/language/debugger/*.go` | Consume `LocalNames` debug table for any slot-backed frame (Section 11, Q3, pending audit) |
| `docs/PERFORMANCE.md` | New Resolution section(s) once implemented, with before/after profiling, following the existing document's own format |
| `tests/flow/slots.ego` (new), `internal/language/compiler/*_slots_test.go` (new) | New tests per Section 10 |
