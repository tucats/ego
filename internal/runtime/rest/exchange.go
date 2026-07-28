package rest

import (
	"encoding/json"
	"net/http"
	"os"
	"reflect"
	"strings"
	"sync/atomic"
	"time"

	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/i18n"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/language/symbols"
	egostrings "github.com/tucats/ego/internal/util/strings"
	"gopkg.in/resty.v1"
)

// Exchange is a helper wrapper around a rest call. This is generally used by all the
// CLI client operations _except_ the logon operation, since at that point the token
// is not known (or used).
func Exchange(endpoint, method string, body any, response any, agentType string, mediaTypes ...string) error {
	var (
		restResponse *resty.Response
		err          error
		stillWaiting atomic.Bool
	)

	// Is there a configuration override for the insecure setting we should check before doing a call?
	if settings.GetBool(defs.InsecureClientSetting) {
		ui.Log(ui.RestLogger, "rest.allow.insecure", nil)
		ui.Say("rest.tls.insecure")
		AllowInsecure(true)
	}

	// If the endpoint already has a full URL (i.e. starts with scheme) then just use it as-is. Otherwise,
	// find the server that should be prepended to the endpoint string to form the full URL
	url := applyDefaultServer(endpoint)

	if ui.IsActive(ui.RestLogger) {
		ui.Log(ui.RestLogger, "rest.method", ui.A{
			"method":   strings.ToUpper(method),
			"endpoint": url})
	}

	// Initialize and configure a new REST client. This also validates that there is a token if one is
	// needed, and it (probably) hasn't expired yet.
	client, err := newClient(endpoint, body)
	if err != nil {
		return err
	}

	// Generate a new RESTY request based on this client.
	r := client.NewRequest()

	// If there is a language specified, add it to the request header.
	if lang := os.Getenv(defs.EgoLangEnv); lang != "" {
		if ui.IsActive(ui.RestLogger) {
			ui.Log(ui.RestLogger, "rest.language", ui.A{
				"language": lang})
		}

		r.Header.Add("Accept-Language", lang)
	}

	// Using the optional parameters, validate and add any specific media
	// request types to the request.
	sendText, _ := applyMediaTypes(mediaTypes, r)

	// Tell the server whether we can accept a compressed response body.
	applyContentEncoding(r)

	// Add the agent type to the request.
	AddAgent(r, agentType)

	if body != nil {
		if sendText {
			r.SetBody(body)
		} else {
			b, err := json.MarshalIndent(body, ui.JSONIndentPrefix, ui.JSONIndentSpacer)
			if err != nil {
				return errors.New(err)
			}

			bodyText := string(b)
			if ui.IsActive(ui.RestLogger) {
				ui.Log(ui.RestLogger, "rest.request.payload", ui.A{
					"body": bodyText})
			}

			r.SetBody([]byte(egostrings.JSONMinify(bodyText)))
		}
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
				ui.Say(i18n.M("rest.waiting", map[string]any{"URL": url}))
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
		if ui.IsActive(ui.RestLogger) {
			status := 0
			if restResponse != nil {
				status = restResponse.StatusCode()
			}

			ui.Log(ui.RestLogger, "rest.error", ui.A{
				"error":  err,
				"status": status})
		}

		return errors.New(err)
	}

	status := restResponse.StatusCode()

	if ui.IsActive(ui.RestLogger) {
		ui.Log(ui.RestLogger, "rest.status", ui.A{
			"status": status})
	}

	// Dump out the response headers if we are in REST logging mode.
	if ui.IsActive(ui.RestLogger) {
		for key, list := range restResponse.Header() {
			ui.Log(ui.RestLogger, "rest.response.header", ui.A{
				"name":   key,
				"values": list,
			})
		}
	}

	// Read the response payload once, up front, undoing any compression that the HTTP
	// client library did not already undo for us. Every use of the payload below works
	// from this slice, so the rest of the function never has to think about encoding.
	bodyBytes := responseBody(restResponse)

	logCompressionSavings(restResponse, len(bodyBytes))

	if status != http.StatusOK && response == nil {
		return mapStatusToError(status, url)
	}

	// Determine if the reply is text or not.
	textReply := false
	if replyMedia := restResponse.Header().Get("Content-Type"); replyMedia != "" {
		textReply = strings.Contains(replyMedia, "text")
	}

	// If there was an error, and the runtime rest automatic error handling is enabled,
	// try to find the message text in the response, and if found, form an error response
	// to the local caller using that text.
	if (status < 200 || status > 299) && settings.GetBool(defs.RestClientErrorSetting) {
		errorResponse := map[string]any{}

		if textReply {
			if v, ok := response.(*string); ok {
				*v = string(bodyBytes)
			} else {
				if v, ok := response.(*[]string); ok {
					*v = strings.Split(string(bodyBytes), "\n")
				} else {
					t := reflect.TypeOf(response).String()
					// We ignore a rest status response on a shutdown. Anything else gets an error
					if t != "*defs.RestStatusResponse" || status != 503 {
						if ui.IsActive(ui.RestLogger) {
							ui.Log(ui.RestLogger, "rest.payload.media", ui.A{
								"type": t,
								"body": string(bodyBytes)})
						}

						return errors.New(errors.ErrInvalidType).Context(t)
					}
				}
			}
		}

		err := json.Unmarshal(bodyBytes, &errorResponse)
		if err == nil {
			// Check for both "msg" and "message" fields
			if msg, found := errorResponse["msg"]; found {
				if ui.IsActive(ui.RestLogger) {
					ui.Log(ui.RestLogger, "rest.response.payload", ui.A{
						"body": string(bodyBytes)})
				}

				// Don't throw the server stopped error as a real error. A 503 status
				// always means the server we just hit is shutting down, regardless of
				// what the (possibly localized) message text says -- comparing against
				// the English-only defs.ServerStoppedMessage text would break once that
				// message is localized for a non-English server.
				if status != http.StatusServiceUnavailable {
					return errors.Message(data.String(msg))
				}
			}

			if msg, found := errorResponse["message"]; found {
				if ui.IsActive(ui.RestLogger) {
					ui.Log(ui.RestLogger, "rest.response.payload", ui.A{
						"body": string(bodyBytes)})
				}

				if ui.IsActive(ui.InternalLogger) {
					ui.Log(ui.InternalLogger, "json.field.error", ui.A{
						"found":    "message",
						"expected": "msg"})
				}

				// Don't throw the server stopped error as a real error. See the comment
				// on the identical check above.
				if status != http.StatusServiceUnavailable {
					return errors.Message(data.String(msg))
				}
			}
		}
	}

	// Successful exchange, what do we do with the reply if we get one?
	if response != nil {
		if textReply {
			if v, ok := response.(*string); ok {
				*v = string(bodyBytes)
			} else {
				if v, ok := response.(*[]string); ok {
					*v = strings.Split(string(bodyBytes), "\n")
				} else {
					t := reflect.TypeOf(response).String()
					// We ignore a text status response on a shutdown. Anything else gets an error
					if (t != "*defs.RestStatusResponse" || status != 503) && ui.IsActive(ui.RestLogger) {
						ui.Log(ui.RestLogger, "rest.payload.media", ui.A{
							"type": t,
							"body": string(bodyBytes)})
					}

					err = storeResponse(restResponse, bodyBytes, response, err)
				}
			}
		} else {
			err = storeResponse(restResponse, bodyBytes, response, err)
		}
	}

	if err != nil {
		err = errors.New(err)
	}

	return err
}

