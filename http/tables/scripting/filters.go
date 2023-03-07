package scripting

import (
	"strings"

	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/http/tables/parsing"
)

func filterErrorMessage(q string) string {
	if p := strings.Index(q, parsing.SyntaxErrorPrefix); p >= 0 {
		msg := q[p+len(parsing.SyntaxErrorPrefix):]
		if p := strings.Index(msg, defs.RowIDName); p > 0 {
			msg = msg[:p]
		}

		return "filter error: " + msg
	}

	return strings.TrimPrefix(q, "pq: ")
}
