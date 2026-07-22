# Unit tests

The "tests" directory contains Ego language unit tests. These tests are
at a higher level than the internal Go unit tests in the various packages
used by Ego. The directory contains subdirectories containing sets of
tests, each of which is run when the `ego test` command is executed on
the command line.

The "tests" directory is traversed recursively in alphabetical order to
find the test programs to execute. The directory tree is structured such
tht any Ego tests for a given runtime package are in `tests/{package-name}`.
For example, the `fmt` tests are in `tests/fmt/`. Additionally there are
directories basic language and runtime functionality:

- builtins: language builtin functions like `make` or `len`
- cast: test of data type cast functions like `int()`
- compiler: test o specific compiler features
- datamodel: tests of the basic Go data model
- defer: tests of the `defer` statement
- directives: tests of the various directives like `@capture`
- flow: tests of flow-of-control statements, like `if` or `switch`
- functions: tests of language `func` features
- packages: tests of user-written `package` files
- server: basic tests of client/server functionality
- types: basic test of language support for data types

A test has a basic structure:

```go

     @test "name of test" // Note, the label string must be <= 48 ASCII chars in length

     // Declare any global or common values here. These must be unique across
     // all tests.

     {
         // Put each sub-task of the text in it's own braces so it has local
         // scope.

         // Use @fail to indicate that if code got to this line, it was an 
         // error. This is most often used to test flow-of-control operations
         // such as loops.
         
         @fail "incorrect switch case selected"

         // You can test for a specific value or state using @assert, which
         // requires that the expression be true. If the expression is false,
         // the text of the expression itself is used as the error message.

         @assert count == 5

     }
```

Each test should be written so it can run alone, or be part of a test suite. Because it
could run along with other tests, any variable declared outside the scope of each sub-test
must be a unique name across all tests. This also applies to `type` declarations made in
that same outer position — but a `type` declared *inside* a test's own `{}` block (or a
nested sub-block) is private to that block and can reuse the same name freely across
different tests. See "Type scoping" under "Runtime / execution differences" below.

## The test command

Tests can be run as standalone programs, using the `ego run` command. Tests generally are intended to be run as a suite of tests, and can
validate language functionality and ensure that a change does not break any language features unexpectedly. As such, often a suite of tests is run from a given directory path, using all the `.ego` programs in the path that contains @TEST directives.

```sh
   ego test [file-or-path]
```

Use the `ego test` command to run all the tests named. The single parameter can either be a
single test program, or a directory containing one or more test programs. The directory
is scanned recursively, so you can use sub-folders to group tests. The scan is always done
in alphabetical order so you can use test names and/or subdirectory names to control the
order of execution.

There are a number of options to the `ego test` command that can be helpful in managing
the test environment in which the code runs.

| Option | Usage |
| ------ | ----- |
| `--count` | How many times to repeat the test suite (default is once) |
| `--optimimize` | Determine if the optimizer is run over tests cases or not |
| `--sandbox` | Determines if sandbox restrictions are in effect when running tests |
| `--types` | Specify type rules (stirct, relaxed, or dynamic) during test runs |

Additionally, a number  global Ego options placed on the command line after the
word `ego` before any verb are useful for controlling execution of tests:

| Option | Usage |
| ------ | ----- |
| `--maxcpus` | Specify maximum number of cpus to consume during use of go routines |
| `--set` | Override one or more configuration settings for this execution only |
| `--timeout` | Specify a duration used to abort a test that is in an endless loop or taking too long |

See the `ego --help` command for more information on these options and the values they can accept
that help influence how a test program is run.

## Directives

Below is additional information about each of the individual test directives.

### @test

All unit tests must start with the `@test` directive, which is followed by a string
expression (usually a quoted string constant) that labels the test. The label cannot
be longer than 48 ASCII characters in length or an error is generated. This directive
creates an instance of a built-in `Testing` struct type and stores it in a variable
named `T`, which provides the `assert`/`Fail`/`Nil`/`NotNil`/`True`/`False`/`Equal`/
`NotEqual` methods that `@assert` and the other test-related directives compile into
calls on. If you use the other directives without first specifying `@test` they will
fail.  

A single file can contain more than one `@test`; each test gets it's own name and is
reported to the console as a new test execution.

