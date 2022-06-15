package auth

import (
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/functions"
	"github.com/tucats/ego/symbols"
)

type UserIOService interface {
	ReadUser(name string) (defs.User, *errors.EgoError)
	WriteUser(user defs.User) *errors.EgoError
	DeleteUser(name string) *errors.EgoError
	ListUsers() map[string]defs.User
	Flush() *errors.EgoError
}

var AuthService UserIOService

var userDatabaseFile = ""

// loadUserDatabase uses command line options to locate and load the authorized users
// database, or initialize it to a helpful default.
func LoadUserDatabase(c *cli.Context) *errors.EgoError {
	defaultUser := "admin"
	defaultPassword := "password"

	if up := settings.Get(defs.DefaultCredentialSetting); up != "" {
		if pos := strings.Index(up, ":"); pos >= 0 {
			defaultUser = up[:pos]
			defaultPassword = strings.TrimSpace(up[pos+1:])
		} else {
			defaultUser = up
			defaultPassword = ""
		}
	}

	// Is there a user database to load?
	userDatabaseFile, _ = c.String("users")
	if userDatabaseFile == "" {
		userDatabaseFile = settings.Get(defs.LogonUserdataSetting)
	}

	if userDatabaseFile == "" {
		userDatabaseFile = defs.DefaultUserdataFileName
	}

	var err *errors.EgoError

	if !ui.LoggerIsActive(ui.AuthLogger) {
		ui.Debug(ui.ServerLogger, "Initializing credentials and authorizations")
	} else {
		ui.Debug(ui.AuthLogger, "Initializing credentials and authorizations using %s", userDatabaseFile)
	}

	AuthService, err = defineCredentialService(userDatabaseFile, defaultUser, defaultPassword)

	// If there is a --superuser specified on the command line, or in the persistent profile data,
	// mark that user as having ROOT privileges
	su, ok := c.String("superuser")
	if !ok {
		su = settings.Get(defs.LogonSuperuserSetting)
	}

	if su != "" {
		err = setPermission(su, "root", true)
		if err != nil {
			err = setPermission(su, "logon", true)
		}
	}

	return err
}

func defineCredentialService(path, user, password string) (UserIOService, *errors.EgoError) {
	var err *errors.EgoError

	path = strings.TrimSuffix(strings.TrimPrefix(path, "\""), "\"")

	if isDatabaseURL(path) {
		AuthService, err = NewDatabaseService(path, user, password)
	} else {
		AuthService, err = NewFileService(path, user, password)
	}

	return AuthService, err
}

// setPermission sets a given permission string to true for a given user. Returns an error
// if the username does not exist.
func setPermission(user, privilege string, enabled bool) *errors.EgoError {
	var err *errors.EgoError

	privname := strings.ToLower(privilege)

	if u, err := AuthService.ReadUser(user); errors.Nil(err) {
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

		err = AuthService.WriteUser(u)
		if !errors.Nil(err) {
			return err
		}

		err = AuthService.Flush()
		if !errors.Nil(err) {
			return err
		}

		ui.Debug(ui.AuthLogger, "Setting %s privilege for user \"%s\" to %v", privname, user, enabled)
	} else {
		return errors.New(errors.ErrNoSuchUser).Context(user)
	}

	return err
}

// GetPermission returns a boolean indicating if the given username and privilege are valid and
// set. If the username or privilege does not exist, then the reply is always false.
func GetPermission(user, privilege string) bool {
	privname := strings.ToLower(privilege)

	if u, ok := AuthService.ReadUser(user); errors.Nil(ok) {
		pn := findPermission(u, privname)

		return (pn >= 0)
	}

	ui.Debug(ui.AuthLogger, "User %s does not have %s privilege", user, privilege)

	return false
}

// findPermission searches the permission strings associated with the given user,
// and returns the position in the permissions array where the matching name is
// found. It returns -1 if there is no such permission.
func findPermission(u defs.User, perm string) int {
	for i, p := range u.Permissions {
		if p == perm {
			return i
		}
	}

	return -1
}

// ValidatePassword checks a username and password against the database and
// returns true if the user exists and the password is valid.
func ValidatePassword(user, pass string) bool {
	ok := false

	if u, userExists := AuthService.ReadUser(user); errors.Nil(userExists) {
		realPass := u.Password
		// If the password in the database is quoted, do a local hash
		if strings.HasPrefix(realPass, "{") && strings.HasSuffix(realPass, "}") {
			realPass = HashString(realPass[1 : len(realPass)-1])
		}

		hashPass := HashString(pass)
		ok = realPass == hashPass

		if findPermission(u, "logon") < 0 {
			ok = false
		}
	}

	return ok
}

// HashString converts a given string to it's hash. This is used to manage
// passwords as opaque objects.
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

// Authenticated implements the Authenticated(user,pass) function. This accepts a username
// and password string, and determines if they are authenticated using the
// users database.
func Authenticated(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	var user, pass string

	// If there are no arguments, then we look for the _user and _password
	// variables and use those. Otherwise, fetch them as the two parameters.
	if len(args) == 0 {
		if ux, ok := s.Get("_user"); ok {
			user = datatypes.GetString(ux)
		}

		if px, ok := s.Get("_password"); ok {
			pass = datatypes.GetString(px)
		}
	} else {
		if len(args) != 2 {
			return false, errors.New(errors.ErrArgumentCount)
		}

		user = datatypes.GetString(args[0])
		pass = datatypes.GetString(args[1])
	}

	// If the user exists and the password matches then valid.
	return ValidatePassword(user, pass), nil
}

