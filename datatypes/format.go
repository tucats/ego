package datatypes

// Handle generic formatting chores for all data types.

import (
	"fmt"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/gopackages/datatypes"
)

func Format(element interface{}) string {
	if IsNil(element) {
		return "<nil>"
	}

	switch v := element.(type) {
	case Type:
		return "T(" + v.String() + v.FunctionsList() + ")"

	case *Type:
		return "T(" + v.String() + ")"

	// Naked WaitGroup is a model for a type
	case sync.WaitGroup:
		return "T(sync.WaitGroup)"
	// Pointer to WaitGroup is what an _Ego_ WaitGroup is
	case *sync.WaitGroup:
		return "sync.WaitGroup{}"

		// Naked WaitGroup is a model for a type
	case sync.Mutex:
		return "Mutex type"
	// Pointer to WaitGroup is what an _Ego_ WaitGroup is
	case *sync.Mutex:
		return "sync.Mutex{}"
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
		if v != nil {
			vv := *v
			switch vv := vv.(type) {
			case *sync.Mutex:
				return "&sync.Mutex"

			case *sync.WaitGroup:
				return "&sync.WaitGroup{}"

			default:
				return fmt.Sprintf("&%#v", vv)
			}
		} else {
			return "nil<*interface{}>"
		}

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
