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

// TestIsKnownUserClaim verifies that IsKnownUserClaim returns true only for
// the three natively supported user claim names (OAUTH-L5).
//
// The return value drives the startup warning emitted by commands/server.go:
// when it returns false, the operator is warned that ego.server.oauth.user.claim
// is set to a name that extractUsername cannot read.  Without the warning, the
// server silently falls back to claims.Subject ("sub"), which is usually an
// opaque UUID from the IdP — causing Ego usernames to appear as UUIDs in audit
// logs and breaking any username-based access policy.
func TestIsKnownUserClaim(t *testing.T) {
	tests := []struct {
		claim string
		want  bool
	}{
		// The three natively supported names.
		{"sub", true},
		{"email", true},
		{"preferred_username", true},

		// Real IdP-specific claim names that are NOT supported — operators who
		// configure these will silently get UUID-based usernames.
		{"upn", false},               // Azure AD / Microsoft Entra
		{"login", false},             // GitHub
		{"unique_name", false},       // ADFS
		{"username", false},          // generic
		{"nickname", false},          // OIDC optional claim
		{"phone_number", false},      // OIDC optional claim
		{"", false},                  // empty string

		// Case sensitivity — "Sub" and "Email" are not supported.
		{"Sub", false},
		{"Email", false},
		{"Preferred_Username", false},
	}

	for _, tt := range tests {
		t.Run(tt.claim, func(t *testing.T) {
			got := IsKnownUserClaim(tt.claim)
			if got != tt.want {
				t.Errorf("IsKnownUserClaim(%q) = %v, want %v", tt.claim, got, tt.want)
			}
		})
	}
}

// TestIsKnownUserClaim_FallbackBehavior verifies the end-to-end consequence of
// an unsupported user claim name: extractUsername returns claims.Subject ("sub")
// regardless of what other claims are present (OAUTH-L5).
//
// This test documents WHY the startup warning matters.  The silent fallback
// means that:
//   - A token carrying a human-readable "login" claim still produces a UUID
//     username (from "sub"), making audit logs harder to read.
//   - Username-based access policies configured for "alice" or "bob" have no
//     effect because the server assigns UUID strings as usernames instead.
func TestIsKnownUserClaim_FallbackBehavior(t *testing.T) {
	// Build a realistic claims object with both a "sub" UUID and a human-readable
	// email address.
	claims := &jwtClaims{
		Email:             "alice@example.com",
		PreferredUsername: "alice",
	}
	// Set Subject directly (it comes from RegisteredClaims.Subject in real JWTs).
	claims.Subject = "550e8400-e29b-41d4-a716-446655440000" // a UUID

	// With a known claim name, the human-readable value is used.
	gotEmail := extractUsername(claims, "email")
	if gotEmail != "alice@example.com" {
		t.Errorf("extractUsername(\"email\") = %q, want %q", gotEmail, "alice@example.com")
	}

	gotPreferred := extractUsername(claims, "preferred_username")
	if gotPreferred != "alice" {
		t.Errorf("extractUsername(\"preferred_username\") = %q, want %q", gotPreferred, "alice")
	}

	// With an unsupported claim name, the UUID subject is returned — the
	// human-readable values in the token are silently ignored.
	for _, unsupported := range []string{"login", "upn", "nickname", "custom_claim"} {
		got := extractUsername(claims, unsupported)
		if got != claims.Subject {
			t.Errorf("extractUsername(%q) = %q, want Subject %q (OAUTH-L5 fallback)",
				unsupported, got, claims.Subject)
		}

		// IsKnownUserClaim must return false for each of these.
		if IsKnownUserClaim(unsupported) {
			t.Errorf("IsKnownUserClaim(%q) = true, want false", unsupported)
		}
	}
}
