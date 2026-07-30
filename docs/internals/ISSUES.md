# Ego Issues — Consolidated Bug, Design, and Security Tracking

## Introduction

This document describes the contents of the docs/issues directory. This
contains a markdown file for each issue found over a series of code
quality audits done in the spring and summer of 2026.

Each file describes a single issue, identified by a unique code, such as
BUG-01, or OAUTH-L5. The code sometimes includes an encoding of the
issue severity (the "L" in OAUTH-L5, for example) but this is not a
consistent pattern.

Previously, this information was all in this single file (12k+ lines)
but this because uttery unweidly so the issues were broken into individual
files, and this document only serves as an explaination.

Each audit covered a distinct focus area of the codebase — general
language behavior, functional/behavioral differences from Go, the
`builtins` package, the bytecode instruction set, the interactive
debugger, and a security review of the server and CLI.

**Auditing context**, preserved from the original tasks:

- **General Language Bugs** (originally `BUGS.md`): Tracks general Ego-language bugs discovered through systematic testing, distinct from the documented behavioral differences tracked elsewhere. BUG-16 cross-references FLOW-M4 (defer lazy argument evaluation) and is included here only for completeness, not as a duplicate open issue.
- **Functional / Behavioral Issues** (originally `FUNCTIONAL_ISSUES.md`): Records known behavioral differences between Ego and Go, plus Ego-specific limitations, uncovered during testing of functions, flow control, the JSON package, the type system, and the @transaction scripting endpoint.
- **Builtin Function Issues** (originally `BUILTIN_ISSUES.md`): Documents behavioral anomalies, potential bugs, and design concerns found during a comprehensive review of the builtins Go package. All issues discovered in the initial audit have been resolved.
- **Bytecode Instruction Issues** (originally `BYTECODE_ISSUES.md`): Documents behavioral anomalies, potential bugs, and design concerns found during a comprehensive bytecode-instruction unit-test effort covering branch, call, math, optimizer, range, store, struct-indexing, and other opcode-execution paths.
- **Debugger Package Issues** (originally `DEBUGGER_ISSUES.md`): Documents behavioral anomalies, potential bugs, and design concerns found during a comprehensive review of the debugger package, which intercepts the ErrSignalDebugger sentinel from the bytecode.Context run loop to offer an interactive prompt.
- **Security Issues** (originally `SECURITY_ISSUES.md`): Records known security weaknesses in Ego found via security code reviews (April-June 2026) across authentication, WebAuthn, the HTTP server, the tables and asset endpoints, profile encryption, dashboard code execution, and the OAuth2 Authorization/Resource Server. Each issue documents affected files, a description, a recommendation, and (where resolved) the resolution actually implemented.
- **Goroutine Lifecycle Issues** (`GORTNS-1` … `GORTNS-4`): Documents goroutines left running with no way to terminate, or launched redundantly. GORTNS-1 is the significant one: every bytecode execution leaked a goroutine and the Context it captured, because signal.Stop does not close the channel its watcher blocked on — one leak per service request, one per comparison in an Ego sort comparator. The rest add stop channels, a cache eviction callback, and a launch guard.
- **Nil Pointer Issues** (`NILPTR-1` … `NILPTR-8`): Documents nil-pointer dereferences and related crash paths in the HTTP server, prioritized by whether a panic could make the server unavailable rather than merely fail one request. NILPTR-1 and NILPTR-6 add the panic-recovery machinery (`ego.server.panic.recovery`, `util.SafeCall`); the rest are individual guards, several of them cases where a nil test was present but the code dereferenced the value anyway on the next line. All were resolved.
- **Index Guard Issues** (`INDEX-1` … `INDEX-17`): Documents index expressions whose index was not trustworthy — bytecode operands, saved stack and frame pointers, token positions from a failed parse, command-line arguments, and values reflected out of caller-supplied objects. The audit covered both missing guards and defective ones: an off-by-one bound, a range whose ends were never compared, and checks validating the index against a different slice than the one indexed. All were resolved.
