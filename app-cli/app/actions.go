package app

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/persistence"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/util"
)

// OutputFormatAction sets the default output format to use. This must be one of
// the supported types ("text", "json", or "indented").
func OutputFormatAction(c *cli.Context) *errors.EgoError {
	if formatString, present := c.FindGlobal().GetString("format"); present {
		if util.InList(strings.ToLower(formatString),
			ui.JSONIndentedFormat, ui.JSONFormat, ui.TextFormat) {
			ui.OutputFormat = formatString
		} else {
			return errors.New(errors.ErrInvalidOutputFormat).Context(formatString)
		}

		persistence.SetDefault("ego.output-format", strings.ToLower(formatString))
	}

	return nil
}

// DebugAction is an action routine to set the loggers that will get debug messages
// during execution. This must be a string list, and each named logger is enabled.
// If a logger name is not valid, an error is returned.
func DebugAction(c *cli.Context) *errors.EgoError {
	loggers, specified := c.FindGlobal().GetStringList("debug")

	if specified {
		for _, v := range loggers {
			logger := ui.Logger(v)
			if logger < 0 {
				return errors.New(errors.ErrInvalidLoggerName).Context(v)
			}

			ui.SetLogger(logger, true)
		}
	}

	return nil
}

// QuietAction is an action routine to set the global debug status if specified.
func QuietAction(c *cli.Context) *errors.EgoError {
	ui.QuietMode = c.FindGlobal().GetBool("quiet")

	return nil
}

func VersionAction(c *cli.Context) *errors.EgoError {
	arch := fmt.Sprintf("%s, %s", runtime.GOOS, runtime.GOARCH)
	if arch == "darwin, arm64" {
		arch = "Apple Silicon"
	}

	if ui.OutputFormat == "text" {
		fmt.Printf("%s version %s (%s, %s)\n",
			c.FindGlobal().AppName,
			c.FindGlobal().Version,
			runtime.Version(),
			arch)
	} else {
		type VersionInfo struct {
			Name      string `json:"name"`
			Version   string `json:"version"`
			GoVersion string `json:"go"`
			OS        string `json:"os"`
			Arch      string `json:"arch"`
		}

		v := VersionInfo{
			Name:      c.FindGlobal().AppName,
			Version:   c.FindGlobal().Version,
			GoVersion: runtime.Version(),
			OS:        runtime.GOOS,
			Arch:      runtime.GOARCH,
		}
		if ui.OutputFormat == "json" {
			b, _ := json.Marshal(v)
			fmt.Println(string(b))
		} else {
			b, _ := json.MarshalIndent(v, "", "  ")
			fmt.Println(string(b))
		}
	}

	return nil
}

// UseProfileAction is the action routine when --profile is specified as a global
// option. Its string value is used as the name of the active profile.
func UseProfileAction(c *cli.Context) *errors.EgoError {
	name, _ := c.GetString("profile")
	persistence.UseProfile(name)

	ui.Debug(ui.AppLogger, "Using profile %s", name)

	return nil
}

// ShowVersionAction is the action routine called when --version is specified.
// It prints the version number information and then exits the application.
func ShowVersionAction(c *cli.Context) *errors.EgoError {
	fmt.Printf("%s %s\n", c.MainProgram, c.Version)
	os.Exit(0)

	return nil
}
