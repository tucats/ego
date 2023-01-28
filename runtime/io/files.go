package io

import (
	"bufio"
	"bytes"
	"encoding/gob"
	"fmt"
	"os"
	"strings"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

func asString(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	var b strings.Builder

	f := getThis(s)
	if f == nil {
		return nil, errors.ErrNoFunctionReceiver.In("String()")
	}

	b.WriteString("<file")

	if bx, ok := f.Get(validFieldName); ok {
		if data.Bool(bx) {
			b.WriteString("; open")
			b.WriteString("; name \"")

			if name, ok := f.Get(nameFieldName); ok {
				b.WriteString(data.String(name))
			}

			b.WriteString("\"")

			if f, ok := f.Get(fileFieldName); ok {
				b.WriteString(fmt.Sprintf("; fileptr %v", f))
			}

			b.WriteString(">")

			return b.String(), nil
		}
	}

	b.WriteString("; closed>")

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
func getFile(fn string, s *symbols.SymbolTable) (*os.File, error) {
	this := getThis(s)
	if v, ok := this.Get(validFieldName); ok && data.Bool(v) {
		fh, ok := this.Get(fileFieldName)
		if ok {
			f, ok := fh.(*os.File)
			if ok {
				return f, nil
			}
		}
	}

	return nil, errors.ErrInvalidfileIdentifier.In(fn)
}

// readString reads the next line from the file as a string.
func readString(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	f, err := getFile("ReadString", s)
	if err != nil {
		return data.List(nil, err), err
	}

	var scanner *bufio.Scanner

	this := getThis(s)

	scanX, found := this.Get(scannerFieldName)
	if !found {
		scanner = bufio.NewScanner(f)
		this.SetAlways(scannerFieldName, scanner)
	} else {
		scanner = scanX.(*bufio.Scanner)
	}

	scanner.Scan()

	return data.List(scanner.Text(), err), err
}

// writeString writes a string value to a file.
func writeString(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	var e2 error

	length := 0

	f, err := getFile("WriteString", s)
	if err == nil {
		length, e2 = f.WriteString(data.String(args[0]) + "\n")
		if e2 != nil {
			err = errors.NewError(e2)
		}
	}

	return data.List(length, err), err
}

// write writes an arbitrary binary object to a file.
func write(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	var buf bytes.Buffer

	enc := gob.NewEncoder(&buf)

	err := enc.Encode(args[0])
	if err != nil {
		return data.List(nil, err), errors.NewError(err)
	}

	bytes := buf.Bytes()
	length := len(bytes)

	f, err := getFile("Write", s)
	if err == nil {
		length, err = f.Write(bytes)
	}

	if err != nil {
		err = errors.NewError(err)
	}

	return data.List(length, err), err
}

// Write writes an arbitrary binary object to a file at an offset.
func writeAt(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	var buf bytes.Buffer

	offset := data.Int(args[1])
	enc := gob.NewEncoder(&buf)

	err := enc.Encode(args[0])
	if err != nil {
		return nil, errors.NewError(err)
	}

	bytes := buf.Bytes()
	length := len(bytes)

	f, err := getFile("WriteAt", s)
	if err == nil {
		length, err = f.WriteAt(bytes, int64(offset))
	}

	if err != nil {
		err = errors.NewError(err)
	}

	return data.List(length, err), err
}
