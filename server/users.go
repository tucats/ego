package server

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/persistence"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/functions"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

var userDatabase map[string]defs.User

var userDatabaseFile = ""

// loadUserDatabase uses command line options to locate and load the authorized users
// database, or initialize it to a helpful default.
func LoadUserDatabase(c *cli.Context) *errors.EgoError {
	defaultUser := "admin"
	defaultPassword := "password"

	if up := persistence.Get(defs.DefaultCredentialSetting); up != "" {
		if pos := strings.Index(up, ":"); pos >= 0 {
			defaultUser = up[:pos]
			defaultPassword = strings.TrimSpace(up[pos+1:])
		} else {
			defaultUser = up
			defaultPassword = ""
		}
	}

	// Is there a user database to load?
	userDatabaseFile, _ = c.GetString("users")
	if userDatabaseFile == "" {
		userDatabaseFile = persistence.Get(defs.LogonUserdataSetting)
	}

	if userDatabaseFile == "" {
		userDatabaseFile = defs.DefaultUserdataFileName
	}

	if userDatabaseFile != "" {
		b, err := ioutil.ReadFile(userDatabaseFile)
		if errors.Nil(err) {
			if key := persistence.Get(defs.LogonUserdataKeySetting); key != "" {
				r, err := util.Decrypt(string(b), key)
				if !errors.Nil(err) {
					return err
				}

				b = []byte(r)
			}

			if errors.Nil(err) {
				err = json.Unmarshal(b, &userDatabase)
			}

			if !errors.Nil(err) {
				return errors.New(err)
			}

			ui.Debug(ui.ServerLogger, "Using stored credentials with %d items", len(userDatabase))
		}
	}

	if userDatabase == nil {
		userDatabase = map[string]defs.User{
			defaultUser: {
				ID:          uuid.New(),
				Name:        defaultUser,
				Password:    HashString(defaultPassword),
				Permissions: []string{"root"},
			},
		}

		ui.Debug(ui.ServerLogger, "Using default credentials %s:%s", defaultUser, defaultPassword)
	}

	// If there is a --superuser specified on the command line, or in the persistent profile data,
	// mark that user as having ROOT privileges
	var err *errors.EgoError

	su, ok := c.GetString("superuser")
	if !ok {
		su = persistence.Get(defs.LogonSuperuserSetting)
	}

	if su != "" {
		err = setPermission(su, "root", true)
	}

	return err
}

// setPermission sets a given permission string to true for a given user. Returns an error
// if the username does not exist.
func setPermission(user, privilege string, enabled bool) *errors.EgoError {
	var err *errors.EgoError

	privname := strings.ToLower(privilege)

	if u, ok := userDatabase[user]; ok {
		if u.Permissions == nil {
			u.Permissions = []string{"logon"}
		}

		pn := -1

		for i, p := range u.Permissions {
			if p == privname {
				pn = i
			}
		}

		if enabled {
			if pn == -1 {
				u.Permissions = append(u.Permissions, privname)
			}
		} else {
			if pn >= 0 {
				u.Permissions = append(u.Permissions[:pn], u.Permissions[pn+1:]...)
			}
		}

		userDatabase[user] = u

		ui.Debug(ui.ServerLogger, "Setting %s privilege for user \"%s\" to %v", privname, user, enabled)
	} else {
		err = errors.New(errors.NoSuchUserError).Context(user)
	}

	return err
}

// getPermission returns a boolean indicating if the given username and privilege are valid and
// set. If the username or privilege does not exist, then the reply is always false.
func getPermission(user, privilege string) bool {
	privname := strings.ToLower(privilege)

	if u, ok := userDatabase[user]; ok {
		pn := findPermission(u, privname)
		v := (pn >= 0)

		ui.Debug(ui.ServerLogger, "Check %s permission for user \"%s\" (%v)", privilege, user, v)

		return v
	}

	ui.Debug(ui.ServerLogger, "Check %s permission for user \"%s\" (false)", privilege, user)

	return false
}

func findPermission(u defs.User, perm string) int {
	for i, p := range u.Permissions {
		if p == perm {
			return i
		}
	}

	return -1
}

