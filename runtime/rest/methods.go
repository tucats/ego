package rest

import (
	"crypto/tls"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-resty/resty"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// Get implements the rest Get() function. This must be provided with a URL or
// URL fragment (depending on whether Base() was called). The URL is constructed, and
// authentication set, and a GET HTTP operation is generated. The result is either a
// string (for media type of text) or a struct (media type of JSON).
func Get(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	client, err := getClient(s)
	if err != nil {
		return nil, err
	}

	this := getThis(s)

	client.SetRedirectPolicy(resty.FlexibleRedirectPolicy(MaxRedirectCount))

	if !data.Bool(this.GetAlways("verify")) {
		client.SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true})
	}

	if len(args) != 1 {
		return nil, errors.ErrArgumentCount
	}

	url := applyBaseURL(data.String(args[0]), this)
	r := client.NewRequest()
	isJSON := false

	if media := this.GetAlways(mediaTypeFieldName); media != nil {
		ms := data.String(media)
		isJSON = (strings.Contains(ms, defs.JSONMediaType))
		r.Header.Add("Accept", ms)
		r.Header.Add("Content-Type", ms)
	}

	AddAgent(r, defs.ClientAgent)

	response, e2 := r.Get(url)
	if e2 != nil {
		this.SetAlways(statusFieldName, http.StatusServiceUnavailable)

		return nil, errors.NewError(e2)
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
			err = errors.NewError(err)
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

// Post implements the Post() rest function.
func Post(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	var body interface{} = ""

	if len(args) < 1 || len(args) > 2 {
		return nil, errors.ErrArgumentCount
	}

	client, err := getClient(s)
	if err != nil {
		return nil, err
	}

	this := getThis(s)

	client.SetRedirectPolicy()

	if !data.Bool(this.GetAlways("verify")) {
		client.SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true})
	}

	url := applyBaseURL(data.String(args[0]), this)

	if len(args) > 1 {
		body = args[1]
	}

	// If the media type is json, then convert the value passed
	// into a json value for the request body.
	if mt := this.GetAlways(mediaTypeFieldName); mt != nil {
		media := data.String(mt)
		if strings.Contains(media, defs.JSONMediaType) {
			b, err := json.Marshal(body)
			if err != nil {
				return nil, errors.NewError(err)
			}

			body = string(b)
		}
	}

	r := client.NewRequest().SetBody(body)
	isJSON := false

	if media := this.GetAlways(mediaTypeFieldName); media != nil {
		ms := data.String(media)
		isJSON = strings.Contains(ms, defs.JSONMediaType)

		r.Header.Add("Accept", ms)
		r.Header.Add("Content-Type", ms)
	}

	AddAgent(r, defs.ClientAgent)

	response, e2 := r.Post(url)
	if e2 != nil {
		this.SetAlways(statusFieldName, http.StatusServiceUnavailable)

		return nil, errors.NewError(e2)
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
			err = errors.NewError(err)
		}

		this.SetAlways(responseFieldName, jsonResponse)

		return jsonResponse, err
	}

	this.SetAlways(responseFieldName, rb)

	return rb, nil
}

// Delete implements the Delete() rest function.
func Delete(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	var body interface{} = ""

	if len(args) < 1 || len(args) > 2 {
		return nil, errors.ErrArgumentCount
	}

	client, err := getClient(s)
	if err != nil {
		return nil, err
	}

	this := getThis(s)

	client.SetRedirectPolicy()

	if !data.Bool(this.GetAlways("verify")) {
		client.SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true})
	}

	url := applyBaseURL(data.String(args[0]), this)

	if len(args) > 1 {
		body = args[1]
	}

	// If the media type is json, then convert the value passed
	// into a json value for the request body.
	if mt := this.GetAlways(mediaTypeFieldName); mt != nil {
		media := data.String(mt)
		if strings.Contains(media, defs.JSONMediaType) {
			b, err := json.Marshal(body)
			if err != nil {
				return nil, errors.NewError(err)
			}

			body = string(b)
		}
	}

	r := client.NewRequest().SetBody(body)
	isJSON := false

	if media := this.GetAlways(mediaTypeFieldName); media != nil {
		ms := data.String(media)
		isJSON = (strings.Contains(ms, defs.JSONMediaType))

		r.Header.Add("Accept", ms)
		r.Header.Add("Content-Type", ms)
	}

	AddAgent(r, defs.ClientAgent)

	response, e2 := r.Delete(url)
	if e2 != nil {
		this.SetAlways(statusFieldName, http.StatusServiceUnavailable)

		return nil, errors.NewError(e2)
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
			err = errors.NewError(err)
		}

		this.SetAlways(responseFieldName, jsonResponse)

		return jsonResponse, err
	}

	this.SetAlways(responseFieldName, rb)

	return rb, nil
}
