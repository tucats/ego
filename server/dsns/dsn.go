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

// The DSN service interface. This is the interface that must be
// implemented by any DSN service provider. This is used to abstract
// the actual storage mechanism for the DSN data (file-based versus
// database-based, for example).
type dsnService interface {
	AuthDSN(user, dsn string, action DSNAction) bool
	ReadDSN(user, name string, doNotLog bool) (defs.DSN, error)
	WriteDSN(user string, dsname defs.DSN) error
	DeleteDSN(user, name string) error
	ListDSNS(user string) (map[string]defs.DSN, error)
	GrantDSN(user, name string, action DSNAction, grant bool) error
	Permissions(user, name string) (map[string]DSNAction, error)
	Flush() error
}

// DSNAuthorization is a structure that describes the authorization
// for a given user to access a given DSN. The user is the name of the
// user, the DSN is the name of the DSN, and the action is the action
// bit-mask that describes the access rights for the user to the DSN.
type DSNAuthorization struct {
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

// Initialize uses command line options to locate and load the authorized users
// database, or initialize it to a helpful default.
func Initialize(c *cli.Context) error {
	var err error

	// Is there a user database to load? We use the same database that the user
	// authorization data is stored in. If it was not specified, use the default from
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

	if !ui.IsActive(ui.AuthLogger) {
		ui.Log(ui.ServerLogger, "auth.dsn.init", nil)
	} else {
		ui.Log(ui.AuthLogger, "auth.dsn.init", nil)
	}

	DSNService, err = defineDSNService(userDatabaseFile)

	return err
}

// defineDSNService creates a new DSN service provider based on the
// given path. If the path is a database URL, a database service is
// created. Otherwise, a file service is created. If the path is
// "memory" then an in-memory service is created.
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

// NewDSN creates a new DSN object with the given parameters. The name
// is the name of the DSN, and the provider is the database provider
// (e.g. "postgres" or "sqlite3"). The database is the name of the
// database to connect to, and the user and password are the credentials
// to use to connect to the database. The host and port are the network
// address of the database server, and the native and secured flags
// indicate if the database is a native database (e.g. not a cloud
// service) and if the connection should be secured.
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
		password, _ = encrypt(password)
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

// Connection returns a connection string for the given DSN. This is used
// to connect to the database.
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
				pw, err = decrypt(d.Password)
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
