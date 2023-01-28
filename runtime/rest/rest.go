package rest

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/go-resty/resty"
	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// New implements the New() rest function.
func New(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	client := resty.New()

	if len(args) > 0 {
		username := data.String(args[0])
		password := ""

		if len(args) > 1 {
			password = data.String(args[1])
		}

		client.SetBasicAuth(username, password)
		client.SetDisableWarn(true)
	} else {
		token := settings.Get(defs.LogonTokenSetting)
		if token != "" {
			client.SetAuthToken(token)
		}
	}

	if config, err := GetTLSConfiguration(); err != nil {
		return nil, err
	} else {
		client.SetTLSClientConfig(config)
	}

	r := data.NewStruct(restType).FromBuiltinPackage()

	_ = r.Set(clientFieldName, client)
	_ = r.Set(mediaTypeFieldName, defs.JSONMediaType)
	_ = r.Set(verifyFieldName, true)

	r.SetReadonly(true)

	return r, nil
}

func closeClient(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	c, err := getClient(s)
	if err != nil {
		return nil, err
	}

	c.GetClient().CloseIdleConnections()

	this := getThis(s)
	c = nil
	this.SetAlways(clientFieldName, nil)
	this.SetAlways(statusFieldName, 0)

	return true, nil
}

// setDebug implements the setDebug() rest function. This specifies a boolean value that
// enables or disables debug logging for the client.
func setDebug(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	r, err := getClient(s)
	if err != nil {
		return nil, err
	}

	this := getThis(s)

	flag := data.Bool((args[0]))
	r.SetDebug(flag)

	return this, nil
}

// setMedia implements the setMedia() function. This specifies a string containing the media
// type that the REST service expects. In it's simplest form, this can be "application/text"
// for free text responses, or "application/json" for JSON data payloads.
func setMedia(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	_, err := getClient(s)
	if err != nil {
		return nil, err
	}

	this := getThis(s)
	media := data.String(args[0])
	this.SetAlways(mediaTypeFieldName, media)

	return this, nil
}

// fetchCookies extracts the cookies from the response, and format them as an Ego array
// of structs.
func fetchCookies(s *symbols.SymbolTable, r *resty.Response) *data.Array {
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
// variable, validates that it contains a REST client object, and returns
// the native client object.
func getClient(symbols *symbols.SymbolTable) (*resty.Client, error) {
	if g, ok := symbols.Get(defs.ThisVariable); ok {
		if gc, ok := g.(*data.Struct); ok {
			if client := gc.GetAlways(clientFieldName); client != nil {
				if cp, ok := client.(*resty.Client); ok {
					if cp == nil {
						return nil, errors.ErrRestClientClosed
					}

					return cp, nil
				}
			}
		}
	}

	return nil, errors.ErrNoFunctionReceiver
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

	if x, found := symbols.RootSymbolTable.Get(defs.VersionName); found {
		version = data.String(x)
	}

	platform := runtime.Version() + ", " + runtime.GOOS + ", " + runtime.GOARCH
	agent := "Ego " + version + " (" + platform + ") " + agentType

	r.Header.Add("User-Agent", agent)
	ui.Log(ui.RestLogger, "User agent: %s", agent)
}
