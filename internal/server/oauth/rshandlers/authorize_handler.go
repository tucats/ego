package rshandlers

import (
	"net/http"

	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/router"
	"github.com/tucats/ego/internal/server/oauth"
	"github.com/tucats/ego/internal/util"
)

// AuthorizeRedirectHandler processes GET /services/admin/oauth/authorize.
//
// This endpoint is a convenience entry point for browser-based clients — for
// example, a "Sign in with [Your Organization]" button on the Ego dashboard.
// It performs the first step of the Authorization Code + PKCE flow: building
// the full IdP authorization URL (using the server-configured redirect URI)
// and redirecting the browser to it.
//
// After the user authenticates with the IdP, the browser is redirected back
// to /services/admin/oauth/callback (CallbackHandler) with the authorization
// code.
//
// The redirect URI used in the authorization request is always the value
// configured in ego.server.oauth.redirect.uri.  No per-request override is
// accepted.  Allowing callers to substitute an arbitrary URI would constitute
// an open-redirect vulnerability (OAUTH-H5), because the IdP would send the
// user — and the authorization code — to a potentially attacker-controlled
// address.
func AuthorizeRedirectHandler(session *router.Session, w http.ResponseWriter, r *http.Request) int {
	cfg := oauth.GetConfig()

	if cfg.Provider == "" {
		return util.ErrorResponse(w, session.ID,
			"ego.server.oauth.provider is not configured",
			http.StatusServiceUnavailable)
	}

	if cfg.RedirectURI == "" {
		return util.ErrorResponse(w, session.ID,
			"ego.server.oauth.redirect.uri must be configured for Authorization Code flow",
			http.StatusInternalServerError)
	}

	// Build the PKCE authorization URL using the server-configured redirect URI.
	authURL, state, _, err := oauth.BuildAuthorizeURL(cfg)
	if err != nil {
		ui.Log(ui.AuthLogger, "oauth.rs.authorize.failed", ui.A{
			"session": session.ID,
			"error":   err.Error(),
		})

		return util.ErrorResponse(w, session.ID,
			"failed to build authorization URL",
			http.StatusInternalServerError)
	}

	ui.Log(ui.AuthLogger, "oauth.rs.authorize.redirect", ui.A{
		"session":  session.ID,
		"provider": cfg.Provider,
		"state":    state,
	})

	http.Redirect(w, r, authURL, http.StatusFound)

	return http.StatusFound
}
