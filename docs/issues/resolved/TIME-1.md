# TIME-1 — `time.ParseAny`'s bare zone-abbreviation resolution depends on the host's local timezone

**Severity:** Low — narrow input shape, silent wrong offset rather than a crash or security issue

**Affected file:** `internal/runtime/time/parse.go` — `Parse()`

**Discovered by:** `tools/test_container.sh` isolated test runs. `tests/time/parse.ego`'s
"ParseAny flexible format detection" test failed inside a fresh Docker
container (default timezone UTC) while passing on every developer machine
that happened to be configured for US Eastern time.

**Status:** FIXED

## Description

`Parse()` called `dateparse.ParseAny(value)` with no location argument:

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

```text
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

## Fix

A new configuration setting, `ego.runtime.timezone`, names the reference
location that gives a bare abbreviation its meaning. `Parse()` resolves
abbreviations against that location instead of against the ambient
`time.Local`, so the result is a function of the input and the configuration
rather than of the host the program happens to run on.

The parse is done in two passes, which is what keeps the change confined to
the ambiguous case:

1. `dateparse.ParseIn(value, time.UTC)`. Pinning the location to UTC fixes
   both places the location can matter: a string with no zone information is
   read as UTC (exactly what `ParseAny` already did, since Go's `time.Parse`
   defaults to UTC), and a bare abbreviation is left *unresolved* — its name
   is kept, its offset is zero.

2. The parsed value's `Zone()` name says which case the input was. An empty
   name means the string gave a numeric offset; `"UTC"` means it named no
   zone, or said `UTC`/`Z` explicitly. Both are already correct and are
   returned as-is. Any other name is an abbreviation that needs resolving, and
   only then is the value re-parsed with `dateparse.ParseIn(value, loc)`
   against the configured location.

Routing only the third case through the configured location is what makes
this safe to land: `"Dec 7, 1959"` still parses as UTC, `"2024-01-15T10:00:00-08:00"`
still keeps its stated offset, and a Unix epoch string like `"1500000000"` —
an absolute instant by definition — is not shifted by hours. A single-pass
`ParseIn(value, loc)` would have changed all three.

### Answers to the open questions in the original write-up

**What should the reference location be?** Configurable, with the host's own
local timezone as the fallback — `ego.runtime.timezone`, whose value is an
IANA name (`America/New_York`), the word `UTC`, or the word `local`. A new
profile is seeded with `local`, and a missing setting is treated identically
to `local`, so existing installations see no behavior change from the default.
No fixed location was chosen as the built-in default: `"CST"` is US Central,
China Standard, and Cuba Standard Time, so no single choice makes every input
unambiguous, and picking `America/New_York` because the test corpus happens to
assume it would have silently imposed a US reading on everyone else.

`local` is explicitly documented as a *guess*. Go derives `time.Local` from
`TZ`, then `/etc/localtime`, then UTC — and that last fallback is the original
bug's environment. There is no better source to guess from: Go exposes no
other locale information, and a language or country locale does not determine
a timezone anyway (the United States spans six). `docs/CONFIG.md` says so
directly and tells anyone parsing abbreviations to set the value explicitly.

**Should it be configurable?** Yes — that is the substance of the fix. The
setting is settable from the CLI (`ego config set`), per-run
(`ego --set ego.runtime.timezone=... run`), and from Ego test code
(`profile.Set`), which is how the tests below pin it.

**What should happen when the abbreviation doesn't match the chosen
location** (parsing `"JST"` against `America/New_York`)? The name is kept and
the offset stays zero — deliberately *not* an error. Erroring would be a
behavior change for every existing caller that currently tolerates the zero
offset, and it is the same answer `ParseAny` has always given for an
abbreviation it could not resolve. What changed is that the answer is now
reproducible instead of depending on the host. A caller needing certainty
should arrange for a numeric offset in the input. Setting
`ego.runtime.timezone` to a name Go cannot load *is* reported, as
`ErrInvalidTimeZone`, but only from a call that actually needed a reference
zone — a bad setting cannot break programs that never relied on it.

### Timezone database in slim images

`parse.go` imports `_ "time/tzdata"`, which embeds a copy of the IANA database
in the executable as a fallback for hosts that ship none. Without it,
`time.LoadLocation("America/New_York")` fails on a minimal container image —
that is, the new setting would have been unusable in precisely the deployments
this issue was reported from. The cost is roughly 450KB of binary size, and the
embedded copy is only consulted when the host has no database of its own.

## Verification

Same input, same three host timezones, with the setting pinned and then left
at its `local` default:

```text
TZ=UTC                setting=America/New_York -> 1959-12-07 10:35:00 -0500 EST
TZ=America/New_York   setting=America/New_York -> 1959-12-07 10:35:00 -0500 EST
TZ=Asia/Tokyo         setting=America/New_York -> 1959-12-07 10:35:00 -0500 EST

