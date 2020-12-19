package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/tucats/gopackages/app-cli/ui"
	"github.com/tucats/gopackages/symbols"
	"github.com/tucats/gopackages/util"
)

// UserHandler is the rest handler for /admin/user endpoint
// operations
func UserHandler(w http.ResponseWriter, r *http.Request) {

	ui.Debug(ui.ServerLogger, "%s %s", r.Method, r.URL.Path)
	w.Header().Add("Content_Type", "application/json")

	user, hasAdminPrivs := isAdminRequestor(r)
	if !hasAdminPrivs {
		ui.Debug(ui.ServerLogger, "User %s not authorized", user)
		w.WriteHeader(403)
		msg := `{ "status" : 403, "msg" : "Not authorized" }`
		_, _ = io.WriteString(w, msg)
		return
	}

	var err error
	// Currently we only support POST & DELETE
	if r.Method != "POST" && r.Method != "DELETE" {
		w.WriteHeader(418)
		msg := `{ "status" : 418, "msg" : "Unsupported method %s" }`
		_, _ = io.WriteString(w, fmt.Sprintf(msg, r.Method))
		return
	}
	// Get the payload which must be a user spec in JSON
	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(r.Body)

	type userData struct {
		Name        string   `json:"name"`
		Password    string   `json:"password"`
		Permissions []string `json:"permissions"`
	}
	u := userData{Permissions: []string{}}
	err = json.Unmarshal(buf.Bytes(), &u)
	verb := "made no change to"

	if err == nil {
		s := symbols.NewSymbolTable(r.URL.Path)
		_ = s.SetAlways("_superuser", true)
		switch strings.ToUpper(r.Method) {

		case "POST":
			_, err = SetUser(s, []interface{}{map[string]interface{}{
				"name":        u.Name,
				"password":    u.Password,
				"permissions": u.Permissions}})
			verb = "updated"

		case "DELETE":
			v, err := DeleteUser(s, []interface{}{u.Name})
			if err == nil && !util.GetBool(v) {
				w.WriteHeader(404)
				msg := `{ "status" : 404, "msg" : "No username entry for '%s'" }`
				_, _ = io.WriteString(w, fmt.Sprintf(msg, u.Name))
				ui.Debug(ui.ServerLogger, "404 No such user")
				return
			}
			verb = "deleted"
		}
	}

	// Clean up and go home
	if err != nil {
		w.WriteHeader(500)
		msg := `{ "status" : 500, "msg" : "%s"`
		_, _ = io.WriteString(w, fmt.Sprintf(msg, err.Error()))
		ui.Debug(ui.ServerLogger, "500 Internal server error %v", err)
	} else {
		w.WriteHeader(200)
		msg := `{ "status" : 200, "msg" : "Successfully %s entry for '%s'" }`
		_, _ = io.WriteString(w, fmt.Sprintf(msg, verb, u.Name))
		ui.Debug(ui.ServerLogger, "200 Success")
	}

}

// UserListHandler is the rest handler for /admin/user endpoint
// operations
func UserListHandler(w http.ResponseWriter, r *http.Request) {

	ui.Debug(ui.ServerLogger, "%s %s", r.Method, r.URL.Path)
	w.Header().Add("Content_Type", "application/json")

	user, hasAdminPrivs := isAdminRequestor(r)
	if !hasAdminPrivs {
		ui.Debug(ui.ServerLogger, "User %s not authorized", user)
		w.WriteHeader(403)
		msg := `{ "status" : 403, "msg" : "Not authorized" }`
		_, _ = io.WriteString(w, msg)
		return
	}

	var err error
	// Currently we only support POST & DELETE
	if r.Method != "GET" {
		w.WriteHeader(418)
		msg := `{ "status" : 418, "msg" : "Unsupported method %s" }`
		_, _ = io.WriteString(w, fmt.Sprintf(msg, r.Method))
		return
	}

	type userData struct {
		Name        string   `json:"name"`
		Permissions []string `json:"permissions"`
	}
	type userList []userData
	result := userList{}
	for k, u := range userDatabase {
		ud := userData{}
		ud.Name = k
		ud.Permissions = []string{}
		for p, v := range u.Permissions {
			if v {
				ud.Permissions = append(ud.Permissions, p)
			}
		}
		result = append(result, ud)
	}

	b, err := json.Marshal(result)
	w.WriteHeader(200)
	_, _ = w.Write(b)

	// Clean up and go home
	if err != nil {
		w.WriteHeader(500)
		msg := `{ "status" : 500, "msg" : "%s"`
		_, _ = io.WriteString(w, fmt.Sprintf(msg, err.Error()))
		ui.Debug(ui.ServerLogger, "500 Internal server error %v", err)
	}
}

// For a given userid, indicate if this user exists and has admin privileges
func isAdminRequestor(r *http.Request) (string, bool) {
	var user string

	hasAdminPrivs := false
	auth := r.Header.Get("Authorization")
	if auth == "" {
		ui.Debug(ui.ServerLogger, "No authentication credentials given")
		return "<invalid>", false
	}

	// IF the authorization header has the auth scheme prefix, extract and
	// validate the token
	if strings.HasPrefix(strings.ToLower(auth), AuthScheme) {
		token := strings.TrimSpace(strings.TrimPrefix(strings.ToLower(auth), AuthScheme))
		ui.Debug(ui.ServerLogger, "Auth using token %s...", token[:20])
		if validateToken(token) {
			user := tokenUser(token)
			if user == "" {
				ui.Debug(ui.ServerLogger, "No username associated with token")
			}
			hasAdminPrivs = getPermission(user, "root")
		} else {
			ui.Debug(ui.ServerLogger, "No valid token presented")
		}
	} else {
		// Not a token, so assume BasicAuth
		user, pass, ok := r.BasicAuth()
		if ok {
			ui.Debug(ui.ServerLogger, "Auth using user %s", user)
			if ok := validatePassword(user, pass); ok {
				hasAdminPrivs = getPermission(user, "root")
			}
		}
	}

	if !hasAdminPrivs && user == "" {
		user = "<invalid>"
	}

	return user, hasAdminPrivs
}
