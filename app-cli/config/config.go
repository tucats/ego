// This package manages the "config" subcommand. It includes the grammar
// definition and the action routines for the config subcommands.
package config

import (
	"fmt"
	"strings"

	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/tables"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/i18n"
	"github.com/tucats/ego/util"
)

const maxKeyValuePrintWidth = 60

// ShowAction implements the "config show" subcommand. This displays the
// current contents of the active configuration.
func ShowAction(c *cli.Context) error {
	// Is the user asking for a single value?
	if c.ParameterCount() > 0 {
		key := c.Parameter(0)
		if !settings.Exists(key) {
			return errors.ErrNoSuchProfileKey.Context(key)
		}

		fmt.Println(settings.Get(key))

		return nil
	}

	t, _ := tables.New([]string{i18n.L("Key"), i18n.L("Value")})

	for k, v := range settings.CurrentConfiguration.Items {
		if len(fmt.Sprintf("%v", v)) > maxKeyValuePrintWidth {
			v = fmt.Sprintf("%v", v)[:maxKeyValuePrintWidth] + "..."
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
			map[string]interface{}{
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
	// Generic --key and --value specification.
	key := c.Parameter(0)
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

	msg := i18n.M("config.written", map[string]interface{}{"key": key})

	ui.Say("%s", msg)

	return nil
}

// DeleteAction implements the "config delete" subcommand This deletes a
// named key value from the active configuration.
func DeleteAction(c *cli.Context) error {
	var err error

	key := c.Parameter(0)

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

	return nil
}

// DeleteProfileAction implements the "config remove" actcion. This
// deletes a named configuration.
func DeleteProfileAction(c *cli.Context) error {
	name := c.Parameter(0)

	err := settings.DeleteProfile(name)
	if err == nil {
		ui.Say("%s", i18n.M("config.deleted", map[string]interface{}{"name": name}))

		return nil
	}

	return err
}

// SetDescriptionAction sets the configuration's description string.
func SetDescriptionAction(c *cli.Context) error {
	config := settings.Configurations[settings.ProfileName]
	config.Description = c.Parameter(0)
	config.Dirty = true
	settings.Configurations[settings.ProfileName] = config

	return nil
}

func DescribeAction(c *cli.Context) error {
	ui.Say("Active configuration: %s", settings.ProfileName)

	t, _ := tables.New([]string{i18n.L("Key"), i18n.L("Present"), i18n.L("Description")})

	for key := range defs.ValidSettings {
		msg := "config." + key

		desc := i18n.T(msg)
		if desc == msg {
			continue
		}

		_ = t.AddRowItems(key, settings.Exists(key), desc)
	}

	_ = t.SortRows(0, true)
	t.ShowHeadings(true).SetPagination(0, 0).RowLimit(-1).ShowUnderlines(true).Print(ui.TextFormat)

	return nil
}
