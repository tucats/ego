package commands

import (
	"net/http"

	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/tables"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/i18n"
	"github.com/tucats/ego/runtime/rest"
)

// AddUser is used to add a new user to the security database of the
// running server.
func AddUser(c *cli.Context) error {
	var err error

	user, _ := c.String("username")
	pass, passSpecified := c.String("password")
	permissions, _ := c.StringList("permissions")

	if c.ParameterCount() == 1 {
		if user == "" {
			user = c.Parameter(0)
		} else {
			return errors.ErrUnexpectedParameters
		}
	}

	for user == "" {
		user = ui.Prompt(i18n.L("username.prompt"))
	}

	// If the user didn't specify a password on the command line, prompt for
	// it now. You can specify an empty password only by using the command
	// line option.
	if !passSpecified {
		for pass == "" {
			pass = ui.PromptPassword(i18n.L("password.prompt"))
		}
	}

	payload := defs.User{
		Name:        user,
		Password:    pass,
		Permissions: permissions,
	}
	resp := defs.UserResponse{}

	err = rest.Exchange(defs.AdminUsersPath, http.MethodPost, payload, &resp, defs.AdminAgent, defs.UserMediaType)
	if err == nil {
		displayUser(c, &resp.User, "created")
	} else {
		err = errors.New(err)
	}

	return err
}

// Update is used to modify an existing user on the server.
func UpdateUser(c *cli.Context) error {
	var err error

	user, _ := c.String("username")
	pass, _ := c.String("password")
	permissions, _ := c.StringList("permissions")

	if c.ParameterCount() == 1 {
		if user == "" {
			user = c.Parameter(0)
		} else {
			return errors.ErrUnexpectedParameters
		}
	}

	for user == "" {
		user = ui.Prompt(i18n.L("username.prompt"))
	}

	payload := defs.User{
		Name:        user,
		Password:    pass,
		Permissions: permissions,
	}
	resp := defs.UserResponse{}

	err = rest.Exchange(defs.AdminUsersPath+user, http.MethodPatch, payload, &resp, defs.AdminAgent, defs.UserMediaType)
	if err == nil {
		displayUser(c, &resp.User, "updated")
	} else {
		err = errors.New(err)
	}

	return err
}

// Show is used to fetch and display the user information for a single user.
func ShowUser(c *cli.Context) error {
	var err error

	user, _ := c.String("username")

	if c.ParameterCount() == 1 {
		if user == "" {
			user = c.Parameter(0)
		} else {
			return errors.ErrUnexpectedParameters
		}
	}

	for user == "" {
		user = ui.Prompt(i18n.L("username.prompt"))
	}

	resp := defs.UserResponse{}

	err = rest.Exchange(defs.AdminUsersPath+user, http.MethodGet, nil, &resp, defs.AdminAgent, defs.UserMediaType)
	if err == nil {
		displayUser(c, &resp.User, "")
	} else {
		err = errors.New(err)
	}

	return err
}

// AddUser is used to add a new user to the security database of the
// running server.
func DeleteUser(c *cli.Context) error {
	var err error

	user, _ := c.String("username")

	if c.ParameterCount() == 1 {
		if user == "" {
			user = c.Parameter(0)
		} else {
			return errors.ErrUnexpectedParameters
		}
	}

	for user == "" {
		user = ui.Prompt("Username: ")
	}

	resp := defs.UserResponse{}
	url := rest.URLBuilder(defs.AdminUsersNamePath, user)

	err = rest.Exchange(url.String(), http.MethodDelete, nil, &resp, defs.AdminAgent, defs.UserMediaType)
	if err == nil {
		if ui.OutputFormat == ui.TextFormat {
			ui.Say("msg.user.deleted", map[string]interface{}{"user": user})
		} else {
			_ = c.Output(resp)
		}
	}

	if err != nil {
		err = errors.New(err)
	}

	return err
}

func ListUsers(c *cli.Context) error {
	var userCollection = defs.UserCollection{}

	err := rest.Exchange(defs.AdminUsersPath, http.MethodGet, nil, &userCollection, defs.AdminAgent, defs.UsersMediaType)
	if err != nil {
		return errors.New(err)
	}

	if ui.OutputFormat == ui.TextFormat {
		return formatUserCollectionAsText(c, userCollection)
	} else {
		_ = c.Output(userCollection)
	}

	return err
}

func formatUserCollectionAsText(c *cli.Context, ud defs.UserCollection) error {
	var headings []string

	showID := c.Boolean("id")

	if showID {
		headings = []string{i18n.L("User"), i18n.L("ID"), i18n.L("Permissions")}
	} else {
		headings = []string{i18n.L("User"), i18n.L("Permissions")}
	}

	t, err := tables.New(headings)
	if err != nil {
		return err
	}

	for _, u := range ud.Items {
		perms := ""

		for i, p := range u.Permissions {
			if i > 0 {
				perms = perms + ", "
			}

			perms = perms + p
		}

		if perms == "" {
			perms = "."
		}

		if showID {
			if err = t.AddRowItems(u.Name, u.ID, perms); err != nil {
				return err
			}
		} else {
			if err = t.AddRowItems(u.Name, perms); err != nil {
				return err
			}
		}
	}

	if err = t.SortRows(0, true); err != nil {
		return err
	}

	t.SetPagination(0, 0)

	return t.Print(ui.TextFormat)
}

// Display a single user (with an action word) to show the result
// of an update, create, etc. of a user.
func displayUser(c *cli.Context, user *defs.User, action string) {
	if ui.OutputFormat == ui.TextFormat {
		if action != "" {
			ui.Say("msg.user.show", map[string]interface{}{
				"action": action,
				"user":   user.Name,
			})
		}

		t, _ := tables.New([]string{i18n.L("Field"), i18n.L("Value")})

		pwString := "Enabled"
		if user.Password == "" {
			pwString = "Disabled"
		}

		permString := data.Format(user.Permissions)
		if len(user.Permissions) == 0 {
			permString = "No permissions"
		}

		_ = t.AddRowItems("Name", user.Name)
		_ = t.AddRowItems("ID", user.ID)
		_ = t.AddRowItems("Permissions", permString)
		_ = t.AddRowItems("Password", pwString)

		t.Print(ui.TextFormat)
	} else {
		_ = c.Output(user)
	}
}
