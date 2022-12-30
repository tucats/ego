package tokenizer

// Symbolic names for each string token value.
var (
	AssertToken              = NewReservedToken("assert")
	BoolToken                = NewTypeToken("bool")
	BlockBeginToken          = NewSpecialToken("{")
	BlockEndToken            = NewSpecialToken("}")
	BreakToken               = NewReservedToken("break")
	ByteToken                = NewTypeToken("byte")
	CallToken                = NewReservedToken("call")
	CaseToken                = NewIdentifierToken("case")
	CatchToken               = NewReservedToken("catch")
	ChanToken                = NewTypeToken("chan")
	ClearToken               = NewIdentifierToken("clear")
	ConstToken               = NewReservedToken("const")
	ContinueToken            = NewReservedToken("continue")
	DataBeginToken           = NewSpecialToken("{")
	DataEndToken             = NewSpecialToken("}")
	DefaultToken             = NewIdentifierToken("default")
	DeferToken               = NewReservedToken("defer")
	DirectiveToken           = NewSpecialToken("@")
	ElseToken                = NewReservedToken("else")
	EmptyBlockToken          = NewSpecialToken("{}")
	EmptyInitializerToken    = NewSpecialToken("{}")
	EmptyInterfaceToken      = NewTypeToken("interface{}")
	ExitToken                = NewReservedToken("exit")
	Float32Token             = NewTypeToken("float32")
	Float64Token             = NewTypeToken("float64")
	ForToken                 = NewReservedToken("for")
	FuncToken                = NewReservedToken("func")
	GoToken                  = NewReservedToken("go")
	IfToken                  = NewReservedToken("if")
	IntToken                 = NewTypeToken("int")
	Int32Token               = NewTypeToken("int32")
	Int64Token               = NewTypeToken("int64")
	InterfaceToken           = NewIdentifierToken("interface")
	ImportToken              = NewReservedToken("import")
	MakeToken                = NewReservedToken("make")
	MapToken                 = NewTypeToken("map")
	NilToken                 = NewReservedToken("nil")
	PackageToken             = NewReservedToken("package")
	PanicToken               = NewReservedToken("panic")
	PrintToken               = NewReservedToken("print")
	RangeToken               = NewIdentifierToken("range")
	ReturnToken              = NewReservedToken("return")
	StringToken              = NewTypeToken("string")
	StructToken              = NewTypeToken("struct")
	SwitchToken              = NewReservedToken("switch")
	TestToken                = NewIdentifierToken("test")
	TypeToken                = NewReservedToken("type")
	TryToken                 = NewReservedToken("try")
	VarToken                 = NewReservedToken("var")
	WhenToken                = NewIdentifierToken("when")
	SemicolonToken           = NewSpecialToken(";")
	ColonToken               = NewSpecialToken(":")
	DefineToken              = NewSpecialToken(":=")
	AssignToken              = NewSpecialToken("=")
	CommaToken               = NewSpecialToken(",")
	EqualsToken              = NewSpecialToken("==")
	GreaterThanToken         = NewSpecialToken(">")
	GreaterThanOrEqualsToken = NewSpecialToken(">=")
	LessThanToken            = NewSpecialToken("<")
	LessThanOrEqualsToken    = NewSpecialToken("<=")
	ShiftLeftToken           = NewSpecialToken("<<")
	ShiftRightToken          = NewSpecialToken(">>")
	NotToken                 = NewSpecialToken("!")
	NotEqualsToken           = NewSpecialToken("!=")
	ModuloToken              = NewSpecialToken("%")
	ExponentToken            = NewSpecialToken("^")
	AddToken                 = NewSpecialToken("+")
	SubtractToken            = NewSpecialToken("-")
	MultiplyToken            = NewSpecialToken("*")
	DivideToken              = NewSpecialToken("/")
	PointerToken             = NewSpecialToken("*")
	AddressToken             = NewSpecialToken("&")
	AndToken                 = NewSpecialToken("&")
	OrToken                  = NewSpecialToken("|")
	BooleanAndToken          = NewSpecialToken("&&")
	BooleanOrToken           = NewSpecialToken("||")
	AddAssignToken           = NewSpecialToken("+=")
	SubtractAssignToken      = NewSpecialToken("-=")
	MultiplyAssignToken      = NewSpecialToken("*=")
	DivideAssignToken        = NewSpecialToken("/=")
	IncrementToken           = NewSpecialToken("++")
	DecrementToken           = NewSpecialToken("--")
	DotToken                 = NewSpecialToken(".")
	VariadicToken            = NewSpecialToken("...")
	ChannelReceiveToken      = NewSpecialToken("<-")
	ChannelSendToken         = NewSpecialToken("->")
	StartOfListToken         = NewSpecialToken("(")
	EndOfListToken           = NewSpecialToken(")")
	StartOfArrayToken        = NewSpecialToken("[")
	EndOfArrayToken          = NewSpecialToken("]")
	OptionalToken            = NewSpecialToken("?")
	EmptyToken               = NewSpecialToken("")
	NegateToken              = NewSpecialToken("-")
)

