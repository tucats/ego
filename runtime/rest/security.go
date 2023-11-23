package rest

import (
	"os"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/symbols"
)

// Do we allow outbound REST calls with invalid/insecure certificates?
var allowInsecure = false

// setVerify implements the setVerify() rest function. This accepts a boolean value
// and sets the TLS server certificate authentication accordingly. When set to true,
// a connection will not be made if the server's certificate cannot be authenticated.
// This is the default mode for HTTPS connections. During debugging, you may wish to
// turn this off when using self-generated certificates.
func setVerify(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	this := getThis(s)
	verify := allowInsecure

	if args.Len() == 1 {
		verify = data.Bool(args.Get(0))
	}

	this.SetAlways(verifyFieldName, verify)

	return this, nil
}

// setAuthentication implements the setAuthentication() rest function. When present, it accepts a username and
// password as parameters, and sets the rest client to use BasicAuth authentication, where
// the username and password are part of an Authentication header.
func setAuthentication(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	r, err := getClient(s)
	if err != nil {
		return nil, err
	}

	this := getThis(s)

	user := data.String(args.Get(0))
	pass := data.String(args.Get(1))

	r.SetBasicAuth(user, pass)

	return this, nil
}

// setToken implements the setToken() rest function. When present, it accepts a token string
// and sets the rest client to use Bearer token authentication using this token value.
func setToken(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	r, err := getClient(s)
	if err != nil {
		return nil, err
	}

	this := getThis(s)

	token := settings.Get(defs.LogonTokenSetting)

	if args.Len() > 0 {
		token = data.String(args.Get(0))
	}

	r.SetAuthToken(token)

	return this, nil
}

// Externalized function that sets the "insecure" flag, which turns off
// server validation. This is called from the CLI parsing action when
// "--insecure" is specified as a global option in the Ego command line.
func AllowInsecure(flag bool) {
	allowInsecure = flag

	if flag {
		os.Setenv(defs.EgoInsecureClientEnv, defs.True)
	} else {
		os.Setenv(defs.EgoInsecureClientEnv, "")
	}
}
