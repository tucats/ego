package tokenizer

// The variables below are pre-built Token values for every keyword, operator,
// and punctuation symbol in the Ego language. Declaring them once here and
// reusing them throughout the codebase avoids constructing identical Token
// values repeatedly and makes comparison sites readable: instead of writing
//
//	token.Is(NewSpecialToken("{"))
//
// callers write
//
//	token.Is(BlockBeginToken)
//
// Each variable is created by one of the typed constructor functions
// (NewReservedToken, NewSpecialToken, NewTypeToken, NewIdentifierToken) so
// that its class is set correctly at construction time.
var (
	// "assert" token.
	AssertToken = NewReservedToken("assert")

	// "bool" token.
	BoolToken = NewTypeToken("bool")

	// "{" token.
	BlockBeginToken = NewSpecialToken("{")

	// "}" token.
	BlockEndToken = NewSpecialToken("}")

	// "break" token.
	BreakToken = NewReservedToken("break")

	// "byte" token.
	ByteToken = NewTypeToken("byte")

	// "call" token.
	CallToken = NewReservedToken("call")

	// "case" token.
	CaseToken = NewIdentifierToken("case")

	// "catch" token.
	CatchToken = NewReservedToken("catch")

	// "chan"  token.
	ChanToken = NewTypeToken("chan")

	// "clear" token.
	ClearToken = NewIdentifierToken("clear")

	// "const" token.
	ConstToken = NewReservedToken("const")

	// "continue"  token.
	ContinueToken = NewReservedToken("continue")

	// "{" token.
	DataBeginToken = NewSpecialToken("{")

	// "}" token.
	DataEndToken = NewSpecialToken("}")

	// "default" token.
	DefaultToken = NewIdentifierToken("default")

	// "defer" token.
	DeferToken = NewReservedToken("defer")

	// "@" token.
	DirectiveToken = NewSpecialToken("@")

	// "else" token.
	ElseToken = NewReservedToken("else")

	// "{}" token.
	EmptyBlockToken = NewSpecialToken("{}")

	// "{}" token.
	EmptyInitializerToken = NewSpecialToken("{}")

	// "interface{}" token.
	EmptyInterfaceToken = NewTypeToken("interface{}")

	// "any" token.
	AnyToken = NewTypeToken("any")

	// "error" token.
	ErrorToken = NewIdentifierToken("error")

	// "exit" token.
	ExitToken = NewReservedToken("exit")

	// "fallthrough" token.
	FallthroughToken = NewReservedToken("fallthrough")

	// "float32" token.
	Float32Token = NewTypeToken("float32")

	// "float64" token.
	Float64Token = NewTypeToken("float64")

	// "for" token.
	ForToken = NewReservedToken("for")

	// "func" token.
	FuncToken = NewReservedToken("func")

	// "go" token.
	GoToken = NewReservedToken("go")

	// "if" token.
	IfToken = NewReservedToken("if")

	// "int8" token.
	Int8Token = NewTypeToken("int8")

	// "int16" token.
	Int16Token = NewTypeToken("int16")

	// "uint16" token.
	UInt16Token = NewTypeToken("uint16")

	// "uint32" token.
	UInt32Token = NewTypeToken("uint32")

	// "uint64" token.
	UInt64Token = NewTypeToken("uint64")

	// "uint" token.
	UIntToken = NewTypeToken("uint")

	// "uint8" token.
	UInt8Token = NewTypeToken("uint8")

	// "void" token.
	VoidToken = NewTypeToken("void")

	// "int" token.
	IntToken = NewTypeToken("int")

	// "int32"  token.
	Int32Token = NewTypeToken("int32")

	// "int64" token.
	Int64Token = NewTypeToken("int64")

	// "interface"  token.
	InterfaceToken = NewIdentifierToken("interface")

	// "import" token.
	ImportToken = NewReservedToken("import")

	// "make" token.
	MakeToken = NewReservedToken("make")

	// "map" token.
	MapToken = NewTypeToken("map")

	// "nil" token.
	NilToken = NewReservedToken("nil")

	// "package" token.
	PackageToken = NewReservedToken("package")

	// "panic" token.
	PanicToken = NewReservedToken("panic")

	// "print" token.
	PrintToken = NewReservedToken("print")

	// "recover" token.
	RecoverToken = NewReservedToken("recover")

	// "range" token.
	RangeToken = NewIdentifierToken("range")

	// "return" token.
	ReturnToken = NewReservedToken("return")

	// "string" token.
	StringToken = NewTypeToken("string")

	// "struct" token.
	StructToken = NewTypeToken("struct")

	// "switch" token.
	SwitchToken = NewReservedToken("switch")

	// "test" token.
	TestToken = NewIdentifierToken("test")

	// "type" token.
	TypeToken = NewReservedToken("type")

	// "try" token.
	TryToken = NewReservedToken("try")

	// "var" token.
	VarToken = NewReservedToken("var")

	// "when" token.
	WhenToken = NewIdentifierToken("when")

	// ";" token.
	SemicolonToken = NewSpecialToken(";")

	// ":" token.
	ColonToken = NewSpecialToken(":")

	// ":=" token.
	DefineToken = NewSpecialToken(":=")

	// "=" token.
	AssignToken = NewSpecialToken("=")

	// "," token.
	CommaToken = NewSpecialToken(",")

	// "==" token.
	EqualsToken = NewSpecialToken("==")

	// ">" token.
	GreaterThanToken = NewSpecialToken(">")

	// ">=" token.
	GreaterThanOrEqualsToken = NewSpecialToken(">=")

	// "<" token.
	LessThanToken = NewSpecialToken("<")

	// "<=" token.
	LessThanOrEqualsToken = NewSpecialToken("<=")

	// "<<" token.
	ShiftLeftToken = NewSpecialToken("<<")

	// ">>" token.
	ShiftRightToken = NewSpecialToken(">>")

	// "!" token.
	NotToken = NewSpecialToken("!")

	// "!=" token.
	NotEqualsToken = NewSpecialToken("!=")

	// "%" token.
	ModuloToken = NewSpecialToken("%")

	// "^" token.
	ExponentToken = NewSpecialToken("^")

	// "+" token.
	AddToken = NewSpecialToken("+")

	// "-" token.
	SubtractToken = NewSpecialToken("-")

	// "*" token.
	MultiplyToken = NewSpecialToken("*")

	// "/" token.
	DivideToken = NewSpecialToken("/")

	// "*" token.
	PointerToken = NewSpecialToken("*")

	// '&" token.
	AddressToken = NewSpecialToken("&")

	// "&" token.
	AndToken = NewSpecialToken("&")

	// "|" token.
	OrToken = NewSpecialToken("|")

	// "&&" token.
	BooleanAndToken = NewSpecialToken("&&")

	// "||" token.
	BooleanOrToken = NewSpecialToken("||")

	// "+=" token.
	AddAssignToken = NewSpecialToken("+=")

	// "-=" token.
	SubtractAssignToken = NewSpecialToken("-=")

	// "*=" token.
	MultiplyAssignToken = NewSpecialToken("*=")

	// "/=" token.
	DivideAssignToken = NewSpecialToken("/=")

	// "++" token.
	IncrementToken = NewSpecialToken("++")

	// "--" token.
	DecrementToken = NewSpecialToken("--")

	// "." token.
	DotToken = NewSpecialToken(".")

	// "..." token.
	VariadicToken = NewSpecialToken("...")

	// "<-" token.
	ChannelReceiveToken = NewSpecialToken("<-")

	// "(" token.
	StartOfListToken = NewSpecialToken("(")

	// ")"" token.
	EndOfListToken = NewSpecialToken(")")

	// "[" token.
	StartOfArrayToken = NewSpecialToken("[")

	// "]" token.
	EndOfArrayToken = NewSpecialToken("]")

	// "?" token.
	OptionalToken = NewSpecialToken("?")

	// Empty token.
	EmptyToken = NewSpecialToken("")

	// "-" token.
	NegateToken = NewSpecialToken("-")

	// "true" token.
	TrueToken = NewToken(BooleanTokenClass, "true")

	// "false" token.
	FalseToken = NewToken(BooleanTokenClass, "false")
)

