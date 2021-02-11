package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/tucats/ego/app-cli/app"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/util"
)

// BuildVersion is the incremental build version that is
// injected into the version number string by the build
// script.
var BuildVersion = "0"

// Copyright is the copyright string for this application.
var Copyright = "(C) Copyright Tom Cole 2020, 2021"

func main() {
	buildVer, _ := strconv.Atoi(BuildVersion)
	app := app.New("ego: execute code in the Ego language").
		SetVersion(1, 1, buildVer).
		SetCopyright(Copyright)

	// If there aren't any arguments, default to "run".
	args := os.Args
	if len(args) == 1 {
		args = append(args, "run")
	}

	err := app.Run(EgoGrammar, args)

	// If something went wrong, report it to the user and force an exit
	// status from the error, else a default General error.
	if !errors.Nil(err) {
		fmt.Printf("Error: %v\n", err.Error())

		if value := err.GetContext(); value != nil {
			errorCode := 1

			if _, ok := value.(string); !ok {
				errorCode = util.GetInt(value)
			}

			if errorCode == 0 {
				errorCode = 1
			}

			os.Exit(errorCode)
		}

		os.Exit(1)
	}
}
