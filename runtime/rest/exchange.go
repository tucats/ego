package rest

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/i18n"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
	"gopkg.in/resty.v1"
)

// ServerCertificateFile is the default file name for the server certificate.
var ServerCertificateFile = envDefault(defs.EgoCertFileEnv, "https-server.crt")

// ServerKeyFile is the default file name for the server key.
var ServerKeyFile = envDefault(defs.EgoKeyFileEnv, "https-server.key")

// Max number of times we will chase a redirect before failing.
const MaxRedirectCount = 10

// Cache for the TLS configuration object we'll be using. This is set globally
// the first time a REST call is made, and stored here. GetTLSConfiguration()
// checks this first, and if nil, sets it up based on the current settings.
var tlsConfiguration *tls.Config
var tlsConfigurationMutex sync.Mutex

// openServices is a list of endpoint paths that do not require the
// addition of an authorization token.
var openServices = []string{
	defs.ServicesUpPath,
	defs.AdminHeartbeatPath,
}

// Get an environment variable value. If it is not present (or empty) then use
// the provided default value as the result.
func envDefault(name, defaultValue string) string {
	result := os.Getenv(name)
	if result == "" {
		result = defaultValue
	}

	return result
}

// Exchange is a helper wrapper around a rest call. This is generally used by all the
// CLI client operations _except_ the logon operation, since at that point the token
// is not known (or used).
func Exchange(endpoint, method string, body interface{}, response interface{}, agentType string, mediaTypes ...string) error {
	var (
		resp         *resty.Response
		err          error
		stillWaiting atomic.Bool
		url          string
	)

	// Is there a configuration override for the insecure setting we should check before doing a call?
	if settings.GetBool(defs.InsecureClientSetting) {
		ui.Log(ui.RestLogger, "Configuration profile allows insecure client")
		AllowInsecure(true)
	}

	// If the endpoint already has a full URL (i.e. starts with scheme) then just use it as-is. Otherwise, find the server
	// that should be prepended to the endpoint string.
	if strings.HasPrefix(strings.ToLower(endpoint), "http://") || strings.HasPrefix(strings.ToLower(endpoint), "https://") {
		url = endpoint
	} else {
		url = settings.Get(defs.ApplicationServerSetting)
		if url == "" {
			url = settings.Get(defs.LogonServerSetting)
		}

		if url == "" {
			url = "http://localhost:80"
		}

		url = strings.TrimSuffix(url, "/") + endpoint
	}

	ui.Log(ui.RestLogger, "%s %s", strings.ToUpper(method), url)

	client := resty.New().SetRedirectPolicy(resty.FlexibleRedirectPolicy(MaxRedirectCount))

	// Unless this is a open (un-authenticate) service, let's verify that the
	// authentication token is still valid.
	if util.InList(endpoint, openServices...) {
		ui.Log(ui.RestLogger, "Endpoint %s does not require token", endpoint)
	} else {
		// if this is the check for authentication, use the body as the token.
		if strings.HasSuffix(url, "/services/admin/authenticate/") {
			token := data.String(body)
			client.SetAuthToken(token)
		} else if token := settings.Get(defs.LogonTokenSetting); token != "" {
			// Let's check to see if it's expired already...
			if expirationString := settings.Get(defs.LogonTokenExpirationSetting); expirationString != "" {
				expireTime, err := time.Parse(time.UnixDate, expirationString)
				if err != nil {
					return errors.New(err)
				}

				now := time.Since(expireTime)
				if now > 0 {
					ui.Say("Your login has expired. Use the ego logon command to login again to %s", settings.Get(defs.LogonServerSetting))

					os.Exit(1)
				}
			}

			client.SetAuthToken(token)
			ui.Log(ui.RestLogger, "Authorization set using bearer token: %s...", token[:4])
		}
	}

	if config, err := GetTLSConfiguration(); err != nil {
		return err
	} else {
		client.SetTLSClientConfig(config)
	}

	// Get the maximum timeout for a REST call if there is duration in the
	// configuration. A setting of an empty string, "0", or "none" means no
	// timeout. Otherwise, the value is a duration string.
	if t := settings.Get(defs.RestClientTimeoutSetting); t != "" {
		if t != "0" && t != "0s" && t != "none" {
			timeout, err := time.ParseDuration(t)
			if err != nil {
				return errors.New(err)
			}

			client.SetTimeout(timeout)
		}
	}

	// Generate a new request based on this client.
	r := client.NewRequest()

	// Lets figure out what media types we're sending and reciving. By default, they
	// are anonymous JSON. But if the call included one or two strings, they are used
	// as the receiving and sending media types respectively.
	receiveMediaType := defs.JSONMediaType
	sendMediaType := defs.JSONMediaType

	if len(mediaTypes) > 0 {
		receiveMediaType = mediaTypes[0]

		ui.Log(ui.RestLogger, "Adding media type: %s", receiveMediaType)
	}

	if len(mediaTypes) > 1 {
		sendMediaType = mediaTypes[1]

		ui.Log(ui.RestLogger, "Adding media type: %s", sendMediaType)
	}

	r.Header.Add("Content-Type", sendMediaType)
	r.Header.Add("Accept", receiveMediaType)
	AddAgent(r, agentType)

	if body != nil {
		b, err := json.MarshalIndent(body, "", "  ")
		if err != nil {
			return errors.New(err)
		}

		ui.Log(ui.RestLogger, "Request payload:\n%s", string(b))

		r.SetBody(b)
	}

	// Before we execute the request (which can stall out) let's start a short Go
	// routine whose job will be to put a helpful message to the log that we're trying
	// if the request takes too long. We only do this when running as a command client,
	// not when running as an environment with user code.
	stillWaiting.Store(true)

	if v, found := symbols.RootSymbolTable.Get(defs.UserCodeRunningVariable); found && !data.Bool(v) {
		go func() {
			time.Sleep(1 * time.Second)

			for stillWaiting.Load() {
				ui.Say(i18n.M("rest.waiting", map[string]interface{}{"URL": url}))
				time.Sleep(3 * time.Second)
			}
		}()
	}

	defer func() {
		stillWaiting.Store(false)
	}()

	// Execute the request. This could wait for a while...
	resp, err = r.Execute(method, url)
	if err != nil {
		ui.Log(ui.RestLogger, "REST failed, %v", err)

		return errors.New(err)
	}

	status := resp.StatusCode()

	ui.Log(ui.RestLogger, "Status: %d", status)

	if status != http.StatusOK && response == nil {
		switch status {
		case http.StatusUnauthorized:
			return errors.ErrNoCredentials.Context(url)

		case http.StatusForbidden:
			return errors.ErrNoPermission.Context(url)

		case http.StatusInternalServerError:
			return errors.ErrServerError.Context(url)

		case http.StatusBadRequest:
			return errors.ErrInvalidRequest.Context(url)

		case http.StatusNotFound:
			return errors.ErrURLNotFound.Context(url)
		}

		return errors.ErrHTTP.Context(status)
	}

	if replyMedia := resp.Header().Get("Content-Type"); replyMedia != "" {
		ui.Log(ui.RestLogger, "Reply media type: %s", replyMedia)
	}

	if serverHeader := resp.Header().Get(defs.EgoServerInstanceHeader); serverHeader != "" {
		ui.Log(ui.RestLogger, "Server header: %s", serverHeader)
	}

	// If there was an error, and the runtime rest automatic error handling is enabled,
	// try to find the message text in the response, and if found, form an error response
	// to the local caller using that text.
	if (status < 200 || status > 299) && settings.GetBool(defs.RestClientErrorSetting) {
		errorResponse := map[string]interface{}{}

		err := json.Unmarshal(resp.Body(), &errorResponse)
		if err == nil {
			if msg, found := errorResponse["msg"]; found {
				ui.Log(ui.RestLogger, "Response payload:\n%v", string(resp.Body()))

				return errors.Message(data.String(msg))
			}

			if msg, found := errorResponse["message"]; found {
				ui.Log(ui.RestLogger, "Response payload:\n%v", string(resp.Body()))

				return errors.Message(data.String(msg))
			}
		}
	}

	if response != nil {
		body := string(resp.Body())
		if body != "" {
			if !util.InList(body[0:1], "{", "[", "\"") {
				r := defs.RestStatusResponse{
					Status:  resp.StatusCode(),
					Message: strings.TrimSuffix(body, "\n"),
				}
				b, _ := json.Marshal(r)
				body = string(b)
			}

			if s, ok := response.(*data.Struct); ok {
				m := map[string]interface{}{}

				err = json.Unmarshal([]byte(body), &m)
				if err == nil && ui.IsActive(ui.RestLogger) {
					responseBytes, _ := json.MarshalIndent(response, "", "  ")

					ui.Log(ui.RestLogger, "Response payload:\n%s", string(responseBytes))
				}

				fieldList := s.FieldNames(true)
				if len(fieldList) == 0 {
					for k, v := range m {
						s.SetAlways(k, v)
					}
				} else {
					for _, field := range fieldList {
						if v, found := m[field]; found {
							s.SetAlways(field, v)
						}
					}
				}
			} else {
				err = json.Unmarshal([]byte(body), response)
				if err == nil && ui.IsActive(ui.RestLogger) {
					responseBytes, _ := json.MarshalIndent(response, "", "  ")

					ui.Log(ui.RestLogger, "Response payload:\n%s", string(responseBytes))
				}

				if err == nil && status != http.StatusOK {
					if m, ok := response.(map[string]interface{}); ok {
						if msg, ok := m["Message"]; ok {
							err = errors.Message(data.String(msg))
						}
					}
				}
			}
		}
	}

	if err != nil {
		err = errors.New(err)
	}

	return err
}

