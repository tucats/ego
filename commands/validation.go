package commands

import (
	"encoding/json"
	"net/http"

	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/runtime/rest"
)

func ServerValidations(c *cli.Context) error {
	var (
		b        []byte
		err      error
		response map[string]interface{}
		path     string
		method   string
		entry    string
	)

	url := defs.AdminValidationPath

	if c.WasFound("path") {
		path, _ = c.String("path")
	}

	if c.WasFound("method") {
		method, _ = c.String("method")
	}

	if c.WasFound("entry") {
		entry, _ = c.String("entry")
	}

	if c.Boolean("all") {
		// no action needed, default is all items
	} else if entry > "" {
		url = url + "?entry=" + entry
	} else {
		if method == "" {
			method = http.MethodPost
		}

		url = url + "?method=" + method + "&path=" + path
	}

	err = rest.Exchange(url, http.MethodGet, nil, &response, defs.AdminAgent)

	if err == nil {
		b, err = json.MarshalIndent(response, ui.JSONIndentPrefix, ui.JSONIndentSpacer)
		if err == nil {
			ui.Say(string(b))
		}
	}

	return err
}
