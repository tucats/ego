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
// The data package contains 
package data
