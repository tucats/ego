package cli

import "strings"

// GetParameter returns the ith parameter string parsed, or an
// empty string if not found.
func (c *Context) GetParameter(i int) string {
	g := c.FindGlobal()
	if i < g.GetParameterCount() {
		return g.Parameters[i]
	}
	return ""
}

// GetParameterCount returns the number of parameters processed.
func (c *Context) GetParameterCount() int {
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

// GetInteger returns the value of a named integer from the
// parsed grammar, or a zero if not found. The boolean return
// value confirms if the value was specified on the command line.
func (c *Context) GetInteger(name string) (int, bool) {

	for _, entry := range c.Grammar {

		if entry.OptionType == Subcommand && entry.Found {
			subContext := entry.Value.(Context)
			return subContext.GetInteger(name)
		}
		if entry.Found && entry.OptionType == IntType && name == entry.LongName {
			return entry.Value.(int), true
		}
	}
	return 0, false
}

// GetBool returns the value of a named boolean. If the boolean option
// was found during processing, this routine returns true. Otherwise it
// returns false.
func (c *Context) GetBool(name string) bool {

	for _, entry := range c.Grammar {

		if entry.OptionType == Subcommand && entry.Found {
			subContext := entry.Value.(Context)
			return subContext.GetBool(name)
		}
		if entry.Found && (entry.OptionType == BooleanType || entry.OptionType == BooleanValueType) && name == entry.LongName {
			return entry.Value.(bool)
		}
	}
	return false
}

// GetString returns the value of a named string parameter from the
// parsed grammar, or an empty string if not found. The second return
// value indicates if the value was explicitly specified. This is used
// to differentiate between "not specified" and "specified as empty".
func (c *Context) GetString(name string) (string, bool) {

	for _, entry := range c.Grammar {

		if entry.OptionType == Subcommand && entry.Found {
			subContext := entry.Value.(Context)
			return subContext.GetString(name)
		}
		if entry.Found && (entry.OptionType == StringListType || entry.OptionType == UUIDType || entry.OptionType == StringType) && name == entry.LongName {

			if entry.OptionType == StringType || entry.OptionType == UUIDType {
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

// GetStringList returns the array of strings that are the value of
// the named item. If the item is not found, an empty array is returned.
// The second value in the result indicates of the option was explicitly
// specified in the command line.
func (c *Context) GetStringList(name string) ([]string, bool) {
	for _, entry := range c.Grammar {

		if entry.OptionType == Subcommand && entry.Found {
			subContext := entry.Value.(Context)
			return subContext.GetStringList(name)
		}
		if entry.Found && entry.OptionType == StringListType && name == entry.LongName {
			return entry.Value.([]string), true
		}
	}
	return make([]string, 0), false
}
