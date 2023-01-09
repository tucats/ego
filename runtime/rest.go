package runtime

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/go-resty/resty"
	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/compiler"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/functions"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

// ServerCertificateFile is the default file name for the server certificate.
var ServerCertificateFile = envDefault("EGO_CERT_FILE", "https-server.crt")

// ServerKeyFile is the default file name for the server key.
var ServerKeyFile = envDefault("EGO_KEY_FILE", "https-server.key")

// Max number of times we will chase a redirect before failing.
const MaxRedirectCount = 10

// Do we allow outbound REST calls with invalid/insecure certificates?
var allowInsecure = false

// openServices is a list of endpoing paths that do not require the
// addition of an authorization token.
var openServices = []string{
	defs.ServicesUpPath,
	defs.AdminHeartbeatPath,
}

// This maps HTTP status codes to a message string.
var httpStatusCodeMessages = map[int]string{
	http.StatusContinue:                     "Continue",
	http.StatusSwitchingProtocols:           "Switching protocol",
	http.StatusProcessing:                   "Processing",
	http.StatusEarlyHints:                   "Early hints",
	http.StatusOK:                           "OK",
	http.StatusCreated:                      "Created",
	http.StatusAccepted:                     "Accepted",
	http.StatusNonAuthoritativeInfo:         "Non-authoritative information",
	http.StatusNoContent:                    "No content",
	http.StatusResetContent:                 "Reset content",
	http.StatusPartialContent:               "Partial content",
	http.StatusMultiStatus:                  "Multi-status",
	http.StatusAlreadyReported:              "Already reported",
	http.StatusIMUsed:                       "IM Used",
	http.StatusMultipleChoices:              "Multiple choice",
	http.StatusMovedPermanently:             "Moved permanently",
	http.StatusFound:                        "Found",
	http.StatusSeeOther:                     "See other",
	http.StatusNotModified:                  "Not modified",
	http.StatusUseProxy:                     "Use proxy",
	http.StatusTemporaryRedirect:            "Temporary redirect",
	http.StatusPermanentRedirect:            "Permanent redirect",
	http.StatusBadRequest:                   "Bad request",
	http.StatusUnauthorized:                 "Unauthorized",
	http.StatusPaymentRequired:              "Payment required",
	http.StatusForbidden:                    "Forbidden",
	http.StatusNotFound:                     "Not found",
	http.StatusMethodNotAllowed:             "Method not allowed",
	http.StatusNotAcceptable:                "Not acceptable",
	http.StatusProxyAuthRequired:            "Proxy authorization required",
	http.StatusRequestTimeout:               "Request timeout",
	http.StatusConflict:                     "Conflict",
	http.StatusGone:                         "Gone",
	http.StatusLengthRequired:               "Length required",
	http.StatusPreconditionFailed:           "Precondition failed",
	http.StatusRequestEntityTooLarge:        "Payload too large",
	http.StatusRequestURITooLong:            "URI too long",
	http.StatusUnsupportedMediaType:         "Unsupported media type",
	http.StatusRequestedRangeNotSatisfiable: "Range not satisfiable",
	http.StatusExpectationFailed:            "Expectation failed",
	http.StatusTeapot:                       "I'm a teapot",
	http.StatusMisdirectedRequest:           "Misdirected request",
	http.StatusUnprocessableEntity:          "Unprocessable entity",
	http.StatusLocked:                       "Locked",
	http.StatusFailedDependency:             "Failed dependency",
	http.StatusTooEarly:                     "Too early",
	http.StatusUpgradeRequired:              "Upgrade required",
	http.StatusPreconditionRequired:         "Precondition required",
	http.StatusTooManyRequests:              "Too many requests",
	http.StatusRequestHeaderFieldsTooLarge:  "Request header fields too large",
	http.StatusUnavailableForLegalReasons:   "Unavilable for legal reasons",
	http.StatusInternalServerError:          "Internal server error",
	http.StatusServiceUnavailable:           "Unavailable",
}

var tlsConfiguration *tls.Config
var tlsConfigurationMutex sync.Mutex

