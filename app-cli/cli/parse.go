package cli

import (
	"path/filepath"
	"runtime"
	"strings"

	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/egostrings"
	"github.com/tucats/ego/errors"
)

// When used in a parameter count field, this value indicates that
// an unknown but variable number of arguments can be presented.
const Variable = -1

type parseState struct {
	args           []string
	defaultVerb    *Option
	currentArg     int
	lastArg        int
	parsedSoFar    int
	parametersOnly bool
	helpVerb       bool
	done           bool
}

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

		return errors.ErrExit
	}

	// Start parsing using the top-level grammar.
	return c.parseGrammar(args[1:])
}

// ParseGrammar accepts an argument list and parses it using the current context grammar
// definition. This is abstracted from Parse because it allows for recursion for subcomamnds.
// This is never called by the user directly.
func (c *Context) parseGrammar(args []string) error {
	var (
		err error
	)

	state := &parseState{
		args:     args,
		helpVerb: true,
		lastArg:  len(args),
	}

	// Are there parameters already stored away in the global? If so,
	// they are unrecognized verbs that were hoovered up by the grammar
	// parent processing, and are not parameters (which must always
	// follow the grammar verbs and options).
	parmList := c.FindGlobal().Parameters
	if len(parmList) > 0 {
		list := strings.Join(parmList, ", ")

		ui.Log(ui.CLILogger, "cli.unexp.parm.parsed", "list", list)

		return errors.ErrUnrecognizedCommand.Context(parmList[0])
	}

	// No dangling parameters, let's keep going.  See if we have a default verb we should know about.
	state.defaultVerb = findDefaultVerb(c)

	// Scan over the tokens, parsing until we hit a subcommand. If we get to the end of the tokens,
	// we are done parsing. If there is an error we are done parsing.
	for state.currentArg = 0; state.currentArg < state.lastArg; state.currentArg++ {
		err = parseToken(c, state)
		if err != nil || state.done {
			return err
		}
	}

	// No subcommand found, but was there a default we should use anyway?
	if err == nil && state.defaultVerb != nil {
		return doDefaultSubcommand(state.parsedSoFar, c, state.defaultVerb, args)
	}

	// Whew! Everything parsed and in it's place. Before we wind up, let's verify that
	// all required options were in fact found.
	if err == nil {
		err = verifyRequiredOptionsPresent(c)
	}

	// If the parse went okay, let's check to make sure we don't have dangling
	// parameters, and then call the action if there is one.
	if err == nil {
		// Search the tree to see if we have any environment variable settings that
		// should be pulled into the grammar.
		// Did we ever find an action routine? If so, let's run it. Otherwise,
		// there wasn't enough command to determine what to do, so show the help.
		err = invokeAction(c)
	}

	return err
}

// Scan the grammar to confirm that all required options were found during the
// parse. If any required options were not found, an error is returned.
func verifyRequiredOptionsPresent(c *Context) error {
	for _, entry := range c.Grammar {
		if entry.Required && !entry.Found {
			return errors.ErrRequiredNotFound.Context(entry.LongName)
		}
	}

	return nil
}

