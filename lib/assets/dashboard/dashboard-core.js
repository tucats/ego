// dashboard-core.js
// Foundations used by every other dashboard file: cookie helpers, the
// persisted settings layer, token storage, the inactivity timer, the
// authenticated fetch wrapper, and overlay dismissal.
//
// This file must load first -- everything else assumes these exist.
//
// THE DASHBOARD (HTML, CSS, AND JAVASCRIPT) WERE PROTOTYPED BY CLAUDE
// CODE, and extended by both Claude Code and human developers. The dashboard
// code is reviewed and tested by humans before any changes are committed.
// The dashboard uses api endpoints in the Ego  server that were written by
// humans, as is the rest of the Ego server.
//
// LOAD ORDER MATTERS. These files are plain <script> tags, not modules, so
// they all share one global scope -- but a function declaration is hoisted
// only within its own file. Anything that runs immediately at the top level
// may therefore only call functions declared in the same file or an earlier
// one. Deferred code (event handlers, callbacks, timers) is unrestricted,
// because by the time it runs every file has loaded. dashboard.html lists the
// files in this order:
//
//     dashboard-core.js        cookies, settings, token, idle timer, fetch
//     dashboard-admin.js       tab loaders, DSN permission and config sheets
//     dashboard-data.js        Data tab and its row editor
//     dashboard-sql.js         SQL tab, highlighting, and the SQL formatter
//     dashboard-sqlwizard.js   Build wizard and the SQL statement parser
//     dashboard-ui.js          tab switching, login, user/DSN sheets, log tab
//     dashboard-code.js        Code tab: editor, run, debugger, console
//     dashboard-startup.js     entry point, then passkey support
//
// Note also that names declared at the top level of any of these files are
// shared across all of them. The minifier deliberately never renames such
// names (see internal/util/javascript/minify.go), which is what makes serving
// them as separate files safe.
//
// ==========================================================================
// Cookie helpers
//
// Thin wrappers around document.cookie so the rest of the code doesn't need
// to deal with cookie string parsing directly.
// ==========================================================================

// Attributes applied to every cookie written by this dashboard:
//   Secure       — transmitted only over HTTPS (same scheme as the dashboard)
//   SameSite=Strict — never sent on cross-site requests (same host)
//   path=/       — scoped to the entire site (no Domain attribute, so the
//                  browser restricts it to the exact host — no subdomains)
//
// Browsers do not support port-level cookie isolation, so Secure +
// SameSite=Strict + no Domain is the tightest restriction available.
//
// Secure is only appended when the dashboard itself was loaded over HTTPS.
// Browsers silently refuse to *set* a cookie carrying the Secure attribute
// from a plain-HTTP page — so when the server is run without TLS (e.g.
// `ego server start -k`), unconditionally including Secure meant every
// cookie write from this file was a silent no-op and nothing ever persisted.
const COOKIE_SECURE = location.protocol === 'https:' ? '; Secure' : '';
const COOKIE_ATTRS = '; path=/; SameSite=Strict' + COOKIE_SECURE;

// Write a cookie. maxAgeSeconds sets when it expires (omit or 0 for session).
function setCookie(name, value, maxAgeSeconds) {
    let cookie = encodeURIComponent(name) + '=' + encodeURIComponent(value) + COOKIE_ATTRS;
    if (maxAgeSeconds) cookie += '; max-age=' + maxAgeSeconds;
    document.cookie = cookie;
}

// Read a cookie by name. Returns the value string, or null if not set.
function getCookie(name) {
    const prefix = encodeURIComponent(name) + '=';
    for (const part of document.cookie.split(';')) {
        const trimmed = part.trim();
        if (trimmed.startsWith(prefix)) {
            return decodeURIComponent(trimmed.slice(prefix.length));
        }
    }
    return null;
}

// Delete a cookie by setting its max-age to 0.
// Must use the same attributes as setCookie so the browser matches the cookie.
function deleteCookie(name) {
    document.cookie = encodeURIComponent(name) + '=; max-age=0' + COOKIE_ATTRS;
}

// ==========================================================================
// Settings — persisted as browser cookies
// ==========================================================================

