package dsns

import (
	"github.com/google/uuid"
	"github.com/tucats/ego/defs"
)

// NewDSN creates a new DSN object with the given parameters. The name
// is the name of the DSN, and the provider is the database provider
// (e.g. "postgres" or "sqlite3"). The database is the name of the
// database to connect to, and the user and password are the credentials
// to use to connect to the database. The host and port are the network
// address of the database server, and the native and secured flags
// indicate if the database is a native database (e.g. not a cloud
// service) and if the connection should be secured.
func NewDSN(name, provider, database, user, password string, host string, port int, restricted, secured bool) *defs.DSN {
	if database == "" {
		database = name
	} else if name == "" {
		name = database
	}

	if port < 32 {
		port = 5432
	}

	if host == "" {
		host = defs.LocalHost
	}

	if password != "" {
		password, _ = encrypt(password)
	}

	if provider == "" {
		provider = "sqlite3"
	}

	return &defs.DSN{
		Name:       name,
		ID:         uuid.NewString(),
		Provider:   provider,
		Restricted: restricted,
		Username:   user,
		Password:   password,
		Database:   database,
		Host:       host,
		Port:       port,
		Secured:    secured,
	}
}
