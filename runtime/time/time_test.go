// Package time contains the Ego runtime shim for the Go standard "time" package.
// Functions here are of two kinds:
//
//  1. Native pass-through: the Ego runtime calls the real Go function directly,
//     with no wrapper code in this package (e.g. time.Now, time.Sleep). These
//     are registered in types.go with IsNative: true and a nil Value field for
//     the Go implementation.
//
//  2. Ego-implemented wrappers: functions that need Ego-specific behavior sit in
//     this package and receive a *symbols.SymbolTable plus a data.List of
//     arguments. The symbol table carries the method receiver (if any) under the
//     key defs.ThisVariable. Results that would be Go multi-return tuples are
//     bundled into a data.List so they fit the single-any return required by the
//     Ego runtime dispatch.
//
// Tests in this file exercise the Ego-implemented wrappers — not the native
// pass-throughs, which are covered by Go's own standard-library tests.
package time

import (
	"testing"
	"time"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/symbols"
)

// ---------------------------------------------------------------------------
// Helpers shared by multiple tests
// ---------------------------------------------------------------------------

// newSymbolTableWithReceiver creates a minimal symbol table and stores value
// as the method receiver (defs.ThisVariable). In Ego, every method call binds
// its receiver — the object the method is called on — to this special key
// before dispatching to the Go implementation.
func newSymbolTableWithReceiver(value any) *symbols.SymbolTable {
	st := symbols.NewSymbolTable("test")
	st.SetAlways(defs.ThisVariable, value)

	return st
}

// ---------------------------------------------------------------------------
// parseDuration tests
// ---------------------------------------------------------------------------

// parseDuration wraps util.ParseDuration (which extends Go's time.ParseDuration
// to accept a "d" unit for days) and returns its result as a data.List so the
// Ego runtime can unpack the (Duration, error) tuple.

