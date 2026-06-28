package rshandlers

import (
	"encoding/json"
	"net/http"

	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/router"
	"github.com/tucats/ego/internal/server/oauth"
)

// rsConfigView is the sanitized view of the RS configuration returned by
// ConfigHandler.  The client secret is always omitted.
type rsConfigView struct {
	// Enabled reports whether the RS role is active (provider is configured).
	Enabled bool `json:"enabled"`

	// Provider is the identity provider base URL.
	Provider string `json:"provider"`

	// ClientID is the application identifier assigned to Ego by the IdP.
	// Non-secret; safe to return.
	ClientID string `json:"client_id"`

	// Scopes is the space-delimited list of OAuth2 scopes requested during
	// Authorization Code flow.
	Scopes string `json:"scopes"`

	// RedirectURI is the callback URL registered with the IdP.
	RedirectURI string `json:"redirect_uri"`

	// UserClaim is the JWT claim name that Ego maps to the username.
	UserClaim string `json:"user_claim"`

	// PermissionClaim is the JWT claim name that Ego maps to permissions.
	PermissionClaim string `json:"permission_claim"`

	// Audience is the expected value of the JWT "aud" claim.  Empty means
	// audience validation is disabled.
	Audience string `json:"audience,omitempty"`

	// Mode is the integration mode: resource-server, proxy, or hybrid.
	Mode string `json:"mode"`

	// JWKSCacheTTL is the JWKS key cache lifetime as a duration string.
	JWKSCacheTTL string `json:"jwks_cache_ttl"`

	// PermissionMap is the configured scope-to-permission table.
	// An empty map means the built-in defaults are in use.
	PermissionMap map[string]string `json:"permission_map,omitempty"`
}

// ConfigHandler processes GET /services/admin/oauth/config.
//
// This diagnostic endpoint returns a sanitized view of the current OAuth2
// Resource Server configuration.  It is admin-only — the caller must present
// a valid Bearer token with the "ego.root" permission.
//
// The client secret is never included in the response, even for admin callers.
//
// Intended use: verifying that the server has picked up the correct provider URL,
// scopes, audience, and permission mapping without accessing the raw profile file.
func ConfigHandler(session *router.Session, w http.ResponseWriter, r *http.Request) int {
	cfg := oauth.GetConfig()

	view := rsConfigView{
		Enabled:         cfg.Provider != "",
		Provider:        cfg.Provider,
		ClientID:        cfg.ClientID,
		Scopes:          cfg.Scopes,
		RedirectURI:     cfg.RedirectURI,
		UserClaim:       cfg.UserClaim,
		PermissionClaim: cfg.PermissionClaim,
		Audience:        cfg.Audience,
		Mode:            cfg.Mode,
		JWKSCacheTTL:    cfg.JWKSCacheTTL.String(),
		PermissionMap:   cfg.PermissionMap,
	}

	w.Header().Set("Content-Type", defs.JSONMediaType)
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(view)

	return http.StatusOK
}
