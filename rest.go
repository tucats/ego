package main

import (
	"errors"
	"fmt"

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
		token := persistence.Get("login-token")
		if token != "" {
			client.SetAuthToken(token)
		}
	}

	client.SetRedirectPolicy(resty.FlexibleRedirectPolicy(10))

	return map[string]interface{}{
		"client":     client,
		"get":        RestGet,
		"post":       RestPost,
		"response":   "",
		"status":     0,
		"__readonly": true,
	}, nil
}

func RestGet(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	client, err := getClient(s)
	if err != nil {
		return nil, err
	}

	if len(args) != 1 {
		return nil, fmt.Errorf("incorrect number of arguments")
	}
	url := util.GetString((args[0]))

	response, err := client.NewRequest().Get(url)
	if err != nil {
		return nil, err
	}
	t, _ := s.Get("_this")
	this := t.(map[string]interface{})
	this["status"] = response.StatusCode()
	this["response"] = string(response.Body())
	return this, nil
}

func RestPost(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	client, err := getClient(s)
	if err != nil {
		return nil, err
	}

	if len(args) < 1 || len(args) > 2 {
		return nil, fmt.Errorf("incorrect number of arguments")
	}
	url := util.GetString((args[0]))
	var body interface{} = ""
	if len(args) > 1 {
		body = args[1]
	}

	response, err := client.NewRequest().SetBody(body).Post(url)
	if err != nil {
		return nil, err
	}
	t, _ := s.Get("_this")
	this := t.(map[string]interface{})
	this["status"] = response.StatusCode()
	this["response"] = response.Body()
	return this, nil
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
