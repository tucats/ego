package app

import (
	"os"

	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/config"
	"github.com/tucats/ego/app-cli/persistence"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/errors"
)

// Run sets up required data structures and parses the command line. It then
// automatically calls any action routines specfied in the grammar, which do
// the work of the command.
func runFromContext(context *cli.Context) *errors.EgoError {
	// Create a new grammar which prepends the default supplied options
	// to the caller's grammar definition.
	grammar := []cli.Option{
		{
			LongName:            "insecure",
			ShortName:           "k",
			OptionType:          cli.BooleanType,
			Description:         "Do not require X509 server certificate verification",
			Action:              InsecureAction,
			EnvironmentVariable: "EGO_INSECURE_CLIENT",
		},
		{
			LongName:    "config",
			Aliases:     []string{"configuration", "profile", "prof"},
			OptionType:  cli.Subcommand,
			Description: "Manage the default configuration",
			Value:       config.Grammar,
		},
		{
			LongName:    "logon",
			Aliases:     []string{"login"},
			OptionType:  cli.Subcommand,
			Description: "Log on to a remote server",
			Action:      Logon,
			Value:       LogonGrammar,
		},
		{
			ShortName:           "p",
			LongName:            "profile",
			Description:         "Name of profile to use",
			OptionType:          cli.StringType,
			Action:              UseProfileAction,
			EnvironmentVariable: "CLI_PROFILE",
		},
		{
			ShortName:   "d",
			LongName:    "debug",
			Description: "Debug loggers to enable",
			OptionType:  cli.StringListType,
			Action:      DebugAction,
		},
		{
			LongName:            "format",
			ShortName:           "f",
			Description:         "Specify text, json or indented output format",
			OptionType:          cli.KeywordType,
			Keywords:            []string{ui.JSONFormat, ui.JSONIndentedFormat, ui.TextFormat},
			Action:              OutputFormatAction,
			EnvironmentVariable: "CLI_OUTPUT_FORMAT",
		},
		{
			ShortName:   "v",
			LongName:    "version",
			Description: "Show version number of command line tool",
			OptionType:  cli.BooleanType,
			Action:      ShowVersionAction,
		},
		{
			ShortName:           "q",
			LongName:            "quiet",
			Description:         "If specified, suppress extra messaging",
			OptionType:          cli.BooleanType,
			Action:              QuietAction,
			EnvironmentVariable: "CLI_QUIET",
		},
		{
			LongName:    "version",
			Description: "Display the version number",
			OptionType:  cli.Subcommand,
			Action:      VersionAction,
		},
	}

	// Add the user-provided grammar
	grammar = append(grammar, context.Grammar...)

	// Load the active profile, if any from the profile for this application.
	_ = persistence.Load(context.AppName, "default")

	// If the CLI_DEBUG environment variable is set, then turn on
	// debugging now, so messages will come out before that particular
	// option is processed.
	ui.SetLogger(ui.DebugLogger, os.Getenv("CLI_DEBUG") != "")

	// Parse the grammar and call the actions (essentially, execute
	// the function of the CLI)
	context.Grammar = grammar
	err := context.Parse()

	// If no errors, then write out an updated profile as needed.
	if errors.Nil(err) {
		err = persistence.Save()
	}

	return err
}
