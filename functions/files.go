package functions

import (
	"bufio"
	"bytes"
	"encoding/gob"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

const (
	fileFieldName    = "File"
	nameFieldName    = "Name"
	validFieldName   = "Valid"
	scannerFieldName = "Scanner"
	modeFieldName    = "Mode"
)

var fileType *data.Type

func initializeFileType() {
	if fileType == nil {
		structType := data.StructureType()
		structType.DefineField(fileFieldName, &data.InterfaceType).
			DefineField(validFieldName, data.BoolType).
			DefineField(nameFieldName, &data.StringType).
			DefineField(modeFieldName, &data.StringType)

		t := data.TypeDefinition("io.File", structType)

		t.DefineFunction("Close", nil, Close)
		t.DefineFunction("ReadString", nil, ReadString)
		t.DefineFunction("WriteString", nil, WriteString)
		t.DefineFunction("Write", nil, Write)
		t.DefineFunction("WriteAt", nil, WriteAt)
		t.DefineFunction("String", nil, AsString)

		fileType = t
	}
}

// OpenFile opens a file.
func OpenFile(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	var mask os.FileMode = 0644

	var f *os.File

	mode := os.O_RDONLY

	fname, err := filepath.Abs(sandboxName(data.String(args[0])))
	if err != nil {
		return nil, errors.NewError(err)
	}

	modeValue := "input"

	if len(args) > 1 {
		modeValue = strings.ToLower(data.String(args[1]))

		// Is it a valid mode name?
		if !util.InList(modeValue, "input", "read", "output", "write", "create", "append") {
			return nil, errors.ErrInvalidFileMode.Context(modeValue)
		}
		// If we are opening for output mode, delete the file if it already
		// exists
		if util.InList(modeValue, "create", "write", "output") {
			_ = os.Remove(fname)
			mode = os.O_CREATE | os.O_WRONLY
			modeValue = "output"
		} else if modeValue == "append" {
			// For append, adjust the mode bits
			mode = os.O_APPEND | os.O_WRONLY
		} else {
			modeValue = "input"
		}
	}

	if len(args) > 2 {
		mask = os.FileMode(data.Int(args[2]) & math.MaxInt8)
	}

	f, err = os.OpenFile(fname, mode, mask)
	if err != nil {
		return nil, errors.NewError(err)
	}

	initializeFileType()

	fobj := data.NewStruct(fileType)
	fobj.SetReadonly(true)
	fobj.SetAlways(fileFieldName, f)
	fobj.SetAlways(validFieldName, true)
	fobj.SetAlways(nameFieldName, fname)
	fobj.SetAlways(modeFieldName, modeValue)

	return fobj, nil
}

func AsString(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
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
	t, ok := s.Get("__this")
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

// Close closes a file.
func Close(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) > 0 {
		return nil, errors.ErrArgumentCount.In("Close()")
	}

	f, err := getFile("Close", s)
	if err == nil {
		e2 := f.Close()
		if e2 != nil {
			err = errors.NewError(e2)
		}

		this := getThis(s)

		this.SetAlways(validFieldName, false)
		this.SetAlways(modeFieldName, "closed")
		this.SetAlways(fileFieldName, nil)
		this.SetAlways(nameFieldName, "")
	}

	return err, nil
}

// ReadString reads the next line from the file as a string.
func ReadString(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) > 0 {
		return nil, errors.ErrArgumentCount.In("ReadString()")
	}

	f, err := getFile("ReadString", s)
	if err != nil {
		return MultiValueReturn{Value: []interface{}{nil, err}}, err
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

	return MultiValueReturn{Value: []interface{}{scanner.Text(), err}}, err
}

// WriteString writes a string value to a file.
func WriteString(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	var e2 error

	if len(args) != 1 {
		return nil, errors.ErrArgumentCount.In("WriteString()")
	}

	length := 0

	f, err := getFile("WriteString", s)
	if err == nil {
		length, e2 = f.WriteString(data.String(args[0]) + "\n")
		if e2 != nil {
			err = errors.NewError(e2)
		}
	}

	return MultiValueReturn{Value: []interface{}{length, err}}, err
}

// Write writes an arbitrary binary object to a file.
func Write(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) != 1 {
		return nil, errors.ErrArgumentCount.In("Write()")
	}

	var buf bytes.Buffer

	enc := gob.NewEncoder(&buf)

	err := enc.Encode(args[0])
	if err != nil {
		return nil, errors.NewError(err)
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

	return MultiValueReturn{Value: []interface{}{length, err}}, err
}

// Write writes an arbitrary binary object to a file at an offset.
func WriteAt(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	var buf bytes.Buffer

	if len(args) != 2 {
		return nil, errors.ErrArgumentCount.In("WriteAt()")
	}

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

	return MultiValueReturn{Value: []interface{}{length, err}}, err
}
