package dsns

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	egostrings "github.com/tucats/ego/internal/util/strings"
)

// SystemDSNName is the name of the DSN automatically created (when enabled
// via the ego.server.dsn.catalog setting) to point at the same database
// used to store the DSN catalog itself.
const SystemDSNName = "ego-system"

// EnsureSystemDSN creates the "ego-system" DSN if it does not already
// exist, pointing at the same database that stores the DSN catalog itself
// (DSNDatabaseURL). It is a no-op unless the DSN store is database-backed
// (a Postgres or SQLite URL, as opposed to the in-memory or JSON file
// store) -- callers are expected to gate on the ego.server.dsn.catalog
// setting before calling this.
//
// The DSN is always created restricted, with no ownership grant made for
// any user, so only an administrator -- who bypasses per-DSN authorization
// checks entirely -- can access it. If the DSN already exists, only its
// Restricted flag is checked and corrected; its connection details are
// left alone since those are fixed by the database this server was
// started against.
func EnsureSystemDSN() error {
	connStr := strings.TrimSuffix(strings.TrimPrefix(DSNDatabaseURL, "\""), "\"")

	if !isDatabaseURL(connStr) {
		return nil
	}

	existing, err := DSNService.ReadDSN(0, "", SystemDSNName, true)
	if err == nil {
		if existing.Restricted {
			return nil
		}

		existing.Restricted = true

		if err := DSNService.WriteDSN(0, "", existing); err != nil {
			return err
		}

		ui.Log(ui.AuthLogger, "auth.dsn.system.restrict", ui.A{"name": SystemDSNName})

		return nil
	}

	if !errors.Equal(err, errors.ErrNoSuchDSN) {
		return err
	}

	systemDSN, err := systemDSNFromURL(connStr)
	if err != nil {
		return err
	}

	if err := DSNService.WriteDSN(0, "", *systemDSN); err != nil {
		return err
	}

	ui.Log(ui.AuthLogger, "auth.dsn.system.create", ui.A{"name": SystemDSNName})

	return nil
}

// systemDSNFromURL builds the "ego-system" DSN definition from the resolved
// database URL used to store the DSN catalog, reusing that same connection
// information (type, host, port, credentials, database name) so the new
// DSN points at the very database that holds it.
func systemDSNFromURL(connStr string) (*defs.DSN, error) {
	scheme, err := egostrings.FindScheme(connStr)
	if err != nil {
		return nil, errors.New(err)
	}

	if scheme == defs.SqliteProvider || scheme == defs.DeprecatedSqliteProvider {
		path := egostrings.StripScheme(connStr)

		return NewDSN(SystemDSNName, defs.SqliteProvider, path, "", "", "", 0, true, false), nil
	}

	u, err := url.Parse(connStr)
	if err != nil {
		return nil, errors.New(err)
	}

	user, password := "", ""
	if u.User != nil {
		user = u.User.Username()
		password, _ = u.User.Password()
	}

	port := 0
	if p := u.Port(); p != "" {
		port, _ = strconv.Atoi(p)
	}

	secured := u.Query().Get("sslmode") != "disable"
	database := strings.TrimPrefix(u.Path, "/")

	return NewDSN(SystemDSNName, defs.PostgresProvider, database, user, password, u.Hostname(), port, true, secured), nil
}
