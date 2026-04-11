package commands

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/runtime/rest"
)

// RestGet issues an HTTP GET request to the given URL and prints the response.
// The URL may be a full URL or a path relative to the configured application server.
//
// Invoked by:
//
//	Traditional: ego rest get <url>
//	Verb:        ego rest get <url>
func RestGet(c *cli.Context) error {
	return restAction(c, http.MethodGet)
}

// RestPost issues an HTTP POST request to the given URL with an optional request body,
// and prints the response. The body can be provided as a JSON string (--data) or as
// individual key=value fields (--field).
//
// Invoked by:
//
//	Traditional: ego rest post <url>
//	Verb:        ego rest post <url>
func RestPost(c *cli.Context) error {
	return restAction(c, http.MethodPost)
}

// RestPut issues an HTTP PUT request to the given URL with an optional request body,
// and prints the response.
//
// Invoked by:
//
//	Traditional: ego rest put <url>
//	Verb:        ego rest put <url>
func RestPut(c *cli.Context) error {
	return restAction(c, http.MethodPut)
}

// RestDelete issues an HTTP DELETE request to the given URL and prints the response.
//
// Invoked by:
//
//	Traditional: ego rest delete <url>
//	Verb:        ego rest delete <url>
func RestDelete(c *cli.Context) error {
	return restAction(c, http.MethodDelete)
}

// RestPatch issues an HTTP PATCH request to the given URL with an optional request body,
// and prints the response.
//
// Invoked by:
//
//	Traditional: ego rest patch <url>
//	Verb:        ego rest patch <url>
func RestPatch(c *cli.Context) error {
	return restAction(c, http.MethodPatch)
}

func restAction(c *cli.Context, method string) error {
	var (
		requestBody interface{}
		response    interface{}
	)

	// Get the URL from the parameter, and make it a full URL.
	url := c.FindGlobal().Parameters[0]

	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		appServer := settings.Get(defs.ApplicationServerSetting)
		if appServer == "" {
			appServer = settings.Get(defs.LogonServerSetting)
		}

		url = strings.TrimPrefix(url, "/")
		appServer = strings.TrimSuffix(appServer, "/")

		url = appServer + "/" + url
	}

	if params, found := c.StringList("params"); found {
		list := map[string][]string{}

		for _, param := range params {
			kv := strings.SplitN(param, "=", 2)
			list[kv[0]] = append(list[kv[0]], kv[1])
		}

		first := true
		for key, values := range list {
			if first {
				url += "?"
				first = false
			} else {
				url += "&"
			}

			url += key + "=" + strings.Join(values, ",")
		}
	}

	// Get the media types, if any
	media, _ := c.StringList("accepts")
	isJSON := true

	for _, m := range media {
		if strings.Contains(strings.ToLower(m), "text") {
			isJSON = false

			break
		}
	}

	// If they asked for verbose output, enable the logger.
	verbose := c.Boolean("verbose")
	if verbose {
		ui.Active(ui.RestLogger, true)
	}

	// If there is a request body, get it now.
	if body, found := c.String("data"); found {
		if strings.HasPrefix(body, "@") {
			fn := body[1:]

			b, err := ui.ReadJSONFile(fn)
			if err != nil {
				return errors.New(err)
			}

			err = json.Unmarshal(b, &requestBody)
			if err != nil {
				return errors.New(err)
			}
		} else {
			requestBody = body
		}
	}

	// The rest body might be specified as one or more fields.
	if fieldList, ok := c.StringList("field"); ok {
		body := map[string]string{}

		for _, field := range fieldList {
			parts := strings.SplitN(field, "=", 2)
			body[parts[0]] = parts[1]
		}

		requestBody = body
	}

	err := rest.Exchange(url, method, requestBody, &response, defs.ClientAgent, media...)

	if errors.Nil(err) {
		if isJSON {
			var b []byte

			b, err = json.MarshalIndent(response, ui.JSONIndentPrefix, ui.JSONIndentSpacer)
			if err == nil {
				ui.Say(string(b))
			}
		} else {
			text := data.Format(response)
			ui.Say(text)
		}
	}

	return err
}
