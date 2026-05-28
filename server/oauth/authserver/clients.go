package authserver

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"golang.org/x/crypto/bcrypt"
)

// OAuthClient describes a single registered OAuth2 client. The file on disk
// may contain either a plaintext client_secret (which is hashed and cleared on
// load) or a pre-hashed client_secret_hash.
//
// Client definitions are stored in the JSON file at ego.server.oauth.as.clients.
// Example file:
//
//	[
//	  {
//	    "client_id": "myapp",
//	    "client_secret": "supersecret",
//	    "redirect_uris": ["https://myapp.example.com/callback"],
//	    "grant_types": ["authorization_code","refresh_token"],
//	    "scopes": ["openid","profile","ego:read"]
//	  }
//	]
type OAuthClient struct {
	// ClientID is the public identifier for the client (e.g. "myapp").
	ClientID string `json:"client_id"`

	// ClientSecretHash holds the bcrypt hash of the client secret.
	// This field is written back to the in-memory registry after load;
	// it is NOT written back to the JSON file.
	ClientSecretHash string `json:"client_secret_hash,omitempty"`

	// ClientSecret is accepted in the JSON file as a convenience.
	// It is bcrypt-hashed into ClientSecretHash on load and then cleared
	// from memory so that plaintext secrets are never kept in RAM.
	ClientSecret string `json:"client_secret,omitempty"`

	// RedirectURIs is the set of allowed callback URIs for the authorization
	// code flow.  The redirect_uri in an authorization request must exactly
	// match one entry here.
	RedirectURIs []string `json:"redirect_uris"`

	// GrantTypes lists the OAuth2 grant types this client may use.
	// Valid values: "authorization_code", "client_credentials", "refresh_token".
	GrantTypes []string `json:"grant_types"`

	// Scopes is the set of OAuth2 scopes this client is allowed to request.
	Scopes []string `json:"scopes"`

	// Description is a human-readable label for the client (optional).
	Description string `json:"description,omitempty"`
}

// clients holds the in-memory registry of registered OAuth2 clients.
// It is loaded once at startup by loadClients().
var clients []OAuthClient

// loadClients reads the client registry JSON file and populates the in-memory
// client list.  If a client entry has a plaintext ClientSecret it is bcrypt-hashed
// and the plaintext is cleared so it does not linger in memory.
func loadClients(clientFile string) error {
	data, err := os.ReadFile(clientFile)
	if err != nil {
		if os.IsNotExist(err) {
			// No client file is not an error — the server just starts with zero
			// registered clients.  Only public flows (client_credentials without
			// a secret) would work in this state.
			clients = []OAuthClient{}
			
			ui.Log(ui.ServerLogger, "oauth.as.clients.loaded", ui.A{
				"count": 0,
				"path":  clientFile,
			})

			return nil
		}

		return fmt.Errorf("reading client file %s: %w", clientFile, err)
	}

	// Client secrets are sensitive; the file must be owner-only.
	if permErr := ensureFilePermissions(clientFile); permErr != nil {
		return permErr
	}

	var loaded []OAuthClient

	if err = json.Unmarshal(data, &loaded); err != nil {
		return fmt.Errorf("parsing client file %s: %w", clientFile, err)
	}

	// Hash any plaintext secrets so we never keep them in memory.
	for i := range loaded {
		if loaded[i].ClientSecret != "" && loaded[i].ClientSecretHash == "" {
			hash, hashErr := bcrypt.GenerateFromPassword([]byte(loaded[i].ClientSecret), bcrypt.DefaultCost)
			if hashErr != nil {
				return fmt.Errorf("hashing secret for client %q: %w", loaded[i].ClientID, hashErr)
			}

			loaded[i].ClientSecretHash = string(hash)

			// Clear the plaintext so it is not stored in RAM.
			loaded[i].ClientSecret = ""
		}
	}

	clients = loaded

	ui.Log(ui.ServerLogger, "oauth.as.clients.loaded", ui.A{
		"count": len(clients),
		"path":  clientFile,
	})

	return nil
}

