package cli

import (
	"fmt"
	"reflect"
	"runtime"
	"strconv"
	"strings"
)

// DumpGrammar is an internal routine used to print out a textual representation
// of the entire grammar tree associated with the context. This includes chained
// grammar structures, and the default grammar values provided by the app-cli
// package (logon, config, and the help options and subcommand).
func DumpGrammar(ctx *Context) {
	fmt.Println("// Representation of the CLI grammar. This is used for diagnostic")
	fmt.Println("// purposes only, and is not compiled into the program.")
	fmt.Printf("\nvar context = &Context ")

	dumpGrammarLevel(ctx, 0)
}

// dumpGrammarLevel recursively prints a Go-literal representation of one
// Context at the given indentation level. Nested grammars (held in Option.Value
// for Subcommand entries) are printed at level+1. The output is not valid Go
// source on its own — it is a diagnostic snapshot intended for human reading.
func dumpGrammarLevel(ctx *Context, level int) {
	prefix := strings.Repeat("  ", level)

	fmt.Printf("%s  {\n", prefix)

	dumpItem(level+1, "AppName", ctx.AppName)
	dumpItem(level+1, "MainProgram", ctx.MainProgram)
	dumpItem(level+1, "Description", ctx.Description)
	dumpItem(level+1, "Copyright", ctx.Copyright)
	dumpItem(level+1, "Version", ctx.Version)
	dumpItem(level+1, "Command", ctx.Command)
	dumpItem(level+1, "Grammar", ctx.Grammar)
	dumpItem(level+1, "Parameters", ctx.Parameters)

	if level > 0 {
		dumpItem(level+1, "Action", ctx.Action)
		dumpItem(level+1, "Args", ctx.Args)
	}

	dumpItem(level+1, "ParameterDescription", ctx.ParameterDescription)
	dumpItem(level+1, "Expected", ctx.Expected)

	fmt.Printf("%s  }\n", prefix)
}

// dumpOption prints a single Option struct in a pseudo-Go-literal style at
// the given indentation level. When comma is true, a trailing comma is
// appended (used when this option is not the last element in a slice).
func dumpOption(level int, option Option, comma bool) {
	prefix := strings.Repeat("  ", level)
	fmt.Printf("%s  {\n", prefix)

	dumpItem(level+1, "LongName", option.LongName)
	dumpItem(level+1, "ShortName", option.ShortName)
	dumpItem(level+1, "Aliases", option.Aliases)
	dumpItem(level+1, "Description", option.Description)
	dumpItem(level+1, "ParameterDescription", option.ParmDesc)
	dumpItem(level+1, "ParametersExpected", option.ExpectedParms)
	dumpItem(level+1, "OptionType", optionType(option.OptionType))
	dumpItem(level+1, "Keywords", option.Keywords)
	dumpItem(level+1, "Action", option.Action)
	dumpItem(level+1, "Value", option.Value)
	dumpItem(level+1, "Required", option.Required)
	dumpItem(level+1, "Private", option.Private)

	commaString := ""
	if comma {
		commaString = ","
	}

	fmt.Printf("%s  }%s\n", prefix, commaString)
}

// dumpItem prints a single field of a struct in pseudo-Go-literal style. It
// uses a type switch so that each Go type is formatted appropriately:
//   - nil or zero values are silently omitted (keeping the output concise).
//   - Strings are quoted unless they begin with "!" (a signal that the string
//     already contains a valid Go expression and should be emitted verbatim).
//   - Slices of strings and []Option are printed with their own helpers.
//   - Function values are printed by resolving their symbol name via reflection.
func dumpItem(level int, label string, value any) {
	prefix := strings.Repeat("  ", level)

	switch v := value.(type) {
	case nil:

	case []string:
		dumpStringArrayItem(v, prefix, label)

	case string:
		dumpStringItem(v, prefix, label)

	case int:
		if v != 0 {
			fmt.Printf("%s  %s %d,\n", prefix, pad(label), v)
		}

	case bool:
		if v {
			fmt.Printf("%s  %s %v,\n", prefix, pad(label), v)
		}

	case *Context:
		if v != nil {
			fmt.Printf("%s  %s,\n", prefix, pad(label))
			dumpGrammarLevel(v, level+1)
		}

	case []Option:
		dumpOptionArrayItem(v, prefix, label, level)

	case func(*Context) error:
		dumpActionRoutineItem(v, prefix, label)

	default:
		fmt.Printf("%s  %s %v,\n", prefix, pad(label), v)
	}
}

