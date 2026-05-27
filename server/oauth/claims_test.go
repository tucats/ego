package oauth

import (
	"sort"
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

// sortedEqual returns true when two string slices contain exactly the same
// elements regardless of order.  It sorts copies so the originals are unchanged.
func sortedEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	ac := make([]string, len(a))
	bc := make([]string, len(b))
	copy(ac, a)
	copy(bc, b)
	sort.Strings(ac)
	sort.Strings(bc)

	for i := range ac {
		if ac[i] != bc[i] {
			return false
		}
	}

	return true
}

// TestMapClaimsToPermissions verifies that OAuth2 scopes are translated into
// Ego permission names using the built-in mapping table.
func TestMapClaimsToPermissions(t *testing.T) {
	tests := []struct {
		name            string
		scope           string
		roles           []string
		permissionClaim string
		permissionMap   map[string]string
		expected        []string
	}{
		{
			name:            "openid scope grants logon",
			scope:           "openid",
			permissionClaim: "scope",
			permissionMap:   nil,
			expected:        []string{"logon"},
		},
		{
			name:            "ego:admin grants root",
			scope:           "openid ego:admin",
			permissionClaim: "scope",
			permissionMap:   nil,
			expected:        []string{"logon", "root"},
		},
		{
			name:            "ego:write grants tables",
			scope:           "openid ego:write",
			permissionClaim: "scope",
			permissionMap:   nil,
			expected:        []string{"logon", "tables"},
		},
		{
			name:            "ego:code grants code_run",
			scope:           "openid ego:code",
			permissionClaim: "scope",
			permissionMap:   nil,
			expected:        []string{"logon", "code_run"},
		},
		{
			name:            "ego:read duplicates logon but only one entry",
			scope:           "openid ego:read",
			permissionClaim: "scope",
			permissionMap:   nil,
			expected:        []string{"logon"},
		},
		{
			name:            "all standard scopes",
			scope:           "openid ego:read ego:write ego:admin ego:code",
			permissionClaim: "scope",
			permissionMap:   nil,
			expected:        []string{"logon", "tables", "root", "code_run"},
		},
		{
			name:            "unknown scope falls back to logon",
			scope:           "unknown_scope",
			permissionClaim: "scope",
			permissionMap:   nil,
			expected:        []string{"logon"},
		},
		{
			name:            "empty scope falls back to logon",
			scope:           "",
			permissionClaim: "scope",
			permissionMap:   nil,
			expected:        []string{"logon"},
		},
		{
			name:            "custom permission map overrides defaults",
			scope:           "myrole",
			permissionClaim: "scope",
			permissionMap:   map[string]string{"myrole": "tables"},
			expected:        []string{"tables"},
		},
		{
			name:            "custom map unknown scope falls back to logon",
			scope:           "otherrole",
			permissionClaim: "scope",
			permissionMap:   map[string]string{"myrole": "tables"},
			expected:        []string{"logon"},
		},
		{
			name:            "roles claim with matching role",
			roles:           []string{"ego:admin"},
			permissionClaim: "roles",
			permissionMap:   nil,
			expected:        []string{"root"},
		},
		{
			name:            "roles claim with unknown role",
			roles:           []string{"unknown_role"},
			permissionClaim: "roles",
			permissionMap:   nil,
			expected:        []string{"logon"},
		},
		{
			name:            "unrecognized permission claim returns logon",
			scope:           "openid",
			permissionClaim: "custom_claim",
			permissionMap:   nil,
			expected:        []string{"logon"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims := &jwtClaims{
				RegisteredClaims: jwt.RegisteredClaims{},
				Scope:            tt.scope,
				Roles:            tt.roles,
			}

			got := mapClaimsToPermissions(claims, tt.permissionClaim, tt.permissionMap)

			if !sortedEqual(got, tt.expected) {
				t.Errorf("mapClaimsToPermissions() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestExtractPermissionTokens verifies that scope and role values are split
// correctly for the different claim types.
func TestExtractPermissionTokens(t *testing.T) {
	tests := []struct {
		name            string
		scope           string
		roles           []string
		permissionClaim string
		expected        []string
	}{
		{
			name:            "scope with multiple scopes",
			scope:           "openid profile email",
			permissionClaim: "scope",
			expected:        []string{"openid", "profile", "email"},
		},
		{
			name:            "scope with single scope",
			scope:           "openid",
			permissionClaim: "scope",
			expected:        []string{"openid"},
		},
		{
			name:            "empty scope returns empty slice",
			scope:           "",
			permissionClaim: "scope",
			expected:        []string{},
		},
		{
			name:            "roles claim returns slice directly",
			roles:           []string{"admin", "user"},
			permissionClaim: "roles",
			expected:        []string{"admin", "user"},
		},
		{
			name:            "nil roles returns nil",
			roles:           nil,
			permissionClaim: "roles",
			expected:        nil,
		},
		{
			name:            "unknown claim returns empty slice",
			scope:           "openid",
			permissionClaim: "custom",
			expected:        []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims := &jwtClaims{
				Scope: tt.scope,
				Roles: tt.roles,
			}

			got := extractPermissionTokens(claims, tt.permissionClaim)

			// Normalize nil vs empty for comparison.
			if len(got) == 0 && len(tt.expected) == 0 {
				return
			}

			if !sortedEqual(got, tt.expected) {
				t.Errorf("extractPermissionTokens(%q) = %v, want %v",
					tt.permissionClaim, got, tt.expected)
			}
		})
	}
}