// Map key names for parsing a URL.
const (
	urlSchemeElement   = "urlScheme"
	urlHostElement     = "urlHost"
	urlUsernameElement = "urlUsername"
	urlPasswordElement = "urlPassword"
	urlPathElement     = "urlPath"
	urlQueryElmeent    = "urlQuery"
)

var restType *data.Type
var restTypeLock sync.Mutex

// Get an environment variable value. If it is not present (or empty) then use
// the provided default value as the result.
func envDefault(name, defaultValue string) string {
	result := os.Getenv(name)
	if result == "" {
		result = defaultValue
	}

	return result
}

func AllowInsecure(flag bool) {
	allowInsecure = flag

	if flag {
		os.Setenv("EGO_INSECURE_CLIENT", defs.True)
	} else {
		os.Setenv("EGO_INSECURE_CLIENT", "")
	}
}

func initializeRestType() {
	restTypeLock.Lock()
	defer restTypeLock.Unlock()

	if restType == nil {
		t, _ := compiler.CompileTypeSpec(restTypeSpec)

		t.DefineFunctions(map[string]interface{}{
			"Close":  RestClose,
			"Get":    RestGet,
			"Post":   RestPost,
			"Delete": RestDelete,
			"Base":   RestBase,
			"Debug":  RestDebug,
			"Media":  RestMedia,
			"Token":  RestToken,
			"Auth":   RestAuth,
			"Verify": VerifyServer,
			"Status": RestStatusMessage,
		})

		restType = t
	}
}

// RestNew implements the New() rest function.
func RestNew(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) != 0 && len(args) != 2 {
		return nil, errors.EgoError(errors.ErrArgumentCount)
	}

	client := resty.New()

	if len(args) == 2 {
		username := data.String(args[0])
		password := data.String(args[1])

		client.SetBasicAuth(username, password)
		client.SetDisableWarn(true)
	} else {
		token := settings.Get(defs.LogonTokenSetting)
		if token != "" {
			client.SetAuthToken(token)
		}
	}

	initializeRestType()

	if config, err := GetTLSConfiguration(); err != nil {
		return nil, err
	} else {
		client.SetTLSClientConfig(config)
	}

	r := data.NewStruct(restType)

	_ = r.Set(clientFieldName, client)
	_ = r.Set(mediaTypeFieldName, defs.JSONMediaType)
	_ = r.Set(verifyFieldName, true)

	r.SetReadonly(true)

	return r, nil
}

// utility function that prepends the base URL for this instance
// of a rest service to the supplied URL string. If there is
// no base URL defined, then nothing is changed.
func applyBaseURL(url string, this *data.EgoStruct) string {
	if b, ok := this.Get(baseURLFieldName); ok {
		base := data.String(b)
		if base == "" {
			return url
		}

		base = strings.TrimSuffix(base, "/")

		if !strings.HasPrefix(url, "/") {
			url = "/" + url
		}

		url = base + url
	}

	return url
}

func RestParseURL(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) < 1 || len(args) > 2 {
		return nil, errors.EgoError(errors.ErrArgumentCount)
	}

	urlString := data.String(args[0])

	url, err := url.Parse(urlString)
	if err != nil {
		return nil, errors.EgoError(err).Context(urlString)
	}

	hasSchema := strings.Contains(urlString, "://")
	urlParts := map[string]interface{}{}

	// If the second parameter was provided, it's a template string. Use it to parse
	// apart the path components of the url.
	if len(args) > 1 {
		var valid bool

		path := url.Path
		templateString := data.String(args[1])

		// Scan the URL and the template, and bulid a map of the parts.
		urlParts, valid = functions.ParseURLPattern(path, templateString)
		if !valid {
			return nil, errors.EgoError(errors.ErrInvalidURL).Context(path)
		}
	}

	// Store parsed parts based on the parsed URL. Empty elements are not
	// reported in the string. This has to be done after the above because
	// otherwise the template parser will re-initialize the hash map of parts.

	// Clunky, but... if there was no scheme in the original URL string, then
	// the URL parser will have assigned the hostname as the scheme. If there
	// was a proper scheme, then the host is the hostname as expected.
	if !hasSchema && url.Scheme != "" {
		urlParts[urlHostElement] = url.Scheme
	} else if host := url.Hostname(); host != "" {
		urlParts[urlHostElement] = host
	}

	if port := url.Port(); port != "" {
		urlParts["urlPort"] = port
	}

	// Note that if there was no schema in the original URL, then we don't
	// have a schema. Otherwise, record any non-empty schema
	if schema := url.Scheme; hasSchema && schema != "" {
		urlParts[urlSchemeElement] = url.Scheme
	}

	if user := url.User.Username(); user != "" {
		urlParts[urlUsernameElement] = user
	}

	if pw, found := url.User.Password(); found {
		urlParts[urlPasswordElement] = pw
	}

	if path := url.Path; path != "" {
		urlParts[urlPathElement] = path
	}

	if queryParts := url.Query(); len(queryParts) != 0 {
		query := map[string]interface{}{}

		for key, value := range queryParts {
			values := make([]interface{}, len(value))
			for i, j := range value {
				values[i] = j
			}

			query[key] = data.NewArrayFromArray(&data.StringType, values)
		}

		urlParts[urlQueryElmeent] = data.NewMapFromMap(query)
	}

	return data.NewStructFromMap(urlParts), nil
}

