package time

import (
	"testing"
	"time"

	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/data"
)

// ---------------------------------------------------------------------------
// Helpers for the Parse (Ego "time.ParseAny") tests
//
// These tests cover TIME-1: a timestamp containing a bare timezone
// abbreviation ("... 10:35am EST") used to be resolved against whatever
// timezone the host process happened to be configured for, so the same input
// produced -0500 on a US-Eastern developer machine and +0000 in a UTC
// container. Parse() now resolves such abbreviations against the location
// named by the ego.runtime.timezone setting instead.
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

	previous, existed := settings.Get(defs.RuntimeTimeZoneSetting), settings.Exists(defs.RuntimeTimeZoneSetting)

	if value == "" {
		settings.DeleteDefault(defs.RuntimeTimeZoneSetting)
	} else {
		settings.SetDefault(defs.RuntimeTimeZoneSetting, value)
	}

	// Parse() caches the last location it resolved, keyed by the setting
	// string. Changing the setting normally invalidates that cache on its own,
	// but a test that changes time.Local while leaving the setting at "local"
	// would otherwise still see the previously cached location. Clearing the
	// cache directly (these tests are in the same package, so they can reach
	// the unexported variables) removes that ordering dependency entirely.
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

// resetLocationCache empties the memoized location lookup in parse.go.
func resetLocationCache() {
	locationLock.Lock()
	defer locationLock.Unlock()

	locationName = ""
	locationValue = nil
}

// withLocalTimeZone temporarily replaces Go's notion of the host's local
// timezone. Several tests use this to prove that Parse()'s result no longer
// depends on the host -- the whole point of TIME-1 -- by producing the same
// answer with time.Local set to two very different zones.
func withLocalTimeZone(t *testing.T, name string) {
	t.Helper()

	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatalf("cannot load location %q: %v", name, err)
	}

	previous := time.Local
	time.Local = loc

	t.Cleanup(func() {
		time.Local = previous
		resetLocationCache()
	})

	resetLocationCache()
}

// parseString calls Parse() the way the Ego runtime does and returns the
// parsed time. Parse() reports (value, error) as a two-element data.List, so
// the value has to be unwrapped from index 0.
func parseString(t *testing.T, value string) time.Time {
	t.Helper()

	result, err := Parse(nil, data.NewList(value))
	if err != nil {
		t.Fatalf("Parse(%q) returned unexpected error: %v", value, err)
	}

	list, ok := result.(data.List)
	if !ok {
		t.Fatalf("Parse(%q) returned %T, expected data.List", value, result)
	}

	parsed, ok := list.Get(0).(time.Time)
	if !ok {
		t.Fatalf("Parse(%q) produced %T at index 0, expected time.Time", value, list.Get(0))
	}

	return parsed
}

// ---------------------------------------------------------------------------
// The TIME-1 regression: an abbreviation must not depend on the host timezone
// ---------------------------------------------------------------------------

