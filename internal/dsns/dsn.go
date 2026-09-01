// The "dsns" package handles the abstraction of a data source name (dsn)
// which is a named object describing a connection to a data source (usually
// a database). These provide a way of allowing user code to identify a named
// entity, and subsequently determine if the item is available to the user,
// open the connection, etc.
//
// This lives at the top package level because it must be shared between the
// Ego runtime sql package and the REST server's understanding of dsns used
// in endpoints.
package dsns

import (
	"path/filepath"
	"strings"

	"github.com/tucats/ego/internal/cli/cli"
	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/defs"
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

// ActionString returns a human-readable string representation of the
// DSN action values such as DSNReadAction, DSNWriteAction, and
// DSNAdminAction.
func ActionString(action DSNAction) string {
	if action == DSNNoAccess {
		return "no access"
	}

	actions := []string{}

	if action&DSNReadAction != 0 {
		actions = append(actions, "read access")
	}

	if action&DSNWriteAction != 0 {
		actions = append(actions, "write access")
	}

	if action&DSNAdminAction != 0 {
		actions = append(actions, "admin access")
	}

	return strings.Join(actions, ", ")
}

// The DSN service interface. This is the interface that must be
// implemented by any DSN service provider. This is used to abstract
// the actual storage mechanism for the DSN data (file-based versus
// database-based, for example).
type dsnService interface { //nolint: interfacebloat
	AuthDSN(session int, user, dsn string, action DSNAction) bool
	ReadDSN(session int, user, name string, doNotLog bool) (defs.DSN, error)
	WriteDSN(session int, user string, dataSourceName defs.DSN) error
	DeleteDSN(session int, user, name string) error
	ListDSNS(session int, user string) (map[string]defs.DSN, error)
	GrantDSN(session int, user, name string, action DSNAction, grant bool) error
	Permissions(session int, user, name string) (map[string]DSNAction, error)
	RevokeAllDSN(session int, name string) error
	RevokeAllDSNForUser(session int, user string) error
	Flush() error

	// Close releases any resources (open database connections, file
	// handles) held by this service. It should be called once the service
	// is no longer needed -- for example, by a test that creates a fresh
	// service per test case. It is safe to call Close without first
	// calling Flush; implementations flush any pending data themselves
	// before releasing resources.
	Close() error
}

// DSNAuthorization is a structure that describes the authorization
// for a given user to access a given DSN. The user is the name of the
// user, the DSN is the name of the DSN, and the action is the action
// bit-mask that describes the access rights for the user to the DSN.
//
// ID must come first and be populated with a fresh UUID for every new
// row (see databaseService.GrantDSN). resources.ResHandle.Create falls
// back to the struct's first field as the table's primary key whenever
// SetPrimaryKey hasn't been called explicitly, and databaseService.
// NewDatabaseService doesn't call it for this table -- without a
// synthetic ID column, that fallback would have picked User (the
// previous first field), making it the SQL-backed dsns_auth table's
// sole primary key and silently limiting every user to a grant on at
// most one restricted DSN, table-wide, since a second GrantDSN for the
// same user (on any other DSN) would violate that column's implied
// uniqueness. The natural key is the (User, DSN) pair, not User alone.
type DSNAuthorization struct {
	ID     string
	User   string
	DSN    string
	Action DSNAction
}

// DSNService stores the specific instance of a service provider for
// authentication services (there are builtin providers for JSON based
// file service and a database service that can connect to Postgres or
// SQLite3).
var DSNService dsnService

// DSNDatabaseURL is the resolved database path or URL used to initialize
// the DSN service. It is set by Initialize() so child service processes
// can receive it via the request payload and replicate the same service.
var DSNDatabaseURL string

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
	if strings.HasPrefix(userDatabaseFile, "/sqlite:/") {
		userDatabaseFile = "sqlite://" + strings.TrimPrefix(userDatabaseFile, "/sqlite:/")
	}

	if !found {
		userDatabaseFile = settings.Get(defs.LogonUserdataSetting)
		if userDatabaseFile == "" {
			authPath := settings.Get(defs.EgoPathSetting)
			userDatabaseFile = defs.DefaultUserdataScheme + "://" + filepath.Join(authPath, defs.DefaultUserdataFileName)
		}
	}

	if !ui.IsActive(ui.AuthLogger) {
		ui.Log(ui.ServerLogger, "auth.dsn.init", nil)
	} else {
		ui.Log(ui.AuthLogger, "auth.dsn.init", nil)
	}

	DSNDatabaseURL = userDatabaseFile
	DSNService, err = defineDSNService(userDatabaseFile)

	return err
}

// InitializeFromURL initializes the DSN service directly from a pre-resolved
// database URL or path string. This is used by child service processes that
// receive the URL from the parent server via the request payload, bypassing
// the CLI flag and settings lookup that Initialize() performs.
func InitializeFromURL(url string) error {
	var err error

	DSNService, err = defineDSNService(url)

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
			fullPath, _ := filepath.Abs(path)

			dir := filepath.Dir(fullPath)
			ext := filepath.Ext(fullPath)
			base := strings.TrimSuffix(filepath.Base(fullPath), ext)

			path = filepath.Join(dir, base+"_dsns"+ext)
		}

		DSNService, err = NewFileService(path)
	}

	return DSNService, err
}

// Utility function to determine if a given path is a database URL or
// not.
func isDatabaseURL(path string) bool {
	path = strings.ToLower(path)
	drivers := []string{
		defs.PostgresProvider + "://",
		defs.SqliteProvider + "://",
		defs.DeprecatedSqliteProvider + "://"}

	for _, driver := range drivers {
		if strings.HasPrefix(path, driver) {
			return true
		}
	}

	return false
}
