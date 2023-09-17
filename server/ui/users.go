package ui

import (
	"net/http"
	"sort"
	"strings"
	"text/template"

	"github.com/tucats/ego/server/assets"
	"github.com/tucats/ego/server/auth"
	"github.com/tucats/ego/server/server"
	"github.com/tucats/ego/util"
)

type userData struct {
	Name        string
	Permissions string
}

// Generate html page that shows a table where each row contains the fields from an array of users.
// This uses a template to generate the HTML page, loaded from the assets cache. The template also
// internally includes a style sheet reference, which is also loaded from the assets cache by the
// client browser.
func HTMLUsersHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	// Get the HTML template text the assets cache.
	htmlPage, err := assets.Loader(session.ID, "/assets/ui-users-table.html")
	if err != nil {
		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
	}

	// Generate a new template from the HTML template text.
	t, err := template.New("users_page").Parse(string(htmlPage))
	if err != nil {
		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
	}

	// Get list of users from the internal service.
	users := auth.AuthService.ListUsers()

	// Make a sorted array of the key names from the dsn map so we can iterate over them in
	// the template.
	keys := make([]string, 0, len(users))
	for key := range users {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	userList := []userData{}

	for _, key := range keys {
		userList = append(userList, userData{
			Name:        key,
			Permissions: strings.Join(users[key].Permissions, ", "),
		})
	}

	// Execute the template, passing in the array of DSN objects. The resulting HTML text is
	// written directly to the response writer.
	if err = t.Execute(w, userList); err != nil {
		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
	}

	return http.StatusOK
}
