package main

import (
	"fmt"
	"os"

	"github.com/tucats/ego/app-cli/app"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/commands"
	"github.com/tucats/ego/data"
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

	// Run the app using the associated grammar and command line arguments.
	err := app.Run(EgoGrammar, os.Args)

	// If we executed bytecode instructions, report the instruction count
	// and maximum stack size used to the tracing log.
	if count := bytecode.InstructionsExecuted.Load(); count > 0 {
		ui.Log(ui.TraceLogger,
			"Executed %d bytecode instructions; maximum runtime stack size was %d elements",
			count, bytecode.MaxStackSize.Load())
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
