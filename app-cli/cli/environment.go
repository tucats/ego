// The "cli" package provides basic grammar-based processing of command line options,
// and dispatches to action routines when a completed grammar is processed. It also
// includes the builtin "help" command and the associated "-h" option added to any
// subcommand.
package cli

import (
	"os"
	"strconv"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
)

// ResolveEnvironmentVariables searches the grammar tree backwards looking
// for options that can be specified by an environment variable, and
// marking those found as needed. This is done after the command line
// options are processed, to provide defaults for un-specified options.
func (c *Context) ResolveEnvironmentVariables() error {
	var err error

	// Search the current tree. Note that if we find the item,
	// the updates have to be written back to the option array,
	// not to the local entry which is a copy of the item...
	for found, entry := range c.Grammar {
		if !entry.Found && entry.EnvVar > "" {
			if value, wasFound := os.LookupEnv(entry.EnvVar); wasFound {
				ui.Log(ui.CLILogger, "resolving env %s = \"%s\"", entry.EnvVar, value)

				c.Grammar[found].Found = true

				switch c.Grammar[found].OptionType {
				case BooleanType:
					c.Grammar[found].Value = value != ""

				case BooleanValueType:
					c.Grammar[found].Value, _ = validateBoolean(value)

				case IntType:
					c.Grammar[found].Value, _ = strconv.Atoi(value)

				case StringListType:
					c.Grammar[found].Value = strings.Split(value, ",")

				default:
					c.Grammar[found].Value = value
				}

				if c.Grammar[found].Action != nil {
					ui.Log(ui.CLILogger, "Invoking %s handler for value %#v", c.Grammar[found].LongName, c.Grammar[found].Value)

					err = c.Grammar[found].Action(c)
				}
			}
		}
	}

	// If there is a parent grammar, search that as well.
	if err == nil && c.Parent != nil {
		err = c.Parent.ResolveEnvironmentVariables()
	}

	return err
}
