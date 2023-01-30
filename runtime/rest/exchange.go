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
	"time"

	"github.com/go-resty/resty"
	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/util"
)

// ServerCertificateFile is the default file name for the server certificate.
var ServerCertificateFile = envDefault("EGO_CERT_FILE", "https-server.crt")

// ServerKeyFile is the default file name for the server key.
var ServerKeyFile = envDefault("EGO_KEY_FILE", "https-server.key")

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
	var resp *resty.Response

	var err error

	url := settings.Get(defs.ApplicationServerSetting)
	if url == "" {
		url = settings.Get(defs.LogonServerSetting)
	}

	if url == "" {
		url = "http://localhost:8080"
	}

	url = strings.TrimSuffix(url, "/") + endpoint

	ui.Log(ui.RestLogger, "%s %s", strings.ToUpper(method), url)

	client := resty.New().SetRedirectPolicy(resty.FlexibleRedirectPolicy(MaxRedirectCount))

	// Unless this is a open (un-authenticate) service, let's verify that the
	// authentication token is still valid.
	if util.InList(endpoint, openServices...) {
		ui.Log(ui.RestLogger, "Endpoint %s does not require token", endpoint)
	} else {
		if token := settings.Get(defs.LogonTokenSetting); token != "" {
			// Let's check to see if it's expired already... Note we skip this if the
			// agent string is "status".
			if !strings.EqualFold(agentType, defs.StatusAgent) {
				if expirationString := settings.Get(defs.LogonTokenExpirationSetting); expirationString != "" {
					expireTime, err := time.Parse(time.UnixDate, expirationString)
					if err != nil {
						return errors.NewError(err)
					}

					now := time.Since(expireTime)
					if now > 0 {
						ui.Say("Your login has expired. Use the ego logon command to login again to %s", settings.Get(defs.LogonServerSetting))

						os.Exit(1)
					}
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
			return errors.NewError(err)
		}

		ui.Log(ui.RestLogger, "Request payload:\n%s", string(b))

		r.SetBody(b)
	}

	resp, err = r.Execute(method, url)
	if err != nil {
		ui.Log(ui.RestLogger, "REST failed, %v", err)

		return errors.NewError(err)
	}

	if err == nil {
		status := resp.StatusCode()

		ui.Log(ui.RestLogger, "Status: %d", status)

		if err == nil && status != http.StatusOK && response == nil {
			return errors.ErrHTTP.Context(status)
		}

		if replyMedia := resp.Header().Get("Content-Type"); replyMedia != "" {
			ui.Log(ui.RestLogger, "Reply media type: %s", replyMedia)
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

					return errors.NewMessage(data.String(msg))
				}

				if msg, found := errorResponse["message"]; found {
					ui.Log(ui.RestLogger, "Response payload:\n%v", string(resp.Body()))

					return errors.NewMessage(data.String(msg))
				}
			}
		}

		if err == nil && response != nil {
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

				err = json.Unmarshal([]byte(body), response)
				if err == nil && ui.IsActive(ui.RestLogger) {
					responseBytes, _ := json.MarshalIndent(response, "", "  ")

					ui.Log(ui.RestLogger, "Response payload:\n%s", string(responseBytes))
				}

				if err == nil && status != http.StatusOK {
					if m, ok := response.(map[string]interface{}); ok {
						if msg, ok := m["Message"]; ok {
							err = errors.NewMessage(data.String(msg))
						}
					}
				}
			}
		}
	}

	if err != nil {
		err = errors.NewError(err)
	}

	return err
}

func GetTLSConfiguration() (*tls.Config, error) {
	// If we haven't ever set up the TLS configuration, let's do so now. This is a serialized
	// operation, so for most use cases, the existing TLS configuration is used. For the first
	// case, we will either set up an insecure TLS (not recommended) or will use the CRT and
	// KEY files found in the EGO PATH if present. If no cert files found, then it assumes it
	// should just use the native certs.
	tlsConfigurationMutex.Lock()

	if tlsConfiguration == nil {
		kind := "using certificate file"

		// If insecure is specified, then skip verification for TLS
		if os.Getenv("EGO_INSECURE_CLIENT") == defs.True {
			tlsConfiguration = &tls.Config{InsecureSkipVerify: true}
			kind = "skipping server verification"
		} else {
			// Is there a server cert file we can/should be using?
			var err error

			var b []byte

			filename := ServerCertificateFile

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

				return nil, errors.NewError(err)
			}
		}

		ui.Log(ui.RestLogger, "Client TLS %s", kind)
	}

	tlsConfigurationMutex.Unlock()

	return tlsConfiguration, nil
}