// findClient returns a pointer to the OAuthClient with the given client_id, or
// nil if no matching client is registered.
func findClient(clientID string) *OAuthClient {
	for i := range clients {
		if clients[i].ClientID == clientID {
			return &clients[i]
		}
	}

	return nil
}

// validateClientSecret returns true if the supplied plaintext secret matches the
// bcrypt hash stored for the given client.
func validateClientSecret(client *OAuthClient, secret string) bool {
	if client.ClientSecretHash == "" {
		// A client with no secret hash is a public client; any (or no) secret
		// is accepted. Public clients are appropriate for client_credentials flows
		// where the "secret" is a shared environment variable rather than a user
		// credential.
		return true
	}

	err := bcrypt.CompareHashAndPassword([]byte(client.ClientSecretHash), []byte(secret))

	return err == nil
}

// clientAllowsGrant returns true if the given client's grant_types list includes
// the requested grant type.
func clientAllowsGrant(client *OAuthClient, grantType string) bool {
	for _, g := range client.GrantTypes {
		if g == grantType {
			return true
		}
	}

	return false
}

// clientAllowsRedirect returns true if the given redirectURI is registered for
// the given client.
//
// Per RFC 8252 §7.3 (OAuth 2.0 for Native Apps), loopback redirect URIs are
// treated specially: if the client has a registered URI whose host is localhost
// or 127.0.0.1, any port on that host is considered a valid match. This allows
// the CLI to pick a random free port for its loopback listener without needing
// to register every possible port number.
func clientAllowsRedirect(client *OAuthClient, redirectURI string) bool {
	for _, registered := range client.RedirectURIs {
		if registered == redirectURI {
			return true
		}

		if isLoopbackURI(registered) && loopbackBaseMatches(registered, redirectURI) {
			return true
		}
	}

	return false
}

// isLoopbackURI reports whether uri uses http:// on localhost or 127.0.0.1.
func isLoopbackURI(uri string) bool {
	u, err := url.Parse(uri)
	if err != nil {
		return false
	}

	if u.Scheme != "http" {
		return false
	}

	host := strings.ToLower(u.Hostname())

	return host == "localhost" || host == "127.0.0.1"
}

// loopbackBaseMatches reports whether registered (a loopback URI) and requested
// share the same scheme, host (ignoring port), and path.
func loopbackBaseMatches(registered, requested string) bool {
	r, err := url.Parse(registered)
	if err != nil {
		return false
	}

	q, err := url.Parse(requested)
	if err != nil {
		return false
	}

	if r.Scheme != q.Scheme {
		return false
	}

	if !strings.EqualFold(r.Hostname(), q.Hostname()) {
		return false
	}

	// Paths must match exactly (e.g. both must be "/callback").
	return r.Path == q.Path
}

// injectBuiltinCLIClient ensures the "ego-cli" public client is present in
// the in-memory registry. It is called after loadClients() so that an
// administrator can override the built-in by adding their own "ego-cli" entry
// to the client file — the explicit registration takes precedence.
func injectBuiltinCLIClient() {
	if findClient("ego-cli") != nil {
		return
	}

	clients = append(clients, OAuthClient{
		ClientID: "ego-cli",
		// No ClientSecretHash — ego-cli is a public client; PKCE provides
		// proof of possession instead of a shared secret (RFC 7636).
		RedirectURIs: []string{"http://localhost", "http://127.0.0.1"},
		GrantTypes:   []string{"authorization_code", "refresh_token"},
		Scopes:       []string{"openid", "profile", "ego:read", "ego:write", "ego:admin", "ego:code"},
		Description:  "Ego CLI (built-in public client, RFC 8252)",
	})

	ui.Log(ui.ServerLogger, "oauth.as.client.builtin.injected", ui.A{
		"client_id": "ego-cli",
	})
}

// clientAllowsScope returns true if every scope in the requested list is
// included in the client's allowed scopes list.
func clientAllowsScope(client *OAuthClient, requestedScopes []string) bool {
	allowed := make(map[string]bool, len(client.Scopes))
	for _, s := range client.Scopes {
		allowed[s] = true
	}

	for _, s := range requestedScopes {
		if !allowed[s] {
			return false
		}
	}

	return true
}
