package tokenizer

import (
	"strings"
	"testing"
)

// =============================================================================
// Token constructor tests
// =============================================================================

func TestNewTokenConstructors(t *testing.T) {
	tests := []struct {
		name      string
		tok       Token
		wantClass TokenClass
		wantSpell string
	}{
		{"NewToken identifier", NewToken(IdentifierTokenClass, "foo"), IdentifierTokenClass, "foo"},
		{"NewToken integer", NewToken(IntegerTokenClass, "99"), IntegerTokenClass, "99"},
		{"NewFloatToken", NewFloatToken("3.14"), FloatTokenClass, "3.14"},
		{"NewIdentifierToken", NewIdentifierToken("bar"), IdentifierTokenClass, "bar"},
		{"NewTypeToken", NewTypeToken("int"), TypeTokenClass, "int"},
		{"NewStringToken", NewStringToken("hello"), StringTokenClass, "hello"},
		{"NewReservedToken", NewReservedToken("var"), ReservedTokenClass, "var"},
		{"NewSpecialToken", NewSpecialToken("+"), SpecialTokenClass, "+"},
		{"NewIntegerToken", NewIntegerToken("42"), IntegerTokenClass, "42"},
		{"empty spelling", NewIdentifierToken(""), IdentifierTokenClass, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.tok.Class() != tt.wantClass {
				t.Errorf("Class() = %v, want %v", tt.tok.Class(), tt.wantClass)
			}

			if tt.tok.Spelling() != tt.wantSpell {
				t.Errorf("Spelling() = %q, want %q", tt.tok.Spelling(), tt.wantSpell)
			}
		})
	}
}

// =============================================================================
// Token.Location
// =============================================================================

func TestToken_Location_ManuallyCreated(t *testing.T) {
	tok := NewIdentifierToken("foo")

	line, pos := tok.Location()
	if line != 0 || pos != 0 {
		t.Errorf("Location() = (%d, %d), want (0, 0) for manually created token", line, pos)
	}
}

func TestToken_Location_FromTokenizer(t *testing.T) {
	tk := New("foo bar", false)

	l, p := tk.Tokens[0].Location()
	if l != 1 {
		t.Errorf("first token line = %d, want 1", l)
	}

	if p <= 0 {
		t.Errorf("first token col = %d, want > 0", p)
	}
}

// =============================================================================
// Token.IsType
// =============================================================================

func TestToken_IsType(t *testing.T) {
	tests := []struct {
		name string
		tok  Token
		want bool
	}{
		{"int type", IntToken, true},
		{"string type", StringToken, true},
		{"bool type", BoolToken, true},
		{"float64 type", Float64Token, true},
		{"identifier", NewIdentifierToken("foo"), false},
		{"reserved word", VarToken, false},
		{"special char", AddToken, false},
		{"integer literal", NewIntegerToken("42"), false},
		{"float literal", NewFloatToken("3.14"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.tok.IsType(); got != tt.want {
				t.Errorf("IsType() = %v, want %v", got, tt.want)
			}
		})
	}
}

// =============================================================================
// Token.IsName
// =============================================================================

func TestToken_IsName(t *testing.T) {
	tests := []struct {
		name string
		tok  Token
		want bool
	}{
		{"identifier", NewIdentifierToken("foo"), true},
		{"reserved word", VarToken, true},
		{"type token", IntToken, false},
		{"special char", AddToken, false},
		{"integer literal", NewIntegerToken("42"), false},
		{"string literal", NewStringToken("hi"), false},
		{"float literal", NewFloatToken("1.0"), false},
		{"end of tokens", EndOfTokens, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.tok.IsName(); got != tt.want {
				t.Errorf("IsName() = %v, want %v", got, tt.want)
			}
		})
	}
}

// =============================================================================
// Token.IsClass
// =============================================================================

