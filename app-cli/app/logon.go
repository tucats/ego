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

// LogonGrammar describes the login subcommand options. This grammar is
// added to the active grammar automatically (it does not need to be
// specified explicitly in the caller's grammar).
var LogonGrammar = []cli.Option{
	{
		LongName:    defs.UsernameOption,
		ShortName:   "u",
		OptionType:  cli.StringType,
		Description: "username",
		EnvVar:      defs.EgoUserEnv,
	},
	{
		LongName:    defs.PasswordOption,
		ShortName:   "p",
		OptionType:  cli.StringType,
		Description: "password",
		EnvVar:      defs.EgoPasswordEnv,
	},
	{
		LongName:    "logon-server",
		ShortName:   "l",
		Aliases:     []string{"server"},
		OptionType:  cli.StringType,
		Description: "logon.server",
		EnvVar:      defs.EgoLogonServerEnv,
	},
	{
		LongName:    "expiration",
		ShortName:   "e",
		Aliases:     []string{"expires"},
		OptionType:  cli.StringType,
		Description: "logon.expiration",
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
	user, _ := c.String(defs.UsernameOption)
	for strings.TrimSpace(user) == "" {
		user = ui.Prompt("Username: ")
	}

	user = strings.TrimSpace(user)

	// Get the password. If not supplied by the user, prompt until provided.
	pass, _ := c.String(defs.PasswordOption)
	for strings.TrimSpace(pass) == "" {
		pass = ui.PromptPassword(i18n.L("password.prompt"))
	}

	pass = strings.TrimSpace(pass)

	// Lets not log this until we're successfully prompted for missing input
	// and validated that the expiration is okay.
	ui.Log(ui.RestLogger, "logon.url", ui.A{
		"url": url})

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
				Expiration: expiration}, ui.JSONIndentPrefix, ui.JSONIndentSpacer)

			ui.Log(ui.RestLogger, "logon.request", ui.A{
				"body": string(b)})
		}

		req.Header.Set("Accept", defs.JSONMediaType)
		rest.AddAgent(req, defs.LogonAgent)

		r, err = req.Post(url)
		if err != nil {
			// This is a gross hack, but I don't yet know how to determine the
			// specific error value.
			if !strings.Contains(err.Error(), "auto redirect is disabled") {
				ui.Log(ui.RestLogger, "logon.post.error", ui.A{
					"url":   url,
					"error": err})

				return errors.New(err)
			}
		}

		if r.StatusCode() == http.StatusMovedPermanently {
			url = r.Header().Get("Location")
			if url != "" {
				ui.Log(ui.RestLogger, "logon.redirecting", ui.A{
					"url": url})

				continue
			}
		}

		ui.Log(ui.RestLogger, "logon.post.status", ui.A{
			"url":    url,
			"status": r.StatusCode()})

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
			ui.Log(ui.RestLogger, "logon.request", ui.A{
				"body": string(b)})
		} else {
			ui.Log(ui.RestLogger, "logon.request", ui.A{
				"body": "<empty>"})
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
		ui.Log(ui.RestLogger, "logon.payload.error", ui.A{
			"error": err})

		return errors.New(err).In("logon")
	}

	if ui.IsActive(ui.RestLogger) {
		b, _ := json.MarshalIndent(payload, ui.JSONIndentPrefix, ui.JSONIndentSpacer)
		ui.Log(ui.RestLogger, "logon.response", ui.A{
			"body": string(b)})
	}

	settings.Set(defs.LogonTokenSetting, payload.Token)
	settings.Set(defs.LogonTokenExpirationSetting, payload.Expiration)

	err := settings.Save()
	if err == nil {
		msg := i18n.M("logged.in", map[string]any{
			"user":    user,
			"expires": payload.Expiration,
		})

		ui.Say("%s", msg)
	}

	return errors.New(err)
}

// Use the configuration and command line context to find the logon server
// that will be used to authenticate. The default is to use the logon server
// from the profile, but if the logon server was explicitly set on the command
// line, that overrides the profile setting.
//
// The resolved name is normalized to resolve localhost, set port number if
// needed, etc. before returning it to the caller.
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

// This function retrieves the expiration time from the command line, if
// provided. If no expiration parameter was given, return an empty string.
// If an expiration time is provided, it must be a valid duration string
// and the result is normalized as a time.Duration string value and
// returned to the caller.
func validateExpiration(c *cli.Context) (string, error) {
	// Was the expiration time specified? If not, return an empty string.
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
	ui.Log(ui.RestLogger, "rest.heartbeat", ui.A{
		"name": normalizedName})

	err = rest.Exchange(defs.AdminHeartbeatPath, http.MethodGet, nil, nil, defs.LogonAgent)
	if err == nil {
		return normalizedName, nil
	}

	// Nope. Same deal with http scheme.
	normalizedName = "http://" + name + port

	settings.SetDefault(defs.ApplicationServerSetting, normalizedName)

	ui.Log(ui.RestLogger, "rest.heartbeat", ui.A{
		"name": normalizedName})

	err = rest.Exchange(defs.AdminHeartbeatPath, http.MethodGet, nil, nil, defs.LogonAgent)
	if err != nil {
		err = errors.ErrUnableToReachHost.Context(name)
	}

	return normalizedName, err
}
