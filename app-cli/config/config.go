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
	"github.com/tucats/ego/util"
)

const maxKeyValuePrintWidth = 60

// Grammar describes profile subcommands.
var Grammar = []cli.Option{
	{
		LongName:    "list",
		Description: "List all configurations",
		Action:      ListAction,
		OptionType:  cli.Subcommand,
	},
	{
		LongName:             "show",
		Description:          "Show the current configuration",
		Action:               ShowAction,
		ParameterDescription: "key",
		ParametersExpected:   -1,
		OptionType:           cli.Subcommand,
	},
	{
		LongName:             "set-output",
		OptionType:           cli.Subcommand,
		Description:          "Set the default output type (text or json)",
		ParameterDescription: "type",
		Action:               SetOutputAction,
		ParametersExpected:   1,
	},
	{
		LongName:             "set-description",
		OptionType:           cli.Subcommand,
		Description:          "Set the configuration description",
		ParameterDescription: "text",
		ParametersExpected:   1,
		Action:               SetDescriptionAction,
	},
	{
		LongName:             "delete",
		Aliases:              []string{"unset"},
		OptionType:           cli.Subcommand,
		Description:          "Delete a key from the configuration",
		Action:               DeleteAction,
		ParametersExpected:   1,
		ParameterDescription: "key",
		Value: []cli.Option{
			{
				LongName:    "force",
				ShortName:   "f",
				OptionType:  cli.BooleanType,
				Description: "Do not signal error if option not found",
			},
		},
	},
	{
		LongName:             "remove",
		OptionType:           cli.Subcommand,
		Description:          "Delete an entire configuration",
		Action:               DeleteProfileAction,
		ParametersExpected:   1,
		ParameterDescription: "name",
	},
	{
		LongName:             "set",
		Description:          "Set a configuration value",
		Action:               SetAction,
		OptionType:           cli.Subcommand,
		ParametersExpected:   1,
		ParameterDescription: "key=value",
	},
}

// ShowAction Displays the current contents of the active configuration.
func ShowAction(c *cli.Context) *errors.EgoError {
	// Is the user asking for a single value?
	if c.GetParameterCount() > 0 {
		key := c.GetParameter(0)
		if !settings.Exists(key) {
			return errors.New(errors.ErrNoSuchProfileKey).Context(key)
		}

		fmt.Println(settings.Get(key))

		return nil
	}

	t, _ := tables.New([]string{"Key", "Value"})

	for k, v := range settings.CurrentConfiguration.Items {
		if len(fmt.Sprintf("%v", v)) > maxKeyValuePrintWidth {
			v = fmt.Sprintf("%v", v)[:maxKeyValuePrintWidth] + "..."
		}

		_ = t.AddRowItems(k, v)
	}

	_ = t.SetOrderBy("key")
	t.ShowUnderlines(false)
	t.Print(ui.TextFormat)

	return nil
}

// ListAction Displays the current contents of the active configuration.
func ListAction(c *cli.Context) *errors.EgoError {
	t, _ := tables.New([]string{"Name", "Description"})

	for k, v := range settings.Configurations {
		_ = t.AddRowItems(k, v.Description)
	}

	_ = t.SetOrderBy("name")
	t.ShowUnderlines(false)
	t.Print(ui.TextFormat)

	return nil
}

// SetOutputAction is the action handler for the set-output subcommand.
func SetOutputAction(c *cli.Context) *errors.EgoError {
	if c.GetParameterCount() == 1 {
		outputType := c.GetParameter(0)
		if util.InList(outputType,
			ui.TextFormat,
			ui.JSONFormat,
			ui.JSONIndentedFormat) {
			settings.Set(defs.OutputFormatSetting, outputType)

			return nil
		}

		return errors.New(errors.ErrInvalidOutputFormat).Context(outputType)
	}

	return errors.New(errors.ErrMissingOutputType)
}

// SetAction uses the first two parameters as a key and value.
func SetAction(c *cli.Context) *errors.EgoError {
	// Generic --key and --value specification.
	key := c.GetParameter(0)
	value := "true"

	if equals := strings.Index(key, "="); equals >= 0 {
		value = key[equals+1:]
		key = key[:equals]
	}

	// Sanity check -- if it is a privileged setting, is it valid?
	if invalidKeyError := validateKey(key); !errors.Nil(invalidKeyError) {
		return invalidKeyError
	}

	settings.Set(key, value)
	ui.Say("Profile key %s written", key)

	return nil
}

// DeleteAction deletes a named key value.
func DeleteAction(c *cli.Context) *errors.EgoError {
	var err *errors.EgoError

	key := c.GetParameter(0)

	// Sanity check -- if it is a privileged setting, is it valid?
	if err = validateKey(key); !errors.Nil(err) {
		return err
	}

	if err = settings.Delete(key); !errors.Nil(err) {
		if c.Boolean("force") {
			err = nil
		}

		return err
	}

	ui.Say("Profile key %s deleted", key)

	return nil
}

// DeleteProfileAction deletes a named profile.
func DeleteProfileAction(c *cli.Context) *errors.EgoError {
	key := c.GetParameter(0)

	err := settings.DeleteProfile(key)
	if errors.Nil(err) {
		ui.Say("Profile %s deleted", key)

		return nil
	}

	return err
}

// SetDescriptionAction sets the profile description string.
func SetDescriptionAction(c *cli.Context) *errors.EgoError {
	config := settings.Configurations[settings.ProfileName]
	config.Description = c.GetParameter(0)
	settings.Configurations[settings.ProfileName] = config
	settings.ProfileDirty = true

	return nil
}

// Determine if a key is allowed to be updated by the CLI. This rule
// applies to keys with the privileged key prefix ("ego.").
func validateKey(key string) *errors.EgoError {
	if strings.HasPrefix(key, defs.PrivilegedKeyPrefix) {
		allowed, found := defs.ValidSettings[key]
		if !found {
			return errors.New(errors.ErrInvalidConfigName)
		}
		
		if !allowed {
			return errors.New(errors.ErrNoPrivilegeForOperation)
		}
	}

	return nil
}