const COOKIE_TOKEN           = 'ego_dashboard_token';
const COOKIE_REMEMBER        = 'ego_dashboard_remember';
const COOKIE_DARK_MODE       = 'ego_dashboard_dark';
const COOKIE_ACTIVE_TAB      = 'ego_dashboard_tab';
const COOKIE_LOG_TAIL        = 'ego_dashboard_log_tail';
const COOKIE_LOG_FILTER      = 'ego_dashboard_log_filter'; // JSON: {session, msg, classes}
const COOKIE_ROLE            = 'ego_dashboard_role'; // 'admin', or a comma-separated list of 'serveradmin'/'coder'/'sql', or ''
const COOKIE_IDENTITY        = 'ego_dashboard_identity'; // logged-in username, for display only
const COOKIE_SHOW_CONSOLE    = 'ego_dashboard_show_console';
const COOKIE_CODE_FORMAT     = 'ego_dashboard_code_format'; // reformat editor source before Run/Debug (default OFF)
const COOKIE_TOOLBAR_STYLE   = 'ego_dashboard_toolbar_style'; // 'text' (icon+label, default) or 'icons' (icon only)
const COOKIE_PASSKEY_OFFERED = 'ego_dashboard_passkey_no'; // set when user says "don't ask again"
const COOKIE_USE_PASSKEYS    = 'ego_dashboard_use_passkeys'; // user preference: use passkeys (default ON)
const COOKIE_IDLE_TIMEOUT    = 'ego_dashboard_idle_timeout'; // server-provided duration string (e.g. "15m"), from login
const TOKEN_MAX_AGE          = 86400;      // 24 hours in seconds
const PASSKEY_NO_MAX_AGE     = 7776000;    // 90 days in seconds
// UI preferences (dark mode, remember-login choice, active tab, etc.) carry
// no credential material, so there is no security reason to cap their
// lifetime the way the bearer token is capped -- give them a long, effectively
// "until changed" lifetime so they survive closing and reopening the browser.
const SETTINGS_MAX_AGE       = 31536000;   // 1 year in seconds

// Tab IDs whose visibility depends on the logged-in user's permissions.
// 'dsns', 'tables', and 'data' are deliberately absent -- every user who can
// log in at all is allowed to use them, regardless of role. See
// tabPermitted() below for the rule applied to each tab in this list.
const PERMISSION_TABS = ['memory', 'users', 'log', 'sql', 'code', 'tasks'];

// Permission-string constants understood by the dashboard. Must match the
// server's internal/defs/permissions.go values.
const PERM_ROOT         = 'ego.root';
const PERM_SERVER_ADMIN = 'ego.server.admin';
const PERM_SQL          = 'ego.sql';
const PERM_CODE         = 'ego.code';
const PERM_DSN_ADMIN    = 'ego.dsn.admin';

// Derive the dashboard's role booleans from the permissions list returned
// by a successful login (POST /services/admin/logon, or the passkey
// login/finish endpoint). ego.root grants every dashboard privilege; the
// other permissions each unlock one additional area on top of the baseline
// every logged-in user gets (DSNs, Tables, and Data).
function rolesFromPermissions(permissions) {
    const perms = permissions || [];
    const admin = perms.includes(PERM_ROOT);

    return {
        admin:       admin,
        serverAdmin: admin || perms.includes(PERM_SERVER_ADMIN),
        coder:       admin || perms.includes(PERM_CODE),
        sql:         admin || perms.includes(PERM_SQL),
        dsnAdmin:    admin || perms.includes(PERM_DSN_ADMIN),
    };
}

// EGO_LANG holds the language code explicitly requested via ?lang= when the
// dashboard was fetched (e.g. /ui?lang=fr).  The server validates the value,
// substitutes it for the __EGO_LANG__ placeholder in dashboard.html, and embeds
// it in a <meta name="ego-lang"> tag before sending the page.
//
// When the operator did not supply ?lang=, or supplied an unsupported code, the
// server leaves the meta tag content empty.  In that case EGO_LANG is '' here,
// and apiFetch() deliberately omits the Accept-Language header so the browser
// can send its own native value — reflecting the user's actual OS/browser locale
// settings — rather than the server silently assuming a default language.
const EGO_LANG = document.querySelector('meta[name="ego-lang"]')?.content || '';

// Return the number of log lines to fetch (stored as a cookie, default 500).
function getLogTail() {
    // parseInt(string, 10) converts a string to a whole number in base 10 (decimal).
    // Always pass the second argument (the radix) to prevent misinterpretation
    // of strings with leading zeros, which some engines read as octal (base 8).
    const v = parseInt(getCookie(COOKIE_LOG_TAIL), 10);
    return v > 0 ? v : 500;
}

// Save the number of log lines to fetch.
function setLogTail(value) {
    setCookie(COOKIE_LOG_TAIL, String(value), SETTINGS_MAX_AGE);
}

// The server-side log filter currently in effect.
//
// session is a number (0 means every session), msg is a wildcard pattern (''
// means every message), and classes is an array of logging class names (an
// empty array means every class). archive asks the server to also read past
// the active log file into older rolled-over files and the zip archive, if
// one is configured. since and until are RFC 3339 timestamp strings ('' means
// no bound on that end) that restrict results to a time range. serverId is a
// glob pattern ('' means every server) matched against the writing server's
// UUID -- it only means anything together with archive (the active log file
// is written by one running process, so every entry in it already shares one
// ID), so the UI never lets it be set while archive is off; see
// applyLogFilter() and the archive checkbox's change handler. These seven are
// sent to the server as query parameters; the line count is kept separately
// in its own cookie because it is a lasting preference rather than part of a
// filter the user clears.
let logFilterState = { session: 0, msg: '', classes: [], archive: false, since: '', until: '', serverId: '' };

