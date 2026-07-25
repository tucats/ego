package defs

import "errors"

var AbortError = "connect: connection refused"
var ErrAbort = errors.New("connection refused or timed out")

const (
	DeleteTask = "DELETE"
)