func RestStatusMessage(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) != 1 {
		return nil, errors.EgoError(errors.ErrArgumentCount)
	}

	code := data.Int(args[0])
	if text, ok := httpStatusCodeMessages[code]; ok {
		return text, nil
	}

	return fmt.Sprintf("HTTP status %d", code), nil
}

func RestClose(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) > 0 {
		return nil, errors.EgoError(errors.ErrArgumentCount)
	}

	c, err := getClient(s)
	if err != nil {
		return nil, err
	}

	c.GetClient().CloseIdleConnections()

	this := getThisStruct(s)
	c = nil
	this.SetAlways(clientFieldName, nil)
	this.SetAlways(statusFieldName, 0)

	return true, nil
}

// VerifyServer implements the Verify() rest function. This accepts a boolean value
// and sets the TLS server certificate authentication accordingly. When set to true,
// a connection will not be made if the server's certificate cannot be authenticated.
// This is the default mode for HTTPS connections. During debugging, you may wish to
// turn this off when using self-generated certificates.
func VerifyServer(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) != 1 {
		return nil, errors.EgoError(errors.ErrArgumentCount)
	}

	client, err := getClient(s)
	if err != nil {
		return nil, err
	}

	this := getThisStruct(s)
	verify := allowInsecure

	if len(args) == 1 {
		verify = data.Bool(args[0])
	}

	client.SetTLSClientConfig(&tls.Config{InsecureSkipVerify: verify})

	this.SetAlways(verifyFieldName, verify)

	return this, nil
}

// RestBase implements the Base() rest function. This specifies a string that is used
// as the base prefix for any URL formed in a REST call. This lets you specify the
// protocol/host/port information once, and then have each Get(), Post(), etc. call
// just specify the endpoint.
func RestBase(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) != 1 {
		return nil, errors.EgoError(errors.ErrArgumentCount)
	}

	_, err := getClient(s)
	if err != nil {
		return nil, err
	}

	this := getThisStruct(s)
	base := ""

	if len(args) > 0 {
		base = data.String(args[0])
	} else {
		base = settings.Get(defs.LogonServerSetting)
	}

	this.SetAlways(baseURLFieldName, base)

	return this, nil
}

// RestDebug implements the Debug() rest function. This specifies a boolean value that
// enables or disables debug logging for the client.
func RestDebug(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) != 1 {
		return nil, errors.EgoError(errors.ErrArgumentCount)
	}

	r, err := getClient(s)
	if err != nil {
		return nil, err
	}

	this := getThisStruct(s)

	flag := data.Bool((args[0]))
	r.SetDebug(flag)

	return this, nil
}

// RestAuth implements the Auth() rest function. When present, it accepts a username and
// password as parameters, and sets the rest client to use BasicAuth authentication, where
// the username and password are part of an Authentication header.
func RestAuth(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	r, err := getClient(s)
	if err != nil {
		return nil, err
	}

	this := getThisStruct(s)

	if len(args) != 2 {
		return nil, errors.EgoError(errors.ErrArgumentCount)
	}

	user := data.String(args[0])
	pass := data.String(args[1])

	r.SetBasicAuth(user, pass)

	return this, nil
}

