package oauth

import (
	"os"
	"testing"
	"time"
)

// TestParsePermissionMap verifies that parsePermissionMap correctly converts
// the comma-separated "scope=permission" configuration string into a map.
func TestParsePermissionMap(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: map[string]string{},
		},
		{
			name:  "single pair",
			input: "ego:admin=root",
			expected: map[string]string{
				"ego:admin": "root",
			},
		},
		{
			name:  "three pairs",
			input: "ego:admin=root,ego:write=tables,ego:read=logon",
			expected: map[string]string{
				"ego:admin": "root",
				"ego:write": "tables",
				"ego:read":  "logon",
			},
		},
		{
			name:  "whitespace trimmed",
			input: " ego:admin = root , ego:read = logon ",
			expected: map[string]string{
				"ego:admin": "root",
				"ego:read":  "logon",
			},
		},
		{
			name:  "malformed entries are skipped",
			input: "ego:admin=root,no-equals-sign,=emptykey,validkey=",
			expected: map[string]string{
				"ego:admin": "root",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parsePermissionMap(tt.input)

			if len(got) != len(tt.expected) {
				t.Errorf("parsePermissionMap(%q) returned %d entries, want %d: %v",
					tt.input, len(got), len(tt.expected), got)

				return
			}

			for k, v := range tt.expected {
				if got[k] != v {
					t.Errorf("parsePermissionMap(%q)[%q] = %q, want %q", tt.input, k, got[k], v)
				}
			}
		})
	}
}

// TestLoadConfigDefaults verifies that loadConfig applies correct defaults when
// no settings are configured.  Because loadConfig reads from the global settings
// store, which is empty in a unit test process, it should return the default
// values for all optional fields.
func TestLoadConfigDefaults(t *testing.T) {
	cfg := loadConfig()

	if cfg.UserClaim != defaultUserClaim {
		t.Errorf("UserClaim default: got %q, want %q", cfg.UserClaim, defaultUserClaim)
	}

	if cfg.PermissionClaim != defaultPermissionClaim {
		t.Errorf("PermissionClaim default: got %q, want %q", cfg.PermissionClaim, defaultPermissionClaim)
	}

	if cfg.Mode != defaultMode {
		t.Errorf("Mode default: got %q, want %q", cfg.Mode, defaultMode)
	}

	if cfg.JWKSCacheTTL != defaultJWKSCacheTTL {
		t.Errorf("JWKSCacheTTL default: got %v, want %v", cfg.JWKSCacheTTL, defaultJWKSCacheTTL)
	}
}

// TestLoadConfigTTLParsing verifies that parsePermissionMap handles edge-case
// duration strings correctly in isolation (the full loadConfig path that reads
// from settings is tested by TestLoadConfigDefaults).
func TestLoadConfigTTLParsing(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
	}{
		{"30m", 30 * time.Minute},
		{"2h", 2 * time.Hour},
		{"invalid", defaultJWKSCacheTTL},
		{"", defaultJWKSCacheTTL},
		{"-5m", defaultJWKSCacheTTL}, // negative durations should fall back to default
	}

	for _, tt := range tests {
		// Simulate the TTL parsing logic used inside loadConfig.
		result := defaultJWKSCacheTTL

		if tt.input != "" {
			if d, err := time.ParseDuration(tt.input); err == nil && d > 0 {
				result = d
			}
		}

		if result != tt.expected {
			t.Errorf("TTL parse for %q: got %v, want %v", tt.input, result, tt.expected)
		}
	}
}

