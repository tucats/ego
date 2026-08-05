package util

import (
	"testing"
	"time"

	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
)

// ---------------------------------------------------------------------------
// Helpers
//
// These tests cover TIME-1 and TIME-2. A timestamp containing a bare timezone
// abbreviation ("... 10:35am EST") used to be resolved against whatever
// timezone the host process happened to be configured for, so the same input
// produced -0500 on a US-Eastern machine and +0000 in a UTC container.
// Abbreviations are now resolved against the location named by the
// ego.runtime.timezone setting, and the strict form rejects an abbreviation
// that location cannot resolve.
// ---------------------------------------------------------------------------

// withTimeZoneSetting sets ego.runtime.timezone for the duration of one test
// and restores the previous state afterwards.
//
// SetDefault/DeleteDefault are used rather than Set/Delete because they touch
// only the in-memory overlay: a test must never write to the developer's real
// configuration file on disk.
//
// t.Cleanup registers a function that Go runs when the test finishes, however
// it finishes -- including a t.Fatalf part-way through. That makes it a safer
// way to undo test setup than putting the restore code at the end of the test
// body, where an early exit would skip it.
func withTimeZoneSetting(t *testing.T, value string) {
	t.Helper()

	previous := settings.Get(defs.RuntimeTimeZoneSetting)
	existed := settings.Exists(defs.RuntimeTimeZoneSetting)

	if value == "" {
		settings.DeleteDefault(defs.RuntimeTimeZoneSetting)
	} else {
		settings.SetDefault(defs.RuntimeTimeZoneSetting, value)
	}

	// DefaultLocation caches the last location it resolved, keyed by the
	// setting string. Changing the setting normally invalidates that cache on
	// its own, but a test that changes time.Local while leaving the setting at
	// "local" would otherwise still see the previously cached location.
	// Clearing the cache directly (these tests are in the same package, so
	// they can reach the unexported variables) removes that ordering
	// dependency entirely.
	resetLocationCache()

	t.Cleanup(func() {
		if existed {
			settings.SetDefault(defs.RuntimeTimeZoneSetting, previous)
		} else {
			settings.DeleteDefault(defs.RuntimeTimeZoneSetting)
		}

		resetLocationCache()
	})
}

// resetLocationCache empties the memoized location lookup.
func resetLocationCache() {
	locationLock.Lock()
	defer locationLock.Unlock()

	locationName = ""
	locationValue = nil
}

// withLocalTimeZone temporarily replaces Go's notion of the host's local
// timezone. Several tests use this to prove the result no longer depends on
// the host -- the whole point of TIME-1 -- by producing the same answer with
// time.Local set to very different zones.
func withLocalTimeZone(t *testing.T, name string) {
	t.Helper()

	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatalf("cannot load location %q: %v", name, err)
	}

	previous := time.Local
	time.Local = loc

	resetLocationCache()

	t.Cleanup(func() {
		time.Local = previous

		resetLocationCache()
	})
}

// ---------------------------------------------------------------------------
// DefaultLocation: resolving the setting itself
// ---------------------------------------------------------------------------

func TestDefaultLocation_Resolution(t *testing.T) {
	// The host's local zone is pinned to something recognizable so the
	// "local" and "missing" cases below have a definite expected answer.
	withLocalTimeZone(t, "America/Denver")

	testCases := []struct {
		name    string
		setting string
		want    *time.Location
	}{
		{"missing setting falls back to local", "", time.Local},
		{"explicit local", defs.LocalTimeZone, time.Local},
		{"local is case-insensitive", "LOCAL", time.Local},
		{"explicit UTC", defs.UTCTimeZone, time.UTC},
		{"utc is case-insensitive", "utc", time.UTC},
		{"surrounding whitespace ignored", "  UTC  ", time.UTC},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			withTimeZoneSetting(t, testCase.setting)

			got, err := DefaultLocation()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != testCase.want {
				t.Errorf("DefaultLocation() = %v, want %v", got, testCase.want)
			}
		})
	}
}

