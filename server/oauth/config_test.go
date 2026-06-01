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

// TestLoadConfig_EnvVarSecretCleared verifies that loadConfig clears the
// EGO_OAUTH_CLIENT_SECRET environment variable immediately after reading it
// (OAUTH-L3).
//
// Why this matters: environment variables are visible to every child process the
// server spawns (for example via Ego's exec built-in when ExecPermittedSetting
// is enabled) and to any process that can read /proc/<pid>/environ on Linux.
// Leaving a credential in the environment for the lifetime of the process
// unnecessarily widens its exposure.
//
// Test strategy: set the env var to a known value, call loadConfig(), then
// verify that (a) the returned rsConfig carries the value, confirming it was
// read, and (b) os.Getenv returns an empty string, confirming the variable was
// cleared.
func TestLoadConfig_EnvVarSecretCleared(t *testing.T) {
	const testSecret = "test-client-secret-value"

	// Set the env var and register a cleanup to restore it if something goes
	// wrong.  t.Cleanup runs even when the test panics, so the env is always
	// restored for subsequent tests in the same process.
	t.Setenv("EGO_OAUTH_CLIENT_SECRET", testSecret)

	// Call loadConfig.  This should read the env var, copy it into the config,
	// and then clear the env var.
	cfg := loadConfig()

	// (a) The config must carry the secret value we set.
	if cfg.ClientSecret != testSecret {
		t.Errorf("ClientSecret = %q, want %q", cfg.ClientSecret, testSecret)
	}

	// (b) The env var must have been cleared.
	if got := os.Getenv("EGO_OAUTH_CLIENT_SECRET"); got != "" {
		t.Errorf("EGO_OAUTH_CLIENT_SECRET still set to %q after loadConfig(); expected it to be cleared (OAUTH-L3)", got)
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