func TestParseDuration_StandardGoFormat(t *testing.T) {
	// Strings without a "d" are passed straight through to time.ParseDuration.
	result, err := parseDuration(nil, data.NewList("1h30m"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The result is a data.List — element 0 is the duration, element 1 is the error.
	list, ok := result.(data.List)
	if !ok {
		t.Fatalf("expected data.List, got %T", result)
	}

	got, ok := list.Get(0).(time.Duration)
	if !ok {
		t.Fatalf("expected time.Duration at index 0, got %T", list.Get(0))
	}

	want := 1*time.Hour + 30*time.Minute
	if got != want {
		t.Errorf("parseDuration(\"1h30m\") = %v, want %v", got, want)
	}
}

func TestParseDuration_DaysOnly(t *testing.T) {
	result, err := parseDuration(nil, data.NewList("2d"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	list := result.(data.List)
	got := list.Get(0).(time.Duration)
	want := 48 * time.Hour

	if got != want {
		t.Errorf("parseDuration(\"2d\") = %v, want %v", got, want)
	}
}

func TestParseDuration_DaysHoursMinutes(t *testing.T) {
	result, err := parseDuration(nil, data.NewList("1d6h30m"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	list := result.(data.List)
	got := list.Get(0).(time.Duration)
	want := 24*time.Hour + 6*time.Hour + 30*time.Minute

	if got != want {
		t.Errorf("parseDuration(\"1d6h30m\") = %v, want %v", got, want)
	}
}

func TestParseDuration_DaysWithMilliseconds(t *testing.T) {
	// "ms" suffix for milliseconds must be correctly distinguished from "m" (minutes)
	// followed by "s" (seconds). The parser uses a look-ahead flag (mSeen) to tell
	// "5ms" from "5m0s".
	result, err := parseDuration(nil, data.NewList("1d30ms"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	list := result.(data.List)
	got := list.Get(0).(time.Duration)
	want := 24*time.Hour + 30*time.Millisecond

	if got != want {
		t.Errorf("parseDuration(\"1d30ms\") = %v, want %v", got, want)
	}
}

func TestParseDuration_DaysWithSeconds(t *testing.T) {
	result, err := parseDuration(nil, data.NewList("1d45s"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	list := result.(data.List)
	got := list.Get(0).(time.Duration)
	want := 24*time.Hour + 45*time.Second

	if got != want {
		t.Errorf("parseDuration(\"1d45s\") = %v, want %v", got, want)
	}
}

func TestParseDuration_DaysWithSpaces(t *testing.T) {
	// The Ego extension allows spaces between units: "1d 6h 30m".
	result, err := parseDuration(nil, data.NewList("1d 6h 30m"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	list := result.(data.List)
	got := list.Get(0).(time.Duration)
	want := 24*time.Hour + 6*time.Hour + 30*time.Minute

	if got != want {
		t.Errorf("parseDuration(\"1d 6h 30m\") = %v, want %v", got, want)
	}
}

func TestParseDuration_InvalidUnit(t *testing.T) {
	// "q" is not a recognised duration unit; the parser must return an error.
	// Because there is no "d" in the string, time.ParseDuration handles this
	// and returns its own error.
	result, err := parseDuration(nil, data.NewList("3q"))
	if err == nil {
		t.Fatalf("expected error for invalid unit, got result: %v", result)
	}
}

func TestParseDuration_InvalidUnitWithDays(t *testing.T) {
	// When the string contains "d", the extended parser is used. An unknown
	// unit character after a day component must still be rejected.
	result, err := parseDuration(nil, data.NewList("1d3q"))
	if err == nil {
		t.Fatalf("expected error for \"1d3q\", got result: %v", result)
	}
}

func TestParseDuration_EmptyString(t *testing.T) {
	// An empty (or whitespace-only) string is invalid; the standard parser
	// returns an error, and the Ego wrapper must propagate it.
	result, err := parseDuration(nil, data.NewList(""))
	if err == nil {
		t.Fatalf("expected error for empty string, got result: %v", result)
	}
}

// The following two tests are regression tests for a bug in parseDurationWithDays
// where the accumulated digit buffer was not flushed as minutes when 'm' was
// followed immediately by 'd' or 'h' (another named-unit letter). Before the fix,
// "30md" silently re-used the "30" for the 'd' unit (yielding 30 days), instead of
// recording it as 30 minutes and giving 'd' an implicit count of 0.

func TestParseDuration_MinutesBeforeDays(t *testing.T) {
	// "30md" — 30 minutes, then 'd' with no preceding number (0 days).
	// Expected: 30 minutes total.
	result, err := parseDuration(nil, data.NewList("30md"))
	if err != nil {
		t.Fatalf("parseDuration(\"30md\") unexpected error: %v", err)
	}

	list := result.(data.List)
	got := list.Get(0).(time.Duration)
	want := 30 * time.Minute

	if got != want {
		t.Errorf("parseDuration(\"30md\") = %v, want %v", got, want)
	}
}

func TestParseDuration_MinutesBeforeHours(t *testing.T) {
	// "1d30mh" — the extended parser is active (because 'd' is present), so the
	// "mh" portion tests the same fix: "30" before 'm' must be recorded as minutes,
	// and 'h' with no preceding digit contributes 0 hours.
	// Expected: 1 day + 30 minutes = 24h30m.
	result, err := parseDuration(nil, data.NewList("1d30mh"))
	if err != nil {
		t.Fatalf("parseDuration(\"1d30mh\") unexpected error: %v", err)
	}

	list := result.(data.List)
	got := list.Get(0).(time.Duration)
	want := 24*time.Hour + 30*time.Minute

	if got != want {
		t.Errorf("parseDuration(\"1d30mh\") = %v, want %v", got, want)
	}
}

// TestParseDuration_TableDriven exercises additional edge cases using the table
// pattern, which keeps each case self-documenting while sharing the assertion logic.
func TestParseDuration_TableDriven(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    time.Duration
		wantErr bool
	}{
		{
			name:  "full day+hour+minute+second+ms",
			input: "1d2h3m4s5ms",
			want:  24*time.Hour + 2*time.Hour + 3*time.Minute + 4*time.Second + 5*time.Millisecond,
		},
		{
			name:  "standard sub-second (no days path)",
			input: "500ms",
			want:  500 * time.Millisecond,
		},
		{
			name:  "standard hours+minutes (no days path)",
			input: "1h30m",
			want:  time.Hour + 30*time.Minute,
		},
		{
			name:    "whitespace only — invalid",
			input:   "   ",
			wantErr: true,
		},
		{
			name:    "bogus string with days",
			input:   "1dbogus",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := parseDuration(nil, data.NewList(tc.input))
			if tc.wantErr {
				if err == nil {
					t.Errorf("parseDuration(%q): expected error, got nil (result=%v)", tc.input, result)
				}

				return
			}

			if err != nil {
				t.Fatalf("parseDuration(%q): unexpected error: %v", tc.input, err)
			}

			list, ok := result.(data.List)
			if !ok {
				t.Fatalf("parseDuration(%q): result is %T, want data.List", tc.input, result)
			}

			got, ok := list.Get(0).(time.Duration)
			if !ok {
				t.Fatalf("parseDuration(%q): list[0] is %T, want time.Duration", tc.input, list.Get(0))
			}

			if got != tc.want {
				t.Errorf("parseDuration(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// durationString tests
// ---------------------------------------------------------------------------

// durationString implements the Duration.String() method for Ego. The optional
// boolean argument selects "extended" formatting (spaces between units, days
// expressed separately) vs. the default compact Go format ("1h30m0s").

func TestDurationString_NoReceiver(t *testing.T) {
	// When no receiver is bound in the symbol table, durationString must return
	// ErrNoFunctionReceiver. An empty symbol table simulates a bare function call
	// that bypassed the normal method-dispatch mechanism.
	st := symbols.NewSymbolTable("test")
	_, err := durationString(st, data.NewList())

	if err == nil {
		t.Fatal("expected ErrNoFunctionReceiver, got nil")
	}
}

func TestDurationString_DefaultFormat(t *testing.T) {
	// Calling with no arguments uses the compact Go format.
	d := time.Hour + 30*time.Minute
	st := newSymbolTableWithReceiver(d)

	result, err := durationString(st, data.NewList())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Go's time.Duration.String() for 1h30m is "1h30m0s".
	want := d.String() // "1h30m0s"
	if result != want {
		t.Errorf("durationString (no args) = %q, want %q", result, want)
	}
}

func TestDurationString_ExplicitFalse(t *testing.T) {
	// Passing extendedFormat=false explicitly must give the same compact output as the default.
	d := time.Hour + 30*time.Minute
	st := newSymbolTableWithReceiver(d)

	result, err := durationString(st, data.NewList(false))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := d.String()
	if result != want {
		t.Errorf("durationString (false) = %q, want %q", result, want)
	}
}

func TestDurationString_ExtendedFormat(t *testing.T) {
	// With extendedFormat=true, the output is human-readable with spaces and
	// a "d" for days (e.g. "1h 30m" instead of "1h30m0s").
	d := time.Hour + 30*time.Minute
	st := newSymbolTableWithReceiver(d)

	result, err := durationString(st, data.NewList(true))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "1h 30m"
	if result != want {
		t.Errorf("durationString (true) = %q, want %q", result, want)
	}
}

func TestDurationString_ExtendedFormat_DaysAndHours(t *testing.T) {
	// Extended format must break hours >= 24 into days + hours.
	d := 25 * time.Hour
	st := newSymbolTableWithReceiver(d)

	result, err := durationString(st, data.NewList(true))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "1d 1h"
	if result != want {
		t.Errorf("durationString (25h, extended) = %q, want %q", result, want)
	}
}

func TestDurationString_ExtendedFormat_SubSecond(t *testing.T) {
	// Durations shorter than one second fall through to the default Go formatter
	// even when extendedFormat is true — the extended formatter only engages for
	// durations >= 1s.
	d := 300 * time.Millisecond
	st := newSymbolTableWithReceiver(d)

	result, err := durationString(st, data.NewList(true))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Falls back to Go default for sub-second values.
	want := "300ms"
	if result != want {
		t.Errorf("durationString (300ms, extended) = %q, want %q", result, want)
	}
}

func TestDurationString_ExtendedFormat_Zero(t *testing.T) {
	d := time.Duration(0)
	st := newSymbolTableWithReceiver(d)

	result, err := durationString(st, data.NewList(true))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "0s"
	if result != want {
		t.Errorf("durationString (0, extended) = %q, want %q", result, want)
	}
}

func TestDurationString_PtrReceiver(t *testing.T) {
	// getDuration accepts both time.Duration and *time.Duration as the receiver.
	d := time.Hour + 30*time.Minute
	st := newSymbolTableWithReceiver(&d)

	result, err := durationString(st, data.NewList(false))
	if err != nil {
		t.Fatalf("unexpected error for *time.Duration receiver: %v", err)
	}

	want := d.String()
	if result != want {
		t.Errorf("durationString (*time.Duration) = %q, want %q", result, want)
	}
}

// ---------------------------------------------------------------------------
// Parse tests
// ---------------------------------------------------------------------------

// Parse wraps the third-party dateparse.ParseAny function, which accepts a
// wide variety of common date/time string layouts and returns a time.Time.
// The Ego wrapper bundles the (time.Time, error) tuple into a data.List.

func TestParse_ValidISO8601Date(t *testing.T) {
	result, err := Parse(nil, data.NewList("2024-01-15"))
	if err != nil {
		t.Fatalf("Parse(\"2024-01-15\") unexpected error: %v", err)
	}

	list, ok := result.(data.List)
	if !ok {
		t.Fatalf("expected data.List, got %T", result)
	}

	if list.Get(1) != nil {
		t.Errorf("expected nil error in list[1], got %v", list.Get(1))
	}

	got, ok := list.Get(0).(time.Time)
	if !ok {
		t.Fatalf("expected time.Time at list[0], got %T", list.Get(0))
	}

	if got.Year() != 2024 || got.Month() != time.January || got.Day() != 15 {
		t.Errorf("Parse(\"2024-01-15\") = %v, want year=2024 month=January day=15", got)
	}
}

func TestParse_ValidISO8601DateTime(t *testing.T) {
	result, err := Parse(nil, data.NewList("2024-06-01 10:30:00"))
	if err != nil {
		t.Fatalf("Parse(\"2024-06-01 10:30:00\") unexpected error: %v", err)
	}

	list := result.(data.List)
	got := list.Get(0).(time.Time)

	if got.Year() != 2024 || got.Month() != time.June || got.Day() != 1 {
		t.Errorf("date mismatch: got %v", got)
	}

	if got.Hour() != 10 || got.Minute() != 30 {
		t.Errorf("time mismatch: got %v", got)
	}
}

func TestParse_USLongDateFormat(t *testing.T) {
	// dateparse.ParseAny recognises US-style "January 2, 2006" layouts.
	result, err := Parse(nil, data.NewList("January 15, 2024"))
	if err != nil {
		t.Fatalf("Parse(\"January 15, 2024\") unexpected error: %v", err)
	}

	list := result.(data.List)
	got := list.Get(0).(time.Time)

	if got.Year() != 2024 || got.Month() != time.January || got.Day() != 15 {
		t.Errorf("Parse(\"January 15, 2024\") = %v, want year=2024 month=January day=15", got)
	}
}

func TestParse_InvalidDateString(t *testing.T) {
	// dateparse.ParseAny must fail on an unparseable string; the wrapper must
	// propagate that error and return a non-nil error value.
	result, err := Parse(nil, data.NewList("not a date at all"))
	if err == nil {
		t.Fatalf("expected error for unparseable string, got %v", result)
	}

	// The wrapper should also store the error in list[1] so Ego code can
	// inspect it via the second return value.
	list, ok := result.(data.List)
	if !ok {
		t.Fatalf("expected data.List on error path, got %T", result)
	}

	if list.Get(1) == nil {
		t.Error("list[1] should hold the error but is nil")
	}

	if list.Get(0) != nil {
		t.Errorf("list[0] should be nil on error path, got %v", list.Get(0))
	}
}

func TestParse_EmptyString(t *testing.T) {
	// An empty string has no recognisable date layout.
	_, err := Parse(nil, data.NewList(""))
	if err == nil {
		t.Fatal("expected error for empty string, got nil")
	}
}

// ---------------------------------------------------------------------------
// String (time method) tests
// ---------------------------------------------------------------------------

// String formats the time receiver using time.UnixDate layout, e.g.
// "Mon Jan  2 15:04:05 MST 2006". It is the Ego implementation of the
// time.Time.String() method.

func TestTimeString_TimeValue(t *testing.T) {
	// Pass a bare time.Time as the receiver; String must format it correctly.
	fixed := time.Date(2024, time.March, 10, 14, 30, 0, 0, time.UTC)
	st := newSymbolTableWithReceiver(fixed)

	result, err := String(st, data.NewList())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := fixed.Format(time.UnixDate)
	if result != want {
		t.Errorf("String() = %q, want %q", result, want)
	}
}

func TestTimeString_TimePtrValue(t *testing.T) {
	// getTime also accepts *time.Time as the receiver.
	fixed := time.Date(2024, time.March, 10, 14, 30, 0, 0, time.UTC)
	st := newSymbolTableWithReceiver(&fixed)

	result, err := String(st, data.NewList())
	if err != nil {
		t.Fatalf("unexpected error with *time.Time receiver: %v", err)
	}

	want := fixed.Format(time.UnixDate)
	if result != want {
		t.Errorf("String() (*time.Time) = %q, want %q", result, want)
	}
}

func TestTimeString_NoReceiver(t *testing.T) {
	// An empty symbol table means no receiver was bound; String must return an error.
	st := symbols.NewSymbolTable("test")

	_, err := String(st, data.NewList())
	if err == nil {
		t.Fatal("expected error when no receiver is set, got nil")
	}
}

func TestTimeString_OutputParsesBack(t *testing.T) {
	// End-to-end sanity check: the formatted string must round-trip through
	// time.Parse with the same layout to recover the original time.
	fixed := time.Date(2024, time.July, 4, 8, 0, 0, 0, time.UTC)
	st := newSymbolTableWithReceiver(fixed)

	result, err := String(st, data.NewList())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	str, ok := result.(string)
	if !ok {
		t.Fatalf("expected string, got %T", result)
	}

	parsed, err := time.Parse(time.UnixDate, str)
	if err != nil {
		t.Fatalf("output %q cannot be parsed back with time.UnixDate: %v", str, err)
	}

	if !parsed.Equal(fixed) {
		t.Errorf("round-trip mismatch: got %v, want %v", parsed, fixed)
	}
}
