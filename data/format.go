package data

// Handle generic formatting chores for all data types.

import (
	"fmt"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/tucats/ego/defs"
	"gopkg.in/resty.v1"
)

var verbose = false

// This is a map of well known types and their corresponding format strings.
var knownTypes = map[string]string{
	"tokenizer.Token":      "%v",
	"Tokenizer.Tokenizer":  "%v",
	"bytecode.StackMarker": "%v",
	"bytecode.CallFrame":   "Frame<%v>",
	"bytecode.Immutable":   "%v",
}

// FormatUnquoted formats a value but does not put quotes on strings.
func FormatUnquoted(arg interface{}) string {
	if arg == nil {
		return defs.NilTypeString
	}

	switch v := arg.(type) {
	case string:
		return v

	default:
		return Format(v)
	}
}

// This formats a value with additional decoration that describes the
// type of the object. Strings are identified by being in quotes, and
// other types have a type enclosure, such as int(42) for the value
// 42 expressed as an int. This is used to format data on the stack
// during debugging and tracking, for example.
func FormatWithType(element interface{}) string {
	if element == nil {
		return defs.NilTypeString
	}

	switch actual := element.(type) {
	case error:
		return "E<" + actual.Error() + ">"

	case bool:
		if actual {
			return "true"
		}

		return "false"

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

	case *Type:
		return actual.String()

	case *Array:
		return actual.StringWithType()

	case *Struct:
		return actual.StringWithType()

	case *Map:
		return actual.StringWithType()
	}

	fmtString := Format(element)

	// For things already formatted with a type prefix, don't add to the string.
	for _, prefix := range []string{"[", "Marker<", "Frame<", "Package<"} {
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
	ts := TypeOf(element).String()
	if ts == InterfaceTypeName {
		return fmtString
	}

	return ts + "{" + fmtString + "}"
}

// Format a value as a human-readable value, such as you would see from fmt.Printf()
// for the associated value. This includes formatting for non-concrete objects, such
// as types, nil values, constants, etc.
func Format(element interface{}) string {
	if IsNil(element) {
		return defs.NilTypeString
	}

	switch v := element.(type) {
	// Interface object
	case Interface:
		return formatInterface(v)

	// Immutable object
	case Immutable:
		return "^" + Format(v.Value)

	// Built-in types
	case Type:
		return v.String() + v.FunctionNameList()

	// User types
	case *Type:
		return v.String() + v.FunctionNameList()

	case *time.Time:
		return v.String()

	case Channel:
		return v.String()

	case *Channel:
		return "*" + v.String()

	case error:
		return v.Error()

	case bool:
		if v {
			return defs.True
		}

		return defs.False

	case byte:
		return strconv.Itoa(int(v))

	case int32:
		return strconv.Itoa(int(v))

	case int:
		return strconv.Itoa(v)

	case int64:
		return strconv.FormatInt(v, 10)

	case float32:
		return strconv.FormatFloat(float64(v), 'g', 8, 32)

	case float64:
		return strconv.FormatFloat(v, 'g', 10, 64)

	case string:
		return "\"" + v + "\""

	case *Declaration:
		return v.String()

	case *Package:
		return formatPackageAsString(v)

	case *Struct:
		return v.String()

	case *Array:
		return v.String()

	case *Map:
		return v.String()

	case *interface{}:
		return formatInterfacePointer(v)

	case Function:
		if v.Declaration != nil {
			return v.Declaration.String()
		}

		return Format(v.Value)

	case List:
		return formatList(v)

	case []interface{}:
		return formatInterfaceArray(v)

	default:
		// See if this is a package type we know about that has a formatter. This data
		// is stored in a map, so serialize this access.
		packageTypesLock.Lock()
		defer packageTypesLock.Unlock()

		typeName := reflect.TypeOf(v).String()

		t, found := packageTypes[typeName]
		if !found && strings.HasPrefix(typeName, "*") {
			typeName = strings.TrimPrefix(typeName, "*")

			t, found = packageTypes[typeName]
			if found {
				typeName = typeName[:1]
			}
		}

		if found && t.format != nil {
			return t.format(v)
		}

		// Not a known type, so see if we can figure ou the native value on our own.
		return formatNativeGoValue(v)
	}
}

func (t *Type) SetFormatFunc(f func(interface{}) string) *Type {
	t.format = f

	return t
}

func formatPackageAsString(v *Package) string {
	var b strings.Builder

	keys := v.Keys()

	b.WriteString("Package<")
	b.WriteString(strconv.Quote(v.Name))

	if v.Builtins {
		b.WriteString(", builtins")
	}

	if v.Source {
		b.WriteString(", source")
	}

	if v.HasTypes() {
		b.WriteString(", types")
	}

	if verbose {
		for _, k := range keys {
			// Skip over invisible symbols or symbols that are not exported.
			if strings.HasPrefix(k, defs.InvisiblePrefix) || !hasCapitalizedName(k) {
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
}

func formatNativeGoValue(v interface{}) string {
	vv := reflect.ValueOf(v)

	// IF it's an internal function, show it's name. If it is a standard builtin from the
	// function library, show the short form of the name.
	if vv.Kind() == reflect.Func {
		fn := runtime.FuncForPC(reflect.ValueOf(v).Pointer())

		name := fn.Name()
		name = strings.Replace(name, "github.com/tucats/ego/builtins.", "", 1)
		name = strings.Replace(name, "github.com/tucats/ego/", "", 1)

		if name == "" {
			name = defs.Anon
		}

		// Ugly hack. The Ego len() function cannot be named "len" because it would
		// shadow the Go function is must use. So look for the special case of a
		// builtin named "Length" and remap it to "len".
		if name == "Length" {
			name = "len"
		}

		// Get the declaration from the dictionary, and format as a string.
		// Something went wrong, there isn't a dictionary entry. Just format
		// it so it makes some sense.
		if d, found := BuiltinsDictionary[strings.ToLower(name)]; found {
			name = d.String()
		} else {
			name = "builtin " + name + "()"
		}

		return name
	}

	ts := strings.TrimPrefix(vv.String(), "<")
	for key, format := range knownTypes {
		if strings.Contains(ts, key) {
			return fmt.Sprintf(format, v)
		}
	}

	if vv.Kind() == reflect.Struct {
		// Use reflection to format the contents of the structure vv
		return formatNativeStruct(v)
	}

	if vv.Kind() == reflect.Ptr {
		ts := vv.String()

		// If it's a bytecode.Bytecode pointer, use reflection to get the
		// Name field value and use that with the name. A function literal
		// will have no name.
		if ts == defs.ByteCodeReflectionTypeString {
			return fmt.Sprintf("%v", v)
		}

		return strings.TrimPrefix(strings.TrimSuffix(ts, " Value>"), "<") + " " + formatNativeGoValue(vv.Elem())
	}

	ts = strings.TrimPrefix(vv.String(), "<")
	for key, format := range knownTypes {
		if strings.Contains(ts, key) {
			return fmt.Sprintf(format, v)
		}
	}

	valueString := fmt.Sprintf("%v", v)
	valueKind := vv.Kind()

	// If it's a slice of an interface array, used to pass compound
	// parameters to bytecodes, then format it as {a, b, c}
	if valueKind == reflect.Slice {
		return valueString
	}

	return fmt.Sprintf("%v %v %v", vv.Type(), vv.Kind(), v)
}

// formatNativeStruct returns a string representation of a struct. The string
// is based on the native Reflection API.
func formatNativeStruct(v interface{}) string {
	var b strings.Builder

	vv := reflect.ValueOf(v)
	ts := vv.String()
	ts = strings.TrimPrefix(ts, "<")
	ts = strings.TrimSuffix(ts, " Value>") + " "

	if ts != "reflect.Value " {
		b.WriteString(ts)
	}

	b.WriteString("struct{")

	for i := 0; i < vv.NumField(); i++ {
		field := vv.Type().Field(i)

		if i > 0 {
			b.WriteString(", ")
		}

		b.WriteString(field.Name)
		b.WriteString(": ")
		b.WriteString(vv.Field(i).Type().String())
	}

	b.WriteRune('}')
	b.WriteString(fmt.Sprintf(" = %v", v))

	return b.String()
}

// formatInterface returns a string representation of an interface. The string
// representation includes the base type and the value it wraps.
func formatInterface(v Interface) string {
	if v.BaseType == nil {
		return InterfaceTypeName
	}

	return "interface{ " + v.BaseType.String() + " " + Format(v.Value) + " }"
}

// formatInterfacePointer returns a string representation of an interface pointer.
// It is aware of a few specific Go-native interface pointers that it resolves to
// the corresponding Ego type.
func formatInterfacePointer(v *interface{}) string {
	if v != nil {
		vv := *v
		switch vv := vv.(type) {
		case *resty.Client:
			return "&rest.Client{}"

		default:
			return "&" + Format(vv)
		}
	} else {
		return "nil<*interface{}>"
	}
}

// formatList returns a string representation of a list.
func formatList(v List) string {
	text := strings.Builder{}

	text.WriteString("L<")

	for i := 0; i < v.Len(); i++ {
		if i > 0 {
			text.WriteString(", ")
		}

		text.WriteString(Format(v.Get(i)))
	}

	text.WriteString(">")

	return text.String()
}

// formatInterfaceArray returns a string representation of an interface array.
func formatInterfaceArray(v []interface{}) string {
	text := strings.Builder{}

	for i := 0; i < len(v); i++ {
		if i > 0 {
			text.WriteString(", ")
		}

		text.WriteString(Format(v[i]))
	}

	return text.String()
}
