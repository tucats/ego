package profile

import "fmt"

const (
	ProfileErrPrefix = "profile"

	InvalidOutputError     = "invalid output type: %s"
	MissingOutputTypeError = "missing output type"
)

type ProfileErr struct {
	err error
}

func NewProfileErr(msg string, args ...interface{}) ProfileErr {
	return ProfileErr{
		err: fmt.Errorf(msg, args...),
	}
}

func (e ProfileErr) Error() string {
	return fmt.Sprintf("%s, %s", ProfileErrPrefix, e.err.Error())
}
