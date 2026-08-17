package commands

import (
	"net/http"

	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/router"
	"github.com/tucats/ego/internal/server/admin"
	"github.com/tucats/ego/internal/server/admin/caches"
	"github.com/tucats/ego/internal/server/admin/users"
	"github.com/tucats/ego/internal/server/assets"
	"github.com/tucats/ego/internal/server/dsns"
	"github.com/tucats/ego/internal/server/tables"
	"github.com/tucats/ego/internal/util"
)

const (
	nameParameter = "{{name}}"
)

func defineStaticRoutes() *router.Router {
	// Let's use a private r for more flexibility with path patterns and providing session
	// context to the handler functions.
	r := router.NewRouter(defs.InstanceID)

	// Establish the admin endpoints
	ui.Log(ui.ServerLogger, "server.endpoints.admin", nil)

	// Define the payload validations
	router.InitializeValidations()

	// Get the current status of the server))
	// Get all config values
	r.New(defs.AdminConfigPath, admin.GetAllConfigHandler, http.MethodGet).
		Authentication(true, true).
		Class(router.AdminRequestCounter).
		AcceptMedia(defs.ConfigMediaType)

	// Get specific config values
	r.New(defs.AdminConfigPath, admin.GetConfigHandler, http.MethodPost).
		Authentication(true, true).
		Class(router.AdminRequestCounter).
		AcceptMedia(defs.ConfigMediaType)

	// BUG (found while documenting DATA-SECURITY.md's permission model):
	// every /admin/* route below that declares Permissions(defs.
	// ServerAdminPermission) also used Authentication(true, true) --
	// mustBeAdmin, which serve.go enforces before the Permissions() check
	// is even reached (and Permissions() is itself skipped once
	// session.Admin is true). Exactly the same shape as §3.12's DSN-route
	// bug, just across the whole admin/users/caches/logging/token/
	// resources surface instead of DSNs: ego.server.admin -- defined
	// specifically so an operator could administer the server without
	// full ego.root -- had no effect on any route that named it, because
	// reaching the route already required literal root. Changed every
	// one below to Authentication(true, false); Permissions() now makes
	// the only admin decision, and a root caller loses nothing since it
	// already treats session.Admin as satisfying any permission.
	//
	// SetCacheSizeHandler (just below GetCacheHandler) had no
	// Permissions() declaration of its own -- an oversight, not a
	// deliberately stricter route, given its two siblings (view, purge)
	// both use ServerAdminPermission for the same cache resource. Added
	// it for consistency rather than leaving it as the one cache
	// operation still requiring root.

	// Get the current memory status
	r.New(defs.AdminMemoryPath, admin.GetMemoryHandler, http.MethodGet).
		Authentication(true, false).
		Class(router.AdminRequestCounter).
		Permissions(defs.ServerAdminPermission)

	// Compile and run Ego code submitted from the dashboard Code tab
	r.New(defs.AdminRunPath, admin.RunCodeHandler, http.MethodPost).
		Authentication(true, false).
		Class(router.AdminRequestCounter).
		Permissions(defs.CodeRunPermission)

	// Parse and reformat Ego source submitted from the dashboard Code tab's
	// Format toggle. Same permission as /admin/run since it's the same
	// Code tab feature set.
	r.New(defs.AdminASTPath, admin.ASTHandler, http.MethodPost).
		Authentication(true, false).
		Class(router.AdminRequestCounter).
		Permissions(defs.CodeRunPermission)

	// Parse and reformat Ego source submitted from the dashboard Code tab's
	// Format toggle. Same permission as /admin/run since it's the same
	// Code tab feature set.
	r.New(defs.AdminFormatPath, admin.FormatCodeHandler, http.MethodPost).
		Authentication(true, false).
		Class(router.AdminRequestCounter).
		Permissions(defs.CodeRunPermission)

	// Get the current validation dictionary. Can request a specific method and
	// path to retrieve using parameters.
	r.New(defs.AdminValidationPath, admin.GetValidationsHandler, http.MethodGet).
		Authentication(true, false).
		Class(router.AdminRequestCounter).
		Parameter("method", util.StringParameterType).
		Parameter("path", util.StringParameterType).
		Parameter("entry", util.StringParameterType).
		Disallow("entry:path,method").
		Permissions(defs.ServerAdminPermission)

	// Start the dashboard UI
	r.New(defs.UIPath, admin.UIHandler, http.MethodGet).
		Class(router.AdminRequestCounter).
		Parameter("lang", util.StringParameterType).
		Parameter("language", util.StringParameterType)

	// Read an asset from disk or cache.
	r.New(defs.AssetsPath+"{{item...}}", assets.AssetsHandler, http.MethodGet).
		Class(router.AssetRequestCounter)

	// Same handler for HEAD: it produces the identical headers and suppresses
	// the body itself. Without this route an asset HEAD returned 404, which is
	// misleading to anything that probes for existence or inspects headers
	// before fetching -- including "curl -I" and any cache-validation check.
	r.New(defs.AssetsPath+"{{item...}}", assets.AssetsHandler, http.MethodHead).
		Class(router.AssetRequestCounter)

	// Create a new user
	r.New(defs.AdminUsersPath, users.CreateUserHandler, http.MethodPost).
		Authentication(true, false).
		Class(router.AdminRequestCounter).
		Permissions(defs.ServerAdminPermission)

	// Delete an existing user
	r.New(defs.AdminUsersPath+nameParameter, users.DeleteUserHandler, http.MethodDelete).
		Authentication(true, false).
		Class(router.AdminRequestCounter).
		Permissions(defs.ServerAdminPermission)

	// List user(s)
	r.New(defs.AdminUsersPath, users.ListUsersHandler, http.MethodGet).
		Authentication(true, false).
		Parameter(defs.StartParameterName, util.IntParameterType).
		Parameter(defs.LimitParameterName, util.IntParameterType).
		Class(router.AdminRequestCounter).
		Permissions(defs.ServerAdminPermission)

	// Get a specific user
	r.New(defs.AdminUsersPath+nameParameter, users.GetUserHandler, http.MethodGet).
		Authentication(true, false).
		AcceptMedia(defs.UserMediaType).
		Class(router.AdminRequestCounter).
		Permissions(defs.ServerAdminPermission)

	// Modify a specific user
	r.New(defs.AdminUsersPath+nameParameter, users.UpdateUserHandler, http.MethodPatch).
		Authentication(true, false).
		AcceptMedia(defs.UserMediaType).
		Class(router.AdminRequestCounter).
		Permissions(defs.ServerAdminPermission)

	// Get the status of the server cache.
	r.New(defs.AdminCachesPath, caches.GetCacheHandler, http.MethodGet).
		Authentication(true, false).
		Parameter("order-by", util.StringParameterType).
		Class(router.AdminRequestCounter).
		Permissions(defs.ServerAdminPermission)

	// Set the size of the cache.
	r.New(defs.AdminCachesPath, caches.SetCacheSizeHandler, http.MethodPost).
		Authentication(true, false).
		Class(router.AdminRequestCounter).
		Permissions(defs.ServerAdminPermission)

	// Purge items from the cache.
	r.New(defs.AdminCachesPath, caches.PurgeCacheHandler, http.MethodDelete).
		Authentication(true, false).
		Class(router.AdminRequestCounter).
		Permissions(defs.ServerAdminPermission).
		Parameter("class", util.ListParameterType)

	// Get the current logging status
	r.New(defs.AdminLoggersPath, admin.GetLoggingHandler, http.MethodGet).
		Authentication(true, false).
		Class(router.AdminRequestCounter).
		Permissions(defs.ServerAdminPermission)

	// Purge old logs
	r.New(defs.AdminLoggersPath, admin.PurgeLogHandler, http.MethodDelete).
		Authentication(true, false).
		Parameter("keep", util.IntParameterType).
		Class(router.AdminRequestCounter).
		Permissions(defs.ServerAdminPermission)

	// Set loggers
	r.New(defs.AdminLoggersPath, admin.SetLoggingHandler, http.MethodPost).
		Authentication(true, false).
		Class(router.AdminRequestCounter).
		Permissions(defs.ServerAdminPermission)

	// Simplest possible "are you there" endpoint.
	r.New(defs.AdminHeartbeatPath, admin.HeartbeatHandler, http.MethodGet).
		LightWeight(true).
		Class(router.HeartbeatRequestCounter)

	// Add a token ID to the blacklist for this server
	r.New(defs.AdminTokenPath, admin.TokenRevokeHandler, http.MethodPut).
		Authentication(true, false).
		Class(router.AdminRequestCounter).
		Permissions(defs.ServerAdminPermission)

	// Get the list of all blacklisted tokens
	r.New(defs.AdminTokenPath, admin.TokenListHandler, http.MethodGet).
		Authentication(true, false).
		Parameter(defs.StartParameterName, util.IntParameterType).
		Parameter(defs.LimitParameterName, util.IntParameterType).
		Class(router.AdminRequestCounter).
		Permissions(defs.ServerAdminPermission)

	// Delete an individual token from the blacklist
	r.New(defs.AdminTokenIDPath, admin.TokenDeleteHandler, http.MethodDelete).
		Authentication(true, false).
		Class(router.AdminRequestCounter).
		Permissions(defs.ServerAdminPermission)

	// Flush/delete the entire blacklist
	r.New(defs.AdminTokenPath, admin.TokenFlushHandler, http.MethodDelete).
		Authentication(true, false).
		Class(router.AdminRequestCounter).
		Permissions(defs.ServerAdminPermission)

	// Get overall server status (mashup of memory and caches, really)
	r.New(defs.AdminResourcesPath, admin.GetResourcesHandler, http.MethodGet).
		Authentication(true, false).
		Class(router.AdminRequestCounter).
		Permissions(defs.ServerAdminPermission)

	// Get information about the host machine (CPU, memory, OS, architecture)
	r.New(defs.AdminServerInfoPath, admin.GetServerInfoHandler, http.MethodGet).
		Authentication(true, false).
		Class(router.AdminRequestCounter).
		Permissions(defs.ServerAdminPermission)

	ui.Log(ui.ServerLogger, "server.endpoints.dsn", nil)

	// List all DSNS. Not gated by Permissions() here because the two
	// permissions that unlock it -- ego.server.admin and ego.sql -- are
	// alternatives, not both required; see the check inside ListDSNHandler.
	r.New(defs.DSNPath, dsns.ListDSNHandler, http.MethodGet).
		Authentication(true, false).
		AcceptMedia(defs.DSNListMediaType).
		Parameter("limit", util.IntParameterType).
		Parameter("start", util.IntParameterType).
		Class(router.TableRequestCounter)

	// BUG (found while validating DATA-SECURITY.md §3.3, now documented as
	// §3.12): all five DSN-admin routes below used Authentication(true,
	// true) -- the second argument sets Route.mustBeAdmin, which
	// serve.go enforces as "route.mustBeAdmin && !session.Admin" before
	// the Permissions() check is even reached (Permissions() is itself
	// skipped whenever session.Admin is true). That made the
	// Permissions(defs.DSNAdminPermission) declaration on each of these
	// routes dead code: only a literal ego.root caller could ever reach
	// any of them, contradicting the model's own stated intent -- item 1
	// says creating a DSN "requires ... ego.dsn.admin permission",
	// identity-level, not root -- and silently defeating §3.3's DSN-
	// creator self-grant fix, since a non-root identity-admin holder
	// could never reach CreateDSNHandler to trigger it in the first
	// place. Changed to Authentication(true, false): authentication is
	// still required, but the admin-or-equivalent decision is now made
	// once, correctly, by Permissions() -- which already treats
	// session.Admin as satisfying any permission, so a root caller loses
	// nothing.

	// Create a new DSN
	r.New(defs.DSNPath, dsns.CreateDSNHandler, http.MethodPost).
		Authentication(true, false).
		AcceptMedia(defs.DSNMediaType).
		Class(router.TableRequestCounter).
		Permissions(defs.DSNAdminPermission)

	// Read an existing DSN
	r.New(defs.DSNNamePath, dsns.GetDSNHandler, http.MethodGet).
		Authentication(true, false).
		AcceptMedia(defs.DSNMediaType).
		Class(router.TableRequestCounter).
		Permissions(defs.DSNAdminPermission)

	// Delete an existing DSN. Not gated by Permissions() here (DATA-
	// SECURITY.md §3.6): a caller may be authorized either by identity-
	// level ego.dsn.admin or by a DSN-specific dsns_auth admin record for
	// this one DSN, and Route.Permissions() can only express the former
	// -- it has no notion of "for this specific resource". The OR of the
	// two is checked inside DeleteDSNHandler instead, the same pattern
	// already used for ListDSNHandler/ListTablesHandler/DSNMetadataHandler.
	r.New(defs.DSNNamePath, dsns.DeleteDSNHandler, http.MethodDelete).
		Authentication(true, false).
		AcceptMedia(defs.DSNMediaType).
		Class(router.TableRequestCounter)

	// Add or delete DSN permissions. Not gated by Permissions() for the
	// same reason as delete, above -- DSNPermissionsHandler checks
	// identity ego.dsn.admin OR a per-DSN admin record for each item's
	// own {{dsn}} (a single request can name more than one DSN).
	r.New(defs.DSNPath+defs.PermissionsPseudoTable, dsns.DSNPermissionsHandler, http.MethodPost).
		Authentication(true, false).
		AcceptMedia(defs.DSNPermissionsType).
		Class(router.TableRequestCounter)

	// List permissions for a DSN. Not gated by Permissions() for the same
	// reason as delete/grant above -- the plan (DATA-SECURITY.md §3.6)
	// doesn't name this route explicitly, but it has the identical
	// route-level-only gate and gap, and a DSN-specific admin ought to be
	// able to see what they can already grant/revoke.
	r.New(defs.DSNNamePath+defs.PermissionsPseudoTable, dsns.ListDSNPermHandler, http.MethodGet).
		Authentication(true, false).
		AcceptMedia(defs.DSNListPermsMediaType).
		Class(router.TableRequestCounter)

	ui.Log(ui.ServerLogger, "server.endpoints.tables", nil)

	// Handlers that manipulate a table are defined the in tables package.
	tables.AddStaticRoutes(r)

	return r
}
