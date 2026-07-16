package ast

import "reflect"

// Walk traverses the AST rooted at node in depth-first, pre-order, calling fn
// for each node. If fn returns false, the children of that node are not
// visited (but traversal continues with the node's siblings). Because Walk is
// built entirely on Node.Children, it traverses external node types correctly
// with no changes here.
func Walk(node Node, fn func(Node) bool) {
	if node == nil {
		return
	}

	if !fn(node) {
		return
	}

	for _, child := range node.Children() {
		Walk(child, fn)
	}
}

// Inspect is a convenience wrapper around Walk for callers that prefer the
// name used by Go's go/ast package.
func Inspect(node Node, fn func(Node) bool) {
	Walk(node, fn)
}

// nodes is a helper used by Children implementations to build a child slice
// from a mix of individual nodes and node slices while dropping nils (both
// typed-nil interface values and nil slice entries). Centralizing this keeps
// every Children method short and guarantees the "no nil entries" contract.
func nodes(items ...interface{}) []Node {
	var result []Node

	for _, item := range items {
		switch v := item.(type) {
		case nil:
			// skip
		case Node:
			if !isNil(v) {
				result = append(result, v)
			}
		case []Node:
			for _, n := range v {
				if !isNil(n) {
					result = append(result, n)
				}
			}
		}
	}

	return result
}

// isNil reports whether a Node interface value is nil, including the typed-nil
// case where the interface holds a nil pointer of a concrete node type (e.g.
// (*Ident)(nil)), which is non-nil as an interface but must be treated as
// absent. This runs only during tree construction, so the reflection cost is
// immaterial.
func isNil(n Node) bool {
	if n == nil {
		return true
	}

	v := reflect.ValueOf(n)
	switch v.Kind() {
	case reflect.Ptr, reflect.Interface, reflect.Slice, reflect.Map, reflect.Func, reflect.Chan:
		return v.IsNil()
	default:
		return false
	}
}
