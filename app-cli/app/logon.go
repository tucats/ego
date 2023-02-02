package app

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-resty/resty"
	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/i18n"
	"github.com/tucats/ego/runtime/rest"
)

// LogonGrammar describes the login subcommand options.
var LogonGrammar = []cli.Option{
	{
		LongName:            "username",
		ShortName:           "u",
		OptionType:          cli.StringType,
		Description:         "username",
		EnvironmentVariable: "EGO_USERNAME",
	},
	{
		LongName:            "password",
		ShortName:           "p",
		OptionType:          cli.StringType,
		Description:         "password",
		EnvironmentVariable: "EGO_PASSWORD",
	},
	{
		LongName:            "logon-server",
		ShortName:           "l",
		Aliases:             []string{"server"},
		OptionType:          cli.StringType,
		Description:         "logon.server",
		EnvironmentVariable: "EGO_LOGON_SERVER",
	},
}

// Logon handles the logon subcommand. This accepts a username and
// password string from the user via the command line, or console
// input if not provided on the command line. These credentials are
// used to connect to an Ego logon server and request an authentication
// token to be used for subsequent operations.
//
// If the user credentials are valid and a token is returned, it is
// stored in the user's active profile where it can be accessed by
// other Ego commands as needed.
func Logon(c *cli.Context) error {
	var err error

	// Do we know where the logon server is? Start with the default from
	// the profile, but if it was explicitly set on the command line, use
	// the command line item and update the saved profile setting.
	url := settings.Get(defs.LogonServerSetting)
	if c.WasFound("logon-server") {
		url, _ = c.String("logon-server")

		if url, err = resolveServerName(url); err != nil {
			return err
		} else {
			settings.Set(defs.LogonServerSetting, url)
		}
	}

	if url == "" {
		return errors.ErrNoLogonServer
	} else {
		ui.Log(ui.RestLogger, "Logon URL is %s", url)
	}

	// Get the username. If not supplied by the user, prompt until provided.
	user, _ := c.String("username")
	for user == "" {
		user = ui.Prompt("Username: ")
	}

	// Get the password. If not supplied by the user, prompt until provided.
	pass, _ := c.String("password")
	for pass == "" {
		pass = ui.PromptPassword(i18n.L("password.prompt"))
	}

	// Turn logon server address and endpoint into full URL.
	url = strings.TrimSuffix(url, "/") + defs.ServicesLogonPath

	// Create a new client, set it's attribute for basic authentication, and
	// generate a request. The request is made using the logon agent info.
	// Finall, call the endpoint.
	restClient := resty.New().SetDisableWarn(true)

	if tlsConf, err := rest.GetTLSConfiguration(); err != nil {
		return err
	} else {
		restClient.SetTLSClientConfig(tlsConf)
	}

	req := restClient.NewRequest()
	req.Body = defs.Credentials{Username: user, Password: pass}

	if ui.IsActive(ui.RestLogger) {
		// Use a fake password payload for the REST logging so we don't expose the password
		b, _ := json.MarshalIndent(defs.Credentials{Username: user, Password: "********"}, "", "  ")
		ui.Log(ui.RestLogger, "REST Request:\n%s", string(b))
	}

	req.Header.Set("Accept", defs.JSONMediaType)
	rest.AddAgent(req, defs.LogonAgent)

	r, err := req.Post(url)
	if err != nil {
		ui.Log(ui.RestLogger, "REST POST %s; failed %v", url, err)

		return errors.NewError(err)
	}

	ui.Log(ui.RestLogger, "REST POST %s; status %d", url, r.StatusCode())

	// If the call was successful and the server responded with Success, remove any trailing
	// newline from the result body and store the string as the new token value.
	if err == nil && r.StatusCode() == http.StatusOK {
		payload := defs.LogonResponse{}

		err := json.Unmarshal(r.Body(), &payload)
		if err != nil {
			return errors.NewError(err).In("logon")
		}

		if ui.IsActive(ui.RestLogger) {
			b, _ := json.MarshalIndent(payload, "", "  ")
			ui.Log(ui.RestLogger, "REST Response:\n%s", string(b))
		}

		settings.Set(defs.LogonTokenSetting, payload.Token)
		settings.Set(defs.LogonTokenExpirationSetting, payload.Expiration)

		err = settings.Save()
		if err == nil {
			msg := i18n.M("logged.in", map[string]interface{}{
				"user":    user,
				"expires": payload.Expiration,
			})

			ui.Say("%s", msg)
		}

		if err != nil {
			err = errors.NewError(err)
		}

		return err
	}

	// If there was an HTTP error condition, let's report it now.
	if err == nil {
		switch r.StatusCode() {
		case http.StatusUnauthorized:
			err = errors.ErrNoCredentials

		case http.StatusForbidden:
			err = errors.ErrInvalidCredentials

		case http.StatusNotFound:
			err = errors.ErrLogonEndpoint

		default:
			err = errors.ErrHTTP.Context(r.StatusCode())
		}
	}

	if err != nil {
		err = errors.NewError(err)
	}

	return err
}

// Resolve a name that may not be fully qualified, and make it the default
// application host name. This is used by commands that allow a host name
// specification as part of the command (login, or server logging, etc.).
func resolveServerName(name string) (string, error) {
	hasScheme := true

	normalizedName := strings.ToLower(name)
	if !strings.HasPrefix(normalizedName, "https://") && !strings.HasPrefix(normalizedName, "http://") {
		hasScheme = false
	}

	// Now make sure it's well-formed.
	url, err := url.Parse(normalizedName)
	if err != nil {
		return "", errors.NewError(err)
	}

	port := url.Port()
	if port == "" {
		port = ":8080"
	} else {
		port = ""
	}

	// Start by trying to connect with what we have, if it had a scheme. In this
	// case, the string is expected to be complete.
	if hasScheme {
		settings.SetDefault("ego.application.server", name)

		err = rest.Exchange(defs.AdminHeartbeatPath, http.MethodGet, nil, nil, defs.LogonAgent)
		if err == nil {
			return name, nil
		}
	}

	// No scheme, so let's try https. If no port supplied, assume the default port.
	normalizedName = "https://" + name + port

	settings.SetDefault("ego.application.server", normalizedName)

	err = rest.Exchange(defs.AdminHeartbeatPath, http.MethodGet, nil, nil, defs.LogonAgent)
	if err == nil {
		return normalizedName, nil
	}

	// Nope. Same deal with http scheme.
	normalizedName = "http://" + name + port

	settings.SetDefault("ego.application.server", normalizedName)

	err = rest.Exchange(defs.AdminHeartbeatPath, http.MethodGet, nil, nil, defs.LogonAgent)

	if err != nil {
		err = errors.NewError(err)
	}

	return normalizedName, err
}
