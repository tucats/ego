package ui

import (
	"net/http"
	"sort"
	"text/template"

	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/server/assets"
	"github.com/tucats/ego/server/dsns"
	"github.com/tucats/ego/server/server"
	"github.com/tucats/ego/util"
)

// Generate html page that shows a table where each row contains the fields from an array of DSNS.
// This uses a template to generate the HTML page, loaded from the assets cache. The template also
// internally includes a style sheet reference, which is also loaded from the assets cache by the
// client browser.
func HTMLDataSourceNamesHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	// Get the HTML template text from the assets cache.
	htmlPage, err := assets.Loader(session.ID, "/assets/ui-dsns-table.html")
	if err != nil {
		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
	}

	// Generate a new template from the HTML template text.
	t, err := template.New("dsns_page").Parse(string(htmlPage))
	if err != nil {
		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
	}

	// Get the map of the data source names from the internal service.
	dsns, err := dsns.DSNService.ListDSNS(session.User)
	if err != nil {
		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
	}

	// Make a sorted array of the key names from the dsn map so we can iterate over them in
	// the template.
	keys := make([]string, 0, len(dsns))
	for key := range dsns {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	dsnList := []defs.DSN{}
	for _, key := range keys {
		dsnList = append(dsnList, dsns[key])
	}

	// Execute the template, passing in the array of DSN objects. The resulting HTML text is
	// written directly to the response writer.
	if err = t.Execute(w, dsnList); err != nil {
		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
	}

	return http.StatusOK
}
