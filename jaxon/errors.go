package jaxon

import "fmt"

type Error struct {
	Code string
	Ctx  string
}

const (
	ErrArrayIndex          = "jaxon.array.index"
	ErrArrayType           = "jaxon.array.type"
	ErrArrayNotFound       = "jaxon.array.not.found"
	ErrNotFound            = "jaxon.not.found"
	ErrAmbiguous           = "jaxon.ambiguous"
	ErrJSONQuery           = "jaxon.json.query"
	ErrJSONElementNotFound = "jaxon.json.element.not.found"
	ErrJSONInvalidContent  = "jaxon.json.invalid.content"
	ErrInvalidInteger      = "jaxon.invalid.integer"
	ErrInvalidRange        = "jaxon.invalid.range"
)

func Err(code string) *Error {
	return &Error{
		Code: code,
		Ctx:  "",
	}
}

func (e *Error) Context(ctx any) *Error {
	if e != nil {
		e.Ctx = fmt.Sprintf("%v", ctx)
	}

	return e
}

func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}

	if e.Ctx == "" {
		return e.Code
	}

	return e.Code + ": " + e.Ctx
}

func (e *Error) Extract() (string, string) {
	if e == nil {
		return "", ""
	}

	return e.Code, e.Ctx
}