Everything from a `@test` directive up to the next `@test` (or the end of the file) is
that test's own body, and is compiled and guarded independently of every other test in
the file. If that body fails to compile, or raises an uncaught runtime error (most
commonly a failed `@assert`), the test is reported as failed -- a `(FAIL)` status line
in the same format as a passing test's `(PASS)` line, immediately followed by the
underlying error -- and `ego test` moves on to compile and run the rest of the file's
tests normally. Neither kind of failure aborts the whole file the way it used to; a
single broken or failing test no longer hides whether any of the others in the same
file are passing. (The one deliberate exception is `@fail`, described below, which is
still an unconditional, unrecoverable stop by design.) A test that reaches its own end
without error is automatically reported as passed -- there is no directive needed to
mark it so; see `@pass`'s removal, noted at the end of this section.

### @assert

The `@assert` accepts a single expression that must be resolvable to a boolean value:

```go
@assert count == 5
```

If the expression resolves to `true`, no error is flagged and execution continues.
If the expression resolves to `false`, the CURRENT test stops executing, is reported
as failed with the source text of the expression itself as the error message, and
`ego test` continues on to the next test in the file (see `@test` above).

### @capture

`@assert` can only check values a test already has in hand — it can't check what a
piece of code *prints*. `@capture` fills that gap by redirecting everything a block
of statements would otherwise print (`fmt.Println`, `fmt.Printf`, `fmt.Print`, or the
`print` language extension) into a string variable, so it can be checked directly with
`@assert` instead of only being eyeballed on the console:

```go
@capture output := {
    fmt.Println("Hello")
    fmt.Printf("%d\n", 53)
}
@assert output == "Hello\n53\n"
```

Use `:=` to declare a brand-new variable, or `=` to assign into one already declared —
the same distinction as an ordinary assignment. As a bonus, wrapping otherwise-noisy
`fmt.Println` calls in `@capture` also keeps them from cluttering the console while
`ego test` runs, since the printed text ends up in the variable instead of in the
test's own output block.

If a test only cares about a call's return value and has no further use for what it
printed — for example, a `fmt.Printf` byte-count/error test like the ones in
`tests/io/printf.ego` — use `_` in place of a variable name to keep the console quiet
without bothering to name (or assert against) the captured text:

```go
@capture _ := {
    n, err := fmt.Printf("hello %d\n", 42)
    @assert n == 9
}
```

`_` works with either `:=` or `=`, and — unlike a real variable — can be reused as many
times as needed in the same scope, since Ego never actually declares anything named `_`
anywhere in the language. Discarding on success doesn't affect error handling: if the
block raises an error, the partial output is still printed to the console with the usual
heading, exactly as it would be for a named variable.

If something inside the block raises an error, `@capture` still turns capturing off
cleanly, saves whatever was printed before the error into the variable (so a partial
result is never silently lost), prints that partial text to the console with a
`"@capture <var>:"` heading so it isn't missed during test development, and then lets
the error keep propagating outward exactly as if `@capture` weren't there — an
enclosing `try`/`catch` can still catch it normally. See the `@capture` entry in
`docs/LANGUAGE.md`'s Directives section for the full details, including a known
limitation around `@fail` and unrecovered `panic()`.

### @sandbox

`@sandbox true|false [path=expr]` sets the running context's sandboxed-IO mode,
letting a test exercise sandboxed code paths — for example, the path-traversal
containment enforced by the `os`, `io`, and `filepath` packages' sandbox
checks — without needing a real server- or dashboard-triggered sandboxed
session. This is the same restricted mode a request to the dashboard's
sandboxed "run code" handler runs under.

```go
@test "os: sandboxed reads cannot escape the sandbox root"
{
    marker := "inside.txt"
    @assert os.WriteFile(marker, "safe", 0o644) == nil

    @sandbox true path="."

    content, err := os.ReadFile(marker)
    @assert err == nil
    @assert string(content) == "safe"

    _, escapeErr := os.ReadFile("../outside.txt")
    @assert escapeErr != nil     // clamped to the sandbox root, not the real file

    @sandbox false path=""
    @assert os.Remove(marker) == nil
}
```

The optional `path=` clause sets the sandbox root and accepts any expression —
not just a string literal — so a test can compute the root (from a
temp-directory helper, a variable, etc.) instead of hardcoding one. Both the
flag and the path take effect at **runtime**, at the exact point `@sandbox`
appears in the compiled instruction stream — not at compile time — so
statements earlier in the same test are never retroactively affected by a
`path=` that appears later on. `path=""` is a real, given value (it clears the
sandbox root) and is distinct from omitting the clause entirely (which leaves
whatever root was previously set untouched).

The path is written to the same ephemeral, non-persisted settings overlay
`@optimizer` uses, so a test never needs to save and restore the real
`ego.runtime.sandbox.path` profile setting.

