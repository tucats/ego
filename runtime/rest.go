package runtime

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-resty/resty"
	"github.com/tucats/ego/app-cli/persistence"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

// This maps HTTP status codes to a message string.
var codes = map[int]string{
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

// RestNew implements the New() rest function.
func RestNew(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
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

	return map[string]interface{}{
		"client":        client,
		"Close":         RestClose,
		"Get":           RestGet,
		"Post":          RestPost,
		"Delete":        RestDelete,
		"Base":          RestBase,
		"Media":         RestMedia,
		"Token":         RestToken,
		"Auth":          RestAuth,
		"Verify":        VerifyServer,
		"StatusMessage": RestStatusMessage,
		"media_type":    defs.JSONMediaType,
		"baseURL":       "",
		"response":      "",
		"status":        0,
		"verify":        true,
		"headers":       map[string]interface{}{},
		datatypes.MetadataKey: map[string]interface{}{
			datatypes.TypeMDKey:     "rest",
			datatypes.ReadonlyMDKey: true,
		},
	}, nil
}

// utility function that prepends the base URL for this instance
// of a rest service to the supplied URL string. If there is
// no base URL defined, then nothing is changed.
func applyBaseURL(url string, this map[string]interface{}) string {
	if b, ok := this["baseURL"]; ok {
		base := util.GetString(b)
		if base == "" {
			return url
		}

		if strings.HasSuffix(base, "/") {
			base = base[:len(base)-1]
		}

		if !strings.HasPrefix(url, "/") {
			url = "/" + url
		}

		url = base + url
	}

	return url
}

func RestStatusMessage(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) != 1 {
		return nil, errors.New(defs.IncorrectArgumentCount)
	}

	code := util.GetInt(args[0])
	if text, ok := codes[code]; ok {
		return text, nil
	}

	return fmt.Sprintf("HTTP status %d", code), nil
}

func RestClose(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	c, err := getClient(s)
	if err != nil {
		return nil, err
	}

	c.GetClient().CloseIdleConnections()

	this := getThis(s)
	c = nil
	this["client"] = nil
	this["Close"] = released
	this["Get"] = released
	this["Post"] = released
	this["Delete"] = released
	this["Base"] = released
	this["Media"] = released
	this["Auth"] = released
	this["Token"] = released
	this["Verify"] = released
	this["StatusMessage"] = released
	this["status"] = 0

	return true, nil
}

// RestBase implements the Base() rest function.
func VerifyServer(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	client, err := getClient(s)
	if err != nil {
		return nil, err
	}

	this := getThis(s)
	verify := true

	if len(args) == 1 {
		verify = util.GetBool(args[0])
	}

	client.SetTLSClientConfig(&tls.Config{InsecureSkipVerify: verify})

	this["verify"] = verify

	return this, nil
}

// RestBase implements the Base() rest function.
func RestBase(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	_, err := getClient(s)
	if err != nil {
		return nil, err
	}

	this := getThis(s)
	base := ""

	if len(args) > 0 {
		base = util.GetString(args[0])
	} else {
		base = persistence.Get(defs.LogonServerSetting)
	}

	this["baseURL"] = base

	return this, nil
}

// RestAuth implements the Auth() rest function.
func RestAuth(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	r, err := getClient(s)
	if err != nil {
		return nil, err
	}

	this := getThis(s)

	if len(args) != 2 {
		return nil, errors.New(defs.IncorrectArgumentCount)
	}

	user := util.GetString(args[0])
	pass := util.GetString(args[1])

	r.SetBasicAuth(user, pass)

	return this, nil
}

// RestToken implements the Token() rest function.
func RestToken(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	r, err := getClient(s)
	if err != nil {
		return nil, err
	}

	this := getThis(s)

	if len(args) > 1 {
		return nil, errors.New(defs.IncorrectArgumentCount)
	}

	token := persistence.Get(defs.LogonTokenSetting)

	if len(args) > 0 {
		token = util.GetString(args[0])
	}

	r.SetAuthToken(token)

	return this, nil
}

