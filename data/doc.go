// Data is a package that supports builtin (scalar) data types
// as well as complex Ego data types such as Arrays, Maps, and
// Structs.
//
// The data package has two primary functions:
//
//   - Implement instantiateion of, and access to data items (either
//     Ego constructs like Maps, or native Go items like float64).
//
//   - Implement Types, which describe data items, and are used to
//     create complex types.
//
// In the Ego implementation, virtually all user data (that is, values
// created by executing Ego statements) are expressed as interface{}
// objects. This is needed to support running in relaxed or dynamic type
// checking mode. Based on the mode, code can elect to unwrap the
// interface{} value as is, convert it to a compatible type, or return an
// error state.
//
// The data package contains a set of functions for unwrapping interface{}
// values and coercing to a native Go type. These are functions like String(),
// Int32(), or Float64(). If the value provided cannot be converted to the
// implicit type of the accessor function, it returns the zero-value for
// that type. For example, Float64("fred") will return 0.0 since "fred" cannot
// be parsed as a valid floating point value.
//
// The data package also contains operations that will help coerce a value to
// match another type. These are the Coerce() and Normalize() functions. These
// are used during processing of expressions to ensure that a value is of the
// correct (Coerce) or compatable (Normalize) types. For example, adding a
// float64 value to an int32 value requires that they be promoted to the higher
// precision of the two values, so Normalize() would return two values that are
// the float64 versions of the values.
//
// Data also provides Type support. The Type object describes everything Ego knows
// about the type of a value. This includes base types, where the Type holds a
// value that indicates the type of the base value. For complex types like Struct
// or Map, the Type also contains information about key and index value types,
// element types, field names, etc.
//
// The data package includes functions for determining the type of any interface{}
// value (TypeOf()) or testing if an interfaces{} is of a given type (InstanceOf()).
//
// The data package includes helper functions for creating the complex types like
// Struct, Map, Array, and Pointer (to another type). For Struct, the helper
// functions can be given a simple map[string]interface{} to populate the fields
// of the structure with a given set of values for an instance of the struct. Similarly
// for Map, the helper function accepts a map[interface{}]interface{} which populates
// the map using the provided keys and elements, regardless of type. Finally, helper
// functions for Arrays can populate the array with an []interface{} list in the
// native Go code as needed.
//
// The data package includes miscellaneous helper functions as well. For example,
// Format() is used to create a human-readable representation of any interface{}
// value. If the value is of a known type, it converts it appropraitely. If it is
// not a known type, the native reflect package is used to provide descriptive
// information about the value. Additionally, any Type can be converted to a string
// representation. This includes simple types which result in type names like "int32"
// or "bool". Complex types are displayed using the syntax of the Ego declaration that
// defined the type.
package data
