package authserver

import (
	"encoding/json"
	"net/http"

	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/i18n"
	"github.com/tucats/ego/router"
	"github.com/tucats/ego/util"
)

// oidcDiscovery describes the JSON structure of the OIDC Provider Metadata
// document served at /.well-known/openid-configuration (RFC 8414 §2).
//
// Clients fetch this document once to discover all AS endpoint URLs, the
// signing algorithms supported, and the claims the AS can provide.  By
// publishing a standard discovery document, any compliant OAuth2/OIDC client
// library can configure itself automatically from the issuer URL alone.
type oidcDiscovery struct {
	// Issuer is the canonical URL of this Authorization Server.
	// It must exactly match the "iss" claim in every JWT this AS issues.
	Issuer string `json:"issuer"`

	// AuthorizationEndpoint is the URL where clients begin the authorization
	// code flow (the login page).
	AuthorizationEndpoint string `json:"authorization_endpoint"`

	// TokenEndpoint is where clients exchange codes, refresh tokens, or
	// client credentials for access tokens.
	TokenEndpoint string `json:"token_endpoint"`

	// UserinfoEndpoint is the OIDC UserInfo endpoint (returns identity claims).
	UserinfoEndpoint string `json:"userinfo_endpoint"`

	// JWKsURI is the URL of the JSON Web Key Set — the public signing key(s).
	JWKsURI string `json:"jwks_uri"`

	// RevocationEndpoint is where clients can revoke an access or refresh token.
	RevocationEndpoint string `json:"revocation_endpoint"`

	// ResponseTypesSupported lists the response_type values this AS accepts.
	// "code" is the only value for the Authorization Code flow.
	ResponseTypesSupported []string `json:"response_types_supported"`

	// GrantTypesSupported lists the grant types this AS supports.
	GrantTypesSupported []string `json:"grant_types_supported"`

	// SubjectTypesSupported describes how the "sub" claim is calculated.
	// "public" means it is the same username for all clients (no pairwise pseudonyms).
	SubjectTypesSupported []string `json:"subject_types_supported"`

	// IDTokenSigningAlgValuesSupported names the signing algorithms used for ID tokens.
	IDTokenSigningAlgValuesSupported []string `json:"id_token_signing_alg_values_supported"`

	// ScopesSupported lists the scope values this AS understands.
	ScopesSupported []string `json:"scopes_supported"`

	// ClaimsSupported lists the JWT claims this AS may include.
	ClaimsSupported []string `json:"claims_supported"`

	// CodeChallengeMethodsSupported lists the PKCE methods this AS accepts.
	// Only "S256" is supported (not "plain", which offers no security benefit).
	CodeChallengeMethodsSupported []string `json:"code_challenge_methods_supported"`
}

// discoveryDoc is the pre-built discovery document, serialized to JSON once at
// startup and served verbatim on every discovery request.
var discoveryDoc []byte

// buildDiscoveryDoc constructs the OIDC discovery document for the given issuer
// URL and stores the JSON in discoveryDoc.  Called once by initialize().
func buildDiscoveryDoc(issuer string) error {
	doc := oidcDiscovery{
		Issuer:                issuer,
		AuthorizationEndpoint: issuer + defs.OAuthAuthorizePath,
		TokenEndpoint:         issuer + defs.OAuthTokenPath,
		UserinfoEndpoint:      issuer + defs.OAuthUserinfoPath,
		JWKsURI:               issuer + defs.OAuthJWKSPath,
		RevocationEndpoint:    issuer + defs.OAuthRevokePath,

		ResponseTypesSupported: []string{"code"},
		GrantTypesSupported: []string{
			"authorization_code",
			"client_credentials",
			"refresh_token",
		},
		SubjectTypesSupported:            []string{"public"},
		IDTokenSigningAlgValuesSupported: []string{"ES256"},
		ScopesSupported:                  []string{"openid", "profile", "email", "ego:read", "ego:write", "ego:admin", "ego:code"},
		ClaimsSupported:                  []string{"sub", "iss", "aud", "iat", "exp", "jti", "scope", "client_id", "name", "email"},
		CodeChallengeMethodsSupported:    []string{"S256"},
	}

	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}

	discoveryDoc = b

	return nil
}

// DiscoveryHandler serves the OIDC provider metadata document at
// GET /.well-known/openid-configuration.
//
// Clients fetch this document once at startup to learn all AS endpoint URLs.
// The response is a pre-built JSON blob (no per-request computation).
func DiscoveryHandler(session *router.Session, w http.ResponseWriter, r *http.Request) int {
	if discoveryDoc == nil {
		return util.ErrorResponse(w, session.ID,
			i18n.TLang(session.Language, "error.oauth.as.not.initialized"), http.StatusServiceUnavailable)
	}

	w.Header().Set(defs.ContentTypeHeader, defs.JSONMediaType)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(discoveryDoc)

	return http.StatusOK
}