// Lets figure out what media types we're sending and receiving. By default, they
// are anonymous JSON. But if the call included one or two strings, they are used
// as the receiving and sending media types respectively.
// The return value is set if the content is meant to be text (not marshalled as JSON).
func applyMediaTypes(mediaTypes []string, r *resty.Request) (bool, bool) {
	receiveMediaType := defs.JSONMediaType
	sendMediaType := defs.JSONMediaType

	if len(mediaTypes) > 0 {
		receiveMediaType = mediaTypes[0]

		if ui.IsActive(ui.RestLogger) {
			ui.Log(ui.RestLogger, "rest.apply.media", ui.A{
				"media": receiveMediaType})
		}
	}

	if len(mediaTypes) > 1 {
		sendMediaType = mediaTypes[1]

		if ui.IsActive(ui.RestLogger) {
			ui.Log(ui.RestLogger, "rest.apply.media", ui.A{
				"media": sendMediaType})
		}
	}

	r.Header.Add("Content-Type", sendMediaType)
	r.Header.Add("Accept", receiveMediaType)

	// Return flag indicating if the body is meant to be marshalled or not.
	// If true, it's text and should be left alone.
	sendText := strings.Contains(strings.ToLower(sendMediaType), "text")
	receivedText := strings.Contains(strings.ToLower(receiveMediaType), "text")

	return sendText, receivedText
}