// RestMedia implements the Media() function.
func RestMedia(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	_, err := getClient(s)
	if err != nil {
		return nil, err
	}

	this := getThis(s)
	media := util.GetString(args[0])
	this["media_type"] = media

	return this, nil
}

// RestGet implements the rest Get() function.
func RestGet(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	client, err := getClient(s)
	if err != nil {
		return nil, err
	}

	client.SetRedirectPolicy(resty.FlexibleRedirectPolicy(10))

	this := getThis(s)

	if len(args) != 1 {
		return nil, errors.New(defs.IncorrectArgumentCount)
	}

	url := applyBaseURL(util.GetString(args[0]), this)
	r := client.NewRequest()
	isJSON := false

	if media, ok := this["media_type"]; ok {
		ms := util.GetString(media)
		isJSON = (strings.Contains(ms, defs.JSONMediaType))
		r.Header.Add("Accept", ms)
		r.Header.Add("Content_Type", ms)
	}

	response, err := r.Get(url)
	if err != nil {
		this["status"] = http.StatusServiceUnavailable

		return nil, err
	}

	this["cookies"] = fetchCookies(s, response)
	status := response.StatusCode()
	this["status"] = status
	this["headers"] = headerMap(response)
	rb := string(response.Body())

	if isJSON && ((status >= http.StatusOK && status <= 299) || strings.HasPrefix(rb, "{") || strings.HasPrefix(rb, "[")) {
		var jsonResponse interface{}

		err := json.Unmarshal([]byte(rb), &jsonResponse)
		this["response"] = jsonResponse

		return jsonResponse, err
	}

	this["response"] = rb

	return rb, nil
}

// Extract the cookies from the response, and format them as an Ego array
// of structs.
func fetchCookies(s *symbols.SymbolTable, r *resty.Response) []interface{} {
	cookies := r.Cookies()
	result := make([]interface{}, len(cookies))

	for i, v := range r.Cookies() {
		cookie := map[string]interface{}{}
		cookie["expires"] = v.Expires.String()
		cookie["name"] = v.Name
		cookie["domain"] = v.Domain
		cookie["value"] = v.Value
		cookie["path"] = v.Path
		result[i] = cookie
	}

	return result
}

// headerMap is a support function that extracts the header data from a
// rest response, and formats it to be an Ego struct. It also mangles
// struct member names so "-" is converted to "_".
func headerMap(response *resty.Response) map[string]interface{} {
	headers := map[string]interface{}{}

	for k, v := range response.Header() {
		k = strings.ReplaceAll(k, "-", "_")
		vs := fmt.Sprintf("%v", v)
		vs = strings.TrimPrefix(strings.TrimSuffix(vs, "]"), "[")
		headers[k] = vs
	}

	return headers
}

// RestPost implements the Post() rest function.
func RestPost(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	var body interface{} = ""

	if len(args) < 1 || len(args) > 2 {
		return nil, errors.New(defs.IncorrectArgumentCount)
	}

	client, err := getClient(s)
	if err != nil {
		return nil, err
	}

	client.SetRedirectPolicy()

	this := getThis(s)
	url := applyBaseURL(util.GetString(args[0]), this)

	if len(args) > 1 {
		body = args[1]
	}

	// If the media type is json, then convert the value passed
	// into a json value for the request body.
	if mt, ok := this["media_type"]; ok {
		media := util.GetString(mt)
		if strings.Contains(media, defs.JSONMediaType) {
			b, err := json.Marshal(body)
			if err != nil {
				return nil, err
			}

			body = string(b)
		}
	}

	r := client.NewRequest().SetBody(body)
	isJSON := false

	if media, ok := this["media_type"]; ok {
		ms := util.GetString(media)
		isJSON = strings.Contains(ms, defs.JSONMediaType)

		r.Header.Add("Accept", ms)
		r.Header.Add("Content_Type", ms)
	}

	response, err := r.Post(url)
	if err != nil {
		this["status"] = http.StatusServiceUnavailable

		return nil, err
	}

	status := response.StatusCode()
	this["cookies"] = fetchCookies(s, response)
	this["status"] = status
	this["headers"] = headerMap(response)
	rb := string(response.Body())

	if isJSON {
		var jsonResponse interface{}

		err := json.Unmarshal([]byte(rb), &jsonResponse)
		this["response"] = jsonResponse

		return jsonResponse, err
	}

	this["response"] = rb

	return rb, nil
}