func TestToken_IsClass(t *testing.T) {
	tests := []struct {
		name  string
		tok   Token
		class TokenClass
		want  bool
	}{
		{"identifier matches", NewIdentifierToken("x"), IdentifierTokenClass, true},
		{"identifier vs type", NewIdentifierToken("x"), TypeTokenClass, false},
		{"type matches", IntToken, TypeTokenClass, true},
		{"reserved matches", VarToken, ReservedTokenClass, true},
		{"reserved vs identifier", VarToken, IdentifierTokenClass, false},
		{"special matches", AddToken, SpecialTokenClass, true},
		{"float matches", NewFloatToken("3.14"), FloatTokenClass, true},
		{"integer matches", NewIntegerToken("5"), IntegerTokenClass, true},
		{"string matches", NewStringToken("hi"), StringTokenClass, true},
		{"end-of-tokens class", EndOfTokens, EndOfTokensClass, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.tok.IsClass(tt.class); got != tt.want {
				t.Errorf("IsClass(%v) = %v, want %v", tt.class, got, tt.want)
			}
		})
	}
}

// =============================================================================
// Token.Is and Token.IsNot
// =============================================================================

func TestToken_Is_And_IsNot(t *testing.T) {
	tests := []struct {
		name string
		tok  Token
		test Token
		want bool
	}{
		{"same token", AddToken, AddToken, true},
		{"same class different spelling", AddToken, SubtractToken, false},
		{"same spelling different class", NewIdentifierToken("var"), VarToken, false},
		{"integer match", NewIntegerToken("42"), NewIntegerToken("42"), true},
		{"integer mismatch value", NewIntegerToken("42"), NewIntegerToken("43"), false},
		{"string match", NewStringToken("hi"), NewStringToken("hi"), true},
		{"string mismatch value", NewStringToken("hi"), NewStringToken("there"), false},
		{"EndOfTokens self", EndOfTokens, EndOfTokens, true},
		{"float match", NewFloatToken("1.5"), NewFloatToken("1.5"), true},
		{"float mismatch", NewFloatToken("1.5"), NewFloatToken("2.5"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.tok.Is(tt.test); got != tt.want {
				t.Errorf("Is() = %v, want %v", got, tt.want)
			}

			if got := tt.tok.IsNot(tt.test); got != !tt.want {
				t.Errorf("IsNot() = %v, want %v", got, !tt.want)
			}
		})
	}
}

// =============================================================================
// Token.IsIdentifier
// =============================================================================

func TestToken_IsIdentifier(t *testing.T) {
	tests := []struct {
		name string
		tok  Token
		want bool
	}{
		{"regular identifier", NewIdentifierToken("foo"), true},
		{"int type (can be identifier)", IntToken, true},
		{"string type (can be identifier)", StringToken, true},
		{"make (reserved, but in reservedIdentifiers)", MakeToken, true},
		{"type keyword (in reservedIdentifiers)", TypeToken, true},
		{"var (reserved, not in reservedIdentifiers)", VarToken, false},
		{"for (reserved, not in reservedIdentifiers)", ForToken, false},
		{"if (reserved, not in reservedIdentifiers)", IfToken, false},
		{"special char", AddToken, false},
		{"integer literal", NewIntegerToken("42"), false},
		{"string literal", NewStringToken("hello"), false},
		{"float literal", NewFloatToken("3.14"), false},
		{"end of tokens", EndOfTokens, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.tok.IsIdentifier(); got != tt.want {
				t.Errorf("IsIdentifier() = %v, want %v", got, tt.want)
			}
		})
	}
}

// =============================================================================
// Token.IsValue
// =============================================================================

func TestToken_IsValue(t *testing.T) {
	tests := []struct {
		name string
		tok  Token
		want bool
	}{
		{"string literal", NewStringToken("hello"), true},
		{"integer literal", NewIntegerToken("42"), true},
		{"value class token", NewToken(ValueTokenClass, "x"), true},
		{"identifier", NewIdentifierToken("foo"), false},
		{"reserved word", VarToken, false},
		{"float literal", NewFloatToken("3.14"), true},
		{"boolean token", TrueToken, true},
		{"type token", IntToken, false},
		{"special char", AddToken, false},
		{"end of tokens", EndOfTokens, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.tok.IsValue(); got != tt.want {
				t.Errorf("IsValue() = %v, want %v", got, tt.want)
			}
		})
	}
}

// =============================================================================
// Token.IsString
// =============================================================================

