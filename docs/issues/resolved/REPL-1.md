# REPL-1 — Line numbers reported for interactive console statements are wrong, and a shebang line shifts every line number in a script

**Severity:** MEDIUM

**Discovered by:** an audit of `internal/commands/run.go` requested by the user. Confirmed present on `master` before that work began. Recorded here rather than fixed at the time, at the user's request, because line-number tracking was to be investigated as a whole -- across the compiler and bytecode as well as the REPL -- rather than patched in one place.

**Status:** FIXED

## Description

Several related defects were found while auditing the REPL. All of them are
about the line number that appears in a diagnostic. None of them affects
whether a program produces the right answer; all of them make it harder to find
the statement that went wrong.

### 1. The REPL's line counter never advances

Every statement typed at the interactive prompt was reported as being on line 1.
Typing three bad statements in a row:

```text
ego> badone()
Error: at line 1:8, unknown symbol: badone
ego> badtwo()
Error: at line 1:8, unknown symbol: badtwo
ego> badthree()
Error: at line 1:8, unknown symbol: badthree
```

All three report line 1. The second statement is the second line of the
session, and the third is the third.

### 2. After a multi-line block, the reported line number goes backwards

Entering a block that spans several lines and then making a mistake on the next
statement:

```text
ego> if true {
...>    x := 1
...> }
Error: at line 3:1, variable created but never used: x
ego> undefined_two()
Error: at line 1:8, unknown symbol: undefined_two
```

The statement typed immediately after the block was reported as line 1.

### 3. A shebang line shifts every line number in a script

`loadFile` strips the interpreter line from a script that begins with `#!` by
deleting it outright, so every remaining line moved up by one and every
diagnostic was reported one line earlier than the line the user would count in
their editor.

### 4. A plain script was off by one anyway

Noted at the time as "a separate, pre-existing off-by-one that is not specific
to the REPL and not specific to shebang handling", and the main reason this
issue was left open for a wider investigation:

```text
$ ego run plain.ego            # undefined_thing() is on line 7
Error: at line 5:17, unknown symbol: undefined_thing
```

### 5. A runtime error in a script was reported twice

Also found during the investigation. A program run from a file, a project, or a
pipe printed the same message twice, word for word:

```text
$ ego run divzero.ego
Error: at main(line 7), division by zero
Error: at main(line 7), division by zero
```

### 6. Every file of a project after the first was numbered wrong

Found during the investigation, not in the original audit. `ego run --project`
joins every source file in a directory into one piece of text, and the numbers
it reported for anything but the first file ran on from the end of the file
before it:

```text
$ ego run --project proj       # bogus_thing() is on line 5 of b.ego
Error: at b.ego(line 15:1), unknown symbol: bogus_thing
```

## Cause

The six symptoms above come from five independent faults. They compound, which
is why the REPL's numbers looked as if the counter simply did not work: the
counter did work, and was verified to inject `@line 1`, `@line 2`, `@line 3`
correctly, but nothing downstream honored it.

**a. `compileError` named the token before the offending one.**
`internal/language/compiler/errors.go` located the error with `c.t.Peek(0)`.
The tokenizer numbers its look-ahead from one, so `Peek(1)` is the token about
to be read and `Peek(0)` is the one *behind* it. Nearly every caller reports a
problem with a token it is looking at but has not yet consumed, so the location
was consistently one token early. Within a line that is invisible; when the
offending token is the first on its line -- which is true of most statements --
the error is attributed to the end of the previous line. This alone accounts
for symptom 4, and for the odd-looking column in symptoms 1 and 2 (`1:8` is the
`;` that ends the injected `@line 1;` directive).

**b. `Clone` did not carry `lineNumberOffset`.**
`internal/language/compiler/compiler.go` copies a long list of fields into a
cloned compiler, and the `@line` offset was not among them, so a clone always
started at zero. Every expression is compiled by a clone, and an undefined name
is found while compiling an expression, so an `@line` directive had no effect
at all on the errors that mattered most. This is what made every console
statement report line 1 regardless of the directive.

**c. The `@line` offset assumed the directive was the first line of the text.**
`lineDirective` in `internal/language/compiler/directives.go` computed
`c.lineNumberOffset = line - 2`, which is correct only when the directive
itself sits on physical line 1. That holds for a single file and for the
console, but not for a project, where an `@line 1` is inserted ahead of each
joined file. This is symptom 6.

