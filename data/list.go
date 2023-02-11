package data

// List is a type used to hold multiple values. It is most often
// used to describe a list of return values to be treated as a tuple
// when returning from a builtin or runtime function.
type List struct {
	elements []interface{}
}

// NewList creates a new Values list object, placing the items in the list.
func NewList(items ...interface{}) List {
	return List{elements: items}
}

func (l List) Len() int {
	return len(l.elements)
}

func (l List) Get(n int) interface{} {
	if n < 0 || n >= len(l.elements) {
		return nil
	}

	return l.elements[n]
}
