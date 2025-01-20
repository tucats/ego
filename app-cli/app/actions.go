package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/config"
	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/i18n"
	"github.com/tucats/ego/runtime/rest"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

func InsecureAction(c *cli.Context) error {
	rest.AllowInsecure(true)

	return nil
}

// OutputFormatAction sets the default output format to use. This must be one of
// the supported types "test"", "json"", or "indented").
func OutputFormatAction(c *cli.Context) error {
	if formatString, present := c.FindGlobal().String("format"); present {
		if !util.InList(strings.ToLower(formatString),
			ui.JSONIndentedFormat, ui.JSONFormat, ui.TextFormat) {
			return errors.ErrInvalidOutputFormat.Context(formatString)
		}

		ui.OutputFormat = strings.ToLower(formatString)
		settings.SetDefault(defs.OutputFormatSetting, ui.OutputFormat)
	}

	return nil
}

// LogFormatAction sets the default log format to use. This must be one of
// the supported types "test"", "json"", or "indented").
func LogFormatAction(c *cli.Context) error {
	if formatString, present := c.FindGlobal().String("log-format"); present {
		if !util.InList(strings.ToLower(formatString),
			ui.JSONIndentedFormat, ui.JSONFormat, ui.TextFormat) {
			return errors.ErrInvalidOutputFormat.Context(formatString)
		}

		ui.LogFormat = strings.ToLower(formatString)
		settings.SetDefault(defs.LogFormatSetting, ui.LogFormat)
	}

	return nil
}

// LanguageAction sets the default language to use. This must be one of the
// supported languages ("en", "es", "fr", "de", "it", "pt", "ru", "zh").
func LanguageAction(c *cli.Context) error {
	if language, ok := c.FindGlobal().String("language"); ok {
		i18n.Language = strings.ToLower(language)[0:2]
	}

	return nil
}

// LogAction is an action routine to set the loggers that will get debug messages
// during execution. This must be a string list, and each named logger is enabled.
// If a logger name is not valid, an error is returned.
func LogAction(c *cli.Context) error {
	if loggers, specified := c.FindGlobal().StringList("log"); specified {
		for _, v := range loggers {
			if name := strings.TrimSpace(v); name != "" {
				if logger := ui.LoggerByName(name); logger < 0 {
					return errors.ErrInvalidLoggerName.Context(name)
				} else {
					ui.Active(logger, true)
				}
			}
		}
	}

	return nil
}

// LogFileAction is an action routine to set the name of the output log file.
func LogFileAction(c *cli.Context) error {
	if logFile, specified := c.FindGlobal().String("log-file"); specified {
		return ui.OpenLogFile(logFile, false)
	}

	return nil
}

// EnvAction is an action routine in the global grammar that tells the application
// to load the configuration values from environment variables if present.
func EnvAction(c *cli.Context) error {
	count := loadEnvSettings()

	ui.Log(ui.AppLogger, "log.app.env.load", "count", count)

	return nil
}

// SetAction is the action routine for the "--set" option which sets configuration
// values for this execution of Ego.
func SetAction(c *cli.Context) error {
	items, _ := c.StringList("set")
	for _, item := range items {
		value := "true"

		if pos := strings.Index(item, "="); pos > 0 {
			value = item[pos+1:]
			item = item[:pos]
		}

		if err := config.ValidateKey(item); err != nil {
			ui.Log(ui.AppLogger, "log.app.set.invalid", "item", item, "error", err)

			return err
		}

		settings.SetDefault(item, value)
	}

	return nil
}

func MaxProcsAction(c *cli.Context) error {
	if maxProcs, present := c.FindGlobal().Integer("maxcpus"); present {
		if maxProcs > 1 {
			ui.Log(ui.AppLogger, "log.app.maxcpus", "count", maxProcs)

			runtime.GOMAXPROCS(maxProcs)

			// Force update the value in the _platform structure
			if platform, ok := symbols.RootSymbolTable.Get(defs.PlatformVariable); ok {
				if p, ok := platform.(*data.Struct); ok {
					p.SetAlways("cpus", maxProcs)
				}
			}
		} else {
			ui.Log(ui.AppLogger, "log.app.invalid.value", "item", "--maxcpus", "value", maxProcs)

			return errors.ErrInvalidInteger.Context(maxProcs)
		}
	}

	return nil
}

// QuietAction is an action routine to set the global debug status if specified.
func QuietAction(c *cli.Context) error {
	ui.QuietMode = c.FindGlobal().Boolean("quiet")

	return nil
}

// ArchiveFileAction sets the name of the archive file to use for log files
// that are eligible for purging.
func ArchiveLogFileAction(c *cli.Context) error {
	if archiveFile, specified := c.FindGlobal().String("archive-log"); specified {
		ui.SetArchive(archiveFile)
	}

	return nil
}

// VersionAction is the action routine when the version subcommand is given
// on the command line. This prints the version information and the app will
// exit (since this is a subcommand verb).
func VersionAction(c *cli.Context) error {
	arch := fmt.Sprintf("%s, %s", runtime.GOOS, runtime.GOARCH)
	if arch == "darwin, arm64" {
		arch = "Apple Silicon"
	}

	if ui.OutputFormat == ui.TextFormat {
		fmt.Printf("%s %s %s (%s, %s)\n",
			c.FindGlobal().AppName,
			i18n.L("version"),
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
			File      string `json:"file"`
		}

		fullPath, _ := filepath.Abs(os.Args[0])
		v := VersionInfo{
			Name:      c.FindGlobal().AppName,
			Version:   c.FindGlobal().Version,
			GoVersion: runtime.Version(),
			OS:        runtime.GOOS,
			Arch:      runtime.GOARCH,
			File:      fullPath,
		}

		if ui.OutputFormat == ui.JSONFormat {
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
func UseProfileAction(c *cli.Context) error {
	name, _ := c.String("profile")
	settings.UseProfile(name)

	ui.Log(ui.AppLogger, "log.app.using.profile", "name", name)
	settings.Load(c.AppName, name)

	return nil
}

// ShowVersionAction is the action routine called when --version is specified.
// It prints the version number information and then exits the application if
// there are no additional subcommands on the command line.
func ShowVersionAction(c *cli.Context) error {
	fmt.Printf("%s %s\n", c.MainProgram, c.Version)

	// If there are only two arguments in the copy of the arg list,
	// we are done and can exit. Otherwise, continue to process the
	// remaining arguments.
	if len(os.Args) == 2 {
		os.Exit(0)
	}

	return nil
}
