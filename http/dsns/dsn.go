package dsns

import (
	"crypto/sha256"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/util"
)

// An action code. These can be ANDed together to describe a request
// for multiple actions being authorized, so the auth table will have
// the sum of all authorized actions. These must be bit-unique values.
type DSNAction int

const (
	// No action authorized.
	DSNNoAccess DSNAction = 0

	// Read a DSN.
	DSNReadAction DSNAction = 1

	// Write or update a DSN.
	DSNWriteAction DSNAction = 2

	// Create or delete a DSN.
	DSNAdminAction DSNAction = 8
)

type dsnService interface {
	AuthDSN(user, dsn string, action DSNAction) bool
	ReadDSN(user, name string, doNotLog bool) (defs.DSN, error)
	WriteDSN(user string, dsname defs.DSN) error
	DeleteDSN(user, name string) error
	ListDSNS(user string) map[string]defs.DSN
	GrantDSN(user, name string, action DSNAction, grant bool) error
	Flush() error
}

type dsnAuthorization struct {
	User   string
	DSN    string
	Action DSNAction
}

// DSNService stores the specific instance of a service provider for
// authentication services (there are builtin providers for JSON based
// file service and a database serivce that can connect to Postgres or
// SQLite3).
var DSNService dsnService

var (
	dsnDatabaseFile = ""
)

// loadUserDatabase uses command line options to locate and load the authorized users
// database, or initialize it to a helpful default.
func Initialize(c *cli.Context) error {
	// Is there a user database to load? We use the same database that the users
	// data was stored in. If it was not specified, use the default from
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
		ui.Log(ui.ServerLogger, "Initializing data source names")
	} else {
		displayName := userDatabaseFile
		if displayName == "" {
			displayName = "in-memory database"
		}

		ui.Log(ui.AuthLogger, "Initializing data source names using %s", displayName)
	}

	DSNService, err = defineDSNService(userDatabaseFile)

	return err
}

func defineDSNService(path string) (dsnService, error) {
	var err error

	path = strings.TrimSuffix(strings.TrimPrefix(path, "\""), "\"")

	if isDatabaseURL(path) {
		DSNService, err = NewDatabaseService(path)
	} else {
		if path != "memory" {
			dir := filepath.Dir(path)
			base := filepath.Base(path)
			ext := filepath.Ext(path)

			path = filepath.Join(dir, base+"_dsns", ext)
		}

		DSNService, err = NewFileService(path)
	}

	return DSNService, err
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

func NewDSN(name, provider, database, user, password string, host string, port int, native, secured bool) *defs.DSN {
	if database == "" {
		database = name
	} else if name == "" {
		name = database
	}

	if port < 32 {
		port = 5432
	}

	if host == "" {
		host = "localhost"
	}

	if password != "" {
		password, _ = util.Encrypt(password, settings.Get(defs.ServerTokenKeySetting))
	}

	if provider == "" {
		provider = "sqlite3"
	}

	return &defs.DSN{
		Name:     name,
		ID:       uuid.NewString(),
		Provider: provider,
		Native:   native,
		Username: user,
		Password: password,
		Database: database,
		Host:     host,
		Port:     port,
		Secured:  secured,
	}
}

func Connection(d *defs.DSN) (string, error) {
	var (
		err error
		pw  string
	)

	isSQLLite := strings.EqualFold(d.Provider, "sqlite3")

	result := strings.Builder{}

	result.WriteString(d.Provider)
	result.WriteString("://")

	if !isSQLLite {
		if d.Username != "" {
			result.WriteString(d.Username)

			if d.Password != "" {
				pw, err = util.Decrypt(d.Password, settings.Get(defs.ServerTokenKeySetting))
				if err != nil {
					return "", err
				}

				result.WriteString(":")
				result.WriteString(pw)
			}

			result.WriteString("@")
		}

		if d.Host == "" {
			result.WriteString("localhost")
		} else {
			result.WriteString(d.Host)
		}

		result.WriteString(":")

		if d.Port > 0 {
			result.WriteString(strconv.Itoa(d.Port))
		} else {
			result.WriteString("5432")
		}

		result.WriteString("/")
	}

	result.WriteString(d.Database)

	if !isSQLLite {
		if !d.Secured {
			result.WriteString("?sslmode=disable")
		}
	}

	return result.String(), err
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
