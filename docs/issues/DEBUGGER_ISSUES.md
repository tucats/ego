# Debugger Package Issues

This file documents behavioral anomalies, potential bugs, and design concerns
found during a comprehensive review of the `debugger/` package.  The debugger
interacts closely with the `bytecode.Context` run loop — it intercepts the
`ErrSignalDebugger` sentinel, offers the user an interactive prompt, and then
resumes execution.

Each entry includes the affected file(s), a description of the original
behavior, the risk level, and the resolution.

---

## Table of Contents

- [DEBUGGER-BREAKS-1 — Hit tracking lost in evaluationBreakpoint range loop](#debugger-breaks-1)
- [DEBUGGER-BREAKS-2 — Magic number instead of named constant in load case](#debugger-breaks-2)
- [DEBUGGER-BREAKS-3 — clearBreak functions continue iterating after first deletion](#debugger-breaks-3)
- [DEBUGGER-COMMANDS-1 — errors.Equal (full string) used instead of errors.Equals (key-only)](#debugger-commands-1)
- [DEBUGGER-SHOW-1 — showSource silently swallows invalid-range parse errors](#debugger-show-1)
- [DEBUGGER-SHOW-2 — Source indentation counts braces inside string literals](#debugger-show-2)

---

<a name="debugger-breaks-1"></a>

## DEBUGGER-BREAKS-1 — Hit tracking lost in evaluationBreakpoint range loop

**File:** `debugger/breaks.go`  
**Function:** `evaluationBreakpoint`  
**Risk:** High — conditional breakpoints (`break when <expr>`) fire on every
matching step instead of once per "becoming true" edge  
**Status: RESOLVED**

### DEBUGGER-BREAKS-1: Original behavior

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

### DEBUGGER-BREAKS-1: Fix

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

---

<a name="debugger-breaks-2"></a>

## DEBUGGER-BREAKS-2 — Magic number instead of named constant in load case

**File:** `debugger/breaks.go`  
**Function:** `breakCommand` (the `"load"` sub-case)  
**Risk:** Low — currently harmless, but fragile if the `breakPointType` iota
values are ever reordered  
**Status: RESOLVED**

### DEBUGGER-BREAKS-2: Original behavior

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

### DEBUGGER-BREAKS-2: Fix

Replaced the magic literal with the named constant:

```go
if bp.Kind == BreakValue {
```

---

<a name="debugger-breaks-3"></a>

## DEBUGGER-BREAKS-3 — clearBreak functions continue iterating after first deletion

**File:** `debugger/breaks.go`  
**Functions:** `clearBreakWhen`, `clearBreakAtLine`  
**Risk:** Medium — if duplicate breakpoints ever exist (due to a future bug),
only the first match is cleanly removed and subsequent matches may be skipped
due to slice-shift confusion  
**Status: RESOLVED**

### DEBUGGER-BREAKS-3: Original behavior

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

### DEBUGGER-BREAKS-3: Fix

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

---

<a name="debugger-commands-1"></a>

## DEBUGGER-COMMANDS-1 — errors.Equal (full string) used instead of errors.Equals (key-only)

**File:** `debugger/commands.go`  
**Function:** `debuggerPrompt`, the `"print"` case  
**Risk:** Medium — `print` command output may be silently discarded when the
inner execution stops with a contextualized `ErrStop`  
**Status: RESOLVED**

### DEBUGGER-COMMANDS-1: Original behavior

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

### DEBUGGER-COMMANDS-1: Fix

Changed the call at the affected line from `errors.Equal` to `errors.Equals`:

```go
if err == nil || errors.Equals(err, errors.ErrStop) {
```

---

<a name="debugger-show-1"></a>

## DEBUGGER-SHOW-1 — showSource silently swallows invalid-range parse errors

**File:** `debugger/show.go`, `debugger/source.go`  
**Functions:** `showSource`, `showCommand`  
**Risk:** Medium — `show source bad-arg` silently does nothing; the user
receives no feedback that the argument was invalid  
**Status: RESOLVED**

### DEBUGGER-SHOW-1: Original behavior

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

### DEBUGGER-SHOW-1: Fix

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

---

<a name="debugger-show-2"></a>

## DEBUGGER-SHOW-2 — Source indentation counts braces inside string literals

**File:** `debugger/source.go`  
**Function:** `showSource`  
**Risk:** Low — cosmetic display defect only; does not affect execution  
**Status: DOCUMENTED (not fixed)**

### DEBUGGER-SHOW-2: Original behavior

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

### DEBUGGER-SHOW-2: Analysis

The formatter is a best-effort display aid rather than a semantic renderer.
Fixing it properly would require tokenizing each source line and skipping
characters inside string literals.  Given that the error only affects display
alignment (not execution), and that the current tokenizer's `New()` call
carries measurable overhead for short expressions, this issue is documented
but deferred.