// TypeTokens is a list of tokens that represent type names.
var TypeTokens = map[Token]bool{
	BoolToken:    true,
	ByteToken:    true,
	IntToken:     true,
	Int32Token:   true,
	Int64Token:   true,
	Float32Token: true,
	Float64Token: true,
	StringToken:  true,
	StructToken:  true,
	MapToken:     true,
}

// SpecialTokens is a list of tokens that are considered special symantic characters.
var SpecialTokens = map[Token]bool{
	BlockBeginToken:          true,
	BlockEndToken:            true,
	DataBeginToken:           true,
	DataEndToken:             true,
	DirectiveToken:           true,
	EmptyBlockToken:          true,
	EmptyInitializerToken:    true,
	SemicolonToken:           true,
	ColonToken:               true,
	DefineToken:              true,
	AssignToken:              true,
	CommaToken:               true,
	EqualsToken:              true,
	GreaterThanToken:         true,
	GreaterThanOrEqualsToken: true,
	LessThanToken:            true,
	LessThanOrEqualsToken:    true,
	ShiftLeftToken:           true,
	ShiftRightToken:          true,
	NotToken:                 true,
	NotEqualsToken:           true,
	ModuloToken:              true,
	ExponentToken:            true,
	AddToken:                 true,
	SubtractToken:            true,
	MultiplyToken:            true,
	DivideToken:              true,
	PointerToken:             true,
	AddressToken:             true,
	AndToken:                 true,
	OrToken:                  true,
	BooleanAndToken:          true,
	BooleanOrToken:           true,
	AddAssignToken:           true,
	SubtractAssignToken:      true,
	MultiplyAssignToken:      true,
	DivideAssignToken:        true,
	IncrementToken:           true,
	DecrementToken:           true,
	DotToken:                 true,
	VariadicToken:            true,
	ChannelReceiveToken:      true,
	ChannelSendToken:         true,
	StartOfListToken:         true,
	EndOfListToken:           true,
	StartOfArrayToken:        true,
	EndOfArrayToken:          true,
	OptionalToken:            true,
	EmptyToken:               true,
	NegateToken:              true,
}

// ReservedWords is the list of reserved words in the _Ego_ language.
var ReservedWords = map[Token]bool{
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
var ExtendedReservedWords = map[Token]bool{
	CallToken:  true,
	CatchToken: true,
	PrintToken: true,
	TryToken:   true,
	ExitToken:  true,
}

// This is a list of spellings of reserved words that should be
// considered as identifiers as well.
var reservedIdentifiers = map[Token]bool{
	MakeToken:      true,
	TypeToken:      true,
	InterfaceToken: true,
}

// IsReserved indicates if a name is a reserved word.
func (t Token) IsReserved(includeExtensions bool) bool {
	_, reserved := ReservedWords[t]

	if !reserved && includeExtensions {
		_, extendedReserved := ExtendedReservedWords[t]
		reserved = reserved || extendedReserved
	}

	return reserved
}
