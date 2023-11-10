package errors

// Equal compares two errors for equality. If either error is nil, the result is false.
func Equal(err1, err2 error) bool {
	if err1 == nil && err2 == nil {
		return true
	}

	if err1 == nil || err2 == nil {
		return false
	}

	if e, ok := err1.(*Error); ok {
		err1 = e.err
	}

	if e, ok := err2.(*Error); ok {
		err2 = e.err
	}

	return err1.Error() == err2.Error()
}
