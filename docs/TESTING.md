# Unit tests

The "tests" directory contains Ego language unit tests. These tests are at a higher
level than the internal Go unit tests in the various packages used by Ego. The
directory contains a set of tests, each of which is run when the `ego test` command
is executed on the command line.

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
must be a unique name across all tests.

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

## Directives

Below is additional information about each of the individual test directives.

### @test

All unit tests must start with the `@test` directive, which is followed by a string
expression (usually a quoted string constant) that labels the test. The label cannot
be longer than 48 ASCII characters in length or an error is generated. This directive
also creates an additional package in the running program called "testing" which contains
functions needed by the other test directives. If you use the other directives without
first specifying `@test` they will fail.  

A single file can contain more than one `@test`; each test gets it's own name and is
reported to the console as a new test execution.

### @assert

The `@assert` accepts a single expression that must be resolvable to a boolean value:

```go
@assert count == 5
```

If the expression resolves to `true`, no error is flagged and execution continues.
If the expression resolves to `false`, the test stops executing and the source text of
the expression itself is printed as the error message on the console.

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
remainder of the test does not execute and an error is reported. The `@fail`
directive cannot be caught in a `try...catch...` block. (If you need to generate
a non-fatal error message that can be caught, use `@error`) There is no conditional expression.
This is a simpler version of using `@assert fail ...`. This is most commonly used in
tests that are validating flow-of-control, such as:

```go
    count := 5
    if count == 5 {
        // no action, all good
    } else {
        @fail "incorrect branch in if-statement"
    }
```

### @pass

This should be the last directive or statement in the test. If the test has not incurred
any errors at this point, a PASS message is added to the output console.

---

## How `ego test` differs from `ego run`

Understanding the differences between the two execution environments matters when
writing tests that use `panic()`, `recover()`, goroutines, or multi-value returns.

### Compilation differences

| Feature | `ego run` | `ego test` |
| :------ | :-------- | :--------- |
| Entry point | Requires a `func main()` | No `main()` required; `@test` blocks run at the top level |
| Type-checking mode | Default (`dynamic`) unless overridden | Same defaults; `ego test --types=strict` activates strict mode |
| Extensions | Off by default | Off by default; `@extensions true` enables them |
| Auto-import | Controlled by `ego.compiler.auto-import` | Same |

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

**Output capture.** The test runner calls `ctx.EnableConsoleOutput(false)`,
which redirects `fmt.Print*` output from `os.Stdout` to an internal capture
buffer that is replayed to the console at test completion. Code that writes
directly to `os.Stderr` or uses `ui.Log` is not affected.

**Error handling.** Errors returned by the bytecode run loop cause both
environments to terminate with an error message. In `ego test` the error is
prefixed with the test name so it is clear which test failed.

**Symbol tables.** The test runner pre-populates the symbol table with the `T`
struct (the testing object) and the built-in test directives. These names are
reserved in test mode and cannot be redefined by test code.

**Global scope.** Functions and variables declared outside any `@test` block
are compiled into the same top-level bytecode as the `@test` blocks. They share
a single symbol table for the whole file — there is no isolation between tests
unless values are declared inside `{}` blocks.

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