// TypeTokens is a list of tokens that represent built-in type names.
var TypeTokens = map[Token]bool{
	AnyToken:     true,
	BoolToken:    true,
	ByteToken:    true,
	Int8Token:    true,
	Int16Token:   true,
	UInt16Token:  true,
	Int32Token:   true,
	UInt32Token:  true,
	UInt64Token:  true,
	IntToken:     true,
	Int32Token:   true,
	Int64Token:   true,
	Float32Token: true,
	Float64Token: true,
	StringToken:  true,
	StructToken:  true,
	MapToken:     true,
}

// SpecialTokens is a list of tokens that are considered special semantic characters.
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
	BoolToken:        true,
	BreakToken:       true,
	ByteToken:        true,
	ChanToken:        true,
	ConstToken:       true,
	ContinueToken:    true,
	DeferToken:       true,
	ElseToken:        true,
	FallthroughToken: true,
	Float32Token:     true,
	Float64Token:     true,
	ForToken:         true,
	FuncToken:        true,
	GoToken:          true,
	IfToken:          true,
	ImportToken:      true,
	InterfaceToken:   true,
	IntToken:         true,
	Int32Token:       true,
	Int64Token:       true,
	MapToken:         true,
	NilToken:         true,
	PackageToken:     true,
	PanicToken:       true,
	RecoverToken:     true,
	ReturnToken:      true,
	SwitchToken:      true,
	StringToken:      true,
	StructToken:      true,
	TypeToken:        true,
	VarToken:         true,
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

// reservedIdentifiers lists the reserved words that are also permitted to appear
// in identifier positions. For example, "type" is a reserved word that begins a
// type declaration, but it can also appear as a field name or a function
// argument. Tokens whose spelling appears in this map will return true from
// IsIdentifier() even though their class is ReservedTokenClass.
var reservedIdentifiers = map[Token]bool{
	MakeToken:      true,
	TypeToken:      true,
	InterfaceToken: true,
}

// IsReserved indicates if a name is a reserved word. The flag parameter
// indicates if the extended reserved words should be considered.
func (t Token) IsReserved(includeExtensions bool) bool {
	_, reserved := ReservedWords[t]

	if !reserved && includeExtensions {
		_, extendedReserved := ExtendedReservedWords[t]
		reserved = reserved || extendedReserved
	}

	return reserved
}
