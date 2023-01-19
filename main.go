package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/tucats/ego/app-cli/app"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/commands"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/i18n"
)

// BuildVersion is the incremental build version. This is normally
// injected during a build by the build script.
var BuildVersion = "0.0-0"

// BuildTime is a timestamp for this build.
var BuildTime string

// Copyright is the copyright string for this application.
var Copyright = "(C) Copyright Tom Cole 2020, 2021, 2022, 2023"

func main() {
	app := app.New("ego: " + i18n.T("ego")).
		SetVersion(parseVersion(BuildVersion)).
		SetCopyright(Copyright).
		SetDefaultAction(commands.RunAction)

	if BuildTime > "" {
		app.SetBuildTime(BuildTime)
	}

	// Hack. If the second argument is a filename ending in ".ego"
	// then assume it was mean to be a "run" command.
	args := os.Args
	if len(args) > 1 && filepath.Ext(args[1]) == defs.EgoFilenameExtension {
		args = make([]string, 0)

		for i, arg := range os.Args {
			if i == 1 {
				args = append(args, "run")
			}

			args = append(args, arg)
		}
	}

	err := app.Run(EgoGrammar, args)

	// If the runtime stack was used, report this to the tracing log.
	if maxStackSize := bytecode.MaxStackSize.Load(); maxStackSize > 0 {
		ui.Log(ui.TraceLogger, "Maximum runtime stack depth: %d", maxStackSize)
	}

	// If we executed bytecode instructions, report this to the tracing log.
	if count := bytecode.InstructionsExecuted.Load(); count > 0 {
		ui.Log(ui.TraceLogger, "Executed %d bytecode instructions", count)
	}

	// If something went wrong, report it to the user and force an exit
	// status from the error, else a default General error.
	if err != nil {
		if egoErr, ok := err.(*errors.Error); ok {
			if !egoErr.Is(errors.ErrExit) {
				msg := fmt.Sprintf("%s: %v\n", i18n.L("Error"), err.Error())
				os.Stderr.Write([]byte(msg))
				os.Exit(1)
			} else {
				if value := egoErr.GetContext(); value != nil {
					errorCode := 1

					if _, ok := value.(string); ok {
						errorCode = data.Int(value)
					}

					if _, ok := value.(int); ok {
						errorCode = data.Int(value)
					}

					os.Exit(errorCode)
				}
			}
		}

		os.Exit(0)
	}
}

func parseVersion(version string) (major int, minor int, build int) {
	count, err := fmt.Sscanf(version, "%d.%d-%d", &major, &minor, &build)
	if count != 3 || err != nil {
		fmt.Printf("%s\n", i18n.E("version.parse", map[string]interface{}{
			"v": version,
			"c": count,
			"e": err}))
		os.Exit(1)
	}

	return
}
