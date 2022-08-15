package functions

import (
	"bytes"
	"os/exec"
	"strings"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// Exec implements the util.Exec() function. This should be replaced with a proper
// emulation of the Go command object type in the future.
func Exec(s *symbols.SymbolTable, args []interface{}) (result interface{}, err *errors.EgoError) {
	// Is this function authorized?
	if !settings.GetBool(defs.ExecPermittedSetting) {
		return nil, errors.New(errors.ErrNoPrivilegeForOperation).Context("Exec")
	}

	// Get the arguments as a string array
	argStrings := make([]string, 0)

	for _, arg := range args {
		s := datatypes.GetString(arg)
		argStrings = append(argStrings, s)
	}

	// User may have packed evertying into a single argument. If so,
	// unpack that now.
	if len(argStrings) == 1 {
		argStrings = strings.Split(argStrings[0], " ")
	}

	cmd := exec.Command(argStrings[0], argStrings[1:]...)

	var out bytes.Buffer
	cmd.Stdout = &out

	if e := cmd.Run(); e != nil {
		return nil, errors.New(e)
	}

	resultStrings := strings.Split(out.String(), "\n")
	resultArray := make([]interface{}, len(resultStrings))

	for n, v := range resultStrings {
		resultArray[n] = v
	}

	result = datatypes.NewArrayFromArray(datatypes.StringType, resultArray)

	return result, err
}
