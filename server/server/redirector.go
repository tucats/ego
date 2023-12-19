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

func ReadRedirects() (map[string]map[string]string, *errors.Error) {
	// Get the default library path
	egoPath := os.Getenv("EGO_PATH")
	if egoPath == "" {
		egoPath = settings.Get(defs.EgoPathSetting)
	}

	// Read the redirector file, if it exists.
	redirectFile := filepath.Join(egoPath, defs.LibPathName, "redirects.json")
	if _, err := os.Stat(redirectFile); nativeerrors.Is(err, os.ErrNotExist) {
		return nil, nil
	}

	b, err := os.ReadFile(redirectFile)
	if err != nil {
		return nil, errors.NewError(err)
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

	b = []byte(strings.Join(lines, "\n"))

	// Parse the JSON
	redirects := make(map[string]map[string]string)
	err = json.Unmarshal(b, &redirects)

	return redirects, errors.NewError(err)
}