func parseToken(c *Context, state *parseState) error {
	var (
		location *Option
	)

	option := state.args[state.currentArg]
	state.parsedSoFar = state.currentArg

	ui.Log(ui.CLILogger, "cli.token", "token", option)

	// Are we now only eating parameter values?
	if state.parametersOnly {
		globalContext := c.FindGlobal()
		globalContext.Parameters = append(globalContext.Parameters, option)
		count := len(globalContext.Parameters)

		ui.Log(ui.CLILogger, "cli.parm", "count", count)

		return nil
	}

	// Handle the special cases automatically.
	if (state.helpVerb && option == "help") || option == "-h" || option == "--help" {
		ShowHelp(c)

		return errors.ErrExit
	}

	// Handle the "empty option" that means the remainder of the command
	// line will be treated as parameters, even if it looks like it has
	// options, etc.
	if option == "--" {
		state.parametersOnly = true
		state.helpVerb = false

		ui.Log(ui.CLILogger, "cli.remaining.parms")

		return nil
	}

	// Convert the option name to a proper name, and remember if it was a short versus long form.
	// Also, if there was an embedded value in the name, return that as well.
	name, isShort, value, hasValue := parseOptionName(option)

	location = nil

	if name > "" {
		location = findShortName(c, isShort, name, location)
	}

	if location != nil {
		ui.Log(ui.CLILogger, "cli.set.name", "name", location.LongName)
	}

	// If it was an option (short or long) and not found, this is an error.
	if name != "" && location == nil && state.defaultVerb == nil {
		return errors.ErrUnknownOption.Context(option)
	}

	// It could be a parameter, or a subcommand.
	if location == nil {
		// Is it a subcommand?
		if done, err := evaluatePossibleSubcommand(c, option, state.args, state.currentArg, state.defaultVerb, state.parsedSoFar); done {
			state.done = true

			return err
		}
	} else {
		location.Found = true
		// If it's not a boolean type, see it already has a value from the = construct.
		// If not, claim the next argument as the value.
		if location.OptionType != BooleanType {
			if !hasValue {
				state.currentArg = state.currentArg + 1
				if state.currentArg >= state.lastArg {
					return errors.ErrMissingOptionValue.Context(name)
				}

				value = state.args[state.currentArg]
				hasValue = true
			}
		}

		// Validate the argument type and store the appropriately typed value.
		// if it has a value, use that to set the boolean (so it
		// behaves like a BooleanValueType). If no value was parsed,
		// just assume it is true.
		if err := validateOption(location, value, hasValue); err != nil {
			return err
		}

		ui.Log(ui.CLILogger, "cli.set.value", location.Value)

		// After parsing the option value, if there is an action routine, call it
		if location.Action != nil {
			if err := location.Action(c); err != nil {
				return err
			}
		}
	}

	return nil
}

// For an option token, extract the name and note if it was the short form or long form.
// Also, if the option has an value appended after an "=" sign, return the value and
// a flag indicated it was present.
func parseOptionName(option string) (string, bool, string, bool) {
	var (
		isShort bool
		name    string
	)

	if len(option) > 2 && option[:2] == "--" {
		name = option[2:]
	} else if len(option) > 1 && option[:1] == "-" {
		name = option[1:]
		isShort = true
	}

	value := ""
	hasValue := false

	if equals := strings.Index(name, "="); equals >= 0 {
		value = name[equals+1:]
		name = name[:equals]
		hasValue = true
	}

	return name, isShort, value, hasValue
}

// Search the grammar to see if there is a default verb. If no default verb is specified,
// the function returns nil.
func findDefaultVerb(c *Context) *Option {
	var defaultVerb *Option

	for index, entry := range c.Grammar {
		if entry.DefaultVerb {
			defaultVerb = &c.Grammar[index]

			ui.Log(ui.CLILogger, "cli.set.default", "verb", defaultVerb.LongName)
		}
	}

	return defaultVerb
}

// Invoke the action specified in the context. This includes validating that the (unprocessed)
// parameters assocated with the invocation are valid for the action context.
func invokeAction(c *Context) error {
	var err error

	g := c.FindGlobal()

	if g.Expected == -99 {
		if g.MinParams > 0 {
			ui.Log(ui.CLILogger, "cli.parm.expect.min", "min", g.MinParams, "count", g.ParameterCount())
		} else {
			ui.Log(ui.CLILogger, "cli.parm.expect.var", "count", g.ParameterCount())
		}
	} else {
		ui.Log(ui.CLILogger, "cli.parm.expect.num", "want", g.Expected, "count", g.ParameterCount())
	}

	if g.Expected == 0 && len(g.Parameters) > 0 {
		return errors.ErrUnexpectedParameters
	}

	if g.MinParams > 0 && len(g.Parameters) < g.MinParams {
		return errors.ErrWrongParameterCount
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

	if err = c.ResolveEnvironmentVariables(); err != nil {
		return err
	}

	if c.Action != nil {
		ui.Log(ui.CLILogger, "cli.invoke")

		err = c.Action(c)
	} else {
		ui.Log(ui.CLILogger, "cli.no.action")
		ShowHelp(c)

		return errors.ErrExit
	}

	return err
}

// For a given option location in the grammar, verify the given value to determine
// if it matches the rquired type of the option. If not, return an error.
func validateOption(location *Option, value string, hasValue bool) error {
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
			return errors.New(err)
		}

		location.Value = uuid.String()

	case StringListType:
		location.Value = makeList(value)

	case IntType:
		if i, err := egostrings.Atoi(value); err != nil {
			return errors.ErrInvalidInteger.Context(value)
		} else {
			location.Value = i
		}
	}

	unsupported := false

	for _, platform := range location.Unsupported {
		ui.Log(ui.CLILogger, "cli.platform.check", "platform", platform)

		if runtime.GOOS == platform {
			unsupported = true

			ui.Log(ui.CLILogger, "cli.platform.unsupported", "platform", platform)

			break
		}
	}

	if unsupported {
		return errors.ErrUnsupportedOnOS.Context("--" + location.LongName)
	}

	return nil
}