// RestDelete implements the Delete() rest function.
func RestDelete(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	var body interface{} = ""

	if len(args) < 1 || len(args) > 2 {
		return nil, errors.New(defs.IncorrectArgumentCount)
	}

	client, err := getClient(s)
	if err != nil {
		return nil, err
	}

	this := getThis(s)
	url := applyBaseURL(util.GetString(args[0]), this)

	if len(args) > 1 {
		body = args[1]
	}

	// If the media type is json, then convert the value passed
	// into a json value for the request body.
	if mt, ok := this["media_type"]; ok {
		media := util.GetString(mt)
		if strings.Contains(media, defs.JSONMediaType) {
			b, err := json.Marshal(body)
			if err != nil {
				return nil, err
			}

			body = string(b)
		}
	}

	r := client.NewRequest().SetBody(body)
	isJSON := false

	if media, ok := this["media_type"]; ok {
		ms := util.GetString(media)
		isJSON = (strings.Contains(ms, defs.JSONMediaType))

		r.Header.Add("Accept", ms)
		r.Header.Add("Content_Type", ms)
	}

	response, err := r.Delete(url)
	if err != nil {
		this["status"] = http.StatusServiceUnavailable

		return nil, err
	}

	status := response.StatusCode()
	this["cookies"] = fetchCookies(s, response)
	this["status"] = status
	this["headers"] = headerMap(response)
	rb := string(response.Body())

	if isJSON {
		var jsonResponse interface{}

		err := json.Unmarshal([]byte(rb), &jsonResponse)
		this["response"] = jsonResponse

		return jsonResponse, err
	}

	this["response"] = rb

	return rb, nil
}

// getClient searches the symbol table for the client receiver ("__this")
// variable, validates that it contains a REST client object, and returns
// the native client object.
func getClient(symbols *symbols.SymbolTable) (*resty.Client, error) {
	if g, ok := symbols.Get("__this"); ok {
		if gc, ok := g.(map[string]interface{}); ok {
			if client, ok := gc["client"]; ok {
				if cp, ok := client.(*resty.Client); ok {
					if cp == nil {
						return nil, errors.New("rest client was closed")
					}

					return cp, nil
				}
			}
		}
	}

	return nil, errors.New(defs.NoFunctionReceiver)
}

func released(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	return nil, errors.New("rest client closed")
}

// getThis returns a map for the "this" object in the current
// symbol table.
func getThis(s *symbols.SymbolTable) map[string]interface{} {
	t, ok := s.Get("__this")
	if !ok {
		return nil
	}

	this, ok := t.(map[string]interface{})
	if !ok {
		return nil
	}

	return this
}

// Exchange is a helper wrapper around a rest call.
func Exchange(endpoint, method string, body interface{}, response interface{}) error {
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
	client := resty.New().SetRedirectPolicy(resty.FlexibleRedirectPolicy(10))

	if token := persistence.Get(defs.LogonTokenSetting); token != "" {
		client.SetAuthToken(token)
	}

	r := client.NewRequest()

	r.Header.Add("Accept", defs.JSONMediaType)
	r.Header.Add("Content_Type", defs.JSONMediaType)

	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}

		r.SetBody(b)
	}

	resp, err = r.Execute(method, url)
	status := resp.StatusCode()

	switch status {
	case http.StatusForbidden:
		err = errors.New(defs.NoPrivilegeForOperation)

	case http.StatusNotFound:
		err = errors.New(defs.NotFound)
	}

	if err == nil && response != nil {
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

	return err
}