func GetTLSConfiguration() (*tls.Config, error) {
	var (
		err      error
		b        []byte
		filename = ServerCertificateFile
	)

	// If we haven't ever set up the TLS configuration, let's do so now. This is a serialized
	// operation, so for most use cases, the existing TLS configuration is used. For the first
	// case, we will either set up an insecure TLS (not recommended) or will use the CRT and
	// KEY files found in the EGO PATH if present. If no cert files found, then it assumes it
	// should just use the native certs.
	tlsConfigurationMutex.Lock()

	kind := "default system trust store"

	if tlsConfiguration == nil {
		if mode := settings.Get(defs.RestClientServerCert); mode == "" {
			tlsConfiguration = &tls.Config{}
		} else {
			kind = "using certificate file"

			// If the configuration value has a non-empty value, use that as the filename
			// for the server certificate unless it has already been set by the environment
			// variable.
			if filename == "https-server.crt" && mode != "default" {
				filename = mode
			}

			// If insecure is specified, then skip verification for TLS
			if os.Getenv(defs.EgoInsecureClientEnv) == defs.True {
				tlsConfiguration = &tls.Config{InsecureSkipVerify: true}
				kind = "skipping server verification"
			} else {
				// Is there a server cert file we can/should be using?
				b, err = os.ReadFile(filename)
				if err != nil {
					path := ""
					if libpath := settings.Get(defs.EgoLibPathSetting); libpath != "" {
						path = libpath
					} else {
						path = filepath.Join(settings.Get(defs.EgoPathSetting), defs.LibPathName)
					}

					filename = filepath.Join(path, ServerCertificateFile)
					b, err = os.ReadFile(filename)
				}

				if err == nil {
					kind = kind + " " + filename
					roots := x509.NewCertPool()

					ok := roots.AppendCertsFromPEM(b)
					if !ok {
						ui.Log(ui.RestLogger, "Failed to parse root certificate for client configuration")

						return nil, errors.ErrCertificateParseError.Context(filename)
					} else {
						tlsConfiguration = &tls.Config{RootCAs: roots}
					}
				} else {
					ui.Log(ui.RestLogger, "Failed to read server certificate file: %v", err)

					tlsConfiguration = &tls.Config{}
					kind = "using system default config"
				}
			}
		}

		ui.Log(ui.RestLogger, "Client TLS %s", kind)
	}

	tlsConfigurationMutex.Unlock()

	return tlsConfiguration, nil
}
