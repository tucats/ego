package tokenizer

// ReservedWords is the list of reserved words in the _Ego_ language.
var ReservedWords = map[string]bool{
	"bool":      true,
	"break":     true,
	"byte":      true,
	"chan":      true,
	"const":     true,
	"continue":  true,
	"defer":     true,
	"else":      true,
	"float32":   true,
	"float64":   true,
	"for":       true,
	"func":      true,
	"go":        true,
	"if":        true,
	"import":    true,
	"interface": true,
	"int":       true,
	"int32":     true,
	"int64":     true,
	"map":       true,
	"nil":       true,
	"package":   true,
	"return":    true,
	"string":    true,
	"struct":    true,
	"type":      true,
	"var":       true,
}

// Additionally, these verbs are permitted when running with
// language extensions enabled.
var ExtendedReservedWords = map[string]bool{
	"call":  true,
	"catch": true,
	"print": true,
	"try":   true,
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
