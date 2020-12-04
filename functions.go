package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strings"

	"github.com/northwesternmutual/grammes"
	"github.com/tucats/gopackages/expressions"
	"github.com/tucats/gopackages/symbols"
	"github.com/tucats/gopackages/util"
)

var typeMap map[reflect.Kind]string = map[reflect.Kind]string{
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

// FunctionPi implements the pi() function
func FunctionPi(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) > 0 {
		return nil, errors.New("too many arguments to pi()")
	}
	return 3.1415926535, nil
}

// FunctionPrompt implements the prompt() function, which uses the console
// reader
func FunctionPrompt(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	prompt := ""
	if len(args) > 0 {
		prompt = util.GetString(args[0])
	}
	text := readConsoleText(prompt)
	if text == "\n" {
		text = ""
	} else {
		if strings.HasSuffix(text, "\n") {
			text = text[:len(text)-1]
		}
	}
	return text, nil
}

// FunctionEval implements the eval() function, which uses the expressions
// package to compile an expression fragment and execute it to get the resulting
// value.
func FunctionEval(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) != 1 {
		return nil, errors.New("wrong number of arguments")
	}
	return expressions.Evaluate(util.GetString(args[0]), symbols)
}

// FunctionGremlinOpen opens a gremlin connetion and stores it in the result value
func FunctionGremlinOpen(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {

	var username, password string

	var url string
	if len(args) == 0 {
		url = "ws://localhost:8182/gremlin"
	} else {
		url = util.GetString(args[0])
	}
	if len(args) > 1 {
		username = util.GetString(args[1])
		if len(args) > 2 {
			password = util.GetString(args[2])
		}
	}

	var client *grammes.Client
	var err error
	if username != "" {
		auth := grammes.WithAuthUserPass(username, password)
		client, err = grammes.DialWithWebSocket(url, auth)
	} else {
		client, err = grammes.DialWithWebSocket(url)
	}

	return map[string]interface{}{
		"client":     client,
		"query":      FunctionGremlinQuery,
		"map":        FunctionGremlinMap,
		"querymap":   FunctionGremlinQueryMap,
		"asjson":     FunctionAsJSON,
		"__readonly": true,
	}, err

}

// FunctionGremlinQuery executes a string query against an open client
func FunctionGremlinQuery(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {

	client, err := getGremlinClient(symbols)
	if err != nil {
		return nil, err
	}
	if len(args) != 1 {
		return nil, errors.New("incorrect number of arguments")
	}
	query := util.GetString(args[0])
	res, err := client.ExecuteStringQuery(query)
	if err != nil {
		return nil, err
	}
	return gremlinResult(string(res[0]))
}

// FunctionGremlinQueryMap executes a string query against an open client. The
// result is then normalized against a map provided by the user.
func FunctionGremlinQueryMap(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {

	if len(args) < 1 || len(args) > 2 {
		return nil, errors.New("incorrect number of arguments")
	}

	var m interface{}
	var err error

	if len(args) > 1 {
		m = args[1]
	} else {
		m, err = FunctionGremlinMap(symbols, args)
		if err != nil {
			return m, err
		}
	}
	client, err := getGremlinClient(symbols)
	if err != nil {
		return nil, err
	}

	query := util.GetString(args[0])
	res, err := client.ExecuteStringQuery(query)
	if err != nil {
		return nil, err
	}

	r, err := gremlinResult(string(res[0]))
	if err != nil {
		return r, err
	}

	return gremlinApplyMap(r, m)
}

type column struct {
	Name           string `json:"name"`
	FormattedWidth int    `json:"formattedWidth"`
	Kind           int    `json:"kind"`
	KindName       string `json:"kindName"`
}

// FunctionAsJSON executes a query and constructs a JSON string that represents the
// resulting result set. This is similar to using QueryMap followed by Table but
// generating JSON instead of a formatted text output.
func FunctionAsJSON(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) != 1 {
		return nil, errors.New("incorrect number of arguments")
	}

	// Get a normalized result set from the query.
	resultSet, err := FunctionGremlinQueryMap(symbols, args)

	// Scan over the first data element to pick up the column names and types
	a := util.GetArray(resultSet)
	if a == nil || len(a) == 0 {
		return nil, errors.New("not a result set")
	}

	// Make a list of the sort key names
	rowMap := util.GetMap(a[0])
	keys := []string{}
	for k := range rowMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	type rowvalues struct {
		kind  string
		value string
	}

	type Row []interface{}

	type jsonData struct {
		Columns []column `json:"columns"`
		Rows    []Row    `json:"rows"`
	}

	r := jsonData{}

	// Construct a list of the columns.
	r.Columns = []column{}
	for _, k := range keys {
		v := rowMap[k]
		c := column{}
		c.Name = k
		kind := reflect.TypeOf(v).Kind()
		c.Kind = int(kind)
		c.KindName = typeMap[kind]
		r.Columns = append(r.Columns, c)
	}

	// Determine the column width of each column of the result set
	for _, rowValue := range a {
		rowMap := util.GetMap(rowValue)
		for n := 0; n < len(r.Columns); n = n + 1 {
			c := r.Columns[n]
			v, ok := rowMap[c.Name]
			if ok {
				width := len(util.FormatUnquoted(v))
				if width > c.FormattedWidth {
					c.FormattedWidth = width
					r.Columns[n] = c
				}
			}
		}
	}

	// Loop over the rows and fill in the values
	for _, rowElement := range a {
		rowMap := util.GetMap(rowElement)
		rs := make([]interface{}, len(r.Columns))
		for i, c := range r.Columns {
			v := rowMap[c.Name]
			rs[i] = v
		}
		r.Rows = append(r.Rows, rs)
	}

	bytes, err := json.Marshal(r)
	return string(bytes), err
}

// FunctionTable generates a string describing a rectangular result map
func FunctionTable(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {

	if len(args) < 1 || len(args) > 2 {
		return nil, errors.New("incorrect number of arguments")
	}
	includeHeadings := true
	if len(args) == 2 {
		includeHeadings = util.GetBool(args[1])
	}

	// Scan over the first data element to pick up the column names and types
	a := util.GetArray(args[0])
	if a == nil || len(a) == 0 {
		return nil, errors.New("not a result set")
	}

	// Make a list of the sort key names
	row := util.GetMap(a[0])
	keys := []string{}
	for k := range row {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	columns := []column{}
	for _, k := range keys {
		v := row[k]
		c := column{}
		c.Name = k
		c.Kind = int(reflect.TypeOf(v).Kind())
		if includeHeadings {
			c.FormattedWidth = len(k)
		}
		columns = append(columns, c)
	}

	// Scan all rows to get maximum length values
	for _, r := range a {
		row := util.GetMap(r)
		for n := 0; n < len(columns); n = n + 1 {
			c := columns[n]
			v, ok := row[c.Name]
			if ok {
				width := len(util.FormatUnquoted(v))
				if width > c.FormattedWidth {
					c.FormattedWidth = width
					columns[n] = c
				}
			}
		}
	}

	// Go over the columns and right-align numeric values
	for n := 0; n < len(columns); n = n + 1 {
		c := columns[n]
		if isNumeric(c.Kind) {
			c.FormattedWidth = -c.FormattedWidth
			columns[n] = c
		}
	}

	// Fill in the headers
	result := []interface{}{}

	if includeHeadings {
		var b strings.Builder
		var h strings.Builder
		for _, c := range columns {
			b.WriteString(pad(c.Name, c.FormattedWidth))
			b.WriteRune(' ')
			w := c.FormattedWidth
			if w < 0 {
				w = -w
			}
			h.WriteString(strings.Repeat("=", w))
			h.WriteRune(' ')
		}
		result = append(result, b.String())
		result = append(result, h.String())
	}

	// Loop over the rows and fill in the values
	for _, r := range a {
		row := util.GetMap(r)
		var b strings.Builder
		for _, c := range columns {
			v, ok := row[c.Name]
			if ok {
				b.WriteString(pad(v, c.FormattedWidth))
				b.WriteRune(' ')
			} else {
				b.WriteString(strings.Repeat(" ", c.FormattedWidth+1))
			}
		}
		result = append(result, b.String())
	}

	return result, nil
}

// Pad the formatted value of a given object to the specified number
// of characters. Negative numbers are right-aligned, positive numbers
// are left-aligned.
func pad(v interface{}, w int) string {
	s := util.FormatUnquoted(v)
	count := w
	if count < 0 {
		count = -count
	}
	padString := ""
	if count > len(s) {
		padString = strings.Repeat(" ", count-len(s))
	}
	var r string
	if w < 0 {
		r = padString + s
	} else {
		r = s + padString
	}
	if len(r) > count {
		r = r[:count]
	}
	return string(r)
}

func getGremlinClient(symbols *symbols.SymbolTable) (*grammes.Client, error) {

	g, ok := symbols.Get("_this")
	if !ok {
		return nil, errors.New("no function reciver")
	}
	gc, ok := g.(map[string]interface{})
	if !ok {
		return nil, errors.New("not a valid gremlin client struct")
	}
	client, ok := gc["client"]
	if !ok {
		return nil, errors.New("no 'client' member found")
	}
	cp, ok := client.(*grammes.Client)
	if !ok {
		return nil, errors.New("'client' is not a gremlin client pointer")
	}
	return cp, nil
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
	case string:
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
	err := json.Unmarshal([]byte(str), &r)
	if err != nil {
		return nil, err
	}
	return gremlinResultValue(r)
}

func isNumeric(t int) bool {
	if t >= int(reflect.Int) && t <= int(reflect.Complex128) {
		return true
	}
	return false
}

func gremlinResultValue(i interface{}) (interface{}, error) {

	switch m := i.(type) {
	// The nil value
	case nil:
		return nil, nil

	// Native Ego types
	case string:
		return m, nil
	case bool:
		return m, nil
	case int:
		return m, nil
	case int64:
		return m, nil
	case float32:
		return m, nil
	case float64:
		return m, nil

	// Complex results
	case map[string]interface{}:
		v := m["@value"]
		switch m["@type"] {
		case "g:UUID":
			return v, nil // treat as string in the Go code
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

func gremlinApplyMap(r interface{}, m interface{}) (interface{}, error) {
	// Unwrap the parameters
	rows := util.GetArray(r)
	if rows == nil {
		return nil, errors.New("rowset not an array")
	}
	columnList := util.GetArray(m)
	if columnList == nil {
		return nil, errors.New("map not a struct")
	}

	columns := map[string]string{}
	for _, v := range columnList {
		columnMap := util.GetMap(v)
		name := util.GetString(columnMap["name"])
		kind := util.GetString(columnMap["type"])
		columns[name] = kind
	}

	// Scan over the rows of the result set. For each row, force a new
	// row to be constructed that is normalized to the map.
	result := make([]interface{}, len(rows))
	for n, row := range rows {
		resultRow := map[string]interface{}{
			"_row_": n + 1,
		}
		data := util.GetMap(row)
		for columnName, columnInfo := range columns {
			v, ok := data[columnName]
			if !ok {
				v = nil
			}
			resultRow[columnName] = util.CoerceType(v, columnInfo)
		}
		result[n] = resultRow
	}

	return result, nil
}
