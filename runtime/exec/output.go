package exec

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

// output implements the command.output functionality.
func output(s *symbols.SymbolTable, args data.List) (any, error) {
	// Check to see if we're even allowed to do this.
	if !settings.GetBool(defs.ExecPermittedSetting) {
		return nil, errors.ErrNoPrivilegeForOperation.In("Run")
	}

	// Get the Ego structure and the embedded exec.Cmd structure
	cmd := &exec.Cmd{}

	cmdStruct := getThis(s)
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
			stringList := make([]string, strArray.Len())
			for n := 0; n < len(stringList); n++ {
				v, _ := strArray.Get(n)
				stringList[n] = data.String(v)
			}

			buffer := strings.Join(stringList, "\n")
			cmd.Stdin = strings.NewReader(buffer)
		}
	}

	var stdErr bytes.Buffer

	cmd.Stderr = &stdErr

	if e := cmd.Run(); e != nil {
		// Convert the text to an Ego string array. If there is a trailing
		// blank line in the output, remove it
		text := stdErr.String()

		textArray := strings.Split(text, "\n")
		if len(textArray) > 0 && textArray[len(textArray)-1] == "" {
			textArray = textArray[:len(textArray)-1]
		}

		result := data.NewArrayFromStrings(textArray...)
		_ = cmdStruct.Set("Stderr", result)

		return data.NewList(result, errors.New(e)), errors.New(e)
	}

	resultStrings := strings.Split(out.String(), "\n")

	// If there is a trailing blank line in the output, remove it
	if len(resultStrings) > 0 && resultStrings[len(resultStrings)-1] == "" {
		resultStrings = resultStrings[:len(resultStrings)-1]
	}

	// Otherwise, return the array of strings as the output.
	result := data.NewArrayFromStrings(resultStrings...)
	_ = cmdStruct.Set("Stdout", result)

	return data.NewList(result, nil), nil
}
