package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-resty/resty"
	"github.com/tucats/gopackages/app-cli/cli"
	"github.com/tucats/gopackages/app-cli/persistence"
	"github.com/tucats/gopackages/app-cli/ui"
)

const LogonEndpoint = "/services/logon"

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

	if c.GetBool("hash") {
		fmt.Printf("HASH %s\n", hashString(pass))
	}

	// Turn logon server into full URL
	url = strings.TrimSuffix(url, "/") + LogonEndpoint

	// Call the endpoint
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

	// IF there was an error condition, let's report it now.
	if err == nil {
		switch r.StatusCode() {
		case 401:
			err = errors.New("No credentials provided")
		case 403:
			err = errors.New("Invalid credentials")
		case 404:
			err = errors.New("Logon endpoint not found on server")
		default:
			err = fmt.Errorf("HTTP %d", r.StatusCode())
		}
	}

	return err
}
