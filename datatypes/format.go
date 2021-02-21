package datatypes

// Handle generic formatting chores for all data types.

import (
	"fmt"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/gopackages/datatypes"
)

func Format(element interface{}) string {
	if element == nil {
		return "<nil>"
	}

	switch v := element.(type) {
	case *datatypes.Channel:
		return v.String()

	case error:
		return fmt.Sprintf("%v", v)

	case int:
		return fmt.Sprintf("%d", v)

	case int64:
		return fmt.Sprintf("%d", v)

	case bool:
		if v {
			return "true"
		}

		return "false"

	case float64:
		return fmt.Sprintf("%v", v)

	case map[string]interface{}:
		var b strings.Builder

		// Make a list of the keys, ignoring hidden members whose name
		// starts with "__"
		keys := make([]string, 0)

		for k := range v {
			if len(k) < 2 || k[0:2] != "__" {
				keys = append(keys, k)
			}
		}

		sort.Strings(keys)
		b.WriteString("{")

		for n, k := range keys {
			i := v[k]

			if n > 0 {
				b.WriteString(",")
			}

			b.WriteRune(' ')
			b.WriteString(k)
			b.WriteString(": ")
			b.WriteString(Format(i))
		}

		b.WriteString(" }")

		return b.String()

	case []interface{}:
		var b strings.Builder

		b.WriteRune('[')

		for n, i := range v {
			if n > 0 {
				b.WriteString(", ")
			}

			b.WriteString(Format(i))
		}

		b.WriteRune(']')

		return b.String()

	case string:
		return "\"" + v + "\""

	case *EgoArray:
		return v.String()

	case *EgoMap:
		return v.String()

	case *interface{}:
		return fmt.Sprintf("&%#v", *v)

	default:
		vv := reflect.ValueOf(v)

		// IF it's an internal function, show it's name. If it is a standard builtin from the
		// function library, show the short form of the name.
		if vv.Kind() == reflect.Func {
			if ui.DebugMode {
				name := runtime.FuncForPC(reflect.ValueOf(v).Pointer()).Name()
				name = strings.Replace(name, "github.com/tucats/ego/", "", 1)
				name = strings.Replace(name, "github.com/tucats/ego/runtime.", "", 1)

				return "builtin " + name
			} else {
				return "builtin"
			}
		}

		// If it's a bytecode.Bytecode pointer, use reflection to get the
		// Name field value and use that with the name. A function literal
		// will have no name.
		if vv.Kind() == reflect.Ptr {
			ts := vv.String()
			if ts == "<*bytecode.ByteCode Value>" {
				e := reflect.ValueOf(v).Elem()

				if ui.DebugMode {
					name := fmt.Sprintf("%v", e.Field(0).Interface())

					return "func " + name
				} else {
					return "func"
				}
			}

			return fmt.Sprintf("ptr %s %#v", ts, element)
		}

		if strings.HasPrefix(vv.String(), "<bytecode.StackMarker") {
			e := reflect.ValueOf(v).Field(0)
			name := fmt.Sprintf("%v", e.Interface())

			return fmt.Sprintf("<%s>", name)
		}

		if strings.HasPrefix(vv.String(), "<bytecode.CallFrame") {
			e := reflect.ValueOf(v).Field(0)
			module := fmt.Sprintf("%v", e.Interface())
			e = reflect.ValueOf(v).Field(1)
			line := GetInt(e.Interface())

			return fmt.Sprintf("<frame %s:%d>", module, line)
		}

		if ui.DebugMode {
			return fmt.Sprintf("kind %v %#v", vv.Kind(), v)
		}

		return fmt.Sprintf("kind %v %v", vv.Kind(), v)
	}
}

func GetString(v interface{}) string {
	return fmt.Sprintf("%v", v)
}

func GetInt(v interface{}) int {
	switch actual := v.(type) {
	case int32:
		return int(actual)

	case int:
		return actual

	case int64:
		return int(actual)

	case float32:
		return int(actual)

	case float64:
		return int(actual)

	case bool:
		if actual {
			return 1
		}

		return 0

	default:
		v, _ := strconv.Atoi(fmt.Sprintf("%v", actual))

		return v
	}
}
