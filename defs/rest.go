package defs

const (
	TableParameterName         = "table"
	SchemaParameterName        = "schema"
	UserParameterName          = "user"
	RowIDs                     = "rowids"
	DSNParameterName           = "dsn"
	ColumnParameterName        = "columns"
	FilterParameterName        = "filter"
	SortParameterName          = "sort"
	StartParameterName         = "start"
	LimitParameterName         = "limit"
	RowCountParameterName      = "rowcounts"
	AbstractParameterName      = "abstract"
	UpsertParameterName        = "upsert"
	PermissionsPseudoTable     = "@permissions"
	SQLPseudoTable             = "@sql"
	ExpiresParameterName       = "expires"
	TransactionIDParameterName = "transaction"
)

const (
	AdminPath           = "/admin/"
	AdminCachesPath     = AdminPath + "caches"
	AdminHeartbeatPath  = AdminPath + "heartbeat"
	AdminLoggersPath    = AdminPath + "loggers/"
	AdminUsersPath      = AdminPath + "users/"
	AdminMemoryPath     = AdminPath + "memory"
	AdminRunPath        = AdminPath + "run"
	AdminTokenPath      = AdminPath + "tokens/"
	AdminResourcesPath  = AdminPath + "resources"
	AdminTokenIDPath    = AdminTokenPath + "{{id}}"
	AdminValidationPath = AdminPath + "validation/"

	AdminUsersNamePath       = AdminUsersPath + "%s"
	AdminConfigPath          = AdminPath + "config"
	AssetsPath               = "/assets/"
	DSNPath                  = "/dsns/"
	DSNNamePath              = DSNPath + "{{dsn}}/"
	DSNBeginPath             = DSNNamePath + "begin"
	DSNCommitPath            = DSNNamePath + "commit"
	DSNRollbackPath          = DSNNamePath + "rollback"
	TablesPath               = DSNNamePath + "tables/"
	TablesNamePath           = TablesPath + "%s"
	TablesRowsPath           = TablesPath + "{{table}}/rows"
	TablesSQLPath            = TablesPath + SQLPseudoTable
	ServicesPath             = "/services/"
	ServicesDownPath         = ServicesPath + "admin/down/"
	ServicesLogonPath        = ServicesPath + "admin/logon"
	ServicesLogLinesPath     = ServicesPath + "admin/log"
	ServicesAuthenticatePath = ServicesPath + "admin/authenticate"

	// WebAuthn (passkey) endpoints — unauthenticated login ceremony and
	// authenticated registration ceremony.
	ServicesWebAuthnLoginBeginPath    = ServicesPath + "admin/webauthn/login/begin"
	ServicesWebAuthnLoginFinishPath   = ServicesPath + "admin/webauthn/login/finish"
	ServicesWebAuthnRegisterBeginPath = ServicesPath + "admin/webauthn/register/begin"
	ServicesWebAuthnRegFinishPath     = ServicesPath + "admin/webauthn/register/finish"
	ServicesWebAuthnClearPasskeysPath = ServicesPath + "admin/webauthn/passkeys/{{name}}"
	ServicesWebAuthnConfigPath        = ServicesPath + "admin/webauthn/config"
	ServicesUpPath                    = ServicesPath + "up/"

	// Cluster control endpoints — used for node-to-node communication within a
	// named cluster. Authentication is via the cluster HMAC token, not normal
	// user credentials.
	ServicesClusterPath         = ServicesPath + "cluster"
	ServicesClusterFlushPath    = ServicesPath + "cluster/flush"
	ServicesClusterShutdownPath = ServicesPath + "cluster/shutdown"
	ServicesClusterRemovePath   = ServicesPath + "cluster/remove"
	TablesPermissionsPath             = TablesPath + PermissionsPseudoTable
	TablesNamePermissionsPath         = TablesPath + "{{table}}/permissions"
	UIPath                            = "/ui"

	// OAuth2 Authorization Server endpoints — registered at the server root using
	// the standard OIDC path conventions so that any compliant client library can
	// discover them automatically from the issuer URL. Only active when
	// ego.server.oauth.as.enabled is true.

	// OAuthDiscoveryPath is the standard OIDC discovery document endpoint.
	// Clients fetch this once to learn all AS endpoint URLs.
	OAuthDiscoveryPath = "/.well-known/openid-configuration"

	// OAuthJWKSPath is the JSON Web Key Set endpoint that publishes the AS's
	// public signing key. Resource Servers fetch this to verify JWT signatures.
	OAuthJWKSPath = "/.well-known/jwks.json"

	// OAuthAuthorizePath is the authorization endpoint. GET serves the login
	// form; POST processes credentials and issues an authorization code.
	OAuthAuthorizePath = "/oauth2/authorize"

	// OAuthTokenPath is the token endpoint where authorization codes are
	// exchanged for JWTs and refresh tokens are used to obtain new access tokens.
	OAuthTokenPath = "/oauth2/token"

	// OAuthUserinfoPath is the OIDC UserInfo endpoint that returns identity
	// claims for the holder of a valid Bearer JWT.
	OAuthUserinfoPath = "/oauth2/userinfo"

	// OAuthRevokePath is the token revocation endpoint (RFC 7009). Clients post
	// a token here to invalidate it; the JTI is added to Ego's blacklist.
	OAuthRevokePath = "/oauth2/revoke"

	// OAuth2 Resource Server endpoints — registered under /services/admin/oauth/
	// and only active when ego.server.oauth.provider is configured.

	// ServicesOAuthPath is the base path for all OAuth2 Resource Server endpoints.
	ServicesOAuthPath = ServicesPath + "admin/oauth/"

	// ServicesOAuthCallbackPath is the redirect target that the identity provider
	// sends the browser to after successful user authentication (Authorization Code
	// flow). Ego extracts the authorization code, validates the state parameter,
	// exchanges the code for a JWT, and then issues a native Ego token (proxy/hybrid
	// modes) or redirects with the JWT (resource-server mode).
	ServicesOAuthCallbackPath = ServicesOAuthPath + "callback"

	// ServicesOAuthAuthorizePath initiates the Authorization Code + PKCE flow by
	// building the identity provider's authorization URL and redirecting the browser.
	// This is the endpoint a dashboard "Sign in with [Your Organization]" button calls.
	ServicesOAuthAuthorizePath = ServicesOAuthPath + "authorize"

	// ServicesOAuthConfigPath returns a sanitized view of the current OAuth2 RS
	// configuration (provider URL, scopes, mode, audience). Requires admin credentials.
	// The client secret is never returned.
	ServicesOAuthConfigPath = ServicesOAuthPath + "config"
)

