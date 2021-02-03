package profile

import (
	"fmt"
	"strings"

	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/persistence"
	"github.com/tucats/ego/app-cli/tables"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/util"
)

// Grammar describes profile subcommands
var Grammar = []cli.Option{
	{
		LongName:    "list",
		Description: "List all profiles",
		Action:      ListAction,
		OptionType:  cli.Subcommand,
	},
	{
		LongName:    "show",
		Description: "Show the current profile",
		Action:      ShowAction,
		OptionType:  cli.Subcommand,
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
		Description:          "Set the profile description",
		ParameterDescription: "text",
		ParametersExpected:   1,
		Action:               SetDescriptionAction,
	},
	{
		LongName:             "delete",
		Aliases:              []string{"unset"},
		OptionType:           cli.Subcommand,
		Description:          "Delete a key from the profile",
		Action:               DeleteAction,
		ParametersExpected:   1,
		ParameterDescription: "key",
	},
	{
		LongName:             "remove",
		OptionType:           cli.Subcommand,
		Description:          "Delete an entire profile",
		Action:               DeleteProfileAction,
		ParametersExpected:   1,
		ParameterDescription: "name",
	},
	{
		LongName:             "set",
		Description:          "Set a profile value",
		Action:               SetAction,
		OptionType:           cli.Subcommand,
		ParametersExpected:   1,
		ParameterDescription: "key=value",
	},
}

// ShowAction Displays the current contents of the active profile
func ShowAction(c *cli.Context) error {
	t, _ := tables.New([]string{"Key", "Value"})
	for k, v := range persistence.CurrentConfiguration.Items {
		if len(fmt.Sprintf("%v", v)) > 60 {
			v = fmt.Sprintf("%v", v)[:60] + "..."
		}
		_ = t.AddRowItems(k, v)
	}
	_ = t.SetOrderBy("key")
	t.ShowUnderlines(false)
	t.Print(ui.TextFormat)

	return nil
}

// ListAction Displays the current contents of the active profile
func ListAction(c *cli.Context) error {
	t, _ := tables.New([]string{"Name", "Description"})
	for k, v := range persistence.Configurations {
		_ = t.AddRowItems(k, v.Description)
	}
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
			persistence.Set("ego.output-format", outputType)

			return nil
		}

		return NewProfileErr(InvalidOutputError, outputType)
	}

	return NewProfileErr(MissingOutputTypeError)
}

// SetAction uses the first two parameters as a key and value
func SetAction(c *cli.Context) error {
	// Generic --key and --value specification
	key := c.GetParameter(0)
	value := "true"

	if equals := strings.Index(key, "="); equals >= 0 {
		value = key[equals+1:]
		key = key[:equals]
	}
	persistence.Set(key, value)
	ui.Say("Profile key %s written", key)

	return nil
}

// DeleteAction deletes a named key value
func DeleteAction(c *cli.Context) error {
	key := c.GetParameter(0)
	persistence.Delete(key)
	ui.Say("Profile key %s deleted", key)

	return nil
}

// DeleteProfileAction deletes a named profile.
func DeleteProfileAction(c *cli.Context) error {
	key := c.GetParameter(0)
	err := persistence.DeleteProfile(key)
	if err == nil {
		ui.Say("Profile %s deleted", key)

		return nil
	}

	return err
}

// SetDescriptionAction sets the profile description string
func SetDescriptionAction(c *cli.Context) error {
	config := persistence.Configurations[persistence.ProfileName]
	config.Description = c.GetParameter(0)
	persistence.Configurations[persistence.ProfileName] = config
	persistence.ProfileDirty = true

	return nil
}