// RestToken implements the Token() rest function. When present, it accepts a token string
// and sets the rest client to use Bearer token authentication using this token value.
func RestToken(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	r, err := getClient(s)
	if err != nil {
		return nil, err
	}

	this := getThisStruct(s)

	if len(args) > 1 {
		return nil, errors.EgoError(errors.ErrArgumentCount)
	}

	token := settings.Get(defs.LogonTokenSetting)

	if len(args) > 0 {
		token = data.String(args[0])
	}

	r.SetAuthToken(token)

	return this, nil
}

// RestMedia implements the Media() function. This specifies a string containing the media
// type that the REST service expects. In it's simplest form, this can be "application/text"
// for free text responses, or "application/json" for JSON data payloads.
func RestMedia(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) != 1 {
		return nil, errors.EgoError(errors.ErrArgumentCount)
	}

	_, err := getClient(s)
	if err != nil {
		return nil, err
	}

	this := getThisStruct(s)
	media := data.String(args[0])
	this.SetAlways(mediaTypeFieldName, media)

	return this, nil
}

// RestGet implements the rest Get() function. This must be provided with a URL or
// URL fragment (depending on whether Base() was called). The URL is constructed, and
// authentication set, and a GET HTTP operation is generated. The result is either a
// string (for media type of text) or a struct (media type of JSON).
func RestGet(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	client, err := getClient(s)
	if err != nil {
		return nil, err
	}

	client.SetRedirectPolicy(resty.FlexibleRedirectPolicy(MaxRedirectCount))

	this := getThisStruct(s)

	if len(args) != 1 {
		return nil, errors.EgoError(errors.ErrArgumentCount)
	}

	url := applyBaseURL(data.String(args[0]), this)
	r := client.NewRequest()
	isJSON := false

	if media, ok := this.Get(mediaTypeFieldName); ok {
		ms := data.String(media)
		isJSON = (strings.Contains(ms, defs.JSONMediaType))
		r.Header.Add("Accept", ms)
		r.Header.Add("Content-Type", ms)
	}

	AddAgent(r, defs.ClientAgent)

	response, e2 := r.Get(url)
	if e2 != nil {
		this.SetAlways(statusFieldName, http.StatusServiceUnavailable)

		return nil, errors.EgoError(e2)
	}

	this.SetAlways("cookies", fetchCookies(s, response))
	status := response.StatusCode()
	this.SetAlways(statusFieldName, status)
	this.SetAlways(headersFieldName, headerMap(response))
	rb := string(response.Body())

	if isJSON && ((status >= http.StatusOK && status <= 299) || strings.HasPrefix(rb, "{") || strings.HasPrefix(rb, "[")) {
		var jsonResponse interface{}

		err := json.Unmarshal([]byte(rb), &jsonResponse)
		if err != nil {
			err = errors.EgoError(err)
		}

		// For well-known complex types, make them Ego-native versions.
		switch actual := jsonResponse.(type) {
		case map[string]interface{}:
			jsonResponse = data.NewMapFromMap(actual)

		case []interface{}:
			jsonResponse = data.NewArrayFromArray(&data.InterfaceType, actual)
		}

		this.SetAlways(responseFieldName, jsonResponse)

		return jsonResponse, err
	}

	this.SetAlways(responseFieldName, rb)

	return rb, nil
}

