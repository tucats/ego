package strings

import (
	"fmt"
	"strings"

	"github.com/tucats/ego/data"
)

// Initialize the strings package Builder type. This is a wrapper around the
// native strings.Builder type.
func initializeBuilder() *data.Type {
	// Generate the Builder type, which is just a struct wrapper around a single
	// field that will contain the native builder object.
	t := data.TypeDefinition("Builder",
		data.StructureType()).
		SetNativeName("strings.Builder").
		SetFormatFunc(func(v interface{}) string {
			b := v.(*strings.Builder)

			return fmt.Sprintf(`strings.Builder{String: "%s", Len: %d, Cap: %d}`, b.String(), b.Len(), b.Cap())
		}).
		SetPackage("strings").
		SetNew(func() interface{} {
			return &strings.Builder{}
		})

	// Define the native functions for the strings.Builder type
	t.DefineNativeFunction("Cap", &data.Declaration{
		Name: "Cap",
		Type: t,
		Parameters: []data.Parameter{
			{
				Name: "s",
				Type: data.StringType,
			},
		},
		Returns: []*data.Type{data.IntType},
	}, nil)

	t.DefineNativeFunction("Grow", &data.Declaration{
		Name: "Grow",
		Type: t,
		Parameters: []data.Parameter{
			{
				Name: "n",
				Type: data.IntType,
			},
		},
		Returns: []*data.Type{data.IntType},
	}, nil)

	t.DefineNativeFunction("Len", &data.Declaration{
		Name:    "Len",
		Type:    t,
		Returns: []*data.Type{data.IntType},
	}, nil)

	t.DefineNativeFunction("Reset", &data.Declaration{
		Name: "Reset",
		Type: t,
	}, nil)

	t.DefineNativeFunction("String", &data.Declaration{
		Name:    "String",
		Type:    t,
		Returns: []*data.Type{data.StringType},
	}, nil)

	t.DefineNativeFunction("Write", &data.Declaration{
		Name: "Write",
		Type: t,
		Parameters: []data.Parameter{
			{
				Name: "data",
				Type: data.ArrayType(data.ByteType),
			},
		},
		Returns: []*data.Type{data.IntType, data.ErrorType},
	}, nil)

	t.DefineNativeFunction("WriteString", &data.Declaration{
		Name: "WriteString",
		Type: t,
		Parameters: []data.Parameter{
			{
				Name: "text",
				Type: data.StringType,
			},
		},
		Returns: []*data.Type{data.IntType, data.ErrorType},
	}, nil)

	t.DefineNativeFunction("WriteByte", &data.Declaration{
		Name: "WriteByte",
		Type: t,
		Parameters: []data.Parameter{
			{
				Name: "c",
				Type: data.ByteType,
			},
		},
		Returns: []*data.Type{data.IntType, data.ErrorType},
	}, nil)

	t.DefineNativeFunction("WriteRune", &data.Declaration{
		Name: "WriteRune",
		Type: t,
		Parameters: []data.Parameter{
			{
				Name: "r",
				Type: data.Int32Type,
			},
		},
		Returns: []*data.Type{data.IntType, data.ErrorType},
	}, nil)

	return t
}
