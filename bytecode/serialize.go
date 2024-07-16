package bytecode

import (
	"fmt"
	"strings"

	"github.com/tucats/ego/data"
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
		buff.WriteString(fmt.Sprintf(`{"%s": %s}`, opcodeNames[inst.Operation], serializeArg(inst.Operand)))
	}

	buff.WriteString("]")
	return buff.String()
}

func serializeArg(arg interface{}) string {
	switch arg := arg.(type) {
	case nil:
		return `{"type":"null"}`
	case *data.Type:
		return fmt.Sprintf(`{"type":"type", "valye":"%s"}`, arg.String())
	case bool:
		return fmt.Sprintf(`{"type":"bool", "value":%t}`, arg)
	case byte:
		return fmt.Sprintf(`{"type":"byte", "value":"%d"}`, arg)
	case float32:
		return fmt.Sprintf(`{"type":"float32", "value":"%f"}`, arg)
	case float64:
		return fmt.Sprintf(`{"type":"float64", "value":"%f"}`, arg)
	case int:
		return fmt.Sprintf(`{"type":"int", "value":"%d"}`, arg)
	case int32:
		return fmt.Sprintf(`{"type":"int32", "value":"%d"}`, arg)
	case int64:
		return fmt.Sprintf(`{"type":"int64", "value":"%d"}`, arg)
	case string:
		return fmt.Sprintf(`{"type":"string", "value": "%s"}`, arg)
	case []byte:
		return fmt.Sprintf(`{"type":"[]byte", "value":"%s"}`, string(arg))
	case []interface{}:
		list := data.NewList(arg...)
		return fmt.Sprintf(`{"type":"[]interface{}]", "value":%s}`, serializeList(list))

	case data.List:
		return fmt.Sprintf(`{"type":"list", "value":%s}`, serializeList(arg))
	default:
		value := util.Escape(fmt.Sprintf("%#v", arg))
		return fmt.Sprintf(`{"type":"%T", "value":"%s"}`, arg, value)
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
