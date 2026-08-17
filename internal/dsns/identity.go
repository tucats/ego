package dsns

import (
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/util"
)

// IdentityAuthorizesAction reports whether a caller's identity-wide
// permissions -- as opposed to a per-DSN grant recorded in the dsns_auth
// table, which AuthDSN checks -- satisfy the requested action bitmask.
//
// Per DATA-SECURITY.md §3.4 and the resolved model in §1a: identity-level
// ego.dsn.admin satisfies any action, and identity-level ego.dsn.read/
// ego.dsn.write each satisfy the matching bit of a (possibly combined)
// action bitmask, mirroring AuthDSN's own "(auth.Action & action) != 0"
// semantics. This is the identity-wide tier of the identity > per-DSN >
// per-table hierarchy; a caller for whom this returns false still has the
// per-DSN dsns_auth grant (AuthDSN) available as a fallback.
//
// permissions must already be the caller's resolved identity permission
// list (e.g. session.Permissions). This package cannot resolve it itself
// (via internal/server/auth) without creating an import cycle back through
// internal/runtime/sql, which uses this package -- callers that need the
// auth.GetPermission fallback for an unresolved session (see
// sql_permissions.go's hasPermission for why that case exists) must
// resolve permissions themselves before calling this function.
func IdentityAuthorizesAction(permissions []string, action DSNAction) bool {
	if util.InListInsensitive(defs.DSNAdminPermission, permissions...) {
		return true
	}

	if action&DSNReadAction != 0 && util.InListInsensitive(defs.DSNReadPermission, permissions...) {
		return true
	}

	if action&DSNWriteAction != 0 && util.InListInsensitive(defs.DSNWritePermission, permissions...) {
		return true
	}

	return false
}
