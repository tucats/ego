package rest

import (
	"crypto/tls"
	"crypto/x509"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
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

// Construct a new go-resty client. This includes validating the token (or getting the token from the)
// body of the request if needed), setting timeout and redirect policies.
func newClient(endpoint string, body interface{}) (*resty.Client, error) {
	client := resty.New().SetRedirectPolicy(resty.FlexibleRedirectPolicy(MaxRedirectCount))

	// Unless this is a open (un-authenticate) service, let's verify that the authentication token is still valid.
	if util.InList(endpoint, openServices...) {
		ui.Log(ui.RestLogger, "rest.no.token",
			"path", endpoint)
	} else {
		// if this is the check for authentication, use the body as the token.
		if strings.HasSuffix(endpoint, "/services/admin/authenticate/") {
			token := data.String(body)
			client.SetAuthToken(token)
		} else if token := settings.Get(defs.LogonTokenSetting); token != "" {
			// Let's check to see if it's expired already...
			if expirationString := settings.Get(defs.LogonTokenExpirationSetting); expirationString != "" {
				expireTime, err := time.Parse(time.UnixDate, expirationString)
				if err != nil {
					return nil, errors.New(err)
				}

				tokenAge := time.Since(expireTime)
				if tokenAge > 0 {
					ui.Say("Your login has expired. Use the ego logon command to login again to %s",
						settings.Get(defs.LogonServerSetting))

					os.Exit(1)
				}
			}

			client.SetAuthToken(token)
			loggableToken := token

			if len(loggableToken) > 9 {
				loggableToken = loggableToken[:4] + "..." + loggableToken[len(loggableToken)-4:]
			}

			ui.Log(ui.RestLogger, "rest.auth.bearer",
				"token", loggableToken)
		}
	}

	if config, err := GetTLSConfiguration(); err != nil {
		return nil, err
	} else {
		client.SetTLSClientConfig(config)
	}

	// Get the maximum timeout for a REST call if there is duration in the configuration. A setting of
	// an empty string, "0", or "none" means no timeout. Otherwise, the value is a duration string.
	if t := settings.Get(defs.RestClientTimeoutSetting); t != "" {
		if t != "0" && t != "0s" && t != "none" {
			timeout, err := util.ParseDuration(t)
			if err != nil {
				return nil, errors.New(err)
			}

			client.SetTimeout(timeout)
		}
	}

	return client, nil
}

func applyDefaultServer(endpoint string) string {
	var url string

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

	return url
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
						ui.Log(ui.RestLogger, "rest.cert.parse.error")

						return nil, errors.ErrCertificateParseError.Context(filename)
					} else {
						tlsConfiguration = &tls.Config{RootCAs: roots}
					}
				} else {
					ui.Log(ui.RestLogger, "rest.cert.read.error",
						"error", err)

					tlsConfiguration = &tls.Config{}
					kind = "using system default config"
				}
			}
		}

		ui.Log(ui.RestLogger, "rest.tls",
			"status", kind)
	}

	tlsConfigurationMutex.Unlock()

	return tlsConfiguration, nil
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
