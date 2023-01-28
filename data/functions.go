package data

import (
	"fmt"
	"strings"
)

type Parameter struct {
	Name string
	Type *Type
}

type Range [2]int

type Declaration struct {
	Name       string
	Type       *Type
	Parameters []Parameter
	Returns    []*Type
	Variadic   bool
	Scope      bool
	ArgCount   Range
}

// dictionary is a descriptive dictionary that shows the declaration string for
// built-in functions. These are used when you attempt to format a function
// that is a builtin (as opposed to compiled) function.  Note that this data
// MUST be kept in sync with the function definitions in the functions package.
var dictionary = map[string]string{
	"functions.Abs":                "Abs(any interface{}) interface{}",
	"functions.Append":             "append( any []interface{}, item... interface{}) []interface{}",
	"functions.BlockFonts":         "Blockfonts() []string",
	"functions.BlockPrint":         "BlockPrint(text string <, font string>) []string",
	"functions.Chars":              "Chars(text string) []string",
	"functions.CloseAny":           "close(any interface{})",
	"functions.Compare":            "Compare(a, b string) int",
	"functions.Contains":           "Contains(text, test string) bool",
	"functions.ContainsAny":        "ContainsAny(text, chars string) bool",
	"functions.Delete":             "delete(map interface{}, key string)",
	"functions.EqualFold":          "EqualFold(a, b string) bool",
	"functions.Fields":             "Fields(text string) []string",
	"functions.Format":             "Format(formatString string, args... interface{}) string",
	"functions.Index":              "index(text string, substr string) int",
	"functions.Join":               "Join( text []string, withText string) string",
	"functions.JSONMarshal":        "Marshal(any interface) ([]byte, error)",
	"functions.JSONMarshalIndent":  "MarshalIndent(any interface, prefix string, indent string) ([]byte, error)",
	"functions.JSONUnmarshal":      "Marshal([]byte, *interface{}) error",
	"functions.Left":               "Left(text string, pos int) string",
	"functions.Length":             "len(any interface{}) int",
	"functions.Log":                "log(f float64) float64",
	"functions.Lower":              "ToLower(s string) string",
	"functions.Make":               "make(t type, count int) interface{}",
	"functions.Max":                "Max(item... interface{}) interface{}",
	"functions.Members":            "members(any interface{}) []string",
	"functions.Min":                "Min(item... interface{}) interface{}",
	"functions.New":                "InstanceOf(any interface{}) interface{}",
	"functions.Normalize":          "Normalize(any1, any2 interface{}) interface{}",
	"functions.ProfileDelete":      "Delete(key string)",
	"functions.ProfileGet":         "Get(key string)",
	"functions.ProfileKeys":        "Keys() []string",
	"functions.ProfileSet":         "Set(key string, value string)",
	"functions.Sizeof":             "sizeof(any interface{}) int",
	"functions.Random":             "Random(maximumValue int) int",
	"functions.ReflectReflect":     "Reflect(any interface) struct",
	"functions.ReflectType":        "Type(any interface{}) string",
	"functions.ReflectMembers":     "Members(any interface) []string]",
	"functions.ReflectNew":         "New(any interface) any",
	"functions.ReflectDeepCopy":    "DeepCopy(any interface{}, depth) string",
	"functions.Right":              "Right(s string, count int) string",
	"functions.Signal":             "error(msg string) error",
	"functions.Split":              "Split(text string <, delim text>) []string",
	"functions.Sqrt":               "Sqrt(value float64) float64",
	"functions.StrConvAtoi":        "Atoi(value string) (int, error)",
	"functions.StrConvFormatBool":  "Formatbool(value bool) string",
	"functions.StrConvFormatFloat": "Formatfloat(value float64, fmt byte, prec int, bitsize int) string",
	"functions.StrConvFormatInt":   "Formatint(value int64, base int) string",
	"functions.StrConvItoa":        "Itoa(value int) string",
	"functions.StrConvQuote":       "Quote(s string) string",
	"functions.StrConvUnquote":     "Unquote(s string) (string, error)",
	"functions.StrLen":             "Length(text string) int",
	"functions.Substring":          "Substring(text string, position int, count int) string",
	"functions.Sum":                "Sum(item... int) int",
	"functions.Template":           "Template(name string <,values interface{}>) string",
	"functions.ToString":           "String(any interface{}) string",
	"functions.Upper":              "ToUpper(s string) string",
	"functions.Tokenize":           "Tokenize(s string) []string",
	"functions.Truncate":           "Truncate(s string, maxLength int) string",
}

func GetBuiltinDeclaration(name string) string {
	return dictionary[name]
}

func (f Declaration) String() string {
	r := strings.Builder{}

	if f.Type != nil {
		ptr := ""
		ft := f.Type

		if ft.kind == PointerKind {
			ptr = "*"
			ft = ft.valueType
		}

		varName := ft.name[:1]

		if strings.Contains(ft.name, ".") {
			names := strings.Split(ft.name, ".")
			varName = strings.ToLower(names[1][:1])
		} else {
			varName = strings.ToLower(varName)
		}

		typeName := ft.name
		r.WriteString(fmt.Sprintf("(%s %s%s) ", varName, ptr, typeName))
	}

	r.WriteString(f.Name)
	r.WriteRune('(')

	variable := (f.ArgCount[0] != 0 || f.ArgCount[1] != 0)

	for i, p := range f.Parameters {
		if variable && i == f.ArgCount[1]-1 {
			if i == 0 {
				r.WriteString("[")
			} else {
				r.WriteString(" [")
			}
		}

		if i > 0 {
			r.WriteString(", ")
		}

		r.WriteString(p.Name)

		if f.Variadic && i == len(f.Parameters)-1 {
			r.WriteString("...")
		}

		r.WriteRune(' ')
		r.WriteString(p.Type.ShortString())
	}

	if variable {
		r.WriteString("]")
	}

	r.WriteRune(')')

	if len(f.Returns) > 0 {
		r.WriteRune(' ')

		if len(f.Returns) > 1 {
			r.WriteRune('(')
		}

		for i, p := range f.Returns {
			if i > 0 {
				r.WriteString(", ")
			}

			r.WriteString(p.ShortTypeString())
		}

		if len(f.Returns) > 1 {
			r.WriteRune(')')
		}
	}

	return r.String()
}
