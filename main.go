package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/tucats/ego/app-cli/app"
	"github.com/tucats/ego/app-cli/cli"
)

// BuildVersion is the incremental build version that is
// injected into the version number string by the build
// script.
var BuildVersion = "0"

// Copyright is the copyright string for this application
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
	if err != nil {
		fmt.Printf("Error: %v\n", err.Error())

		if e2, ok := err.(cli.ExitError); ok {
			os.Exit(e2.ExitStatus)
		}

		os.Exit(1)
	}
}
