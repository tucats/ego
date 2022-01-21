package commands

import (
	"encoding/json"
	"fmt"
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
			var b []byte

			b, err = json.Marshal(resp)
			if errors.Nil(err) {
				fmt.Printf("%s\n", string(b))
			}
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
			var b []byte

			b, err = json.Marshal(resp)
			if errors.Nil(err) {
				fmt.Printf("%s\n", string(b))
			}
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

	switch ui.OutputFormat {
	case ui.TextFormat:
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
		_ = t.Print(ui.TextFormat)

	case ui.JSONFormat:
		b, _ := json.Marshal(ud)
		fmt.Printf("%s\n", string(b))

	case ui.JSONIndentedFormat:
		b, _ := json.MarshalIndent(ud, ui.JSONIndentPrefix, ui.JSONIndentSpacer)
		fmt.Printf("%s\n", string(b))
	}

	return errors.New(err)
}
