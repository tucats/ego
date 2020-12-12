package main

import (
	"fmt"
	"strings"

	"github.com/go-resty/resty"
	"github.com/tucats/gopackages/app-cli/cli"
	"github.com/tucats/gopackages/app-cli/persistence"
	"github.com/tucats/gopackages/app-cli/ui"
)

// Logon handles the logon subcommand
func Logon(c *cli.Context) error {

	// Do we know where the logon server is?
	url := persistence.Get("logon-server")
	if c.WasFound("logon-server") {
		url, _ = c.GetString("logon-server")
		persistence.Set("logon-server", url)
	}
	if url == "" {
		return fmt.Errorf("No --logon-server specified")
	}

	user, _ := c.GetString("username")
	pass, _ := c.GetString("password")

	for user == "" {
		user = ui.Prompt("Username: ")
	}
	for pass == "" {
		pass = ui.PromptPassword("Password: ")
	}

	r, err := resty.New().SetDisableWarn(true).SetBasicAuth(user, pass).NewRequest().Get(url)
	if err == nil && r.StatusCode() == 200 {
		token := string(r.Body())
		if strings.HasSuffix(token, "\n") {
			token = token[:len(token)-1]
		}
		persistence.Set("logon-token", token)
		ui.Say("Successfully logged in as \"%s\"", user)
		return nil
	}

	return err
}
