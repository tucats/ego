package server

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	auth "github.com/tucats/ego/http/auth"
)

const (
	credentialInvalidMessage = ", invalid credential"
	credentialAdminMessage   = ", root privilege user"
	credentialNormalMessage  = ", normal user"
)

// Authenticate examaines the request for valid credentials. These can be a bearer
// token or a Basic username/password specification. Finally, if the request is a
// POST (create) and there are no credentials and the body contains a username/password
// specification, then use those as the credentials.
//
// A handler can examine the session object to determine the status of authentication.
func (s *Session) Authenticate(r *http.Request) *Session {
	var authenticatedCredentials bool

	user := ""
	pass := ""

	authorization := ""
	if len(r.Header.Values("Authorization")) > 0 {
		authorization = r.Header.Get("Authorization")
	}

	// If there are no authentication credentials provided, but the method is PUT with a payload
	// containing credentials, use them.
	if authorization == "" && (r.Method == http.MethodPut || r.Method == http.MethodPost) {
		credentials := defs.Credentials{}

		err := json.NewDecoder(r.Body).Decode(&credentials)
		if err == nil && credentials.Username != "" && credentials.Password != "" {
			// Create the authorization header from the payload
			authorization = "Basic " + base64.StdEncoding.EncodeToString([]byte(credentials.Username+":"+credentials.Password))
			r.Header.Set("Authorization", authorization)
			ui.Log(ui.AuthLogger, "[%d] Authorization credentials found in request payload", s.ID)
		} else {
			ui.Log(ui.AuthLogger, "[%d] failed attempt at payload credentials, %v, user=%s", s.ID, err, credentials.Username)
		}
	}

	// If there was no autheorization item, or the credentials payload was incorrectly formed,
	// we don't really have any credentials to use.
	if authorization == "" {
		// No authentication credentials provided
		authenticatedCredentials = false

		ui.Log(ui.AuthLogger, "[%d] No authentication credentials given", s.ID)
	} else if strings.HasPrefix(strings.ToLower(authorization), defs.AuthScheme) {
		// Bearer token provided. Extract the token part of the header info, and
		// attempt to validate it.
		token := strings.TrimSpace(authorization[len(defs.AuthScheme):])
		authenticatedCredentials = auth.ValidateToken(token)

		s.Token = token
		s.Authenticated = authenticatedCredentials

		user = auth.TokenUser(token)

		// If doing INFO logging, make a neutered version of the token showing
		// only the first few bytes of the token string.
		if ui.IsActive(ui.AuthLogger) {
			tokenstr := token
			if len(tokenstr) > 10 {
				tokenstr = tokenstr[:10] + "..."
			}

			valid := credentialInvalidMessage
			if authenticatedCredentials {
				if auth.GetPermission(user, "root") {
					valid = credentialAdminMessage
				} else {
					valid = credentialNormalMessage
				}
			}

			ui.WriteLog(ui.AuthLogger, "[%d] Auth using token %s, user %s%s", s.ID, tokenstr, user, valid)
		}
	} else {
		// Must have a valid username:password. This must be syntactically valid, and
		// if so, is also checked to see if the credentials are valid for our user
		// database.
		var ok bool

		user, pass, ok = r.BasicAuth()
		if !ok {
			ui.Log(ui.AuthLogger, "[%d] BasicAuth invalid", s.ID)
		} else {
			authenticatedCredentials = auth.ValidatePassword(user, pass)
		}

		valid := credentialInvalidMessage
		if authenticatedCredentials {
			if auth.GetPermission(user, "root") {
				valid = credentialAdminMessage
				s.Admin = true
			} else {
				valid = credentialNormalMessage
			}
		}

		ui.Log(ui.AuthLogger, "[%d] Auth using user \"%s\"%s", s.ID,
			user, valid)

		s.User = user
		s.Authenticated = authenticatedCredentials
	}

	// Store the rest of the credentials status information we've accumulated.
	s.User = user
	s.Authenticated = authenticatedCredentials
	s.Admin = authenticatedCredentials && auth.GetPermission(user, "root")

	return s
}