func TestToken_IsString(t *testing.T) {
	tests := []struct {
		name string
		tok  Token
		want bool
	}{
		{"string token", NewStringToken("hello"), true},
		{"empty string token", NewStringToken(""), true},
		{"identifier", NewIdentifierToken("foo"), false},
		{"integer", NewIntegerToken("42"), false},
		{"float", NewFloatToken("3.14"), false},
		{"reserved", VarToken, false},
		{"special", AddToken, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.tok.IsString(); got != tt.want {
				t.Errorf("IsString() = %v, want %v", got, tt.want)
			}
		})
	}
}

// =============================================================================
// Token.String (debug representation)
// =============================================================================

func TestToken_String(t *testing.T) {
	tests := []struct {
		name        string
		tok         Token
		wantContain []string
	}{
		{"identifier token", NewIdentifierToken("foo"), []string{"Identifier", "foo"}},
		{"reserved token", VarToken, []string{"Reserved", "var"}},
		{"special token", AddToken, []string{"Special", "+"}},
		{"integer token", NewIntegerToken("99"), []string{"Integer", "99"}},
		{"float token", NewFloatToken("3.14"), []string{"Float", "3.14"}},
		{"string token", NewStringToken("hi"), []string{"String", "hi"}},
		{"type token", IntToken, []string{"Type", "int"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := tt.tok.String()
			for _, want := range tt.wantContain {
				if !strings.Contains(s, want) {
					t.Errorf("String() = %q, expected to contain %q", s, want)
				}
			}
		})
	}
}

// =============================================================================
// Token.Integer
// =============================================================================

func TestToken_Integer(t *testing.T) {
	tests := []struct {
		name string
		tok  Token
		want int64
	}{
		{"valid integer", NewIntegerToken("42"), 42},
		{"zero", NewIntegerToken("0"), 0},
		{"large value", NewIntegerToken("9999999999"), 9999999999},
		{"non-numeric spelling", NewIntegerToken("notANumber"), 0},
		{"string token", NewStringToken("hello"), 0},
		{"identifier", NewIdentifierToken("x"), 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.tok.Integer(); got != tt.want {
				t.Errorf("Integer() = %v, want %v", got, tt.want)
			}
		})
	}
}

// =============================================================================
// Token.Float
// =============================================================================

func TestToken_Float(t *testing.T) {
	tests := []struct {
		name string
		tok  Token
		want float64
	}{
		{"float token", NewFloatToken("3.14"), 3.14},
		{"negative float", NewFloatToken("-1.5"), -1.5},
		{"zero float", NewFloatToken("0.0"), 0.0},
		{"non-numeric token", NewStringToken("hello"), 0.0},
		{"identifier", NewIdentifierToken("x"), 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.tok.Float(); got != tt.want {
				t.Errorf("Float() = %v, want %v", got, tt.want)
			}
		})
	}
}

// =============================================================================
// Token.Boolean
// =============================================================================

func TestToken_Boolean(t *testing.T) {
	tests := []struct {
		name string
		tok  Token
		want bool
	}{
		{"TrueToken", TrueToken, true},
		{"FalseToken", FalseToken, false},
		{"string 'true'", NewStringToken("true"), true},
		{"string 'false'", NewStringToken("false"), false},
		{"identifier", NewIdentifierToken("x"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.tok.Boolean(); got != tt.want {
				t.Errorf("Boolean() = %v, want %v", got, tt.want)
			}
		})
	}
}

// =============================================================================
// Token.Spelling and Token.Class
// =============================================================================

func TestToken_SpellingAndClass(t *testing.T) {
	tok := NewIntegerToken("99")
	if tok.Spelling() != "99" {
		t.Errorf("Spelling() = %q, want %q", tok.Spelling(), "99")
	}

	if tok.Class() != IntegerTokenClass {
		t.Errorf("Class() = %v, want IntegerTokenClass", tok.Class())
	}

	tok2 := VarToken
	if tok2.Spelling() != "var" {
		t.Errorf("Spelling() = %q, want %q", tok2.Spelling(), "var")
	}

	if tok2.Class() != ReservedTokenClass {
		t.Errorf("Class() = %v, want ReservedTokenClass", tok2.Class())
	}
}

// =============================================================================
// TokenClass.String
// =============================================================================

