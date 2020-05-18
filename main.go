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
		Value:                RunGrammar,
		ParametersExpected:   -99,
		ParameterDescription: "file-name",
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
	cli.Option{
		LongName:    "trace",
		ShortName:   "t",
		Description: "Display trace of bytecode execution",
		OptionType:  cli.BooleanType,
	},
	cli.Option{
		LongName:    "symbols",
		ShortName:   "s",
		Description: "Display symbol table",
		OptionType:  cli.BooleanType,
	},
	cli.Option{
		LongName:    "environment",
		ShortName:   "e",
		Description: "Automatically add environment vars as symbols",
		OptionType:  cli.BooleanType,
	},
	cli.Option{
		LongName:    "source-tracing",
		ShortName:   "x",
		Description: "Print source lines as they are executed",
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
