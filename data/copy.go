package data

import (
	"fmt"

	"github.com/tucats/ego/errors"
)

// maxCopyDepth is the recursion limit used by the one-argument Copy function.
// Any object whose nesting level exceeds this value returns ErrCopyDepthExceeded
// rather than crashing the Go runtime with a stack overflow.
const maxCopyDepth = 100

// Copy creates a fully independent deep copy of any Ego value, using a
// maximum recursion depth of 100.  For callers that need a different limit
// use CopyDepth instead.
//
// Unlike the existing DeepCopy helper (which silently returns unknown types
// unchanged), Copy returns an error when it encounters:
//   - a Go type that does not belong to the Ego data model, or
//   - a nesting depth that would exceed the recursion limit.
//
// See CopyDepth for a full description of type handling.
//
// Example usage:
//
//	newItem, err := data.Copy(oldItem)
func Copy(v any) (any, error) {
	return CopyDepth(v, maxCopyDepth)
}

// CopyDepth is like Copy but lets the caller set the maximum recursion depth
// explicitly.  maxDepth is the number of nesting levels the function is
// allowed to descend into before returning ErrCopyDepthExceeded.
//
// A maxDepth of 1 is sufficient for any scalar value and for empty containers.
// Each additional level of nesting (array of arrays, struct containing a map,
// etc.) consumes one unit of depth.  When in doubt, use Copy (depth 100) and
// reserve CopyDepth for code that must guard against pathologically deep or
// self-referential structures.
//
// How each type is handled
// ─────────────────────────
//
//	nil                     → nil is returned with no error.
//
//	bool, byte, int8 …      → Scalar values are returned as-is.  In Go, scalar
//	float32, string, etc.     types are always copied on assignment, so the
//	                          returned value is already independent.
//
//	Immutable               → The inner value is recursively copied (depth-1)
//	                          and the result is re-wrapped in a new Immutable.
//
//	Interface               → The inner value is recursively copied (depth-1)
//	                          and the result is re-wrapped in a new Interface.
//
//	*Array                  → A new *Array of the same element type and length
//	                          is allocated; each element is copied at depth-1.
//
//	*Map                    → A new *Map with the same key/value types is
//	                          allocated; each value is copied at depth-1.
//
//	*Struct                 → A new *Struct with the same type metadata is
//	                          allocated; each field is copied at depth-1.
//
//	*Channel                → Channels are inherently shared communication
//	                          objects; the original pointer is returned unchanged.
//
//	*Type                   → Type objects are shared singletons; the original
//	                          pointer is returned unchanged.
//
//	Function                → Function descriptors are immutable after
//	                          compilation; the original value is returned.
//
//	Package                 → Package values are global singletons; the
//	                          original is returned unchanged.
//
//	Ego scalar pointer      → The pointer is returned unchanged so original
//	(*int, *bool, etc.)       and copy share the same target (pointer semantics).
//
//	Ego reference pointer   → Same reasoning: **Map, **Array, etc. are
//	(**Map, **Array, etc.)    returned unchanged.
//
//	maxDepth exhausted      → ErrCopyDepthExceeded is returned.
//
//	anything else           → ErrNotCopyable is returned with the Go type name
//	                          appended as context.
//
// Example usage:
//
//	newItem, err := data.CopyDepth(oldItem, 10)
func CopyDepth(v any, maxDepth int) (any, error) {
	return copyValue(v, maxDepth)
}