// Read the saved filter back out of its cookie. A filter is worth remembering
// across a page reload -- otherwise switching tabs and back silently throws it
// away -- but not worth remembering for a year like the display preferences
// are, so it rides on the same settings lifetime and is easy to clear.
function loadLogFilter() {
    // JSON.parse turns the stored text back into an object. It throws on
    // malformed input, so a cookie corrupted or written by an older version of
    // the dashboard falls back to "no filter" rather than breaking the tab.
    try {
        const saved = JSON.parse(getCookie(COOKIE_LOG_FILTER) || '{}');
        const archive = saved.archive === true;

        logFilterState = {
            session:  parseInt(saved.session, 10) > 0 ? parseInt(saved.session, 10) : 0,
            msg:      typeof saved.msg === 'string' ? saved.msg : '',
            classes:  Array.isArray(saved.classes) ? saved.classes : [],
            archive:  archive,
            since:    typeof saved.since === 'string' ? saved.since : '',
            until:    typeof saved.until === 'string' ? saved.until : '',
            // Only trusted when archive is also on -- a cookie written by an
            // older dashboard version, or hand-edited, could otherwise smuggle
            // in the one combination the server refuses.
            serverId: archive && typeof saved.serverId === 'string' ? saved.serverId : '',
        };
    } catch (e) {
        logFilterState = { session: 0, msg: '', classes: [], archive: false, since: '', until: '', serverId: '' };
    }
}

// Persist the current filter.
function saveLogFilter() {
    setCookie(COOKIE_LOG_FILTER, JSON.stringify(logFilterState), SETTINGS_MAX_AGE);
}

// Is any filter actually restricting what comes back?
function isLogFilterActive() {
    return logFilterState.session > 0 ||
           logFilterState.msg !== '' ||
           logFilterState.classes.length > 0 ||
           logFilterState.archive ||
           logFilterState.since !== '' ||
           logFilterState.until !== '' ||
           logFilterState.serverId !== '';
}

// Load the "remember login" preference from its cookie (default: false).
function getRememberLogin() {
    return getCookie(COOKIE_REMEMBER) === '1';
}

// Save the "remember login" preference. Turning it off immediately forgets
// any already-persisted session (token + role) instead of merely leaving it
// untouched -- otherwise a stale token cookie from an earlier "remembered"
// login (possibly a different user) would still be silently restored the
// next time the dashboard loads, even though the checkbox now shows unchecked.
function setRememberLogin(value) {
    setCookie(COOKIE_REMEMBER, value ? '1' : '0', SETTINGS_MAX_AGE);
    if (!value) {
        deleteCookie(COOKIE_TOKEN);
        deleteCookie(COOKIE_ROLE);
        deleteCookie(COOKIE_IDENTITY);
    }
}

// Load the "dark mode" preference: 'on', 'off', or 'auto' (match the
// browser/OS setting -- see systemPrefersDark). Defaults to 'auto' when the
// cookie is absent. A legacy '1'/'0' value, from before this setting had
// three states, is read back as 'on'/'off' so an existing explicit choice
// carries over instead of silently becoming 'auto'.
function getDarkMode() {
    const v = getCookie(COOKIE_DARK_MODE);
    if (v === '1') return 'on';
    if (v === '0') return 'off';
    return (v === 'on' || v === 'off') ? v : 'auto';
}

// Save the "dark mode" preference and apply it immediately.
function setDarkMode(value) {
    setCookie(COOKIE_DARK_MODE, value, SETTINGS_MAX_AGE);
    applyDarkModeSetting(value);
}

// True when the browser/OS reports a dark color-scheme preference.
// prefers-color-scheme has been part of the standard Media Queries spec
// since 2019 and is supported by every current browser (Safari 12.1+,
// Chrome/Edge 76+, Firefox 67+) -- not a Safari-specific feature.
function systemPrefersDark() {
    return window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches;
}

// Resolve the 'on'/'off'/'auto' setting to an actual true/false and apply it.
function applyDarkModeSetting(setting) {
    applyDarkMode(setting === 'auto' ? systemPrefersDark() : setting === 'on');
}

// If the browser/OS theme changes while "Auto" is selected, follow it live
// instead of waiting for the next page load.
if (window.matchMedia) {
    window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
        if (getDarkMode() === 'auto') applyDarkModeSetting('auto');
    });
}

