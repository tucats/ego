package tokenizer

// This describes a token that is "crushed"; that is converting a sequence of individual tokens as generated
// by the standard Go scanner into a single Ego token where appropriate. For example, when scanning "x<=5",
// the Go scanner generates four tokens: Identifier("x"), Special("<"), Special("=") and IntegerToken("5").
// However, Ego wants to see the "<=" as a single token, so this table maps Special("<"), Special("=") into
// a single token, Special("<=").
//
// The "crust" operation is done as tokens are accumulated, when the lexer has finished scanning, the crushedTokens
// map is used to determine if sequential tokens should be merged into a single token.
type crushedToken struct {
	source []Token
	result Token
}

// This is the table of tokens that are "crushed" into a single token.
var crushedTokens = []crushedToken{
	{
		source: []Token{AddToken, AssignToken},
		result: AddAssignToken,
	},
	{
		source: []Token{SubtractToken, AssignToken},
		result: SubtractAssignToken,
	},
	{
		source: []Token{MultiplyToken, AssignToken},
		result: MultiplyAssignToken,
	},
	{
		source: []Token{DivideToken, AssignToken},
		result: DivideAssignToken,
	},
	{
		source: []Token{AddToken, AddToken},
		result: IncrementToken,
	},
	{
		source: []Token{SubtractToken, SubtractToken},
		result: DecrementToken,
	},
	{
		source: []Token{InterfaceToken, DataBeginToken, DataEndToken},
		result: EmptyInterfaceToken,
	},
	{
		source: []Token{NewIdentifierToken(InterfaceToken.spelling), DataBeginToken, DataEndToken},
		result: EmptyInterfaceToken,
	},
	{
		source: []Token{BlockBeginToken, BlockEndToken},
		result: EmptyBlockToken,
	},
	{
		source: []Token{DotToken, DotToken, DotToken},
		result: VariadicToken,
	},
	{
		source: []Token{LessThanToken, SubtractToken},
		result: ChannelReceiveToken,
	},
	{
		source: []Token{GreaterThanToken, AssignToken},
		result: GreaterThanOrEqualsToken,
	},
	{
		source: []Token{LessThanToken, AssignToken},
		result: LessThanOrEqualsToken,
	},
	{
		source: []Token{AssignToken, AssignToken},
		result: EqualsToken,
	},
	{
		source: []Token{NotToken, AssignToken},
		result: NotEqualsToken,
	},
	{
		source: []Token{ColonToken, AssignToken},
		result: DefineToken,
	},
	{
		source: []Token{AndToken, AndToken},
		result: BooleanAndToken,
	},
	{
		source: []Token{OrToken, OrToken},
		result: BooleanOrToken,
	},
	{
		source: []Token{LessThanToken, LessThanToken},
		result: ShiftLeftToken,
	},
	{
		source: []Token{GreaterThanToken, GreaterThanToken},
		result: ShiftRightToken,
	},
}
