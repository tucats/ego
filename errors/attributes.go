package errors

func (e *Error) SetUser(user bool) *Error {
	if e != nil {
		e.user = user
	}

	return e
}

// Returns true if this is a user-defined error.
func (e *Error) IsUser() bool {
	if e == nil {
		return false
	}

	return e.user
}

// Unwrap retrieves the native or wrapped error from this
// Error.
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}

	return e.err
}
