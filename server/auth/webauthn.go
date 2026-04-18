package auth

import (
	"encoding/json"
	"net"
	"net/http"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/defs"
)

// webAuthnUser wraps a defs.User and implements the webauthn.User interface
// required by the go-webauthn library for both registration and login ceremonies.
type webAuthnUser struct {
	user defs.User
}

// WebAuthnID returns the opaque user handle — we use the raw bytes of the UUID.
func (w webAuthnUser) WebAuthnID() []byte {
	b, _ := w.user.ID.MarshalBinary()

	return b
}

// WebAuthnName returns the account name used in the authenticator UI.
func (w webAuthnUser) WebAuthnName() string { return w.user.Name }

// WebAuthnDisplayName returns the human-readable name shown by the authenticator.
func (w webAuthnUser) WebAuthnDisplayName() string { return w.user.Name }

// WebAuthnCredentials returns the slice of passkey credentials stored for this
// user.  The credentials are persisted as a JSON array in the defs.User.Passkeys
// RawMessage field.
func (w webAuthnUser) WebAuthnCredentials() []webauthn.Credential {
	if len(w.user.Passkeys) == 0 {
		return nil
	}

	var creds []webauthn.Credential

	_ = json.Unmarshal(w.user.Passkeys, &creds)

	return creds
}

// NewWebAuthn creates a configured *webauthn.WebAuthn instance using the RPID
// stored in the server profile (ego.server.webauthn.rpid).  Returns nil and an
// error when the RPID is not configured or the library rejects the config.
//
// When the RPID is "localhost", both http://localhost and https://localhost are
// accepted as origins so that local development works without a TLS certificate.
// In production the browser enforces HTTPS anyway.
func NewWebAuthn() (*webauthn.WebAuthn, error) {
	return NewWebAuthnForRequest(nil)
}

// NewWebAuthnForRequest creates a *webauthn.WebAuthn instance.  When
// ego.server.webauthn.rpid is configured that value is used as the RPID.
// Otherwise the RPID and allowed origin are derived from the incoming HTTP
// request's Host header (and TLS state), enabling zero-config local
// development with Apple passkeys or any platform authenticator.
//
// The port, if present in the Host header, is stripped for the RPID (WebAuthn
// spec §13.4.9) but kept in the allowed origin so the library's exact-match
// check passes for servers running on non-standard ports (e.g. localhost:8080).
func NewWebAuthnForRequest(r *http.Request) (*webauthn.WebAuthn, error) {
	rpid := settings.Get(defs.WebAuthnRPIDSetting)

	var origins []string

	if rpid == "" && r != nil {
		// Auto-derive from the incoming request.
		host := r.Host
		if host == "" {
			host = r.Header.Get("X-Forwarded-Host")
		}

		// Strip port for the RPID; keep full host:port for the origin.
		hostOnly := host
		if h, _, err := net.SplitHostPort(host); err == nil {
			hostOnly = h
		}

		rpid = hostOnly

		scheme := "https"
		
		if r.TLS == nil {
			// Respect a reverse-proxy forwarded scheme if present.
			if fwd := r.Header.Get("X-Forwarded-Proto"); fwd != "" {
				scheme = fwd
			} else {
				scheme = "http"
			}
		}

		origins = []string{scheme + "://" + host}
	} else {
		origins = []string{"https://" + rpid}
		if rpid == "localhost" {
			origins = append(origins, "http://localhost")
		}
	}

	cfg := &webauthn.Config{
		RPDisplayName: "Ego Server",
		RPID:          rpid,
		RPOrigins:     origins,
	}

	return webauthn.New(cfg)
}

// WebAuthnUserFrom wraps a defs.User in the webAuthnUser adapter so it
// satisfies the webauthn.User interface expected by the library.
func WebAuthnUserFrom(u defs.User) webauthn.User {
	return webAuthnUser{user: u}
}

// MarshalCredentials encodes a slice of webauthn.Credential into the
// json.RawMessage format stored in defs.User.Passkeys.
func MarshalCredentials(creds []webauthn.Credential) (json.RawMessage, error) {
	return json.Marshal(creds)
}

// FindPermission is an exported shim so the server/server package can call the
// unexported findPermission helper without importing internals.
func FindPermission(u defs.User, perm string) int {
	return findPermission(u, perm)
}
