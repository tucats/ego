package data

// Handle generic formatting chores for all data types.

import (
	"fmt"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"gopkg.in/resty.v1"
)

var verbose = false

// FormatUnquoted formats a value but does not put quotes on strings.
func FormatUnquoted(arg interface{}) string {
	switch v := arg.(type) {
	case string:
		return v

	default:
		return Format(v)
	}
}

func FormatWithType(element interface{}) string {
	switch actual := element.(type) {
	case error:
		return "E<" + actual.Error() + ">"

	case string:
		return strconv.Quote(actual)

	case byte:
		return fmt.Sprintf("byte(%d)", actual)

	case int32:
		return fmt.Sprintf("int32(%d)", actual)

	case int:
		return fmt.Sprintf("int(%d)", actual)

	case int64:
		return fmt.Sprintf("int64(%d)", actual)

	case float32:
		return fmt.Sprintf("float32(%f)", actual)

	case float64:
		return fmt.Sprintf("float64(%f)", actual)
	}

	fmtString := Format(element)
	//if strings.HasPrefix(fmtString, "[") && strings.HasSuffix(fmtString, "]") {
	//	fmtString = strings.TrimSuffix(strings.TrimPrefix(fmtString, "["), "]")
	//}

	// For things already formatted with a type prefix, don't add to the string.
	for _, prefix := range []string{"[", "M<", "F<", "Pkg<"} {
		if strings.HasPrefix(fmtString, prefix) {
			return fmtString
		}
	}

	// Is it a function declaration?
	parens := 0
	braces := 0
	found := false

	for _, ch := range fmtString {
		if ch == rune('(') {
			parens = parens + 1
			found = true
		} else if ch == rune(')') {
			parens = parens - 1
		} else if ch == rune('{') {
			braces = braces + 1
			found = true
		} else if ch == rune('}') {
			braces = braces - 1
		}
	}

	if found && parens == 0 && braces == 0 {
		return fmtString
	}

	// It's some kind of more complex type, output the type and the value(s)
	return TypeOf(element).String() + "{" + fmtString + "}"
}

func Format(element interface{}) string {
	if IsNil(element) {
		return "<nil>"
	}

	switch v := element.(type) {
	// Built-in types
	case Type:
		return v.String() + v.FunctionNameList() + " type"

	// User types
	case *Type:
		return v.String() + v.FunctionNameList()

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

	case *time.Time:
		return v.String()

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

	case *Package:
		var b strings.Builder

		keys := v.Keys()

		b.WriteString("Pkg<")

		b.WriteString(v.name)

		if v.Builtins() {
			b.WriteString(", builtins")
		}

		if v.Imported() {
			b.WriteString(", imports")
		}

		if verbose {
			for _, k := range keys {
				// Skip over hidden values
				if strings.HasPrefix(k, "__") {
					continue
				}

				i, _ := v.Get(k)

				b.WriteRune(' ')
				b.WriteString(k)
				b.WriteString(": ")
				b.WriteString(Format(i))
			}
		}

		b.WriteString(">")

		return b.String()

	case *Struct:
		return v.String()

	case *Array:
		return v.String()

	case *Map:
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

	case Function:
		if v.Declaration != nil {
			return v.Declaration.String()
		}

		return Format(v.Value)

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
				name = defs.Anon
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
				return fmt.Sprintf("%v", v)
			}

			return fmt.Sprintf("ptr %s", ts)
		}

		if strings.HasPrefix(vv.String(), "<bytecode.StackMarker") {
			return fmt.Sprintf("%v", v)
		}

		if strings.HasPrefix(vv.String(), "<bytecode.CallFrame") {
			return fmt.Sprintf("F<%v>", v)
		}

		if strings.HasPrefix(vv.String(), "<bytecode.ConstantWrapper") {
			return fmt.Sprintf("%v", v)
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
