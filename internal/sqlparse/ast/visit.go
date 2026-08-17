package ast

import "reflect"

// This file implements generic tree traversal over any Node, plus a couple
// of small helpers that every Children() method in this package leans on.
// If you're comfortable with the visitor pattern and Go's type system this
// file holds no surprises except isNil, which is worth reading carefully —
// see its comment below for what a "typed nil" is and why it matters here.

// Walk traverses the AST rooted at node in depth-first, pre-order, calling fn
// for each node. If fn returns false, the children of that node are not
// visited (but traversal continues with the node's siblings). Because Walk is
// built entirely on Node.Children, it traverses external node types correctly
// with no changes here.
//
// fn is a callback: a plain function value passed in by the caller, the same
// way you'd pass a closure to sort.Slice or a comparison function to a
// generic algorithm in most languages. Walk doesn't know or care what fn
// does; it just calls it once per node and looks at the bool it returns.
// This is the standard shape of a "visitor" in Go — no Visitor interface or
// double-dispatch machinery needed, just a function value.
func Walk(node Node, fn func(Node) bool) {
	if node == nil || isNil(node) {
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

// nodes is a helper used by every Children() method in this package (see
// expr.go, stmt.go, ddl.go, ...) to build a child slice from a mix of
// individual nodes and node slices, while dropping absent ones. A typical
// call looks like:
//
//	func (n *BinaryExpr) Children() []Node { return nodes(n.X, n.Y) }
//	func (n *SelectCore) Children() []Node { return nodes(cols, n.From, n.Where, n.GroupBy, n.Having) }
//
// where n.X/n.Y are single Node fields and n.From/n.GroupBy are []Node
// fields, and some of them (n.Where, a WHERE clause) may be nil because the
// statement didn't have one. nodes() lets every Children() method hand all
// of its fields to one call and not worry about which ones are set.
//
// The parameter type "items ...interface{}" means nodes can be called with
// any number of arguments (that's what the "..." does — see the two call
// sites above passing two and five arguments respectively), and each one can
// be of any type (that's what interface{} means: the empty interface, which
// every Go value satisfies since it requires zero methods; in code you may
// see elsewhere written as the newer alias "any"). Here only two shapes are
// actually meaningful, Node and []Node, and both are handled below.
func nodes(items ...interface{}) []Node {
	var result []Node

	for _, item := range items {
		// This is a Go "type switch": unlike a normal switch, it branches on
		// the concrete type stored inside the interface{} value item, not on
		// item's value. Each case re-binds v to that case's type, so inside
		// "case Node:", v has type Node, and inside "case []Node:", v has
		// type []Node — the compiler enforces this, you can't accidentally
		// use v as the wrong type in the wrong branch.
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
// absent.
//
// Background for readers who haven't hit this before: a Go interface value
// is really a pair under the hood — (concrete type, concrete value). The
// literal nil, and a zero-value interface variable, are the pair (nil type,
// nil value). But if you take a nil pointer of a concrete type and store it
// in an interface variable —
//
//	var x *ColumnRef        // x is nil, type *ColumnRef
//	var n Node = x           // n is now the pair (*ColumnRef, nil) — NOT (nil, nil)
//	n == nil                 // false! n has a type, even though its value is nil.
//
// — the resulting interface value is NOT == nil, because its type half is
// non-nil, even though the pointer it holds is nil. This trips up a lot of
// Go code: a field like "type CaseExpr struct { ... Else Node }" set from an
// uninitialized *SomeExprType variable will hold a non-nil Node that is
// nonetheless unsafe to use as if it pointed to a real node.
//
// It matters here because several parser fields are typed as concrete
// pointers before being assigned into a Node-typed struct field (see e.g.
// CastExpr.Type, which is *TypeName, embedded via nodes(n.X, n.Type) in
// CastExpr.Children()); if such a pointer is ever left nil, a plain "n ==
// nil" check inside nodes() would miss it and Walk would recurse into a nil
// pointer. isNil uses the reflect package to look underneath the interface
// at its dynamic type and, for the kinds of types that can be nil (pointers,
// interfaces, slices, maps, funcs, channels), asks reflect.Value.IsNil()
// directly instead of relying on == nil.
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
