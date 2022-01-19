package commands

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/go-resty/resty"
	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/settings"
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
	resp := defs.UserResponse{}

	err = runtime.Exchange(defs.AdminUsersPath, http.MethodPost, payload, &resp, defs.AdminAgent)
	if errors.Nil(err) {
		if ui.OutputFormat == ui.TextFormat {
			ui.Say(resp.Message)
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

	resp := defs.UserResponse{}
	url := runtime.URLBuilder(defs.AdminUsersNamePath, user)

	err = runtime.Exchange(url.String(), http.MethodDelete, nil, &resp, defs.AdminAgent)
	if errors.Nil(err) {
		if ui.OutputFormat == ui.TextFormat {
			ui.Say(resp.Message)
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
	path := settings.Get(defs.LogonServerSetting)
	if path == "" {
		path = "http://localhost:8080"
	}

	url := strings.TrimSuffix(path, "/") + "/admin/users/"

	client := resty.New().SetRedirectPolicy(resty.FlexibleRedirectPolicy(runtime.MaxRedirectCount))
	if os.Getenv("EGO_INSECURE_CLIENT") == defs.True {
		client.SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true})
	}

	if token := settings.Get(defs.LogonTokenSetting); token != "" {
		client.SetAuthToken(token)
	}

	var err error

	var ud = defs.UserCollection{}

	var response *resty.Response

	r := client.NewRequest()

	r.Header.Add("Accepts", defs.JSONMediaType)

	response, err = r.Get(url)
	if response.StatusCode() == http.StatusNotFound && len(response.Body()) == 0 {
		err = errors.New(errors.ErrNotFound)
	}

	status := response.StatusCode()
	if status == http.StatusForbidden {
		err = errors.New(errors.ErrNoPrivilegeForOperation)
	}

	if errors.Nil(err) && status == http.StatusOK {
		body := string(response.Body())

		err = json.Unmarshal([]byte(body), &ud)
		if errors.Nil(err) {
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
				fmt.Printf("%s\n", body)

			case ui.JSONIndentedFormat:
				b, _ := json.MarshalIndent(ud, ui.JSONIndentPrefix, ui.JSONIndentSpacer)
				fmt.Printf("%s\n", string(b))
			}
		}
	}

	return errors.New(err)
}
