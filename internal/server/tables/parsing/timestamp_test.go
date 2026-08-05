package parsing

import (
	"testing"
	"time"

	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
)

// These tests cover TIME-2: CoerceToColumnType used dateparse.ParseAny, which
// resolved a bare zone abbreviation ("December 7, 1959 10:35am EST") against
// whatever timezone the *server process* happened to be configured for. The
// resulting instant was then normalized to UTC and stored, so the same REST
// request wrote values five hours apart depending on which host served it --
// permanently, and reading the row back gave no hint anything was wrong.
//
// Timestamps now go through util.StrictParseTimestamp, which resolves
// abbreviations against ego.runtime.timezone and rejects any it cannot
// resolve rather than storing a guess.

// withTimeZoneSetting pins ego.runtime.timezone for one test. SetDefault and
// DeleteDefault touch only the in-memory overlay, so a test never writes to
// the developer's real configuration file on disk.
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

// withLocalTimeZone temporarily replaces Go's notion of the host's local
// timezone, so a test can prove the coerced value no longer depends on it.
func withLocalTimeZone(t *testing.T, name string) {
	t.Helper()

	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatalf("cannot load location %q: %v", name, err)
	}

	previous := time.Local
	time.Local = loc

	t.Cleanup(func() { time.Local = previous })
}

// timeColumns is the column metadata used throughout: one timestamp column.
func timeColumns() []defs.DBColumn {
	return []defs.DBColumn{{Name: "when", Type: "timestamp"}}
}

func TestCoerceToColumnType_TimestampIsHostIndependent(t *testing.T) {
	// The heart of TIME-2. The same payload value is coerced with the server
	// pretending to run in three different timezones, and then run through
	// bindTimeValue to get the text SQLite would actually store. Before the
	// fix this produced 1959-12-07T10:35:00Z on a UTC host and
	// 1959-12-07T15:35:00Z on a US-Eastern one.
	const want = "1959-12-07T15:35:00Z"

	for _, hostZone := range []string{"UTC", "America/New_York", "Asia/Tokyo"} {
		t.Run(hostZone, func(t *testing.T) {
			withLocalTimeZone(t, hostZone)
			withTimeZoneSetting(t, "America/New_York")

			got, err := CoerceToColumnType("when", "December 7, 1959 10:35am EST", timeColumns())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			stored := bindTimeValue(got, defs.SqliteProvider)
			if stored != want {
				t.Errorf("SQLite would store %v, want %v (host %s)", stored, want, hostZone)
			}
		})
	}
}

func TestCoerceToColumnType_RejectsAmbiguousAbbreviation(t *testing.T) {
	// "JST" is not an abbreviation America/New_York uses. Rather than storing
	// it with a zero offset -- an instant nine hours away from what the client
	// meant, unrecoverable once written -- the request is refused.
	withTimeZoneSetting(t, "America/New_York")

	_, err := CoerceToColumnType("when", "December 7, 1959 10:35am JST", timeColumns())
	if err == nil {
		t.Fatal("expected an error for an unresolvable abbreviation, got nil")
	}

	if !errors.Equals(err, errors.ErrAmbiguousTimeZone) {
		t.Errorf("error = %v, want ErrAmbiguousTimeZone", err)
	}
}

func TestCoerceToColumnType_RejectsAbbreviationUnderUTCReference(t *testing.T) {
	// A server left at the default "local" setting in a bare container has a
	// local zone of UTC, where no regional abbreviation resolves. That is the
	// most common deployment shape, so it must refuse rather than store zero.
	withTimeZoneSetting(t, defs.UTCTimeZone)

	_, err := CoerceToColumnType("when", "December 7, 1959 10:35am EST", timeColumns())
	if err == nil {
		t.Fatal("expected an error for an abbreviation under a UTC reference, got nil")
	}

	if !errors.Equals(err, errors.ErrAmbiguousTimeZone) {
		t.Errorf("error = %v, want ErrAmbiguousTimeZone", err)
	}
}

