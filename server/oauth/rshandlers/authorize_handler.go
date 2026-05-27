package rshandlers

import (
	"net/http"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/router"
	"github.com/tucats/ego/server/oauth"
	"github.com/tucats/ego/util"
)

// AuthorizeRedirectHandler processes GET /services/admin/oauth/authorize.
//
// This endpoint is a convenience entry point for browser-based clients — for
// example, an "Sign in with [Your Organization]" button on the Ego dashboard.
// It performs the first step of the Authorization Code + PKCE flow: building the
// full IdP authorization URL and redirecting the browser to it.
//
// After the user authenticates with the IdP, the browser is redirected back to
// /services/admin/oauth/callback (CallbackHandler) with the authorization code.
//
// An optional "redirect" query parameter overrides the configured
// ego.server.oauth.redirect.uri for this specific request.  The override URI
// must be registered with the IdP or the token exchange will fail.
func AuthorizeRedirectHandler(session *router.Session, w http.ResponseWriter, r *http.Request) int {
	cfg := oauth.GetConfig()

	// Optional per-request redirect URI override.
	if override := r.URL.Query().Get("redirect"); override != "" {
		cfg.RedirectURI = override
	}

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

	// Build the PKCE authorization URL.
	authURL, state, _, err := oauth.BuildAuthorizeURL(cfg)
	if err != nil {
		ui.Log(ui.AuthLogger, "oauth.rs.authorize.failed", ui.A{
			"session": session.ID,
			"error":   err.Error(),
		})

		return util.ErrorResponse(w, session.ID,
			"failed to build authorization URL: "+err.Error(),
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
