package datatypes

import (
	"reflect"
	"strings"

	"github.com/tucats/ego/defs"
)

type FunctionParameter struct {
	Name     string
	ParmType *Type
}

type FunctionDeclaration struct {
	Name        string
	Parameters  []FunctionParameter
	ReturnTypes []*Type
}

// dictionary is a descriptive dictionary that shows the declaration string for
// built-in functions. These are used when you attempt to format a function
// that is a builtin (as opposed to compiled) function.  Note that this data
// MUST be kept in sync with the function definitions in the functions package.
var dictionary = map[string]string{
	"functions.Abs":          "Abs(any interface{}) interface{}",
	"functions.Append":       "append( any []interface{}, item... interface{}) []interface{}",
	"functions.BlockFonts":   "Blockfonts() []string",
	"functions.BlockPrint":   "BlockPrint(text string <, font string>) []string",
	"functions.Chars":        "Chars(text string) []string",
	"functions.CipherRandom": "Random(bits int) string",
	"functions.Compare":      "Compare(a, b string) int",
	"functions.Contains":     "Contains(text, test string) bool",
	"functions.ContainsAny":  "ContainsAny(text, chars string) bool",
	"functions.CreateToken":  "Create(name, data string) string",
	"functions.DecodeBase64": "Decode(encodedText string) string",
	"functions.Decrypt":      "Decrypt(data string, key string) string",
	"functions.EncodeBase64": "Encode(plainText string) string",
	"functions.Encrypt":      "Encrypt(encryptedText string, key string) string",
	"functions.EqualFold":    "EqualFold(a, b string) bool",
	"functions.Extract":      "Token(t string) struct{}",
	"functions.Fields":       "Fields(text string) []string",
	"functions.Format":       "Format(formatString string <, args... interface{}>) string",
	"functions.Hash":         "Hash(data string) string",
	"functions.Index":        "Index(text string, substr string) int",
	"functions.Ints":         "Ints(s string) []int",
	"functions.Join":         "Join( text []string, withText string) string",
	"functions.Left":         "Left(text string, pos int) string",
	"functions.Length":       "len(any interface{}) int",
	"functions.Log":          "log(f float64) float64",
	"functions.Lower":        "ToLower(s string) string",
	"functions.Min":          "Min(item... interface{}) interface{}",
	"functions.Max":          "Max(item... interface{}) interface{}",
	"functions.Normalize":    "Normalize(any1, any2 interface{}) interface{}",
	"functions.Packages":     "Packages() []string",
	"functions.PathAbs":      "Abs(partialPath string) string",
	"functions.PathBase":     "Base(path string) string",
	"functions.PathClean":    "Clean(path string) string",
	"functions.PathDir":      "Dir(path string) string",
	"functions.PathExt":      "Ext(path string) string",
	"functions.PathJoin":     "Join(pathParts... string) string",
	"functions.Print":        "Print(item... interface{})",
	"functions.Printf":       "Println(item... interface{})",
	"functions.Println":      "Print(item... interface{})",
	"functions.Sprintf":      "Sprintf(fmt string, item... interface{}) string",
	"functions.Sscanf":       "Scanf(data string, fmt string, items... &interface()) int",
	"functions.Random":       "Random(maximumValue int) int",
	"functions.Right":        "Right(s string, count int) string",
	"functions.Signal":       "New(msg string) error",
	"functions.Split":        "Split(text string <, delim text>) []string",
	"functions.StrLen":       "Length(text string) int",
	"functions.Sqrt":         "Sqrt(value float64) float64",
	"functions.ToString":     "String(any interface{}) string",
	"functions.Substring":    "Substring(text string, position int, count int) string",
	"functions.Sum":          "Sum(item... int) int",
	"functions.Template":     "Template(name string <,values interface{}>) string",
	"functions.Upper":        "ToUpper(s string) string",
	"functions.Tokenize":     "Tokenize(s string) []string",
	"functions.Truncate":     "Truncate(s string, maxLength int) string",
	"functions.URLPattern":   "URLPattern(url string, pattern string) map[string]string",
	"functions.Validate":     "Valicate(token string) bool",
	"runtime.DBNew":          "New(connection string) *db",
	"runtime.LookPath":       "LookPath(command string) string",
	"runtime.NewCommand":     "Command(command string, args...string) Cmd",
}

func GetBuiltinDeclaration(name string) string {
	return dictionary[name]
}

func (f FunctionDeclaration) String() string {
	r := strings.Builder{}
	r.WriteString(f.Name)
	r.WriteRune('(')

	for i, p := range f.Parameters {
		if i > 0 {
			r.WriteString(", ")
		}

		r.WriteString(p.Name)
		r.WriteRune(' ')
		r.WriteString(p.ParmType.String())
	}

	r.WriteRune(')')

	if len(f.ReturnTypes) > 0 {
		r.WriteRune(' ')

		if len(f.ReturnTypes) > 1 {
			r.WriteRune('(')
		}

		for i, p := range f.ReturnTypes {
			if i > 0 {
				r.WriteRune(',')
			}

			r.WriteString(p.String())
		}

		if len(f.ReturnTypes) > 1 {
			r.WriteRune(')')
		}
	}

	return r.String()
}

// GetDeclaration returns the embedded function declaration from a
// bytecode stream, if any. It has to use reflection (ick) to do this
// because my package structure needs work and I haven't found a way to
// do this without creating import cycles.
func GetDeclaration(bc interface{}) *FunctionDeclaration {
	vv := reflect.ValueOf(bc)
	ts := vv.String()

	// If it's a bytecode.Bytecode pointer, use reflection to get the
	// Name field value and use that with the name. A function literal
	// will have no name.
	if vv.Kind() == reflect.Ptr {
		if ts == defs.ByteCodeReflectionTypeString {
			switch v := bc.(type) {
			default:
				e := reflect.ValueOf(v).Elem()
				fd, _ := e.Field(3).Interface().(*FunctionDeclaration)

				return fd
			}
		}
	}

	return nil
}
