package server

import (
	"encoding/json"
	nativeerrors "errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
)

type Redirect map[string]map[string]string

// Function that handles redirecting a request to a different URL, as
// defined by the route we are executing.
func Redirector(session *Session, w http.ResponseWriter, r *http.Request) int {
	ui.Log(ui.ServerLogger, "[%d] Redirected %s to %s", session.ID, r.URL.Path, session.Redirect)
	http.Redirect(w, r, session.Redirect, http.StatusMovedPermanently)

	return http.StatusMovedPermanently
}

// Initialize the redirector routes by reading the file "redirects.json" from
// the lib directory, and adding them to the router.
//
// The redirect JSON file is a map of maps. The outmost key is address of the
// local route to redirect. The inner key is the HTTP method (GET, PUT, etc.)
// and the path to redirect the user to. For example, the following JSON has
// a single redirect for the service path "/apple" which, if called with a GET
// method, will redirect to the Apple web site.
//
//		{
//		  "/apple": {
//	     "GET": "www.apple.com"
//		  }
//		}
func (m *Router) InitRedirectors() *errors.Error {
	// Read the redirector file. If the result is a nil map, we have no work to do.
	redirects, err := ReadRedirects()
	if err != nil || redirects == nil {
		return err
	}

	ui.Log(ui.ServerLogger, "Registering static redirects")

	// Add each redirect to the router
	for from, redirect := range redirects {
		for method, to := range redirect {
			m.New(from, Redirector, method).Redirect(to)
			ui.Log(ui.RouteLogger, "  redirect %s %s to %s", method, from, to)
		}
	}

	return err
}

// Read the JSON file that describes the redirect operations. This is read from the
// file "Redirects.json" and is normally found in the lib directory of the Ego path.
func ReadRedirects() (map[string]map[string]string, *errors.Error) {
	// Get the default library path
	egoPath := os.Getenv("EGO_PATH")
	if egoPath == "" {
		egoPath = settings.Get(defs.EgoPathSetting)
	}

	// Form the path name of the file and see if it exists. If it does not exist
	// or cannot be read, we have no work to do.
	redirectFile := filepath.Join(egoPath, defs.LibPathName, "redirects.json")
	if _, err := os.Stat(redirectFile); nativeerrors.Is(err, os.ErrNotExist) {
		return nil, nil
	}

	// Read the contents of the file.
	b, err := os.ReadFile(redirectFile)
	if err != nil {
		return nil, errors.New(err)
	}

	// Convert the byte buffer to an array of strings for each line of
	// text and strip out comment lines that start with "#" or "//".
	// Then reassemble the lines for parsing.
	lines := strings.Split(string(b), "\n")
	for i := 0; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "#") || strings.HasPrefix(lines[i], "//") {
			lines = append(lines[:i], lines[i+1:]...)
			i--
		}
	}

	// Now that blank lines and comments have been removed, reassemble the array of
	// strings back into a byte array, and parse the JSON. Any JSON parsing errors are
	// passed back to the caller.
	b = []byte(strings.Join(lines, "\n"))
	redirects := make(map[string]map[string]string)
	err = json.Unmarshal(b, &redirects)

	return redirects, errors.New(err)
}
