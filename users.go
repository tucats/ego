package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/tucats/gopackages/app-cli/cli"
	"github.com/tucats/gopackages/app-cli/persistence"
	"github.com/tucats/gopackages/app-cli/ui"
	"github.com/tucats/gopackages/symbols"
	"github.com/tucats/gopackages/util"
)

type user struct {
	Password    string          `json:"password"`
	Permissions map[string]bool `json:"permissions"`
}

var userDatabase map[string]user

// loadUserDatabase uses command line options to locate and load the authorized users
// database, or initialize it to a helpful default.
func loadUserDatabase(c *cli.Context) error {

	defaultUser := "admin"
	defaultPassword := "{password}"
	if up := persistence.Get("default-credential"); up != "" {
		if pos := strings.Index(up, ":"); pos >= 0 {
			defaultUser = up[:pos]
			defaultPassword = up[pos+1:]
		} else {
			defaultUser = up
			defaultPassword = ""
		}
	}

	// Is there a user database to load?
	userFile, _ := c.GetString("users")
	if userFile == "" {
		userFile = persistence.Get("logon-userdata")
	}
	if userFile != "" {
		b, err := ioutil.ReadFile(userFile)
		if err == nil {
			err = json.Unmarshal(b, &userDatabase)
		}
		if err != nil {
			return err
		}
		ui.Debug(ui.ServerLogger, "Using stored credentials with %d items", len(userDatabase))
	} else {
		userDatabase = map[string]user{
			defaultUser: {Password: defaultPassword},
		}
		ui.Debug(ui.ServerLogger, "Using default credentials with user %s", defaultUser)
	}

	// If there is a --superuser specified on the command line, or in the persistent profile data,
	// mark that user as having ROOT privileges
	var err error
	su, ok := c.GetString("superuser")
	if !ok {
		su = persistence.Get("logon-superuser")
	}
	if su != "" {
		err = setPermission(su, "root", true)
	}
	return err
}

// setPermission sets a given permission string to true for a given user. Returns an error
// if the username does not exist.
func setPermission(user, privilege string, enabled bool) error {
	var err error
	privname := strings.ToLower(privilege)
	if u, ok := userDatabase[user]; ok {
		if u.Permissions == nil {
			u.Permissions = map[string]bool{}
		}
		u.Permissions[privname] = enabled
		userDatabase[user] = u
		ui.Debug(ui.ServerLogger, "Setting %s privilege for user \"%s\" to %v", privname, user, enabled)
	} else {
		err = fmt.Errorf("no such user: %s", user)
	}
	return err
}

// getPermission returns a boolean indicating if the given username and privilege are valid and
// set. If the username or privilege does not exist, then the reply is always false
func getPermission(user, privilege string) bool {
	privname := strings.ToLower(privilege)
	if u, ok := userDatabase[user]; ok {
		if u.Permissions != nil {
			if p, ok := u.Permissions[privname]; ok {
				ui.Debug(ui.ServerLogger, "Check %s permission for user \"%s\" (%v)", privilege, user, p)
				return p
			}
		}
	}
	ui.Debug(ui.ServerLogger, "Check %s permission for user \"%s\" (false)", privilege, user)
	return false
}

// validatePassword checks a username and password against the databse and
// returns true if the user exists and the password is valid
func validatePassword(user, pass string) bool {
	ok := false
	if p, userExists := userDatabase[user]; userExists {
		realPass := p.Password
		// If the password in the database is quoted, do a local hash
		if strings.HasPrefix(realPass, "{") && strings.HasSuffix(realPass, "}") {
			realPass = hashString(realPass[1 : len(realPass)-1])
		}
		hashPass := hashString(pass)
		ok = realPass == hashPass
	}
	return ok
}

// hashString converts a given string to it's hash. This is used to manage
// passwords
func hashString(s string) string {
	h := sha256.New()
	_, _ = h.Write([]byte(s))
	v := h.Sum(nil)

	var r strings.Builder
	for _, b := range v {
		r.WriteString(fmt.Sprintf("%02x", b))
	}
	return r.String()
}

// Authenticated implmeents the Authenticated(user,pass) function. This accepts a username
// and password string, and determines if they are authenticated using the
// users database.
func Authenticated(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {

	var user, pass string

	// If there are no arguments, then we look for the _user and _password
	// variables and use those. Otherwise, fetch them as the two parameters.
	if len(args) == 0 {
		if ux, ok := s.Get("_user"); ok {
			user = util.GetString(ux)
		}
		if px, ok := s.Get("_password"); ok {
			pass = util.GetString(px)
		}
	} else {
		if len(args) != 2 {
			return false, fmt.Errorf("incorrect number of arguments")
		}
		user = util.GetString(args[0])
		pass = util.GetString(args[1])
	}

	// If no user database, then we're done.
	if userDatabase == nil {
		return false, nil
	}

	// If the user exists and the password matches then valid.
	return validatePassword(user, pass), nil
}

// Permission implements the Permission(user,priv) function.
func Permission(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {

	var user, priv string

	if len(args) != 2 {
		return false, fmt.Errorf("incorrect number of arguments")
	}
	user = util.GetString(args[0])
	priv = strings.ToUpper(util.GetString(args[1]))

	// If no user database, then we're done.
	if userDatabase == nil {
		return false, nil
	}

	// If the user exists and the privilege exists, return it's status
	return getPermission(user, priv), nil
}
