package runtime

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strings"

	"github.com/go-resty/resty"
	"github.com/tucats/ego/app-cli/persistence"
	"github.com/tucats/ego/compiler"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

// Max number of times we will chase a redirect before failing.
const MaxRedirectCount = 10

// Do we allow outbound REST calls with invalid/insecure certificates?
var allowInsecure = false

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

var restType *datatypes.Type

func AllowInsecure(flag bool) {
	allowInsecure = flag

	os.Setenv("EGO_INSECURE_CLIENT", "true")
}

func initializeRestType() {
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

		restType = &t
	}
}

// RestNew implements the New() rest function.
func RestNew(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	client := resty.New()

	if len(args) == 2 {
		username := util.GetString(args[0])
		password := util.GetString(args[1])

		client.SetBasicAuth(username, password)
		client.SetDisableWarn(true)
	} else {
		token := persistence.Get(defs.LogonTokenSetting)
		if token != "" {
			client.SetAuthToken(token)
		}
	}

	initializeRestType()
	client.SetTLSClientConfig(&tls.Config{InsecureSkipVerify: allowInsecure})

	r := datatypes.NewStruct(*restType)

	_ = r.Set(clientFieldName, client)
	_ = r.Set(mediaTypeFieldName, defs.JSONMediaType)
	_ = r.Set(verifyFieldName, true)

	r.SetReadonly(true)

	return r, nil
}

