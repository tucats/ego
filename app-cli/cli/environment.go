package cli

import (
	"os"
	"strconv"

	"github.com/tucats/ego/app-cli/ui"
)

// ResolveEnvironmentVariables searches the grammar tree backwards
// looking for options that can be specified by an environment
// variable, and marking those found as needed.
func (c *Context) ResolveEnvironmentVariables() error {
	var err error

	// Search the current tree. Note that if we find the item,
	// the updates have to be written back to the option array,
	// not to the local entry which is a copy of the item...
	for found, entry := range c.Grammar {
		if !entry.Found && entry.EnvironmentVariable > "" {
			value, wasFound := os.LookupEnv(entry.EnvironmentVariable)
			if wasFound {
				ui.Debug(ui.CLILogger, "resolving env %s = \"%s\"", entry.EnvironmentVariable, value)

				c.Grammar[found].Found = true

				switch c.Grammar[found].OptionType {
				case BooleanType:
					c.Grammar[found].Value = value != ""

				case BooleanValueType:
					c.Grammar[found].Value, _ = ValidateBoolean(value)

				case IntType:
					c.Grammar[found].Value, _ = strconv.Atoi(value)

				default:
					c.Grammar[found].Value = value
				}

				if c.Grammar[found].Action != nil {
					ui.Debug(ui.CLILogger, "Invoking %s handler for value %#v", c.Grammar[found].LongName, c.Grammar[found].Value)

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
