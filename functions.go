package main

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/go-gremlin/gremlin"
	"github.com/northwesternmutual/grammes"
	"github.com/tucats/gopackages/expressions"
	"github.com/tucats/gopackages/symbols"
	"github.com/tucats/gopackages/util"
)

// FunctionPi implements the pi() function
func FunctionPi(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) > 0 {
		return nil, errors.New("too many arguments to pi()")
	}
	return 3.1415926535, nil
}

// FunctionEval implements the pi() function
func FunctionEval(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) != 1 {
		return nil, errors.New("wrong number of arguments")
	}
	exprString := util.GetString(args[0])
	e := expressions.New(exprString)
	v, err := e.Eval(symbols)
	return v, err
}

// FunctionGremlinOpen opens a gremlin connetion and stores it in the result value
func FunctionGremlinOpen(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {

	var username, password string

	url := util.GetString(args[0])
	if len(args) > 1 {
		username = util.GetString(args[1])
		if len(args) > 2 {
			password = util.GetString(args[2])
		}
	}

	// Are we using the grammes client?
	if true {
		client, err := grammes.DialWithWebSocket(url)
		return client, err
	}
	var auth gremlin.OptAuth

	if username != "" {
		auth = gremlin.OptAuthUserPass(username, password)
		return gremlin.NewClient(url, auth)
	}
	return gremlin.NewClient(url)

}

// FunctionGremlinQuery executes a string query against an open client
func FunctionGremlinQuery(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {

	if len(args) != 2 {
		return nil, errors.New("incorrect number of arguments")
	}
	query := util.GetString(args[1])
	client, ok := args[0].(*gremlin.Client)
	if ok {
		res, err := client.ExecQuery(query)
		if err != nil {
			return nil, err
		}
		return gremlinResult(string(res))
	}
	grammes, ok := args[0].(*grammes.Client)
	if !ok {
		return nil, errors.New("not a valid gremlin client")
	}
	res, err := grammes.ExecuteStringQuery(query)
	if err != nil {
		return nil, err
	}
	return gremlinResult(string(res[0]))
}

func gremlinResult(str string) (interface{}, error) {

	var r interface{}
	fmt.Printf("DEBUG: %s\n", str)
	err := json.Unmarshal([]byte(str), &r)
	if err != nil {
		return nil, err
	}
	return gremlinResultValue(r)
}

func gremlinResultValue(i interface{}) (interface{}, error) {

	switch m := i.(type) {
	case nil:
		return nil, nil

	case string:
		return m, nil

	case map[string]interface{}:
		v := m["@value"]
		switch m["@type"] {
		case "g:List":
			return gremlinResultArray(v)
		case "g:Map":
			return gremlinResultMapList(v)
		case "g:Vertex":
			return gremlinResultMap(v)
		case "g:Int64":
			return util.GetInt(v), nil
		default:
			return nil, fmt.Errorf("unexpected item type %s", m["@type"])
		}
	default:
		return nil, fmt.Errorf("unexpected value %#v", m)
	}
}

func gremlinResultArray(i interface{}) (interface{}, error) {
	a, ok := i.([]interface{})
	if !ok {
		return nil, fmt.Errorf("expected array not found")
	}

	r := []interface{}{}
	for _, element := range a {
		v, err := gremlinResultValue(element)
		if err != nil {
			return nil, err
		}
		r = append(r, v)
	}
	return r, nil
}

func gremlinResultMapList(i interface{}) (interface{}, error) {
	r := map[string]interface{}{}

	a, ok := i.([]interface{})
	if !ok {
		return nil, fmt.Errorf("expected map not found")
	}

	for i := 0; i < len(a); i = i + 2 {
		k := util.GetString(a[i])
		v, err := gremlinResultValue(a[i+1])
		if err != nil {
			return nil, err
		}
		r[k] = v
	}
	return r, nil
}

func gremlinResultMap(i interface{}) (interface{}, error) {
	r := map[string]interface{}{}

	/*
		a, ok := i.([]interface{})
		if !ok {
			return nil, fmt.Errorf("expected map not found")
		}


		for i := 0; i < len(a); i = i + 1 {
			k := util.GetString(a[i])
			v, err := gremlinResultValue(a[i+1])
			if err != nil {
				return nil, err
			}
			r[k] = v
		}*/
	var err error
	a, ok := i.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("expected map not found")
	}
	for k, v := range a {
		r[k], err = gremlinResultValue(v)
		if err != nil {
			return nil, err
		}
	}
	return r, nil
}
