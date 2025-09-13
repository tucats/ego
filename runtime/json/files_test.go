package json

import (
	"fmt"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/symbols"
)

func TestWriteFile_ValidInput(t *testing.T) {
	fileName := fmt.Sprintf("test-%s.json", uuid.New().String())
	defer func() {
		if err := os.Remove(fileName); err != nil {
			t.Errorf("Error removing file: %v", err)
		}
	}()

	item := data.NewMapFromMap(map[string]any{
		"key": "value",
	})

	symbols := &symbols.SymbolTable{}
	args := data.NewList(fileName, item)

	result, err := writeFile(symbols, args)

	assert.Nil(t, err, "Expected no error, but got: %v", err)
	assert.IsType(t, data.NewList(nil), result, "Expected result to be a data.List")

	args = data.NewList(fileName)
	result, err = readFile(symbols, args)

	assert.Nil(t, err, "Expected no error, but got: %v", err)
	assert.IsType(t, data.NewList(nil), result, "Expected result to be a data.List")

	resultList, _ := result.(data.List)

	if m, ok := resultList.Get(0).(*data.Map); ok {
		v, found, _ := m.Get("key")

		assert.Equal(t, true, found)
		assert.Equal(t, "value", data.String(v))
	} else {
		assert.Fail(t, "Expected result to be a data.Map")
	}
}

func TestWriteFile_NullInput(t *testing.T) {
	fileName := fmt.Sprintf("test-%s.json", uuid.New().String())
	defer func() {
		if err := os.Remove(fileName); err != nil {
			t.Errorf("Error removing file: %v", err)
		}
	}()

	var item any = nil

	symbols := &symbols.SymbolTable{}
	args := data.NewList(fileName, item)

	result, err := writeFile(symbols, args)

	assert.Nil(t, err, "Expected no error, but got: %v", err)
	assert.IsType(t, data.NewList(nil), result, "Expected result to be a data.List")

	args = data.NewList(fileName)
	result, err = readFile(symbols, args)

	assert.Nil(t, err, "Expected no error, but got: %v", err)
	assert.IsType(t, data.NewList(nil), result, "Expected result to be a data.List")

	if resultList, ok := result.(data.List); ok {
		v := resultList.Get(0)

		assert.Nil(t, v)
	} else {
		assert.Fail(t, "Expected result to be a data.List")
	}
}
