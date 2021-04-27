package profile

import "fmt"

const (
	ProfileErrPrefix = "profile"

	InvalidOutputError     = "invalid output type: %s"
	MissingOutputTypeError = "missing output type"
)

type Error struct {
	err error
}

func NewProfileErr(msg string, args ...interface{}) Error {
	return Error{
		err: fmt.Errorf(msg, args...),
	}
}

func (e Error) Error() string {
	return fmt.Sprintf("%s, %s", ProfileErrPrefix, e.err.Error())
}
