package main

import (
	"fmt"
	"os"

	"github.com/tucats/ego/app-cli/app"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
)

// BuildVersion is the incremental build version. This is normally
// injected during a build by the build script.
var BuildVersion = "0.0-0"

// BuildTime is a timestamp for this build.
var BuildTime string

// Copyright is the copyright string for this application.
var Copyright = "(C) Copyright Tom Cole 2020, 2021"

func main() {
	app := app.New("ego: execute code in the Ego language").
		SetVersion(parseVersion(BuildVersion)).
		SetCopyright(Copyright)

	if BuildTime > "" {
		app.SetBuildTime(BuildTime)
	}

	// If there aren't any arguments, default to "run".
	args := os.Args
	if len(args) == 1 {
		args = append(args, "run")
	}

	err := app.Run(EgoGrammar, args)

	// If something went wrong, report it to the user and force an exit
	// status from the error, else a default General error.
	if !errors.Nil(err) {
		msg := fmt.Sprintf("Error: %v\n", err.Error())
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
		fmt.Printf("Unable to process version number %s; count=%d, err=%v\n", version, count, err)
		os.Exit(1)
	}

	return
}
