# TIME-1 — `time.ParseAny`'s bare zone-abbreviation resolution depends on the host's local timezone

**Severity:** Low — narrow input shape, silent wrong offset rather than a crash or security issue

**Affected file:** `internal/runtime/time/parse.go` — `Parse()`

**Discovered by:** `tools/test_container.sh` isolated test runs. `tests/time/parse.ego`'s
"ParseAny flexible format detection" test failed inside a fresh Docker
container (default timezone UTC) while passing on every developer machine
that happened to be configured for US Eastern time.

**Status: OPEN**

**Description:**

`Parse()` calls `dateparse.ParseAny(value)` with no location argument:

```go
t, e := dateparse.ParseAny(value)
```

The `dateparse` package documents `ParseAny` as following "Equivalent
Timezone rules as `time.Parse()`". Go's standard library `time.Parse` docs
describe what that means for a bare zone abbreviation with no numeric offset
(e.g. `"EST"`, `"PST"`, `"CST"`): it is only resolved to a real UTC offset if
it happens to match the abbreviation of the *process's local timezone*
(`time.Local`); otherwise the abbreviation is kept but the offset is silently
recorded as `+0000`.

Confirmed directly (same input, only `TZ` changed):

```
$ TZ=UTC              ./ego run parse_est.ego   # "December 7, 1959 10:35am EST"
GOT: 1959-12-07 10:35:00 +0000 EST

$ TZ=America/New_York ./ego run parse_est.ego   # identical input
GOT: 1959-12-07 10:35:00 -0500 EST
```

The offset is wrong in the first case, but nothing about the call indicates
that — no error, no warning, and the zone abbreviation in the formatted
output ("EST") is identical either way. Any Ego program or server parsing a
timestamp with a bare zone abbreviation gets a result that depends entirely
on the timezone the *host process* happens to be configured with, not on
the input string. Since servers commonly default to UTC, this is arguably
backwards from a user's perspective: the environment least likely to know
what "EST" means (UTC) is also the most common deployment default.

**Current mitigation (not a fix):** `tools/test.sh` now pins
`TZ=America/New_York` for the duration of the test run, purely so
`tests/time/parse.ego`'s expected value is reproducible in CI/containers
regardless of the host's configured timezone. This makes the *test*
deterministic; it does nothing for the underlying behavior seen by an actual
user or server running with a different `TZ`.

**Suggested fix (future work):**

Switch to `dateparse.ParseIn(value, loc)`, passing an explicit,
documented reference `*time.Location` instead of relying on the ambient
process timezone. This makes the result deterministic and reproducible
regardless of where the Ego program happens to run, rather than incidentally
tied to host configuration. Open questions to resolve before implementing:

- **What should the reference location be?** Zone abbreviations are
  inherently overloaded even with an explicit choice — "CST" alone is
  ambiguous between US Central Standard Time and China Standard Time, for
  example — so no fixed location makes every input unambiguous. A US-focused
  default (e.g. `America/New_York`, matching this test's existing
  assumption) is a plausible choice given the existing test corpus, but is a
  real product decision, not just an implementation detail.
- **Should it be configurable?** A new setting (e.g.
  `ego.runtime.timezone.default`) would let a deployment pick the locale
  that makes sense for its users, at the cost of one more setting to
  document and maintain.
- **What should happen when the abbreviation doesn't match the chosen
  location at all** (e.g. parsing "JST" against an `America/New_York`
  reference)? Silently falling back to `+0000` reproduces today's
  surprising behavior; returning an error instead would be more honest
  about the ambiguity, but is a behavior change for any existing caller
  that currently gets a (possibly wrong) zero-offset result rather than an
  error.