func TestDefaultLocation_IANAName(t *testing.T) {
	// An IANA name must load even on a host with no timezone database of its
	// own, because this package embeds a copy via the time/tzdata import.
	withTimeZoneSetting(t, "Asia/Tokyo")

	loc, err := DefaultLocation()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if loc.String() != "Asia/Tokyo" {
		t.Errorf("DefaultLocation() = %v, want Asia/Tokyo", loc)
	}
}

func TestDefaultLocation_ChangedSettingIsNoticed(t *testing.T) {
	// The resolved location is cached, so this confirms the cache is keyed by
	// the setting value and not simply remembered forever.
	withTimeZoneSetting(t, "Asia/Tokyo")

	first, err := DefaultLocation()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	settings.SetDefault(defs.RuntimeTimeZoneSetting, "America/New_York")

	second, err := DefaultLocation()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if first.String() != "Asia/Tokyo" || second.String() != "America/New_York" {
		t.Errorf("cache did not follow the setting: got %v then %v", first, second)
	}
}

func TestDefaultLocation_InvalidName(t *testing.T) {
	withTimeZoneSetting(t, "Not/AZone")

	if _, err := DefaultLocation(); err == nil {
		t.Fatal("expected an error for an unloadable timezone name, got nil")
	} else if !errors.Equals(err, errors.ErrInvalidTimeZone) {
		t.Errorf("error = %v, want ErrInvalidTimeZone", err)
	}
}

// ---------------------------------------------------------------------------
// ParseTimestamp: the lenient form used by Ego's time.ParseAny()
// ---------------------------------------------------------------------------