// Load the "use passkeys" preference from its cookie (default: true — absent cookie means ON).
// When false, passkey UI is suppressed regardless of the server configuration.
function getUsePasskeys() {
    const v = getCookie(COOKIE_USE_PASSKEYS);
    return v === null || v === '1'; // default ON when cookie is absent
}

// Save the "use passkeys" preference and re-apply all passkey UI immediately.
function setUsePasskeys(value) {
    setCookie(COOKIE_USE_PASSKEYS, value ? '1' : '0', SETTINGS_MAX_AGE);
    applyPasskeyLoginUI();
}

// passkeysActive returns true only when BOTH the server has passkeys enabled AND
// the user has not turned them off in Settings.  Use this everywhere instead of
// checking _passkeysEnabled directly.
function passkeysActive() {
    return _passkeysEnabled && getUsePasskeys();
}

// Load the "show console" preference from its cookie (default: true).
function getShowConsole() {
    const v = getCookie(COOKIE_SHOW_CONSOLE);
    return v === null || v === '1'; // default ON when cookie is absent
}

// Save the "show console" preference.
function setShowConsole(value) {
    setCookie(COOKIE_SHOW_CONSOLE, value ? '1' : '0', SETTINGS_MAX_AGE);
}

// Load the "reformat before Run/Debug" preference from its cookie (default:
// false). Unlike Console/passkeys, this defaults OFF when the cookie is
// absent -- it drives a still-new server-side AST formatter, so new users
// have to opt in rather than have it silently rewrite their source.
function getCodeFormat() {
    return getCookie(COOKIE_CODE_FORMAT) === '1';
}

// Save the "reformat before Run/Debug" preference.
function setCodeFormat(value) {
    setCookie(COOKIE_CODE_FORMAT, value ? '1' : '0', SETTINGS_MAX_AGE);
}

// Load the "toolbar button style" preference: 'text' (icon + label, the
// default) or 'icons' (icon only, no label). Applies uniformly to every
// tab's toolbar, including the Log tab, which shows icons only unless this
// is 'text'.
function getToolbarStyle() {
    return getCookie(COOKIE_TOOLBAR_STYLE) === 'icons' ? 'icons' : 'text';
}

// Save the "toolbar button style" preference and apply it immediately.
function setToolbarStyle(value) {
    setCookie(COOKIE_TOOLBAR_STYLE, value, SETTINGS_MAX_AGE);
    applyToolbarStyle(value);
}

// Toggle the body class that CSS uses to collapse every toolbar button down
// to icon-only (or expand every icon-only button up to icon+label). No
// per-button JS is needed: every affected button's label lives in a
// <span class="btn-label">, and this class alone decides whether it's shown.
function applyToolbarStyle(value) {
    document.body.classList.toggle('toolbar-icons-only', value === 'icons');
}

// Apply or remove the dark class on <body>. All tabs, including the Code tab,
// respond to this — the Code tab uses CSS custom properties that are overridden
// by body.dark #code-ui, so no special handling is needed here.
function applyDarkMode(value) {
    document.body.classList.toggle('dark', value);
    const logoSrc = value
        ? 'assets/dashboard/dark-logo.png'
        : 'assets/dashboard/logo.png';
    const logo = document.getElementById('header-logo');
    if (logo) logo.src = logoSrc;
    const loginLogo = document.getElementById('login-logo');
    if (loginLogo) loginLogo.src = logoSrc;
}

// ==========================================================================
// Token storage — in-memory, optionally also persisted as a cookie
//
// The bearer token is always kept in the plain JS variable _token for the
// current session. When the "Remember login" setting is enabled, it is also
// written to a cookie so that a page refresh restores the session without
// requiring a new login.
// ==========================================================================

// The current bearer token. null means the user is not logged in.
let _token = null;

// Server start time string (time.UnixDate format), hostname, version, and
// instance UUID, all captured once from /services/up at page load. Also used
// by fmtUptime() in loadMemory() (Metrics' Uptime row) and by
// showConfigSheet(), which displays all four at the top of the Configuration
// sheet.
let _serverStartTime = null;
let _serverHostName  = null;
let _serverVersion   = null;
let _serverId        = null;

// When the user clicks a DSN row, this is set to the DSN name before openTab('tables')
// is called, so loadTables() can pre-select it in the picker.
let _pendingTablesDsn = null;

// Role flags for the currently logged-in user. All start false; they are
// set by setRole() after a successful login. _isAdmin means the user holds
// ego.root (every privilege); _isServerAdmin additionally covers
// ego.server.admin, which unlocks the Status/Users/Log tabs but not
// Code/SQL. _isDsnAdmin covers ego.dsn.admin, which unlocks creating new
// DSNs (the DSNs tab itself is visible to everyone).
let _isAdmin = false;
let _isServerAdmin = false;
let _isCoder = false;
let _isSql = false;
let _isDsnAdmin = false;

