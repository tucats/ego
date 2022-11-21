package tokenizer

import "github.com/tucats/ego/util"

// ReservedWords is the list of reserved words in the _Ego_ language.
var ReservedWords []string = []string{
	"bool",
	"break",
	"byte",
	"chan",
	"const",
	"continue",
	"defer",
	"else",
	"float32",
	"float64",
	"for",
	"func",
	"go",
	"if",
	"import",
	"int",
	"int32",
	"int64",
	"map",
	"nil",
	"package",
	"return",
	"string",
	"struct",
	"var",
}

var ExtendedReservedWords = []string{
	"call",
	"catch",
	"print",
	"try",
}

// IsReserved indicates if a name is a reserved word.
func IsReserved(name string, includeExtensions bool) bool {
	r := util.InList(name, ReservedWords...)
	if includeExtensions {
		r = r || util.InList(name, ExtendedReservedWords...)
	}

	return r
}
