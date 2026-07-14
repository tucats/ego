package cipher

import (
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/util/strings"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/symbols"
	"github.com/tucats/ego/internal/language/tokens"
)

// Validate determines if a token is valid and returns true/false. The reason a
// token failed validation (expired, tampered with, blacklisted, etc.) is not
// reported here; call Extract() instead if that detail is needed, since it
// performs the same checks and also returns the underlying error.
func Validate(s *symbols.SymbolTable, args data.List) (any, error) {
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

	tokenString := data.String(args.Get(0))

	valid, _ := tokens.Validate(tokenString, session)

	return valid, nil
}

// Extract extracts the data from a token and returns it as a Token struct. Unlike
// Validate, the underlying error is returned so the caller can distinguish between
// an expired token, a tampered/invalid token, and other failures.
func Extract(s *symbols.SymbolTable, args data.List) (any, error) {
	var (
		err     error
		session int
	)

	if i, ok := s.Get(defs.SessionVariable); ok {
		session, err = data.Int(i)
		if err != nil {
			err = errors.New(err).In("cipher.Extract")

			return data.NewList(nil, err), err
		}
	}

	t, err := tokens.Unwrap(data.String(args.Get(0)), session)
	if err != nil {
		if ui.IsActive(ui.AuthLogger) {
			ui.Log(ui.AuthLogger, "auth.invalid.encoding", ui.A{
				"session": session,
				"error":   err})
		}

		err = errors.New(err)

		return data.NewList(nil, err), err
	}

	// Has the expiration passed?
	if time.Since(t.Expires).Seconds() > 0 {
		if ui.IsActive(ui.AuthLogger) {
			ui.Log(ui.AuthLogger, "auth.expired", ui.A{
				"session": session,
				"id":      t.TokenID})
		}

		err = errors.ErrExpiredToken.In("cipher.Extract")

		return data.NewList(nil, err), err
	}

	result := data.NewStructOfTypeFromMap(CipherAuthType, map[string]any{
		"Expires": t.Expires.Format(time.RFC822Z),
		"Name":    t.Name,
		"Data":    t.Data,
		"AuthID":  t.AuthID.String(),
		"TokenID": t.TokenID.String(),
	})

	return data.NewList(result, nil), nil
}

// NewToken creates a new token with a username and a data payload.
func NewToken(s *symbols.SymbolTable, args data.List) (any, error) {
	var (
		err                       error
		session                   int
		name, datum, id, interval string
	)

	if i, ok := s.Get(defs.SessionVariable); ok {
		session, err = data.Int(i)
		if err != nil {
			err = errors.New(err).In("cipher.New")

			return data.NewList(nil, err), err
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

	result, err := tokens.New(name, datum, interval, id, session)

	return data.NewList(result, err), err
}
