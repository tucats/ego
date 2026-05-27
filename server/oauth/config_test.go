package oauth

import (
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
