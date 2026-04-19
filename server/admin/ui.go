package admin

import (
	"net/http"

	"github.com/tucats/ego/server/assets"
	"github.com/tucats/ego/server/server"
	"github.com/tucats/ego/util"
)

// UIHandler launches the dashboard UI. The handler loads the required html
// assets and writes them to the response writer. The bulk of the UI functionality
// is found in the dashboard/dashboard.html file, and it's associated CSS and
// JavaScript.
func UIHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	// Load the dashboard.html asset from the local asset store. We don't care about it's size.
	uiAsset, _, err := assets.Loader(session.ID, "/assets/dashboard/dashboard.html", assets.StartOfData, assets.EndOfData)
	if err != nil {
		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
	}

	// Set the Content-Type header to text/html.
	w.Header().Set("Content-Type", "text/html")

	// Write the dashboard.html content to the response writer.
	_, err = w.Write(uiAsset)
	if err != nil {
		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
	}

	return http.StatusOK
}
