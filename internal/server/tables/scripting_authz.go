package tables

import "github.com/tucats/ego/internal/server/tables/scripting"

// Wire this package's Authorized (security.go) into the scripting package
// at startup. scripting cannot import tables directly -- tables already
// imports scripting (routes.go registers scripting.Handler for the
// @transaction endpoint) -- so the dependency is injected in this
// direction instead. This init runs whenever tables is imported, which
// every server binary does unconditionally, so scripting.AuthorizedFunc is
// always populated before any request can be routed. See scripting's
// authz.go for the consumer side.
func init() {
	scripting.AuthorizedFunc = Authorized
}
