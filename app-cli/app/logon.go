package app

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/i18n"
	"github.com/tucats/ego/runtime/rest"
	"github.com/tucats/ego/util"
	"gopkg.in/resty.v1"
)

const hoursInADay = 24

// LogonGrammar describes the login subcommand options.
var LogonGrammar = []cli.Option{
	{
		LongName:    "username",
		ShortName:   "u",
		OptionType:  cli.StringType,
		Description: "username",
		EnvVar:      "EGO_USERNAME",
	},
	{
		LongName:    "password",
		ShortName:   "p",
		OptionType:  cli.StringType,
		Description: "password",
		EnvVar:      "EGO_PASSWORD",
	},
	{
		LongName:    "logon-server",
		ShortName:   "l",
		Aliases:     []string{"server"},
		OptionType:  cli.StringType,
		Description: "logon.server",
		EnvVar:      "EGO_LOGON_SERVER",
	},
	{
		LongName:    "expiration",
		ShortName:   "e",
		Aliases:     []string{"expires"},
		OptionType:  cli.StringType,
		Description: "logon.expiration",
		EnvVar:      "EGO_LOGON_EXPIRATION",
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
	var (
		err error
		r   *resty.Response
	)

	// If present, set the requested expiration time for the token. This must
	// be either empty or a valid duration string. If the user specified a duration
	// of days, this is automatically converted to hours. The validation of the
	// expiration parameter should be done here, before prompting the user for
	// any missing information.
	expiration, err := validateExpiration(c)
	if err != nil {
		return err
	}

	// Do we know where the logon server is? Start with the default from
	// the profile, but if it was explicitly set on the command line, use
	// the command line item and update the saved profile setting.
	url, err := findLogonServer(c)
	if err != nil {
		return err
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

	// Lets not log this until we're successfully prompted for missing input
	// and validated that the expiration is okay.
	ui.Log(ui.RestLogger, "Logon URL is %s", url)

	// Turn logon server address and endpoint into full URL.
	url = strings.TrimSuffix(url, "/") + defs.ServicesLogonPath

	// Create a new client, set it's attribute for basic authentication, and
	// generate a request. The request is made using the logon agent info.
	// We allow up to 10 redirects on the logon if we are forwarded to a
	// different authority server.
	restClient := resty.New().
		SetDisableWarn(true)

	if tlsConf, err := rest.GetTLSConfiguration(); err != nil {
		return err
	} else {
		restClient.SetTLSClientConfig(tlsConf)
	}

	retryCount := 5
	for retryCount >= 0 {
		retryCount--

		req := restClient.NewRequest()
		req.Body = defs.Credentials{Username: user, Password: pass, Expiration: expiration}

		if ui.IsActive(ui.RestLogger) {
			// Use a fake password payload for the REST logging so we don't expose the password
			b, _ := json.MarshalIndent(defs.Credentials{
				Username:   user,
				Password:   "********",
				Expiration: expiration}, "", "  ")

			ui.Log(ui.RestLogger, "REST Request:\n%s", string(b))
		}

		req.Header.Set("Accept", defs.JSONMediaType)
		rest.AddAgent(req, defs.LogonAgent)

		r, err = req.Post(url)
		if err != nil {
			// This is a gross hack, but I don't yet know how to determine the
			// specific error value.
			if !strings.Contains(err.Error(), "auto redirect is disabled") {
				ui.Log(ui.RestLogger, "POST %s; failed %v", url, err)

				return errors.New(err)
			}
		}

		if r.StatusCode() == http.StatusMovedPermanently {
			url = r.Header().Get("Location")
			if url != "" {
				ui.Log(ui.RestLogger, "Redirecting to %s", url)

				continue
			}
		}

		ui.Log(ui.RestLogger, "REST POST %s; status %d", url, r.StatusCode())

		break
	}

	// If the call was successful and the server responded with Success, remove any trailing
	// newline from the result body and store the string as the new token value.
	if err == nil && r.StatusCode() == http.StatusOK {
		return storeLogonToken(r, user)
	}

	if ui.IsActive(ui.RestLogger) {
		b := r.Body()

		if len(b) > 0 {
			ui.Log(ui.RestLogger, "REST Request:\n%s", string(b))
		} else {
			ui.Log(ui.RestLogger, "REST Request: <empty>")
		}
	}

	// If there was an HTTP error condition, let's report it now.
	if err == nil {
		switch r.StatusCode() {
		case http.StatusUnauthorized:
			err = errors.ErrInvalidCredentials

		case http.StatusForbidden:
			err = errors.ErrNoPermission

		case http.StatusNotFound:
			err = errors.ErrLogonEndpoint

		default:
			err = errors.ErrHTTP.Context(r.StatusCode())
		}
	}

	if err != nil {
		err = errors.New(err)
	}

	return err
}

func storeLogonToken(r *resty.Response, user string) error {
	payload := defs.LogonResponse{}

	if err := json.Unmarshal(r.Body(), &payload); err != nil {
		ui.Log(ui.RestLogger, "[%d] Bad payload: %v", 0, err)

		return errors.New(err).In("logon")
	}

	if ui.IsActive(ui.RestLogger) {
		b, _ := json.MarshalIndent(payload, "", "  ")
		ui.Log(ui.RestLogger, "REST Response:\n%s", string(b))
	}

	settings.Set(defs.LogonTokenSetting, payload.Token)
	settings.Set(defs.LogonTokenExpirationSetting, payload.Expiration)

	err := settings.Save()
	if err == nil {
		msg := i18n.M("logged.in", map[string]interface{}{
			"user":    user,
			"expires": payload.Expiration,
		})

		ui.Say("%s", msg)
	}

	return errors.New(err)
}

func findLogonServer(c *cli.Context) (string, error) {
	var err error

	url := settings.Get(defs.LogonServerSetting)
	if c.WasFound("logon-server") {
		url, _ = c.String("logon-server")

		if url, err = resolveServerName(url); err != nil {
			return "", err
		} else {
			settings.Set(defs.LogonServerSetting, url)
		}
	}

	if url == "" {
		return "", errors.ErrNoLogonServer
	}

	return url, nil
}

func validateExpiration(c *cli.Context) (string, error) {
	// Was the expiratin time specified? If not, return an empty string.
	expiration, found := c.String("expiration")
	if !found {
		return "", nil
	}

	// Parse the duration to make sure it's valid. If not, return an error.
	// Otherwise, reformat the duration as a string for later use and return
	// it. We parse-and-reassemble the duration because this duration parser
	// can handle cases the regular time package parser cannot -- specifically
	// it handles "d" for days in the duration string. By parsing and reformatting
	// it, the days get converted automatically to hours.
	if d, err := util.ParseDuration(expiration); err != nil {
		return "", errors.New(err)
	} else {
		return d.String(), nil
	}
}

// Resolve a name that may not be fully qualified, and make it the default
// application host name. This is used by commands that allow a host name
// specification as part of the command (login, or server logging, etc.).
func resolveServerName(name string) (string, error) {
	var (
		hasScheme = true
		urlString string
	)

	normalizedName := strings.ToLower(name)
	if !strings.HasPrefix(normalizedName, "https://") && !strings.HasPrefix(normalizedName, "http://") {
		hasScheme = false
		urlString = "https://" + normalizedName
	} else {
		urlString = normalizedName
	}

	// Now make sure it's well-formed.
	url, err := url.Parse(urlString)
	if err != nil {
		return "", errors.New(err)
	}

	port := url.Port()
	if port == "" {
		port = ":443"
	} else {
		port = ""
	}

	// Start by trying to connect with what we have, if it had a scheme. In this
	// case, the string is expected to be complete.
	if hasScheme {
		settings.SetDefault(defs.ApplicationServerSetting, name)

		err = rest.Exchange(defs.AdminHeartbeatPath, http.MethodGet, nil, nil, defs.LogonAgent)
		if err == nil {
			return name, nil
		}
	}

	// No scheme, so let's try https. If no port supplied, assume the default port.
	normalizedName = "https://" + name + port

	settings.SetDefault(defs.ApplicationServerSetting, normalizedName)
	ui.Log(ui.RestLogger, "Checking for heartbeat at %s", normalizedName)

	err = rest.Exchange(defs.AdminHeartbeatPath, http.MethodGet, nil, nil, defs.LogonAgent)
	if err == nil {
		return normalizedName, nil
	}

	// Nope. Same deal with http scheme.
	normalizedName = "http://" + name + port

	settings.SetDefault(defs.ApplicationServerSetting, normalizedName)

	ui.Log(ui.RestLogger, "Checking for heartbeat at %s", normalizedName)

	err = rest.Exchange(defs.AdminHeartbeatPath, http.MethodGet, nil, nil, defs.LogonAgent)
	if err != nil {
		err = errors.ErrUnableToReachHost.Context(name)
	}

	return normalizedName, err
}
