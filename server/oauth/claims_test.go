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
			expected:        []string{"ego.logon"},
		},
		{
			name:            "ego:admin grants root",
			scope:           "openid ego:admin",
			permissionClaim: "scope",
			permissionMap:   nil,
			expected:        []string{"ego.logon", "ego.root"},
		},
		{
			name:            "ego:write grants tables",
			scope:           "openid ego:write",
			permissionClaim: "scope",
			permissionMap:   nil,
			expected:        []string{"ego.logon", "ego.tables.write"},
		},
		{
			name:            "ego:code grants code_run",
			scope:           "openid ego:code",
			permissionClaim: "scope",
			permissionMap:   nil,
			expected:        []string{"ego.logon", "ego.code"},
		},
		{
			name:            "all standard scopes",
			scope:           "openid ego:read ego:write ego:admin ego:code",
			permissionClaim: "scope",
			permissionMap:   nil,
			expected:        []string{"ego.logon", "ego.tables.read", "ego.tables.write", "ego.root", "ego.code"},
		},
		{
			name:            "unknown scope falls back to logon",
			scope:           "unknown_scope",
			permissionClaim: "scope",
			permissionMap:   nil,
			expected:        []string{"ego.logon"},
		},
		{
			name:            "empty scope falls back to logon",
			scope:           "",
			permissionClaim: "scope",
			permissionMap:   nil,
			expected:        []string{"ego.logon"},
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
			expected:        []string{"ego.logon"},
		},
		{
			name:            "roles claim with matching role",
			roles:           []string{"ego:admin"},
			permissionClaim: "roles",
			permissionMap:   nil,
			expected:        []string{"ego.root"},
		},
		{
			name:            "roles claim with unknown role",
			roles:           []string{"unknown_role"},
			permissionClaim: "roles",
			permissionMap:   nil,
			expected:        []string{"ego.logon"},
		},
		{
			name:            "unrecognized permission claim returns logon",
			scope:           "openid",
			permissionClaim: "custom_claim",
			permissionMap:   nil,
			expected:        []string{"ego.logon"},
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

// TestIsKnownPermissionClaim verifies that IsKnownPermissionClaim returns true
// only for the two natively-supported claim names ("scope" and "roles"), and
// false for everything else (OAUTH-M8).
//
// The return value of IsKnownPermissionClaim drives the startup warning emitted
// in commands/server.go: when it returns false, the operator is warned that
// ego.server.oauth.permission.claim is set to a name that extractPermissionTokens
// cannot read, so JWT holders will silently receive only the minimum ego.logon
// permission regardless of what the token actually carries.
func TestIsKnownPermissionClaim(t *testing.T) {
	tests := []struct {
		claim string
		want  bool
	}{
		// The two natively supported names.
		{"scope", true},
		{"roles", true},

		// Everything else — these are all real claim names used by various IdPs
		// that Ego currently cannot read.
		{"groups", false},
		{"authorities", false},
		{"realm_access.roles", false},
		{"custom_claim", false},
		{"", false},

		// Near-misses — case differences must not match.
		{"Scope", false},
		{"SCOPE", false},
		{"Roles", false},
		{"ROLES", false},
	}

	for _, tt := range tests {
		t.Run(tt.claim, func(t *testing.T) {
			got := IsKnownPermissionClaim(tt.claim)
			if got != tt.want {
				t.Errorf("IsKnownPermissionClaim(%q) = %v, want %v", tt.claim, got, tt.want)
			}
		})
	}
}

// TestIsKnownPermissionClaim_FallbackBehavior verifies the end-to-end consequence
// of an unsupported claim name: mapClaimsToPermissions falls back to "ego.logon"
// for all JWT holders, even when the token carries scopes that would normally map
// to elevated permissions (OAUTH-M8).
//
// This test demonstrates WHY the startup warning matters — the silent fallback
// can either grant access to users who should be blocked (if the IdP issues no
// meaningful scopes anyway) or silently drop elevated permissions that were
// intended (if the IdP puts them in a claim name Ego does not read).
func TestIsKnownPermissionClaim_FallbackBehavior(t *testing.T) {
	claims := &jwtClaims{
		// The token carries ego:admin in the "scope" claim and "ego:admin" in
		// a hypothetical "groups" claim.  In a correct configuration (scope),
		// the user would receive ego.root.  With the wrong claim name, they
		// only receive ego.logon.
		Scope: "openid ego:admin",
		Roles: []string{"ego:admin"},
	}

	// When the permission claim is a known name, elevated permission is granted.
	permsWithScope := mapClaimsToPermissions(claims, "scope", nil)

	foundRootFromScope := false
	for _, p := range permsWithScope {
		if p == "ego.root" {
			foundRootFromScope = true
		}
	}

	if !foundRootFromScope {
		t.Errorf("scope claim: expected ego.root in permissions, got %v", permsWithScope)
	}

	// When the permission claim is an unsupported name, only ego.logon is granted
	// regardless of the token's actual content.
	permsWithCustom := mapClaimsToPermissions(claims, "groups", nil)

	if len(permsWithCustom) != 1 || permsWithCustom[0] != "ego.logon" {
		t.Errorf("unsupported claim %q: expected [ego.logon], got %v", "groups", permsWithCustom)
	}

	// Confirm that IsKnownPermissionClaim correctly identifies "scope" as known
	// and "groups" as unknown, matching the runtime behavior shown above.
	if !IsKnownPermissionClaim("scope") {
		t.Error("IsKnownPermissionClaim(\"scope\") should return true")
	}

	if IsKnownPermissionClaim("groups") {
		t.Error("IsKnownPermissionClaim(\"groups\") should return false")
	}
}
