package rest

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/symbols"
	"gopkg.in/resty.v1"
)

// isUserCodeRunning reports whether the interpreter is currently executing an
// Ego program (via "ego run", a server-hosted service, or a spawned child
// process) as opposed to one of the CLI's own native Go command
// implementations (e.g. "ego logon") calling directly into the rest package's
// internal Exchange()/newClient() helpers. See the security note on New()
// below for why this distinction matters.
func isUserCodeRunning() bool {
	v, found := symbols.RootSymbolTable.Get(defs.UserCodeRunningVariable)

	return found && data.BoolOrFalse(v)
}

// New implements the New() rest function.
//
// Security note: the server's own logon token is only ever attached
// automatically when isUserCodeRunning() is false -- i.e. never, for a
// client created by an Ego program. A rest.Client's Base() can point
// anywhere, including a host the caller does not control, so auto-attaching
// a valid bearer token here would let any Ego script -- trusted or
// otherwise -- exfiltrate it to an arbitrary third party simply by creating
// a client with no explicit credentials and pointing it elsewhere. A script
// that genuinely needs to call back into its own Ego server using the
// ambient identity must opt in explicitly with UseToken(true).
func New(s *symbols.SymbolTable, args data.List) (any, error) {
	client := resty.New()

	if args.Len() > 0 {
		username := data.String(args.Get(0))
		password := ""

		if args.Len() > 1 {
			password = data.String(args.Get(1))
		}

		if username != "" || password != "" {
			client.SetBasicAuth(username, password)
			client.SetDisableWarn(true)
		}
	} else if !isUserCodeRunning() {
		token := settings.Get(defs.LogonTokenSetting)
		if token != "" {
			client.SetAuthToken(token)
		}
	}

	config, err := GetTLSConfiguration()
	if err != nil {
		return data.NewList(nil, err), err
	}

	client.SetTLSClientConfig(config)

	r := data.NewStruct(RestClientType).FromBuiltinPackage()

	_ = r.Set(clientFieldName, client)
	_ = r.Set(mediaTypeFieldName, defs.JSONMediaType)
	_ = r.Set(verifyFieldName, true)

	r.SetReadonly(true)

	return data.NewList(r, nil), nil
}

func closeClient(s *symbols.SymbolTable, args data.List) (any, error) {
	c, err := getClient(s)
	if err != nil {
		// Returned as the value (not the native error) so this is a normal
		// catchable/assignable error, matching io.File.Close()'s convention
		// for a single-ErrorType-return function.
		return err, nil
	}

	c.GetClient().CloseIdleConnections()

	this := getThis(s)
	c = nil
	this.SetAlways(clientFieldName, nil)
	this.SetAlways(statusFieldName, 0)

	return nil, nil
}

// setDebug implements the setDebug() rest function. This specifies a boolean value that
// enables or disables debug logging for the client.
func setDebug(s *symbols.SymbolTable, args data.List) (any, error) {
	r, err := getClient(s)
	if err != nil {
		return nil, err
	}

	this := getThis(s)

	flag := data.BoolOrFalse((args.Get(0)))
	r.SetDebug(flag)

	return this, nil
}

// setMedia implements the setMedia() function. This specifies a string containing the media
// type that the REST service expects. In it's simplest form, this can be "application/text"
// for free text responses, or "application/json" for JSON data payloads.
func setMedia(s *symbols.SymbolTable, args data.List) (any, error) {
	if _, err := getClient(s); err != nil {
		return nil, err
	}

	this := getThis(s)
	media := data.String(args.Get(0))
	this.SetAlways(mediaTypeFieldName, media)

	return this, nil
}

// fetchCookies extracts the cookies from the response, and format them as an Ego array
// of structs.
func fetchCookies(r *resty.Response) *data.Array {
	cookies := r.Cookies()
	result := data.NewArray(data.InterfaceType, len(cookies))

	for i, v := range r.Cookies() {
		cookie := data.NewMap(data.StringType, data.InterfaceType)

		_, _ = cookie.Set("expires", v.Expires.String())
		_, _ = cookie.Set("name", v.Name)
		_, _ = cookie.Set("domain", v.Domain)
		_, _ = cookie.Set("value", v.Value)
		_, _ = cookie.Set("path", v.Path)
		_ = result.Set(i, cookie)
	}

	return result
}

// headerMap is a support function that extracts the header data from a
// rest response, and formats it to be an Ego map.
func headerMap(response *resty.Response) *data.Map {
	headers := data.NewMap(data.StringType, data.InterfaceType)

	for k, v := range response.Header() {
		_, _ = headers.Set(k, strings.TrimPrefix(strings.TrimSuffix(fmt.Sprintf("%v", v), "]"), "["))
	}

	return headers
}

// getClient searches the symbol table for the client receiver (defs.ThisVariable)
// variable, validates that it contains a REST client object, and returns the native
// client object.
func getClient(symbols *symbols.SymbolTable) (*resty.Client, error) {
	// Default error
	err := errors.ErrNoFunctionReceiver

	thisValue, ok := symbols.Get(defs.ThisVariable)
	if !ok {
		return nil, err
	}

	thisStruct, ok := thisValue.(*data.Struct)
	if !ok {
		return nil, err
	}

	client := thisStruct.GetAlways(clientFieldName)
	if client == nil {
		return nil, err
	}

	restPtr, ok := client.(*resty.Client)
	if !ok {
		return nil, err
	}

	if restPtr == nil {
		return nil, errors.ErrRestClientClosed
	}

	return restPtr, nil
}

// getThis returns a map for the "this" object in the current
// symbol table.
func getThis(s *symbols.SymbolTable) *data.Struct {
	t, ok := s.Get(defs.ThisVariable)
	if !ok {
		return nil
	}

	this, ok := t.(*data.Struct)
	if !ok {
		return nil
	}

	return this
}

func AddAgent(r *resty.Request, agentType string) {
	var version string

	if x, found := symbols.RootSymbolTable.Get(defs.VersionNameVariable); found {
		version = data.String(x)
	}

	platform := runtime.Version() + ", " + runtime.GOOS + ", " + runtime.GOARCH
	agent := "Ego " + version + " (" + platform + ") " + agentType

	r.Header.Add("User-Agent", agent)

	if ui.IsActive(ui.RestLogger) {
		ui.Log(ui.RestLogger, "rest.agent", ui.A{
			"agent": agent})
	}
}
