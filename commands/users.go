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
	pass, _ := c.String("password")
	permissions, _ := c.StringList("permissions")

	for user == "" {
		user = ui.Prompt(i18n.L("username.prompt"))
	}

	for pass == "" {
		pass = ui.PromptPassword(i18n.L("password.prompt"))
	}

	payload := defs.User{
		Name:        user,
		Password:    pass,
		Permissions: permissions,
	}
	resp := defs.UserResponse{}

	err = rest.Exchange(defs.AdminUsersPath, http.MethodPost, payload, &resp, defs.AdminAgent, defs.UserMediaType)
	if err == nil {
		if ui.OutputFormat == ui.TextFormat {
			ui.Say("msg.user.added", map[string]interface{}{
				"user": resp.User.Name,
			})
		} else {
			_ = commandOutput(resp)
		}
	} else {
		err = errors.NewError(err)
	}

	return err
}

// Update is used to modify an existing user on the server.
func UpdateUser(c *cli.Context) error {
	var err error

	user, _ := c.String("username")
	pass, _ := c.String("password")
	permissions, _ := c.StringList("permissions")

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
		if ui.OutputFormat == ui.TextFormat {
			// Currently, the only thing we can say is the permissions list.
			if len(resp.User.Permissions) == 0 {
				ui.Say("msg.user.show.noperms", map[string]interface{}{
					"user": resp.User.Name,
				})
			} else {
				ui.Say("msg.user.show", map[string]interface{}{
					"user":        resp.User.Name,
					"permissions": data.Format(resp.User.Permissions),
				})
			}
		} else {
			_ = commandOutput(resp)
		}
	} else {
		err = errors.NewError(err)
	}

	return err
}

// Show is used to fetch and display the user information for a single user.
func ShowUser(c *cli.Context) error {
	var err error

	user, _ := c.String("username")

	for user == "" {
		user = ui.Prompt(i18n.L("username.prompt"))
	}

	resp := defs.UserResponse{}

	err = rest.Exchange(defs.AdminUsersPath+user, http.MethodGet, nil, &resp, defs.AdminAgent, defs.UserMediaType)
	if err == nil {
		if ui.OutputFormat == ui.TextFormat {
			// Currently, the only thing we can say is the permissions list.
			if len(resp.User.Permissions) == 0 {
				ui.Say("msg.user.show.noperms", map[string]interface{}{
					"user": resp.User.Name,
				})
			} else {
				ui.Say("msg.user.show", map[string]interface{}{
					"user":        resp.User.Name,
					"permissions": data.Format(resp.User.Permissions),
				})
			}
		} else {
			_ = commandOutput(resp)
		}
	} else {
		err = errors.NewError(err)
	}

	return err
}

// AddUser is used to add a new user to the security database of the
// running server.
func DeleteUser(c *cli.Context) error {
	var err error

	user, _ := c.String("username")

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
			_ = commandOutput(resp)
		}
	}

	if err != nil {
		err = errors.NewError(err)
	}

	return err
}

func ListUsers(c *cli.Context) error {
	var ud = defs.UserCollection{}

	err := rest.Exchange(defs.AdminUsersPath, http.MethodGet, nil, &ud, defs.AdminAgent, defs.UsersMediaType)
	if err != nil {
		return errors.NewError(err)
	}

	if ui.OutputFormat == ui.TextFormat {
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

		if err = t.Print(ui.TextFormat); err != nil {
			return err
		}
	} else {
		_ = commandOutput(ud)
	}

	if err != nil {
		err = errors.NewError(err)
	}

	return err
}