// Permission implements the Permission(user,priv) function. It returns
// a boolean value indicating if the given username has the given permission.
func Permission(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	var user, priv string

	if len(args) != 2 {
		return false, errors.New(errors.ErrArgumentCount)
	}

	user = datatypes.GetString(args[0])
	priv = strings.ToUpper(datatypes.GetString(args[1]))

	// If the user exists and the privilege exists, return it's status
	return GetPermission(user, priv), nil
}

// SetUser implements the SetUser() function. For the super user, this function
// can be used to update user data in the persistent use database for the Ego
// web server.
func SetUser(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	var err *errors.EgoError

	// Before we do anything else, are we running this call as a superuser?
	superUser := false

	if s, ok := s.Get("_superuser"); ok {
		superUser = datatypes.GetBool(s)
	}

	if !superUser {
		return nil, errors.New(errors.ErrNoPrivilegeForOperation)
	}

	// There must be one parameter, which is a struct containing
	// the user data
	if len(args) != 1 {
		return nil, errors.New(errors.ErrArgumentCount)
	}

	if u, ok := args[0].(*datatypes.EgoMap); ok {
		name := ""
		if n, ok, _ := u.Get("name"); ok {
			name = strings.ToLower(datatypes.GetString(n))
		}

		r, ok := AuthService.ReadUser(name)
		if !errors.Nil(ok) {
			r = defs.User{
				Name:        name,
				ID:          uuid.New(),
				Permissions: []string{},
			}
		}

		if n, ok, _ := u.Get("password"); ok {
			r.Password = HashString(datatypes.GetString(n))
		}

		if n, ok, _ := u.Get("permissions"); ok {
			if m, ok := n.([]interface{}); ok {
				if len(m) > 0 {
					r.Permissions = []string{}

					for _, p := range m {
						permissionName := datatypes.GetString(p)
						if permissionName != "." {
							r.Permissions = append(r.Permissions, permissionName)
						}
					}
				}
			}
		}

		err = AuthService.WriteUser(r)
		if err == nil {
			err = AuthService.Flush()
		}
	}

	return true, err
}

// DeleteUser implements the DeleteUser() function. For a privileged user,
// this will delete a record from the persistent user database. Returns true
// if the name was deleted, else false if it was not a valid username.
func DeleteUser(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	// Before we do anything else, are we running this call as a superuser?
	superUser := false

	if s, ok := s.Get("_superuser"); ok {
		superUser = datatypes.GetBool(s)
	}

	if !superUser {
		return nil, errors.New(errors.ErrNoPrivilegeForOperation)
	}

	// There must be one parameter, which is the username
	if len(args) != 1 {
		return nil, errors.New(errors.ErrArgumentCount)
	}

	name := strings.ToLower(datatypes.GetString(args[0]))

	if _, ok := AuthService.ReadUser(name); errors.Nil(ok) {
		err := AuthService.DeleteUser(name)
		if !errors.Nil(err) {
			return false, err
		}

		return true, AuthService.Flush()
	}

	return false, nil
}

// GetUser implements the GetUser() function. This returns a struct defining the
// persisted information about an existing user in the user database.
func GetUser(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	// There must be one parameter, which is a username
	if len(args) != 1 {
		return nil, errors.New(errors.ErrArgumentCount)
	}

	r := datatypes.NewMap(datatypes.StringType, datatypes.InterfaceType)
	name := strings.ToLower(datatypes.GetString(args[0]))

	t, ok := AuthService.ReadUser(name)
	if !errors.Nil(ok) {
		return r, nil
	}

	permArray := datatypes.NewArray(datatypes.StringType, len(t.Permissions))
	for i, perm := range t.Permissions {
		permArray.SetAlways(i, perm)
	}

	_, _ = r.Set("name", name)
	_, _ = r.Set("permissions", permArray)
	_, _ = r.Set("superuser", GetPermission(name, "root"))

	return r, nil
}

// validateToken is a helper function that calls the builtin cipher.validate(). The
// optional second argument (true) tells the function to generate an error state for
// the various ways the token was considered invalid.
func ValidateToken(t string) bool {
	v, err := functions.CallBuiltin(&symbols.SymbolTable{}, "cipher.Validate", t, true)
	if !errors.Nil(err) {
		ui.Debug(ui.AuthLogger, "Token validation error: "+err.Error())
	}

	return v.(bool)
}

// TokenUser is a helper function that calls the builtin cipher.token() and returns
// the user field.
func TokenUser(t string) string {
	v, _ := functions.CallBuiltin(&symbols.SymbolTable{}, "cipher.Validate", t)
	if datatypes.GetBool(v) {
		t, _ := functions.CallBuiltin(&symbols.SymbolTable{}, "cipher.Token", t)
		if m, ok := t.(*datatypes.EgoStruct); ok {
			if n, ok := m.Get("name"); ok {
				return datatypes.GetString(n)
			}
		}
	}

	return ""
}

// Utility function to determine if a given path is a database URL or
// not.
func isDatabaseURL(path string) bool {
	path = strings.ToLower(path)
	drivers := []string{"postgres://", "sqlite3://"}

	for _, driver := range drivers {
		if strings.HasPrefix(path, driver) {
			return true
		}
	}

	return false
}
