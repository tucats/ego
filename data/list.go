package data

import (
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/errors"
)

// List is a type used to hold multiple values. It is most often
// used to describe a list of return values to be treated as a tuple
// when returning from a builtin or runtime function. It is also used
// as the argument list to native functions.
type List struct {
	elements []any
}

// NewList creates a new Values list object, placing the items in the list.
func NewList(items ...any) List {
	return List{elements: items}
}

// Len returns an integer value indicating the length of the list.
func (l List) Len() int {
	return len(l.elements)
}

// Get retrieves the nth value from the list. If the index is less than
// zero or greater than the size of the list, nil is returned.
func (l List) Get(n int) any {
	if n < 0 || n >= len(l.elements) {
		return nil
	}

	// If the item is an immutable value, return the embedded value.
	v := l.elements[n]
	if c, ok := v.(Immutable); ok {
		v = c.Value
	}

	return v
}

// Get retrieves the nth value from the list and returns it as an
// int value. If there is no such element in the list, zero is returned.
func (l List) GetInt(n int) (int, error) {
	if n < 0 || n >= len(l.elements) {
		return 0, errors.ErrArrayIndex.Context(n)
	}

	// If the item is an immutable value, return the embedded value.
	v := l.elements[n]
	if c, ok := v.(Immutable); ok {
		v = c.Value
	}

	return Int(v)
}

// Set stores the nth value from the list. If the index is less than
// zero or greater than the size of the list, no operation is performed.
func (l *List) Set(n int, value any) {
	if l == nil {
		ui.Log(ui.InternalLogger, "runtime.list.nil.set", nil)

		return
	}

	if n >= 0 && n < len(l.elements) {
		l.elements[n] = value
	}
}

// Elements returns an array of interface elements reflecting the individual
// items stored in the list.
func (l *List) Elements() []any {
	if l == nil {
		ui.Log(ui.InternalLogger, "runtime.list.nil.read", nil)

		return nil
	}

	return l.elements
}

// Slice returns a new List that is a subset of the original list. It is built
// using native Go slices of the list elements, so the storage is not duplicated.
func (l *List) Slice(begin, end int) List {
	if l == nil {
		ui.Log(ui.InternalLogger, "runtime.list.nil.slice", nil)

		return List{nil}
	}

	if begin < 0 || begin > len(l.elements) || end < begin || end > len(l.elements) {
		return List{nil}
	}

	return List{l.elements[begin:end]}
}

// Append adds elements to the list. The argument list can be any interface{}
// desired, and it is added to the list. The function returns the number of
// elements now in the list.
func (l *List) Append(i ...any) int {
	if l == nil {
		ui.Log(ui.InternalLogger, "runtime.list.nil.append", nil)

		return 0
	}

	l.elements = append(l.elements, i...)

	return len(l.elements)
}

func (l List) String() string {
	var b strings.Builder

	b.WriteString("[")

	for i, v := range l.elements {
		if i > 0 {
			b.WriteString(", ")
		}

		b.WriteString(Format(v))
	}

	b.WriteByte(']')

	return b.String()
}