// Whether the server has the scheduled-task subsystem enabled
// (ego.server.tasks.enabled). Loaded via loadTasksFeatureFlag() -- only ever
// attempted for an ego.root user, since GET/POST /admin/config is root-only
// -- after every login and token restore, right before applyTabVisibility()
// is called. Defaults to false so the Tasks tab stays hidden until confirmed.
let _tasksEnabled = false;

// Username of the currently logged-in user. Set on every successful login so
// the edit-user sheet can decide whether to show the passkey registration button
// (passkeys can only be registered by the owner of the account).
let _currentUser = '';

// Whether the server has passkeys enabled (ego.server.allow.passkeys).
// Loaded once at startup from /services/admin/webauthn/config. Defaults to
// false so all passkey UI stays hidden until the server confirms it's on.
let _passkeysEnabled = false;

// Return the current token (or null if not logged in).
function getToken() {
    return _token;
}

// Store a new token. If "remember login" is on, persist it to a cookie too;
// otherwise make sure no stale token cookie from an earlier remembered login
// is left behind to be silently restored on the next page load.
function setToken(token) {
    _token = token;
    if (getRememberLogin()) {
        setCookie(COOKIE_TOKEN, token, TOKEN_MAX_AGE);
    } else {
        deleteCookie(COOKIE_TOKEN);
    }
}

// Discard the token from memory and from any persisted cookie. Also clears
// the stored role so the next login starts from a clean state.
function clearToken() {
    _token = null;
    deleteCookie(COOKIE_TOKEN);
    clearRole();
}

// Encode the current in-memory role flags (_isAdmin/_isServerAdmin/_isCoder/
// _isSql/_isDsnAdmin) as the COOKIE_ROLE string value: 'admin' for full
// access, otherwise a comma-separated list of whichever of
// 'serveradmin'/'coder'/'sql'/'dsnadmin' apply (e.g. 'coder',
// 'serveradmin,sql'), or '' for none of those -- which still leaves the
// baseline DSNs/Tables/Data tabs available. Shared by setRole() and the
// "remember login" toggle handler in dashboard-ui.js so the encoding lives
// in exactly one place.
function roleCookieValue() {
    if (_isAdmin) return 'admin';

    const flags = [];
    if (_isServerAdmin) flags.push('serveradmin');
    if (_isCoder) flags.push('coder');
    if (_isSql) flags.push('sql');
    if (_isDsnAdmin) flags.push('dsnadmin');

    return flags.join(',');
}

// Store the user's role flags. 'admin' means full access (ego.root);
// 'serverAdmin' unlocks Status/Users/Log, 'coder' unlocks Code, 'sql'
// unlocks SQL, 'dsnAdmin' unlocks creating new DSNs -- a non-admin user can
// hold any combination of these, or none, and still use the baseline
// DSNs/Tables/Data tabs. The optional identity parameter records the
// logged-in username.
//
// The role and identity cookies' lifetimes always mirror the token cookie's:
// both persist across restarts when "remember login" is on, and both are
// session-scoped (and cleaned up immediately) when it's off. They used to
// have independent lifetimes (role was always session-only, and identity
// was never persisted at all) which could desync after a restart -- token
// still valid, role/identity gone -- leaving a "logged in" user with no role
// (and therefore no visible tabs) or an unlabeled "Logging in as" line.
function setRole(admin, serverAdmin, coder, sqlUser, dsnAdmin, identity) {
    _isAdmin = !!admin;
    _isServerAdmin = _isAdmin || !!serverAdmin;
    _isCoder = _isAdmin || !!coder;
    _isSql = _isAdmin || !!sqlUser;
    _isDsnAdmin = _isAdmin || !!dsnAdmin;
    if (identity) _currentUser = identity;

    if (getRememberLogin()) {
        setCookie(COOKIE_ROLE, roleCookieValue(), TOKEN_MAX_AGE);
        if (_currentUser) setCookie(COOKIE_IDENTITY, _currentUser, TOKEN_MAX_AGE);
    } else {
        deleteCookie(COOKIE_ROLE);
        deleteCookie(COOKIE_IDENTITY);
    }
}

// Restore role flags and identity from their saved cookies. Called on page
// load when a remembered token is restored.
function restoreRole() {
    const r = getCookie(COOKIE_ROLE) || '';
    const flags = r.split(',');
    _isAdmin = (r === 'admin');
    _isServerAdmin = _isAdmin || flags.includes('serveradmin');
    _isCoder = _isAdmin || flags.includes('coder');
    _isSql = _isAdmin || flags.includes('sql');
    _isDsnAdmin = _isAdmin || flags.includes('dsnadmin');
    _currentUser = getCookie(COOKIE_IDENTITY) || '';
}

