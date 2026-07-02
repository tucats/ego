# Ego Issues — Consolidated Bug, Design, and Security Tracking

## Introduction

This document consolidates every issue-tracking document that previously lived
separately under `docs/issues/`: `BUGS.md`, `FUNCTIONAL_ISSUES.md`,
`BUILTIN_ISSUES.md`, `BYTECODE_ISSUES.md`, `DEBUGGER_ISSUES.md`, and
`SECURITY_ISSUES.md`. Those files remain in place unmodified for historical
reference and are cross-linked from any related documentation, but this file
is the single, up-to-date place to look up a specific issue.

Each source document originally used its own audit and covered a distinct
focus area of the codebase — general language behavior, functional/behavioral
differences from Go, the `builtins` package, the bytecode instruction set, the
interactive debugger, and a security review of the server and CLI. Rather than
duplicate that structure as separate files, this document keeps each
originating focus area as its own group of chapters (noted below), while
presenting every individual **issue topic area** as its own chapter with a
consistent format:

- an anchored chapter heading for the topic area (e.g. `BUG`, `CALL`, `LOGIN`),
- a table of the issues in that area with a one-sentence summary and a ✓ if
  the issue has been resolved, and
- the full text of each issue immediately below the table, including its
  resolution where one exists.

**Naming convention:** every issue has a stable identifier of the form
`AREA-N` (e.g. `FLOW-M4`, `CALL-3`, `LOGIN-C1`), where `AREA` designates the
topic area and `N` is a sequence number (sometimes itself prefixed with a
severity/priority letter such as `C`/`H`/`M`/`L`). These identifiers are
referenced from unit tests and, in some cases, from source comments — **they
are never renumbered or renamed** when material is reorganized, including in
this consolidation.

**Where resolution details lived separately from the issue description** in a
source document (most notably a "Remediation Checklist" section at the end of
`FUNCTIONAL_ISSUES.md` and `SECURITY_ISSUES.md`), that information has been
merged into the issue's own section below, immediately after its description
and recommendation. The standalone checklists themselves are not reproduced
here — their content now lives inline, and their resolved/unresolved status is
reflected in each area's summary table.

**Auditing context**, preserved from the original documents:

- **General Language Bugs** (originally `BUGS.md`): Tracks general Ego-language bugs discovered through systematic testing, distinct from the documented behavioral differences tracked elsewhere. BUG-16 cross-references FLOW-M4 (defer lazy argument evaluation) and is included here only for completeness, not as a duplicate open issue.
- **Functional / Behavioral Issues** (originally `FUNCTIONAL_ISSUES.md`): Records known behavioral differences between Ego and Go, plus Ego-specific limitations, uncovered during testing of functions, flow control, the JSON package, the type system, and the @transaction scripting endpoint.
- **Builtin Function Issues** (originally `BUILTIN_ISSUES.md`): Documents behavioral anomalies, potential bugs, and design concerns found during a comprehensive review of the builtins Go package. All issues discovered in the initial audit have been resolved.
- **Bytecode Instruction Issues** (originally `BYTECODE_ISSUES.md`): Documents behavioral anomalies, potential bugs, and design concerns found during a comprehensive bytecode-instruction unit-test effort covering branch, call, math, optimizer, range, store, struct-indexing, and other opcode-execution paths.
- **Debugger Package Issues** (originally `DEBUGGER_ISSUES.md`): Documents behavioral anomalies, potential bugs, and design concerns found during a comprehensive review of the debugger package, which intercepts the ErrSignalDebugger sentinel from the bytecode.Context run loop to offer an interactive prompt.
- **Security Issues** (originally `SECURITY_ISSUES.md`): Records known security weaknesses in Ego found via security code reviews (April-June 2026) across authentication, WebAuthn, the HTTP server, the tables and asset endpoints, profile encryption, dashboard code execution, and the OAuth2 Authorization/Resource Server. Each issue documents affected files, a description, a recommendation, and (where resolved) the resolution actually implemented.

Across all six areas, this document currently tracks **224 issues**:
**216 resolved** and **8 still open**. Open issues are
listed in their area's table with a blank status cell and include whatever
Description/Recommendation the source audit already had — no resolution is
invented for them here.

You can find a specific issue two ways:

1. **Direct lookup by ID** — every issue ID in this document is a unique,
   literal anchor (e.g. `#LOGIN-C1`, `#CALL-3`). Search the page for the ID,
   or use the [Issue Index](#issue-index) below, which lists every ID
   alphabetically with a direct link.
2. **Browse by topic area** — use the [Table of Contents](#table-of-contents)
   below to find the topic area chapter, then its summary table for the
   specific issue.

---

<a id="table-of-contents"></a>

## Table of Contents

### General Language Bugs

*(originally `BUGS.md`)*

- [BUG — General Language Bugs](#area-bug) — 24 issues (22 resolved)

### Functional / Behavioral Issues

*(originally `FUNCTIONAL_ISSUES.md`)*

- [FUNC — Functions](#area-func) — 7 issues (7 resolved)
- [FLOW — Flow Control](#area-flow-functional) — 6 issues (6 resolved)
- [JSON — JSON package](#area-json) — 5 issues (5 resolved)
- [TYPE — Type system](#area-type) — 1 issue (1 resolved)
- [SCRIPT — @transaction Scripting endpoint](#area-script) — 4 issues (4 resolved)

### Builtin Function Issues

*(originally `BUILTIN_ISSUES.md`)*

- [BUILTIN-APPEND — append.go — Append](#area-builtin-append) — 1 issue (1 resolved)
- [BUILTIN-CAST — cast.go — Cast, castToStringValue](#area-builtin-cast) — 2 issues (2 resolved)
- [BUILTIN-COPY — copy.go — DeepCopy](#area-builtin-copy) — 2 issues (2 resolved)
- [BUILTIN-DELETE — delete.go — Delete](#area-builtin-delete) — 1 issue (1 resolved)
- [BUILTIN-FUNCTIONS — functions.go — AddBuiltins, FindFunction, FindName, CallBuiltin, AddFunction](#area-builtin-functions) — 2 issues (2 resolved)
- [BUILTIN-INDEX — index.go — Index](#area-builtin-index) — 1 issue (1 resolved)
- [BUILTIN-LENGTH — length.go — Length](#area-builtin-length) — 1 issue (1 resolved)
- [BUILTIN-MAKE — make.go — Make](#area-builtin-make) — 2 issues (2 resolved)
- [BUILTIN-NEW — new.go — NewInstanceOf, newReflectKind](#area-builtin-new) — 2 issues (2 resolved)
- [BUILTIN-TYPES — types.go — typeOf](#area-builtin-types) — 1 issue (1 resolved)

### Bytecode Instruction Issues

*(originally `BYTECODE_ISSUES.md`)*

- [BRANCH — Branch Instructions](#branch) — 3 issues (3 resolved)
- [CALL — Call Instructions](#call) — 10 issues (10 resolved)
- [TRYCATCH — Try/Catch Instructions](#trycatch) — 1 issue (1 resolved)
- [COERCE — Coerce and Conversions Instructions](#coerce) — 2 issues (2 resolved)
- [COMPARE — Comparison Instructions](#compare) — 4 issues (3 resolved)
- [CONTEXT — Context Management](#context) — 2 issues (2 resolved)
- [CREATE — Array, Map, Structure Creation](#create) — 3 issues (3 resolved)
- [DEFER — Defer Management](#defer) — 2 issues (1 resolved)
- [EQUAL — Equality Testing](#equal) — 3 issues (3 resolved)
- [LOAD — Load Instructions](#load) — 3 issues (2 resolved)
- [FLOW — FLOW (Bytecode) — Flow Control Instructions](#area-flow-bytecode) — 3 issues (3 resolved)
- [MATH — Math Operations](#math) — 11 issues (11 resolved)
- [MEMBERS — Member Access](#members) — 7 issues (7 resolved)
- [OPTIMIZER — Bytecode Optimizer](#optimizer) — 9 issues (9 resolved)
- [RANGE — Range Loops](#range) — 5 issues (5 resolved)
- [STACK — Stack Management](#stack) — 3 issues (3 resolved)
- [STORE — Store Instructions](#store) — 4 issues (4 resolved)
- [STRUCT — Struct, Map, Array, Channel Indexing](#structs) — 3 issues (3 resolved)
- [PACKAGES — Package Instructions](#packages) — 2 issues (2 resolved)
- [PRINT — Print Instructions](#print) — 2 issues (2 resolved)
- [RETURN — Return Instruction](#return) — 2 issues (2 resolved)
- [TYPES — Type Instructions](#area-types-bytecode) — 3 issues (3 resolved)

### Debugger Package Issues

*(originally `DEBUGGER_ISSUES.md`)*

- [DEBUGGER-BREAKS — Breakpoint Handling](#area-debugger-breaks) — 3 issues (3 resolved)
- [DEBUGGER-COMMANDS — Debugger Commands](#area-debugger-commands) — 1 issue (1 resolved)
- [DEBUGGER-SHOW — Source Display](#area-debugger-show) — 2 issues (1 resolved)

### Security Issues

*(originally `SECURITY_ISSUES.md`)*

- [LOGIN — Logins and Passwords](#area-login) — 13 issues (13 resolved)
- [WEBAUTH — Webauthn and Passkeys](#area-webauth) — 8 issues (8 resolved)
- [HTTP — HTTP Server](#area-http) — 7 issues (7 resolved)
- [TABLES — Tables Endpoint](#area-tables) — 9 issues (9 resolved)
- [ASSET — Asset Handler](#area-asset) — 5 issues (5 resolved)
- [PROFILE — Profile Encryption](#area-profile) — 2 issues (2 resolved)
- [CODE — Dashboard Code Execution](#area-code) — 7 issues (5 resolved)
- [OAUTH — OAuth2 Authorization Server and Resource Server](#area-oauth) — 18 issues (18 resolved)

---

<a id="issue-index"></a>

## Issue Index

Every issue in this document, sorted alphabetically by identifier, for direct lookup.

| ID | Area | Summary | Status |
| :--- | :--- | :--- | :---: |
| [ASSET-H1](#ASSET-H1) | ASSET | DoS via open-ended Range header | ✓ |
| [ASSET-L1](#ASSET-L1) | ASSET | Double os.ReadFile call in readAssetFile | ✓ |
| [ASSET-L2](#ASSET-L2) | ASSET | Path normalization uses fragile string removal instead of filepath.Clean | ✓ |
| [ASSET-M1](#ASSET-M1) | ASSET | Wrong Content-Range header for open-ended ranges | ✓ |
| [ASSET-M2](#ASSET-M2) | ASSET | Error response discloses server filesystem paths | ✓ |
| [BRANCH-1](#BRANCH-1) | BRANCH | Stack mutated before address validation in conditional branches | ✓ |
| [BRANCH-2](#BRANCH-2) | BRANCH | nil operand silently accepted as address 0 | ✓ |
| [BRANCH-3](#BRANCH-3) | BRANCH | Misleading error context for non-integer operands | ✓ |
| [BUG-01](#BUG-01) | BUG | `for v := range ch` over a channel yielded loop indices instead of the channel's values. | ✓ |
| [BUG-02](#BUG-02) | BUG | Anonymous goroutine closures (`go func() {}()`) could not read variables from the enclosing scope. | ✓ |
| [BUG-03](#BUG-03) | BUG | Type assertions `v.(T)` always succeeded regardless of the value's actual type. | ✓ |
| [BUG-04](#BUG-04) | BUG | `recover()` in a deferred function of a value-returning function caused the caller to fail with a return-value-count error. | ✓ |
| [BUG-05](#BUG-05) | BUG | Calling a function value stored in an `any` variable failed with "invalid function invocation". | ✓ |
| [BUG-06](#BUG-06) | BUG | `++`/`--` were not permitted on struct fields or array elements, only simple variable names. | ✓ |
| [BUG-07](#BUG-07) | BUG | The two-value channel receive form `v, ok := <-ch` was rejected with "incorrect number of return values". | ✓ |
| [BUG-08](#BUG-08) | BUG | `delete(struct, key)` failed on dynamic structs despite being documented behavior. | ✓ |
| [BUG-09](#BUG-09) | BUG | An import alias (`import alias "pkg"`) was not recognized at its use site. | ✓ |
| [BUG-10](#BUG-10) | BUG | The single-argument form of `json.Unmarshal(b)` was rejected with an argument-count error. | ✓ |
| [BUG-11](#BUG-11) | BUG | `fmt.Printf()`'s two-value return form `n, err := fmt.Printf(...)` failed. | ✓ |
| [BUG-12](#BUG-12) | BUG | Writing to a nil map succeeded silently instead of raising an error like Go does. | ✓ |
| [BUG-13](#BUG-13) | BUG | `typeof()` results were incompatible with `switch` case matching due to an inconsistent type-to-string comparison. | ✓ |
| [BUG-14](#BUG-14) | BUG | Typed array element types were not enforced when assigning to an element in dynamic mode. | ✓ |
| [BUG-15](#BUG-15) | BUG | `append()` to a typed array silently accepted elements of the wrong type. | ✓ |
| [BUG-16](#BUG-16) | BUG | `defer namedFunc(arg)` evaluates its arguments lazily instead of at defer time (cross-referenced with FLOW-M4). | ✓ |
| [BUG-17](#BUG-17) | BUG | `var name = value` (type-inferred initializer, without an explicit type) was not supported. | ✓ |
| [BUG-18](#BUG-18) | BUG | LANGUAGE.md documented a `type()` function, but the actual builtin is named `typeof()`. | ✓ |
| [BUG-19](#BUG-19) | BUG | `for v := range someString` yielded single-character strings instead of `int32` Unicode code points. | ✓ |
| [BUG-20](#BUG-20) | BUG | `iota` is not supported inside `const` blocks. | ✓ |
| [BUG-21](#BUG-21) | BUG | The `@compile` test directive cannot pass values computed inside the compiled block/program back to the enclosing test. | ✓ |
| [BUG-22](#BUG-22) | BUG | `make(map[K]V)` failed with an "incorrect function argument count" error. | ✓ |
| [BUG-23](#BUG-23) | BUG | `var` declarations of struct types shared a single compile-time struct instance across function calls, causing state to leak between calls. | ✓ |
| [BUG-24](#BUG-24) | BUG | Multi-target assignment lists rejected indexed/member lvalues such as `m[k], arr[i] = ...`. | |
| [BUILTIN-APPEND-1](#BUILTIN-APPEND-1) | BUILTIN-APPEND | Append skipped type inference when the first argument was a raw []any slice, always returning []interface{}. | ✓ |
| [BUILTIN-CAST-1](#BUILTIN-CAST-1) | BUILTIN-CAST | castToStringValue used a byte-length check, so multi-byte Unicode character literals failed to cast. | ✓ |
| [BUILTIN-CAST-2](#BUILTIN-CAST-2) | BUILTIN-CAST | Cast incorrectly returned ErrInvalidType when data.Coerce succeeded but produced a valid nil result. | ✓ |
| [BUILTIN-COPY-1](#BUILTIN-COPY-1) | BUILTIN-COPY | DeepCopy for *data.Array wrote deep-copied elements back into the source array instead of the result, leaving the result empty and mutating the source. | ✓ |
| [BUILTIN-COPY-2](#BUILTIN-COPY-2) | BUILTIN-COPY | DeepCopy for *data.Struct used a shallow Copy(), so nested pointer fields still shared storage with the original. | ✓ |
| [BUILTIN-DELETE-1](#BUILTIN-DELETE-1) | BUILTIN-DELETE | Delete's error path used an unnamed magic literal 1 to denote the first argument position. | ✓ |
| [BUILTIN-FUNCTIONS-1](#BUILTIN-FUNCTIONS-1) | BUILTIN-FUNCTIONS | AddFunction and related helpers accessed the package-level FunctionDictionary map without locking, causing data races under concurrent use. | ✓ |
| [BUILTIN-FUNCTIONS-2](#BUILTIN-FUNCTIONS-2) | BUILTIN-FUNCTIONS | The "make" function registration declared its return type as int even though Make never returns an int. | ✓ |
| [BUILTIN-INDEX-1](#BUILTIN-INDEX-1) | BUILTIN-INDEX | Index returned bool for *data.Map lookups but int for all other argument types, contradicting its declared return type. | ✓ |
| [BUILTIN-LENGTH-1](#BUILTIN-LENGTH-1) | BUILTIN-LENGTH | Length returned math.MaxInt32 for any open channel instead of the actual buffered item count. | ✓ |
| [BUILTIN-MAKE-1](#BUILTIN-MAKE-1) | BUILTIN-MAKE | Make did not validate that size is non-negative, causing an unrecoverable Go runtime panic instead of a catchable error. | ✓ |
| [BUILTIN-MAKE-2](#BUILTIN-MAKE-2) | BUILTIN-MAKE | Unrecognized element types (e.g. int64, int32, float32) silently produced a nil-filled array with no error. | ✓ |
| [BUILTIN-NEW-1](#BUILTIN-NEW-1) | BUILTIN-NEW | newReflectKind(reflect.Int64) returned untyped 0, which Go infers as int rather than int64. | ✓ |
| [BUILTIN-NEW-2](#BUILTIN-NEW-2) | BUILTIN-NEW | NewInstanceOf returned the same channel instance instead of creating a new, independent channel with the same capacity. | ✓ |
| [BUILTIN-TYPES-1](#BUILTIN-TYPES-1) | BUILTIN-TYPES | typeOf returned a bare string "<builtin>" for builtin functions instead of a *data.Type, breaking the uniform return contract. | ✓ |
| [CALL-1](#CALL-1) | CALL | Argument count mismatch silently ignored for non-variadic functions with default ArgCount | ✓ |
| [CALL-10](#CALL-10) | CALL | `synthesizeDefinition` sets `MinArgCount = -1` for zero-parameter variadic functions | ✓ |
| [CALL-2](#CALL-2) | CALL | First extra variadic argument bypasses strict type checking | ✓ |
| [CALL-3](#CALL-3) | CALL | Nil pointer dereference in callRuntimeFunction when savedDefinition is nil and context is sandboxed | ✓ |
| [CALL-4](#CALL-4) | CALL | `parentTable` nil guard is dead code for non-literal named functions | ✓ |
| [CALL-5](#CALL-5) | CALL | `getPackageSymbols` passes the `this` struct instead of `this.value` to `GetPackageSymbolTable` | ✓ |
| [CALL-6](#CALL-6) | CALL | `SetBreakOnReturn` reads the wrong stack slot (off-by-one) | ✓ |
| [CALL-7](#CALL-7) | CALL | `callTypeCast` panics when Path-A struct types receive empty argument list | ✓ |
| [CALL-8](#CALL-8) | CALL | `makeNativeArrayArgument` missing `Int64Kind` and `Float32Kind` for `*data.Array` conversion | ✓ |
| [CALL-9](#CALL-9) | CALL | `CallWithReceiver` panics when method name is not found on receiver | ✓ |
| [CODE-H1](#CODE-H1) | CODE | Exec sandbox bypass when exec is globally permitted | ✓ |
| [CODE-L1](#CODE-L1) | CODE | Full user-submitted code body written to REST log | ✓ |
| [CODE-M1](#CODE-M1) | CODE | No request body size limit on POST /admin/run | ✓ |
| [CODE-M2](#CODE-M2) | CODE | Global trace logger state mutated per-request without synchronization | ✓ |
| [CODE-M3](#CODE-M3) | CODE | Client-supplied session UUID not validated or bound to the authenticated user | ✓ |
| [CODE-M4](#CODE-M4) | CODE | Sandbox I/O path confinement can be bypassed via symlinks | |
| [CODE-M5](#CODE-M5) | CODE | Language extensions enabled in sandboxed symbol table | |
| [COERCE-1](#COERCE-1) | COERCE | `NeedsCoerce` returns the wrong answer when the `Push` operand does not match the target type | ✓ |
| [COERCE-2](#COERCE-2) | COERCE | `data.UInt` accessor panics with a type assertion failure | ✓ |
| [COMPARE-1](#COMPARE-1) | COMPARE | `notEqualByteCode` and `greaterThanOrEqualByteCode` had no tests | ✓ |
| [COMPARE-2](#COMPARE-2) | COMPARE | `notEqualByteCode` uses value types instead of pointer types for composite cases | |
| [COMPARE-3](#COMPARE-3) | COMPARE | `int8` missing from signed-integer case in four ordering functions | ✓ |
| [COMPARE-4](#COMPARE-4) | COMPARE | Four comparison operators returned raw errors without `c.runtimeError` decoration | ✓ |
| [CONTEXT-1](#CONTEXT-1) | CONTEXT | `GetModuleName` panics with a nil pointer dereference when `bc` is nil | ✓ |
| [CONTEXT-2](#CONTEXT-2) | CONTEXT | `SetDebug` unconditionally sets `singleStep = true` regardless of argument | ✓ |
| [CREATE-1](#CREATE-1) | CREATE | `makeArrayByteCode` called `result.Set` twice per element | ✓ |
| [CREATE-2](#CREATE-2) | CREATE | `addMissingFields` inverted error check skipped coerced-value write-back | ✓ |
| [CREATE-3](#CREATE-3) | CREATE | `makeArrayByteCode` element-pop loop swallowed stack underflow silently | ✓ |
| [DEBUGGER-BREAKS-1](#DEBUGGER-BREAKS-1) | DEBUGGER-BREAKS | Range-loop copy semantics silently discarded hit counter updates in evaluationBreakpoint. | ✓ |
| [DEBUGGER-BREAKS-2](#DEBUGGER-BREAKS-2) | DEBUGGER-BREAKS | The load case compared bp.Kind to a hard-coded magic number instead of the BreakValue constant. | ✓ |
| [DEBUGGER-BREAKS-3](#DEBUGGER-BREAKS-3) | DEBUGGER-BREAKS | clearBreakWhen/clearBreakAtLine kept iterating a shifted slice after removing a match instead of returning immediately. | ✓ |
| [DEBUGGER-COMMANDS-1](#DEBUGGER-COMMANDS-1) | DEBUGGER-COMMANDS | The print case used full-string errors.Equal instead of key-based errors.Equals, discarding output when ErrStop carried context. | ✓ |
| [DEBUGGER-SHOW-1](#DEBUGGER-SHOW-1) | DEBUGGER-SHOW | showSource mutated its err parameter by value, so invalid range arguments were silently swallowed by the caller. | ✓ |
| [DEBUGGER-SHOW-2](#DEBUGGER-SHOW-2) | DEBUGGER-SHOW | Indentation logic counts braces/parens in raw source text, so characters inside string literals miscount indentation. | |
| [DEFER-1](#DEFER-1) | DEFER | `deferByteCode` receiver slice captures wrong elements when new count ≠ deferThisSize | ✓ |
| [DEFER-2](#DEFER-2) | DEFER | `deferByteCode` skips receiver capture when `deferThisSize == 0` | |
| [EQUAL-1](#EQUAL-1) | EQUAL | `equalTypes` returns an undecorated error (no module or line info) | ✓ |
| [EQUAL-2](#EQUAL-2) | EQUAL | `getComparisonTerms` returns raw coerce error (no location info) | ✓ |
| [EQUAL-3](#EQUAL-3) | EQUAL | `case nil:` branch in `equalByteCode`'s switch is dead code | ✓ |
| [FLOW-1](#FLOW-1) | FLOW | `Test_branchFalseByteCode` called `branchTrueByteCode` for its invalid-address sub-case | ✓ |
| [FLOW-2](#FLOW-2) | FLOW | `moduleByteCode` and `atLineByteCode` access `array[1]` without a bounds check | ✓ |
| [FLOW-3](#FLOW-3) | FLOW | Pre-helper tests in `flow_test.go` used raw `&Context{}` struct literals | ✓ |
| [FLOW-H1](#FLOW-H1) | FLOW | for range over an array marked it read-only, so writes through the index inside the loop body failed. | ✓ |
| [FLOW-L1](#FLOW-L1) | FLOW | Labeled break/continue were not supported. | ✓ |
| [FLOW-L2](#FLOW-L2) | FLOW | The switch init; expr semicolon-separated form was not supported. | ✓ |
| [FLOW-M1](#FLOW-M1) | FLOW | The for init clause only accepted :=, rejecting = assignment to a pre-declared variable. | ✓ |
| [FLOW-M2](#FLOW-M2) | FLOW | Multi-value case v1, v2: clauses were not supported and produced a spurious compile error. | ✓ |
| [FLOW-M4](#FLOW-M4) | FLOW | defer namedFunc(arg) evaluates its argument lazily at run time instead of eagerly at registration time. | ✓ |
| [FUNC-H1](#FUNC-H1) | FUNC | Calling a variadic function with zero variadic arguments failed with an argument-count error. | ✓ |
| [FUNC-H2](#FUNC-H2) | FUNC | Closures storing a loop variable became invalid (unknown identifier) once the loop exited. | ✓ |
| [FUNC-L1](#FUNC-L1) | FUNC | String multiplication (*) behaved asymmetrically, treating a left-hand string operand as repetition. | ✓ |
| [FUNC-L2](#FUNC-L2) | FUNC | Using an explicit return value expression in a function with named returns was a compile error. | ✓ |
| [FUNC-M1](#FUNC-M1) | FUNC | Named nested functions silently failed to capture the enclosing named function's scope. | ✓ |
| [FUNC-M2](#FUNC-M2) | FUNC | Calling a value-receiver method on a pointer variable raised an unsupported-type runtime error. | ✓ |
| [FUNC-M3](#FUNC-M3) | FUNC | Dynamic mode silently accepted wrong-type arguments at function call boundaries instead of coercing or erroring. | ✓ |
| [HTTP-H1](#HTTP-H1) | HTTP | No request body size limit | ✓ |
| [HTTP-H2](#HTTP-H2) | HTTP | No timeouts on the HTTP server - Slowloris vulnerability | ✓ |
| [HTTP-L1](#HTTP-L1) | HTTP | User-supplied URL path reflected verbatim in error messages | ✓ |
| [HTTP-L2](#HTTP-L2) | HTTP | Request body read after permission check already set a failure status | ✓ |
| [HTTP-M1](#HTTP-M1) | HTTP | Security response headers not set | ✓ |
| [HTTP-M2](#HTTP-M2) | HTTP | Server UUID and session counter disclosed on every response | ✓ |
| [HTTP-M3](#HTTP-M3) | HTTP | Redirect server (port 80) has no timeouts | ✓ |
| [JSON-H1](#JSON-H1) | JSON | json.Unmarshal into a struct stored a nested JSON object as a raw Go map instead of the declared Ego struct type. | ✓ |
| [JSON-H2](#JSON-H2) | JSON | json.Unmarshal type-mismatch errors surfaced as an uncatchable-by-default exception rather than the function's return value. | ✓ |
| [JSON-M1](#JSON-M1) | JSON | json.Marshal of a []byte value produced "null" instead of a base64-encoded JSON string. | ✓ |
| [JSON-M2](#JSON-M2) | JSON | json.Parse("[]", ".") returned an error while the equivalent json.Parse("{}", ".") succeeded. | ✓ |
| [JSON-M3](#JSON-M3) | JSON | json.Unmarshal into a struct returned an error for unknown JSON fields instead of ignoring them (as Go does). | ✓ |
| [LOAD-1](#LOAD-1) | LOAD | `explodeByteCode` returned raw error from `c.Pop()` without `c.runtimeError` decoration | ✓ |
| [LOAD-2](#LOAD-2) | LOAD | `explodeByteCode` doc comment incorrectly described the operand as "a struct" | ✓ |
| [LOAD-3](#LOAD-3) | LOAD | `Test_explodeByteCode` in `data_test.go` exits early on the first matched error, silently skipping later table cases | |
| [LOGIN-C1](#LOGIN-C1) | LOGIN | Passwords hashed with bare, unsalted SHA-256, enabling parallel rainbow-table cracking | ✓ |
| [LOGIN-C2](#LOGIN-C2) | LOGIN | No brute-force or rate-limiting protection on the login endpoint | ✓ |
| [LOGIN-H1](#LOGIN-H1) | LOGIN | Timing attack possible via non-constant-time password comparison | ✓ |
| [LOGIN-H2](#LOGIN-H2) | LOGIN | Weak (MD5-based) key derivation used for AES encryption of tokens and DSN passwords | ✓ |
| [LOGIN-H3](#LOGIN-H3) | LOGIN | Token signing key potentially stored in plaintext configuration | ✓ |
| [LOGIN-H4](#LOGIN-H4) | LOGIN | Login credentials forwarded on HTTP 301 redirect to an arbitrary host | ✓ |
| [LOGIN-L1](#LOGIN-L1) | LOGIN | Password supplied via environment variable is exposed to other processes | ✓ |
| [LOGIN-L2](#LOGIN-L2) | LOGIN | InsecureSkipVerify available without a prominent warning | ✓ |
| [LOGIN-L3](#LOGIN-L3) | LOGIN | Returned token expiration is recalculated independently of the token contents | ✓ |
| [LOGIN-M1](#LOGIN-M1) | LOGIN | HTTP downgrade fallback when the HTTPS connection attempt fails | ✓ |
| [LOGIN-M2](#LOGIN-M2) | LOGIN | Password whitespace silently stripped before hashing | ✓ |
| [LOGIN-M3](#LOGIN-M3) | LOGIN | Token cache bypasses expiry and revocation checks for up to 60 seconds | ✓ |
| [LOGIN-M4](#LOGIN-M4) | LOGIN | Quoted-password legacy format allows plaintext password storage | ✓ |
| [MATH-1](#MATH-1) | MATH | `exponentByteCode` returns 0 for signed integer `x^0` (should return 1) | ✓ |
| [MATH-10](#MATH-10) | MATH | `addByteCode` function comment incorrectly says "OR" for boolean operands | ✓ |
| [MATH-11](#MATH-11) | MATH | `notByteCode` and `negateByteCode` return raw (undecorated) errors from `c.Pop()` | ✓ |
| [MATH-2](#MATH-2) | MATH | `multiplyByteCode` `case int16:` asserts `v1.(int8)` when v1 is `int16` — panics | ✓ |
| [MATH-3](#MATH-3) | MATH | `multiplyByteCode` `case uint16:` asserts `v1.(int8)` when v1 is `uint16` — panics | ✓ |
| [MATH-4](#MATH-4) | MATH | `subtractByteCode` `case int8:` asserts `v1.(int16)` when v1 is `int8` — panics | ✓ |
| [MATH-5](#MATH-5) | MATH | `divideByteCode` `case int16:` asserts `v1.(int8)` when v1 is `int16` — panics | ✓ |
| [MATH-6](#MATH-6) | MATH | `divideByteCode` `case uint16:` asserts `v1.(int8)` when v1 is `uint16` — panics | ✓ |
| [MATH-7](#MATH-7) | MATH | `moduloByteCode` `case int16:` asserts `v1.(int8)` when v1 is `int16` — panics | ✓ |
| [MATH-8](#MATH-8) | MATH | `moduloByteCode` `case uint16:` asserts `v1.(int8)` when v1 is `uint16` — panics | ✓ |
| [MATH-9](#MATH-9) | MATH | `notByteCode` multi-type case returns wrong result for zero values of non-`int` integer types | ✓ |
| [MEMBERS-1](#MEMBERS-1) | MEMBERS | `memberByteCode` returns raw errors from `c.Pop()` without `c.runtimeError` decoration | ✓ |
| [MEMBERS-2](#MEMBERS-2) | MEMBERS | `getMemberValue` returns raw `ErrFunctionReturnedVoid` when stack marker detected | ✓ |
| [MEMBERS-3](#MEMBERS-3) | MEMBERS | `getStructMemberValue` returns raw errors without `c.runtimeError` decoration | ✓ |
| [MEMBERS-4](#MEMBERS-4) | MEMBERS | `memberByteCode` doc comment says "second a map" but the function handles many types | ✓ |
| [MEMBERS-5](#MEMBERS-5) | MEMBERS | `getPackageMemberValue` signature includes dead parameters `v any` and `found bool` | ✓ |
| [MEMBERS-6](#MEMBERS-6) | MEMBERS | `getMemberValue` ignores the member name when the object is `*data.Type` | ✓ |
| [MEMBERS-7](#MEMBERS-7) | MEMBERS | `getMemberValue` silently returns `(nil, nil)` for a nil `*data.Type` behind `*any` | ✓ |
| [OAUTH-H1](#OAUTH-H1) | OAUTH | No rate limiting on the AS login form endpoint | ✓ |
| [OAUTH-H2](#OAUTH-H2) | OAUTH | Revoked JWT tokens bypass the RS validation cache | ✓ |
| [OAUTH-H3](#OAUTH-H3) | OAUTH | PKCE not required for public clients in the AS authorization flow | ✓ |
| [OAUTH-H4](#OAUTH-H4) | OAUTH | CSRF token not regenerated when login form is re-rendered on failure | ✓ |
| [OAUTH-H5](#OAUTH-H5) | OAUTH | Unvalidated redirect query parameter enables open redirect in RS authorize handler | ✓ |
| [OAUTH-L1](#OAUTH-L1) | OAUTH | CSRF cookie missing Secure flag on AS authorize handler | ✓ |
| [OAUTH-L2](#OAUTH-L2) | OAUTH | AS token endpoint router registration uses JSON content type | ✓ |
| [OAUTH-L3](#OAUTH-L3) | OAUTH | EGO_OAUTH_CLIENT_SECRET environment variable not cleared after reading | ✓ |
| [OAUTH-L4](#OAUTH-L4) | OAUTH | Internal error details from token exchange and JWT validation returned to browser clients | ✓ |
| [OAUTH-L5](#OAUTH-L5) | OAUTH | Custom ego.server.oauth.user.claim values silently fall back to sub | ✓ |
| [OAUTH-M1](#OAUTH-M1) | OAUTH | Audience validation skipped by default | ✓ |
| [OAUTH-M2](#OAUTH-M2) | OAUTH | No timeout on outbound IdP HTTP calls | ✓ |
| [OAUTH-M3](#OAUTH-M3) | OAUTH | Revocation endpoint ignores HTTP Basic Auth for client authentication | ✓ |
| [OAUTH-M4](#OAUTH-M4) | OAUTH | OIDC discovery and JWKS responses read without a size limit | ✓ |
| [OAUTH-M5](#OAUTH-M5) | OAUTH | RS PKCE state store has no maximum size | ✓ |
| [OAUTH-M6](#OAUTH-M6) | OAUTH | Token exchange response body read without size limit | ✓ |
| [OAUTH-M7](#OAUTH-M7) | OAUTH | Every JWT with an unknown kid triggers an unconditional JWKS refresh | ✓ |
| [OAUTH-M8](#OAUTH-M8) | OAUTH | Custom permission claim names silently unsupported; all JWT holders granted minimum ego.logon | ✓ |
| [OPTIMIZER-1](#OPTIMIZER-1) | OPTIMIZER | Branch-target scan is O(n²): pre-build a target set instead | ✓ |
| [OPTIMIZER-2](#OPTIMIZER-2) | OPTIMIZER | `reflect.DeepEqual` for operand comparison is unnecessarily expensive | ✓ |
| [OPTIMIZER-3](#OPTIMIZER-3) | OPTIMIZER | No opcode-indexed dispatch: all rules tried at every position | ✓ |
| [OPTIMIZER-4](#OPTIMIZER-4) | OPTIMIZER | Backtracking by `maxPatternSize` instead of the matched pattern size | ✓ |
| [OPTIMIZER-5](#OPTIMIZER-5) | OPTIMIZER | `Patch` corrupts the instruction array when the replacement is longer than the deleted region | ✓ |
| [OPTIMIZER-6](#OPTIMIZER-6) | OPTIMIZER | `continue` after `found=false` in placeholder mismatch should be `break` | ✓ |
| [OPTIMIZER-7](#OPTIMIZER-7) | OPTIMIZER | `executeFragment` creates full interpreter context for trivial constant folding | ✓ |
| [OPTIMIZER-8](#OPTIMIZER-8) | OPTIMIZER | `data.Int` failure in branch-check scan aborts the entire optimization pass | ✓ |
| [OPTIMIZER-9](#OPTIMIZER-9) | OPTIMIZER | Dead `else if` condition in placeholder consistency check | ✓ |
| [PACKAGES-1](#PACKAGES-1) | PACKAGES | `inPackageByteCode` panics when the symbol table holds a non-package value under the package name | ✓ |
| [PACKAGES-2](#PACKAGES-2) | PACKAGES | `makePackageItemList` panics on nil values in the package dictionary or symbol table | ✓ |
| [PRINT-1](#PRINT-1) | PRINT | Unchecked type assertion in `formatValueForPrinting` panics on nil struct-array elements | ✓ |
| [PRINT-2](#PRINT-2) | PRINT | `case *data.Function:` in `formatValueForPrinting` is unreachable dead code | ✓ |
| [PROFILE-H1](#PROFILE-H1) | PROFILE | MD5 used to derive the AES encryption key | ✓ |
| [PROFILE-M1](#PROFILE-M1) | PROFILE | Legacy ciphertext not re-encrypted on successful read | ✓ |
| [RANGE-1](#RANGE-1) | RANGE | `rangeNextInteger` unconditionally calls `c.symbols.Set` without guarding empty or discarded variable names | ✓ |
| [RANGE-2](#RANGE-2) | RANGE | `rangeNextByteCode` default case does not pop the range stack | ✓ |
| [RANGE-3](#RANGE-3) | RANGE | Map remains readonly if a for-range loop exits before exhaustion | ✓ |
| [RANGE-4](#RANGE-4) | RANGE | `rangeNextByteCode` returns a raw error for the `[]any` case without `c.runtimeError()` decoration | ✓ |
| [RANGE-5](#RANGE-5) | RANGE | `rangeInitByteCode` appends a stale entry to `rangeStack` even when the type-switch default sets an error | ✓ |
| [RETURN-1](#RETURN-1) | RETURN | `isStackMarker(c.Result)` uses the bound method instead of the field — guard never fires | ✓ |
| [RETURN-2](#RETURN-2) | RETURN | `err` from `c.Pop()` in the bool branch is silently overwritten | ✓ |
| [SCRIPT-H1](#SCRIPT-H1) | SCRIPT | The readrows opcode capped results at one row due to a copy-pasted ?limit=1 fragment. | ✓ |
| [SCRIPT-L1](#SCRIPT-L1) | SCRIPT | An empty transaction body returned a plain-text response with no Content-Type header. | ✓ |
| [SCRIPT-M1](#SCRIPT-M1) | SCRIPT | _rows_ was always 0 in user error-condition expressions after an insert opcode. | ✓ |
| [SCRIPT-M2](#SCRIPT-M2) | SCRIPT | The drop opcode (and sql ... DROP TABLE) did not flush the schema cache, leaving stale column metadata. | ✓ |
| [STACK-1](#STACK-1) | STACK | `copyByteCode` pushes the integer literal `2` instead of the deep copy | ✓ |
| [STACK-2](#STACK-2) | STACK | `readStackByteCode` guard uses `>` instead of `>=`, causing panics on boundary indices | ✓ |
| [STACK-3](#STACK-3) | STACK | `dropByteCode` silently swallows stack underflow when dropping more items than exist | ✓ |
| [STORE-1](#STORE-1) | STORE | Misleading comment in `storeByteCode` for the readonly-prefix branch | ✓ |
| [STORE-2](#STORE-2) | STORE | `defs.DiscardedVariable` used where `defs.ReadonlyVariablePrefix` is intended | ✓ |
| [STORE-3](#STORE-3) | STORE | Scalar pointer helpers check `d.(string)` instead of target type in strict/relaxed mode | ✓ |
| [STORE-4](#STORE-4) | STORE | `storeChanByteCode` passes nil (`x`) as error context instead of the variable name | ✓ |
| [STRUCT-1](#STRUCT-1) | STRUCT | Dead code check in `storeInPackage` is unreachable | ✓ |
| [STRUCT-2](#STRUCT-2) | STRUCT | `storeIndexByteCode` mutates a struct field before checking package visibility | ✓ |
| [STRUCT-3](#STRUCT-3) | STRUCT | `flattenByteCode` produces incorrect `argCountDelta` for empty arrays | ✓ |
| [TABLES-C1](#TABLES-C1) | TABLES | Inverted guard in CommitHandler panics on every valid commit | ✓ |
| [TABLES-H1](#TABLES-H1) | TABLES | SQL injection via raw username in table-list query | ✓ |
| [TABLES-H2](#TABLES-H2) | TABLES | Inverted authorization filter in ListTables leaks table names | ✓ |
| [TABLES-L1](#TABLES-L1) | TABLES | Raw database error messages returned to clients | ✓ |
| [TABLES-L2](#TABLES-L2) | TABLES | SQLite PRAGMA statements use unquoted table and index names | ✓ |
| [TABLES-M1](#TABLES-M1) | TABLES | SQL injection in DeleteTable via un-parameterized DROP TABLE | ✓ |
| [TABLES-M2](#TABLES-M2) | TABLES | Row ID concatenated un-parameterized into UPDATE WHERE clause | ✓ |
| [TABLES-M3](#TABLES-M3) | TABLES | Wrong permission constant in DeleteTable allows any user to delete tables | ✓ |
| [TABLES-M4](#TABLES-M4) | TABLES | No server-side cap on query result size | ✓ |
| [TRYCATCH-1](#TRYCATCH-1) | TRYCATCH | `willCatchByteCode` panics on negative integer operands | ✓ |
| [TYPE-H1](#TYPE-H1) | TYPE | json.Unmarshal could not reconstruct deeply nested types or typed elements inside []any arrays. | ✓ |
| [TYPES-1](#TYPES-1) | TYPES | Dead nil guard in `deRefByteCode` leaves `*c3` vulnerable to panic | ✓ |
| [TYPES-2](#TYPES-2) | TYPES | `reflect.TypeOf(v).String()` panics when `v` is nil in `relaxedConformanceCheck` | ✓ |
| [TYPES-3](#TYPES-3) | TYPES | Int-dispatch switch in `relaxedConformanceCheck` has unreachable cases | ✓ |
| [WEBAUTH-H1](#WEBAUTH-H1) | WEBAUTH | allow.passkeys setting not enforced in ceremony handlers | ✓ |
| [WEBAUTH-H2](#WEBAUTH-H2) | WEBAUTH | Authenticator clone warning not checked after login | ✓ |
| [WEBAUTH-L1](#WEBAUTH-L1) | WEBAUTH | Secure flag absent on challenge cookie | ✓ |
| [WEBAUTH-L2](#WEBAUTH-L2) | WEBAUTH | Cache item expiration only refreshed when CACHE logging is active | ✓ |
| [WEBAUTH-L3](#WEBAUTH-L3) | WEBAUTH | No user notification when passkeys are cleared by an administrator | ✓ |
| [WEBAUTH-M1](#WEBAUTH-M1) | WEBAUTH | No rate limiting on unauthenticated ceremony-begin endpoints | ✓ |
| [WEBAUTH-M2](#WEBAUTH-M2) | WEBAUTH | RPID derived from user-controlled Host header | ✓ |
| [WEBAUTH-M3](#WEBAUTH-M3) | WEBAUTH | storeChallenge mutates shared cache expiration on every call | ✓ |

---

---

<a id="area-bug"></a>

## BUG — General Language Bugs

This area records general Ego-language bugs discovered through systematic testing (as opposed to the documented behavioral differences tracked elsewhere). `FLOW-M4` (defer lazy argument evaluation, tracked in the Functional Issues area) is cross-referenced here as BUG-16 for completeness only.

### Severity Classification

| Level | Meaning |
| :---- | :------ |
| **CRITICAL** | Incorrect behavior with no workaround; likely data corruption or crashes |
| **HIGH** | Core language feature behaves incorrectly; program logic is wrong |
| **MEDIUM** | Spec or documented behavior violated; workaround usually exists |
| **LOW** | Minor inconsistency, missing feature, or documentation error |

| ID | Severity | Summary | Status |
| :--- | :------- | :------ | :----- |
| [BUG-01](#BUG-01) | HIGH | `for v := range ch` over a channel yielded loop indices instead of the channel's values. | ✓ |
| [BUG-02](#BUG-02) | HIGH | Anonymous goroutine closures (`go func() {}()`) could not read variables from the enclosing scope. | ✓ |
| [BUG-03](#BUG-03) | HIGH | Type assertions `v.(T)` always succeeded regardless of the value's actual type. | ✓ |
| [BUG-04](#BUG-04) | HIGH | `recover()` in a deferred function of a value-returning function caused the caller to fail with a return-value-count error. | ✓ |
| [BUG-05](#BUG-05) | HIGH | Calling a function value stored in an `any` variable failed with "invalid function invocation". | ✓ |
| [BUG-06](#BUG-06) | HIGH | `++`/`--` were not permitted on struct fields or array elements, only simple variable names. | ✓ |
| [BUG-07](#BUG-07) | MEDIUM | The two-value channel receive form `v, ok := <-ch` was rejected with "incorrect number of return values". | ✓ |
| [BUG-08](#BUG-08) | MEDIUM | `delete(struct, key)` failed on dynamic structs despite being documented behavior. | ✓ |
| [BUG-09](#BUG-09) | MEDIUM | An import alias (`import alias "pkg"`) was not recognized at its use site. | ✓ |
| [BUG-10](#BUG-10) | MEDIUM | The single-argument form of `json.Unmarshal(b)` was rejected with an argument-count error. | ✓ |
| [BUG-11](#BUG-11) | MEDIUM | `fmt.Printf()`'s two-value return form `n, err := fmt.Printf(...)` failed. | ✓ |
| [BUG-12](#BUG-12) | MEDIUM | Writing to a nil map succeeded silently instead of raising an error like Go does. | ✓ |
| [BUG-13](#BUG-13) | MEDIUM | `typeof()` results were incompatible with `switch` case matching due to an inconsistent type-to-string comparison. | ✓ |
| [BUG-14](#BUG-14) | MEDIUM | Typed array element types were not enforced when assigning to an element in dynamic mode. | ✓ |
| [BUG-15](#BUG-15) | MEDIUM | `append()` to a typed array silently accepted elements of the wrong type. | ✓ |
| [BUG-16](#BUG-16) | MEDIUM | `defer namedFunc(arg)` evaluates its arguments lazily instead of at defer time (cross-referenced with FLOW-M4). | ✓ |
| [BUG-17](#BUG-17) | LOW | `var name = value` (type-inferred initializer, without an explicit type) was not supported. | ✓ |
| [BUG-18](#BUG-18) | LOW | LANGUAGE.md documented a `type()` function, but the actual builtin is named `typeof()`. | ✓ |
| [BUG-19](#BUG-19) | LOW | `for v := range someString` yielded single-character strings instead of `int32` Unicode code points. | ✓ |
| [BUG-20](#BUG-20) | LOW | `iota` is not supported inside `const` blocks. | ✓ |
| [BUG-21](#BUG-21) | LOW | The `@compile` test directive cannot pass values computed inside the compiled block/program back to the enclosing test. | ✓ |
| [BUG-22](#BUG-22) | MEDIUM | `make(map[K]V)` failed with an "incorrect function argument count" error. | ✓ |
| [BUG-23](#BUG-23) | MEDIUM | `var` declarations of struct types shared a single compile-time struct instance across function calls, causing state to leak between calls. | ✓ |
| [BUG-24](#BUG-24) | MEDIUM | Multi-target assignment lists rejected indexed/member lvalues such as `m[k], arr[i] = ...`. | |

---

<a id="BUG-01"></a>

### BUG-01 — `for v := range ch` over channel yields indices, not values

**Severity:** HIGH

**Description:**  
When ranging over a channel with a single loop variable, the variable receives
successive integer indices (0, 1, 2, …) instead of the values stored in the channel.
The channel values are discarded. This makes the standard Go idiom of consuming a
channel with `for v := range ch` produce completely wrong results.

**Reproducer:**

```go
import "fmt"

func producer(ch chan) {
    ch <- 100
    ch <- 200
    ch <- 300
    close(ch)
}

func main() {
    ch := make(chan, 5)
    go producer(ch)

    for v := range ch {
        fmt.Println("received:", v)
    }
}
```

**Actual output:**

```text
received: 0
received: 1
received: 2
```

**Expected output:**

```text
received: 100
received: 200
received: 300
```

**Notes:**  
Direct receive (`v := <-ch`) returns the correct values. Only the `for range`
form is broken. The bug appears to be that the range loop treats the channel
like an array and emits the iteration count rather than the received value.

**Resolution (June 2026):**  
`bytecode/range.go` — `rangeNextChannel`: the function now distinguishes between
the single-variable and two-variable loop forms by checking whether `r.valueName`
is present:

- **Two-variable form** (`for i, v := range ch`): `r.indexName` receives the loop
  counter (`r.index`) and `r.valueName` receives the channel value (`datum`) —
  behavior unchanged.
- **Single-variable form** (`for v := range ch`): the compiler emits
  `RangeInit ["v", ""]`, placing the sole variable name in `r.indexName` and
  leaving `r.valueName` empty. The function now routes `datum` to `r.indexName`
  in this case. The loop counter is meaningless for channel receives and is
  no longer exposed.

Three new unit tests were added to `bytecode/range_test.go` (Section 8):

- `Test_rangeNextChannel_SingleVarReceivesValue` — the direct regression test;
  verifies that the single-variable form delivers channel values, not indices.
- `Test_rangeNextChannel_SingleVarDiscarded` — verifies the discard form
  (`for _ := range ch`) consumes values without writing to any symbol.
- `Test_rangeNextChannel_TwoVarCounterAndValue` — verifies the two-variable form
  still delivers the counter to `i` and the value to `v` after the fix.

Two new Ego language tests were added to `tests/flow/rangechannels.ego`:

- `"flow: single-variable channel range receives values, not indices"` — the
  end-to-end regression test.
- `"flow: two-variable channel range counter and value are correct"` — ensures
  the two-variable form continues to work.

---

<a id="BUG-02"></a>

### BUG-02 — `go func() {}()` closures cannot read outer-scope variables

**Severity:** HIGH

**Description:**  
An anonymous function literal launched as a goroutine (`go func() { ... }()`)
cannot access variables from the enclosing scope. Any reference to an outer
variable produces a runtime error `"unknown identifier: <name>"`. This breaks
the idiomatic Go pattern of launching goroutines as closures.

The bug occurs when the goroutine runs asynchronously in a separate OS thread.
When the goroutine happens to run on the same thread (because the main goroutine
blocks on an unbuffered channel receive), it can sometimes access the parent
scope — but this is unreliable and race-dependent.

**Reproducer:**

```go
import "fmt"

func main() {
    x := 42
    go func() {
        fmt.Println("x:", x)   // should print 42
    }()
    for i := 0; i < 1000000; i++ {}   // spin to let goroutine run
    fmt.Println("done")
}
```

**Actual output:**

```text
Error: at go func ()(line 6), unknown identifier: x
```

**Expected output:**

```text
x: 42
done
```

**Workaround:**  
Pass all captured variables as explicit arguments to the anonymous function:

```go
go func(val int) {
    fmt.Println("x:", val)
}(x)
```

**Notes:**  
This also affects `sync.WaitGroup` usage where the goroutine captures `wg`
from the outer scope — `wg.Wait()` hangs because the goroutines cannot call
`wg.Done()` through the captured reference.

**Resolution (June 2026):**

**Root cause:** `pushByteCode` in `bytecode/stack.go` unconditionally overwrote
`capturedScope` with `c.symbols` every time a literal `*ByteCode` was pushed.
The sequence for `go func() { ... }()` is:

1. The parent context executes `Push <literal>` — clones the raw compiled
   bytecode and stamps `c.symbols` (the parent's local scope, containing `x`)
   as `capturedScope`. This is correct.
2. `goByteCode` pops the clone as `fx`.  `fx.capturedScope` now points at the
   parent's local scope.
3. `GoRoutine` constructs a tiny call script `Push fx; Call N` that runs in a
   minimal goroutine context whose `c.symbols` is a child of the global root
   (knows nothing about `x`).
4. When `Push fx` executes in the goroutine, `pushByteCode` cloned `fx` (the
   clone inherits `capturedScope` from `fx`) then **overwrote**
   `capturedScope = c.symbols` — the goroutine's root-child scope.
5. The closure's `callBytecodeFunction` then created a function scope as a
   child of the goroutine's root-child scope instead of the parent's local
   scope, so `x` was never reachable.

The user hint was confirmed: anonymous functions are (and must be) compiled with
`PushScope` (no boundary flag) while named functions use
`PushScope BoundaryScope`. This was already correct in the compiler
(`compiler/function.go` lines 146–149). The bug was purely in `pushByteCode`.

**Fixes applied:**

`bytecode/stack.go` — `pushByteCode`: added a guard `if clone.capturedScope == nil` so
the current scope is only stamped onto a clone when none has been captured yet.
An already-captured scope (the goroutine re-push case) is left intact, preserving
the parent's local variables in the chain. The loop-closure case (FUNC-H2) is
unaffected: the original compiled literal always starts with `capturedScope == nil`,
so each loop iteration still captures its per-iteration scope.

`bytecode/goroutine.go` — `GoRoutine`: when `fx` is a literal closure with a non-nil
`capturedScope`, calls `capturedScope.Shared(true)` before the goroutine runs.
`Shared(true)` propagates up the entire ancestor chain, so every symbol table
the closure will traverse is protected by read/write locks. This prevents data
races when the parent thread and the goroutine access the same tables concurrently.

**Tests added:**

- `bytecode/stack_test.go` — two new Go unit tests:
  - `Test_pushByteCode_PreservesCapturedScope` — direct regression test simulating
    the parent → goroutine double-push sequence; verifies the second push does
    not overwrite the scope captured by the first.
  - `Test_pushByteCode_LoopIterationsCaptureDifferentScopes` — verifies the
    loop-closure case continues to produce a distinct per-iteration scope.

- `tests/flow/go_func_literal.ego` — four new Ego language tests:
  - `"flow: goroutine closure reads outer-scope variable"` — direct BUG-02
    regression test; a goroutine closure must be able to read `x` from the
    outer scope.
  - `"flow: goroutine closure captures multiple outer-scope variables"` — same
    scenario with three captured variables.
  - `"flow: goroutine closure with explicit arguments still works"` — verifies
    the original argument-passing form is unaffected.
  - `"flow: named-function goroutine unaffected by BUG-02 fix"` — verifies named
    function goroutines are not broken by the change.

---

<a id="BUG-03"></a>

### BUG-03 — Type assertions `v.(T)` always succeed regardless of actual type

**Severity:** HIGH

**Description:**  
A type assertion `v.(T)` on an `any`-typed variable always succeeds and returns
the underlying value regardless of whether the value's actual type matches `T`.
The type name `T` is ignored. The two-value form `val, ok := v.(T)` also always
returns `ok = true`.

In Go, `v.(string)` on a value containing `42` (int) panics. In Ego, it silently
returns `42` and reports `typeof` as `"string"`.

**Reproducer:**

```go
import "fmt"

func main() {
    var v any = 42

    s := v.(string)             // should panic or error
    fmt.Println("s:", s)        // prints: 42  (WRONG)
    fmt.Println("type:", typeof(s)) // prints: string  (WRONG - the int was not converted)

    s2, ok := v.(string)        // should return ("", false)
    fmt.Println("ok:", ok, "s2:", s2)  // prints: true 42  (WRONG)
}
```

**Actual output:**

```text
s: 42
type: string
ok: true s2: 42
```

**Expected output:**

```text
Error: interface conversion: interface{} is int, not string
```

or in a two-value assertion: `ok: false s2:`

**Notes:**  
Correct type assertion (`v.(int)`) does work when the type matches. The bug is
that incorrect assertions don't fail — the assertion target type is ignored and
the stored value is returned as-is.

**Resolution (June 2026):**

**Design decision:** Type assertions validate the actual stored type in **all**
type-strictness modes (strict, relaxed, and dynamic). A type assertion is not a
coercion — it asks "does this interface value actually hold a T?" rather than
"can this value be converted to T?" Cast functions (`int()`, `string()`,
`float64()`, etc.) already provide coercion. Making assertions succeed by
coercion eliminates the only runtime mechanism for asking the first question;
the `v, ok := x.(T)` form only has value if `ok` can be `false`.

**Root cause:** `unwrapByteCode` in `bytecode/types.go` had two separate code
paths based on `c.typeStrictness`:

- **Strict mode**: checked `actualType.IsType(newType)`; returned `(nil, false)`
  on mismatch. Correct.
- **Non-strict modes**: called `data.Coerce(value, newType.InstanceOf(...))`,
  unconditionally converting any value to any type and reporting success,
  ignoring the actual stored type entirely. Broken.

**Secondary bug:** the `"any"` alias for `interface{}` was not recognized. The
type lookup loop compared `td.Kind.Name()` (which returns `"interface{}"`)
against the string `"any"`, so `v.(any)` always returned `ErrInvalidType` instead
of succeeding. Fixed by normalizing `"any"` to `data.InterfaceTypeName`
(`"interface{}"`) before the loop.

**Fixes applied:**

`bytecode/types.go` — `unwrapByteCode`: the two-path `if/else` was replaced
with a unified check:

- If `newType.Kind() == data.InterfaceKind` (target is `any`): always succeeds,
  since every value satisfies the empty interface.
- If `!actualType.IsType(newType)`: push `(nil, false)` and return. Applies in
  all three type-strictness modes.
- If types match: push `(value, true)`.

`bytecode/types.go` — `unwrapByteCode` (type lookup): added normalization of
the string `"any"` to `data.InterfaceTypeName` before the `TypeDeclarations`
loop.

**Tests added:**

- `bytecode/types_test.go` — nine new Go unit tests (section 2b) covering the
  BUG-03 regression in all three modes, plus the `"any"` alias fix:
  - `Test_unwrapByteCode_BUG03_Relaxed_WrongType`
  - `Test_unwrapByteCode_BUG03_Dynamic_WrongType`
  - `Test_unwrapByteCode_BUG03_Relaxed_CorrectType`
  - `Test_unwrapByteCode_BUG03_Dynamic_CorrectType`
  - `Test_unwrapByteCode_BUG03_IntToString`
  - `Test_unwrapByteCode_BUG03_FloatToInt`
  - `Test_unwrapByteCode_BUG03_AnyAlias`
  - `Test_unwrapByteCode_BUG03_InterfaceAlias`
  - `Test_unwrapByteCode_BUG03_AllModesConsistent`

- `tests/types/type_assertions.ego` — twelve new Ego language tests covering
  correct-type and wrong-type assertions in both the single-value and two-value
  forms, all scalar types, the `any` alias, and the multi-assert type-guard
  pattern.

---

<a id="BUG-04"></a>

### BUG-04 — `recover()` in deferred function of a value-returning function causes caller error

**Severity:** HIGH

**Description:**  
When a function that has a return value panics and a deferred function calls
`recover()`, the panic is correctly caught. However, the caller then receives
the error `"function did not return the expected number of values"` instead of
getting the function's zero/named return value.

`recover()` works correctly for `void` (no return value) functions. The bug is
specific to functions that declare a return type.

**Reproducer:**

```go
@extensions true
import "fmt"

func panicIfZero(b int) int {
    defer func() {
        if r := recover(); r != nil {
            fmt.Println("recovered:", r)
        }
    }()
    if b == 0 {
        panic("zero!")
    }
    return b * 2
}

func main() {
    r1 := panicIfZero(5)
    fmt.Println("panicIfZero(5):", r1)   // OK: prints 10

    r2 := panicIfZero(0)
    fmt.Println("panicIfZero(0):", r2)   // ERROR
}
```

**Actual output:**

```text
panicIfZero(5): 10
recovered: zero!
Error: at main(line 19), function did not return the expected number of values
```

**Expected output:**

```text
panicIfZero(5): 10
recovered: zero!
panicIfZero(0): 0
```

**Notes:**  
The `recover()` call itself works (prints "recovered: zero!"). The issue is that
after recovery the function does not push a return value onto the result stack,
so the caller's multi-return unpack fails. Named return variables should be
returned at their current (zero) value after recovery.

**Resolution (June 2026):**

**Root cause:** `unwindPanic` (in `bytecode/panic.go`) popped the call frame
without pushing any return value onto the caller's stack. The caller's next
instruction (`CreateAndStore` for single-value, `StackCheck` for multi-value)
then consumed the stack marker that had been placed before the function call,
producing "function did not return the expected number of values".

**Design decisions:**

- **Unnamed returns** → push `nil` (unambiguously signals "no useful value was
  set before the panic"). This is consistent with the user's Q3 preference over
  zero-value synthesis.
- **Named returns** → push the variable's *current* symbol-table value, which
  the compiler initialized to zero at function entry; the deferred function that
  called `recover()` may have explicitly set it before returning. This matches
  Go's semantics: named return variables retain whatever value they held at the
  point of recovery.
- **Void functions** → unchanged (no values pushed).
- **All three type-strictness modes** → identical behavior.

**Fixes applied:**

`bytecode/bytecode.go`: Added `returnVarNames []string` field to `ByteCode`
with `SetReturnVarNames`/`GetReturnVarNames` accessors. This records the
source-code names of any named return variables so `unwindPanic` can look them
up in the panicking function's symbol table at recovery time without changing
`data.Declaration`.

`compiler/function.go` — `generateFunctionBytecode`: After `c.returnVariables`
is fully parsed, copies the names into a `[]string` and calls
`b.SetReturnVarNames(names)`.

`bytecode/panic.go` — `unwindPanic`, recovery block: after clearing the defer
stack and resetting `c.stackPointer = c.framePointer`, synthesizes return
values before calling `callFramePop`:

- Single return: sets `c.result` and `c.resultSet = true` — `callFramePop`
  pushes the result onto the caller's stack via the existing single-result path.
- Multiple returns: pushes `NewStackMarker(c.bc.name, N)` followed by N values
  in reverse declaration order — exactly matching what `compileReturn` emits in
  the normal (non-panic) path. The `StackMarker` is required because
  `stackCheckByteCode` scans the stack looking for any marker to confirm the
  expected number of return values are present.

**Tests added:**

`tests/flow/panic_recover.ego` — 12 new Ego language tests in five groups:

1. **Unnamed single return** — BUG-04 primary regression; caller gets `nil`.
2. **Named single return** — zero value when defer doesn't modify; set value
   when defer does; pre-panic value preserved when defer doesn't touch it.
3. **Nested scope** — named return reachable when panic occurs inside an `if`
   block; reachable when panic occurs inside a `for` loop.
4. **Multiple returns** — unnamed gives `(nil, nil)`; named deferred-set gives
   correct values in correct order; partial deferred set; pre-panic values
   preserved.
5. **Void functions** — unchanged behavior confirmed.

---

<a id="BUG-05"></a>

### BUG-05 — Calling a function stored in an `any` variable fails

**Severity:** HIGH

**Description:**  
The LANGUAGE.md documents passing a function as an `any` parameter and calling
it inside the receiving function. In practice this fails with
`"invalid function invocation"` — calling a variable typed as `any` that holds
a function value is not supported.

**Reproducer:**

```go
import "fmt"

func show(fn any, name string) {
    fn("The name is ", name)   // documented in LANGUAGE.md
}

func main() {
    p := fmt.Println
    show(p, "tom")   // Error: invalid function invocation
}
```

**Actual output:**

```text
Error: at show(line 4), invalid function invocation: {{false Println(...) ...}}
```

**Expected output:**

```text
The name is  tom
```

**Workaround:**  
There is no clean workaround within the `any` parameter type. Pass the function
as its concrete type if known, or restructure to avoid passing functions as `any`.

**Notes:**  
Calling a function stored in a concrete-typed variable (not `any`) works
correctly. The bug is specific to function values wrapped in the `any`/interface
type. This makes the documented pattern in LANGUAGE.md non-functional.

**Resolution:**

Added code to the Call bytecode handler to detect when the item being used as the
target of the call is wrapped as a data.Interface (the Ego version of an `any` value),
the value is unwrapped before proceeding to determine who the value is meant to be
used (i.e. the target can be bytecode, a built-in function, a type, etc.)

---

<a id="BUG-06"></a>

### BUG-06 — `++`/`--` not permitted on struct fields or array elements

**Severity:** HIGH

**Description:**  
The `++` and `--` operators only work on simple (unqualified) variable names.
Applying them to struct field access (`s.field++`) or array element access
(`a[i]++`) produces a compile-time error: `"invalid use of auto increment/decrement
operation"`. In Go, both forms are valid.

**Reproducer:**

```go
import "fmt"

type Foo struct { x int }

func main() {
    a := []int{1, 2, 3}
    a[0]++        // compile error

    f := Foo{x: 10}
    f.x++         // compile error
}
```

**Actual error:**

```text
Error: at line 7:4, invalid use of auto increment/decrement operation
```

**Expected behavior:**  
`a[0]++` increments `a[0]` from 1 to 2.  
`f.x++` increments `f.x` from 10 to 11.

**Workaround:**  
Use explicit assignment: `a[0] = a[0] + 1` and `f.x = f.x + 1`.

**Resolution (June 2026):**

**Root cause:** `compileAssignment` in `compiler/assignment.go` checked whether
the lvalue store bytecode had more than two instructions and, if so, immediately
returned `ErrInvalidAuto`. A simple variable like `x` produces exactly two
instructions (`Store "x"` + `DropToMarker`), but any qualified lvalue — an array
subscript `a[0]` or a struct field `s.field` — produces at least four instructions
(`Load base`, index expression, `StoreIndex`, `DropToMarker`), which always tripped
the guard.

The same pattern existed in the for-loop increment compiler (`compiler/for.go`)
where the increment clause of a `for init; cond; incr` loop assumed instruction 0
of the lvalue was always `Store "name"`, which is only true for simple variables.

**Design:**

A qualified lvalue's store bytecode has this structure:

```text
[0]       Load "base"     — load the container (array or struct variable)
[1..n-3]  <index exprs>   — push each intermediate index
[n-2]     StoreIndex      — write back (patched from LoadIndex by patchStore)
[n-1]     DropToMarker    — clean up the "let" stack marker
```

For `a[0]++` the load half can be reconstructed by copying instructions 0 through
`n-3` from the store lvalue and appending `LoadIndex` (with the same operand as
`StoreIndex`). This reads the current value of `a[0]` onto the stack. After applying
`Push 1` + `Add`/`Sub`, the full `storeLValue` is appended to write the result back
and clean up the marker.

**Fixes applied:**

`compiler/assignment.go` — `compileAssignment`: replaced the blanket
`if storeLValue.Mark() > 2 { return ErrInvalidAuto }` guard with a two-branch
path:

- **Simple variable** (`storeLValue.Mark() == 2` with `Store` as instruction 0):
  existing `Load / Push 1 / Add|Sub / Dup / Store` sequence, behavior unchanged.
- **Qualified lvalue** (second-to-last instruction is `StoreIndex`): emits the
  load-path instructions from `storeLValue`, a `LoadIndex` to read the current
  value, `Push 1` + `Add`/`Sub`, then appends the full `storeLValue` (which runs
  `StoreIndex` + `DropToMarker`) to write back and clean up. Any other lvalue
  shape (e.g. pointer dereference) falls through to `ErrInvalidAuto`.

`compiler/for.go` — for-loop increment clause: the same structural change was
applied so that `for a[0] := 0; a[0] < n; a[0]++` works in the increment position.

**Tests added:**

`compiler/assignment_test.go` — four new compile-time cases in
`TestCompiler_compileAssignment` (one per form: array `++`, array `--`, struct
`++`, struct `--`), verifying that compilation now succeeds where it previously
returned `ErrInvalidAuto`.

`compiler/run_test.go` — seven new runtime cases in `TestArbitraryCodeFragments`
that build arrays and dynamic structs, apply `++`/`--`, and verify the resulting
values.

`tests/flow/qualified_increment.ego` — ten Ego language tests covering:

- `"flow: array element increment (BUG-06)"` — basic `a[0]++`, checks value and
  sibling elements.
- `"flow: array element decrement (BUG-06)"` — `a[2]--`.
- `"flow: array element increment, variable index (BUG-06)"` — `a[i]++` where
  `i` is a variable.
- `"flow: array element decrement, variable index (BUG-06)"` — `a[idx]--`.
- `"flow: multiple array element increments (BUG-06)"` — three consecutive `a[0]++`
  must be cumulative.
- `"flow: struct field increment (BUG-06)"` — `s.x++` on a dynamic struct.
- `"flow: struct field decrement (BUG-06)"` — `s.count--`.
- `"flow: multiple struct field increments (BUG-06)"` — five consecutive `s.n++`.
- `"flow: struct other fields unaffected by increment (BUG-06)"` — increments one
  field and confirms the other is unchanged.
- `"flow: for loop with array element increment (BUG-06)"` — uses `a[0]++` as the
  increment clause of a `for` loop.

---

<a id="BUG-07"></a>

### BUG-07 — Two-value channel receive `v, ok := <-ch` not supported

**Severity:** MEDIUM

**Description:**  
Go's standard idiom for detecting a closed channel is the two-value receive
form: `v, ok := <-ch`. In Ego, this produces `"incorrect number of return values"`.
Only the single-value form `v := <-ch` is supported.

**Reproducer:**

```go
import "fmt"

func main() {
    ch := make(chan, 2)
    ch <- 10
    close(ch)

    v, ok := <-ch   // ERROR: incorrect number of return values
    fmt.Println(v, ok)
}
```

**Actual output:**

```text
Error: at main(line 8), incorrect number of return values
```

**Expected output:**

```text
10 true
```

**Notes:**  
A second receive from the closed-and-drained channel should give `(<nil>, false)`.
Without this form, there is no reliable way in Ego to detect when a channel has
been closed.

**Resolution (June 2026):**

**Root cause:** A multi-target assignment such as `v, ok := <-ch` compiles its
left-hand side via `assignmentTargetList`, which always begins the generated
store bytecode with `StackCheck 2` — a check that exactly two values are present
above a stack marker before unpacking them to `v` and `ok`. For an ordinary
multi-return call (`a, b := f()`) the callee pushes both values itself. For a
channel receive, however, the only thing on the stack before this point was the
single channel object pushed by evaluating `<-ch`'s operand — one value, not two
— so `StackCheck 2` always failed with `"incorrect number of return values"`.
The single-value form `v := <-ch` was unaffected because it goes through a
different code path (`StoreChan`) that doesn't use `StackCheck`.

**Design:** A new `ReceiveChannel` bytecode instruction performs the actual
channel receive and leaves the stack in the shape `StackCheck 2` expects:

```text
[0]  StackMarker("receive")
[1]  ok bool      — true if a value was received, false if the channel
                     is closed and drained
[2]  datum any    — the received value, or nil when ok is false
```

`compileAssignment` detects the specific combination of a channel-receive RHS
(`<-` was consumed) together with a multi-target LHS (`c.flags.multipleTargets`)
and emits `ReceiveChannel` instead of leaving the plain `Load "ch"` result on the
stack. The existing `storeLValue` bytecode (already built by
`assignmentTargetList` for any multi-target assignment) is unchanged and unpacks
the two pushed values to `v` and `ok` exactly as it would for a function's
multi-value return.

**Fixes applied:**

`bytecode/opcodes.go` — added the `ReceiveChannel` opcode: a new constant, an
entry in `opcodeNames`, and a dispatch-table registration pointing at
`receiveChannelByteCode`.

`bytecode/store.go` — added `receiveChannelByteCode`: pops the channel value
(erroring with `ErrInvalidChannel` if the popped value isn't a `*data.Channel`),
calls `Receive()`, and pushes `[StackMarker("receive"), ok, datum]`. A closed,
drained channel (`Receive()` returning `ErrChannelNotOpen`) is translated to
`ok = false, datum = nil` rather than propagated as an error, matching Go's
"second value is the success flag" convention.

`compiler/assignment.go` — `compileAssignment`: captures whether the RHS began
with `<-` (`isChannelReceive`) before parsing the expression. When
`isChannelReceive && c.flags.multipleTargets` is true, emits `ReceiveChannel`
in place of the plain expression result and appends `storeLValue` as usual. The
single-value channel-receive path (`v := <-ch`) is untouched since
`multipleTargets` is false in that case.

**Tests added:**

`bytecode/store_test.go` — five new Go unit tests (Section 7):
`Test_receiveChannelByteCode_SuccessfulReceive`,
`Test_receiveChannelByteCode_ClosedChannel`,
`Test_receiveChannelByteCode_NonChannelValue`,
`Test_receiveChannelByteCode_StackMarker`, and
`Test_receiveChannelByteCode_StackLayout` — covering the success case, the
closed-channel case, error handling for non-channel and stack-marker inputs,
and the exact push order/count on the stack.

`compiler/assignment_test.go` — one new compile-time case verifying
`v, ok := <-ch` compiles without error.

`compiler/run_test.go` — `TestBUG07TwoValueChannelReceive`, a standalone Go
test (separate from `TestArbitraryCodeFragments`, which lacks builtins like
`make`) that injects a pre-built `*data.Channel` directly into the symbol table
and exercises three scenarios: a value is waiting (`ok == true`), the received
value itself is correct, and a closed channel yields `ok == false`. Because this
test compiles a fragment using a channel that was never declared with `make()`
in the Ego source, it pins `defs.UnknownVarSetting` to `false` for its duration
so its outcome doesn't depend on a developer's persisted `~/.ego/` profile
settings or on what other tests in the package happen to run first.

`tests/flow/two_value_receive.ego` — five Ego language tests: receiving a
buffered value (`ok == true`, correct value), draining multiple buffered values
in order, detecting a closed channel (`ok == false`, `v == nil`), draining all
buffered items before reporting closure, and receiving correctly when the
sender is a separate goroutine.

---

<a id="BUG-08"></a>

### BUG-08 — `delete(struct, key)` fails on dynamic structs

**Severity:** MEDIUM

**Description:**  
LANGUAGE.md documents `delete()` as: `"Remove the named field from a map, or a
delete a dynamic struct member"`. In practice, calling `delete(s, "field")` on
a dynamic (empty-literal) struct fails with `"invalid or unsupported data type
for this operation: argument 1: struct"`.

**Reproducer:**

```go
import "fmt"

func main() {
    s := {}
    s.x = 1
    s.y = 2
    delete(s, "x")    // should remove field x
    fmt.Println(s)    // expected: {y: 2}
}
```

**Actual output:**

```text
Error: at delete(line 7), invalid or unsupported data type for this operation: argument 1: struct
```

**Expected output:**

```text
{y: 2}
```

**Notes:**  
`delete()` on a `map` type works correctly. Only the struct variant is broken.

**Resolution:**

The `delete()` function now works with dynamic structs (Ego structs created using
an empty struct constant). If the struct is not a dynamic struct (which is an Ego
extension), most structs are static like Go) then a read-only error is generated.
If the field name does not exist, an error is generated.

Along the way, noted that field order was not tracked for dynamic structs. That is,
the order in which the fields are declared should be used as the order to print
the fields when printing a formatted version of the struct. This was added, along
with code to remove a field name from the field order list when the field was
deleted.

Ego unit tests added.

---

<a id="BUG-09"></a>

### BUG-09 — Import alias (`import alias "pkg"`) not recognized at use site

**Severity:** MEDIUM

**Description:**  
LANGUAGE.md documents an import alias syntax: `import str "strings"`, after which
the package should be accessible as `str.ToUpper(...)`. In practice, using the
alias name produces `"unknown symbol: str"` — the alias is ignored and the
package is not accessible under that name or its original name.

**Reproducer:**

```go
import str "strings"
import "fmt"

func main() {
    result := str.ToUpper("hello")   // unknown symbol: str
    fmt.Println(result)
}
```

**Actual output:**

```text
Error: at line 5:8, unknown symbol: str
```

**Expected output:**

```text
HELLO
```

**Workaround:**  
Import the package under its canonical name and use it without aliasing.

---

<a id="BUG-10"></a>

### BUG-10 — `json.Unmarshal(b)` single-argument form rejected

**Severity:** MEDIUM

**Description:**  
LANGUAGE.md documents that `json.Unmarshal` can be called with just one argument
(the JSON byte array), in which case the decoded value is returned directly:
`r := json.Unmarshal(s)`. In practice this fails with
`"incorrect function argument count: 1"`.

**Reproducer:**

```go
import "fmt"
import "json"

func main() {
    b := json.Marshal({name: "Tom", age: 44})
    r := json.Unmarshal(b)     // single-arg form documented to return decoded value
    fmt.Println(r)
}
```

**Actual output:**

```text
Error: incorrect function argument count: 1
```

**Expected output:**

```text
{age: 44, name: "Tom"}
```

**Workaround:**  
Always use the two-argument form: `err := json.Unmarshal(b, &target)`.

**Resolution:**

The single-argument version was a legacy from a much older version of Ego
that didn't yet properly support pointers, so the Go-compliant version
was not possible. There's no reason to retain this legacy variable, so
the fix is to delete it from the documentation and require that the
`Unmarshal()` function be called properly.

---

<a id="BUG-11"></a>

### BUG-11 — `fmt.Printf()` two-value return fails

**Severity:** MEDIUM

**Description:**  
LANGUAGE.md states that `fmt.Printf` returns `(length int, error)`. When callers
use the two-value assignment form `n, err := fmt.Printf(...)`, Ego reports
`"incorrect number of return values"`. The single-value form `n := fmt.Printf(...)`
works and returns the character count.

**Reproducer:**

```go
import "fmt"

func main() {
    n, err := fmt.Printf("hello %d\n", 42)
    fmt.Println("n:", n, "err:", err)
}
```

**Actual output:**

```text
hello 42
Error: at main(line 4), incorrect number of return values
```

**Expected output:**

```text
hello 42
n: 8 err: <nil>
```

**Notes:**  
`fmt.Sscanf` correctly supports the two-value return form. The bug is specific to
`fmt.Printf` and `fmt.Println` (which also returns `(int, error)` in Go but is not
documented to do so in Ego).

**Resolution:**

A number of the fmt functions were not observing the convention where a function that
can return an optional error value (like fmt.Printf()) must use a data.List as the
function argument, where the list contains the return values, and the return error is
also returned as the second parameter. When the caller function sees these, it tolerates
not having enough values on the stack for all destinations of the assignment. For
example:

```go
   fmt.Println("Hello")
   len = fmt.Println("Hello")        // result is 6
   len, err = fmt.Println("Hello")   // result is 6, <nil>
```

This fix addressed Print(), Println(), and Printf() which all had variations of
this issue.

---

<a id="BUG-12"></a>

### BUG-12 — Writing to a nil map succeeds; should error

**Severity:** MEDIUM  **Status:** Fixed

**Description:**  
In Go, assigning to an entry in a nil map panics: `"assignment to entry in nil map"`.
In Ego, a nil map (declared with `var m map[string]int`) silently accepted writes
and retained the written values. This silently corrupted program state instead of
alerting the programmer.

**Fix:**  
Ego maps are already Go reference types (`*data.Map`). The nil-state is now represented
by leaving the internal `m.data` field as Go's zero value (`nil` native map) while keeping
the `*Map` wrapper non-nil (so type metadata is preserved for introspection). Two call
sites were changed:

- `data.InstanceOfType` and `(*data.Type).InstanceOf` for `MapKind` now return
  `data.NewNilMap(keyType, valueType)` (nil-state) instead of `data.NewMap(...)`.
  This covers `var m map[K]V` declarations.
- `Map.Set()` now checks `m.data == nil` and returns `errors.ErrNilMapWrite` before
  attempting any write. This is a catchable Ego runtime error.
- `Map.SetAlways()` auto-vivifies `m.data` on first write (for trusted native Go runtime
  callers that initialize struct-owned map fields via `SetAlways`).
- `data.IsNil()` was extended with a `*Map` case so that `m == nil` in Ego scripts
  correctly returns true for nil-state maps.
- `builtins.NewInstanceOf` (`$new`) uses `data.NewMap()` directly for `MapKind` types,
  so that map literals (`map[K]V{}`) produce usable initialized maps while `var m map[K]V`
  (which calls `InstanceOf` directly) produces nil-state maps.

**Reproducer (now correctly errors):**

```go
import "fmt"

func main() {
    var m map[string]int   // nil map

    try {
        m["key"] = 42      // raises ErrNilMapWrite
    } catch(e) {
        fmt.Println(e)     // "assignment to entry in nil map"
    }
}
```

**Notes:**

- Reading from a nil map (`v := m["key"]`) correctly returns `nil` with no error (Go spec).
- `len(m)`, `range m`, and `delete(m, k)` are all safe on nil maps (zero iterations / no-ops).
- Assigning an initialized literal (`m = map[string]int{}`) escapes the nil state.
- Tests in `tests/types/nil_map.ego` cover all 14 nil-map behavioral cases.

---

<a id="BUG-13"></a>

### BUG-13 — `typeof()` result incompatible with `switch` case matching

**Severity:** MEDIUM  
**Status:** Fixed (EQUAL-4 / COMPARE-4)

**Description:**  
`typeof(v)` returns a type object (`*data.Type`). A "cheat" in `equalByteCode` allowed
`typeof(v) == "int"` to return `true` by comparing `actual.String()` to the string literal
via `equalTypes()`. This predated the Ego type system and created an inconsistency: `==`
coerced the type to a string, but `switch` case matching did not — it tried to coerce
`"int"` (the case value) to an integer, failing with `"invalid integer value: int"`.

**Root cause:**  
`equalTypes()` in `internal/language/bytecode/equal.go` accepted a `string` v2 argument and
compared `actual.String() == v`, enabling type-to-string equality. The compiler pushes the
case value first and loads the switch expression second, so in a `switch typeof(n)` with
`case "int":`, v1 = `"int"` (string) and v2 = `*data.Type`. This reversed order went to
`genericEqualCompare` → `Normalize` → `Coerce`, which tried to coerce `"int"` into an
integer — producing the error.

**Fix:**  
Removed the string branch from `equalTypes()`. A `*data.Type` is now only equal to another
`*data.Type` with the same canonical name (`actual.String() == v.String()`), and unequal to
all non-type values. Added a symmetric guard in both `equalByteCode` and `notEqualByteCode`
to handle the case where v2 is a `*data.Type` and v1 is not (the switch-case ordering).

**Behavior after fix:**

- `typeof(n) == int` → `true` (type constant comparison — correct)
- `typeof(n) == "int"` → `false` (type is never equal to a string literal)
- `typeof(n) != "int"` → `true`
- `switch typeof(n) { case int: ... }` → matches correctly
- `switch typeof(n) { case "int": ... }` → does not match (no error)

**Callers updated:**  
Test files that compared `reflect.Reflect(v).BaseType == "typename"` (a `*data.Type` field
compared to a string literal via the cheat) were updated to either use a type constant
(`== int`, `== float32`, etc.) or a string cast (`string(r.BaseType) == "struct{...}"`).

**Files changed:**

- `internal/language/bytecode/equal.go` — removed string branch from `equalTypes()`,
  added EQUAL-4 guard for type-on-right ordering
- `internal/language/bytecode/notEqual.go` — added COMPARE-4 guard and `case *data.Type:`
- `internal/language/bytecode/equal_test.go` — updated tests for new behavior
- `tests/json/unmarshal.ego` — `reflect.Type(arr[0]) == "map[...]"` → string cast
- `tests/reflect/reflect_scalars.ego` — `.BaseType == "typename"` → type constants
- `tests/reflect/reflect_structs.ego` — `.BaseType == "struct"` → `string()` cast or `Index`
- `tests/reflect/reflect_packages.ego` — `.BaseType == "struct"` → `string()` cast or `Index`
- `tests/datamodel/float32.ego` — `.BaseType == "float32"` → `== float32` type constant
- `tests/typeof/comparison.ego` — new tests verifying the corrected behavior

---

<a id="BUG-14"></a>

### BUG-14 — Typed array element type not enforced in dynamic mode

**Severity:** MEDIUM

**Description:**  
A typed array (`[]int{1, 2, 3}`) should reject assignments of incompatible types
to its elements. LANGUAGE.md explicitly states this: `"a[1] = 1325.0 // Failed,
must be of type int"`. In practice, `[]int` elements accept strings, floats, and
other types silently in dynamic mode.

**Reproducer:**

```go
import "fmt"

func main() {
    a := []int{1, 2, 3}
    a[0] = "hello"           // should error: wrong type for []int
    fmt.Println("a:", a)     // prints: ["hello", 2, 3]  (array corrupted)
}
```

**Actual output:**

```text
a: ["hello", 2, 3]
```

**Expected output:**

```text
Error: wrong type for element of []int: string
```

(or a coercion to int with an error for non-numeric strings)

**Notes:**  
The LANGUAGE.md says `a[1] = 1325.0` on a `[]int` should fail, but in practice
`1325.0` is silently truncated to `1325`. Even `a[0] = "string"` silently
converts the array to a mixed-type array. Element type enforcement is entirely
absent.

***Resolution:***
Its a little more complicated than that. The Ego-correct behavior depends on the
current mode checking:

- Strict requires that the type match
- Relaxed will try to coerce the type to fit if possible
- Dynamic will just convert the array type to []any

Changes where made in StoreIndex to evaluate if type coercion of the value
or the array are possible. A function `MakeAny()` was added to *data.Array
elements that converts the type from a specific array type to []any.

---

<a id="BUG-15"></a>

### BUG-15 — `append()` to typed array silently accepts wrong-type elements

**Severity:** MEDIUM  
**Status:** Fixed (APPEND-2)

**Description:**  
`append()` did not enforce the element type of a typed array in dynamic (default)
mode. Appending a `string` to a `[]int` silently succeeded and corrupted the
array's type contract.

**Root cause:**  
`builtins/append.go` gated the element-type check with
`typeChecking < defs.NoTypeEnforcement`. In dynamic mode (`NoTypeEnforcement = 2`),
`2 < 2` is false, so the check never ran and any value was accepted.

**Fix:**  
Removed the `typeChecking < defs.NoTypeEnforcement` guard from the element-type
check (APPEND-2 in `internal/builtins/append.go`). A typed array's element-type
contract is now always enforced regardless of the type-checking mode. The mode
still controls *how* a mismatch is handled:

- **strict (0)**: reject immediately with `ErrWrongArrayValueType`
- **relaxed (1)**: attempt coercion; error if coercion fails
- **dynamic (2)**: same as relaxed — coerce if possible, error if not

Interface-typed arrays (`[]interface{}`) continue to accept any element.

**Behavior after fix:**

```go
a := []int{1, 2, 3}
a = append(a, "hello")   // error: wrong type for element of []int (all modes)
a = append(a, true)      // succeeds: bool→int coercion (relaxed/dynamic)
f := []float64{1.1}
f = append(f, 3)         // succeeds: int→float64 coercion (relaxed/dynamic)
```

**Files changed:**

- `internal/builtins/append.go` — removed `typeChecking < defs.NoTypeEnforcement`
  gate; added APPEND-2 comment block explaining the invariant
- `internal/builtins/append_test.go` — renamed `Test_Append_NoTypeEnforcementSkipsCheck`
  to `Test_Append_DynamicModeCoercesCompatibleType` (the old test documented wrong
  behavior); added `Test_Append_DynamicModeRejectsIncompatibleType` (BUG-15 fix test)
- `tests/types/append_type_check.ego` — new Ego-level tests covering: compatible
  appends, string→int rejection, coercible types, interface arrays, multi-element

---

<a id="BUG-16"></a>

### BUG-16 — `defer namedFunc(arg)` evaluates arguments lazily (cross-ref: FLOW-M4)

**Severity:** MEDIUM

**Description:**  
Already documented as `FLOW-M4` in `FUNCTIONAL_ISSUES.md`. Included here for
completeness. In Go, the arguments of a deferred function call are evaluated
immediately when the `defer` statement executes. In Ego, they are evaluated
lazily when the deferred function runs (at function return time), so the final
value of the variable is seen instead of the value at defer time.

**Reproducer:**

```go
import "fmt"

func main() {
    x := 1
    defer fmt.Println("defer x (should be 1):", x)
    x = 2
    defer fmt.Println("defer x (should be 2):", x)
    x = 3
    fmt.Println("main, x =", x)
}
```

**Actual output (LIFO, all see final x):**

```text
main, x = 3
defer x (should be 2): 3
defer x (should be 1): 3
```

**Expected output:**

```text
main, x = 3
defer x (should be 2): 2
defer x (should be 1): 1
```

**Workaround (no longer required, kept for historical reference):**  
Use a closure that receives the value as an argument:

```go
defer func(v int) { fmt.Println("defer x:", v) }(x)
```

**Resolution:**  
See `FLOW-M4` for the full write-up of the fix — the two identifiers track the
same underlying bug and were fixed together. Running the reproducer above now
correctly prints:

```text
main, x = 3
defer x (should be 2): 2
defer x (should be 1): 1
```

---

<a id="BUG-17"></a>

### BUG-17 — `var name = value` (type-inferred initializer) not supported

**Severity:** LOW  
**Status:** Fixed

**Description:**  
The originally reported symptom was that the `var (...)` group form failed
with `"invalid type specification"`. Further investigation showed the bug
report's title was not quite accurate: the Ego compiler already accepted
grouped `var` declarations that include an explicit type, for example:

```go
var (
    a int
    b string
)
```

The actual bug is narrower: **`var` initializers without an explicit type**
were rejected, whether written as a single declaration or inside a `var (...)`
group. That is,

```go
var pi = 3.14
```

failed with `"invalid type specification"`, while the fully-typed equivalent

```go
var pi float64 = 3.14
```

worked correctly. The compiler had no code path to infer a variable's type
from its initializer expression when no type token was present — it only knew
how to (a) use an explicit type token, or (b) treat the token after the name
as a user-defined type name. Since `=` is neither, case (b)'s error path fired.

**Reproducer:**

```go
import "fmt"

func main() {
    var (
        p = 3.14
        q = "pi"
    )
    fmt.Println(p, q)
}
```

**Actual output (before fix):**

```text
Error: at line 5:5, invalid type specification
```

**Expected (and now actual) output:**

```text
3.14 pi
```

**Fix:**  
`compileVar` in `internal/language/compiler/var.go` now distinguishes two
different reasons `parseTypeSpec()` can return an undefined type:

1. The name is immediately followed by `=` (no type token at all) — the type
   must be inferred from the initializer expression, exactly like the short
   variable declaration `pi := 3.14`. This case is now handled by a new
   function, `varInferredInitializer`, which compiles the right-hand-side
   expression with the ordinary expression compiler (no target type, no
   coercion) and stores the result — the variable's type becomes whatever the
   expression's runtime type turns out to be.
2. The name is followed by an identifier that isn't a recognized built-in
   type — this is presumed to be a user-defined type name and is still
   handled by the existing `varUserType`.

A second, related bug was found and fixed as part of this change:
`compileVar`'s per-declaration loop (used for the parenthesized `var (...)`
list form) was controlled by a second return value from
`collectVarListNames` that was supposed to mean "are there more declarations
in this list?" but actually just signaled "did name-collection stop early
because a name was immediately followed by `=` or `)`?" — a fact already
available for free from an existing check at the top of the loop. Those two
meanings coincide exactly in the newly-enabled "name immediately followed by
`=`" case, so a `var (...)` list containing more than one type-inferred
declaration would have silently stopped after the first one. The redundant
return value was removed from `collectVarListNames`, and list continuation is
now decided solely by the top-of-loop "have we reached the closing `)`"
check — which already existed and is correct for every declaration form.

**Known limitation (unchanged, not introduced by this fix):** `var a, b =
42` duplicates the single computed value across every declared name, rather
than implementing Go's true multi-value form (`var a, b = 1, 2`, one value
per name). This mirrors the pre-existing behavior of the typed path (`var a,
b int = 42` already worked the same way before this fix); real Go actually
rejects `var a, b int = 42` at compile time, so this is a long-standing Ego
extension/quirk, not a regression from this fix. `var a, b = 1, 2` (distinct
values) is not supported by either path and was out of scope for this fix.

**Files changed:**

- `internal/language/compiler/var.go` — added `varInferredInitializer`;
  restructured `compileVar`'s branch on `kind.IsUndefined()` to check for a
  following `=` before falling back to `varUserType`; removed the unused
  `bool` return from `collectVarListNames` and decoupled list-loop
  continuation from it
- `internal/language/compiler/var_inferred_test.go` — new Go unit tests
  covering single-name inferred declarations (scalar, expression, and typed
  composite-literal initializers), multi-declaration `var (...)` lists
  (two, three, and mixed typed/inferred/user-type entries), the multi-name
  shared-value case, and regression guards for unknown user types and
  malformed names
- `tests/compiler/var_inferred.ego` — new Ego-level tests covering the same
  cases using `@test`/`@assert`, plus `@compile block` regression guards for
  the two error paths that must still fail correctly

---

<a id="BUG-18"></a>

### BUG-18 — `type()` documented but actual builtin is `typeof()`

**Severity:** LOW

**Description:**  
LANGUAGE.md (Functions section for `func` statement) states: `"If the function
body needs to know the actual type of the value passed, the type() function would
be used."` The actual builtin is named `typeof()`. Calling `type(v)` fails with
`"in type, invalid value: <v>"`.

**Test:**

```go
import "fmt"
func main() {
    n := 42
    t1 := type(n)     // Error: in type, invalid value: 42
    t2 := typeof(n)   // OK: returns "int"
    fmt.Println(t1, t2)
}
```

**Notes:**  
This is a documentation bug, not an implementation bug. `typeof()` works correctly.
The LANGUAGE.md should be updated to use `typeof()`.

**Resolution:**
Documentation updated to use correct function name `typeof()`.

---

<a id="BUG-19"></a>

### BUG-19 — `for v := range string` yields single-char strings, not int32 runes

**Severity:** LOW  
**Status:** Fixed

**Description:**  
In Go, ranging over a string yields `(byte_index int, rune int32)` — the rune
value is an integer representing the Unicode code point. In Ego, the second
variable received a single-character `string`, not an `int32`. This behavioral
difference could cause porting issues for Go developers and silently produced
a different type than what Go documentation describes.

**Test:**

```go
import "fmt"
func main() {
    for i, c := range "ABC" {
        fmt.Printf("i=%d, c=%v, type=%T\n", i, c, c)
    }
}
```

**Previous (buggy) output:**

```text
i=0, c=A, type=string
i=1, c=B, type=string
i=2, c=C, type=string
```

**Expected (and now actual) Go-compatible output:**

```text
i=0, c=65, type=int32
i=1, c=66, type=int32
i=2, c=67, type=int32
```

**Fix:**  
`rangeNextString` in `internal/language/bytecode/range.go` pre-decodes a
string being ranged over into two parallel slices during `rangeInitByteCode`:
`keySet` (byte offsets) and `runes` (decoded `rune` values, i.e. `int32` code
points). On each step, `rangeNextString` previously wrapped the decoded rune in
`string(value)` before storing it into the loop's value variable, converting
it from an `int32` code point into a one-character string. That conversion
call was removed — the rune (already Go type `rune`, an alias for `int32`) is
now stored directly, matching Go's `for i, ch := range s` semantics where `ch`
has type `rune`.

No compiler changes were required: `compiler/for.go` and `rangeInitByteCode`
were already type-agnostic about the loop value; only the final assignment
step in `rangeNextString` needed correcting.

**Files changed:**

- `internal/language/bytecode/range.go` — `rangeNextString` now stores the
  decoded rune value directly instead of converting it to a string first
- `internal/language/bytecode/range_test.go` — updated
  `Test_rangeNextString_FullIteration` and
  `Test_rangeNextString_MultiByteUTF8` to assert `int32` rune values instead
  of single-character strings
- `tests/flow/forrange.ego` — updated the "range over a string" test case to
  compare against `int32` code points (`'f'`, `int32(0x2318)`, etc.) instead
  of single-character strings

**Notes:**  
This is an intentional behavior change to match Go semantics, not a
documentation-only clarification — any existing Ego program that relied on
`for _, ch := range someString` producing single-character strings must be
updated to treat `ch` as an integer code point (e.g., use
`string([]int32{ch})` or `fmt.Sprintf("%c", ch)` to get a one-character string
back, or compare `ch` against a rune literal like `'A'` directly).

---

<a id="BUG-20"></a>

### BUG-20 — `iota` not supported in `const` blocks

**Severity:** LOW

**Description:**  
Go's `iota` identifier provides automatically-incrementing constants in a `const`
block. Ego does not support `iota` — any reference to it in a `const` block
produces `"invalid constant expression file"`. This is not documented as an
unsupported feature.

**Reproducer:**

```go
const (
    Apple  = iota
    Banana
    Cherry
)
```

**Actual output:**

```text
Error: at line 2:9, invalid constant expression file
```

**Notes:**  
Since `iota` is not mentioned in LANGUAGE.md, it may be intentionally unsupported.
The error message is confusing and unhelpful — it should say something like
`"iota is not supported in Ego const blocks"`.

**Resolution:**  
`iota` is now supported inside `const` declarations, matching Go's behavior,
including the common idiom of omitting `= expr` on later entries in a block so
they repeat the nearest preceding entry's expression:

```go
const (
    Apple  = iota   // 0
    Banana          // 1 (repeats "= iota")
    Cherry          // 2 (repeats "= iota")
)
```

Two changes were needed, both in `internal/language/compiler/`:

1. **Recognizing `iota` as a value, not a symbol lookup** (`expr_atom.go`,
   `expressionAtom()`): `iota` is not a reserved word — there is no dedicated
   tokenizer type for it — so it is matched by its spelling, the same way the
   `true`/`false` boolean literals are recognized. When the compiler is inside
   a `const(...)` declaration, a bare `iota` reference is compiled as a literal
   integer push instead of falling through to the ordinary symbol-lookup path
   (which is what previously produced `ErrInvalidConstant`, since `iota` isn't
   itself a previously-declared constant).

2. **Tracking the current counter and supporting the "omit the expression"
   shorthand** (`constant.go`, `compileConst()`): a new `Compiler.iota` field
   holds the zero-based position of the ConstSpec currently being compiled,
   or `-1` when the compiler is not inside a `const` declaration at all (so an
   unrelated use of the name `iota` elsewhere in a program still resolves as a
   normal, possibly-undefined, identifier — matching Go's own
   `"undefined: iota"` behavior outside of `const`). `compileConst()` sets this
   to `0` at the start of a block, increments it after every spec (whether or
   not that spec had its own `= expr`), and restores the previous value on
   exit via a `defer`. `Compiler.Clone()` (used internally by `Expression()`)
   was updated to copy this field so the isolated expression-compiling clone
   still sees the active counter.

   To support omitting `= expr`, `compileConst()` now remembers the token
   positions spanning the most recent explicit right-hand side. When a later
   spec has no `=` of its own, those same tokens are re-parsed (via the
   tokenizer's existing `Mark()`/`Set()` bookmarking, the same mechanism
   already used elsewhere in the compiler for speculative parsing) against the
   *current* `iota` value, which reproduces Go's specified "textual
   substitution of the first preceding non-empty expression list" semantics
   without needing a general-purpose constant-folding evaluator. This only
   applies inside a parenthesized `const ( ... )` block, and only when a
   preceding spec actually had an expression to repeat — a single,
   non-parenthesized `const Name` with no `=`, or the very first spec in a
   block with no `=`, are still compile errors (`ErrMissingEqual`), exactly as
   before this change.

**Tests:**

- Go unit tests in `internal/language/compiler/constant_test.go`:
  `TestCompiler_compileConst` gained table cases for a bare `iota`, the
  repeat-the-previous-expression idiom, `iota` inside a larger expression
  (`1 << iota`), the standalone (non-block) form, and two guard cases
  confirming `ErrMissingEqual` is still raised when there's nothing to repeat.
  `TestCompiler_compileConst_IotaValues` inspects the emitted bytecode's
  `Push` operands directly to confirm `iota` actually takes the values `0, 1,
  2, ...` in order (not just that compilation succeeds).
  `TestCompiler_compileConst_IotaScopeRestored` confirms `Compiler.iota` is
  restored to its `-1` sentinel after a `const` block finishes compiling.
- Ego-language tests in `tests/types/iota.ego` cover the exact reproducer
  above, explicit `iota` on every spec, the classic bit-shift/blank-identifier
  idiom (`KB = 1 << (10 * iota)`), independent counters across separate
  `const` blocks, the standalone form, mixing `iota` with plain constants, and
  `iota` inside an arithmetic expression. Two `@compile block { ... }
  catch(e) { ... }` tests verify the error paths: referencing `iota` outside
  any `const` declaration reports `symbol.not.found` (matching Go's own
  "undefined: iota" behavior), and a `const` block whose first entry omits `=`
  still reports `equals` (`ErrMissingEqual`).

---

<a id="BUG-21"></a>

### BUG-21 — `@compile` test directive cannot pass computed values back to the enclosing test

**Severity:** LOW

**Description:**  
The `@compile { ... } catch(e) { ... }` directive (see
`internal/language/compiler/directives.go:compileBlockDirective` and
`tests/directives/compile.ego`) lets a test compile a snippet of code at
runtime and inspect any compile error in `catch(e)` instead of aborting the
whole test. It has two forms, and both have a scoping gap:

1. **Full-program form** (`@compile { ... }`, no `block` keyword): the content
   must be a complete, standalone program with its own prolog/`main()`. By
   design, only symbols declared directly at that sub-program's *top level*
   (`subCompiler.s.Names()`) are copied back into the enclosing test via
   `CreateAndStore`. Anything computed inside a function body in that
   sub-program (including `main()`) lives in that function's own call-time
   scope and is discarded once the synthetic program finishes running — there
   is no mechanism to read it back out from the enclosing `@test` block.

2. **Block form** (`@compile block { ... }`): this form is supposed to share
   the caller's scope chain, and plain assignment to a pre-declared outer
   variable does work for simple cases (e.g. the existing
   `tests/directives/compile.ego` example `x = 33`). However, if the block
   *also* contains an `import` statement, a subsequent assignment to a
   pre-declared outer variable silently fails to propagate — no error is
   raised, the outer variable is just left unchanged. This looks like a
   narrower, distinct defect from (1), possibly related to how compiling an
   `import` statement inside block mode interacts with the parent scope
   chain, but it has not been root-caused.

**Reproducer (full-program form, item 1):**

```go
@test "demo: @compile full-program form cannot pass data out"
{
    @compile {
        func foo() {
            x := 33   // computed here, but unreachable from outside
        }
    } catch(e) {
        @fail "unexpected compile error"
    }
    // No way to observe x's value (33) from here.
}
```

**Reproducer (block form + import, item 2):**

```go
@test "demo: @compile block + import drops an outer assignment"
{
    var result string

    @compile block {
        import alias "strings"
        result = alias.ToUpper("hi")
    } catch(e) {
        @fail "unexpected compile error"
    }

    @assert result == "HI"   // FAILS: result is still ""
}
```

**Actual output:**  
No compile error in either case; the code runs successfully, but the computed
value never reaches the enclosing test. In the block-form reproducer,
`result` is `""` instead of `"HI"`.

**Expected output:**  
Some supported way to read a value computed inside an `@compile`-compiled
block or program back into the enclosing test, or (at minimum) clear
documentation that `@compile` is pass/fail-only and values must be checked
with assertions *inside* the compiled block itself.

**Notes:**  
Workaround: write the assertions inside the `@compile`/`@compile block` body
itself (or just check the `failed`/`catch` flag) rather than relying on an
outer variable being updated. This is a test-infrastructure limitation, not a
defect in user-facing Ego programs — flagged during work on BUG-09, filed for
later investigation.

**Resolution:**
The directive hoists symbols exported from the compilation to the current
symbol scope at which the `@compile{}` runs. This allows the code to make
a call to a function insice the compilation unit to extract values or
status from the execution of the compilation unit.

---

<a id="BUG-22"></a>

### BUG-22 — `make(map[K]V)` errors with "incorrect function argument count"

**Severity:** MEDIUM  
**Status:** Fixed

**Description:**  
In Go, maps may be created with `make(map[K]V)` or `make(map[K]V, initialCapacity)`.
In Ego, calling `make` with a map type argument previously failed at runtime:

```text
Error: in make, incorrect function argument count: 1
```

The `make` built-in only handled channels and arrays/slices. Map types were not
handled, so any call to `make(map[K]V)` errored regardless of whether an initial
capacity hint was provided.

**Fix:**  
Added a map branch at the top of `Make()` in `internal/builtins/make.go`. When the
first argument is a `*data.Type` with `Kind() == MapKind`, the function:

1. Validates the optional capacity hint (must be a non-negative integer if present)
2. Creates and returns a new initialized `*data.Map` via `data.NewMap(keyType, valueType)`

The capacity value is accepted for Go source compatibility but is currently ignored
for Ego maps, matching Go's semantics where it is only a performance hint.

The function declaration in `internal/builtins/functions.go` was updated to allow
1–3 arguments (`MinArgCount: 1, MaxArgCount: 3`), accommodating both
`make(map[K]V)` and `make(map[K]V, n)`.

**Array capacity (third argument) also validated:**  
The array path already accepted a third `capacity` argument but only checked for
negative values. Validation now also rejects capacity < size (matching Go's
`make([]T, size, cap)` panic semantics with a catchable Ego error instead).

**Files changed:**

- `internal/builtins/make.go` — added map branch; validated array capacity range
- `internal/builtins/functions.go` — updated `make` declaration to `MinArgCount: 1`
- `internal/builtins/make_test.go` — added 11 new Go unit tests covering all map
  and array-with-capacity combinations (no-size map, capacity hint, zero capacity,
  negative capacity, non-integer capacity, writable result, empty after creation;
  array with valid/equal/less-than/negative capacity)
- `tests/types/make_map.ego` — 12 new Ego-level tests covering the same cases

---

<a id="BUG-23"></a>

### BUG-23 — `var` declarations of struct types share a single compile-time instance across calls

**Severity:** MEDIUM

**Description:**  
When a named function contains a `var c MyStruct` declaration, the compiler evaluates
`InstanceOf(MyStruct)` **once at compile time** and embeds the resulting `*Struct`
pointer as a bytecode `Push` constant. Every call to the function pushes and mutates
that same shared pointer, so state accumulates across calls.

**Reproducer:**

```go
type Counter struct { n int }

func increment() Counter {
    var c Counter   // same *Struct every call
    c.n = c.n + 1
    return c
}

func main() {
    fmt.Println(increment().n)  // want 1, got 1  ✓
    fmt.Println(increment().n)  // want 1, got 2  ✗
    fmt.Println(increment().n)  // want 1, got 3  ✗
}
```

**Actual output:**

```text
1
2
3
```

**Expected output:**

```text
1
1
1
```

**Notes:**  
Discovered during the BUG-12 nil-map investigation. The aliasing bug affects
`*Struct` instances declared with `var`; it does **not** affect maps (nil-state maps
cannot be mutated, so the shared pointer is harmless) or arrays declared with `var`
(operations like `append` return a new array rather than mutating the original).

The fix requires the compiler or the `SymbolCreate`/`CreateAndStore` opcode to call
`data.DeepCopy` on the embedded constant before storing it, so each function call
receives a fresh copy rather than the shared compile-time instance.

***Resolution:***
The fundamental issue was that the `var` statement was using the common "zero-value"
for the type, but was using the same one for any `var` value for that type. The
correct fix is to modify the `var` compilation to call the internal `$new()`
function at runtime which generates a unique instance of the item.

---

<a id="BUG-24"></a>

### BUG-24 — Multi-target assignment lists reject indexed/member lvalues (`m[k], arr[i] = ...`)

**Severity:** MEDIUM

**Description:**  
A comma-separated list of assignment targets only works correctly when every target
is a simple, unqualified variable name (`a, b, c = ...`). As soon as any target in the
list is an indexed or member expression (`m["k"]`, `arr[0]`, `s.field`), the assignment
fails at runtime with `invalid or unsupported data type for this operation`. This is
pre-existing behavior in `assignmentTargetList` (`internal/language/compiler/lvalue.go`)
found while adding Go-style parallel assignment support (`a, b, c = 10, 20, 30`); it
reproduces even on the multi-return-call form that has been supported for a long time,
so it is not a regression from that new feature.

**Reproducer:**

```go
func pair() (int, int) {
    return 5, 6
}

func main() {
    m := map[string]int{}
    arr := []int{0, 0}

    m["k"], arr[0] = pair()
    fmt.Println(m["k"], arr[0])
}
```

**Actual output:**

```text
Error: at main(line 8), invalid or unsupported data type for this operation
```

**Expected output:**

```text
5 6
```

**Notes:**  
Single-target indexed/member assignment (`m["k"] = 5`) works fine on its own; the bug
is specific to mixing indexed/member targets into a *multi-target* list. The same
failure occurs with a literal expression list (`m["k"], arr[0] = 5, 6`), with the
two-value form of a multi-return call, and regardless of whether `:=` or `=` is used.
A workaround is to assign through a temporary simple variable and then copy it into the
indexed/member target on its own line.

---

### Testing Methodology

All bugs were found by writing small Ego programs to `/tmp/test_*.ego` and running
them with:

```sh
./ego run /tmp/test_xxx.ego
./ego --timeout 5s run /tmp/test_xxx.ego   # for tests involving goroutines
```

Tests covered: arithmetic, type conversions, string operations, maps, structs,
closures, goroutines, channels, defer/recover, error handling, built-in functions,
packages (math, sort, strings, strconv, errors, json, fmt), operators, and scoping.

---

### About This Section

There is a section (`##`) for each area of Ego that was evaluated. Within each
section, issues are grouped by priority (HIGH, MEDIUM, LOW). Each issue has a
unique identifier of the form `AREA-PN`, with three components to the issue
identifier:

- `AREA` the functional area such as "FUNC" for issues with functions,
- `P` is the priority (H for high, M for medium, L for low),
- `N` is a sequence number for that AREA and Priority.

So for example, the first high-priority issue for Functions (FUNC) is named `FUNC-H1`,
and the second low-priority issue around input/output (IO) would be named `IO-L2`.

---

<a id="area-func"></a>

## FUNC — Functions

| ID | Priority | Summary | Status |
| -- | -------- | ------- | ------ |
| [FUNC-H1](#FUNC-H1) | High | Calling a variadic function with zero variadic arguments failed with an argument-count error. | ✓ |
| [FUNC-H2](#FUNC-H2) | High | Closures storing a loop variable became invalid (unknown identifier) once the loop exited. | ✓ |
| [FUNC-M1](#FUNC-M1) | Medium | Named nested functions silently failed to capture the enclosing named function's scope. | ✓ |
| [FUNC-M2](#FUNC-M2) | Medium | Calling a value-receiver method on a pointer variable raised an unsupported-type runtime error. | ✓ |
| [FUNC-M3](#FUNC-M3) | Medium | Dynamic mode silently accepted wrong-type arguments at function call boundaries instead of coercing or erroring. | ✓ |
| [FUNC-L1](#FUNC-L1) | Low | String multiplication (`*`) behaved asymmetrically, treating a left-hand string operand as repetition. | ✓ |
| [FUNC-L2](#FUNC-L2) | Low | Using an explicit return value expression in a function with named returns was a compile error. | ✓ |

### High priority issues

<a id="FUNC-H1"></a>

#### FUNC-H1 — Calling a variadic function with zero variadic arguments fails

**Affected files:**

- `compiler/function.go` — argument count validation
- `bytecode/callframe.go` — argument unpacking at call time

**Description:**  
In Go, a function declared as `func f(args ...int)` may be called as `f()` —
passing zero variadic arguments is valid, and `args` inside the function body
is `nil` (or an empty slice). The same is true for functions with a fixed
parameter plus varargs: `func g(base int, args ...int)` may be called as
`g(10)` — the varargs receive an empty slice.

In Ego, both forms fail at runtime with `"incorrect function argument count"`:

```go
func sum(args... int) int { ... }
sum()       // ERROR: incorrect function argument count

func sumFrom(base int, args... int) int { ... }
sumFrom(10) // ERROR: incorrect function argument count
```

This breaks common Go patterns such as logging helpers, optional-parameter
functions, and aggregation functions that are sometimes called with no
additional values.

The spread operator (`sum(nums...)`) works correctly.

**Test file:** `tests/functions/variadics.ego` — tests
`"functions: zero variadic args is an error"` and
`"functions: zero varargs after fixed is error"` document the current behavior.

**Recommendation:**  
In the argument-count validation pass, treat a variadic parameter as
contributing zero or more arguments (not one or more). When the call provides
exactly the number of fixed parameters and no varargs, pass an empty (nil)
slice for the variadic parameter rather than rejecting the call.

**Resolution (May 2026):**  
Three changes to `compiler/function.go`:

1. **`generateFunctionBytecode` (lines 150–161):** The `ArgCheck` bytecode was
   emitted as `(len(parameters), -1)` for variadic functions, using the total
   parameter count as both the minimum and (sentinel) maximum. The minimum is
   now emitted as `len(parameters) - 1`, so only the fixed parameters are
   required and the variadic parameter contributes zero or more.

2. **`ParseFunctionDeclaration` (line 487):** The `hasVarArgs` return value from
   `parseParameterDeclaration` was silently discarded with `_`. It is now
   captured and used to set `funcDef.Variadic = hasVarArgs`, so the
   `Declaration.Variadic` flag is correctly propagated when a function is
   called via a `data.Function` wrapper (e.g. as a type method).

3. **`storeOrInvokeFunction` (line 323):** When building a `data.Declaration`
   for a function literal whose declaration was not already known, `Variadic`
   is now derived from the `parms` slice by checking whether the last
   parameter's kind is `VarArgsKind`.

The `ArgCheck` opcode handler (`bytecode/argcheck.go`) and the variable-argument
extraction instruction (`bytecode/stack.go:getVarArgsByteCode`) already handled
the empty-slice case correctly — no changes were needed there.

Tests updated in `tests/functions/variadics.ego`: the two tests that previously
expected errors on zero-argument calls now assert the correct return values
instead, and their names and comments reflect Go-compatible behavior.

---

<a id="FUNC-H2"></a>

#### FUNC-H2 — Closures stored during a loop are invalid after the loop ends

**Affected files:**

- `bytecode/bytecode.go` — `ByteCode` struct and accessor methods
- `bytecode/stack.go` — `pushByteCode` opcode handler
- `bytecode/callBytecodeFunction.go` — closure dispatch

**Description:**  
In Go, closures capture variables by reference and those references remain
valid for the lifetime of the closure (the garbage collector keeps the
captured variable alive as long as the closure lives). A common Go pattern is
to build a slice of closures inside a loop and call them later:

```go
var funcs []func() int
for i := 0; i < 3; i++ {
    i := i                          // capture-copy in Go
    funcs = append(funcs, func() int { return i })
}
// All closures are valid and callable after the loop ends
fmt.Println(funcs[0](), funcs[1](), funcs[2]()) // → 0 1 2
```

In Ego, the loop variable `i` (and any variable declared inside the loop
body, including an explicit copy `i_copy := i`) goes out of scope when the
loop exits. Any closure that references such a variable produces a runtime
error `"unknown identifier: i"` when called after the loop:

```go
for i := 0; i < 3; i = i + 1 {
    i_copy := i
    funcs = append(funcs, func() int { return i_copy })
}
funcs[0]()  // ERROR: unknown identifier: i_copy
```

Even the standard Go workaround of taking a copy of the loop variable does
not help in Ego, because the copy variable is itself scoped to the loop
body and goes out of scope at the same time as `i`.

Closures called *immediately* within the loop (before the variable leaves
scope) work correctly.

**Test file:** `tests/functions/scope_advanced.ego` — test
`"functions: stored closure is invalid after loop"` documents this behavior.

**Recommendation:**  
Extend closure capture semantics so that variables referenced by a closure are
kept alive (allocated on the heap or in a persistent parent symbol table) for
the full lifetime of the closure, even after the enclosing scope is popped.
This is the standard "escape analysis" approach used by Go and other languages
with first-class functions.

**Resolution (May 2026):**  
The root cause was two separate gaps:

1. **`ByteCode` had no field for the captured scope.** A compiled function
   literal is a single `*ByteCode` object. Every iteration of a loop reuses
   the same pointer — so simply setting a scope on the object would cause all
   iterations to overwrite each other.

2. **`callBytecodeFunction` always parented the closure's symbol table to
   `c.symbols` (the runtime scope at call time)**, not the scope that was
   active when the closure was defined.

Three changes fixed this:

- **`bytecode/bytecode.go`**: Added a `capturedScope *symbols.SymbolTable`
  field to `ByteCode` plus `Clone()`, `CaptureScope()`, and
  `GetCapturedScope()` accessors. `Clone()` returns a shallow copy of the
  struct; the instructions slice is shared (read-only after `Seal`) so the
  clone is cheap.

- **`bytecode/stack.go` — `pushByteCode`**: When the operand is a literal
  `*ByteCode`, the handler clones it and stamps `c.symbols` onto the clone's
  `capturedScope` field before pushing. Each push in a loop iteration
  produces a fresh clone with the scope at that moment. Non-literal bytecodes
  are pushed unchanged.

- **`bytecode/callBytecodeFunction.go`**: When dispatching a literal closure
  whose `capturedScope` is non-nil, the function's symbol table is created as
  a child of the captured scope (via `NewChildSymbolTable` +
  `callFramePushWithTable`) rather than of `c.symbols`. This ensures that
  variable lookup walks up through the captured scope chain, where the loop
  variable is still alive as a Go heap object even after `PopScope` removed
  it from the active parent chain.

The scope object is never freed by `PopScope` — Go's garbage collector keeps
it alive as long as any reference to it exists. Once a closure's
`capturedScope` holds that reference, the entire ancestor chain remains valid
and reachable for the lifetime of the closure, which matches Go's escape
analysis semantics.

Tests updated in `tests/functions/scope_advanced.ego`: the test previously
named `"functions: stored closure is invalid after loop"` is renamed to
`"functions: stored closure survives after loop"` and now asserts that
`captured()` returns `3` (the post-loop value of `i`) instead of asserting
that a runtime error is thrown.

---

### Medium priority issues

<a id="FUNC-M1"></a>

#### FUNC-M1 — Named nested functions do not capture enclosing function scope

**Affected files:**

- `compiler/function.go` — compilation of named function declarations

**Description:**  
Ego allows named functions to be declared inside other named functions, which
Go does not. However, Ego's nested named functions behave differently from
Go's nested closures: they cannot see the parameters or local variables of the
enclosing named function. Only anonymous function literals (closures) capture
the enclosing scope.

```go
func outer(a int) int {
    func inner(b int) int {
        return a + b   // ERROR in Ego: unknown identifier: a
    }
    return inner(5)
}

// But closures work:
func outer(a int) int {
    adder := func(b int) int {
        return a + b   // OK: closure captures 'a'
    }
    return adder(5)
}
```

In Go, `func inner` inside `outer` is not legal syntax — you must use a
function literal. Ego's behavior of accepting but not capturing is therefore
surprising: user code that is syntactically valid in Ego but semantically
broken.

**Test file:** `tests/functions/scope_advanced.ego` — tests
`"functions: nested named funcs do not share scope"` and
`"functions: closure captures named func parameter"` document the distinction.

**Recommendation:**  
Either (a) make nested named function declarations capture the enclosing scope
(consistent with the expectations set by function literals), or (b) produce a
compile-time error when a nested named function references a variable from the
enclosing named function's scope. Option (b) is safer and maintains the current
semantics; option (a) is more consistent with user expectations.

**Resolution (May 2026):**  
Option (b) was implemented: a compile-time error is now emitted when a nested
named function body references a parameter or local variable that belongs to an
enclosing named function. The error message guides the user to use a closure
instead: `"nested named function cannot access enclosing function variable; use
a closure"`.

Implementation:

- **`errors/messages.go`**: Added `ErrNestedFunctionScope` (`"nested.function.scope"`)
  with corresponding translations in all three language files.

- **`compiler/compiler.go`**: Added three fields to `Compiler`:
  - `functionLocalScopeStart int` — index in `c.scopes` where the current
    function's own body scopes begin; copied by `Clone`.
  - `ownParamNames map[string]bool` — this function's own parameter names;
    stored separately because `parseParameterDeclaration` adds them to the
    outer compiler's scope before the function body clone is created.
  - `forbiddenSymbols map[string]bool` — names from the immediately enclosing
    named function that produce a compile error if referenced inside this nested
    named function.
  - `Clone` propagates `forbiddenSymbols` and `ownParamNames` to clones so that
    the expression-eval clone in `compiler/expression.go` also enforces the
    boundary.

- **`compiler/function.go` — `generateFunctionBytecode`**: For each non-literal
  (named) function, before the parameter-assignment loop, the function builds a
  `forbiddenForNested` map from the outer function's own param names
  (`c.ownParamNames`) plus any body-scope variables above
  `c.functionLocalScopeStart` that are not globally accessible. Inner function
  own-params are excluded (they are known upfront via the `parameters` slice
  because `parseParameterDeclaration` already added them to `c.scopes`). After
  the clone, `cx.forbiddenSymbols`, `cx.ownParamNames`, and
  `cx.functionLocalScopeStart` are set on the new compiler.

- **`compiler/symbols.go` — `validateSymbol`**: The forbidden-symbols check now
  runs **before** the scope search, not after. Without this ordering, outer
  locals (still present in `cx.scopes` — no truncation) would be silently found
  by the search and accepted.

Closures (function literals, `isLiteral == true`) never receive a
`forbiddenSymbols` assignment, so they continue to see the full enclosing scope
chain, consistent with Go behavior.

Tests in `tests/functions/scope_advanced.ego` cover both directions:
`"functions: nested named funcs do not share scope"` confirms that inner's own
parameters and globals are accessible, while `"functions: closure captures named
func parameter"` confirms that a closure can still see an enclosing named
function's parameters.

---

<a id="FUNC-M2"></a>

#### FUNC-M2 — Value receiver method cannot be called on a pointer variable

**Affected files:**

- `bytecode/` — method dispatch
- `data/` — type handling for pointer and value types

**Description:**  
In Go, if a method is declared with a value receiver (`func (p Pair) Sum() int`),
it can be called on both a value and a pointer to that value — Go automatically
dereferences the pointer:

```go
pp := &Pair{a: 3, b: 7}
pp.Sum()   // Go: valid, auto-dereferences to (*pp).Sum()
```

In Ego, calling a value receiver method on a pointer variable produces a
runtime error `"invalid or unsupported data type for this operation"`:

```go
pp := &Pair{a: 3, b: 7}
pp.Sum()   // Ego ERROR: invalid or unsupported data type
```

Note: the reverse case (calling a pointer receiver method on a value variable)
works in Ego via auto-addressing. It is only value receiver + pointer variable
that fails.

**Test file:** `tests/functions/receivers.ego` — test
`"functions: value receiver on pointer var errors"` documents this behavior.

**Recommendation:**  
In the method dispatch path, detect when the receiver is a pointer type and the
method is declared for the base (non-pointer) type. In this case, auto-deref
the pointer before dispatching, consistent with Go's method set rules.

**Resolution (May 2026):**  
One change to `bytecode/this.go`:

- **`getThisByteCode`**: After popping the receiver from the "this" stack, if
  the value is `*any` (the runtime representation of an Ego pointer created with
  `&`), it is automatically dereferenced to the underlying value before being
  stored in the receiver variable. This is a pure runtime fix — no compiler
  changes were needed. The dynamic nature of Ego means the same runtime path
  handles both value receivers and pointer receivers, and whether the caller
  passed a pointer or a value variable is only known at runtime.

  For value receivers (`byValue = true`), the dereferenced `*data.Struct` is
  then passed to `$new` for copying, which already had a handler for
  `*data.Struct`. For pointer receivers (`byValue = false`), the dereferenced
  `*data.Struct` is a Go pointer, so field writes inside the method still
  propagate to the original struct. Both paths now work correctly.

Test updated in `tests/functions/receivers.ego`: the test previously named
`"functions: value receiver on pointer var errors"` is renamed to
`"functions: value receiver called on pointer var"` and now asserts that
`pp.Sum()` returns `10` instead of asserting that a runtime error is thrown.

---

<a id="FUNC-M3"></a>

#### FUNC-M3 — Dynamic mode silently accepts wrong-type arguments

**Affected files:**

- `compiler/function.go` — argument type checking
- `bytecode/coerce.go` — runtime coercion

**Description:**  
In Go, passing a value of the wrong type to a function parameter is always a
compile-time error. In Ego's dynamic mode (the default), type mismatches at
function call boundaries are not rejected. The argument retains its actual type
inside the function body, which can produce silently wrong results.

The most common case is passing a string where an integer is expected:

```go
func double(n int) int { return n * 2 }
double("5")   // No error in dynamic mode
// Inside double: n is still a string "5"
// "5" * 2 = "55" (string repetition — see FUNC-L1)
```

The result is neither the expected integer `10` nor an error — it is the string
`"55"` silently coerced into the `int` result variable.

In strict mode (`ego test --types=strict`), the mismatch is caught as a runtime
type error.

**Test file:** `tests/functions/arg_types.ego` — test
`"functions: dynamic mode string to int no error"` documents this behavior.

**Recommendation:**  
In dynamic mode, add an optional warning (controllable by a setting) when a
value of a clearly incompatible type is silently accepted for a statically-typed
parameter. Alternatively, promote this check from strict-only to a standard
runtime check, and only bypass it in a more permissive "relaxed" mode.

**Resolution (May 2026):**  
Two changes:

- **`data/types.go`**: Added `IsCoercible(*Type) bool`, which returns `true` for
  scalar types (bool, all integer widths, both float widths, and string). Complex
  types (struct, array, map, channel, function, pointer) return `false` and are
  never silently coerced.

- **`bytecode/arg.go` — `argByteCode`**: After all existing type-guard checks,
  if the expected parameter type is coercible and the argument's type does not
  already match, `data.Coerce` is called to convert the value before it is stored
  in the function's local symbol table. On failure (e.g., `"abc"` where `int` is
  expected), a descriptive error including argument position and original value is
  returned — and because it goes through the normal runtime error path, `try/catch`
  can catch it.

  As a result, `double("5")` now returns `10` instead of `"55"`. Passing a
  non-coercible value such as `double("abc")` raises a catchable runtime error
  rather than producing a silently wrong result.

The previously-used test `"functions: dynamic mode string to int no error"` was
replaced by `"functions: type mismatch is always catchable"` which covers both
the success case (coercible string) and the error case.

---

### Low priority issues

<a id="FUNC-L1"></a>

#### FUNC-L1 — String multiplication is asymmetric

**Affected files:**

- `bytecode/mul.go` (or equivalent arithmetic opcode handler)

**Description:**  
Ego supported string repetition via the `*` operator when the string was the
**left** operand:

```go
"A" * 3   // → "AAA"   (string repetition, former behavior)
3 * "A"   // → ERROR   (numeric multiplication fails on string)
```

This was documented as intentional (similar to Python), but the asymmetry was
confusing and combined badly with FUNC-5: `double("5")` where `double` expects
`int` silently produced `"55"` (string repetition) rather than `10`.

Additionally, the function `double("5")` where `double` expects an `int`
produces `"55"` (string repetition) rather than `10` (arithmetic) in dynamic
mode — a consequence of FUNC-M3 combined with this behavior.

**Test file:** `tests/functions/arg_types.ego` — tests
`"functions: string times int is string repetition"` and
`"functions: int times string is an error"` document the current behavior.

**Recommendation:**  
The string-repetition shortcut was already listed as informational with no
required action. Given that FUNC-5 has been resolved (coercion at call
boundaries now produces correct results), the string-repetition shortcut became
unnecessary and its asymmetry was a source of confusion. Removing it simplifies
the operator model.

**Resolution (May 2026):**  
The string-repetition special case was removed from `bytecode/math.go`:

- The `multiplyByteCode` handler no longer checks for a `(string, numeric)` pair
  and no longer calls `strings.Repeat`. Both `"A" * 3` and `3 * "A"` now
  produce a runtime error (`"invalid or unsupported data type"`), consistent with
  Go's behavior. The `strings.Repeat` function remains available for the
  intentional use case.

The test `"functions: string times int is string repetition"` was removed from
`tests/functions/arg_types.ego`. The test `"functions: int times string is an
error"` was retained (its stale comment about the left-operand exception was
removed).

The unit test `"multiply strings"` was removed from `bytecode/math_test.go`.

---

<a id="FUNC-L2"></a>

#### FUNC-L2 — Named return with explicit return value is a compile error

**Affected files:**

- `compiler/return.go` — return statement compilation

**Description:**  
In Go, a function with named return variables can mix bare returns (`return`)
and explicit-value returns (`return someValue`):

```go
func clamp(x, lo, hi int) (result int) {
    if x < lo { return lo }   // explicit value with named return — valid in Go
    if x > hi { return hi }
    result = x
    return                    // bare return — valid in Go
}
```

In Ego, using an explicit return value expression inside a function declared
with named returns is a compile error:

```go
func clamp(x, lo, hi int) (result int) {
    if x < lo { return lo }   // Ego compile error
    ...
}
```

Users must assign to the named return variable and then use a bare `return`:

```go
if x < lo { result = lo; return }
```

Bare returns and defErred modifications to named returns both work correctly.

**Test file:** `tests/functions/named_returns.ego` covers the working cases
(bare return, zero values, early return paths). The explicit-value form is
excluded from tests because it does not compile.

**Recommendation:**  
In the return statement compiler (`compiler/return.go`), when the current
function has named return variables and an explicit return value expression
is provided, assign the expression to the named return variable (if there is
exactly one) or to all named return variables in order (if there are multiple)
and then proceed as a bare return. This is consistent with Go's semantics.

**Resolution (May 2026):**  
`compiler/return.go` — `compileReturn` restructured:

1. **Explicit-value path added**: when the function has named return variables
   and the return statement is not at a statement end, each expression is
   parsed, coerced via `c.coercions[i]`, and stored into the corresponding
   named return variable with `Store`. Too many or too few values produce
   `ErrReturnValueCount` / `ErrMissingReturnValues` respectively.

2. **`RunDefers` moved inside the named-return block**, placed *after* any
   explicit assignments. This matches Go semantics: the named variable receives
   the explicit value first; defErred functions then run and may read or modify
   it; the final value is loaded and returned. The previous bare-return
   behavior is unchanged — when no explicit values are present the assignment
   loop is skipped and `RunDefers` still fires before the loads.

3. **`ErrInvalidReturnValues` check removed** — the error that previously
   rejected any token after the `Return` instruction in the named-return branch
   is deleted.

Three new tests added to `tests/functions/named_returns.ego`:

- `"functions: single named return with explicit value"` — the `clamp` pattern
  from the issue description, exercising all three return paths in one function
- `"functions: two named returns with explicit values"` — multi-value explicit
  return via `split`
- `"functions: named return explicit value interacts with defer"` — verifies
  that a defErred closure observes the post-assignment value of the named
  variable (`f(5)` returns `12`, not `6`)

---

<a id="area-flow-functional"></a>

## FLOW (Functional) — Flow Control

| ID | Priority | Summary | Status |
| -- | -------- | ------- | ------ |
| [FLOW-H1](#FLOW-H1) | High | `for range` over an array marked it read-only, so writes through the index inside the loop body failed. | ✓ |
| [FLOW-M1](#FLOW-M1) | Medium | The `for` init clause only accepted `:=`, rejecting `=` assignment to a pre-declared variable. | ✓ |
| [FLOW-M2](#FLOW-M2) | Medium | Multi-value `case v1, v2:` clauses were not supported and produced a spurious compile error. | ✓ |
| [FLOW-M4](#FLOW-M4) | Medium | `defer namedFunc(arg)` evaluates its argument lazily at run time instead of eagerly at registration time. | ✓ |
| [FLOW-L1](#FLOW-L1) | Low | Labeled `break`/`continue` were not supported. | ✓ |
| [FLOW-L2](#FLOW-L2) | Low | The `switch init; expr` semicolon-separated form was not supported. | ✓ |

### High priority issues

<a id="FLOW-H1"></a>

#### FLOW-H1 — `for range` over an array iterates an immutable snapshot; mutation fails

**Affected files:**

- `bytecode/range.go` — `rangeInitByteCode`, `rangeNextArray`

**Description:**  
In Go, a `for i, v := range a` loop iterates over the array `a` directly and it is
perfectly legal to modify elements through the index during the loop body:

```go
a := []int{1, 2, 3}
for i := range a {
    a[i] *= 10     // modifies original; result: [10, 20, 30]
}
```

In Ego, `rangeInitByteCode` called `actual.SetReadonly(true)` on the original array
before iterating. Because the symbol `a` in the Ego symbol table pointed to the same
`*data.Array` object, any `a[i] = val` inside the loop body reached
`array.Set()` → `if a.immutable > 0 { return ErrImmutableArray }` → runtime error.

A secondary bug existed: if the loop was exited early via `break`,
`rangeNextArray`'s cleanup (`SetReadonly(false)`) was never called, leaving the
array permanently immutable for the rest of the function scope.

**Test file:** `tests/flow/for_range_advanced.ego` — test
`"flow: range over array allows element mutation"` verifies the fixed behavior, and
`"flow: range mutation and direct index loop produce same result"` confirms both loop
forms produce identical results.

**Resolution (May 2026):**  
Two lines removed from `bytecode/range.go`:

1. **`rangeInitByteCode` (line 89):** Removed `actual.SetReadonly(true)` from the
   `*data.Array` case. Ego arrays are fixed-size — there is no structural-modification
   hazard to guard against. Element writes inside the loop body now succeed, matching
   Go's slice range semantics.

2. **`rangeNextArray` (line 195):** Removed the corresponding `actual.SetReadonly(false)`
   call from the loop-exhaustion branch. Since the array is never marked immutable at
   range start, there is nothing to restore. This also eliminates the secondary bug
   where a `break` out of a range loop left the array permanently immutable.

The `immutable` counting semaphore on `data.Array` and the `SetReadonly` method are
unchanged; they continue to serve other legitimate read-only use cases (`_`-prefixed
variables, server runtime arrays, etc.).

---

### Medium priority issues

<a id="FLOW-M1"></a>

#### FLOW-M1 — `for` init clause only accepts `:=`; `=` for existing variables fails — **Resolved**

**Affected files:**

- `compiler/for.go` — for-statement parsing

**Description:**  
In Go, the init clause of a C-style `for` loop accepts either `:=` (declare a new
variable) or `=` (assign to an existing variable):

```go
var i int
for i = 0; i < 10; i++ { ... }  // Go: valid; i retains its value after the loop
```

The LANGUAGE.md guide documents this pattern explicitly and provides a code example.
In Ego, the init clause only accepted `:=`; using `=` for a pre-declared variable
produced a compile error:

```go
var i int
for i = 0; i < 10; i++ {}   // Ego: ERROR "missing ':='"
```

**Resolution:**  
Fixed in `compiler/for.go` — `IsNext(DefineToken)` changed to
`AnyNext(DefineToken, AssignToken)` at line 93. The `assignmentTarget()` function
already handled both forms correctly; only the guard that rejected `=` needed removal.
Test added: `"flow: for-loop init clause with assignment to existing variable"` in
`tests/flow/while_loop.ego`.

---

<a id="FLOW-M2"></a>

#### FLOW-M2 — Multi-value `case` clause not supported — **Resolved**

**Affected files:**

- `compiler/switch.go` — `compileSwitchCase`

**Description:**  
In Go, a `switch` case clause can list multiple comma-separated values:

```go
switch day {
case "Sat", "Sun":
    fmt.Println("weekend")
default:
    fmt.Println("weekday")
}
```

In Ego, `compileSwitchCase` called `c.emitExpression()` once and then immediately
expected a `:` token. If a `,` followed the first value, the colon check failed
silently and the compiler subsequently reported a spurious `"missing 'case'"` error.

**Resolution:**  
Fixed in `compiler/switch.go` — `compileSwitchCase` now loops on `CommaToken` after
the first expression, emitting `Equal` + `Or` per additional value so that the case
matches when the switch expression equals any listed value. Works for both value
switches and conditional switches. Tests added: `"flow: switch case with multiple
comma-separated values"` and the existing `"flow: switch on string value"` test was
updated to use `case "Sat", "Sun":` directly.

---

<a id="FLOW-M4"></a>

#### FLOW-M4 — `defer namedFunc(arg)` evaluates arguments lazily, not eagerly

**Status:** Fixed.

**Affected files:**

- `compiler/defer.go` — defer statement compilation
- `bytecode/defer.go` (or equivalent) — argument capture at defer registration time

**Description:**  
In Go, the arguments to any defErred function call — named function or closure —
are evaluated immediately when the `defer` statement executes:

```go
x := "first"
defer fmt.Println(x)   // captures "first" NOW
x = "second"
// Output: "first"
```

In Ego, the closure-with-immediate-invocation form `defer func(p T){...}(arg)`
correctly captures `arg` at registration time. However, `defer namedFunc(arg)` does
**not** capture `arg` eagerly; the argument is re-read from the symbol table when
the defErred function actually runs:

```go
x := "first"
defer setLog(x)   // Ego: x is read lazily when defer runs
x = "second"
setLog is called with "second", not "first"
```

This silent behavioral difference can produce hard-to-diagnose bugs when the
variable changes between the `defer` statement and the function's return.

**Workaround (no longer required, kept for historical reference):**  
Use the closure form with an explicit argument to get Go-compatible eager capture:

```go
x := "first"
defer func(v string) { setLog(v) }(x)   // captures "first" eagerly
x = "second"
// setLog receives "first"
```

**Test file:** `tests/flow/defer_lifo.ego` — tests `"flow: closure arg captured at
defer time (eager)"` and `"flow: named function defer captures arg eagerly"` document
both forms now behaving identically, plus additional coverage described in the
Resolution section below.

**Recommendation (superseded by Resolution below):**  
When compiling `defer namedFunc(arg)`, evaluate and snapshot all argument expressions
at the point of the `defer` statement and store them in a local temporary, exactly
as is done for the closure-invocation form. This would make both forms consistent
with Go and with each other.

**Resolution:**  
Fixed exactly along the lines of the recommendation above, entirely in
`internal/language/compiler/defer.go`. No changes were needed in the runtime
(`internal/language/bytecode/defer.go`, `context.go`, or the `deferStatement`
struct) — that plumbing already correctly stores and replays a list of
pre-computed argument values; it was only ever fed those values *lazily* for
the named-function form.

**How `defer namedFunc(arg)` was compiled before the fix:** `compileDefer()`
rewrites any deferred call that isn't already a `func(){...}()` literal into
one, so `defer namedFunc(arg)` was rewritten to `defer func(){ namedFunc(arg) }()`
*before* being compiled. Because the closure's body is compiled once but only
*runs* later (when the deferred call actually executes), the token `arg`
inside that body was compiled as an ordinary variable reference — a `Load`
instruction — that reads whatever `arg` holds from the live, captured symbol
table at the time the closure finally runs. That is what made the argument
"lazy": the expression that computes its value was physically inside the
deferred closure, not in the compiler's normal (immediately-executing)
bytecode stream.

**The fix:** a new function, `hoistDeferCallArguments()`, runs *before* the
existing `func(){...}()` rewrite. For a call like `namedFunc(arg1, arg2)`, it:

1. Locates the call's argument list in the token stream (reusing the same
   "skip the identifier/dot chain" logic `findDeferCallEnd()` already used,
   factored out into a small sibling helper, `findDeferCallArgsStart()`).
2. Compiles each argument expression, left to right, using the compiler's
   normal expression grammar (`c.conditional()` — the same entry point an
   ordinary function call's argument list uses). Because this happens in the
   compiler's current (non-deferred) bytecode stream, each argument's value is
   computed right now, at the `defer` statement's point in the program — this
   is the crux of the fix.
3. Immediately stores each computed value into its own compiler-generated
   temporary variable (`data.GenerateName()` + a `StoreAlways` instruction —
   the same "materialize this expression under a private name" idiom already
   used by `compileAddressOf`/`compilePointerDereference` for `&expr`/`*expr`).
4. Rewrites the token stream so the call now reads `namedFunc($1, $2)` instead
   of `namedFunc(arg1, arg2)`, using the tokenizer's existing `Delete`/`Insert`
   primitives (the same primitives the surrounding `func(){...}()` rewrite
   already relied on).

The existing `func(){...}()` wrap then proceeds completely unchanged, now
wrapping `namedFunc($1, $2)`. When the deferred closure eventually runs, `$1`
and `$2` are simple variable *reads* of values that were already computed and
frozen — there is nothing left to re-evaluate, so the bug cannot recur.

Two details worth calling out:

- **Why this doesn't defer the call itself late enough to matter:** the
  callee (and, for a method call, its receiver) are still resolved only when
  the closure finally runs — exactly as before. Only the *arguments* moved
  earlier. This preserves the reason the closure-wrap exists in the first
  place (so a deferred method call's receiver setup is itself deferred; see
  `DEFER-1`/`DEFER-2`).
- **`$`-prefixed temporary names need no special scoping:** `DefineSymbol`/
  `ReferenceSymbol` (`compiler/symbols.go`) already special-case any name
  starting with `$` and never flag it as unused or undeclared, so the nested
  closure body's reference to `$1` resolves through the same "read a captured
  outer-scope variable at run time" mechanism any ordinary closure already
  uses — no new symbol-table plumbing was required.
- **Variadic spread (`defer f(s...)`) is handled correctly too:** the slice
  value itself is hoisted and frozen (matching Go, where a slice is a
  reference-type value — reassigning the variable afterward to point at a
  *different* slice does not affect the deferred call, but mutating the
  *same* underlying array would still be visible, exactly as in Go).

**Tests:**

- Go unit tests in `internal/language/compiler/defer_test.go`: new table
  cases for one argument, multiple arguments, a variadic argument, and a
  dotted method call, plus a case confirming a malformed argument expression
  is still correctly reported as a compile error. A dedicated
  `TestCompiler_compileDefer_ArgumentsHoistedEagerly` inspects the actual
  emitted bytecode shape (`DeferStart` → `Push` the argument value → `StoreAlways`
  the temp → `Push` the closure → `Defer`) and confirms the closure body loads
  the temp variable by name rather than containing any trace of the original
  literal argument value.
- Ego-language tests in `tests/flow/defer_lifo.ego`: the pre-existing test
  that had documented the buggy lazy behavior as expected
  (`"flow: named function defer evaluates arg lazily"`) was rewritten to
  assert the correct eager behavior and renamed
  `"flow: named function defer captures arg eagerly"`. New tests were added
  for: a named-function defer inside a loop capturing each iteration's value
  without needing a manual closure workaround; multiple arguments; an
  arithmetic expression argument; a value-receiver method call; and a
  variadic spread argument.

---

### Low priority issues

<a id="FLOW-L1"></a>

#### FLOW-L1 — Labeled `break` and `continue` ✓ FIXED

**Fixed in:** `compiler/compiler.go`, `compiler/for.go`, `compiler/statement.go`

**Description:**  
Labeled `break` and `continue` are now supported. A label is an identifier
immediately followed by `:` placed before a `for` statement (on the same line
or on the preceding line):

```go
outer:
for i := 0; i < 3; i++ {
    for j := 0; j < 3; j++ {
        if j == 1 { break outer }
    }
}
```

Both `break label` and `continue label` find the nearest enclosing loop with
that label and target it. Using an unknown label is a compile-time error.

**Test file:** `tests/flow/labeled_break.ego`

---

<a id="FLOW-L2"></a>

#### FLOW-L2 — `switch init; expr` semicolon-separated form not supported

**Affected files:**

- `compiler/switch.go` — `compileSwitchAssignedValue`

**Description:**  
In Go, a switch statement can separate the init statement from the switch expression
with a semicolon:

```go
switch x := compute(); x > 0 {   // init-statement ; condition
case true: ...
case false: ...
}
// or for a value switch:
switch x := getCode(); x {
case 1: ...
case 2: ...
}
```

In Ego, the `switchInit` production accepts either an assignment OR an expression,
but not both separated by a semicolon. The Go form `switch x := f(); x { ... }`
produces a compile error `"missing '{}'"`. The Ego-supported form is
`switch x := f() { ... }` where the init variable becomes the switch value
automatically.

**Workaround:**  
Use a preliminary assignment before the switch:

```go
x := compute()
switch x {
case 1: ...
}
// or the Ego init-only form (init is also the switch value):
switch x := compute() {
case 1: ...
}
```

**Test file:** `tests/flow/switch_advanced.ego` — comment at the top of the file
references this issue; `"flow: switch with init assignment"` demonstrates the
supported Ego init-only form.

**Recommendation:**  
In `compileSwitchAssignedValue`, after parsing the first clause, check for a
semicolon token and, if present, parse a second expression as the switch value.
This would make `switch x := f(); x { ... }` a valid Ego form. Low priority since
the Ego init-only form and the pre-assignment workaround cover most use cases.

**Resolution (May 2026):**  
One change to `compiler/switch.go` — `compileSwitchAssignedValue`:

After `emitExpression()` returns for the first (init) clause, the function now
checks for a `SemicolonToken`. If found, it takes one of two paths:

- **Named init** (`switch x := f(); expr`): stores the first expression result
  under the declared variable name via `DefineSymbol` + `CreateAndStore`, making
  `x` available in the second expression and in case bodies. Then generates a new
  synthetic name, emits the second expression, and stores it as the actual switch
  test value.

- **Anonymous init** (`switch f(); expr`): discards the first expression result
  with `Drop` (side effects still execute), then emits the second expression and
  stores it under the originally-generated synthetic name.

When no semicolon is present, the existing single-expression paths are unchanged.
The `hasScope` flag continues to control whether `PopScope` or `SymbolDelete` is
emitted at the end of the switch — both forms still clean up correctly because in
the named case both variables live inside the pushed scope.

Three new tests added to `tests/flow/switch_advanced.ego`:

- `"flow: switch semicolon form with same variable as switch value"` — verifies
  `switch code := getCode(); code { case 3: ... }` selects the correct branch.
- `"flow: switch semicolon form with boolean condition"` — verifies
  `switch n := getValue(); n > 0 { case true: ... }` with a derived boolean.
- `"flow: switch semicolon init var visible in case body"` — verifies that the
  init variable `code` is readable inside the matching case body.

---

<a id="area-json"></a>

## JSON — JSON package

Issues discovered during a review of `runtime/json/` and `tests/json/` in May 2026.
The implementation wraps Go's `encoding/json` and the `jaxon` path-query library.

| ID | Priority | Summary | Status |
| -- | -------- | ------- | ------ |
| [JSON-H1](#JSON-H1) | High | `json.Unmarshal` into a struct stored a nested JSON object as a raw Go map instead of the declared Ego struct type. | ✓ |
| [JSON-H2](#JSON-H2) | High | `json.Unmarshal` type-mismatch errors surfaced as an uncatchable-by-default exception rather than the function's return value. | ✓ |
| [JSON-M1](#JSON-M1) | Medium | `json.Marshal` of a `[]byte` value produced `"null"` instead of a base64-encoded JSON string. | ✓ |
| [JSON-M2](#JSON-M2) | Medium | `json.Parse("[]", ".")` returned an error while the equivalent `json.Parse("{}", ".")` succeeded. | ✓ |
| [JSON-M3](#JSON-M3) | Medium | `json.Unmarshal` into a struct returned an error for unknown JSON fields instead of ignoring them (as Go does). | ✓ |

### High priority issues

<a id="JSON-H1"></a>

#### JSON-H1 — Unmarshal nested struct stores raw Go map instead of Ego struct

**Affected files:**

- `runtime/json/unmarshal.go` — `remapDecodedValue`, `case *data.Struct`

**Description:**
When `json.Unmarshal` writes to an Ego struct and the JSON contains a nested
object, the nested object is stored in the struct field as a raw
`map[string]interface{}` (a Go native map) rather than as the corresponding
Ego struct type that the field was initialized with.

```go
inner := {x: 0, y: 0}
outer := {label: "", pt: inner}
err := json.Unmarshal([]byte(`{"label":"origin","pt":{"x":1,"y":2}}`), &outer)
// outer.pt is now map[string]interface{}, not {x, y}
// reflect.Type(outer.pt) == interface{}   ← not the struct type
```

The root cause is that `remapDecodedValue` iterates over the decoded JSON map
and calls `target.Set(k, v)` with the raw Go value `v` for each key. It does
not check whether the field's declared type is a struct and does not recursively
convert the nested map to that struct type.

Go's `encoding/json` handles this by using reflection on typed struct fields.
Ego's implementation would need to inspect the type of each struct field (via
`target.FieldTypes()` or similar) and call `data.NewStructOfTypeFromMap` when
the field type is a struct kind.

**Test file:** `tests/json/unmarshal.ego` —
`"json: Unmarshal - nested struct stores raw map"` documents this behavior.

**Recommendation:**
In `remapDecodedValue` under `case *data.Struct`, for each key-value pair look
up the declared field type. If the field type is `StructKind` and the decoded
value is `map[string]any`, call `data.NewStructOfTypeFromMap(fieldType, m)` to
create a proper Ego struct before calling `target.Set(k, v)`. This is the same
pattern already applied to struct elements in the `case *data.Array` path.

**Resolution (May 2026):**
One change to `runtime/json/unmarshal.go` — `remapDecodedValue`, `case *data.Struct`:

Inside the `for k, v := range m` loop, before the `target.Set(k, v)` call, a
type-check was added:

```go
if mm, ok := v.(map[string]any); ok {
    if fieldType, fErr := target.Type().Field(k); fErr == nil && fieldType.Kind() == data.StructKind {
        v = data.NewStructOfTypeFromMap(fieldType, mm)
    }
}
```

`target.Type().Field(k)` (`data/types.go:1299`) returns the declared `*data.Type`
for field `k`. When its kind is `StructKind` and the decoded JSON value is a
`map[string]any`, `data.NewStructOfTypeFromMap` converts the raw map into a
properly-typed Ego struct before it is stored. Fields whose declared type is not
`StructKind` (scalars, arrays, maps) fall through unchanged. Unknown field names
produce a non-nil `fErr` and also fall through, letting the existing
`target.Set(k, v)` error path handle them (covered separately by JSON-M3).

This is the same pattern already used in the `case *data.Array` branch (lines
123–126 before this change).

Test in `tests/json/unmarshal.ego` renamed from
`"json: Unmarshal - nested struct stores raw map"` to
`"json: Unmarshal - nested struct field is correct type"` and updated to assert
that `outer.pt.x == 1` and `outer.pt.y == 2` after unmarshal rather than
asserting the buggy `interface{}` type.

---

<a id="JSON-H2"></a>

#### JSON-H2 — Unmarshal type mismatch raises exception instead of returning error

**Affected files:**

- `runtime/json/unmarshal.go` — `remapDecodedValue`, `default` case

**Description:**
When the JSON value cannot be coerced to the destination type (for example,
the JSON string `"notanumber"` decoded into an `int` variable), the error does
not come back as the return value of `json.Unmarshal`. Instead it is raised as
a catchable exception that bypasses the normal return path.

```go
var i int
err := json.Unmarshal([]byte(`"notanumber"`), &i)
// err is nil -- the error did NOT come back here
// A try/catch block would catch the exception instead
```

The root cause is that the `default` case of `remapDecodedValue` returns
`(nil, err)` — not a `data.List` — when `data.Coerce` fails. Because
`json.Unmarshal` is declared with a single `error` return type and its wrapper
returns a non-list result with a non-nil Go error, the runtime takes the
special single-error-return path in `callRuntimeFunction`, which routes the
error as a catchable exception rather than the Ego-level return value.

In Go, `json.Unmarshal` always returns the error normally; callers use
`if err := json.Unmarshal(...); err != nil {}`.

**Test file:** `tests/json/unmarshal.ego` —
`"json: Unmarshal - type mismatch is catchable exception"` documents this
behavior.

**Recommendation:**
In the `default` case of `remapDecodedValue`, change the error return from
`return nil, err` to `return data.NewList(err), nil`. This ensures the error
flows through the `data.List` path and becomes the Ego-level return value of
`json.Unmarshal`, consistent with all other error paths in the function.

**Resolution (May 2026):**
One change to `runtime/json/unmarshal.go` — `remapDecodedValue`, `default` case:

The `data.Coerce` failure path was:

```go
if err != nil {
    return nil, err
}
```

Changed to:

```go
if err != nil {
    return data.NewList(errors.New(err).In("Unmarshal")), nil
}
```

Because `json.Unmarshal` is declared with a single `error` return type, returning
`(nil, err)` (a non-list result with a non-nil Go error) caused `callRuntimeFunction`
to take the special single-error-return path: the error became a catchable exception
rather than the Ego-level return value of `Unmarshal`. Wrapping it in `data.NewList`
forces the list path, where the Go second return is ignored and the error inside the
list becomes what the caller sees as the `error` return of `json.Unmarshal`.

Test in `tests/json/unmarshal.ego` renamed from
`"json: Unmarshal - type mismatch is catchable exception"` to
`"json: Unmarshal - type mismatch returns error"` and simplified: the `try/catch`
wrapper was removed. The test now calls `err := json.Unmarshal(...)` directly and
asserts `err != nil`, matching Go's `encoding/json` behavior.

---

### Medium priority issues

<a id="JSON-M1"></a>

#### JSON-M1 — Marshal of []byte produces "null" instead of base64 string

**Affected files:**

- `data/sanitize.go` — `Sanitize` function, `*data.Array` of `ByteKind`

**Description:**
Go's `encoding/json` encodes a `[]byte` value as a base64-encoded JSON string.
In Ego, `json.Marshal([]byte{65, 66, 67})` produces `"null"` instead of
`"QUJD"`.

The cause is that `data.Sanitize` — which converts Ego runtime values to Go
native types before passing them to `json.Marshal` — does not convert a
`*data.Array` whose element type is `ByteKind` into a native Go `[]byte` slice.
The sanitized value ends up as `nil`, which `json.Marshal` encodes as `null`.

```go
byt := []byte{65, 66, 67}
b, err := json.Marshal(byt)
// Expected:  b == []byte(`"QUJD"`)
// Actual:    b == []byte("null")
```

This also affects `json.WriteFile` when given a byte array value to write.

**Test file:** `tests/json/marshal.ego` —
`"json: Marshal - []byte produces null"` documents the current (incorrect)
behavior.

**Recommendation:**
In `data.Sanitize`, add a case for `*data.Array` values whose `Type().Kind()`
is `data.ByteKind`: call `array.GetBytes()` and return the resulting `[]byte`
slice. This will allow `json.Marshal` to receive a native `[]byte` and produce
the standard base64 encoding.

**Resolution (May 2026):**
One change to `data/sanitize.go` — `Sanitize`, `case *Array`:

Before returning `v.data`, a `ByteKind` check was added:

```go
if v.Type().Kind() == ByteKind {
    return v.GetBytes()
}
```

`GetBytes()` returns the internal `[]byte` backing the byte array. Passing that
native slice to `json.Marshal` triggers Go's standard base64 encoding. For all
other array element types the existing `v.data` path is unchanged.

Test in `tests/json/marshal.ego` renamed from
`"json: Marshal - []byte produces null"` to
`"json: Marshal - []byte produces base64 string"` and updated to assert
`string(b)` equals the JSON string `"QUJD"` (the base64 encoding of `[65,66,67]` = `"ABC"`).

---

<a id="JSON-M2"></a>

#### JSON-M2 — Parse of empty JSON array with "." returns error

**Affected files:**

- External: `github.com/tucats/jaxon` — `GetItem` function

**Description:**
`json.Parse("[]", ".")` returns an error `"element not found: ."` rather than
a string representation of the empty array. By contrast, `json.Parse("{}", ".")`
returns `"{}"` without error.

```go
a, err := json.Parse("{}", ".")  // a == "{}", err == nil   ← correct
b, err := json.Parse("[]", ".")  // b == "",   err != nil   ← inconsistent
```

The inconsistency is in the `jaxon` library's handling of the root expression
`"."` applied to a JSON array vs. a JSON object. Jaxon returns the root object
representation for `{}` but fails for `[]`.

**Test file:** `tests/json/parse.ego` —
`"json: Parse - empty array root returns error"` documents this behavior.

**Recommendation:**
If this is a jaxon library limitation, a workaround could be applied in
`runtime/json/parse.go`: detect when `jaxon.GetItem` returns an "element not
found" error for a root expression `"."`, re-parse the input JSON, and if the
top-level value is an array return its string representation directly. This
avoids changing the external library.

**Resolution (May 2026):**
One change to `runtime/json/parse.go` — `parse`:

After `jaxon.GetItem` returns an error, a recovery block now checks whether
the expression is `"."` and the input is a valid JSON array:

```go
if err != nil && expression == "." {
    var root any
    if decodeErr := stdjson.Unmarshal([]byte(text), &root); decodeErr == nil {
        if _, ok := root.([]any); ok {
            if encoded, encErr := stdjson.Marshal(root); encErr == nil {
                return data.NewList(string(encoded), nil), nil
            }
        }
    }
}
```

`encoding/json` is imported as `stdjson` to avoid the naming collision with the
enclosing package. `json.Marshal([]any{})` returns `[]byte("[]")`, so
`json.Parse("[]", ".")` now returns `("[]", nil)`, consistent with
`json.Parse("{}", ".")` returning `("{}", nil)`.

Test in `tests/json/parse.ego` renamed from
`"json: Parse - empty array root returns error"` to
`"json: Parse - empty array root returns empty array string"` and updated to
assert `err == nil` and `result == "[]"`.

---

<a id="JSON-M3"></a>

#### JSON-M3 — Unmarshal into struct with unknown fields returns error

**Affected files:**

- `runtime/json/unmarshal.go` — `remapDecodedValue`, `case *data.Struct`

**Description:**
In Go, `json.Unmarshal` ignores JSON keys that do not correspond to any field
in the destination struct. In Ego, such keys cause `json.Unmarshal` to return
an error: `"invalid field name for type: <key>"`.

Additionally, because Go's `map` iteration order is non-deterministic, some
fields may already be written to the struct before the unknown key is
encountered. This produces a partial update with an error, which is both
unexpected and non-deterministic.

```go
person := {name: ""}
err := json.Unmarshal([]byte(`{"name":"Alice","ghost":"x"}`), &person)
// err != nil: "invalid field name for type: ghost"
// person.name may or may not be "Alice" depending on iteration order
```

**Test file:** `tests/json/unmarshal.ego` —
`"json: Unmarshal - unknown fields return error"` documents this behavior.

**Recommendation:**
In `remapDecodedValue` under `case *data.Struct`, check whether the key exists
as a field of the struct before calling `target.Set`. If the key does not exist,
silently skip it (matching Go's default behavior). If strict unknown-field
rejection is desired it can be added as a separate option or configuration
setting.

**Resolution (May 2026):**
One change to `runtime/json/unmarshal.go` — `remapDecodedValue`, `case *data.Struct`:

The loop now captures the bool result of `target.Get(k)` and skips the field when
not found, rather than discarding the found indicator and letting `target.Set` fail:

```go
existing, found := target.Get(k)
if !found {
    continue
}
```

`data.Struct.Get` returns `(value, bool)` — `false` for any key that is not a
declared field. The `continue` discards unknown keys silently, matching Go's
`encoding/json` default behavior. Known fields are still updated as before.

Test in `tests/json/unmarshal.ego` renamed from
`"json: Unmarshal - unknown fields return error"` to
`"json: Unmarshal - unknown fields are silently ignored"` and updated to assert
`err == nil` and `person.name == "Alice"`.

---

<a id="area-type"></a>

## TYPE — Type system

Issues discovered during investigation of JSON unmarshal correctness in May 2026.
These affect how Ego's runtime reconstructs typed values from raw Go values produced
by the standard `encoding/json` decoder.

| ID | Priority | Summary | Status |
| -- | -------- | ------- | ------ |
| [TYPE-H1](#TYPE-H1) | High | `json.Unmarshal` could not reconstruct deeply nested types or typed elements inside `[]any` arrays. | ✓ |

### High priority issues

<a id="TYPE-H1"></a>

#### TYPE-H1 — json.Unmarshal cannot reconstruct deeply nested or array-element types

**Affected files:**

- `runtime/json/unmarshal.go` — `remapDecodedValue`

**Description:**
Go's `encoding/json` decodes all JSON into a small set of native Go types:
`map[string]any`, `[]any`, `float64`, `string`, `bool`, and `nil`. Ego's
`remapDecodedValue` then attempts to convert that generic tree into typed Ego
values, guided by the destination pointer. The current implementation is a flat
`switch` on the destination's runtime type (`*data.Struct`, `*data.Map`,
`*data.Array`, scalar) with case-specific conversion logic at each level. This
approach has two related gaps:

**Gap 1 — Array elements are not converted when the element type is `interface{}`.**

The standard Ego idiom for an array of structs is `[]any{pt, pt}`, which gives the
array an element type of `InterfaceKind`. The existing `case *data.Array` code only
converts `map[string]any` elements to Ego structs when `target.Type().Kind() ==
data.StructKind`. For `[]any` the check is always false, so JSON array elements that
represent objects are stored as raw `map[string]any` rather than the struct values
the array already contained before the unmarshal call.

```go
pt := {x: 0, y: 0}
arr := []any{pt, pt}
err := json.Unmarshal([]byte(`[{"x":1,"y":2},{"x":3,"y":4}]`), &arr)
// arr[0] is map[string]interface{}, not {x:1, y:2}
```

The existing elements of `arr` before the call carry the struct type information
(they are `*data.Struct` values), but `SetSize` is called before they are examined,
and the conversion loop has no access to that type hint.

**Gap 2 — Nesting beyond one level is not handled.**

The fix for JSON-H1 converts a `map[string]any` value into a struct when the
containing struct's field type is `StructKind`. But if that nested struct itself has
a field that is an array of structs, or if a map value is a struct, those inner
conversions are not applied. Only one level of nesting is handled in any given
`case` branch.

**Root cause:**
The flat `switch`-based approach in `remapDecodedValue` is inherently limited to
one level of conversion per call. Arbitrarily deep nesting (arrays of structs
containing arrays of maps containing structs, etc.) requires a recursive traversal
of the decoded value tree guided by the destination object's type information at
every level.

**Recommendation:**
Redesign `remapDecodedValue` as a recursive function. The strategy:

1. Let `encoding/json` decode normally into a bare `any` (current behavior —
   no change here).
2. Replace the flat `switch` with a recursive `convertToEgoType(decoded any,
   targetType *data.Type) any` that walks both the decoded value tree and the
   declared type tree simultaneously:
   - If the target type is `StructKind` and the decoded value is `map[string]any`,
     create a new `*data.Struct` of that type and recursively convert each field
     value by looking up the field's declared type.
   - If the target type is `ArrayKind` and the decoded value is `[]any`, create a
     new `*data.Array` of the declared element type and recursively convert each
     element.
   - If the target type is `MapKind` and the decoded value is `map[string]any`,
     create a new `*data.Map` and recursively convert each value using the map's
     declared value type.
   - For scalar types, apply `data.Coerce` as today.
   - If the target type is `InterfaceKind`, apply a best-effort conversion: a
     `map[string]any` becomes an Ego `*data.Map`, a `[]any` becomes a `*data.Array`
     of `InterfaceType`, scalars are kept as-is. This preserves the current
     no-model behavior.
3. The entry point (`remapDecodedValue`) seeds the recursion with the declared type
   of the destination pointer's current value.

This approach handles arbitrary nesting depth, eliminates the per-type special
cases, and makes it straightforward to add future target types (e.g., typed
channels, user-defined types).

**Resolution (May 2026):**
`remapDecodedValue` was refactored and a new `reconstructValue(decoded any, model any) (any, error)`
helper was added in `runtime/json/unmarshal.go`.

**Design:** Instead of a `*data.Type`-based recursive target type, the helper takes the
*existing Ego value* at each position as the `model` and uses its concrete Go type (via a
`switch` on the model) to drive conversion. This sidesteps the need for an unexported
`ValueType()` accessor on `*data.Type` and works naturally with Ego's runtime representation:

- `model = *data.Struct` → creates a new `*data.Struct` of the same type, recurses into each field using `m.Get(fieldName)` as the per-field model.
- `model = *data.Array` → creates a new `*data.Array` with the same element type (`m.Type()`), uses the original element at each index (or the first element as a fallback for new slots) as the per-element model.
- `model = *data.Map` → creates a new `*data.Map` with the same key and value types; uses `InstanceOfType(m.ElementType())` as the per-value model (or `nil` when element type is `interface{}` to avoid spurious coercion).
- scalar / nil model → attempt `data.Coerce` to the model's type; fall back to best-effort (`map[string]any` → Ego map, `[]any` → Ego `[]any` array, scalars as-is).

**`remapDecodedValue` changes:**

- `case *data.Struct`: replaced the JSON-H1 single-level fix with `target.Get(k)` + `reconstructValue(v, existing)` for each field.
- `case *data.Array`: before `SetSize`, captures `origLen` and `elemModel` (first existing element). After resize, calls `reconstructValue(v, thisModel)` per element using the original element as the type hint.
- `case *data.Map`: uses `InstanceOfType(target.ElementType())` as `elemModel` (nil when interface); calls `reconstructValue(v, elemModel)` per value. The `default` case is unchanged (JSON-H2 tracked separately).

**Observable behavior change:** Unmarshaling a JSON array of objects into `[]any{}` (no struct model available) now produces Ego `*data.Map` elements instead of raw Go `map[string]any` values. The Ego type seen by callers changes from `interface{}` to `map[string]interface{}`. This is strictly better — callers can now index into the maps with normal Ego map syntax.

Test in `tests/json/unmarshal.ego`:

- `"json: Unmarshal - array of objects returns Go maps"` → renamed `"json: Unmarshal - array of objects into []any gives Ego maps"` and updated assertion from `interface{}` to `"map[string]interface{}"`.
- `"json: Unmarshal - array of structs restores element type"` added: verifies that a `[]any{pt, pt}` destination produces typed struct elements after unmarshal.

---

<a id="area-script"></a>

## SCRIPT — @transaction Scripting endpoint

Issues discovered during the creation of API tests for the `@transaction`
endpoint (`POST /dsns/{dsn}/tables/@transaction`) in May 2026. The handler
lives in `server/tables/scripting/handler.go`; individual opcodes are
implemented in sibling files (`rows.go`, `insert.go`, `drop.go`, etc.).

| ID | Priority | Summary | Status |
| -- | -------- | ------- | ------ |
| [SCRIPT-H1](#SCRIPT-H1) | High | The `readrows` opcode capped results at one row due to a copy-pasted `?limit=1` fragment. | ✓ |
| [SCRIPT-M1](#SCRIPT-M1) | Medium | `_rows_` was always 0 in user error-condition expressions after an `insert` opcode. | ✓ |
| [SCRIPT-M2](#SCRIPT-M2) | Medium | The `drop` opcode (and `sql ... DROP TABLE`) did not flush the schema cache, leaving stale column metadata. | ✓ |
| [SCRIPT-L1](#SCRIPT-L1) | Low | An empty transaction body returned a plain-text response with no `Content-Type` header. | ✓ |

### High priority issues

<a id="SCRIPT-H1"></a>

#### SCRIPT-H1 — `readrows` opcode returns at most one row

**Affected files:**

- `server/tables/scripting/rows.go` — `doRows`, `fakeURL` construction

**Description:**
The `readrows` opcode is intended to return all rows that match the specified
filters — an unbounded SELECT. However, `doRows` constructs its internal URL
with a hard-coded `?limit=1` parameter, which was copied from `doSelect` (the
`select` opcode handler) where limiting to one row is the correct behavior:

```go
fakeURL, _ := url.Parse("http://localhost/tables/" + task.Table + "/rows?limit=1")
```

Because `parsing.FormSelectorDeleteQuery` honours the `limit` query parameter
from the URL, this means the `readrows` opcode silently returns at most one row
regardless of how many rows match. Transactions that use `readrows` to
aggregate result sets will receive incomplete data with no error or warning.

The `sql` opcode with a raw `SELECT` statement is unaffected — it bypasses
`fakeURL` entirely and calls the SQL layer directly.

**Test file:** `tools/apitest/tests/4-dsns/dsns-304-tx-readrows.json` — the
test inserts three rows and then calls `readrows`. It expects `count=3`; if
this bug is present the test will fail with `count=1`.

**Recommendation:**
Remove the `?limit=1` fragment from the `fakeURL` in `doRows`, or replace it
with a sufficiently large sentinel value (e.g., `?limit=10000000`). The
`select` opcode (`doSelect`) should retain `?limit=1` since it is a
point-lookup by design. A cleaner solution is to add an explicit `limit` field
to `defs.TXOperation` and pass it through; the `readrows` default would be
unlimited.

**Resolution (May 2026):**
`doRows()` in `server/tables/scripting/rows.go` was modified to remove the
unwanted `?limit=1` parameter on the "readrows" operation. This is distinct
from "select" which intentionally reads a single row to set dictionary variables
to the values of each column. The "readrows" operator wants to read the entire
rowset into memory, as it will be the return data for the @transaction call.

Along the way, fixed an issue where the escape operation "/{{value}}" was poorly
chosen, as it interfered with the injection of dictionary values at the apitest
level into URL paths. The escape was changed to "\{{value}}" to avoid this.
APITEST streams were updated accordingly.

Also, augmented the FullTableName function to be told the provider name, so the
name path could be formed properly according to the provider. Specifically,
if the provider is sqlite3 then we can't and shouldn't use dotted names.

---

### Medium priority issues

<a id="SCRIPT-M1"></a>

#### SCRIPT-M1 — `_rows_` is always 0 for error conditions after an `insert` opcode

**Affected files:**

- `server/tables/scripting/handler.go` — main dispatch loop, `insertOpcode` case

**Description:**
When processing each opcode the handler maintains two variables that are
exposed to user-defined error condition expressions: `_rows_` (rows affected by
the current operation) and `_all_rows_` (cumulative rows affected so far).

For every opcode except `insert`, the handler captures the return value of the
opcode handler into `count` before evaluating error conditions:

```go
count, httpStatus, operationErr = doSelect(...)   // count updated
...
evalSymbols.SetAlways("_rows_", count)
```

The `insertOpcode` case does not update `count` — it increments `rowsAffected`
directly and ignores the return value of `doInsert`:

```go
case insertOpcode:
    httpStatus, operationErr = doInsert(...)
    rowsAffected++
    // count is never set — still 0 from declaration
```

Because `evalSymbols.SetAlways("_rows_", count)` is evaluated after the
`switch`, `_rows_` is always `0` for `insert` operations regardless of the
actual row count. A user-defined error condition such as `EQ(_rows_, 0)` will
always trigger after an `insert`, even when a row was successfully inserted.

**Recommendation:**
Set `count = 1` immediately before or after `rowsAffected++` in the
`insertOpcode` branch, so that `_rows_` reflects the one row inserted:

```go
case insertOpcode:
    httpStatus, operationErr = doInsert(...)
    count = 1
    rowsAffected++
```

**Resolution (May 2026):**
One change to `server/tables/scripting/handler.go` — `insertOpcode` case:

Added `count = 1` between the `doInsert` call and `rowsAffected++`:

```go
case insertOpcode:
    httpStatus, operationErr = doInsert(session.ID, session.User, db, task, n+1, &dictionary)
    count = 1
    rowsAffected++
```

`doInsert` always inserts exactly one row (its return value is an HTTP status
code, not a row count). Setting `count = 1` unconditionally is correct because
the error-condition evaluation block is guarded by `operationErr == nil`, so
`_rows_` is only read from `count` when the insert actually succeeded.

Two API tests added to `tools/apitest/tests/4-dsns/`:

- `dsns-314-tx-ins-rowcount.json` — inserts a row with an `EQ(_rows_, 0)`
  error condition; before the fix this condition always triggered (returning
  409 Conflict); after the fix `_rows_` is 1 so the condition does not trigger
  and the transaction commits with HTTP 200 and `count=1`.

- `dsns-315-tx-ins-errcond.json` — inserts a row with an `EQ(_rows_, 1)`
  error condition; confirms that a user-defined condition that fires on a
  successful insert (`_rows_ == 1`) correctly rolls back the transaction and
  returns 409 Conflict with the custom error message.

The existing `dsns-314-tx-drop.json` was renumbered to `dsns-316-tx-drop.json`
to preserve alphabetical execution order.

---

<a id="SCRIPT-M2"></a>

#### SCRIPT-M2 — `drop` opcode does not flush the schema cache

**Affected files:**

- `server/tables/scripting/handler.go` — `needCacheFlush` logic
- `server/tables/scripting/drop.go` — `doDrop`

**Description:**
The server maintains an in-memory schema cache to avoid repeated database
introspection for table column lists and types. After a DDL change the cache
must be flushed so that subsequent operations see the updated schema.

The handler sets `needCacheFlush = true` only when `sqlOpcode` returns a
flush signal — specifically for `ALTER TABLE` statements executed via the `sql`
opcode. The `drop` opcode (`dropOpCode` / `doDrop`) does not set
`needCacheFlush`, meaning the schema cache retains an entry for a table that
no longer exists.

In a test sequence that drops and recreates a table (or in any transaction that
drops a table and then the same connection subsequently references that table),
a stale cache entry may produce `"column 'x' does not exist"` errors or
incorrect column type metadata.

**Recommendation:**
Either have `doDrop` return a flush-needed flag (parallel to the `doSQL`
signature) and propagate it to `needCacheFlush`, or unconditionally set
`needCacheFlush = true` in the `dropOpCode` case of the handler dispatch loop.

**Resolution (May 2026):**
Two changes, fixing both the `drop` opcode and the `sql` opcode `DROP TABLE`
path simultaneously:

**`server/tables/scripting/handler.go` — `dropOpCode` case:**

```go
case dropOpCode:
    httpStatus, operationErr = doDrop(session.ID, session.User, db, task, n+1, &dictionary)
    if operationErr == nil {
        needCacheFlush = true
    }
```

The guard `operationErr == nil` ensures that a failed drop (e.g. table does not
exist → 404) does not spuriously flush the cache before the transaction is rolled
back.

**`server/tables/scripting/sql.go` — `doSQL`, `cacheFlush` detection:**

```go
// Before: ALTER TABLE only
if len(tokens) > 2 && tokens[0] == "alter" && tokens[1] == "table" {
    cacheFlush = true
}
// After: ALTER TABLE and DROP TABLE
if len(tokens) > 2 && tokens[1] == "table" && (tokens[0] == "alter" || tokens[0] == "drop") {
    cacheFlush = true
}
```

`CREATE TABLE` is intentionally excluded: a brand-new table has no stale cache
entry to evict.

**API tests added** in `tools/apitest/tests/4-dsns/`:

*Subgroup A — `drop` opcode (table `sca{{SQLUUID}}`, tests 317–321):*

- `dsns-317-tx-sca-create.json` — creates `sca{{SQLUUID}}` with two columns (id, name) and inserts one row.
- `dsns-318-tx-sca-warm.json` — `GET .../sca{{SQLUUID}}/rows` warms the `SchemaCache` with the 2-column layout.
- `dsns-319-tx-sca-drop-recreate.json` — `@transaction` with `drop sca{{SQLUUID}}` + `sql CREATE TABLE sca{{SQLUUID}} (..., extra TEXT)` + `insert {id:2, name:"after", extra:"bonus"}`.
- `dsns-320-tx-sca-verify.json` — `GET .../sca{{SQLUUID}}/rows?columns=extra` must return **200** with `rows.0.extra == "bonus"`. Without the fix, the stale cache does not know about `extra` and `ReadRows` returns **400 "invalid column name: extra"`.
- `dsns-321-tx-sca-cleanup.json` — drops `sca{{SQLUUID}}`.

*Subgroup B — `sql` opcode `DROP TABLE` (table `scb{{SQLUUID}}`, tests 322–326):*

- `dsns-322-tx-scb-create.json` — same setup with `scb{{SQLUUID}}`.
- `dsns-323-tx-scb-warm.json` — warms the cache for `scb{{SQLUUID}}`.
- `dsns-324-tx-scb-sqldrop-recreate.json` — `@transaction` with `sql "DROP TABLE scb{{SQLUUID}}"` + `sql CREATE TABLE` (with `extra`) + `insert`.
- `dsns-325-tx-scb-verify.json` — `GET .../scb{{SQLUUID}}/rows?columns=extra` must return **200**; same failure mode without the fix.
- `dsns-326-tx-scb-cleanup.json` — drops `scb{{SQLUUID}}`.

The verify tests (`dsns-320`, `dsns-325`) are the regression anchors: they pass
only when the schema cache has been flushed and `ReadRows` re-queries the
database to obtain the updated column list for the recreated table.

---

### Low priority issues

<a id="SCRIPT-L1"></a>

#### SCRIPT-L1 — Empty transaction body returns plain text, not JSON

**Affected files:**

- `server/tables/scripting/handler.go` — empty-task early return (lines 51–59)

**Description:**
When a `POST /@transaction` request is received with an empty body (zero
operations), the handler writes a plain text message and returns HTTP 200:

```go
text := i18n.T("msg.table.tx.empty")
w.WriteHeader(http.StatusOK)
_, _ = w.Write([]byte(text))
```

No `Content-Type` response header is set, so the response defaults to
`text/plain; charset=utf-8`. Every other `@transaction` response carries a
structured `application/vnd.ego.*+json` content type. A client that inspects
the `Content-Type` header to decide how to parse the response will need a
special case for the empty-body path.

**Recommendation:**
Replace the plain text response with a `rowcount+json` response carrying
`count: 0`, consistent with how a non-empty transaction with zero affected rows
is encoded. This makes all `@transaction` success responses uniform and
removes the special-case parsing requirement from clients.

**Resolution:**
The empty-body path now returns a structured JSON response consistent with all
other `@transaction` responses, rather than a plain-text body with no
`Content-Type` header. The response returns HTTP 200 with the message field set
to `"No transactions in task"`, matching the format used by non-empty
transaction responses.

---

---

This chapter covers the review of the `builtins` Go package — the implementation of Ego's built-in functions (`append`, `cast`, `copy`, `delete`, `index`, `length`, `make`, `new`, `typeof`, and the function-registration/dispatch machinery). All issues found in the initial audit have been resolved.

### Testing Infrastructure

The builtin function unit tests live alongside each source file:

| Source file | Test file |
| :---------- | :-------- |
| `builtins/append.go` | `builtins/append_test.go` |
| `builtins/cast.go` | `builtins/cast_test.go` |
| `builtins/close.go` | `builtins/close_test.go` |
| `builtins/copy.go` | `builtins/copy_test.go` |
| `builtins/delete.go` | `builtins/delete_test.go` |
| `builtins/functions.go` | `builtins/functions_test.go` |
| `builtins/index.go` | `builtins/index_test.go` |
| `builtins/length.go` | `builtins/utility_test.go`, `builtins/length_test.go` |
| `builtins/make.go` | `builtins/make_test.go` |
| `builtins/new.go` | `builtins/new_test.go` |
| `builtins/types.go` | `builtins/types_test.go` |

All tests use flat (non-table) naming so each case is independently runnable
with `-run Test_FunctionName_ScenarioDescription`.  Each test function is
prefixed with a comment explaining what is being verified and why, targeted at
a developer who is new to the codebase.

---

<a id="area-builtin-append"></a>

## BUILTIN-APPEND — append.go — Append

| ID | Risk | Summary | Status |
| :- | :--- | :------ | :----- |
| [BUILTIN-APPEND-1](#BUILTIN-APPEND-1) | Low | `Append` skipped type inference when the first argument was a raw `[]any` slice. | ✓ |

<a id="BUILTIN-APPEND-1"></a>

### BUILTIN-APPEND-1 — `Append` skipped type inference when first arg is a raw `[]any`

**Affected function:** `Append`
**File:** `builtins/append.go`
**Risk:** Low
**Status: RESOLVED**

#### Original behavior

When the first argument was a raw `[]any` slice (rather than a `*data.Array`),
the element type `kind` was never updated from its initial `data.InterfaceType`
value.  The returned array was always typed as `[]interface{}` regardless of
the actual element types.

#### Fix

After flattening the `[]any` into the result, `Append` now inspects the first
element and promotes `kind` to the uniform element type when all elements agree.
If the slice is empty or elements have mixed types, `kind` stays as
`InterfaceType` (the correct representation for a heterogeneous array):

```go
if len(array) > 0 && kind.IsInterface() {
    candidate := data.TypeOf(array[0])
    uniform := true
    for _, elem := range array[1:] {
        if !data.TypeOf(elem).IsType(candidate) {
            uniform = false
            break
        }
    }
    if uniform {
        kind = candidate
    }
}
```

**Tests:** `Test_Append_RawGoSliceUniformTypeInferred`,
`Test_Append_RawGoSliceMixedTypesStaysInterface`

---

<a id="area-builtin-cast"></a>

## BUILTIN-CAST — cast.go — Cast, castToStringValue

| ID | Risk | Summary | Status |
| :- | :--- | :------ | :----- |
| [BUILTIN-CAST-1](#BUILTIN-CAST-1) | Low | `castToStringValue` only handled single-byte (ASCII) character literals. | ✓ |
| [BUILTIN-CAST-2](#BUILTIN-CAST-2) | Low | `Cast` returned `ErrInvalidType` for a valid nil coercion result. | ✓ |

<a id="BUILTIN-CAST-1"></a>

### BUILTIN-CAST-1 — `castToStringValue` only handled ASCII (single-byte) character literals

**Affected function:** `castToStringValue`
**File:** `builtins/cast.go`
**Risk:** Low
**Status: RESOLVED**

#### Original behavior

The character-literal detection used `len(actual) == 3` and byte-indexed the
string (`actual[1]`).  A multi-byte Unicode character like `'é'` (2 UTF-8
bytes) produced `len == 4`, so the condition was never true and the cast failed.

#### Fix

The byte-index check was replaced with a rune-based check:

```go
runes := []rune(actual)
if len(runes) == 3 && runes[0] == '\'' && runes[2] == '\'' {
    return int32(runes[1]), nil
}
```

`len(runes)` counts Unicode code points, so any single character enclosed in
single quotes (regardless of byte width) is handled correctly.

**Tests:** `Test_CastToStringValue_ASCIICharLiteral`,
`Test_CastToStringValue_MultibyteCharLiteral`

<a id="BUILTIN-CAST-2"></a>

### BUILTIN-CAST-2 — `Cast` returned `ErrInvalidType` for a valid nil coercion result

**Affected function:** `Cast`
**File:** `builtins/cast.go`
**Risk:** Low
**Status: RESOLVED**

#### Original behavior

After calling `data.Coerce`, the code tested `if v != nil` and returned
`ErrInvalidType` when the coercion succeeded but produced `nil`.  This was
incorrect: `data.Coerce` returning `(nil, nil)` is a valid success for certain
target types.

#### Fix

The `if v != nil` guard was removed.  When `err == nil`, the coercion
succeeded; the result (including nil) is returned directly:

```go
v, err := data.Coerce(source, data.InstanceOfType(t))
if err != nil {
    return nil, errors.New(err).In(t.String())
}
return v, nil
```

---

<a id="area-builtin-copy"></a>

## BUILTIN-COPY — copy.go — DeepCopy

| ID | Risk | Summary | Status |
| :- | :--- | :------ | :----- |
| [BUILTIN-COPY-1](#BUILTIN-COPY-1) | High | `DeepCopy` wrote deep-copied array elements into the source array instead of the result. | ✓ |
| [BUILTIN-COPY-2](#BUILTIN-COPY-2) | Low | `DeepCopy` for `*data.Struct` used a shallow `Copy()`, sharing nested pointer fields. | ✓ |

<a id="BUILTIN-COPY-1"></a>

### BUILTIN-COPY-1 — `DeepCopy` wrote deep-copied elements into the source array, not the result

**Affected function:** `DeepCopy`
**File:** `builtins/copy.go`
**Risk:** High
**Status: RESOLVED**

#### Original behavior

The `*data.Array` case created an empty result array `r` and iterated the
source `v`, but wrote the deep-copied elements back into `v` via `v.Set(i, vv)`
instead of `r.Set(i, vv)`.  The returned `r` was always empty; the source `v`
was mutated as a side effect.

#### Fix

One character changed — `v.Set` became `r.Set`:

```go
_ = r.Set(i, vv)   // was: _ = v.Set(i, vv)
```

**Tests:** `Test_DeepCopy_ArrayCopiesElements`,
`Test_DeepCopy_ArrayIsIndependentFromSource`

<a id="BUILTIN-COPY-2"></a>

### BUILTIN-COPY-2 — `DeepCopy` for `*data.Struct` used a shallow `Copy()`

**Affected function:** `DeepCopy`
**File:** `builtins/copy.go`
**Risk:** Low
**Status: RESOLVED**

#### Original behavior

`v.Copy()` produced a shallow copy — fields that were themselves pointers
(nested `*data.Array`, `*data.Map`, etc.) shared storage between the original
and the copy, unlike the `*data.Map` case which recursed with `DeepCopy`.

#### Fix

After the shallow `Copy()`, `DeepCopy` now iterates every public field and
replaces its value with a recursive deep copy:

```go
r := v.Copy()
for _, fieldName := range v.FieldNames(false) {
    fv, _ := v.Get(fieldName)
    _ = r.Set(fieldName, DeepCopy(fv, depth-1))
}
return r
```

---

<a id="area-builtin-delete"></a>

## BUILTIN-DELETE — delete.go — Delete

| ID | Risk | Summary | Status |
| :- | :--- | :------ | :----- |
| [BUILTIN-DELETE-1](#BUILTIN-DELETE-1) | Low | The `default` error case used an unnamed magic literal `1` for the first-argument position. | ✓ |

<a id="BUILTIN-DELETE-1"></a>

### BUILTIN-DELETE-1 — Hard-coded argument position in error context

**Affected function:** `Delete`
**File:** `builtins/delete.go`
**Risk:** Low
**Status: RESOLVED**

#### Original behavior

The `default` error case used the magic integer literal `1` to indicate the
first argument.  While technically correct, the literal had no name to
communicate its intent.

#### Fix

A local named constant `firstArgument = 1` was introduced at the call site,
making the intent explicit without changing the error output:

```go
const firstArgument = 1
return nil, errors.ErrInvalidType.In("delete").Context(
    fmt.Sprintf("argument %d: %s", firstArgument, data.TypeOf(v).String()))
```

---

<a id="area-builtin-functions"></a>

## BUILTIN-FUNCTIONS — functions.go — AddBuiltins, FindFunction, FindName, CallBuiltin, AddFunction

| ID | Risk | Summary | Status |
| :- | :--- | :------ | :----- |
| [BUILTIN-FUNCTIONS-1](#BUILTIN-FUNCTIONS-1) | Low | `AddFunction` and related helpers accessed the package-level `FunctionDictionary` map without any locking, producing data races. | ✓ |
| [BUILTIN-FUNCTIONS-2](#BUILTIN-FUNCTIONS-2) | Medium | The `"make"` registration declared its return type as `int`, but `Make` never returns an `int`. | ✓ |

<a id="BUILTIN-FUNCTIONS-1"></a>

### BUILTIN-FUNCTIONS-1 — `AddFunction` and related helpers were not safe for concurrent use

**Affected functions:** `AddFunction`, `AddBuiltins`, `FindFunction`,
`FindName`, `CallBuiltin`
**File:** `builtins/functions.go`
**Risk:** Low
**Status: RESOLVED**

#### Original behavior

`FunctionDictionary` is a package-level `map`.  All five functions that read
or write it did so without holding any lock, producing data races detectable
by Go's `-race` flag when called concurrently.

#### Fix

A package-level `sync.RWMutex` named `functionDictionaryMu` was added.
All read operations (`FindFunction`, `FindName`, `CallBuiltin`, and the key
snapshot in `AddBuiltins`) hold `RLock`/`RUnlock`.  The write in `AddFunction`
holds `Lock`/`Unlock`, and the existence check + insert are performed
atomically under the same lock:

```go
functionDictionaryMu.Lock()
_, alreadyExists := FunctionDictionary[fd.Name]
if !alreadyExists {
    FunctionDictionary[fd.Name] = fd
}
functionDictionaryMu.Unlock()
```

<a id="BUILTIN-FUNCTIONS-2"></a>

### BUILTIN-FUNCTIONS-2 — `make()` declaration incorrectly declared return type as `int`

**Affected registration:** `"make"` entry in `FunctionDictionary`
**File:** `builtins/functions.go`
**Risk:** Medium
**Status: RESOLVED**

#### Original behavior

The `Returns` field for `"make"` was `[]*data.Type{data.IntType}`.  `Make`
actually returns a `[]any`, a `*data.Array`, or a `*data.Channel` — never an
`int`.  This mismatch exposed incorrect metadata to the compiler's type checker.

#### Fix

The `Returns` declaration was changed to `data.InterfaceType`, reflecting the
polymorphic return:

```go
Returns: []*data.Type{data.InterfaceType},
```

---

<a id="area-builtin-index"></a>

## BUILTIN-INDEX — index.go — Index

| ID | Risk | Summary | Status |
| :- | :--- | :------ | :----- |
| [BUILTIN-INDEX-1](#BUILTIN-INDEX-1) | Medium | `Index` returned `bool` for `*data.Map` but `int` for every other argument type, contradicting its declared return type. | ✓ |

<a id="BUILTIN-INDEX-1"></a>

### BUILTIN-INDEX-1 — `Index` returned `bool` for `*data.Map` but `int` for all other types

**Affected function:** `Index`
**File:** `builtins/index.go`
**Risk:** Medium
**Status: RESOLVED**

#### Original behavior

The `*data.Map` case returned the raw `bool` from `arg.Get`:

```go
_, found, err := arg.Get(args.Get(1))
return found, err   // ← bool, not int
```

All other cases returned an `int` (array index, 1-based string position, or
-1 for not found).  The function's declaration said it returns `int`.

#### Fix

The map case now returns `1` (found) or `0` (not found):

```go
_, found, err := arg.Get(args.Get(1))
if found {
    return 1, err
}
return 0, err
```

**Tests:** `TestIndex_MapFoundReturnsInt1`, `TestIndex_MapNotFoundReturnsInt0`

---

<a id="area-builtin-length"></a>

## BUILTIN-LENGTH — length.go — Length

| ID | Risk | Summary | Status |
| :- | :--- | :------ | :----- |
| [BUILTIN-LENGTH-1](#BUILTIN-LENGTH-1) | Low | `Length` returned `math.MaxInt32` for any open channel instead of the actual buffered item count. | ✓ |

<a id="BUILTIN-LENGTH-1"></a>

### BUILTIN-LENGTH-1 — `Length` returned `math.MaxInt32` for any non-empty channel

**Affected function:** `Length`
**File:** `builtins/length.go`
**Risk:** Low
**Status: RESOLVED**

#### Original behavior

The channel case returned `math.MaxInt32` for any open channel and `0` only
when the channel was both closed and empty.  An open channel with zero buffered
items returned `math.MaxInt32` instead of `0`, diverging from Go's `len()`.

#### Fix

The channel case now calls `arg.Len()` — a new method added to `data.Channel`
that delegates to Go's built-in `len(c.channel)`:

```go
case *data.Channel:
    return arg.Len(), nil
```

The `math` import, which was only needed for `math.MaxInt32`, was also removed.
A companion `Cap()` method was added to `data.Channel` to support
BUILTIN-NEW-2.

**Tests:** `Test_Length_ChannelEmptyOpenReturnsZero`,
`Test_Length_ChannelWithItemsReturnsCount`,
`Test_Length_ChannelEmptyAfterCloseReturnsZero`

---

<a id="area-builtin-make"></a>

## BUILTIN-MAKE — make.go — Make

| ID | Risk | Summary | Status |
| :- | :--- | :------ | :----- |
| [BUILTIN-MAKE-1](#BUILTIN-MAKE-1) | High | `Make` did not validate that `size` is non-negative, allowing an unrecoverable Go runtime panic. | ✓ |
| [BUILTIN-MAKE-2](#BUILTIN-MAKE-2) | Low | Unrecognized element types silently returned a nil-filled array instead of an error. | ✓ |

<a id="BUILTIN-MAKE-1"></a>

### BUILTIN-MAKE-1 — `Make` did not validate that `size` is non-negative

**Affected function:** `Make`
**File:** `builtins/make.go`
**Risk:** High
**Status: RESOLVED**

#### Original behavior

Passing a negative size to `Make` caused Go's runtime to panic with
"makeslice: len out of range" — an unrecoverable error that bypassed Ego's
`try/catch` mechanism.

#### Fix

A bounds check was added immediately after the size conversion:

```go
if size < 0 {
    return nil, errors.ErrInvalidValue.In("make").Context(size)
}
```

**Tests:** `Test_Make_NegativeSizeReturnsError`,
`Test_Make_ZeroSizeReturnsEmptyArray`

<a id="BUILTIN-MAKE-2"></a>

### BUILTIN-MAKE-2 — Unrecognized element types silently returned a nil-filled array

**Affected function:** `Make`
**File:** `builtins/make.go`
**Risk:** Low
**Status: RESOLVED**

#### Original behavior

Types not covered by the element-type switch (e.g. `int64`, `int32`, `float32`)
fell through without populating the array.  The caller received a `[]any` of
`size` nil elements with no error.

#### Fix

Cases were added for `int64`, `int32`, `int16`, `int8`, `byte`, and `float32`
so each produces the correct typed zero value.  An explicit `default` case now
returns `ErrInvalidType` for any unrecognized element kind instead of silently
returning a nil-filled slice.

---

<a id="area-builtin-new"></a>

## BUILTIN-NEW — new.go — NewInstanceOf, newReflectKind

| ID | Risk | Summary | Status |
| :- | :--- | :------ | :----- |
| [BUILTIN-NEW-1](#BUILTIN-NEW-1) | Medium | `newReflectKind(reflect.Int64)` returned untyped `0`, which Go infers as `int` rather than `int64`. | ✓ |
| [BUILTIN-NEW-2](#BUILTIN-NEW-2) | Low | `NewInstanceOf` returned the existing channel instead of creating a new, independent one. | ✓ |

<a id="BUILTIN-NEW-1"></a>

### BUILTIN-NEW-1 — `newReflectKind(reflect.Int64)` returned `int`, not `int64`

**Affected function:** `newReflectKind`
**File:** `builtins/new.go`
**Risk:** Medium
**Status: RESOLVED**

#### Original behavior

`reflect.Int` and `reflect.Int64` shared a single `case` clause that returned
the untyped integer literal `0`.  Go infers untyped `0` as `int`, so a caller
requesting a `reflect.Int64` zero value received `int(0)` instead of `int64(0)`.

#### Fix

The two cases were split so each returns the correct concrete Go type:

```go
case reflect.Int:
    return 0, nil        // int zero value

case reflect.Int64:
    return int64(0), nil // int64 zero value — explicit cast required
```

**Tests:** `Test_NewReflectKind_Int`, `Test_NewReflectKind_Int64ReturnsInt64`

<a id="BUILTIN-NEW-2"></a>

### BUILTIN-NEW-2 — `NewInstanceOf` returned the existing channel instead of a new one

**Affected function:** `NewInstanceOf`
**File:** `builtins/new.go`
**Risk:** Low
**Status: RESOLVED**

#### Original behavior

When the argument was a `*data.Channel`, `NewInstanceOf` returned the same
channel unchanged.  `$new(ch)` and `ch` therefore aliased the same underlying
channel object.

#### Fix

`NewInstanceOf` now calls `data.NewChannel(typeValue.Cap())` to create a fresh,
independent channel with the same buffer capacity.  `Cap()` is a new method
added to `data.Channel` (alongside the `Len()` method added for
BUILTIN-LENGTH-1) that returns the `size` field set at construction:

```go
if typeValue, ok := args.Get(0).(*data.Channel); ok {
    return data.NewChannel(typeValue.Cap()), nil
}
```

**Tests:** `Test_NewInstanceOf_ChannelReturnsNewInstance`

---

<a id="area-builtin-types"></a>

## BUILTIN-TYPES — types.go — typeOf

| ID | Risk | Summary | Status |
| :- | :--- | :------ | :----- |
| [BUILTIN-TYPES-1](#BUILTIN-TYPES-1) | Low | `typeOf` returned a bare string for builtin functions instead of a `*data.Type`, the only case violating the uniform return type. | ✓ |

<a id="BUILTIN-TYPES-1"></a>

### BUILTIN-TYPES-1 — `typeOf` returned a string for builtin functions, not a `*data.Type`

**Affected function:** `typeOf`
**File:** `builtins/types.go`
**Risk:** Low
**Status: RESOLVED**

#### Original behavior

Every other `typeOf` case returned a `*data.Type`.  The builtin-function case
returned the string `"<builtin>"`, making `typeof(someBuiltinFunc)` the only
path that violated the uniform return type.

#### Fix

The case now constructs a minimal `data.Function` descriptor with a
`"<builtin>"` name and returns a proper `*data.Type` of `FunctionKind`:

```go
builtinFn := data.Function{
    Declaration: &data.Declaration{Name: "<builtin>"},
}
return data.FunctionType(&builtinFn), nil
```

**Tests:** `Test_TypeOf_BuiltinFunctionReturnsFuncType`

---

This chapter documents behavioral anomalies, potential bugs, and design concerns found during the comprehensive bytecode-instruction unit-test effort. Each entry includes the affected instruction(s), a description of the original behavior, the risk level, and the resolution.

### Testing Infrastructure

The comprehensive bytecode unit-test suite is built on shared helpers in
`bytecode/testhelpers_test.go`.  All bytecode instruction tests in this
package use these helpers instead of constructing their own contexts.

#### `testContext` builder

Create a fresh context for each test with `newTestContext(t)`, then chain
"with" methods to set up initial state before calling the instruction under
test:

| Method | Effect |
| :----- | :----- |
| `withStack(items...)` | Push items onto the stack left-to-right; last item ends up on top |
| `withSymbol(name, value)` | Create and initialize a named variable in the local symbol table |
| `withArgList(args...)` | Store a `*data.Array` as `__args` (defs.ArgumentListVariable) |
| `withTypeStrictness(level)` | Set strict / relaxed / dynamic type enforcement |
| `withExtensions(bool)` | Enable or disable Ego language extensions |
| `withBytecodeSize(n)` | Set `bc.nextAddress` so addresses 0..n are valid branch targets |

#### Assertion helpers

After calling the instruction under test, verify outcomes with these methods:

| Method | What it checks |
| :----- | :------------- |
| `assertNoError(err)` | Fails if err is non-nil |
| `assertError(err, want)` | Compares error keys via `errors.Equals`; context suffixes are ignored |
| `assertTopStack(want)` | Pops the stack top and compares with `reflect.DeepEqual` |
| `assertSymbolValue(name, want)` | Reads the named symbol and compares with `reflect.DeepEqual` |
| `assertStackEmpty()` | Fails if `stackPointer != 0` |
| `assertProgramCounter(want)` | Fails if `ctx.programCounter != want` |

#### Key facts for test authors

- **`interface{}` wrapping**: values stored via `data.InterfaceType` are
  wrapped in `data.Interface{Value: v, BaseType: data.TypeOf(v)}`.  Assert
  the wrapped form, not the raw value.
- **Error key comparison**: `assertError` uses `errors.Equals` on the i18n
  key, so `.Context(...)` suffixes added by `runtimeError` are ignored.
- **Stack discipline**: instructions that temporarily push a value must leave
  the stack clean on success.  Use `assertStackEmpty()` to verify.
- **Branch addresses**: `branchByteCode` and the conditional variants validate
  operands against `bc.nextAddress` (not `len(instructions)`).  Call
  `withBytecodeSize(n)` to widen the valid window before testing branches.
- **`__args` type**: `withArgList` stores
  `data.NewArrayFromInterfaces(data.InterfaceType, ...)`.  Storing anything
  other than a `*data.Array` there triggers `ErrInvalidArgumentList`.

#### Test file naming convention

Each source file `bytecode/xxx.go` gets a corresponding test file
`bytecode/xxx_test.go`.  Test functions follow the pattern:

```text
Test_xxxByteCode_ScenarioDescription
```

Tests use flat (non-table) style so each case is independently named and
runnable with `-run`.  Sections inside a test file are separated by banner
comments and group related code paths together.

The shared helper infrastructure lives in the `testContext` type defined in
the `bytecode` package test files.  When adding tests for a new instruction,
read that source first — it is the single source of truth for how to create
a context, push stack items, and assert outcomes.

---

<a id="branch"></a>

## BRANCH — Branch Instructions

| ID | Summary | Status |
| :-- | :-- | :-- |
| [BRANCH-1](#BRANCH-1) | Stack mutated before address validation in conditional branches | ✓ |
| [BRANCH-2](#BRANCH-2) | nil operand silently accepted as address 0 | ✓ |
| [BRANCH-3](#BRANCH-3) | Misleading error context for non-integer operands | ✓ |

<a id="BRANCH-1"></a>

### BRANCH-1 — Stack mutated before address validation in conditional branches

**Affected instructions:** `branchFalseByteCode`, `branchTrueByteCode`  
**File:** `bytecode/branch.go`  
**Risk:** Medium — stack corruption visible after caught runtime errors  
**Discovered by:** `Test_branchFalseByteCode_StackPreservedOnAddressError`,
`Test_branchTrueByteCode_StackPreservedOnAddressError`  
**Status: RESOLVED**

#### BRANCH-1: Original behavior

Both conditional branch instructions popped the top-of-stack (TOS) value
**before** validating the branch destination address.  The sequence was:

```text
1. v, err := c.Pop()         ← TOS consumed here
2. [strict-mode type check]
3. if address out-of-range → return ErrInvalidBytecodeAddress
```

When step 3 returned an error the Pop in step 1 had already been committed.
If the error was caught by a surrounding `try/catch` block in Ego code, the
stack was one item shorter than the caller expected, causing subsequent
instructions to read wrong values or underflow.

#### BRANCH-1: Fix

The `validateBranchAddress` helper was extracted and called **before** any
`c.Pop()` call.  The new execution order is:

1. Validate address (no stack side effects).
2. `c.Pop()` — only reached when the address is known valid.
3. Strict-mode type constraint.
4. Coerce to bool and conditionally update the program counter.

Tests `Test_branchFalseByteCode_StackPreservedOnAddressError` and
`Test_branchTrueByteCode_StackPreservedOnAddressError` confirm that the TOS
value remains on the stack after an address-validation error.

---

<a id="BRANCH-2"></a>

### BRANCH-2 — nil operand silently accepted as address 0

**Affected instructions:** `branchByteCode`, `branchFalseByteCode`,
`branchTrueByteCode`  
**File:** `bytecode/branch.go`  
**Risk:** Low — only triggers if the compiler emits a nil operand, which is
not expected in well-formed bytecode  
**Discovered by:** `Test_branchByteCode_NilOperand`,
`Test_branchFalseByteCode_NilOperand_Rejected`,
`Test_branchTrueByteCode_NilOperand_Rejected`  
**Status: RESOLVED**

#### BRANCH-2: Original behavior

`data.Int(nil)` returns `(0, nil)`.  Since address 0 is always a valid branch
target, passing `nil` as the operand to any branch instruction silently set the
program counter to 0 rather than returning an error.  This masked compiler bugs
where a branch operand was accidentally left nil.

#### BRANCH-2: Fix

An explicit nil check was added at the top of `validateBranchAddress` (called
by all three branch instructions) before `data.Int` is invoked:

```go
if i == nil {
    return 0, c.runtimeError(errors.ErrInvalidBytecodeAddress).Context("nil")
}
```

`Test_branchByteCode_NilOperand` was updated to expect `ErrInvalidBytecodeAddress`
instead of `nil`.  Two new tests were added for the conditional variants.

---

<a id="BRANCH-3"></a>

### BRANCH-3 — Misleading error context for non-integer operands

**Affected instructions:** `branchByteCode`, `branchFalseByteCode`,
`branchTrueByteCode`  
**File:** `bytecode/branch.go`  
**Risk:** Low — only affects diagnostic error messages, not correctness  
**Discovered by:** `Test_branchByteCode_NonIntOperand`  
**Status: RESOLVED**

#### BRANCH-3: Original behavior

When `data.Int(i)` failed because the operand was not an integer, the code
used the `address` variable (zero-valued on failure) as the error context:

```go
if address, err := data.Int(i); err != nil || ... {
    return c.runtimeError(errors.ErrInvalidBytecodeAddress).Context(address)
}
```

The error message read `"invalid bytecode address: 0"` even when the actual
operand was, for example, the string `"not-a-number"`.

#### BRANCH-3: Fix

The `validateBranchAddress` helper separates the two failure modes.  When
`data.Int` itself fails the original operand `i` is used as context; when the
conversion succeeds but the address is out of range the numeric address is
used:

```go
address, err := data.Int(i)
if err != nil {
    // Show the actual bad operand, not the zero-value placeholder.
    return 0, c.runtimeError(errors.ErrInvalidBytecodeAddress).Context(i)
}
if address < 0 || address > c.bc.nextAddress {
    // Address is a valid int but out of the legal range.
    return 0, c.runtimeError(errors.ErrInvalidBytecodeAddress).Context(address)
}
```

---

<a id="call"></a>

## CALL — Call Instructions

| ID | Summary | Status |
| :-- | :-- | :-- |
| [CALL-1](#CALL-1) | Argument count mismatch silently ignored for non-variadic functions with default ArgCount | ✓ |
| [CALL-2](#CALL-2) | First extra variadic argument bypasses strict type checking | ✓ |
| [CALL-3](#CALL-3) | Nil pointer dereference in callRuntimeFunction when savedDefinition is nil and context is sandboxed | ✓ |
| [CALL-4](#CALL-4) | `parentTable` nil guard is dead code for non-literal named functions | ✓ |
| [CALL-5](#CALL-5) | `getPackageSymbols` passes the `this` struct instead of `this.value` to `GetPackageSymbolTable` | ✓ |
| [CALL-6](#CALL-6) | `SetBreakOnReturn` reads the wrong stack slot (off-by-one) | ✓ |
| [CALL-7](#CALL-7) | `callTypeCast` panics when Path-A struct types receive empty argument list | ✓ |
| [CALL-8](#CALL-8) | `makeNativeArrayArgument` missing `Int64Kind` and `Float32Kind` for `*data.Array` conversion | ✓ |
| [CALL-9](#CALL-9) | `CallWithReceiver` panics when method name is not found on receiver | ✓ |
| [CALL-10](#CALL-10) | `synthesizeDefinition` sets `MinArgCount = -1` for zero-parameter variadic functions | ✓ |

<a id="CALL-1"></a>

### CALL-1 — Argument count mismatch silently ignored for non-variadic functions with default ArgCount

**Affected function:** `validateArgCount`  
**File:** `bytecode/call.go`  
**Risk:** Medium — runtime functions can be called with the wrong number of
arguments and no error is raised  
**Discovered by:** `Test_validateArgCount_DefaultArgCountZeroZero_Mismatch`  
**Status: RESOLVED**

#### CALL-1: Original behavior

`validateArgCount` wrapped its mismatch logic in `if argumentCount != argc { ... }`.
Inside that block the first sub-block fired whenever `dp.Declaration` was
non-nil and non-variadic:

```go
if dp.Declaration != nil && !dp.Declaration.Variadic && len(dp.Declaration.ArgCount) == 2 {
    minArgc := dp.Declaration.ArgCount[0]
    maxArgc := dp.Declaration.ArgCount[1]

    if minArgc >= 0 && maxArgc > 0 && (argc < minArgc || argc > maxArgc) {
        return c.runtimeError(errors.ErrArgumentCount).Context(argc)
    }

    return nil  // ← unconditional return
}
```

Because `ArgCount` is `[2]int`, `len(dp.Declaration.ArgCount) == 2` was
**always true**.  The block fired for every non-nil non-variadic declaration
and always returned — via the error path or the bare `return nil`.

When `ArgCount` held its zero value `[2]int{0, 0}`, `maxArgc == 0` so
`maxArgc > 0` was false, the error was not triggered, and `return nil` was
reached unconditionally, silently accepting the argument count mismatch.

The two code paths below this block — the extensions check and the variadic
check — were dead code for all non-nil non-variadic declarations.

#### CALL-1: Fix

`validateArgCount` was restructured to separate the three cases cleanly:

1. Non-variadic with an explicit ArgCount range (`max > 0`): enforce
   `[min, max]` and return.
2. Non-variadic with default ArgCount (`[0, 0]`) and extensions disabled:
   the exact count is required — return `ErrArgumentCount`.
3. Non-variadic with default ArgCount and extensions enabled: allow the
   function to validate its own arg count — return nil.
4. Variadic or nil Declaration: require at least `argumentCount-1` args.

The outer `if argumentCount != argc` block was also replaced with an early
return (`if argumentCount == argc { return nil }`) which removes one level of
nesting and makes the structure clearer.

`Test_validateArgCount_DefaultArgCountZeroZero_Mismatch` now asserts
`ErrArgumentCount`, and a companion test
`Test_validateArgCount_DefaultArgCountZeroZero_ExtensionsAllowsMismatch`
confirms the extensions path returns nil.

---

<a id="CALL-2"></a>

### CALL-2 — First extra variadic argument bypasses strict type checking

**Affected function:** `validateStrictParameterTyping`  
**File:** `bytecode/call.go`  
**Risk:** Medium — type-unsafe values can silently reach variadic runtime
functions in strict mode  
**Discovered by:** `Test_validateStrict_Variadic_ExtraArgTypeMismatch`,
`Test_validateStrict_Variadic_SecondExtraArgTypeMismatch`  
**Status: RESOLVED**

#### CALL-2: Original behavior

The variadic type-check block fired when `n > len(parms)`, where `n` is the
argument index and `parms` is `dp.Declaration.Parameters`.  For a two-parameter
variadic function `f(a int, b ...int)` with `len(parms) == 2`:

- n=0: `n > 2` → false; `n < 2` → true → regular check on parms[0] ✓
- n=1: `n > 2` → false; `n < 2` → true → regular check on parms[1] ✓
- n=2: `n > 2` → false; `n < 2` → false → **neither block fired** ✗
- n=3: `n > 2` → true → variadic check ✓

The argument at index `n == len(parms)` (the first *extra* variadic arg)
escaped both checks and was never type-validated.

#### CALL-2: Fix

The condition was changed from `n > len(parms)` to `n >= len(parms)`, and a
`len(parms) > 0` guard was added to prevent an index-out-of-bounds panic for
the pathological case of a variadic function with no declared parameters:

```go
if dp.Declaration.Variadic && len(parms) > 0 && n >= len(parms) {
```

A `continue` statement was also added after the variadic type-error check to
prevent double-checking (the `n < len(parms)` block below would always be
false for n >= len(parms), but the explicit continue makes the intent clear).

`Test_validateStrict_Variadic_ExtraArgTypeMismatch` now asserts
`ErrArgumentType` for args[2] at the previously unchecked index.

---

<a id="CALL-3"></a>

### CALL-3 — Nil pointer dereference in callRuntimeFunction when savedDefinition is nil and context is sandboxed

**Affected function:** `callRuntimeFunction`  
**File:** `bytecode/callRuntimeFunction.go`  
**Risk:** High in sandboxed deployments — accessing `savedDefinition.Sandboxed`
panics when `savedDefinition == nil`  
**Discovered by:** code review during call.go test development  
**Status: RESOLVED**

#### CALL-3: Original behavior

`callRuntimeFunction` included this guard:

```go
if c.sandboxedIO.Load() && savedDefinition.Sandboxed {
    return errors.ErrNoPrivilegeForOperation...
}
```

When a bare `func(*symbols.SymbolTable, data.List) (any, error)` was pushed
directly onto the stack (not wrapped in a `data.Function`), `callByteCode`
passed `savedDefinition = nil` to `callRuntimeFunction`.  If the execution
context was sandboxed (`c.sandboxedIO.Load()` returns `true`), Go's `&&`
operator did not short-circuit because the left operand was true, and
`savedDefinition.Sandboxed` panicked with a nil pointer dereference.

In the non-sandboxed default the panic did not occur because the left operand
was false.  The bug was latent in production server code that uses
`ctx.Sandboxed(true)`.

#### CALL-3: Fix

A nil check was added as the middle operand:

```go
if c.sandboxedIO.Load() && savedDefinition != nil && savedDefinition.Sandboxed {
    return errors.ErrNoPrivilegeForOperation.Context(savedDefinition.Declaration.Name + "()")
}
```

When `savedDefinition` is nil the middle operand short-circuits and the field
access is never reached.  The function proceeds normally — the bare runtime
function is not sandboxed, so no privilege check is needed.

`Test_callRuntimeFunction_NilDefinition_SandboxedNoPanic` confirms that calling
a bare runtime function with `sandboxedIO = true` no longer panics.

---

<a id="CALL-4"></a>

### CALL-4 — `parentTable` nil guard is dead code for non-literal named functions

**Affected function:** `callBytecodeFunction`  
**File:** `bytecode/callBytecodeFunction.go`  
**Risk:** Low — no crash occurs; the behavior is correct despite the dead guard  
**Discovered by:** `Test_callBytecodeFunction_NilFindNextScope_StillSucceeds`  
**Status: RESOLVED**

#### CALL-4: Original behavior

`callBytecodeFunction` guarded `parentTable` against nil inside the
`functionSymbols == nil` block using a raw, uninitialized struct literal:

```go
if parentTable == nil {
    parentTable = &symbols.SymbolTable{Name: "<none>"}  // raw struct — nil maps/values
}
```

The guard was never meaningful: for the `callFramePush` path `parentTable`
was not consumed, and for the captured-scope path `capturedScope != nil`
guaranteed `parentTable` was already non-nil.  Meanwhile the package-method
path (`functionSymbols != nil`) had no guard at all, so `Clone(nil)` could
be called when `FindNextScope` returned nil from a root context.

#### CALL-4: Fix

The entire `getPackageSymbols()` call and the associated `functionSymbols`
branch were removed from `callBytecodeFunction` (see the broader CALL-4/CALL-5
investigation note below).  The dead nil guard and the raw struct literal
disappeared with the code they guarded.

The log statement is now safe by computing `parentName` conditionally:

```go
parentName := "<none>"
if parentTable != nil {
    parentName = parentTable.Name
}
```

`Test_callBytecodeFunction_NilFindNextScope_StillSucceeds` confirms that a
named function call succeeds from a root-level context (nil `FindNextScope`).

#### CALL-4 / CALL-5: Combined investigation note

Fixing CALL-5 (making `getPackageSymbols()` correctly return the package's
embedded symbol table) exposed a deeper issue: the `functionSymbols != nil`
clone path in `callBytecodeFunction` broke global scope access.

When a package function is called as `math.Factor(n)`, the compiler emits
`SetThis` which pushes `math` onto the receiver stack.  After the CALL-5
fix, `getPackageSymbols()` returned the math package's embedded symbol table.
`callBytecodeFunction` then cloned that table and used it as the function's
scope.  The clone only contained the math package's own symbols — not the
global package registry — so any reference inside `Factor` to another package
(e.g. `math.Sqrt`) failed with "unknown identifier".

The root cause: `SetThis` pushes the receiver for ALL member calls, including
plain package-function calls (`math.Floor(x)`) where `callNative` does NOT
pop the receiver (it only calls `popThis()` when `dp.Declaration.Type != nil`).
The receiver stack therefore retains stale package objects, and any subsequent
`*ByteCode` call with a non-empty receiver stack would incorrectly take the
clone path.

The fix: the `getPackageSymbols()` call was removed from `callBytecodeFunction`
entirely.  Compiled Ego functions always use `callFramePush` which creates a
fresh boundary scope as a child of `c.symbols`, giving the function access to
the full scope chain including the global package registry.  The existing
`updatePackageFromLocalSymbols` mechanism in `callFramePop` already handles
writing modified package-level symbols back to the package on return — no
clone path is needed.

---

<a id="CALL-5"></a>

### CALL-5 — `getPackageSymbols` passes the `this` struct instead of `this.value` to `GetPackageSymbolTable`

**Affected function:** `getPackageSymbols`  
**File:** `bytecode/flow.go`  
**Risk:** Medium — the package-method cloning path in `callBytecodeFunction`
is permanently unreachable; package method calls silently fall back to the
generic `callFramePush` path  
**Discovered by:** `Test_getPackageSymbols_PackageReceiver_ReturnsSymbolTable`  
**Status: RESOLVED**

#### CALL-5: Original behavior

`getPackageSymbols` passed the raw `this` struct (the bookkeeping wrapper) to
`GetPackageSymbolTable` instead of the unwrapped `receiver.value` field:

```go
this := c.receiverStack[len(c.receiverStack)-1]
table := symbols.GetPackageSymbolTable(this)   // ← was: this (the wrapper struct)
```

`c.receiverStack` stores `this{name string, value any}` structs.
`GetPackageSymbolTable` type-asserts its argument to `*data.Package`.  The
assertion on a `this` struct always failed, `nil` was returned, and
`callBytecodeFunction` always took the plain `callFramePush` branch.  The
package-method clone path was permanently dead.

#### CALL-5: Fix

One word changed in `flow.go`: the `this` local variable was renamed to
`receiver` (for clarity) and `receiver.value` is now passed:

```go
receiver := c.receiverStack[len(c.receiverStack)-1]
table := symbols.GetPackageSymbolTable(receiver.value)
```

The existing tests were updated:

- `Test_getPackageSymbols_PackageReceiver_CurrentlyReturnsNil` was renamed to
  `Test_getPackageSymbols_PackageReceiver_ReturnsSymbolTable` and now asserts
  a non-nil result with the expected table name `"package math"`.
- `Test_callBytecodeFunction_PackageMethod_ClonesPackageSymbolTable` was added
  to verify the previously dead clone path is now reachable and that the
  resulting scope carries `IsClone() == true`.

<a id="CALL-6"></a>

### CALL-6 — `SetBreakOnReturn` reads the wrong stack slot (off-by-one)

**Affected function:** `SetBreakOnReturn`  
**File:** `bytecode/callframe.go`  
**Risk:** Medium — the debugger "step out / break on return" feature was
silently non-functional; it logged an error but never set the flag  
**Discovered by:** `Test_SetBreakOnReturn_SetsBreakOnReturnFlag`,
`Test_SetBreakOnReturn_FrameAtFPMinusOne`  
**Status: RESOLVED**

#### CALL-6: Frame-pointer convention

After `callFramePushWithTable` the runtime stack layout is:

```text
index:  ... | old_sp      | old_sp+1 ...
             | *CallFrame  | [callee locals / return values]
             | fp - 1      |
```

`c.framePointer` is set to `old_sp + 1` — one slot **past** the frame.  All
frame-reading code (`callFramePop`, `FormatFrames`, `GetFrame`) uses
`stack[framePointer-1]` to reach the `*CallFrame`.

#### CALL-6: Original behavior

`SetBreakOnReturn` used `stack[framePointer]` (without the `-1`):

```go
callFrameValue := c.stack[c.framePointer]   // ← one slot too high
if callFrame, ok := callFrameValue.(*CallFrame); ok {
    callFrame.breakOnReturn = true
    c.stack[c.framePointer] = callFrame     // ← one slot too high
} else {
    ui.Log(...)   // always reached — type assertion always failed
}
```

When the callee had no local data on its stack, `stack[framePointer]` was nil.
The type assertion `nil.(*CallFrame)` failed silently, the `else` branch logged
an error, and `breakOnReturn` was **never set**.  The debugger's "step out"
command therefore had no effect.

#### CALL-6: Fix

Both accesses changed from `c.framePointer` to `c.framePointer-1`, matching
the convention used everywhere else in the file:

```go
callFrameValue := c.stack[c.framePointer-1]   // was: c.framePointer
if callFrame, ok := callFrameValue.(*CallFrame); ok {
    callFrame.breakOnReturn = true
    c.stack[c.framePointer-1] = callFrame     // was: c.framePointer
} else {
    ui.Log(...)
}
```

A comment was also added to `SetBreakOnReturn` explaining the frame-pointer
convention so the same mistake is not repeated.

`Test_SetBreakOnReturn_SetsBreakOnReturnFlag` confirms that
`ctx.breakOnReturn` is `true` after `SetBreakOnReturn()` + `callFramePop()`.
`Test_SetBreakOnReturn_FrameAtFPMinusOne` confirms the slot layout invariant.
All 869 Ego-language integration tests continue to pass.

After the fix, `Test_SetBreakOnReturn_CurrentlyFailsDueToOffByOne` must be
updated to assert `ctx.breakOnReturn == true`.

---

<a id="CALL-7"></a>

### CALL-7 — `callTypeCast` panics when Path-A struct types receive empty argument list

**Affected function:** `callTypeCast`  
**File:** `bytecode/callCastFunction.go`  
**Risk:** Medium — a well-formed Ego program that calls a struct-based type
constructor with no arguments (e.g. `time.Duration()`) caused an unrecoverable
runtime panic instead of a clean error  
**Discovered by:** `Test_callTypeCast_Duration_EmptyArgs`,
`Test_callTypeCast_Month_EmptyArgs`  
**Status: RESOLVED**

#### CALL-7: Original behavior

`callTypeCast` dispatched on the kind of the target type.  When the type was a
`StructKind` or a `TypeKind` wrapping a `StructKind` (Path A), the function
accessed `args[0]` directly without first checking that the slice was non-empty:

```go
case defs.TimeDurationTypeName:
    if d, err := data.Int64(args[0]); err == nil {  // ← panic if args is empty
```

When `callByteCode` called `callTypeCast` with `argc == 0` (the user wrote
`time.Duration()` or `time.Month()` with no arguments), `args` was an empty
slice and either access panicked with a runtime index-out-of-bounds error.

Path B (scalar/array types) was not affected because it appends the type to
`args` and delegates to `builtins.Cast`, which handles empty lists gracefully.

#### CALL-7: Fix

A single bounds check was added at the top of the Path-A block, before the
switch statement:

```go
if function.Kind() == data.StructKind || ... {
    // Guard against zero-argument calls (CALL-7 fix).
    // Struct-based type constructors always require exactly one argument.
    if len(args) == 0 {
        return c.runtimeError(errors.ErrArgumentCount)
    }
    switch function.NativeName() {
    ...
    }
}
```

`Test_callTypeCast_Duration_EmptyArgs` and `Test_callTypeCast_Month_EmptyArgs`
now assert `ErrArgumentCount` and an empty stack rather than catching a panic.

---

<a id="CALL-8"></a>

### CALL-8 — `makeNativeArrayArgument` missing `Int64Kind` and `Float32Kind` for `*data.Array` conversion

**Affected function:** `makeNativeArrayArgument`  
**File:** `bytecode/callNative.go`  
**Risk:** Low — passing a `*data.Array` of int64 or float32 elements to a
native function expecting `[]int64` or `[]float32` silently returned
`ErrInvalidType` instead of converting correctly  
**Discovered by:** `Test_makeNativeArrayArgument_Int64Kind`,
`Test_makeNativeArrayArgument_Float32Kind`  
**Status: RESOLVED**

#### CALL-8: Original behavior

`makeNativeArrayArgument` converts a `*data.Array` to the equivalent native
Go slice so it can be passed to a Go function via reflection.  The switch on
element kind handled: `IntKind`, `Int16Kind`, `UInt16Kind`, `Int32Kind`,
`BoolKind`, `ByteKind`, `Float64Kind`, and `StringKind`.

Two kinds were absent: `Int64Kind` and `Float32Kind`.  A `*data.Array` of
int64 or float32 elements fell through to the `default` case and returned
`ErrInvalidType`.  The asymmetry was visible because native `[]int64` and
`[]float32` slices were already handled by direct pass-through, and
`convertFromNativeArray` already converted `[]int64` and `[]float32` back to
`*data.Array` on the return path.

#### CALL-8: Fix

Two new cases were added to the switch, each with a clear comment explaining
why they were previously absent:

```go
case data.Float32Kind:
    // Added by CALL-8 fix — was missing despite []float32 pass-through above.
    arrayArgument := make([]float32, arg.Len())
    for i := 0; i < arg.Len(); i++ {
        v, _ := arg.Get(i)
        arrayArgument[i], err = data.Float32(v)
        ...
    }

case data.Int64Kind:
    // Added by CALL-8 fix — mirrors the existing []int64 return-path support.
    arrayArgument := make([]int64, arg.Len())
    ...
```

`Test_makeNativeArrayArgument_Int64Kind` and
`Test_makeNativeArrayArgument_Float32Kind` now assert that the conversion
succeeds and produces the expected concrete slice type.

---

<a id="CALL-9"></a>

### CALL-9 — `CallWithReceiver` panics when method name is not found on receiver

**Affected function:** `CallWithReceiver`  
**File:** `bytecode/callNative.go`  
**Risk:** Medium — an invalid method name in compiled Ego code caused an
unrecoverable runtime panic instead of a clean error  
**Discovered by:** `Test_CallWithReceiver_UnknownMethod`  
**Status: RESOLVED**

#### CALL-9: Original behavior

For non-struct, non-pointer receivers, `CallWithReceiver` used reflection to
look up the method by name and call it without checking whether the lookup
succeeded:

```go
m = ax.MethodByName(methodName)
results := m.Call(argList)   // ← panicked if m was a zero Value
```

`MethodByName` returns a zero `reflect.Value` when the method does not exist
on the type.  Calling `.Call()` on a zero `reflect.Value` caused an
unrecoverable panic:

```text
panic: reflect: call of reflect.Value.Call on zero Value
```

#### CALL-9: Fix

An `m.IsValid()` guard was inserted between the lookup and the call, with a
comment explaining the zero-Value risk:

```go
m = ax.MethodByName(methodName)
// Guard: MethodByName returns a zero reflect.Value when the method
// does not exist.  Without this check, m.Call() panics (CALL-9 fix).
if !m.IsValid() {
    return nil, errors.ErrNoFunctionReceiver.Context(methodName)
}
results := m.Call(argList)
```

`Test_CallWithReceiver_UnknownMethod` now asserts that no panic occurs and
that a non-nil error is returned.

---

<a id="CALL-10"></a>

### CALL-10 — `synthesizeDefinition` sets `MinArgCount = -1` for zero-parameter variadic functions

**Affected function:** `synthesizeDefinition`  
**File:** `bytecode/callRuntimeFunction.go`  
**Risk:** Low — the condition `len(args) < -1` is never true, so the -1 value
effectively permitted any number of arguments; the practical behavior was
correct by accident, but the intended minimum of 0 was not expressed  
**Discovered by:** `Test_synthesizeDefinition_Variadic_ZeroParams`  
**Status: RESOLVED**

#### CALL-10: Original behavior

`synthesizeDefinition` computed the minimum argument count for a variadic
function as `len(Parameters) - 1`:

```go
} else {
    definition.MinArgCount = len(savedDefinition.Declaration.Parameters) - 1
    definition.MaxArgCount = 99999
}
```

The formula was correct for the common case — a two-parameter variadic
`func f(a int, b ...int)` with `len(params) = 2` yielded `MinArgCount = 1`.

When `len(Parameters) == 0` the formula yielded `-1`.  The guard
`len(args) < -1` was never true, so any call passed — correct behavior, but
achieved by accident rather than by design.

#### CALL-10: Fix

A clamp was added immediately after the formula, with a comment explaining the
zero-param edge case:

```go
minCount := len(savedDefinition.Declaration.Parameters) - 1
// Clamp: a variadic function with no declared parameters requires at
// least 0 arguments, not -1 (CALL-10 fix).
if minCount < 0 {
    minCount = 0
}
definition.MinArgCount = minCount
```

`Test_synthesizeDefinition_Variadic_ZeroParams` now asserts
`MinArgCount == 0` instead of `-1`, and a comment in the source explains
why the clamp is needed.

---

<a id="trycatch"></a>

## TRYCATCH — Try/Catch Instructions

| ID | Summary | Status |
| :-- | :-- | :-- |
| [TRYCATCH-1](#TRYCATCH-1) | `willCatchByteCode` panics on negative integer operands | ✓ |

<a id="TRYCATCH-1"></a>

### `TRYCATCH-1` — `willCatchByteCode` panics on negative integer operands

**Affected function:** `willCatchByteCode`  
**File:** `bytecode/try.go`  
**Risk:** Medium — a negative integer operand slipped past the upper-bound guard
and caused a runtime index-out-of-bounds panic when accessing `catchSets`  
**Discovered by:** `Test_willCatchByteCode_NegativeInt_ReturnsError`  
**Status: RESOLVED**

#### `TRYCATCH-1`: Original behavior

`willCatchByteCode` checked whether the integer operand was within the bounds
of the `catchSets` slice using only an upper-bound guard:

```go
if i > len(catchSets) {   // only rejects values too large
    return c.runtimeError(errors.ErrInternalCompiler)...
}
...
try.catches = append(try.catches, catchSets[i-1]...)
```

Negative values passed the guard (`-1 > 1` is false) and then caused an
unrecoverable panic when `catchSets[i-1]` accessed the slice at a negative
index:

```text
panic: runtime error: index out of range [-2]
```

#### `TRYCATCH-1`: Fix

The guard was extended to check both directions, and a comment was added
explaining the valid range of operand values:

```go
// Valid values: 0 (catch-all), 1..len(catchSets) (named sets).
// Negative values slipped past the original guard — TRYCATCH-1 fix.
if i < 0 || i > len(catchSets) {
    return c.runtimeError(errors.ErrInternalCompiler).Context(...)
}
```

`Test_willCatchByteCode_NegativeInt_ReturnsError` confirms that `-1` now
returns `ErrInternalCompiler` without panicking.

---

<a id="coerce"></a>

## COERCE — Coerce and Conversions Instructions

| ID | Summary | Status |
| :-- | :-- | :-- |
| [COERCE-1](#COERCE-1) | `NeedsCoerce` returns the wrong answer when the `Push` operand does not match the target type | ✓ |
| [COERCE-2](#COERCE-2) | `data.UInt` accessor panics with a type assertion failure | ✓ |

<a id="COERCE-1"></a>

### COERCE-1 — `NeedsCoerce` returns the wrong answer when the `Push` operand does not match the target type

**Affected function:** `NeedsCoerce` (method on `ByteCode`)  
**File:** `bytecode/coerce.go`  
**Risk:** Low — caused unnecessary Coerce instructions when types already
matched, and skipped needed Coerce instructions when a `Push` operand had a
different type than the target (e.g. int literal in a `[]float64` array)  
**Discovered by:** `Test_NeedsCoerce_LastInstructionPush_NonMatchingType`  
**Status: RESOLVED**

#### COERCE-1: Original behavior

The `Push` branch of `NeedsCoerce` returned `data.IsType(i.Operand, kind)`,
which is `true` when the pushed value already IS the target type:

- **Matching type** → `true` → redundant Coerce emitted (no-op at runtime)
- **Non-matching type** → `false` → Coerce silently skipped; values were left
  in their original Go type rather than being converted to the array's declared
  element type

#### COERCE-1: Fix

The `Push` branch now returns `!data.IsType(i.Operand, kind)`:

```go
if i.Operation == Push {
    // Coerce is needed only when the pushed value does NOT already match.
    return !data.IsType(i.Operand, kind)   // was: data.IsType(...)
}
```

`Test_NeedsCoerce_LastInstructionPush_MatchingType` now asserts `false`
(no redundant Coerce), and `Test_NeedsCoerce_LastInstructionPush_NonMatchingType`
now asserts `true` (Coerce IS emitted when types differ).

All 869 Ego-language integration tests pass in both `--types strict` and
`--types dynamic` mode after this change.

---

<a id="COERCE-2"></a>

### COERCE-2 — `data.UInt` accessor panics with a type assertion failure

**Affected path:** `coerceByteCode` → `data.UInt`  
**File:** `data/coerce.go` (root cause); `data/accessor.go` (assertion site)  
**Risk:** Medium — any Ego program that coerced a value to the `uint` type
panicked instead of producing a clean runtime result  
**Discovered by:** `Test_coerceByteCode_ToUInt`  
**Status: RESOLVED**

#### COERCE-2: Original behavior

The package-level `Coerce(value, model)` function routed `case uint:` to
`coerceUInt64`, which returns `uint64`.  `data.UInt()` then asserted the
result as `uint`, causing a panic:

```text
panic: interface conversion: interface {} is uint64, not uint
```

#### COERCE-2: Fix

A dedicated `coerceUInt` helper was added to `data/coerce.go`.  It mirrors
`coerceUInt64` in structure but returns `uint` for every input type.  The
`case uint:` branch of the package-level `Coerce` function now calls
`coerceUInt` instead of `coerceUInt64`, so `data.UInt()` receives the correct
concrete type and the type assertion succeeds.

`Test_coerceByteCode_ToUInt` now asserts a clean `nil` error and `uint(10)`
on the stack rather than catching a panic.

---

<a id="compare"></a>

## COMPARE — Comparison Instructions

| ID | Summary | Status |
| :-- | :-- | :-- |
| [COMPARE-1](#COMPARE-1) | `notEqualByteCode` and `greaterThanOrEqualByteCode` had no tests | ✓ |
| [COMPARE-2](#COMPARE-2) | `notEqualByteCode` uses value types instead of pointer types for composite cases | |
| [COMPARE-4](#COMPARE-4) | Four comparison operators returned raw errors without `c.runtimeError` decoration | ✓ |
| [COMPARE-3](#COMPARE-3) | `int8` missing from signed-integer case in four ordering functions | ✓ |

<a id="COMPARE-1"></a>

### COMPARE-1 — `notEqualByteCode` and `greaterThanOrEqualByteCode` had no tests

**Affected operators:** `!=`, `>=`  
**File:** `bytecode/compare_test.go`  
**Risk:** Low — untested paths; the absence of tests allowed the COMPARE-2 and
COMPARE-3 bugs to go undetected  
**Discovered by:** audit of `compare_test.go`  
**Status: RESOLVED**

#### COMPARE-1: Description

The original `compare_test.go` contained a single `TestComparisons` table-driven
function that covered `==`, `<`, `>`, and `<=`.  `notEqualByteCode` (`!=`) and
`greaterThanOrEqualByteCode` (`>=`) had zero test cases.

Additionally, the original tests used raw `Context{}` struct literals with direct
`ctx.stack[0]` index reads, bypassing the `newTestContext` / `withStack` helpers
established in all other bytecode test files.

#### COMPARE-1: Fix

`compare_test.go` was rewritten with 79 flat test functions following the
established helper pattern.  All six comparison operators are now covered across
the full set of scalar types, composite types, nil values, strict/dynamic mode,
and unsigned integers.

---

<a id="COMPARE-2"></a>

### COMPARE-2 — `notEqualByteCode` uses value types instead of pointer types for composite cases

**Affected function:** `notEqualByteCode`  
**File:** `bytecode/notEqual.go`  
**Risk:** Medium — comparing two `*data.Map`, `*data.Array`, or `*data.Struct`
values with `!=` silently returns `false` (equal) even when the values differ  
**Discovered by:** `Test_notEqualByteCode_MapNotEqual_CurrentlyBroken`,
`Test_notEqualByteCode_ArrayNotEqualValues_CurrentlyBroken`  
**Status: OPEN**

#### COMPARE-2: Description

The switch statement in `notEqualByteCode` has:

```go
case data.Map:      // ← value type; *data.Map never matches
    result = !reflect.DeepEqual(v1, v2)

case data.Array:    // ← value type; *data.Array never matches
    result = !reflect.DeepEqual(v1, v2)

case data.Struct:   // ← value type; *data.Struct never matches
    result = !reflect.DeepEqual(v1, v2)
```

Ego always represents maps, arrays, and structs as pointer values (`*data.Map`,
`*data.Array`, `*data.Struct`).  The value-type cases never match, so the values
fall through to the `default:` branch.  The default branch normalizes the values
as scalars (which changes nothing for composite types) and then falls through an
inner switch with no matching case, leaving `result` at its zero value (`false`).

Compare with `equalByteCode`, which correctly uses `*data.Map`, `*data.Array`,
and `*data.Struct`.

#### COMPARE-2: Suggested fix

Replace the three value-type cases with pointer types:

```go
case *data.Map:
    result = !reflect.DeepEqual(v1, v2)

case *data.Array:
    if array, ok := v2.(*data.Array); ok {
        result = !actual.DeepEqual(array)
    }

case *data.Struct:
    str, ok := v2.(*data.Struct)
    result = !ok || !reflect.DeepEqual(actual, str)
```

After the fix, invert the bug-documentation assertions in
`Test_notEqualByteCode_MapNotEqual_CurrentlyBroken` and
`Test_notEqualByteCode_ArrayNotEqualValues_CurrentlyBroken`.

---

<a id="COMPARE-4"></a>

### COMPARE-4 — Four comparison operators returned raw errors without `c.runtimeError` decoration

**Affected functions:** `lessThanByteCode`, `lessThanOrEqualByteCode`,
`greaterThanOrEqualByteCode`, `notEqualByteCode`  
**Files:** `bytecode/lessThan.go`, `bytecode/lessThanorEqual.go` (sic — "or" is lowercase in the file name),
`bytecode/greaterThanorEqual.go` (sic), `bytecode/notEqual.go`  
**Risk:** Low — error messages lack source-location context; correctness is not
affected  
**Discovered by:** comprehensive test audit in `bytecode/compare_test.go`
(Section 9–13 stack-underflow tests)  
**Status: RESOLVED**

#### COMPARE-4: Original behavior

Every error return in the bytecode package is expected to be wrapped via
`c.runtimeError(err)`, which attaches the current module name and source line
so that error messages displayed to the user include a precise location.

`greaterThanByteCode` was already correct:

```go
// greaterThanByteCode — correct:
v1, v2, err := getComparisonTerms(c, i)
if err != nil {
    return c.runtimeError(err)   // ← decorated
}
...
v1, v2, err = data.Normalize(v1, v2)
if err != nil {
    return c.runtimeError(err)   // ← decorated
}
```

The other four comparison functions returned these errors raw:

```go
// lessThanByteCode, lessThanOrEqualByteCode, greaterThanOrEqualByteCode,
// and notEqualByteCode — original (buggy):
v1, v2, err := getComparisonTerms(c, i)
if err != nil {
    return err    // ← raw; no module/line annotation
}
...
v1, v2, err = data.Normalize(v1, v2)
if err != nil {
    return err    // ← raw; no module/line annotation
}
```

This meant that a stack-underflow error (`ErrStackUnderflow`) from any of those
four operators appeared without the source location that `greaterThanByteCode`'s
identical error would include.

#### COMPARE-4: Fix

Both raw `return err` sites in each of the four affected functions were changed
to `return c.runtimeError(err)`, making the error-decoration pattern uniform
across all six comparison operators.

The four stack-underflow tests added in Sections 9–13 of `compare_test.go`
document this fix by asserting `ErrStackUnderflow` for each previously
inconsistent operator.

---

<a id="COMPARE-3"></a>

### COMPARE-3 — `int8` missing from signed-integer case in four ordering functions

**Affected functions:** `lessThanByteCode`, `greaterThanByteCode`,
`lessThanOrEqualByteCode`, `greaterThanOrEqualByteCode`  
**Files:** `bytecode/` — one source file per operator  
**Risk:** Low — ordering comparisons on `int8` values return `ErrInvalidType`
instead of a boolean result  
**Discovered by:** `Test_lessThanByteCode_Int8`,
`Test_greaterThanByteCode_Int8`,
`Test_lessThanOrEqualByteCode_Int8`,
`Test_greaterThanOrEqualByteCode_Int8`  
**Status: RESOLVED**

#### COMPARE-3: Description

Every ordering function had an inner switch that handled signed integers via
`int64`-based comparison:

```go
// lessThanByteCode (identical pattern in >, <=, >=):
case byte, int32, int16, int, int64:   // ← int8 was missing
    x1, err := data.Int64(v1)
    x2, err := data.Int64(v2)
    result = x1 < x2
```

`int8` was absent.  After normalization two `int8` values remained `int8`, the
inner switch found no match, and the `default` case returned `ErrInvalidType`.

Compare with `genericEqualCompare` (inside `equalByteCode`) which correctly
listed `int8`:

```go
case byte, int8, int16, int32, int, int64:   // int8 present ✓
```

And `notEqualByteCode` which also included `int8` in its inner switch.

#### COMPARE-3: Fix

`int8` was added to the signed-integer case in all four functions:

```go
case byte, int8, int32, int16, int, int64:
```

The four `_CurrentlyBroken` tests were renamed (dropping the suffix) and
updated to assert `nil` error and the correct boolean result.

---

<a id="context"></a>

## CONTEXT — Context Management

| ID | Summary | Status |
| :-- | :-- | :-- |
| [CONTEXT-1](#CONTEXT-1) | `GetModuleName` panics with a nil pointer dereference when `bc` is nil | ✓ |
| [CONTEXT-2](#CONTEXT-2) | `SetDebug` unconditionally sets `singleStep = true` regardless of argument | ✓ |

<a id="CONTEXT-1"></a>

### CONTEXT-1 — `GetModuleName` panics with a nil pointer dereference when `bc` is nil

**Affected function:** `GetModuleName`  
**File:** `bytecode/context.go`  
**Risk:** Low — any caller that creates a context with a nil `ByteCode` and then
calls `GetModuleName` panics  
**Discovered by:** `Test_Context_GetModuleName_NilBytecode`  
**Status: RESOLVED**

#### CONTEXT-1: Description

`GetName` and `GetModuleName` both return the bytecode object's name, but they
differed in nil safety:

```go
// GetName — nil-safe:
func (c *Context) GetName() string {
    if c.bc != nil {
        return c.bc.name
    }
    return defs.Main
}

// GetModuleName — NOT nil-safe (CONTEXT-1 bug):
func (c *Context) GetModuleName() string {
    return c.bc.name   // ← panics when c.bc == nil
}
```

`NewContext` accepts a nil `ByteCode` argument (it sets `c.bc = nil` in that
case).  Any caller that subsequently invoked `GetModuleName` on such a context
received an unrecoverable nil pointer dereference panic.

#### CONTEXT-1: Fix

A nil guard identical to the one in `GetName` was added to `GetModuleName`:

```go
func (c *Context) GetModuleName() string {
    if c.bc != nil {
        return c.bc.name
    }
    return defs.Main
}
```

`Test_Context_GetModuleName_NilBytecode` confirms the function returns
`defs.Main` rather than panicking when the context has no bytecode object.

---

<a id="CONTEXT-2"></a>

### CONTEXT-2 — `SetDebug` unconditionally sets `singleStep = true` regardless of argument

**Affected function:** `SetDebug`  
**File:** `bytecode/context.go`  
**Risk:** Low — no runtime correctness impact (singleStep has no effect when
`debugging` is false), but semantically unexpected and masks explicit calls
to `SetSingleStep(false)`  
**Discovered by:** `Test_Context_SetDebug_False_ClearsBothFlags`  
**Status: RESOLVED**

#### CONTEXT-2: Description

`SetDebug(b)` set `c.debugging = b` but always assigned `c.singleStep = true`,
regardless of `b`:

```go
func (c *Context) SetDebug(b bool) *Context {
    c.debugging = b
    c.singleStep = true   // ← unconditional; was the bug
    return c
}
```

Calling `SetDebug(false)` left `singleStep` enabled.  If a caller explicitly
disabled step mode with `SetSingleStep(false)` and then called `SetDebug(false)`
(e.g., to temporarily pause the debugger), `singleStep` was silently reset to
`true`.  Re-enabling debugging with `SetDebug(true)` would always start in step
mode, making it impossible to re-enter the debugger in run-free (non-step) mode
without an explicit `SetSingleStep(false)` call immediately afterward.

#### CONTEXT-2: Fix

`singleStep` is now assigned the same value as `b`, mirroring the pattern of
every other boolean setter in the file:

```go
func (c *Context) SetDebug(b bool) *Context {
    c.debugging = b
    c.singleStep = b   // enable step mode when debugging on; clear when off
    return c
}
```

`Test_Context_SetDebug_True_SetsBothFlags` confirms that `SetDebug(true)` sets
both `debugging` and `singleStep` to `true`.
`Test_Context_SetDebug_False_ClearsBothFlags` confirms that `SetDebug(false)`
clears both fields.

---

<a id="create"></a>

## CREATE — Array, Map, Structure Creation

| ID | Summary | Status |
| :-- | :-- | :-- |
| [CREATE-1](#CREATE-1) | `makeArrayByteCode` called `result.Set` twice per element | ✓ |
| [CREATE-2](#CREATE-2) | `addMissingFields` inverted error check skipped coerced-value write-back | ✓ |
| [CREATE-3](#CREATE-3) | `makeArrayByteCode` element-pop loop swallowed stack underflow silently | ✓ |

<a id="CREATE-1"></a>

### CREATE-1 — `makeArrayByteCode` called `result.Set` twice per element

**Affected function:** `makeArrayByteCode`  
**File:** `bytecode/create.go`  
**Risk:** Low — sets the same array index to the same value twice; harmless but
wasteful and obscures intent  
**Discovered by:** code audit during `create_test.go` comprehensive review  
**Status: RESOLVED**

#### CREATE-1: Description

Inside the element-population loop in `makeArrayByteCode`, a copy-paste error
caused `result.Set(count-i-1, value)` to be called twice in a row:

```go
if err := result.Set(count-i-1, value); err != nil {
    return err
}

if err = result.Set(count-i-1, value); err != nil {   // ← duplicate
    return err
}
```

Both calls write the same index with the same value.  Because array Set is
idempotent for the same index/value pair, the output was always correct —
but the second call added unnecessary overhead and the divergent `:=` vs `=`
in the two `if` initializers silently wrote into different `err` scopes,
which was confusing.

#### CREATE-1: Fix

The second `result.Set` call and its surrounding `if` block were removed.  A
comment was added to the remaining call explaining the reverse-index formula:

```go
// result[count-i-1] places each popped element at its correct zero-based index
// because the compiler pushes elements left-to-right (rightmost element on top).
if err := result.Set(count-i-1, value); err != nil {
    return err
}
```

---

<a id="CREATE-2"></a>

### CREATE-2 — `addMissingFields` inverted error check skipped coerced-value write-back

**Affected function:** `addMissingFields`  
**File:** `bytecode/create.go`  
**Risk:** Medium — when a struct field has a coercible but mismatched type, the
coerced value was silently discarded and the field retained its original type  
**Discovered by:** code audit during `create_test.go` comprehensive review  
**Status: RESOLVED**

#### CREATE-2: Description

`addMissingFields` coerces existing field values to the type declared in the
struct model.  The post-coercion error check was inverted:

```go
existingValue, err = data.Coerce(existingValue, fieldModel)
if err == nil {
    return err   // ← returned nil on SUCCESS, exiting without updating structMap
}

structMap[fieldName] = existingValue  // ← only reached on FAILURE
```

When coercion succeeded (`err == nil`):

- The function returned `nil` (no error) before writing the coerced value back.
- `structMap[fieldName]` retained the original pre-coercion value.

When coercion failed (`err != nil`):

- The function fell through to `structMap[fieldName] = existingValue`,
  storing the **un-coerced** value.

#### CREATE-2: Accessibility constraint

The coercion block is guarded by `ft.Kind() != data.UndefinedKind`.
`data.Type.Field()` returns `UndefinedType` (kind = `UndefinedKind`) for any
type that is not a raw `StructKind` — including `TypeDefinition` wrappers
(kind = `TypeKind`).  The coercion path is therefore only reachable when the
struct model was created from a raw `data.StructureType`, not from a named
`data.TypeDefinition`.  The test uses a raw `StructureType` to ensure the path
is exercised.

#### CREATE-2: Fix

The condition was corrected from `err == nil` to `err != nil`:

```go
existingValue, err = data.Coerce(existingValue, fieldModel)
if err != nil {
    return err   // bail out on failure
}

structMap[fieldName] = existingValue  // reached on success — stores coerced value
```

`Test_addMissingFields_FieldTypeCoercion_CREATE2` confirms that a `float64`
value in a field declared as `int` is coerced to `int` and written back.
`float64` is used rather than `int32` because `data.TypeOf(int32).IsType(IntType)`
returns `true` in Ego's type system (both are integer kinds), which would bypass
the coercion block entirely.

---

<a id="CREATE-3"></a>

### CREATE-3 — `makeArrayByteCode` element-pop loop swallowed stack underflow silently

**Affected function:** `makeArrayByteCode`  
**File:** `bytecode/create.go`  
**Risk:** Low — a compiler that emits the wrong number of Push instructions
before MakeArray would produce a silently zeroed array instead of a runtime
error  
**Discovered by:** `Test_makeArrayByteCode_ElementStackUnderflow`  
**Status: RESOLVED**

#### CREATE-3: Description

The element-population loop in `makeArrayByteCode` wrapped each `c.Pop()` in an
`if err == nil` guard:

```go
for i := 0; i < count; i++ {
    if value, err := c.Pop(); err == nil {
        // ... set element ...
    }
    // If Pop() failed, the body was silently skipped — no error returned.
}
```

When the stack ran out of elements before all `count` slots were filled,
`c.Pop()` returned `ErrStackUnderflow`.  The `err == nil` condition was false,
the loop body was skipped, and the unset element retained the zero value of the
base type.  After the loop the partially-initialized array was pushed and `nil`
was returned, making the underflow completely invisible.

This was in contrast to `arrayByteCode` (the `Array` opcode), whose element loop
correctly propagated `Pop` errors immediately.

Note: a `StackMarker` in the element position was **not** affected by this bug —
`Pop()` returns a `StackMarker` successfully (no error), and
`coerceConstantArrayInitializer` then detects it with `isStackMarker` and returns
`ErrFunctionReturnedVoid`.  The silent-skip only affected genuine stack underflow.

#### CREATE-3: Fix

The `if err == nil` guard was replaced with an explicit two-statement pop and
immediate error return, matching the pattern used in `arrayByteCode`:

```go
for i := 0; i < count; i++ {
    // Pop the next element value.  Any error (including stack underflow) is
    // returned immediately — CREATE-3 fix.
    value, err := c.Pop()
    if err != nil {
        return err
    }
    // ... set element ...
}
```

`Test_makeArrayByteCode_ElementStackUnderflow` now asserts `ErrStackUnderflow`
when only the base type is on the stack and count=1 requires one element pop.

---

<a id="defer"></a>

## DEFER — Defer Management

| ID | Summary | Status |
| :-- | :-- | :-- |
| [DEFER-1](#DEFER-1) | `deferByteCode` receiver slice captures wrong elements when new count ≠ deferThisSize | ✓ |
| [DEFER-2](#DEFER-2) | `deferByteCode` skips receiver capture when `deferThisSize == 0` | |

<a id="DEFER-1"></a>

### DEFER-1 — `deferByteCode` receiver slice captures wrong elements when new count ≠ deferThisSize

**Affected function:** `deferByteCode`  
**File:** `bytecode/defer.go`  
**Risk:** Medium — deferred method calls in deeply nested receiver contexts may
pass incorrect receiver values, causing the deferred function to operate on the
wrong object  
**Discovered by:** `Test_deferByteCode_ReceiverCapture_MultipleNewReceivers_DEFER1`  
**Status: RESOLVED**

#### DEFER-1: Description

When `deferByteCode` detects that new receivers were pushed to the receiver stack
during evaluation of the deferred call's target (i.e. `c.deferThisSize > 0 &&
c.deferThisSize < len(c.receiverStack)`), it must:

1. **Capture** the newly added receivers into the `deferStatement`'s
   `receiverStack` slice so they are replayed when the defer executes.
2. **Trim** the live receiver stack back to its pre-defer length (`c.deferThisSize`)
   so the caller's receiver state is restored.

The trim is correct:

```go
c.receiverStack = c.receiverStack[:c.deferThisSize]
```

But the capture used the wrong formula:

```go
// BUGGY — captures the last deferThisSize elements, not the newly added ones:
receivers = c.receiverStack[len(c.receiverStack)-c.deferThisSize:]
```

**Why this is wrong:**  The newly added receivers start at index `deferThisSize`
(the pre-defer stack size).  The correct slice is:

```go
receivers = c.receiverStack[c.deferThisSize:]
```

The buggy formula, `receiverStack[len-deferThisSize:]`, only coincidentally
equals the correct formula when exactly `deferThisSize` new receivers were added
(i.e. `len == 2 * deferThisSize`).  Any other count produces a wrong slice:

| deferThisSize | New receivers | len | Buggy start | Correct start | Effect |
| :---: | :---: | :---: | :---: | :---: | :--- |
| 1 | 1 | 2 | 1 | 1 | ✓ coincidentally correct |
| 1 | 2 | 3 | 2 | 1 | ✗ misses first new receiver |
| 2 | 1 | 3 | 1 | 2 | ✗ includes one pre-existing receiver |
| 2 | 3 | 5 | 3 | 2 | ✗ misses first new receiver |

#### DEFER-1: Fix

Changed the capture line from:

```go
receivers = c.receiverStack[len(c.receiverStack)-c.deferThisSize:]
```

to the correct slice that starts exactly at the pre-defer boundary:

```go
receivers = c.receiverStack[c.deferThisSize:]
```

`Test_deferByteCode_ReceiverCapture_MultipleNewReceivers_DEFER1` now passes:
with `deferThisSize=1` and two new receivers pushed, both `[R1, R2]` are
captured rather than only `[R2]`.

---

<a id="DEFER-2"></a>

### DEFER-2 — `deferByteCode` skips receiver capture when `deferThisSize == 0`

**Affected function:** `deferByteCode`  
**File:** `bytecode/defer.go`  
**Risk:** Low — the compiler currently only pushes receivers for deferred method
calls that already have a non-empty receiver stack; a `deferThisSize == 0` context
represents a top-level (non-nested) defer where no method receiver is expected  
**Discovered by:** `Test_deferByteCode_ReceiverCapture_ZeroDeferThisSize_NoCapture`  
**Status: DOCUMENTED / NOT FIXED**

#### DEFER-2: Description

The capture block in `deferByteCode` is guarded by `c.deferThisSize > 0`:

```go
if c.deferThisSize > 0 && (c.deferThisSize < len(c.receiverStack)) {
    receivers = c.receiverStack[c.deferThisSize:]  // (after DEFER-1 fix)
    c.receiverStack = c.receiverStack[:c.deferThisSize]
}
```

When `deferThisSize == 0` (i.e. the receiver stack was empty when
`deferStartByteCode` ran), the block is never entered.  If a receiver IS pushed
while evaluating the deferred call's target — for example, a deferred method
call made from a context where no receivers existed yet — that receiver is never
captured in the `deferStatement` and is also never trimmed from the live stack.

In practice this scenario does not appear to arise: the Ego compiler does not
emit `SetThis` / `LoadThis` instructions for deferred top-level function calls,
so no new receivers are pushed between `deferStart` and `deferByteCode` at the
top level.  The `deferThisSize > 0` guard was therefore safe for all currently
generated bytecode.

#### DEFER-2: Non-fix rationale

Because no Ego language construct currently produces code that would push
receivers while `deferThisSize == 0`, this is a latent design concern rather
than an active bug.  The issue is documented here so that future compiler changes
which add new defer forms (e.g. deferred interface-method calls at top level)
are aware of this guard and adjust it if needed.

`Test_deferByteCode_ReceiverCapture_ZeroDeferThisSize_NoCapture` asserts the
current (zero-capture) behavior as a regression anchor.

---

<a id="equal"></a>

## EQUAL — Equality Testing

| ID | Summary | Status |
| :-- | :-- | :-- |
| [EQUAL-1](#EQUAL-1) | `equalTypes` returns an undecorated error (no module or line info) | ✓ |
| [EQUAL-2](#EQUAL-2) | `getComparisonTerms` returns raw coerce error (no location info) | ✓ |
| [EQUAL-3](#EQUAL-3) | `case nil:` branch in `equalByteCode`'s switch is dead code | ✓ |

<a id="EQUAL-1"></a>

### EQUAL-1 — `equalTypes` returns an undecorated error (no module or line info)

**Affected function:** `equalTypes`  
**File:** `bytecode/equal.go`  
**Risk:** Low — error messages lack the source location that other runtime errors
include; debuggability is slightly reduced  
**Discovered by:** `Test_equalTypes_TypeVsNonType`  
**Status: RESOLVED**

#### EQUAL-1: Original behavior

When `equalTypes` received a v2 value that was neither a `string` nor a
`*data.Type`, it returned an error directly without location annotation:

```go
return errors.ErrNotAType.Context(v2)   // ← no c.runtimeError wrap
```

Every other error path in `equal.go` and the rest of the bytecode package
uses `c.runtimeError(...)`, which annotates the error with the current
module name and source line before returning it.  The direct `return` in
`equalTypes` bypassed that annotation, so an error in a catch block or stack
trace showed only the message key, not where in the Ego program the bad
comparison occurred.

#### EQUAL-1: Fix

The return statement was changed to pass the error through `c.runtimeError`,
which attaches the current module name (via `e.In(c.module)`) and source line
(via `e.At(c.GetLine(), 0)`) before returning:

```go
// Before:
return errors.ErrNotAType.Context(v2)

// After:
return c.runtimeError(errors.ErrNotAType, v2)
```

`c.runtimeError` accepts variadic context values and calls `.Context(v2)` on
the annotated error internally, so the v2 value still appears in the message.

`Test_equalTypes_TypeVsNonType_ErrorIsDecorated` confirms that after the fix,
`HasIn()` returns `true` on the error when the context has a non-empty module
name.

---

<a id="EQUAL-2"></a>

### EQUAL-2 — `getComparisonTerms` returns raw coerce error (no location info)

**Affected function:** `getComparisonTerms`  
**File:** `bytecode/equal.go`  
**Risk:** Low — coercion errors during constant-folding lack source location;
in practice `data.Coerce` never fails for two valid numeric values  
**Discovered by:** code review during `equal_test.go` development  
**Status: RESOLVED**

#### EQUAL-2: Original behavior

The constant-coercion block returned the error from `data.Coerce` directly,
and the shared `err` variable meant the coerce error was also used for the
stack-pop operations above it:

```go
var err error   // shared with pop calls above
...
if k1 > k2 {
    v2, err = data.Coerce(v2, v1)
} else {
    v1, err = data.Coerce(v1, v2)
}
return v1, v2, err   // ← raw error, no c.runtimeError wrap
```

The `return v1, v2, err` at the end of the function leaked the raw
`data.Coerce` error to callers without module or line annotation, inconsistent
with all other error returns in the package.

#### EQUAL-2: Fix

The coerce block was restructured to use a local `coerceErr` variable (not
shared with the pop-error path), explicitly test for failure, and wrap via
`c.runtimeError`:

```go
var coerceErr error

if k1 > k2 {
    v2, coerceErr = data.Coerce(v2, v1)
} else {
    v1, coerceErr = data.Coerce(v1, v2)
}

if coerceErr != nil {
    return nil, nil, c.runtimeError(coerceErr)
}
```

The final `return` was also changed from `return v1, v2, err` to
`return v1, v2, nil` — removing the vestigial use of `err` at that point in
the function, which could never be non-nil after the earlier explicit error
checks.

The coerce success path is exercised by
`Test_equalByteCode_ImmutableCoercion_PromotesConstantToFloat64` and
`Test_equalByteCode_ImmutableCoercion_PromotesConstantToInt32`.  A direct test
of the error wrap is not feasible because `data.Coerce` never returns an error
for two valid numeric values; the fix is defensive.

---

<a id="EQUAL-3"></a>

### EQUAL-3 — `case nil:` branch in `equalByteCode`'s switch is dead code

**Affected function:** `equalByteCode`  
**File:** `bytecode/equal.go`  
**Risk:** None — the code is unreachable and has no runtime effect  
**Discovered by:** `Test_equalByteCode_NilNilHandledByGuard`,
`Test_equalByteCode_NilOneNilHandledByGuard`  
**Status: RESOLVED**

#### EQUAL-3: Original behavior

`equalByteCode` contained two nil guards that ran before the type switch:

```go
if data.IsNil(v1) && data.IsNil(v2) {
    return c.push(true)
}
if data.IsNil(v1) || data.IsNil(v2) {
    return c.push(false)
}
```

`data.IsNil(nil)` returns `true` for a pure Go nil interface value, and
`getComparisonTerms` unwraps any `data.Immutable{Value: nil}` to a bare `nil`
before the guards run.  As a result, by the time the
`switch actual := v1.(type)` statement executed, `v1` was guaranteed to be
non-nil — and the switch contained:

```go
case nil:
    if err, ok := v2.(error); ok {
        result = errors.Nil(err)
    } else {
        result = (v2 == nil)
    }
```

This branch could never be reached.

#### EQUAL-3: Fix

The `case nil:` branch was removed from the type switch.  The function-level
comment on `equalByteCode` was updated to explain the nil-guard invariant
explicitly, so the absence of the case is documented:

```go
// Nil handling is resolved by two guards before the type switch runs:
//   - both nil  → push true
//   - one nil   → push false
//
// These two guards also mean that by the time the type switch executes, v1
// is guaranteed to be non-nil.  There is therefore no `case nil:` branch in
// the switch — such a branch would be unreachable dead code (EQUAL-3).
```

`Test_equalByteCode_NilNilHandledByGuard` and
`Test_equalByteCode_NilOneNilHandledByGuard` confirm that nil comparisons
still produce the correct bool results after the dead code was removed.

---

<a id="load"></a>

## LOAD — Load Instructions

| ID | Summary | Status |
| :-- | :-- | :-- |
| [LOAD-1](#LOAD-1) | `explodeByteCode` returned raw error from `c.Pop()` without `c.runtimeError` decoration | ✓ |
| [LOAD-2](#LOAD-2) | `explodeByteCode` doc comment incorrectly described the operand as "a struct" | ✓ |
| [LOAD-3](#LOAD-3) | `Test_explodeByteCode` in `data_test.go` exits early on the first matched error, silently skipping later table cases | |

<a id="LOAD-1"></a>

### LOAD-1 — `explodeByteCode` returned raw error from `c.Pop()` without `c.runtimeError` decoration

**Affected function:** `explodeByteCode`  
**File:** `bytecode/load.go`  
**Risk:** Low — stack-underflow errors from `explodeByteCode` lacked the
module name and source line that all other runtime errors carry; correctness
was not affected  
**Discovered by:** `Test_explodeByteCode_StackUnderflow` in `bytecode/load_test.go`  
**Status: RESOLVED**

#### LOAD-1: Original behavior

Every error returned by a bytecode instruction function is expected to be
decorated via `c.runtimeError(err)`, which attaches the current module name
and source line before returning the error to the caller.  This annotation
lets the Ego runtime (and the user-facing stack trace) identify exactly where
in the program the error occurred.

`explodeByteCode` returned the raw error from `c.Pop()` directly:

```go
// Original (buggy):
v, err = c.Pop()
if err != nil {
    return err   // ← raw; no module/line annotation
}
```

When the stack was empty, `c.Pop()` returned `ErrStackUnderflow`.  The error
reached the caller without any location information, inconsistent with every
other error path in `explodeByteCode` and the rest of the bytecode package.

This is the same pattern documented in COMPARE-4 for the comparison operators.

#### LOAD-1: Fix

The `return err` was changed to `return c.runtimeError(err)`:

```go
// Fixed:
v, err = c.Pop()
if err != nil {
    return c.runtimeError(err)   // ← decorated with module/line
}
```

`Test_explodeByteCode_StackUnderflow` in `bytecode/load_test.go` confirms
that `ErrStackUnderflow` is returned when the stack is empty and documents
the expected behavior after the fix.

---

<a id="LOAD-2"></a>

### LOAD-2 — `explodeByteCode` doc comment incorrectly described the operand as "a struct"

**Affected function:** `explodeByteCode`  
**File:** `bytecode/load.go`  
**Risk:** None — documentation only; behavior was correct  
**Discovered by:** `Test_explodeByteCode_NonMapStruct` in `bytecode/load_test.go`  
**Status: RESOLVED**

#### LOAD-2: Original behavior

The function-level comment on `explodeByteCode` read:

```go
// explodeByteCode implements Explode. This accepts a struct on the top of
// the stack, and creates local variables for each of the members of the
// struct by their name.
```

The word "struct" is incorrect.  The implementation unconditionally asserts
the popped value as `*data.Map`:

```go
if m, ok := v.(*data.Map); ok {
```

A `*data.Struct` on the stack fails this assertion and falls through to the
`else` branch, returning `ErrInvalidType`.  The original comment would lead
a reader to believe that passing a struct to the Explode opcode was valid.

`Test_explodeByteCode_NonMapStruct` was added as a regression anchor: it
confirms that a `*data.Struct` is rejected with `ErrInvalidType` and documents
the gap between the comment and the implementation.

#### LOAD-2: Fix

The comment was rewritten to describe the actual behavior accurately:

```go
// explodeByteCode implements Explode. This accepts a *data.Map on the top of
// the stack, and creates local variables for each of the key-value pairs in
// the map.  The map must have string keys; non-string keys are rejected with
// ErrWrongMapKeyType.  After creating the variables, a bool is pushed
// indicating whether the map was empty (true = empty, false = had entries).
```

---

<a id="LOAD-3"></a>

### LOAD-3 — `Test_explodeByteCode` in `data_test.go` exits early on the first matched error, silently skipping later table cases

**Affected test:** `Test_explodeByteCode` in `bytecode/data_test.go`  
**File:** `bytecode/data_test.go`  
**Risk:** Low — test coverage gap only; the production function was correct  
**Discovered by:** code review of `data_test.go` during the `load.go` audit  
**Status: DOCUMENTED**

#### LOAD-3: Description

`Test_explodeByteCode` is a table-driven test with four cases:

```text
1. "simple map explosion"   — no error expected
2. "wrong map key type"     — error expected
3. "not a map"              — error expected
4. "empty stack"            — error expected
```

The test loop does **not** use `t.Run` subtests.  Instead it calls
`explodeByteCode` directly and compares errors inline.  When an expected
error is matched, the code uses a bare `return` statement:

```go
for _, tt := range tests {
    // ... setup ...
    err := explodeByteCode(c, nil)
    if err != nil {
        // ...
        if e1 == e2 {
            return   // ← exits Test_explodeByteCode entirely!
        }
        t.Errorf(...)
    }
    // ... check want values ...
}
```

When case 2 ("wrong map key type") produces its expected error and `e1 == e2`
is true, the bare `return` exits `Test_explodeByteCode` rather than just
skipping the current loop iteration.  Cases 3 and 4 are **never executed**.

The same pattern (`return` inside a `t.Run` closure) is correct and exits
only the sub-test — but here there is no `t.Run` wrapper, so `return`
terminates the outer function.

#### LOAD-3: Non-fix rationale

The comprehensive new tests in `bytecode/load_test.go` cover the missing
paths (non-map type, stack underflow, stack marker) using the standard
`newTestContext` helper pattern, so the coverage gap is now filled.

The original `Test_explodeByteCode` in `data_test.go` is left as-is to avoid
churn in a file that is outside the scope of this audit.  A future cleanup
should either add `t.Run` wrappers (making each case a named sub-test) or replace the
bare `return` with `continue` to let the loop reach cases 3 and 4.

---

<a id="area-flow-bytecode"></a>

## FLOW (Bytecode) — Flow Control Instructions

| ID | Summary | Status |
| :-- | :-- | :-- |
| [FLOW-1](#FLOW-1) | `Test_branchFalseByteCode` called `branchTrueByteCode` for its invalid-address sub-case | ✓ |
| [FLOW-2](#FLOW-2) | `moduleByteCode` and `atLineByteCode` access `array[1]` without a bounds check | ✓ |
| [FLOW-3](#FLOW-3) | Pre-helper tests in `flow_test.go` used raw `&Context{}` struct literals | ✓ |

<a id="FLOW-1"></a>

### FLOW-1 — `Test_branchFalseByteCode` called `branchTrueByteCode` for its invalid-address sub-case

**Affected test:** `Test_branchFalseByteCode` in `bytecode/flow_test.go`  
**File:** `bytecode/flow_test.go`  
**Risk:** Low — `branchFalseByteCode`'s address-validation path was not tested
by the legacy test; the gap was covered by `branch_test.go`  
**Discovered by:** code review of `flow_test.go` during the flow-test audit  
**Status: RESOLVED**

#### FLOW-1: Original behavior

The third sub-case in `Test_branchFalseByteCode` was intended to verify that
an out-of-range address is rejected:

```go
// Test if target is invalid
_ = ctx.push(true)
e = branchTrueByteCode(ctx, 20)   // ← wrong function: should be branchFalseByteCode
if !e.(*errors.Error).Equal(errors.ErrInvalidBytecodeAddress) {
    t.Errorf("branchFalseByteCode unexpected error %v", e)   // message still says False
}
```

The call targets `branchTrueByteCode` rather than `branchFalseByteCode`.  The
error message in the `t.Errorf` even refers to `branchFalseByteCode`, showing
the intent was clear — but the wrong function was called.  As a result,
`branchFalseByteCode`'s `validateBranchAddress` path for too-large addresses
was never exercised by this test (though it was covered by the later
`Test_branchFalseByteCode_InvalidAddress_TooLarge` in `branch_test.go`).

#### FLOW-1: Fix

The call was corrected to target `branchFalseByteCode`:

```go
// Sub-case 3: target address is out of range → ErrInvalidBytecodeAddress.
// FLOW-1 fix: the original code mistakenly called branchTrueByteCode here.
_ = ctx.push(true)
e = branchFalseByteCode(ctx, 20)   // 20 > nextAddress(5) → invalid
if !e.(*errors.Error).Equal(errors.ErrInvalidBytecodeAddress) {
    t.Errorf("branchFalseByteCode sub-case 3 unexpected error %v", e)
}
```

---

<a id="FLOW-2"></a>

### FLOW-2 — `moduleByteCode` and `atLineByteCode` access `array[1]` without a bounds check

**Affected functions:** `moduleByteCode`, `atLineByteCode`  
**File:** `bytecode/flow.go`  
**Risk:** Low — a one-element array operand panics with "index out of range";
the compiler always emits two-element arrays for these opcodes  
**Discovered by:** code review of `flow.go` during the flow-test audit  
**Status: RESOLVED**

#### FLOW-2: Original behavior

Both functions accepted a `[]any` array operand but accessed `array[1]`
unconditionally:

```go
// moduleByteCode (original):
if array, ok := i.([]any); ok {
    c.module = data.String(array[0])
    if t, ok := array[1].(*tokenizer.Tokenizer); ok {   // ← no len check → panic
        c.tokenizer = t
        t.Close()
    }
}

// atLineByteCode (original):
if array, ok := i.([]any); ok {
    if line, err = data.Int(array[0]); err != nil {
        return err
    }
    text = data.String(array[1])   // ← no len check → panic
}
```

If the compiler ever emits a one-element array for either opcode (e.g., when
the tokenizer is unavailable at compile time), `array[1]` panics at runtime
with "index out of range [1] with length 1".

#### FLOW-2: Fix

A `len(array) > 1` guard was added before each `array[1]` access in both
functions, and the function-level comments were updated to document the
optional-slot contract:

```go
// moduleByteCode (fixed):
if array, ok := i.([]any); ok {
    c.module = data.String(array[0])
    // Only look for the tokenizer when a second element is actually present.
    if len(array) > 1 {
        if t, ok := array[1].(*tokenizer.Tokenizer); ok {
            c.tokenizer = t
            t.Close()
        }
    }
}

// atLineByteCode (fixed):
if array, ok := i.([]any); ok {
    if line, err = data.Int(array[0]); err != nil {
        return err
    }
    // Only read the source text when a second element is present.
    if len(array) > 1 {
        text = data.String(array[1])
    }
}
```

Regression tests confirm both functions handle a single-element array
without panicking and still set the primary field correctly:

- `Test_moduleByteCode_SingleElementArray` — module name set, no panic
- `Test_atLineByteCode_SingleElementArray` — line number set, source stays ""

---

<a id="FLOW-3"></a>

### FLOW-3 — Pre-helper tests in `flow_test.go` used raw `&Context{}` struct literals

**Affected tests:** `Test_stopByteCode`, `Test_panicByteCode`, `Test_typeCast`,
`Test_localCallAndReturnByteCode`, `Test_branchFalseByteCode`,
`Test_branchTrueByteCode`  
**File:** `bytecode/flow_test.go`  
**Risk:** Low — the tests passed but bypassed the initialization that
`NewContext` performs, which could mask future bugs in that path  
**Discovered by:** code review of `flow_test.go` during the flow-test audit  
**Status: RESOLVED**

#### FLOW-3: Original behavior

Six tests constructed a `*Context` with a raw struct literal, skipping
`NewContext`:

```go
ctx := &Context{
    stack:          make([]any, 5),
    stackPointer:   0,
    symbols:        symbols.NewSymbolTable("cast test"),
    programCounter: 1,
    bc:             &ByteCode{instructions: make([]instruction, 5), nextAddress: 5},
}
ctx.running.Store(true)
```

The `newTestContext(t)` helper (from `testhelpers_test.go`, mandated by
`CLAUDE.md`) creates a properly initialized context via `NewContext` with a
two-level root→local symbol table.  The raw literal bypasses that, which can
mask bugs in `NewContext` or in code that relies on the full initialization.

#### FLOW-3: Fix

All six tests were rewritten to use `newTestContext` and the "with" builder
chain.  Key conversion notes:

- **`Test_stopByteCode`**: Trivial — `stopByteCode` only uses `c.running`.
  The companion `Test_stopByteCode_WithNewContext` was removed (it became
  redundant once the original was converted).
- **`Test_panicByteCode`** → **`Test_panicByteCode_OperandMode`**: The original
  test pre-loaded a stack item that was never consumed (the operand was always
  non-nil).  The converted test uses an empty stack to make the intent clear.
- **`Test_typeCast`** → **`Test_typeCast_IntToString` + `Test_typeCast_BoolToString`**:
  Split into two flat tests; `newTestContext` supplies the symbol table and
  bytecode that `callByteCode` needs.
- **`Test_localCallAndReturnByteCode`**: Uses `withBytecodeSize(5)` for the
  `localCallByteCode(ctx, 5)` call.  The saved-table-name assertion was updated
  from `"local call test"` (the old manual name) to `"test local"` (the name
  that `newTestContext` assigns to its local symbol table).
- **`Test_branchFalseByteCode`** and **`Test_branchTrueByteCode`**: Both use
  `withBytecodeSize(5)` and a shared `tc` across the three sub-cases so that
  the program-counter carry-over from sub-case 1 to sub-case 2 is preserved.

---

<a id="math"></a>

## MATH — Math Operations

| ID | Summary | Status |
| :-- | :-- | :-- |
| [MATH-1](#MATH-1) | `exponentByteCode` returns 0 for signed integer `x^0` (should return 1) | ✓ |
| [MATH-2](#MATH-2) | `multiplyByteCode` `case int16:` asserts `v1.(int8)` when v1 is `int16` — panics | ✓ |
| [MATH-3](#MATH-3) | `multiplyByteCode` `case uint16:` asserts `v1.(int8)` when v1 is `uint16` — panics | ✓ |
| [MATH-4](#MATH-4) | `subtractByteCode` `case int8:` asserts `v1.(int16)` when v1 is `int8` — panics | ✓ |
| [MATH-5](#MATH-5) | `divideByteCode` `case int16:` asserts `v1.(int8)` when v1 is `int16` — panics | ✓ |
| [MATH-6](#MATH-6) | `divideByteCode` `case uint16:` asserts `v1.(int8)` when v1 is `uint16` — panics | ✓ |
| [MATH-7](#MATH-7) | `moduloByteCode` `case int16:` asserts `v1.(int8)` when v1 is `int16` — panics | ✓ |
| [MATH-8](#MATH-8) | `moduloByteCode` `case uint16:` asserts `v1.(int8)` when v1 is `uint16` — panics | ✓ |
| [MATH-9](#MATH-9) | `notByteCode` multi-type case returns wrong result for zero values of non-`int` integer types | ✓ |
| [MATH-10](#MATH-10) | `addByteCode` function comment incorrectly says "OR" for boolean operands | ✓ |
| [MATH-11](#MATH-11) | `notByteCode` and `negateByteCode` return raw (undecorated) errors from `c.Pop()` | ✓ |

<a id="MATH-1"></a>

### MATH-1 — `exponentByteCode` returns 0 for signed integer `x^0` (should return 1)

**Affected function:** `exponentByteCode`  
**File:** `bytecode/math.go`  
**Risk:** Medium — `x^0` for any signed integer type silently returns 0; the
correct mathematical result is 1 for all non-zero bases  
**Discovered by:** `Test_exponentByteCode_SignedInt_PowerZero_CurrentlyBroken_MATH1`  
**Status: RESOLVED**

#### MATH-1: Description

The signed-integer exponentiation path (matching `byte, int8, int16, int32, int,
int64`) contained an explicit fast-exit for the exponent-zero case:

```go
if vv2 == 0 {
    return c.push(0)   // BUG: x^0 should be 1
}
```

The unsigned-integer path directly below it handled the same case correctly:

```go
if vv2 == 0 {
    return c.push(uint64(1))   // was already correct
}
```

#### MATH-1: Fix

Changed the signed-integer zero-exponent return to push `int64(1)`, matching the
type that the multiplication loop returns on success:

```go
if vv2 == 0 {
    // MATH-1 fix: x^0 == 1 for all non-zero bases.  The previous code
    // pushed the untyped literal 0 (which becomes int(0)), giving the
    // mathematically wrong result.  int64(1) matches the type that the
    // success path pushes after the multiplication loop.
    return c.push(int64(1))
}
```

`Test_exponentByteCode_SignedInt_PowerZero` now asserts `int64(1)` and confirms
the correct result.

---

<a id="MATH-2"></a>

### MATH-2 — `multiplyByteCode` `case int16:` asserts `v1.(int8)` when v1 is `int16` — panics

**Affected function:** `multiplyByteCode`  
**File:** `bytecode/math.go`  
**Risk:** High — any multiplication of two `int16` values causes an unrecoverable
runtime panic  
**Discovered by:** `Test_multiplyByteCode_Int16_MATH2`  
**Status: RESOLVED**

#### MATH-2: Description

After `data.Normalize` leaves two `int16` values unchanged (equal kinds), the
type switch entered `case int16:`.  The body asserted `v1.(int8)`:

```go
case int16:
    return c.push(int16(v1.(int8)) * int16(v2.(int16)))
                       ↑ BUG: v1 is int16, not int8
```

#### MATH-2: Fix

```go
case int16:
    // MATH-2 fix: original cast v1.(int8) panicked because v1 is int16
    // after data.Normalize leaves two equal-kind values unchanged.
    return c.push(v1.(int16) * v2.(int16))
```

`Test_multiplyByteCode_Int16` now asserts `int16(12)` for `3 * 4`.

---

<a id="MATH-3"></a>

### MATH-3 — `multiplyByteCode` `case uint16:` asserts `v1.(int8)` when v1 is `uint16` — panics

**Affected function:** `multiplyByteCode`  
**File:** `bytecode/math.go`  
**Risk:** High — any multiplication of two `uint16` values panics  
**Discovered by:** `Test_multiplyByteCode_Uint16_MATH3`  
**Status: RESOLVED**

#### MATH-3: Description

Identical root cause as MATH-2 but in the `uint16` case:

```go
case uint16:
    return c.push(uint16(v1.(int8)) * uint16(v2.(uint16)))
                         ↑ BUG: v1 is uint16, not int8
```

#### MATH-3: Fix

```go
case uint16:
    // MATH-3 fix: original cast v1.(int8) panicked; v1 is uint16
    // after data.Normalize leaves two equal-kind uint16 values unchanged.
    return c.push(v1.(uint16) * v2.(uint16))
```

`Test_multiplyByteCode_Uint16` now asserts `uint16(12)` for `3 * 4`.

---

<a id="MATH-4"></a>

### MATH-4 — `subtractByteCode` `case int8:` asserts `v1.(int16)` when v1 is `int8` — panics

**Affected function:** `subtractByteCode`  
**File:** `bytecode/math.go`  
**Risk:** High — any subtraction of two `int8` values panics  
**Discovered by:** `Test_subtractByteCode_Int8_MATH4`  
**Status: RESOLVED**

#### MATH-4: Description

When two `int8` values are on the stack, `data.Normalize` left them as `int8`
(same kind).  The type switch entered `case int8:`, and the body asserted `v1.(int16)`:

```go
case int8:
    return c.push(int8(v1.(int16)) - int8(v2.(int8)))
                      ↑ BUG: v1 is int8, not int16
```

#### MATH-4: Fix

```go
case int8:
    // MATH-4 fix: original cast v1.(int16) panicked because after
    // data.Normalize both values remain int8 (same kind is unchanged).
    return c.push(v1.(int8) - v2.(int8))
```

`Test_subtractByteCode_Int8` now asserts `int8(2)` for `5 - 3`.

---

<a id="MATH-5"></a>

### MATH-5 — `divideByteCode` `case int16:` asserts `v1.(int8)` when v1 is `int16` — panics

**Affected function:** `divideByteCode`  
**File:** `bytecode/math.go`  
**Risk:** High — any division of two `int16` values panics  
**Discovered by:** `Test_divideByteCode_Int16_MATH5`  
**Status: RESOLVED**

#### MATH-5: Description

```go
case int16:
    ...
    return c.push(int16(v1.(int8)) / int16(v2.(int16)))
                         ↑ BUG: v1 is int16, not int8
```

#### MATH-5: Fix

```go
case int16:
    if v2.(int16) == 0 {
        return c.runtimeError(errors.ErrDivisionByZero)
    }
    // MATH-5 fix: original cast v1.(int8) panicked; v1 is int16.
    return c.push(v1.(int16) / v2.(int16))
```

`Test_divideByteCode_Int16` now asserts `int16(3)` for `9 / 3`.

---

<a id="MATH-6"></a>

### MATH-6 — `divideByteCode` `case uint16:` asserts `v1.(int8)` when v1 is `uint16` — panics

**Affected function:** `divideByteCode`  
**File:** `bytecode/math.go`  
**Risk:** High — any division of two `uint16` values panics  
**Discovered by:** `Test_divideByteCode_Uint16_MATH6`  
**Status: RESOLVED**

#### MATH-6: Description

```go
case uint16:
    ...
    return c.push(uint16(v1.(int8)) / uint16(v2.(uint16)))
                          ↑ BUG: v1 is uint16, not int8
```

#### MATH-6: Fix

```go
case uint16:
    if v2.(uint16) == 0 {
        return c.runtimeError(errors.ErrDivisionByZero)
    }
    // MATH-6 fix: original cast v1.(int8) panicked; v1 is uint16.
    return c.push(v1.(uint16) / v2.(uint16))
```

`Test_divideByteCode_Uint16` now asserts `uint16(3)` for `9 / 3`.

---

<a id="MATH-7"></a>

### MATH-7 — `moduloByteCode` `case int16:` asserts `v1.(int8)` when v1 is `int16` — panics

**Affected function:** `moduloByteCode`  
**File:** `bytecode/math.go`  
**Risk:** High — any modulo of two `int16` values panics  
**Discovered by:** `Test_moduloByteCode_Int16_MATH7`  
**Status: RESOLVED**

#### MATH-7: Description

```go
case int16:
    ...
    return c.push(int16(v1.(int8)) % int16(v2.(int16)))
                         ↑ BUG: v1 is int16, not int8
```

#### MATH-7: Fix

```go
case int16:
    if v2.(int16) == 0 {
        return c.runtimeError(errors.ErrDivisionByZero)
    }
    // MATH-7 fix: original cast v1.(int8) panicked; v1 is int16.
    return c.push(v1.(int16) % v2.(int16))
```

`Test_moduloByteCode_Int16` now asserts `int16(1)` for `10 % 3`.

---

<a id="MATH-8"></a>

### MATH-8 — `moduloByteCode` `case uint16:` asserts `v1.(int8)` when v1 is `uint16` — panics

**Affected function:** `moduloByteCode`  
**File:** `bytecode/math.go`  
**Risk:** High — any modulo of two `uint16` values panics  
**Discovered by:** `Test_moduloByteCode_Uint16_MATH8`  
**Status: RESOLVED**

#### MATH-8: Description

```go
case uint16:
    ...
    return c.push(uint16(v1.(int8)) % uint16(v2.(uint16)))
                          ↑ BUG: v1 is uint16, not int8
```

#### MATH-8: Fix

```go
case uint16:
    if v2.(uint16) == 0 {
        return c.runtimeError(errors.ErrDivisionByZero)
    }
    // MATH-8 fix: original cast v1.(int8) panicked; v1 is uint16.
    return c.push(v1.(uint16) % v2.(uint16))
```

`Test_moduloByteCode_Uint16` now asserts `uint16(1)` for `10 % 3`.

---

<a id="MATH-9"></a>

### MATH-9 — `notByteCode` multi-type case returns wrong result for zero values of non-`int` integer types

**Affected function:** `notByteCode`  
**File:** `bytecode/math.go`  
**Risk:** Medium — `!byte(0)`, `!int64(0)`, `!int32(0)`, `!int16(0)`, etc. all
return `false` instead of `true`  
**Discovered by:** `Test_notByteCode_Int64Zero_CurrentlyBroken_MATH9`,
`Test_notByteCode_ByteZero_CurrentlyBroken_MATH9`,
`Test_notByteCode_Int32Zero_CurrentlyBroken_MATH9`  
**Status: RESOLVED**

#### MATH-9: Description

`notByteCode` used a multi-type case to handle all integer types at once:

```go
case byte, int8, int32, int16, uint32, uint16, uint, uint64, int, int64:
    return c.push(value == 0)
```

When multiple types appear in a single `case`, Go types the case variable (`value`)
as `any` (interface{}).  The comparison `value == 0` compiles as an interface
comparison where the untyped constant `0` takes its default type `int`.

In Go, two interface values are equal only when both their **dynamic type** and
**dynamic value** match.  So:

| Stack value | `value == 0` evaluated as | Result |
| :---------- | :------------------------ | :----- |
| `int(0)` | `int(0) == int(0)` | `true` ✓ |
| `int64(0)` | `int64(0) == int(0)` | `false` ✗ |
| `byte(0)` | `byte(0) == int(0)` | `false` ✗ |
| `int32(0)` | `int32(0) == int(0)` | `false` ✗ |

Only `int(0)` gave the correct answer.

#### MATH-9: Fix

Replaced the single multi-type case with ten individual single-type cases.  In a
single-type case, the switch variable takes the matched concrete type, so
`value == 0` uses the correctly-typed zero literal for each width:

```go
// MATH-9 fix: splitting into individual cases makes 'value' typed,
// so value == 0 uses the correctly-typed zero literal.

case byte:
    return c.push(value == 0)   // value is byte; 0 → byte(0)

case int8:
    return c.push(value == 0)   // value is int8; 0 → int8(0)

case int16:
    return c.push(value == 0)
// ... (int32, uint16, uint32, int, uint, int64, uint64 follow the same pattern)
```

The three `_CurrentlyBroken_MATH9` tests were renamed to drop the suffix and
updated to assert `true` for zero values of each affected type.

---

<a id="MATH-10"></a>

### MATH-10 — `addByteCode` function comment incorrectly says "OR" for boolean operands

**Affected function:** `addByteCode`  
**File:** `bytecode/math.go`  
**Risk:** None — documentation only; the implementation is correct  
**Discovered by:** `Test_addByteCode_BoolAND_TrueAndTrue`,
`Test_addByteCode_BoolAND_MixedValues`  
**Status: RESOLVED**

#### MATH-10: Description

The function-level comment on `addByteCode` said "OR" but the implementation
performs logical AND (`&&`):

```go
case bool:
    return c.push(v1.(bool) && v2.(bool))
```

#### MATH-10: Fix

The comment was corrected and a cross-reference to `multiplyByteCode` (which
performs OR) was added:

```go
// addByteCode bytecode instruction processor. This removes the top two
// items and adds them together. For boolean values, this is an AND (&&)
// operation — note that multiplyByteCode performs OR (||) for booleans.
// MATH-10 fix: the previous comment incorrectly said "OR"; the implementation
// uses && (AND), which is what this comment now reflects.
```

---

<a id="MATH-11"></a>

### MATH-11 — `notByteCode` and `negateByteCode` return raw (undecorated) errors from `c.Pop()`

**Affected functions:** `notByteCode`, `negateByteCode`  
**File:** `bytecode/math.go`  
**Risk:** Low — stack-underflow errors from these two functions lack the module
name and source-line annotation that all other runtime errors carry  
**Discovered by:** code review during `math_test.go` comprehensive audit  
**Status: RESOLVED**

#### MATH-11: Description

Both `notByteCode` and `negateByteCode` returned raw errors from `c.Pop()` /
`c.PopWithoutUnwrapping()` without wrapping them in `c.runtimeError(err)`.

#### MATH-11: Fix

Both call sites now wrap via `c.runtimeError`, matching the pattern used in all
other bytecode instruction functions and previously fixed in COMPARE-4 and LOAD-1:

```go
// notByteCode:
v, err = c.Pop()
if err != nil {
    // MATH-11 fix: decorate the error with module/line info.
    return c.runtimeError(err)
}

// negateByteCode:
v, err := c.PopWithoutUnwrapping()
if err != nil {
    // MATH-11 fix: wrap with c.runtimeError to attach source-location info.
    return c.runtimeError(err)
}
```

---

<a id="members"></a>

## MEMBERS — Member Access

| ID | Summary | Status |
| :-- | :-- | :-- |
| [MEMBERS-1](#MEMBERS-1) | `memberByteCode` returns raw errors from `c.Pop()` without `c.runtimeError` decoration | ✓ |
| [MEMBERS-2](#MEMBERS-2) | `getMemberValue` returns raw `ErrFunctionReturnedVoid` when stack marker detected | ✓ |
| [MEMBERS-3](#MEMBERS-3) | `getStructMemberValue` returns raw errors without `c.runtimeError` decoration | ✓ |
| [MEMBERS-4](#MEMBERS-4) | `memberByteCode` doc comment says "second a map" but the function handles many types | ✓ |
| [MEMBERS-5](#MEMBERS-5) | `getPackageMemberValue` signature includes dead parameters `v any` and `found bool` | ✓ |
| [MEMBERS-6](#MEMBERS-6) | `getMemberValue` ignores the member name when the object is `*data.Type` | ✓ |
| [MEMBERS-7](#MEMBERS-7) | `getMemberValue` silently returns `(nil, nil)` for a nil `*data.Type` behind `*any` | ✓ |

<a id="MEMBERS-1"></a>

### MEMBERS-1 — `memberByteCode` returns raw errors from `c.Pop()` without `c.runtimeError` decoration

**Affected function:** `memberByteCode`  
**File:** `bytecode/member.go`  
**Risk:** Low — stack-underflow errors from the two Pop calls in `memberByteCode`
lack the module name and source-line annotation that all other runtime errors carry  
**Discovered by:** `Test_memberByteCode_StackUnderflow_EmptyStack`,
`Test_memberByteCode_StackUnderflow_OperandButEmptyStack`  
**Status: RESOLVED**

#### MEMBERS-1: Description

`memberByteCode` made two `c.Pop()` calls — one for the member name (when the
operand is nil) and one for the object.  Both error paths returned the raw error
without `c.runtimeError()` wrapping, so stack-underflow errors lacked module/line
info.  This is the same pattern fixed in MATH-11, COMPARE-4, and LOAD-1.

#### MEMBERS-1: Fix

Both error paths now use `c.runtimeError()`:

```go
// name pop — MEMBERS-1 fix:
v, err = c.Pop()
if err != nil {
    return c.runtimeError(err)
}

// object pop — MEMBERS-1 fix:
m, err = c.Pop()
if err != nil {
    return c.runtimeError(err)
}
```

---

<a id="MEMBERS-2"></a>

### MEMBERS-2 — `getMemberValue` returns raw `ErrFunctionReturnedVoid` when stack marker detected

**Affected function:** `getMemberValue`  
**File:** `bytecode/member.go`  
**Risk:** Low — the error that reaches user code lacks source-location information  
**Discovered by:** `Test_memberByteCode_StackMarkerAsObject`  
**Status: RESOLVED**

#### MEMBERS-2: Description

When `getMemberValue` detected a `StackMarker` as the object, it returned the
error bare — inconsistent with all other error paths in the file.

#### MEMBERS-2: Fix

```go
// MEMBERS-2 fix: was returning errors.ErrFunctionReturnedVoid bare.
if isStackMarker(m) {
    return nil, c.runtimeError(errors.ErrFunctionReturnedVoid)
}
```

---

<a id="MEMBERS-3"></a>

### MEMBERS-3 — `getStructMemberValue` returns raw errors without `c.runtimeError` decoration

**Affected function:** `getStructMemberValue`  
**File:** `bytecode/member.go`  
**Risk:** Low — struct member-access errors (`ErrUnknownMember`,
`ErrSymbolNotExported`) lack module/line info  
**Discovered by:** `Test_memberByteCode_Struct_FieldNotFound_Error`,
`Test_memberByteCode_Struct_UnexportedField_OtherPackage_Error`  
**Status: RESOLVED**

#### MEMBERS-3: Description

Both error returns in `getStructMemberValue` were bare, so struct member-access
errors lacked source-location context.

#### MEMBERS-3: Fix

Both returns now use `c.runtimeError()`:

```go
// MEMBERS-3 fix:
return nil, c.runtimeError(errors.ErrUnknownMember).Context(name)
return nil, c.runtimeError(errors.ErrSymbolNotExported).Context(name)
```

---

<a id="MEMBERS-4"></a>

### MEMBERS-4 — `memberByteCode` doc comment says "second a map" but the function handles many types

**Affected function:** `memberByteCode`  
**File:** `bytecode/member.go`  
**Risk:** None — documentation only; the implementation is correct  
**Discovered by:** code review during `member_test.go` comprehensive audit  
**Status: RESOLVED**

#### MEMBERS-4: Description

The original doc comment read:

```go
// memberByteCode instruction processor. This pops two values from
// the stack (the first must be a string and the second a
// map) and indexes into the map to get the matching value
// and puts back on the stack.
```

"The second a map" is incorrect.  The function actually dispatches over
`*data.Struct`, `*data.Package`, `*data.Map` (only with extensions), `*any`,
`*data.Type`, and native Go types.  The name also does not have to come from the
stack — it can come from the instruction operand.

#### MEMBERS-4: Fix

The comment was rewritten to accurately describe both name-source paths and all
supported object types.  See the updated `memberByteCode` doc comment in
`bytecode/member.go`.

---

<a id="MEMBERS-5"></a>

### MEMBERS-5 — `getPackageMemberValue` signature includes dead parameters `v any` and `found bool`

**Affected function:** `getPackageMemberValue`  
**File:** `bytecode/member.go`  
**Risk:** None — no correctness impact; the parameters are always zero-valued
when passed and are immediately overwritten inside the function  
**Discovered by:** code review during `member_test.go` comprehensive audit  
**Status: RESOLVED**

#### MEMBERS-5: Description

`getPackageMemberValue` had two dead parameters — `v any` and `found bool` — that
were always passed as zero values and immediately overwritten inside the function.

#### MEMBERS-5: Fix

The dead parameters were removed.  New signature:

```go
// MEMBERS-5 fix: removed dead parameters v any and found bool.
func getPackageMemberValue(name string, mv *data.Package, c *Context) (any, error)
```

The single call site in `getMemberValue` was updated accordingly:

```go
case *data.Package:
    return getPackageMemberValue(name, mv, c)
```

The function now declares its own local `v` and `found` variables.

---

<a id="MEMBERS-6"></a>

### MEMBERS-6 — `getMemberValue` ignores the member name when the object is `*data.Type`

**Affected function:** `getMemberValue`  
**File:** `bytecode/member.go`  
**Risk:** Medium — any named member access on a `*data.Type` value silently
returns the type's string representation instead of the requested member's value
or an error  
**Discovered by:** `Test_memberByteCode_TypeObject_ReturnsTypeName_MEMBERS6`,
`Test_memberByteCode_TypeObject_IntType_MEMBERS6`  
**Status: RESOLVED**

#### MEMBERS-6: Description

The original `*data.Type` fast path in `getMemberValue` returned `t.String()` for
every member access, completely ignoring the `name` parameter:

```go
// Original (buggy):
if t, ok := m.(*data.Type); ok {
    v = t.String()
    return v, nil   // name was never consulted
}
```

This meant `someType.NonExistent` succeeded silently and pushed the type name
string onto the stack rather than returning an error.

#### MEMBERS-6: Fix

The fast path now performs a proper member lookup via `t.Function(name)`.  If a
function is registered on the type under that name it is returned; otherwise
`ErrUnknownMember` is reported:

```go
// MEMBERS-6 fix:
if t, ok := m.(*data.Type); ok {
    if fn := t.Function(name); fn != nil {
        return data.UnwrapConstant(fn), nil
    }
    return nil, c.runtimeError(errors.ErrUnknownMember).Context(name)
}
```

The two `_MEMBERS6` tests were renamed and updated:

- `Test_memberByteCode_TypeObject_UnregisteredName_Error` — asserts `ErrUnknownMember`
- `Test_memberByteCode_TypeObject_RegisteredFunction_OK` — positive case showing a registered function IS returned
- `Test_memberByteCode_PtrAny_PointingToType_UnregisteredName` — the *any recursive path also now returns `ErrUnknownMember`

---

<a id="MEMBERS-7"></a>

### MEMBERS-7 — `getMemberValue` silently returns `(nil, nil)` for a nil `*data.Type` behind `*any`

**Affected function:** `getMemberValue`  
**File:** `bytecode/member.go`  
**Risk:** Low — the nil `*data.Type` behind a `*any` case is an edge case
unlikely to appear in well-formed Ego code, but it causes a silent nil push  
**Discovered by:** `Test_memberByteCode_PtrAny_PointingToNilType_MEMBERS7`  
**Status: RESOLVED**

#### MEMBERS-7: Description

When the value behind a `*any` was a nil `*data.Type`, `BaseType()` returned nil,
the `bv != nil` guard failed, and no return statement executed.  Control fell
through to the end of `getMemberValue`, which returned `(nil, nil)`.
`memberByteCode` then pushed nil onto the stack with no error.

#### MEMBERS-7: Fix

An explicit error return was added for the nil-BaseType path:

```go
case *data.Type:
    if bv := actual.BaseType(); bv != nil {
        return getMemberValue(c, bv, name)
    }
    // MEMBERS-7 fix: return an error instead of falling through with (nil, nil).
    return nil, c.runtimeError(errors.ErrInvalidType).Context("nil type")
```

`Test_memberByteCode_PtrAny_PointingToNilType` (renamed from the `_MEMBERS7`
form) now asserts `ErrInvalidType`.

---

<a id="optimizer"></a>

## OPTIMIZER — Bytecode Optimizer

| ID | Summary | Status |
| :-- | :-- | :-- |
| [OPTIMIZER-1](#OPTIMIZER-1) | Branch-target scan is O(n²): pre-build a target set instead | ✓ |
| [OPTIMIZER-2](#OPTIMIZER-2) | `reflect.DeepEqual` for operand comparison is unnecessarily expensive | ✓ |
| [OPTIMIZER-3](#OPTIMIZER-3) | No opcode-indexed dispatch: all rules tried at every position | ✓ |
| [OPTIMIZER-4](#OPTIMIZER-4) | Backtracking by `maxPatternSize` instead of the matched pattern size | ✓ |
| [OPTIMIZER-5](#OPTIMIZER-5) | `Patch` corrupts the instruction array when the replacement is longer than the deleted region | ✓ |
| [OPTIMIZER-6](#OPTIMIZER-6) | `continue` after `found=false` in placeholder mismatch should be `break` | ✓ |
| [OPTIMIZER-7](#OPTIMIZER-7) | `executeFragment` creates full interpreter context for trivial constant folding | ✓ |
| [OPTIMIZER-8](#OPTIMIZER-8) | `data.Int` failure in branch-check scan aborts the entire optimization pass | ✓ |
| [OPTIMIZER-9](#OPTIMIZER-9) | Dead `else if` condition in placeholder consistency check | ✓ |

<a id="OPTIMIZER-1"></a>

### OPTIMIZER-1 — Branch-target scan is O(n²): pre-build a target set instead

**Affected function:** `optimize`  
**File:** `bytecode/optimizer.go`  
**Risk:** Performance — for large bytecode bodies the optimizer is dominated by
this scan; it is the primary reason optimizer mode 1 (conditional) skips
short sequences  
**Status: RESOLVED**

#### OPTIMIZER-1: Description

Inside the main `optimize` loop, for every position `idx` and every
optimization rule, the code performed a linear scan over **all** instructions
to check whether any branch instruction targeted an address inside the candidate
pattern window — O(n²m) total cost.

#### OPTIMIZER-1: Fix

A `map[int]bool` of branch target addresses (`branchTargets`) is now built
lazily: it is populated before the first outer-loop iteration and rebuilt
after every `Patch` call (which adjusts branch operands throughout the stream).
A `needsRebuild` flag controls when the rebuild fires.

Inside the rule loop, the branch-target check is now O(patternLen):

```go
for offset := 0; offset < len(optimization.Pattern); offset++ {
    if branchTargets[idx+offset] {
        found = false
        break
    }
}
```

Malformed branch operands (non-integer) are logged and skipped instead of
aborting the pass (subsumed from OPTIMIZER-8).

---

<a id="OPTIMIZER-2"></a>

### OPTIMIZER-2 — `reflect.DeepEqual` for operand comparison is unnecessarily expensive

**Affected function:** `optimize`  
**File:** `bytecode/optimizer.go`  
**Risk:** Performance — `reflect.DeepEqual` is called for every non-string,
non-placeholder operand pair; it uses reflection even for simple integers  
**Status: RESOLVED**

#### OPTIMIZER-2: Description

The pattern-match loop used `reflect.DeepEqual` for every non-string,
non-placeholder operand comparison.

#### OPTIMIZER-2: Fix

The dedicated `operandEqual(a, b any) bool` helper was added.  It uses a
type-switch for `int`, `int64`, `float64`, `bool`, `string`, and `nil`,
falling back to `reflect.DeepEqual` only for composite types such as
`StackMarker` (which contains a `[]any` field and cannot be compared with `==`
directly).  The separate fast-path string check in the old match loop was
removed; `operandEqual` subsumes it.  `reflect` is still imported for the
fallback path.

---

<a id="OPTIMIZER-3"></a>

### OPTIMIZER-3 — No opcode-indexed dispatch: all rules tried at every position

**Affected function:** `optimize`  
**File:** `bytecode/optimizer.go`  
**Risk:** Performance — redundant work proportional to (number of rules) ×
(fraction of instructions that cannot start any pattern)  
**Status: RESOLVED**

#### OPTIMIZER-3: Description

The inner loop tried every enabled optimization rule at every position,
regardless of whether the current opcode could possibly start any of those
rules.

#### OPTIMIZER-3: Fix

`rulesByFirstOpcode` (a `map[Opcode][]int`) is built once before the main
loop in the same pass that computes `maxPatternSize`.  At each position, only
the rules whose first-pattern opcode matches the current instruction are
tried:

```go
candidates := rulesByFirstOpcode[b.instructions[idx].Operation]
for _, ruleIdx := range candidates {
    optimization := optimizations[ruleIdx]
    ...
}
```

The rule loop is also broken out of immediately when a match fires, so the
outer loop controls the retry position cleanly (previously the inner loop
kept running with the backed-up `idx` but was iterating a pre-filtered slice
for the old opcode).

---

<a id="OPTIMIZER-4"></a>

### OPTIMIZER-4 — Backtracking by `maxPatternSize` instead of the matched pattern size

**Affected function:** `optimize`  
**File:** `bytecode/optimizer.go`  
**Risk:** Performance — after a small pattern fires, the scanner may revisit
instructions that were already checked  
**Status: RESOLVED**

#### OPTIMIZER-4: Description

After a successful substitution the scanner originally backed up by
`maxPatternSize + 1` (extra -1 in the formula plus the loop's `idx++`),
which was one step more than necessary.

#### OPTIMIZER-4: Fix and correction to proposed approach

The proposed fix (retreat by matched pattern length) is not safe in general:
a rule with `maxPatternSize` instructions can start up to `maxPatternSize - 1`
positions *before* the match and overlap the replacement.  Using the smaller
matched length would miss those opportunities.

The correct minimum retreat is `maxPatternSize - 1`.  The old code used
`maxPatternSize` (one extra step).  The formula was changed to:

```go
idx -= maxPatternSize - 1
if idx < -1 {
    idx = -1  // after loop's idx++, resumes at 0
}
```

This is the exact minimum: a `maxPatternSize`-instruction rule starting at
`idx - (maxPatternSize - 1)` is the earliest rule that could overlap the new
replacement.  Combined with OPTIMIZER-3 (opcode dispatch), the extra iterations
at re-examined positions are near-free anyway.

---

<a id="OPTIMIZER-5"></a>

### OPTIMIZER-5 — `Patch` corrupts the instruction array when the replacement is longer than the deleted region

**Affected function:** `Patch`  
**File:** `bytecode/optimizer.go`  
**Risk:** Latent correctness — no current optimization triggers this; all rules
shrink or preserve the instruction count  
**Status: RESOLVED**

#### OPTIMIZER-5: Description

The old two-append splice pattern corrupted the instruction-array tail when
`len(insert) > deleteSize`, because the first append would overwrite positions
`start+deleteSize` and beyond before the second append captured that tail.

#### OPTIMIZER-5: Fix

`Patch` now explicitly copies the tail into a fresh slice before any
appending, then assembles the final slice from scratch:

```go
tail := make([]instruction, b.nextAddress-tailStart)
copy(tail, b.instructions[tailStart:b.nextAddress])

instructions := make([]instruction, 0, newLen)
instructions = append(instructions, b.instructions[:start]...)
instructions = append(instructions, insert...)
instructions = append(instructions, tail...)
```

This is safe for any relative sizes of `insert` and `deleteSize`.

---

<a id="OPTIMIZER-6"></a>

### OPTIMIZER-6 — `continue` after `found=false` in placeholder mismatch should be `break`

**Affected function:** `optimize` (inner pattern-match loop)  
**File:** `bytecode/optimizer.go`  
**Risk:** Minor performance — after a mismatch is detected, the inner loop
wastes time checking additional pattern positions  
**Status: RESOLVED**

#### OPTIMIZER-6: Description

The placeholder consistency check used `continue` after setting `found = false`,
causing the inner pattern loop to keep checking more instructions even though
the match was already known to have failed.

#### OPTIMIZER-6: Fix

The entire consistency-check block was simplified as part of OPTIMIZER-9 (see
below).  The resulting code uses `break` uniformly:

```go
if value.Value != i.Operand {
    found = false
    break
}
```

All three early-exit paths in the inner pattern loop now use `break`
consistently.

---

<a id="OPTIMIZER-7"></a>

### OPTIMIZER-7 — `executeFragment` creates full interpreter context for trivial constant folding

**Affected function:** `executeFragment`  
**File:** `bytecode/optimizer.go`  
**Risk:** Performance — each constant-fold optimization allocates a ByteCode,
SymbolTable, and Context object and runs the full interpreter for 2–3
instructions  
**Status: RESOLVED**

#### OPTIMIZER-7: Description

Every constant-fold optimization (Add, Sub, Mul) previously called
`executeFragment` which builds a full interpreter context even for trivially
simple arithmetic.

#### OPTIMIZER-7: Fix

`tryConstantArithmetic(op Opcode, v1, v2 any) (any, bool)` was added.  It
handles `int`, `int64`, `float64`, and string concatenation directly with
type assertions and native Go arithmetic — no allocations beyond the return
value.  The `optRunConstantFragment` handler in the replacement loop tries
this fast path first:

```go
if result, ok := tryConstantArithmetic(arithOp, v1, v2); ok {
    newInstruction.Operand = result
} else {
    v, err := b.executeFragment(idx, idx+patLen)
    ...
}
```

`executeFragment` is retained as the fallback for type-alias operands and any
non-numeric types that reach the constant-fold rules.

---

<a id="OPTIMIZER-8"></a>

### OPTIMIZER-8 — `data.Int` failure in branch-check scan aborts the entire optimization pass

**Affected function:** `optimize`  
**File:** `bytecode/optimizer.go`  
**Risk:** Low correctness concern — malformed bytecode (branch with non-integer
operand) causes the entire optimization pass to fail instead of simply skipping
that branch  
**Status: RESOLVED**

#### OPTIMIZER-8: Description

A malformed branch operand (non-integer) caused `optimize` to return an error
and abort the entire optimization pass.

#### OPTIMIZER-8: Fix

Subsumed by OPTIMIZER-1.  When the `branchTargets` set is built, a malformed
branch operand is now logged and simply omitted from the set rather than
aborting the pass:

```go
if dest, err := data.Int(i.Operand); err == nil {
    branchTargets[dest] = true
} else {
    ui.Log(ui.OptimizerLogger, "optimizer.branch.malformed", ui.A{"operand": i.Operand})
}
```

Omitting the address is conservative: a pattern overlapping that address is
not rejected (the entry is absent), but in practice the address will never
coincide with a valid pattern window unless the bytecode is severely malformed.

---

<a id="OPTIMIZER-9"></a>

### OPTIMIZER-9 — Dead `else if` condition in placeholder consistency check

**Affected function:** `optimize`  
**File:** `bytecode/optimizer.go`  
**Risk:** None — code clarity only; the condition is always true for real
bytecode  
**Status: RESOLVED**

#### OPTIMIZER-9: Description

The placeholder consistency check contained a dead `else if` whose condition
(`i.Operand != sourceInstruction.Operand`) is always true for real bytecode
because real operands are never `placeholder` structs.

#### OPTIMIZER-9: Fix

The entire `if value.Value == … { } else if … { }` chain was replaced with a
direct negation, and `continue` was changed to `break` (OPTIMIZER-6):

```go
if value.Value != i.Operand {
    found = false
    break
}
```

The `inMap` true-branch now has a single clear code path: if the previously
captured value matches the current operand, we continue silently; if not, we
reject the match and short-circuit.

---

<a id="range"></a>

## RANGE — Range Loops

| ID | Summary | Status |
| :-- | :-- | :-- |
| [RANGE-1](#RANGE-1) | `rangeNextInteger` unconditionally calls `c.symbols.Set` without guarding empty or discarded variable names | ✓ |
| [RANGE-2](#RANGE-2) | `rangeNextByteCode` default case does not pop the range stack | ✓ |
| [RANGE-3](#RANGE-3) | Map remains readonly if a for-range loop exits before exhaustion | ✓ |
| [RANGE-4](#RANGE-4) | `rangeNextByteCode` returns a raw error for the `[]any` case without `c.runtimeError()` decoration | ✓ |
| [RANGE-5](#RANGE-5) | `rangeInitByteCode` appends a stale entry to `rangeStack` even when the type-switch default sets an error | ✓ |

<a id="RANGE-1"></a>

### RANGE-1 — `rangeNextInteger` unconditionally calls `c.symbols.Set` without guarding empty or discarded variable names

**Affected function:** `rangeNextInteger`  
**File:** `bytecode/range.go`  
**Risk:** High — any for-range loop over an integer where the index variable is
discarded (`_`) or absent (`""`) fails at runtime with `ErrUnknownSymbol`
instead of silently skipping the assignment  
**Discovered by:** `Test_rangeNextInteger_DiscardedIndex_CurrentlyBroken_RANGE1`,
`Test_rangeNextInteger_EmptyIndexName_CurrentlyBroken_RANGE1`  
**Status: RESOLVED**

#### RANGE-1: Description

Every `rangeNext*` helper except `rangeNextInteger` guards the index-variable
write with:

```go
if r.indexName != "" && r.indexName != defs.DiscardedVariable {
    err = c.symbols.Set(r.indexName, r.index)
}
```

`rangeNextInteger` was missing this guard, so discarded or absent index names
caused `ErrUnknownSymbol` instead of silently skipping the assignment.

#### RANGE-1: Fix

Added the same guard to `rangeNextInteger`:

```go
// Only store the index when the caller declared a real variable for it.
// Skip when the name is "" (not declared) or "_" (deliberately discarded).
if r.indexName != "" && r.indexName != defs.DiscardedVariable {
    err = c.symbols.Set(r.indexName, r.index)
}
```

Tests renamed from `_CurrentlyBroken_RANGE1` to `_RANGE1`; both now assert
`nil` error and verify the full iteration completes successfully.

---

<a id="RANGE-2"></a>

### RANGE-2 — `rangeNextByteCode` default case does not pop the range stack

**Affected function:** `rangeNextByteCode`  
**File:** `bytecode/range.go`  
**Risk:** Low — only reachable with a value type that bypassed `rangeInitByteCode`'s
type guard; in practice this path is not reached by well-formed Ego programs  
**Discovered by:** `Test_rangeNextByteCode_DefaultCase_LeavesStaleStackEntry_RANGE2`  
**Status: RESOLVED**

#### RANGE-2: Description

The `default` case in `rangeNextByteCode` set `c.programCounter = destination`
but did not trim `c.rangeStack`, leaving a stale entry that could corrupt an
outer loop's state on its next `RangeNext` call.

#### RANGE-2: Fix

Added the stack trim to the default case, matching every other exhaustion path:

```go
default:
    c.programCounter = destination
    c.rangeStack = c.rangeStack[:stackSize-1]  // added
```

Test renamed from `_LeavesStaleStackEntry_RANGE2` to `_PopsRangeStack_RANGE2`;
now asserts `len(tc.ctx.rangeStack) == 0`.

---

<a id="RANGE-3"></a>

### RANGE-3 — Map remains readonly if a for-range loop exits before exhaustion

**Affected functions:** `rangeInitByteCode`, `rangeNextMap`, `popScopeByteCode`  
**Files:** `bytecode/range.go`, `bytecode/symbols.go`  
**Risk:** Medium — a map used inside a for-range loop cannot be modified for
the rest of the function scope if the loop body exits early via `break` or
`return`  
**Discovered by:** `Test_rangeNextMap_EarlyExitLeavesMapReadonly_RANGE3`  
**Status: RESOLVED**

#### RANGE-3: Description

`rangeInitByteCode` locked the map readonly at the start of iteration.
`rangeNextMap` unlocked it only when the iterator was exhausted.  An early
exit via `break` or `return` bypassed the exhaustion path, leaving the map
permanently locked within the function scope.

#### RANGE-3: Fix

Implemented Option B (PopScope awareness) with three coordinated changes:

**1. `rangeDefinition` — new fields** (`bytecode/range.go`):

```go
type rangeDefinition struct {
    ...
    scopeDepth int    // c.blockDepth at the time RangeInit ran
    cleanup    func() // called once when this entry is retired
}
```

A `release()` method calls cleanup exactly once (nil-clears it after the first
call to prevent double-release).

**2. `rangeInitByteCode`** — for maps, store a cleanup closure:

```go
case *data.Map:
    r.keySet = actual.Keys()
    actual.SetReadonly(true)
    r.cleanup = func() { actual.SetReadonly(false) }  // ← new
```

`r.scopeDepth = c.blockDepth` is set for all value types before pushing.

**3. `rangeNextMap`** — call `r.release()` instead of `actual.SetReadonly(false)`:

```go
if r.index >= len(r.keySet) {
    c.programCounter = destination
    c.rangeStack = c.rangeStack[:stackSize-1]
    r.release()   // ← was: actual.SetReadonly(false)
}
```

**4. `popScopeByteCode`** (`bytecode/symbols.go`) — release range entries
belonging to the scope just popped:

```go
c.blockDepth--

// Release range entries whose scope was just popped (RANGE-3 fix).
for len(c.rangeStack) > 0 && c.rangeStack[len(c.rangeStack)-1].scopeDepth > c.blockDepth {
    c.rangeStack[len(c.rangeStack)-1].release()
    c.rangeStack = c.rangeStack[:len(c.rangeStack)-1]
}
```

This fires on both `break` (which jumps to the code just before `PopScope`)
and normal exhaustion (where `release()` is a no-op because cleanup was
already called by `rangeNextMap`).

Test renamed from `_EarlyExitLeavesMapReadonly_RANGE3` to
`_EarlyExitReleasesMap_RANGE3`; now asserts the map is writable after
`popScopeByteCode` is called.

---

<a id="RANGE-4"></a>

### RANGE-4 — `rangeNextByteCode` returns a raw error for the `[]any` case without `c.runtimeError()` decoration

**Affected function:** `rangeNextByteCode`  
**File:** `bytecode/range.go`  
**Risk:** Low — the `[]any` case is dead code for well-formed programs; but if
reached, the error lacks module and source-line information  
**Discovered by:** `Test_rangeNextByteCode_SliceAnyValue_CurrentlyBroken_RANGE4`  
**Status: RESOLVED**

#### RANGE-4: Description

The `[]any` case returned a raw error without `c.runtimeError()` decoration,
losing module and source-line context inconsistently with the rest of the package.

#### RANGE-4: Fix

```go
// Before:
case []any:
    return errors.ErrInvalidType.Context("[]any")

// After:
case []any:
    return c.runtimeError(errors.ErrInvalidType)
```

The `[]any` branch is retained as a defensive guard (with an updated comment
explaining it is unreachable in well-formed programs since `rangeInitByteCode`
already rejects `[]any` values before they can reach the range stack).

Test renamed from `_CurrentlyBroken_RANGE4` to `_RANGE4`.

---

<a id="RANGE-5"></a>

### RANGE-5 — `rangeInitByteCode` appends a stale entry to `rangeStack` even when the type-switch default sets an error

**Affected function:** `rangeInitByteCode`  
**File:** `bytecode/range.go`  
**Risk:** Low — only triggered by unsupported value types (already caught by
the type guard); a try/catch block could observe the stale entry  
**Discovered by:** `Test_rangeInitByteCode_UnsupportedType`  
**Status: RESOLVED**

#### RANGE-5: Description

After the type-switch, `r.index = 0` and the `rangeStack` append ran
unconditionally even when the `default` case had already set an error.  A
`try/catch` block that caught the error and then executed `RangeNext` would
find the stale entry with an invalid value type.

#### RANGE-5: Fix

The `rangeStack` push is now guarded by `if err == nil`, and the RANGE-3
`scopeDepth` assignment was incorporated into the same block:

```go
if err == nil {
    r.index = 0
    r.scopeDepth = c.blockDepth   // for RANGE-3 cleanup tracking
    c.rangeStack = append(c.rangeStack, &r)
}
```

`Test_rangeInitByteCode_UnsupportedType` now asserts
`len(tc.ctx.rangeStack) == 0` after an unsupported type error.

---

<a id="stack"></a>

## STACK — Stack Management

| ID | Summary | Status |
| :-- | :-- | :-- |
| [STACK-1](#STACK-1) | `copyByteCode` pushes the integer literal `2` instead of the deep copy | ✓ |
| [STACK-2](#STACK-2) | `readStackByteCode` guard uses `>` instead of `>=`, causing panics on boundary indices | ✓ |
| [STACK-3](#STACK-3) | `dropByteCode` silently swallows stack underflow when dropping more items than exist | ✓ |

<a id="STACK-1"></a>

### STACK-1 — `copyByteCode` pushes the integer literal `2` instead of the deep copy

**Affected function:** `copyByteCode`  
**File:** `bytecode/stack.go`  
**Risk:** High — every call to the Copy opcode produces a wrong stack layout:
the second stack slot holds the integer `2` rather than a deep copy of the
original value; any code that reads the copy gets `2` instead of the expected
duplicate  
**Discovered by:** `Test_copyByteCode_PushesIntegerTwo_CurrentlyBroken_STACK1`  
**Status: RESOLVED**

#### STACK-1: Description

`copyByteCode` correctly marshalled the original value into `v2` via a JSON
round-trip but then pushed the integer literal `2` instead of `v2`.

#### STACK-1: Fix

Changed `c.push(2)` to `c.push(v2)`:

```go
byt, _ := json.Marshal(v)
err = json.Unmarshal(byt, &v2)
_ = c.push(v2)           // was: c.push(2)
```

A function-level comment was added noting that `json.Unmarshal` produces
`float64` for all numeric values, so the copy of an integer has a different
Go type than the original.

Test renamed from `_PushesIntegerTwo_CurrentlyBroken_STACK1` to
`_PushesDeepCopy_STACK1`; now asserts `TOS == float64(99)` after copying
`int(99)`.

---

<a id="STACK-2"></a>

### STACK-2 — `readStackByteCode` guard uses `>` instead of `>=`, causing panics on boundary indices

**Affected function:** `readStackByteCode`  
**File:** `bytecode/stack.go`  
**Risk:** High — two boundary conditions trigger an unrecoverable runtime panic
(negative slice index) instead of returning `ErrStackUnderflow`  
**Discovered by:** `Test_readStackByteCode_EmptyStack_CurrentlyBroken_STACK2`,
`Test_readStackByteCode_IndexBeyondStack_CurrentlyBroken_STACK2`  
**Status: RESOLVED**

#### STACK-2: Description

The guard `idx > c.stackPointer` missed the boundary case where
`idx == c.stackPointer`, causing `c.stack[(stackPointer-1)-idx]` to compute a
negative slice index and panic.

#### STACK-2: Fix

Changed the guard from `>` to `>=`:

```go
// was: if idx > c.stackPointer {
if idx >= c.stackPointer {
    return c.runtimeError(errors.ErrStackUnderflow)
}
```

An expanded function-level comment was added explaining the correctness
requirement: `idx` must be strictly less than `stackPointer` so the computed
slice index `(stackPointer-1)-idx` is always non-negative.

Both tests were renamed (dropping `_CurrentlyBroken_`) and updated to assert
`ErrStackUnderflow` directly rather than catching a recover() panic:

- `Test_readStackByteCode_EmptyStack_STACK2`
- `Test_readStackByteCode_IndexEqualsStackPointer_STACK2`

---

<a id="STACK-3"></a>

### STACK-3 — `dropByteCode` silently swallows stack underflow when dropping more items than exist

**Affected function:** `dropByteCode`  
**File:** `bytecode/stack.go`  
**Risk:** Low — over-drops are silently accepted; callers that rely on Drop
removing a precise number of items cannot detect that fewer items were actually
removed  
**Discovered by:** `Test_dropByteCode_SilentUnderflow_STACK3`  
**Status: RESOLVED**

#### STACK-3: Description

When the stack ran dry before `count` items had been dropped, `dropByteCode`
returned `nil` instead of propagating the underflow error, inconsistently with
every other stack-consuming function in the package.

#### STACK-3: Fix

Implemented Option A (strict): `return nil` changed to `return err` inside the
Pop loop, with an explanatory comment:

```go
for n := 0; n < count; n++ {
    if _, err = c.Pop(); err != nil {
        // Propagate stack underflow rather than silently swallowing it
        // (STACK-3 fix).
        return err
    }
}
```

The 869-test Ego integration suite confirmed that no existing program relies on
the silent over-drop behavior.

Test renamed from `_SilentUnderflow_STACK3` to `_UnderflowReturnsError_STACK3`;
now asserts `ErrStackUnderflow` rather than logging that nil was returned.

---

<a id="store"></a>

## STORE — Store Instructions

| ID | Summary | Status |
| :-- | :-- | :-- |
| [STORE-1](#STORE-1) | Misleading comment in `storeByteCode` for the readonly-prefix branch | ✓ |
| [STORE-2](#STORE-2) | `defs.DiscardedVariable` used where `defs.ReadonlyVariablePrefix` is intended | ✓ |
| [STORE-3](#STORE-3) | Scalar pointer helpers check `d.(string)` instead of target type in strict/relaxed mode | ✓ |
| [STORE-4](#STORE-4) | `storeChanByteCode` passes nil (`x`) as error context instead of the variable name | ✓ |

<a id="STORE-1"></a>

### STORE-1 — Misleading comment in `storeByteCode` for the readonly-prefix branch

**Affected function:** `storeByteCode`  
**File:** `bytecode/store.go`  
**Risk:** None — documentation only; behavior was correct  
**Discovered by:** comment review during `store_test.go` development  
**Status: RESOLVED**

#### STORE-1: Original behavior

The comment immediately before the `strings.HasPrefix` guard read:

```go
// If we are writing to the "_" variable, no action is taken.
if strings.HasPrefix(name, defs.DiscardedVariable) {
    return c.set(name, data.Constant(value))
}
```

This was wrong on two counts:

1. The `name == "_"` (exact discard) case had already been handled and returned
   `nil` four lines earlier.  The `HasPrefix` guard handles all OTHER names that
   start with `"_"` (e.g., `"_foo"`, `"_bar"`).
2. "No action is taken" is the opposite of the actual behavior: the function
   **does** store the value, wrapping it in `data.Constant` to make it immutable.

#### STORE-1: Fix

The comment was rewritten to accurately describe both cases:

```go
// Variables whose names start with "_" (the readonly prefix) receive
// their value wrapped in data.Constant so that subsequent loads see
// an immutable value.  The readonly-existence check above already
// ensured the variable exists and holds symbols.UndefinedValue, so
// this is always the first (and only) write to the variable.
if strings.HasPrefix(name, defs.ReadonlyVariablePrefix) {
    return c.set(name, data.Constant(value))
}
```

---

<a id="STORE-2"></a>

### STORE-2 — `defs.DiscardedVariable` used where `defs.ReadonlyVariablePrefix` is intended

**Affected functions:** `storeByteCode`, `storeGlobalByteCode`, `storeAlwaysByteCode`  
**File:** `bytecode/store.go`  
**Risk:** None — both constants equal `"_"` so behavior is identical; but the
wrong constant name obscures intent and could cause confusion if either constant
is ever changed to a different value  
**Discovered by:** comment review during `store_test.go` development  
**Status: RESOLVED**

#### STORE-2: Original behavior

Three guards in `store.go` checked whether a variable name started with the
readonly prefix by comparing against `defs.DiscardedVariable`:

```go
// storeByteCode (line ~97 before fix):
if strings.HasPrefix(name, defs.DiscardedVariable) { ... }

// storeGlobalByteCode (line ~185 before fix):
if len(name) > 1 && name[0:1] == defs.DiscardedVariable { ... }

// storeAlwaysByteCode (line ~503 before fix):
if len(symbolName) > 1 && symbolName[0:1] == defs.DiscardedVariable { ... }
```

`defs.DiscardedVariable = "_"` is the blank identifier used to discard values
(as in `_ = someExpr`).  The guards are checking for the **readonly prefix**,
which is `defs.ReadonlyVariablePrefix = "_"`.  Both constants happen to be
`"_"`, so the behavior is correct today — but using the wrong constant
communicates the wrong intent.

#### STORE-2: Fix

All three guards were updated to use `defs.ReadonlyVariablePrefix`:

```go
// storeByteCode:
if strings.HasPrefix(name, defs.ReadonlyVariablePrefix) { ... }

// storeGlobalByteCode:
if len(name) > 1 && name[0:1] == defs.ReadonlyVariablePrefix { ... }

// storeAlwaysByteCode:
if len(symbolName) > 1 && symbolName[0:1] == defs.ReadonlyVariablePrefix { ... }
```

---

<a id="STORE-3"></a>

### STORE-3 — Scalar pointer helpers check `d.(string)` instead of target type in strict/relaxed mode

**Affected functions:** `storeBoolViaPointer`, `storeByteViaPointer`,
`storeInt32ViaPointer`, `storeIntViaPointer`, `storeInt64ViaPointer`,
`storeFloat64ViaPointer`, `storeFloat32ViaPointer`  
**File:** `bytecode/store.go`  
**Risk:** Medium — in strict or relaxed type-enforcement mode, storing a
correctly-typed value through its own native pointer returns `ErrInvalidVarType`
instead of succeeding; storing a string value through a numeric pointer would
then panic at the type assertion  
**Discovered by:** `Test_storeViaPointerByteCode_Float32Pointer_StrictMode`  
**Status: RESOLVED**

#### STORE-3: Original behavior

Each scalar pointer helper followed the same copy-paste pattern:

```go
func storeFloat32ViaPointer(c *Context, name string, src any, destinationPointer *float32) error {
    var err error
    d := src
    if c.typeStrictness > defs.RelaxedTypeEnforcement {
        // NoTypeEnforcement (2 > 1): coerce to target type — correct.
        d, err = data.Coerce(src, float32(0))
        if err != nil { return c.runtimeError(err) }
    } else if _, ok := d.(string); !ok {   // ← BUG: should be d.(float32)
        return c.runtimeError(errors.ErrInvalidVarType).Context(name)
    }
    *destinationPointer = d.(float32)  // panics if d is a string
    return nil
}
```

The `else` branch checked `d.(string)` regardless of the helper's target type:

| Scenario | Expected | Actual (buggy) |
| :------- | :------- | :----- |
| Store `float32(3.14)` through `*float32` in strict mode | success | `ErrInvalidVarType` (float32 ≠ string) |
| Store `string("3.14")` through `*float32` in strict mode | `ErrInvalidVarType` | panic on `d.(float32)` |

#### STORE-3: Fix

The `d.(string)` assertion was replaced with the correct target type in each
helper:

```go
// storeFloat32ViaPointer:
} else if _, ok := d.(float32); !ok { ... }

// storeFloat64ViaPointer:
} else if _, ok := d.(float64); !ok { ... }

// storeBoolViaPointer:
} else if _, ok := d.(bool); !ok { ... }

// storeByteViaPointer:
} else if _, ok := d.(byte); !ok { ... }

// storeInt32ViaPointer:
} else if _, ok := d.(int32); !ok { ... }

// storeIntViaPointer:
} else if _, ok := d.(int); !ok { ... }

// storeInt64ViaPointer:
} else if _, ok := d.(int64); !ok { ... }
```

The original documentation test was replaced with four targeted tests:

- `Test_storeViaPointerByteCode_Float32Pointer_StrictMode` — float32 accepted in strict mode
- `Test_storeViaPointerByteCode_Float32Pointer_RelaxedMode` — float32 accepted in relaxed mode
- `Test_storeViaPointerByteCode_Float32Pointer_StrictMode_WrongType` — float64 rejected in strict mode
- `Test_storeViaPointerByteCode_BoolPointer_StrictMode` and `_IntPointer_StrictMode` — additional type coverage

---

<a id="STORE-4"></a>

### STORE-4 — `storeChanByteCode` passes nil (`x`) as error context instead of the variable name

**Affected function:** `storeChanByteCode`  
**File:** `bytecode/store.go`  
**Risk:** Low — the error is still returned with the correct error key
(`ErrUnknownIdentifier`); only the context string in the error message is wrong  
**Discovered by:** `Test_storeChanByteCode_NonChanDestVarNotFound`  
**Status: RESOLVED**

#### STORE-4: Original behavior

When the stack value is not a channel and the destination variable does not
exist, `storeChanByteCode` built the error using `x` (the un-found value, which
is always `nil`):

```go
x, found := c.get(variableName)
if !found {
    if sourceChan {
        err = c.create(variableName)
    } else {
        err = c.runtimeError(errors.ErrUnknownIdentifier).Context(x)  // x is nil
    }
}
```

The error message read `"unknown identifier: <nil>"` instead of
`"unknown identifier: missing"`.

#### STORE-4: Fix

`.Context(x)` was replaced with `.Context(variableName)`:

```go
err = c.runtimeError(errors.ErrUnknownIdentifier).Context(variableName)
```

`Test_storeChanByteCode_NonChanDestVarNotFound` was strengthened to assert
both the error key and that the variable name `"missing"` appears in the
error message via `strings.Contains`.

---

<a id="structs"></a>

## STRUCT — Struct, Map, Array, Channel Indexing

| ID | Summary | Status |
| :-- | :-- | :-- |
| [STRUCT-1](#STRUCT-1) | Dead code check in `storeInPackage` is unreachable | ✓ |
| [STRUCT-2](#STRUCT-2) | `storeIndexByteCode` mutates a struct field before checking package visibility | ✓ |
| [STRUCT-3](#STRUCT-3) | `flattenByteCode` produces incorrect `argCountDelta` for empty arrays | ✓ |

<a id="STRUCT-1"></a>

### STRUCT-1 — Dead code check in `storeInPackage` is unreachable

**Affected function:** `storeInPackage`  
**File:** `bytecode/structs.go`  
**Risk:** None — dead code only; behavior is correct  
**Discovered by:** `Test_storeInPackage_NoDiscardedVariableCheck_STRUCT1`  
**Status: RESOLVED**

#### STRUCT-1: Original behavior

`storeInPackage` contained two consecutive guards on the `name` parameter:

```go
// Guard 1: reject unexported (lowercase) names.
if !egostrings.HasCapitalizedName(name) {
    return c.runtimeError(errors.ErrSymbolNotExported, pkg.Name+"."+name)
}

// Guard 2 (dead code): reject names starting with "_".
if name[0:1] == defs.DiscardedVariable {
    return c.runtimeError(errors.ErrReadOnlyValue, pkg.Name+"."+name)
}
```

`egostrings.HasCapitalizedName` returns `true` only when the first Unicode
character of the name is uppercase (A–Z or Unicode uppercase).  The underscore
character `_` is **not** uppercase, so any name starting with `_` returned
`false` from guard 1 and exited with `ErrSymbolNotExported` before reaching
guard 2.  Guard 2 could therefore never execute.

`defs.DiscardedVariable = "_"`, so the intent of guard 2 was probably to
catch names like `"_internalHelper"` — but such names were already caught by
guard 1.

#### STRUCT-1: Fix

Guard 2 (the unreachable `name[0:1] == defs.DiscardedVariable` block) was
removed entirely.  The function now has a single exported-name check followed
directly by the read-only and constant checks.

```go
// Must be an exported (capitalized) name.
if !egostrings.HasCapitalizedName(name) {
    return c.runtimeError(errors.ErrSymbolNotExported, pkg.Name+"."+name)
}

// If it's a declared item in the package, is it one of the ones
// that is readOnly by default?
if oldItem, found := pkg.Get(name); found { ... }
```

`Test_storeInPackage_NoDiscardedVariableCheck_STRUCT1` confirms that a name
like `"_Foo"` is still correctly rejected by the first guard with
`ErrSymbolNotExported`, and that no `defs` import is required in `structs.go`
solely for this guard.

---

<a id="STRUCT-2"></a>

### STRUCT-2 — `storeIndexByteCode` mutates a struct field before checking package visibility

**Affected function:** `storeIndexByteCode`  
**File:** `bytecode/structs.go`  
**Risk:** Medium — an unexported field write from outside the owning package
modifies the struct in memory and then returns an error, leaving the struct
in a partially-modified state with no rollback  
**Discovered by:** `Test_storeIndexByteCode_StructPackageVisibility_STRUCT2`  
**Status: RESOLVED**

#### STRUCT-2: Original behavior

The `*data.Struct` case in `storeIndexByteCode` called `a.Set(key, v)` to
write the field value and **then** checked whether the field was visible from
the current package:

```go
case *data.Struct:
    key := data.String(index)

    if err = a.Set(key, v); err != nil {   // ← write happened here
        return c.runtimeError(err)
    }

    // Visibility check ran AFTER the write — buggy ordering.
    if pkg := a.PackageName(); pkg != "" && pkg != c.pkg {
        if !egostrings.HasCapitalizedName(key) {
            return c.runtimeError(errors.ErrSymbolNotExported).Context(key)
        }
    }
```

When an unexported field from a different package was written, the error was
correctly returned — but the `a.Set` call had already committed the change.
Because `*data.Struct` is a pointer type, the modification was visible to all
callers that held a reference to the same struct.  The same ordering problem
affected the `*any` wrapping a `*data.Struct` path immediately below it.

#### STRUCT-2: Fix

The package visibility check was moved to run **before** `a.Set` in both the
`*data.Struct` case and the `*any → *data.Struct` case:

```go
case *data.Struct:
    key := data.String(index)

    // Check package visibility before modifying the struct.
    if pkg := a.PackageName(); pkg != "" && pkg != c.pkg {
        if !egostrings.HasCapitalizedName(key) {
            return c.runtimeError(errors.ErrSymbolNotExported).Context(key)
        }
    }

    if err = a.Set(key, v); err != nil {
        return c.runtimeError(err)
    }

    _ = c.push(a)
```

`Test_storeIndexByteCode_StructPackageVisibility_STRUCT2` now asserts that the
field retains its original value (`0`) after the rejected write, confirming
that the struct is not modified before the error is returned.

---

<a id="STRUCT-3"></a>

### STRUCT-3 — `flattenByteCode` produces incorrect `argCountDelta` for empty arrays

**Affected function:** `flattenByteCode`  
**File:** `bytecode/structs.go`  
**Risk:** Low — the Ego compiler does not appear to spread empty arrays at
call sites; the bug was latent and required a malformed or hand-crafted
bytecode sequence to trigger  
**Discovered by:** `Test_flattenByteCode_EmptyArray_STRUCT3`  
**Status: RESOLVED**

#### STRUCT-3: Original behavior

`flattenByteCode` replaced the top-of-stack array with its individual elements
and set `c.argCountDelta` to `N-1`, where `N` is the number of elements
expanded.  The "-1" accounted for the fact that the compiler already counted the
original array as one argument.

The decrement was guarded by `argCountDelta > 0`:

```go
// After the expansion loop:
if c.argCountDelta > 0 {
    c.argCountDelta--   // net: N-1 for N > 0, but 0 for N = 0 (wrong)
}
```

For an **empty** array (`N == 0`), the loop did not execute, `argCountDelta`
stayed at 0, and the guard prevented the decrement.  The following `Call`
opcode computed `argc = compiler_count + argCountDelta = 1 + 0 = 1`, but the
stack had 0 expanded values — causing the call to either underflow the stack
or pop a stale value as an argument.

#### STRUCT-3: Fix

The `if c.argCountDelta > 0` conditional was replaced with an `isArray` flag
that tracks whether a `*data.Array` or `[]any` was expanded.  The decrement
now runs unconditionally for any array expansion, including empty arrays:

```go
isArray := false

if array, ok := v.(*data.Array); ok {
    isArray = true
    for idx := 0; idx < array.Len(); idx++ { ... c.argCountDelta++ }
} else if array, ok := v.([]any); ok {
    isArray = true
    for _, vv := range array { ... c.argCountDelta++ }
} else {
    _ = c.push(v)   // scalar: no delta adjustment
}

// Subtract 1 for any array, including empty, to remove the original slot.
if isArray {
    c.argCountDelta--
}
```

Result for each case:

| Input | Elements pushed | argCountDelta |
| :---- | :-------------- | :------------ |
| empty array (N=0) | 0 | 0 − 1 = **−1** ✓ |
| single-element array (N=1) | 1 | 1 − 1 = 0 ✓ |
| three-element array (N=3) | 3 | 3 − 1 = 2 ✓ |
| scalar (not an array) | 1 (unchanged) | 0 ✓ |

`Test_flattenByteCode_EmptyArray_STRUCT3` now asserts `argCountDelta == -1`
after flattening an empty array, confirming correct behavior.  The full
869-test Ego integration suite continued to pass after the change.

---

<a id="packages"></a>

## PACKAGES — Package Instructions

| ID | Summary | Status |
| :-- | :-- | :-- |
| [PACKAGES-1](#PACKAGES-1) | `inPackageByteCode` panics when the symbol table holds a non-package value under the package name | ✓ |
| [PACKAGES-2](#PACKAGES-2) | `makePackageItemList` panics on nil values in the package dictionary or symbol table | ✓ |

<a id="PACKAGES-1"></a>

### PACKAGES-1 — `inPackageByteCode` panics when the symbol table holds a non-package value under the package name

**Affected function:** `inPackageByteCode`  
**File:** `bytecode/package.go`  
**Risk:** High — any local variable whose name matches a package name (e.g. a
loop counter named `math`) causes an unrecoverable nil-pointer panic rather
than a clean error or graceful fallthrough to the package cache  
**Discovered by:** `Test_inPackageByteCode_NonPackageInSymbolTable_PACKAGES1`  
**Status: RESOLVED**

#### PACKAGES-1: Original behavior

`inPackageByteCode` searched the scope chain with `GetAnyScope` and, on any
hit, immediately passed the result to `GetPackageSymbolTable` and called
`NewChildProxy` on the return value:

```go
if pkg, found := c.symbols.GetAnyScope(c.pkg); found {
    c.symbols = symbols.GetPackageSymbolTable(pkg).NewChildProxy(c.symbols)
    // ↑ GetPackageSymbolTable returns nil when pkg is not *data.Package,
    //   so NewChildProxy is called on a nil *SymbolTable → panic
    return nil
}
```

`GetPackageSymbolTable` type-asserts its argument to `*data.Package` and
returns `nil` when the assertion fails.  Calling `.NewChildProxy(...)` on a
nil `*SymbolTable` receiver panics with a nil pointer dereference.

In practice this can be triggered by Ego code that uses a local variable with
the same name as a package (e.g. `math := 3.14`) and then enters a `package
math` scope inside the same function.

#### PACKAGES-1: Fix

A type assertion was added between the `GetAnyScope` call and the proxy
creation.  When the found value is not a `*data.Package`, the code falls
through to the global package-cache lookup instead of panicking:

```go
if found, ok := c.symbols.GetAnyScope(c.pkg); ok {
    // Guard: must actually be a *data.Package.  A local variable that
    // shadows the package name must not cause a panic (PACKAGES-1 fix).
    if pkg, isPkg := found.(*data.Package); isPkg {
        c.symbols = symbols.GetPackageSymbolTable(pkg).NewChildProxy(c.symbols)
        return nil
    }
    // Fall through: the shadowing value is not a package; try the cache.
}
```

Two tests cover this fix:

- `Test_inPackageByteCode_NonPackageInSymbolTable_PACKAGES1` — confirms that a
  non-package shadow returns `ErrInvalidPackageName` (no panic) when the cache
  also has no entry.
- `Test_inPackageByteCode_NonPackageInSymbolTable_CacheFallback_PACKAGES1` —
  confirms that when the cache does have the real package the call succeeds
  despite the shadowing variable.

---

<a id="PACKAGES-2"></a>

### PACKAGES-2 — `makePackageItemList` panics on nil values in the package dictionary or symbol table

**Affected function:** `makePackageItemList`  
**File:** `bytecode/package.go`  
**Risk:** Medium — any package that stores a nil value under an exported key
causes `dumpPackagesByteCode` to panic, crashing the REPL or any tool that
lists packages  
**Discovered by:** `Test_makePackageItemList_NilValueNoPanic_PACKAGES2`,
`Test_makePackageItemList_NilInSymbolTable_PACKAGES2`  
**Status: RESOLVED**

#### PACKAGES-2: Original behavior

In both loops inside `makePackageItemList`, the `default` branch called
`reflect.TypeOf(v).String()` without first checking whether `v` was nil:

```go
// First loop (package dictionary):
default:
    r := reflect.TypeOf(v).String()   // ← panics when v is nil

// Second loop (symbol table):
r := reflect.TypeOf(value).String()   // ← panics when value is nil
```

`reflect.TypeOf(nil)` returns a nil `reflect.Type`.  Calling `.String()` on a
nil interface value panics with:

```text
panic: runtime error: invalid memory address or nil pointer dereference
```

Nil values can legitimately appear in a package when an Ego program stores
`nil` in a package-level variable, or when a built-in package registers a
placeholder entry during initialization.

#### PACKAGES-2: Fix

A nil guard was inserted before each `reflect.TypeOf` call.  When the value is
nil, a descriptive `"3var … = nil"` string is emitted directly instead of
delegating to reflection:

```go
// First loop (package dictionary):
default:
    if v == nil {
        item = "3var " + key + " = nil"
    } else {
        r := reflect.TypeOf(v).String()
        ...
    }

// Second loop (symbol table):
if value == nil {
    item = "3var " + name + " = nil"
} else {
    r := reflect.TypeOf(value).String()
    ...
}
```

The nil case is classified as a variable (`"3var"`) because a nil value most
naturally represents an uninitialized variable slot rather than a type,
constant, or function.

`Test_makePackageItemList_NilValueNoPanic_PACKAGES2` confirms no panic for a
nil value in the package dictionary, and
`Test_makePackageItemList_NilInSymbolTable_PACKAGES2` confirms the same for the
symbol-table path.  Both tests also verify that the nil entry still appears in
the output list so the item is not silently discarded.

---

<a id="print"></a>

## Print Instructions

The print-related instructions live in `bytecode/print.go`.  Tests are in
`bytecode/print_test.go`.

| ID | Summary | Status |
| :-- | :-- | :-- |
| [PRINT-1](#PRINT-1) | Unchecked type assertion in `formatValueForPrinting` panics on nil struct-array elements | ✓ |
| [PRINT-2](#PRINT-2) | `case *data.Function:` in `formatValueForPrinting` is unreachable dead code | ✓ |

---

<a id="PRINT-1"></a>

### PRINT-1: Unchecked type assertion in `formatValueForPrinting` panics on nil struct-array elements

| | |
| :-- | :-- |
| **Affected function** | `formatValueForPrinting` in `bytecode/print.go` |
| **Risk** | MEDIUM — any Ego program that prints a struct array before all elements are assigned |
| **Discovering test** | `Test_formatValueForPrinting_ArrayOfStructs` (tests the safe path only; nil path would panic the test binary) |
| **Status** | RESOLVED |

#### PRINT-1: Original behavior

`formatValueForPrinting` handles `*data.Array` values whose element type is a
struct kind by iterating over the elements and casting each one to `*data.Struct`:

```go
for i := 0; i < actualValue.Len(); i++ {
    rowValue, _ := actualValue.Get(i)
    row := rowValue.(*data.Struct)   // ← unchecked assertion
    ...
}
```

`data.Array.Get(i)` returns `a.data[i]` directly.  When an array is created
with `data.NewArray(structType, n)`, the `data` slice is allocated but struct
elements are **not** initialized — only scalar kinds (bool, int, float, string)
get zero values.  Struct elements remain `nil`.

Calling `nil.(*data.Struct)` panics at runtime:

```text
panic: interface conversion: interface is nil, not *data.Struct
```

#### PRINT-1: Conditions that trigger the panic

1. Ego code creates a struct array with a pre-allocated size but does not
   assign all elements before printing — e.g., `arr := make([]MyStruct, 3)`
   followed by partial assignment.
2. Test code uses `data.NewArray(structType, n)` without populating every slot.

Note: `data.Array.Make` (used by the Ego `make()` built-in) calls
`InstanceOfType` which correctly initializes struct elements with
`data.NewStruct(t)`.  Only direct calls to `data.NewArray` leave slots nil.

#### PRINT-1: Fix

A nil guard and a type-ok check were added before the assertion.  Nil elements
and any non-struct elements are silently skipped, so a partially-initialized
array simply omits those rows from the output instead of panicking:

```go
rowValue, _ := actualValue.Get(i)
if rowValue == nil {
    continue
}
row, ok := rowValue.(*data.Struct)
if !ok {
    continue
}
```

`Test_formatValueForPrinting_ArrayOfStructs_NilElement_PRINT1` creates a
three-element struct array with only slots 0 and 2 populated, calls
`formatValueForPrinting`, and confirms that the output contains the two valid
rows without panicking.

---

<a id="PRINT-2"></a>

### PRINT-2: `case *data.Function:` in `formatValueForPrinting` is unreachable dead code

| | |
| :-- | :-- |
| **Affected function** | `formatValueForPrinting` in `bytecode/print.go` |
| **Risk** | LOW — current behavior (functions printing their declaration string) is not harmful, but the suppression intent is never fulfilled |
| **Discovering test** | `Test_formatValueForPrinting_FunctionPointer_ProducesEmptyString` and `Test_formatValueForPrinting_FunctionValue_SuppressedAfterFix_PRINT2` |
| **Status** | RESOLVED |

#### PRINT-2: Original behavior

`formatValueForPrinting` has an empty case for `*data.Function`:

```go
case *data.Function:
    // intentionally empty — s stays ""
```

The intent is to suppress function values so that `print myFunc` produces no
output.  However, Ego stores function values on the runtime stack as
`data.Function` (**value** type, not pointer).  The case only matches
`*data.Function` (**pointer** type).

Since nothing in the normal execution path ever pushes a `*data.Function`
pointer onto the stack, this case is dead code.  When a function value IS
passed to `printByteCode`, it is a `data.Function` value, which falls through
to the `default` branch and is formatted by `data.FormatUnquoted` as its
declaration string (e.g., `"myFunc()"`).

#### PRINT-2: Fix

The case was broadened to cover both the value and pointer forms:

```go
case data.Function, *data.Function:
    // Function values — both value and pointer — are suppressed.
    // s stays as "" so nothing is written to the output.
```

`Test_formatValueForPrinting_FunctionValue_SuppressedAfterFix_PRINT2` confirms
that a `data.Function` value now produces empty output, and the existing
`Test_formatValueForPrinting_FunctionPointer_ProducesEmptyString` continues to
confirm the same for `*data.Function` pointers.

---

<a id="return"></a>

## Return Instruction

The return instruction lives in `bytecode/return.go`.  Tests are in
`bytecode/return_test.go`.

| ID | Summary | Status |
| :-- | :-- | :-- |
| [RETURN-1](#RETURN-1) | `isStackMarker(c.Result)` uses the bound method instead of the field — guard never fires | ✓ |
| [RETURN-2](#RETURN-2) | `err` from `c.Pop()` in the bool branch is silently overwritten | ✓ |

---

<a id="RETURN-1"></a>

### RETURN-1: `isStackMarker(c.Result)` uses the bound method instead of the field — guard never fires

| | |
| :-- | :-- |
| **Affected function** | `returnByteCode` in `bytecode/return.go` |
| **Risk** | MEDIUM — a StackMarker on the stack where a return value is expected is silently propagated to the caller instead of raising an error |
| **Discovering test** | `Test_returnByteCode_BoolReturn_StackMarker_ReturnsVoidError_RETURN1` |
| **Status** | RESOLVED |

#### RETURN-1: Original behavior

In the `bool` branch of `returnByteCode`, after popping the return value, there
is a guard that is supposed to detect when a StackMarker was returned (which
means a function that returns void was incorrectly used as an expression):

```go
c.result, err = c.Pop()
if isStackMarker(c.Result) {          // ← RETURN-1 bug
    return c.runtimeError(errors.ErrFunctionReturnedVoid)
}
c.resultSet = true
```

`c.Result` (uppercase `R`) is the **bound method expression** — it evaluates to
a value of type `func() any`, not the `any` stored in `c.result`.

```text
c.result   — the unexported field of type any; holds the popped value
c.Result   — the exported method func (c *Context) Result() any
c.Result   — WITHOUT parentheses: a bound method of type func() any
```

`isStackMarker` checks whether its argument is a `StackMarker` struct or a
`*CallFrame`.  A bound method `func() any` is neither, so `isStackMarker`
always returns `false`.  The guard is dead code.

When a StackMarker is on the stack:

- It is popped and stored in `c.result` (the field).
- `c.resultSet` is set to `true`.
- `callFramePop` re-pushes the StackMarker onto the caller's stack.
- The caller receives a StackMarker as a "value", which can corrupt subsequent
  operations.

#### RETURN-1: Fix

Changed `c.Result` (method reference) to `c.result` (field):

```go
c.result, err = c.Pop()
if err != nil {                        // RETURN-2 fix applied together
    return c.runtimeError(err)
}
if isStackMarker(c.result) {           // ← corrected (was c.Result)
    return c.runtimeError(errors.ErrFunctionReturnedVoid)
}
```

`Test_returnByteCode_BoolReturn_StackMarker_ReturnsVoidError_RETURN1` confirms
that pushing a StackMarker and calling Return(true) now returns
`ErrFunctionReturnedVoid`.

---

<a id="RETURN-2"></a>

### RETURN-2: `err` from `c.Pop()` in the bool branch is silently overwritten

| | |
| :-- | :-- |
| **Affected function** | `returnByteCode` in `bytecode/return.go` |
| **Risk** | LOW — in practice the compiler only generates `Return(true)` when a value is guaranteed to be on the stack, so Pop() should never fail here |
| **Discovering test** | `Test_returnByteCode_BoolReturn_EmptyStack_RETURN2` |
| **Status** | RESOLVED |

#### RETURN-2: Original behavior

In the `bool` branch the error from `c.Pop()` is captured in `err` but is never
checked before the function continues:

```go
if b, ok := i.(bool); ok && b {
    c.result, err = c.Pop()  // err set here...
    if isStackMarker(c.Result) {
        ...
    }
    c.resultSet = true
}
// ...
if c.framePointer > 0 {
    err = c.callFramePop()  // ...but overwritten here
}
```

If `c.Pop()` fails (the stack is unexpectedly empty), the error is silently
replaced by whatever `callFramePop()` returns.  In the `int(1)` sub-branch the
same issue exists, though there `err` is at least tested before the secondary
marker pop.

#### RETURN-2: Fix

An explicit error check was added immediately after the Pop, combined with the
RETURN-1 correction in one contiguous block:

```go
c.result, err = c.Pop()
if err != nil {
    return c.runtimeError(err)
}
if isStackMarker(c.result) {   // c.result (field), not c.Result (method)
    return c.runtimeError(errors.ErrFunctionReturnedVoid)
}
```

`Test_returnByteCode_BoolReturn_EmptyStack_RETURN2` confirms that calling
Return(true) with an empty callee stack now returns a non-nil error instead of
silently proceeding.

---

<a id="area-types-bytecode"></a>

## Type Instructions

The type-related bytecode instructions live in `bytecode/types.go`.  Tests are
in `bytecode/types_test.go`.

| ID | Summary | Status |
| :-- | :-- | :-- |
| [TYPES-1](#TYPES-1) | Dead nil guard in `deRefByteCode` leaves `*c3` vulnerable to panic | ✓ |
| [TYPES-2](#TYPES-2) | `reflect.TypeOf(v).String()` panics when `v` is nil in `relaxedConformanceCheck` | ✓ |
| [TYPES-3](#TYPES-3) | Int-dispatch switch in `relaxedConformanceCheck` has unreachable cases | ✓ |

---

<a id="TYPES-1"></a>

### TYPES-1: Dead nil guard in `deRefByteCode` leaves `*c3` vulnerable to panic

| | |
| :-- | :-- |
| **Affected function** | `deRefByteCode` in `bytecode/types.go` |
| **Risk** | MEDIUM — dereferencing an Ego variable that holds a nil pointer (`*any(nil)`) panics the runtime instead of returning ErrNilPointerReference |
| **Discovering test** | `Test_deRefByteCode_TypedNilInnerPointer_TYPES1` |
| **Status** | RESOLVED |

#### TYPES-1: Original behavior

`deRefByteCode` walks two levels of indirection to dereference an Ego pointer
variable:

```go
addr     = GetAddress("p")    // *any → symbol's value slot
content  = addr.(*any)        // outer pointer (always non-nil here)
c2       = *content           // the Ego pointer value stored in the slot
c3, ok   = c2.(*any)          // inner pointer (the Ego pointer target)
*c3                           // final dereference ← potential panic
```

Inside the `c3` branch there is a nil check meant to guard against `*c3`
panicking:

```go
if data.IsNil(content) {           // ← TYPES-1 bug: checks 'content', not 'c3'
    return c.runtimeError(errors.ErrNilPointerReference)
}
return c.push(*c3)
```

`content` is the outer `*any` pointer, which was already verified non-nil by
an earlier guard (line ~362).  Because `content` can never be nil at this
point, the check is dead code and `*c3` is reached unconditionally.

If the symbol holds a **typed nil** `*any` value — for example, an Ego pointer
variable that has been declared but not assigned — the type assertion
`c2.(*any)` succeeds with `c3 = nil`.  The dead guard passes (content is non-nil),
and `*c3` panics.

#### TYPES-1: Fix

The nil guard was moved to before the first dereference and corrected to check
`c3` (the inner pointer) instead of `content` (the outer pointer):

```go
if c3, ok := c2.(*any); ok {
    if c3 == nil {   // ← corrected: was data.IsNil(content)
        return c.runtimeError(errors.ErrNilPointerReference)
    }
    xc3 := *c3      // safe — c3 is guaranteed non-nil
    ...
}
```

`Test_deRefByteCode_TypedNilInnerPointer_TYPES1` stores a typed-nil `*any` in
a symbol and confirms that `deRefByteCode` returns `ErrNilPointerReference`
instead of panicking.

---

<a id="TYPES-2"></a>

### TYPES-2: `reflect.TypeOf(v).String()` panics when `v` is nil in `relaxedConformanceCheck`

| | |
| :-- | :-- |
| **Affected function** | `relaxedConformanceCheck` in `bytecode/types.go` |
| **Risk** | LOW — requires the RequiredType operand to be a string and the stack value to be nil simultaneously; uncommon in practice |
| **Discovering test** | `Test_relaxedConformanceCheck_NilValue_StringOperand_TYPES2` |
| **Status** | RESOLVED |

#### TYPES-2: Original behavior

In the string-operand branch of `relaxedConformanceCheck`:

```go
if t, ok := i.(string); ok {
    if t != reflect.TypeOf(v).String() {  // ← panics when v is nil
        err = c.runtimeError(errors.ErrArgumentType)
    }
}
```

`reflect.TypeOf(nil)` returns a nil `reflect.Type`.  Calling `.String()` on a
nil `reflect.Type` panics:

```text
panic: runtime error: invalid memory address or nil pointer dereference
```

This is triggered when `requiredTypeByteCode` is called with a string operand
(a Go type name such as `"int"`) and the top-of-stack value is nil.

#### TYPES-2: Fix

A nil guard was added before the reflect call:

```go
if t, ok := i.(string); ok {
    if v == nil || t != reflect.TypeOf(v).String() {  // ← v == nil guard added
        err = c.runtimeError(errors.ErrArgumentType)
    }
}
```

`Test_relaxedConformanceCheck_NilValue_StringOperand_TYPES2` calls
`relaxedConformanceCheck` with a string operand and a nil value, confirming
that an error is returned rather than a panic.

---

<a id="TYPES-3"></a>

### TYPES-3: Int-dispatch switch in `relaxedConformanceCheck` has unreachable cases

| | |
| :-- | :-- |
| **Affected function** | `relaxedConformanceCheck` in `bytecode/types.go` |
| **Risk** | LOW — current behavior is incorrect for non-`int` integer types (e.g., int16) but these cases are never reached, so no observable error is produced |
| **Discovering test** | `Test_requiredTypeByteCode_Int16Operand_MatchingValue_TYPES3` and `Test_requiredTypeByteCode_Int16Operand_WrongValue_TYPES3` |
| **Status** | RESOLVED |

#### TYPES-3: Original behavior

The int-operand branch extracts an `int` from the operand and then switches on
the Ego kind of that value:

```go
if t, ok := i.(int); ok {
    switch data.TypeOf(t).Kind() {
    case data.Int16Kind:
        _, ok = v.(int16)
    case data.UInt16Kind:
        _, ok = v.(uint16)
    ...
    case data.IntKind:
        _, ok = v.(int)
    ...
    }
}
```

`t` is the result of `i.(int)` — a plain Go `int`.  `data.TypeOf(int(anything))`
always returns `data.IntType`, whose `.Kind()` is always `data.IntKind`.
Therefore:

- The `case data.IntKind` branch is the only one ever taken.
- All other cases (`Int16Kind`, `UInt16Kind`, `Int8Kind`, `Int32Kind`,
  `Int64Kind`, `Float32Kind`, `Float64Kind`, `ByteKind`, `BoolKind`,
  `StringKind`) are dead code and can never execute.

The practical consequence is that when `requiredTypeByteCode` receives an int
operand and a non-`int` value (e.g., `int16(5)`), the check `v.(int)` fails
and `ErrArgumentType` is returned even if int16 might be a valid match for the
intended type.

#### TYPES-3: Fix

The `i.(int)` extraction was removed and replaced with a `switch i.(type)` that
dispatches on the exact Go type of the operand:

```go
switch i.(type) {
case int:
    _, kindOk = v.(int)
case int16:
    _, kindOk = v.(int16)
case uint16:
    _, kindOk = v.(uint16)
// … int8, int32, int64, byte, bool, float32, float64 …
default:
    kindOk = true   // non-integer operands pass through unchanged
}
```

Non-integer operands (such as `*data.Type` values handled by the block above)
hit the `default` branch, preserving the original pass-through behavior for
those cases.

`Test_requiredTypeByteCode_Int16Operand_MatchingValue_TYPES3` confirms that an
`int16(5)` value now passes when `int16(0)` is the operand, and
`Test_requiredTypeByteCode_Int16Operand_WrongValue_TYPES3` confirms that a
plain `int` value fails for the same operand.

---

This section documents behavioral anomalies, potential bugs, and design concerns found during a comprehensive review of the `debugger/` package, which intercepts the `ErrSignalDebugger` sentinel from the `bytecode.Context` run loop to offer an interactive prompt.

<a id="area-debugger-breaks"></a>

## DEBUGGER-BREAKS — Breakpoint Handling

| ID | Summary | Status |
| -- | ------- | ------ |
| [DEBUGGER-BREAKS-1](#DEBUGGER-BREAKS-1) | Range-loop copy semantics silently discarded `hit` counter updates in `evaluationBreakpoint`. | ✓ |
| [DEBUGGER-BREAKS-2](#DEBUGGER-BREAKS-2) | The `"load"` case compared `bp.Kind` to a hard-coded magic number instead of the `BreakValue` constant. | ✓ |
| [DEBUGGER-BREAKS-3](#DEBUGGER-BREAKS-3) | `clearBreakWhen`/`clearBreakAtLine` kept iterating a shifted slice after removing a match instead of returning immediately. | ✓ |

<a id="DEBUGGER-BREAKS-1"></a>

### DEBUGGER-BREAKS-1 — Hit tracking lost in evaluationBreakpoint range loop

**File:** `debugger/breaks.go`  
**Function:** `evaluationBreakpoint`  
**Risk:** High — conditional breakpoints (`break when <expr>`) fire on every
matching step instead of once per "becoming true" edge  
**Status: RESOLVED**

#### DEBUGGER-BREAKS-1: Original behavior

`evaluationBreakpoint` iterates over `breakPoints` with a `for _, b := range`
loop.  In Go, the range loop copies each element into the loop variable `b`
before executing the body.  Any changes to `b` inside the loop modify only
that **copy** — the original element in the slice is untouched.

Two `b.hit` mutations inside the loop were therefore silently lost:

```go
// BreakValue case (conditional breakpoint):
if b.hit > 0 {
    break          // intended to suppress re-trigger — but b.hit is always 0!
}
...
if prompt {
    b.hit++        // increments the copy; breakPoints[n].hit stays 0
} else {
    b.hit = 0      // also a no-op on the original
}

// BreakAlways case (line breakpoint):
b.hit++            // hit-count statistics never increment
```

**Consequence for `BreakValue`:** The guard `if b.hit > 0` is designed to
suppress a conditional breakpoint from re-triggering on every subsequent step
once it has first fired (the user must resume before it fires again).  Because
`b.hit` is always reset to 0 by the copy semantics, the guard never activates.
Every single source line where the condition evaluates to true causes another
debugger stop, making programs with conditional breakpoints nearly unusable.

**Consequence for `BreakAlways`:** The hit counter statistics in the slice are
never updated, so any future `show breaks` display showing hit counts would
always show 0.

#### DEBUGGER-BREAKS-1: Fix

Changed the loop from `for _, b := range breakPoints` to
`for n := range breakPoints` and used `breakPoints[n]` directly for all reads
and writes.  This ensures that `hit` changes are applied to the real slice
element, not a temporary copy.

```go
for n := range breakPoints {
    switch breakPoints[n].Kind {
    case BreakValue:
        if breakPoints[n].hit > 0 {
            break
        }
        ...
        if prompt {
            breakPoints[n].hit++
        } else {
            breakPoints[n].hit = 0
        }
    case BreakAlways:
        ...
        breakPoints[n].hit++
    }
}
```

<a id="DEBUGGER-BREAKS-2"></a>

### DEBUGGER-BREAKS-2 — Magic number instead of named constant in load case

**File:** `debugger/breaks.go`  
**Function:** `breakCommand` (the `"load"` sub-case)  
**Risk:** Low — currently harmless, but fragile if the `breakPointType` iota
values are ever reordered  
**Status: RESOLVED**

#### DEBUGGER-BREAKS-2: Original behavior

When loading breakpoints from a JSON file, the `"load"` case recompiled
conditional-breakpoint expressions with:

```go
if bp.Kind == 2 {
```

The value `2` is the current numeric value of the `BreakValue` constant
(defined via `iota` as the second entry after `BreakDisabled = 0` and
`BreakAlways = iota`), but the constant name was not used.  Hard-coded iota
values are fragile: inserting or reordering a constant in the future would
silently change which kind of breakpoint gets recompiled without a compiler
error.

#### DEBUGGER-BREAKS-2: Fix

Replaced the magic literal with the named constant:

```go
if bp.Kind == BreakValue {
```

<a id="DEBUGGER-BREAKS-3"></a>

### DEBUGGER-BREAKS-3 — clearBreak functions continue iterating after first deletion

**File:** `debugger/breaks.go`  
**Functions:** `clearBreakWhen`, `clearBreakAtLine`  
**Risk:** Medium — if duplicate breakpoints ever exist (due to a future bug),
only the first match is cleanly removed and subsequent matches may be skipped
due to slice-shift confusion  
**Status: RESOLVED**

#### DEBUGGER-BREAKS-3: Original behavior

Both clear functions used `for n, b := range breakPoints` and, after finding a
match, modified `breakPoints` in-place with `append(breakPoints[:n],
breakPoints[n+1:]...)`.  This shifts every element after position `n` one
slot to the left in the backing array.

The `range` loop captured the original slice header (pointer + length) at the
start of the loop.  After the shift, element `n+1` in the backing array now
holds what was originally element `n+2`.  On the next loop iteration
`n+1` is visited — but the element originally at that position has already
moved to `n`, so it is **skipped**.

In practice, `breakAtLine` and `breakWhen` both check for an existing
breakpoint before adding, so duplicate entries should not exist.  The bug is
therefore latent.  However, failing to `break` after the first (and only
expected) deletion leaves the loop running over stale data.

#### DEBUGGER-BREAKS-3: Fix

Added a `return` statement immediately after each deletion path so the
function exits as soon as the matching breakpoint is removed.  This is
consistent with the invariant that breakpoints are unique and avoids any
further iteration over the shifted slice.

```go
func clearBreakWhen(text string) {
    for n, b := range breakPoints {
        if b.Kind == BreakValue && b.Text == text {
            if len(breakPoints) == 1 {
                breakPoints = []breakPoint{}
            } else if n == len(breakPoints)-1 {
                breakPoints = breakPoints[:n]
            } else {
                breakPoints = append(breakPoints[:n], breakPoints[n+1:]...)
            }
            return  // ← added
        }
    }
}
```

(Same change applied to `clearBreakAtLine`.)

<a id="area-debugger-commands"></a>

## DEBUGGER-COMMANDS — Debugger Commands

| ID | Summary | Status |
| -- | ------- | ------ |
| [DEBUGGER-COMMANDS-1](#DEBUGGER-COMMANDS-1) | The `print` case used full-string `errors.Equal` instead of key-based `errors.Equals`, discarding output when `ErrStop` carried context. | ✓ |

<a id="DEBUGGER-COMMANDS-1"></a>

### DEBUGGER-COMMANDS-1 — errors.Equal (full string) used instead of errors.Equals (key-only)

**File:** `debugger/commands.go`  
**Function:** `debuggerPrompt`, the `"print"` case  
**Risk:** Medium — `print` command output may be silently discarded when the
inner execution stops with a contextualized `ErrStop`  
**Status: RESOLVED**

#### DEBUGGER-COMMANDS-1: Original behavior

After running the print expression in a child context, the code checked:

```go
if err == nil || errors.Equal(err, errors.ErrStop) {
    output := strings.TrimSuffix(printCtx.GetOutput(), "\n")
    sessionContext.println(output)
} else {
    sessionContext.say("msg.debug.error", ...)
}
```

`errors.Equal` (no `s`) compares the fully formatted `.Error()` strings of
both sides.  `errors.ErrStop` carries a bare message such as `"stop"`.  When
the child context exits normally via `ErrStop`, the error value stored in
`err` is often a contextualized form — e.g. `ErrStop.Context("line 5")` —
whose `.Error()` string is `"stop: line 5"`.  That does not match the bare
`"stop"` produced by `errors.ErrStop.Error()`, so `errors.Equal` returns
`false`.

The result is that the print output is discarded and a spurious error message
is shown to the user instead.

`errors.Equals` (with `s`) delegates to `(*Error).Is()`, which compares only
the underlying i18n key and ignores any `.Context(...)` suffix.  It correctly
identifies any flavour of `ErrStop` as a match.

#### DEBUGGER-COMMANDS-1: Fix

Changed the call at the affected line from `errors.Equal` to `errors.Equals`:

```go
if err == nil || errors.Equals(err, errors.ErrStop) {
```

<a id="area-debugger-show"></a>

## DEBUGGER-SHOW — Source Display

| ID | Summary | Status |
| -- | ------- | ------ |
| [DEBUGGER-SHOW-1](#DEBUGGER-SHOW-1) | `showSource` mutated its `err` parameter by value, so invalid range arguments were silently swallowed by the caller. | ✓ |
| [DEBUGGER-SHOW-2](#DEBUGGER-SHOW-2) | Indentation logic counts braces/parens in raw source text, so characters inside string literals miscount indentation. | |

<a id="DEBUGGER-SHOW-1"></a>

### DEBUGGER-SHOW-1 — showSource silently swallows invalid-range parse errors

**File:** `debugger/show.go`, `debugger/source.go`  
**Functions:** `showSource`, `showCommand`  
**Risk:** Medium — `show source bad-arg` silently does nothing; the user
receives no feedback that the argument was invalid  
**Status: RESOLVED**

#### DEBUGGER-SHOW-1: Original behavior

`showSource` was declared as returning nothing:

```go
func showSource(tx *tokenizer.Tokenizer, tokens *tokenizer.Tokenizer, err error, sessionContext *session)
```

It received the caller's `err` **by value** — a copy.  When an invalid integer
argument was found in the range spec:

```go
if e2 != nil {
    err = errors.New(errors.ErrInvalidInteger)
}
```

…the assignment modified only the local copy.  The caller in `showCommand`
used:

```go
case "source":
    showSource(tx, tokens, err, sessionContext)
```

After the call, the caller's `err` was still `nil`.  The command handler
returned `nil` to the user, giving no indication that the argument was
unparseable.

Additionally, `showSource` received the incoming `err` parameter as an input
gate (`if err == nil { /* list source */ }`), but the caller always passed
`nil`, making that parameter pointless as an input.

#### DEBUGGER-SHOW-1: Fix

Changed `showSource` to return an `error`:

```go
func showSource(...) error {
    ...
    if e2 != nil {
        return errors.New(errors.ErrInvalidInteger)
    }
    ...
    return nil
}
```

Removed the now-redundant `err error` input parameter (the caller always
passed `nil`).  Updated the caller in `showCommand`:

```go
case "source":
    err = showSource(tx, tokens, sessionContext)
```

<a id="DEBUGGER-SHOW-2"></a>

### DEBUGGER-SHOW-2 — Source indentation counts braces inside string literals

**File:** `debugger/source.go`  
**Function:** `showSource`  
**Risk:** Low — cosmetic display defect only; does not affect execution  
**Status: DOCUMENTED (not fixed)**

#### DEBUGGER-SHOW-2: Original behavior

The source formatter in `showSource` adjusts indentation by counting `{`/`}`
and `(`/`)` characters in each line:

```go
opened := strings.Count(t, "{") + strings.Count(t, "(")
closed := strings.Count(t, "}") + strings.Count(t, ")")
```

This approach operates on raw source text, not on tokens.  Any line that
contains these characters inside a string literal — for example:

```ego
fmt.Printf("value: %v {ok}\n", x)
```

— causes `opened` to exceed `closed` by 1, permanently increasing the
indentation level for all subsequent lines.

#### DEBUGGER-SHOW-2: Analysis

The formatter is a best-effort display aid rather than a semantic renderer.
Fixing it properly would require tokenizing each source line and skipping
characters inside string literals.  Given that the error only affects display
alignment (not execution), and that the current tokenizer's `New()` call
carries measurable overhead for short expressions, this issue is documented
but deferred.

---

### About This Section

There is a section (`##`) for each area of Ego that was evaluated for security and
operational risk issues. Within each section, the risks are grouped by severity
(critical, high, medium, and low). In each severity area, each exposure area is
documented with a unique identifier. For example, LOGIN-C1 is the first critical
issue in the LOGIN section. The information usually includes affected files, a
description detailed enough to help understand the issue, and recommended
remediation. When a fix is made, this may include adding additional details
about the resolution, particularly if the process of resolving it changes the
remediation plan somewhat.

<a id="area-login"></a>

## LOGIN — Logins and Passwords

| ID | Severity | Summary | Status |
| :--- | :--- | :--- | :---: |
| [LOGIN-C1](#LOGIN-C1) | Critical | Passwords hashed with bare, unsalted SHA-256, enabling parallel rainbow-table cracking | ✓ |
| [LOGIN-C2](#LOGIN-C2) | Critical | No brute-force or rate-limiting protection on the login endpoint | ✓ |
| [LOGIN-H1](#LOGIN-H1) | High | Timing attack possible via non-constant-time password comparison | ✓ |
| [LOGIN-H2](#LOGIN-H2) | High | Weak (MD5-based) key derivation used for AES encryption of tokens and DSN passwords | ✓ |
| [LOGIN-H3](#LOGIN-H3) | High | Token signing key potentially stored in plaintext configuration | ✓ |
| [LOGIN-H4](#LOGIN-H4) | High | Login credentials forwarded on HTTP 301 redirect to an arbitrary host | ✓ |
| [LOGIN-M1](#LOGIN-M1) | Medium | HTTP downgrade fallback when the HTTPS connection attempt fails | ✓ |
| [LOGIN-M2](#LOGIN-M2) | Medium | Password whitespace silently stripped before hashing | ✓ |
| [LOGIN-M3](#LOGIN-M3) | Medium | Token cache bypasses expiry and revocation checks for up to 60 seconds | ✓ |
| [LOGIN-M4](#LOGIN-M4) | Medium | Quoted-password legacy format allows plaintext password storage | ✓ |
| [LOGIN-L1](#LOGIN-L1) | Low | Password supplied via environment variable is exposed to other processes | ✓ |
| [LOGIN-L2](#LOGIN-L2) | Low | `InsecureSkipVerify` available without a prominent warning | ✓ |
| [LOGIN-L3](#LOGIN-L3) | Low | Returned token expiration is recalculated independently of the token contents | ✓ |

### Critical

<a id="LOGIN-C1"></a>

#### LOGIN-C1 — Weak password hashing (SHA-256, no salt)

**Affected files:**

- `egostrings/gibberish.go:119` — `HashString()`
- `server/auth/validate.go:26` — call site in `ValidatePassword()`

**Description:**  
Passwords are hashed with a bare SHA-256 digest. SHA-256 is a general-purpose
hash optimized for speed, which is the opposite of what password storage
requires. Because no per-user salt is applied, all users who share the same
password produce identical hash values. An attacker who obtains the user
database can attack all accounts in parallel using precomputed rainbow tables,
and can crack typical passwords in seconds on commodity hardware.

**Recommendation:**  
Replace `HashString()` as the password-storage primitive with `bcrypt` at a
cost factor of 12 or higher (`golang.org/x/crypto/bcrypt`). Alternatively,
`argon2id` or `scrypt` are acceptable. A migration path for existing stored
hashes can be implemented by detecting the old format on login success and
re-hashing in place with the new algorithm.

**Resolution:**  
Replace SHA-256 with bcrypt (cost ≥ 12) for password storage; implement
on-login migration for existing hashes.

---

<a id="LOGIN-C2"></a>

#### LOGIN-C2 — No brute-force or rate-limiting protection on the login endpoint

**Affected files:**

- `server/server/admin.go:27` — `LogonHandler()`
- `server/server/auth.go:34` — `Authenticate()`
- `server/auth/validate.go:12` — `ValidatePassword()`

**Description:**  
The `/services/admin/logon` endpoint and the Basic Auth validation path accept
an unlimited number of credential attempts with no throttling, no failed-attempt
counter, and no account lockout. An attacker can attempt passwords at full
network speed without any server-side friction.

**Recommendation:**  
Track failed authentication attempts per username (and optionally per source IP)
with a short-lived counter. Lock accounts temporarily after a threshold of
failures (e.g. 5–10), using exponential backoff before unlocking. If the
database backend is in use, the counter should survive server restarts. A
simple in-memory counter is an acceptable starting point for the file backend.

**Resolution:**  
Implement per-username failed-attempt counter and temporary lockout on the
login endpoint.

---

### High

<a id="LOGIN-H1"></a>

#### LOGIN-H1 — Timing attack in password comparison

**Affected file:** `server/auth/validate.go:27`

```go
ok = realPass == hashPass
```

**Description:**  
The built-in `==` operator on strings returns as soon as it finds the first
differing byte. An attacker making many requests can measure response-time
variations to determine how many leading bytes of their guess match the stored
hash — a well-known side-channel attack that can significantly narrow the
search space for an offline crack.

**Recommendation:**  
Replace the equality check with `crypto/subtle.ConstantTimeCompare`:

```go
ok = subtle.ConstantTimeCompare([]byte(realPass), []byte(hashPass)) == 1
```

**Resolution:**  
Replace `==` password comparison with `crypto/subtle.ConstantTimeCompare`.

---

<a id="LOGIN-H2"></a>

#### LOGIN-H2 — Weak key derivation in AES encryption

**Affected files:**

- `util/crypto.go` — `encrypt()` / `decrypt()` — token and DSN password encryption
- `app-cli/settings/crypto.go` — `encrypt()` / `decrypt()` — profile sidecar file encryption

**Description:**  
Both encryption layers originally derived AES keys from the passphrase using a
single MD5 computation: no salt, no iterations. MD5 produces a 128-bit output
that is hex-encoded to 32 ASCII bytes (coincidentally matching the AES-256 key
length), but the derivation is trivially fast — a GPU cluster can test billions
of candidate passphrases per second. No per-encryption salt means identical
passphrases always produce identical keys, enabling pre-computation attacks.

This is the most consequential of the two affected files because
`app-cli/settings/crypto.go` protects the highest-value secrets in the system:
`ego.server.token.key` (the AES key that signs all bearer tokens),
`ego.logon.token` (the stored bearer token), and database credentials.

**Recommendation:**  
Replace MD5 key derivation with a memory-hard KDF — Argon2id
(`golang.org/x/crypto/argon2`) — with a random per-encryption salt.
Add a version-discriminator (magic prefix) to existing ciphertext so old
data can be decrypted via the legacy MD5 path while new encryptions use the
stronger algorithm. Existing data migrates silently on the next write.

**Resolution (April 2026, stage 1):**  
`util/crypto.go` upgraded from MD5 to PBKDF2-SHA256 (100,000 iterations,
16-byte random salt). New ciphertext identified by a 4-byte magic prefix
`ÿEGO`; legacy MD5 ciphertext (no prefix) still decrypts via the existing path.

**Resolution (April 2026, stage 2):**  
Both files upgraded from their respective weak KDFs to **Argon2id**
(32 MiB memory, 2 iterations, 1 thread, 16-byte random salt) — the current
OWASP-recommended algorithm for password and key derivation.

- `util/crypto.go`: `encrypt()` now emits the `ÿEG3` magic prefix (v3).
  `decrypt()` dispatcher recognizes `ÿEG3` (Argon2id), `ÿEGO` (PBKDF2, v2),
  and the legacy no-prefix (MD5) format, so all existing tokens and DSN
  passwords continue to decrypt. New tokens and DSN passwords are written in
  v3 on the next encryption.

- `app-cli/settings/crypto.go`: `encrypt()` now emits the `ÿEG3` magic prefix.
  `decrypt()` recognizes `ÿEG3` (Argon2id) and the legacy no-prefix (MD5)
  format. Existing profile sidecar files transparently decrypt; they are
  re-encrypted in v2 on the next profile save.

---

<a id="LOGIN-H3"></a>

#### LOGIN-H3 — Token signing key stored in plaintext configuration

**Affected file:** `tokens/key.go:13` — `getTokenKey()`

**Description:**  
The AES key used to encrypt all bearer tokens is written to the user's settings
profile (`~/.ego/...`) in plaintext. Anyone with read access to that file can
extract the key and forge valid tokens for any username on that server instance.
The `EGO_SERVER_TOKEN_KEY` environment variable fallback also exposes the key
in the process environment, which is readable by other processes owned by the
same user on Linux (`/proc/<pid>/environ`).

**Recommendation:**  
The token key must not appear in plaintext in any configuration file. Options
include:

- Deriving the key from a master passphrase using PBKDF2 with a stored salt
  (the passphrase is never stored).
- Requiring the operator to supply the key via a secrets manager or sealed
  configuration at startup and refusing to start without it.
- At minimum, verify that the settings file is created with `0600` permissions
  and document the requirement.

**Resolution (April 2026):**  
Upon review, `ego.server.token.key` is already handled by the settings
infrastructure's "outboard encrypted config" mechanism. Sensitive keys listed in
`encryptedKeyValue` (`app-cli/settings/defs.go`) are never written to the main
profile JSON. Instead, each is written to a separate sidecar file (e.g.
`$.key`) encrypted with AES-256-GCM; the encryption passphrase is derived from
`profile.Name + profile.Salt + profile.ID`. The main profile JSON contains no
plaintext token key. This satisfies the "at minimum" requirement above for the
file-backend deployment model. The `EGO_SERVER_TOKEN_KEY` env-var path remains
a minor informational risk (see LOGIN-L1-adjacent concerns) but is not required
for normal operation.

---

<a id="LOGIN-H4"></a>

#### LOGIN-H4 — Login credentials forwarded on HTTP 301 redirect to arbitrary host

**Affected file:** `app-cli/app/logon.go:167`

**Description:**  
When the logon server returns an HTTP 301 redirect, the CLI re-sends the full
credential payload (username + password in the JSON request body) to the
`Location` URL without validating that the redirect target is the same host or
a trusted authority. A network attacker, or a misconfigured authority setting,
can redirect login requests — and the credentials they carry — to an
attacker-controlled host.

**Recommendation:**  
Do not automatically re-send credentials on redirect. Either refuse to follow
3xx responses that contain a body (and ask the user to update their configured
server URL), or validate that the redirect target shares the same origin
(scheme + host + port) before resending. The existing retry loop should at
most follow redirects for unauthenticated probes (heartbeat), not for
credential POST requests.

**Resolution:**  
Redirect following disabled on logon POST; 3xx responses return an error
telling the user to update their server URL.

---

### Medium

<a id="LOGIN-M1"></a>

#### LOGIN-M1 — HTTP downgrade when HTTPS connection fails

**Affected file:** `app-cli/app/logon.go:362` — `resolveServerName()`

**Description:**  
When resolving an unqualified server name, the CLI first tries HTTPS, then
silently falls back to plain HTTP if the HTTPS attempt fails. A network
attacker who blocks port 443 (or injects a TLS error) will cause the CLI to
retransmit the login credentials over an unencrypted connection without any
warning to the user.

**Recommendation:**  
Remove the HTTP fallback from the credential-submission path entirely. If HTTP
support is needed for development environments, require an explicit
`--insecure` or `--http` flag and print a prominent warning before sending.

**Resolution:**  
HTTP fallback removed from `resolveServerName`; unqualified names only try
HTTPS. Explicit `http://` scheme still accepted as the user's deliberate
choice.

---

<a id="LOGIN-M2"></a>

#### LOGIN-M2 — Password whitespace stripped before hashing

**Affected file:** `app-cli/app/logon.go:104`

```go
pass = strings.TrimSpace(pass)
```

**Description:**  
Leading and trailing whitespace is removed from the password before it is sent
to the server. A user who has intentionally chosen a password that begins or
ends with a space will find that their password silently changes, potentially
breaking login or allowing authentication with a truncated version of their
intended credential. It also subtly reduces the effective password space.

**Recommendation:**  
Remove the `TrimSpace` call. If legacy compatibility with passwords that were
stored after trimming is a concern, handle this explicitly in a migration step
rather than silently mangling all input going forward.

**Resolution:**  
Removed `strings.TrimSpace` from password handling; prompt loop now uses
`pass == ""` so spaces-only passwords are accepted as-is.

---

<a id="LOGIN-M3"></a>

#### LOGIN-M3 — Token cache bypasses expiry and revocation checks

**Affected file:** `server/server/auth.go:117`

```go
if userItem, found := caches.Find(caches.TokenCache, token); found {
    isAuthenticated = true
    user = data.String(userItem)
    // expiry and blacklist not rechecked
}
```

**Description:**  
When a bearer token is found in the 60-second in-memory cache, it is accepted
as valid without rechecking whether it has expired or been added to the
revocation blacklist since it was first cached. A token that expires or is
revoked can continue to authenticate requests for up to 60 seconds.

**Recommendation:**  
On a cache hit, still verify `tokens.Validate()` for expiry and blacklist
status. The cache value should store the parsed `Token` struct (including
its expiry field) rather than just the username string, so these checks can
be performed without a full re-decryption on every request.

**Resolution:**  
Cache now stores `*tokens.Token`; cache hits check `Expires` directly (no
re-decryption). Blacklist is already handled: `tokens.Blacklist()` purges the
token cache at revocation time.

---

<a id="LOGIN-M4"></a>

#### LOGIN-M4 — Quoted-password legacy format allows plaintext storage

**Affected file:** `server/auth/validate.go:22`

```go
if strings.HasPrefix(realPass, "{") && strings.HasSuffix(realPass, "}") {
    realPass = egostrings.HashString(realPass[1 : len(realPass)-1])
}
```

**Description:**  
A stored password value wrapped in `{` `}` braces is treated as a plaintext
value that is hashed at runtime instead of at storage time. This means
passwords can be deliberately (or accidentally) stored in plaintext in the
user database. Any user record whose stored password field begins and ends
with braces is vulnerable to having its password derived from the inner value,
which could enable authentication bypass in unexpected edge cases.

**Recommendation:**  
Treat this as a migration-only escape hatch. Add a guard so that the
`{plaintext}` form can only be consumed once — on the first successful login,
re-hash the value with the production algorithm and store the result, removing
the braces. Log a warning when the legacy format is encountered. Once all
known passwords have been migrated, remove the special-case logic entirely.

**Resolution:**  
`{quoted}` format now logs an `auth.password.plaintext` warning on every use;
bcrypt migration on first successful login was already in place from
LOGIN-C1. Remove the special-case block once no `{quoted}` entries remain in
the user database.

---

### Low / Informational

<a id="LOGIN-L1"></a>

#### LOGIN-L1 — Password supplied via environment variable

**Affected file:** `app-cli/app/logon.go:38` — `EgoPasswordEnv`

Environment variables are visible to all processes owned by the same user and
are inherited by child processes. Using `EGO_PASSWORD` to supply credentials is
a common CI convenience but should be noted in any threat model. Consider
supporting a credentials file with `0600` permissions or a secrets-manager
integration as a more secure alternative for automated contexts.

**Resolution (April 2026):**  
`app-cli/app/logon.go` now checks `os.Getenv(defs.EgoPasswordEnv)` after
reading the password option. When the variable is set, `ui.Say("logon.password.env")`
emits a visible warning to the user's console regardless of log level, and
`os.Unsetenv` clears the variable immediately so child processes do not inherit
the credential. Localized strings added to all three language files.

---

<a id="LOGIN-L2"></a>

#### LOGIN-L2 — `InsecureSkipVerify` available without prominent warning

TLS certificate verification can be disabled for self-signed certificates.
In development this is acceptable, but the flag should trigger a visible
warning in production-like configurations and should not be silently inherited
by default from a stored profile setting.

**Resolution (April 2026):**  
Two always-visible warning paths added using `ui.Say("rest.tls.insecure")`:
one in `runtime/rest/exchange.go` when the `ego.runtime.insecure.client`
profile setting activates insecure mode, and one in `runtime/rest/client.go`
when the `EGO_INSECURE_CLIENT` environment variable triggers it. For the
Ego-program `rest.Open({verify:false})` path in `runtime/rest/methods.go`, a
REST-logger entry is added (the disable is deliberate user code in that context,
not silent ambient configuration). Localized warning strings added to all three
language files under the key `rest.tls.insecure`.

---

<a id="LOGIN-L3"></a>

#### LOGIN-L3 — Returned token expiration recalculated independently of token contents

**Affected file:** `server/server/admin.go:99`

The expiration timestamp returned in the logon response body is computed from
the server's current max-expiration setting at response time, independently of
the expiry embedded inside the encrypted token. If the server's configured
maximum token duration changes between token issuance and token use, the
advisory expiration seen by the client and the enforced expiry inside the token
can diverge. This does not affect security directly but can cause confusing
"token expired" errors before the displayed expiry has passed.

**Resolution (April 2026):**  
`LogonHandler` in `server/server/admin.go` now calls `tokens.Unwrap` on the
freshly-minted token string immediately after creating it. `response.Expiration`
is set from `t.Expires.Format(time.UnixDate)` — the value baked into the token
itself — and the previous independent duration re-calculation block has been
removed. The `tokens.Unwrap` call also surfaces the `TokenID` needed for
`response.ID`, consolidating two previously separate concerns into one call.

---

<a id="area-webauth"></a>

## WEBAUTH — Webauthn and Passkeys

This section records security weaknesses in the WebAuthn passkey subsystem
identified during a code review in April 2026. Issues are rated using the same
severity scale as the authentication section above.

| ID | Severity | Summary | Status |
| :--- | :--- | :--- | :---: |
| [WEBAUTH-H1](#WEBAUTH-H1) | High | `allow.passkeys` setting not enforced in ceremony handlers | ✓ |
| [WEBAUTH-H2](#WEBAUTH-H2) | High | Authenticator clone warning not checked after login | ✓ |
| [WEBAUTH-M1](#WEBAUTH-M1) | Medium | No rate limiting on unauthenticated ceremony-begin endpoints | ✓ |
| [WEBAUTH-M2](#WEBAUTH-M2) | Medium | RPID derived from user-controlled `Host` header | ✓ |
| [WEBAUTH-M3](#WEBAUTH-M3) | Medium | `storeChallenge` mutates shared cache expiration on every call | ✓ |
| [WEBAUTH-L1](#WEBAUTH-L1) | Low | `Secure` flag absent on challenge cookie | ✓ |
| [WEBAUTH-L2](#WEBAUTH-L2) | Low | Cache item expiration only refreshed when CACHE logging is active | ✓ |
| [WEBAUTH-L3](#WEBAUTH-L3) | Low | No user notification when passkeys are cleared by an administrator | ✓ |

### High

<a id="WEBAUTH-H1"></a>

#### WEBAUTH-H1 — `allow.passkeys` setting not enforced in ceremony handlers

**Affected file:** `server/server/webauthn.go` — `WebAuthnLoginBeginHandler`,
`WebAuthnLoginFinishHandler`, `WebAuthnRegisterBeginHandler`,
`WebAuthnRegisterFinishHandler`, `WebAuthnClearPasskeysHandler`

**Description:**  
The `ego.server.allow.passkeys` setting was only checked in
`WebAuthnConfigHandler`, which returns the feature-flag value to the dashboard.
All five ceremony handlers ignored the setting entirely. A caller who bypassed
the dashboard UI (e.g. `curl`) could drive the full passkey registration and
login ceremonies even when an operator had disabled passkeys on the server.
The UI gate provided security by obscurity only.

**Recommendation:**  
Each ceremony handler must check the setting at entry and return 404 before
performing any work when passkeys are disabled. Extract the check into a shared
`passkeyGuard()` helper to ensure consistent enforcement.

**Resolution (April 2026):**  
`passkeyGuard()` added to `server/server/webauthn.go`; called at the top of all
five handlers. Returns HTTP 404 with an `auth.webauthn.disabled` audit log entry
when passkeys are disabled. Covered by `TestPasskeyGuard_*` tests.

---

<a id="WEBAUTH-H2"></a>

#### WEBAUTH-H2 — Authenticator clone warning not checked after login

**Affected file:** `server/server/webauthn.go:302` — `WebAuthnLoginFinishHandler`

**Description:**  
The WebAuthn specification (§6.1, step 21) requires servers to detect when an
authenticator's sign counter does not advance between uses, which is a strong
indicator that the credential has been cloned and is being replayed. The
`go-webauthn` library surfaces this condition as
`credential.Authenticator.CloneWarning == true` on the returned
`*webauthn.Credential`. The code was inspecting neither the returned credential
nor this flag, so a cloned passkey would authenticate successfully.

**Recommendation:**  
After `FinishDiscoverableLogin` returns without error, inspect
`credential.Authenticator.CloneWarning`. If `true`, reject the login with 401
and log an audit event naming the affected user.

**Resolution (April 2026):**  
Clone check added immediately after the `FinishDiscoverableLogin` call in
`WebAuthnLoginFinishHandler`. A `true` CloneWarning rejects the login with 401
and emits `auth.webauthn.clone.warning` to the AUTH audit log. Localized strings
added to all three language files.

---

### Medium

<a id="WEBAUTH-M1"></a>

#### WEBAUTH-M1 — No rate limiting on unauthenticated ceremony-begin endpoints

**Affected file:** `server/server/webauthn.go` — `WebAuthnLoginBeginHandler`,
`WebAuthnRegisterBeginHandler`

**Description:**  
Both begin endpoints are unauthenticated. Each successful call allocates a UUID
nonce and stores a serialized `webauthn.SessionData` value in the
`WebAuthnChallengeCache`. There is no per-IP rate limit, no global cap on the
number of pending ceremonies, and no maximum cache size. A sustained flood of
requests would continuously fill the cache, consuming server memory at a rate
bounded only by network bandwidth and the 5-minute TTL on each entry.

**Recommendation:**  
Apply a per-IP rate limit (token bucket or sliding-window counter) to both begin
endpoints, and/or enforce a maximum number of pending WebAuthn sessions at any
given time, rejecting new begin requests with 429 when the cap is reached.

**Resolution (April 2026):**  
`webAuthnBeginGuard()` added in `server/server/webauthn_limiter.go`; called at
the top of both begin handlers after `passkeyGuard`. Enforces a sliding-window
per-IP limit of 10 requests per minute and a global cap of 200 concurrent
pending ceremonies. Both limits return HTTP 429 with an audit log entry.
`clientIP()` honours `X-Forwarded-For` when the direct peer is a loopback
address. Covered by `TestIPLimiter_*` and `TestWebAuthnBeginGuard_*` tests.

---

<a id="WEBAUTH-M2"></a>

#### WEBAUTH-M2 — RPID derived from user-controlled `Host` header

**Affected file:** `server/auth/webauthn.go:72` — `NewWebAuthnForRequest()`

**Description:**  
When `ego.server.webauthn.rpid` is not configured, the RPID and allowed origin
are derived directly from the `r.Host` header, which is caller-supplied. Behind
a misconfigured reverse proxy an attacker can set an arbitrary `Host` value,
causing the server to issue challenges bound to an attacker-chosen RPID. The
real users' passkeys will not satisfy such challenges, but the attack can
generate junk ceremony state and is a defense-in-depth gap.

**Recommendation:**  
Document that `ego.server.webauthn.rpid` **must** be set in any
internet-facing or production deployment. Add a startup warning when the setting
is absent and the server is not bound to `localhost`. The auto-derive path
should be treated as a local-development convenience only.

**Resolution (April 2026):**  
A startup warning is emitted via the `server.webauthn.no.rpid` SERVER log key
in `defineNativeAdminHandlers` (`commands/server.go`) when
`ego.server.allow.passkeys` is true and `ego.server.webauthn.rpid` is empty.
Localized strings added to all three language files.

---

<a id="WEBAUTH-M3"></a>

#### WEBAUTH-M3 — `storeChallenge` mutates shared cache expiration on every call

**Affected file:** `server/server/webauthn.go:74` — `storeChallenge()`

**Description:**  
`storeChallenge` calls `caches.SetExpiration(caches.WebAuthnChallengeCache, "5m")`
on every invocation before adding the new nonce. `SetExpiration` acquires a
write lock and mutates the `Expiration` field that governs **all** entries
subsequently added to that cache. Because the duration is hardcoded to `"5m"`
the mutation is idempotent in practice, but it introduces an unnecessary
write-lock contention point on every ceremony begin request and creates a
time-of-check/time-of-use race window between the `SetExpiration` and `Add`
calls.

**Recommendation:**  
Call `SetExpiration` once at server startup alongside route registration, and
remove the call from `storeChallenge`.

**Resolution (April 2026):**  
`caches.SetExpiration(caches.WebAuthnChallengeCache, "5m")` moved from
`storeChallenge` to `defineNativeAdminHandlers` in `commands/server.go`, where
it is called once when the WebAuthn routes are registered. The redundant call
has been removed from `storeChallenge`.

---

### Low / Informational

<a id="WEBAUTH-L1"></a>

#### WEBAUTH-L1 — `Secure` flag absent on challenge cookie

**Affected file:** `server/server/webauthn.go:51` — `challengeCookie()`

**Description:**  
The nonce cookie used to correlate the browser with the server's challenge cache
is `HttpOnly` and `SameSite: Strict`, but the `Secure` attribute is not set.
In any configuration where the server accepts plain HTTP (before an HTTPS
redirect, or in a development setup), the nonce could be transmitted in
cleartext. Browsers enforce HTTPS for WebAuthn themselves, which limits
practical exposure in the field, but the cookie hardening is incomplete.

**Resolution:**  
`challengeCookie` now accepts a `secure bool` parameter. The `isSecureRequest(r)`
helper returns `true` when `r.TLS != nil` or `X-Forwarded-Proto: https` is set.
All four call sites pass `isSecureRequest(r)`.

---

<a id="WEBAUTH-L2"></a>

#### WEBAUTH-L2 — Cache item expiration only refreshed when CACHE logging is active

**Affected file:** `caches/find.go:46`

**Description:**  
The sliding-expiration update inside `caches.Find` is nested inside
`if ui.IsActive(ui.CacheLogger)`. When CACHE logging is disabled (the normal
production state), cache items are never given a refreshed TTL on access — they
always expire at their original creation time. When CACHE logging is enabled,
any access extends an item's lifetime. This means server behavior changes
silently based on a logging flag. For WebAuthn challenge nonces the effect is
harmless (nonces are deleted immediately after the first successful
`loadChallenge` call), but the pattern is fragile for other cache classes such
as `TokenCache` and `AuthCache`.

**Resolution:**  
The expiration-refresh update in `caches/find.go` has been moved outside the
`if ui.IsActive(ui.CacheLogger)` block. It now executes on every cache hit
regardless of log level.

---

<a id="WEBAUTH-L3"></a>

#### WEBAUTH-L3 — No user notification when passkeys are cleared by an administrator

**Affected file:** `server/server/webauthn.go` — `WebAuthnClearPasskeysHandler`

**Description:**  
An administrator can silently remove all passkeys for any user account. The
operation is audit-logged server-side, but the affected user receives no
in-band notification. A compromised admin account could degrade every user's
authentication to password-only without any visible indication to those users.

**Resolution:**  
`WebAuthnClearPasskeysHandler` now emits a `SERVER`-logger entry using the new
`server.webauthn.admin.cleared.passkeys` key when the actor differs from the
target user. The SERVER log level is always shown in the dashboard Log tab,
making the action visible without requiring elevated log settings.

---

<a id="area-http"></a>

## HTTP — HTTP Server

This section covers vulnerabilities in the HTTP request/response pipeline as
implemented in `server/server/serve.go`, `server/server/router.go`, and the
server-startup code in `commands/server.go`. These issues are independent of the
authentication and WebAuthn concerns documented above.

| ID | Severity | Summary | Status |
| :--- | :--- | :--- | :---: |
| [HTTP-H1](#HTTP-H1) | High | No request body size limit | ✓ |
| [HTTP-H2](#HTTP-H2) | High | No timeouts on the HTTP server — Slowloris vulnerability | ✓ |
| [HTTP-M1](#HTTP-M1) | Medium | Security response headers not set | ✓ |
| [HTTP-M2](#HTTP-M2) | Medium | Server UUID and session counter disclosed on every response | ✓ |
| [HTTP-M3](#HTTP-M3) | Medium | Redirect server (port 80) has no timeouts | ✓ |
| [HTTP-L1](#HTTP-L1) | Low | User-supplied URL path reflected verbatim in error messages | ✓ |
| [HTTP-L2](#HTTP-L2) | Low | Request body read after permission check already set a failure status | ✓ |

### High

<a id="HTTP-H1"></a>

#### HTTP-H1 — No request body size limit

**Affected file:** `server/server/serve.go:313`

```go
session.Body, _ = io.ReadAll(r.Body)
```

**Description:**  
Every non-lightweight request has its body read into memory with a bare
`io.ReadAll`. There is no call to `http.MaxBytesReader` and no size check
before the read. An attacker — authenticated or not — can send a POST or PUT
request with a body of arbitrary size (limited only by their bandwidth and the
server's available RAM). Because the read happens unconditionally before the
handler is invoked, even endpoints that ignore the body will fully consume it.
A single large request can cause the Go garbage collector to thrash; a flood of
them can exhaust virtual memory and crash the process.

**Recommendation:**  
Wrap the request body with `http.MaxBytesReader` before calling `io.ReadAll`.
Choose a generous-but-bounded limit appropriate to the largest legitimate
payload (e.g., 32 MiB for the Ego server's use cases, configurable via a
setting):

```go
const maxBodyBytes = 32 << 20  // 32 MiB

r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
session.Body, _ = io.ReadAll(r.Body)
```

`http.MaxBytesReader` returns a `*http.MaxBytesError` when the limit is
exceeded, which `io.ReadAll` surfaces as a non-nil error. Check the error and
return 413 Request Entity Too Large.

**Resolution:**  
`http.MaxBytesReader` wraps `r.Body` before `io.ReadAll`; returns 413 on
oversize body; limit defaults to 32 MiB, configurable via
`ego.server.max.body.size`.

---

<a id="HTTP-H2"></a>

#### HTTP-H2 — No timeouts on the HTTP server — Slowloris vulnerability

**Affected files:**

- `commands/server.go:250` — plain HTTP path: `http.ListenAndServe(addr, router)`
- `commands/server.go:506` — TLS path: `http.ListenAndServeTLS(addr, certFile, keyFile, router)`

**Description:**  
Both server start paths use the convenience functions `http.ListenAndServe` and
`http.ListenAndServeTLS`, which create an `http.Server` with all timeout fields
left at their zero value — meaning no timeout at all. This makes the server
vulnerable to the Slowloris attack: an attacker opens many connections and
sends HTTP request headers one byte at a time, never completing them. Each
such connection holds a Go goroutine and a file descriptor open indefinitely.
With enough connections (default Linux limit is typically 1024 open file
descriptors per process), the server stops accepting new legitimate requests.
No authentication is required — the attack happens before the request headers
are complete.

**Recommendation:**  
Replace the convenience functions with an explicit `http.Server` that sets
all four timeout fields. Recommended starting values:

```go
srv := &http.Server{
    Addr:              addr,
    Handler:           router,
    ReadHeaderTimeout: 10 * time.Second,  // time to receive all headers
    ReadTimeout:       60 * time.Second,  // time to receive the full request
    WriteTimeout:      120 * time.Second, // time to send the full response
    IdleTimeout:       120 * time.Second, // keep-alive idle before closing
}
err = srv.ListenAndServeTLS(certFile, keyFile)
```

`ReadHeaderTimeout` is the most critical: it directly stops the Slowloris
pattern by closing connections that do not finish sending headers within the
window. The values above are reasonable defaults; consider making them
configurable via server settings for environments with large response bodies
(e.g., log retrieval).

**Resolution:**  
`makeHTTPServer()` helper constructs `http.Server` with `ReadHeaderTimeout`
(10 s), `ReadTimeout` (30 s), `WriteTimeout` (120 s), and `IdleTimeout` (120 s);
all three listeners (plain HTTP, TLS, and HTTP→HTTPS redirect) use it; all four
values are configurable via `ego.server.{read.header|read|write|idle}.timeout`.

---

### Medium

<a id="HTTP-M1"></a>

#### HTTP-M1 — Security response headers not set

**Affected file:** `server/server/serve.go` — `ServeHTTP()`

**Description:**  
The server never adds any of the standard browser-security response headers.
While most Ego server clients are CLI tools or the dashboard SPA rather than
general browsers, the dashboard is a browser application and omitting these
headers leaves it exposed to well-known classes of attack:

| Missing header | Risk if absent |
| :--- | :--- |
| `Content-Security-Policy` | XSS via injected scripts in the dashboard |
| `X-Content-Type-Options` (no-sniff) | MIME-sniffing attacks on API responses |
| `X-Frame-Options: DENY` | Click-jacking via embedding the dashboard in an iframe |
| `Strict-Transport-Security` | Browser allows downgrade to HTTP after first HTTPS visit |
| `Referrer-Policy` | Auth tokens in URL leaked via the HTTP referrer request header |

**Recommendation:**  
Add a thin middleware layer (or add directly to `ServeHTTP` before calling the
handler) that sets the defensive headers on every response:

```go
w.Header().Set("X-Content-Type-Options", "nosniff")
w.Header().Set("X-Frame-Options", "DENY")
w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
if r.TLS != nil {
    w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
}
```

A `Content-Security-Policy` appropriate for the dashboard requires more
thought (it must whitelist the sources used by the dashboard's JavaScript and
CSS), but at minimum a restrictive default of `default-src 'self'` should be
set and relaxed only for the dashboard routes that need it.

Due to dashboard problems, current CSP settings are as follows. The unsafe-inline
was added because dashboard was being blocked from itself.

- default-src 'self'
- script-src 'self' 'unsafe-inline'
- style-src 'self' 'unsafe-inline'
- object-src 'none'
- base-uri 'self'

**Resolution:**  
`addSecurityHeaders()` in `serve.go` sets `X-Content-Type-Options`,
`X-Frame-Options`, `Referrer-Policy`, `Content-Security-Policy`, and (TLS only)
`Strict-Transport-Security` on every response.

---

<a id="HTTP-M2"></a>

#### HTTP-M2 — Server UUID and session counter disclosed on every response

**Affected file:** `server/server/serve.go:65`

```go
w.Header()[defs.EgoServerInstanceHeader] = []string{
    fmt.Sprintf("%s:%d", defs.InstanceID, sessionID),
}
```

**Description:**  
Every non-lightweight response carries the `X-Ego-Server` header whose value
is `<UUID>:<sessionID>`. This discloses two pieces of information:

1. **Persistent server UUID** — `defs.InstanceID` is stable for the life of the
   server process. An attacker learns a fingerprint that lets them confirm they
   are talking to the same server instance across requests, enumerate multiple
   instances in a cluster, and correlate responses in log analysis.

2. **Monotonically increasing session counter** — `sessionID` is a global
   sequence number that increments for every request. An attacker can measure
   the exact rate of legitimate traffic, detect low-activity windows optimal
   for an attack, and infer whether a request they injected was processed by
   comparing counter values before and after.

Neither piece of information is needed by well-behaved clients; the dashboard
reads it only for display purposes. The same UUID is already in the
`server_info` JSON body for clients that genuinely need it.

**Recommendation:**  
Remove the header from production responses, or make it opt-in for clients
that need it (e.g. behind an `X-Ego-Debug` request header only recognized
when debug logging is active). At minimum, omit the session counter from the
header value so the UUID alone is disclosed — it is already available in the
response body for clients that need it.

**Resolution:**  
`X-Ego-Server` header removed entirely.

---

<a id="HTTP-M3"></a>

#### HTTP-M3 — Redirect server (port 80) has no timeouts

**Affected file:** `commands/server.go:705` — `redirectToHTTPS()`

```go
httpSrv := http.Server{
    Addr:    httpAddr,
    Handler: http.HandlerFunc(...),
}
```

**Description:**  
The plain-HTTP listener created to redirect traffic from port 80 to HTTPS
suffers the same timeout omission as the main server (HTTP-H2). Because it
must remain reachable from the public internet to redirect HTTP clients, it is
the most exposed component, yet it has zero protection against slow-reading
attackers.

**Recommendation:**  
Apply the same `ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`, and
`IdleTimeout` values to `httpSrv` as recommended in HTTP-H2. For a redirect-
only server, even more aggressive timeouts are appropriate (e.g.,
`ReadHeaderTimeout: 5s`), since legitimate redirect clients will complete their
headers in milliseconds.

**Resolution:**  
Resolved as a side-effect of HTTP-H2: `redirectToHTTPS` now builds its listener
via `makeHTTPServer()`, which applies all four timeout values.

---

### Low / Informational

<a id="HTTP-L1"></a>

#### HTTP-L1 — User-supplied URL path reflected verbatim in error messages

**Affected file:** `server/server/serve.go:79`

```go
msg = "endpoint " + r.URL.Path + " not found"
```

**Description:**  
When a route is not found, the raw URL path from the request is concatenated
directly into the error message string that is returned to the client (and
written to the server log). This is not a direct injection risk for JSON-
consuming API clients, but it has two minor consequences:

1. **Information disclosure** — The exact URL string the attacker sent is
   reflected back, confirming what path patterns do and do not exist on the
   server. This makes reconnaissance easier.
2. **Log injection** — If the URL path contains newline characters or log-
   format control sequences, they appear verbatim in the structured SERVER log
   entry, potentially corrupting log output or confusing log parsers.

**Recommendation:**  
Return a generic message to the client (`"not found"`) and include the raw path
only in the server-side log:

```go
util.ErrorResponse(w, sessionID, "not found", status)
ui.Log(ui.ServerLogger, "server.route.error", ui.A{
    ...
    "path": r.URL.Path,  // path stays in the log, not in the response
})
```

**Resolution:**  
Generic `"not found"` / `"forbidden"` returned to client; raw URL path kept
only in the `server.route.error` log entry.

---

<a id="HTTP-L2"></a>

#### HTTP-L2 — Request body read after permission check already set a failure status

**Affected file:** `server/server/serve.go:302–318`

**Description:**  
When the `mustBeAdmin` check fails (the user is authenticated but lacks admin
privileges), the code sets `status = http.StatusForbidden` but does **not**
`return`. Execution falls through to the unconditional `io.ReadAll(r.Body)` on
line 313, which reads the full request body into memory before the handler
(correctly) declines to run. The same pattern applies to the
`mustAuthenticate + canAuthenticate` branch that sets 401. In both cases a
non-privileged caller can force the server to allocate memory for whatever body
they attach to a privileged endpoint. While the individual impact is low, it
directly compounds HTTP-H1 (no body size limit): a stream of 403-destined
requests with large bodies can exhaust memory without ever triggering the
handler path.

**Recommendation:**  
Add an early `return` after each auth/permission failure that sets a non-OK
status, so the body is never read for requests that are going to be rejected:

```go
} else if route.mustBeAdmin && !session.Admin {
    ...
    util.ErrorResponse(w, session.ID, "not authorized", http.StatusForbidden)
    return  // ← add this
}
```

**Resolution:**  
`mustAuthenticate` and `mustBeAdmin` failure branches in `ServeHTTP` now
`return` immediately after sending the error response; request body is never
read for rejected requests.

---

<a id="area-tables"></a>

## TABLES — Tables Endpoint

This section records security weaknesses in the `/tables/` and `/dsns/` endpoint
handlers as implemented in `server/tables/`, identified during a code review in
April 2026.

| ID | Severity | Summary | Status |
| :--- | :--- | :--- | :---: |
| [TABLES-C1](#TABLES-C1) | Critical | Inverted guard in `CommitHandler` panics on every valid commit | ✓ |
| [TABLES-H1](#TABLES-H1) | High | SQL injection via raw username in table-list query | ✓ |
| [TABLES-H2](#TABLES-H2) | High | Inverted authorization filter in `ListTables` leaks table names | ✓ |
| [TABLES-M1](#TABLES-M1) | Medium | SQL injection in `DeleteTable` via un-parameterized `DROP TABLE` | ✓ |
| [TABLES-M2](#TABLES-M2) | Medium | Row ID concatenated un-parameterized into `UPDATE` WHERE clause | ✓ |
| [TABLES-M3](#TABLES-M3) | Medium | Wrong permission constant in `DeleteTable` allows any user to delete tables | ✓ |
| [TABLES-M4](#TABLES-M4) | Medium | No server-side cap on query result size | ✓ |
| [TABLES-L1](#TABLES-L1) | Low | Raw database error messages returned to clients | ✓ |
| [TABLES-L2](#TABLES-L2) | Low | SQLite `PRAGMA` statements use unquoted table and index names | ✓ |

### Critical

<a id="TABLES-C1"></a>

#### TABLES-C1 — `CommitHandler` inverted guard panics on every valid commit

**Affected file:** `server/tables/transactions.go:207` — `CommitHandler()`

```go
parameters := session.Parameters[defs.TransactionIDParameterName]
if len(parameters) != 0 {          // ← should be != 1
    return util.ErrorResponse(...)  // rejects requests WITH a transaction ID
}
id := data.String(parameters[0])   // panics: index 0 on empty slice
```

**Description:**  
The guard condition is inverted relative to `RollbackHandler` (line 170, which
correctly uses `!= 1`). The result is that any request that arrives *with* a
valid transaction ID parameter — i.e., every legitimate commit — is immediately
rejected with "missing transaction ID". Any request that arrives *without* a
transaction ID parameter — i.e., every malformed commit — passes the guard and
then panics on line 212 (`parameters[0]` on an empty slice), crashing the
goroutine handling that connection.

**Recommendation:**  
Change `!= 0` to `!= 1` on line 207, matching `RollbackHandler`.

**Resolution:**  
Change `CommitHandler` guard from `len(parameters) != 0` to `!= 1`; matches
`RollbackHandler` and prevents panic on `parameters[0]` with empty slice.

---

### High

<a id="TABLES-H1"></a>

#### TABLES-H1 — SQL injection via raw username in table-list query

**Affected file:** `server/tables/list.go:69` — `listTables()`

```go
schema := session.User
q := strings.ReplaceAll(tablesListQuery, "{{schema}}", schema)
// tablesListQuery = `... WHERE table_schema = '{{schema}}' ORDER BY ...`
```

**Description:**  
The authenticated user's name is substituted directly into a SQL string using
`strings.ReplaceAll`, bypassing the `parsing.QueryParameters` / `SQLEscape`
pipeline that all other query parameters go through. The value lands inside a
single-quoted SQL literal, but `SQLEscape` is never called. A username whose
middle characters include a single quote (e.g. `O'Reilly`) breaks the literal
and allows injection:

```sql
WHERE table_schema = 'O'Reilly' ORDER BY table_name
```

A more deliberately crafted name (`x' UNION SELECT username,passwd FROM
pg_shadow--`) could exfiltrate sensitive data from the database.

**Recommendation:**  
Route the schema substitution through `parsing.QueryParameters`, which calls
`SQLEscape` on every value. Alternatively, use a parameterized query where the
driver handles quoting: `db.Query("... WHERE table_schema = $1 ...", schema)`.

**Resolution:**  
Route schema substitution in `listTables` through `parsing.QueryParameters`
(which calls `SQLEscape`) instead of bare `strings.ReplaceAll`.

---

<a id="TABLES-H2"></a>

#### TABLES-H2 — Inverted authorization filter in `ListTables` leaks table names

**Affected file:** `server/tables/list.go:136` — `getTableNames()`

```go
if !db.Session.Admin && Authorized(db.Session, db.Session.User, name, defs.TableReadPermission) {
    continue  // skips tables the user IS authorized to read
}
```

**Description:**  
The `!` before `Authorized` is missing. `Authorized` returns `true` when the
user has the requested permission. The current code therefore skips (hides)
every table the user *is* permitted to read, and exposes every table they are
*not* permitted to read.

For non-secured DSNs (the common case) `Authorized` always returns `true`, so
the `continue` fires for every table and non-admin users see an empty list —
a functional denial of service. For secured DSNs the effect is reversed access
control: tables a non-admin user may read are hidden from them, while tables
they have no business seeing are visible.

**Recommendation:**  
Add the negation: `if !db.Session.Admin && !Authorized(...)`.

**Resolution:**  
Add missing `!` to `Authorized` call in `getTableNames` so tables the user
cannot read are filtered out, not those they can.

---

### Medium

<a id="TABLES-M1"></a>

#### TABLES-M1 — SQL injection in `DeleteTable` via un-parameterized `DROP TABLE`

**Affected file:** `server/tables/tables.go:327` — `DeleteTable()`

```go
if dsnName != "" {
    tableName = table                // table = data.String(session.URLParts["table"])
    q = "DROP TABLE " + tableName   // raw concatenation
}
```

**Description:**  
When a DSN name is present in the request, the table-deletion query is
constructed by concatenating the URL path parameter directly into a SQL
string, without quoting or escaping. An authenticated caller who can reach
the delete-table endpoint can supply a table name such as
`users; DROP TABLE admin; --` to execute arbitrary SQL on the DSN's database.
The code path that uses `parsing.QueryParameters` (lines 316–318) is bypassed
entirely in the DSN case.

**Recommendation:**  
Use double-quote escaping consistent with the non-DSN path:
`q = "DROP TABLE \"" + tableName + "\""`. Better still, route through
`parsing.QueryParameters` regardless of whether a DSN is present.

**Resolution:**  
Replace `"DROP TABLE " + tableName` in the DSN branch of `DeleteTable` with a
quoted identifier; route through `parsing.QueryParameters` for consistency.

---

<a id="TABLES-M2"></a>

#### TABLES-M2 — Row ID concatenated un-parameterized into `UPDATE` WHERE clause

**Affected file:** `server/tables/parsingAbstract.go:77` — `formAbstractUpdateQuery()`

```go
where = "WHERE " + defs.RowIDName + " = '" + idString + "'"
```

**Description:**  
The row ID value taken from the update payload is placed inside a SQL WHERE
clause using string concatenation with single-quote delimiters. While the rest
of the UPDATE statement uses `$N` parameterized placeholders (lines 62–63),
the row-ID filter reverts to direct embedding. A caller who can control the
row ID value (e.g. from the JSON body of a PATCH request) and supplies
`' OR '1'='1` causes the UPDATE to affect every row in the table.

**Recommendation:**  
Append a numbered parameter for the row ID instead of interpolating it:

```go
where = fmt.Sprintf("WHERE %s = $%d", defs.RowIDName, filterCount+1)
// pass idString as the corresponding argument to db.Exec
```

**Resolution:**  
Convert row ID filter in `formAbstractUpdateQuery` to a `$N` numbered
parameter passed to `db.Exec` rather than string-embedded in the WHERE clause.

---

<a id="TABLES-M3"></a>

#### TABLES-M3 — Wrong permission constant in `DeleteTable` allows any authenticated user to delete tables

**Affected file:** `server/tables/tables.go:312` — `DeleteTable()`

```go
if !isAdmin && dsnName == "" && !Authorized(session, user, tableName, defs.AdminAgent) {
    return util.ErrorResponse(..., "User does not have read permission", ...)
}
```

**Description:**  
Two bugs combine here. First, `defs.AdminAgent` has the value `"admin"` — a
string constant identifying an agent type, not a table-permission name. The
`Authorized` switch statement has no case for `"admin"`, so the per-operation
loop runs without ever setting `auth = false`, and `Authorized` returns `true`
for any authenticated user. The guard `!Authorized(...)` is therefore always
`false`, meaning no non-admin user is ever blocked from deleting a table they
do not own. Second, the error message says "read permission" when the intent
is an admin/delete permission check.

**Recommendation:**  
Replace `defs.AdminAgent` with `defs.TableAdminPermission` (= `"ego.table.admin"`)
and update the error message accordingly.

**Resolution:**  
Replace `defs.AdminAgent` with `defs.TableAdminPermission` in the `DeleteTable`
authorization check; correct the error message from "read permission" to
"admin permission".

---

<a id="TABLES-M4"></a>

#### TABLES-M4 — No server-side cap on query result size

**Affected file:** `server/tables/parsing/generators.go:748` — `PagingClauses()`

```go
if limit != 0 {
    result.WriteString(" LIMIT ")
    result.WriteString(strconv.Itoa(limit))
}
// No maximum enforced; if limit == 0 (absent or zero), no LIMIT clause added.
```

**Description:**  
When no `?limit=` query parameter is supplied, or when it is supplied as `0`,
no `LIMIT` clause is appended to the generated SQL. An authenticated caller
can retrieve an entire table — potentially millions of rows — in a single
request. The rows are buffered in the server process before being marshalled
to JSON and written to the response, making this an effective memory-exhaustion
DoS against the server.

**Recommendation:**  
Enforce a server-side maximum: if `limit <= 0 || limit > maxRowLimit`, set
`limit = defaultRowLimit` (e.g. 1000). Expose the maximum as a configurable
setting. Document the behavior so callers can paginate deliberately.

**Resolution:**  
Enforce a server-side maximum row limit in `PagingClauses`; default to 1000
rows when no limit is specified.

---

### Low / Informational

<a id="TABLES-L1"></a>

#### TABLES-L1 — Raw database error messages returned to clients

**Affected files (representative):**

- `server/tables/list.go:51,56`
- `server/tables/transactions.go:225`
- `server/tables/tables.go:320`

```go
msg := fmt.Sprintf("Database list error, %v", err)
util.ErrorResponse(w, session.ID, msg, http.StatusBadRequest)
```

**Description:**  
Database driver errors — which include table names, column names, constraint
names, SQL syntax fragments, and internal driver details — are forwarded
verbatim to the HTTP response body. An attacker can exploit error responses to
enumerate schema structure, discover column and constraint names without having
read access, and tailor further injection attempts to the precise SQL dialect
in use.

**Recommendation:**  
Log the full error at `ui.DBLogger` and return a generic message
(`"database operation failed"`) to the caller. Reserve detailed messages for
the server log where access is controlled.

**Resolution:**  
Log full database errors server-side and return generic messages in HTTP
error responses from the tables package.

---

<a id="TABLES-L2"></a>

#### TABLES-L2 — SQLite `PRAGMA` statements use unquoted table and index names

**Affected file:** `server/tables/describe.go:244,276,307`

```go
q := fmt.Sprintf("PRAGMA index_list(%s)", tableName)
q := fmt.Sprintf("PRAGMA index_info(%s)", index)
q  = fmt.Sprintf("PRAGMA table_info(%s)", tableName)
```

**Description:**  
SQLite PRAGMA arguments are not parameterizable via `database/sql`, but names
with spaces or special characters still need to be quoted. The table names
passed here come from the server's own schema metadata (a prior `sqlite_schema`
query), so the practical risk is low — an attacker would need to have already
created a table with a malicious name. Nevertheless, the pattern violates the
principle of always quoting identifiers and would allow a name such as
`my table` (with a space) to silently truncate the PRAGMA to `PRAGMA
index_list(my)`.

**Recommendation:**  
Wrap each name in backticks or double-quotes:

```go
q := fmt.Sprintf("PRAGMA index_list(\"%s\")", tableName)
```

**Resolution:**  
Wrap table and index names in double-quotes in all three SQLite `PRAGMA`
format strings in `describe.go`.

---

<a id="area-asset"></a>

## ASSET — Asset Handler

This section records security weaknesses in the `/assets/` endpoint as implemented
in `server/assets/handler.go`, identified during a code review in April 2026.

| ID | Severity | Summary | Status |
| :--- | :--- | :--- | :---: |
| [ASSET-H1](#ASSET-H1) | High | DoS via open-ended `Range` header | ✓ |
| [ASSET-M1](#ASSET-M1) | Medium | Wrong `Content-Range` header for open-ended ranges | ✓ |
| [ASSET-M2](#ASSET-M2) | Medium | Error response discloses server filesystem paths | ✓ |
| [ASSET-L1](#ASSET-L1) | Low | Double `os.ReadFile` call in `readAssetFile` | ✓ |
| [ASSET-L2](#ASSET-L2) | Low | Path normalization uses fragile string removal instead of `filepath.Clean` | ✓ |

### High

<a id="ASSET-H1"></a>

#### ASSET-H1 — DoS via open-ended `Range` header

**Affected file:** `server/assets/handler.go:296` — `readAssetRange()`

**Description:**  
A request carrying `Range: bytes=N-` (open-ended range, no explicit end byte)
leaves the `end` variable at its sentinel value `EndOfData = math.MaxInt64`.
Because `start != StartOfData`, the Loader skips the cache path and calls
`readAssetRange`. Inside that function, `size := end - start` evaluates to
approximately 9.2 EB, and the subsequent `make([]byte, size)` attempts to
allocate that many bytes. The Go runtime raises an out-of-memory condition
before the allocation completes, which — if not recovered — terminates the
server process. A single unauthenticated GET request with a valid asset path
and an open-ended Range header is sufficient to trigger this.

**Recommendation:**  
After calling `os.Stat` to obtain the true file size, clamp `end` to
`totalSize - 1` whenever it equals `EndOfData` or exceeds the file size.
This is the correct RFC 7233 interpretation of an open-ended range.

**Resolution (April 2026):**  
Clamping added in `readAssetRange` immediately after `totalSize = info.Size()`.
`size` is now computed as `end - start + 1` (inclusive, per RFC 7233). The
`make` call is therefore bounded by the actual file size.

---

### Medium

<a id="ASSET-M1"></a>

#### ASSET-M1 — Wrong `Content-Range` header for open-ended ranges

**Affected file:** `server/assets/handler.go:176` — `AssetsHandler()`

**Description:**  
When a client sends `Range: bytes=N-`, the `end` variable in `AssetsHandler`
remains `EndOfData` (= `math.MaxInt64`) after parsing, because no explicit end
byte was specified. The handler then wrote this unmodified value directly into
the `Content-Range` response header: `bytes N-9223372036854775807/1000`. This
violates RFC 7233 §4.2, which requires the last-byte-pos in the Content-Range
header to reflect the actual last byte sent (i.e. `totalSize - 1`). Clients
that strictly parse the Content-Range value may reject or mishandle the
response.

**Recommendation:**  
After `Loader` returns `totalSize`, clamp `end` to `totalSize - 1` before
formatting the Content-Range header.

**Resolution (April 2026):**  
A `reportEnd` local variable is computed from `end`, clamped to `totalSize - 1`
when `end == EndOfData || end >= totalSize`, and used in the `Content-Range`
header format string. The handler's own `end` variable is not mutated.

---

<a id="ASSET-M2"></a>

#### ASSET-M2 — Error response discloses server filesystem paths

**Affected file:** `server/assets/handler.go:131` — `AssetsHandler()`

**Description:**  
When `Loader` returned an error (file not found, permission denied, etc.), the
handler embedded the raw OS error string — which contains the absolute
filesystem path — directly in the JSON response body:
`{"err": "open /home/tom/ego/lib/foo.txt: no such file or directory"}`. A
partial `strings.ReplaceAll` only stripped the `services` subdirectory; all
other paths were exposed verbatim. An unauthenticated caller could use this to
map the server's directory layout, confirm the existence of files, and discover
the configured `EGO_PATH`.

**Recommendation:**  
Return a fixed generic message to the caller and keep full error detail in the
server log only.

**Resolution (April 2026):**  
The error branch in `AssetsHandler` now writes the literal string
`{"err": "asset not found"}` to the response for all load failures. The
original error (including the real path) continues to be written to the
`AssetLogger` via the existing `asset.load.error` log key.

---

### Low / Informational

<a id="ASSET-L1"></a>

#### ASSET-L1 — Double `os.ReadFile` call in `readAssetFile`

**Affected file:** `server/assets/handler.go:330,356` — `readAssetFile()`

**Description:**  
`os.ReadFile(fn)` was called twice: once at line 330 (result captured in `data`
and `err`), then again at line 356 as the function's return value. The first
result was used only for logging and then discarded; the second read was what
actually propagated to the caller. If the file was modified or deleted between
the two reads, the caller received different content (or an error) than what
was logged. In addition, it doubled the I/O cost of every uncached asset load.

**Recommendation:**  
Return the `data` and `err` captured by the first read.

**Resolution (April 2026):**  
`return os.ReadFile(fn)` at line 356 replaced with `return data, err`.

---

<a id="ASSET-L2"></a>

#### ASSET-L2 — Path normalization uses fragile string removal instead of `filepath.Clean`

**Affected file:** `server/assets/handler.go:359` — `normalizeAssetPath()`

**Description:**  
The function removed `..` components by calling `strings.ReplaceAll(path, "..", "")`
twice — once on the relative path and once on the fully-joined path. While no
active bypass was demonstrated against this specific code, naive string removal
is a well-known anti-pattern for path traversal defense: variations such as
`....//` survive the replacement and produce unexpected results after
`filepath.Join` normalizes double-slashes. The `AssetsHandler` check at line 69
only guards against `/../` mid-path and would not catch a traversal that reached
`normalizeAssetPath` directly (e.g. via the exported `Loader` function called
from dashboard handlers).

**Recommendation:**  
Use `filepath.Clean(filepath.Join(root, path))` for canonical resolution, then
verify the result is still inside `root` with a `strings.HasPrefix` confinement
check. Return a guaranteed-nonexistent path on confinement failure so the
caller handles it uniformly as a 404.

**Resolution (April 2026):**  
`normalizeAssetPath` rewritten to: (1) compute `root` as before, (2) call
`filepath.Clean(filepath.Join(root, path))` for one-pass canonical resolution,
(3) verify the result starts with `root + string(filepath.Separator)`, and
(4) return `filepath.Join(root, "__invalid__")` on confinement failure.
The loop that stripped leading dots/slashes and both `strings.ReplaceAll`
calls have been removed.

---

<a id="area-profile"></a>

## PROFILE — Profile Encryption

This section records security weaknesses in the profile encryption subsystem
implemented in `app-cli/settings/crypto.go`. Profile encryption protects
sensitive configuration values — bearer tokens, database credentials, the
server token-signing key — that are stored in sidecar files alongside the main
profile JSON or in the settings database.

| ID | Severity | Summary | Status |
| :--- | :--- | :--- | :---: |
| [PROFILE-H1](#PROFILE-H1) | High | MD5 used to derive the AES encryption key | ✓ |
| [PROFILE-M1](#PROFILE-M1) | Medium | Legacy ciphertext not re-encrypted on successful read | ✓ |

### High

<a id="PROFILE-H1"></a>

#### PROFILE-H1 — MD5 used to derive the AES encryption key

**Affected file:** `app-cli/settings/crypto.go` — `Hash()` / `encrypt()` / `decrypt()`

```go
// old code
block, _ := aes.NewCipher([]byte(Hash(passphrase)))
// Hash() returns hex.EncodeToString(md5.Sum(passphrase))
```

**Description:**  
The `Hash()` function runs the passphrase through MD5 and hex-encodes the
result to produce a 32-character string used as the raw AES-256 key. MD5 has
been cryptographically broken since 1996: collision attacks are trivially
feasible on commodity hardware, and pre-image resistance is significantly
weakened. While the 32-byte key length technically satisfies AES-256's
requirement, the key material itself carries only the entropy of an MD5
digest. An attacker who obtains a sidecar file (e.g. `$.token`, `$.key`,
`$.cred2`) can attempt an offline dictionary or brute-force attack using MD5
at billions of hashes per second on a consumer GPU, orders of magnitude faster
than would be possible with a modern KDF.

The passphrase itself is `profile.Name + profile.Salt + profile.ID`, which
provides some variable entropy, but the MD5 layer strips away any security
margin that salt would otherwise add.

**Recommendation:**  
Replace the `Hash()`-based key derivation with `sha256.Sum256([]byte(passphrase))`
which produces 32 bytes directly without the MD5 weakness and without the
double-encoding through hex. For a further improvement, adopt PBKDF2 or
HKDF with the profile salt as the KDF salt and a suitable iteration count,
though this requires a format change to store the per-value salt.

To preserve backward compatibility with existing sidecar files, prefix newly
encrypted output with a version tag (e.g. `"v2:"`) and detect the absence of
the tag in `Decrypt()` to route to the legacy decryption path.

**Resolution (April 2026):**  
`encrypt()` and `decrypt()` in `app-cli/settings/crypto.go` now derive the
AES-256 key via `deriveKey()`, which calls `sha256.Sum256([]byte(passphrase))`
and returns the 32-byte digest directly. All new ciphertext produced by
`Encrypt()` is prefixed with `"v2:"` before base64 encoding. `Decrypt()`
inspects the prefix: a `"v2:"` prefix routes to the SHA-256 path; the absence
of a prefix routes to `decryptLegacy()`, which preserves the original MD5-hex
key derivation for existing stored values. `decryptLegacy()` is clearly marked
for future removal once all stored values have been re-encrypted.

---

### Medium

<a id="PROFILE-M1"></a>

#### PROFILE-M1 — Legacy ciphertext not re-encrypted on successful read

**Affected files:** `app-cli/settings/files.go:303`, `app-cli/settings/databases.go:260`

**Description:**  
When `Decrypt()` successfully decodes a value that has no `"v2:"` prefix, the
result is returned and stored in the in-memory configuration, but the
underlying sidecar file (or database field) retains the old MD5-encrypted
ciphertext. As long as legacy values are never written back — for example, if
the setting is never explicitly changed — they remain permanently protected
only by the weaker MD5 key derivation. A long-running server that never
touches certain settings (the token key, for instance) may never trigger a
write that would upgrade those values.

**Recommendation:**  
After a successful `decryptLegacy()` call in the `Decrypt()` routing, mark
the configuration as dirty so that the next profile save re-encrypts the value
with the new SHA-256 scheme. Alternatively, perform an eager re-encrypt-and-
save immediately after the read. Either approach ensures that legacy ciphertext
is upgraded within one profile-read cycle.

**Resolution (April 2026):**  
Both read paths now call `NeedsNewHash()` on the raw ciphertext immediately
after a successful `Decrypt()`. When it returns true, the plaintext is
re-encrypted with `Encrypt()` (SHA-256 / v2 scheme) and written back to the
same storage location before the function returns:

- **File path** (`app-cli/settings/files.go` — `readOutboardConfigFiles()`):
  the sidecar file is overwritten with the new ciphertext via `os.WriteFile`.
- **Database path** (`app-cli/settings/databases.go` — `Load()`): upgraded
  items are collected during the row scan and written back with `UPDATE`
  statements after the cursor is closed.

Both paths log `config.reencrypted` (the i18n key) on success. After this change, legacy
MD5-encrypted values are upgraded to SHA-256 on the first read, with no
manual migration step required.

---

<a id="area-code"></a>

## CODE — Dashboard Code Execution

This section records security weaknesses in the `POST /admin/run` endpoint that
compiles and executes Ego source code submitted from the dashboard Code and
Console tabs. The handler is implemented in `server/admin/run.go`, with sandbox
enforcement spread across `bytecode/context.go`, `runtime/exec/`, and
`runtime/io/`. Issues are rated using the same severity scale as the sections
above.

| ID | Severity | Summary | Status |
| :--- | :--- | :--- | :---: |
| [CODE-H1](#CODE-H1) | High | Exec sandbox bypass when exec is globally permitted | ✓ |
| [CODE-M1](#CODE-M1) | Medium | No request body size limit on `POST /admin/run` | ✓ |
| [CODE-M2](#CODE-M2) | Medium | Global trace logger state mutated per-request without synchronization | ✓ |
| [CODE-M3](#CODE-M3) | Medium | Client-supplied session UUID not validated or bound to the authenticated user | ✓ |
| [CODE-M4](#CODE-M4) | Medium | Sandbox I/O path confinement can be bypassed via symlinks | |
| [CODE-M5](#CODE-M5) | Medium | Language extensions enabled in sandboxed symbol table | |
| [CODE-L1](#CODE-L1) | Low | Full user-submitted code body written to REST log | ✓ |

### High

<a id="CODE-H1"></a>

#### CODE-H1 — Exec sandbox bypass when exec is globally permitted

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

---

### Medium

<a id="CODE-M1"></a>

#### CODE-M1 — No request body size limit on `POST /admin/run`

**Affected file:** `server/admin/run.go:163` — `RunCodeHandler()`

```go
if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
```

**Description:**  
The request body is decoded with no preceding call to `http.MaxBytesReader`.
Any user who holds the `ego.server.admin` or `ego.code` permission can POST an
arbitrarily large body. The full body is buffered before the JSON decoder
returns, so a multi-megabyte `Code` field will be compiled and executed (or
at least compiled). A sustained flood of large requests can exhaust server
memory without triggering the global body-size limit applied at the transport
layer by HTTP-H1, because `RunCodeHandler` re-reads `r.Body` directly rather
than consuming the pre-read `session.Body` buffer used by most other handlers.

**Recommendation:**  
Wrap the body before decoding, and add a post-decode length check on the `Code`
field:

```go
const maxRunBodyBytes = 1 << 18  // 256 KiB — generous for any plausible script
r.Body = http.MaxBytesReader(w, r.Body, maxRunBodyBytes)
if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
    return util.ErrorResponse(w, session.ID, err.Error(), http.StatusRequestEntityTooLarge)
}
if len(req.Code) > maxRunCodeBytes {
    return util.ErrorResponse(w, session.ID, "code too large", http.StatusRequestEntityTooLarge)
}
```

**Resolution (April 2026):**  
`RunCodeHandler` now wraps `r.Body` with `http.MaxBytesReader` (256 KiB limit)
before JSON decoding. A `*http.MaxBytesError` from the decoder returns 413;
other decode errors return 400. A post-decode `len(req.Code) > maxRunCodeBytes`
check also returns 413 for oversized code fields.

---

<a id="CODE-M2"></a>

#### CODE-M2 — Global trace logger state mutated per-request without synchronization

**Affected file:** `server/admin/run.go:186` — `RunCodeHandler()`

```go
savedTrace := ui.IsActive(ui.TraceLogger)
ui.Active(ui.TraceLogger, req.Trace)
output, runErr := executeAdminEgo(session.ID, req.Code, req.Console, req.Trace, req.Session)
ui.Active(ui.TraceLogger, savedTrace)
```

**Description:**  
`ui.IsActive` and `ui.Active` operate on a single global logger-state map
shared across all goroutines. The read-modify-execute-restore sequence above is
not protected by any mutex. When two concurrent requests arrive with different
`Trace` values, one request can overwrite the other's saved state. This creates
two observable problems:

1. **Unintended trace exposure** — a request that did not ask for tracing may
   run with the trace logger enabled because a concurrent request enabled it
   after the first request saved `savedTrace = false`.
2. **Data race** — Go's race detector will flag concurrent reads and writes to
   the shared logger state as a data race (no synchronization).

Since the execution context already accepts a per-request trace flag via
`ctx.SetTrace(trace)`, the global mutation is unnecessary for controlling
per-execution tracing. The `ui.Active` calls are only needed if some code path
outside the context checks the global flag directly.

**Recommendation:**  
Remove the global `ui.Active` mutations from `RunCodeHandler`. Pass the trace
flag exclusively through the `bytecode.Context` (`ctx.SetTrace(req.Trace)`)
so each request controls its own tracing without touching shared state. If
global trace output is still required for some paths, protect the
save/restore pair with a dedicated mutex.

**Resolution (April 2026):**  
The save/restore of `ui.Active(ui.TraceLogger)` has been replaced with a
package-level `traceRunMu sync.Mutex`. When `req.Trace` is true, the handler
acquires the mutex, sets the logger active, and defers both the restore and the
unlock. Non-trace requests proceed without serialization. This eliminates the
data race while preserving global trace output for the bytecode run-loop log
messages that check `ui.IsActive(ui.TraceLogger)` directly.

---

<a id="CODE-M3"></a>

#### CODE-M3 — Client-supplied session UUID not validated or bound to the authenticated user

**Affected file:** `server/admin/run.go:179` — `RunCodeHandler()`

```go
if req.Session == "" {
    req.Session = uuid.New().String()
}
```

**Description:**  
The `Session` field is taken verbatim from the JSON request body and used
directly as the key into both `codeSessions` (persistent symbol tables) and
`debugSessions` (active debugger contexts). No format validation is performed —
the field accepts any string. More importantly, there is no binding between a
session key and the authenticated user who created it.

Two consequences follow:

1. **Session fixation** — a malicious user can specify a session UUID they
   already know (e.g. one observed or guessed from another user's traffic) and
   interact with that user's persistent symbol table or inject commands into
   their active debug session.
2. **Log injection** — the raw UUID is written to the SERVER log via
   `ui.A{"id": uuid}`. A crafted value containing newline characters or
   log-format control sequences can corrupt structured log output.

**Recommendation:**  
Validate that the client-supplied `Session` value conforms to UUID v4 format
before accepting it (reject with 400 otherwise). Additionally, bind each session
entry to the authenticated username at creation time and enforce that the
requesting user matches the session owner on every subsequent call:

```go
if entry.owner != session.User {
    return util.ErrorResponse(w, session.ID, "session not found", http.StatusNotFound)
}
```

Returning 404 rather than 403 avoids confirming the existence of another user's
session.

**Resolution (April 2026):**  
`RunCodeHandler` now calls `uuid.Parse(req.Session)` before using the value
and returns 400 for any non-UUID string. `codeSessionEntry` and `debugSession`
both gained an `owner string` field set to `session.User` at creation time.
`getOrCreateSymbolTable` and `executeAdminDebug` each check `entry.owner != user`
on session lookup and return an opaque error (`run.not.found` /
`ErrNoPrivilegeForOperation`) that does not reveal whether the session belongs
to another user. The localized `run.not.found` key has been added to all three
language files.

---

<a id="CODE-M4"></a>

#### CODE-M4 — Sandbox I/O path confinement can be bypassed via symlinks

**Affected file:** `runtime/io/io.go:121` — `sandboxName()`

```go
func sandboxName(flag bool, path string) string {
    if sandboxPrefix := settings.Get(defs.SandboxPathSetting); flag && sandboxPrefix != "" {
        if strings.HasPrefix(path, "../") || ... {
            path = strings.ReplaceAll(path, "..", "<invalid path>")
        }
        if strings.HasPrefix(path, sandboxPrefix) {
            return path
        }
        return filepath.Join(sandboxPrefix, path)
    }
    return path
}
```

**Description:**  
`sandboxName` prevents `..`-based directory traversal in the path string itself,
but does not resolve symlinks before checking or returning the path. If user
code (or a prior operation) creates a symlink inside the sandbox directory that
points to a location outside it, subsequent `io.Open` and `io.ReadDir` calls
will follow that symlink and access the target path, bypassing the confinement
entirely. For example:

```text
sandbox/escape -> /etc
io.Open("escape/passwd")  // sandboxName returns sandbox/escape/passwd
                           // os.OpenFile follows symlink → /etc/passwd
```

This is a well-known weakness of path-prefix confinement without symlink
resolution: the check is done on the path string rather than on the filesystem
object it ultimately refers to.

**Recommendation:**  
After computing the candidate path, resolve all symlinks with
`filepath.EvalSymlinks` and verify the result still falls under the sandbox
root before opening or listing:

```go
resolved, err := filepath.EvalSymlinks(candidate)
if err != nil || !strings.HasPrefix(resolved, sandboxPrefix+string(filepath.Separator)) {
    return filepath.Join(sandboxPrefix, "__invalid__")
}
return resolved
```

Note that `filepath.EvalSymlinks` requires the file to already exist; for
write operations (creating new files) that will not yet exist, verify the
parent directory instead.

**Status:** Open — not yet resolved.

---

<a id="CODE-M5"></a>

#### CODE-M5 — Language extensions enabled in sandboxed symbol table

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

**Status:** Open — not yet resolved.

---

### Low / Informational

<a id="CODE-L1"></a>

#### CODE-L1 — Full user-submitted code body written to REST log

**Affected file:** `server/admin/run.go:167` — `RunCodeHandler()`

```go
if ui.IsActive(ui.RestLogger) {
    b, _ := json.MarshalIndent(req, ui.JSONIndentPrefix, ui.JSONIndentSpacer)
    ui.Log(ui.RestLogger, "rest.request.payload", ui.A{
        "session": session.ID,
        "body":    string(b),
    })
}
```

**Description:**  
When the REST logger is active, the entire deserialized request — including the
`Code` field — is written to the server log. If a user submits code that
contains sensitive values (database credentials, API keys, personal data
embedded in test scripts), those values are persisted in the server log files
for the duration of the log retention period. This is particularly notable
because log files are typically accessible to a broader audience than the
dashboard session itself.

**Recommendation:**  
Redact or truncate the `Code` field before logging. A reasonable approach is to
log the first 120 characters and append an ellipsis when the field is longer:

```go
logReq := req
if len(logReq.Code) > 120 {
    logReq.Code = logReq.Code[:120] + "…"
}
b, _ := json.MarshalIndent(logReq, ui.JSONIndentPrefix, ui.JSONIndentSpacer)
```

This preserves enough context to identify the request in the log without
capturing the full script content.

**Resolution (April 2026):**  
`RunCodeHandler` now copies the request into a local `logReq` variable and
truncates `logReq.Code` to 120 characters (appending `"..."`) before passing
it to `json.MarshalIndent`. The original `req.Code` is unmodified and used
for execution as before.

---

<a id="area-oauth"></a>

## OAUTH — OAuth2 Authorization Server and Resource Server

This section records security weaknesses in the OAuth2 Authorization Server (AS)
implemented in `server/oauth/authserver/` and the OAuth2 Resource Server (RS) client
implemented in `server/oauth/` and `server/oauth/rshandlers/`. The AS provides
standard OAuth2/OIDC endpoints; the RS validates JWT Bearer tokens issued by an
external identity provider and drives the Authorization Code + PKCE login flow.
Issues are rated using the same severity scale as sections above.

| ID | Severity | Summary | Status |
| :--- | :--- | :--- | :---: |
| [OAUTH-H1](#OAUTH-H1) | High | No rate limiting on the AS login form endpoint | ✓ |
| [OAUTH-H2](#OAUTH-H2) | High | Revoked JWT tokens bypass the RS validation cache | ✓ |
| [OAUTH-H3](#OAUTH-H3) | High | PKCE not required for public clients in the AS authorization flow | ✓ |
| [OAUTH-H4](#OAUTH-H4) | High | CSRF token not regenerated when login form is re-rendered on failure | ✓ |
| [OAUTH-H5](#OAUTH-H5) | High | Unvalidated `redirect` query parameter enables open redirect in RS authorize handler | ✓ |
| [OAUTH-M1](#OAUTH-M1) | Medium | Audience validation skipped by default | ✓ |
| [OAUTH-M2](#OAUTH-M2) | Medium | No timeout on outbound IdP HTTP calls | ✓ |
| [OAUTH-M3](#OAUTH-M3) | Medium | Revocation endpoint ignores HTTP Basic Auth for client authentication | ✓ |
| [OAUTH-M4](#OAUTH-M4) | Medium | OIDC discovery and JWKS responses read without a size limit | ✓ |
| [OAUTH-M5](#OAUTH-M5) | Medium | RS PKCE state store has no maximum size | ✓ |
| [OAUTH-M6](#OAUTH-M6) | Medium | Token exchange response body read without size limit | ✓ |
| [OAUTH-M7](#OAUTH-M7) | Medium | Every JWT with an unknown `kid` triggers an unconditional JWKS refresh | ✓ |
| [OAUTH-M8](#OAUTH-M8) | Medium | Custom permission claim names silently unsupported; all JWT holders granted minimum "ego.logon" | ✓ |
| [OAUTH-L1](#OAUTH-L1) | Low | CSRF cookie missing `Secure` flag on AS authorize handler | ✓ |
| [OAUTH-L2](#OAUTH-L2) | Low | AS token endpoint router registration uses JSON content type | ✓ |
| [OAUTH-L3](#OAUTH-L3) | Low | `EGO_OAUTH_CLIENT_SECRET` environment variable not cleared after reading | ✓ |
| [OAUTH-L4](#OAUTH-L4) | Low | Internal error details from token exchange and JWT validation returned to browser clients | ✓ |
| [OAUTH-L5](#OAUTH-L5) | Low | Custom `ego.server.oauth.user.claim` values silently fall back to `sub` | ✓ |

### High

<a id="OAUTH-H1"></a>

#### OAUTH-H1 — No rate limiting on the AS login form endpoint

**Affected file:** `server/oauth/authserver/authorize.go:181` — `AuthorizePostHandler()`

**Description:**
`AuthorizePostHandler` calls `auth.ValidatePassword` directly without invoking
the rate-limiting infrastructure (`CheckRateLimit` / `RecordFailure` / `RecordSuccess`)
that `router/auth.go:Authenticate` uses for the native auth path. An attacker who
can reach the AS can submit an unlimited number of credential guesses through
`POST /oauth2/authorize` at full network speed — the per-account lockout added by
LOGIN-C2 simply does not apply here. Because the handler re-renders the form on
failure rather than returning 401, automated tools can drive the form POST loop
without any HTTP-level friction and without triggering the native auth lockout
counters.

The bcrypt validation that backs `auth.ValidatePassword` is inherently slow
(~100 ms per attempt), which provides a modest natural throttle, but that
alone is not adequate protection against distributed attacks.

**Recommendation:**
Call `CheckRateLimit(username)` before attempting `validatePassword` and return
a 429 response when the account is locked. Call `RecordSuccess(username)` and
`RecordFailure(sessionID, username)` after the validation result is known. This
is exactly the pattern in `router/auth.go:272–278` and can be extracted into a
shared helper so both paths stay in sync.

**Resolution (May 2026):**
`AuthorizePostHandler` now calls `router.CheckRateLimit(username)` before
attempting credential validation. A locked account causes the login form to
re-render with a lockout message (via the new `reRenderWithError` helper) rather
than proceeding to `validatePassword`. After a failed credential check,
`router.RecordFailure(session.ID, username)` is called so the counter advances.
After a successful credential check, `router.RecordSuccess(username)` clears the
counter. The per-account lockout budget is now shared between the native auth
path and the OAuth2 form login path.  New log message key
`oauth.as.authorize.locked` added to all three language files. Tests in
`server/oauth/authserver/authorize_test.go`.

---

<a id="OAUTH-H2"></a>

#### OAUTH-H2 — Revoked JWT tokens bypass the RS validation cache

**Affected file:** `server/oauth/oauth.go:204` — `ValidateJWT()`

```go
if v, found := caches.Find(caches.OAuthJWTCache, tokenStr); found {
    entry, ok := v.(*JWTCacheEntry)
    if ok && time.Now().Before(entry.Expires) {
        return entry.User, entry.Permissions, nil  // ← no blacklist check
    }
```

**Description:**
When a JWT is found in `caches.OAuthJWTCache`, it is returned as valid after a
single `time.Now().Before(entry.Expires)` check. The JTI blacklist populated by
`POST /oauth2/revoke` is never consulted on a cache hit. A token that has been
explicitly revoked can therefore continue to authenticate all RS requests for up
to the configured JWKS cache TTL (default 1 hour) after revocation.

By contrast, the AS's own `UserinfoHandler` does perform a blacklist check on
every request (`tokens.IsIDBlacklisted(claims.ID)`), so the inconsistency is
not caused by a missing API — the call is simply absent from the hot cache-hit
path in `ValidateJWT`.

This is analogous to the cached-token expiry bypass described in LOGIN-M3.

**Recommendation:**
Store the JTI (`claims.ID`) inside `JWTCacheEntry` and check
`tokens.IsIDBlacklisted(entry.JTI)` before returning from the cache-hit branch.
A blacklisted JTI should result in immediate cache eviction and a validation
error, identical to the behavior of a post-expiry entry.

**Resolution (May 2026):**
`JWTCacheEntry` gained a `JTI string` field that stores the JWT ID claim.
`ValidateJWT` now calls `tokens.IsIDBlacklisted(entry.JTI)` on every cache hit
before returning. A positive blacklist result evicts the cache entry immediately
and returns an error; the caller is denied. `ValidateJWT` also stores
`JTI: claims.ID` when writing new cache entries. New log message key
`oauth.rs.jwt.revoked` added to all three language files. Tests in
`server/oauth/oauth_cache_test.go`.

---

<a id="OAUTH-H3"></a>

#### OAUTH-H3 — PKCE not required for public clients in the AS authorization flow

**Affected file:** `server/oauth/authserver/codes.go:168` — `verifyPKCE()`

```go
func verifyPKCE(pending PendingAuthorization, codeVerifier string) error {
    if pending.CodeChallenge == "" {
        // No PKCE was used in this authorization request — nothing to verify.
        return nil
    }
```

**Description:**
PKCE (RFC 7636) is enforced only when the client chose to include a
`code_challenge` in the authorization request. Public clients — those registered
without a `client_secret_hash`, such as the built-in `ego-cli` — can omit PKCE
entirely. Without PKCE, an attacker who intercepts or steals the authorization
code (e.g., via a malicious redirect-URI registration at the OS level, or log
exposure) can exchange it for tokens at the token endpoint without possessing the
original code_verifier.

RFC 9700 (OAuth 2.0 Security Best Current Practice) §2.1.1 mandates that all
public clients use PKCE. PKCE is also strongly recommended for confidential
clients. Accepting a code from a public client without PKCE removes this
protection entirely.

**Recommendation:**
If the client is public (`ClientSecretHash == ""`), `handleAuthorizationCodeGrant`
must reject the exchange when `pending.CodeChallenge == ""`:

```go
if client.ClientSecretHash == "" && pending.CodeChallenge == "" {
    return util.ErrorResponse(w, session.ID,
        i18n.T("oauth.as.pkce.required"), http.StatusBadRequest)
}
```

For confidential clients, PKCE should be strongly recommended via documentation;
making it mandatory for all clients is also an option and is the safest default.

**Resolution (May 2026):**
`handleAuthorizationCodeGrant` in `server/oauth/authserver/token.go` now checks
`client.ClientSecretHash == "" && pending.CodeChallenge == ""` after validating
the redirect URI. When both conditions hold (public client, no PKCE in the
authorization request), the exchange is rejected with 400 and a log entry under
`oauth.as.pkce.missing`. The `verifyPKCE` function is unchanged — it still
validates PKCE when a challenge is present. Confidential clients are unaffected.
New error key `oauth.as.pkce.required` and log key `oauth.as.pkce.missing` added
to all three language files. Tests in `server/oauth/authserver/token_test.go`.

---

<a id="OAUTH-H4"></a>

#### OAUTH-H4 — CSRF token not regenerated when login form is re-rendered on failure

**Affected file:** `server/oauth/authserver/authorize.go:233` — `AuthorizePostHandler()`

```go
data := loginFormData{
    ClientID:            clientID,
    RedirectURI:         redirectURI,
    Scope:               scope,
    State:               state,
    CodeChallenge:       codeChallenge,
    CodeChallengeMethod: codeChallengeMethod,
    Error:               "Invalid username or password.",
    // CSRFToken is zero-value ("") — the field is not set
}
```

**Description:**
When credential validation fails, `AuthorizePostHandler` re-renders the login form
with an error message but does not generate a new CSRF token. The `CSRFToken`
field of `loginFormData` is left at its zero value, producing an empty
`<input type="hidden" name="csrf_token" value="">` in the re-rendered HTML.

The CSRF cookie set during the original GET still holds the original random
token. When the user corrects their credentials and submits the re-rendered form,
the POST handler compares the cookie value (non-empty) against the form value
(`""`) — they do not match — and returns 403 Forbidden. The user cannot retry
without navigating back and restarting the entire authorization flow from scratch,
which is not indicated anywhere in the error UI.

Two consequences follow:

1. **Correctness / usability:** Any password mistake permanently breaks the in-
   progress login session. The user must restart the full browser-based flow.
2. **Security:** Because the CSRF re-render path does not set a new CSRF cookie,
   an automated attacker who drives the GET → POST loop would also need to restart
   the GET after every failed attempt — providing a minor additional friction but
   not a meaningful security control.

**Recommendation:**
In the failure branch, generate a fresh CSRF token, set a new cookie, and include
the token in `loginFormData`:

```go
newCSRF, _ := generateCSRFToken()
http.SetCookie(w, &http.Cookie{
    Name: csrfCookieName, Value: newCSRF,
    Path: "/oauth2/authorize", HttpOnly: true, SameSite: http.SameSiteStrictMode,
})
data := loginFormData{ ..., CSRFToken: newCSRF, Error: "Invalid username or password." }
```

This ensures that each form render (whether first-load or after failure) pairs a
fresh nonce in the cookie with the same nonce embedded in the form.

**Resolution (May 2026):**
All re-render paths in `AuthorizePostHandler` are now routed through the new
`reRenderWithError` helper in `server/oauth/authserver/authorize.go`. The helper
generates a fresh CSRF token via `generateCSRFToken`, replaces the CSRF cookie in
the response, and populates `loginFormData.CSRFToken` with the new nonce. This
is called for both the rate-limit lockout path (OAUTH-H1) and the bad-credential
path, ensuring the form is always submittable after an error. Tests in
`server/oauth/authserver/authorize_test.go`.

---

<a id="OAUTH-H5"></a>

#### OAUTH-H5 — Unvalidated `redirect` query parameter enables open redirect in RS authorize handler

**Affected file:** `server/oauth/rshandlers/authorize_handler.go:29-31`

```go
if override := r.URL.Query().Get("redirect"); override != "" {
    cfg.RedirectURI = override
}
```

**Description:**
`AuthorizeRedirectHandler` accepts an optional `redirect` query parameter and
substitutes it for the configured `ego.server.oauth.redirect.uri` without any
server-side validation. The substituted URI is then sent to the IdP as the
`redirect_uri` parameter of the authorization request.

Two separate problems arise:

1. **Open redirect / phishing.** If the IdP's client registration uses a
   wildcard, prefix match, or pattern for allowed redirect URIs (common with
   some providers, or when an operator accidentally registers too broadly), an
   attacker can supply an arbitrary URI. After the user authenticates with the
   IdP, the browser is redirected to the attacker-controlled URI. Because PKCE
   protects the subsequent code exchange, the attacker cannot obtain tokens, but
   they receive the authorization code in the URL and can display a convincing
   fake "login successful" page. Even with an exact-match IdP registration, the
   capability to redirect to any registered URI (including a staging endpoint)
   is unintended and violates the principle that a server should enforce its own
   policy, not rely solely on the IdP.

2. **Broken feature — stored redirect URI is silently ignored.** The overridden
   `RedirectURI` is stored in the PKCE `pendingState` as `ps.RedirectURI`
   (`state.go:119`). However, `CallbackHandler` retrieves the global config with
   `oauth.GetConfig()` and calls `ExchangeCodePublic(cfg, code, ps.CodeVerifier)`,
   which sends `cfg.RedirectURI` (the original configured value) to the IdP token
   endpoint. The stored `ps.RedirectURI` is never read. As a result, the token
   exchange always fails with a redirect-URI mismatch error from the IdP, making
   the override feature entirely inoperative.

**Recommendation:**
Remove the `redirect` override entirely from `AuthorizeRedirectHandler`. If
per-request redirect URI overrides are a future requirement, validate the
supplied URI against a server-side allowlist of permitted redirect URIs before
accepting it, AND pass `ps.RedirectURI` to `ExchangeCode` instead of
`cfg.RedirectURI` so that the same URI is used consistently across both legs of
the flow.

**Resolution (June 2026):**
Five coordinated changes were made:

1. **`server/oauth/rshandlers/authorize_handler.go`** — The `redirect` query
   parameter override is removed entirely.  `AuthorizeRedirectHandler` now
   always uses `cfg.RedirectURI` (the server-configured value) and never reads
   the `redirect` query parameter.  The function-level doc comment was updated
   to explain why no per-request override is accepted.  The internal error
   message for `BuildAuthorizeURL` failure was also changed to a generic string
   (no longer leaks the raw error to the browser — partial fix for OAUTH-L4).

2. **`server/oauth/rshandlers/routes.go`** — The `.Parameter("redirect",
   "string")` chain call was removed from the `GET /services/admin/oauth/authorize`
   route registration, so the router no longer declares that parameter as
   expected input.

3. **`server/oauth/state.go`** — `RedirectURI string` was removed from
   `pendingState` (the internal struct) and `newState()` no longer accepts a
   `redirectURI` argument.  The field was dead storage: it was set by
   `AuthorizeURL` but never read by `CallbackHandler`.

4. **`server/oauth/oauth.go`** — `RedirectURI string` was removed from the
   exported `PendingState` struct.  `ValidateCallbackState` no longer copies
   the field.

5. **`server/oauth/flow_authcode.go`** — The `newState(cfg.RedirectURI)` call
   was updated to `newState()`.

Tests in `server/oauth/rshandlers/authorize_handler_test.go` cover:
`TestAuthorizeRedirectIgnoresRedirectParam` (primary regression — verifies that
`?redirect=<attacker-uri>` has no effect on the redirect_uri embedded in the
authorization URL), `TestAuthorizeRedirectUsesConfiguredURI` (happy path with
all required PKCE parameters present), `TestAuthorizeRedirectNoProvider` (503
when provider is not configured), `TestAuthorizeRedirectNoRedirectURI` (500
when redirect URI is not configured), and
`TestAuthorizeRedirectTwoCallsProduceDifferentStates` (each flow gets a unique
PKCE state token).

Existing tests in `server/oauth/state_test.go` and `medium_test.go` were
updated to match the new `newState()` signature.

---

### Medium

<a id="OAUTH-M1"></a>

#### OAUTH-M1 — Audience validation skipped by default

**Affected file:** `server/oauth/jwt.go:86` — `parseAndValidateJWT()`

```go
if audience != "" {
    parserOpts = append(parserOpts, jwt.WithAudience(audience))
}
```

**Description:**
When `ego.server.oauth.audience` is not set (the default), `cfg.Audience` is an
empty string and audience validation is entirely skipped. A JWT issued for any
other resource server by the same IdP — including a test or staging environment —
will be accepted as valid by the production Ego RS. Per RFC 9700 §2.8, "Resource
servers MUST validate the audience claim" because it is the principal mechanism
that prevents token confusion attacks across services sharing the same IdP.

**Recommendation:**
Document `ego.server.oauth.audience` as a **required** setting in any production
deployment. Add a startup warning (analogous to `WEBAUTH-M2`) when
`ego.server.oauth.provider` is set but `ego.server.oauth.audience` is empty:

```go
if cfg.Provider != "" && cfg.Audience == "" {
    ui.Log(ui.ServerLogger, "oauth.rs.no.audience", ui.A{})
}
```

Optionally, refuse to start in `resource-server` or `hybrid` mode unless the
audience is configured, treating it the same way as a missing issuer.

**Resolution (May 2026):**
`commands/server.go` now calls `oauth.GetConfig().Audience == ""` immediately
after a successful `oauth.Initialize()`.  When the audience is unconfigured,
`ui.Log(ui.ServerLogger, "oauth.rs.no.audience", ...)` emits a SERVER-level log
entry that is always visible in the server log and the dashboard Log tab.  The
new message key `oauth.rs.no.audience` (with a `{{provider}}` argument) was
added to all three language files.  The server starts normally — a hard refusal
would be a breaking change for existing deployments that have not yet set the
audience — but the warning makes the gap impossible to miss.

---

<a id="OAUTH-M2"></a>

#### OAUTH-M2 — No timeout on outbound IdP HTTP calls

**Affected files:**

- `server/oauth/discovery.go:79` — `discoverEndpoints()`: `http.Get(discoveryURL)`
- `server/oauth/jwks.go:89` — `refreshJWKS()`: `http.Get(jwksURL)`
- `server/oauth/flow_authcode.go:143` — `ExchangeCode()`: `http.DefaultClient.Do(req)`
- `app-cli/app/logon_oauth.go:201` — `fetchOIDCDiscovery()`: `http.Get(discoveryURL)`
- `app-cli/app/logon_oauth.go:374` — `postTokenRequest()`: `http.DefaultClient.Do(req)`

**Description:**
All outbound HTTP calls to the identity provider (discovery, JWKS fetch, and token
exchange) use either `http.Get` or `http.DefaultClient.Do`, neither of which sets a
deadline. If the IdP is slow or unresponsive, each goroutine handling an inbound
request blocks indefinitely on the outbound call. A network partition, an
overloaded IdP, or a deliberate slowdown by a malicious upstream server can
exhaust all available server goroutines — effectively causing a DoS of the Ego
server without any involvement of the attacker's client.

This is analogous to the Slowloris vulnerability described in HTTP-H2, but for
outbound connections rather than inbound ones.

**Recommendation:**
Replace `http.DefaultClient` with a client that carries a context deadline derived
from the inbound request, or use a package-level client with a bounded timeout:

```go
var idpClient = &http.Client{Timeout: 10 * time.Second}
```

Apply this to `discoverEndpoints`, `refreshJWKS`, `ExchangeCode`, and the CLI's
`postTokenRequest`. The discovery and JWKS fetches happen at startup and on cache
miss; 10–30 seconds is a reasonable wall-clock limit. The token exchange happens
per-request; 10 seconds matches common IdP SLAs.

**Resolution (May 2026):**
A new file `server/oauth/client.go` defines a package-level `idpClient =
&http.Client{Timeout: 10 * time.Second}`.  All three server-side outbound call
sites — `discoverEndpoints` (`discovery.go`), `refreshJWKS` (`jwks.go`), and
`ExchangeCode` (`flow_authcode.go`) — now use `idpClient.Get(...)` /
`idpClient.Do(...)` instead of `http.Get` / `http.DefaultClient.Do`.  The CLI
gains a parallel `oauthHTTPClient = &http.Client{Timeout: 30 * time.Second}` in
`app-cli/app/logon_oauth.go` (30 s is more generous for a user-facing flow).
Both `fetchOIDCDiscovery` and `postTokenRequest` in the CLI now use
`oauthHTTPClient`.  Tests in `server/oauth/medium_test.go` verify that
`idpClient.Timeout` is non-zero and that the client is actually used for
discovery requests.

---

<a id="OAUTH-M3"></a>

#### OAUTH-M3 — Revocation endpoint ignores HTTP Basic Auth for client authentication

**Affected file:** `server/oauth/authserver/revoke.go:31` — `RevokeHandler()`

```go
clientID := r.FormValue("client_id")
clientSecret := r.FormValue("client_secret")
```

**Description:**
The AS token endpoint (`handleAuthorizationCodeGrant`, `handleClientCredentialsGrant`,
`handleRefreshTokenGrant`) uses `validateBasicAuth(r)`, which prefers the HTTP
`Authorization: Basic` header and falls back to form-encoded `client_id` /
`client_secret` fields. This matches RFC 6749 §2.3.1.

The revocation endpoint (`POST /oauth2/revoke`) reads credentials only from form
values, completely ignoring any `Authorization: Basic` header. RFC 7009 §2.1
requires the revocation endpoint to use the same client authentication mechanism
as the token endpoint. Confidential clients that prefer Basic Auth cannot
authenticate at the revocation endpoint and will receive a 401 error even when
supplying valid credentials.

**Recommendation:**
Replace the two `r.FormValue` calls with a call to the existing `validateBasicAuth`
helper:

```go
clientID, clientSecret := validateBasicAuth(r)
```

This is a one-line fix that brings the revocation endpoint into alignment with
the token endpoint and RFC 7009.

**Resolution (May 2026):**
The two `r.FormValue("client_id")` / `r.FormValue("client_secret")` lines in
`RevokeHandler` (`server/oauth/authserver/revoke.go`) were replaced with a
single call to the existing `validateBasicAuth(r)` helper (defined in
`token.go`).  `validateBasicAuth` tries the `Authorization: Basic` header first
and falls back to form fields, so all existing clients that POST credentials
in the body continue to work without changes.  Tests in
`server/oauth/authserver/revoke_test.go` cover: Basic Auth accepted, form
credentials still accepted, wrong secret rejected, unknown client rejected, and
public-client Basic Auth with empty password accepted.

---

<a id="OAUTH-M4"></a>

#### OAUTH-M4 — OIDC discovery and JWKS responses read without a size limit

**Affected files:**

- `server/oauth/discovery.go:86` — `discoverEndpoints()`: `io.ReadAll(resp.Body)`
- `server/oauth/jwks.go:99` — `refreshJWKS()`: `io.ReadAll(resp.Body)`

**Description:**
Both functions read the IdP's HTTP response body into memory with a bare
`io.ReadAll`, applying no size limit before the allocation. A malicious or
compromised IdP, or a network attacker who can intercept the outbound TLS
connection (e.g., via a CA compromise), can return an arbitrarily large response
body. The full response is allocated into a single byte slice, so even a
moderately large payload (tens of megabytes) causes a visible memory spike;
gigabyte payloads can exhaust heap and crash the server.

Discovery documents and JWKS responses are inherently small — a few kilobytes at
most for any realistic deployment. A generous limit of 1 MiB is far more than
any legitimate document requires.

**Recommendation:**
Wrap the response body with `http.MaxBytesReader` (or `io.LimitReader`) before
reading:

```go
body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MiB cap
```

A response that exceeds the limit should be treated as an error and logged so the
operator can investigate.

**Resolution (May 2026):**
Both `discoverEndpoints` (`discovery.go`) and `refreshJWKS` (`jwks.go`) now wrap
`resp.Body` in `&io.LimitedReader{R: resp.Body, N: maxDiscoveryBytes + 1}` (where
`maxDiscoveryBytes = 1 << 20`, 1 MiB) before calling `io.ReadAll`.  Using N+1
rather than N avoids an off-by-one: when `lr.N` reaches zero after reading,
it proves the body is strictly larger than the limit rather than exactly equal to
it.  A zero `lr.N` after `io.ReadAll` returns causes the function to return a
descriptive error containing "exceeds" so the operator knows why the document was
rejected.  Tests in `server/oauth/medium_test.go` cover: a body one byte over the
limit is rejected, a body exactly at the limit passes the size check (then fails
JSON parsing, confirming no off-by-one regression).

---

<a id="OAUTH-M5"></a>

#### OAUTH-M5 — RS PKCE state store has no maximum size

**Affected file:** `server/oauth/state.go:75` — `newState()`

```go
stateStore.items[state] = &pendingState{ ... }
```

**Description:**
The `stateStore.items` map that holds pending PKCE states has no upper bound on
the number of entries. Every call to `GET /services/admin/oauth/authorize`
(or, transitively, to `AuthorizeRedirectHandler`) inserts a new entry. The
background goroutine in `Initialize()` purges entries older than 10 minutes every
10 minutes, but a flood of requests between purge cycles can fill the map beyond
what a single purge pass can evict. In the worst case an attacker can call the
authorize endpoint in a tight loop to exhaust memory, since each entry is created
without any per-IP or global cap.

This is analogous to the unbounded WebAuthn challenge cache issue described in
WEBAUTH-M1.

**Recommendation:**
Add a global cap on the number of pending state entries and enforce a per-IP rate
limit on calls to `AuthorizeRedirectHandler`. A cap of 500 concurrent pending
states is generous for normal usage:

```go
const maxPendingStates = 500

stateStore.mu.Lock()
if len(stateStore.items) >= maxPendingStates {
    stateStore.mu.Unlock()
    return "", "", fmt.Errorf("too many pending OAuth2 flows")
}
stateStore.items[state] = &pendingState{ ... }
stateStore.mu.Unlock()
```

Additionally, move the `purgeExpiredStates()` ticker to run more frequently
(e.g., every 2 minutes instead of 10) so the cap is only reached under genuine
sustained load.

**Resolution (May 2026):**
Three constants were added to `server/oauth/state.go`:
`statePurgeInterval = 2 * time.Minute` (the new ticker interval),
`maxPendingStates = 500` (the global cap), and the existing `stateMaxAge` is
unchanged (10 minutes).  `newState()` now acquires `stateStore.mu`, checks
`len(stateStore.items) >= maxPendingStates`, and returns an error before
inserting if the cap is reached.  The check and insert share a single lock
acquisition, eliminating the TOCTOU race between them.  In `oauth.go`'s
`Initialize()`, the background goroutine's ticker was changed from `stateMaxAge`
to `statePurgeInterval`, so expired entries are swept every 2 minutes rather
than every 10.  Tests in `server/oauth/medium_test.go` verify: injection to
exactly the cap causes the next `newState` to fail, the below-cap positive path
works, and the atomicity invariant holds (cap-1 → cap-1 insert succeeds, cap →
error returned).

---

<a id="OAUTH-M6"></a>

#### OAUTH-M6 — Token exchange response body read without size limit

**Affected file:** `server/oauth/flow_authcode.go:155` — `ExchangeCode()`

```go
body, err := io.ReadAll(resp.Body)
```

**Description:**
`ExchangeCode` reads the IdP token endpoint response with a bare `io.ReadAll`,
applying no byte limit before the allocation. OAUTH-M4 fixed the same class of
issue for the OIDC discovery document and JWKS responses, but the token exchange
response was not updated at the same time.

A malicious or compromised IdP token endpoint (or a network attacker who can
intercept the server-to-server TLS connection) can return an arbitrarily large
response body. The `idpClient` timeout (10 s, added by OAUTH-M2) bounds the
total wall-clock time, but a slowly-streaming attacker can deliver megabytes
within that window. The full body is allocated as a single byte slice, so a
large payload causes a memory spike that can be repeated on every user login that
triggers a token exchange (i.e., every Authorization Code flow completion).

**Recommendation:**
Apply the same `io.LimitedReader` pattern used in `discoverEndpoints` and
`refreshJWKS`:

```go
const maxTokenBodyBytes = 64 << 10  // 64 KiB — generous for any real token response
lr := &io.LimitedReader{R: resp.Body, N: maxTokenBodyBytes + 1}
body, err := io.ReadAll(lr)
if err != nil {
    return "", "", errors.New(errors.ErrOAuthTokenRead).Context(err.Error())
}
if lr.N == 0 {
    return "", "", errors.New(errors.ErrOAuthTokenSizeLimit).Context(doc.TokenEndpoint)
}
```

A real token endpoint response is a small JSON object (access token, expiry,
scopes) — 64 KiB is many times larger than any legitimate response.

**Resolution (June 2026):**
`ExchangeCode` in `server/oauth/flow_authcode.go` now wraps `resp.Body` in
`&io.LimitedReader{R: resp.Body, N: maxTokenBodyBytes + 1}` (where
`maxTokenBodyBytes = 64 << 10`, 64 KiB) before calling `io.ReadAll`.  Using
N+1 rather than N avoids an off-by-one: when `lr.N` reaches zero after reading,
it proves the body is strictly larger than the limit rather than exactly equal
to it.  A zero `lr.N` after `io.ReadAll` returns causes `ExchangeCode` to
return the new `ErrOAuthTokenSizeLimit` error so operators can identify why a
token exchange was rejected.  New error constant `ErrOAuthTokenSizeLimit` added
to `errors/messages.go` with key `oauth.token.size`; the localized message was
added to all three language files (`messages_en.txt`, `messages_fr.txt`,
`messages_es.txt`) in the same alphabetical position as the other
`oauth.token.*` entries.  Tests `TestExchangeCode_OversizedBody` and
`TestExchangeCode_ExactlyAtLimit` added to `server/oauth/medium_test.go`;
they follow the same pattern as the OAUTH-M4 boundary tests.

---

<a id="OAUTH-M7"></a>

#### OAUTH-M7 — Every JWT with an unknown `kid` triggers an unconditional JWKS refresh

**Affected file:** `server/oauth/jwks.go:200-209` — `keyByID()`

```go
// Try the cache first if it is fresh.
if len(keys) > 0 && age < ttl {
    if key := findKeyByID(keys, kid); key != nil {
        return key, nil
    }
}

// Cache miss or stale — refresh and try again.
if err := refreshJWKS(jwksURL); err != nil { ...
```

**Description:**
`keyByID` refreshes the JWKS unconditionally whenever the requested `kid` is not
found in the local cache, including when the cache is still fresh (`age < ttl`).
The intent is to handle key rotation gracefully: if the IdP rotates its signing
key, the first request bearing the new kid triggers a fetch. However, the guard
condition only prevents a refresh when the same kid is already in the cache; it
does not prevent repeated refreshes when a novel unknown kid is presented on every
request.

An attacker who can present Bearer tokens (even syntactically valid ones that
will ultimately fail signature verification) with a unique, non-existent `kid`
header on each request forces one JWKS network fetch per request. With many
concurrent such requests:

- Each `refreshJWKS` call makes a round-trip to the IdP that can last up to
  10 seconds (the `idpClient` timeout), blocking the handling goroutine.
- Many concurrent fetches starve the server's goroutine pool.
- The IdP is hammered with repeated JWKS requests, potentially triggering IdP-
  side rate limiting that blocks legitimate key lookups from the same server IP.

There is no throttle, cooldown, or minimum inter-refresh interval guarding the
cache-fresh-but-kid-unknown branch.

**Recommendation:**
Track the timestamp of the most recent JWKS refresh triggered by a cache miss and
refuse to refresh more than once per configurable minimum interval (e.g., 30
seconds) in the cache-is-fresh-but-kid-missing branch. A package-level
`lastMissRefresh time.Time` protected by a mutex is sufficient:

```go
var missRefresh struct {
    mu   sync.Mutex
    last time.Time
}
const minMissRefreshInterval = 30 * time.Second

// Inside keyByID, after the cache-fresh-kid-missing case:
missRefresh.mu.Lock()
if time.Since(missRefresh.last) < minMissRefreshInterval {
    missRefresh.mu.Unlock()
    return nil, errors.New(errors.ErrJWKSKeyNotFound).Context(kid)
}
missRefresh.last = time.Now()
missRefresh.mu.Unlock()
// then call refreshJWKS
```

The key-rotation use case is fully preserved: a genuine rotation will succeed
on the first unknown-kid request; subsequent requests within the 30-second window
will see the refreshed cache and find the new key (or correctly fail if the kid
is genuinely absent).

**Resolution (June 2026):**
`server/oauth/jwks.go` received three coordinated changes:

1. **`missRefresh` state variable** — a package-level struct with a `sync.Mutex`
   and a `last time.Time` field tracks when the most recent miss-triggered JWKS
   refresh occurred.

2. **`minMissRefreshInterval` constant** — set to 30 seconds.  A miss-triggered
   refresh is allowed at most once per this window.

3. **`keyByID` cooldown logic** — a `freshCacheMiss` boolean is set to `true`
   when the cache is within its TTL but the requested kid is absent.  When
   `freshCacheMiss` is true, the function acquires `missRefresh.mu` and checks
   whether `time.Since(missRefresh.last) < minMissRefreshInterval`.  If the
   cooldown is still active, it returns `ErrJWKSKeyNotFound` immediately without
   a network call.  If the cooldown has expired, it records `missRefresh.last =
   time.Now()` and proceeds to `refreshJWKS`.  Stale or empty caches are always
   refreshed without consulting the cooldown (the `freshCacheMiss` guard ensures
   the cooldown only applies to the fresh-cache miss path).

A `resetMissRefresh()` test helper was added to `jwks.go` alongside the existing
`resetJWKSCache()`.  Five tests were added to `server/oauth/medium_test.go`:
`TestKeyByID_FirstMissTriggersFetch` (first unknown-kid miss fires one refresh),
`TestKeyByID_CooldownBlocksSecondMiss` (second miss within cooldown fires zero
refreshes and returns `ErrJWKSKeyNotFound`),
`TestKeyByID_CooldownExpiryAllowsRefresh` (after cooldown window, refresh fires
again — time-warp via direct mutation of `missRefresh.last`),
`TestKeyByID_StaleCacheBypassesCooldown` (stale cache always refreshes regardless
of the cooldown), and `TestKeyByID_KnownKidInFreshCacheHitsNoNetwork` (fast path
regression — a known kid in a fresh cache still returns immediately with no fetch).

---

<a id="OAUTH-M8"></a>

#### OAUTH-M8 — Custom permission claim names silently unsupported; all JWT holders granted minimum "ego.logon"

**Affected file:** `server/oauth/claims.go:96-114` — `extractPermissionTokens()`

```go
default:
    // Custom claim names return an empty slice.
    return []string{}
```

**Description:**
`extractPermissionTokens` handles only two claim names: `"scope"` (standard
OAuth2, space-delimited) and `"roles"` (common IdP extension, string array). Any
other value for `ego.server.oauth.permission.claim` — including `"groups"`,
`"authorities"`, `"realm_access.roles"` (Keycloak), or provider-specific names —
silently returns an empty slice.

The empty slice propagates to `mapClaimsToPermissions`, which finds no matching
tokens and applies an unconditional fallback:

```go
if len(permissions) == 0 {
    permissions = []string{"ego.logon"}
}
```

As a result, any operator who configures a custom permission claim name finds:

1. **Users who should be blocked still get logon.** If the intended custom claim
   would have produced no Ego permissions for certain JWT holders (external
   accounts, low-privilege IdP users), they silently receive `ego.logon` and can
   authenticate to the Ego server.
2. **Elevated permissions are silently dropped.** Users whose custom claim would
   have mapped to `ego.root` or table-write permissions only receive logon; admin
   operations fail without a clear error.
3. **No warning or error is generated.** The misconfiguration is invisible in
   logs until an operator notices that elevated operations fail for users who
   should have admin access.

**Recommendation:**
At startup, when `ego.server.oauth.permission.claim` is set to a value other than
`"scope"` or `"roles"`, emit a SERVER-level warning (analogous to OAUTH-M1's
audience warning):

```go
if cfg.PermissionClaim != "scope" && cfg.PermissionClaim != "roles" {
    ui.Log(ui.ServerLogger, "oauth.rs.unsupported.permission.claim",
        ui.A{"claim": cfg.PermissionClaim})
}
```

Longer-term, support custom claim lookup by removing the `json:"-"` tag from
`jwtClaims.AdditionalClaims`, or by switching the claims struct to embed
`jwt.MapClaims` for the non-registered fields.

**Resolution (June 2026):**
Three coordinated changes implement the startup warning:

1. **`IsKnownPermissionClaim(claim string) bool`** — new exported function added
   to `server/oauth/claims.go`.  Returns `true` only for `"scope"` and `"roles"`,
   the two names that `extractPermissionTokens` handles natively.  Any other value
   returns `false`.  The function is exported so `commands/server.go` can call it
   without duplicating the constant set.

2. **Startup warning in `commands/server.go`** — immediately after the existing
   OAUTH-M1 audience check (inside the `oauth.Initialize()` success branch), the
   code now calls `oauth.IsKnownPermissionClaim(oauthCfg.PermissionClaim)`.  When
   it returns `false`, `ui.Log(ui.ServerLogger, "oauth.rs.unsupported.permission.claim", ...)`
   emits a SERVER-level log entry that is always visible in the server log and the
   dashboard Log tab.  The server starts normally — a hard refusal would break
   existing deployments that discovered the misconfiguration after the fact.
   The `GetConfig()` call was refactored to use a single `oauthCfg` local variable
   shared between the M1 and M8 checks.

3. **Log message key `oauth.rs.unsupported.permission.claim`** — added to all
   three language files (`messages_en.txt`, `messages_fr.txt`, `messages_es.txt`)
   in the alphabetical position after `oauth.rs.no.audience`, carrying a `{{claim}}`
   substitution argument so the operator can see exactly which claim name is
   misconfigured.

The `extractPermissionTokens` function and its default branch are unchanged;
this fix is purely observational (warn early, fail gracefully at runtime).
Tests `TestIsKnownPermissionClaim` (11 cases covering the two supported names,
six unsupported names, and four case-variant near-misses) and
`TestIsKnownPermissionClaim_FallbackBehavior` (end-to-end demonstration that an
unsupported claim silently degrades to `ego.logon`) added to
`server/oauth/claims_test.go`.

---

### Low / Informational

<a id="OAUTH-L1"></a>

#### OAUTH-L1 — CSRF cookie missing `Secure` flag on AS authorize handler

**Affected file:** `server/oauth/authserver/authorize.go:147` — `AuthorizeGetHandler()`

```go
http.SetCookie(w, &http.Cookie{
    Name:     csrfCookieName,
    Value:    csrfToken,
    Path:     "/oauth2/authorize",
    HttpOnly: true,
    SameSite: http.SameSiteStrictMode,
    // Secure is not set
})
```

**Description:**
The CSRF cookie used to protect the AS login form is `HttpOnly` and
`SameSite: Strict`, but the `Secure` attribute is not set. In any configuration
where the Ego AS accepts plain HTTP connections (before HTTPS redirect, or in
development), the CSRF nonce can be transmitted in cleartext. A network observer
can capture the cookie and the form nonce, enabling CSRF attacks against users on
unencrypted connections.

This is the same issue as WEBAUTH-L1, which was already resolved for the WebAuthn
challenge cookie. The `isSecureRequest(r)` helper added there can be reused here.

**Recommendation:**
Set the `Secure` attribute conditionally using `isSecureRequest(r)`:

```go
http.SetCookie(w, &http.Cookie{
    Name:     csrfCookieName,
    Value:    csrfToken,
    Path:     "/oauth2/authorize",
    HttpOnly: true,
    SameSite: http.SameSiteStrictMode,
    Secure:   isSecureRequest(r),
})
```

Apply the same fix to the re-render path in `AuthorizePostHandler` once
OAUTH-H4 is resolved and a new CSRF token is generated there as well.

**Resolution (May 2026):**
`isSecureRequest` in `router/webauthn.go` was renamed to `IsSecureRequest`
(exported) so that the `authserver` package can call it without duplicating the
logic.  All four internal call sites in `router/webauthn.go` and all three
references in `router/webauthn_test.go` were updated to use the new name.
Both cookie-setting locations in `authorize.go` — `AuthorizeGetHandler` and
`reRenderWithError` — now pass `Secure: router.IsSecureRequest(r)`.  The flag
is `true` when `r.TLS != nil` (direct TLS) or when the `X-Forwarded-Proto:
https` header is set (proxy-terminated TLS); it is `false` for plain HTTP so
development servers are not broken.  Tests in
`server/oauth/authserver/low_test.go` and
`server/oauth/authserver/secure_request_test.go` cover: HTTPS sets Secure,
plain HTTP omits Secure, X-Forwarded-Proto sets Secure, and
`router.IsSecureRequest` is callable from outside the router package.

---

<a id="OAUTH-L2"></a>

#### OAUTH-L2 — AS token endpoint router registration uses JSON content type

**Affected file:** `server/oauth/authserver/authserver.go:109` — `RegisterRoutes()`

```go
r.New(defs.OAuthTokenPath, TokenHandler, http.MethodPost).
    Class(router.ServiceRequestCounter).
    AcceptMedia(defs.JSONMediaType)
```

**Description:**
The token endpoint (`POST /oauth2/token`) is registered in the router with
`AcceptMedia(defs.JSONMediaType)` (i.e., `application/json`). However, RFC 6749
§4.1.3 and §4.4.2 require clients to send token requests as
`application/x-www-form-urlencoded`. The handler already uses `r.ParseForm()` to
read the request body, which is consistent with form encoding, not JSON.

A strictly compliant OAuth2 client library that sets
`Content-Type: application/x-www-form-urlencoded` (as required) may be rejected
by the Ego router before `TokenHandler` is ever called, if the router enforces
the declared `AcceptMedia` type. The `Authorization Code` flow from the CLI is
unaffected in practice because the CLI's `postTokenRequest` sets
`Content-Type: application/x-www-form-urlencoded` and the router may be lenient
in practice, but third-party clients that rely on strict content-type enforcement
(e.g., `application/json`) on the wrong side of the mismatch will fail.

**Recommendation:**
Remove the `AcceptMedia(defs.JSONMediaType)` chain call from the token endpoint
registration, or replace it with the correct media type:

```go
r.New(defs.OAuthTokenPath, TokenHandler, http.MethodPost).
    Class(router.ServiceRequestCounter)
    // No AcceptMedia constraint — RFC 6749 requires form-encoded bodies
```

**Resolution (May 2026):**
The `.AcceptMedia(defs.JSONMediaType)` chain call was removed from the
`POST /oauth2/token` route registration in `RegisterRoutes`
(`server/oauth/authserver/authserver.go`).  The router now imposes no
restriction on the Accept header for this endpoint, matching the RFC 6749
requirement that clients send `application/x-www-form-urlencoded` request
bodies.  The handler (`TokenHandler`) uses `r.ParseForm()` internally and
always writes JSON responses, regardless of what the client declares in its
Accept header.  Tests in `server/oauth/authserver/low_test.go` verify that
the handler processes form-encoded requests without an Accept header and that
the error path (unsupported grant type) produces a grant-type error rather
than a media-type rejection.

---

<a id="OAUTH-L3"></a>

#### OAUTH-L3 — `EGO_OAUTH_CLIENT_SECRET` environment variable not cleared after reading

**Affected file:** `server/oauth/config.go:130` — `loadConfig()`

```go
if envSecret := os.Getenv("EGO_OAUTH_CLIENT_SECRET"); envSecret != "" {
    clientSecret = envSecret
}
```

**Description:**
The OAuth2 client secret can be supplied via the `EGO_OAUTH_CLIENT_SECRET`
environment variable. Unlike `EGO_PASSWORD`, which was fixed by LOGIN-L1 to call
`os.Unsetenv` and emit a visible warning immediately after reading, the OAuth
client secret is read and stored but the environment variable is never cleared.
The secret therefore remains accessible to any child process spawned after server
startup (for example, via Ego's built-in exec functions if `ExecPermittedSetting`
is enabled) and is visible in `/proc/<pid>/environ` on Linux for the lifetime of
the server process.

**Recommendation:**
Clear the environment variable and emit a visible warning immediately after
reading, matching the pattern from LOGIN-L1:

```go
if envSecret := os.Getenv("EGO_OAUTH_CLIENT_SECRET"); envSecret != "" {
    clientSecret = envSecret
    os.Unsetenv("EGO_OAUTH_CLIENT_SECRET")
    ui.Log(ui.ServerLogger, "oauth.rs.client.secret.env", ui.A{})
}
```

**Resolution (June 2026):**
`loadConfig()` in `server/oauth/config.go` was updated to match the LOGIN-L1
pattern:

- The `ui` package import was added to `config.go`.
- After copying `envSecret` into `clientSecret`, the code now calls
  `_ = os.Unsetenv("EGO_OAUTH_CLIENT_SECRET")` to clear the variable from the
  process environment, and then calls
  `ui.Log(ui.ServerLogger, "oauth.rs.client.secret.env", ui.A{})` to emit a
  SERVER-level log entry that is always visible in the server log and the
  dashboard Log tab.
- New log message key `oauth.rs.client.secret.env` added to all three language
  files in alphabetical position before `oauth.rs.discovery.ok`.

Two tests added to `server/oauth/config_test.go`:
`TestLoadConfig_EnvVarSecretCleared` sets the env var to a known value, calls
`loadConfig()`, and confirms (a) the returned config carries the value and (b)
`os.Getenv` returns `""` afterward.  `TestLoadConfig_NoEnvVar` verifies that
the absent-env-var path does not panic.

---

<a id="OAUTH-L4"></a>

#### OAUTH-L4 — Internal error details from token exchange and JWT validation returned to browser clients

**Affected files:**

- `server/oauth/rshandlers/callback.go:55-65` — IdP error branch
- `server/oauth/rshandlers/callback.go:97-103` — token exchange failure
- `server/oauth/rshandlers/callback.go:109-111` — JWT validation failure

```go
return util.ErrorResponse(w, session.ID,
    "token exchange failed: "+err.Error(), http.StatusBadGateway)

return util.ErrorResponse(w, session.ID,
    "received JWT is invalid: "+err.Error(), http.StatusBadGateway)
```

**Description:**
Three error responses in `CallbackHandler` include verbatim Ego error strings in
the HTTP response body returned to the browser:

1. **Token exchange failure** — `err.Error()` from `ExchangeCode` can contain
   the IdP token endpoint URL and the IdP's own error response.
2. **JWT validation failure** — `err.Error()` from `ValidateJWT` can reveal
   which specific validation step failed (signature, expiry, issuer, audience,
   missing claim), aiding an attacker who is probing token validation behavior.
3. **IdP error** — the raw `error` and `error_description` query parameters
   from the IdP redirect are concatenated verbatim into the response body
   (`"IdP authorization error: " + idpError + ": " + desc`). An attacker who
   can craft a redirect to the callback endpoint with arbitrary query parameters
   can inject arbitrary text into the response body — and also into the server
   log (`"desc": desc`), which may corrupt structured log output if the value
   contains newline characters or JSON control sequences.

**Recommendation:**
Return a fixed, generic message to the browser for all three failure paths and
keep full error detail in the AUTH log only:

```go
ui.Log(ui.AuthLogger, "oauth.rs.callback.exchange.failed", ui.A{
    "session": session.ID, "error": err.Error(),
})
return util.ErrorResponse(w, session.ID,
    "OAuth2 login failed", http.StatusBadGateway)
```

For the IdP error branch, sanitize `error_description` (replace newlines and
non-printable characters) before writing it to the log.

**Resolution (June 2026):**
Four changes were made to `server/oauth/rshandlers/callback.go`:

1. **`sanitizeLogValue(s string) string`** — new unexported helper added before
   `CallbackHandler`.  It iterates over the runes in `s` and replaces any
   character where `unicode.IsControl(r)` is true with a space, then trims
   leading and trailing spaces with `strings.TrimSpace`.  `unicode.IsControl`
   covers the full Unicode Cc category (U+0000–U+001F and U+007F–U+009F),
   which includes all ASCII control codes including `\n`, `\r`, `\t`, and NUL.
   Non-ASCII printable Unicode (accented letters, CJK, emoji) is passed through
   unchanged.  The `strings` and `unicode` packages were added to the import
   block.

2. **IdP error branch** — both `idpError` and `desc` are now passed through
   `sanitizeLogValue` before being written to the AUTH log under
   `oauth.rs.callback.idp.error`.  The browser response was changed from
   `"IdP authorization error: "+idpError+": "+desc` to the fixed string
   `"OAuth2 login failed"`.

3. **Token exchange failure** — the browser response was changed from
   `"token exchange failed: "+err.Error()` to `"OAuth2 login failed"`.  The
   existing AUTH log entry (under `oauth.rs.callback.exchange.failed`) is
   unchanged and still carries the full error detail.

4. **JWT validation failure** — the browser response was changed from
   `"received JWT is invalid: "+err.Error()` to `"OAuth2 login failed"`.
   `ValidateJWT` already logs the failure internally under `oauth.rs.jwt.invalid`,
   so no additional log entry was needed here.

Tests added:

- `callback_test.go` (external `package rshandlers_test`) — three tests verify
  that the response body is the fixed generic message and never contains the raw
  IdP error codes, server addresses, or injected newline sequences.
- `sanitize_test.go` (internal `package rshandlers`) — `TestSanitizeLogValue`
  with 13 sub-cases directly exercises `sanitizeLogValue`, covering plain ASCII,
  newline, CR, CRLF, tab, NUL, DEL, leading/trailing whitespace trimming, non-ASCII
  Unicode passthrough, and the all-control-chars edge case.

---

<a id="OAUTH-L5"></a>

#### OAUTH-L5 — Custom `ego.server.oauth.user.claim` values silently fall back to `sub`

**Affected file:** `server/oauth/oauth.go:329-344` — `extractUsername()`

```go
func extractUsername(claims *jwtClaims, userClaim string) string {
    switch userClaim {
    case "sub":   return claims.Subject
    case "email": ...
    case "preferred_username": ...
    }
    return claims.Subject   // ← silent fallback for any other configured value
}
```

**Description:**
`extractUsername` handles only three well-known claim names (`sub`, `email`,
`preferred_username`). If an operator sets `ego.server.oauth.user.claim` to any
other value — for example `"upn"` (Azure AD), `"login"` (GitHub),
`"unique_name"` (ADFS), or a provider-specific custom claim — the function
silently returns `claims.Subject` with no warning or error.

Depending on the IdP, `sub` is often an opaque UUID rather than a human-readable
account name. Consequences include:

- Ego usernames appear as UUIDs in audit logs, making security reviews difficult.
- Username-based access policies apply to UUIDs rather than the account names
  the operator intended to control.
- The misconfiguration is invisible until an operator compares login usernames
  against expected values.

This is the user-identity analogue of OAUTH-M8 (custom permission claim silently
ignored).

**Recommendation:**
Log a startup warning when `ego.server.oauth.user.claim` is set to a value that
`extractUsername` does not handle. Longer-term, support custom claim lookup by
populating `AdditionalClaims` from the JWT body (removing its `json:"-"` tag) so
arbitrary claim names can be read at runtime.

**Resolution (June 2026):**
Three coordinated changes implement the startup warning, mirroring the OAUTH-M8
pattern for permission claims:

1. **`IsKnownUserClaim(claim string) bool`** — new exported function added to
   `server/oauth/oauth.go` immediately before `extractUsername`.  Returns `true`
   only for `"sub"`, `"email"`, and `"preferred_username"`.  The doc comment
   explains the three claims, the UUID-fallback consequence, and the OAUTH-L5
   reference.  `extractUsername` was updated with a doc comment referencing this
   function, and the silent-fallback `default` branch was annotated to explain
   that a startup warning is emitted when it would be reached.

2. **Startup warning in `commands/server.go`** — added immediately after the
   OAUTH-M8 permission-claim check (both inside the `oauth.Initialize()` success
   branch, sharing the `oauthCfg` local variable).  Calls
   `oauth.IsKnownUserClaim(oauthCfg.UserClaim)` and emits
   `ui.Log(ui.ServerLogger, "oauth.rs.unsupported.user.claim", ...)` when it
   returns `false`.  A detailed comment explains both failure modes (UUID
   usernames in audit logs; username-based policies that never match).

3. **Log message key `oauth.rs.unsupported.user.claim`** — added to all three
   language files in alphabetical position after
   `oauth.rs.unsupported.permission.claim`.  The French translation was kept
   under 100 characters by omitting the surrounding quotes from the claim-name
   examples.

Tests added to `server/oauth/claims_test.go`:
`TestIsKnownUserClaim` (11 sub-cases covering the three supported names, six
unsupported IdP-specific names, empty string, and three case-variant near-misses)
and `TestIsKnownUserClaim_FallbackBehavior` (end-to-end: a token carrying a
human-readable `email` and `preferred_username` still produces a UUID username
when `"login"`, `"upn"`, `"nickname"`, or `"custom_claim"` is the configured
claim name, documenting why the warning matters).
