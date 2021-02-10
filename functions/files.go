package functions

import (
	"bufio"
	"bytes"
	"encoding/gob"
	"os"
	"path/filepath"
	"strings"

	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

const fileMemberName = "file"

// OpenFile opens a file.
func OpenFile(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	var mask os.FileMode = 0644

	var f *os.File

	mode := os.O_RDONLY

	fname, err := filepath.Abs(util.GetString(args[0]))
	if err != nil {
		return nil, errors.New(err)
	}

	if len(args) > 1 {
		modeValue := strings.ToLower(util.GetString(args[1]))

		// If we are opening for output mode, delete the file if it already
		// exists
		if util.InList(modeValue, "true", "create", "output") {
			_ = os.Remove(fname)
			mode = os.O_CREATE | os.O_WRONLY
		}

		// For append, adjust the mode bits
		if modeValue == "append" {
			mode = os.O_APPEND | os.O_WRONLY
		}
	}

	if len(args) > 2 {
		mask = os.FileMode(util.GetInt(args[2]))
	}

	f, err = os.OpenFile(fname, mode, mask)
	if err != nil {
		return nil, errors.New(err)
	}

	fobj := map[string]interface{}{
		"Close":        Close,
		"ReadString":   ReadString,
		"WriteString":  WriteString,
		"Write":        Write,
		"WriteAt":      WriteAt,
		fileMemberName: f,
		"valid":        true,
		"name":         fname,
	}

	datatypes.SetMetadata(fobj, datatypes.ReadonlyMDKey, true)
	datatypes.SetMetadata(fobj, datatypes.TypeMDKey, "file")

	return fobj, nil
}

// getThis returns a map for the "this" object in the current
// symbol table.
func getThis(s *symbols.SymbolTable) map[string]interface{} {
	t, ok := s.Get("__this")
	if !ok {
		return nil
	}

	this, ok := t.(map[string]interface{})
	if !ok {
		return nil
	}

	return this
}

// Helper function that gets the file handle for a all to a
// handle-based function.
func getFile(fn string, s *symbols.SymbolTable) (*os.File, *errors.EgoError) {
	this := getThis(s)
	if v, ok := this["valid"]; ok && util.GetBool(v) {
		fh, ok := this[fileMemberName]
		if ok {
			f, ok := fh.(*os.File)
			if ok {
				return f, nil
			}
		}
	}

	return nil, errors.New(errors.InvalidfileIdentifierError).In(fn)
}

// Close closes a file.
func Close(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if len(args) > 0 {
		return nil, errors.New(errors.ArgumentCountError).In("Close()")
	}

	f, err := getFile("Close", s)
	if err == nil {
		this := getThis(s)
		this["valid"] = false

		e2 := f.Close()
		if e2 == nil {
			delete(this, "Close")
			delete(this, "ReadString")
			delete(this, "Write")
			delete(this, "WriteAt")
			delete(this, "WriteString")
			delete(this, fileMemberName)
			delete(this, "name")
			delete(this, "file")
		} else {
			err = errors.New(e2)
		}
	}

	return err, nil
}

// ReadString reads the next line from the file as a string.
func ReadString(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if len(args) > 0 {
		return nil, errors.New(errors.ArgumentCountError).In("ReadString()")
	}

	f, err := getFile("ReadString", s)
	if err != nil {
		return MultiValueReturn{Value: []interface{}{nil, err}}, err
	}

	var scanner *bufio.Scanner

	this := getThis(s)

	scanX, found := this["scanner"]
	if !found {
		scanner = bufio.NewScanner(f)
		this["scanner"] = scanner
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
		return nil, errors.New(errors.ArgumentCountError).In("WriteString()")
	}

	length := 0

	f, err := getFile("WriteString", s)
	if err == nil {
		length, e2 = f.WriteString(util.GetString(args[0]) + "\n")
		if e2 != nil {
			err = errors.New(e2)
		}
	}

	return MultiValueReturn{Value: []interface{}{length, err}}, err
}

// Write writes an arbitrary binary object to a file.
func Write(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if len(args) != 1 {
		return nil, errors.New(errors.ArgumentCountError).In("Write()")
	}

	var buf bytes.Buffer

	enc := gob.NewEncoder(&buf)

	err := enc.Encode(args[0])
	if err != nil {
		return nil, errors.New(err)
	}

	bytes := buf.Bytes()
	length := len(bytes)

	f, err := getFile("Write", s)
	if err == nil {
		length, err = f.Write(bytes)
	}

	return MultiValueReturn{Value: []interface{}{length, err}}, errors.New(err)
}

// Write writes an arbitrary binary object to a file at an offset.
func WriteAt(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	var buf bytes.Buffer

	if len(args) != 2 {
		return nil, errors.New(errors.ArgumentCountError).In("WriteAt()")
	}

	offset := util.GetInt(args[1])
	enc := gob.NewEncoder(&buf)

	err := enc.Encode(args[0])
	if err != nil {
		return nil, errors.New(err)
	}

	bytes := buf.Bytes()
	length := len(bytes)

	f, err := getFile("WriteAt", s)
	if err == nil {
		length, err = f.WriteAt(bytes, int64(offset))
	}

	return MultiValueReturn{Value: []interface{}{length, err}}, errors.New(err)
}
