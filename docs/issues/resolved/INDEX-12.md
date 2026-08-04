# INDEX-12 — `urlString.Path` slices its argument list using a count taken from the format string

{% raw %}

**Affected functions:** `(*urlString).Path`, `URLBuilder`
**File:** `runtime/rest/builder.go`
**Risk:** Low — latent; every current call site passes a balanced format and
argument list
**Status: RESOLVED**

## INDEX-12: Description

Two defective guards in the same file. `URLBuilder` returns an unexported type
and is called only from Go code in `internal/commands`, where the format strings
are `defs` constants — all of which currently balance their verbs against their
arguments. Neither defect is reachable today; both are latent, and `Path` is
exported.

**1. `Path` slices `parts` to a length derived from `format`:**

```go
substitutions := strings.Count(format, "%")

subs := make([]any, substitutions)
copy(subs, parts[:substitutions])
```

`substitutions` counts `%` characters in the format string; `parts` is the
caller's argument list. Nothing checks that there are that many arguments, so
`parts[:substitutions]` panics whenever the format has more verbs than
arguments. A format containing a literal `%%` also inflates the count, since
`strings.Count` reports it as two.

**2. `URLBuilder` loops forever on an unterminated `{{`:**

```go
for strings.Contains(format, "{{") {
    start := strings.Index(format, "{{")
    end := strings.Index(format, "}}")
    format = format[:start] + "%v" + format[end+2:]
}
```

With no `}}` in the string, `end` is -1, so `format[end+2:]` is `format[1:]` —
which still contains the `{{` that the loop condition tests. Each pass appends
another `%v` and re-splices the same text, growing the string without bound
until the process exhausts memory. `strings.Index` also searches the whole
string for `}}` rather than the portion after `start`, so a `}}` appearing
*before* the `{{` produces a garbled rewrite.

## INDEX-12: Fix

`Path` copies only as many arguments as are actually available, leaving any
surplus verbs to be rendered by `fmt` as `%!v(MISSING)` — the normal Go
behavior for a short argument list:

```go
available := substitutions
if available > len(parts) {
    available = len(parts)
}

subs := make([]any, available)
copy(subs, parts[:available])
```

`URLBuilder` searches for the closing `}}` only after the opening `{{`, and
stops rewriting when there is no match:

```go
end := strings.Index(format[start:], "}}")
if end < 0 {
    break
}

end += start
```

{% endraw %}
