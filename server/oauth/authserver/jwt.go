package authserver

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/ui"
)

// tokenResponse is the JSON body returned from the /oauth2/token endpoint.
// Field names follow the OAuth2 specification (RFC 6749 §5.1).
type tokenResponse struct {
	// AccessToken is the signed JWT that the client presents to Resource Servers.
	AccessToken string `json:"access_token"`

	// TokenType is always "Bearer" for OAuth2 access tokens.
	TokenType string `json:"token_type"`

	// ExpiresIn is the lifetime of the access token in seconds.
	ExpiresIn int64 `json:"expires_in"`

	// RefreshToken is present when the client is allowed the "refresh_token" grant
	// and is absent for client_credentials tokens (which should not be refreshed).
	RefreshToken string `json:"refresh_token,omitempty"`

	// IDToken is included when the "openid" scope was granted.  It is a signed JWT
	// carrying identity claims (sub, name, email) about the authenticated user.
	IDToken string `json:"id_token,omitempty"`

	// Scope is the space-delimited list of scopes that were actually granted,
	// which may be a subset of what was requested.
	Scope string `json:"scope"`
}

// egoClaims extends the standard JWT registered claims with the additional
// fields that Ego's Authorization Server includes in every access token.
type egoClaims struct {
	jwt.RegisteredClaims

	// Scope is the space-delimited list of granted OAuth2 scopes.
	Scope string `json:"scope,omitempty"`

	// ClientID identifies the OAuth2 client that requested the token.
	ClientID string `json:"client_id,omitempty"`
}

// idTokenClaims carries the identity information included in an OIDC ID token.
type idTokenClaims struct {
	jwt.RegisteredClaims

	// Name is the user's display name. Ego uses the username.
	Name string `json:"name,omitempty"`

	// Email is the user's email. Ego uses the username as a stand-in.
	Email string `json:"email,omitempty"`
}

// createAccessToken mints a new signed ES256 JWT access token.
// Parameters:
//   - cfg: resolved AS configuration (used for issuer and token lifetime)
//   - clientID: the OAuth2 client that requested the token
//   - subject: the authenticated username, or "" for client_credentials tokens
//   - audience: the intended audience (Resource Server) for the token
//   - scope: the space-delimited list of granted scopes
//
// The returned string is a compact-serialized JWT (three base64url-encoded
// segments separated by dots).
func createAccessToken(cfg asConfig, clientID, subject, audience, scope string) (string, string, error) {
	now := time.Now()
	jti := uuid.New().String() // unique token ID — used for revocation

	claims := egoClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    cfg.Issuer,
			Subject:   subject,
			Audience:  jwt.ClaimStrings{audience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(cfg.TokenExpiration)),
			ID:        jti,
		},
		Scope:    scope,
		ClientID: clientID,
	}

	// jwt.NewWithClaims selects the signing method (ES256) and attaches the claims.
	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)

	// SignedString uses signingKey (the EC private key loaded at startup) to produce
	// the final compact JWT string.
	signed, err := token.SignedString(signingKey)
	if err != nil {
		return "", "", fmt.Errorf("signing access token: %w", err)
	}

	ui.Log(ui.ServerLogger, "oauth.as.token.issued", ui.A{
		"client": clientID,
		"user":   subject,
		"jti":    jti,
		"scope":  scope,
	})

	return signed, jti, nil
}

// createIDToken mints an OIDC ID token for the given user.  An ID token is only
// issued when "openid" appears in the granted scope list.
//
// The ID token audience is set to the clientID (per OIDC Core §2) rather than
// the Resource Server audience used for access tokens.
func createIDToken(cfg asConfig, clientID, subject string) (string, error) {
	now := time.Now()

	claims := idTokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    cfg.Issuer,
			Subject:   subject,
			Audience:  jwt.ClaimStrings{clientID},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(cfg.TokenExpiration)),
			ID:        uuid.New().String(),
		},
		// Ego has no user profile store, so name and email are the username.
		Name:  subject,
		Email: subject,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)

	signed, err := token.SignedString(signingKey)
	if err != nil {
		return "", fmt.Errorf("signing ID token: %w", err)
	}

	return signed, nil
}

// parseToken validates a compact JWT string against signingKey's public key and
// returns the parsed claims on success.  The token must be signed with ES256.
//
// Returns an error if the token is expired, has a bad signature, or cannot
// be parsed.
func parseToken(tokenString string) (*egoClaims, error) {
	token, err := jwt.ParseWithClaims(
		tokenString,
		&egoClaims{},
		func(t *jwt.Token) (any, error) {
			// Guard against algorithm substitution attacks — require ES256.
			if _, ok := t.Method.(*jwt.SigningMethodECDSA); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}

			// Return the public key so the library can verify the signature.
			return &signingKey.PublicKey, nil
		},
	)
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*egoClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	return claims, nil
}
