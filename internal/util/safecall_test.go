package util

// Regression tests for the NILPTR-6 fix (util.SafeCall) and for the
// "enabled unless explicitly disabled" reading of ego.server.panic.recovery.

import (
	"testing"

	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/defs"
)

// withRecoverySetting sets ego.server.panic.recovery for the duration of a test
// and restores the previous state afterwards, including the case where the key
// was not present at all -- which is a distinct state from "present but empty"
// as far as PanicRecoveryEnabled is concerned.
func withRecoverySetting(t *testing.T, value string, unset bool) {
	t.Helper()

	existed := settings.Exists(defs.ServerPanicRecoverySetting)
	saved := settings.Get(defs.ServerPanicRecoverySetting)

	if unset {
		settings.DeleteDefault(defs.ServerPanicRecoverySetting)
	} else {
		settings.SetDefault(defs.ServerPanicRecoverySetting, value)
	}

	t.Cleanup(func() {
		if existed {
			settings.SetDefault(defs.ServerPanicRecoverySetting, saved)
		} else {
			settings.DeleteDefault(defs.ServerPanicRecoverySetting)
		}
	})
}

// TestPanicRecoveryEnabledDefaultsOn is the important one for deployments that
// already exist. settings.GetBool reports false for a key that was never
// configured, so reading the setting with GetBool alone would silently disable
// recovery for every configuration file written before the setting existed.
// PanicRecoveryEnabled must treat "absent" as enabled.
func TestPanicRecoveryEnabledDefaultsOn_NILPTR6(t *testing.T) {
	withRecoverySetting(t, "", true)

	if settings.Exists(defs.ServerPanicRecoverySetting) {
		t.Fatal("test setup failed: setting should be absent")
	}

	if !PanicRecoveryEnabled() {
		t.Error("PanicRecoveryEnabled() = false with the setting absent, want true (recovery is on by default)")
	}
}

// TestPanicRecoveryEnabledRespectsExplicitFalse confirms the developer-facing
// escape hatch actually turns recovery off.
func TestPanicRecoveryEnabledRespectsExplicitFalse_NILPTR6(t *testing.T) {
	withRecoverySetting(t, defs.False, false)

	if PanicRecoveryEnabled() {
		t.Error("PanicRecoveryEnabled() = true with the setting explicitly false, want false")
	}
}

// TestSafeCallRecoversPanic confirms a panicking task is contained and reported
// as a failed call rather than taking the process down.
func TestSafeCallRecoversPanic_NILPTR6(t *testing.T) {
	withRecoverySetting(t, defs.True, false)

	if completed := SafeCall("nil map write", func() {
		var broken map[string]string

		broken["key"] = "value"
	}); completed {
		t.Error("SafeCall reported completed = true for a task that panicked")
	}
}

// TestSafeCallReportsSuccess confirms the healthy path is untouched: the task
// runs, and SafeCall reports that it finished.
func TestSafeCallReportsSuccess_NILPTR6(t *testing.T) {
	withRecoverySetting(t, defs.True, false)

	ran := false

	if completed := SafeCall("healthy task", func() { ran = true }); !completed {
		t.Error("SafeCall reported completed = false for a task that returned normally")
	}

	if !ran {
		t.Error("SafeCall did not run the task")
	}
}

// TestSafeCallLoopSurvivesPanic is the behavior the background goroutines
// actually depend on: one bad iteration must not end the loop. Before the fix
// these loops called the task directly, so a single panic terminated the whole
// process -- not just the loop -- because Go does not recover panics in
// goroutines other than the one net/http created for a request.
func TestSafeCallLoopSurvivesPanic_NILPTR6(t *testing.T) {
	withRecoverySetting(t, defs.True, false)

	iterations := 0

	for i := 0; i < 4; i++ {
		SafeCall("alternating task", func() {
			iterations++

			// Panic on the odd-numbered passes only.
			if iterations%2 == 1 {
				var broken map[string]string

				broken["key"] = "value"
			}
		})
	}

	if iterations != 4 {
		t.Errorf("loop ran %d iterations, want 4; a panic ended the loop early", iterations)
	}
}

// TestSafeCallPropagatesWhenDisabled confirms the setting is honored here too,
// so a developer can still get the original crash and stack trace.
func TestSafeCallPropagatesWhenDisabled_NILPTR6(t *testing.T) {
	withRecoverySetting(t, defs.False, false)

	recovered := func() (value any) {
		defer func() {
			value = recover()
		}()

		SafeCall("nil map write", func() {
			var broken map[string]string

			broken["key"] = "value"
		})

		return nil
	}()

	if recovered == nil {
		t.Error("SafeCall swallowed the panic even though ego.server.panic.recovery is false")
	}
}