func TestCoerceToColumnType_RFC3339Unaffected(t *testing.T) {
	// RFC 3339 is the documented contract for clients (docs/TABLES.md). It
	// always states its offset numerically, so it must coerce identically
	// under every reference zone -- this is what makes the strict rule safe
	// for essentially all real traffic.
	inputs := []struct {
		name  string
		input string
		want  string
	}{
		{"UTC designator", "2024-06-15T12:00:00Z", "2024-06-15T12:00:00Z"},
		{"negative offset", "2024-06-15T12:00:00-08:00", "2024-06-15T20:00:00Z"},
		{"positive offset", "2024-06-15T12:00:00+09:00", "2024-06-15T03:00:00Z"},
		{"date only", "2024-06-15", "2024-06-15T00:00:00Z"},
	}

	for _, testCase := range inputs {
		t.Run(testCase.name, func(t *testing.T) {
			for _, zone := range []string{defs.UTCTimeZone, "America/New_York", "Asia/Tokyo"} {
				withTimeZoneSetting(t, zone)

				got, err := CoerceToColumnType("when", testCase.input, timeColumns())
				if err != nil {
					t.Fatalf("with setting %s: unexpected error: %v", zone, err)
				}

				if stored := bindTimeValue(got, defs.SqliteProvider); stored != testCase.want {
					t.Errorf("with ego.runtime.timezone=%s, %q stored as %v, want %v",
						zone, testCase.input, stored, testCase.want)
				}
			}
		})
	}
}

func TestCoerceToColumnType_ErrorNamesTheColumn(t *testing.T) {
	// A rejected timestamp should say which column it came from; a client
	// sending several timestamps in one row otherwise has no way to tell
	// which one was refused.
	withTimeZoneSetting(t, "America/New_York")

	_, err := CoerceToColumnType("when", "December 7, 1959 10:35am JST", timeColumns())
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if text := err.Error(); text == "" {
		t.Fatal("expected a non-empty error message")
	} else if !contains(text, "when") {
		t.Errorf("error %q does not name the column", text)
	}
}

// contains is a tiny substring helper, avoiding a strings import for one use.
func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}

	return false
}

// ---------------------------------------------------------------------------
// Behavior the CLI copy used to have, now that there is only one function
// ---------------------------------------------------------------------------

func TestCoerceToColumnType_Int16(t *testing.T) {
	// "int16" was handled by the CLI's copy of this function but not by this
	// one, so an int16 column coerced differently depending on which side did
	// it. Consolidating the two functions fixed that drift.
	columns := []defs.DBColumn{{Name: "count", Type: "int16"}}

	got, err := CoerceToColumnType("count", "42", columns)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := got.(int16); !ok {
		t.Errorf("expected int16, got %T (%v)", got, got)
	}
}

func TestCoerceToColumnType_NullableNilStaysNull(t *testing.T) {
	// A nil in a nullable column must bind as SQL NULL. Without the explicit
	// check this now performs, a nil in a nullable *timestamp* column fell
	// into the time case and became a zero time.Time -- storing an actual
	// timestamp of January 1 of year 1 instead of NULL.
	columns := []defs.DBColumn{
		{
			Name:     "when",
			Type:     "timestamp",
			Nullable: defs.BoolValue{Specified: true, Value: true},
		},
	}

	got, err := CoerceToColumnType("when", nil, columns)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got != nil {
		t.Errorf("expected nil for a nullable column, got %T (%v)", got, got)
	}
}

func TestCoerceToColumnType_NonNullableNilIsZeroTime(t *testing.T) {
	// A nil in a column that is not marked nullable keeps the previous
	// safety-net behavior of producing a typed zero value.
	got, err := CoerceToColumnType("when", nil, timeColumns())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	parsed, ok := got.(time.Time)
	if !ok {
		t.Fatalf("expected time.Time, got %T", got)
	}

	if !parsed.IsZero() {
		t.Errorf("expected the zero time, got %v", parsed)
	}
}
