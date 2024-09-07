package strings

import (
	nativestrings "strings"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// Initialize the strings package Builder type. This is a wrapper around the
// native strings.Builder type.
func initializeBuilder() *data.Type {
	// Generate the Builder type, which is just a struct wrapper around a single
	// field that will contain the native builder object.
	structType := data.StructureType().DefineField("builder", data.InterfaceType)
	t := data.TypeDefinition("Builder", structType)

	// Define the Write function for the Builder type.
	t.DefineFunction("Write", &data.Declaration{
		Name: "Write",
		Type: data.PointerType(t),
		Parameters: []data.Parameter{
			{
				Name: "bytes",
				Type: data.ArrayType(data.ByteType),
			},
		},
		Returns: []*data.Type{data.IntType, data.ErrorType},
	}, writeBytes)

	// Define the WriteString function for the Builder type.
	t.DefineFunction("WriteString", &data.Declaration{
		Name: "WriteString",
		Type: data.PointerType(t),
		Parameters: []data.Parameter{
			{
				Name: "text",
				Type: data.StringType,
			},
		},
		Returns: []*data.Type{data.IntType, data.ErrorType},
	}, writeString)

	// Define the WriteRune function for the Builder type.
	t.DefineFunction("WriteRune", &data.Declaration{
		Name: "WriteRune",
		Type: data.PointerType(t),
		Parameters: []data.Parameter{
			{
				Name: "rune",
				Type: data.Int32Type,
			},
		},
		Returns: []*data.Type{data.IntType, data.ErrorType},
	}, writeRune)

	// Define the WriteByte function for the Builder type.
	t.DefineFunction("WriteByte", &data.Declaration{
		Name: "WriteByte",
		Type: data.PointerType(t),
		Parameters: []data.Parameter{
			{
				Name: "value",
				Type: data.ByteType,
			},
		},
		Returns: []*data.Type{data.ErrorType},
	}, writeByte)

	// Define the String function for the Builder type.
	t.DefineFunction("String", &data.Declaration{
		Name:    "String",
		Type:    data.PointerType(t),
		Returns: []*data.Type{data.StringType},
	}, builderToString)

	// Define the Len function for the Builder type.
	t.DefineFunction("Len", &data.Declaration{
		Name:    "Len",
		Type:    data.PointerType(t),
		Returns: []*data.Type{data.IntType},
	}, builderLen)

	// Define the Cap function for the Builder type.
	t.DefineFunction("Cap", &data.Declaration{
		Name:    "Cap",
		Type:    data.PointerType(t),
		Returns: []*data.Type{data.IntType},
	}, builderCap)

	// Define the Grow function for the Builder type.
	t.DefineFunction("Grow", &data.Declaration{
		Name: "Grow",
		Type: data.PointerType(t),
		Parameters: []data.Parameter{
			{
				Name: "bytes",
				Type: data.IntType,
			},
		},
	}, builderGrow)

	// Define the Reset function for the Builder type.
	t.DefineFunction("Reset", &data.Declaration{
		Name: "Reset",
		Type: data.PointerType(t),
	}, builderReset)

	// Define this type as part of the strings package and return it to the caller,
	// which is the strings package initialization function.
	return t.SetPackage("strings")
}

// Implement the Builder funcion Len() which returns the current length of the
// builder's internal buffer.
func builderLen(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	b, err := getBuilder(s)
	if err != nil {
		return nil, err
	}

	return b.Len(), nil
}

// Implement the Builder funcion Reset() which resets the builder's internal buffer.
func builderReset(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	b, err := getBuilder(s)
	if err != nil {
		return nil, err
	}

	b.Reset()

	return nil, nil
}

// Implement the Builder funcion Cap() which returns the current capacity of the
// builder's internal buffer.
func builderCap(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	b, err := getBuilder(s)
	if err != nil {
		return nil, err
	}

	return b.Cap(), nil
}

// Implement the Builder funcion Grow() which grows the builder's internal buffer
// by the specified number of bytes.
func builderGrow(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	b, err := getBuilder(s)
	if err != nil {
		return nil, err
	}

	bytes := data.Int(args.Get(0))
	if bytes < 0 {
		return nil, errors.ErrInvalidValue
	}

	b.Grow(bytes)

	return nil, nil
}

// Implement the Builder funcion WriteString() which writes a string to the builder's
// internal buffer.
func writeString(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	b, err := getBuilder(s)
	if err != nil {
		return nil, err
	}

	return b.WriteString(data.String(args.Get(0)))
}

// Implement the Builder funcion Write() which writes a byte array to the builder's
// internal buffer. The byte array can be a string, or an array of bytes.
func writeBytes(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	b, err := getBuilder(s)
	if err != nil {
		return nil, err
	}

	v := args.Get(0)
	if a, ok := v.(*data.Array); ok {
		bytes := a.GetBytes()

		return b.Write(bytes)
	}

	return nil, errors.ErrInvalidType.In("Write")
}

// Implement the Builder funcion WriteRune() which writes a rune to the builder's
// internal buffer.
func writeRune(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	b, err := getBuilder(s)
	if err != nil {
		return nil, err
	}

	r := data.Rune(args.Get(0))

	return b.WriteRune(r)
}

// Implement the Builder funcion WriteByte() which writes a byte to the builder's
// internal buffer.
func writeByte(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	b, err := getBuilder(s)
	if err != nil {
		return nil, err
	}

	r := data.Byte(args.Get(0))

	b.WriteByte(r)

	return nil, nil
}

// Implement the Builder funcion String() which returns the contents of the builder's
// internal buffer as a string.
func builderToString(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	b, err := getBuilder(s)
	if err != nil {
		return nil, err
	}

	return b.String(), nil
}

// getThis returns a map for the "this" object in the current
// symbol table.
func getThis(s *symbols.SymbolTable) *data.Struct {
	t, ok := s.Get(defs.ThisVariable)
	if !ok {
		return nil
	}

	this, ok := t.(*data.Struct)
	if !ok {
		return nil
	}

	return this
}

// Helper function that gets the file handle for a all to a
// handle-based function.
func getBuilder(s *symbols.SymbolTable) (*nativestrings.Builder, error) {
	this := getThis(s)
	if v, ok := this.Get("builder"); ok {
		if b, ok := v.(*nativestrings.Builder); ok {
			return b, nil
		}

		b := &nativestrings.Builder{}

		return b, this.Set("builder", b)
	}

	return nil, errors.ErrInvalidThis
}
