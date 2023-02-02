package cli

import (
	"fmt"
	"reflect"
	"runtime"
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

	p(level+1, "AppName", ctx.AppName)
	p(level+1, "MainProgram", ctx.MainProgram)
	p(level+1, "Description", ctx.Description)
	p(level+1, "Copyright", ctx.Copyright)
	p(level+1, "Version", ctx.Version)
	p(level+1, "Command", ctx.Command)
	p(level+1, "Grammar", ctx.Grammar)
	p(level+1, "Parameters", ctx.Parameters)

	if level > 0 {
		p(level+1, "Action", ctx.Action)
		p(level+1, "Args", ctx.Args)
	}

	p(level+1, "ParameterDescription", ctx.ParameterDescription)
	p(level+1, "Expected", ctx.Expected)

	fmt.Printf("%s  }\n", prefix)
}

func dumpOption(level int, option Option, comma bool) {
	prefix := strings.Repeat("  ", level)
	fmt.Printf("%s  {\n", prefix)

	p(level+1, "LongName", option.LongName)
	p(level+1, "ShortName", option.ShortName)
	p(level+1, "Aliases", option.Aliases)
	p(level+1, "Description", option.Description)
	p(level+1, "ParameterDescription", option.ParameterDescription)
	p(level+1, "ParametersExpected", option.ParametersExpected)
	p(level+1, "OptionType", optionType(option.OptionType))
	p(level+1, "Keywords", option.Keywords)
	p(level+1, "Action", option.Action)
	p(level+1, "Value", option.Value)
	p(level+1, "Required", option.Required)
	p(level+1, "Private", option.Private)

	commaString := ""
	if comma {
		commaString = ","
	}

	fmt.Printf("%s  }%s\n", prefix, commaString)
}

func p(level int, label string, value interface{}) {
	prefix := strings.Repeat("  ", level)

	switch v := value.(type) {
	case nil:

	case []string:
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

	case string:
		if v != "" {
			if strings.HasPrefix(v, "!") {
				fmt.Printf("%s  %s %s,\n", prefix, pad(label), v[1:])
			} else {
				fmt.Printf("%s  %s \"%s\",\n", prefix, pad(label), v)
			}
		}

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
		if len(v) > 0 {
			fmt.Printf("%s  %s []Option{\n", prefix, pad(label))

			for n, option := range v {
				dumpOption(level+1, option, n < len(v))
			}

			fmt.Printf("%s  },\n", prefix)
		}

	case func(*Context) error:
		if v != nil {
			vv := reflect.ValueOf(v)

			// IF it's an internal function, show it's name. If it is a standard builtin from the
			// function library, show the short form of the name.
			if vv.Kind() == reflect.Func {
				name := runtime.FuncForPC(reflect.ValueOf(v).Pointer()).Name()
				name = strings.Replace(name, "github.com/tucats/ego/", "", 1)
				name = strings.Replace(name, "github.com/tucats/ego/runtime.", "", 1)
				fmt.Printf("%s  %s %s(),\n", prefix, pad(label), name)
			}
		}

	default:
		fmt.Printf("%s  %s %v,\n", prefix, pad(label), v)
	}
}

func optionType(t int) string {
	typeNames := []string{
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

	var name string

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
