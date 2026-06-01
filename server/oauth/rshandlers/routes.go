package rshandlers

import (
	"net/http"

	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/router"
)

// RegisterRoutes registers the OAuth2 Resource Server HTTP endpoints on the
// given router.  Called from commands/server.go when
// ego.server.oauth.provider is configured.
//
// Registered endpoints:
//
//	GET /services/admin/oauth/authorize  — redirect browser to IdP login page
//	GET /services/admin/oauth/callback   — receive IdP redirect, exchange code
//	GET /services/admin/oauth/config     — admin-only sanitized RS config view
func RegisterRoutes(r *router.Router) {
	// authorize and callback are entry points for the OAuth2 login flow.
	// They do not require prior Ego authentication.
	r.New(defs.ServicesOAuthAuthorizePath, AuthorizeRedirectHandler, http.MethodGet).
		Class(router.ServiceRequestCounter).
		Authentication(false, false)

	r.New(defs.ServicesOAuthCallbackPath, CallbackHandler, http.MethodGet).
		Class(router.ServiceRequestCounter).
		Authentication(false, false).
		Parameter("code", "string").
		Parameter("state", "string").
		Parameter("error", "string").
		Parameter("error_description", "string")

	// The config endpoint exposes configuration details and requires admin access.
	r.New(defs.ServicesOAuthConfigPath, ConfigHandler, http.MethodGet).
		Class(router.ServiceRequestCounter).
		Authentication(true, true).
		Permissions(defs.RootPermission)
}
