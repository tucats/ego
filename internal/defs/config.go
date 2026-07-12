package defs

// This section describes the profile keys used by Ego.
const (
	// The prefix for all configuration keys reserved to Ego. These names must be valid names
	// and there are access control limits on when/where these can be modified. Configuration
	// values that do not start with this value are available to the user, and are restricted
	// or controlled in any way.
	PrivilegedKeyPrefix = "ego."

	// The base URL of the Ego server providing application services. Often, this is the same
	// as the logon server, in which case an explicit application server setting is not needed.
	// However, it may be set to a different value if the logon services are hosted on a
	// different server or port.
	ApplicationServerSetting = PrivilegedKeyPrefix + "application.server"

	// LOGON CONFIGURATION KEYS
	// The prefix for all logon configuration keys.
	LogonKeyPrefix = PrivilegedKeyPrefix + "logon."

	// The base URL of the Ego server providing logon services.
	LogonServerSetting = LogonKeyPrefix + "server"

	// The value of the token created by a ego logon command, which is used by
	// default for server admin commands as well as rest calls.
	LogonTokenSetting = LogonKeyPrefix + "token"

	// Stores the expiration date from the last login. This can be used to detect
	// an expired token and provide a better message to the client user than
	// "not authorized".
	LogonTokenExpirationSetting = LogonKeyPrefix + "token.expiration"

	// LogonRefreshTokenSetting stores the OAuth2 refresh token obtained during
	// an --oauth logon. It is written programmatically (not user-settable) and
	// used to silently renew an expired access token on the next logon attempt.
	LogonRefreshTokenSetting = LogonKeyPrefix + "refresh.token"

	// OAUTH2 CLI LOGIN CONFIGURATION KEYS
	// OAuthCLIKeyPrefix is the prefix for settings that control how the CLI
	// performs OAuth2 Authorization Code + PKCE logins (ego logon --oauth).
	OAuthCLIKeyPrefix = LogonKeyPrefix + "oauth."

	// OAuthCLIServerSetting is an explicit OAuth2 issuer URL for CLI logins.
	// When set, it overrides the auto-detection priority order. Must be the
	// base URL of an OIDC-compliant authorization server (Ego's own AS or an
	// external IdP). If absent, the CLI tries ego.server.oauth.as.issuer then
	// ego.server.oauth.provider in order.
	OAuthCLIServerSetting = OAuthCLIKeyPrefix + "server"

	// OAuthCLIClientIDSetting is the OAuth2 client_id the CLI presents when
	// starting an Authorization Code flow. The matching client must be registered
	// in the AS's client registry. Default: "ego-cli" (the built-in public client
	// that Ego's AS pre-registers automatically).
	OAuthCLIClientIDSetting = OAuthCLIKeyPrefix + "client.id"

	// OAuthCLIScopesSetting is the space-separated list of OAuth2 scopes the
	// CLI requests during login. "openid" is always included regardless of this
	// setting. Default: "openid profile".
	OAuthCLIScopesSetting = OAuthCLIKeyPrefix + "scopes"

	// LOG CONFIGURATION KEYS
	// The prefix for all log-related configuration keys.
	LogKeyPrefix = PrivilegedKeyPrefix + "log."

	// If specified, has the Go-style format string to be used for log
	// messages showing the time of the event.
	LogTimestampFormat = LogKeyPrefix + "timestamp"

	// This is the file name that is used to store the log file when it
	// rolls over and needs to be added to a zip archive. If not specified,
	// log files that are rolled off are deleted.
	LogArchiveSetting = LogKeyPrefix + "archive"

	// How many old logs do we maintain by default when in server mode?
	LogRetainCountSetting = LogKeyPrefix + "retain"

	// The prefix for all configuration keys.
	ConfigKeyPrefix = PrivilegedKeyPrefix + "config."

	// The prefix for all database configuration keys.
	DatabaseKeyPrefix = PrivilegedKeyPrefix + "database."

	// RUNTIME CONFIGURATION KEYS
	// The prefix for all runtime configuration keys.
	RuntimeKeyPrefix = PrivilegedKeyPrefix + "runtime."

	// File system location used to locate services, lib,
	// and test directories.
	EgoPathSetting = RuntimeKeyPrefix + "path"

	// If specified, all filename references in ego programs (such as the
	// ReadFile() function) must start with this path, or it will be prefixed
	// with this path. This lets you limit where/how the files can be managed
	// by an ego program. This is especially important in server mode.
	SandboxPathSetting = RuntimeKeyPrefix + "sandbox.path"

	// File system location used to locate the lib directory. If this setting
	// isn't defined, it defaults to the runtime.path  concatenated with "/lib".
	// This lets the user set the lib location to be a standard location like
	// /usr/local/lib if desired.
	EgoLibPathSetting = EgoPathSetting + ".lib"

	// Specify if the automatic creation of the lib/ directory
	// should be suppressed.
	SuppressLibraryInitSetting = RuntimeKeyPrefix + "suppress.library.init"

	// If true, the util.Exec() function can be executed to run an arbitrary
	// native shell command. This defaults to being disabled.
	ExecPermittedSetting = RuntimeKeyPrefix + "exec"

	// If true, the REST client will not require SSL/HTTPS for connections.
	// This is useful for testing and development but should not be used in
	// production.
	InsecureClientSetting = RuntimeKeyPrefix + "insecure.client"

	// If true, cast operations that cause a loss of data (casting a value that
	// is larger than 255 into a byte, etc.) will result in an error.
	PrecisionErrorSetting = RuntimeKeyPrefix + "precision.error"

	// Default allocation factor to set on symbol table create/expand
	// operations. Larger numbers are more efficient for larger symbol
	// tables, but too large a number wastes time and memory.
	SymbolTableAllocationSetting = RuntimeKeyPrefix + "symbol.allocation"

	// If true, functions that return multiple values including an error that
	// do not assign that error to a value will result in an error return.
	ThrowUncheckedErrorsSetting = RuntimeKeyPrefix + "unchecked.errors"

	// If true, then an invocation of the panic() function will result in
	// an actual native Go panic, ending the process immediately.
	RuntimePanicsSetting = RuntimeKeyPrefix + "panics"

	// If true, functions do not create symbol scope barriers. This is generally
	// only true when running in test mode.
	RuntimeDeepScopeSetting = RuntimeKeyPrefix + "deep.scope"

	// If true, the TRACE operation will print the full stack instead of
	// a shorter single-line version.
	FullStackTraceSetting = RuntimeKeyPrefix + "stack.trace"

	// REST CONFIGURATION KEYS
	// The prefix for all REST  configuration keys.
	RestKeyPrefix = RuntimeKeyPrefix + "rest."

	// Do we automatically process non-success ErrorResponse payloads from
	// client REST calls as if they were the return code value? Default is
	// true.
	RestClientErrorSetting = RestKeyPrefix + "errors"

	// Is there a timeout on REST client operations? If specified, must be a
	// valid duration string.
	RestClientTimeoutSetting = RestKeyPrefix + "timeout"

	// If set to "system", we do not load a server cert file to trust, and
	// depend on the default system trust store.
	RestClientServerCert = RestKeyPrefix + "server.cert"

	// COMPILER CONFIGURATION KEYS
	// The prefix for compiler configuration keys.
	CompilerKeyPrefix = PrivilegedKeyPrefix + "compiler."

	// Do we normalize the case of all symbols to a common (lower) case
	// string. If not true, symbol names are case-sensitive.
	CaseNormalizedSetting = CompilerKeyPrefix + "normalized"

	// If true, the script language includes language  extensions such as
	// print, call, try/catch.
	ExtensionsEnabledSetting = CompilerKeyPrefix + "extensions"

	// Should an interactive session automatically import all the pre-
	// defined packages?
	AutoImportSetting = CompilerKeyPrefix + "import"

	// Set to true if the full stack should be listed during tracing.
	FullStackListingSetting = CompilerKeyPrefix + "full.stack"

	// Should the bytecode generator attempt an optimization pass?
	OptimizerSetting = CompilerKeyPrefix + "optimize"

	// Should the Ego program(s) be run with "strict" or  "dynamic" typing?
	// The default is "dynamic".
	StaticTypesSetting = CompilerKeyPrefix + "types"

	// Should a local variable be permitted to shadow the name of a built-in
	// type (e.g. "int := 5")? Defaults to true, matching Go's own behavior
	// and Ego's historical behavior. When set to false, declaring a
	// variable named after a built-in type is a compile-time error --
	// useful in teaching contexts where accidentally shadowing a type name
	// is a common, confusing mistake worth calling out explicitly.
	TypeShadowingSetting = CompilerKeyPrefix + "type.shadowing"

	// Should a variable that is declared but never used be an error?
	UnusedVarsSetting = CompilerKeyPrefix + "unused.var.error"

	// Should the compiler report an unknown symbol error without waiting
	// for the runtime symbol table manager to report it? This is currently
	// somewhat experimental.
	UnknownVarSetting = CompilerKeyPrefix + "unknown.var.error"

	// When true, compiler logging includes tracking  variable usage scope.
	UnusedVarLoggingSetting = CompilerKeyPrefix + "var.usage.logging"

	// CONSOLE CONFIGURATION KEYS
	// The prefix for console configuration keys.
	ConsoleKeyPrefix = PrivilegedKeyPrefix + "console."

	// PromptMissingOptions indicates if the console should display a prompt
	// for command line options that are required but not found.
	ConsolePromptMissingOptions = ConsoleKeyPrefix + "prompt.missing.options"

	// AutoHelpConfigSetting indicates if help is automatically displayed when
	// a CLI command is incomplete. If false (the default), help is not
	// displayed but a list of expected terms is displayed.
	AutoHelpConfigSetting = ConsoleKeyPrefix + "auto.help"

	// ConsoleHistorySetting is the name of the readline console history
	// file. This contains a line of text for each command previously read
	// using readline. If not specified in the profile, a default is used.
	ConsoleHistorySetting = ConsoleKeyPrefix + "history"

	// Should the copyright message be omitted when in interactive mode?
	NoCopyrightSetting = ConsoleKeyPrefix + "no.copyright"

	// Should the interactive command input processor use the readline
	// library?
	UseReadlineSetting = ConsoleKeyPrefix + "readline"

	// This setting is only used internally in Ego to indicate that the
	// console is operating interactively (i.e acting as a REPL). It is not
	// intended to be set by the user.
	AllowFunctionRedefinitionSetting = ConsoleKeyPrefix + "interactive"

	// What is the output format that should be used by default for operations
	// that could return either "text" , "indented", or "json" output.
	OutputFormatSetting = ConsoleKeyPrefix + "output"

	// What is the log format that should be used by default for logging.
	// Valid choices are "text", "json", and "indented".
	LogFormatSetting = ConsoleKeyPrefix + "log"

	// TABLE CONFIGURATION KEYS
	//The prefix for database table configuration keys.
	TableKeyPrefix = PrivilegedKeyPrefix + "table."

	// If true, command lines that contain "foo.bar" table names will
	// assume the dsn is foo and the table is bar.
	TableAutoParseDSNSetting = TableKeyPrefix + "autoparse.dsn"

	// The default data source name to use for table commands. If not specified,
	// no default is used.
	DefaultDataSourceSetting = TableKeyPrefix + "default.dsn"

	// SERVER CONFIGURATION KEYS
	// The prefix for all server configuration keys.
	ServerKeyPrefix = PrivilegedKeyPrefix + "server."

	// If true, the REST payload will report the fully-qualified domain name for
	// the server. Otherwise, the "shortname" is used, which is the default case.
	ServerReportFQDNSetting = ServerKeyPrefix + "report.fqdn"

	// The default user if no user database has been initialized yet. This is a
	// string of the form "user:password", which is defined as the root user.
	DefaultCredentialSetting = ServerKeyPrefix + "default.credential"

	// If present, this user is always assigned super-user (root) privileges
	// regardless of the user authorization settings.
	LogonSuperuserSetting = ServerKeyPrefix + "superuser"

	// The file system location where the user database is stored.
	LogonUserdataSetting = ServerKeyPrefix + "userdata"

	// The encryption key for the userdata file. If not present the file is
	// not encrypted and is readable json. Note that this should only be used
	// when initially developing and testing a server configuration, but is
	// not safe in production deployments.
	LogonUserdataKeySetting = ServerKeyPrefix + "userdata.key"

	// ServerDefaultLogFileNameSetting is the default name for the server log file.
	// If not supplied in the configuration, the default is "ego-server.log".
	// The name will have a datestamp appended to the name.
	ServerDefaultLogFileNameSetting = ServerKeyPrefix + "default.log.file"

	// Interval for server memory usage logging. If not specified, the default
	// is every three minutes. If there is no server activity during an interval,
	// no logging is done, so making this too long will risk dropping data. But
	// making it too frequent will generate logs of logging.
	MemoryLogIntervalSetting = ServerKeyPrefix + "memory.log.interval"

	// The host that provides authentication services on our behalf. If not
	// specified, the current server is also the authentication service.
	ServerAuthoritySetting = ServerKeyPrefix + "authority"

	// The number of seconds between scans to see if cached authentication
	// data from a remote authorization server should be checked for expired
	// values. The default is every 180 seconds (3 minutes).
	AuthCacheScanSetting = ServerKeyPrefix + "auth.cache.scan"

	// If true, when REST logging is enabled, the server log itself will be
	// logged as a response payload to the /log service request. This is
	// normally off and should only be enable when debugging logging.
	ServerLogResponseSetting = ServerKeyPrefix + "log.response"

	// Indicator if /service requests are executed by a child process instead
	// of in-process.
	ChildServicesSetting = ServerKeyPrefix + "child.services"

	// Prefix for server child process configuration settings.
	ChildServicesKeyPrefix = ChildServicesSetting + "."

	// Optional override for the location where request payloads are stored.
	ChildRequestDirSetting = ChildServicesKeyPrefix + "dir"

	// If a positive integer, this limits the number of simultaneous child
	// processes that can be spawned to handle requests.
	ChildRequestLimitSetting = ChildServicesKeyPrefix + "limit"

	// Flag to indicate if the child response payload files should be retained
	// for debugging, etc. By default they are deleted when the request completes.
	ChildRequestRetainSetting = ChildServicesKeyPrefix + "retain"

	// Duration string indicating how long we wait for an available child
	// process before returning an error.
	ChildRequestTimeoutSetting = ChildServicesKeyPrefix + "timeout"

	// The URL path for the tables database functionality.
	ServerDatabaseKeyPrefix = ServerKeyPrefix + "database."

	// True if a destructive operation like delete or update is done without any filter.
	DatabaseServerEmptyFilterError = ServerDatabaseKeyPrefix + "empty.filter.error"

	// The URL path for the tables database functionality.
	TablesServerEmptyFilterError = ServerDatabaseKeyPrefix + "empty.filter.error"

	// The URL path for the tables database functionality.
	TablesServerEmptyRowsetError = ServerDatabaseKeyPrefix + "empty.rowset.error"

	// If true, the insert of a row _must_ specify all values in the table.
	TableServerPartialInsertError = ServerDatabaseKeyPrefix + "partial.insert.error"

	// The key string used to encrypt authentication tokens.
	ServerTokenKeySetting = ServerKeyPrefix + "token.key"

	// A string indicating the duration of a token before it is considered
	// expired. Examples are "15m" or "24h".
	ServerTokenExpirationSetting = ServerKeyPrefix + "token.expiration"

	// A string indicating the default logging to be assigned to a server
	// that is started without an explicit --log setting.
	ServerDefaultLogSetting = ServerKeyPrefix + "default.logging"

	// The default port number for the server to listen on if not specified
	// on the command line or in an env variable.
	ServerDefaultPortSetting = ServerKeyPrefix + "default.port"

	// ServerStartLogAgeSetting is the number of days worth of start log
	// entries to keep in the system database "starts" table. This value
	// defaults to 30 days. The intent is that if you have a lot of restarts,
	// the system database won't fill up too quickly, and entries older thank
	// this are automatically purged on startup.
	ServerStartLogAgeSetting = ServerKeyPrefix + "start.log.age"

	// PidDirectorySettings has the path used to store and find PID files for
	// server invocations and management.
	PidDirectorySetting = ServerKeyPrefix + "piddir"

	// If true, the default state for staring an Ego is to not require HTTPS/SSL
	// but rather run in "insecure" mode.
	InsecureServerSetting = ServerKeyPrefix + "insecure"

	// Maximum cache size for server cache. The default is zero, no caching
	// performed.
	MaxCacheSizeSetting = ServerKeyPrefix + "cache.size"

	// Maximum number of consecutive failed login attempts before an account is
	// temporarily locked. The default when not set is 5. Set to 0 to disable
	// the lockout mechanism entirely.
	AuthMaxAttemptsSetting = ServerKeyPrefix + "auth.maxattempts"

	// How long an account stays locked after exceeding the failed-attempt threshold.
	// Expressed as a Go duration string (e.g. "15m", "1h"). Default is "15m".
	AuthLockoutDurationSetting = ServerKeyPrefix + "auth.lockout"

	// If true, JavaScript assets served via the /assets endpoint are minified
	// before being cached and returned to the client. Default is false.
	JSMinifySetting = ServerKeyPrefix + "js.minify"

	// If true, the JavaScript minifier will also rename local variable and
	// function parameter identifiers to short generated names (a, b, …).
	// Only takes effect when JSMinifySetting is also true. Default is false.
	JSShortVarNamesSetting = ServerKeyPrefix + "js.shortvarnames"

	// If true, legacy {quoted} plaintext passwords in the authentication store
	// are accepted and migrated to bcrypt on first successful login. When false
	// (the default), any stored password in that format is rejected and an error
	// is written to the auth log instead.
	PlaintextPasswordSetting = ServerKeyPrefix + "plaintext.passwords"

	// WebAuthnRPIDSetting is the Relying Party ID used for WebAuthn / passkey
	// ceremonies. This must be a registrable domain suffix of the origin the
	// dashboard is served from (e.g. "ego.example.com"). When empty, passkey
	// endpoints return 501 Not Implemented.
	WebAuthnRPIDSetting = ServerKeyPrefix + "webauthn.rpid"

	// WebAuthnAllowPasskeysSetting controls whether the server exposes passkey
	// (WebAuthn) functionality. When false the /config endpoint reports
	// passkeys:false and the dashboard hides all passkey UI.
	WebAuthnAllowPasskeysSetting = ServerKeyPrefix + "allow.passkeys"

	// ServerReadHeaderTimeoutSetting is the maximum time allowed to receive all
	// HTTP request headers before the connection is closed. Must be a Go duration
	// string (e.g. "10s"). Defaults to "10s". This is the primary protection
	// against Slowloris-style connection exhaustion attacks.
	ServerReadHeaderTimeoutSetting = ServerKeyPrefix + "read.header.timeout"

	// ServerReadTimeoutSetting is the maximum time allowed to read the complete
	// HTTP request (headers + body). Must be a Go duration string (e.g. "30s").
	// Defaults to "30s".
	ServerReadTimeoutSetting = ServerKeyPrefix + "read.timeout"

	// ServerInsecureRedirect indicates if the secure server should also start an insecure
	// HTTP server that listens on port 80 and redirects all requests to the secure HTTPS
	// server. This is useful when you want to run the secure server on the default HTTPS
	// port (443) but still want to capture requests that come in on the default HTTP port.
	// IF the value is true, the redirect will be set up. The default is false.
	ServerInsecureRedirect = ServerKeyPrefix + "insecure.redirect"

	// ServerWriteTimeoutSetting is the maximum time allowed to send the complete
	// HTTP response. Must be a Go duration string (e.g. "120s"). Defaults to
	// "120s". Set this generously enough to cover large responses such as log
	// retrieval.
	ServerWriteTimeoutSetting = ServerKeyPrefix + "write.timeout"

	// ServerIdleTimeoutSetting is the maximum time a keep-alive connection may
	// remain idle before the server closes it. Must be a Go duration string
	// (e.g. "120s"). Defaults to "120s".
	ServerIdleTimeoutSetting = ServerKeyPrefix + "idle.timeout"

	// ServerMaxBodySizeSetting is the maximum accepted request body size in bytes.
	// Requests with a larger body are rejected with HTTP 413 before the handler
	// is invoked. Defaults to 33554432 (32 MiB) when not set or set to zero.
	ServerMaxBodySizeSetting = ServerKeyPrefix + "max.body.size"

	// ServerMaxItemLimitSetting is the maximum number of items that may be returned
	// by a single paged GET request (via the "limit" query parameter). If not
	// configured, a server default of 1000 is used. A client-supplied "limit" value
	// that exceeds this ceiling is rejected with HTTP 400.
	ServerMaxItemLimitSetting = ServerKeyPrefix + "max.item.limit"

	// CLUSTER CONFIGURATION KEYS
	// The prefix for all cluster-related configuration keys.
	ClusterKeyPrefix = PrivilegedKeyPrefix + "cluster."

	// ClusterNameSetting is the name of the cluster this node belongs to. When
	// empty (the default) the server runs in standalone mode and no cluster
	// logic is activated. Set via the --cluster flag on startup.
	ClusterNameSetting = ClusterKeyPrefix + "name"

	// ClusterPingIntervalSetting controls how often (as a Go duration string) the
	// health-check goroutine contacts each peer node. The default is "30s".
	ClusterPingIntervalSetting = ClusterKeyPrefix + "ping.interval"

	// ClusterPingTimeoutSetting is the maximum time (as a Go duration string) to
	// wait for a single peer health-check ping to complete. The default is "5s".
	ClusterPingTimeoutSetting = ClusterKeyPrefix + "ping.timeout"

	// OAUTH2 AUTHORIZATION SERVER CONFIGURATION KEYS
	// OAuthASKeyPrefix is the prefix for all OAuth2 Authorization Server settings.
	// These are only active when OAuthASEnabledSetting is true.
	OAuthASKeyPrefix = ServerKeyPrefix + "oauth.as."

	// OAuthASEnabledSetting controls whether this Ego server acts as an OAuth2
	// Authorization Server. When true, the standard OIDC endpoints are registered
	// at startup. Intended for development and testing environments; for production
	// use a dedicated identity provider such as Okta, Entra ID, or Keycloak.
	OAuthASEnabledSetting = OAuthASKeyPrefix + "enabled"

	// OAuthASKeyFileSetting is the filesystem path to the PEM file containing the
	// EC private signing key used to sign JWTs. If the file does not exist, Ego
	// generates a new P-256 key pair and saves it here automatically.
	// Default: {EGO_PATH}/lib/oauth/oauth-signing.pem
	// The file and its parent directory are chmod'd 0600/0700 (owner-only) at startup.
	OAuthASKeyFileSetting = OAuthASKeyPrefix + "key.file"

	// OAuthASClientFileSetting is the filesystem path to a JSON file that lists the
	// OAuth2 clients permitted to request tokens from this AS. Each client entry
	// specifies a client_id, client_secret, allowed redirect URIs, grant types, and
	// scopes. See docs/OAUTH.md for the file format.
	// Default: {EGO_PATH}/lib/oauth/oauth-clients.json
	// The file and its parent directory are chmod'd 0600/0700 (owner-only) at startup.
	OAuthASClientFileSetting = OAuthASKeyPrefix + "clients"

	// OAuthASIssuerSetting is the base URL of this server, reported as the "iss"
	// (issuer) claim in every JWT and used to build the OIDC discovery document URLs.
	// It must exactly match the publicly reachable URL of the Ego server.
	// Example: "https://ego.example.com" or "http://localhost:4040"
	OAuthASIssuerSetting = OAuthASKeyPrefix + "issuer"

	// OAuthASTokenExpirationSetting is the lifetime of OAuth2 access tokens issued by
	// this AS. Must be a Go duration string such as "30m", "1h", or "8h".
	// Default: "1h".
	OAuthASTokenExpirationSetting = OAuthASKeyPrefix + "token.expiration"

	// OAuthASRefreshExpirationSetting is the lifetime of OAuth2 refresh tokens issued
	// by this AS. Must be a Go duration string. Refresh tokens allow clients to obtain
	// new access tokens without repeating the full authorization flow.
	// Default: "24h".
	OAuthASRefreshExpirationSetting = OAuthASKeyPrefix + "refresh.expiration"

	// OAuthASCodeExpirationSetting is the lifetime of OAuth2 authorization codes
	// issued during the Authorization Code flow. Codes must be short-lived; the
	// OAuth2 spec recommends no more than 10 minutes.
	// Default: "5m".
	OAuthASCodeExpirationSetting = OAuthASKeyPrefix + "code.expiration"

	// OAUTH2 RESOURCE SERVER CONFIGURATION KEYS
	// OAuthKeyPrefix is the prefix for all OAuth2 Resource Server settings.
	// These are active when OAuthProviderSetting is non-empty.
	OAuthKeyPrefix = ServerKeyPrefix + "oauth."

	// OAuthProviderSetting is the base URL of the external OAuth2/OIDC identity
	// provider. Ego appends /.well-known/openid-configuration to this URL to
	// perform OIDC discovery. Setting this value activates the RS role.
	// Example: "https://dev-12345.okta.com/oauth2/default"
	OAuthProviderSetting = OAuthKeyPrefix + "provider"

	// OAuthClientIDSetting is the application (client) identifier assigned to Ego
	// when it was registered with the identity provider. This is a non-secret public
	// value that identifies which application is making OAuth2 requests.
	OAuthClientIDSetting = OAuthKeyPrefix + "client.id"

	// OAuthClientSecretSetting is the client secret assigned to Ego by the identity
	// provider. Treat this as a password — do not put it in config files checked into
	// source control. It can also be supplied via the EGO_OAUTH_CLIENT_SECRET
	// environment variable, which takes precedence over this setting.
	OAuthClientSecretSetting = OAuthKeyPrefix + "client.secret"

	// OAuthScopesSetting is the space-separated list of OAuth2 scopes Ego will
	// request when initiating Authorization Code flow. Must include "openid" for
	// OIDC identity claims.
	// Example: "openid profile email ego:read ego:write".
	OAuthScopesSetting = OAuthKeyPrefix + "scopes"

	// OAuthRedirectURISetting is the callback URL that the identity provider
	// redirects to after a successful user login. It must exactly match the URI
	// registered with the provider. Required for Authorization Code flow.
	// Example: "https://ego.example.com/services/admin/oauth/callback"
	OAuthRedirectURISetting = OAuthKeyPrefix + "redirect.uri"

	// OAuthUserClaimSetting is the name of the JWT claim that carries the username
	// or user identifier to use as the Ego user identity.
	// Default: "sub". Common alternatives: "email", "preferred_username".
	OAuthUserClaimSetting = OAuthKeyPrefix + "user.claim"

	// OAuthPermissionClaimSetting is the name of the JWT claim that carries the
	// roles, groups, or scopes used to derive Ego permissions.
	// Default: "scope". Some providers use "roles", "groups", or a custom claim.
	OAuthPermissionClaimSetting = OAuthKeyPrefix + "permission.claim"

	// OAuthAudienceSetting is the expected value of the JWT "aud" (audience) claim.
	// Ego rejects tokens whose audience does not match this value, preventing tokens
	// intended for other services from being accepted. When empty, audience
	// validation is skipped (not recommended for production).
	// Typically set to the Ego client_id or a dedicated resource identifier.
	OAuthAudienceSetting = OAuthKeyPrefix + "audience"

	// OAuthModeSetting controls how Ego integrates with the identity provider.
	// Allowed values:
	//   "resource-server" — accept JWT Bearer tokens directly; clients must obtain
	//                       JWTs from the IdP themselves.
	//   "proxy"           — redirect logon through OAuth2 and return a native Ego
	//                       token; best for browser users and existing clients.
	//   "hybrid"          — accept both JWT Bearer tokens and native Ego tokens;
	//                       also supports proxy logon for browser users.
	// Default: "hybrid" when ego.server.oauth.provider is set.
	OAuthModeSetting = OAuthKeyPrefix + "mode"

	// OAuthJWKSCacheTTLSetting controls how long Ego caches the identity provider's
	// public signing keys (JWKS) before re-fetching them. Longer TTLs reduce network
	// round-trips; shorter TTLs pick up key rotations faster.
	// Must be a Go duration string. Default: "1h".
	OAuthJWKSCacheTTLSetting = OAuthKeyPrefix + "jwks.cache.ttl"

	// OAuthPermissionMapSetting is a comma-separated list of "scope=permission" pairs
	// that map OAuth2 scope strings to Ego permission names. Any scope not listed here
	// is ignored. When this setting is empty, a built-in default table is used.
	// Example: "ego:admin=root,ego:write=tables,ego:read=logon,ego:code=code_run".
	OAuthPermissionMapSetting = OAuthKeyPrefix + "permission.map"
)

