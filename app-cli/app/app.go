// Package app provides the top-level framework for CLI execution. This includes
// the Run() method to run the program, plus a number of action routines that can
// be invoked from the grammar or by a user action routine. These support common or
// global actions, like specifying which profile to use.
package app

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// App is the wrapper type for information needed for a command line application.
// It contains the globals needed for the application as well as the runtime
// context root.
type App struct {
	Name        string
	Description string
	Copyright   string
	Version     string
	Context     *cli.Context
	Action      func(c *cli.Context) *errors.EgoError
}

// New creates a new instance of an application context, given the name of the
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

// SetVersion sets the version number for the application.
func (app *App) SetVersion(major, minor, delta int) *App {
	app.Version = fmt.Sprintf("%d.%d-%d", major, minor, delta)
	_ = symbols.RootSymbolTable.SetAlways("_version", app.Version)

	return app
}

// SetCopyright sets the copyright string (if any) used in the
// help output.
func (app *App) SetCopyright(s string) *App {
	app.Copyright = s
	_ = symbols.RootSymbolTable.SetAlways("_copyright", app.Copyright)

	return app
}

// Parse runs a grammar, and then calls the provided action routine. It is typically
// used in cases where there are no subcommands, and an action should be run after
// parsing options.
func (app *App) Parse(grammar []cli.Option, args []string, action func(c *cli.Context) *errors.EgoError) *errors.EgoError {
	app.Action = action

	return app.Run(grammar, args)
}

// Run runs a grammar given a set of arguments in the current
// applciation. The grammar must declare action routines for the
// various subcommands, which will be executed by the parser.
func (app *App) Run(grammar []cli.Option, args []string) *errors.EgoError {
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
	platformType := datatypes.Structure(
		datatypes.Field{
			Name: "os",
			Type: datatypes.StringType,
		},
		datatypes.Field{
			Name: "arch",
			Type: datatypes.StringType,
		},
		datatypes.Field{
			Name: "go",
			Type: datatypes.StringType,
		},
		datatypes.Field{
			Name: "cpus",
			Type: datatypes.IntType,
		},
	)

	platform := datatypes.NewStruct(platformType)
	_ = platform.Set("go", runtime.Version())
	_ = platform.Set("os", runtime.GOOS)
	_ = platform.Set("arch", runtime.GOARCH)
	_ = platform.Set("cpus", runtime.NumCPU())
	platform.SetReadonly(true)
	_ = symbols.RootSymbolTable.SetAlways("_platform", platform)

	return runFromContext(app.Context)
}
