// Package oauth implements the OAuth2/OIDC Resource Server (RS) client for Ego.
// When configured via ego.server.oauth.provider, Ego can:
//
//   - Accept JWT Bearer tokens from an external identity provider (IdP) and validate
//     them locally using the IdP's public signing keys (resource-server mode).
//
//   - Redirect browser users to the IdP's login page and, after successful
//     authentication, issue a native Ego token (proxy and hybrid modes).
//
//   - Accept both JWT Bearer tokens and native Ego tokens simultaneously (hybrid mode),
//     which is the default and incurs no backward-compatibility cost for existing clients.
//
// Three integration modes are supported via ego.server.oauth.mode:
//
//	resource-server  Accept JWT Bearer tokens only.  Clients obtain tokens from the IdP.
//	proxy            Redirect browser logins through OAuth2; issue Ego tokens on return.
//	hybrid           Accept both JWT and Ego tokens; also supports proxy login for browsers.
//
// For a detailed design walkthrough and administrator setup guide see docs/OAUTH.md.
package oauth

import (
	"os"
	"strings"
	"time"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
)

// Integration mode constants.  These are the accepted values for
// ego.server.oauth.mode.
const (
	// ModeResourceServer instructs Ego to accept JWT Bearer tokens directly.
	// Clients must obtain tokens from the IdP themselves.  Ego's /services/admin/logon
	// endpoint is not used for OAuth2 clients in this mode.
	ModeResourceServer = "resource-server"

	// ModeProxy instructs Ego to redirect browser logons through the IdP's
	// Authorization Code flow.  After the IdP authenticates the user, Ego issues a
	// native Ego token — existing clients need no changes beyond the login step.
	ModeProxy = "proxy"

	// ModeHybrid (default) accepts both JWT Bearer tokens (RS mode) and native Ego
	// tokens.  Proxy logon is also available for browser users.  This mode is the
	// lowest-risk integration path.
	ModeHybrid = "hybrid"
)

// Default values applied when the corresponding setting is absent or invalid.
const (
	// defaultUserClaim is the JWT claim used as the Ego username when
	// ego.server.oauth.user.claim is not set.  "sub" (subject) is the standard
	// OIDC field that uniquely identifies a user within a provider.
	defaultUserClaim = "sub"

	// defaultPermissionClaim is the JWT claim parsed for Ego permission mapping
	// when ego.server.oauth.permission.claim is not set.  "scope" is the standard
	// OAuth2 field containing a space-delimited list of granted scopes.
	defaultPermissionClaim = "scope"

	// defaultMode is the integration mode used when ego.server.oauth.mode is
	// absent, empty, or set to an unrecognized value.
	defaultMode = ModeHybrid

	// defaultJWKSCacheTTL is the JWKS key cache lifetime when
	// ego.server.oauth.jwks.cache.ttl is not set.
	defaultJWKSCacheTTL = 1 * time.Hour
)

// rsConfig holds the fully resolved configuration for the OAuth2 Resource Server
// client.  It is populated once by loadConfig() at Initialize() time and shared
// read-only by all subsequent calls.
type rsConfig struct {
	// Provider is the base URL of the identity provider.  Ego appends
	// /.well-known/openid-configuration to perform OIDC discovery.
	// Example: "https://dev-12345.okta.com/oauth2/default"
	Provider string

	// ClientID is the application identifier assigned to Ego by the IdP.
	// This is a non-secret public value.
	ClientID string

	// ClientSecret is the credential used to authenticate Ego to the IdP when
	// exchanging an authorization code for tokens.  May be overridden by the
	// EGO_OAUTH_CLIENT_SECRET environment variable.
	ClientSecret string

	// Scopes is the space-delimited list of OAuth2 scopes to request during
	// Authorization Code flow.  Must include "openid" for OIDC identity claims.
	Scopes string

	// RedirectURI is the callback URL registered with the IdP. Required for
	// Authorization Code flow; must exactly match what was registered.
	RedirectURI string

	// UserClaim is the JWT claim name that contains the Ego username.
	UserClaim string

	// PermissionClaim is the JWT claim name that contains the roles/scopes used
	// to derive Ego permissions.
	PermissionClaim string

	// Audience is the expected value of the JWT "aud" claim.  When empty,
	// audience validation is skipped (not recommended in production).
	Audience string

	// Mode is the integration mode: resource-server, proxy, or hybrid.
	Mode string

	// JWKSCacheTTL controls how long the IdP's public signing keys are cached.
	JWKSCacheTTL time.Duration

	// PermissionMap maps OAuth2 scope names to Ego permission names.
	// Built from ego.server.oauth.permission.map, or from the built-in defaults
	// when that setting is empty.
	PermissionMap map[string]string
}

