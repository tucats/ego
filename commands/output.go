package commands

import (
	"encoding/json"
	"fmt"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/errors"
)

// commandOutput is used to output the results of a command to the user's
// console, respecting the current format of the output processor and
// whether the --quiet option was specified. It will work with text (it
// behaves identically to the ui.Say() operataor, including allowing a
// format string with substitution values) as well as JSON output in
// standard or indented formats.
func commandOutput(thing ...interface{}) error {
	switch ui.OutputFormat {
	case ui.TextFormat:
		var msg string

		if len(thing) == 1 {
			msg, _ = thing[0].(string)
		} else {
			formatString, _ := thing[0].(string)
			msg = fmt.Sprintf(formatString, thing[1:]...)
		}

		ui.Say(msg)

		return nil

	case ui.JSONFormat:
		if len(thing) > 1 {
			return errors.ErrArgumentCount
		}

		b, err := json.Marshal(thing[0])
		if err != nil {
			return errors.New(err)
		}

		ui.Say("%s", string(b))

		return nil

	case ui.JSONIndentedFormat:
		if len(thing) > 1 {
			return errors.ErrArgumentCount
		}

		b, err := json.MarshalIndent(thing[0], ui.JSONIndentPrefix, ui.JSONIndentSpacer)
		if err != nil {
			return errors.New(err)
		}

		ui.Say("%s", string(b))

		return nil

	default:
		return errors.ErrInvalidOutputFormat
	}
}