// Clear the role state from memory and from the cookie.
function clearRole() {
    _isAdmin = false;
    _isServerAdmin = false;
    _isCoder = false;
    _isSql = false;
    _isDsnAdmin = false;
    _currentUser = '';
    deleteCookie(COOKIE_ROLE);
    deleteCookie(COOKIE_IDENTITY);
}

// Whether the current user is allowed to see tabId. 'memory' (Server),
// 'users', and 'log' require server-admin privilege (ego.root or
// ego.server.admin); 'sql' requires ego.sql (or ego.root); 'code' requires
// ego.code (or ego.root); 'tasks' requires ego.root specifically (matching
// /admin/tasks' own Permissions(RootPermission) gate -- ego.server.admin is
// not enough) AND the server-side ego.server.tasks.enabled setting, since
// the routes themselves don't even exist when that's off. Every tab not in
// PERMISSION_TABS -- DSNs, Tables, Data -- is available to any logged-in user.
function tabPermitted(tabId) {
    switch (tabId) {
        case 'memory':
        case 'users':
        case 'log':
            return _isServerAdmin;
        case 'sql':
            return _isSql;
        case 'code':
            return _isCoder;
        case 'tasks':
            return _isAdmin && _tasksEnabled;
        default:
            return true;
    }
}

// Show or hide the permission-gated tab buttons based on the current user's
// role -- see tabPermitted() above for the rule applied to each. Also shows
// or hides the "+ New DSN" button: the DSNs tab itself is visible to any
// logged-in user, but creating a DSN additionally requires ego.dsn.admin
// (or ego.root). This must be called after every login and after every
// page-load token restore.
function applyTabVisibility() {
    PERMISSION_TABS.forEach(tabId => {
        // Each tab button is a <div> with class matching the tab ID (e.g. class="memory").
        // There is exactly one such element per tab, so querySelector is safe here.
        const btn = document.querySelector('.tab-container .' + tabId);
        if (btn) {
            btn.style.display = tabPermitted(tabId) ? '' : 'none';
        }
    });

    const newDsnBtn = document.getElementById('new-dsn-btn');
    if (newDsnBtn) newDsnBtn.style.display = _isDsnAdmin ? '' : 'none';
}

// Fetch the ego.server.tasks.enabled setting and store it in _tasksEnabled,
// so applyTabVisibility() (called right after this resolves, at every login
// and token-restore call site) knows whether to show the Tasks tab. Only
// attempted for an ego.root user -- POST /admin/config is root-only, so a
// non-root caller would just get a 403 -- and any failure (old server,
// network error, non-root caller) leaves _tasksEnabled false, matching the
// tab's fail-closed default.
async function loadTasksFeatureFlag() {
    _tasksEnabled = false;

    if (!_isAdmin) return;

    try {
        const token = getToken();
        const res = await fetch('admin/config', {
            method:  'POST',
            headers: {
                'Content-Type':  'application/json',
                'Authorization': token ? 'Bearer ' + token : '',
            },
            body: JSON.stringify(['ego.server.tasks.enabled']),
        });

        if (res.ok) {
            const data = await res.json();
            const item = (data.items || {})['ego.server.tasks.enabled'];
            _tasksEnabled = !!item && item.value === 'true';
        }
    } catch (_) {
        // Server unreachable -- leave _tasksEnabled false so the tab stays hidden.
    }
}

// Pick the tab a user without server-admin access should land on: Code if
// they hold ego.code (or ego.root, preferred when they hold both, matching
// the historical "coders land on Code" behavior), otherwise SQL if they
// hold ego.sql, otherwise DSNs -- the first of the tabs every logged-in
// user can see regardless of permissions.
function defaultNonAdminTab() {
    if (_isCoder) return 'code';
    if (_isSql) return 'sql';

    return 'dsns';
}

// ==========================================================================
// Inactivity timer
//
// If the user does nothing for idleTimeoutMs the token is automatically
// cleared and the login overlay is shown. Any mouse movement, click, key
// press, or scroll on the page resets the clock.
//
// The timeout duration comes from the server (ego.server.dashboard.inactivity,
// a Go duration string such as "15m"), delivered as part of the logon
// response -- see setIdleTimeout() below, called from both the password and
// passkey login success handlers. It is also cached in a cookie so a
// restored ("Remember login") session uses the same value without needing a
// fresh login. DEFAULT_IDLE_TIMEOUT_MS is only a fallback for the brief
// window before any login has ever supplied a real value (or if the cookie
// is absent/unparseable), and matches the server's own default.
//
// How it works:
//   - setInterval() runs a function repeatedly on a fixed interval.
//     Here we check every minute whether the user has been idle too long.
//   - "Activity" events update the lastActivity timestamp each time they fire.
//   - When the idle check finds that (now - lastActivity) exceeds the
//     timeout, it clears the token and shows the login screen.
// ==========================================================================

