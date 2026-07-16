package parse

import (
	"github.com/tucats/ego/internal/language/parse/ast"
	"github.com/tucats/ego/internal/language/tokenizer"
)

// This file holds the data tables that drive the parser. Keeping them in one
// place is deliberate: extending the Ego grammar with a new operator, statement
// keyword, or type keyword should, in the common case, be a one-line edit here
// plus a single production function — not a hunt through the recursive-descent
// code. Each table is written to be checked line-by-line against docs/SYNTAX.md.

// binaryPrecedence maps a binary operator's spelling to its precedence level,
// mirroring the precedence summary in SYNTAX.md section 17 (higher binds
// tighter). The single precedence-climbing expression parser (parseBinary in
// expression.go) is driven entirely by this table, so adding or retiring a
// binary operator is one entry here.
//
// Levels:
//
//	1  ||
//	2  &&
//	3  == != < <= > >=
//	4  +  -  |  << >>
//	5  *  /  %  ^  &
var binaryPrecedence = map[string]int{
	"||": 1,
	"&&": 2,
	"==": 3, "!=": 3, "<": 3, "<=": 3, ">": 3, ">=": 3,
	"+": 4, "-": 4, "|": 4, "<<": 4, ">>": 4,
	"*": 5, "/": 5, "%": 5, "^": 5, "&": 5,
}

// lowestBinaryPrecedence is the precedence floor passed to the top-level
// expression parser. Any real operator level (>= 1) exceeds it.
const lowestBinaryPrecedence = 0

// assignOps is the set of assignment operator spellings (SYNTAX.md section 8).
// Membership here classifies a statement as an assignment.
var assignOps = map[string]bool{
	"=":  true,
	":=": true,
	"+=": true,
	"-=": true,
	"*=": true,
	"/=": true,
}

// primitiveTypeTokens is the set of built-in primitive type keywords
// (SYNTAX.md section 6). The tokenizer already classifies these as
// TypeTokenClass; this table lets parseTypeSpec confirm a leading token starts
// a primitive type and record its spelling.
var primitiveTypeTokens = map[string]bool{
	"bool": true, "byte": true, "chan": true,
	"int": true, "int8": true, "int16": true, "int32": true, "int64": true,
	"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true,
	"float32": true, "float64": true,
	"complex64": true, "complex128": true,
	"string": true,
}

// statementKeywords maps a leading reserved word's spelling to the parser
// method that parses that statement form. It is consulted by parseStatement
// after the non-keyword-led cases (assignment, expression statement) have been
// ruled out. Adding a keyword-led statement is one entry here plus one
// production method.
//
// The tables are keyed by spelling (not by tokenizer.Token) on purpose: a
// tokenizer.Token includes the source line/column, so using a token as a map
// key would fail to match lexer tokens, which carry a real position, against
// the zero-position package-level token constants. Every statement keyword is a
// reserved word with a unique spelling, so a spelling key is unambiguous.
//
// The map is populated in init() rather than as a literal because the values
// are methods with a *Parser receiver.
var statementKeywords map[string]func(*Parser) (ast.Node, error)

func init() {
	statementKeywords = map[string]func(*Parser) (ast.Node, error){
		tokenizer.IfToken.Spelling():       (*Parser).parseIf,
		tokenizer.ForToken.Spelling():      (*Parser).parseFor,
		tokenizer.SwitchToken.Spelling():   (*Parser).parseSwitch,
		tokenizer.ReturnToken.Spelling():   (*Parser).parseReturn,
		tokenizer.BreakToken.Spelling():    (*Parser).parseBreak,
		tokenizer.ContinueToken.Spelling(): (*Parser).parseContinue,
		tokenizer.DeferToken.Spelling():    (*Parser).parseDefer,
		tokenizer.GoToken.Spelling():       (*Parser).parseGo,
		tokenizer.TryToken.Spelling():      (*Parser).parseTry,
		tokenizer.PanicToken.Spelling():    (*Parser).parsePanicStmt,
		tokenizer.PrintToken.Spelling():    (*Parser).parsePrint,
		tokenizer.CallToken.Spelling():     (*Parser).parseCall,
		tokenizer.ThrowToken.Spelling():    (*Parser).parseThrow,
		tokenizer.ExitToken.Spelling():     (*Parser).parseExit,
	}
}

// declKeywords maps a leading reserved word's spelling to the declaration
// parser method. Declarations are legal at top level (unlike most executable
// statements). Adding a declaration keyword is one entry here. See
// statementKeywords for why these tables are keyed by spelling.
var declKeywords map[string]func(*Parser) (ast.Node, error)

func init() {
	declKeywords = map[string]func(*Parser) (ast.Node, error){
		tokenizer.ConstToken.Spelling():   (*Parser).parseConst,
		tokenizer.ImportToken.Spelling():  (*Parser).parseImport,
		tokenizer.PackageToken.Spelling(): (*Parser).parsePackage,
		tokenizer.TypeToken.Spelling():    (*Parser).parseTypeDecl,
		tokenizer.VarToken.Spelling():     (*Parser).parseVar,
	}
}
