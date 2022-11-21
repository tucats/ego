package app

import (
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/go-resty/resty"
	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/i18n"
	"github.com/tucats/ego/runtime"
)

// LogonGrammar describes the login subcommand.
var LogonGrammar = []cli.Option{
	{
		LongName:            "username",
		ShortName:           "u",
		OptionType:          cli.StringType,
		Description:         "opt.username",
		EnvironmentVariable: "EGO_USERNAME",
	},
	{
		LongName:            "password",
		ShortName:           "p",
		OptionType:          cli.StringType,
		Description:         "opt.password",
		EnvironmentVariable: "EGO_PASSWORD",
	},
	{
		LongName:            "logon-server",
		ShortName:           "l",
		Aliases:             []string{"server"},
		OptionType:          cli.StringType,
		Description:         "opt.logon.server",
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
func Logon(c *cli.Context) *errors.EgoError {
	// Do we know where the logon server is? Start with the default from
	// the profile, but if it was explicitly set on the command line, use
	// the command line item and update the saved profile setting.
	url := settings.Get(defs.LogonServerSetting)
	if c.WasFound("logon-server") {
		url, _ = c.String("logon-server")

		var e2 *errors.EgoError

		url, e2 = resolveServerName(url)
		if !errors.Nil(e2) {
			return e2
		}

		settings.Set(defs.LogonServerSetting, url)
	}

	if url == "" {
		return errors.New(errors.ErrNoLogonServer)
	}

	// Get the username. If not supplied by the user, prompt until provided.
	user, _ := c.String("username")
	for user == "" {
		user = ui.Prompt("Username: ")
	}

	// Get the password. If not supplied by the user, prompt until provided.
	pass, _ := c.String("password")
	for pass == "" {
		pass = ui.PromptPassword(i18n.T("label.password.prompt"))
	}

	// Turn logon server address and endpoint into full URL.
	url = strings.TrimSuffix(url, "/") + defs.ServicesLogonPath

	// Create a new client, set it's attribute for basic authentication, and
	// generate a request. The request is made using the logon agent info.
	// Finall, call the endpoint.
	restClient := resty.New().SetDisableWarn(true)
	if os.Getenv("EGO_INSECURE_CLIENT") == defs.True {
		restClient.SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true})
	}

	req := restClient.NewRequest()
	req.Body = defs.Credentials{Username: user, Password: pass}

	if ui.LoggerIsActive(ui.RestLogger) {
		// Use a fake password payload for the REST logging so we don't expose the password
		b, _ := json.MarshalIndent(defs.Credentials{Username: user, Password: "********"}, "", "  ")
		ui.Debug(ui.RestLogger, "REST Request:\n%s", string(b))
	}

	req.Header.Set("Accept", defs.JSONMediaType)
	runtime.AddAgent(req, defs.LogonAgent)

	r, err := req.Post(url)

	ui.Debug(ui.RestLogger, "REST POST %s; status %d", url, r.StatusCode())

	// If the call was successful and the server responded with Success, remove any trailing
	// newline from the result body and store the string as the new token value.
	if errors.Nil(err) && r.StatusCode() == http.StatusOK {
		payload := defs.LogonResponse{}

		err := json.Unmarshal(r.Body(), &payload)
		if err != nil {
			return errors.New(err).Context("logon")
		}

		if ui.LoggerIsActive(ui.RestLogger) {
			b, _ := json.MarshalIndent(payload, "", "  ")
			ui.Debug(ui.RestLogger, "REST Response:\n%s", string(b))
		}

		settings.Set(defs.LogonTokenSetting, payload.Token)
		settings.Set(defs.LogonTokenExpirationSetting, payload.Expiration)

		err = settings.Save()
		if errors.Nil(err) {
			msg := i18n.T("msg.logged.in", map[string]interface{}{
				"user":    user,
				"expires": payload.Expiration,
			})

			ui.Say("%s", msg)
		}

		return errors.New(err)
	}

	// If there was an HTTP error condition, let's report it now.
	if errors.Nil(err) {
		switch r.StatusCode() {
		case http.StatusUnauthorized:
			err = errors.New(errors.ErrNoCredentials)

		case http.StatusForbidden:
			err = errors.New(errors.ErrInvalidCredentials)

		case http.StatusNotFound:
			err = errors.New(errors.ErrLogonEndpoint)

		default:
			err = errors.New(errors.ErrHTTP).Context(r.StatusCode())
		}
	}

	return errors.New(err)
}

// Resolve a name that may not be fully qualified, and make it the default
// application host name. This is used by commands that allow a host name
// specification as part of the command (login, or server logging, etc.).
func resolveServerName(name string) (string, *errors.EgoError) {
	hasScheme := true

	normalizedName := strings.ToLower(name)
	if !strings.HasPrefix(normalizedName, "https://") && !strings.HasPrefix(normalizedName, "http://") {
		normalizedName = "https://" + name
		hasScheme = false
	}

	// Now make sure it's well-formed.
	url, err := url.Parse(normalizedName)
	if err != nil {
		return "", errors.New(err)
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

		err = runtime.Exchange(defs.AdminHeartbeatPath, http.MethodGet, nil, nil, defs.LogonAgent)
		if errors.Nil(err) {
			return name, nil
		}
	}

	// No scheme, so let's try https. If no port supplied, assume the default port.
	normalizedName = "https://" + name + port

	settings.SetDefault("ego.application.server", normalizedName)

	err = runtime.Exchange(defs.AdminHeartbeatPath, http.MethodGet, nil, nil, defs.LogonAgent)
	if errors.Nil(err) {
		return normalizedName, nil
	}

	// Nope. Same deal with http scheme.
	normalizedName = "http://" + name + port

	settings.SetDefault("ego.application.server", normalizedName)

	err = runtime.Exchange(defs.AdminHeartbeatPath, http.MethodGet, nil, nil, defs.LogonAgent)

	return normalizedName, errors.New(err)
}
