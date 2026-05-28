package oauth

import (
	"strings"
)

// defaultScopePermissions is the built-in mapping from OAuth2 scope names to Ego
// permission names.  It is used when ego.server.oauth.permission.map is not set.
//
// The mapping reflects the scope names that Ego's own Authorization Server (Phase 1)
// assigns when issuing JWTs, so that a single-instance setup (Ego-as-AS + Ego-as-RS)
// works without any extra configuration.
//
// Administrators can override any or all of these mappings via
// ego.server.oauth.permission.map.
var defaultScopePermissions = map[string]string{
	// openid is the mandatory OIDC scope; all authenticated users get logon.
	"openid": "ego.logon",

	// ego:read maps to the logon permission — the user can call the server.
	"ego:read": "ego.tables.read",

	// ego:write maps to the tables permission — the user can read and write tables.
	"ego:write": "ego.tables.write",

	// ego:admin maps to the root permission — full administrative access.
	"ego:admin": "ego.root",

	// ego:code maps to the code_run permission — the user can run Ego code via /admin/run.
	"ego:code": "ego.code",
}

// mapClaimsToPermissions derives the Ego permission list from the JWT claims
// extracted by parseAndValidateJWT.
//
// The function reads the "permission claim" from the JWT (by default "scope",
// overridable via ego.server.oauth.permission.claim) and maps each scope/role
// token to an Ego permission using the configured mapping table.
//
// The "scope" claim uses space-separated tokens (RFC 6749).
// The "roles" claim (used by some providers) uses a JSON string array.
// If the configured permission claim name is neither "scope" nor "roles",
// its value is treated as a space-separated string for compatibility.
//
// Rules:
//  1. If the administrator provided a permission map via
//     ego.server.oauth.permission.map, only that map is used.
//  2. If no administrator map is set (empty map), the built-in
//     defaultScopePermissions table is used.
//  3. Ego permission names are always lowercase; duplicates are removed.
//  4. If no scopes map to any known permission, "logon" is granted to any
//     successfully verified JWT holder — the token is cryptographically valid
//     even if its scopes are unrecognized.
func mapClaimsToPermissions(claims *jwtClaims, permissionClaim string, permissionMap map[string]string) []string {
	var permissions []string

	// Choose the mapping table: administrator override or built-in defaults.
	table := permissionMap
	if len(table) == 0 {
		table = defaultScopePermissions
	}

	// Collect the scope/role tokens from the JWT.
	tokens := extractPermissionTokens(claims, permissionClaim)

	// Map tokens to Ego permissions, deduplicating as we go.
	seen := make(map[string]bool)

	for _, tok := range tokens {
		if perm, ok := table[tok]; ok {
			lower := strings.ToLower(perm)
			if !seen[lower] {
				seen[lower] = true

				permissions = append(permissions, lower)
			}
		}
	}

	// Grant "logon" to any verified JWT holder even if no scope maps to a
	// known permission.  This ensures the token is not silently rejected by
	// the resource-server path in router/auth.go.
	if len(permissions) == 0 {
		permissions = []string{"ego.logon"}
	}

	return permissions
}

// extractPermissionTokens reads the appropriate field from the JWT claims and
// returns a flat slice of individual permission-token strings.
//
// The "scope" claim is space-separated (RFC 6749); the "roles" claim is a JSON
// array stored as []string.  Any other claim name is treated as space-separated.
// The "email" and "preferred_username" claims are never used as permission tokens.
func extractPermissionTokens(claims *jwtClaims, permissionClaim string) []string {
	switch permissionClaim {
	case "scope":
		return strings.Fields(claims.Scope)

	case "roles":
		// Some IdPs (e.g., Keycloak) provide roles as a flat string array in a
		// top-level "roles" claim.
		return claims.Roles

	default:
		// For any other claim name, look it up in the raw RegisteredClaims extra
		// space.  Since jwtClaims embeds RegisteredClaims (not MapClaims), we handle
		// a limited set of well-known custom claim names here.  Providers that use
		// truly custom claim names should set ego.server.oauth.permission.claim to
		// "scope" and configure their IdP to include the ego:* scopes.
		return []string{}
	}
}
