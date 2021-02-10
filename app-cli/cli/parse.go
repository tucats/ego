package cli

import (
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/errors"
)

// Parse processes the grammar associated with the current context,
// using the []string array of arguments for that context.
//
// Unrecognized options or subcommands, as well as invalid values
// are reported as an error. If there is an action routine associated
// with an option or a subcommand, that action is executed.
func (c *Context) Parse() *errors.EgoError {
	args := c.Args
	c.MainProgram = filepath.Base(args[0])
	c.Command = ""

	// If there are no arguments other than the main program name, dump out the help by default.
	if len(args) == 1 && c.Action == nil {
		ShowHelp(c)

		return nil
	}

	// Start parsing using the top-level grammar.
	return c.parseGrammar(args[1:])
}

// ParseGrammar accepts an argument list and parses it using the current context grammar
// definition. This is abstracted from Parse because it allows for recursion for subcomamnds.
// This is never called by the user directly.
func (c *Context) parseGrammar(args []string) *errors.EgoError {
	var err *errors.EgoError

	lastArg := len(args)
	parametersOnly := false
	helpVerb := true

	for currentArg := 0; currentArg < lastArg; currentArg++ {
		var location *Option

		var name string

		var value string

		option := args[currentArg]
		isShort := false

		ui.Debug(ui.CLILogger, "Processing token: %s", option)

		// Are we now only eating parameter values?
		if parametersOnly {
			globalContext := c.FindGlobal()
			globalContext.Parameters = append(globalContext.Parameters, option)
			count := len(globalContext.Parameters)

			ui.Debug(ui.CLILogger, "added parameter %d", count)

			continue
		}

		// Handle the special cases automatically.
		if (helpVerb && option == "help") || option == "-h" || option == "--help" {
			ShowHelp(c)

			return nil
		}

		// Handle the "empty option" that means the remainder of the command
		// line will be treated as parameters, even if it looks like it has
		// options, etc.
		if option == "--" {
			parametersOnly = true
			helpVerb = false

			continue
		}

		// If it's a long-name option, search for it.
		if len(option) > 2 && option[:2] == "--" {
			name = option[2:]
		} else if len(option) > 1 && option[:1] == "-" {
			name = option[1:]
			isShort = true
		}

		value = ""
		hasValue := false

		if equals := strings.Index(name, "="); equals >= 0 {
			value = name[equals+1:]
			name = name[:equals]
			hasValue = true
		}

		location = nil

		if name > "" {
			for n, entry := range c.Grammar {
				if (isShort && entry.ShortName == name) || (!isShort && entry.LongName == name) {
					location = &(c.Grammar[n])

					break
				} else {
					for _, a := range entry.Aliases {
						if a == name {
							location = &(c.Grammar[n])

							break
						}
					}
				}
			}
		}

		// If it was an option (short or long) and not found, this is an error.
		if name != "" && location == nil {
			return errors.New(errors.UnknownOptionError).WithContext(option)
		}

		// It could be a parameter, or a subcommand.
		if location == nil {
			// Is it a subcommand?
			for _, entry := range c.Grammar {
				// Is it one of the aliases permitted?
				isAlias := false

				for _, n := range entry.Aliases {
					if option == n {
						isAlias = true

						break
					}
				}

				if (isAlias || entry.LongName == option) && entry.OptionType == Subcommand {
					// We're doing a subcommand! Create a new context that defines the
					// next level down. It should include the current context information,
					// and an updated grammar tree, command text, and description adapted
					// for this subcommand.
					subContext := *c
					subContext.Parent = c

					if entry.Value != nil {
						subContext.Grammar = entry.Value.([]Option)
					} else {
						subContext.Grammar = []Option{}
					}

					subContext.Command = c.Command + entry.LongName + " "
					subContext.Description = entry.Description
					entry.Found = true
					c.FindGlobal().ExpectedParameterCount = entry.ParametersExpected
					c.FindGlobal().ParameterDescription = entry.ParameterDescription

					if entry.Action != nil {
						subContext.Action = entry.Action

						ui.Debug(ui.CLILogger, "Saving action routine in subcommand context")
					}

					ui.Debug(ui.CLILogger, "Transferring control to subgrammar for %s", entry.LongName)

					return subContext.parseGrammar(args[currentArg+1:])
				}
			}

			// Not a subcommand, just save it as an unclaimed parameter
			g := c.FindGlobal()
			g.Parameters = append(g.Parameters, option)
			count := len(g.Parameters)

			ui.Debug(ui.CLILogger, "Unclaimed token added parameter %d", count)
		} else {
			location.Found = true
			// If it's not a boolean type, see it already has a value from the = construct.
			// If not, claim the next argument as the value.
			if location.OptionType != BooleanType {
				if !hasValue {
					currentArg = currentArg + 1
					if currentArg >= lastArg {
						return errors.New(errors.MissingOptionValueError).WithContext(name)
					}

					value = args[currentArg]
					hasValue = true
				}
			}

			// Validate the argument type and store the appropriately typed value.
			switch location.OptionType {
			case KeywordType:
				found := false

				for _, validKeyword := range location.Keywords {
					if strings.EqualFold(value, validKeyword) {
						found = true
						location.Value = validKeyword
					}
				}

				if !found {
					return errors.New(errors.InvalidKeywordError).WithContext(value)
				}

			case BooleanType:
				location.Value = true

			case BooleanValueType:
				b, valid := ValidateBoolean(value)
				if !valid {
					return errors.New(errors.InvalidBooleanValueError).WithContext(value)
				}

				location.Value = b

			case StringType:
				location.Value = value

			case UUIDType:
				uuid, err := uuid.Parse(value)
				if err != nil {
					return err
				}

				location.Value = uuid.String()

			case StringListType:
				location.Value = MakeList(value)

			case IntType:
				i, err := strconv.Atoi(value)
				if err != nil {
					return errors.New(InvalidIntegerError, location.LongName, value)
				}

				location.Value = i
			}

			ui.Debug(ui.CLILogger, "Option value set to %#v", location.Value)

			// After parsing the option value, if there is an action routine, call it
			if location.Action != nil {
				err = location.Action(c)
				if err != nil {
					break
				}
			}
		}
	}

	// Whew! Everything parsed and in it's place. Before we wind up, let's verify that
	// all required options were in fact found.

	for _, entry := range c.Grammar {
		if entry.Required && !entry.Found {
			err = errors.New(RequiredNotFoundError, entry.LongName)

			break
		}
	}

	// If the parse went okay, let's check to make sure we don't have dangling
	// parameters, and then call the action if there is one.

	if err == nil {
		g := c.FindGlobal()

		if g.ExpectedParameterCount == -99 {
			ui.Debug(ui.CLILogger, "Parameters expected: <varying> found %d", g.GetParameterCount())
		} else {
			ui.Debug(ui.CLILogger, "Parameters expected: %d  found %d", g.ExpectedParameterCount, g.GetParameterCount())
		}

		if g.ExpectedParameterCount == 0 && len(g.Parameters) > 0 {
			return errors.New(UnexpectedParametersError)
		}

		if g.ExpectedParameterCount < 0 {
			if len(g.Parameters) > -g.ExpectedParameterCount {
				return errors.New(TooManyParametersError)
			}
		} else {
			if len(g.Parameters) != g.ExpectedParameterCount {
				return errors.New(WrongParameterCountError)
			}
		}

		// Search the tree to see if we have any environment variable settings that
		// should be pulled into the grammar.
		_ = c.ResolveEnvironmentVariables()

		// Did we ever find an action routine? If so, let's run it. Otherwise,
		// there wasn't enough command to determine what to do, so show the help.
		if c.Action != nil {
			ui.Debug(ui.CLILogger, "Invoking command action")

			err = c.Action(c)
		} else {
			ui.Debug(ui.CLILogger, "No command action was ever specified during parsing")
			ShowHelp(c)
		}
	}

	return err
}
