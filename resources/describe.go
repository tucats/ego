package resources

import (
	"encoding/json"
	"reflect"
	"strings"

	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/ui"
)

// describe creates column information for a native Go object, which must
// be a structure. This information includes the column name and SQL data
// type for the column. Not all Go values are supported (only string, integer,
// boolean, float32 and float64).
//
// If the object passed is not a structure, then the result is a nil array.
func describe(object interface{}) []Column {
	var result []Column

	value := reflect.ValueOf(object)
	if value.Kind() == reflect.Pointer {
		value = reflect.Indirect(value.Elem())
	}

	if value.Kind() != reflect.Struct {
		return nil
	}

	count := value.NumField()
	result = make([]Column, count)

	for i := 0; i < count; i++ {
		column := Column{}
		field := value.Field(i)

		t := value.Type()
		f := t.Field(i)

		column.Name = f.Name
		column.SQLName = strings.ToLower(f.Name)

		tt := field.Type()

		switch {
		case tt == reflect.TypeOf(uuid.New()):
			column.SQLType = SQLStringType
			column.IsUUID = true

		case tt == reflect.TypeOf([]string{}):
			column.SQLType = SQLStringType
			column.IsJSON = true

		default:
			switch tt.Kind() {
			case reflect.Int:
				column.SQLType = SQLIntType

			case reflect.String:
				column.SQLType = SQLStringType

			case reflect.Float32:
				column.SQLType = SQLFloatType

			case reflect.Float64:
				column.SQLType = SQLDoubleType

			case reflect.Bool:
				column.SQLType = SQLBoolType
			}
		}

		result[i] = column
	}

	return result
}

// Create an array of interface objects, one for each value in a
// structure. The order of the objects matches the column order
// from the describe() call.
//
// This is used to copy the data values of the structure into
// SQL statements that will write or update instances of the
// resource in the underlying table.
//
// The object passed in must either be the resource structure itself
// or a pointer to the struct.
func (r *ResHandle) explode(object interface{}) []interface{} {
	var result []interface{}

	value := reflect.ValueOf(object)
	if value.Kind() == reflect.Pointer {
		value = reflect.Indirect(value.Elem())
	}

	if value.Kind() != reflect.Struct {
		ui.Log(ui.ResourceLogger, "[0] invalid explode on type %#v", object)

		return nil
	}

	count := value.NumField()
	result = make([]interface{}, count)

	for i := 0; i < count; i++ {
		field := value.Field(i)
		value := field.Interface()

		if r.Columns[i].IsJSON {
			b, _ := json.Marshal(value)
			value = string(b)
		} else if r.Columns[i].IsUUID {
			u := value.(uuid.UUID)
			value = u.String()
		}

		result[i] = value
	}

	return result
}