**d. `inputUntilBlocksBalance` kept its extra lines to itself.**
`internal/commands/run.go`'s continuation-line reader accumulated the extra
lines into its own local `text` and advanced its own local `lineNumber`, and
returned neither -- only the tokenizer. Its sibling `inputUntilQuotesBalance`,
doing the same job for an unterminated raw string, returned all three. The
caller therefore never learned that three lines had been consumed instead of
one. This is the "goes backwards" half of symptom 2.

**e. `getExitStatusFromError` printed the error and handed it back.**
It wrote the message to stderr itself, and its caller in `compileAndRun`
returned the same error, which `RunAction` passes up to `main.go`'s
`reportError` -- which prints it again. Nothing noticed, because the interactive
console does need the message printed there: it goes straight back round for
the next statement, so there is nobody else left to report it. A program run
from a file has somebody, and got both. This is symptom 5.

Separately, the shebang line was deleted rather than blanked, which is symptom 3.

## Fix

1. `compileError` locates the error with `Peek(1)`, falling back to `Peek(0)`
   at the very end of the input, where there is no next token and `Peek(1)`
   answers with the empty end-of-tokens marker at 0:0.
2. `Clone` copies `lineNumberOffset` into the clone, and `Close` hands any
   change back to the parent. The two compilers share one tokenizer, so the
   parent resumes exactly where the clone stopped reading; the offset that
   turns those physical line numbers into reported ones has to travel with it.
3. `lineDirective` computes `line - (directiveLine + 1)` using the directive's
   own position, so a directive anywhere in the text means what it says: the
   line after it is the line it names.
4. `inputUntilBlocksBalance` returns the text it gathered and the line number it
   reached, matching `inputUntilQuotesBalance`, and the caller keeps both.
5. `removeShebang` empties the interpreter line rather than deleting it, so the
   byte count changes but the line count does not.
6. `getExitStatusFromError` no longer prints anything; deciding what a finished
   program's error means is separate from reporting it. `compileAndRun` makes
   the reporting decision, because it is the only place that knows whether the
   error is also being handed back to someone who will report it: the console
   prints and reports success, and everything else hands the error back for
   `main.go` to print once.

## Verification

All of these now report the line the user would count:

```text
ego> badone()
Error: at line 1:1, unknown symbol: badone
ego> badtwo()
Error: at line 2:1, unknown symbol: badtwo
ego> badthree()
Error: at line 3:1, unknown symbol: badthree

ego> if true {
...>    x := 1
...> }
Error: at line 2:3, variable created but never used: x
ego> undefined_two()
Error: at line 4:1, unknown symbol: undefined_two

ego> a := 10
ego> b := 0
ego> fmt.Println(a/b)
Error: at console(line 3), division by zero
```

```text
$ ego run plain.ego            # undefined_thing() is on line 7
Error: at line 7:1, unknown symbol: undefined_thing

$ ego run with-shebang.ego     # the same program, one line lower
Error: at line 8:1, unknown symbol: undefined_thing

$ ego run --project proj       # bogus_thing() is on line 5 of b.ego
Error: at b.ego(line 5:1), unknown symbol: bogus_thing

$ ego run divzero.ego          # reported once, not twice
Error: at main(line 7), division by zero
```

The console still prints its own errors and carries on, and the exit status of
a failed script is unchanged at 1.

Note that the unused-variable error inside the block is now reported at line 2,
where `x` is declared, rather than at line 3 where the block closes and the
scope is swept. The original write-up called line 3 "right"; on reflection line
2 is the better answer and matches what Go reports for the same mistake.

## Tests

`internal/language/compiler/lines_test.go` pins down the reported line for a
plain script, for the `@line`-directive shape the console uses, for a directive
part-way down the text as a project produces, and for an error raised from
inside an expression (which is what a clone compiles). Each of the four fails
against the previous code.

`TestRemoveShebang` in `internal/commands/run_source_test.go` checks that the
interpreter line is blanked, and that the line count of the text never changes.

`TestGetExitStatusFromError` in `internal/commands/run_exit_test.go` checks the
statuses, and that working them out prints nothing -- which is the part that
fails against the previous code.

## Related

The audit that found this also found a number of unrelated defects in
`internal/commands/run.go` -- `ego run .` executing nothing, piped input being
silently truncated, and Ctrl-D never exiting the REPL among them. Those were
fixed separately.

One thing seen during this work was deliberately left alone, because it is a
distinct fault with its own risks rather than part of this one: `os.Exit(3)`
exits with a status of 0. `compileAndRun` drops the `ErrExit` error on the way
out, so the status the program asked for never reaches `main.go`'s
`reportError`, which is the code that knows how to read it. Simply handing the
error back would fix `os.Exit(3)` and break a plain `exit`, which carries no
status in its context and would then exit 1 instead of 0. Worth a separate look.
