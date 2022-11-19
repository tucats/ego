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

	"github.com/tucats/ego/datatypes"
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

var fileType *datatypes.Type

func initializeFileType() {
	if fileType == nil {
		structType := datatypes.Structure()
		structType.DefineField(fileFieldName, &datatypes.InterfaceType).
			DefineField(validFieldName, datatypes.BoolType).
			DefineField(nameFieldName, &datatypes.StringType).
			DefineField(modeFieldName, &datatypes.StringType)

		t := datatypes.TypeDefinition("io.File", structType)

		t.DefineFunction("Close", Close)
		t.DefineFunction("ReadString", ReadString)
		t.DefineFunction("WriteString", WriteString)
		t.DefineFunction("Write", Write)
		t.DefineFunction("WriteAt", WriteAt)
		t.DefineFunction("String", AsString)

		fileType = t
	}
}

// OpenFile opens a file.
func OpenFile(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	var mask os.FileMode = 0644

	var f *os.File

	mode := os.O_RDONLY

	fname, err := filepath.Abs(sandboxName(datatypes.GetString(args[0])))
	if !errors.Nil(err) {
		return nil, errors.New(err)
	}

	modeValue := "input"

	if len(args) > 1 {
		modeValue = strings.ToLower(datatypes.GetString(args[1]))

		// Is it a valid mode name?
		if !util.InList(modeValue, "input", "read", "output", "write", "create", "append") {
			return nil, errors.New(errors.ErrInvalidFileMode).Context(modeValue)
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
		mask = os.FileMode(datatypes.GetInt(args[2]) & math.MaxInt8)
	}

	f, err = os.OpenFile(fname, mode, mask)
	if !errors.Nil(err) {
		return nil, errors.New(err)
	}

	initializeFileType()

	fobj := datatypes.NewStruct(fileType)
	fobj.SetReadonly(true)
	fobj.SetAlways(fileFieldName, f)
	fobj.SetAlways(validFieldName, true)
	fobj.SetAlways(nameFieldName, fname)
	fobj.SetAlways(modeFieldName, modeValue)

	return fobj, nil
}

func AsString(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	var b strings.Builder

	f := getThis(s)
	if f == nil {
		return nil, errors.New(errors.ErrNoFunctionReceiver).In("String()")
	}

	b.WriteString("<file")

	if bx, ok := f.Get(validFieldName); ok {
		if datatypes.GetBool(bx) {
			b.WriteString("; open")
			b.WriteString("; name \"")

			if name, ok := f.Get(nameFieldName); ok {
				b.WriteString(datatypes.GetString(name))
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
func getThis(s *symbols.SymbolTable) *datatypes.EgoStruct {
	t, ok := s.Get("__this")
	if !ok {
		return nil
	}

	this, ok := t.(*datatypes.EgoStruct)
	if !ok {
		return nil
	}

	return this
}

// Helper function that gets the file handle for a all to a
// handle-based function.
func getFile(fn string, s *symbols.SymbolTable) (*os.File, *errors.EgoError) {
	this := getThis(s)
	if v, ok := this.Get(validFieldName); ok && datatypes.GetBool(v) {
		fh, ok := this.Get(fileFieldName)
		if ok {
			f, ok := fh.(*os.File)
			if ok {
				return f, nil
			}
		}
	}

	return nil, errors.New(errors.ErrInvalidfileIdentifier).In(fn)
}

// Close closes a file.
func Close(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if len(args) > 0 {
		return nil, errors.New(errors.ErrArgumentCount).In("Close()")
	}

	f, err := getFile("Close", s)
	if errors.Nil(err) {
		e2 := f.Close()
		if e2 != nil {
			err = errors.New(e2)
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
func ReadString(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if len(args) > 0 {
		return nil, errors.New(errors.ErrArgumentCount).In("ReadString()")
	}

	f, err := getFile("ReadString", s)
	if !errors.Nil(err) {
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
func WriteString(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	var e2 error

	if len(args) != 1 {
		return nil, errors.New(errors.ErrArgumentCount).In("WriteString()")
	}

	length := 0

	f, err := getFile("WriteString", s)
	if errors.Nil(err) {
		length, e2 = f.WriteString(datatypes.GetString(args[0]) + "\n")
		if e2 != nil {
			err = errors.New(e2)
		}
	}

	return MultiValueReturn{Value: []interface{}{length, err}}, err
}

// Write writes an arbitrary binary object to a file.
func Write(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if len(args) != 1 {
		return nil, errors.New(errors.ErrArgumentCount).In("Write()")
	}

	var buf bytes.Buffer

	enc := gob.NewEncoder(&buf)

	err := enc.Encode(args[0])
	if !errors.Nil(err) {
		return nil, errors.New(err)
	}

	bytes := buf.Bytes()
	length := len(bytes)

	f, err := getFile("Write", s)
	if errors.Nil(err) {
		length, err = f.Write(bytes)
	}

	return MultiValueReturn{Value: []interface{}{length, err}}, errors.New(err)
}

// Write writes an arbitrary binary object to a file at an offset.
func WriteAt(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	var buf bytes.Buffer

	if len(args) != 2 {
		return nil, errors.New(errors.ErrArgumentCount).In("WriteAt()")
	}

	offset := datatypes.GetInt(args[1])
	enc := gob.NewEncoder(&buf)

	err := enc.Encode(args[0])
	if !errors.Nil(err) {
		return nil, errors.New(err)
	}

	bytes := buf.Bytes()
	length := len(bytes)

	f, err := getFile("WriteAt", s)
	if errors.Nil(err) {
		length, err = f.WriteAt(bytes, int64(offset))
	}

	return MultiValueReturn{Value: []interface{}{length, err}}, errors.New(err)
}
