package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/tucats/gopackages/app-cli/app"
	"github.com/tucats/gopackages/app-cli/cli"
)

// BuildVersion is the incremental build version that is
// injected into the version number string by the build
// script
var BuildVersion = "0"

func main() {
	app := app.New("ego: execute code in the ego language")

	// Use the build number from the externally-generated build processor.
	buildVer, _ := strconv.Atoi(BuildVersion)
	app.SetVersion(1, 0, buildVer)
	app.SetCopyright("(C) Copyright Tom Cole 2020")

	// fF there aren't any arguments, default to "run"
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
