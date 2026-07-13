package rest

import (
	"crypto/tls"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/symbols"
	"github.com/tucats/ego/internal/util"
	"gopkg.in/resty.v1"
)

// doGet implements the rest doGet() function. This must be provided with a URL or
// URL fragment (depending on whether Base() was called). The URL is constructed, and
// authentication set, and a GET HTTP operation is generated. The result is either a
// string (for media type of text) or a struct (media type of JSON).
func doGet(s *symbols.SymbolTable, args data.List) (any, error) {
	client, err := getClient(s)
	if err != nil {
		return data.NewList(nil, err), err
	}

	this := getThis(s)

	// Set the client to follow redirects, but limit the number of redirects.
	client.SetRedirectPolicy(resty.FlexibleRedirectPolicy(MaxRedirectCount))

	if !data.BoolOrFalse(this.GetAlways("verify")) {
		client.SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true})
		ui.Log(ui.RestLogger, "rest.tls.insecure", nil)
	}

	url := applyBaseURL(data.String(args.Get(0)), this)
	r := client.NewRequest()

	isJSON := false

	if media := this.GetAlways(mediaTypeFieldName); media != nil {
		ms := data.String(media)
		isJSON = (strings.Contains(ms, defs.JSONMediaType))
		r.Header.Add("Accept", ms)
		r.Header.Add("Content-Type", ms)
	}

	AddAgent(r, defs.ClientAgent)

	logRequest(r, "GET", url)
	response, e2 := r.Get(url)

	if e2 != nil {
		this.SetAlways(statusFieldName, http.StatusServiceUnavailable)

		err = errors.New(e2)

		return data.NewList(nil, err), err
	}

	logResponse(response)

	this.SetAlways(cookiesFieldName, fetchCookies(response))
	status := response.StatusCode()
	this.SetAlways(statusFieldName, status)
	this.SetAlways(headersFieldName, headerMap(response))
	rb := string(response.Body())

	if isJSON && ((status >= http.StatusOK && status <= 299) || strings.HasPrefix(rb, "{") || strings.HasPrefix(rb, "[")) {
		var jsonResponse any

		if len(rb) > 0 {
			err = json.Unmarshal([]byte(rb), &jsonResponse)
			if err != nil {
				err = errors.New(err)
			}

			// For well-known complex types, make them Ego-native versions.
			switch actual := jsonResponse.(type) {
			case map[string]any:
				jsonResponse = data.NewMapFromMap(actual)

			case []any:
				jsonResponse = data.NewArrayFromInterfaces(data.InterfaceType, actual...)
			}
		} else {
			jsonResponse = ""
		}

		this.SetAlways(responseFieldName, jsonResponse)

		return data.NewList(jsonResponse, err), err
	}

	this.SetAlways(responseFieldName, rb)

	return data.NewList(rb, nil), nil
}

// doPost implements the doPost() rest function.
func doPost(s *symbols.SymbolTable, args data.List) (any, error) {
	var body any = ""

	client, err := getClient(s)
	if err != nil {
		return data.NewList(nil, err), err
	}

	this := getThis(s)

	client.SetRedirectPolicy(resty.FlexibleRedirectPolicy(MaxRedirectCount))

	if tlsConf, err := GetTLSConfiguration(); err != nil {
		return data.NewList(nil, err), err
	} else {
		client.SetTLSClientConfig(tlsConf)
	}

	if !data.BoolOrFalse(this.GetAlways("verify")) {
		client.SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true})
		ui.Log(ui.RestLogger, "rest.tls.insecure", nil)
	}

	url := applyBaseURL(data.String(args.Get(0)), this)

	if args.Len() > 1 {
		body = args.Get(1)
	}

	// If the media type is json, then convert the value passed
	// into a json value for the request body.
	if mt := this.GetAlways(mediaTypeFieldName); mt != nil {
		media := data.String(mt)
		if strings.Contains(media, defs.JSONMediaType) {
			body = makeBodyFromEgoType(body)
		}
	}

	r := client.NewRequest()
	switch actual := body.(type) {
	default:
		r.Body = actual
	}

	r.SetContentLength(true)

	isJSON := false

	if media := this.GetAlways(mediaTypeFieldName); media != nil {
		ms := data.String(media)
		isJSON = strings.Contains(ms, defs.JSONMediaType)

		r.Header.Set("Accept", ms)
		r.Header.Set("Content-Type", ms+"; charset=utf-8")
	}

	AddAgent(r, defs.ClientAgent)
	logRequest(r, "POST", url)

	response, e2 := r.Post(url)
	if e2 != nil {
		this.SetAlways(statusFieldName, http.StatusServiceUnavailable)

		err = errors.New(e2)

		return data.NewList(nil, err), err
	}

	logResponse(response)

	status := response.StatusCode()
	this.SetAlways(cookiesFieldName, fetchCookies(response))
	this.SetAlways(statusFieldName, status)
	this.SetAlways(headersFieldName, headerMap(response))
	rb := string(response.Body())

	if isJSON {
		var jsonResponse any

		if len(rb) > 0 {
			err = json.Unmarshal([]byte(rb), &jsonResponse)
			if err != nil {
				err = errors.New(err)
			}

			jsonResponse = makeEgoTypeFromBody(jsonResponse)
		}

		this.SetAlways(responseFieldName, jsonResponse)

		return data.NewList(jsonResponse, err), err
	}

	this.SetAlways(responseFieldName, rb)

	return data.NewList(rb, nil), nil
}

