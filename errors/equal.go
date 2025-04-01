package errors

// Equal compares two errors for equality. If either error is nil, the result is false.
func Equal(err1, err2 error) bool {
	if err1 == nil && err2 == nil {
		return true
	}

	if err1 == nil || err2 == nil {
		return false
	}

	if e, ok := err1.(*Error); ok && e != nil {
		err1 = e.err
	}

	if e, ok := err2.(*Error); ok && e != nil {
		err2 = e.err
	}

	s1 := err1.Error()
	s2 := err2.Error()

	return s1 == s2
}

// Is compares the current error to the supplied error, and
// return a boolean indicating if they are the same.
func (e *Error) Is(err error) bool {
	if e == nil && err == nil {
		return true
	}

	if e == nil || err == nil || e.err == nil {
		return false
	}

	// Is the test error one of the Ego "native" errors? If so
	// we need to compare both underlying error states.
	if e1, ok := err.(*Error); ok {
		if e1 == nil || e.err == nil || e1.err == nil {
			return false
		}

		return e.err.Error() == e1.err.Error()
	}

	// Otherwise, we're comparing against a Go native error, so
	// we compare our underlying error to the provided error.
	return e.err.Error() == err.Error()
}

// Similar to the Is() method, but this one compares any two error
// values without regard for whether they are Ego errors or not.
func Equals(e1, e2 error) bool {
	if e1 == nil && e2 == nil {
		return true
	}

	if e1 == nil || e2 == nil {
		return false
	}

	if e, ok := e1.(*Error); ok {
		return e.Is(e2)
	}

	return e1 == e2
}

// Equal compares an error to an arbitrary object. If the
// object is not an error, then the result is always false.
// If it is a native error or an Ego Error, the error and
// wrapped error are compared.
func (e *Error) Equal(v interface{}) bool {
	if e == nil {
		return v == nil
	}

	if v == nil {
		return Nil(e)
	}

	switch a := v.(type) {
	case *Error:
		return e.err == a.err

	case error:
		return e.err == a

	default:
		return false
	}
}

// Nil tests to see if the error is "nil". If it is a native Go
// error, it is just tested to see if it is nil. If it is an
// Ego Error then additionally we test to see if it is a valid
// pointer but to a null error, in which case it is also considered
// a nil value.
func Nil(e error) bool {
	if e == nil {
		return true
	}

	if ee, ok := e.(*Error); ok {
		if ee == nil {
			return true
		}

		return ee.err == nil
	}

	return false
}

// SameBaseError checks to see if the two errors are the same base error,
// irrespective of any additional attributes of the error, if the error
// is an Ego error. This allows comparisons for error types without concern
// for line numbers, module names, etc.
func SameBaseError(e error, other error) bool {
	var t1, t2 string

	if e == nil && other == nil {
		return true
	}

	if e == nil || other == nil {
		return false
	}

	t1 = e.Error()
	if err, ok := e.(*Error); ok {
		t1 = err.err.Error()
	}

	t2 = other.Error()
	if err, ok := other.(*Error); ok {
		t2 = err.err.Error()
	}

	return t1 == t2
}
