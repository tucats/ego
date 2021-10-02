package runtime

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strings"

	"github.com/northwesternmutual/grammes"
	"github.com/tucats/ego/compiler"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

type column struct {
	Name           string `json:"name"`
	FormattedWidth int    `json:"formattedWidth"`
	Kind           int    `json:"kind"`
	KindName       string `json:"kindName"`
}

type Row []interface{}

type jsonData struct {
	Columns []column `json:"columns"`
	Rows    []Row    `json:"rows"`
}

// GremlinColumn describes everything we know about a column in a gremlin
// result set.
type GremlinColumn struct {
	Name string
	Type int
}

var gremlinType *datatypes.Type

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

func initializeGremlinType() {
	if gremlinType == nil {
		t, _ := compiler.CompileTypeSpec(gremlinTypeSpec)

		t.DefineFunctions(map[string]interface{}{
			"Query":    GremlinQuery,
			"Map":      GremlinMap,
			"QueryMap": GremlinQueryMap,
			"AsJSON":   AsJSON,
		})

		gremlinType = &t
	}
}

// GremlinOpen opens a gremlin connetion and stores it in the result value.
func GremlinOpen(symbols *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	var username, password string

	var url string

	var client *grammes.Client

	var err error

	if len(args) == 0 {
		url = "ws://localhost:8182/gremlin"
	} else {
		url = datatypes.GetString(args[0])
	}

	if len(args) > 1 {
		username = datatypes.GetString(args[1])

		if len(args) > 2 {
			password = datatypes.GetString(args[2])
		}
	}

	if username != "" {
		auth := grammes.WithAuthUserPass(username, password)
		client, err = grammes.DialWithWebSocket(url, auth)
	} else {
		client, err = grammes.DialWithWebSocket(url)
	}

	initializeGremlinType()

	r := datatypes.NewStruct(*gremlinType)

	_ = r.Set(clientFieldName, client)
	r.SetReadonly(true)

	return r, errors.New(err)
}

// GremlinQuery executes a string query against an open client.
func GremlinQuery(symbols *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	client, err := getGremlinClient(symbols)
	if !errors.Nil(err) {
		return nil, err
	}

	if len(args) != 1 {
		return nil, errors.New(errors.ErrArgumentCount)
	}

	query := datatypes.GetString(args[0])

	res, e2 := client.ExecuteStringQuery(query)
	if e2 != nil {
		return nil, errors.New(e2)
	}

	return gremlinResult(string(res[0]))
}

// GremlinQueryMap executes a string query against an open client. The
// result is then normalized against a map provided by the user.
func GremlinQueryMap(symbols *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	var m interface{}

	var err *errors.EgoError

	if len(args) < 1 || len(args) > 2 {
		return nil, errors.New(errors.ErrArgumentCount)
	}

	if len(args) > 1 {
		m = args[1]
	} else {
		m, err = GremlinMap(symbols, args)
		if !errors.Nil(err) {
			return m, err
		}
	}

	client, err := getGremlinClient(symbols)
	if !errors.Nil(err) {
		return nil, err
	}

	query := datatypes.GetString(args[0])

	res, e2 := client.ExecuteStringQuery(query)
	if e2 != nil {
		return nil, errors.New(e2)
	}

	r, err := gremlinResult(string(res[0]))
	if !errors.Nil(err) {
		return r, err
	}

	return gremlinApplyMap(r, m)
}

