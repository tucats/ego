package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/go-resty/resty"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/runtime"
	"github.com/tucats/gopackages/app-cli/cli"
	"github.com/tucats/gopackages/app-cli/persistence"
	"github.com/tucats/gopackages/app-cli/tables"
	"github.com/tucats/gopackages/app-cli/ui"
)

// AddUser is used to add a new user to the security database of the
// running server.
func AddUser(c *cli.Context) error {
	var err error
	user, _ := c.GetString("username")
	pass, _ := c.GetString("password")
	permissions, _ := c.GetStringList("permissions")

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
	resp := defs.UserReponse{}
	err = runtime.Exchange("/admin/users/", "POST", payload, &resp)
	if err == nil {
		if ui.OutputFormat == "text" {
			ui.Say(resp.Message)
		} else {
			var b []byte
			b, err = json.Marshal(resp)
			if err == nil {
				fmt.Printf("%s\n", string(b))
			}
		}
	}
	return err
}

// AddUser is used to add a new user to the security database of the
// running server.
func DeleteUser(c *cli.Context) error {
	var err error
	user, _ := c.GetString("username")

	for user == "" {
		user = ui.Prompt("Username: ")
	}
	resp := defs.UserReponse{}
	err = runtime.Exchange(fmt.Sprintf("/admin/users/%s", user), "DELETE", nil, &resp)
	if err == nil {
		if ui.OutputFormat == "text" {
			ui.Say(resp.Message)
		} else {
			var b []byte
			b, err = json.Marshal(resp)
			if err == nil {
				fmt.Printf("%s\n", string(b))
			}
		}
	}
	return err
}

func ListUsers(c *cli.Context) error {

	path := persistence.Get(defs.LogonServerSetting)
	if path == "" {
		path = "http://localhost:8080"
	}
	url := strings.TrimSuffix(path, "/") + "/admin/users/"

	client := resty.New().SetRedirectPolicy(resty.FlexibleRedirectPolicy(10))
	if token := persistence.Get(defs.LogonTokenSetting); token != "" {
		client.SetAuthScheme(defs.AuthScheme)
		client.SetAuthToken(token)
	}

	var err error
	var ud = defs.UserCollection{}

	r := client.NewRequest()
	r.Header.Add("Accepts", defs.JSONMediaType)
	var response *resty.Response
	response, err = r.Get(url)
	if response.StatusCode() == 404 && len(response.Body()) == 0 {
		err = errors.New(defs.NotFound)
	}
	status := response.StatusCode()
	if status == 403 {
		err = errors.New(defs.NoPrivilegeForOperation)
	}
	if err == nil {
		if status == 200 {
			body := string(response.Body())
			err = json.Unmarshal([]byte(body), &ud)
			if err == nil {
				switch ui.OutputFormat {
				case "text":
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
					_ = t.Print("text")

				case "json":
					fmt.Printf("%s\n", body)
				}
			}
		}
	}

	return err
}