const DEFAULT_IDLE_TIMEOUT_MS = 15 * 60 * 1000; // 15 minutes -- matches the server's own default

// Parse a Go-style duration string ("15m", "1h30m", "2d") into milliseconds.
// Supports the same units the server accepts for this setting: d (days, an
// Ego-specific extension -- see util.ParseDuration), h, m, s. Returns null if
// the string contains no recognizable number+unit pair.
function parseDurationMs(str) {
    if (!str) return null;

    const unitMs = { d: 86400000, h: 3600000, m: 60000, s: 1000 };
    const re = /(\d+(?:\.\d+)?)(d|h|m|s)/g;
    let total = 0;
    let matched = false;
    let m;

    while ((m = re.exec(str)) !== null) {
        matched = true;
        total += parseFloat(m[1]) * unitMs[m[2]];
    }

    return matched ? total : null;
}

// The active idle timeout, in milliseconds. Initialized from the cookie left
// by a previous login (if any and if it still parses), so a page reload uses
// the last known server value even before any fresh login response arrives.
let idleTimeoutMs = parseDurationMs(getCookie(COOKIE_IDLE_TIMEOUT)) || DEFAULT_IDLE_TIMEOUT_MS;

// Apply the server's ego.server.dashboard.inactivity value from a logon
// response. Called after every successful password or passkey login. An
// empty or unparseable string is ignored, leaving whatever timeout was
// already in effect.
function setIdleTimeout(durationString) {
    const ms = parseDurationMs(durationString);
    if (ms) {
        idleTimeoutMs = ms;
        setCookie(COOKIE_IDLE_TIMEOUT, durationString, SETTINGS_MAX_AGE);
    }
}

// Record the time of the most recent user activity. Date.now() returns the
// current time as a number (milliseconds since 1 January 1970).
let lastActivity = Date.now();

// Update lastActivity whenever the user interacts with the page.
// We listen on the document (the whole page) for four common activity events.
['mousemove', 'mousedown', 'keydown', 'scroll'].forEach(eventName => {
    document.addEventListener(eventName, () => { lastActivity = Date.now(); }, { passive: true });
    // passive:true is a performance hint — it tells the browser this listener
    // will never call preventDefault(), so it doesn't need to wait for it.
});

// Check for inactivity every 60 seconds. If the gap since the last activity
// exceeds the timeout, treat it as an automatic logoff.
setInterval(() => {
    if (_token && (Date.now() - lastActivity) >= idleTimeoutMs) {
        clearToken();
        showLogin('Signed out due to inactivity.');
    }
}, 60 * 1000); // run this check once per minute

// ==========================================================================
// Authenticated fetch wrapper
//
// All API calls in this dashboard go through apiFetch() rather than calling
// fetch() directly. This ensures every request automatically includes the
// Authorization header with the bearer token, and that a 401 response
// (meaning the token itself is missing or invalid) is handled consistently
// by showing the login overlay. A 403 (valid token, insufficient permission
// for this resource) is left for the caller to handle inline -- see the
// per-status comments below.
//
// "async" means this function returns a Promise and can use "await" inside
// it, allowing asynchronous network calls to be written in a linear style.
// ==========================================================================
async function apiFetch(url) {
    const token = getToken();

    // Build the request headers incrementally so that each header is only
    // present when it actually has a meaningful value to contribute.
    const headers = {};

    // Include the bearer token only when one is available.  Omitting the
    // Authorization header entirely (rather than sending an empty value) is
    // the correct behavior for unauthenticated requests.
    if (token) {
        headers['Authorization'] = 'Bearer ' + token;
    }

    // Include an explicit Accept-Language header only when the operator
    // requested a specific language via ?lang= when loading the dashboard.
    // When EGO_LANG is empty the browser automatically sends its own
    // Accept-Language header based on the user's OS/browser locale settings,
    // which is exactly the behavior we want — no server-imposed default.
    if (EGO_LANG) {
        headers['Accept-Language'] = EGO_LANG;
    }

    const res = await fetch(url, { headers });

    // 401 = Unauthorized (token missing or invalid) — the session itself is
    // no good, so discard it and send the user back to the login overlay.
    if (res.status === 401) {
        clearToken();
        showLogin('Session expired or invalid. Please sign in again.');
        // Throwing an error stops execution in the calling function and jumps
        // to the nearest catch block, so the caller doesn't try to read a
        // response body that won't make sense.
        throw new Error('Unauthorized');
    }

    // 403 = Forbidden (token is valid, but the user lacks permission for this
    // specific resource). The session is still good, so do NOT log the user
    // out — just throw a distinguishable error and let the caller decide how
    // to surface it (typically an inline "not authorized" message built from
    // the server's response body).
    if (res.status === 403) {
        const data = await res.json().catch(() => ({}));
        const err = new Error(data.msg || 'Forbidden');
        err.status = 403;
        throw err;
    }

    return res;
}