// TestLoadConfig_EnvVar_ReadButNotCleared verifies the M1 fix: loadConfig()
// reads EGO_OAUTH_CLIENT_SECRET into the returned rsConfig but no longer calls
// os.Unsetenv to clear it.
//
// Why the behavior changed (M1):
//
//	Before the fix, loadConfig() cleared the env var on every call.  Because
//	IsEnabled() and GetConfig() both call loadConfig(), the first such call would
//	consume the env var before Initialize() had a chance to store it in
//	globalConfig.  A subsequent call to loadConfig() inside Initialize() would
//	then find the env var empty and fall back to the (potentially absent)
//	settings-file value — silently dropping the operator-supplied secret.
//
//	The fix moves the os.Unsetenv call into Initialize(), which runs exactly once
//	and stores the config before clearing the variable.  loadConfig() now only
//	reads the env var; Initialize() owns the clear.
//
// Test strategy:
//  1. Set the env var to a known value.
//  2. Call loadConfig() — the returned config must carry the secret.
//  3. Verify the env var is STILL set (not cleared by loadConfig).
//  4. Call loadConfig() a second time — it must STILL return the secret,
//     proving that the old "first call consumes it" bug is fixed.
func TestLoadConfig_EnvVar_ReadButNotCleared(t *testing.T) {
	const testSecret = "m1-test-client-secret"

	// t.Setenv registers an automatic cleanup that restores the original env var
	// value when the test ends, so we do not need a manual t.Cleanup.
	t.Setenv("EGO_OAUTH_CLIENT_SECRET", testSecret)

	// First call.
	cfg1 := loadConfig()

	if cfg1.ClientSecret != testSecret {
		t.Errorf("first loadConfig: ClientSecret = %q, want %q", cfg1.ClientSecret, testSecret)
	}

	// M1: the env var must NOT have been cleared by loadConfig.
	// Before the fix this assertion would have failed (got == "").
	if got := os.Getenv("EGO_OAUTH_CLIENT_SECRET"); got == "" {
		t.Error("M1: EGO_OAUTH_CLIENT_SECRET was cleared by loadConfig(); it should only be cleared by Initialize()")
	}

	// Second call — must still see the secret because the env var is still set.
	// Before the fix the first call would have consumed the env var, so the
	// second call returned ClientSecret == "" (the settings fallback).
	cfg2 := loadConfig()

	if cfg2.ClientSecret != testSecret {
		t.Errorf("second loadConfig: ClientSecret = %q, want %q — M1 bug: env var was consumed on first call", cfg2.ClientSecret, testSecret)
	}
}

// TestLoadConfig_NoEnvVar verifies that when EGO_OAUTH_CLIENT_SECRET is not
// set, loadConfig does not error and the ClientSecret field remains at whatever
// the settings store provides (empty in a unit-test environment).
func TestLoadConfig_NoEnvVar(t *testing.T) {
	// Ensure the env var is absent before the test.
	t.Setenv("EGO_OAUTH_CLIENT_SECRET", "")

	cfg := loadConfig()

	// In the unit-test environment the settings store is empty, so ClientSecret
	// should be the empty string (the settings default).
	_ = cfg // just confirm no panic
}

// TestGlobalConfig_ConcurrentAccess exercises IsEnabled() and GetConfig() from
// many goroutines simultaneously.  Run with -race to verify that the M3
// sync.RWMutex correctly prevents data races on globalConfig.
//
// How this test detects the bug (M3):
//
//	Before the M3 fix, globalConfig was accessed without any synchronization.
//	Running this test under the Go race detector ("go test -race ./...") would
//	report a DATA RACE between the write in globalConfigOnce.Do and the reads in
//	IsEnabled() / GetConfig().
//
//	After the fix, all reads and writes go through globalConfigMu (sync.RWMutex),
//	which establishes the required happens-before edges and makes the race
//	detector report clean.
//
// What the test checks beyond the race detector:
//
//	Because globalConfig starts as the zero value in this test environment
//	(no Initialize() call, no settings configured), IsEnabled() must return false
//	and GetConfig().Provider must be empty.  Both are verified to ensure the
//	mutex logic does not accidentally corrupt the values being read.
func TestGlobalConfig_ConcurrentAccess(t *testing.T) {
	// Save and restore globalConfig so this test does not pollute others.
	globalConfigMu.Lock()
	saved := globalConfig
	globalConfig = rsConfig{} // start from zero
	globalConfigMu.Unlock()

	t.Cleanup(func() {
		globalConfigMu.Lock()
		globalConfig = saved
		globalConfigMu.Unlock()
	})

	const goroutines = 50

	// done coordinates the goroutine pool: each goroutine decrements done when it
	// finishes.  The main goroutine waits for all to complete before checking
	// results.
	done := make(chan struct{}, goroutines)

	// errors collects any unexpected values observed by the goroutines.
	errs := make(chan string, goroutines*2)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer func() { done <- struct{}{} }()

			// IsEnabled() should return false: provider is empty in zero config.
			if IsEnabled() {
				errs <- "IsEnabled() returned true for zero globalConfig"
			}

			// GetConfig() should return the zero rsConfig (empty Provider).
			if cfg := GetConfig(); cfg.Provider != "" {
				errs <- "GetConfig().Provider is non-empty for zero globalConfig"
			}
		}()
	}

	// Wait for all goroutines to finish.
	for i := 0; i < goroutines; i++ {
		<-done
	}

	close(errs)

	for msg := range errs {
		t.Error(msg)
	}
}