// utility function that prepends the base URL for this instance
// of a rest service to the supplied URL string. If there is
// no base URL defined, then nothing is changed.
func applyBaseURL(url string, this *datatypes.EgoStruct) string {
	if b, ok := this.Get(baseURLFieldName); ok {
		base := util.GetString(b)
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

func RestStatusMessage(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if len(args) != 1 {
		return nil, errors.New(errors.ErrArgumentCount)
	}

	code := util.GetInt(args[0])
	if text, ok := httpStatusCodeMessages[code]; ok {
		return text, nil
	}

	return fmt.Sprintf("HTTP status %d", code), nil
}

func RestClose(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	c, err := getClient(s)
	if !errors.Nil(err) {
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
func VerifyServer(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	client, err := getClient(s)
	if !errors.Nil(err) {
		return nil, err
	}

	this := getThisStruct(s)
	verify := allowInsecure

	if len(args) == 1 {
		verify = util.GetBool(args[0])
	}

	client.SetTLSClientConfig(&tls.Config{InsecureSkipVerify: verify})

	this.SetAlways(verifyFieldName, verify)

	return this, nil
}

// RestBase implements the Base() rest function. This specifies a string that is used
// as the base prefix for any URL formed in a REST call. This lets you specify the
// protocol/host/port information once, and then have each Get(), Post(), etc. call
// just specify the endpoint.
func RestBase(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	_, err := getClient(s)
	if !errors.Nil(err) {
		return nil, err
	}

	this := getThisStruct(s)
	base := ""

	if len(args) > 0 {
		base = util.GetString(args[0])
	} else {
		base = persistence.Get(defs.LogonServerSetting)
	}

	this.SetAlways(baseURLFieldName, base)

	return this, nil
}

// RestDebug implements the Debug() rest function. This specifies a boolean value that
// enables or disables debug logging for the client.
func RestDebug(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	r, err := getClient(s)
	if !errors.Nil(err) {
		return nil, err
	}

	this := getThisStruct(s)

	flag := util.GetBool((args[0]))
	r.SetDebug(flag)

	return this, nil
}

// RestAuth implements the Auth() rest function. When present, it accepts a username and
// password as parameters, and sets the rest client to use BasicAuth authentication, where
// the username and password are part of an Authentication header.
func RestAuth(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	r, err := getClient(s)
	if !errors.Nil(err) {
		return nil, err
	}

	this := getThisStruct(s)

	if len(args) != 2 {
		return nil, errors.New(errors.ErrArgumentCount)
	}

	user := util.GetString(args[0])
	pass := util.GetString(args[1])

	r.SetBasicAuth(user, pass)

	return this, nil
}

// RestToken implements the Token() rest function. When present, it accepts a token string
// and sets the rest client to use Bearer token authentication using this token value.
func RestToken(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	r, err := getClient(s)
	if !errors.Nil(err) {
		return nil, err
	}

	this := getThisStruct(s)

	if len(args) > 1 {
		return nil, errors.New(errors.ErrArgumentCount)
	}

	token := persistence.Get(defs.LogonTokenSetting)

	if len(args) > 0 {
		token = util.GetString(args[0])
	}

	r.SetAuthToken(token)

	return this, nil
}

// RestMedia implements the Media() function. This specifies a string containing the media
// type that the REST service expects. In it's simplest form, this can be "application/text"
// for free text responses, or "application/json" for JSON data payloads.
func RestMedia(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	_, err := getClient(s)
	if !errors.Nil(err) {
		return nil, err
	}

	this := getThisStruct(s)
	media := util.GetString(args[0])
	this.SetAlways(mediaTypeFieldName, media)

	return this, nil
}

// RestGet implements the rest Get() function. This must be provided with a URL or
// URL fragment (depending on whether Base() was called). The URL is constructed, and
// authentication set, and a GET HTTP operation is generated. The result is either a
// string (for media type of text) or a struct (media type of JSON).
func RestGet(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	client, err := getClient(s)
	if !errors.Nil(err) {
		return nil, err
	}

	client.SetRedirectPolicy(resty.FlexibleRedirectPolicy(MaxRedirectCount))

	this := getThisStruct(s)

	if len(args) != 1 {
		return nil, errors.New(errors.ErrArgumentCount)
	}

	url := applyBaseURL(util.GetString(args[0]), this)
	r := client.NewRequest()
	isJSON := false

	if media, ok := this.Get(mediaTypeFieldName); ok {
		ms := util.GetString(media)
		isJSON = (strings.Contains(ms, defs.JSONMediaType))
		r.Header.Add("Accept", ms)
		r.Header.Add("Content-Type", ms)
	}

	AddAgent(r, defs.ClientAgent)

	response, e2 := r.Get(url)
	if e2 != nil {
		this.SetAlways(statusFieldName, http.StatusServiceUnavailable)

		return nil, errors.New(e2)
	}

	this.SetAlways("cookies", fetchCookies(s, response))
	status := response.StatusCode()
	this.SetAlways(statusFieldName, status)
	this.SetAlways(headersFieldName, headerMap(response))
	rb := string(response.Body())

	if isJSON && ((status >= http.StatusOK && status <= 299) || strings.HasPrefix(rb, "{") || strings.HasPrefix(rb, "[")) {
		var jsonResponse interface{}

		err := json.Unmarshal([]byte(rb), &jsonResponse)

		// For well-known complex types, make them Ego-native versions.
		switch actual := jsonResponse.(type) {
		case map[string]interface{}:
			jsonResponse = datatypes.NewMapFromMap(actual)

		case []interface{}:
			jsonResponse = datatypes.NewFromArray(datatypes.InterfaceType, actual)
		}

		this.SetAlways(responseFieldName, jsonResponse)

		return jsonResponse, errors.New(err)
	}

	this.SetAlways(responseFieldName, rb)

	return rb, nil
}

// fetchCookies extracts the cookies from the response, and format them as an Ego array
// of structs.
func fetchCookies(s *symbols.SymbolTable, r *resty.Response) *datatypes.EgoArray {
	cookies := r.Cookies()
	result := datatypes.NewArray(datatypes.InterfaceType, len(cookies))

	for i, v := range r.Cookies() {
		cookie := datatypes.NewMap(datatypes.StringType, datatypes.InterfaceType)

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
func headerMap(response *resty.Response) *datatypes.EgoMap {
	headers := datatypes.NewMap(datatypes.StringType, datatypes.InterfaceType)

	for k, v := range response.Header() {
		_, _ = headers.Set(k, strings.TrimPrefix(strings.TrimSuffix(fmt.Sprintf("%v", v), "]"), "["))
	}

	return headers
}

// RestPost implements the Post() rest function.
func RestPost(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	var body interface{} = ""

	if len(args) < 1 || len(args) > 2 {
		return nil, errors.New(errors.ErrArgumentCount)
	}

	client, err := getClient(s)
	if !errors.Nil(err) {
		return nil, err
	}

	client.SetRedirectPolicy()

	this := getThisStruct(s)
	url := applyBaseURL(util.GetString(args[0]), this)

	if len(args) > 1 {
		body = args[1]
	}

	// If the media type is json, then convert the value passed
	// into a json value for the request body.
	if mt, ok := this.Get(mediaTypeFieldName); ok {
		media := util.GetString(mt)
		if strings.Contains(media, defs.JSONMediaType) {
			b, err := json.Marshal(body)
			if !errors.Nil(err) {
				return nil, errors.New(err)
			}

			body = string(b)
		}
	}

	r := client.NewRequest().SetBody(body)
	isJSON := false

	if media, ok := this.Get(mediaTypeFieldName); ok {
		ms := util.GetString(media)
		isJSON = strings.Contains(ms, defs.JSONMediaType)

		r.Header.Add("Accept", ms)
		r.Header.Add("Content-Type", ms)
	}

	AddAgent(r, defs.ClientAgent)

	response, e2 := r.Post(url)
	if e2 != nil {
		this.SetAlways(statusFieldName, http.StatusServiceUnavailable)

		return nil, errors.New(e2)
	}

	status := response.StatusCode()
	this.SetAlways("cookies", fetchCookies(s, response))
	this.SetAlways(statusFieldName, status)
	this.SetAlways(headersFieldName, headerMap(response))
	rb := string(response.Body())

	if isJSON {
		var jsonResponse interface{}

		err := json.Unmarshal([]byte(rb), &jsonResponse)
		this.SetAlways(responseFieldName, jsonResponse)

		return jsonResponse, errors.New(err)
	}

	this.SetAlways(responseFieldName, rb)

	return rb, nil
}

// RestDelete implements the Delete() rest function.
func RestDelete(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	var body interface{} = ""

	if len(args) < 1 || len(args) > 2 {
		return nil, errors.New(errors.ErrArgumentCount)
	}

	client, err := getClient(s)
	if !errors.Nil(err) {
		return nil, err
	}

	this := getThisStruct(s)
	url := applyBaseURL(util.GetString(args[0]), this)

	if len(args) > 1 {
		body = args[1]
	}

	// If the media type is json, then convert the value passed
	// into a json value for the request body.
	if mt, ok := this.Get(mediaTypeFieldName); ok {
		media := util.GetString(mt)
		if strings.Contains(media, defs.JSONMediaType) {
			b, err := json.Marshal(body)
			if !errors.Nil(err) {
				return nil, errors.New(err)
			}

			body = string(b)
		}
	}

	r := client.NewRequest().SetBody(body)
	isJSON := false

	if media, ok := this.Get(mediaTypeFieldName); ok {
		ms := util.GetString(media)
		isJSON = (strings.Contains(ms, defs.JSONMediaType))

		r.Header.Add("Accept", ms)
		r.Header.Add("Content-Type", ms)
	}

	AddAgent(r, defs.ClientAgent)

	response, e2 := r.Delete(url)
	if e2 != nil {
		this.SetAlways(statusFieldName, http.StatusServiceUnavailable)

		return nil, errors.New(e2)
	}

	status := response.StatusCode()
	this.SetAlways("cookies", fetchCookies(s, response))
	this.SetAlways(statusFieldName, status)
	this.SetAlways(headersFieldName, headerMap(response))
	rb := string(response.Body())

	if isJSON {
		var jsonResponse interface{}

		err := json.Unmarshal([]byte(rb), &jsonResponse)
		this.SetAlways(responseFieldName, jsonResponse)

		return jsonResponse, errors.New(err)
	}

	this.SetAlways(responseFieldName, rb)

	return rb, nil
}

// getClient searches the symbol table for the client receiver ("__this")
// variable, validates that it contains a REST client object, and returns
// the native client object.
func getClient(symbols *symbols.SymbolTable) (*resty.Client, *errors.EgoError) {
	if g, ok := symbols.Get("__this"); ok {
		if gc, ok := g.(*datatypes.EgoStruct); ok {
			if client, ok := gc.Get(clientFieldName); ok {
				if cp, ok := client.(*resty.Client); ok {
					if cp == nil {
						return nil, errors.New(errors.ErrRestClientClosed)
					}

					return cp, nil
				}
			}
		}
	}

	return nil, errors.New(errors.ErrNoFunctionReceiver)
}

// getThis returns a map for the "this" object in the current
// symbol table.
func getThisStruct(s *symbols.SymbolTable) *datatypes.EgoStruct {
	t, ok := s.Get("__this")
	if !ok {
		return nil
	}

	this, ok := t.(*datatypes.EgoStruct)
	if !ok {
		return nil
	}

	return this
}

// Exchange is a helper wrapper around a rest call.
func Exchange(endpoint, method string, body interface{}, response interface{}, agentType string) *errors.EgoError {
	var resp *resty.Response

	var err error

	url := persistence.Get(defs.ApplicationServerSetting)
	if url == "" {
		url = persistence.Get(defs.LogonServerSetting)
	}

	if url == "" {
		url = "http://localhost:8080"
	}

	url = strings.TrimSuffix(url, "/") + endpoint
	client := resty.New().SetRedirectPolicy(resty.FlexibleRedirectPolicy(MaxRedirectCount))

	if token := persistence.Get(defs.LogonTokenSetting); token != "" {
		client.SetAuthToken(token)
	}

	if os.Getenv("EGO_INSECURE_CLIENT") == "true" {
		client.SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true})
	}

	r := client.NewRequest()

	r.Header.Add("Accept", defs.JSONMediaType)
	r.Header.Add("Content-Type", defs.JSONMediaType)

	AddAgent(r, agentType)

	if body != nil {
		b, err := json.Marshal(body)
		if !errors.Nil(err) {
			return errors.New(err)
		}

		r.SetBody(b)
	}

	resp, err = r.Execute(method, url)
	status := resp.StatusCode()

	switch status {
	case http.StatusForbidden:
		err = errors.New(errors.ErrNoPrivilegeForOperation)

	case http.StatusNotFound:
		err = errors.New(errors.ErrNotFound)
	}

	if errors.Nil(err) && status != 200 && response == nil {
		return errors.New(errors.ErrHTTP).Context(status)
	}

	if errors.Nil(err) && response != nil {
		body := string(resp.Body())
		if !util.InList(body[0:1], "{", "[", "\"") {
			r := defs.RestResponse{
				Status:  resp.StatusCode(),
				Message: strings.TrimSuffix(body, "\n"),
			}
			b, _ := json.Marshal(r)
			body = string(b)
		}

		err = json.Unmarshal([]byte(body), response)
	}

	return errors.New(err)
}

func AddAgent(r *resty.Request, agentType string) {
	var version string

	if x, found := symbols.RootSymbolTable.Get("_version"); found {
		version = util.GetString(x)
	}

	platform := runtime.Version() + ", " + runtime.GOOS + ", " + runtime.GOARCH
	agent := "Ego " + version + " (" + platform + ") " + agentType

	r.Header.Add("User-Agent", agent)
}