// ==========================================================================
// Overlay backdrop-click dismiss
//
// Every dismissible overlay div carries onclick="overlayBackdropClick(event)".
// The handler fires only when the click lands on the dim backdrop itself
// (event.target === event.currentTarget), not on the sheet inside it.
//
// For editable sheets a baseline snapshot of all form fields is captured when
// the sheet opens (captureBaseline). On dismiss the current field values are
// compared to that snapshot; if anything changed the user is asked to confirm.
// ==========================================================================

// Per-overlay field snapshots.  Key = overlay element ID, value = serialized
// field state captured by captureBaseline().
const _sheetBaseline = {};

// Serialize every input/select/textarea inside an overlay into a single string
// and store it so isSheetModified() can detect later changes.
// Call this at the end of each showX() function, after all fields are set.
function captureBaseline(overlayId) {
    const overlay = document.getElementById(overlayId);
    const fields  = overlay.querySelectorAll('input, select, textarea');
    _sheetBaseline[overlayId] = Array.from(fields).map(f =>
        f.type === 'checkbox' ? String(f.checked) : f.value
    ).join('\x00');
}

// Return true if the current field values inside overlayId differ from the
// snapshot taken when the sheet was opened.
// The data-edit and config-item sheets delegate to their own save-button
// disabled state: data-edit's inputs are built dynamically, and config-item's
// New Value field is hidden entirely for readonly items, so in both cases the
// button's state is already the authoritative change signal.
function isSheetModified(overlayId) {
    if (overlayId === 'data-edit-overlay') {
        const btn = document.getElementById('data-edit-save-btn');
        return btn ? !btn.disabled : false;
    }
    if (overlayId === 'config-item-overlay') {
        const btn = document.getElementById('config-item-save-btn');
        return btn ? !btn.disabled : false;
    }
    const baseline = _sheetBaseline[overlayId];
    if (baseline === undefined) return false;
    const overlay = document.getElementById(overlayId);
    const fields  = overlay.querySelectorAll('input, select, textarea');
    const current = Array.from(fields).map(f =>
        f.type === 'checkbox' ? String(f.checked) : f.value
    ).join('\x00');
    return current !== baseline;
}

// Maps each dismissible overlay ID to its hide function and whether to
// perform a dirty check before dismissing.
const _overlayDismiss = {
    'new-user-overlay':      { hide: () => hideNewUserSheet(),  dirty: true  },
    'edit-user-overlay':     { hide: () => hideEditUserSheet(), dirty: true  },
    'new-dsn-overlay':       { hide: () => hideNewDsnSheet(),   dirty: true  },
    'dsn-perm-edit-overlay': { hide: () => hideDsnPermEdit(),   dirty: true  },
    'dsn-perm-add-overlay':  { hide: () => hideDsnPermAdd(),    dirty: true  },
    'table-perm-edit-overlay': { hide: () => hideTablePermEdit(), dirty: true  },
    'table-perm-add-overlay':  { hide: () => hideTablePermAdd(),  dirty: true  },
    'data-edit-overlay':     { hide: () => hideDataEdit(),      dirty: true  },
    'logger-config-overlay': { hide: () => hideLoggerConfig(),  dirty: true  },
    'log-filter-overlay':    { hide: () => hideLogFilter(),     dirty: true  },
    'dsn-detail-overlay':    { hide: () => hideDsnDetail(),     dirty: false },
    'table-detail-overlay':  { hide: () => hideTableDetail(),   dirty: false },
    'config-overlay':        { hide: () => hideConfigSheet(),   dirty: false },
    'config-item-overlay':   { hide: () => hideConfigItemDetail(), dirty: true  },
    'data-col-overlay':      { hide: () => hideDataColumns(),   dirty: false },
    'settings-overlay':      { hide: () => hideSettings(),      dirty: false },
    'sql-build-overlay':     { hide: () => hideSqlBuild(),      dirty: true  },
    'sql-generate-overlay':  { hide: () => hideSqlGenerate(),   dirty: false },
};

// onclick handler attached to each overlay backdrop div.
// Dismisses the sheet when the user clicks outside the sheet panel.
function overlayBackdropClick(event) {
    if (event.target !== event.currentTarget) return;
    const overlayId = event.currentTarget.id;
    const cfg = _overlayDismiss[overlayId];
    if (!cfg) return;
    if (cfg.dirty && isSheetModified(overlayId)) {
        if (!confirm('Do you wish to discard changes?')) return;
    }
    cfg.hide();
}

