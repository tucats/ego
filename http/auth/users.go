// Package auth handles authentication for an Ego server. It includes
// the service providers for database and filesystem authentication
// storage modes.
package auth

import (
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/builtins"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/runtime"
	"github.com/tucats/ego/symbols"
)

type userIOService interface {
	ReadUser(name string, doNotLog bool) (defs.User, error)
	WriteUser(user defs.User) error
	DeleteUser(name string) error
	ListUsers() map[string]defs.User
	Flush() error
}

// AuthService stores the specific instance of a service provider for
// authentication services (there are builtin providers for JSON based
// file service and a database serivce that can connect to Postgres or
// SQLite3).
var AuthService userIOService

var (
	userDatabaseFile = ""
	agingMutex       sync.Mutex
	aging            map[string]time.Time
)

// loadUserDatabase uses command line options to locate and load the authorized users
// database, or initialize it to a helpful default.
func LoadUserDatabase(c *cli.Context) error {
	defaultUser := "admin"
	defaultPassword := "password"
	aging = map[string]time.Time{}

	credential := ""

	if creds, _ := c.String("default-credential"); creds != "" {
		credential = creds
	} else if creds := settings.Get(defs.DefaultCredentialSetting); creds != "" {
		credential = creds
	}

	if credential != "" {
		if pos := strings.Index(credential, ":"); pos >= 0 {
			defaultUser = credential[:pos]
			defaultPassword = strings.TrimSpace(credential[pos+1:])
		} else {
			defaultUser = credential
			defaultPassword = ""
		}

		settings.SetDefault(defs.LogonSuperuserSetting, defaultUser)
	}

	// Is there a user database to load? If it was not specified, use the default from
	// the configuration, and if that's empty then use the default SQLITE3 database.
	// The use of "found" here allows the user to specify no database by specifying
	// an empty string, or using the value "memory" to mean in-memory database only
	userDatabaseFile, found := c.String("users")
	if !found {
		userDatabaseFile = settings.Get(defs.LogonUserdataSetting)

		if userDatabaseFile == "" {
			userDatabaseFile = defs.DefaultUserdataFileName
		}
	}

	var err error

	if !ui.IsActive(ui.AuthLogger) {
		ui.Log(ui.ServerLogger, "Initializing credentials and authorizations")
	} else {
		dbName := userDatabaseFile
		if dbName == "" {
			dbName = "in-memory database"
			// Since we're doing in-memory, launch the aging mechanism that
			// deletes credentials extracted from expired tokens.
			go ageCredentials()

		}

		ui.Log(ui.AuthLogger, "Initializing credentials and authorizations using %s", dbName)
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

func defineCredentialService(path, user, password string) (userIOService, error) {
	var err error

	path = strings.TrimSuffix(strings.TrimPrefix(path, "\""), "\"")

	if isDatabaseURL(path) {
		AuthService, err = NewDatabaseService(path, user, password)
	} else {
		AuthService, err = NewFileService(path, user, password)
	}

	return AuthService, err
}

// Authenticated implements the Authenticated(user,pass) function. This accepts a username
// and password string, and determines if they are authenticated using the
// users database.
func Authenticated(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	var user, pass string

	// If there are no arguments, then we look for the _user and _password
	// variables and use those. Otherwise, fetch them as the two parameters.
	if args.Len() == 0 {
		if ux, ok := s.Get("_user"); ok {
			user = data.String(ux)
		}

		if px, ok := s.Get("_password"); ok {
			pass = data.String(px)
		}
	} else {
		if args.Len() != 2 {
			return false, errors.ErrArgumentCount
		}

		user = data.String(args.Get(0))
		pass = data.String(args.Get(1))
	}

	// If the user exists and the password matches then valid.
	return ValidatePassword(user, pass), nil
}

// Permission implements the Permission(user,priv) function. It returns
// a boolean value indicating if the given username has the given permission.
func Permission(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	var user, priv string

	if args.Len() != 2 {
		return false, errors.ErrArgumentCount
	}

	user = data.String(args.Get(0))
	priv = strings.ToUpper(data.String(args.Get(1)))

	// If the user exists and the privilege exists, return it's status
	return GetPermission(user, priv), nil
}

// SetUser implements the SetUser() function. For the super user, this function
// can be used to update user data in the persistent use database for the Ego
// web server.
func SetUser(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	var err error

	// Before we do anything else, are we running this call as a superuser?
	superUser := false

	if s, ok := s.Get("_superuser"); ok {
		superUser = data.Bool(s)
	}

	if !superUser {
		return nil, errors.ErrNoPrivilegeForOperation
	}

	// There must be one parameter, which is a struct containing
	// the user data
	if args.Len() != 1 {
		return nil, errors.ErrArgumentCount
	}

	if u, ok := args.Get(0).(*data.Map); ok {
		name := ""
		if n, ok, _ := u.Get("name"); ok {
			name = strings.ToLower(data.String(n))
		}

		r, ok := AuthService.ReadUser(name, false)
		if ok != nil {
			r = defs.User{
				Name:        name,
				ID:          uuid.New(),
				Permissions: []string{},
			}
		}

		if n, ok, _ := u.Get("password"); ok {
			r.Password = HashString(data.String(n))
		}

		if n, ok, _ := u.Get("permissions"); ok {
			if m, ok := n.([]interface{}); ok {
				if len(m) > 0 {
					r.Permissions = []string{}

					for _, p := range m {
						permissionName := data.String(p)
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
func DeleteUser(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	// Before we do anything else, are we running this call as a superuser?
	superUser := false

	if s, ok := s.Get("_superuser"); ok {
		superUser = data.Bool(s)
	}

	if !superUser {
		return nil, errors.ErrNoPrivilegeForOperation
	}

	// There must be one parameter, which is the username
	if args.Len() != 1 {
		return nil, errors.ErrArgumentCount
	}

	name := strings.ToLower(data.String(args.Get(0)))

	if _, ok := AuthService.ReadUser(name, false); ok == nil {
		err := AuthService.DeleteUser(name)
		if err != nil {
			return false, err
		}

		return true, AuthService.Flush()
	}

	return false, nil
}

// GetUser implements the GetUser() function. This returns a struct defining the
// persisted information about an existing user in the user database.
func GetUser(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	// There must be one parameter, which is a username
	if args.Len() != 1 {
		return nil, errors.ErrArgumentCount
	}

	r := data.NewMap(data.StringType, data.InterfaceType)
	name := strings.ToLower(data.String(args.Get(0)))

	t, ok := AuthService.ReadUser(name, false)
	if ok != nil {
		return r, nil
	}

	permArray := data.NewArray(data.StringType, len(t.Permissions))
	for i, perm := range t.Permissions {
		permArray.SetAlways(i, perm)
	}

	_, _ = r.Set("name", name)
	_, _ = r.Set("permissions", permArray)
	_, _ = r.Set("superuser", GetPermission(name, "root"))

	return r, nil
}

// TokenUser is a helper function that calls the builtin cipher.token() and returns
// the user field.
func TokenUser(t string) string {
	// Are we an authority? If not, let's see who is.
	authServer := settings.Get(defs.ServerAuthoritySetting)
	if authServer != "" {
		u, err := remoteUser(authServer, t)
		if err != nil {
			return ""
		}

		return u.Name
	}

	s := symbols.NewSymbolTable("get user")
	runtime.AddPackages(s)

	v, e := builtins.CallBuiltin(s, "cipher.Validate", t)
	if e != nil {
		ui.Log(ui.AuthLogger, "Failed to validate token: %v", e)

		return ""
	}

	if data.Bool(v) {
		t, e := builtins.CallBuiltin(s, "cipher.Extract", t)
		if e != nil {
			ui.Log(ui.AuthLogger, "Failed to decode token: %v", e)

			return ""
		}

		if m, ok := t.(*data.Struct); ok {
			if n, ok := m.Get("Name"); ok {
				return data.String(n)
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
