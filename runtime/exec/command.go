package exec

import (
	"os/exec"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/fork"
	"github.com/tucats/ego/symbols"
)

// newCommand implements exec.newCommand() which executes a command in a
// subprocess and returns a *exec.Cmd object that can be used to
// interrogate the success of the operation and view the results.
func newCommand(s *symbols.SymbolTable, args data.List) (any, error) {
	// Check to see if we're even allowed to do this.
	if !settings.GetBool(defs.ExecPermittedSetting) {
		return nil, errors.ErrNoPrivilegeForOperation.In("Run")
	}

	// Let's build the Ego instance of exec.Cmd
	result := data.NewStruct(ExecCmdType).FromBuiltinPackage()

	strArray := make([]string, args.Len())
	for n, v := range args.Elements() {
		strArray[n] = data.String(v)
	}

	strArray = fork.MungeArguments(strArray...)

	cmd := exec.Command(strArray[0], strArray[1:]...)

	// Store the native structure, and the path from the resulting command object
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

// lookPath implements the exec.LookPath() function.
func lookPath(s *symbols.SymbolTable, args data.List) (any, error) {
	path, err := exec.LookPath(data.String(args.Get(0)))
	if err != nil {
		return "", errors.New(err).In("LookPath")
	}

	return path, nil
}

// getThis returns a map for the "this" object in the current
// symbol table.
func getThis(s *symbols.SymbolTable) *data.Struct {
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
