package bytecode

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/tokenizer"
)

type cacheItemType int

const (
	cachedType cacheItemType = iota
	cachedByteCode
)

type cachedItem struct {
	id   int64
	kind cacheItemType
	data interface{}
}

var (
	lock   sync.Mutex
	nextID atomic.Int64
	cache  map[interface{}]cachedItem
)

// Implement the Serialize bytecode. If the instruction argument is
// non-null, it must be a bytecoee pointer. If null, then the bytecode
// pointer is popped from the stack.
func serializeByteCode(c *Context, i interface{}) error {
	var err error

	if i == nil {
		i, err = c.Pop()
		if err != nil {
			return err
		}
	}

	// This operation must be serialized so the pointer caches are
	// limited to only the pointers for this specific bytecode instance.
	lock.Lock()
	defer lock.Unlock()

	// Initialize the id and cache. Each serialized bytecode instance is complete
	// and you cannot share the cached values across serializations.
	nextID.Store(1)

	cache = map[interface{}]cachedItem{}

	// If it's a bytecode, use the bytecode serizlizer.
	if bc, ok := i.(*ByteCode); ok {
		text, err := bc.Serialize()
		if err != nil {
			return err
		}

		_ = c.push(text)
	} else {
		// Otherwise, it's a value so serialize that.
		text, err := serializeValue(i)
		if err != nil {
			return err
		}

		r := strings.Builder{}
		r.WriteString(`{"type":"@value",`)

		if len(cache) > 0 {
			r.WriteString(fmt.Sprintf(`"pointers":%s`, serializeCache()))
		}

		r.WriteString(fmt.Sprintf(`"value":%s}`, text))

		return c.push(r.String())
	}

	return nil
}

// For a given bytecode object, produce the serialized JSON representation of the bytecode.
func (b ByteCode) Serialize() (string, error) {
	var (
		err  error
		buff = strings.Builder{}
	)

	buff.WriteString("{\n")
	buff.WriteString(`"type": "@code",`)
	buff.WriteString("\n")
	buff.WriteString(fmt.Sprintf(`"name": "%s",`, b.Name()))
	buff.WriteString("\n")

	if d := b.Declaration(); d != nil {
		buff.WriteString(fmt.Sprintf(`"declaration": "%s",`, b.Declaration().String()))
		buff.WriteString("\n")
	}

	// Generate the code JSON. This also populates the cache, after which we ca generate
	// the cached pointer information before outputting the actual code.
	code, err := serializeCode(b.instructions, b.nextAddress)
	if err != nil {
		return "", err
	}

	buff.WriteString(fmt.Sprintf(`"pointers": %s`, serializeCache()))
	buff.WriteString(fmt.Sprintf(`"code": %s`, code))
	buff.WriteString("\n")
	buff.WriteString("}\n")

	return buff.String(), err
}

func serializeCode(instructions []instruction, length int) (string, error) {
	buff := strings.Builder{}

	buff.WriteString("[\n")

	for i, inst := range instructions {
		if i >= length {
			break
		}

		if i > 0 {
			buff.WriteString(",\n")
		}

		argPart := ""

		if inst.Operand != nil {
			argText, err := serializeValue(inst.Operand)
			if err != nil {
				return "", err
			}

			argPart = fmt.Sprintf(`, "i":%s`, argText)
		}

		buff.WriteString(fmt.Sprintf(`{"o":"%s"%s}`, opcodeNames[inst.Operation], argPart))
	}

	buff.WriteString("]")

	return buff.String(), nil
}

