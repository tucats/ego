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
// automatically calls any action routines specified in the grammar, which do
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
			EnvVar:      defs.EgoInsecureClientEnv,
		},
		{
			ShortName:   "p",
			LongName:    "profile",
			Description: "global.profile",
			OptionType:  cli.StringType,
			Action:      UseProfileAction,
			EnvVar:      defs.EgoProfileEnv,
		},
		{
			LongName:   "no-lib-init",
			Private:    true,
			OptionType: cli.BooleanType,
			Action:     LibraryAction,
			EnvVar:     defs.EnvNoLibInitEnv,
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
			LongName:    "localization-file",
			Aliases:     []string{"localizations", "i18n", "i18n-file"},
			Description: "global.localization.file",
			OptionType:  cli.StringType,
			Action:      LocalizationFileAction,
			EnvVar:      defs.EgoLocalizationFileEnv,
		},
		{
			LongName:    "format",
			ShortName:   "f",
			Description: "global.format",
			OptionType:  cli.KeywordType,
			Keywords:    []string{ui.JSONFormat, ui.JSONIndentedFormat, ui.TextFormat},
			Action:      OutputFormatAction,
			EnvVar:      defs.EgoOutputFormatEnv,
		},
		{
			LongName:    "log-format",
			Description: "global.log.format",
			OptionType:  cli.KeywordType,
			Keywords:    []string{ui.JSONFormat, ui.JSONIndentedFormat, ui.TextFormat},
			Action:      LogFormatAction,
			EnvVar:      defs.EgoLogFormatEnv,
		},
		{
			ShortName:   "v",
			LongName:    "version",
			Description: "global.version",
			OptionType:  cli.BooleanType,
			Action:      ShowVersionAction,
		},
		{
			LongName:    "json-query",
			ShortName:   "j",
			Description: "global.json-query",
			OptionType:  cli.StringType,
			Action:      JSONQueryAction,
		},
		{
			ShortName:   "q",
			LongName:    "quiet",
			Description: "global.quiet",
			OptionType:  cli.BooleanType,
			Action:      QuietAction,
			EnvVar:      defs.EgoQuietEnv,
		},
		{
			LongName:    "maxcpus",
			Aliases:     []string{"cpus", "maxprocs", "procs"},
			Description: "global.maxcpus",
			OptionType:  cli.IntType,
			Action:      MaxProcsAction,
			EnvVar:      defs.EgoMaxProcsEnv,
		},
		{
			ShortName:   "s",
			LongName:    "set",
			Description: "global.set",
			OptionType:  cli.StringListType,
			Action:      SetAction,
		},
		{
			LongName:    "archive-log",
			Aliases:     []string{"archive"},
			Description: "global.archive-log",
			OptionType:  cli.StringType,
			Action:      ArchiveLogFileAction,
			EnvVar:      defs.EgoArchiveLogEnv,
		},
	}

	baseCommands := []cli.Option{
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
			LongName:    "version",
			Description: "opt.global.version",
			OptionType:  cli.Subcommand,
			Action:      VersionAction,
		},
	}

	// Add the user-provided grammar. Note that if we are running in verb/subject
	// mode, we don't do this as the commands are already in the grammar.
	grammar = append(grammar, context.Grammar...)

	if !strings.Contains(strings.ToLower(os.Getenv("EGO_GRAMMAR")), "verb") {
		grammar = append(grammar, baseCommands...)
	}

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
		if err = settings.Save(); err != nil {
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

	// Make a local map that describes the environment variables.
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
