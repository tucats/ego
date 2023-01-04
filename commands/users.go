package commands

import (
	"net/http"

	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/tables"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/i18n"
	"github.com/tucats/ego/runtime"
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
	resp := defs.User{}

	err = runtime.Exchange(defs.AdminUsersPath, http.MethodPost, payload, &resp, defs.AdminAgent)
	if err == nil {
		if ui.OutputFormat == ui.TextFormat {
			ui.Say("msg.user.added", map[string]interface{}{
				"user": user,
			})
		} else {
			_ = commandOutput(resp)
		}
	}

	if err != nil {
		err = errors.EgoError(err)
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

	resp := defs.User{}
	url := runtime.URLBuilder(defs.AdminUsersNamePath, user)

	err = runtime.Exchange(url.String(), http.MethodDelete, nil, &resp, defs.AdminAgent)
	if err == nil {
		if ui.OutputFormat == ui.TextFormat {
			ui.Say("msg.user.deleted", map[string]interface{}{"user": user})
		} else {
			_ = commandOutput(resp)
		}
	}

	if err != nil {
		err = errors.EgoError(err)
	}

	return err
}

func ListUsers(c *cli.Context) error {
	var ud = defs.UserCollection{}

	err := runtime.Exchange(defs.AdminUsersPath, http.MethodGet, nil, &ud, defs.AdminAgent)
	if err != nil {
		return errors.EgoError(err)
	}

	if ui.OutputFormat == ui.TextFormat {
		t, _ := tables.New([]string{i18n.L("User"), i18n.L("ID"), i18n.L("Permissions")})

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

			_ = t.AddRowItems(u.Name, u.ID, perms)
		}

		_ = t.SortRows(0, true)
		t.SetPagination(0, 0)

		_ = t.Print(ui.TextFormat)
	} else {
		_ = commandOutput(ud)
	}

	if err != nil {
		err = errors.EgoError(err)
	}

	return err
}
