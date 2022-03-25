package cli

import (
	"fmt"

	"github.com/tucats/ego/app-cli/tables"
	"github.com/tucats/ego/app-cli/ui"
)

// Default indentation for subordinate text, and default
// spacing between columns.
const (
	helpIndent  = 3
	helpSpacing = 3
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
	hasSubcommand := false
	hasOptions := false

	for _, option := range c.Grammar {
		if option.OptionType == Subcommand {
			hasSubcommand = true
		} else {
			hasOptions = true
		}
	}

	if hasOptions {
		composedCommand = composedCommand + "[options] "
	}

	if hasSubcommand {
		composedCommand = composedCommand + "[command] "
	}

	g := c.FindGlobal()
	e := g.ExpectedParameterCount

	if g.ParameterDescription > "" {
		parmDesc := g.ParameterDescription
		if g.ExpectedParameterCount < 1 {
			parmDesc = "[" + parmDesc + "]"
		}

		composedCommand = composedCommand + " " + parmDesc
	} else if e == 1 {
		composedCommand = composedCommand + " [parameter]"
	} else if e > 1 {
		composedCommand = composedCommand + " [parameters]"
	}

	minimumFirstColumnWidth := len(composedCommand)
	if minimumFirstColumnWidth < 26 {
		minimumFirstColumnWidth = 26
	}

	if c.Parent == nil && c.Version != "" {
		c.Description = c.Description + ", " + c.Version
	}

	fmt.Printf("\nUsage:\n   %-26s   %s\n\n", composedCommand, c.Description)

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

	for _, option := range c.Grammar {
		if option.OptionType == Subcommand && !option.Private {
			if !headerShown {
				fmt.Printf("Commands:\n")

				_ = tc.AddRow([]string{"help", "Display help text"})
				headerShown = true
			}

			_ = tc.AddRow([]string{option.LongName, option.Description})
		}
	}

	if headerShown {
		_ = tc.SortRows(0, true)

		tc.Print(ui.TextFormat)
		fmt.Printf("\n")
	}

	headerShown = false
	tc, _ = tables.New([]string{"Parameter"})

	tc.ShowHeadings(false)
	tc.SetPagination(0, 0)

	_ = tc.SetIndent(helpIndent)
	_ = tc.SetMinimumWidth(0, minimumFirstColumnWidth)

	for _, option := range c.Grammar {
		if option.OptionType == ParameterType {
			if !headerShown {
				fmt.Printf("Parameters:\n")

				headerShown = true
				_ = tc.AddRowItems(option.Description)
			}
		}
	}

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

	for _, option := range c.Grammar {
		if option.Private {
			continue
		}

		if option.OptionType != Subcommand {
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
			}

			fullDescription := option.Description

			if option.EnvironmentVariable != "" {
				fullDescription = fullDescription + " [" + option.EnvironmentVariable + "]"
			}

			_ = to.AddRow([]string{name, fullDescription})
		}
	}

	fmt.Printf("Options:\n")

	_ = to.AddRow([]string{"--help, -h", "Show this help text"})
	_ = to.SortRows(0, true)
	_ = to.Print(ui.TextFormat)
}