// dumpActionRoutineItem prints an action function field. Because functions are
// first-class values in Go there is no built-in way to print a function's name —
// we use the reflect and runtime packages to look up the symbol name from the
// function's code pointer. Package path prefixes are stripped for readability.
func dumpActionRoutineItem(v func(*Context) error, prefix string, label string) {
	if v != nil {
		vv := reflect.ValueOf(v)
		// If it's an internal function, show it's name. If it is a standard builtin from the
		// function library, show the short form of the name.
		if vv.Kind() == reflect.Func {
			name := runtime.FuncForPC(reflect.ValueOf(v).Pointer()).Name()
			name = strings.Replace(name, "github.com/tucats/ego/", "", 1)
			name = strings.Replace(name, "github.com/tucats/ego/runtime.", "", 1)
			fmt.Printf("%s  %s %s(),\n", prefix, pad(label), name)
		}
	}
}

// dumpOptionArrayItem prints a []Option slice as a Go-style slice literal,
// calling dumpOption for each element. Empty slices are silently skipped.
func dumpOptionArrayItem(v []Option, prefix string, label string, level int) {
	if len(v) > 0 {
		fmt.Printf("%s  %s []Option{\n", prefix, pad(label))

		for n, option := range v {
			dumpOption(level+1, option, n < len(v))
		}

		fmt.Printf("%s  },\n", prefix)
	}
}

// dumpStringItem prints a string field. If the value begins with "!" the
// leading "!" is stripped and the remainder is printed unquoted — this is the
// convention used by optionType() to embed raw Go identifiers in the output.
// Empty strings are silently skipped.
func dumpStringItem(v string, prefix string, label string) {
	if v != "" {
		if strings.HasPrefix(v, "!") {
			fmt.Printf("%s  %s %s,\n", prefix, pad(label), v[1:])
		} else {
			fmt.Printf("%s  %s %s,\n", prefix, pad(label), strconv.Quote(v))
		}
	}
}

// dumpStringArrayItem prints a []string field as a Go-style slice literal
// (e.g. []string{ "a", "b" }). Empty slices are silently skipped.
func dumpStringArrayItem(v []string, prefix string, label string) {
	if len(v) > 0 {
		a := strings.Builder{}

		a.WriteString("[]string{ ")

		for n, i := range v {
			if n > 0 {
				a.WriteString(", ")
			}

			a.WriteRune('"')
			a.WriteString(i)
			a.WriteRune('"')
		}

		a.WriteString(" }")
		fmt.Printf("%s  %s %s,\n", prefix, pad(label), a.String())
	}
}

// optionType converts a numeric OptionType constant to the human-readable name
// of the corresponding Go constant. The returned string is prefixed with "!" so
// that dumpStringItem prints it without surrounding quotes (raw Go identifier).
// Out-of-range values are formatted as "!Invalid(n)".
func optionType(t int) string {
	var (
		typeNames = []string{
			"None 0",
			"StringType",
			"IntType",
			"BooleanType",
			"BooleanValueType",
			"None 5",
			"SubCommand",
			"StringListType",
			"ParameterType",
			"UUIDType",
			"KeywordType",
		}
		name string
	)

	if t < 0 || t > len(typeNames) {
		name = fmt.Sprintf("!Invalid(%d)", t)
	} else {
		name = "!" + typeNames[t]
	}

	return name
}

// pad right-pads a field label with spaces until it reaches the width of the
// longest expected label ("ExpectedParameterCount:"), so that value columns
// line up neatly in the diagnostic output.
func pad(s string) string {
	s = s + ":"
	for len(s) < len("ExpectedParameterCount:") {
		s = s + " "
	}

	return s
}
