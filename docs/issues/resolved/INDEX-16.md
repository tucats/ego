# INDEX-16 — Profile report re-parses its own sort key and dereferences a nil counter

**Affected function:** `PrintProfileReport`
**File:** `util/profiling/dump.go`
**Risk:** Low — nil pointer dereference for a module name containing `#`
**Status: RESOLVED**

## INDEX-16: Description

```go
for name := range PerformanceData {
    parts := strings.Split(name, ":")
    key := fmt.Sprintf("%s:%4s#%s", parts[0], parts[1], name)
    keys = append(keys, key)
}
...
for _, key := range keys {
    parts := strings.Split(key, "#")
    count := PerformanceData[parts[1]]

    err = t.AddRowItems(parts[1], count.Load())
```

A profile key is `module:line` (see `Count` in `count.go`), where `module` is a
source file name supplied by the user. Neither of the indexed splits accounts for
that:

- `strings.Split(name, ":")` splits on the *first* colon, so a module name
  containing `:` puts part of the module in `parts[1]` where the line number
  belongs — the report sorts and labels those rows wrongly.
- The round trip through `#` is worse. A module name containing `#` makes
  `strings.Split(key, "#")` return more than two fields, so `parts[1]` is a
  fragment rather than the original key. The `PerformanceData` lookup misses,
  `count` is a nil `*atomic.Uint32`, and `count.Load()` panics.

Encoding data into a string and then re-parsing it out is what created the
exposure; the original key was available the whole time.

## INDEX-16: Fix

The line number is separated on the *last* colon, and the original key is carried
alongside the sort key in a small struct instead of being encoded into it and
recovered by splitting:

```go
type profileEntry struct {
    sortKey string
    name    string
}

module, line := name, ""
if at := strings.LastIndex(name, ":"); at >= 0 {
    module, line = name[:at], name[at+1:]
}
```

A nil check on the map lookup was retained as a guard, even though ranging over
`PerformanceData` now guarantees the counter is present.
