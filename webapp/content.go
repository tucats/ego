package webapp

import (
	"fmt"
	"html"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/defs"
)

// serveJS serves the JavaScript application for the web UI.
func serveJS(w http.ResponseWriter, r *http.Request) {
	egoPath := os.Getenv("EGO_PATH")
	if egoPath == "" {
		egoPath = settings.Get(defs.EgoPathSetting)
	}

	libPath := filepath.Join(egoPath, defs.LibPathName)
	js, err := os.ReadFile(filepath.Join(libPath, "webapp", "app.js"))

	if err != nil {
		http.Error(w, fmt.Sprintf("could not read app.js: %v", err), http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	fmt.Fprint(w, string(js))
}

// serveCSS serves the CSS style sheet for the web UI.
func serveCSS(w http.ResponseWriter, r *http.Request) {
	egoPath := os.Getenv("EGO_PATH")
	if egoPath == "" {
		egoPath = settings.Get(defs.EgoPathSetting)
	}

	libPath := filepath.Join(egoPath, defs.LibPathName)
	css, err := os.ReadFile(filepath.Join(libPath, "webapp", "style.css"))

	if err != nil {
		http.Error(w, fmt.Sprintf("could not read style.css: %v", err), http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	fmt.Fprint(w, string(css))
}

// serveIndex serves the single-page editor UI.
func serveIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)

		return
	}

	// Reject a second browser connection while a session is already active.
	// The ping watchdog sets sessionActive on the first heartbeat and clears
	// it when the tab is closed, so this window is small but real.
	if isSessionActive() {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusConflict)
		fmt.Fprint(w, `<!DOCTYPE html><html><head><meta charset="UTF-8">
<title>Ego Playground — busy</title></head><body>
<h2>Session already in use</h2>
<p>The Ego Playground is already open in another browser tab or window.
Close that session before connecting again.</p>
</body></html>`)

		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// Read the contents of the index.html file from the lib directory,
	// and serve it as the response. Get the location of the lib path
	// by reading the setting for the Ego path.
	egoPath := os.Getenv("EGO_PATH")
	if egoPath == "" {
		egoPath = settings.Get(defs.EgoPathSetting)
	}

	libPath := filepath.Join(egoPath, defs.LibPathName)
	indexHTML, err := os.ReadFile(filepath.Join(libPath, "webapp", "index.html"))

	// If there was an error, format that as the HTML to serve.
	if err != nil {
		http.Error(w, fmt.Sprintf("could not read index.html: %v", err), http.StatusInternalServerError)

		return
	}

	// Load the default starter program from lib/webapp/initial.ego.
	// If that file is missing, fall back to a minimal placeholder so the
	// editor is never completely blank.
	initialEgo, err := os.ReadFile(filepath.Join(libPath, "webapp", "initial.ego"))
	initialSource := string(initialEgo)

	if err != nil {
		initialSource = "// Could not read initial.ego\n"
	}

	if sourceFile != "" {
		data, err := os.ReadFile(sourceFile)
		if err == nil {
			initialSource = string(data)
		} else {
			initialSource = fmt.Sprintf("// Could not read %s: %v", sourceFile, err)
		}
	}

	page := strings.ReplaceAll(string(indexHTML), "{{INITIAL_SOURCE}}", html.EscapeString(initialSource))

	fmt.Fprint(w, page)
}
