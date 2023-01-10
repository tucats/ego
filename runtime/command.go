package runtime

import (
	"bytes"
	"os/exec"
	"strings"
	"sync"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/compiler"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

var commandTypeDef *data.Type
var commandTypeDefLock sync.Mutex

func initCommandTypeDef() {
	commandTypeDefLock.Lock()
	defer commandTypeDefLock.Unlock()

	if commandTypeDef == nil {
		t, _ := compiler.CompileTypeSpec(commandTypeSpec)

		t.DefineFunctions(map[string]interface{}{
			"Output": CommandOutput,
			"Run":    CommandRun,
		})

		commandTypeDef = t
	}
}

func NewCommand(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	initCommandTypeDef()

	// Check to see if we're even allowed to do this.
	if !settings.GetBool(defs.ExecPermittedSetting) {
		return nil, errors.EgoError(errors.ErrNoPrivilegeForOperation).Context("Run")
	}

	// Let's build the Ego instance of exec.Cmd
	result := data.NewStruct(commandTypeDef)

	strArray := make([]string, len(args))
	for n, v := range args {
		strArray[n] = data.String(v)
	}

	cmd := exec.Command(strArray[0], strArray[1:]...)

	// Store the native structure, and the path from the rsulting command object
	result.SetAlways("__cmd", cmd)
	_ = result.Set("Path", cmd.Path)

	// Also store away the native argument list as an Ego array
	a := data.NewArray(&data.StringType, len(cmd.Args))
	for n, v := range cmd.Args {
		_ = a.Set(n, v)
	}

	_ = result.Set("Args", a)

	return result, nil
}

func LookPath(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) != 1 {
		return nil, errors.EgoError(errors.ErrArgumentCount).Context("LookPath")
	}

	path, err := exec.LookPath(data.String(args[0]))
	if err != nil {
		return "", errors.EgoError(err).Context("LookPath")
	}

	return path, nil
}

func CommandRun(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	// Check to see if we're even allowed to do this.
	if !settings.GetBool(defs.ExecPermittedSetting) {
		return nil, errors.EgoError(errors.ErrNoPrivilegeForOperation).Context("Run")
	}

	// Get the Ego structure and the embedded exec.Cmd structure
	cmd := &exec.Cmd{}

	cmdStruct := getThisStruct(s)
	if i, ok := cmdStruct.Get("__cmd"); ok {
		cmd, _ = i.(*exec.Cmd)
	}

	if str, ok := cmdStruct.Get("Stdin"); ok {
		s := data.String(str)
		cmd.Stdin = strings.NewReader(s)
	}

	if str, ok := cmdStruct.Get("Path"); ok {
		s := data.String(str)
		cmd.Path = s
	}

	if str, ok := cmdStruct.Get("dir"); ok {
		s := data.String(str)
		cmd.Dir = s
	}

	if argArray, ok := cmdStruct.Get("Args"); ok {
		if args, ok := argArray.(*data.Array); ok {
			r := make([]string, args.Len())
			for n := 0; n < len(r); n++ {
				v, _ := args.Get(n)
				r[n] = data.String(v)
			}

			cmd.Args = r
		}
	}

	if argArray, ok := cmdStruct.Get("Env"); ok {
		if args, ok := argArray.(*data.Array); ok {
			r := make([]string, args.Len())
			for n := 0; n < len(r); n++ {
				v, _ := args.Get(n)
				r[n] = data.String(v)
			}

			cmd.Env = r
		}
	}

	var out bytes.Buffer
	cmd.Stdout = &out

	if a, ok := cmdStruct.Get("Stdin"); ok {
		if strArray, ok := a.(*data.Array); ok {
			strs := make([]string, strArray.Len())
			for n := 0; n < len(strs); n++ {
				v, _ := strArray.Get(n)
				strs[n] = data.String(v)
			}

			buffer := strings.Join(strs, "\n")
			cmd.Stdin = strings.NewReader(buffer)
		}
	}

	if e := cmd.Run(); e != nil {
		return nil, errors.EgoError(e)
	}

	resultStrings := strings.Split(out.String(), "\n")
	resultArray := make([]interface{}, len(resultStrings))

	for n, v := range resultStrings {
		resultArray[n] = v
	}

	result := data.NewArrayFromArray(&data.StringType, resultArray)
	_ = cmdStruct.Set("Stdout", result)

	return nil, nil
}

func CommandOutput(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) > 0 {
		return nil, errors.EgoError(errors.ErrArgumentCount)
	}

	// Check to see if we're even allowed to do this.
	if !settings.GetBool(defs.ExecPermittedSetting) {
		return nil, errors.EgoError(errors.ErrNoPrivilegeForOperation).Context("Run")
	}

	// Get the Ego structure and the embedded exec.Cmd structure
	cmd := &exec.Cmd{}

	cmdStruct := getThisStruct(s)
	if i, ok := cmdStruct.Get("__cmd"); ok {
		cmd, _ = i.(*exec.Cmd)
	}

	if str, ok := cmdStruct.Get("Stdin"); ok {
		s := data.String(str)
		cmd.Stdin = strings.NewReader(s)
	}

	if str, ok := cmdStruct.Get("Path"); ok {
		s := data.String(str)
		cmd.Path = s
	}

	if str, ok := cmdStruct.Get("dir"); ok {
		s := data.String(str)
		cmd.Dir = s
	}

	if argArray, ok := cmdStruct.Get("Args"); ok {
		if args, ok := argArray.(*data.Array); ok {
			r := make([]string, args.Len())
			for n := 0; n < len(r); n++ {
				v, _ := args.Get(n)
				r[n] = data.String(v)
			}

			cmd.Args = r
		}
	}

	if argArray, ok := cmdStruct.Get("Env"); ok {
		if args, ok := argArray.(*data.Array); ok {
			r := make([]string, args.Len())
			for n := 0; n < len(r); n++ {
				v, _ := args.Get(n)
				r[n] = data.String(v)
			}

			cmd.Env = r
		}
	}

	var out bytes.Buffer
	cmd.Stdout = &out

	if a, ok := cmdStruct.Get("Stdin"); ok {
		if strArray, ok := a.(*data.Array); ok {
			strs := make([]string, strArray.Len())
			for n := 0; n < len(strs); n++ {
				v, _ := strArray.Get(n)
				strs[n] = data.String(v)
			}

			buffer := strings.Join(strs, "\n")
			cmd.Stdin = strings.NewReader(buffer)
		}
	}

	if e := cmd.Run(); e != nil {
		return nil, errors.EgoError(e)
	}

	resultStrings := strings.Split(out.String(), "\n")
	resultArray := make([]interface{}, len(resultStrings))

	for n, v := range resultStrings {
		resultArray[n] = v
	}

	result := data.NewArrayFromArray(&data.StringType, resultArray)
	_ = cmdStruct.Set("Stdout", result)

	return result, nil
}
