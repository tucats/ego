package runtime

import (
	"bytes"
	"os/exec"
	"strings"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/compiler"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

var commandTypeDef *datatypes.Type

func initCommandTypeDef() {
	if commandTypeDef == nil {
		t, _ := compiler.CompileTypeSpec(commandTypeSpec)

		t.DefineFunctions(map[string]interface{}{
			"Output": CommandOutput,
			"Run":    CommandRun,
		})

		commandTypeDef = &t
	}
}

func NewCommand(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	initCommandTypeDef()

	// Check to see if we're even allowed to do this.
	if !settings.GetBool(defs.ExecPermittedSetting) {
		return nil, errors.New(errors.ErrNoPrivilegeForOperation).Context("Run")
	}

	// Let's build the Ego instance of exec.Cmd
	result := datatypes.NewStruct(*commandTypeDef)

	strArray := make([]string, len(args))
	for n, v := range args {
		strArray[n] = datatypes.GetString(v)
	}

	cmd := exec.Command(strArray[0], strArray[1:]...)

	// Store the native structure, and the path from the rsulting command object
	result.SetAlways("__cmd", cmd)
	_ = result.Set("Path", cmd.Path)

	// Also store away the native argument list as an Ego array
	a := datatypes.NewArray(datatypes.StringType, len(cmd.Args))
	for n, v := range cmd.Args {
		_ = a.Set(n, v)
	}

	_ = result.Set("Args", a)

	return result, nil
}

func LookPath(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if len(args) != 1 {
		return nil, errors.New(errors.ErrArgumentCount).Context("LookPath")
	}

	path, err := exec.LookPath(datatypes.GetString(args[0]))
	if err != nil {
		return "", errors.New(err).Context("LookPath")
	}

	return path, nil
}

func CommandRun(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	// Check to see if we're even allowed to do this.
	if !settings.GetBool(defs.ExecPermittedSetting) {
		return nil, errors.New(errors.ErrNoPrivilegeForOperation).Context("Run")
	}

	// Get the Ego structure and the embedded exec.Cmd structure
	cmd := &exec.Cmd{}

	cmdStruct := getThisStruct(s)
	if i, ok := cmdStruct.Get("__cmd"); ok {
		cmd, _ = i.(*exec.Cmd)
	}

	if str, ok := cmdStruct.Get("Stdin"); ok {
		s := datatypes.GetString(str)
		cmd.Stdin = strings.NewReader(s)
	}

	if str, ok := cmdStruct.Get("Path"); ok {
		s := datatypes.GetString(str)
		cmd.Path = s
	}

	if str, ok := cmdStruct.Get("dir"); ok {
		s := datatypes.GetString(str)
		cmd.Dir = s
	}

	if argArray, ok := cmdStruct.Get("Args"); ok {
		if args, ok := argArray.(*datatypes.EgoArray); ok {
			r := make([]string, args.Len())
			for n := 0; n < len(r); n++ {
				v, _ := args.Get(n)
				r[n] = datatypes.GetString(v)
			}

			cmd.Args = r
		}
	}

	if argArray, ok := cmdStruct.Get("Env"); ok {
		if args, ok := argArray.(*datatypes.EgoArray); ok {
			r := make([]string, args.Len())
			for n := 0; n < len(r); n++ {
				v, _ := args.Get(n)
				r[n] = datatypes.GetString(v)
			}

			cmd.Env = r
		}
	}

	var out bytes.Buffer
	cmd.Stdout = &out

	if a, ok := cmdStruct.Get("Stdin"); ok {
		if strArray, ok := a.(*datatypes.EgoArray); ok {
			strs := make([]string, strArray.Len())
			for n := 0; n < len(strs); n++ {
				v, _ := strArray.Get(n)
				strs[n] = datatypes.GetString(v)
			}

			buffer := strings.Join(strs, "\n")
			cmd.Stdin = strings.NewReader(buffer)
		}
	}

	if e := cmd.Run(); e != nil {
		return nil, errors.New(e)
	}

	resultStrings := strings.Split(out.String(), "\n")
	resultArray := make([]interface{}, len(resultStrings))

	for n, v := range resultStrings {
		resultArray[n] = v
	}

	result := datatypes.NewArrayFromArray(datatypes.StringType, resultArray)
	_ = cmdStruct.Set("Stdout", result)

	return nil, nil
}

func CommandOutput(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	// Check to see if we're even allowed to do this.
	if !settings.GetBool(defs.ExecPermittedSetting) {
		return nil, errors.New(errors.ErrNoPrivilegeForOperation).Context("Run")
	}

	// Get the Ego structure and the embedded exec.Cmd structure
	cmd := &exec.Cmd{}

	cmdStruct := getThisStruct(s)
	if i, ok := cmdStruct.Get("__cmd"); ok {
		cmd, _ = i.(*exec.Cmd)
	}

	if str, ok := cmdStruct.Get("Stdin"); ok {
		s := datatypes.GetString(str)
		cmd.Stdin = strings.NewReader(s)
	}

	if str, ok := cmdStruct.Get("Path"); ok {
		s := datatypes.GetString(str)
		cmd.Path = s
	}

	if str, ok := cmdStruct.Get("dir"); ok {
		s := datatypes.GetString(str)
		cmd.Dir = s
	}

	if argArray, ok := cmdStruct.Get("Args"); ok {
		if args, ok := argArray.(*datatypes.EgoArray); ok {
			r := make([]string, args.Len())
			for n := 0; n < len(r); n++ {
				v, _ := args.Get(n)
				r[n] = datatypes.GetString(v)
			}

			cmd.Args = r
		}
	}

	if argArray, ok := cmdStruct.Get("Env"); ok {
		if args, ok := argArray.(*datatypes.EgoArray); ok {
			r := make([]string, args.Len())
			for n := 0; n < len(r); n++ {
				v, _ := args.Get(n)
				r[n] = datatypes.GetString(v)
			}

			cmd.Env = r
		}
	}

	var out bytes.Buffer
	cmd.Stdout = &out

	if a, ok := cmdStruct.Get("Stdin"); ok {
		if strArray, ok := a.(*datatypes.EgoArray); ok {
			strs := make([]string, strArray.Len())
			for n := 0; n < len(strs); n++ {
				v, _ := strArray.Get(n)
				strs[n] = datatypes.GetString(v)
			}

			buffer := strings.Join(strs, "\n")
			cmd.Stdin = strings.NewReader(buffer)
		}
	}

	if e := cmd.Run(); e != nil {
		return nil, errors.New(e)
	}

	resultStrings := strings.Split(out.String(), "\n")
	resultArray := make([]interface{}, len(resultStrings))

	for n, v := range resultStrings {
		resultArray[n] = v
	}

	result := datatypes.NewArrayFromArray(datatypes.StringType, resultArray)
	_ = cmdStruct.Set("Stdout", result)

	return result, nil
}