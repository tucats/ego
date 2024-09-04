package cipher

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

func TestSealString_ValidString(t *testing.T) {
	var (
		str    interface{} = "Hello, World!"
		strPtr interface{} = &str
	)

	s := symbols.NewRootSymbolTable("test")
	args := data.NewList(strPtr)

	result, err := sealString(s, args)
	require.Nil(t, err)
	require.NotEmpty(t, result)
	require.NotEqual(t, "Hello, World!", result)
	require.Equal(t, data.String(str), "")

	// Now let's unseal it.
	args = data.NewList(result)

	result, err = unsealString(s, args)
	require.Nil(t, err)
	require.Equal(t, data.String(result), "Hello, World!")
}

func TestSealString_EmptyString(t *testing.T) {
	var (
		str    interface{} = ""
		strPtr interface{} = &str
	)
	
	s := symbols.NewRootSymbolTable("test")
	args := data.NewList(strPtr)

	result, err := sealString(s, args)
	require.Nil(t, err)
	require.NotEmpty(t, result)
	require.NotEqual(t, "", result)
	require.Equal(t, data.String(str), "")

	// Now let's unseal it.
	args = data.NewList(result)

	result, err = unsealString(s, args)
	require.Nil(t, err)
	require.Equal(t, data.String(result), "")
}

func TestSealString_NilArgument(t *testing.T) {
	s := symbols.NewRootSymbolTable("test")
	args := data.NewList(nil)

	result, err := sealString(s, args)
	require.NotNil(t, err)
	require.True(t, errors.Equals(err, errors.ErrInvalidPointerType))
	require.Empty(t, result)
}

func TestSealString_NonStringArgument(t *testing.T) {
	s := symbols.NewRootSymbolTable("test")
	args := data.NewList(data.Int(123))

	result, err := sealString(s, args)
	require.NotNil(t, err)
	require.True(t, errors.Equals(err, errors.ErrInvalidPointerType))
	require.Empty(t, result)
}

func TestSealString_MultipleArguments(t *testing.T) {
	s := symbols.NewRootSymbolTable("test")
	args := data.NewList(data.String("Hello, World!"), data.String("Extra argument"))

	result, err := sealString(s, args)
	require.NotNil(t, err)
	require.True(t, errors.Equals(err, errors.ErrInvalidPointerType))
	require.Empty(t, result)
}