// ValidSettings describes the list of valid settings, and whether they can be set by the
// command line. This is used by the config subcommand to determine if the setting is allowed
// to be set by the user (this test is only done for settings with the "ego." prefix.)
var ValidSettings map[string]bool = map[string]bool{
	ConsolePromptMissingOptions:     true,
	EgoPathSetting:                  true,
	EgoLibPathSetting:               true,
	CaseNormalizedSetting:           true,
	OutputFormatSetting:             true,
	ExtensionsEnabledSetting:        true,
	AutoImportSetting:               true,
	NoCopyrightSetting:              false,
	ConsoleHistorySetting:           true,
	AutoHelpConfigSetting:           true,
	UseReadlineSetting:              true,
	FullStackListingSetting:         true,
	StaticTypesSetting:              true,
	TypeShadowingSetting:            true,
	ApplicationServerSetting:        false,
	LogonServerSetting:              true,
	LogonTokenSetting:               false,
	LogonTokenExpirationSetting:     false,
	LogonRefreshTokenSetting:        false,
	OAuthCLIServerSetting:           true,
	OAuthCLIClientIDSetting:         true,
	OAuthCLIScopesSetting:           true,
	DefaultCredentialSetting:        true,
	LogonSuperuserSetting:           true,
	LogonUserdataSetting:            true,
	LogonUserdataKeySetting:         true,
	ServerTokenKeySetting:           true,
	ServerTokenExpirationSetting:    true,
	ThrowUncheckedErrorsSetting:     true,
	FullStackTraceSetting:           true,
	LogTimestampFormat:              true,
	LogArchiveSetting:               true,
	SandboxPathSetting:              true,
	PidDirectorySetting:             true,
	RestClientTimeoutSetting:        true,
	InsecureServerSetting:           true,
	MaxCacheSizeSetting:             true,
	RestClientErrorSetting:          true,
	LogRetainCountSetting:           true,
	RuntimePanicsSetting:            true,
	TablesServerEmptyFilterError:    true,
	TablesServerEmptyRowsetError:    true,
	ServerDefaultLogSetting:         true,
	TableServerPartialInsertError:   true,
	SymbolTableAllocationSetting:    true,
	ExecPermittedSetting:            true,
	OptimizerSetting:                true,
	ChildServicesSetting:            true,
	ChildRequestDirSetting:          true,
	ChildRequestRetainSetting:       true,
	ChildRequestLimitSetting:        true,
	DefaultDataSourceSetting:        true,
	RestClientServerCert:            true,
	RuntimeDeepScopeSetting:         true,
	TableAutoParseDSNSetting:        true,
	PrecisionErrorSetting:           true,
	UnusedVarsSetting:               true,
	UnknownVarSetting:               true,
	UnusedVarLoggingSetting:         true,
	ServerReportFQDNSetting:         true,
	LogFormatSetting:                true,
	MemoryLogIntervalSetting:        true,
	ServerDefaultLogFileNameSetting: true,
	ServerStartLogAgeSetting:        true,
	JSMinifySetting:                 true,
	JSShortVarNamesSetting:          true,
	PlaintextPasswordSetting:        true,
	WebAuthnRPIDSetting:             true,
	WebAuthnAllowPasskeysSetting:    true,
	ServerReadHeaderTimeoutSetting:  true,
	ServerReadTimeoutSetting:        true,
	ServerWriteTimeoutSetting:       true,
	ServerIdleTimeoutSetting:        true,
	ServerMaxBodySizeSetting:        true,
	ServerMaxItemLimitSetting:       true,
	ClusterNameSetting:              true,
	ClusterPingIntervalSetting:      true,
	ClusterPingTimeoutSetting:       true,
	// OAuth2 Authorization Server settings — all user-settable.
	OAuthASEnabledSetting:           true,
	OAuthASKeyFileSetting:           true,
	OAuthASClientFileSetting:        true,
	OAuthASIssuerSetting:            true,
	OAuthASTokenExpirationSetting:   true,
	OAuthASRefreshExpirationSetting: true,
	OAuthASCodeExpirationSetting:    true,
	// OAuth2 Resource Server settings — all user-settable.
	OAuthProviderSetting:        true,
	OAuthClientIDSetting:        true,
	OAuthClientSecretSetting:    true,
	OAuthScopesSetting:          true,
	OAuthRedirectURISetting:     true,
	OAuthUserClaimSetting:       true,
	OAuthPermissionClaimSetting: true,
	OAuthAudienceSetting:        true,
	OAuthModeSetting:            true,
	OAuthJWKSCacheTTLSetting:    true,
	OAuthPermissionMapSetting:   true,
	ServerDefaultPortSetting:    true,
	ServerInsecureRedirect:      true,
}

// RestrictedSettings is a list of settings that cannot be read using the
// Ego package for reading profile data. This prevents user injection of code
// that could compromise security. Note that not all settings are in this
// category, only those that contains keys or other secure information.
var RestrictedSettings map[string]bool = map[string]bool{
	ServerTokenKeySetting:    true,
	LogonTokenSetting:        true,
	LogonRefreshTokenSetting: true,
	LogonUserdataKeySetting:  true,
	ConsoleHistorySetting:    true,
	LogArchiveSetting:        true,
	EgoDefaultLogFileName:    true,
	RestClientServerCert:     true,
}
