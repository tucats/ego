package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-resty/resty"
	"github.com/tucats/gopackages/app-cli/persistence"
	"github.com/tucats/gopackages/symbols"
	"github.com/tucats/gopackages/util"
)

func RestOpen(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {

	client := resty.New()

	if len(args) == 2 {
		username := util.GetString(args[0])
		password := util.GetString(args[1])
		client.SetBasicAuth(username, password)
		client.SetDisableWarn(true)
	} else {
		token := persistence.Get("logon-token")
		if token != "" {
			client.SetAuthScheme("Token")
			client.SetAuthToken(token)
		}
	}

	client.SetRedirectPolicy(resty.FlexibleRedirectPolicy(10))

	return map[string]interface{}{
		"client":     client,
		"get":        RestGet,
		"post":       RestPost,
		"base":       RestBase,
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

func RestBase(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	_, err := getClient(s)
	if err != nil {
		return nil, err
	}
	this := getThis(s)
	base := util.GetString(args[0])

	this["baseURL"] = base
	return this, nil
}

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
	response, err := client.NewRequest().Get(url)
	if err != nil {
		this["status"] = 503
		return nil, err
	}
	this["status"] = response.StatusCode()
	this["headers"] = response.Header()
	rb := string(response.Body())
	this["response"] = rb
	return rb, nil
}

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

	response, err := client.NewRequest().SetBody(body).Post(url)
	if err != nil {
		this["status"] = 503
		return nil, err
	}

	this["status"] = response.StatusCode()
	this["headers"] = response.Header()
	rb := string(response.Body())
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
