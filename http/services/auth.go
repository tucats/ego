package services

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	auth "github.com/tucats/ego/http/auth"
	"github.com/tucats/ego/symbols"
)

// Handler authentication. This can be either Basic authentiation or using a Bearer
// token in the header. If found, validate the username:password or the token string,
// and set up state variables accordingly. This function does not return an error, but
// sets up the state of the authentication check in the symbol table for use by the
// handler service.
func handlerAuth(sessionID int32, r *http.Request, symbolTable *symbols.SymbolTable) {
	var authenticatedCredentials bool

	user := ""
	pass := ""

	symbolTable.SetAlways("_token", "")
	symbolTable.SetAlways("_token_valid", false)

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
			ui.Log(ui.AuthLogger, "[%d] Authorization credentials found in request payload", sessionID)
		} else {
			ui.Log(ui.AuthLogger, "[%d] failed attempt at payload credentials, %v, user=%s", sessionID, err, credentials.Username)
		}
	}

	// If there was no autheorization item, or the credentials payload was incorrectly formed,
	// we don't really have any credentials to use.
	if authorization == "" {
		// No authentication credentials provided
		authenticatedCredentials = false

		ui.Log(ui.AuthLogger, "[%d] No authentication credentials given", sessionID)
	} else if strings.HasPrefix(strings.ToLower(authorization), defs.AuthScheme) {
		// Bearer token provided. Extract the token part of the header info, and
		// attempt to validate it.
		token := strings.TrimSpace(authorization[len(defs.AuthScheme):])
		authenticatedCredentials = auth.ValidateToken(token)

		symbolTable.SetAlways("_token", token)
		symbolTable.SetAlways("_token_valid", authenticatedCredentials)

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

			ui.WriteLog(ui.AuthLogger, "[%d] Auth using token %s, user %s%s", sessionID, tokenstr, user, valid)
		}
	} else {
		// Must have a valid username:password. This must be syntactically valid, and
		// if so, is also checked to see if the credentials are valid for our user
		// database.
		var ok bool

		user, pass, ok = r.BasicAuth()
		if !ok {
			ui.Log(ui.AuthLogger, "[%d] BasicAuth invalid", sessionID)
		} else {
			authenticatedCredentials = auth.ValidatePassword(user, pass)
		}

		symbolTable.SetAlways("_token", "")
		symbolTable.SetAlways("_token_valid", false)

		valid := credentialInvalidMessage
		if authenticatedCredentials {
			if auth.GetPermission(user, "root") {
				valid = credentialAdminMessage
			} else {
				valid = credentialNormalMessage
			}
		}

		ui.Log(ui.AuthLogger, "[%d] Auth using user \"%s\"%s", sessionID,
			user, valid)
	}

	// Store the rest of the credentials status information we've accumulated.
	symbolTable.SetAlways("_user", user)
	symbolTable.SetAlways("_password", pass)
	symbolTable.SetAlways("_authenticated", authenticatedCredentials)
	symbolTable.SetAlways(defs.RestStatusVariable, http.StatusOK)
	symbolTable.SetAlways("_superuser", authenticatedCredentials && auth.GetPermission(user, "root"))
}
