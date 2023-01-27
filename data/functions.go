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
	Variadic     bool
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
	"functions.CipherRandom":       "Random(bits int) string",
	"functions.CloseAny":           "close(any interface{})",
	"functions.Compare":            "Compare(a, b string) int",
	"functions.Contains":           "Contains(text, test string) bool",
	"functions.ContainsAny":        "ContainsAny(text, chars string) bool",
	"functions.CreateToken":        "Create(name, data string) string",
	"functions.DecodeBase64":       "Decode(encodedText string) string",
	"functions.Decrypt":            "Decrypt(data string, key string) string",
	"functions.Delete":             "delete(map interface{}, key string)",
	"functions.EncodeBase64":       "Encode(plainText string) string",
	"functions.Encrypt":            "Encrypt(encryptedText string, key string) string",
	"functions.EqualFold":          "EqualFold(a, b string) bool",
	"functions.Exec":               "Exec(command string) []string",
	"functions.Exit":               "Exit(code int)",
	"functions.Extract":            "Token(t string) struct{}",
	"functions.Fields":             "Fields(text string) []string",
	"functions.Format":             "Format(formatString string, args... interface{}) string",
	"functions.Hash":               "Hash(data string) string",
	"functions.Hostname":           "Hostname() string",
	"functions.Index":              "index(text string, substr string) int",
	"functions.Ints":               "Ints(s string) []int",
	"functions.I18nLanguage":       "Language() string",
	"functions.I18nT":              "T(key string, parameters map[string]interface{}...) string",
	"functions.Join":               "Join( text []string, withText string) string",
	"functions.JSONMarshal":        "Marshal(any interface) ([]byte, error)",
	"functions.JSONMarshalIndent":  "MarshalIndent(any interface, prefix string, indent string) ([]byte, error)",
	"functions.JSONUnmarshal":      "Marshal([]byte, *interface{}) error",
	"functions.Left":               "Left(text string, pos int) string",
	"functions.Length":             "len(any interface{}) int",
	"functions.Log":                "log(f float64) float64",
	"functions.LogTail":            "Log(count int) []string",
	"functions.Lower":              "ToLower(s string) string",
	"functions.Make":               "make(t type, count int) interface{}",
	"functions.Max":                "Max(item... interface{}) interface{}",
	"functions.Members":            "members(any interface{}) []string",
	"functions.MemStats":           "Memory() struct",
	"functions.Min":                "Min(item... interface{}) interface{}",
	"functios.Mode":                "Mode() string",
	"functions.New":                "InstanceOf(any interface{}) interface{}",
	"functions.Normalize":          "Normalize(any1, any2 interface{}) interface{}",
	"functions.Packages":           "Packages() []string",
	"functions.PathAbs":            "Abs(partialPath string) string",
	"functions.PathBase":           "Base(path string) string",
	"functions.PathClean":          "Clean(path string) string",
	"functions.PathDir":            "Dir(path string) string",
	"functions.PathExt":            "Ext(path string) string",
	"functions.PathJoin":           "Join(pathParts... string) string",
	"functions.Print":              "Print(item... interface{})",
	"functions.Printf":             "Println(item... interface{})",
	"functions.Println":            "Println(item... interface{})",
	"functions.ProfileDelete":      "Delete(key string)",
	"functions.ProfileGet":         "Get(key string)",
	"functions.ProfileKeys":        "Keys() []string",
	"functions.ProfileSet":         "Set(key string, value string)",
	"functions.Sprintf":            "Sprintf(fmt string, item... interface{}) string",
	"functions.Sscanf":             "Scanf(data string, fmt string, items... &interface()) int",
	"functions.Sizeof":             "sizeof(any interface{}) int",
	"functions.Random":             "Random(maximumValue int) int",
	"functions.ReadFile":           "ReadFile(filename string) ([]byte, error)",
	"functions.DeleteFile":         "Remove(filename string) error",
	"functions.ReflectReflect":     "Reflect(any interface) struct",
	"functions.ReflectType":        "Type(any interface{}) string",
	"functions.ReflectMembers":     "Members(any interface) []string]",
	"functions.ReflectNew":         "New(any interface) any",
	"functions.ReflectDeepCopy":    "DeepCopy(any interface{}, depth) string",
	"functions.Right":              "Right(s string, count int) string",
	"functions.SetLogger":          "SetLogger(id string, state bool) bool",
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
	"functions.TimeNow":            "Now() *time.Time",
	"functions.TimeParse":          "Parse(timeSpec string) (*time.Time, error)",
	"functions.ToString":           "String(any interface{}) string",
	"functions.Upper":              "ToUpper(s string) string",
	"functions.Tokenize":           "Tokenize(s string) []string",
	"functions.Truncate":           "Truncate(s string, maxLength int) string",
	"functions.URLPattern":         "URLPattern(url string, pattern string) map[string]string",
	"functions.UUIDNew":            "New() string",
	"functions.UUIDNil":            "Nil() string",
	"functions.UUIDParse":          "Parse(value string) (string, error)",
	"functions.Validate":           "Validate(token string) bool",
	"functions.WriteFile":          "WriteFile(filename string, data []byte, perms int) (int, error)",
	"runtime.Eval":                 "Eval(expressions string) interface{}",
	"runtime.FormatSymbols":        "Symbols([scope[,format]])",
	"runtime.LookPath":             "LookPath(command string) string",
	"runtime.exec.Command":         "Command(command string, args...string) Cmd",
	"runtime.ParseURL":             "ParseURL(url string) struct",
	"runtime.RestStatusMessage":    "Status(code int) string",
	"runtime.sortSlice":            "Slice(data []interface{}, func comparison(i, j int) bool) []interface{}",
	"runtime.SymbolTables":         "SymbolTables() []struct",
	"runtime/db.New":               "New(connection string) *db.Client",
	"runtime/io.Expand":            "Expand(path string[, filter string]) []string",
	"runtime/io.Open":              "Open(filename string) (File, error)",
	"runtime/io.Prompt":            "Prompt(promptString string) string",
	"runtime/io.ReadDir":           "ReadDir(path string) []io.Entry",
	"runtime/os.Args":              "Args() []string",
	"runtime/os.Chdir":             "Chdir(path string) error)",
	"runtime/os.Chmod":             "Chmod(file string, mode int) error",
	"runtime/os.Chown":             "Chown(file string, owner string) error",
	"runtime/os.Clearenv":          "Clearenv()",
	"runtime/os.Environ":           "Environ() []string",
	"runtime/os.Executable":        "Executable() string",
	"runtime/os.Exit":              "Exit(status int)",
	"runtime/os.Getenv":            "Getenv(name string) string",
	"runtime/os.Hostname":          "Hostname() string",
	"runtime/os.Readfile":          "Readfile(filename string) ([]byte, error)",
	"runtime/os.Remove":            "Remove(filename string) error",
	"runtime/os.Writefile":         "Writefile(filename string, mode int, data []byte) error",
	"runtime/rest.New":             "New() *rest.Client",
	"runtime/rest.ParseURL":        "ParseURL(url string) map[string]interface{}",
	"runtime/rest.Status":          "Status() int",
	"runtime/sort.Bytes":           "Bytes(data []byte) []byte",
	"runtime/sort.Float32s":        "Float32s(data []float32) []float32",
	"runtime/sort.Float64s":        "Float64s(data []float64) []float64",
	"runtime/sort.Ints":            "Ints(data []int) []int",
	"runtime/sort.Int32s":          "Int32s(data []int32) []int32",
	"runtime/sort.Int64s":          "Int64s(data []int64) []int64",
	"funtime/sort.Slice":           "Slice([]interface{}, func less(i, j int) bool)",
	"runtime/sort.Sort":            "Sort(data []interface{}) []interface{}",
	"runtime/sort.Strings":         "Strings(data []string) []string",
	"runtime/tables.New":           "New(columns []string) Table",
	"runtime/time.Now":             "Now() time.Time",
	"runtime/time.Parse":           "Parse(text string) time.Time",
	"runtime/time.Since":           "Since(t time.Time) string",
	"runtime/time.Sleep":           "Sleep(duration string)",
	"runtime/uuid.New":             "New() uuid.UUID",
	"runtime/uuid.Nil":             "Nil() uuid.UUID",
	"runtime/uuid.Parse":           "Parse( text string ) uuid.UUID",
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
		} else {
			varName = strings.ToLower(varName)
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

		if f.Variadic && i == len(f.Parameters)-1 {
			r.WriteString("...")
		}

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
				r.WriteString(", ")
			}

			r.WriteString(p.ShortTypeString())
		}

		if len(f.ReturnTypes) > 1 {
			r.WriteRune(')')
		}
	}

	return r.String()
}
