package time

import (
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/language/symbols"
	"github.com/tucats/ego/internal/util"
)

// Parse an arbitrary string value into a native Go datetime value. Uses the dateparse
// package which first scans the string to determine the appropriate Go date format string,
// and then uses that string to do the conversion.
//
// This implements the Ego time.ParseAny() function.
//
// The real work -- including deciding what a bare timezone abbreviation such
// as "EST" means -- lives in util.ParseTimestamp(), which the database table
// layer also uses so the two cannot disagree. See docs/CONFIG.md under
// "Timezones and time.ParseAny()" for what the abbreviation rules are and why
// the ego.runtime.timezone setting exists.
//
// This is the *lenient* of the two parse functions: an abbreviation the
// configured reference zone does not recognize keeps its name and takes a zero
// offset rather than being rejected, which is what this function has always
// returned for an unresolvable abbreviation. util.StrictParseTimestamp() is
// the counterpart used where the value will be stored.
func Parse(s *symbols.SymbolTable, args data.List) (any, error) {
	value := data.String(args.Get(0))

	t, err := util.ParseTimestamp(value)
	if err != nil {
		// errors.New() recognizes an error that is already an Ego error and
		// clones it rather than double-wrapping, so this adds the "ParseAny"
		// function context whether the failure came from the underlying parse
		// or from an unloadable ego.runtime.timezone setting.
		err = errors.New(err).In("ParseAny")

		return data.NewList(nil, err), err
	}

	return data.NewList(t, nil), nil
}