// doDelete implements the doDelete() rest function.
func doDelete(s *symbols.SymbolTable, args data.List) (any, error) {
	var body any = ""

	client, err := getClient(s)
	if err != nil {
		return data.NewList(nil, err), err
	}

	this := getThis(s)

	client.SetRedirectPolicy(resty.FlexibleRedirectPolicy(MaxRedirectCount))

	if !data.BoolOrFalse(this.GetAlways("verify")) {
		client.SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true})
		ui.Log(ui.RestLogger, "rest.tls.insecure", nil)
	}

	url := applyBaseURL(data.String(args.Get(0)), this)

	if args.Len() > 1 {
		body = args.Get(1)
	}

	// If the media type is json, then convert the value passed
	// into a json value for the request body.
	if mt := this.GetAlways(mediaTypeFieldName); mt != nil {
		media := data.String(mt)
		if strings.Contains(media, defs.JSONMediaType) {
			b, err := json.Marshal(body)
			if err != nil {
				err = errors.New(err)

				return data.NewList(nil, err), err
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
	logRequest(r, "DELETE", url)

	response, e2 := r.Delete(url)
	if e2 != nil {
		this.SetAlways(statusFieldName, http.StatusServiceUnavailable)

		err = errors.New(e2)

		return data.NewList(nil, err), err
	}

	logResponse(response)

	status := response.StatusCode()
	this.SetAlways(cookiesFieldName, fetchCookies(response))
	this.SetAlways(statusFieldName, status)
	this.SetAlways(headersFieldName, headerMap(response))
	rb := string(response.Body())

	if isJSON {
		var jsonResponse any

		if len(rb) > 0 {
			err = json.Unmarshal([]byte(rb), &jsonResponse)
			if err != nil {
				err = errors.New(err)
			}

			// Convert well-known complex types (objects, arrays) to their
			// Ego-native equivalents, matching doGet()/doPost() -- without
			// this, a JSON object/array response was a raw Go
			// map[string]any/[]any that Ego code could not index or
			// otherwise use at all ("invalid or unsupported data type").
			jsonResponse = makeEgoTypeFromBody(jsonResponse)
		}

		this.SetAlways(responseFieldName, jsonResponse)

		return data.NewList(jsonResponse, err), err
	}

	this.SetAlways(responseFieldName, rb)

	return data.NewList(rb, nil), nil
}

func logRequest(r *resty.Request, method, url string) {
	if !ui.IsActive(ui.RestLogger) {
		return
	}

	for headerName, headerValues := range r.Header {
		if util.NonSensitiveHeader(headerName) {
			ui.Log(ui.RestLogger, "rest.request.header", ui.A{
				"name":  headerName,
				"value": headerValues})
		}
	}

	if r.Body != nil {
		ui.Log(ui.RestLogger, "rest.request.payload", ui.A{
			"body": r.Body})
	}

	ui.Log(ui.RestLogger, "rest.method", ui.A{
		"method":   strings.ToUpper(method),
		"endpoint": url})
}

func logResponse(r *resty.Response) {
	if !ui.IsActive(ui.RestLogger) {
		return
	}

	bodyAsText := false

	ui.Log(ui.RestLogger, "rest.status", ui.A{
		"status": r.Status()})

	for headerName, headerValues := range r.Header() {
		if util.NonSensitiveHeader(headerName) {
			if strings.EqualFold(headerName, "Content-Type") {
				for _, contentType := range headerValues {
					if strings.Contains(contentType, defs.JSONMediaType) {
						bodyAsText = true
					} else if strings.Contains(contentType, defs.TextMediaType) {
						bodyAsText = true
					}
				}
			}

			ui.Log(ui.RestLogger, "rest.response.header", ui.A{
				"name":  headerName,
				"value": headerValues})
		}
	}

	for _, v := range r.Cookies() {
		ui.Log(ui.RestLogger, "rest.cookie", ui.A{
			"cookie": v})
	}

	if len(r.Body()) > 0 {
		if bodyAsText {
			ui.Log(ui.RestLogger, "rest.response.body.text", ui.A{
				"body": string(r.Body())})
		} else {
			ui.Log(ui.RestLogger, "rest.response.body.bytes", ui.A{
				"body": r.Body()})
		}
	}
}

func makeBodyFromEgoType(v any) any {
	switch actual := v.(type) {
	case *data.Array:
		return actual.BaseArray()

	case *data.Map:
		return actual.ToMap()

	case *data.Struct:
		return actual.ToMap()

	default:
		return actual
	}
}

func makeEgoTypeFromBody(v any) any {
	switch actual := v.(type) {
	case []any:
		return data.NewArrayFromInterfaces(data.InterfaceType, actual...)

	case map[string]any:
		return data.NewMapFromMap(actual)

	default:
		return v
	}
}
