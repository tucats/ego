# REPL-1 — Line numbers reported for interactive console statements are wrong, and a shebang line shifts every line number in a script

**Severity:** MEDIUM

**Discovered by:** an audit of `internal/commands/run.go` requested by the user. Confirmed present on `master` before that work began. Recorded here rather than fixed, at the user's request, because line-number tracking is to be investigated as a whole -- across the compiler and bytecode as well as the REPL -- rather than patched in one place.

**Status: OPEN**

## Description

Two related defects were found while auditing the REPL. Both are about the line
number that appears in a diagnostic. Neither affects whether a program produces
the right answer; both make it harder to find the statement that went wrong.

### 1. The REPL's line counter never advances

Every statement typed at the interactive prompt is reported as being on line 1.
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

`runLoop` in `internal/commands/run.go` maintains a `lineNumber` variable for
exactly this purpose. It starts at 1, and before each fragment is compiled it
injects an `@line` directive and then advances the counter by the number of
lines the fragment contained:

```go
if interactive && !debug {
    text = fmt.Sprintf("@line %d;\n%s", lineNumber, text)

    sourceLineCount := strings.Count(text, "\n") - 1
    lineNumber += sourceLineCount
}
```

The counter is computed, but the number that reaches the user does not follow
it. Whether the fault is in the arithmetic here, in how `@line` is honored by
the compiler, or in how a line number is attached to a runtime error, was not
determined -- which is the reason this is being recorded rather than fixed.

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

The error inside the block is reported at line 3, which is right. The statement
typed immediately afterwards is reported at line 1.

There is at least one clear contributing defect here, and it is worth fixing
whatever else is found. `inputUntilBlocksBalance` prompts for continuation
lines until the braces balance:

```go
func inputUntilBlocksBalance(interactive bool, t *tokenizer.Tokenizer, text string, lineNumber int) *tokenizer.Tokenizer {
    ...
    text = text + io.ReadConsoleText("...> ")
    t = tokenizer.New(text, true)
    lineNumber++
    ...
    return t
}
```

It accumulates the extra lines into its own local copy of `text` and advances
its own local copy of `lineNumber`, and then returns neither. Only the
tokenizer comes back. The caller therefore never learns that three lines were
consumed instead of one, and its own `lineNumber` and `text` are left
describing only the first line of the block.

Its sibling, `inputUntilQuotesBalance`, does the same job for an unterminated
raw string and *does* return all three values:

```go
func inputUntilQuotesBalance(...) (*tokenizer.Tokenizer, string, int)
```

The asymmetry looks like an oversight rather than a decision.

### 3. A shebang line shifts every line number in a script

`loadSource` strips the interpreter line from a script that begins with `#!`:

```go
if strings.HasPrefix(text, "#!") {
    if i := strings.Index(text, "\n"); i > 0 {
        text = text[i+1:]
    }
}
```

The line is removed outright, so every remaining line moves up by one and every
diagnostic is reported one line earlier than the line the user would count in
their editor. Two copies of the same program, one with a shebang and one
without, report the same error at different line numbers:

```text
$ ego run with-shebang.ego         # undefined_thing() is on line 9
Error: at line 7:19, unknown symbol: undefined_thing

$ ego run without-shebang.ego      # undefined_thing() is on line 8
Error: at line 7:19, unknown symbol: undefined_thing
```

The usual fix for this is to replace the shebang line with an empty line rather
than delete it, so the byte count changes but the line count does not, or to
emit an `@line 2` directive in its place.

Note that the second example above is itself off by one: the statement is on
line 8 and is reported as line 7. That suggests a separate, pre-existing
off-by-one that is not specific to the REPL and not specific to shebang
handling, and is the main reason this issue is being left open for a wider
investigation.

## Suggested scope for a fix

1. Establish what `@line` is defined to mean -- whether it sets the number of
   the directive's own line or of the line after it -- and check the compiler
   and bytecode line-attachment code against that definition. The plain-file
   off-by-one above suggests the answer is not what the REPL assumes.
2. Make `inputUntilBlocksBalance` return the extended text and the updated line
   number, matching `inputUntilQuotesBalance`.
3. Replace the shebang line rather than deleting it.
4. Add tests that pin down the reported line number for: successive REPL
   statements, a statement following a multi-line block, a plain script, and a
   script with a shebang.

## Related

The audit that found this also found a number of unrelated defects in
`internal/commands/run.go` -- `ego run .` executing nothing, piped input being
silently truncated, and Ctrl-D never exiting the REPL among them. Those were
fixed separately; only the line-number findings are recorded here.
