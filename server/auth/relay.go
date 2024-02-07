package auth

import (
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/runtime/rest"
)

// remoteUser accepts a token and fetches the remote user information
// associated with that token from an authentication server. This is
// the contents of the token itself (decrypted by the auth server)
// with an additional Permissions field added that contains the
// authorizations data for that user. This is returned in a User object
// locally.
func remoteUser(authServer, token string) (*defs.User, error) {
	url := authServer + "/services/admin/authenticate/"
	resp := data.NewStruct(data.StructType)

	ui.Log(ui.RestLogger, "*** Referring authorization request to %s", authServer)

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
		// @tomcole this should be revised to use an standardized date format string
		format := "2006-01-02 15:04:05.999999999 -0700 MST"
		if expires, err := time.Parse(format, expirationString); err == nil {
			agingMutex.Lock()
			aging[u.Name] = expires
			agingMutex.Unlock()
		}
	}

	return &u, err
}

// Go routine that runs periodically to see if credentials should be
// aged out of the user store. Runs every 180 seconds by default, but
// this can be overridden with the "ego.server.auth.cache.scan" setting.
func ageCredentials() {
	scanDelay := 180

	if scanString := settings.Get(defs.AuthCacheScanSetting); scanString != "" {
		if delay, err := strconv.Atoi(scanString); err != nil {
			scanDelay = delay
		}
	}

	for {
		time.Sleep(time.Duration(scanDelay) * time.Second)
		agingMutex.Lock()

		list := []string{}

		for user, expires := range aging {
			if time.Since(expires) > 0 {
				list = append(list, user)
			}
		}

		if len(list) > 0 {
			ui.Log(ui.AuthLogger, "Removing %d expired proxy user records", len(list))
		}

		for _, user := range list {
			delete(aging, user)
			_ = AuthService.DeleteUser(user)
		}

		agingMutex.Unlock()
	}
}
