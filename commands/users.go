package commands

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-resty/resty"
	"github.com/tucats/ego/reps"
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

	path := persistence.Get("logon-server")
	if path == "" {
		path = "http://localhost:8080"
	}
	url := strings.TrimSuffix(path, "/") + "/admin/user"

	payload := map[string]interface{}{
		"name":        user,
		"password":    pass,
		"permissions": permissions,
	}
	b, err := json.Marshal(payload)

	if err == nil {
		client := resty.New().SetRedirectPolicy(resty.FlexibleRedirectPolicy(10))
		if token := persistence.Get("logon-token"); token != "" {
			client.SetAuthScheme("Token")
			client.SetAuthToken(token)
		}
		r := client.NewRequest()
		r.Header.Add("Accepts", "application/json")
		r.SetBody(string(b))
		var response *resty.Response
		response, err = r.Post(url)
		if response.StatusCode() == 404 && len(response.Body()) == 0 {
			err = fmt.Errorf("%d %s", 404, "not found")
		}
		if response.StatusCode() == 403 {
			err = fmt.Errorf("You do not have permission to delete a user")
		}
		if err == nil {
			status := RestStatus{}
			body := string(response.Body())
			err = json.Unmarshal([]byte(body), &status)
			if err == nil {
				if status.Status < 200 || status.Status > 299 {
					err = fmt.Errorf("%d %s", status.Status, status.Msg)
				} else {
					ui.Say(status.Msg)
				}
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

	path := persistence.Get("logon-server")
	if path == "" {
		path = "http://localhost:8080"
	}
	url := strings.TrimSuffix(path, "/") + "/admin/user"

	payload := map[string]interface{}{
		"name": user,
	}

	b, err := json.Marshal(payload)
	if err == nil {
		client := resty.New().SetRedirectPolicy(resty.FlexibleRedirectPolicy(10))
		if token := persistence.Get("logon-token"); token != "" {
			client.SetAuthScheme("Token")
			client.SetAuthToken(token)
		}
		r := client.NewRequest()
		r.Header.Add("Accepts", "application/json")
		r.SetBody(string(b))
		var response *resty.Response
		response, err = r.Delete(url)
		if response.StatusCode() == 404 && len(response.Body()) == 0 {
			err = fmt.Errorf("%d %s", 404, "not found")
		}
		if response.StatusCode() == 403 {
			err = fmt.Errorf("You do not have permission to for this operation user")
		}
		if err == nil {
			status := RestStatus{}
			body := string(response.Body())
			err = json.Unmarshal([]byte(body), &status)
			if err == nil {
				if status.Status < 200 || status.Status > 299 {
					err = fmt.Errorf("%d %s", status.Status, status.Msg)
				} else {
					ui.Say(status.Msg)
				}
			}
		}
	}
	return err
}

func ListUsers(c *cli.Context) error {

	path := persistence.Get("logon-server")
	if path == "" {
		path = "http://localhost:8080"
	}
	url := strings.TrimSuffix(path, "/") + "/admin/users"

	client := resty.New().SetRedirectPolicy(resty.FlexibleRedirectPolicy(10))
	if token := persistence.Get("logon-token"); token != "" {
		client.SetAuthScheme("Token")
		client.SetAuthToken(token)
	}

	var err error
	var ud = reps.UserCollection{}

	r := client.NewRequest()
	r.Header.Add("Accepts", "application/json")
	var response *resty.Response
	response, err = r.Get(url)
	if response.StatusCode() == 404 && len(response.Body()) == 0 {
		err = fmt.Errorf("%d %s", 404, "not found")
	}
	status := response.StatusCode()
	if status == 403 {
		err = fmt.Errorf("You do not have permission to list users")
	}
	if err == nil {
		if status == 200 {
			body := string(response.Body())
			err = json.Unmarshal([]byte(body), &ud)
			if err == nil {
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
				_ = t.Print(ui.OutputFormat)
			}
		}
	}

	return err
}