Because the sandbox root is a global (not context-scoped) setting, remember to
clear it with `path=""` when you turn sandboxing back off — otherwise
subsequent native-passthrough calls (`os.Remove`, `filepath.*`, etc.) stay
sandboxed even after `@sandbox false`, since those check for a configured root
independently of the context's own sandboxed-IO flag. See
`tests/os/sandbox_escape.ego` for a complete, self-cleaning example.

`@sandbox` may only appear when the compiler is running in test mode — the
same restriction as `@test`/`@assert`. Ordinary compiled Ego source,
including untrusted code submitted to the dashboard's sandboxed "run code"
handler, has no way to reach the underlying `Sandbox` opcode, so it cannot use
this directive to lift its own sandbox restrictions or repoint the sandbox
root.

### @error

The `@error` is used to generate a runtime error; the text of the error message is
the string expression following the directive. Note that this is not a fatal error,
so it can be used in a `try...catch...` construct and the `catch` block will be
executed.

```go
    @error "signal this as an error"
```

### @fail

The `@fail` is used to unilaterally fail the test. This is a fatal error; i.e. the
remainder of the test does not execute and an error is reported. Unlike a failed
`@assert`, `@fail` is deliberately unconditional and unrecoverable: it cannot be caught
by a `try...catch...` block, and -- unique among the ways a test can fail -- it aborts
the ENTIRE remaining test run, not just the current test (`@test`'s own per-test
compile/runtime guard, described above, cannot catch it either; `@fail` flushes that
guard's `try`/`catch` state before signaling, specifically so it can't be). Reach for
`@fail` only when reaching a given line is itself the bug and there is no sense in
letting anything else run afterward. (If you need to generate a non-fatal error message
that can be caught, use `@error`.) There is no conditional expression. This is a
simpler version of using `@assert fail ...`. This is most commonly used in tests that
are validating flow-of-control, such as:

```go
    count := 5
    if count == 5 {
        // no action, all good
    } else {
        @fail "incorrect branch in if-statement"
    }
```

### @pass (removed)

`@pass` used to be required as the last statement of a test to record a passing result,
and `ego test` used to silently append an implicit trailing `@pass` to every file so the
last test in it (which has no following `@test` to trigger a close) still got reported.
Both were a legacy of an earlier, more primitive implementation. Every test now closes
and reports itself -- PASS or FAIL -- automatically as soon as its own body finishes
(see `@test` above), so `@pass` is never required and the directive has been removed
entirely. Existing test files that still called it explicitly have had those calls
deleted; there is no replacement directive to reach for.

---

## How `ego test` differs from `ego run`

Understanding the differences between the two execution environments matters when
writing tests that use `panic()`, `recover()`, goroutines, or multi-value returns.

### Compilation differences

| Feature | `ego run` | `ego test` |
| :------ | :-------- | :--------- |
| Entry point | Requires a `func main()` | No `main()` required; `@test` blocks run at the top level |
| Type-checking mode | Default (`dynamic`) unless overridden | Same defaults; `ego test --types strict` activates strict mode |
| Extensions | Off by default; controlled by `ego.compiler.extensions` | **Always on**, unconditionally (see below) |
| Auto-import | Off by default; controlled by `ego.compiler.import` | **Always on**, unconditionally (see below) |

**Extensions are always on in `ego test`.** `TestAction` (`internal/commands/test.go`)
unconditionally sets `ego.compiler.extensions=true` before compiling any test file,
regardless of the profile setting or any command-line flag — there is no way to make
`ego test` run with extensions off. A file can still narrow this back down for part of
itself with an explicit `@extensions false` directive if a test specifically needs to
verify extensions-off behavior.

**Auto-import is always on in `ego test`, regardless of the `ego.compiler.import`
setting.** `TestAction` unconditionally calls `comp.AutoImport(true, ...)` before
compiling — it does not consult `ego.compiler.import` at all (that setting only affects
`ego run`, where it defaults to off and can be turned on with `--import`). This means
every test file gets roughly twenty packages (`errors`, `fmt`, `math`, `os`, `strings`,
`time`, and others — see the `all` branch of `AutoImport` in
`internal/language/compiler/compiler.go` for the full list) with no `import` statement
needed at all. This does **not** extend to library source under `lib/packages/*.ego`,
which is compiled through the normal import path and must still `import` whatever
packages it uses.

### Runtime / execution differences

**Stack depth.** Under `ego run`, `main()` is called via a full function-call
frame, so the execution stack already has one call frame on it before any user
code runs. Under `ego test`, the `@test` body runs at the **top level** — there
is no enclosing function frame. This means:

- The initial frame pointer (`c.framePointer`) is `0` at the start of a `@test`
  body, whereas it is greater than `0` inside `main()`.
