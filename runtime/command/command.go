package command

import (
	"os/exec"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// Command implements exec.Command() which executes a command in a
// subprocess and returns a *exec.Cmd object that can be used to
// interrogate the success of the operation and view the results.
func Command(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	initCommandTypeDef()

	// Check to see if we're even allowed to do this.
	if !settings.GetBool(defs.ExecPermittedSetting) {
		return nil, errors.ErrNoPrivilegeForOperation.Context("Run")
	}

	// Let's build the Ego instance of exec.Cmd
	result := data.NewStruct(commandTypeDef).FromBuiltinPackage()

	strArray := make([]string, len(args))
	for n, v := range args {
		strArray[n] = data.String(v)
	}

	cmd := exec.Command(strArray[0], strArray[1:]...)

	// Store the native structure, and the path from the rsulting command object
	result.SetAlways("cmd", cmd)
	_ = result.Set("Path", cmd.Path)

	// Also store away the native argument list as an Ego array
	a := data.NewArray(data.StringType, len(cmd.Args))
	for n, v := range cmd.Args {
		_ = a.Set(n, v)
	}

	_ = result.Set("Args", a)

	return result, nil
}

func LookPath(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) != 1 {
		return nil, errors.ErrArgumentCount.Context("LookPath")
	}

	path, err := exec.LookPath(data.String(args[0]))
	if err != nil {
		return "", errors.NewError(err).Context("LookPath")
	}

	return path, nil
}

// getThis returns a map for the "this" object in the current
// symbol table.
func getThisStruct(s *symbols.SymbolTable) *data.Struct {
	t, ok := s.Get(defs.ThisVariable)
	if !ok {
		return nil
	}

	this, ok := t.(*data.Struct)
	if !ok {
		return nil
	}

	return this
}
