package command

import (
	"bytes"
	"os/exec"
	"strings"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

func Run(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	// Check to see if we're even allowed to do this.
	if !settings.GetBool(defs.ExecPermittedSetting) {
		return nil, errors.ErrNoPrivilegeForOperation.Context("Run")
	}

	// Get the Ego structure and the embedded exec.Cmd structure
	cmd := &exec.Cmd{}

	cmdStruct := getThisStruct(s)
	if i, ok := cmdStruct.Get("cmd"); ok {
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
		return nil, errors.NewError(e)
	}

	resultStrings := strings.Split(out.String(), "\n")
	resultArray := make([]interface{}, len(resultStrings))

	for n, v := range resultStrings {
		resultArray[n] = v
	}

	result := data.NewArrayFromArray(data.StringType, resultArray)
	_ = cmdStruct.Set("Stdout", result)

	return nil, nil
}