// AsJSON executes a query and constructs a JSON string that represents the
// resulting result set. This is similar to using QueryMap followed by Table but
// generating JSON instead of a formatted text output.
func AsJSON(symbols *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if len(args) != 1 {
		return nil, errors.New(errors.ErrArgumentCount)
	}

	// Get a normalized result set from the query.
	resultSet, err := GremlinQueryMap(symbols, args)
	if !errors.Nil(err) {
		return nil, err
	}

	// Scan over the first data element to pick up the column names and types
	a := datatypes.GetNativeArray(resultSet)
	if len(a) == 0 {
		return nil, errors.New(errors.ErrInvalidResultSetType)
	}

	// Make a list of the sort key names
	keys := []string{}

	rowMap := datatypes.GetNativeMap(a[0])
	for k := range rowMap {
		keys = append(keys, k)
	}

	sort.Strings(keys)

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
		rowMap := datatypes.GetNativeMap(rowValue)

		for n := 0; n < len(r.Columns); n = n + 1 {
			c := r.Columns[n]

			v, ok := rowMap[c.Name]
			if ok {
				width := len(datatypes.FormatUnquoted(v))
				if width > c.FormattedWidth {
					c.FormattedWidth = width
					r.Columns[n] = c
				}
			}
		}
	}

	// Loop over the rows and fill in the values
	for _, rowElement := range a {
		rowMap := datatypes.GetNativeMap(rowElement)
		rs := make([]interface{}, len(r.Columns))

		for i, c := range r.Columns {
			v := rowMap[c.Name]
			rs[i] = v
		}

		r.Rows = append(r.Rows, rs)
	}

	bytes, e2 := json.Marshal(r)

	return string(bytes), errors.New(e2)
}

