package server

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/tucats/gopackages/app-cli/cli"
	"github.com/tucats/gopackages/app-cli/persistence"
	"github.com/tucats/gopackages/app-cli/ui"
	"github.com/tucats/gopackages/functions"
	"github.com/tucats/gopackages/symbols"
	"github.com/tucats/gopackages/util"
)

type user struct {
	Password    string          `json:"password"`
	Permissions map[string]bool `json:"permissions"`
}

var userDatabase map[string]user
var userDatabaseFile = ""

// loadUserDatabase uses command line options to locate and load the authorized users
// database, or initialize it to a helpful default.
func LoadUserDatabase(c *cli.Context) error {

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
	userDatabaseFile, _ = c.GetString("users")
	if userDatabaseFile == "" {
		userDatabaseFile = persistence.Get("logon-userdata")
	}
	if userDatabaseFile != "" {
		b, err := ioutil.ReadFile(userDatabaseFile)
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
			realPass = HashString(realPass[1 : len(realPass)-1])
		}
		hashPass := HashString(pass)
		ok = realPass == hashPass
	}
	return ok
}

// HashString converts a given string to it's hash. This is used to manage
// passwords
func HashString(s string) string {
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

// Implements the SetUser() function
func SetUser(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {

	var err error

	// Before we do anything else, are we running this call as a superuser?
	superUser := false
	if s, ok := s.Get("_superuser"); ok {
		superUser = util.GetBool(s)
	}
	if !superUser {
		return nil, errors.New("no privilege for operation")
	}

	// There must be one parameter, which is a struct containing
	// the user data
	if len(args) != 1 {
		return nil, errors.New("incorrect number of arguments")
	}

	if u, ok := args[0].(map[string]interface{}); ok {
		name := ""
		if n, ok := u["name"]; ok {
			name = strings.ToLower(util.GetString(n))
		}
		r, ok := userDatabase[name]
		if !ok {
			r = user{Permissions: map[string]bool{}}
		}
		if n, ok := u["password"]; ok {
			r.Password = HashString(util.GetString(n))
		}
		if n, ok := u["permissions"]; ok {
			// If permissions are specified, we clear out all the existing
			// ones and replace them with the new ones we get here.
			r.Permissions = map[string]bool{}
			if m, ok := n.([]interface{}); ok {
				for _, p := range m {
					r.Permissions[util.GetString(p)] = true
				}
			}
		}

		userDatabase[name] = r
		err = updateUserDatabase()

	}
	return true, err
}

// Implements the DeleteUser() function. Returns true if the name was deleted,
// else false if it was not a valid username.
func DeleteUser(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {

	var err error
	// Before we do anything else, are we running this call as a superuser?
	superUser := false
	if s, ok := s.Get("_superuser"); ok {
		superUser = util.GetBool(s)
	}
	if !superUser {
		return nil, errors.New("no privilege for operation")
	}

	// There must be one parameter, which is the username
	if len(args) != 1 {
		return nil, errors.New("incorrect number of arguments")
	}
	name := strings.ToLower(util.GetString(args[0]))

	if _, ok := userDatabase[name]; ok {
		delete(userDatabase, name)
		err = updateUserDatabase()
		return true, err
	}
	return false, nil
}

// Implements the GetUser() function
func GetUser(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	// There must be one parameter, which is a username
	if len(args) != 1 {
		return nil, errors.New("incorrect number of arguments")
	}

	r := map[string]interface{}{}
	name := strings.ToLower(util.GetString(args[0]))
	t, ok := userDatabase[name]
	if !ok {
		return r, nil
	}
	r["name"] = name
	su := false
	perms := []interface{}{}
	for p, f := range t.Permissions {
		if strings.ToLower(p) == "root" {
			su = true
		}
		if f {
			perms = append(perms, p)
		}
	}
	r["permissions"] = perms
	r["superuser"] = su
	return r, nil
}

// updateUserDatabase re-writes the user database file with updated values
func updateUserDatabase() error {

	// Convert the database to a json string

	b, err := json.MarshalIndent(userDatabase, "", "   ")
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(userDatabaseFile, b, os.ModePerm)
	return err
}

// validateToken is a helper function that calls the builtin cipher.validate(). The
// optional second argument (true) tells the function to generate an error state for
// the various ways the token was considered invalid.
func validateToken(t string) bool {
	v, err := functions.CallBuiltin(&symbols.SymbolTable{}, "cipher.Validate", t, true)
	if err != nil {
		ui.Debug(ui.ServerLogger, "Token validation error: "+err.Error())
	}
	return v.(bool)
}

// tokenUser is a helper function that calls the builtin cipher.token() and returns
// the user field
func tokenUser(t string) string {
	v, _ := functions.CallBuiltin(&symbols.SymbolTable{}, "cipher.Validate", t)
	if util.GetBool(v) {
		t, _ := functions.CallBuiltin(&symbols.SymbolTable{}, "cipher.Token", t)
		if m, ok := t.(map[string]interface{}); ok {
			if n, ok := m["name"]; ok {
				return util.GetString(n)
			}
		}
	}
	return ""
}
