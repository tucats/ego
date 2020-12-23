package runtime

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/go-resty/resty"
	"github.com/tucats/ego/defs"
	"github.com/tucats/gopackages/app-cli/persistence"
	"github.com/tucats/gopackages/symbols"
	"github.com/tucats/gopackages/util"
)

// RestNew implements the New() rest function
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
			client.SetAuthScheme(defs.AuthScheme)
			client.SetAuthToken(token)
		}
	}

	client.SetRedirectPolicy(resty.FlexibleRedirectPolicy(10))

	return map[string]interface{}{
		"client":     client,
		"Get":        RestGet,
		"Post":       RestPost,
		"Delete":     RestDelete,
		"Base":       RestBase,
		"Media":      RestMedia,
		"Token":      RestToken,
		"Auth":       RestAuth,
		"media_type": defs.JSONMediaType,
		"baseURL":    "",
		"response":   "",
		"status":     0,
		"headers":    map[string]interface{}{},
		"__readonly": true,
	}, nil
}

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

// RestBase implements the Base() rest function
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

// RestAuth implements the Auth() rest function
func RestAuth(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	r, err := getClient(s)
	if err != nil {
		return nil, err
	}
	this := getThis(s)
	if len(args) != 2 {
		return nil, errors.New("Incorrect number of arguments")
	}
	user := util.GetString(args[0])
	pass := util.GetString(args[1])
	r.SetBasicAuth(user, pass)
	return this, nil
}

// RestToken implements the Token() rest function
func RestToken(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	r, err := getClient(s)
	if err != nil {
		return nil, err
	}
	this := getThis(s)
	if len(args) > 1 {
		return nil, errors.New("Incorrect number of arguments")
	}

	token := persistence.Get("logon-token")
	if len(args) > 0 {
		token = util.GetString(args[0])
	}
	r.SetAuthToken(token)
	r.SetAuthScheme(defs.AuthScheme)
	return this, nil
}

// RestMedia implements the Media() function
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

// RestGet implements the rest Get() function
func RestGet(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	client, err := getClient(s)
	if err != nil {
		return nil, err
	}
	this := getThis(s)

	if len(args) != 1 {
		return nil, fmt.Errorf("incorrect number of arguments")
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
		this["status"] = 503
		return nil, err
	}

	this["cookies"] = fetchCookies(s, response)
	status := response.StatusCode()
	this["status"] = status
	this["headers"] = headerMap(response)

	rb := string(response.Body())
	if isJSON && ((status >= 200 && status <= 299) || strings.HasPrefix(rb, "{") || strings.HasPrefix(rb, "[")) {
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
// struct member names so "-" is converted to "_"
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

// RestPost implements the Post() rest function
func RestPost(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	client, err := getClient(s)
	if err != nil {
		return nil, err
	}
	this := getThis(s)
	if len(args) < 1 || len(args) > 2 {
		return nil, fmt.Errorf("incorrect number of arguments")
	}
	url := applyBaseURL(util.GetString(args[0]), this)
	var body interface{} = ""
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
		isJSON = (ms == defs.JSONMediaType)
		r.Header.Add("Accept", ms)
		r.Header.Add("Content_Type", ms)
	}
	response, err := r.Post(url)
	if err != nil {
		this["status"] = 503
		return nil, err
	}

	this["cookies"] = fetchCookies(s, response)
	status := response.StatusCode()
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

// RestDelete implements the Delete() rest function
func RestDelete(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	client, err := getClient(s)
	if err != nil {
		return nil, err
	}
	this := getThis(s)
	if len(args) < 1 || len(args) > 2 {
		return nil, fmt.Errorf("incorrect number of arguments")
	}
	url := applyBaseURL(util.GetString(args[0]), this)
	var body interface{} = ""
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
		this["status"] = 503
		return nil, err
	}

	this["cookies"] = fetchCookies(s, response)
	status := response.StatusCode()
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

// getClient searches the symbol table for the client receiver ("_this")
// variable, validates that it contains a REST client object, and returns
// the native client object.
func getClient(symbols *symbols.SymbolTable) (*resty.Client, error) {
	g, ok := symbols.Get("_this")
	if !ok {
		return nil, errors.New("no function reciver")
	}
	gc, ok := g.(map[string]interface{})
	if !ok {
		return nil, errors.New("not a valid rest client struct")
	}
	client, ok := gc["client"]
	if !ok {
		return nil, errors.New("no 'client' member found")
	}
	cp, ok := client.(*resty.Client)
	if !ok {
		return nil, errors.New("'client' is not a rest client pointer")
	}
	return cp, nil
}

// getThis returns a map for the "this" object in the current
// symbol table.
func getThis(s *symbols.SymbolTable) map[string]interface{} {
	t, ok := s.Get("_this")
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
		client.SetAuthScheme("Token")
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

	var resp *resty.Response
	var err error
	resp, err = r.Execute(method, url)
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
