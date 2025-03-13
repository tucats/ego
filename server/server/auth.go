package server

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/caches"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	auth "github.com/tucats/ego/server/auth"
	"github.com/tucats/ego/util"
	"github.com/tucats/ego/validate"
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

	// Simplest case -- if there is an Authorization header, start with the information in the header.
	if len(r.Header.Values("Authorization")) > 0 {
		authHeader = r.Header.Get("Authorization")
	}

	// If there are no authentication credentials provided, but the method is PUT or POST with
	// a payload containing credentials, use the payload credentials.
	if authHeader == "" && (r.Method == http.MethodPut || r.Method == http.MethodPost) {
		credentials := defs.Credentials{}

		// Get the bytes of the request body as a byte array. See if it's a valid
		// LOGON reqeust object.
		b, err := io.ReadAll(r.Body)
		if err == nil && len(b) > 0 {
			err = validate.Validate(b, "@credentials")
			if err != nil {
				ui.Log(ui.ServerLogger, "rest.validation", ui.A{
					"session": s.ID,
					"error":   err.Error()})
			}
		}

		if err == nil {
			err = json.Unmarshal(b, &credentials)
		}

		if err == nil && credentials.Username != "" && credentials.Password != "" {
			authHeader = "Basic " + base64.StdEncoding.EncodeToString([]byte(credentials.Username+":"+credentials.Password))
			expiration = credentials.Expiration

			if expiration != "" {
				if _, err := util.ParseDuration(expiration); err != nil {
					ui.Log(ui.AuthLogger, "auth.invalid.expiration.duration", ui.A{
						"session":  s.ID,
						"duration": expiration})
				}
			}

			r.Header.Set("Authorization", authHeader)
			ui.Log(ui.AuthLogger, "auth.payload.creds", ui.A{
				"session": s.ID})

			if expiration != "" {
				ui.Log(ui.AuthLogger, "auth.expires", ui.A{
					"session":  s.ID,
					"duration": expiration})
			}
		} else {
			ui.Log(ui.AuthLogger, "auth.bad.payload.creds", ui.A{
				"session": s.ID,
				"error":   err,
				"user":    credentials.Username})
		}
	}

	// If there was no autheorization found, or the credentials payload was incorrectly formed,
	// we don't really have any credentials to use.
	if authHeader == "" {
		isAuthenticated = false

		ui.Log(ui.AuthLogger, "auth.no.creds", ui.A{
			"session": s.ID})
	} else if strings.HasPrefix(strings.ToLower(authHeader), defs.AuthScheme) {
		// Bearer token provided. Extract the token part of the header info, and
		// attempt to validate it.
		token = strings.TrimSpace(authHeader[len(defs.AuthScheme):])

		// Have we recently decoded this token? If so, we can continue to use it
		// since the cache ages out after 60 seconds.
		if userItem, found := caches.Find(caches.TokenCache, token); found {
			isAuthenticated = true
			user = data.String(userItem)
		} else {
			// Nope, not in the cache so let's revalidate the atoken using the
			// current active auth service (which may be database, filesystem,
			// in-memory, etc.). If ti can be authenticated, then capture the
			// username from the token, and if not empty, add it to the cache
			// for future retrieval.
			isAuthenticated = auth.ValidateToken(token)
			if isAuthenticated {
				user = auth.TokenUser(token)
				// If there was a valid user name in the token, add it to the cache for future use.
				if user != "" {
					caches.Add(caches.TokenCache, token, user)
				}
			}
		}

		// Form a version of the token string that is suitable for logging. If the token is
		// longer than ten characters, truncate it and add elipsese to indicate there is more
		// data we just don't put in the log. Since all Ego tokens are longer than this, it
		// has the effect of obscuring the token in the log but still making it easy to determine
		// from the log if the token matches the one in the confugration data.
		loggableToken := token
		if len(loggableToken) > 10 {
			loggableToken = fmt.Sprintf("%s...%s", loggableToken[:4], loggableToken[len(loggableToken)-4:])
		}

		// Form a string indicating if the crendential was valid that will be used for
		// logging. While we're here, also see if the user was authenticated and has
		// the root permission.
		validationSuffix := credentialInvalidMessage

		if isAuthenticated {
			if auth.GetPermission(user, "root") {
				isRoot = true
				validationSuffix = credentialAdminMessage
			} else {
				validationSuffix = credentialNormalMessage
			}
		}

		ui.Log(ui.AuthLogger, "auth.using.token", ui.A{
			"session": s.ID,
			"token":   loggableToken,
			"user":    user,
			"flag":    validationSuffix})
	} else {
		// No token was involved, so we're going to have to see what we can make fo the user and
		// password provided in the basic authentication area. This must be syntactically valid, and
		// if so, is also checked to see if the credentials are valid for our user database.
		user, pass, ok = r.BasicAuth()
		if !ok {
			ui.Log(ui.AuthLogger, "auth.bad.basic", ui.A{
				"session": s.ID})
		} else {
			isAuthenticated = auth.ValidatePassword(user, pass)
		}

		// Form a logging suffix that indicates if the credentials are invalid, valid,
		// or valid and represent a root user.
		validStatusSuffix := credentialInvalidMessage

		if isAuthenticated {
			if auth.GetPermission(user, "root") {
				validStatusSuffix = credentialAdminMessage
				isRoot = true
			} else {
				validStatusSuffix = credentialNormalMessage
			}
		}

		ui.Log(ui.AuthLogger, "auth.authorized", ui.A{
			"session": s.ID,
			"user":    strconv.Quote(user),
			"flag":    validStatusSuffix})
	}

	// Store the results of the authentication processses and return to the caller.
	s.User = user
	s.Token = token
	s.Authenticated = isAuthenticated
	s.Admin = isAuthenticated && isRoot
	s.Expiration = expiration

	return s
}
