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

// ServerValidations queries the server's request-validation dictionary and displays
// the result. Without arguments it dumps the full dictionary. With a named item it
// shows just that entry. Flags --path, --entry, and --method narrow the query further.
// This is an admin command used to inspect how the server validates incoming requests.
//
// Invoked by:
//
//	Traditional: ego server validation [<item>]
//	Verb:        ego show server validations [<item>]
func ServerValidations(c *cli.Context) error {
	var (
		b        []byte
		err      error
		response map[string]any
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
	method, _ = c.String("method")

	if items := c.FindGlobal().Parameters; len(items) > 0 {
		item = items[0]
		if (item == "all" && !path && !entry) || item == "*" {
			item = ""
		}
	}

	// If there was no item, then the user cannot specify options that imply there
	// is a named path or item.
	if item == "" && entry {
		return errors.ErrMissingItem
	}

	if item == "" && (method != "" || path) {
		return errors.ErrMissingEndPoint
	}

	// If no type was explicitly given, infer from the item name prefix.
	if !path && !entry && item != "" {
		if strings.HasPrefix(item, "/") {
			path = true
		} else {
			entry = true
		}
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
