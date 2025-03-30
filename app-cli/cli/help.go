package cli

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/tucats/ego/app-cli/tables"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/i18n"
)

// Default indentation for subordinate text, and default
// spacing between columns.
const (
	helpIndent       = 3
	helpSpacing      = 3
	optMessagePrefix = "opt."
	optionTextWidth  = 26
)

// ShowHelp displays help text for the grammar, using a standardized format.
// The help shows subcommands as well as options, including value type cues.
// The output is automatically directed to the stdout console output.
//
// This function uses the tables package to create uniform columns of output.
func ShowHelp(c *Context) {
	if c.Copyright != "" {
		fmt.Printf("%s\n", c.Copyright)
	}

	// Prepare a composed version of the command string, which chains
	// together the root, subverbs, and representations of parameters
	// and options.
	composedCommand := c.MainProgram + " " + c.Command
	hasSubcommand, hasOptions := assessGrammar(c)

	if hasOptions {
		composedCommand = composedCommand + "[" + i18n.L("options") + "] "
	}

	if hasSubcommand {
		composedCommand = composedCommand + "[" + i18n.L("command") + "] "
	}

	g := c.FindGlobal()
	e := g.Expected

	composedCommand = addOptionalParameters(g, composedCommand, e)

	minimumFirstColumnWidth := len(composedCommand)
	if minimumFirstColumnWidth < optionTextWidth {
		minimumFirstColumnWidth = optionTextWidth
	}

	commandDescription := i18n.T(c.Description)

	if c.Parent == nil && c.Version != "" {
		commandDescription = commandDescription + ", " + c.Version
	}

	fmt.Printf("\n%s:\n   %-26s   %s\n\n", i18n.L("Usage"), composedCommand, commandDescription)

	// Now prepare the descriptions of the subcommands. This is done using a
	// table format, where the headings are not printed. But this lets the
	// sucommands and their descriptions line up nicely.
	headerShown := false

	tc, _ := tables.New([]string{"subcommand", "description"})

	tc.ShowHeadings(false)
	tc.SetPagination(0, 0)

	_ = tc.SetIndent(helpIndent)
	_ = tc.SetSpacing(helpSpacing)
	_ = tc.SetMinimumWidth(0, minimumFirstColumnWidth)

	hadDefaultVerb := false

	headerShown, hadDefaultVerb = displaySubcommands(c, headerShown, tc, hadDefaultVerb)

	if headerShown {
		_ = tc.SortRows(0, true)

		tc.Print(ui.TextFormat)
		fmt.Printf("\n")
	}

	if hadDefaultVerb {
		fmt.Printf("%s\n\n", i18n.L("had.default.verb"))
	}

	headerShown = false
	tc, _ = tables.New([]string{i18n.L("parameter")})

	tc.ShowHeadings(false)
	tc.SetPagination(0, 0)

	_ = tc.SetIndent(helpIndent)
	_ = tc.SetMinimumWidth(0, minimumFirstColumnWidth)

	headerShown = displayParameters(c, headerShown, tc)

	if headerShown {
		tc.Print(ui.TextFormat)
		fmt.Printf("\n")
	}

	// Now, use tables again to format the list of options
	// and their descriptions.
	to, _ := tables.New([]string{"option", "description"})

	to.ShowHeadings(false)
	to.SetPagination(0, 0)

	_ = to.SetIndent(helpIndent)
	_ = to.SetSpacing(helpSpacing)
	_ = to.SetMinimumWidth(0, minimumFirstColumnWidth)

	addOptionsToTable(c, to)

	fmt.Printf("Options:\n")

	_ = to.AddRow([]string{"--help, -h", i18n.T("opt.help.text")})
	_ = to.SortRows(0, true)
	_ = to.Print(ui.TextFormat)
}

