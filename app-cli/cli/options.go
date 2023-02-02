package cli

const (
	// StringType accepts a string (in quotes if it contains spaces or punctuation).
	StringType = 1

	// IntType accepts a signed integer value.
	IntType = 2

	// BooleanType is true if present, or false if not present.
	BooleanType = 3

	// BooleanValueType is representation of the boolean value (true/false).
	BooleanValueType = 4

	// Subcommand specifies that the LongName is a command name, and parsing continues with the SubGrammar.
	Subcommand = 6

	// StringListType is a string value or a list of string values, separated by commas and enclosed in quotes.
	StringListType = 7

	// ParameterType is a parameter from the command line. They should be declared in order.
	ParameterType = 8

	// UUIDType defines a value that must be a valid (parsable) UUID, though the value is stored
	// as a string datum.
	UUIDType = 9

	// KeywordType is a string that must be from an approved list of keyword values.
	KeywordType = 10
)

// Option defines the structure of each option that can be parsed.
type Option struct {
	// The full name of an option, not including the dash punctuation. For
	// example, the --trace option would have a name of "trace".
	LongName string

	// The short name, if specified, allows a short option name with only
	// a single dash and typically a single letter. For example, the --trace
	// option might be specified as -t, so the short name would be "t".
	ShortName string

	// This is a one-line description of what this option or subcommand is
	// used for. This is displayed when outputing the standard --help output.
	Description string

	// If there parameters are permitted other than subcommands and options,
	// the description of that parameter string is here. This is used by the
	// standard --help output.
	ParameterDescription string

	// If there is an environment variable that can provide the value of this
	// option, specify it here. Any option names that were not specified on
	// the command line, but have an environment variable, will get their value
	// by reading the environment as part of the parsing operation.
	EnvironmentVariable string

	// Aliases is a list of alternate spellings of the LongName value.  For example,
	// an option called --type could also be expresssed as --types or --typing. In
	// that case, the Aliases would be []string{"types", "typing"}. Only the LongName
	// is displayed in the standard --help output, but the parser accepts the aliases
	// while processing the command line.
	Aliases []string

	// If the option takes keyword values (that is, string tokens from a specific list
	// of keyword values), then Keywords is the array of all the allowed values.
	Keywords []string

	// If the option isn't a boolean option, then it's value is stored here. The value
	// is either expressed after LongName (or ShortName) followed by an "=" and the value,
	// or it is the next token on the command line after the option name. The type of
	// the option will control what is stored in the Value.
	Value interface{}

	// Action defines a function to call if this option or subcommand is specified on
	// the command line. For an option, this can call a function to set the value in
	// internal storage, etc. For a subcommand, this function executions the actual
	// subcommand.
	Action func(c *Context) error

	// OptionType describes what kind of option this is, or if it is a subcommand
	// instead. Boolean options are true (the option was specified) or false (the
	// option was not present). All other options have a data value that follows
	// the option name. The parser uses this option type information to validate
	// that the data provided on the command line is in the correct format.
	OptionType int

	// This indicates how many parameters are expected on the command line. If
	// the value is zero, then there are no parameters other than options and
	// subcommands. Specify -1 to allow a variable number of parameters.
	ParametersExpected int

	// Found indicates if the value was found on the command line. If true, then
	// a value was either provided on the command line or optionally located in
	// an environment variable. If false, this option has not been specified by
	// the command line invocation.
	Found bool

	// Required indicates if this option is required. That is, if true, then
	// the parser will report an error if this option's Found value is false
	// after parsing all the command line values.
	Required bool

	// Private indicates that this option or subcommand is not displayed in the
	// help output.
	Private bool

	// For options that are subcommands, this flag indicates that this is to be
	// considered the default subcommand verb. That is, if during parsing, no
	// subcommand was specified but one option is marked as the DefaultVerb,
	// then that verb is assumed to have been present on the command line, and
	// control transfers to the Action routine if specified..
	DefaultVerb bool
}

// Context is the information used by the Parser. It includes metadata
// about the program command or subcommand, the grammar definition, and
// information about what was found on the command line (parameters).
//
// There may be multiple contexts, so it includes a pointer to the "parent"
// context as well. For simple applications without a subcommand, the
// pointer will be nil. But for a command that has multiple subcommands
// such as "foobar config show", there is a global context that is first
// created when the App is run, but when the subcommand "config" is found,
// a new subcontext is created for the information found after that subcommand.
// This can be arbitrarily complex depending on how deeply nested the command
// line grammar is.
type Context struct {
	// A copy of the application name from the enclosing Application
	// context. It is here to help generate the default --help output
	// when a partial command line is followed by the --help option.
	AppName string

	// This is the name of the main program that is run by this application.
	// By default this is the same as the AppName, but if specified as a
	// different string, this is used in the --help output instead.
	MainProgram string

	// This is the description of the current context. For a simple
	// grammar with no subcommands, this is the same as the description
	// from the App object. For grammars with subcommands, this description
	// is unique to each subcommand, and is used when formatting --help
	// output following the command text.
	Description string

	// This is a copy of the copyright string from the App object, and is
	// used to formulate the --help output.
	Copyright string

	// This is a copy of the version string from the App object, and is
	// used to formulate the --help output.
	Version string

	// If specified, this is a text string that expresses how the command
	// syntax should be displayed for the invocation of the verb.
	Command string

	// If specified, this is added to the Command text when forming the
	// help output, to describe the expected parameters.
	ParameterDescription string

	// This is an array of command line option descriptions. Each one
	// represents information used by the parser to validate and store
	// option values, or direct parsing to a sub-grammar when a subcommand
	// is parsed.
	Grammar []Option

	// This is a copy of the command line argument strings originally
	// read from the operation system when the main program was run.
	Args []string

	// This is a list of the parameter values found (if any) during
	// parsing.
	Parameters []string

	// This is a pointer to the parent context (the command grammar
	// that was being parsed up to the point where this subcommand
	// was encountered). This will be nill for the top-level context.
	Parent *Context

	// This is the function that is to be run by default for this
	// context. That is, when all the command line grammar has been
	// parsed, unless control transferred to a subcommand context, then
	// this function represents the actual "work" of the application.
	Action func(c *Context) error

	// This indicates how many were expected for this level of the grammar,
	// by evaluating the grammar options to see how many parameters are
	// explicitly defined.
	Expected int
}