// copyValue is the recursive implementation shared by Copy and CopyDepth.
// depth is the number of additional recursion levels that are permitted after
// this call returns; when it reaches zero the function returns ErrCopyDepthExceeded
// rather than recursing further.
//
// The public API (Copy / CopyDepth) is kept thin so callers never need to
// pass the depth counter themselves.
func copyValue(v any, depth int) (any, error) {
	// Refuse to recurse beyond the caller's stated limit.  This protects
	// against pathologically deep or accidentally self-referential structures
	// that would otherwise exhaust the Go call stack.
	if depth <= 0 {
		return nil, errors.ErrCopyDepthExceeded
	}

	// A nil value has nothing to copy.
	if v == nil {
		return nil, nil
	}

	switch actual := v.(type) {
	// ── Scalar value types ────────────────────────────────────────────────────
	//
	// In Go, value types are copied whenever they are assigned to a new variable
	// or returned from a function.  Returning "actual" (the unwrapped concrete
	// type) rather than "v" (the interface wrapper) ensures the caller gets a
	// true value-type copy, not an interface box holding the same value.
	// No recursion is needed, so depth is not decremented for scalars.
	case bool:
		return actual, nil
	case byte:
		return actual, nil
	case int8:
		return actual, nil
	case int16:
		return actual, nil
	case uint16:
		return actual, nil
	case int32:
		return actual, nil
	case uint32:
		return actual, nil
	case int:
		return actual, nil
	case uint:
		return actual, nil
	case int64:
		return actual, nil
	case uint64:
		return actual, nil
	case float32:
		return actual, nil
	case float64:
		return actual, nil
	case string:
		return actual, nil

	// ── Immutable (Ego read-only constant wrapper) ────────────────────────────
	//
	// Immutable wraps a value to prevent assignment to it.  We copy the inner
	// value recursively at depth-1 so the new Immutable is fully independent,
	// then re-wrap it so the read-only constraint is preserved.
	case Immutable:
		copied, err := copyValue(actual.Value, depth-1)
		if err != nil {
			return nil, err
		}

		return Immutable{Value: copied}, nil

	// ── Interface (Ego "any" wrapper) ─────────────────────────────────────────
	//
	// Interface bundles a value with its Ego type so the runtime always knows
	// the concrete type of an "any"-typed variable.  Copy the inner value
	// recursively at depth-1, then rebuild the wrapper.  BaseType is a *Type
	// singleton so it is safe to share between original and copy.
	case Interface:
		copied, err := copyValue(actual.Value, depth-1)
		if err != nil {
			return nil, err
		}

		return Interface{Value: copied, BaseType: actual.BaseType}, nil

	// ── *Array ────────────────────────────────────────────────────────────────
	//
	// Allocate a new Array of the same element type and length, then populate
	// it with recursive copies of each element at depth-1.  Byte arrays use a
	// dedicated []byte slice internally and get a faster bulk-copy path.
	case *Array:
		size := actual.Len()
		result := NewArray(actual.valueType, size)

		if actual.valueType.kind == ByteKind {
			// Byte arrays are stored as a plain []byte; a single bulk copy is
			// all that is needed since bytes are scalar values.
			copy(result.bytes, actual.bytes)
		} else {
			for i := 0; i < size; i++ {
				elem, _ := actual.Get(i)

				copied, err := copyValue(elem, depth-1)
				if err != nil {
					// Propagate depth-exceeded and not-copyable errors as-is so
					// the caller knows exactly why the copy failed.
					return nil, err
				}

				// Set cannot fail on a freshly allocated, non-immutable array,
				// so we discard the error return.
				_ = result.Set(i, copied)
			}
		}

		return result, nil

	// ── *Map ──────────────────────────────────────────────────────────────────
	//
	// Allocate a new Map with the same key and value type declarations, then
	// populate it with recursively copied values at depth-1.  Map keys in Ego
	// are always scalars (string, int, float64) so we do not need to recurse
	// into them.
	case *Map:
		result := NewMap(actual.keyType, actual.elementType)
		keys := actual.Keys()

		for _, k := range keys {
			val, _, _ := actual.Get(k)

			copiedVal, err := copyValue(val, depth-1)
			if err != nil {
				return nil, err
			}

			// Set can only fail on a type mismatch, which cannot happen here
			// because we are copying from a map with identical type declarations.
			_, _ = result.Set(k, copiedVal)
		}

		return result, nil

	// ── *Struct ───────────────────────────────────────────────────────────────
	//
	// Struct.Copy() duplicates all metadata (type definition, static/readonly
	// flags, field order) without sharing any field values.  We then replace
	// the fields map with a fresh one whose values are recursively copied at
	// depth-1.
	case *Struct:
		result := actual.Copy()

		// Replace the fields map that Struct.Copy() carried over with a fresh
		// map containing independently copied values.
		result.fields = make(map[string]any, len(actual.fields))

		for fieldName, fieldVal := range actual.fields {
			copied, err := copyValue(fieldVal, depth-1)
			if err != nil {
				return nil, err
			}

			result.fields[fieldName] = copied
		}

		return result, nil

	// ── *Channel ──────────────────────────────────────────────────────────────
	//
	// Channels are inherently shared communication objects: the whole purpose of
	// a channel is that multiple goroutines send and receive on the same queue.
	// A "copy" of a channel is simply the same channel, so we return the
	// original pointer without error and without decrementing depth.
	case *Channel:
		return actual, nil

	// ── *Type ─────────────────────────────────────────────────────────────────
	//
	// Type objects describe Ego types and are shared singletons throughout the
	// runtime.  Copying them would break type-identity comparisons (==), so
	// we return the original pointer unchanged.
	case *Type:
		return actual, nil

	// ── Function ─────────────────────────────────────────────────────────────
	//
	// Function descriptors (including the bytecode pointer they carry) are
	// immutable after compilation.  Both original and copy can safely share
	// the same Function value.
	case Function:
		return actual, nil

	// ── Package ───────────────────────────────────────────────────────────────
	//
	// Package values are global singletons (one per imported package).  Sharing
	// the same Package between original and copy is correct and expected.
	case Package:
		return actual, nil

	// ── Ego scalar pointer types ──────────────────────────────────────────────
	//
	// These pointer types are produced by AddressOf when Ego code applies the &
	// operator to a scalar variable (e.g. "&myInt" → *int).  The whole point of
	// a pointer is that two references to the same value see each other's writes.
	// We return the pointer unchanged; depth is not decremented because no
	// recursion is needed.
	case *bool:
		return actual, nil
	case *byte:
		return actual, nil
	case *int8:
		return actual, nil
	case *int16:
		return actual, nil
	case *uint16:
		return actual, nil
	case *int32:
		return actual, nil
	case *uint32:
		return actual, nil
	case *int:
		return actual, nil
	case *uint:
		return actual, nil
	case *int64:
		return actual, nil
	case *uint64:
		return actual, nil
	case *float32:
		return actual, nil
	case *float64:
		return actual, nil
	case *string:
		return actual, nil
	case *any:
		// *any is the fallback pointer type returned by AddressOf for types it
		// does not recognize explicitly.  Same pointer semantics apply.
		return actual, nil

	// ── Ego reference pointer types ───────────────────────────────────────────
	//
	// When Ego code applies & to an Ego reference type (e.g. "&myMap"),
	// AddressOf returns a double pointer (**Map, **Array, etc.) so that
	// dereferencing it gives back the *Map/*Array rather than copying the whole
	// object.  Both original and copy point to the same underlying object —
	// correct pointer semantics.  Depth is not decremented.
	case **Map:
		return actual, nil
	case **Array:
		return actual, nil
	case **Struct:
		return actual, nil
	case **Channel:
		return actual, nil

	// ── Unknown / non-data type ───────────────────────────────────────────────
	//
	// Any type that is not part of the Ego data model (native Go structs, raw
	// function values, etc.) cannot be safely deep-copied because we have no
	// way to enumerate its contents.  Return ErrNotCopyable with the Go type
	// name as context so the caller can produce a useful error message.
	default:
		return nil, errors.ErrNotCopyable.Context(fmt.Sprintf("%T", v))
	}
}
