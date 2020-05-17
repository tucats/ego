package main

import (
	"fmt"
	"os"

	"github.com/tucats/gopackages/app-cli/app"
	"github.com/tucats/gopackages/app-cli/cli"
)

// SolveGrammar handles the command line options
var SolveGrammar = []cli.Option{
	cli.Option{
		LongName:             "run",
		Description:          "Run an existing program",
		OptionType:           cli.Subcommand,
		Action:               RunAction,
		ParametersExpected:   1,
		ParameterDescription: "file-name",
	},
	cli.Option{
		LongName:             "interactive",
		Description:          "Type in program statements from the console",
		OptionType:           cli.Subcommand,
		Action:               RunAction,
		ParametersExpected:   0,
		ParameterDescription: "",
	},
}

// RunGrammar handles the command line options
var RunGrammar = []cli.Option{
	cli.Option{
		LongName:    "disassemble",
		ShortName:   "d",
		Description: "Display a disassembly of the bytecode before execution",
		OptionType:  cli.BooleanType,
	},
}

func main() {
	app := app.New("solve: execute code in the solve language")
	app.SetVersion(1, 0, 0)
	app.SetCopyright("(C) Copyright Tom Cole 2020")

	err := app.Run(SolveGrammar, os.Args)

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
