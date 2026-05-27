// Package authserver implements the OAuth2 Authorization Server (AS) role for Ego.
// When enabled via the ego.server.oauth.as.enabled setting, an Ego server exposes
// standard OIDC/OAuth2 endpoints that any compliant client library can use.
//
// This is intended for development and testing environments only — not production.
// For production deployments, use a dedicated identity provider such as Keycloak,
// Auth0, or Okta.
//
// Only Phase 1 is implemented here: Ego as an Authorization Server. Phase 2
// (Ego as a Resource Server that accepts JWTs from an external IdP) is future work.
package authserver

import (
	"path/filepath"
	"time"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/defs"
)

// Default lifetimes used when the corresponding setting is absent or unparseable.
const (
	// defaultTokenExpiration is the lifetime of access tokens when not overridden.
	defaultTokenExpiration = 1 * time.Hour

	// defaultRefreshExpiration is the lifetime of refresh tokens when not overridden.
	defaultRefreshExpiration = 24 * time.Hour

	// defaultCodeExpiration is the lifetime of authorization codes when not overridden.
	defaultCodeExpiration = 5 * time.Minute

	// defaultKeyFileName is the file name used for the EC signing key when the
	// setting ego.server.oauth.as.key.file is absent. It is placed in EGO_PATH.
	defaultKeyFileName = "oauth-signing.pem"

	// defaultClientFileName is the file name used for the client registry when
	// ego.server.oauth.as.clients is absent. It is placed in EGO_PATH.
	defaultClientFileName = "oauth-clients.json"
)

// asConfig holds the resolved configuration for the Authorization Server.
// It is populated once at startup by loadConfig() and shared by all handlers.
type asConfig struct {
	// Enabled is true when ego.server.oauth.as.enabled is set.
	Enabled bool

	// Issuer is the base URL reported in the "iss" JWT claim and the OIDC
	// discovery document. Example: "https://ego.example.com".
	// Must match the publicly reachable URL of this server.
	Issuer string

	// KeyFile is the absolute path to the PEM file holding the EC private
	// signing key. The public key is derived from it at startup.
	KeyFile string

	// ClientFile is the absolute path to the JSON file containing the
	// registered OAuth2 client definitions.
	ClientFile string

	// TokenExpiration is the lifetime of access tokens issued by this AS.
	TokenExpiration time.Duration

	// RefreshExpiration is the lifetime of refresh tokens.
	RefreshExpiration time.Duration

	// CodeExpiration is the lifetime of short-lived authorization codes.
	CodeExpiration time.Duration
}

// loadConfig reads all ego.server.oauth.as.* settings and returns a populated
// asConfig. Defaults are applied for any missing or malformed duration values.
// The EGO_PATH is used to construct default file paths when the corresponding
// setting is absent.
func loadConfig() asConfig {
	egoPath := settings.Get(defs.EgoPathSetting)

	// Resolve the signing key file path.
	keyFile := settings.Get(defs.OAuthASKeyFileSetting)
	if keyFile == "" {
		keyFile = filepath.Join(egoPath, defaultKeyFileName)
	}

	// Resolve the client registry file path.
	clientFile := settings.Get(defs.OAuthASClientFileSetting)
	if clientFile == "" {
		clientFile = filepath.Join(egoPath, defaultClientFileName)
	}

	// Parse the token lifetime.  Fall back to the default on any error.
	tokenExpiration := defaultTokenExpiration
	if s := settings.Get(defs.OAuthASTokenExpirationSetting); s != "" {
		if d, err := time.ParseDuration(s); err == nil && d > 0 {
			tokenExpiration = d
		}
	}

	// Parse the refresh-token lifetime.
	refreshExpiration := defaultRefreshExpiration
	if s := settings.Get(defs.OAuthASRefreshExpirationSetting); s != "" {
		if d, err := time.ParseDuration(s); err == nil && d > 0 {
			refreshExpiration = d
		}
	}

	// Parse the authorization-code lifetime.
	codeExpiration := defaultCodeExpiration
	if s := settings.Get(defs.OAuthASCodeExpirationSetting); s != "" {
		if d, err := time.ParseDuration(s); err == nil && d > 0 {
			codeExpiration = d
		}
	}

	return asConfig{
		Enabled:           settings.GetBool(defs.OAuthASEnabledSetting),
		Issuer:            settings.Get(defs.OAuthASIssuerSetting),
		KeyFile:           keyFile,
		ClientFile:        clientFile,
		TokenExpiration:   tokenExpiration,
		RefreshExpiration: refreshExpiration,
		CodeExpiration:    codeExpiration,
	}
}