// Table generates a string describing a rectangular result map.
func Table(symbols *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if len(args) < 1 || len(args) > 2 {
		return nil, errors.New(errors.ErrArgumentCount)
	}

	includeHeadings := true

	if len(args) == 2 {
		includeHeadings = datatypes.GetBool(args[1])
	}

	// Scan over the first data element to pick up the column names and types
	a := datatypes.GetNativeArray(args[0])
	if len(a) == 0 {
		return nil, errors.New(errors.ErrInvalidResultSetType)
	}

	// Make a list of the sort key names
	row := datatypes.GetNativeMap(a[0])
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
		row := datatypes.GetNativeMap(r)

		for n := 0; n < len(columns); n = n + 1 {
			c := columns[n]

			v, ok := row[c.Name]
			if ok {
				width := len(datatypes.FormatUnquoted(v))
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
			b.WriteString(Pad(c.Name, c.FormattedWidth))
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
		row := datatypes.GetNativeMap(r)

		var b strings.Builder

		for _, c := range columns {
			v, ok := row[c.Name]
			if ok {
				b.WriteString(Pad(v, c.FormattedWidth))
				b.WriteRune(' ')
			} else {
				b.WriteString(strings.Repeat(" ", c.FormattedWidth+1))
			}
		}

		result = append(result, b.String())
	}

	return result, nil
}

func getGremlinClient(symbols *symbols.SymbolTable) (*grammes.Client, *errors.EgoError) {
	g, ok := symbols.Get("__this")
	if !ok {
		return nil, errors.New(errors.ErrNoFunctionReceiver)
	}

	gc, ok := g.(map[string]interface{}) // @tomcole should be struct, when gremlin is updated to create type
	if !ok {
		return nil, errors.New(errors.ErrInvalidGremlinClient)
	}

	client, ok := gc["client"]
	if !ok {
		return nil, errors.New(errors.ErrInvalidGremlinClient)
	}

	cp, ok := client.(*grammes.Client)
	if !ok {
		return nil, errors.New(errors.ErrInvalidGremlinClient)
	}

	return cp, nil
}

// GremlinMap accepts an opaque object and creates a map of the columns and most-compatible
// types for the columns. This is a precursor to being able to view a gremlin result set as a
// tabular result.
func GremlinMap(symbols *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	var err *errors.EgoError

	if len(args) < 1 || len(args) > 2 {
		return nil, errors.New(errors.ErrArgumentCount)
	}

	r := args[0]

	switch r.(type) {
	case string:
		// We were given a query to execute
		r, err = GremlinQuery(symbols, args)
		if !errors.Nil(err) {
			return nil, err
		}

	case map[string]interface{}:
		r = []interface{}{r}

	case []interface{}:
		// do nothing
	default:
		return nil, errors.New(errors.ErrInvalidResultSetType)
	}

	columns := map[string]GremlinColumn{}

	for _, row := range r.([]interface{}) {
		rowMap, ok := row.(map[string]interface{})
		if !ok {
			return nil, errors.New(errors.ErrInvalidRowSet)
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

// Utility functions for Gremlin support.
func gremlinResult(str string) (interface{}, *errors.EgoError) {
	var r interface{}

	err := json.Unmarshal([]byte(str), &r)
	if !errors.Nil(err) {
		return nil, errors.New(err)
	}

	return gremlinResultValue(r)
}

func isNumeric(t int) bool {
	if t >= int(reflect.Int) && t <= int(reflect.Complex128) {
		return true
	}

	return false
}

func gremlinResultValue(i interface{}) (interface{}, *errors.EgoError) {
	switch m := i.(type) {
	// The nil value
	case nil:
		return nil, nil

	// Native Ego types
	case string:
		return m, nil

	case bool:
		return m, nil

	case byte:
		return m, nil

	case int32:
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
			if !errors.Nil(err) {
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
			return datatypes.GetInt(v), nil

		case "g:Int64":
			i := datatypes.GetInt(v)

			if math.Abs(float64(i)) > math.MaxInt32 {
				return i, nil
			}

			return int(i), nil

		case "gx:BigDecimal":
			if r, ok := v.(float64); ok {
				return r, nil
			}

			return nil, errors.New(errors.ErrUnexpectedValue).Context(v)

		default:
			return nil, errors.New(errors.ErrInvalidTypeName).Context(m["@type"])
		}

	default:
		return nil, errors.New(errors.ErrUnexpectedValue).Context(m)
	}
}

func gremlinResultArray(i interface{}) (interface{}, *errors.EgoError) {
	a, ok := i.([]interface{})
	if !ok {
		return nil, errors.New(errors.ErrInvalidResultSetType)
	}

	r := []interface{}{}

	for _, element := range a {
		v, err := gremlinResultValue(element)
		if !errors.Nil(err) {
			return nil, err
		}

		r = append(r, v)
	}

	return r, nil
}

func gremlinResultMapList(i interface{}) (interface{}, *errors.EgoError) {
	r := map[string]interface{}{}

	a, ok := i.([]interface{})
	if !ok {
		return nil, errors.New(errors.ErrInvalidResultSetType)
	}

	for i := 0; i < len(a); i = i + 2 {
		k := datatypes.GetString(a[i])

		v, err := gremlinResultValue(a[i+1])
		if !errors.Nil(err) {
			return nil, err
		}

		r[k] = v
	}

	return r, nil
}

func gremlinResultMap(i interface{}) (interface{}, *errors.EgoError) {
	var err *errors.EgoError

	r := map[string]interface{}{}

	a, ok := i.(map[string]interface{})
	if !ok {
		return nil, errors.New(errors.ErrInvalidResultSetType)
	}

	for k, v := range a {
		r[k], err = gremlinResultValue(v)
		if !errors.Nil(err) {
			return nil, err
		}
	}

	return r, nil
}

func gremlinApplyMap(r interface{}, m interface{}) (interface{}, *errors.EgoError) {
	// Unwrap the parameters
	rows := datatypes.GetNativeArray(r)
	if rows == nil {
		return nil, errors.New(errors.ErrInvalidRowSet)
	}

	columnList := datatypes.GetNativeArray(m)
	if columnList == nil {
		return nil, errors.New(errors.ErrInvalidStruct)
	}

	columns := map[string]string{}

	for _, v := range columnList {
		columnMap := datatypes.GetNativeMap(v)
		name := datatypes.GetString(columnMap["name"])
		kind := datatypes.GetString(columnMap["type"])
		columns[name] = kind
	}

	// Scan over the rows of the result set. For each row, force a new
	// row to be constructed that is normalized to the map.
	result := make([]interface{}, len(rows))

	for n, row := range rows {
		resultRow := map[string]interface{}{
			"_row_": n + 1,
		}
		data := datatypes.GetNativeMap(row)

		for columnName, columnInfo := range columns {
			v, ok := data[columnName]
			if !ok {
				v = nil
			}

			resultRow[columnName] = datatypes.CoerceType(v, columnInfo)
		}

		result[n] = resultRow
	}

	return result, nil
}
