package app

import (
	"os"
	"strings"

	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/config"
	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
)

// Run sets up required data structures and parses the command line. It then
// automatically calls any action routines specfied in the grammar, which do
// the work of the command.
func runFromContext(context *cli.Context) error {
	// Create a new grammar which prepends the default supplied options
	// to the caller's grammar definition.
	grammar := []cli.Option{
		{
			LongName:   "env-config",
			OptionType: cli.BooleanType,
			Action:     EnvAction,
			Private:    true,
		},
		{
			LongName:    "insecure",
			ShortName:   "k",
			OptionType:  cli.BooleanType,
			Description: "insecure",
			Action:      InsecureAction,
			EnvVar:      "EGO_INSECURE_CLIENT",
		},
		{
			LongName:    "config",
			Aliases:     []string{"configuration", "profile", "prof"},
			OptionType:  cli.Subcommand,
			Description: "ego.config",
			Value:       config.Grammar,
		},
		{
			LongName:    "logon",
			Aliases:     []string{"login"},
			OptionType:  cli.Subcommand,
			Description: "ego.logon",
			Action:      Logon,
			Value:       LogonGrammar,
		},
		{
			ShortName:   "p",
			LongName:    "profile",
			Description: "global.profile",
			OptionType:  cli.StringType,
			Action:      UseProfileAction,
			EnvVar:      "EGO_PROFILE",
		},
		{
			LongName:    "log",
			ShortName:   "l",
			Description: "global.log",
			OptionType:  cli.StringListType,
			Action:      LogAction,
			EnvVar:      defs.EgoDefaultLogging,
		},
		{
			LongName:    "log-file",
			Description: "global.log.file",
			OptionType:  cli.StringType,
			Action:      LogFileAction,
			EnvVar:      defs.EgoDefaultLogFileName,
		},
		{
			LongName:    "format",
			ShortName:   "f",
			Description: "global.format",
			OptionType:  cli.KeywordType,
			Keywords:    []string{ui.JSONFormat, ui.JSONIndentedFormat, ui.TextFormat},
			Action:      OutputFormatAction,
			EnvVar:      "EGO_OUTPUT_FORMAT",
		},
		{
			ShortName:   "v",
			LongName:    "version",
			Description: "global.version",
			OptionType:  cli.BooleanType,
			Action:      ShowVersionAction,
		},
		{
			ShortName:   "q",
			LongName:    "quiet",
			Description: "global.quiet",
			OptionType:  cli.BooleanType,
			Action:      QuietAction,
			EnvVar:      "EGO_QUIET",
		},
		{
			LongName:    "version",
			Description: "global.version",
			OptionType:  cli.Subcommand,
			Action:      VersionAction,
		},
	}

	// Add the user-provided grammar.
	grammar = append(grammar, context.Grammar...)

	// Load the active profile, if any from the profile for this application.
	_ = settings.Load(context.AppName, "default")

	context.Grammar = grammar

	// If we are to dump the grammar (a diagnostic function) do that,
	// then just pack it in and go home.
	if os.Getenv("EGO_DUMP_GRAMMAR") != "" {
		cli.DumpGrammar(context)
		os.Exit(0)
	}

	// Parse the grammar and call the actions (essentially, execute
	// the function of the CLI). If it goes poorly, error out.
	if err := context.Parse(); err != nil {
		return err
	} else {
		// If no errors, then write out an updated profile as needed.
		err = settings.Save()
		if err != nil {
			return err
		}
	}

	return nil
}

// Get temporary settings values from any environment variables that may have
// been defined. The settings names are remapped as upper-case names with the "."
// being replaced by a "_". So ego.runtime.path becomes EGO_RUNTIME_PATH. If there
// is an environment variable of the (mapped) setting name, then it's value is
// used as the default. Note that the variable must exist and be explicitly set
// to an empty string to set the value to an empty string in the settings database.
//
// The function returns the number of settings that were overridden by environment
// variables.
func loadEnvSettings() int {
	count := 0

	// Make a local map that descries the environment variables.
	env := map[string]string{}
	for _, key := range os.Environ() {
		value := ""
		if p := strings.Index(key, "="); p > 0 {
			value = key[p+1:]
			key = key[:p]
		}

		env[key] = value
	}

	// Search over the list of valid setting names, and see if any
	// match up to an exiting environment variable. If the variable
	// exists, set the value in the default settings area.
	for k := range defs.ValidSettings {
		key := strings.ReplaceAll(strings.ToUpper(k), ".", "_")
		if value, found := env[key]; found {
			settings.SetDefault(k, value)
			count++
		}
	}

	return count
}