func addOptionsToTable(c *Context, to *tables.Table) {
	for _, option := range c.Grammar {
		if option.Private {
			continue
		}

		unsupported := false

		for _, platform := range option.Unsupported {
			if runtime.GOOS == platform {
				unsupported = true

				break
			}
		}

		if unsupported {
			continue
		}

		if option.OptionType != Subcommand {
			name := formatOptionName(option)

			fullDescription := i18n.T(option.Description)
			if fullDescription == option.Description {
				fullDescription = i18n.T(optMessagePrefix + option.Description)
			}

			if option.EnvVar != "" {
				fullDescription = fullDescription + " [" + option.EnvVar + "]"
			}

			_ = to.AddRow([]string{name, fullDescription})
		}
	}
}

func formatOptionName(option Option) string {
	name := ""
	if option.LongName > "" {
		name = "--" + option.LongName
	}

	if option.ShortName > "" {
		if name > "" {
			name = name + ", "
		}

		name = name + "-" + option.ShortName
	}

	switch option.OptionType {
	case IntType:
		name = name + " <integer>"

	case StringType:
		name = name + " <string>"

	case BooleanValueType:
		name = name + " <boolean>"

	case StringListType:
		name = name + " <list>"

	case RangeType:
		name = name + " <range>"

	case KeywordType:
		name = name + " " + strings.Join(option.Keywords, "|")
	}

	return name
}

func displayParameters(c *Context, headerShown bool, tc *tables.Table) bool {
	for _, option := range c.Grammar {
		if option.OptionType == ParameterType {
			if !headerShown {
				fmt.Printf("%s:\n", i18n.L("Parameters"))

				headerShown = true

				optionDescription := i18n.T(option.Description)
				if optionDescription == c.Description {
					optionDescription = i18n.T(optMessagePrefix + option.Description)
				}

				_ = tc.AddRowItems(optionDescription)
			}
		}
	}

	return headerShown
}

func displaySubcommands(c *Context, headerShown bool, tc *tables.Table, hadDefaultVerb bool) (bool, bool) {
	for _, option := range c.Grammar {
		if option.OptionType == Subcommand && !option.Private {
			unsupported := false

			for _, platform := range option.Unsupported {
				if runtime.GOOS == platform {
					unsupported = true

					break
				}
			}

			if unsupported {
				continue
			}

			if !headerShown {
				fmt.Printf("%s:\n", i18n.L("Commands"))

				_ = tc.AddRow([]string{"help", i18n.T("opt.help.text")})
				headerShown = true
			}

			optionDescription := i18n.T(option.Description)
			if optionDescription == c.Description {
				optionDescription = i18n.T(optMessagePrefix + option.Description)
			}

			defaultFlag := ""
			if option.DefaultVerb {
				defaultFlag = " (*)"
				hadDefaultVerb = true
			}

			_ = tc.AddRow([]string{option.LongName + defaultFlag, optionDescription})
		}
	}

	return headerShown, hadDefaultVerb
}

// If there is a parameter description or expected parameters, add text to the
// command text being composed to indicate the paraemters. If there is a description
// string, use that, else a generic term for parameter.
func addOptionalParameters(g *Context, composedCommand string, e int) string {
	if g.ParameterDescription > "" {
		parmDesc := i18n.T(g.ParameterDescription)

		if g.Expected < 1 {
			if g.MinParams == 0 {
				parmDesc = "[" + parmDesc + "]"
			}
		}

		composedCommand = composedCommand + " " + parmDesc
	} else if e == 1 {
		composedCommand = composedCommand + " [" + i18n.L("parameter") + "]"
	} else if e > 1 {
		composedCommand = composedCommand + " [" + i18n.L("parameters") + "]"
	}

	return composedCommand
}

// Scan this level of the grammar tree to determine if there are subcommands or options.
func assessGrammar(c *Context) (bool, bool) {
	hasSubcommand := false
	hasOptions := false

	for _, option := range c.Grammar {
		if option.OptionType == Subcommand {
			hasSubcommand = true
		} else {
			hasOptions = true
		}
	}

	return hasSubcommand, hasOptions
}
