package datatypes

// Handle generic formatting chores for all data types.

import (
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"sync"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"gopkg.in/resty.v1"
)

// FormatUnquoted formats a value but does not put quotes on strings.
func FormatUnquoted(arg interface{}) string {
	switch v := arg.(type) {
	case string:
		return v

	default:
		return Format(v)
	}
}

func Format(element interface{}) string {
	if IsNil(element) {
		return "<nil>"
	}

	switch v := element.(type) {
	case Type:
		return "T(" + v.String() + v.FunctionNameList() + ")"

	case *Type:
		return "T(" + "*" + v.String() + v.FunctionNameList() + ")"

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

	case Channel:
		return v.String()

	case *Channel:
		return "*" + v.String()

	case error:
		return fmt.Sprintf("%v", v)

	case bool:
		if v {
			return defs.True
		}

		return defs.False

	case byte:
		return fmt.Sprintf("%v", v)

	case int32:
		return fmt.Sprintf("%v", v)

	case int:
		return fmt.Sprintf("%d", v)

	case int64:
		return fmt.Sprintf("%d", v)

	case float32:
		return fmt.Sprintf("%v", v)

	case float64:
		return fmt.Sprintf("%v", v)

	case string:
		return "\"" + v + "\""

	case *FunctionDeclaration:
		return v.String()

	case FunctionDeclaration:
		return v.String()

	case *EgoPackage:
		var b strings.Builder

		keys := v.Keys()
		needsComma := false

		b.WriteString("Pkg<")

		if v.Builtins() {
			b.WriteString("builtins")

			needsComma = true
		}

		if v.Imported() {
			if needsComma {
				b.WriteString(", ")
			}

			b.WriteString("imports")

			needsComma = true
		}

		for _, k := range keys {
			// Skip over hidden values
			if strings.HasPrefix(k, "__") {
				continue
			}

			i, _ := v.Get(k)

			if needsComma {
				b.WriteString(",")
			}

			needsComma = true

			b.WriteRune(' ')
			b.WriteString(k)
			b.WriteString(": ")
			b.WriteString(Format(i))
		}

		b.WriteString(" >")

		return b.String()

	case *EgoStruct:
		return v.String()

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

			case *resty.Client:
				return "&rest.Client{}"

			default:
				return fmt.Sprintf("&%s", Format(vv))
			}
		} else {
			return "nil<*interface{}>"
		}

	default:
		vv := reflect.ValueOf(v)

		// IF it's an internal function, show it's name. If it is a standard builtin from the
		// function library, show the short form of the name.
		if vv.Kind() == reflect.Func {
			fn := runtime.FuncForPC(reflect.ValueOf(v).Pointer())

			name := fn.Name()
			name = strings.Replace(name, "github.com/tucats/ego/runtime.", "runtime.", 1)
			name = strings.Replace(name, "github.com/tucats/ego/functions.", "functions.", 1)
			name = strings.Replace(name, "github.com/tucats/ego/", "", 1)

			if name == "" {
				name = "<anon>"
			}

			if d := dictionary[name]; d != "" {
				name = d
			} else {
				name = name + "()"
			}

			return name
		}

		// If it's a bytecode.Bytecode pointer, use reflection to get the
		// Name field value and use that with the name. A function literal
		// will have no name.
		if vv.Kind() == reflect.Ptr {
			ts := vv.String()
			if ts == defs.ByteCodeReflectionTypeString {
				e := reflect.ValueOf(v).Elem()

				name := fmt.Sprintf("%v", e.FieldByName("Name").Interface())
				if name == "" {
					name = "<anon>"
				}

				fd := GetDeclaration(v)
				if fd != nil {
					return fd.String()
				}

				return name + "()"
			}

			return fmt.Sprintf("ptr %s", ts)
		}

		if strings.HasPrefix(vv.String(), "<bytecode.StackMarker") {
			return fmt.Sprintf("%v", v)
		}

		if strings.HasPrefix(vv.String(), "<bytecode.CallFrame") {
			e := reflect.ValueOf(v).FieldByName("Module")
			module := fmt.Sprintf("%v", e.Interface())
			e = reflect.ValueOf(v).FieldByName("Line")
			line := GetInt(e.Interface())

			return fmt.Sprintf("F<%s:%d>", module, line)
		}

		if strings.HasPrefix(vv.String(), "<bytecode.ConstantWrapper") {
			e := reflect.ValueOf(v).FieldByName("Value")
			name := fmt.Sprintf("%v", e.Interface())

			return fmt.Sprintf("Constant %s", Format(name))
		}

		// If it's a slice of an interface array, used to pass compound
		// parameters to bytecodes, then format it as {a, b, c}

		valueString := fmt.Sprintf("%v", v)
		valueKind := vv.Kind()

		if valueKind == reflect.Slice {
			return valueString
		}

		if ui.IsActive(ui.DebugLogger) {
			return fmt.Sprintf("kind %v %#v", vv.Kind(), v)
		}

		return fmt.Sprintf("kind %v %v", vv.Kind(), v)
	}
}
