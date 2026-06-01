// Package authserver implements the OAuth2/OIDC Authorization Server (AS) endpoints
// for Ego.  When enabled via ego.server.oauth.as.enabled, this package registers
// the following routes on the Ego HTTP router:
//
//	GET  /.well-known/openid-configuration  — OIDC discovery document
//	GET  /.well-known/jwks.json             — public signing key set
//	GET  /oauth2/authorize                  — show login form
//	POST /oauth2/authorize                  — process login, issue authorization code
//	POST /oauth2/token                      — token endpoint (all grant types)
//	GET  /oauth2/userinfo                   — OIDC UserInfo endpoint
//	POST /oauth2/revoke                     — token revocation (RFC 7009)
//
// All endpoints are registered without Ego user authentication (they handle their
// own authentication per the OAuth2 specification) and without Ego permissions.
package authserver

import (
	"fmt"
	"net/http"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/caches"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/router"
)

// RegisterRoutes initializes the OAuth2 Authorization Server and registers its
// HTTP routes on the given router.  It is called from commands/server.go when
// ego.server.oauth.as.enabled is true.
//
// Initialization steps:
//  1. Load configuration (settings → asConfig)
//  2. Load or generate the EC signing key pair
//  3. Load the client registry from the JSON file
//  4. Build the OIDC discovery document
//  5. Register all seven OAuth2/OIDC endpoints
//
// Returns an error if any initialization step fails (e.g., unreadable key file).
// In that case the routes are NOT registered and the caller logs the error.
func RegisterRoutes(r *router.Router) error {
	cfg := loadConfig()

	// Store the config in the package-level variable so all handlers can access it.
	asGlobalConfig = cfg

	if cfg.Issuer == "" {
		return errors.New(errors.ErrOAuthASMissingIssuer)
	}

	// Step 0: Ensure the OAuth2 working directory exists with the correct (0700)
	// permissions.  Abort startup if the directory cannot be created or secured —
	// the private signing key must never be readable by other users on the host.
	if _, err := ensureOAuthDir(); err != nil {
		return err
	}

	// Step 1: Load or generate the EC signing key.
	if err := loadOrGenerateKey(cfg.KeyFile); err != nil {
		return err
	}

	// Step 2: Load registered clients, then inject the built-in ego-cli public
	// client if it wasn't already defined in the client file.
	if err := loadClients(cfg.ClientFile); err != nil {
		return err
	}

	injectBuiltinCLIClient()

	// Step 3: Build the OIDC discovery document now that we know the issuer URL.
	if err := buildDiscoveryDoc(cfg.Issuer); err != nil {
		return errors.New(errors.ErrOAuthDiscoveryBuild).Context(err.Error())
	}

	// Set the OAuthCodeCache TTL to the configured code expiration.
	// This overrides the default cache TTL for authorization codes only.
	_ = caches.SetExpiration(caches.OAuthCodeCache,
		fmt.Sprintf("%.0fs", cfg.CodeExpiration.Seconds()))

	// Set the OAuthRefreshCache TTL to the configured refresh token expiration.
	_ = caches.SetExpiration(caches.OAuthRefreshCache,
		fmt.Sprintf("%.0fs", cfg.RefreshExpiration.Seconds()))

	// Register all OAuth2/OIDC endpoints.  None of these routes require Ego user
	// authentication — each handler validates credentials according to the OAuth2
	// specification using the client registry and Ego's user database.

	r.New(defs.OAuthDiscoveryPath, DiscoveryHandler, http.MethodGet).
		Class(router.ServiceRequestCounter)

	r.New(defs.OAuthJWKSPath, JWKSHandler, http.MethodGet).
		Class(router.ServiceRequestCounter)

	// The authorization endpoint receives standard OAuth2 query parameters.
	// Each one must be declared so the router's parameter validator accepts it.
	r.New(defs.OAuthAuthorizePath, AuthorizeGetHandler, http.MethodGet).
		Class(router.ServiceRequestCounter).
		Parameter("response_type", "string").
		Parameter("client_id", "string").
		Parameter("redirect_uri", "string").
		Parameter("scope", "string").
		Parameter("state", "string").
		Parameter("code_challenge", "string").
		Parameter("code_challenge_method", "string")

	r.New(defs.OAuthAuthorizePath, AuthorizePostHandler, http.MethodPost).
		Class(router.ServiceRequestCounter)

	// OAUTH-L2: the token endpoint must NOT declare AcceptMedia(JSONMediaType).
	//
	// AcceptMedia validates the request's Accept header, so declaring JSON here
	// caused the router to reject any token request whose Accept header was absent
	// or set to "application/x-www-form-urlencoded" — which is what RFC 6749
	// §4.1.3 actually requires clients to send.  Removing AcceptMedia means all
	// Content-Type and Accept headers are accepted, which is correct: the handler
	// reads its input via r.ParseForm() and writes its output as JSON regardless
	// of what the client declares in its Accept header.
	r.New(defs.OAuthTokenPath, TokenHandler, http.MethodPost).
		Class(router.ServiceRequestCounter)

	r.New(defs.OAuthUserinfoPath, UserinfoHandler, http.MethodGet).
		Class(router.ServiceRequestCounter).
		AcceptMedia(defs.JSONMediaType)

	r.New(defs.OAuthRevokePath, RevokeHandler, http.MethodPost).
		Class(router.ServiceRequestCounter)

	ui.Log(ui.ServerLogger, "oauth.as.startup", ui.A{"issuer": cfg.Issuer})

	return nil
}