// Serialize an arbitrary value into a JSON string. Items that are really
// pointers are represented as "@p" and cached, with the cache id used as
// the value.
func serializeValue(arg interface{}) (string, error) {
	switch arg := arg.(type) {
	case nil:
		return `{"type":"@null"}`, nil

	case *data.Array:
		r := strings.Builder{}
		r.WriteString(fmt.Sprintf(`{"type":"@array", "of":"%s", "values":[`, arg.Type()))

		for i := 0; i < arg.Len(); i++ {
			vv, _ := arg.Get(i)

			vvText, err := serializeValue(vv)
			if err != nil {
				return "", err
			}

			r.WriteString(vvText)

			if i < arg.Len()-1 {
				r.WriteString(", ")
			}
		}

		r.WriteString("]}")

		return r.String(), nil

	case *data.Map:
		r := strings.Builder{}
		r.WriteString(fmt.Sprintf(`{"type":"@map", "t":"%s", "key":"%s", "of":"%s", "values":[`, arg.Type().String(), arg.Type().KeyType(), arg.Type().BaseType()))

		for index, i := range arg.Keys() {
			vv, found, err := arg.Get(i)
			if err != nil {
				return "", err
			}

			if !found {
				return "", errors.ErrNotFound.Context(fmt.Sprintf("%v", i))
			}

			if index > 0 {
				r.WriteString(",\n")
			} else {
				r.WriteString("\n")
			}

			kkText, err := serializeValue(i)
			if err != nil {
				return "", err
			}

			vvText, err := serializeValue(vv)
			if err != nil {
				return "", err
			}

			r.WriteString(fmt.Sprintf(`{"k":%s, "v":%s}`, kkText, vvText))
		}

		r.WriteString("]}")

		return r.String(), nil

	case *data.Struct:
		r := strings.Builder{}
		r.WriteString(fmt.Sprintf(`{"type":"@struct", "t":"%s", "fields":[`, arg.Type().String()))

		for index, i := range arg.FieldNames(true) {
			vv := arg.GetAlways(i)

			if index > 0 {
				r.WriteString(",\n")
			} else {
				r.WriteString("\n")
			}

			vvText, err := serializeValue(vv)
			if err != nil {
				return "", err
			}

			r.WriteString(fmt.Sprintf(`{"%s" : %s}`, i, vvText))
		}

		r.WriteString("]}")

		return r.String(), nil

	case *symbols.SymbolTable:
		return fmt.Sprintf(`{"type":"@symtable", "v":%s)`, arg.Name), nil

	case data.Function:
		return fmt.Sprintf(`{"type":"@func", "name":"%s", "t":"%s"}`, arg.Declaration.Name, arg.Declaration.String()), nil

	case *data.Package:
		r := strings.Builder{}
		r.WriteString(fmt.Sprintf(`{"type":"@pkg", "name":"%s", "id":"%s", "items":[`, arg.Name, arg.ID))

		for index, i := range arg.Keys() {
			vv, found := arg.Get(i)
			if !found {
				return "", errors.ErrNotFound.Context(fmt.Sprintf("%v", i))
			}

			// Is this the symbol table? If so, skip it.
			if _, ok := vv.(*symbols.SymbolTable); ok && i == "__Symbols" {
				continue
			}

			// Format the key/value pair.
			if index > 0 {
				r.WriteString(",\n")
			} else {
				r.WriteString("\n")
			}

			kkText, err := serializeValue(i)
			if err != nil {
				return "", err
			}

			vvText, err := serializeValue(vv)
			if err != nil {
				return "", err
			}

			r.WriteString(fmt.Sprintf(`{"k":%s, "v":%s}`, kkText, vvText))
		}

		r.WriteString("]}")

		return r.String(), nil

	case *data.Type:
		// Is it in the cache already?
		if item, ok := cache[arg]; ok {
			return fmt.Sprintf(`{"t":"@p", "v":%d}`, item.id), nil
		}

		// Cache the type as a declaration string.
		id := nextID.Add(1)

		if cache == nil {
			cache = map[interface{}]cachedItem{}
		}

		cache[arg] = cachedItem{id: id, kind: cachedType, data: arg.String()}

		return fmt.Sprintf(`{"t":"@ptr", "v":"%d"}`, id), nil

	case data.Immutable:
		vv, err := serializeValue(arg.Value)
		if err != nil {
			return "", err
		}

		return fmt.Sprintf(`{"t":"@immutable", "v":%s}`, vv), nil

	case bool:
		return fmt.Sprintf(`{"t":"@b", "v":%t}`, arg), nil

	case byte:
		return fmt.Sprintf(`{"t":"@i8", "v":"%d"}`, arg), nil

	case float32:
		return fmt.Sprintf(`{"t":"@f32", "v":"%f"}`, arg), nil

	case float64:
		return fmt.Sprintf(`{"t":"@f64", "v":"%f"}`, arg), nil

	case int:
		return fmt.Sprintf(`{"t":"@i", "v":"%d"}`, arg), nil

	case int32:
		return fmt.Sprintf(`{"t":"@i32", "v":"%d"}`, arg), nil

	case int64:
		return fmt.Sprintf(`{"t":"@i64", "v":"%d"}`, arg), nil

	case string:
		return fmt.Sprintf(`{"t":"@s", "v": "%s"}`, arg), nil

	case []byte:
		return fmt.Sprintf(`{"t":"@ai8", "v":"%s"}`, string(arg)), nil

	case []interface{}:
		list := data.NewList(arg...)

		listText, err := serializeList(list)
		if err != nil {
			return "", err
		}

		return fmt.Sprintf(`{"t":"@ax", "v":%s}`, listText), nil

	case data.List:
		listText, err := serializeList(arg)
		if err != nil {
			return "", err
		}

		return fmt.Sprintf(`{"t":"@l", "v":%s}`, listText), nil

	case *ByteCode:
		// Is it in our pointer cache already?
		if item, found := cache[arg]; found {
			return fmt.Sprintf(`{"t":"@ptr", "v":%d}`, item.id), nil
		}

		buff := strings.Builder{}
		buff.WriteString("{\n")

		buff.WriteString(fmt.Sprintf(`"name": "%s",`, arg.Name()))
		buff.WriteString("\n")

		if d := arg.Declaration(); d != nil {
			buff.WriteString(fmt.Sprintf(`"declaration": "%s",`, arg.Declaration().String()))
			buff.WriteString("\n")
		}

		codeText, err := serializeCode(arg.instructions, arg.nextAddress)
		if err != nil {
			return "", err
		}

		buff.WriteString(fmt.Sprintf(`"code": %s}`, codeText))

		// Store value in the cache
		id := nextID.Add(1)
		cache[arg] = cachedItem{id: id, kind: cachedByteCode, data: buff.String()}

		return fmt.Sprintf(`{"t":"@bc", "v":%d}`, id), nil

	case StackMarker:
		name := arg.label
		value := ""

		if len(arg.values) > 0 {
			argText, err := serializeValue(arg.values)
			if err != nil {
				return "", err
			}

			value = fmt.Sprintf(`, "v":%s`, argText)
		}

		return fmt.Sprintf(`{"t":"@sm %s"%s}`, name, value), nil

	case tokenizer.Token:
		return fmt.Sprintf(`{"t":"@tk", "v":{"spell":"%s", "class": %d}}`,
			arg.Spelling(), arg.Class()), nil

	case *tokenizer.Tokenizer:
		return `{"t":"@tokenizer"}`, nil

	default:
		return "", errors.ErrInvalidType.Context(fmt.Sprintf("%T", arg))
	}
}

// Generate the JSON for a list. This is most commonly used as an instruction
// argument that contains more than one value.
func serializeList(l data.List) (string, error) {
	buff := strings.Builder{}

	buff.WriteString("[")

	for i, v := range l.Elements() {
		if i > 0 {
			buff.WriteString(", ")
		}

		argText, err := serializeValue(v)
		if err != nil {
			return "", err
		}

		buff.WriteString(argText)
	}

	buff.WriteString("]")

	return buff.String(), nil
}

// Generage the JSON for the pointer cache for this serialization
// operation.
func serializeCache() string {
	if len(cache) == 0 {
		return "[]\n"
	}

	buff := strings.Builder{}

	buff.WriteString("[")

	count := 0

	for _, item := range cache {
		if count > 0 {
			buff.WriteString(",\n")
		}

		count++

		if item.kind == cachedType {
			buff.WriteString(fmt.Sprintf(`{"t":"@t", "v":%d, "d": "%s"}`, item.id, item.data.(string)))
		} else if item.kind == cachedByteCode {
			code := item.data.(string)

			buff.WriteString(fmt.Sprintf(`{"t":"@bc", "v":%d, "d": %s}`, item.id, code))
		}
	}

	buff.WriteString("],\n")

	return buff.String()
}
