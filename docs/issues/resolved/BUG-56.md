# BUG-56 — `fmt.Println` of a `time.Duration`/`time.Time` prints the internal Go struct layout instead of the formatted string

**Severity:** MEDIUM  **Status:** Fixed

**Description:**  
Printing a `time.Duration` or `time.Time` value directly via `fmt.Println`/`fmt.Print`
shows Ego's internal representation of the underlying native Go type (its Go struct field
layout) rather than the human-readable formatted string that `.String()` and `%v` both
correctly produce for the same value. This directly affects `docs/LANGUAGE.md`'s own `time`
examples, which show printing durations/times directly.

**Reproducer:**

```go
import "time"

func main() {
    d, _ := time.ParseDuration("90m")
    fmt.Println(d)
    fmt.Println(d.String())
    fmt.Println(fmt.Sprintf("%v", d))
}
```

**Actual output:**

```text
time.Duration int64 1h30m0s
1h30m0s
1h30m0s
```

**Expected output:**

```text
1h30m0s
1h30m0s
1h30m0s
```

**Notes:**  
`time.Time` shows the same bug: `fmt.Println(t)` produces
`time.Time struct{wall: uint64, ext: int64, loc: *time.Location} = 1960-12-07 15:30:00 +0000 UTC`
instead of the clean timestamp. `.String()` and `%v` are unaffected — only the bare
`fmt.Println`/`fmt.Print` path shows the internal layout, indicating an inconsistency in
how Ego's default value formatter handles these two native types compared to its own
`%v`/`.String()` formatting path.

**Fix:**  
Root cause confirmed: `fmt.Println`/`fmt.Print` (`internal/runtime/fmt/print.go`'s
`formatUsingString`) only special-cases a `*data.Struct` value that has a registered
Ego-level `"String"` function — anything else falls back to `data.FormatUnquoted` →
`data.Format`. `data.Format` in turn only recognizes `*time.Time` (pointer) as a special
case; a bare `time.Time` (value) or `time.Duration` isn't matched by any `case` in its type
switch, so both fall into the `default` branch and are formatted by
`formatNativeGoValue`'s generic reflection-based fallback — exactly the struct-layout dump
seen in the bug report. Meanwhile `.String()` (a real method call) and `%v` (which
delegates straight to Go's own `fmt.Sprintf`, which already understands `fmt.Stringer`)
never go through this code path at all, which is why only bare `fmt.Println`/`fmt.Print`
were affected.

`data.Format`'s `default` branch already has an extensibility mechanism for exactly this
situation: `packageTypes`, a registry of native Go type names to `*data.Type` objects, each
of which can carry an optional `.format` function set via `Type.SetFormatFunc(...)`. Both
`TimeType` and `TimeDurationType` (`internal/runtime/time/types.go`) already call
`SetNativeName(...)` (which populates `packageTypes`) but never called `SetFormatFunc`, so
the registry entry existed with no formatter attached. The fix adds one to each:

```go
var TimeType = data.TypeDefinition("Time", data.StructType).
    SetNativeName("time.Time").
    SetPackage("time").
    SetFormatFunc(func(v any) string {
        t, _ := v.(time.Time)
        return t.String()
    }).
    ...

var TimeDurationType = data.TypeDefinition("Duration", data.StructureType()).
    SetNativeName(defs.TimeDurationTypeName).
    SetPackage("time").
    SetFormatFunc(func(v any) string {
        d, _ := v.(time.Duration)
        return d.String()
    }).
    ...
```

**Incidental fixes (found via the same survey):** `TimeLocationType` (`*time.Location`) and
`TimeMonthType` (`time.Month`) have the identical root cause — both are native Go types
with their own `String()` method, registered via `SetNativeName` but never given a
`SetFormatFunc`. Before this fix, `fmt.Println(loc)` dumped `*time.Location`'s entire
internal zone-transition table, and `fmt.Println(month)` printed `"time.Month int July"`
instead of `"July"`. Both got the same one-line `SetFormatFunc` treatment as `Time` and
`Duration` above, using the same mechanism already present in the codebase.

Verified against the documented reproducer (now prints `1h30m0s` three times, matching
`.String()`/`%v` exactly) and the `time.Time`/`time.Location`/`time.Month` cases. Regression
tests: `TestFormat_TimeDuration_MatchesString`, `TestFormat_TimeTime_MatchesString`,
`TestFormat_TimeLocation_MatchesString`, and `TestFormat_TimeMonth_MatchesString` in
`internal/runtime/time/time_test.go` (calling `data.Format` directly), plus four new
`@test` cases across `tests/time/duration.ego`, `tests/time/time_methods.ego`, and
`tests/time/time_create.ego` that call `fmt.Println` on each type and check the returned
byte count matches the length of `.String()`'s output plus a trailing newline.

