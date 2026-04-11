package scripting

import (
	"strings"

	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/server/tables/parsing"
)

// filterErrorMessage cleans up a raw database or filter-parsing error string so
// it can be shown to the API caller without internal noise.
//
// Two transformations are applied:
//
//  1. If the string contains the sentinel parsing.SyntaxErrorPrefix (injected by
//     the filter compiler when it rejects bad input), everything up to and including
//     that prefix is stripped, leaving just the human-readable message. Any trailing
//     fragment that starts with defs.RowIDName (the internal row-ID column) is also
//     stripped — it is an implementation detail the caller should not see.
//
//  2. If no syntax-error prefix is found, the "pq: " driver prefix that PostgreSQL
//     errors carry is removed so the message reads cleanly.
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
