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
