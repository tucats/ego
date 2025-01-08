package util

import (
	"net/http"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/errors"
)

// AcceptedMediaType validates the media type in the "Accept" header for this
// request against a list of valid media types. This includes common types that
// are always accepted, as well as additional types provided as paraameters to
// this function call.  The result is a nil error value if the media type is
// valid, else an error indicating that there was an invalid media type found.
func AcceptedMediaType(r *http.Request, validList []string) error {
	var (
		err   error
		found bool
	)

	mediaTypes := r.Header["Accept"]
	for _, mediaType := range mediaTypes {
		// Check for common times that are always accepted.
		if InList(strings.ToLower(mediaType),
			"application/json",
			"application/text",
			"text/plain",
			"text/*",
			"text",
			"*/*",
		) {
			found = true

			continue
		}

		// Special case; the wildcard can have weights and other stuff associated. If it
		// is present at all, we allow it.
		if strings.Contains(mediaType, "*/*") {
			found = true

			continue
		}

		// If not, verify that the media type is in the optional list of additional
		// accepted media types.
		if InList(mediaType, validList...) {
			found = true

			continue
		}
	}

	if !found {
		list := strings.Join(mediaTypes, ", ")

		ui.Log(ui.RouteLogger, "[0] no acceptable media types found in %v", list)

		err = errors.ErrInvalidMediaType.Context(list)
	}

	return err
}
