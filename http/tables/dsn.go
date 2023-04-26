package tables

import (
	"strconv"
	"strings"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/util"
)

type DSN struct {
	// Name of this data source name
	Name string `json:"name"`

	// Name of database on server
	Database string `json:"database"`

	// Host name of remote database server
	Host string `json:"host"`

	// Port number to connect on. If zero, no port specified.
	Port int `json:"port"`

	// Usename to send as database credential
	Username string `json:"user"`

	// Password to send as database credental (always encrypted)
	Password string `json:"password,omitempty"`

	// True if the connection should use TLS communications
	Secured bool `json:"secured"`

	// True if we skip Ego database access checks and depend on database.
	Native bool `json:"native"`
}

func NewDSN(name string, native bool) *DSN {
	return &DSN{
		Name:     name,
		Native:   native,
		Database: name,
		Port:     5432,
	}
}

func (d *DSN) DB(dbname string) *DSN {
	d.Database = dbname

	return d
}

func (d *DSN) User(user string, password string) *DSN {
	d.Username = user
	if password != "" {
		d.Password, _ = util.Encrypt(password, settings.Get(defs.ServerTokenKeySetting))
	}

	return d
}

func (d *DSN) Address(host string, port int, secured bool) *DSN {
	if host == "" {
		host = "localhost"
	}

	d.Host = host
	d.Port = port
	d.Secured = secured

	return d
}

func (d *DSN) Connection() (string, error) {
	var (
		err error
		pw  string
	)

	result := strings.Builder{}

	result.WriteString("postgres://")

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
	result.WriteString(d.Database)

	if !d.Secured {
		result.WriteString("?sslmode=disable")
	}

	return result.String(), err
}
