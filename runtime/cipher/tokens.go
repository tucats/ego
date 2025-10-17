package cipher

import (
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/egostrings"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/tokens"
	"github.com/tucats/ego/util"
)

// authToken is the Go native expression of a token value, which contains
// the identity of the creator, an arbitrary data payload, an expiration
// time after which the token is no longer valid, a unique ID for this
// token, and the unique ID of the Ego session that created the token.
type authToken struct {
	Name    string
	Data    string
	TokenID uuid.UUID
	Expires time.Time
	AuthID  uuid.UUID
}

// validate determines if a token is valid and returns true/false.
func Validate(s *symbols.SymbolTable, args data.List) (any, error) {
	var (
		err       error
		reportErr bool
		session   int
	)

	if args.Len() > 1 {
		reportErr, err = data.Bool(args.Get(1))
		if err != nil {
			return nil, errors.New(err).In("cipher.Validate")
		}
	}

	if i, ok := s.Get(defs.SessionVariable); ok {
		session, err = data.Int(i)
		if err != nil {
			return nil, errors.New(err).In("cipher.Validate")
		}
	}

	tokenString := data.String(args.Get(0))

	valid, err := tokens.Validate(tokenString, session)

	if err != nil {
		if reportErr {
			return valid, errors.New(err)
		}

		return valid, nil
	}

	return valid, err
}

// extract extracts the data from a token and returns it as a struct.
func Extract(s *symbols.SymbolTable, args data.List) (any, error) {
	var (
		err     error
		session int
	)

	if i, ok := s.Get(defs.SessionVariable); ok {
		session, err = data.Int(i)
		if err != nil {
			return nil, errors.New(err).In("cipher.Validate")
		}
	}

	t, err := tokens.Unwrap(data.String(args.Get(0)), session)
	if err != nil {
		ui.Log(ui.AuthLogger, "auth.invalid.encoding", ui.A{
			"session": session,
			"error":   err})

		return nil, errors.New(err)
	}

	// Has the expiration passed?
	d := time.Since(t.Expires)
	if d.Seconds() > 0 {
		ui.Log(ui.AuthLogger, "auth.expired", ui.A{
			"session": session,
			"id":      t.TokenID})

		err = errors.ErrExpiredToken.In("Extract")
	}

	ui.Log(ui.AuthLogger, "auth.valid.token", ui.A{
		"session": session,
		"id":      t.TokenID.String(),
		"user":    t.Name,
		"expires": util.FormatDuration(time.Until(t.Expires), true)})

	if err != nil {
		return nil, errors.New(err)
	}

	return data.NewStructOfTypeFromMap(CipherAuthType, map[string]any{
		"Expires": t.Expires.Format(time.RFC822Z),
		"Name":    t.Name,
		"Data":    t.Data,
		"AuthID":  t.AuthID.String(),
		"TokenID": t.TokenID.String(),
	}), err
}

// newToken creates a new token with a username and a data payload.
func NewToken(s *symbols.SymbolTable, args data.List) (any, error) {
	var (
		err                       error
		session                   int
		name, datum, id, interval string
	)

	if i, ok := s.Get(defs.SessionVariable); ok {
		session, err = data.Int(i)
		if err != nil {
			return nil, errors.New(err).In("cipher.Validate")
		}
	}

	// Create a new token object, with the username and an ID. If there was a
	// data payload as well, add that to the token.
	name = data.String(args.Get(0))

	if args.Len() >= 2 {
		datum = data.String(args.Get(1))
	}

	if args.Len() >= 3 {
		interval = data.String(args.Get(2))
	}

	// Get the session ID of the current Ego program and add it to
	// the token. A token can only be validated on the same system
	// that created it.
	if session, ok := s.Get(defs.InstanceUUIDVariable); ok {
		id = data.String(session)
	} else {
		id = uuid.NewString()
		s.SetGlobal(defs.InstanceUUIDVariable, id)
	}

	// Fetch the default interval, or use 15 minutes as the default. If the
	// duration is expressed in days, convert it to hours. Calculate a time
	// value for when this token expires
	defaultDuration := settings.Get(defs.ServerTokenExpirationSetting)
	if interval == "" {
		interval = defaultDuration
		if interval == "" {
			interval = "15m"
		}
	} else {
		if days, err := egostrings.Atoi(strings.TrimSuffix(interval, "d")); err == nil {
			interval = strconv.Itoa(days*24) + "h"
		}
	}

	return tokens.New(name, datum, interval, id, session)
}
