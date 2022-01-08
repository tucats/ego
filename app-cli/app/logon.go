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
	"github.com/tucats/ego/app-cli/persistence"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/runtime"
)

// LogonGrammar describes the login subcommand.
var LogonGrammar = []cli.Option{
	{
		LongName:            "username",
		ShortName:           "u",
		OptionType:          cli.StringType,
		Description:         "Username for login",
		EnvironmentVariable: "CLI_USERNAME",
	},
	{
		LongName:            "password",
		ShortName:           "p",
		OptionType:          cli.StringType,
		Description:         "Password for login",
		EnvironmentVariable: "CLI_PASSWORD",
	},
	{
		LongName:            "logon-server",
		ShortName:           "l",
		Aliases:             []string{"server"},
		OptionType:          cli.StringType,
		Description:         "URL of logon server",
		EnvironmentVariable: "CLI_LOGON_SERVER",
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
	url := persistence.Get(defs.LogonServerSetting)
	if c.WasFound("logon-server") {
		url, _ = c.GetString("logon-server")

		var e2 *errors.EgoError

		url, e2 = resolveServerName(url)
		if !errors.Nil(e2) {
			return e2
		}

		persistence.Set(defs.LogonServerSetting, url)
	}

	if url == "" {
		return errors.New(errors.ErrNoLogonServer)
	}

	// Get the username. If not supplied by the user, prompt until provided.
	user, _ := c.GetString("username")
	for user == "" {
		user = ui.Prompt("Username: ")
	}

	// Get the password. If not supplied by the user, prompt until provided.
	pass, _ := c.GetString("password")
	for pass == "" {
		pass = ui.PromptPassword("Password: ")
	}

	// Turn logon server address and endpoint into full URL.
	url = strings.TrimSuffix(url, "/") + defs.ServicesLogonPath

	// Create a new client, set it's attribute for basic authentication, and
	// generate a request. The request is made using the logon agent info.
	// Finall, call the endpoint.
	restClient := resty.New().SetDisableWarn(true).SetBasicAuth(user, pass)
	if os.Getenv("EGO_INSECURE_CLIENT") == "true" {
		restClient.SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true})
	}

	req := restClient.NewRequest()
	req.Header.Set("Accept", defs.JSONMediaType)

	runtime.AddAgent(req, defs.LogonAgent)

	ui.Debug(ui.DebugLogger, "REST URL: %s", url)

	r, err := req.Get(url)

	// If the call was successful and the server responded with Success, remove any trailing
	// newline from the result body and store the string as the new token value.
	if errors.Nil(err) && r.StatusCode() == http.StatusOK {
		payload := defs.LogonResponse{}
		err := json.Unmarshal(r.Body(), &payload)
		if err != nil {
			return errors.New(err).Context("logon")
		}

		if ui.LoggerIsActive(ui.DebugLogger) {
			b, _ := json.MarshalIndent(payload, "", "  ")
			ui.Debug(ui.DebugLogger, "REST Response:\n%s", string(b))
		}
		token := payload.Token
		persistence.Set(defs.LogonTokenSetting, token)

		err = persistence.Save()
		if errors.Nil(err) {
			ui.Say("Successfully logged in as \"%s\", valid until %s", user, payload.Expiration)
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
		persistence.SetDefault("ego.application.server", name)

		err = runtime.Exchange(defs.AdminHeartbeatPath, http.MethodGet, nil, nil, defs.LogonAgent)
		if errors.Nil(err) {
			return name, nil
		}
	}

	// No scheme, so let's try https. If no port supplied, assume the default port.
	normalizedName = "https://" + name + port

	persistence.SetDefault("ego.application.server", normalizedName)

	err = runtime.Exchange(defs.AdminHeartbeatPath, http.MethodGet, nil, nil, defs.LogonAgent)
	if errors.Nil(err) {
		return normalizedName, nil
	}

	// Nope. Same deal with http scheme.
	normalizedName = "http://" + name + port

	persistence.SetDefault("ego.application.server", normalizedName)

	err = runtime.Exchange(defs.AdminHeartbeatPath, http.MethodGet, nil, nil, defs.LogonAgent)

	return normalizedName, errors.New(err)
}