func TestParseTimestamp_AbbreviationUsesConfiguredZone(t *testing.T) {
	// The same input is parsed with the host pretending to be in three very
	// different places. Before the fix these produced different offsets; now
	// all three must honor the configured reference zone, America/New_York,
	// where "EST" means -05:00.
	const input = "December 7, 1959 10:35am EST"

	for _, hostZone := range []string{"Asia/Tokyo", "America/Denver", "UTC"} {
		t.Run(hostZone, func(t *testing.T) {
			withLocalTimeZone(t, hostZone)
			withTimeZoneSetting(t, "America/New_York")

			parsed, err := ParseTimestamp(input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			name, offset := parsed.Zone()
			if name != "EST" {
				t.Errorf("zone name = %q, want \"EST\"", name)
			}

			// -5 hours, expressed in seconds, which is the unit Zone() uses.
			if want := -5 * 60 * 60; offset != want {
				t.Errorf("offset = %d seconds, want %d (host zone %s)", offset, want, hostZone)
			}
		})
	}
}

func TestParseTimestamp_AbbreviationHonorsDaylightSaving(t *testing.T) {
	// "EDT" is the summer half of America/New_York's zone table, at -04:00.
	// Looking an abbreviation up in a location finds whichever of that
	// location's zones uses it, not simply the one in effect today.
	withTimeZoneSetting(t, "America/New_York")

	parsed, err := ParseTimestamp("July 7, 2024 10:35am EDT")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	name, offset := parsed.Zone()
	if name != "EDT" {
		t.Errorf("zone name = %q, want \"EDT\"", name)
	}

	if want := -4 * 60 * 60; offset != want {
		t.Errorf("offset = %d seconds, want %d", offset, want)
	}
}

func TestParseTimestamp_UnknownAbbreviationIsNotAnError(t *testing.T) {
	// "JST" is not an abbreviation America/New_York uses, so there is nothing
	// to resolve it to. The lenient parse returns the zero offset rather than
	// failing, matching what Ego's time.ParseAny() has always done. The strict
	// parse rejects the same input -- see TestStrictParseTimestamp below.
	withTimeZoneSetting(t, "America/New_York")

	parsed, err := ParseTimestamp("December 7, 1959 10:35am JST")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	name, offset := parsed.Zone()
	if name != "JST" {
		t.Errorf("zone name = %q, want \"JST\"", name)
	}

	if offset != 0 {
		t.Errorf("offset = %d seconds, want 0", offset)
	}
}

func TestParseTimestamp_InputsUnaffectedBySetting(t *testing.T) {
	// A string that names no zone, or that states its offset numerically, has
	// nothing ambiguous about it, so the configured reference zone must not
	// change how it is read. Each of these is parsed under three very
	// different settings and must give the identical instant every time.
	//
	// The unix-timestamp case matters especially: seconds-since-epoch is an
	// absolute instant by definition, and shifting it by a reference zone
	// would silently move it by hours.
	inputs := []struct {
		name  string
		input string
		want  string
	}{
		{"date only", "Dec 7, 1959", "1959-12-07 00:00:00 +0000 UTC"},
		{"date and time", "December 7, 1959 10:35am", "1959-12-07 10:35:00 +0000 UTC"},
		{"ISO 8601 date", "2024-01-15", "2024-01-15 00:00:00 +0000 UTC"},
		{"RFC 3339 with Z", "2024-01-15T10:00:00Z", "2024-01-15 10:00:00 +0000 UTC"},
		{"RFC 3339 with offset", "2024-01-15T10:00:00-08:00", "2024-01-15 10:00:00 -0800 -0800"},
		{"unix timestamp", "1500000000", "2017-07-14 02:40:00 +0000 UTC"},
	}

	for _, testCase := range inputs {
		t.Run(testCase.name, func(t *testing.T) {
			for _, zone := range []string{defs.UTCTimeZone, "America/New_York", "Asia/Tokyo"} {
				withTimeZoneSetting(t, zone)

				parsed, err := ParseTimestamp(testCase.input)
				if err != nil {
					t.Fatalf("with setting %s: unexpected error: %v", zone, err)
				}

				if got := parsed.String(); got != testCase.want {
					t.Errorf("with ego.runtime.timezone=%s, ParseTimestamp(%q) = %q, want %q",
						zone, testCase.input, got, testCase.want)
				}
			}
		})
	}
}

func TestParseTimestamp_InvalidSettingReported(t *testing.T) {
	withTimeZoneSetting(t, "Not/AZone")

	// A misconfigured reference zone only matters for input that needs one.
	if _, err := ParseTimestamp("December 7, 1959 10:35am EST"); err == nil {
		t.Error("expected an error for an unloadable ego.runtime.timezone, got nil")
	} else if !errors.Equals(err, errors.ErrInvalidTimeZone) {
		t.Errorf("error = %v, want ErrInvalidTimeZone", err)
	}

	// A timestamp needing no reference zone still parses, so a bad setting
	// cannot break callers that never relied on it.
	parsed, err := ParseTimestamp("Dec 7, 1959")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := parsed.String(); got != "1959-12-07 00:00:00 +0000 UTC" {
		t.Errorf("ParseTimestamp() = %q, want \"1959-12-07 00:00:00 +0000 UTC\"", got)
	}
}

func TestParseTimestamp_UnrecognizedInput(t *testing.T) {
	// Let's try the French word for "December" to see how it handles it, since
	// there is no layout to deduce and the underlying parse fails.
	withTimeZoneSetting(t, "America/New_York")

	if _, err := ParseTimestamp("Decembre 7, 1959"); err == nil { //nolint:misspell
		t.Error("expected an error for unparseable input, got nil")
	}
}

// ---------------------------------------------------------------------------
// StrictParseTimestamp: the form used where the value will be stored
// ---------------------------------------------------------------------------

func TestStrictParseTimestamp_Accepts(t *testing.T) {
	// Everything unambiguous, plus abbreviations the reference zone really
	// does resolve, must be accepted unchanged.
	withTimeZoneSetting(t, "America/New_York")

	testCases := []struct {
		name  string
		input string
		want  string
	}{
		{"RFC 3339 with Z", "2024-01-15T10:00:00Z", "2024-01-15 10:00:00 +0000 UTC"},
		{"RFC 3339 with offset", "2024-01-15T10:00:00-08:00", "2024-01-15 10:00:00 -0800 -0800"},
		{"no zone at all", "2024-01-15", "2024-01-15 00:00:00 +0000 UTC"},
		{"resolvable abbreviation", "December 7, 1959 10:35am EST", "1959-12-07 10:35:00 -0500 EST"},
		{"daylight-saving abbreviation", "July 7, 2024 10:35am EDT", "2024-07-07 10:35:00 -0400 EDT"},
		// GMT/UT/Z mean an offset of exactly zero everywhere, so they need no
		// reference zone and must not be rejected even though Go cannot find
		// them in America/New_York's zone table.
		{"universal GMT", "December 7, 1959 10:35am GMT", "1959-12-07 10:35:00 +0000 GMT"},
		{"universal UTC", "December 7, 1959 10:35am UTC", "1959-12-07 10:35:00 +0000 UTC"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			parsed, err := StrictParseTimestamp(testCase.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got := parsed.String(); got != testCase.want {
				t.Errorf("StrictParseTimestamp(%q) = %q, want %q", testCase.input, got, testCase.want)
			}
		})
	}
}

func TestStrictParseTimestamp_RejectsUnresolvableAbbreviation(t *testing.T) {
	// This is the TIME-2 behavior change: the lenient parse gives "JST" a zero
	// offset, which would then be stored as a wrong instant with no way to
	// detect it later. The strict parse refuses instead.
	withTimeZoneSetting(t, "America/New_York")

	if _, err := StrictParseTimestamp("December 7, 1959 10:35am JST"); err == nil {
		t.Error("expected an error for an unresolvable abbreviation, got nil")
	} else if !errors.Equals(err, errors.ErrAmbiguousTimeZone) {
		t.Errorf("error = %v, want ErrAmbiguousTimeZone", err)
	}
}

func TestStrictParseTimestamp_RejectsAbbreviationUnderUTCReference(t *testing.T) {
	// UTC's zone table holds no regional abbreviation, so with the reference
	// zone set to UTC every abbreviation is unresolvable. This is the
	// configuration a bare container gets by default, and is exactly where
	// TIME-1 was first seen -- so it must reject rather than silently store
	// a zero offset.
	withTimeZoneSetting(t, defs.UTCTimeZone)

	if _, err := StrictParseTimestamp("December 7, 1959 10:35am EST"); err == nil {
		t.Error("expected an error for an abbreviation under a UTC reference, got nil")
	} else if !errors.Equals(err, errors.ErrAmbiguousTimeZone) {
		t.Errorf("error = %v, want ErrAmbiguousTimeZone", err)
	}
}

func TestStrictParseTimestamp_ZeroOffsetZoneIsStillResolved(t *testing.T) {
	// Europe/Lisbon's winter zone "WET" legitimately has an offset of zero.
	// Detecting "unresolved" by testing for a zero offset would wrongly reject
	// this; the implementation tests the attached location instead, which is
	// what makes this case work.
	withTimeZoneSetting(t, "Europe/Lisbon")

	parsed, err := StrictParseTimestamp("December 7, 1959 10:35am WET")
	if err != nil {
		t.Fatalf("unexpected error for a legitimately zero-offset zone: %v", err)
	}

	if _, offset := parsed.Zone(); offset != 0 {
		t.Errorf("offset = %d seconds, want 0", offset)
	}

	if parsed.Location().String() != "Europe/Lisbon" {
		t.Errorf("location = %v, want Europe/Lisbon", parsed.Location())
	}
}

func TestStrictParseTimestamp_IsHostIndependent(t *testing.T) {
	// The same input must be accepted or rejected identically whatever the
	// host timezone is. Before TIME-1/TIME-2 the host decided both the offset
	// and, in effect, whether the value was usable at all.
	withTimeZoneSetting(t, "America/New_York")

	for _, hostZone := range []string{"UTC", "America/New_York", "Asia/Tokyo"} {
		withLocalTimeZone(t, hostZone)
		withTimeZoneSetting(t, "America/New_York")

		parsed, err := StrictParseTimestamp("December 7, 1959 10:35am EST")
		if err != nil {
			t.Fatalf("host %s: unexpected error: %v", hostZone, err)
		}

		if got := parsed.UTC().String(); got != "1959-12-07 15:35:00 +0000 UTC" {
			t.Errorf("host %s: stored instant = %q, want \"1959-12-07 15:35:00 +0000 UTC\"", hostZone, got)
		}

		if _, err := StrictParseTimestamp("December 7, 1959 10:35am JST"); err == nil {
			t.Errorf("host %s: expected JST to be rejected", hostZone)
		}
	}
}
