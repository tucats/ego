package tokenizer

// Symbolic names for each string token value.
const (
	AssertToken           = "assert"
	BoolToken             = "bool"
	BlockBeginToken       = "{"
	BlockEndToken         = "}"
	BreakToken            = "break"
	ByteToken             = "byte"
	CallToken             = "call"
	CaseToken             = "case"
	CatchToken            = "catch"
	ChanToken             = "chan"
	ConstToken            = "const"
	ContinueToken         = "continue"
	DataBeginToken        = "{"
	DataEndToken          = "}"
	DefaultToken          = "default"
	DeferToken            = "defer"
	ElseToken             = "else"
	EmptyBlockToken       = "{}"
	EmptyInitializerToken = "{}"
	EmptyInterfaceToken   = "interface{}"
	ExitToken             = "exit"
	Float32Token          = "flaot32"
	Float64Token          = "float64"
	ForToken              = "for"
	FuncToken             = "func"
	GoToken               = "go"
	IfToken               = "if"
	IntToken              = "int"
	Int32Token            = "int32"
	Int64Token            = "int64"
	InterfaceToken        = "interface"
	ImportToken           = "import"
	MapToken              = "map"
	NilToken              = "nil"
	PackageToken          = "package"
	PrintToken            = "print"
	ReturnToken           = "return"
	StringToken           = "string"
	StructToken           = "struct"
	SwitchToken           = "switch"
	TypeToken             = "type"
	TryToken              = "try"
	VarToken              = "var"
	SemicolonToken        = ";"
	ColonToken            = ":"
	AssignToken           = ":="
	CommaToken            = ","
)

// ReservedWords is the list of reserved words in the _Ego_ language.
var ReservedWords = map[string]bool{
	BoolToken:      true,
	BreakToken:     true,
	ByteToken:      true,
	ChanToken:      true,
	ConstToken:     true,
	ContinueToken:  true,
	DeferToken:     true,
	ElseToken:      true,
	Float32Token:   true,
	Float64Token:   true,
	ForToken:       true,
	FuncToken:      true,
	GoToken:        true,
	IfToken:        true,
	ImportToken:    true,
	InterfaceToken: true,
	IntToken:       true,
	Int32Token:     true,
	Int64Token:     true,
	MapToken:       true,
	NilToken:       true,
	PackageToken:   true,
	ReturnToken:    true,
	SwitchToken:    true,
	StringToken:    true,
	StructToken:    true,
	TypeToken:      true,
	VarToken:       true,
}

// ExtendedReservedWords are additional reserved words when running with
// language extensions enabled.
var ExtendedReservedWords = map[string]bool{
	CallToken:  true,
	CatchToken: true,
	PrintToken: true,
	TryToken:   true,
}

// IsReserved indicates if a name is a reserved word.
func IsReserved(name string, includeExtensions bool) bool {
	_, reserved := ReservedWords[name]
	if !reserved && includeExtensions {
		_, extendedReserved := ExtendedReservedWords[name]
		reserved = reserved || extendedReserved
	}

	return reserved
}
