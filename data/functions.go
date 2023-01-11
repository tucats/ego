package data

import (
	"fmt"
	"strings"
)

type FunctionParameter struct {
	Name     string
	ParmType *Type
}

type FunctionDeclaration struct {
	Name         string
	ReceiverType *Type
	Parameters   []FunctionParameter
	ReturnTypes  []*Type
}

// dictionary is a descriptive dictionary that shows the declaration string for
// built-in functions. These are used when you attempt to format a function
// that is a builtin (as opposed to compiled) function.  Note that this data
// MUST be kept in sync with the function definitions in the functions package.
var dictionary = map[string]string{
	"functions.Abs":               "Abs(any interface{}) interface{}",
	"functions.Append":            "append( any []interface{}, item... interface{}) []interface{}",
	"functions.BlockFonts":        "Blockfonts() []string",
	"functions.BlockPrint":        "BlockPrint(text string <, font string>) []string",
	"functions.Chars":             "Chars(text string) []string",
	"functions.CipherRandom":      "Random(bits int) string",
	"functions.CloseAny":          "close(any interface{})",
	"functions.Compare":           "Compare(a, b string) int",
	"functions.Contains":          "Contains(text, test string) bool",
	"functions.ContainsAny":       "ContainsAny(text, chars string) bool",
	"functions.CreateToken":       "Create(name, data string) string",
	"functions.DecodeBase64":      "Decode(encodedText string) string",
	"functions.Decrypt":           "Decrypt(data string, key string) string",
	"functions.Delete":            "delete(map interface{}, key string)",
	"functions.EncodeBase64":      "Encode(plainText string) string",
	"functions.Encrypt":           "Encrypt(encryptedText string, key string) string",
	"functions.EqualFold":         "EqualFold(a, b string) bool",
	"functions.Exec":              "Exec(command string) []string",
	"functions.Exit":              "Exit(code int)",
	"functions.Expand":            "Expand(path string, filter string) []string",
	"functions.Extract":           "Token(t string) struct{}",
	"functions.Fields":            "Fields(text string) []string",
	"functions.Format":            "Format(formatString string, args... interface{}) string",
	"functions.GetArgs":           "Args() []string",
	"functions.GetEnv":            "Getenv(name string) string ",
	"functions.Hash":              "Hash(data string) string",
	"functions.Hostname":          "Hostname() string",
	"functions.Index":             "index(text string, substr string) int",
	"functions.Ints":              "Ints(s string) []int",
	"functions.I18nLanguage":      "Language() string",
	"functions.I18nT":             "T(key string, parameters map[string]interface{}...) string",
	"functions.Join":              "Join( text []string, withText string) string",
	"functions.JSONMarshal":       "Marshal(any interface) ([]byte, error)",
	"functions.JSONMarshalIndent": "MarshalIndent(any interface, prefix string, indent string) ([]byte, error)",
	"functions.JSONUnmarshal":     "Marshal([]byte, *interface{}) error",
	"functions.Left":              "Left(text string, pos int) string",
	"functions.Length":            "len(any interface{}) int",
	"functions.Log":               "log(f float64) float64",
	"functions.LogTail":           "Log(count int) []string",
	"functions.Lower":             "ToLower(s string) string",
	"functions.Make":              "make(t type, count int) interface{}",
	"functions.Max":               "Max(item... interface{}) interface{}",
	"functions.Members":           "members(any interface{}) []string",
	"functions.MemStats":          "Memory() struct",
	"functions.Min":               "Min(item... interface{}) interface{}",
	"functios.Mode":               "Mode() string",
	"functions.New":               "new(any interface{}) interface{}",
	"functions.OpenFile":          "Open(filename string) File",
	"functions.Normalize":         "Normalize(any1, any2 interface{}) interface{}",
	"functions.Packages":          "Packages() []string",
	"functions.PathAbs":           "Abs(partialPath string) string",
	"functions.PathBase":          "Base(path string) string",
	"functions.PathClean":         "Clean(path string) string",
	"functions.PathDir":           "Dir(path string) string",
	"functions.PathExt":           "Ext(path string) string",
	"functions.PathJoin":          "Join(pathParts... string) string",
	"functions.Print":             "Print(item... interface{})",
	"functions.Printf":            "Println(item... interface{})",
	"functions.Println":           "Print(item... interface{})",
	"functions.ProfileDelete":     "Delete(key string)",
	"functions.ProfileGet":        "Get(key string)",
	"functions.ProfileKeys":       "Keys() []string",
	"functions.ProfileSet":        "Set(key string, value string)",
	"functions.Sprintf":           "Sprintf(fmt string, item... interface{}) string",
	"functions.Sscanf":            "Scanf(data string, fmt string, items... &interface()) int",
	"functions.Sizeof":            "sizeof(any interface{}) int",
	"functions.Random":            "Random(maximumValue int) int",
	"functions.ReadDir":           "ReadDir(path string) []map[string]interface{}",
	"functions.ReadFile":          "ReadFile(filename string) ([]byte, error)",
	"functions.DeleteFile":        "Remove(filename string) error",
	"functions.Reflect":           "reflect(any interface) struct",
	"functions.Right":             "Right(s string, count int) string",
	"functions.SetLogger":         "SetLogger(id string, state bool) bool",
	"functions.Signal":            "error(msg string) error",
	"functions.SortBytes":         "Bytes(data []byte) []byte",
	"functions.SortFloat32s":      "Float32s(data []float32) []float32",
	"functions.SortFloat64s":      "Float64s(data []float64) []float64",
	"functions.SortInts":          "Ints(data []int) []int",
	"functions.SortInt32s":        "Int32s(data []int32) []int32",
	"functions.SortInt64s":        "Int64s(data []int64) []int64",
	"functions.SortStrings":       "Strings(data []string) []string",
	"functions.Sort":              "Sort(data []interface{}) []interface{}",
	"functions.Split":             "Split(text string <, delim text>) []string",
	"functions.Sqrt":              "Sqrt(value float64) float64",
	"functions.StrLen":            "Length(text string) int",
	"functions.Substring":         "Substring(text string, position int, count int) string",
	"functions.Sum":               "Sum(item... int) int",
	"functions.Template":          "Template(name string <,values interface{}>) string",
	"functions.ToString":          "String(any interface{}) string",
	"functions.Type":              "type(any interface{}) string",
	"functions.Upper":             "ToUpper(s string) string",
	"functions.Tokenize":          "Tokenize(s string) []string",
	"functions.Truncate":          "Truncate(s string, maxLength int) string",
	"functions.URLPattern":        "URLPattern(url string, pattern string) map[string]string",
	"functions.UUIDNew":           "New() string",
	"functions.UUIDNil":           "Nil() string",
	"functions.UUIDParse":         "Parse(value string) (string, error)",
	"functions.Validate":          "Validate(token string) bool",
	"functions.WriteFile":         "WriteFile(filename string, data []byte, perms int) (int, error)",
	"runtime.DBNew":               "New(connection string) *db",
	"runtime.Eval":                "Eval(expressions string) interface{}",
	"runtime.FormatSymbols":       "Symbols([scope[,format]])",
	"runtime.LookPath":            "LookPath(command string) string",
	"runtime.NewCommand":          "Command(command string, args...string) Cmd",
	"runtime.Prompt":              "Prompt(promptString string) string",
	"runtime.ParseURL":            "ParseURL(url string) struct",
	"runtime.RestNew":             "New() rest.Client",
	"runtime.RestStatusMessage":   "Status(code int) string",
	"runtime.sortSlice":           "Slice(data []interface{}, func comparison(i, j int) bool) []interface{}",
	"runtime.SymbolTables":        "SymbolTables() []struct",
	"runtime.TableNew":            "New(columns []string) Table",
}

func GetBuiltinDeclaration(name string) string {
	return dictionary[name]
}

func (f FunctionDeclaration) String() string {
	r := strings.Builder{}

	if f.ReceiverType != nil {
		ptr := ""
		ft := f.ReceiverType

		if ft.kind == PointerKind {
			ptr = "*"
			ft = ft.valueType
		}

		varName := ft.name[:1]

		if strings.Contains(ft.name, ".") {
			names := strings.Split(ft.name, ".")
			varName = strings.ToLower(names[1][:1])
		}

		typeName := ft.name
		r.WriteString(fmt.Sprintf("(%s %s%s) ", varName, ptr, typeName))
	}

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

			r.WriteString(p.ShortTypeString())
		}

		if len(f.ReturnTypes) > 1 {
			r.WriteRune(')')
		}
	}

	return r.String()
}