TZ=UTC                setting=local            -> 1959-12-07 10:35:00 +0000 EST
TZ=America/New_York   setting=local            -> 1959-12-07 10:35:00 -0500 EST
TZ=Asia/Tokyo         setting=local            -> 1959-12-07 10:35:00 +0000 EST
```

The first group is the fix: one answer, whatever the host. The second group
reproduces the old host-dependent behavior, which is what `local` means and why
the documentation says to set the value explicitly.

The full Ego suite (1712 tests) passes with `TZ` set to `UTC`,
`America/New_York`, and `Asia/Tokyo`.

## Tests

`internal/runtime/time/parse_test.go` is new. It replaces Go's `time.Local`
with three different zones and asserts the same input yields the same offset in
all three (the regression that would have caught this originally); checks
daylight-saving abbreviations (`"EDT"` → −04:00) resolve against the same
location's zone table; checks an unresolvable abbreviation returns a zero
offset rather than an error; and checks that zone-less strings, explicit `Z`,
numeric offsets, and Unix timestamps are *unaffected* by the setting under
three different values of it. It also covers resolution of the setting itself:
`local`/`UTC` case-insensitively, whitespace tolerance, IANA names, the
memoized lookup following a changed setting, and `ErrInvalidTimeZone` for a
name that cannot be loaded.

`tests/time/parse.ego` adds three `@test` blocks — abbreviations resolving
against two different configured zones (`"CST"` as −06:00 in `America/Chicago`
and +08:00 in `Asia/Shanghai`, which is the ambiguity made concrete),
timestamps that must ignore the setting, and the unloadable-name error.
`tests/time/parse.ego` and `tests/time/time.ego` both now pin
`ego.runtime.timezone` in the tests that assert an `"EST"` offset, instead of
depending on the process timezone.

## Related

**Other `dateparse.ParseAny` callers were deliberately left alone.** They have
the same latent host-dependence, but each is a separate behavior change with
its own risk, and none is what this issue reported:

- `internal/server/tables/parsing/generators.go` and `internal/commands/tables.go`
  coerce a client-supplied string into a SQL timestamp column. This is the one
  with real exposure — a REST client could send `"... EST"` — but changing how
  a database value is interpreted needs its own testing against the write path
  that is supposed to round-trip with it.
- `internal/runtime/rest/client.go` parses an HTTP expiration header, which
  conventionally carries `GMT` — zero offset in every location, so nothing to
  resolve.
- `internal/server/admin/tokens.go`, `internal/runtime/runtime/info.go`, and
  `internal/cli/app/app.go` parse timestamps Ego itself generated, in formats
  with no bare abbreviation.

If the tables path is addressed later it should reuse `defaultLocation()` from
`internal/runtime/time/parse.go` rather than growing a second setting.

`tools/test.sh` no longer exports `TZ=America/New_York`. That pin was the
mitigation for this issue — it made the *test* reproducible without changing
what a user or server saw. With the tests pinning the setting themselves, the
suite runs in whatever timezone the developer or container actually uses, which
is what would catch a regression of this kind.
