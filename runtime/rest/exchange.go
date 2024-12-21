package rest

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/i18n"
	"github.com/tucats/ego/symbols"
	"gopkg.in/resty.v1"
)

// Exchange is a helper wrapper around a rest call. This is generally used by all the
// CLI client operations _except_ the logon operation, since at that point the token
// is not known (or used).
func Exchange(endpoint, method string, body interface{}, response interface{}, agentType string, mediaTypes ...string) error {
	var (
		restResponse *resty.Response
		err          error
		stillWaiting atomic.Bool
	)

	// Is there a configuration override for the insecure setting we should check before doing a call?
	if settings.GetBool(defs.InsecureClientSetting) {
		ui.Log(ui.RestLogger, "Configuration profile allows insecure client")
		AllowInsecure(true)
	}

	// If the endpoint already has a full URL (i.e. starts with scheme) then just use it as-is. Otherwise,
	// find the server that should be prepended to the endpoint string to form the full URL
	url := applyDefaultServer(endpoint)

	ui.Log(ui.RestLogger, "%s %s", strings.ToUpper(method), url)

	// Initialize and configure a new REST client. This also validates that there is a token if one is
	// needed, and it (probably) hasn't expired yet.
	client, err := newClient(endpoint, body)
	if err != nil {
		return err
	}

	// Generate a new RESTY request based on this client.
	r := client.NewRequest()

	// Using the optional parameters, validate and add any specific media
	// request types to the request.
	applyMediaTypes(mediaTypes, r)

	// Add the agen type to the request.
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

	if v, found := symbols.RootSymbolTable.Get(defs.UserCodeRunningVariable); found && !data.BoolOrFalse(v) {
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
	restResponse, err = r.Execute(method, url)
	if err != nil {
		ui.Log(ui.RestLogger, "REST failed, %v", err)

		return errors.New(err)
	}

	status := restResponse.StatusCode()

	ui.Log(ui.RestLogger, "Status: %d", status)

	if status != http.StatusOK && response == nil {
		return mapStatusToError(status, url)
	}

	if replyMedia := restResponse.Header().Get("Content-Type"); replyMedia != "" {
		ui.Log(ui.RestLogger, "Reply media type: %s", replyMedia)
	}

	if serverHeader := restResponse.Header().Get(defs.EgoServerInstanceHeader); serverHeader != "" {
		ui.Log(ui.RestLogger, "Server header: %s", serverHeader)
	}

	// If there was an error, and the runtime rest automatic error handling is enabled,
	// try to find the message text in the response, and if found, form an error response
	// to the local caller using that text.
	if (status < 200 || status > 299) && settings.GetBool(defs.RestClientErrorSetting) {
		errorResponse := map[string]interface{}{}

		err := json.Unmarshal(restResponse.Body(), &errorResponse)
		if err == nil {
			if msg, found := errorResponse["msg"]; found {
				ui.Log(ui.RestLogger, "Response payload:\n%v", string(restResponse.Body()))

				return errors.Message(data.String(msg))
			}

			if msg, found := errorResponse["message"]; found {
				ui.Log(ui.RestLogger, "Response payload:\n%v", string(restResponse.Body()))

				return errors.Message(data.String(msg))
			}
		}
	}

	if response != nil {
		err = storeResponse(restResponse, response, err)
	}

	if err != nil {
		err = errors.New(err)
	}

	return err
}

// Lets figure out what media types we're sending and reciving. By default, they
// are anonymous JSON. But if the call included one or two strings, they are used
// as the receiving and sending media types respectively.
func applyMediaTypes(mediaTypes []string, r *resty.Request) {
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
}
