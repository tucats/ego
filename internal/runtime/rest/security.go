package rest

import (
	"os"

	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/symbols"
)

// Do we allow outbound REST calls with invalid/insecure certificates?
var allowInsecure = false

// sandboxedIO returns the state of the sandbox flag for IO operations,
// mirroring the same-named check in the exec and io packages.
func sandboxedIO(s *symbols.SymbolTable) bool {
	if v, ok := s.Get(defs.SandboxedIOSymbolName); ok {
		return data.BoolOrFalse(v)
	}

	return false
}

// setVerify implements the setVerify() rest function. This accepts a boolean value
// and sets the TLS server certificate authentication accordingly. When set to true,
// a connection will not be made if the server's certificate cannot be authenticated.
// This is the default mode for HTTPS connections. During debugging, you may wish to
// turn this off when using self-generated certificates.
func setVerify(s *symbols.SymbolTable, args data.List) (any, error) {
	var err error

	this := getThis(s)
	verify := allowInsecure

	if args.Len() == 1 {
		verify, err = data.Bool(args.Get(0))
	}

	this.SetAlways(verifyFieldName, verify)

	return this, err
}

// setAuthentication implements the setAuthentication() rest function. When present, it accepts a username and
// password as parameters, and sets the rest client to use BasicAuth authentication, where
// the username and password are part of an Authentication header.
func setAuthentication(s *symbols.SymbolTable, args data.List) (any, error) {
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
func setToken(s *symbols.SymbolTable, args data.List) (any, error) {
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

// setUseToken implements the UseToken() rest function. New() never attaches
// the server's own logon token automatically while an Ego program is
// running (see the security note on New()), so this is the explicit opt-in
// for a script that genuinely needs to call back into its own Ego server
// using the ambient identity. When flag is true, the stored logon token
// (settings.Get(defs.LogonTokenSetting)) is attached as this client's bearer
// token. When flag is false, any bearer token currently set on the client
// (from this method, from Token(), or -- for a client created outside a
// running Ego program -- from New() itself) is cleared.
//
// This is refused outright when the current execution context is sandboxed
// (e.g. a server-hosted dashboard "run" session executing untrusted code):
// a restricted runtime should not be able to exfiltrate the server's own
// token to an arbitrary host, whether on purpose or by accident, and there
// is no legitimate reason for sandboxed code to need it.
func setUseToken(s *symbols.SymbolTable, args data.List) (any, error) {
	if sandboxedIO(s) {
		return nil, errors.ErrNoPrivilegeForOperation.In("UseToken")
	}

	r, err := getClient(s)
	if err != nil {
		return nil, err
	}

	this := getThis(s)

	flag, err := data.Bool(args.Get(0))
	if err != nil {
		return nil, err
	}

	if flag {
		r.SetAuthToken(settings.Get(defs.LogonTokenSetting))
	} else {
		r.SetAuthToken("")
	}

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
