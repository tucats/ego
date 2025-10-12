// Package app provides the top-level framework for CLI execution. This includes
// the Run() method to run the program, plus a number of action routines that can
// be invoked from the grammar or by a user action routine. These support common or
// global actions, like specifying which profile to use.
package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/validate"
)

// App is the object describing information needed for a command line application.
// It contains the globals needed for the application as well as the runtime
// context root.
type App struct {
	// The name of the application. This is used to form the name of
	// configuration files, and the default prompt for console input.
	Name string

	// This is a text description of the application, which is used
	// when forming the default "help" output.
	Description string

	// This is a copyright string for the app, which is displayed in the
	// "help" output.
	Copyright string

	// This is a version string for the application. This is displayed in
	// the "help" output, and is available as a global variable.
	Version string

	// This is a string representation of the time and date the application
	// was built.
	BuildTime string

	// This is a command-line context object that is associated with the app,
	// and is used to manage parsing command line options.
	Context *cli.Context

	// This is the default action to run to execute the application. This can
	// be overridden in the grammar definition for the command line options and
	// subcommands.
	Action func(c *cli.Context) error
}

// Set this value to true to enable debugging messages when the "env.json" environment
// variables are set. By default, this must be false.
var debugEnv = false

// New creates a new instance of an application object, given the name of the
// application. If the name contains a colon character (":") the first part of
// the string is assumed to be the name, and the second part is assumed to be
// a description of what the application does. This information is used to
// generate information from the "help" output.
func New(appName string) *App {
	// Extract the description of the app if it was given
	var appDescription = ""

	if i := strings.Index(appName, ":"); i > 0 {
		appDescription = strings.TrimSpace(appName[i+1:])
		appName = strings.TrimSpace(appName[:i])
	}

	// Build the App object
	app := &App{Name: appName, Description: appDescription}

	return app
}

// SetProfileDirectory sets the default directory name in the user's home
// directory for storing profile configuration information. The default is
// ".org.fernwood" but this can be overridden by the main program using
// this function.
func (app *App) SetProfileDirectory(name string) *App {
	settings.ProfileDirectory = name

	return app
}

// SetVersion sets the version number for the application. If the
// values for major, minor, and delta (often build number) are
// zero, the default version is "developer build". This value
// is also stored in the global symbol table.
func (app *App) SetVersion(major, minor, delta int) *App {
	if major == 0 && minor == 0 && delta == 0 {
		app.Version = `"developer build"`
	} else {
		app.Version = fmt.Sprintf("%d.%d-%d", major, minor, delta)
	}

	symbols.RootSymbolTable.SetAlways(defs.VersionNameVariable, app.Version)

	return app
}

// SetCopyright sets the copyright string (if any) used in the
// help output. This value is also stored in the global symbol
// table.
func (app *App) SetCopyright(s string) *App {
	app.Copyright = s
	symbols.RootSymbolTable.SetAlways(defs.CopyrightVariable, app.Copyright)

	return app
}

// Set the build time for the app, if a non-empty build time was given.
// If the build time is formatted as a valid build time, it is encoded
// as an Ego time.Time value and stored in _buildtime. If it is not a
// valid build time, the string value is stored as-is in the _buildtime
// global variable.
func (app *App) SetBuildTime(s string) *App {
	if s == "" {
		return app
	}

	app.BuildTime = s

	if t, err := time.Parse("20060102150405", s); err == nil {
		text := t.String()
		symbols.RootSymbolTable.SetAlways(defs.BuildTimeVariable, text)
		app.BuildTime = text
	} else {
		symbols.RootSymbolTable.SetAlways(defs.BuildTimeVariable, app.BuildTime)
	}

	return app
}

// Parse runs a grammar, and then calls the provided action routine. It is typically
// used in cases where there are no subcommands, and an action should be run after
// parsing options.
func (app *App) Parse(grammar []cli.Option, args []string, action func(c *cli.Context) error) error {
	app.Action = action

	return app.Run(grammar, args)
}

// This operation sets the default action function of the application. This is used
// when the command line contains only options without a command verb.
func (app *App) SetDefaultAction(f func(c *cli.Context) error) *App {
	app.Action = f

	return app
}

