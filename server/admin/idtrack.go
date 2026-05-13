package admin

import (
	"net/http"

	"github.com/tucats/ego/router"
	"github.com/tucats/ego/server/assets"
	"github.com/tucats/ego/util"
)

// IDTrackHandler serves the idtrack issue-tracker UI. It loads the HTML
// asset and writes it to the response, analogous to UIHandler for the dashboard.
func IDTrackHandler(session *router.Session, w http.ResponseWriter, r *http.Request) int {
	uiAsset, _, err := assets.Loader(session.ID, "/assets/idtrack/idtrack.html", assets.StartOfData, assets.EndOfData)
	if err != nil {
		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
	}

	w.Header().Set("Content-Type", "text/html")

	_, err = w.Write(uiAsset)
	if err != nil {
		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
	}

	return http.StatusOK
}