var TableColumnTypeNames []string = []string{
	"byte",
	"int",
	"int8",
	"int16",
	"int32",
	"int64",
	"string",
	"float",
	"double",
	"float32",
	"float64",
	"time",
	"timestamp",
	"date",
	"bool",
}

const (
	TextMediaType = "application/text"
	JSONMediaType = "application/json"
	HTMLMediaType = "application/html"

	EgoMediaType                  = "application/vnd.ego."
	SQLStatementsMediaType        = EgoMediaType + "sql+json"
	RowSetMediaType               = EgoMediaType + "rows+json"
	AbstractRowSetMediaType       = EgoMediaType + "rows.abstract+json"
	TransactionMediaType          = EgoMediaType + "transaction+json"
	RowCountMediaType             = EgoMediaType + "rowcount+json"
	TableMetadataMediaType        = EgoMediaType + "columns+json"
	TablesMediaType               = EgoMediaType + "tables+json"
	ErrorMediaType                = EgoMediaType + "error+json"
	UserMediaType                 = EgoMediaType + "user+json"
	DSNMediaType                  = EgoMediaType + "dsn+json"
	DSNPermissionsType            = EgoMediaType + "dsn.permissions+json"
	DSNListPermsMediaType         = EgoMediaType + "dsn.permissions.list+json"
	DSNListMediaType              = EgoMediaType + "dsns+json"
	UsersMediaType                = EgoMediaType + "users+json"
	LogStatusMediaType            = EgoMediaType + "log.status+json"
	LogLinesJSONMediaType         = EgoMediaType + "log.lines+json"
	LogLinesTextMediaType         = EgoMediaType + "log.lines+text"
	CacheMediaType                = EgoMediaType + "cache+json"
	MemoryMediaType               = EgoMediaType + "memory+json"
	ResourcesMediaType            = EgoMediaType + "resources+json"
	LogonMediaType                = EgoMediaType + "logon+json"
	ConfigListMediaType           = EgoMediaType + "config.list+json"
	ConfigMediaType               = EgoMediaType + "config+json"
	ValidationDictionaryMediaType = EgoMediaType + "validation.dictionary+json"
	TokensMediaType               = EgoMediaType + "tokens.list+json"
	TransactionResponseMediaType  = EgoMediaType + "transaction.response+json"
)

const (
	UserAuthenticationRequired  = "user"
	TokenRequired               = "token"
	AdminAuthenticationRequired = "admin"
	NoAuthenticationRequired    = "none"
	AdminTokenRequired          = "admintoken"
)

const (
	ContentTypeHeader  = "Content-Type"
	AuthenticateHeader = "WWW-Authenticate"
)

const ServerStoppedMessage = "Server stopped"

// InstanceID is the UUID of the current Server Instance.
var InstanceID string