func evaluatePossibleSubcommand(c *Context, option string, args []string, currentArg int, defaultVerb *Option, parsedSoFar int) (bool, error) {
	wasSubcommand, err := doIfSubcommand(c, option, args, currentArg)
	if wasSubcommand {
		return true, err
	}

	if defaultVerb != nil {
		ui.Log(ui.CLILogger, "cli.default.verb", "verb", defaultVerb.LongName)

		return true, doSubcommand(c, defaultVerb, args, parsedSoFar-1)
	}

	g := c.FindGlobal()
	g.Parameters = append(g.Parameters, option)
	count := len(g.Parameters)

	ui.Log(ui.CLILogger, "cli.unclaimed", "count", count)

	return false, nil
}

func doIfSubcommand(c *Context, option string, args []string, currentArg int) (bool, error) {
	for _, entry := range c.Grammar {
		isAlias := false

		for _, n := range entry.Aliases {
			if option == n {
				isAlias = true

				break
			}
		}

		if (isAlias || entry.LongName == option) && entry.OptionType == Subcommand {
			for _, platform := range entry.Unsupported {
				ui.Log(ui.CLILogger, "cli.platform.subcommand", "platform", platform)

				if runtime.GOOS == platform {
					return true, errors.ErrUnsupportedOnOS.Context(entry.LongName)
				}
			}

			return true, doSubcommand(c, &entry, args, currentArg)
		}
	}

	return false, nil
}

func findShortName(c *Context, isShort bool, name string, location *Option) *Option {
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

	return location
}

func doDefaultSubcommand(parsedSoFar int, c *Context, defaultVerb *Option, args []string) error {
	parsedSoFar = parsedSoFar - c.ParameterCount() + 1

	ui.Log(ui.CLILogger, "cli.default.verb", "verb", defaultVerb.LongName)

	if parsedSoFar < len(args) {
		ui.Log(ui.CLILogger, "cli.default.action.args", "args", args[parsedSoFar+1:])
	}

	if parsedSoFar > len(args) {
		parsedSoFar = len(args)
	}

	return doSubcommand(c, defaultVerb, args[parsedSoFar:], 0)
}

// doSubCommand executes a subcommand. Create a new context that defines the
// next level down. It should include the current context information,
// and an updated grammar tree, command text, and description adapted
// for this subcommand.
func doSubcommand(c *Context, entry *Option, args []string, currentArg int) error {
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

	globals := c.FindGlobal()
	globals.Expected = entry.ExpectedParms
	globals.MinParams = entry.MinParams
	globals.ParameterDescription = entry.ParmDesc

	if entry.Action != nil {
		subContext.Action = entry.Action

		ui.Log(ui.CLILogger, "cli.saving.action")
	}

	ui.Log(ui.CLILogger, "cli.subgrammar", "verb", entry.LongName)

	if len(args) == 0 {
		return subContext.parseGrammar([]string{})
	}

	tokens := args[currentArg+1:]
	ui.Log(ui.CLILogger, "cli.tokens", "tokens", tokens)

	return subContext.parseGrammar(tokens)
}
