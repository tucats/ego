package commands

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/runtime/rest"
)

func ServerValidations(c *cli.Context) error {
	var (
		b        []byte
		err      error
		response map[string]interface{}
		item     string
		path     bool
		method   string
		entry    bool
	)

	url := defs.AdminValidationPath

	if items := c.FindGlobal().Parameters; len(items) > 0 {
		if items[0] != "all" && items[0] != "*" {
			item = items[0]
		}
	}

	// Can't use --all and also specify an item name.
	if item != "" && c.Boolean("all") {
		return errors.ErrParameterConflict.Clone().Context("-all")
	}

	path = c.Boolean("path")
	entry = c.Boolean("entry")

	// If no type was explicitly given, infer from the item name prefix.
	if !path && !entry {
		if strings.HasPrefix(item, "/") {
			path = true
		} else if strings.HasPrefix(item, "@") {
			entry = true
		}
	}

	if c.WasFound("method") {
		method, _ = c.String("method")
	}

	// If there is no item name (we want all items) then the user cannot have
	// specified options implying a named item.
	if item == "" && (method != "" || path) {
		return errors.ErrMissingEndPoint
	}

	if item == "" && entry {
		return errors.ErrMissingItem
	}

	// If it's one of the types, add the type and name to the URL. If neither is true,
	// we just dump the entire dictionary.
	if entry {
		url = url + "?entry=" + item
	} else if path == true {
		if method == "" {
			method = http.MethodPost
		}

		url = url + "?method=" + method + "&path=" + item
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
