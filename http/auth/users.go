// Package auth handles authentication for an Ego server. It includes
// the service providers for database and filesystem authentication
// storage modes.
package auth

import (
	"crypto/sha256"
	"net/http"
	"strconv"
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
	"github.com/tucats/ego/runtime/rest"
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

var userDatabaseFile = ""

var agingMutex sync.Mutex

var aging map[string]time.Time

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

// setPermission sets a given permission string to true for a given user. Returns an error
// if the username does not exist.
func setPermission(user, privilege string, enabled bool) error {
	var err error

	privname := strings.ToLower(privilege)

	if u, err := AuthService.ReadUser(user, false); err == nil {
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
		if err != nil {
			return err
		}

		err = AuthService.Flush()
		if err != nil {
			return err
		}

		ui.Log(ui.AuthLogger, "Setting %s privilege for user \"%s\" to %v", privname, user, enabled)
	} else {
		return errors.ErrNoSuchUser.Context(user)
	}

	return err
}

// GetPermission returns a boolean indicating if the given username and privilege are valid and
// set. If the username or privilege does not exist, then the reply is always false.
func GetPermission(user, privilege string) bool {
	privname := strings.ToLower(privilege)

	if u, err := AuthService.ReadUser(user, false); err == nil {
		pn := findPermission(u, privname)

		return (pn >= 0)
	}

	ui.Log(ui.AuthLogger, "User %s does not have %s privilege", user, privilege)

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

// Go routine that runs periodically to see if credentials should be
// aged out of the user store. Runs every 180 seconds.
func ageCredentials() {
	for {
		time.Sleep(180 * time.Second)
		agingMutex.Lock()

		list := []string{}

		for user, expires := range aging {
			if time.Since(expires) > 0 {
				list = append(list, user)
			}
		}

		for _, user := range list {
			delete(aging, user)
			AuthService.DeleteUser(user)
		}

		agingMutex.Unlock()

	}

}

// ValidatePassword checks a username and password against the database and
// returns true if the user exists and the password is valid.
func ValidatePassword(user, pass string) bool {
	ok := false

	if u, userExists := AuthService.ReadUser(user, false); userExists == nil {
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
		// Format the byte. It must be two digits long, so if it was a
		// value less than 0x10, add a leading zero.
		byteString := strconv.FormatInt(int64(b), 16)
		if len(byteString) < 2 {
			byteString = "0" + byteString
		}

		r.WriteString(byteString)
	}

	return r.String()
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

// validateToken is a helper function that calls the builtin cipher.validate(). The
// optional second argument (true) tells the function to generate an error state for
// the various ways the token was considered invalid.
func ValidateToken(t string) bool {
	// Are we an authority? If not, let's see who is.
	authServer := settings.Get(defs.ServerAuthoritySetting)
	if authServer != "" {
		url := authServer + "/services/admin/authenticate/"
		resp := data.NewStruct(data.StructType)
		err := rest.Exchange(url, http.MethodGet, t, &resp, "authenticate")
		if err != nil {
			return false
		}

		// If we didn't get a 401 error on the above call, the token is valid.
		// Since we aren't an auth service ourselves, let's copy this info to
		// our local auth store

		u := defs.User{}
		if name, ok := resp.Get("Name"); ok {
			u.Name = data.String(name)
			u.ID = uuid.Nil
		}

		if v, ok := resp.Get("Permissions"); ok {
			if perms, ok := v.(*data.Array); ok {
				u.Permissions = []string{}
				for i := 0; i < perms.Len(); i++ {
					v, _ := perms.Get(i)
					u.Permissions = append(u.Permissions, data.String(v))
				}
			}
		}

		err = AuthService.WriteUser(u)

		// Because this is data that came from a token, let's launch a small thread
		// whose job is to expire the local (ephemeral) user data.
		if v, ok := resp.Get("Expires"); ok {
			expirationString := data.String(v)
			// @tomcole this should be revised to use an standardized date format string
			format := "2006-01-02 15:04:05.999999999 -0700 MST"
			if expires, err := time.Parse(format, expirationString); err == nil {
				agingMutex.Lock()
				aging[u.Name] = expires
				agingMutex.Unlock()
			}
		}
		return true
	}

	// We must be the authority, so use our local authentication service.
	s := symbols.NewSymbolTable("validate")
	runtime.AddPackages(s)

	v, err := builtins.CallBuiltin(s, "cipher.Validate", t, true)
	if err != nil {
		ui.Log(ui.AuthLogger, "Failed to validate token: %v", err)

		return false
	}

	if v == nil {
		return false
	}

	return v.(bool)
}

// TokenUser is a helper function that calls the builtin cipher.token() and returns
// the user field.
func TokenUser(t string) string {
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
