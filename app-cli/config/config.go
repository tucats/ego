// This package manages the "config" subcommand. It includes the grammar
// definition and the action routines for the config subcommands.
package config

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/tables"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/egostrings"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/i18n"
	"github.com/tucats/ego/util"
)

const maxKeyValuePrintWidth = 60

// ShowAction implements the "config show" subcommand. This displays the
// current contents of the active configuration.
func ShowAction(c *cli.Context) error {
	verbose := c.Boolean(defs.VerboseOption)

	// Is the user asking for a single value?
	if c.ParameterCount() > 0 {
		lines := []string{}

		for i := range c.ParameterCount() {
			key := c.Parameter(i)
			// Is this a case of including a value for the key, which makes this an attempt
			// to set the value?
			if strings.Contains(key, "=") {
				parts := strings.Split(key, "=")
				// There can be only a single "=" and therefore two parts to the key string.
				if len(parts) != 2 {
					return errors.ErrInvalidConfigName.Context(key)
				}

				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])

				// Sanity check -- if it is a privileged setting, is it valid?
				if invalidKeyError := ValidateKey(key); invalidKeyError != nil {
					return invalidKeyError
				}

				settings.Set(key, value)

				msg := i18n.M("config.written", map[string]any{"key": key, "value": value})

				ui.Say("%s", msg)

				return nil
			}

			// Check if the key exists. If not, return an error.
			if !settings.Exists(key) {
				return errors.ErrNoSuchProfileKey.Context(key)
			}

			lines = append(lines, settings.Get(key))
		}

		ui.Say("%s", strings.Join(lines, "\n"))

		return nil
	}

	// No list of keys to print provided, so build a table to show them all.
	t, _ := tables.New([]string{i18n.L("Key"), i18n.L("Value")})

	for k, v := range settings.CurrentConfiguration.Items {
		// if this is the token, show only the start and end of the string.
		if !verbose {
			if (k == defs.LogonTokenSetting || k == defs.ServerTokenKeySetting) && len(v) > 8 {
				v = egostrings.TruncateMiddle(v, 10)
			} else if len(v) > maxKeyValuePrintWidth {
				v = fmt.Sprintf("%v", v)[:maxKeyValuePrintWidth] + "..."
			}
		}

		_ = t.AddRowItems(k, v)
	}

	// Pagination makes no sense in this context.
	t.SetPagination(0, 0)

	_ = t.SetOrderBy(i18n.L("Key"))
	t.ShowUnderlines(true)
	t.Print(ui.TextFormat)

	if c.Boolean("version") {
		ui.Say("\n" + i18n.M("config.version",
			map[string]any{
				"version": settings.CurrentConfiguration.Version}),
		)
	}

	return nil
}

// ListAction implements the "config list" subcommand. This displays the
// list of configuration names. If the --version flag is present, the version
// number of the configuration is printed as well.
func ListAction(c *cli.Context) error {
	var (
		t       *tables.Table
		version = c.Boolean("version")
	)

	if version {
		t, _ = tables.New([]string{i18n.L("Name"), i18n.L("Version"), i18n.L("Description")})
	} else {
		t, _ = tables.New([]string{i18n.L("Name"), i18n.L("Description")})
	}

	for k, v := range settings.Configurations {
		if version {
			_ = t.AddRowItems(k, v.Version, v.Description)
		} else {
			_ = t.AddRowItems(k, v.Description)
		}
	}

	// Pagination makes no sense here.
	t.SetPagination(0, 0)

	_ = t.SetOrderBy("name")
	t.ShowUnderlines(true)
	t.Print(ui.TextFormat)

	return nil
}

// SetOutputAction is the action handler for the "config set-output" subcommand.
func SetOutputAction(c *cli.Context) error {
	if c.ParameterCount() == 1 {
		outputType := c.Parameter(0)
		if util.InList(outputType,
			ui.TextFormat,
			ui.JSONFormat,
			ui.JSONIndentedFormat) {
			settings.Set(defs.OutputFormatSetting, outputType)

			return nil
		}

		return errors.ErrInvalidOutputFormat.Context(outputType)
	}

	return errors.ErrMissingOutputType
}

// SetAction implements the "config set" subcommand. This uses the first
// two parameters as a key and value. If the key has an "=" in it, then
// the value is assumed to be the string after the "=".
func SetAction(c *cli.Context) error {
	for idx := range c.ParameterCount() {
		key := c.Parameter(idx)
		value := defs.True

		if equals := strings.Index(key, "="); equals >= 0 {
			value = key[equals+1:]
			key = key[:equals]
		}

		// Sanity check -- if it is a privileged setting, is it valid?
		if invalidKeyError := ValidateKey(key); invalidKeyError != nil {
			return invalidKeyError
		}

		settings.Set(key, value)

		msg := i18n.M("config.written", map[string]any{
			"key":   key,
			"value": value,
		})

		ui.Say("%s", msg)
	}

	return nil
}

