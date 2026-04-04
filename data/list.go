package data

import (
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/errors"
)

// List is an ordered, indexed collection of any values.  It is most often
// used in two places:
//
//  1. Return-value tuples — when an Ego function returns more than one value
//     the results are bundled into a List so a single any can carry them all.
//
//  2. Argument lists — when calling a native Go function through the Ego
//     runtime, the arguments are packed into a List and unpacked on the other
//     side.
//
// Unlike Array, List has no element-type constraint and no immutability
// support — it is a lightweight, untyped container for temporary use.
type List struct {
	elements []any // the stored values, in order
}

// NewList creates a List pre-populated with the given items.  The variadic
// "items ...any" syntax accepts any number of arguments of any type, so a
// caller can write NewList(1, "hello", true) to build a three-element list.
func NewList(items ...any) List {
	return List{elements: items}
}

// Len returns the number of elements in the list.
func (l List) Len() int {
	return len(l.elements)
}

// Get returns the nth element (zero-based).  If n is out of range, nil is
// returned rather than panicking.
//
// If the stored element is wrapped in an Immutable, the wrapper is stripped
// before returning so callers always get the underlying value.
func (l List) Get(n int) any {
	if n < 0 || n >= len(l.elements) {
		return nil
	}

	// Unwrap Immutable constants so the caller works with the real value.
	v := l.elements[n]
	if c, ok := v.(Immutable); ok {
		v = c.Value
	}

	return v
}

// GetInt returns the nth element converted to int.  If n is out of range, an
// error is returned.  If the element is an Immutable, it is unwrapped first.
func (l List) GetInt(n int) (int, error) {
	if n < 0 || n >= len(l.elements) {
		return 0, errors.ErrArrayIndex.Context(n)
	}

	v := l.elements[n]
	if c, ok := v.(Immutable); ok {
		v = c.Value
	}

	return Int(v)
}

// Set replaces the nth element with value.  If n is out of range the call
// is silently ignored — no panic, no error.  The method receiver is a pointer
// (*List) so the change is visible to the caller.
func (l *List) Set(n int, value any) {
	if l == nil {
		ui.Log(ui.InternalLogger, "runtime.list.nil.set", nil)

		return
	}

	if n >= 0 && n < len(l.elements) {
		l.elements[n] = value
	}
}

// Elements returns the raw []any slice that backs the list.  The slice is
// not copied, so the caller must not modify it directly.  Use Set or Append
// for mutations.
func (l *List) Elements() []any {
	if l == nil {
		ui.Log(ui.InternalLogger, "runtime.list.nil.read", nil)

		return nil
	}

	return l.elements
}

// Slice returns a new List that shares the underlying element storage with
// the original list in the range [begin, end).  No data is copied —
// "slice" here has the same semantics as a Go slice expression a[begin:end].
//
// If begin or end are out of range or inconsistent, an empty list is returned.
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

// Append adds one or more items to the end of the list.  It returns the new
// length of the list.  The method receiver is a pointer so the caller sees
// the updated list.
func (l *List) Append(i ...any) int {
	if l == nil {
		ui.Log(ui.InternalLogger, "runtime.list.nil.append", nil)

		return 0
	}

	l.elements = append(l.elements, i...)

	return len(l.elements)
}

// String produces a human-readable representation of the list, e.g.
// "[1, "hello", true]".  It delegates to Format for each element so
// all Ego types are displayed consistently.
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
