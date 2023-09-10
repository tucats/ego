package ui

import (
	"net/http"
	"sort"
	"text/template"

	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/server/dsns"
	"github.com/tucats/ego/server/server"
	"github.com/tucats/ego/util"
)

// HTML tamples that shows a table where each row contains the fields from an array of DSN objects.
const dsnsHTMLpage = `
<html>
<head>
<title>DSNs</title>
</head>
<body>
<h1>DSNs</h1>
<table style="border:1px solid black;border-collapse:collapse">
<tr style="border:1px solid black;">
<th  style="border:2px solid black;">DSN</th>
<th  style="border:2px solid black;">Driver</th>
<th style="border:2px solid black;" >Database</th>
<th  style="border:2px solid black;">Host</th>
<th  style="border:2px solid black;">Port</th>
</tr>
{{range .}}
<tr style="border:1px solid black;" >
<td style="border:1px solid black;">{{.Name}}</td>
<td style="border:1px solid black;">{{.Provider}}</td>
<td style="border:1px solid black;">{{.Database}}</td>
<td style="border:1px solid black;">{{.Host}}</td>
<td style="border:1px solid black" align="right">{{.Port}}</td>
</tr>
{{end}}
</table>
</body>
</html>
`

// Generate html page that shows a table where each row contains the fields from an array of DSNS.
func HTMLdsnsHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	// Use the local template text to generate the HTML page.
	t, err := template.New("dsns_page").Parse(dsnsHTMLpage)
	if err != nil {
		util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)

		return http.StatusInternalServerError
	}

	// Get the map of the DSNS from the internal service.
	dsns, err := dsns.DSNService.ListDSNS(session.User)
	if err != nil {
		util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)

		return http.StatusInternalServerError
	}

	// Make a sorted array of the key names from the dsn map
	// so we can iterate over them in the template.
	keys := make([]string, 0, len(dsns))
	for key := range dsns {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	dsnList := []defs.DSN{}
	for _, key := range keys {
		dsnList = append(dsnList, dsns[key])
	}

	// Execute the template, passing in the array of DSN objects.
	err = t.Execute(w, dsnList)
	if err != nil {
		util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)

		return http.StatusInternalServerError
	}

	return http.StatusOK
}