// DeleteAction implements the "config delete" subcommand This deletes a
// named key value from the active configuration.
func DeleteAction(c *cli.Context) error {
	var err error

	for idx := range c.ParameterCount() {
		key := c.Parameter(idx)

		// Sanity check -- if it is a privileged setting, is it valid?
		if err = ValidateKey(key); err != nil {
			return err
		}

		if err = settings.Delete(key); err != nil {
			if c.Boolean("force") {
				err = nil
			}

			return err
		}

		ui.Say("Profile key %s deleted", key)
	}

	return nil
}

// DeleteProfileAction implements the "config remove" action. This
// deletes a named configuration.
func DeleteProfileAction(c *cli.Context) error {
	name := c.Parameter(0)

	err := settings.DeleteProfile(name)
	if err == nil {
		ui.Say("%s", i18n.M("config.deleted", map[string]any{"name": name}))

		return nil
	}

	return err
}

// SetDescriptionAction sets the configuration's description string.
func SetDescriptionAction(c *cli.Context) error {
	config := settings.Configurations[settings.ProfileName]
	if config == nil {
		return errors.ErrNoSuchProfile.Context(settings.ProfileName)
	}

	if c.ParameterCount() == 0 {
		return errors.ErrWrongParameterCount
	}

	config.Description = c.Parameter(0)
	config.Dirty = true
	settings.Configurations[settings.ProfileName] = config

	return nil
}

func DescribeAction(c *cli.Context) error {
	verbose := c.Boolean(defs.VerboseOption)

	ui.Say("Active configuration: %s", settings.ProfileName)

	t, _ := tables.New([]string{i18n.L("Key"), i18n.L("Value"), i18n.L("Description")})

	for key := range defs.ValidSettings {
		msg := "config." + key

		desc := i18n.T(msg)
		if desc == msg {
			desc = "-- Need description for key: " + msg
		}

		value := settings.Get(key)
		if value == "" && !verbose {
			continue
		}

		if (key == defs.LogonTokenSetting || key == defs.ServerTokenKeySetting) && len(value) > 8 {
			value = fmt.Sprintf("%s...%s", value[:4], value[len(value)-4:])
		} else if len(value) > maxKeyValuePrintWidth {
			if strings.Count(value, string(filepath.Separator)) > 2 {
				value = shortenPath(value, maxKeyValuePrintWidth)
			} else {
				value = fmt.Sprintf("%v", value[:maxKeyValuePrintWidth]) + "..."
			}
		}

		_ = t.AddRowItems(key, value, desc)
	}

	_ = t.SortRows(0, true)
	t.ShowHeadings(true).SetPagination(0, 0).RowLimit(-1).ShowUnderlines(true).Print(ui.TextFormat)

	return nil
}

// shortenPath shortens a long path name by eliding out middle parts of the path.
func shortenPath(path string, maxLen int) string {
	if len(path) <= maxLen {
		return path
	}

	sep := string(filepath.Separator)
	firstSeparator := ""

	if strings.HasPrefix(path, sep) {
		firstSeparator = sep
	}

	// Split into segments using the path separator.
	parts := strings.Split(path, sep)
	lastPart := parts[len(parts)-1]
	dots := "..."

	// If the last part in the path is already too long,
	// shorten it and return that value.
	if len(lastPart) > maxLen {
		pos := len(lastPart) - maxLen - len(dots)
		shortPath := dots + lastPart[pos:]

		return shortPath
	}

	front := 0
	back := len(parts) - 1
	size := 0

	for i := 0; i < len(parts); i++ {
		if front+back > len(parts) {
			break
		}

		// Alternate between end and start of elements, starting with the end
		// path element.
		if i%2 == 1 {
			size += len(parts[i])
			if size+2*len(sep)+len(dots)+len(firstSeparator) > maxLen {
				break
			}

			front++
		} else {
			size += len(parts[len(parts)-i-1])
			if size+2*len(sep)+len(dots)+len(firstSeparator) > maxLen {
				break
			}

			back--
		}
	}

	if back <= front {
		back = front + 1
	}

	elements := make([]string, 0)
	for _, part := range parts[:front] {
		elements = append(elements, part)
	}

	elements = append(elements, dots)

	for _, part := range parts[back:] {
		elements = append(elements, part)
	}

	shortPath := firstSeparator + filepath.Join(elements...)

	return shortPath
}