// fetchCookies extracts the cookies from the response, and format them as an Ego array
// of structs.
func fetchCookies(s *symbols.SymbolTable, r *resty.Response) *data.EgoArray {
	cookies := r.Cookies()
	result := data.NewArray(&data.InterfaceType, len(cookies))

	for i, v := range r.Cookies() {
		cookie := data.NewMap(&data.StringType, &data.InterfaceType)

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
func headerMap(response *resty.Response) *data.EgoMap {
	headers := data.NewMap(&data.StringType, &data.InterfaceType)

	for k, v := range response.Header() {
		_, _ = headers.Set(k, strings.TrimPrefix(strings.TrimSuffix(fmt.Sprintf("%v", v), "]"), "["))
	}

	return headers
}

// RestPost implements the Post() rest function.
func RestPost(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	var body interface{} = ""

	if len(args) < 1 || len(args) > 2 {
		return nil, errors.EgoError(errors.ErrArgumentCount)
	}

	client, err := getClient(s)
	if err != nil {
		return nil, err
	}

	client.SetRedirectPolicy()

	this := getThisStruct(s)
	url := applyBaseURL(data.String(args[0]), this)

	if len(args) > 1 {
		body = args[1]
	}

	// If the media type is json, then convert the value passed
	// into a json value for the request body.
	if mt, ok := this.Get(mediaTypeFieldName); ok {
		media := data.String(mt)
		if strings.Contains(media, defs.JSONMediaType) {
			b, err := json.Marshal(body)
			if err != nil {
				return nil, errors.EgoError(err)
			}

			body = string(b)
		}
	}

	r := client.NewRequest().SetBody(body)
	isJSON := false

	if media, ok := this.Get(mediaTypeFieldName); ok {
		ms := data.String(media)
		isJSON = strings.Contains(ms, defs.JSONMediaType)

		r.Header.Add("Accept", ms)
		r.Header.Add("Content-Type", ms)
	}

	AddAgent(r, defs.ClientAgent)

	response, e2 := r.Post(url)
	if e2 != nil {
		this.SetAlways(statusFieldName, http.StatusServiceUnavailable)

		return nil, errors.EgoError(e2)
	}

	status := response.StatusCode()
	this.SetAlways("cookies", fetchCookies(s, response))
	this.SetAlways(statusFieldName, status)
	this.SetAlways(headersFieldName, headerMap(response))
	rb := string(response.Body())

	if isJSON {
		var jsonResponse interface{}

		err := json.Unmarshal([]byte(rb), &jsonResponse)
		if err != nil {
			err = errors.EgoError(err)
		}

		this.SetAlways(responseFieldName, jsonResponse)

		return jsonResponse, err
	}

	this.SetAlways(responseFieldName, rb)

	return rb, nil
}

// RestDelete implements the Delete() rest function.
func RestDelete(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	var body interface{} = ""

	if len(args) < 1 || len(args) > 2 {
		return nil, errors.EgoError(errors.ErrArgumentCount)
	}

	client, err := getClient(s)
	if err != nil {
		return nil, err
	}

	this := getThisStruct(s)
	url := applyBaseURL(data.String(args[0]), this)

	if len(args) > 1 {
		body = args[1]
	}

	// If the media type is json, then convert the value passed
	// into a json value for the request body.
	if mt, ok := this.Get(mediaTypeFieldName); ok {
		media := data.String(mt)
		if strings.Contains(media, defs.JSONMediaType) {
			b, err := json.Marshal(body)
			if err != nil {
				return nil, errors.EgoError(err)
			}

			body = string(b)
		}
	}

	r := client.NewRequest().SetBody(body)
	isJSON := false

	if media, ok := this.Get(mediaTypeFieldName); ok {
		ms := data.String(media)
		isJSON = (strings.Contains(ms, defs.JSONMediaType))

		r.Header.Add("Accept", ms)
		r.Header.Add("Content-Type", ms)
	}

	AddAgent(r, defs.ClientAgent)

	response, e2 := r.Delete(url)
	if e2 != nil {
		this.SetAlways(statusFieldName, http.StatusServiceUnavailable)

		return nil, errors.EgoError(e2)
	}

	status := response.StatusCode()
	this.SetAlways("cookies", fetchCookies(s, response))
	this.SetAlways(statusFieldName, status)
	this.SetAlways(headersFieldName, headerMap(response))
	rb := string(response.Body())

	if isJSON {
		var jsonResponse interface{}

		err := json.Unmarshal([]byte(rb), &jsonResponse)
		if err != nil {
			err = errors.EgoError(err)
		}

		this.SetAlways(responseFieldName, jsonResponse)

		return jsonResponse, err
	}

	this.SetAlways(responseFieldName, rb)

	return rb, nil
}

// getClient searches the symbol table for the client receiver ("__this")
// variable, validates that it contains a REST client object, and returns
// the native client object.
func getClient(symbols *symbols.SymbolTable) (*resty.Client, error) {
	if g, ok := symbols.Get("__this"); ok {
		if gc, ok := g.(*data.EgoStruct); ok {
			if client, ok := gc.Get(clientFieldName); ok {
				if cp, ok := client.(*resty.Client); ok {
					if cp == nil {
						return nil, errors.EgoError(errors.ErrRestClientClosed)
					}

					return cp, nil
				}
			}
		}
	}

	return nil, errors.EgoError(errors.ErrNoFunctionReceiver)
}

// getThis returns a map for the "this" object in the current
// symbol table.
func getThisStruct(s *symbols.SymbolTable) *data.EgoStruct {
	t, ok := s.Get("__this")
	if !ok {
		return nil
	}

	this, ok := t.(*data.EgoStruct)
	if !ok {
		return nil
	}

	return this
}

// Exchange is a helper wrapper around a rest call. This is generally used by all the
// CLI client operations _except_ the logon operation, since at that point the token
// is not known (or used).
func Exchange(endpoint, method string, body interface{}, response interface{}, agentType string, mediaTypes ...string) error {
	var resp *resty.Response

	var err error

	url := settings.Get(defs.ApplicationServerSetting)
	if url == "" {
		url = settings.Get(defs.LogonServerSetting)
	}

	if url == "" {
		url = "http://localhost:8080"
	}

	url = strings.TrimSuffix(url, "/") + endpoint

	ui.Debug(ui.RestLogger, "%s %s", strings.ToUpper(method), url)

	client := resty.New().SetRedirectPolicy(resty.FlexibleRedirectPolicy(MaxRedirectCount))

	// Unless this is a open (un-authenticate) service, let's verify that the
	// authentication token is still valid.
	if util.InList(endpoint, openServices...) {
		ui.Debug(ui.RestLogger, "Endpoint %s does not require token", endpoint)
	} else {
		if token := settings.Get(defs.LogonTokenSetting); token != "" {
			// Let's check to see if it's expired already... Note we skip this if the
			// agent string is "status".
			if !strings.EqualFold(agentType, defs.StatusAgent) {
				if expirationString := settings.Get(defs.LogonTokenExpirationSetting); expirationString != "" {
					expireTime, err := time.Parse(time.UnixDate, expirationString)
					if err != nil {
						return errors.EgoError(err)
					}

					now := time.Since(expireTime)
					if now > 0 {
						ui.Say("Your login has expired. Use the ego logon command to login again to %s", settings.Get(defs.LogonServerSetting))

						os.Exit(1)
					}
				}
			}

			client.SetAuthToken(token)
			ui.Debug(ui.RestLogger, "Authorization set using bearer token: %s...", token[:4])
		}
	}

	if config, err := GetTLSConfiguration(); err != nil {
		return err
	} else {
		client.SetTLSClientConfig(config)
	}

	r := client.NewRequest()

	// Lets figure out what media types we're sending and reciving. By default, they
	// are anonymous JSON. But if the call included one or two strings, they are used
	// as the receiving and sending media types respectively.
	receiveMediaType := defs.JSONMediaType
	sendMediaType := defs.JSONMediaType

	if len(mediaTypes) > 0 {
		receiveMediaType = mediaTypes[0]

		ui.Debug(ui.RestLogger, "Adding media type: %s", receiveMediaType)
	}

	if len(mediaTypes) > 1 {
		sendMediaType = mediaTypes[1]

		ui.Debug(ui.RestLogger, "Adding media type: %s", sendMediaType)
	}

	r.Header.Add("Content-Type", sendMediaType)
	r.Header.Add("Accept", receiveMediaType)
	AddAgent(r, agentType)

	if body != nil {
		b, err := json.MarshalIndent(body, "", "  ")
		if err != nil {
			return errors.EgoError(err)
		}

		ui.Debug(ui.RestLogger, "Request payload:\n%s", string(b))

		r.SetBody(b)
	}

	resp, err = r.Execute(method, url)
	if err != nil {
		ui.Debug(ui.RestLogger, "REST failed, %v", err)

		return errors.EgoError(err)
	}

	if err == nil {
		status := resp.StatusCode()

		ui.Debug(ui.RestLogger, "Status: %d", status)

		if err == nil && status != http.StatusOK && response == nil {
			return errors.EgoError(errors.ErrHTTP).Context(status)
		}

		if replyMedia := resp.Header().Get("Content-Type"); replyMedia != "" {
			ui.Debug(ui.RestLogger, "Reply media type: %s", replyMedia)
		}

		// If there was an error, and the runtime rest automatic error handling is enabled,
		// try to find the message text in the response, and if found, form an error response
		// to the local caller using that text.
		if (status < 200 || status > 299) && settings.GetBool(defs.RestClientErrorSetting) {
			errorResponse := map[string]interface{}{}

			err := json.Unmarshal(resp.Body(), &errorResponse)
			if err == nil {
				if msg, found := errorResponse["msg"]; found {
					ui.Debug(ui.RestLogger, "Response payload:\n%v", string(resp.Body()))

					return errors.NewMessage(data.String(msg))
				}

				if msg, found := errorResponse["message"]; found {
					ui.Debug(ui.RestLogger, "Response payload:\n%v", string(resp.Body()))

					return errors.NewMessage(data.String(msg))
				}
			}
		}

		if err == nil && response != nil {
			body := string(resp.Body())
			if body != "" {
				if !util.InList(body[0:1], "{", "[", "\"") {
					r := defs.RestStatusResponse{
						Status:  resp.StatusCode(),
						Message: strings.TrimSuffix(body, "\n"),
					}
					b, _ := json.Marshal(r)
					body = string(b)
				}

				err = json.Unmarshal([]byte(body), response)
				if err == nil && ui.IsActive(ui.RestLogger) {
					responseBytes, _ := json.MarshalIndent(response, "", "  ")

					ui.Debug(ui.RestLogger, "Response payload:\n%s", string(responseBytes))
				}

				if err == nil && status != http.StatusOK {
					if m, ok := response.(map[string]interface{}); ok {
						if msg, ok := m["Message"]; ok {
							err = errors.NewMessage(data.String(msg))
						}
					}
				}
			}
		}
	}

	if err != nil {
		err = errors.EgoError(err)
	}

	return err
}

func AddAgent(r *resty.Request, agentType string) {
	var version string

	if x, found := symbols.RootSymbolTable.Get("_version"); found {
		version = data.String(x)
	}

	platform := runtime.Version() + ", " + runtime.GOOS + ", " + runtime.GOARCH
	agent := "Ego " + version + " (" + platform + ") " + agentType

	r.Header.Add("User-Agent", agent)
	ui.Debug(ui.RestLogger, "User agent: %s", agent)
}

func GetTLSConfiguration() (*tls.Config, error) {
	// If we haven't ever set up the TLS configuration, let's do so now. This is a serialized
	// operation, so for most use cases, the existing TLS configuration is used. For the first
	// case, we will either set up an insecure TLS (not recommended) or will use the CRT and
	// KEY files found in the EGO PATH if present. If no cert files found, then it assumes it
	// should just use the native certs.
	tlsConfigurationMutex.Lock()

	if tlsConfiguration == nil {
		kind := "using certificate file"

		// If insecure is specified, then skip verification for TLS
		if os.Getenv("EGO_INSECURE_CLIENT") == defs.True {
			tlsConfiguration = &tls.Config{InsecureSkipVerify: true}
			kind = "skipping server verification"
		} else {
			// Is there a server cert file we can/should be using?
			filename := filepath.Join(settings.Get(defs.EgoPathSetting), ServerCertificateFile)
			if b, err := os.ReadFile(filename); err == nil {
				kind = kind + " " + filename
				roots := x509.NewCertPool()

				ok := roots.AppendCertsFromPEM(b)
				if !ok {
					ui.Debug(ui.RestLogger, "Failed to parse root certificate for client configuration")

					return nil, errors.EgoError(errors.ErrCertificateParseError).Context(filename)
				} else {
					tlsConfiguration = &tls.Config{RootCAs: roots}
				}
			} else {
				ui.Debug(ui.RestLogger, "Failed to read server certificate file: %v", err)

				return nil, errors.EgoError(err)
			}
		}

		ui.Debug(ui.RestLogger, "Client TLS %s", kind)
	}

	tlsConfigurationMutex.Unlock()

	return tlsConfiguration, nil
}
