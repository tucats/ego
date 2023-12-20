// Package app provides the top-level framework for CLI execution. This includes
// the Run() method to run the program, plus a number of action routines that can
// be invoked from the grammar or by a user action routine. These support common or
// global actions, like specifying which profile to use.
package app

import (
	"fmt"
	"os"
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
)

// App is the wrapper type for information needed for a command line application.
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

// New creates a new instance of an application object, given the name of the
// application.
func New(appName string) *App {
	// Extract the description of the app if it was given
	var appDescription = ""

	if i := strings.Index(appName, ":"); i > 0 {
		appDescription = strings.TrimSpace(appName[i+1:])
		appName = strings.TrimSpace(appName[:i])
	}

	app := &App{Name: appName, Description: appDescription}

	return app
}

// SetProfileDirectory sets the default directory name in the user's home
// directory for storing profile configuration inforation. The default is
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

// Set the build time for the app. If the build time is formatted as a valid
// build time, it is encoded as an Ego time.Time value and stored in _buildtime.
// If it is not a valid build time, the string value is stored as-is in the
// _buildtime global variable.
func (app *App) SetBuildTime(s string) *App {
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
// applciation. The grammar must declare action routines for the
// various subcommands, which will be executed by the parser.
func (app *App) Run(grammar []cli.Option, args []string) error {
	app.Context = &cli.Context{
		Description: app.Description,
		Copyright:   app.Copyright,
		Version:     app.Version,
		AppName:     app.Name,
		Grammar:     grammar,
		Args:        args,
		Action:      app.Action,
	}

	// Create the platform definition symbols
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

	platform := data.NewStruct(platformType)
	_ = platform.Set("go", runtime.Version())
	_ = platform.Set("os", runtime.GOOS)
	_ = platform.Set("arch", runtime.GOARCH)
	_ = platform.Set("cpus", runtime.NumCPU())
	platform.SetReadonly(true)
	_ = symbols.RootSymbolTable.SetWithAttributes(defs.PlatformVariable, platform,
		symbols.SymbolAttribute{Readonly: true})

	if err := SetDefaultLoggers(); err != nil {
		return err
	}

	return runFromContext(app.Context)
}

// Enable the loggers that are set using the EGO_DEFAULT_LOOGGERS
// environment variable.
func SetDefaultLoggers() error {
	logList := os.Getenv(defs.EgoDefaultLogging)
	if strings.TrimSpace(logList) == "" {
		return nil
	}

	loggers := strings.Split(logList, ",")

	for _, loggerName := range loggers {
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
