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
