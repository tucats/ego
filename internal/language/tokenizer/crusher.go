package tokenizer

// This describes a token that is "crushed"; that is converting a sequence of individual tokens as generated
// by the standard Go scanner into a single Ego token where appropriate. For example, when scanning "x<=5",
// the Go scanner generates four tokens: Identifier("x"), Special("<"), Special("=") and IntegerToken("5").
// However, Ego wants to see the "<=" as a single token, so this table maps Special("<"), Special("=") into
// a single token, Special("<=").
//
// The "crush" operation is done as tokens are accumulated, when the lexer has finished scanning, the crushedTokens
// map is used to determine if sequential tokens should be merged into a single token. This is done only once
// for a given token stream.
type crushedToken struct {
	// Array of tokens that together will be "crushed" into a single token.
	source []Token
	// The resulting token after a crush operation.
	result Token

	// Flag indicating that this crush operation requires that the tokens
	// be adjacent (no intervening spaces) to be crushed. For example,
	// "x < -5" would not be crushed to x <- 5, but "{  }" would crush to "{}"
	adjacent bool
}

// This is the table of tokens that are "crushed" into a single token.
var crushedTokens = []crushedToken{
	{
		source:   []Token{AddToken, AssignToken},
		result:   AddAssignToken,
		adjacent: true,
	},
	{
		source:   []Token{SubtractToken, AssignToken},
		result:   SubtractAssignToken,
		adjacent: true,
	},
	{
		source:   []Token{MultiplyToken, AssignToken},
		result:   MultiplyAssignToken,
		adjacent: true,
	},
	{
		source:   []Token{DivideToken, AssignToken},
		result:   DivideAssignToken,
		adjacent: true,
	},
	{
		source:   []Token{AddToken, AddToken},
		result:   IncrementToken,
		adjacent: true,
	},
	{
		source:   []Token{SubtractToken, SubtractToken},
		result:   DecrementToken,
		adjacent: true,
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
		source:   []Token{LessThanToken, SubtractToken},
		result:   ChannelReceiveToken,
		adjacent: true,
	},
	{
		source:   []Token{GreaterThanToken, AssignToken},
		result:   GreaterThanOrEqualsToken,
		adjacent: true,
	},
	{
		source: []Token{LessThanToken, AssignToken},
		result: LessThanOrEqualsToken,
	},
	{
		source:   []Token{AssignToken, AssignToken},
		result:   EqualsToken,
		adjacent: true,
	},
	{
		source:   []Token{NotToken, AssignToken},
		result:   NotEqualsToken,
		adjacent: true,
	},
	{
		source:   []Token{ColonToken, AssignToken},
		result:   DefineToken,
		adjacent: true,
	},
	{
		source:   []Token{AndToken, AndToken},
		result:   BooleanAndToken,
		adjacent: true,
	},
	{
		source:   []Token{OrToken, OrToken},
		result:   BooleanOrToken,
		adjacent: true,
	},
	{
		source:   []Token{LessThanToken, LessThanToken},
		result:   ShiftLeftToken,
		adjacent: true,
	},
	{
		source:   []Token{GreaterThanToken, GreaterThanToken},
		result:   ShiftRightToken,
		adjacent: true,
	},
}
