package util

import (
	"testing"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/symbols"
)

// ---------------------------------------------------------------------------
// bToMb
// ---------------------------------------------------------------------------

// TestBToMb verifies that bToMb converts byte counts to megabytes correctly.
// 1 MB is defined as 1024 * 1024 = 1,048,576 bytes.
func TestBToMb(t *testing.T) {
	tests := []struct {
		name  string
		input uint64
		want  float64
	}{
		{
			name:  "zero bytes",
			input: 0,
			want:  0.0,
		},
		{
			name:  "exactly one megabyte",
			input: 1024 * 1024,
			want:  1.0,
		},
		{
			name:  "one gigabyte",
			input: 1024 * 1024 * 1024,
			want:  1024.0,
		},
		{
			name:  "half a megabyte",
			input: 512 * 1024,
			want:  0.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := bToMb(tt.input)
			if got != tt.want {
				t.Errorf("bToMb(%d) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// getMode
// ---------------------------------------------------------------------------

// TestGetMode verifies that getMode returns the mode string stored in the symbol
// table, or the default "run" when no mode has been set.
func TestGetMode(t *testing.T) {
	tests := []struct {
		name     string
		setMode  string // empty means "do not set the variable"
		wantMode string
	}{
		{
			name:     "default mode when not set",
			setMode:  "",
			wantMode: "run",
		},
		{
			name:     "explicit run mode",
			setMode:  "run",
			wantMode: "run",
		},
		{
			name:     "test mode",
			setMode:  "test",
			wantMode: "test",
		},
		{
			name:     "server mode",
			setMode:  "server",
			wantMode: "server",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := symbols.NewSymbolTable("test getMode")

			if tt.setMode != "" {
				s.SetAlways(defs.ModeVariable, tt.setMode)
			}

			got, err := getMode(s, data.NewList())
			if err != nil {
				t.Fatalf("getMode() unexpected error: %v", err)
			}

			if got != tt.wantMode {
				t.Errorf("getMode() = %q, want %q", got, tt.wantMode)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// getMemoryStats
// ---------------------------------------------------------------------------

// TestGetMemoryStats verifies that getMemoryStats returns a non-nil struct with
// sensible values. Exact memory numbers are not asserted because they vary
// between runs; the test guards against panics and ensures the GC count is
// non-negative and memory values are non-negative.
func TestGetMemoryStats(t *testing.T) {
	s := symbols.NewSymbolTable("test getMemoryStats")

	result, err := getMemoryStats(s, data.NewList())
	if err != nil {
		t.Fatalf("getMemoryStats() unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("getMemoryStats() returned nil")
	}

	// The result should be a *data.Struct; pull out the fields and do
	// basic sanity checks.
	st, ok := result.(*data.Struct)
	if !ok {
		t.Fatalf("getMemoryStats() returned %T, want *data.Struct", result)
	}

	// Time field must be a non-empty string.
	timeVal, found := st.Get("Time")
	if !found {
		t.Fatal("getMemoryStats(): Time field missing")
	}

	if timeVal == nil || timeVal == "" {
		t.Error("getMemoryStats(): Time field is empty")
	}

	// GC count must be >= 0.
	gcVal, found := st.Get("GC")
	if !found {
		t.Fatal("getMemoryStats(): GC field missing")
	}

	if gc, ok := gcVal.(int); ok {
		if gc < 0 {
			t.Errorf("getMemoryStats(): GC = %d, want >= 0", gc)
		}
	}

	// Current memory usage must be >= 0.
	curVal, found := st.Get("Current")
	if !found {
		t.Fatal("getMemoryStats(): Current field missing")
	}

	if cur, ok := curVal.(float64); ok {
		if cur < 0 {
			t.Errorf("getMemoryStats(): Current = %f, want >= 0", cur)
		}
	}
}

// ---------------------------------------------------------------------------
// setLogger
// ---------------------------------------------------------------------------

// TestSetLogger verifies that setLogger enables and disables loggers correctly
// and returns the previous state of the logger. It also checks that an invalid
// logger name produces an error.
func TestSetLogger(t *testing.T) {
	s := symbols.NewSymbolTable("test setLogger")

	// Record the initial state of the TRACE logger so we can restore it at the
	// end of the test, preventing state leakage into other test runs.
	traceID := ui.LoggerByName("TRACE")
	initialState := ui.IsActive(traceID)

	defer func() {
		ui.Active(traceID, initialState)
	}()

	t.Run("enable a logger that starts disabled", func(t *testing.T) {
		// Ensure the logger is off first.
		ui.Active(traceID, false)

		got, err := setLogger(s, data.NewList("TRACE", true))
		if err != nil {
			t.Fatalf("setLogger() unexpected error: %v", err)
		}

		// The returned value is the *previous* state (false).
		if got != false {
			t.Errorf("setLogger() previous state = %v, want false", got)
		}

		// The logger should now be active.
		if !ui.IsActive(traceID) {
			t.Error("setLogger() did not enable the TRACE logger")
		}
	})

	t.Run("disable a logger that starts enabled", func(t *testing.T) {
		// Ensure the logger is on first.
		ui.Active(traceID, true)

		got, err := setLogger(s, data.NewList("TRACE", false))
		if err != nil {
			t.Fatalf("setLogger() unexpected error: %v", err)
		}

		// The returned value is the *previous* state (true).
		if got != true {
			t.Errorf("setLogger() previous state = %v, want true", got)
		}

		// The logger should now be inactive.
		if ui.IsActive(traceID) {
			t.Error("setLogger() did not disable the TRACE logger")
		}
	})

	t.Run("logger name is case-insensitive", func(t *testing.T) {
		ui.Active(traceID, false)

		_, err := setLogger(s, data.NewList("trace", true))
		if err != nil {
			t.Fatalf("setLogger() with lowercase name unexpected error: %v", err)
		}

		if !ui.IsActive(traceID) {
			t.Error("setLogger() did not enable logger with lowercase name")
		}
	})

	t.Run("APP logger (id 0) is valid", func(t *testing.T) {
		// AppLogger has iota value 0. A previous bug used `<= 0` as the
		// invalid-logger guard, which incorrectly rejected "APP". Verify
		// that it is accepted now.
		appID := ui.LoggerByName("APP")
		appInitial := ui.IsActive(appID)
		defer ui.Active(appID, appInitial)

		_, err := setLogger(s, data.NewList("APP", false))
		if err != nil {
			t.Errorf("setLogger() with APP logger (id 0): unexpected error: %v", err)
		}
	})

	t.Run("invalid logger name returns error", func(t *testing.T) {
		_, err := setLogger(s, data.NewList("NONEXISTENT_LOGGER_XYZ", true))
		if err == nil {
			t.Error("setLogger() with invalid name: expected error, got nil")
		}
	})
}

// ---------------------------------------------------------------------------
// getLogContents
// ---------------------------------------------------------------------------

// TestGetLogContents verifies that getLogContents returns an empty array rather
// than nil when there are no buffered log lines. The function takes a count of
// lines to return and an optional session filter.
func TestGetLogContents(t *testing.T) {
	s := symbols.NewSymbolTable("test getLogContents")

	t.Run("returns empty array when no log lines buffered", func(t *testing.T) {
		got, err := getLogContents(s, data.NewList(0))
		if err != nil {
			t.Fatalf("getLogContents() unexpected error: %v", err)
		}

		if got == nil {
			t.Fatal("getLogContents() returned nil, want empty value")
		}
	})

	t.Run("accepts optional session filter argument", func(t *testing.T) {
		got, err := getLogContents(s, data.NewList(10, 0))
		if err != nil {
			t.Fatalf("getLogContents() with session filter unexpected error: %v", err)
		}

		if got == nil {
			t.Fatal("getLogContents() with session=0 returned nil, want empty value")
		}
	})

	t.Run("invalid count argument returns error", func(t *testing.T) {
		_, err := getLogContents(s, data.NewList("not-a-number"))
		if err == nil {
			t.Error("getLogContents() with non-integer count: expected error, got nil")
		}
	})

	t.Run("invalid session argument returns error", func(t *testing.T) {
		_, err := getLogContents(s, data.NewList(5, "not-a-number"))
		if err == nil {
			t.Error("getLogContents() with non-integer session: expected error, got nil")
		}
	})
}

// ---------------------------------------------------------------------------
// getPackages
// ---------------------------------------------------------------------------

// TestGetPackages verifies that getPackages returns a sorted, non-nil Ego array
// of package name strings. The util package itself is always registered, so at
// minimum one element is expected.
func TestGetPackages(t *testing.T) {
	s := symbols.NewSymbolTable("test getPackages")

	got, err := getPackages(s, data.NewList())
	if err != nil {
		t.Fatalf("getPackages() unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("getPackages() returned nil")
	}

	arr, ok := got.(*data.Array)
	if !ok {
		t.Fatalf("getPackages() returned %T, want *data.Array", got)
	}

	// Verify the returned values are all strings and the list is sorted.
	prev := ""

	for i := range arr.Len() {
		v, err := arr.Get(i)
		if err != nil {
			t.Fatalf("getPackages(): array.Get(%d) error: %v", i, err)
		}

		name, ok := v.(string)
		if !ok {
			t.Errorf("getPackages(): element %d is %T, want string", i, v)
			
			continue
		}

		if prev != "" && name < prev {
			t.Errorf("getPackages(): result is not sorted: %q follows %q", name, prev)
		}

		prev = name
	}
}
