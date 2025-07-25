package cli

import (
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/egostrings"
	"github.com/tucats/ego/errors"
)

// When used in a parameter count field, this value indicates that
// an unknown but variable number of arguments can be presented.
const Variable = -1

type parseState struct {
	args           []string
	defaultVerb    *Option
	parent         []Option
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
// definition. This is abstracted from Parse because it allows for recursion for sub-commands.
// This is never called by the user directly.
func (c *Context) parseGrammar(args []string) error {
	var (
		err error
	)

	state := &parseState{
		args:     args,
		helpVerb: true,
		lastArg:  len(args),
		parent:   c.Grammar,
	}

	// Are there parameters already stored away in the global? If so,
	// they are unrecognized verbs that were hoovered up by the grammar
	// parent processing, and are not parameters (which must always
	// follow the grammar verbs and options).
	parmList := c.FindGlobal().Parameters
	if len(parmList) > 0 {
		list := strings.Join(parmList, ", ")

		ui.Log(ui.CLILogger, "cli.unexp.parm.parsed", ui.A{
			"list": list})

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

	ui.Log(ui.CLILogger, "cli.token", ui.A{
		"token": option})

	// Are we now only eating parameter values?
	if state.parametersOnly {
		globalContext := c.FindGlobal()
		globalContext.Parameters = append(globalContext.Parameters, option)
		count := len(globalContext.Parameters)

		ui.Log(ui.CLILogger, "cli.parm", ui.A{
			"count": count})

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

		ui.Log(ui.CLILogger, "cli.remaining.parms", nil)

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
		ui.Log(ui.CLILogger, "cli.set.name", ui.A{
			"name": location.LongName})
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

		// Does this option preclude any other options that might already be set?
		for _, name := range location.Excludes {
			for _, entry := range c.Grammar {
				if entry.LongName == name && entry.Found {
					return errors.ErrOptionConflict.Clone().Context("--" + name + ", --" + location.LongName)
				}
			}
		}

		ui.Log(ui.CLILogger, "cli.set.value", ui.A{
			"value": location.Value})

		// After parsing the option value, if there is an action routine, call it
		if location.Action != nil {
			if err := location.Action(c); !errors.Nil(err) {
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

			ui.Log(ui.CLILogger, "cli.set.default", ui.A{
				"verb": defaultVerb.LongName})
		}
	}

	return defaultVerb
}

// Invoke the action specified in the context. This includes validating that the (unprocessed)
// parameters associated with the invocation are valid for the action context.
func invokeAction(c *Context) error {
	var err error

	g := c.FindGlobal()

	if g.Expected == defs.VariableParameterCount {
		if g.MinParams > 0 {
			ui.Log(ui.CLILogger, "cli.parm.expect.min", ui.A{
				"min":   g.MinParams,
				"count": g.ParameterCount()})
		} else {
			ui.Log(ui.CLILogger, "cli.parm.expect.var", ui.A{
				"count": g.ParameterCount()})
		}
	} else {
		ui.Log(ui.CLILogger, "cli.parm.expect.num", ui.A{
			"want":  g.MinParams,
			"count": g.ParameterCount()})
	}

	// Prompt for any missing parameters if a prompt has been provided.
	if len(g.Prompts) > 0 {
		for i := 0; i < g.MinParams; i++ {
			if i >= len(g.Parameters) && i <= len(g.Prompts) && g.Prompts[i] != "" {
				value := ""
				for value == "" {
					value = ui.Prompt(g.Prompts[i] + " ")

					g.Parameters = append(g.Parameters, value)
				}
			}
		}
	}

	if g.Expected == 0 && len(g.Parameters) > 0 {
		expected := []string{}

		for _, param := range c.Grammar {
			if param.OptionType == Subcommand && !param.Private {
				expected = append(expected, param.LongName)
			}
		}

		if len(expected) > 0 {
			return errors.ErrUnexpectedParameters.
				Clone().
				Context(g.Parameters[0]).
				Chain(errors.ErrExpectedSubcommand.
					Clone().
					Context(strings.Join(expected, ", ")))
		}

		return errors.ErrUnexpectedParameters.Clone().Context(g.Parameters[0])
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
		ui.Log(ui.CLILogger, "cli.invoke", nil)

		err = c.Action(c)
	} else {
		ui.Log(ui.CLILogger, "cli.no.action", nil)
		ShowHelp(c)

		return errors.ErrExit
	}

	return err
}

// For a given option location in the grammar, verify the given value to determine
// if it matches the required type of the option. If not, return an error.
func validateOption(location *Option, value string, hasValue bool) error {
	switch location.OptionType {
	case RangeType:
		if hasValue {
			list, err := parseSequence(value)
			if err != nil {
				return errors.ErrInvalidRange.Context(value)
			}

			text := ""
			for _, item := range list {
				if text != "" {
					text += ","
				}

				text += strconv.Itoa(item)
			}

			location.Found = true
			location.Value = text
		}

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
		ui.Log(ui.CLILogger, "cli.platform.check", ui.A{
			"platform": platform})

		if runtime.GOOS == platform {
			unsupported = true

			ui.Log(ui.CLILogger, "cli.platform.unsupported", ui.A{
				"platform": platform})

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
		ui.Log(ui.CLILogger, "cli.default.verb", ui.A{
			"verb": defaultVerb.LongName})

		return true, doSubcommand(c, defaultVerb, args, parsedSoFar-1)
	}

	g := c.FindGlobal()
	g.Parameters = append(g.Parameters, option)
	count := len(g.Parameters)

	ui.Log(ui.CLILogger, "cli.unclaimed", ui.A{
		"count": count})

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
				ui.Log(ui.CLILogger, "cli.platform.subcommand", ui.A{
					"platform": platform})

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

func parseSequence(s string) ([]int, error) {
	var err error

	result := make([]int, 0)

	// Step 1, normalize "-" and ":" characters.
	s = strings.ReplaceAll(s, "-", ":")

	// Step 2, determine sets of values or ranges.
	parts := strings.Split(s, ",")

	for _, part := range parts {
		part = strings.TrimSpace(part)

		if part == "" {
			continue
		}

		if strings.Contains(part, ":") {
			// This is a range.
			start, end, err := parseRange(part)
			if err != nil {
				return nil, err
			}

			for i := start; i <= end; i++ {
				result = append(result, i)
			}
		} else {
			// This is a single value.
			i, err := strconv.Atoi(part)
			if err != nil {
				return nil, errors.ErrInvalidInteger.Clone().Context(part)
			}

			result = append(result, i)
		}
	}

	return result, err
}

func parseRange(s string) (int, int, error) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return 0, 0, errors.ErrInvalidRange.Context(s)
	}

	if len(strings.TrimSpace(parts[0])) == 0 {
		parts[0] = "1"
	}

	start, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, errors.ErrInvalidInteger.Clone().Context(parts[0])
	}

	if len(strings.TrimSpace(parts[1])) == 0 {
		parts[1] = strconv.FormatInt(int64(start+9), 10)
	}

	end, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, errors.ErrInvalidInteger.Clone().Context(parts[1])
	}

	if start > end {
		return 0, 0, errors.ErrInvalidRange.Clone().Context(s)
	}

	return start, end, nil
}

func doDefaultSubcommand(parsedSoFar int, c *Context, defaultVerb *Option, args []string) error {
	parsedSoFar = parsedSoFar - c.ParameterCount() + 1

	ui.Log(ui.CLILogger, "cli.default.verb", ui.A{
		"verb": defaultVerb.LongName})

	if parsedSoFar < len(args) {
		ui.Log(ui.CLILogger, "cli.default.action.args", ui.A{
			"args": args[parsedSoFar+1:]})
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

	// Zero out any action that was set by default, since the sub-grammar now
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
	globals.Prompts = entry.Prompts
	globals.MinParams = entry.MinParams
	globals.ParameterDescription = entry.ParmDesc

	if entry.Action != nil {
		subContext.Action = entry.Action

		ui.Log(ui.CLILogger, "cli.saving.action", nil)
	}

	ui.Log(ui.CLILogger, "cli.subgrammar", ui.A{
		"verb": entry.LongName})

	if len(args) == 0 {
		return subContext.parseGrammar([]string{})
	}

	tokens := args[currentArg+1:]
	ui.Log(ui.CLILogger, "cli.tokens", ui.A{
		"tokens": tokens})

	return subContext.parseGrammar(tokens)
}