// validatePassword checks a username and password against the databse and
// returns true if the user exists and the password is valid.
func validatePassword(user, pass string) bool {
	ok := false

	if u, userExists := userDatabase[user]; userExists {
		realPass := u.Password
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
// passwords.
func HashString(s string) string {
	var r strings.Builder

	h := sha256.New()
	_, _ = h.Write([]byte(s))

	v := h.Sum(nil)
	for _, b := range v {
		r.WriteString(fmt.Sprintf("%02x", b))
	}

	return r.String()
}

// Authenticated implmeents the Authenticated(user,pass) function. This accepts a username
// and password string, and determines if they are authenticated using the
// users database.
func Authenticated(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
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
			return false, errors.New(errors.ArgumentCountError)
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
func Permission(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	var user, priv string

	if len(args) != 2 {
		return false, errors.New(errors.ArgumentCountError)
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

// Implements the SetUser() function.
func SetUser(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	var err *errors.EgoError

	// Before we do anything else, are we running this call as a superuser?
	superUser := false

	if s, ok := s.Get("_superuser"); ok {
		superUser = util.GetBool(s)
	}

	if !superUser {
		return nil, errors.New(errors.NoPrivilegeForOperationError)
	}

	// There must be one parameter, which is a struct containing
	// the user data
	if len(args) != 1 {
		return nil, errors.New(errors.ArgumentCountError)
	}

	if u, ok := args[0].(map[string]interface{}); ok {
		name := ""
		if n, ok := u["name"]; ok {
			name = strings.ToLower(util.GetString(n))
		}

		r, ok := userDatabase[name]
		if !ok {
			r = defs.User{
				Name:        name,
				ID:          uuid.New(),
				Permissions: []string{},
			}
		}

		if n, ok := u["password"]; ok {
			r.Password = HashString(util.GetString(n))
		}

		if n, ok := u["permissions"]; ok {
			if m, ok := n.([]interface{}); ok {
				if len(m) > 0 {
					r.Permissions = []string{}

					for _, p := range m {
						pname := util.GetString(p)
						if pname != "." {
							r.Permissions = append(r.Permissions, pname)
						}
					}
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
func DeleteUser(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	// Before we do anything else, are we running this call as a superuser?
	superUser := false

	if s, ok := s.Get("_superuser"); ok {
		superUser = util.GetBool(s)
	}

	if !superUser {
		return nil, errors.New(errors.NoPrivilegeForOperationError)
	}

	// There must be one parameter, which is the username
	if len(args) != 1 {
		return nil, errors.New(errors.ArgumentCountError)
	}

	name := strings.ToLower(util.GetString(args[0]))

	if _, ok := userDatabase[name]; ok {
		delete(userDatabase, name)

		return true, updateUserDatabase()
	}

	return false, nil
}

// Implements the GetUser() function.
func GetUser(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	// There must be one parameter, which is a username
	if len(args) != 1 {
		return nil, errors.New(errors.ArgumentCountError)
	}

	r := map[string]interface{}{}
	name := strings.ToLower(util.GetString(args[0]))

	t, ok := userDatabase[name]
	if !ok {
		return r, nil
	}

	r["name"] = name
	r["permissions"] = t.Permissions
	r["superuser"] = getPermission(name, "root")

	return r, nil
}

// updateUserDatabase re-writes the user database file with updated values.
func updateUserDatabase() *errors.EgoError {
	// Convert the database to a json string
	b, err := json.MarshalIndent(userDatabase, "", "   ")
	if !errors.Nil(err) {
		return errors.New(err)
	}

	if key := persistence.Get(defs.LogonUserdataKeySetting); key != "" {
		r, err := util.Encrypt(string(b), key)
		if !errors.Nil(err) {
			return err
		}

		b = []byte(r)
	}

	// Write to the database file.
	err = ioutil.WriteFile(userDatabaseFile, b, 0600)

	return errors.New(err)
}

// validateToken is a helper function that calls the builtin cipher.validate(). The
// optional second argument (true) tells the function to generate an error state for
// the various ways the token was considered invalid.
func validateToken(t string) bool {
	v, err := functions.CallBuiltin(&symbols.SymbolTable{}, "cipher.Validate", t, true)
	if !errors.Nil(err) {
		ui.Debug(ui.ServerLogger, "Token validation error: "+err.Error())
	}

	return v.(bool)
}

// tokenUser is a helper function that calls the builtin cipher.token() and returns
// the user field.
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
