package time

import (
	"testing"
	"time"

	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/data"
)

// Parse() is a thin adapter: it unpacks the Ego argument list, calls
// util.ParseTimestamp(), and repacks the result as the (value, error) data.List
// the Ego runtime expects. The parsing and timezone rules themselves are
// covered by internal/util/timezone_test.go, which can also reach the location
// cache to pin down "local" resolution.
//
// What is tested here is what only this layer does: that the Ego-visible
// result carries the right value, that an error is reported through both the
// Go return and the list, and -- as a guard against the adapter being rewired
// to the wrong helper -- that time.ParseAny() stays the *lenient* one. That
// last point matters because the table layer deliberately uses the strict
// helper instead, and swapping them would silently change either the language
// or the database contract.

// withTimeZoneSetting sets ego.runtime.timezone for one test and restores the
// previous state afterwards. SetDefault/DeleteDefault touch only the in-memory
// overlay, so a test never writes to the developer's configuration file.
func withTimeZoneSetting(t *testing.T, value string) {
	t.Helper()

	previous := settings.Get(defs.RuntimeTimeZoneSetting)
	existed := settings.Exists(defs.RuntimeTimeZoneSetting)

	settings.SetDefault(defs.RuntimeTimeZoneSetting, value)

	t.Cleanup(func() {
		if existed {
			settings.SetDefault(defs.RuntimeTimeZoneSetting, previous)
		} else {
			settings.DeleteDefault(defs.RuntimeTimeZoneSetting)
		}
	})
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

func TestParse_ResolvesAbbreviationAgainstSetting(t *testing.T) {
	withTimeZoneSetting(t, "America/New_York")

	if got := parseString(t, "December 7, 1959 10:35am EST").String(); got != "1959-12-07 10:35:00 -0500 EST" {
		t.Errorf("Parse() = %q, want \"1959-12-07 10:35:00 -0500 EST\"", got)
	}
}

func TestParse_ZonelessInputStaysUTC(t *testing.T) {
	// A string naming no zone must not be shifted into the configured zone.
	withTimeZoneSetting(t, "Asia/Tokyo")

	if got := parseString(t, "Dec 7, 1959").String(); got != "1959-12-07 00:00:00 +0000 UTC" {
		t.Errorf("Parse() = %q, want \"1959-12-07 00:00:00 +0000 UTC\"", got)
	}
}

func TestParse_UnresolvableAbbreviationIsLenient(t *testing.T) {
	// time.ParseAny() must keep using the lenient parse: an abbreviation the
	// reference zone cannot resolve yields a zero offset rather than an error.
	// The strict counterpart used by the table layer rejects this same input,
	// so if this test starts failing, the adapter has been pointed at the
	// wrong helper and the language's behavior has changed (TIME-2).
	withTimeZoneSetting(t, "America/New_York")

	parsed := parseString(t, "December 7, 1959 10:35am JST")

	name, offset := parsed.Zone()
	if name != "JST" || offset != 0 {
		t.Errorf("Parse() zone = %q offset = %d, want \"JST\" and 0", name, offset)
	}
}

func TestParse_ErrorIsReportedBothWays(t *testing.T) {
	// An unparseable string must produce a Go error and also carry that error
	// at index 1 of the list, which is where Ego code reading the second
	// return value of ParseAny() finds it.
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

	if list.Get(1) == nil {
		t.Error("expected the error to be reported at index 1 of the result list")
	}
}

func TestParse_InvalidTimeZoneSettingIsReported(t *testing.T) {
	// A reference zone Go cannot load surfaces as ErrInvalidTimeZone, tagged
	// with this function's name.
	withTimeZoneSetting(t, "Not/AZone")

	_, err := Parse(nil, data.NewList("December 7, 1959 10:35am EST"))
	if err == nil {
		t.Fatal("expected an error for an unloadable ego.runtime.timezone, got nil")
	}

	if !errors.Equals(err, errors.ErrInvalidTimeZone) {
		t.Errorf("error = %v, want ErrInvalidTimeZone", err)
	}
}
