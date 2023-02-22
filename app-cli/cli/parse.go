package cli

import (
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/errors"
)

// When used in a parameter count field, this value indicates that
// an unknown but variable number of arguments can be presented.
const Variable = -1

// Parse processes the grammar associated with the current context,
// using the []string array of arguments for that context.
//
// Unrecognized options or subcommands, as well as invalid values
// are reported as an error. If there is an action routine associated
// with an option or a subcommand, that action is executed.
func (c *Context) Parse() error {
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
func (c *Context) parseGrammar(args []string) error {
	var err error

	// Are there parameters already stored away in the global? If so,
	// they are unrecognized verbs that were hoovered up by the grammar
	// parent processing, and are not parameters (which must always
	// follow the grammar verbs and options).
	parmList := c.FindGlobal().Parameters
	if len(parmList) > 0 {
		list := strings.Builder{}

		plural := "s"
		if len(parmList) == 1 {
			plural = ""
		}

		for index, parm := range parmList {
			if index > 0 {
				list.WriteString(", ")
			}

			list.WriteString(parm)
		}

		ui.Log(ui.CLILogger, "Unexpected parameter%s already parsed: %s", plural, list.String())

		return errors.ErrUnrecognizedCommand.Context(parmList[0])
	}

	// No dangling parameters, let's keep going.
	lastArg := len(args)
	parsedSoFar := 0
	parametersOnly := false
	helpVerb := true

	// See if we have a default verb we should know about.
	var defaultVerb *Option

	for index, entry := range c.Grammar {
		if entry.DefaultVerb {
			defaultVerb = &c.Grammar[index]

			ui.Log(ui.CLILogger, "Registering default verb %s", defaultVerb.LongName)
		}
	}

	// Scan over the tokens, parsing until we hit a subcommand.
	for currentArg := 0; currentArg < lastArg; currentArg++ {
		var location *Option

		var name string

		var value string

		option := args[currentArg]
		isShort := false
		parsedSoFar = currentArg

		ui.Log(ui.CLILogger, "Processing token: %s", option)

		// Are we now only eating parameter values?
		if parametersOnly {
			globalContext := c.FindGlobal()
			globalContext.Parameters = append(globalContext.Parameters, option)
			count := len(globalContext.Parameters)

			ui.Log(ui.CLILogger, "added parameter %d", count)

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

		if location != nil {
			ui.Log(ui.CLILogger, "Setting value for option %s", location.LongName)
		}

		// If it was an option (short or long) and not found, this is an error.
		if name != "" && location == nil && defaultVerb == nil {
			return errors.ErrUnknownOption.Context(option)
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
					unsupported := false

					for _, platform := range entry.Unsupported {
						if runtime.GOOS == platform {
							unsupported = true

							break
						}
					}

					if unsupported {
						return errors.ErrUnsupportedOnOS.Context(entry.LongName)
					}

					return doSubcommand(c, entry, args, currentArg)
				}
			}

			// No subcommand found, but was there a default we should use anyway?
			if defaultVerb != nil {
				ui.Log(ui.CLILogger, "Using default verb %s", defaultVerb.LongName)

				return doSubcommand(c, *defaultVerb, args, parsedSoFar-1)
			}

			// Not a subcommand, just save it as an unclaimed parameter
			g := c.FindGlobal()
			g.Parameters = append(g.Parameters, option)
			count := len(g.Parameters)

			ui.Log(ui.CLILogger, "Unclaimed token added parameter %d", count)
		} else {
			location.Found = true
			// If it's not a boolean type, see it already has a value from the = construct.
			// If not, claim the next argument as the value.
			if location.OptionType != BooleanType {
				if !hasValue {
					currentArg = currentArg + 1
					if currentArg >= lastArg {
						return errors.ErrMissingOptionValue.Context(name)
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
					return errors.ErrInvalidKeyword.Context(value)
				}

			case BooleanType:
				// if it has a value, use that to set the boolean (so it
				// behaves like a BooleanValueType). If no value was parsed,
				// just assume it is true.
				if hasValue {
					if b, valid := validateBoolean(value); !valid {
						return errors.ErrInvalidBooleanValue.Context(value)
					} else {
						location.Value = b
					}
				} else {
					location.Value = true
				}

			case BooleanValueType:
				b, valid := validateBoolean(value)
				if !valid {
					return errors.ErrInvalidBooleanValue.Context(value)
				}

				location.Value = b

			case StringType:
				location.Value = value

			case UUIDType:
				uuid, err := uuid.Parse(value)
				if err != nil {
					return errors.NewError(err)
				}

				location.Value = uuid.String()

			case StringListType:
				location.Value = makeList(value)

			case IntType:
				i, err := strconv.Atoi(value)
				if err != nil {
					return errors.ErrInvalidInteger.Context(value)
				}

				location.Value = i
			}

			unsupported := false
			for _, platform := range location.Unsupported {
				if runtime.GOOS == platform {
					unsupported = true
					ui.Log(ui.CLILogger, "Option value unsupported on platform %s", platform)

					break
				}
			}

			if unsupported {
				return errors.ErrUnsupportedOnOS.Context("--" + location.LongName)
			}

			ui.Log(ui.CLILogger, "Option value set to %#v", location.Value)

			// After parsing the option value, if there is an action routine, call it
			if location.Action != nil {
				err = location.Action(c)
				if err != nil {
					break
				}
			}
		}
	}

	// No subcommand found, but was there a default we should use anyway?
	if defaultVerb != nil {
		parsedSoFar = parsedSoFar - c.ParameterCount() + 1

		ui.Log(ui.CLILogger, "Using default verb %s", defaultVerb.LongName)

		if parsedSoFar < len(args) {
			ui.Log(ui.CLILogger, "Passing remaining arguments to default action: %v",
				args[parsedSoFar+1:])
		}

		if parsedSoFar > len(args) {
			parsedSoFar = len(args)
		}

		return doSubcommand(c, *defaultVerb, args[parsedSoFar:], 0)
	}

	// Whew! Everything parsed and in it's place. Before we wind up, let's verify that
	// all required options were in fact found.

	for _, entry := range c.Grammar {
		if entry.Required && !entry.Found {
			err = errors.ErrRequiredNotFound.Context(entry.LongName)

			break
		}
	}

	// If the parse went okay, let's check to make sure we don't have dangling
	// parameters, and then call the action if there is one.

	if err == nil {
		g := c.FindGlobal()

		if g.Expected == -99 {
			ui.Log(ui.CLILogger, "Parameters expected: <varying> found %d", g.ParameterCount())
		} else {
			ui.Log(ui.CLILogger, "Parameters expected: %d  found %d", g.Expected, g.ParameterCount())
		}

		if g.Expected == 0 && len(g.Parameters) > 0 {
			return errors.ErrUnexpectedParameters
		}

		if g.Expected < 0 {
			if len(g.Parameters) > -g.Expected {
				return errors.ErrTooManyParameters
			}
		} else {
			if len(g.Parameters) != g.Expected {
				return errors.ErrWrongParameterCount
			}
		}

		// Search the tree to see if we have any environment variable settings that
		// should be pulled into the grammar.
		_ = c.ResolveEnvironmentVariables()

		// Did we ever find an action routine? If so, let's run it. Otherwise,
		// there wasn't enough command to determine what to do, so show the help.
		if c.Action != nil {
			ui.Log(ui.CLILogger, "Invoking command action")

			err = c.Action(c)
		} else {
			ui.Log(ui.CLILogger, "No command action was ever specified during parsing")
			ShowHelp(c)
		}
	}

	return err
}

func doSubcommand(c *Context, entry Option, args []string, currentArg int) error {
	// We're doing a subcommand! Create a new context that defines the
	// next level down. It should include the current context information,
	// and an updated grammar tree, command text, and description adapted
	// for this subcommand.
	subContext := *c
	subContext.Parent = c

	// Zero out any action that was set by default, since the subgrammar now
	// controls the action to be used.
	c.Action = nil
	subContext.Action = nil

	if entry.Value != nil {
		subContext.Grammar = entry.Value.([]Option)
	} else {
		subContext.Grammar = []Option{}
	}

	subContext.Command = c.Command + entry.LongName + " "
	subContext.Description = entry.Description
	entry.Found = true
	c.FindGlobal().Expected = entry.ExpectedParms
	c.FindGlobal().ParameterDescription = entry.ParmDesc

	if entry.Action != nil {
		subContext.Action = entry.Action

		ui.Log(ui.CLILogger, "Saving action routine in subcommand context")
	}

	ui.Log(ui.CLILogger, "Transferring control to subgrammar for %s", entry.LongName)

	if len(args) == 0 {
		return subContext.parseGrammar([]string{})
	}

	tokens := args[currentArg+1:]
	ui.Log(ui.CLILogger, "Remaining tokens %v", tokens)

	return subContext.parseGrammar(tokens)
}
