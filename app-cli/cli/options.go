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
	ShortName            string
	LongName             string
	Aliases              []string
	Keywords             []string
	Description          string
	OptionType           int
	ParametersExpected   int
	ParameterDescription string
	EnvironmentVariable  string
	Found                bool
	Required             bool
	Private              bool
	SubGrammar           []Option
	Value                interface{}
	Action               func(c *Context) error
}

// Context is a simple array of Option types, and is used to express
// a grammar.
type Context struct {
	AppName                string
	MainProgram            string
	Description            string
	Copyright              string
	Version                string
	Command                string
	Grammar                []Option
	Args                   []string
	Parent                 *Context
	Parameters             []string
	ParameterCount         int
	ExpectedParameterCount int
	ParameterDescription   string
	Action                 func(c *Context) error
}