// applyContentEncoding tells the server whether this client is willing to receive a
// compressed response body, by setting the request's Accept-Encoding header.
//
// Compressing large responses (server log payloads in particular, which are big and
// highly repetitive) can cut the number of bytes on the network by roughly ten to one.
//
// There is a subtlety worth understanding here. Go's HTTP transport quietly adds
// "Accept-Encoding: gzip" to any outgoing request that does not already set that
// header, and transparently decompresses the reply. That means compression is on by
// default whether or not this function exists. Consequently:
//
//   - Setting the header to "gzip" ourselves is mostly about making the intent visible
//     in logs and packet captures; the behavior is unchanged.
//   - Setting the header to "identity" is the only way to genuinely turn compression
//     off, because merely leaving the header out lets Go put "gzip" back.
//
// Either way the caller receives a fully decoded body, so this setting only changes
// what travels over the network, never what the program sees.
func applyContentEncoding(r *resty.Request) {
	// Compression is enabled unless it has been explicitly turned off in the
	// configuration. An absent setting means "use the default", which is on.
	encoding := "gzip"

	if value := settings.Get(defs.RestClientCompressionSetting); value != "" && !settings.GetBool(defs.RestClientCompressionSetting) {
		encoding = "identity"
	}

	if ui.IsActive(ui.RestLogger) {
		ui.Log(ui.RestLogger, "rest.apply.encoding", ui.A{
			"encoding": encoding})
	}

	r.Header.Set("Accept-Encoding", encoding)
}

// logCompressionSavings writes a REST log entry describing how much a compressed
// response saved on the network, given the size of the payload after decoding.
//
// The size actually received is read from the raw HTTP response's ContentLength, which
// is the value of the response's Content-Length header. That header is not always
// present: when a server does not know the final size before it starts sending (a
// large response written incrementally), it uses "chunked" transfer encoding instead
// and ContentLength is reported as -1. In that case the saving cannot be measured, so
// only the fact of compression is logged.
func logCompressionSavings(restResponse *resty.Response, decodedSize int) {
	if !ui.IsActive(ui.RestLogger) {
		return
	}

	// An empty Content-Encoding means the transport already stripped the header after
	// decompressing, or the response was never compressed at all. Either way there is
	// nothing to report.
	if !strings.EqualFold(restResponse.Header().Get("Content-Encoding"), "gzip") {
		return
	}

	wireSize := -1
	if raw := restResponse.RawResponse; raw != nil {
		wireSize = int(raw.ContentLength)
	}

	percent := 0
	if wireSize >= 0 && decodedSize > 0 {
		percent = 100 - (wireSize * 100 / decodedSize)
	}

	ui.Log(ui.RestLogger, "rest.response.compressed", ui.A{
		"size":    decodedSize,
		"sent":    wireSize,
		"percent": percent})
}
