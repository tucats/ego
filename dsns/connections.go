package dsns

import (
	"crypto/sha256"
	"strconv"
	"strings"

	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
)

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
			result.WriteString(defs.LocalHost)
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

// Externally available function to parse a DSN connection string, which
// defines a DSN lookup operation.
//
//	name, user, session, err := dsns.ParseDSN(connStr)
func ParseDSN(conn string) (name string, user string, session int, err error) {
	// Break the connection string up by comma-separated parts. Scan
	// over each part to see what's in it.
	for part := range strings.SplitSeq(conn, ",") {
		var key, value string

		if strings.Contains(part, "=") {
			fields := strings.Split(part, "=")
			if len(fields) != 2 {
				err = errors.ErrInvalidDSN.Context(conn)
			}

			key = strings.TrimSpace(fields[0])
			value = strings.TrimSpace(fields[1])
		} else {
			key = "name"
			value = strings.TrimSpace(part)
		}

		switch strings.ToLower(key) {
		case "name":
			name = value

		case "user", "username", "u":
			user = value

		case "session":
			session, err = strconv.Atoi(value)
			if err != nil {
				err = errors.New(err)

				return name, user, session, err
			}

		default:
			err = errors.ErrInvalidDSN.Context(conn).Chain(errors.ErrInvalidKeyword.Context(key))
		}
	}

	return name, user, session, err
}
