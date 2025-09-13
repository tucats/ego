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

func dumpOptionArrayItem(v []Option, prefix string, label string, level int) {
	if len(v) > 0 {
		fmt.Printf("%s  %s []Option{\n", prefix, pad(label))

		for n, option := range v {
			dumpOption(level+1, option, n < len(v))
		}

		fmt.Printf("%s  },\n", prefix)
	}
}

func dumpStringItem(v string, prefix string, label string) {
	if v != "" {
		if strings.HasPrefix(v, "!") {
			fmt.Printf("%s  %s %s,\n", prefix, pad(label), v[1:])
		} else {
			fmt.Printf("%s  %s %s,\n", prefix, pad(label), strconv.Quote(v))
		}
	}
}

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

func pad(s string) string {
	s = s + ":"
	for len(s) < len("ExpectedParameterCount:") {
		s = s + " "
	}

	return s
}
