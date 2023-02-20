package cli

import (
	"strings"

	"github.com/tucats/ego/errors"
)

// Parameter returns the ith parameter string parsed, or an
// empty string if not found.
func (c *Context) Parameter(index int) string {
	g := c.FindGlobal()
	if index < g.ParameterCount() {
		return g.Parameters[index]
	}

	return ""
}

// ParameterCount returns the number of parameters processed.
func (c *Context) ParameterCount() int {
	return len(c.FindGlobal().Parameters)
}

// WasFound reports if an entry in the grammar was found on
// the processed command line.
func (c *Context) WasFound(name string) bool {
	for _, entry := range c.Grammar {
		if entry.OptionType == Subcommand && entry.Found {
			subgrammar := entry.Value.(Context)

			return subgrammar.WasFound(name)
		}

		if entry.Found && name == entry.LongName {
			return true
		}
	}

	return false
}

// Set will set a value in the grammar as if it was entered in
// the command line. IF the option name does not exist in the
// current grammar tree, an error is returned.
func (c *Context) Set(name string, value interface{}) error {
	for index, option := range c.Grammar {
		if option.LongName == name {
			c.Grammar[index].Value = value
			c.Grammar[index].Found = true

			return nil
		}
	}

	if c.Parent != nil {
		return c.Parent.Set(name, value)
	}

	return errors.ErrUnknownOption.Context(name)
}

// Integer returns the value of a named integer from the
// parsed grammar, or a zero if not found. The boolean return
// value confirms if the value was specified on the command line.
func (c *Context) Integer(name string) (int, bool) {
	for _, entry := range c.Grammar {
		if entry.OptionType == Subcommand && entry.Found {
			subContext := entry.Value.(Context)

			return subContext.Integer(name)
		}

		if entry.Found && entry.OptionType == IntType && name == entry.LongName {
			return entry.Value.(int), true
		}
	}

	return 0, false
}

// Boolean returns the value of a named boolean. If the boolean option
// was found during processing, this routine returns true. Otherwise it
// returns false.
func (c *Context) Boolean(name string) bool {
	for _, entry := range c.Grammar {
		if entry.OptionType == Subcommand && entry.Found {
			subContext := entry.Value.(Context)

			return subContext.Boolean(name)
		}

		if entry.Found && (entry.OptionType == BooleanType || entry.OptionType == BooleanValueType) && name == entry.LongName {
			return entry.Value.(bool)
		}
	}

	return false
}

// String returns the value of a named string parameter from the
// parsed grammar, or an empty string if not found. The second return
// value indicates if the value was explicitly specified. This is used
// to differentiate between "not specified" and "specified as empty".
func (c *Context) String(name string) (string, bool) {
	for _, entry := range c.Grammar {
		if entry.OptionType == Subcommand && entry.Found {
			subContext := entry.Value.(Context)

			return subContext.String(name)
		}

		if entry.Found && (entry.OptionType == StringListType || entry.OptionType == KeywordType || entry.OptionType == UUIDType || entry.OptionType == StringType) && name == entry.LongName {
			if entry.OptionType == StringType || entry.OptionType == KeywordType || entry.OptionType == UUIDType {
				return entry.Value.(string), true
			}

			var b strings.Builder

			var v = entry.Value.([]string)

			for i, n := range v {
				if i > 0 {
					b.WriteRune(',')
				}

				b.WriteString(n)
			}

			return b.String(), true
		}
	}

	return "", false
}

// Keyword returns the value of a named string parameter from the
// parsed grammar. The result is the ordinal position (zero-based)
// on the keyword from the list. If the value is not in the list,
// it returns -1.
func (c *Context) Keyword(name string) (int, bool) {
	for _, entry := range c.Grammar {
		if entry.OptionType == Subcommand && entry.Found {
			subContext := entry.Value.(Context)

			return subContext.Keyword(name)
		}

		if entry.Found && (entry.OptionType == KeywordType) && name == entry.LongName {
			if value, ok := entry.Value.(string); ok {
				for n, k := range entry.Keywords {
					if strings.EqualFold(value, k) {
						return n, true
					}
				}

				return -1, false
			}
		}
	}

	return 0, false
}

// StringList returns the array of strings that are the value of
// the named item. If the item is not found, an empty array is returned.
// The second value in the result indicates of the option was explicitly
// specified in the command line.
func (c *Context) StringList(name string) ([]string, bool) {
	for _, entry := range c.Grammar {
		if entry.OptionType == Subcommand && entry.Found {
			subContext := entry.Value.(Context)

			return subContext.StringList(name)
		}

		if entry.Found && entry.OptionType == StringListType && name == entry.LongName {
			return entry.Value.([]string), true
		}
	}

	return make([]string, 0), false
}
