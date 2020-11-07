package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"

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

// GremlinColumn describes everything we know about a column in a gremlin
// result set.
type GremlinColumn struct {
	Name string
	Type int
}

// FunctionGremlinMap accepts an opaque object and creates a map of the columns and most-compatible
// types for the columns. This is a precursor to being able to view a gremlin result set as a
// tabular result.
func FunctionGremlinMap(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {

	if len(args) < 1 || len(args) > 2 {
		return nil, errors.New("incorrect number of arguments")
	}
	r := args[0]
	var err error

	switch r.(type) {
	case *grammes.Client:
		// We were given a query to execute
		r, err = FunctionGremlinQuery(symbols, args)
		if err != nil {
			return nil, err
		}
	case map[string]interface{}:
		r = []interface{}{r}
	case []interface{}:
		// do nothing
	default:
		return nil, errors.New("invalid result set type")
	}

	columns := map[string]GremlinColumn{}

	for _, row := range r.([]interface{}) {
		rowMap, ok := row.(map[string]interface{})
		if !ok {
			return nil, errors.New("invalid row")
		}
		for k, v := range rowMap {
			gc := GremlinColumn{Name: k}
			t := reflect.TypeOf(v)
			gc.Type = int(t.Kind())

			// Do we already know about this one? If so,
			// calculate best type to use
			if existingColumn, ok := columns[k]; ok {
				if gc.Type < existingColumn.Type {
					gc.Type = existingColumn.Type
				}
			}
			columns[k] = gc
		}
	}

	typeMap := map[reflect.Kind]string{
		reflect.Bool:          "bool",
		reflect.Int:           "int",
		reflect.Int8:          "int8",
		reflect.Int16:         "int16",
		reflect.Int32:         "int32",
		reflect.Int64:         "int64",
		reflect.Uint:          "uint",
		reflect.Uint8:         "uint8",
		reflect.Uint16:        "uint16",
		reflect.Uint32:        "uint32",
		reflect.Uint64:        "uint64",
		reflect.Uintptr:       "uintptr",
		reflect.Float32:       "float32",
		reflect.Float64:       "float64",
		reflect.Complex64:     "complex64",
		reflect.Complex128:    "complex128",
		reflect.Array:         "array",
		reflect.Chan:          "channel",
		reflect.Func:          "func",
		reflect.Interface:     "interface{}",
		reflect.Map:           "map",
		reflect.Ptr:           "ptr",
		reflect.Slice:         "slice",
		reflect.String:        "string",
		reflect.Struct:        "struct",
		reflect.UnsafePointer: "unsafe ptr",
	}
	// Convert the column array set to a normalized form
	rv := []interface{}{}
	for _, v := range columns {

		ts, ok := typeMap[reflect.Kind(v.Type)]
		if !ok {
			ts = fmt.Sprintf("Type %d", v.Type)
		}
		rm := map[string]interface{}{
			"name": v.Name,
			"type": ts,
		}
		rv = append(rv, rm)
	}
	return rv, nil
}

// Utility functions for Gremlin support
func gremlinResult(str string) (interface{}, error) {

	var r interface{}
	//	fmt.Printf("DEBUG: %s\n", str)
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
			r, err := gremlinResultArray(v)
			if err != nil {
				return nil, err
			}
			ra, ok := r.([]interface{})
			if ok && len(ra) == 1 {
				r = ra[0]
			}
			return r, nil
		case "g:Map":
			return gremlinResultMapList(v)
		case "g:Vertex":
			return gremlinResultMap(v)
		case "g:Int32":
			return util.GetInt(v), nil
		case "g:Int64":
			i := util.GetInt64(v)
			if math.Abs(float64(i)) > math.MaxInt32 {
				return i, nil
			}
			return int(i), nil

		case "gx:BigDecimal":
			if r, ok := v.(float64); ok {
				return r, nil
			}
			return nil, fmt.Errorf("unexpected gx:BigDecimal type %#v", v)

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
	var err error
	r := map[string]interface{}{}
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