// loadConfig reads all ego.server.oauth.* settings and returns a populated
// rsConfig.  Defaults are applied for any absent or invalid values.
//
// The client secret is read from the EGO_OAUTH_CLIENT_SECRET environment
// variable first (taking precedence), then from the ego.server.oauth.client.secret
// setting.  This allows sensitive credentials to be injected via the environment
// without appearing in the profile file.
//
// OAUTH-L3: when the secret is read from the environment variable, the variable
// is immediately cleared with os.Unsetenv so child processes (such as Ego programs
// run via exec functions) cannot inherit it.  A visible SERVER-log warning is also
// emitted so operators are aware that the credential was supplied via the
// environment, matching the LOGIN-L1 pattern for EGO_PASSWORD.
func loadConfig() rsConfig {
	// Read client secret from environment first, then from settings.
	clientSecret := settings.Get(defs.OAuthClientSecretSetting)

	if envSecret := os.Getenv("EGO_OAUTH_CLIENT_SECRET"); envSecret != "" {
		clientSecret = envSecret

		// Clear the env var so it is not visible to child processes spawned by
		// the server (e.g., via exec functions when ExecPermittedSetting is true).
		// The error return from Unsetenv is best-effort; there is no safe fallback
		// if the OS refuses to clear the variable.
		_ = os.Unsetenv("EGO_OAUTH_CLIENT_SECRET")

		// Emit a visible warning so operators know the credential came from the
		// environment rather than the encrypted profile file.
		ui.Log(ui.ServerLogger, "oauth.rs.client.secret.env", ui.A{})
	}

	// Parse the JWKS cache TTL; fall back to the default on any error.
	jwksCacheTTL := defaultJWKSCacheTTL

	if s := settings.Get(defs.OAuthJWKSCacheTTLSetting); s != "" {
		if d, err := time.ParseDuration(s); err == nil && d > 0 {
			jwksCacheTTL = d
		}
	}

	// Normalize the mode to lowercase and validate it.
	mode := strings.ToLower(settings.Get(defs.OAuthModeSetting))
	if mode != ModeResourceServer && mode != ModeProxy && mode != ModeHybrid {
		mode = defaultMode
	}

	// Apply claim-name defaults when the settings are absent.
	userClaim := settings.Get(defs.OAuthUserClaimSetting)
	if userClaim == "" {
		userClaim = defaultUserClaim
	}

	permissionClaim := settings.Get(defs.OAuthPermissionClaimSetting)
	if permissionClaim == "" {
		permissionClaim = defaultPermissionClaim
	}

	return rsConfig{
		Provider:        settings.Get(defs.OAuthProviderSetting),
		ClientID:        settings.Get(defs.OAuthClientIDSetting),
		ClientSecret:    clientSecret,
		Scopes:          settings.Get(defs.OAuthScopesSetting),
		RedirectURI:     settings.Get(defs.OAuthRedirectURISetting),
		UserClaim:       userClaim,
		PermissionClaim: permissionClaim,
		Audience:        settings.Get(defs.OAuthAudienceSetting),
		Mode:            mode,
		JWKSCacheTTL:    jwksCacheTTL,
		PermissionMap:   parsePermissionMap(settings.Get(defs.OAuthPermissionMapSetting)),
	}
}

// parsePermissionMap converts the comma-separated "scope=permission" string stored
// in ego.server.oauth.permission.map into a Go map.
//
// For example, the string "ego:admin=root,ego:write=tables,ego:read=logon" produces:
//
//	{"ego:admin": "root", "ego:write": "tables", "ego:read": "logon"}
//
// Any entry that is missing the "=" separator, or that has an empty key or value,
// is silently skipped.  Leading and trailing whitespace is trimmed from each part.
//
// When the setting string is empty, an empty (non-nil) map is returned; the caller
// should fall back to the built-in default scope-to-permission table in claims.go.
func parsePermissionMap(s string) map[string]string {
	result := make(map[string]string)

	for _, pair := range strings.Split(s, ",") {
		parts := strings.SplitN(strings.TrimSpace(pair), "=", 2)
		if len(parts) == 2 && strings.TrimSpace(parts[0]) != "" && strings.TrimSpace(parts[1]) != "" {
			result[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}

	return result
}
