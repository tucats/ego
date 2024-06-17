package server

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	auth "github.com/tucats/ego/server/auth"
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
	var (
		isAuthenticated bool
		isRoot          bool
		ok              bool
		user            string
		expiration      string
		pass            string
		token           string
		authHeader      string
	)

	if len(r.Header.Values("Authorization")) > 0 {
		authHeader = r.Header.Get("Authorization")
	}

	// If there are no authentication credentials provided, but the method is PUT or POST with
	// a payload containing credentials, use the payload credentials.
	if authHeader == "" && (r.Method == http.MethodPut || r.Method == http.MethodPost) {
		credentials := defs.Credentials{}

		err := json.NewDecoder(r.Body).Decode(&credentials)
		if err == nil && credentials.Username != "" && credentials.Password != "" {
			authHeader = "Basic " + base64.StdEncoding.EncodeToString([]byte(credentials.Username+":"+credentials.Password))
			expiration = credentials.Expiration

			if expiration != "" {
				if _, err := time.ParseDuration(expiration); err != nil {
					ui.Log(ui.AuthLogger, "[%d] Invalid expiration duration '%s' ignored", s.ID, expiration)
					expiration = ""
				}
			}

			r.Header.Set("Authorization", authHeader)
			ui.Log(ui.AuthLogger, "[%d] Authorization credentials found in request payload", s.ID)

			if expiration != "" {
				ui.Log(ui.AuthLogger, "[%d] Session request expiration set to %s", s.ID, expiration)
			}
		} else {
			ui.Log(ui.AuthLogger, "[%d] failed attempt at payload credentials, %v, user=%s", s.ID, err, credentials.Username)
		}
	}

	// If there was no autheorization item, or the credentials payload was incorrectly formed,
	// we don't really have any credentials to use.
	if authHeader == "" {
		isAuthenticated = false

		ui.Log(ui.AuthLogger, "[%d] No authentication credentials given", s.ID)
	} else if strings.HasPrefix(strings.ToLower(authHeader), defs.AuthScheme) {
		// Bearer token provided. Extract the token part of the header info, and
		// attempt to validate it.
		token = strings.TrimSpace(authHeader[len(defs.AuthScheme):])
		isAuthenticated = auth.ValidateToken(token)
		if isAuthenticated {
			user = auth.TokenUser(token)
		}

		loggableToken := token
		if len(loggableToken) > 10 {
			loggableToken = loggableToken[:10] + "..."
		}

		validationSuffix := credentialInvalidMessage

		if isAuthenticated {
			if auth.GetPermission(user, "root") {
				isRoot = true
				validationSuffix = credentialAdminMessage
			} else {
				validationSuffix = credentialNormalMessage
			}
		}

		ui.Log(ui.AuthLogger, "[%d] Auth using token %s, user %s%s", s.ID, loggableToken, user, validationSuffix)
	} else {
		// Must have a valid username:password. This must be syntactically valid, and
		// if so, is also checked to see if the credentials are valid for our user
		// database.
		user, pass, ok = r.BasicAuth()
		if !ok {
			ui.Log(ui.AuthLogger, "[%d] Basic Authorization header invalid", s.ID)
		} else {
			isAuthenticated = auth.ValidatePassword(user, pass)
		}

		validStatusSuffix := credentialInvalidMessage

		if isAuthenticated {
			if auth.GetPermission(user, "root") {
				validStatusSuffix = credentialAdminMessage
				isRoot = true
			} else {
				validStatusSuffix = credentialNormalMessage
			}
		}

		ui.Log(ui.AuthLogger, "[%d] Auth using user %s%s", s.ID,
			strconv.Quote(user), validStatusSuffix)
	}

	// Store the rest of the credentials status information we've accumulated.
	s.User = user
	s.Token = token
	s.Authenticated = isAuthenticated
	s.Admin = isAuthenticated && isRoot
	s.Expiration = expiration

	return s
}
