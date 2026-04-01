package webapp

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/defs"
)

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

	fmt.Fprint(w, string(indexHTML))
}
