package rest

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/util"
	"gopkg.in/resty.v1"
)

// gzipMagicFirstByte and gzipMagicSecondByte are the two-byte signature that begins
// every gzip stream (0x1f 0x8b). Every gzip file starts with these bytes, so they are
// a reliable way to recognize a compressed payload without trusting response headers.
const (
	gzipMagicFirstByte  = 0x1f
	gzipMagicSecondByte = 0x8b
)

// responseBody returns the payload of a REST response as plain, usable bytes,
// decompressing it first if it turns out to still be a gzip stream.
//
// In normal operation this decompression never happens, because the underlying HTTP
// client already does it: Go's http.Transport and the resty library both recognize a
// "Content-Encoding: gzip" response and transparently expand the body before any of
// this code sees it. This function exists as a safety net. If the HTTP client library
// is ever replaced with one that does not decompress automatically, every caller here
// would otherwise start handing raw gzip bytes to a JSON parser and failing in a
// confusing way. Checking two bytes on each response is far cheaper than that bug.
//
// The check deliberately looks at the payload itself rather than the Content-Encoding
// header. A client that already decompressed the body also removes that header, so the
// header cannot distinguish "never compressed" from "already decompressed" -- but the
// bytes always can. Ordinary JSON and text payloads cannot begin with 0x1f 0x8b, and
// in the impossible case that one did, gzip.NewReader would reject it and the original
// bytes would be returned unchanged.
func responseBody(restResponse *resty.Response) []byte {
	body := restResponse.Body()

	if len(body) < 2 || body[0] != gzipMagicFirstByte || body[1] != gzipMagicSecondByte {
		return body
	}

	reader, err := gzip.NewReader(bytes.NewReader(body))
	if err != nil {
		// Not actually a valid gzip stream after all; use the bytes as they arrived.
		return body
	}

	defer reader.Close()

	decoded, err := io.ReadAll(reader)
	if err != nil {
		// A truncated or corrupt stream. Returning the original bytes lets the caller
		// report a parsing failure against the real data rather than an empty body.
		return body
	}

	ui.Log(ui.RestLogger, "rest.response.decompressed", ui.A{
		"size": len(body),
		"sent": len(decoded)})

	return decoded
}

// storeResponse unpacks a REST response payload into the caller's response object. The
// body is passed in as bytes rather than read from restResponse here, so that the
// caller can supply a payload that has already been decompressed by responseBody().
func storeResponse(restResponse *resty.Response, bodyBytes []byte, response any, err error) error {
	status := restResponse.StatusCode()

	body := string(bodyBytes)
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
