package rest

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/util"
	"gopkg.in/resty.v1"
)

func storeResponse(restResponse *resty.Response, response any, err error) error {
	status := restResponse.StatusCode()

	body := string(restResponse.Body())
	if body != "" {
		// If the body doesn't contain jSON, then convert it to a response body structure type,
		// using the text of the response as the message into the response object.
		body = convertRawTextToResponseBody(body, restResponse)

		if s, ok := response.(*data.Struct); ok {
			m := map[string]any{}

			err = json.Unmarshal([]byte(body), &m)
			if err == nil && ui.IsActive(ui.RestLogger) {
				responseBytes, _ := json.MarshalIndent(response, ui.JSONIndentPrefix, ui.JSONIndentSpacer)

				ui.Log(ui.RestLogger, "rest.response.payload",
					ui.A{
						"body": string(responseBytes)})
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
				responseBytes, _ := json.MarshalIndent(response, ui.JSONIndentPrefix, ui.JSONIndentSpacer)

				ui.Log(ui.RestLogger, "rest.response.payload", ui.A{
					"body": string(responseBytes)})
			}

			if err == nil && status != http.StatusOK {
				if m, ok := response.(map[string]any); ok {
					if msg, ok := m["Message"]; ok {
						err = errors.Message(data.String(msg))
					}
				}
			}
		}
	}

	return err
}

// If the text of the body isn't a valid JSON object, then convert it to a REST status response body, which
// contains a structure with the status, server info, etc. and the body text is supplied as a message.
func convertRawTextToResponseBody(body string, restResponse *resty.Response) string {
	if !util.InList(body[0:1], "{", "[", "\"") {
		r := defs.RestStatusResponse{
			Status:  restResponse.StatusCode(),
			Message: strings.TrimSuffix(body, "\n"),
		}

		// If there was a server header we can extract the server UUID and session number from, do that and
		// put them in the ServerInfo part of the rest response object.
		if serverHeaders := restResponse.Header()[defs.EgoServerInstanceHeader]; len(serverHeaders) > 0 {
			parts := strings.SplitN(serverHeaders[0], ":", 2)
			r.ServerInfo.ID = parts[0]
			r.ServerInfo.Session = data.IntOrZero(parts[1])
		}

		b, _ := json.Marshal(r)
		body = string(b)
	}

	return body
}

// For a given status and url, return a native Ego error. If the status is a well-known value,
// map it to the corresponding Ego error. Otherwise, return a generic HTTP error.
func mapStatusToError(status int, url string) error {
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
