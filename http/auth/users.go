// Package auth handles authentication for an Ego server. It includes
// the service providers for database and filesystem authentication
// storage modes.
package auth

import (
	"strings"
	"sync"
	"time"

	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
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
		displayName := userDatabaseFile
		if displayName == "" {
			displayName = "in-memory database"
			// Since we're doing in-memory, launch the aging mechanism that
			// deletes cached credentials extracted from tokens when the
			// token expiration arrives.
			go ageCredentials()
		}

		ui.Log(ui.AuthLogger, "Initializing credentials and authorizations using %s", displayName)
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