func TestParse_AbbreviationUsesConfiguredZone(t *testing.T) {
	// The same input is parsed twice, once with the host pretending to be in
	// Tokyo and once pretending to be in Denver. Before the fix, these two
	// runs produced different offsets. Now both must honor the configured
	// reference zone, America/New_York, where "EST" means -05:00.
	const input = "December 7, 1959 10:35am EST"

	for _, hostZone := range []string{"Asia/Tokyo", "America/Denver", "UTC"} {
		t.Run(hostZone, func(t *testing.T) {
			withLocalTimeZone(t, hostZone)
			withTimeZoneSetting(t, "America/New_York")

			parsed := parseString(t, input)

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

func TestParse_AbbreviationHonorsDaylightSaving(t *testing.T) {
	// "EDT" is the summer half of America/New_York's zone table, at -04:00.
	// Looking an abbreviation up in a location finds whichever of that
	// location's zones uses it, not simply the one in effect today.
	withTimeZoneSetting(t, "America/New_York")

	parsed := parseString(t, "July 7, 2024 10:35am EDT")

	name, offset := parsed.Zone()
	if name != "EDT" {
		t.Errorf("zone name = %q, want \"EDT\"", name)
	}

	if want := -4 * 60 * 60; offset != want {
		t.Errorf("offset = %d seconds, want %d", offset, want)
	}
}

func TestParse_AbbreviationWithUTCReference(t *testing.T) {
	// Configuring UTC is the way to ask for the strictest, most reproducible
	// behavior: UTC's zone table contains no regional abbreviations, so "EST"
	// keeps its name but gets no offset. This is a deliberate choice rather
	// than an error -- see the comment at the end of Parse().
	withLocalTimeZone(t, "America/New_York")
	withTimeZoneSetting(t, defs.UTCTimeZone)

	parsed := parseString(t, "December 7, 1959 10:35am EST")

	name, offset := parsed.Zone()
	if name != "EST" {
		t.Errorf("zone name = %q, want \"EST\"", name)
	}

	if offset != 0 {
		t.Errorf("offset = %d seconds, want 0", offset)
	}
}

func TestParse_UnknownAbbreviationIsNotAnError(t *testing.T) {
	// "JST" is not an abbreviation America/New_York uses, so there is nothing
	// to resolve it to. Parse() returns the zero offset rather than failing,
	// matching the behavior callers have always seen for an abbreviation the
	// reference location does not recognize.
	withTimeZoneSetting(t, "America/New_York")

	parsed := parseString(t, "December 7, 1959 10:35am JST")

	name, offset := parsed.Zone()
	if name != "JST" {
		t.Errorf("zone name = %q, want \"JST\"", name)
	}

	if offset != 0 {
		t.Errorf("offset = %d seconds, want 0", offset)
	}
}

// ---------------------------------------------------------------------------
// Inputs the setting must NOT affect
// ---------------------------------------------------------------------------

func TestParse_InputsUnaffectedByTimeZoneSetting(t *testing.T) {
	// A string that names no zone, or that states its offset numerically, has
	// nothing ambiguous about it, so the configured reference zone must not
	// change how it is read. Each of these is parsed under two very different
	// settings and must give the identical instant both times.
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
		{"explicit Z", "2024-01-15T10:00:00Z", "2024-01-15 10:00:00 +0000 UTC"},
		{"numeric offset", "2024-01-15T10:00:00-08:00", "2024-01-15 10:00:00 -0800 -0800"},
		{"unix timestamp", "1500000000", "2017-07-14 02:40:00 +0000 UTC"},
	}

	for _, testCase := range inputs {
		t.Run(testCase.name, func(t *testing.T) {
			for _, zone := range []string{defs.UTCTimeZone, "America/New_York", "Asia/Tokyo"} {
				withTimeZoneSetting(t, zone)

				if got := parseString(t, testCase.input).String(); got != testCase.want {
					t.Errorf("with ego.runtime.timezone=%s, Parse(%q) = %q, want %q",
						zone, testCase.input, got, testCase.want)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Resolution of the setting itself
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
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			withTimeZoneSetting(t, testCase.setting)

			got, err := defaultLocation()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != testCase.want {
				t.Errorf("defaultLocation() = %v, want %v", got, testCase.want)
			}
		})
	}
}

func TestDefaultLocation_IANAName(t *testing.T) {
	// An IANA name must load even on a host with no timezone database of its
	// own, because parse.go embeds a copy via the time/tzdata import.
	withTimeZoneSetting(t, "Asia/Tokyo")

	loc, err := defaultLocation()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if loc.String() != "Asia/Tokyo" {
		t.Errorf("defaultLocation() = %v, want Asia/Tokyo", loc)
	}
}

func TestDefaultLocation_SurroundingWhitespaceIgnored(t *testing.T) {
	// A value typed with a stray space ("ego config set ... = UTC") should
	// still work rather than being reported as an unknown zone.
	withTimeZoneSetting(t, "  UTC  ")

	loc, err := defaultLocation()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if loc != time.UTC {
		t.Errorf("defaultLocation() = %v, want UTC", loc)
	}
}

func TestDefaultLocation_ChangedSettingIsNoticed(t *testing.T) {
	// The resolved location is cached, so this confirms the cache is keyed by
	// the setting value and not simply remembered forever.
	withTimeZoneSetting(t, "Asia/Tokyo")

	first, err := defaultLocation()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	settings.SetDefault(defs.RuntimeTimeZoneSetting, "America/New_York")

	second, err := defaultLocation()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if first.String() != "Asia/Tokyo" || second.String() != "America/New_York" {
		t.Errorf("cache did not follow the setting: got %v then %v", first, second)
	}
}

func TestDefaultLocation_InvalidName(t *testing.T) {
	withTimeZoneSetting(t, "Not/AZone")

	if _, err := defaultLocation(); err == nil {
		t.Fatal("expected an error for an unloadable timezone name, got nil")
	} else if !errors.Equals(err, errors.ErrInvalidTimeZone) {
		t.Errorf("error = %v, want ErrInvalidTimeZone", err)
	}
}

// ---------------------------------------------------------------------------
// Error paths through Parse()
// ---------------------------------------------------------------------------

func TestParse_InvalidTimeZoneSettingIsReported(t *testing.T) {
	// A misconfigured reference zone only matters for input that actually
	// needs one, so it surfaces on an abbreviation...
	withTimeZoneSetting(t, "Not/AZone")

	result, err := Parse(nil, data.NewList("December 7, 1959 10:35am EST"))
	if err == nil {
		t.Fatal("expected an error for an unloadable ego.runtime.timezone, got nil")
	}

	if !errors.Equals(err, errors.ErrInvalidTimeZone) {
		t.Errorf("error = %v, want ErrInvalidTimeZone", err)
	}

	// ...and the error is also carried in the list, which is where Ego code
	// picking up the second return value of ParseAny() reads it from.
	list, ok := result.(data.List)
	if !ok {
		t.Fatalf("expected data.List, got %T", result)
	}

	if list.Get(1) == nil {
		t.Error("expected the error to be reported at index 1 of the result list")
	}
}

func TestParse_InvalidTimeZoneSettingIgnoredWhenUnneeded(t *testing.T) {
	// ...but a timestamp that needs no reference zone still parses, so a bad
	// setting cannot break programs that never relied on it.
	withTimeZoneSetting(t, "Not/AZone")

	if got := parseString(t, "Dec 7, 1959").String(); got != "1959-12-07 00:00:00 +0000 UTC" {
		t.Errorf("Parse() = %q, want \"1959-12-07 00:00:00 +0000 UTC\"", got)
	}
}

func TestParse_UnrecognizedInput(t *testing.T) {
	// "Decembre" is not an English month name, so there is no layout to
	// deduce and the underlying parse fails.
	withTimeZoneSetting(t, "America/New_York")

	result, err := Parse(nil, data.NewList("Decembre 7, 1959"))
	if err == nil {
		t.Fatal("expected an error for unparseable input, got nil")
	}

	list, ok := result.(data.List)
	if !ok {
		t.Fatalf("expected data.List, got %T", result)
	}

	if list.Get(0) != nil {
		t.Errorf("expected a nil time value on error, got %v", list.Get(0))
	}
}