// Run runs a grammar given a set of arguments in the current
// application. The grammar must declare action routines for the
// various subcommands, which will be executed by the parser.
func (app *App) Run(grammar []cli.Option, args []string) error {
	// Build the context used for parsing and executing the command line
	app.Context = &cli.Context{
		Description: app.Description,
		Copyright:   app.Copyright,
		Version:     app.Version,
		AppName:     app.Name,
		Grammar:     grammar,
		Args:        args,
		Action:      app.Action,
	}

	// Create the platform definition data type. This is an Ego structure
	// stored in the global symbol table that describes the basic info
	// about the platform (computer) running this instance of Ego.
	platformType := data.StructureType(
		data.Field{
			Name: "os",
			Type: data.StringType,
		},
		data.Field{
			Name: "arch",
			Type: data.StringType,
		},
		data.Field{
			Name: "go",
			Type: data.StringType,
		},
		data.Field{
			Name: "cpus",
			Type: data.IntType,
		},
	)

	// Create a new instance of the platform structure type.
	platform := data.NewStruct(platformType)

	// Determine the number of CPUs available to run Ego. The default to number
	// of CPUS available to the process. If the number specified for use by Go
	// routine scheduling is less, set that to the max number of CPUs allowed.
	cpuCount := runtime.NumCPU()
	if maxCpuCount := runtime.GOMAXPROCS(-1); cpuCount > maxCpuCount {
		cpuCount = maxCpuCount
	}

	// Set the fields in the data structure.
	_ = platform.Set("go", runtime.Version())
	_ = platform.Set("os", runtime.GOOS)
	_ = platform.Set("arch", runtime.GOARCH)
	_ = platform.Set("cpus", cpuCount)

	// Mark the structure as readonly and add it to the global symbol table.
	platform.SetReadonly(true)
	_ = symbols.RootSymbolTable.SetWithAttributes(
		defs.PlatformVariable,
		platform,
		symbols.SymbolAttribute{Readonly: true})

	// Set the default loggers based on the EGO_DEFAULT_LOGGING environment
	// variable. This variable, if set, contains a comma-separated list of
	// logger names. If the list is empty, no loggers are enabled.
	// If the list contains invalid logger names, an error is returned.
	if err := SetDefaultLoggers(); err != nil {
		return err
	}

	// Using the context variable defined in the app object, run the application.
	// The context is used to hold information about the current invocation and
	// the current command line arguments. The runFromContext function
	// is responsible for parsing the grammar, running the application, and
	// returning any errors that occurred.
	//
	// If an error occurs during the parsing or running of the application,
	// the function returns the error. Otherwise, it returns nil.
	return runFromContext(app.Context)
}

// Enable the loggers that are set using the EGO_DEFAULT_LOGGING
// environment variable. This environment variable can contain a
// comma-separated list of logger names. If the list is empty, no loggers
// are enabled. If the list contains invalid logger names, an error is
// returned. The logger names are not case-sensitive.
func SetDefaultLoggers() error {
	logFormat := os.Getenv(defs.EgoLogFormatEnv)
	if logFormat != "" {
		logFormat = strings.ToLower(logFormat)
		if logFormat != "json" && logFormat != "text" {
			return errors.ErrInvalidLogFormat.Context(logFormat)
		}

		ui.LogFormat = logFormat
	}

	logList := os.Getenv(defs.EgoDefaultLogging)
	if strings.TrimSpace(logList) == "" {
		return nil
	}

	// Make an array containing the logger names. Loop over the
	// array and if the name is valid, set it's state to active.
	for _, loggerName := range strings.Split(logList, ",") {
		trimmedName := strings.TrimSpace(loggerName)
		if trimmedName != "" {
			logger := ui.LoggerByName(trimmedName)
			if logger < 0 {
				return errors.ErrInvalidLoggerName.Context(trimmedName)
			}

			ui.Active(logger, true)
		}
	}

	return nil
}

// SetEnvironmentSettings loads the static environment settings.
func SetEnvironment(path string) error {
	// Load any JSON validations needed for configuration processing
	libPath := settings.Get(defs.LibPathName)
	if libPath == "" {
		libPath = filepath.Join(settings.Get(defs.EgoPathSetting), defs.LibPathName)
	}

	vFn := filepath.Join(libPath, "validations", "env.json")
	hasValidations := false

	bytes, err := os.ReadFile(vFn)
	if err == nil {
		if err = validate.Load("env:config", bytes); err == nil {
			hasValidations = true
		}
	}

	// Get the home directory of the current user.
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	// Construct the path to the environment settings file.
	filePath := filepath.Join(home, path, "env.json")

	// Read the contents of the environment settings json file
	b, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	if hasValidations {
		// Validate the environment settings JSON file against the JSON schema.
		if err := validate.Validate(b, "env:config"); err != nil {
			return errors.New(err).Chain(errors.New(errors.ErrConfig).Context(filePath))
		}
	}

	// Unmarshal the JSON file into a map of strings.
	var settings map[string]any

	err = json.Unmarshal(b, &settings)
	if err != nil {
		return errors.New(errors.ErrConfig).Context(filePath).Chain(errors.New(err))
	}

	if len(settings) == 0 {
		return nil
	}

	// Loop over the map and set the environment variables.
	messages := make([]string, 0, len(settings))

	for k, v := range settings {
		if oldValue := os.Getenv(k); oldValue == "" {
			os.Setenv(k, data.String(v))
			messages = append(messages, fmt.Sprintf("%s=%s", k, v))
		}
	}

	// Enable for debugging only.
	if debugEnv {
		fmt.Println("ENV:", strings.Join(messages, ", "))
	}

	return nil
}