- A function called directly from a `@test` body has a shallower stack. Code
  that relies on a minimum stack depth (for example, `StackCheck` for multi-value
  returns) must account for this.
- `panic()` / `recover()` behavior is affected: `unwindPanic` checks
  `c.framePointer == 0` to detect the top-level case. Because functions called
  from `@test` bodies use `callFramePush` (setting `fp > 0`), recovery works
  correctly, but the stack layout after recovery is shallower than in the
  `ego run` + `main()` case.

**Output capture.** This is driven per-test, not once for the whole file. Each
`@test "..."` directive compiles to code that turns capture on (the `Console`
opcode, backed by `ctx.EnableConsoleOutput(false)`) and starts the elapsed-time
clock, but prints nothing yet. Whatever the test body itself prints accumulates
in that same buffer as it runs. Only once the test's outcome is known -- it
reached its own end without error, or it failed to compile or raised an
uncaught runtime error -- does the compiler-generated code print the complete
"TEST: <description> ... (PASS)" or "TEST: <description> ... (FAIL)" line and
flush the whole buffer to the console in one shot (the `Say` opcode), before
resetting output back to the real console. So everything a single test
prints — whatever the test body itself printed, followed by its one summary
line — is gathered into one buffer and shown as one contiguous block, with
the summary line always coming last and never interrupted partway through by
the test's own output. A failed test's underlying error is printed as a
separate, immediately-following line once console output is back to normal.
Code that writes directly to `os.Stderr` or uses `ui.Log` is not affected. See
also the `@capture` directive above, which uses a related but independent
mechanism to redirect a specific block's output into a variable instead of
the console.

**Error handling.** In `ego run`, an error returned by the bytecode run loop
terminates the program with an error message. In `ego test`, each `@test`
guards its own body in a compile-time and runtime try/catch (see `@test`
above): a compile error or an uncaught runtime error is caught, reported
against that specific test's own `(FAIL)` line, and execution moves on to the
next test. Nothing in a normal test file terminates the whole run early
anymore except `@fail`, which is deliberately excluded from that guard (see
`@fail` above).

**Symbol tables.** The `T` variable (the testing object) isn't pre-populated by
the Go-level test runner itself — it's created at runtime by the compiled
`@test` directive, which stores a fresh `Testing` struct instance into `T` each
time it runs (see `testDirective` in `internal/language/compiler/testing.go`).
In practice this means `T` is reserved and cannot usefully be redefined by test
code, since the next `@test` (or the implicit one before the first) overwrites it.

**Global scope.** Functions and variables declared outside any `@test` block
are compiled into the same top-level bytecode as the `@test` blocks. They share
a single symbol table for the whole file — there is no isolation between tests
unless values are declared inside `{}` blocks.

**Type scoping.** A `type X ...` declaration follows the same scoping rule as a
`var`: it is private to the `{}` block it's declared in, and disappears once
that block ends — including the outermost `{}` that forms a `@test`'s own body.
Since `ego test` compiles an entire file (every `@test` block in it) as a
single compilation unit, this is what lets two unrelated `@test` blocks each
declare their own `type box struct { ... }` — with completely different
fields — without a "duplicate type name" error, and without having to invent
artificial per-test names (`box1`, `box2`, `senderBox`, ...) just to keep them
from colliding. The same applies to nested sub-blocks within a single test:
each one gets its own type namespace, layered on top of its enclosing block's.

This scoping only kicks in for a `{}` block. A `type` declared in the "common
values" position between a `@test "name"` directive and its opening `{` (the
same outer position `shared := 0` occupies in the closures example under
[`tests/functions/closures.ego`](../tests/functions/closures.ego)) is not
inside any block, so it behaves like a top-level `var` or `func`: it is
visible for the rest of the file and must have a name that doesn't collide
with any other top-level declaration in the same file.

### Practical implications for test authors

1. **Multi-value returns from `panic()` + `recover()`** work correctly in both
   environments. In `ego test`, a function's stack marker
   (`NewStackMarker`) is essential for `StackCheck` to pass when the starting
   stack is shallow; the runtime synthesizes this marker correctly on recovery.

2. **Anonymous goroutine closures** (`go func() { ... }()`) must receive all
   outer-scope variables as explicit arguments in both environments.

3. **Named functions defined inside `@test` blocks** are compiled as nested
   named functions with the same scope restrictions as named functions elsewhere:
   they cannot access the enclosing `@test` body's local variables. Use a closure
   literal to capture outer scope.

4. **`try/catch` wrapping of multi-return calls** works in both environments
   but changes the bytecode structure slightly. A test that fails without `try`
   may appear to pass when wrapped — always prefer the direct form to avoid
   masking real failures.
