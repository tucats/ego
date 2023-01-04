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

// Grammar describes profile subcommands.
var Grammar = []cli.Option{
	{
		LongName:    "list",
		Description: "ego.config.list",
		Action:      ListAction,
		OptionType:  cli.Subcommand,
	},
	{
		LongName:             "show",
		Description:          "ego.config.show",
		Action:               ShowAction,
		ParameterDescription: "key",
		ParametersExpected:   -1,
		OptionType:           cli.Subcommand,
	},
	{
		LongName:             "set-output",
		OptionType:           cli.Subcommand,
		Description:          "ego.config.set.output",
		ParameterDescription: "type",
		Action:               SetOutputAction,
		ParametersExpected:   1,
	},
	{
		LongName:             "set-description",
		OptionType:           cli.Subcommand,
		Description:          "ego.config.set.description",
		ParameterDescription: "text",
		ParametersExpected:   1,
		Action:               SetDescriptionAction,
	},
	{
		LongName:             "delete",
		Aliases:              []string{"unset"},
		OptionType:           cli.Subcommand,
		Description:          "ego.config.delete",
		Action:               DeleteAction,
		ParametersExpected:   1,
		ParameterDescription: "parm.key",
		Value: []cli.Option{
			{
				LongName:    "force",
				ShortName:   "f",
				OptionType:  cli.BooleanType,
				Description: "opt.config.force",
			},
		},
	},
	{
		LongName:             "remove",
		OptionType:           cli.Subcommand,
		Description:          "ego.config.remove",
		Action:               DeleteProfileAction,
		ParametersExpected:   1,
		ParameterDescription: "parm.name",
	},
	{
		LongName:             "set",
		Description:          "ego.config.set",
		Action:               SetAction,
		OptionType:           cli.Subcommand,
		ParametersExpected:   1,
		ParameterDescription: "parm.config.key.value",
	},
}

// ShowAction Displays the current contents of the active configuration.
func ShowAction(c *cli.Context) error {
	// Is the user asking for a single value?
	if c.GetParameterCount() > 0 {
		key := c.GetParameter(0)
		if !settings.Exists(key) {
			return errors.EgoError(errors.ErrNoSuchProfileKey).Context(key)
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
	t.ShowUnderlines(false)
	t.Print(ui.TextFormat)

	return nil
}

// ListAction Displays the current contents of the active configuration.
func ListAction(c *cli.Context) error {
	t, _ := tables.New([]string{i18n.L("Name"), i18n.L("Description")})

	for k, v := range settings.Configurations {
		_ = t.AddRowItems(k, v.Description)
	}

	// Pagination makes no sense here.
	t.SetPagination(0, 0)

	_ = t.SetOrderBy("name")
	t.ShowUnderlines(false)
	t.Print(ui.TextFormat)

	return nil
}

// SetOutputAction is the action handler for the set-output subcommand.
func SetOutputAction(c *cli.Context) error {
	if c.GetParameterCount() == 1 {
		outputType := c.GetParameter(0)
		if util.InList(outputType,
			ui.TextFormat,
			ui.JSONFormat,
			ui.JSONIndentedFormat) {
			settings.Set(defs.OutputFormatSetting, outputType)

			return nil
		}

		return errors.EgoError(errors.ErrInvalidOutputFormat).Context(outputType)
	}

	return errors.EgoError(errors.ErrMissingOutputType)
}

// SetAction uses the first two parameters as a key and value.
func SetAction(c *cli.Context) error {
	// Generic --key and --value specification.
	key := c.GetParameter(0)
	value := defs.True

	if equals := strings.Index(key, "="); equals >= 0 {
		value = key[equals+1:]
		key = key[:equals]
	}

	// Sanity check -- if it is a privileged setting, is it valid?
	if invalidKeyError := validateKey(key); invalidKeyError != nil {
		return invalidKeyError
	}

	settings.Set(key, value)

	msg := i18n.M("config.written", map[string]interface{}{"key": key})

	ui.Say("%s", msg)

	return nil
}

// DeleteAction deletes a named key value.
func DeleteAction(c *cli.Context) error {
	var err error

	key := c.GetParameter(0)

	// Sanity check -- if it is a privileged setting, is it valid?
	if err = validateKey(key); err != nil {
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

// DeleteProfileAction deletes a named profile.
func DeleteProfileAction(c *cli.Context) error {
	name := c.GetParameter(0)

	err := settings.DeleteProfile(name)
	if err == nil {
		ui.Say("%s", i18n.M("config.deleted", map[string]interface{}{"name": name}))

		return nil
	}

	return err
}

// SetDescriptionAction sets the profile description string.
func SetDescriptionAction(c *cli.Context) error {
	config := settings.Configurations[settings.ProfileName]
	config.Description = c.GetParameter(0)
	settings.Configurations[settings.ProfileName] = config
	settings.ProfileDirty = true

	return nil
}

// Determine if a key is allowed to be updated by the CLI. This rule
// applies to keys with the privileged key prefix ("ego.").
func validateKey(key string) error {
	if strings.HasPrefix(key, defs.PrivilegedKeyPrefix) {
		allowed, found := defs.ValidSettings[key]
		if !found {
			return errors.EgoError(errors.ErrInvalidConfigName)
		}

		if !allowed {
			return errors.EgoError(errors.ErrNoPrivilegeForOperation)
		}
	}

	return nil
}
