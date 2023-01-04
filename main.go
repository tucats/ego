package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/tucats/ego/app-cli/app"
	"github.com/tucats/ego/commands"
	"github.com/tucats/ego/datatypes"
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

	// If something went wrong, report it to the user and force an exit
	// status from the error, else a default General error.
	if !errors.Nil(err) {
		msg := fmt.Sprintf("%s: %v\n", i18n.L("Error"), err.Error())
		os.Stderr.Write([]byte(msg))

		if value := err.GetContext(); value != nil {
			errorCode := 1

			if _, ok := value.(string); !ok {
				errorCode = datatypes.GetInt(value)
			}

			if errorCode == 0 {
				errorCode = 1
			}

			os.Exit(errorCode)
		}

		os.Exit(1)
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
