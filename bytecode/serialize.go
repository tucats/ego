package bytecode

import (
	"fmt"
	"strings"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/tokenizer"
	"github.com/tucats/ego/util"
)

// Implement the Serialize bytecode
func serializeByteCode(c *Context, i interface{}) error {
	var err error

	if i == nil {
		i, err = c.Pop()
		if err != nil {
			return err
		}
	}

	if bc, ok := i.(*ByteCode); ok {
		text, err := bc.Serialize()
		if err != nil {
			return err
		}

		c.push(text)
	} else {
		return c.error(fmt.Errorf("expected a bytecode instance, got %T", i))
	}

	return nil
}

func (b ByteCode) Serialize() (string, error) {

	var (
		err  error
		buff = strings.Builder{}
	)

	buff.WriteString("{\n")

	buff.WriteString(`"main": true,`)
	buff.WriteString("\n")

	buff.WriteString(fmt.Sprintf(`"name": "%s",`, b.Name()))
	buff.WriteString("\n")

	if d := b.Declaration(); d != nil {
		buff.WriteString(fmt.Sprintf(`"declaration": "%s",`, b.Declaration().String()))
		buff.WriteString("\n")
	}

	buff.WriteString(fmt.Sprintf(`"code": %s`, serializeCode(b.instructions, b.nextAddress)))
	buff.WriteString("\n")

	buff.WriteString("}\n")

	return buff.String(), err
}

func serializeCode(instructions []instruction, length int) string {
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
			argPart = fmt.Sprintf(`, "i":%s`, serializeArg(inst.Operand))
		}

		buff.WriteString(fmt.Sprintf(`{"o":"%s"%s}`, opcodeNames[inst.Operation], argPart))
	}

	buff.WriteString("]")
	return buff.String()
}

func serializeArg(arg interface{}) string {
	switch arg := arg.(type) {
	case nil:
		return `{"type":"null"}`
	case *data.Type:
		return fmt.Sprintf(`{"t":"@t", "v":"%s"}`, arg.String())
	case bool:
		return fmt.Sprintf(`{"t":"@b", "v":%t}`, arg)
	case byte:
		return fmt.Sprintf(`{"t":"@i8", "v":"%d"}`, arg)
	case float32:
		return fmt.Sprintf(`{"t":"@f32", "v":"%f"}`, arg)
	case float64:
		return fmt.Sprintf(`{"t":"@f64", "v":"%f"}`, arg)
	case int:
		return fmt.Sprintf(`{"t":"@i", "v":"%d"}`, arg)
	case int32:
		return fmt.Sprintf(`{"t":"@i32", "v":"%d"}`, arg)
	case int64:
		return fmt.Sprintf(`{"t":"@i64", "v":"%d"}`, arg)
	case string:
		return fmt.Sprintf(`{"t":"@s", "v": "%s"}`, arg)
	case []byte:
		return fmt.Sprintf(`{"t":"@ai8", "v":"%s"}`, string(arg))
	case []interface{}:
		list := data.NewList(arg...)
		return fmt.Sprintf(`{"t":"@ax", "v":%s}`, serializeList(list))

	case data.List:
		return fmt.Sprintf(`{"t":"@l", "v":%s}`, serializeList(arg))

	case *ByteCode:
		buff := strings.Builder{}
		buff.WriteString("{\n")

		buff.WriteString(fmt.Sprintf(`"name": "%s",`, arg.Name()))
		buff.WriteString("\n")

		if d := arg.Declaration(); d != nil {
			buff.WriteString(fmt.Sprintf(`"declaration": "%s",`, arg.Declaration().String()))
			buff.WriteString("\n")
		}

		buff.WriteString(fmt.Sprintf(`"code": %s}`, serializeCode(arg.instructions, arg.nextAddress)))

		return fmt.Sprintf(`{"t":"@bc", "v":%s}`, buff.String())

	case StackMarker:
		name := arg.label
		value := ""
		if len(arg.values) > 0 {
			value = fmt.Sprintf(`, "v":%s`, serializeArg(arg.values))
		}

		return fmt.Sprintf(`{"t":"@sm %s"%s}`, name, value)

	case tokenizer.Token:
		return fmt.Sprintf(`{"t":"@tk", "v":{"spell":"%s", "class": %d}}`,
			arg.Spelling(), arg.Class())

	default:
		value := util.Escape(fmt.Sprintf("%#v", arg))
		return fmt.Sprintf(`{"t":"%T", "v":"%s"}`, arg, value)
	}
}

func serializeList(l data.List) string {
	buff := strings.Builder{}

	buff.WriteString("[")
	for i, v := range l.Elements() {
		if i > 0 {
			buff.WriteString(", ")
		}
		buff.WriteString(serializeArg(v))
	}

	buff.WriteString("]")
	return buff.String()
}
