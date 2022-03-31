package commands

import (
	"net/http"

	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/tables"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/runtime"
)

// AddUser is used to add a new user to the security database of the
// running server.
func AddUser(c *cli.Context) *errors.EgoError {
	var err error

	user, _ := c.String("username")
	pass, _ := c.String("password")
	permissions, _ := c.StringList("permissions")

	for user == "" {
		user = ui.Prompt("Username: ")
	}

	for pass == "" {
		pass = ui.PromptPassword("Password: ")
	}

	payload := defs.User{
		Name:        user,
		Password:    pass,
		Permissions: permissions,
	}
	resp := defs.User{}

	err = runtime.Exchange(defs.AdminUsersPath, http.MethodPost, payload, &resp, defs.AdminAgent)
	if errors.Nil(err) {
		if ui.OutputFormat == ui.TextFormat {
			ui.Say("User %s added", user)
		} else {
			_ = commandOutput(resp)
		}
	}

	return errors.New(err)
}

// AddUser is used to add a new user to the security database of the
// running server.
func DeleteUser(c *cli.Context) *errors.EgoError {
	var err error

	user, _ := c.String("username")

	for user == "" {
		user = ui.Prompt("Username: ")
	}

	resp := defs.User{}
	url := runtime.URLBuilder(defs.AdminUsersNamePath, user)

	err = runtime.Exchange(url.String(), http.MethodDelete, nil, &resp, defs.AdminAgent)
	if errors.Nil(err) {
		if ui.OutputFormat == ui.TextFormat {
			ui.Say("User %s deleted", user)
		} else {
			_ = commandOutput(resp)
		}
	}

	return errors.New(err)
}

func ListUsers(c *cli.Context) *errors.EgoError {
	var ud = defs.UserCollection{}

	err := runtime.Exchange(defs.AdminUsersPath, http.MethodGet, nil, &ud, defs.AdminAgent)
	if !errors.Nil(err) {
		return errors.New(err)
	}

	if ui.OutputFormat == ui.TextFormat {
		t, _ := tables.New([]string{"User", "ID", "Permissions"})

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

	return errors.New(err)
}