func TestTokenClass_String(t *testing.T) {
	tests := []struct {
		class TokenClass
		want  string
	}{
		{EndOfTokensClass, "<end-of-tokens>"},
		{IdentifierTokenClass, "Identifier"},
		{TypeTokenClass, "Type"},
		{StringTokenClass, "String"},
		{BooleanTokenClass, "Boolean"},
		{IntegerTokenClass, "Integer"},
		{FloatTokenClass, "Float"},
		{ReservedTokenClass, "Reserved"},
		{SpecialTokenClass, "Special"},
		{ValueTokenClass, "Value"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.class.String(); got != tt.want {
				t.Errorf("TokenClass.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

// =============================================================================
// Tokenizer utility tests
// =============================================================================

func TestTokenizer_Len(t *testing.T) {
	tk := New("a b c", false)
	if tk.Len() != 3 {
		t.Errorf("Len() = %d, want 3", tk.Len())
	}

	empty := New("", false)
	if empty.Len() != 0 {
		t.Errorf("Len() for empty source = %d, want 0", empty.Len())
	}
}

// TestTokenizer_NewToken_IgnoresClass documents a bug: Tokenizer.NewToken() ignores the
// class argument and always produces a ValueTokenClass token.
func TestTokenizer_NewToken_IgnoresClass(t *testing.T) {
	tk := New("x", false)
	tok := tk.NewToken(IdentifierTokenClass, "myIdent")

	// The spelling should be preserved.
	if tok.Spelling() != "myIdent" {
		t.Errorf("Tokenizer.NewToken() spelling = %q, want %q", tok.Spelling(), "myIdent")
	}

	if tok.Class() != IdentifierTokenClass {
		t.Errorf("Tokenizer.NewToken() class = %v; expected IdentifierTokenClass", tok.Class())
	}
}

func TestTokenizer_GetTokens(t *testing.T) {
	const compressedTokens = "abc"

	tk := New("a b c", false)
	// 3 tokens: "a", "b", "c"

	if got := tk.GetTokens(0, 3, false); got != compressedTokens {
		t.Errorf("GetTokens(0,3,false) = %q, want %q", got, compressedTokens)
	}

	if got := tk.GetTokens(0, 3, true); got != "a b c " {
		t.Errorf("GetTokens(0,3,true) = %q, want %q", got, "a b c ")
	}

	// Negative p1 clamps to 0.
	if got := tk.GetTokens(-5, 3, false); got != compressedTokens {
		t.Errorf("GetTokens(-5,3,false) = %q, want %q", got, compressedTokens)
	}

	// p2 > len(Tokens) clamps to len(Tokens).
	if got := tk.GetTokens(0, 999, false); got != compressedTokens {
		t.Errorf("GetTokens(0,999,false) = %q, want %q", got, compressedTokens)
	}

	// p2 < p1: p2 is set to p1, producing empty output (range p1:p1).
	if got := tk.GetTokens(2, 1, false); got != "" {
		t.Errorf("GetTokens(2,1,false) = %q, want %q (empty for inverted range)", got, "")
	}

	// Partial range.
	if got := tk.GetTokens(1, 2, false); got != "b" {
		t.Errorf("GetTokens(1,2,false) = %q, want %q", got, "b")
	}
}

func TestInList(t *testing.T) {
	tests := []struct {
		name string
		tok  Token
		list []Token
		want bool
	}{
		{"found in list", AddToken, []Token{AddToken, SubtractToken, MultiplyToken}, true},
		{"not in list", DivideToken, []Token{AddToken, SubtractToken, MultiplyToken}, false},
		{"empty list", AddToken, []Token{}, false},
		{"single element match", VarToken, []Token{VarToken}, true},
		// Same spelling but different class should NOT match.
		{"same spelling different class", NewIdentifierToken("var"), []Token{VarToken}, false},
		{"reserved vs type", VarToken, []Token{IntToken, ForToken, IfToken}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := InList(tt.tok, tt.list...); got != tt.want {
				t.Errorf("InList() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTokenizer_Close(t *testing.T) {
	tk := New("a b c", false)
	if tk.Len() == 0 {
		t.Fatal("expected tokens before Close()")
	}

	tk.Close()

	if tk.Len() != 0 {
		t.Errorf("Len() after Close() = %d, want 0", tk.Len())
	}

	// Calling Close() again must be safe (sync.Once ensures idempotency).
	tk.Close()

	if tk.Len() != 0 {
		t.Errorf("Len() after second Close() = %d, want 0", tk.Len())
	}
}

func TestTokenizer_StringMethod(t *testing.T) {
	tk := New("a b c", false)

	s := tk.String()
	if !strings.Contains(s, "Tokenizer") {
		t.Errorf("String() = %q, expected to contain 'Tokenizer'", s)
	}
}

// =============================================================================
// Cursor / navigation tests
// =============================================================================

func TestTokenizer_Next(t *testing.T) {
	tk := New("x y", false)

	tok := tk.Next()
	if tok.IsNot(NewIdentifierToken("x")) {
		t.Errorf("1st Next() = %v, want identifier 'x'", tok)
	}

	tok = tk.Next()
	if tok.IsNot(NewIdentifierToken("y")) {
		t.Errorf("2nd Next() = %v, want identifier 'y'", tok)
	}

	// Exhausted: every subsequent call must return EndOfTokens.
	tok = tk.Next()
	if tok.IsNot(EndOfTokens) {
		t.Errorf("Next() at end = %v, want EndOfTokens", tok)
	}

	tok = tk.Next()
	if tok.IsNot(EndOfTokens) {
		t.Errorf("Next() past end = %v, want EndOfTokens", tok)
	}
}

func TestTokenizer_NextText(t *testing.T) {
	tk := New("hello world", false)

	if got := tk.NextText(); got != "hello" {
		t.Errorf("NextText() = %q, want %q", got, "hello")
	}

	if got := tk.NextText(); got != "world" {
		t.Errorf("NextText() = %q, want %q", got, "world")
	}
	// At end returns EndOfTokens spelling (empty string).
	if got := tk.NextText(); got != "" {
		t.Errorf("NextText() at end = %q, want %q", got, "")
	}
}

func TestTokenizer_Peek(t *testing.T) {
	tk := New("a b c", false)
	// TokenP starts at 0.

	// Peek(1) → Tokens[TokenP + 0] = Tokens[0] = "a"
	if got := tk.Peek(1); got.IsNot(NewIdentifierToken("a")) {
		t.Errorf("Peek(1) at start = %v, want 'a'", got)
	}

	// Peek(2) → Tokens[1] = "b"
	if got := tk.Peek(2); got.IsNot(NewIdentifierToken("b")) {
		t.Errorf("Peek(2) at start = %v, want 'b'", got)
	}

	// Peek(0) → Tokens[-1]: out of bounds → EndOfTokens
	if got := tk.Peek(0); got.IsNot(EndOfTokens) {
		t.Errorf("Peek(0) at start = %v, want EndOfTokens (before first token)", got)
	}

	// Peek with huge offset → EndOfTokens
	if got := tk.Peek(999); got.IsNot(EndOfTokens) {
		t.Errorf("Peek(999) = %v, want EndOfTokens", got)
	}

	// After consuming "a", Peek(1) → "b" and Peek(0) → "a"
	tk.Next()

	if got := tk.Peek(1); got.IsNot(NewIdentifierToken("b")) {
		t.Errorf("after Next, Peek(1) = %v, want 'b'", got)
	}

	if got := tk.Peek(0); got.IsNot(NewIdentifierToken("a")) {
		t.Errorf("after Next, Peek(0) = %v, want 'a' (look-behind)", got)
	}
}

func TestTokenizer_PeekText(t *testing.T) {
	tk := New("hello world", false)

	if got := tk.PeekText(1); got != "hello" {
		t.Errorf("PeekText(1) = %q, want %q", got, "hello")
	}

	if got := tk.PeekText(2); got != "world" {
		t.Errorf("PeekText(2) = %q, want %q", got, "world")
	}
	// Past end → empty string (EndOfTokens spelling).
	if got := tk.PeekText(999); got != "" {
		t.Errorf("PeekText(999) = %q, want %q", got, "")
	}
}

// TestTokenizer_PeekText_NegativeOffset verifies that PeekText correctly handles
// offsets that produce a negative computed index (previously caused a panic due to
// a missing lower-bounds check, now fixed).
func TestTokenizer_PeekText_NegativeOffset(t *testing.T) {
	tk := New("hello", false)
	// TokenP == 0; offset 0 → pos = 0 + (0-1) = -1 (below start).

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("PeekText(0) panicked with TokenP=0 (lower-bounds check regression): %v", r)
		}
	}()

	got := tk.PeekText(0) // negative index → EndOfTokens spelling ("")
	if got != "" {
		t.Errorf("PeekText(0) = %q, want %q (EndOfTokens spelling)", got, "")
	}
}

func TestTokenizer_AtEnd(t *testing.T) {
	tk := New("x", false)

	if tk.AtEnd() {
		t.Error("AtEnd() = true at start, want false")
	}

	tk.Next() // consume "x"

	if !tk.AtEnd() {
		t.Error("AtEnd() = false after consuming all tokens, want true")
	}
}

func TestTokenizer_AtEnd_EmptySource(t *testing.T) {
	tk := New("", false)
	if !tk.AtEnd() {
		t.Error("AtEnd() on empty tokenizer = false, want true")
	}
}

func TestTokenizer_Advance(t *testing.T) {
	tk := New("a b c d e", false)
	// 5 tokens, TokenP starts at 0.

	tk.Advance(3)

	if tk.TokenP != 3 {
		t.Errorf("after Advance(3), TokenP = %d, want 3", tk.TokenP)
	}

	// Clamping: advance far past end.
	tk.Advance(1000)

	if tk.TokenP != len(tk.Tokens) {
		t.Errorf("after Advance(1000), TokenP = %d, want %d (clamped to end)", tk.TokenP, len(tk.Tokens))
	}

	// Clamping: advance far before start.
	tk.Advance(-1000)

	if tk.TokenP != 0 {
		t.Errorf("after Advance(-1000), TokenP = %d, want 0 (clamped to start)", tk.TokenP)
	}

	// Advance(0) is a no-op.
	tk.Advance(2)
	before := tk.TokenP

	tk.Advance(0)

	if tk.TokenP != before {
		t.Errorf("Advance(0) changed TokenP from %d to %d", before, tk.TokenP)
	}

	// ToTheEnd should move to the end.
	tk.Reset()
	tk.Advance(ToTheEnd)

	if tk.TokenP != len(tk.Tokens) {
		t.Errorf("Advance(ToTheEnd) TokenP = %d, want %d", tk.TokenP, len(tk.Tokens))
	}
}

func TestTokenizer_IsNext(t *testing.T) {
	tk := New("var x int", false)
	// Tokens: VarToken, "x", IntToken, ";"

	if !tk.IsNext(VarToken) {
		t.Error("IsNext(VarToken) = false, want true")
	}
	// Position advanced past "var"; "x" is next.
	if !tk.IsNext(NewIdentifierToken("x")) {
		t.Error("IsNext(x) = false, want true")
	}
	// "int" is next, checking wrong token must NOT advance.
	if tk.IsNext(VarToken) {
		t.Error("IsNext(VarToken) after consuming 'x' = true, want false (should not advance)")
	}
	// Position unchanged; "int" is still next.
	if !tk.IsNext(IntToken) {
		t.Error("IsNext(IntToken) = false, want true")
	}
}

func TestTokenizer_AnyNext(t *testing.T) {
	tk := New("a + b", false)
	// Tokens: "a", "+", "b"

	// "a" matches one of the candidates.
	if !tk.AnyNext(AddToken, NewIdentifierToken("a")) {
		t.Error("AnyNext() = false, want true for identifier 'a'")
	}
	// "+" matches AddToken.
	if !tk.AnyNext(SubtractToken, AddToken) {
		t.Error("AnyNext() = false, want true for '+'")
	}
	// "b" does not match [VarToken, ForToken]; position must stay.
	if tk.AnyNext(VarToken, ForToken) {
		t.Error("AnyNext() = true for non-matching tokens, want false (should not advance)")
	}
	// "b" is still next.
	if !tk.AnyNext(NewIdentifierToken("b")) {
		t.Error("AnyNext() = false after non-match; 'b' should still be next")
	}
}

func TestTokenizer_AnyNext_EmptyList(t *testing.T) {
	tk := New("x", false)
	if tk.AnyNext() {
		t.Error("AnyNext() with no candidates = true, want false")
	}
}

func TestTokenizer_EndOfStatement(t *testing.T) {
	// isCode=true inserts semicolons, giving us: "x", ";", and then end-of-tokens.
	tk := New("x", true)

	if tk.EndOfStatement() {
		t.Error("EndOfStatement() at start (before 'x') = true, want false")
	}

	tk.Next() // consume "x"

	// Next token is ";", so we are at end-of-statement.
	if !tk.EndOfStatement() {
		t.Error("EndOfStatement() before semicolon = false, want true")
	}

	tk.Next() // consume ";"

	// Past the last token: AtEnd() is true.
	if !tk.EndOfStatement() {
		t.Error("EndOfStatement() at end-of-tokens = false, want true")
	}
}

func TestTokenizer_CurrentLine(t *testing.T) {
	tk := New("foo bar", false)

	// Before advancing: CurrentLine returns 0 when TokenP == 0.
	if line := tk.CurrentLine(); line != 0 {
		t.Errorf("CurrentLine() at TokenP=0 = %d, want 0", line)
	}

	tk.Next() // advance to TokenP=1

	if line := tk.CurrentLine(); line <= 0 {
		t.Errorf("CurrentLine() after one Next() = %d, want > 0", line)
	}

	// At end: CurrentLine returns 0.
	tk.Advance(ToTheEnd)

	if line := tk.CurrentLine(); line != 0 {
		t.Errorf("CurrentLine() at end = %d, want 0", line)
	}
}

// TestTokenizer_CurrentColumn_BoundsCheck verifies that CurrentColumn() does not
// panic when TokenP >= len(Tokens) (previously missing upper-bounds guard, now fixed).
func TestTokenizer_CurrentColumn_BoundsCheck(t *testing.T) {
	tk := New("foo bar", false)

	// Should return 0 when TokenP == 0.
	if col := tk.CurrentColumn(); col != 0 {
		t.Errorf("CurrentColumn() at TokenP=0 = %d, want 0", col)
	}

	// Advance to end and confirm no panic (upper-bounds check was added).
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("CurrentColumn() panicked when TokenP >= len(Tokens): %v (upper-bounds check regression)", r)
		}
	}()

	tk.Advance(ToTheEnd)
	_ = tk.CurrentColumn()
}

// =============================================================================
// Mark / Set / Reset tests
// =============================================================================

func TestTokenizer_Mark(t *testing.T) {
	tk := New("a b c d", false)

	if mark := tk.Mark(); mark != 0 {
		t.Errorf("Mark() at start = %d, want 0", mark)
	}

	tk.Next()
	tk.Next()

	if mark := tk.Mark(); mark != 2 {
		t.Errorf("Mark() after two Next() calls = %d, want 2", mark)
	}
}

func TestTokenizer_Set(t *testing.T) {
	tk := New("a b c d", false)

	tk.Next()
	tk.Next()
	saved := tk.Mark() // 2

	tk.Next()
	tk.Next() // now at 4

	tk.Set(saved)

	if tk.TokenP != 2 {
		t.Errorf("Set(2) TokenP = %d, want 2", tk.TokenP)
	}

	// Negative clamps to 0.
	tk.Set(-99)

	if tk.TokenP != 0 {
		t.Errorf("Set(-99) TokenP = %d, want 0 (clamped)", tk.TokenP)
	}

	// Huge value clamps to len(Tokens).
	tk.Set(999999)

	if tk.TokenP != len(tk.Tokens) {
		t.Errorf("Set(999999) TokenP = %d, want %d (clamped)", tk.TokenP, len(tk.Tokens))
	}
}

func TestTokenizer_Reset(t *testing.T) {
	tk := New("a b c", false)

	tk.Next()
	tk.Next()

	if tk.TokenP == 0 {
		t.Fatal("expected TokenP > 0 before Reset")
	}

	tk.Reset()

	if tk.TokenP != 0 {
		t.Errorf("Reset() TokenP = %d, want 0", tk.TokenP)
	}

	// Resetting again from zero should be safe.
	tk.Reset()

	if tk.TokenP != 0 {
		t.Errorf("second Reset() TokenP = %d, want 0", tk.TokenP)
	}
}

func TestTokenizer_MarkSetRoundtrip(t *testing.T) {
	tk := New("one two three four", false)

	_ = tk.Next()      // consume "one"
	mark := tk.Mark()  // save at 1
	_ = tk.Next()      // consume "two"
	_ = tk.Next()      // consume "three"
	tok := tk.Peek(1)  // peek at "four"
	tk.Set(mark)       // restore to 1
	tok2 := tk.Peek(1) // peek should again be "two"

	if tok.Is(tok2) {
		t.Errorf("after Set(mark), Peek(1) = %v; expected 'two', not 'four'", tok2)
	}

	if tok2.IsNot(NewIdentifierToken("two")) {
		t.Errorf("after Set(mark), Peek(1) = %v, want 'two'", tok2)
	}
}

// =============================================================================
// Source / line management tests
// =============================================================================

func TestTokenizer_GetLine(t *testing.T) {
	tk := New("line one\nline two\nline three", false)

	tests := []struct {
		line int
		want string
	}{
		{1, "line one"},
		{2, "line two"},
		{3, "line three"},
		{0, ""},   // 0 is out of range (1-indexed)
		{-1, ""},  // negative is out of range
		{4, ""},   // beyond source
		{999, ""}, // way beyond source
	}

	for _, tt := range tests {
		got := tk.GetLine(tt.line)
		if got != tt.want {
			t.Errorf("GetLine(%d) = %q, want %q", tt.line, got, tt.want)
		}
	}
}

func TestTokenizer_GetLine_StripsSuffix(t *testing.T) {
	// In code mode, lines get "  ;" appended; GetLine must strip it.
	tk := New("x + y", true)
	got := tk.GetLine(1)

	if strings.Contains(got, lineEnding) {
		t.Errorf("GetLine(1) = %q, must not contain lineEnding %q", got, lineEnding)
	}

	if got != "x + y" {
		t.Errorf("GetLine(1) = %q, want %q", got, "x + y")
	}
}

func TestTokenizer_GetSource(t *testing.T) {
	src := "line one\nline two"
	tk := New(src, false)
	got := tk.GetSource()

	if !strings.Contains(got, "line one") {
		t.Errorf("GetSource() = %q, want it to contain 'line one'", got)
	}

	if !strings.Contains(got, "line two") {
		t.Errorf("GetSource() = %q, want it to contain 'line two'", got)
	}
	// Must not contain the internal line-ending suffix.
	if strings.Contains(got, lineEnding) {
		t.Errorf("GetSource() = %q, must not contain lineEnding %q", got, lineEnding)
	}
}

func TestTokenizer_GetTokenText(t *testing.T) {
	tk := New("one two three four", false)
	// 4 tokens: "one", "two", "three", "four"

	tests := []struct {
		name  string
		start int
		end   int
		want  string
	}{
		{"all tokens (end=-1)", 0, -1, "one two three four"},
		{"first two", 0, 1, "one two"},
		{"middle two", 1, 2, "two three"},
		{"last token", 3, 3, "four"},
		{"negative start clamps to 0", -5, 1, "one two"},
		{"end past bounds clamps to last", 0, 999, "one two three four"},
		{"single token at start", 0, 0, "one"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tk.GetTokenText(tt.start, tt.end)
			if got != tt.want {
				t.Errorf("GetTokenText(%d, %d) = %q, want %q", tt.start, tt.end, got, tt.want)
			}
		})
	}
}

// =============================================================================
// IsSymbol edge cases
// =============================================================================

// TestIsSymbol_Empty verifies that an empty string is correctly rejected as a symbol
// (previously the loop-never-executes path returned true; now fixed with an early guard).
func TestIsSymbol_Empty(t *testing.T) {
	if IsSymbol("") {
		t.Error("IsSymbol(\"\") = true, want false (empty string is not a valid identifier)")
	}
}

func TestIsSymbol_AdditionalCases(t *testing.T) {
	tests := []struct {
		s    string
		want bool
	}{
		{"a", true},
		{"_x", true},
		{"x1y2z3", true},
		{"ALLCAPS", true},
		{"123", false},  // all digits, no leading letter
		{"a-b", false},  // hyphen is not allowed
		{"a b", false},  // space is not allowed
		{"a.b", false},  // dot is not allowed
		{"a+b", false},  // operator is not allowed
		{"\x00", false}, // null byte
	}

	for _, tt := range tests {
		t.Run(tt.s, func(t *testing.T) {
			if got := IsSymbol(tt.s); got != tt.want {
				t.Errorf("IsSymbol(%q) = %v, want %v", tt.s, got, tt.want)
			}
		})
	}
}
