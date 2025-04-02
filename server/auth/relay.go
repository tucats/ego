package auth

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/runtime/rest"
)

// remoteUser accepts a token and fetches the remote user information
// associated with that token from an authentication server. This is
// used when an Ego server is configured to provide REST services but
// is not the authority for authentication or permissions. Multiple
// Ego service providers can share the same authentication server, for
// example.
//
// The call to the remote server provides the token given to this
// services instance for authentication. The remote authentication
// server decrypts the token (only the authentication server has
// the decryption key for tokens it generates). The username, expiration,
// and permissions data is returned to the local service provider instance
// of Ego.
//
// When a remote user is authenticated, it is stored in the local auth
// store, which is configured to be an in-memory-only store. The local
// (ephemeral) copy of the user data is deleted if it is not accessed
// for 180 seconds (3 minutes). After that time, it is purged from the
// local auth store, and must be re-authenticated by the remote auth
// server the next time the user presents a token. This way the local
// server does not continually re-authenticate the user, but if the
// user is delete or permissions are updated, the local auth store will
// update it's copy to the remote auth server within the 180 seconds.
func remoteUser(authServer, token string) (*defs.User, error) {
	url := authServer + "/services/admin/authenticate/"
	resp := data.NewStruct(data.StructType)

	ui.Log(ui.RestLogger, "rest.auth.refer", ui.A{
		"path": authServer})

	err := rest.Exchange(url, http.MethodGet, token, resp, "authenticate")
	if err != nil {
		return nil, err
	}

	// If we didn't get a 401 error on the above call, the token is valid.
	// Since we aren't an auth service ourselves, let's copy this info to
	// our local auth store
	u := defs.User{}
	if name, ok := resp.Get("Name"); ok {
		u.Name = data.String(name)
		u.ID = uuid.Nil
	}

	if v, ok := resp.Get("Permissions"); ok {
		if perms, ok := v.(*data.Array); ok {
			u.Permissions = []string{}

			for i := 0; i < perms.Len(); i++ {
				v, _ := perms.Get(i)
				u.Permissions = append(u.Permissions, data.String(v))
			}
		} else if perms, ok := v.([]interface{}); ok {
			u.Permissions = []string{}

			for i := 0; i < len(perms); i++ {
				v := perms[i]
				u.Permissions = append(u.Permissions, data.String(v))
			}
		}
	}

	err = AuthService.WriteUser(u)

	// Because this is data that came from a token, let's launch a small thread
	// whose job is to expire the local (ephemeral) user data.
	if v, ok := resp.Get("Expires"); ok {
		expirationString := data.String(v)
		format := time.RFC822Z

		if expires, err := time.Parse(format, expirationString); err == nil {
			agingMutex.Lock()
			aging[u.Name] = expires
			agingMutex.Unlock()
		} else {
			// Something went wrong here. If AUTH logging is enabled, log it as an AUTH
			// issue. But if not enabled, log it as a server issue.
			class := ui.AuthLogger
			if !ui.IsActive(ui.AuthLogger) {
				class = ui.ServerLogger
			}

			ui.Log(class, "auth.invalid.expiration.format", ui.A{
				"user":  u.Name,
				"error": err})
		}
	}

	return &u, err
}
