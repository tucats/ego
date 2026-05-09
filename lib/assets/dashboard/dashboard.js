// dashboard.js
// Client-side logic for the Ego admin dashboard. Handles authentication,
// tab switching, and data loading for all tabs: Status, Memory, Users,
// DSNs, Tables, Data, SQL, Log, and Code.
//
// THE DASHBOARD (HTML, CSS, AND JAVASCRIPT) WERE PROTOTYPED BY CLAUDE
// CODE, and extended by both Claude Code and human developers. The dashboard
// code is reviewed and tested by humans before any changes are committed.
// The dashboard uses api endpoints in the Ego  server that were written by
// humans, as is the rest of the Ego server.
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
const COOKIE_ATTRS = '; path=/; SameSite=Strict; Secure';

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
const COOKIE_ROLE            = 'ego_dashboard_role'; // 'admin', 'coder', or ''
const COOKIE_SHOW_CONSOLE    = 'ego_dashboard_show_console';
const COOKIE_PASSKEY_OFFERED = 'ego_dashboard_passkey_no'; // set when user says "don't ask again"
const COOKIE_USE_PASSKEYS    = 'ego_dashboard_use_passkeys'; // user preference: use passkeys (default ON)
const TOKEN_MAX_AGE          = 86400;      // 24 hours in seconds
const PASSKEY_NO_MAX_AGE     = 7776000;    // 90 days in seconds

// Tab IDs that are only visible to admin users. Coder-only users see just 'code'.
const ADMIN_ONLY_TABS = ['memory', 'users', 'dsns', 'tables', 'data', 'sql', 'log'];

// Return the number of log lines to fetch (stored as a cookie, default 500).
function getLogTail() {
    // parseInt(string, 10) converts a string to a whole number in base 10 (decimal).
    // Always pass the second argument (the radix) to prevent misinterpretation
    // of strings with leading zeros, which some engines read as octal (base 8).
    const v = parseInt(getCookie(COOKIE_LOG_TAIL), 10);
    return v > 0 ? v : 500;
}

// Load the "remember login" preference from its cookie (default: false).
function getRememberLogin() {
    return getCookie(COOKIE_REMEMBER) === '1';
}

// Save the "remember login" preference.
function setRememberLogin(value) {
    setCookie(COOKIE_REMEMBER, value ? '1' : '0');
}

// Load the "dark mode" preference from its cookie (default: false).
function getDarkMode() {
    return getCookie(COOKIE_DARK_MODE) === '1';
}

// Save the "dark mode" preference and apply it immediately.
function setDarkMode(value) {
    setCookie(COOKIE_DARK_MODE, value ? '1' : '0');
    applyDarkMode(value);
}

// Load the "use passkeys" preference from its cookie (default: true — absent cookie means ON).
// When false, passkey UI is suppressed regardless of the server configuration.
function getUsePasskeys() {
    const v = getCookie(COOKIE_USE_PASSKEYS);
    return v === null || v === '1'; // default ON when cookie is absent
}

// Save the "use passkeys" preference and re-apply all passkey UI immediately.
function setUsePasskeys(value) {
    setCookie(COOKIE_USE_PASSKEYS, value ? '1' : '0');
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
    setCookie(COOKIE_SHOW_CONSOLE, value ? '1' : '0');
}

// Apply or remove the dark class on <body>. All tabs, including the Code tab,
// respond to this — the Code tab uses CSS custom properties that are overridden
// by body.dark #code-ui, so no special handling is needed here.
function applyDarkMode(value) {
    document.body.classList.toggle('dark', value);
    const logo = document.getElementById('header-logo');
    if (logo) {
        logo.src = value
            ? '/assets/dashboard/dark-logo.png'
            : '/assets/dashboard/logo.png';
    }
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

// Server start time string (time.UnixDate format) captured from /services/up,
// used by loadMemory() to display uptime without an extra fetch.
let _serverStartTime = null;

// When the user clicks a DSN row, this is set to the DSN name before openTab('tables')
// is called, so loadTables() can pre-select it in the picker.
let _pendingTablesDsn = null;

// Role flags for the currently logged-in user. Both start false; they are
// set by setRole() after a successful login.
let _isAdmin = false;
let _isCoder = false;

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

// Store a new token. If "remember login" is on, persist it to a cookie too.
function setToken(token) {
    _token = token;
    if (getRememberLogin()) {
        setCookie(COOKIE_TOKEN, token, TOKEN_MAX_AGE);
    }
}

// Discard the token from memory and from any persisted cookie. Also clears
// the stored role so the next login starts from a clean state.
function clearToken() {
    _token = null;
    deleteCookie(COOKIE_TOKEN);
    clearRole();
}

// Store the user's role flags. 'admin' means full access; 'coder' means the
// Code tab only. The role is written to a session cookie so that a page
// refresh restores the correct tab visibility without requiring a new login.
// The optional identity parameter records the logged-in username.
function setRole(admin, coder, identity) {
    _isAdmin = !!admin;
    _isCoder = !!coder;
    if (identity) _currentUser = identity;
    // Store as a single string so one cookie covers both flags.
    const roleValue = _isAdmin ? 'admin' : (_isCoder ? 'coder' : '');
    setCookie(COOKIE_ROLE, roleValue);
}

// Restore role flags from the saved cookie. Called on page load when a
// remembered token is restored.
function restoreRole() {
    const r = getCookie(COOKIE_ROLE);
    _isAdmin = (r === 'admin');
    _isCoder = (r === 'coder') || _isAdmin;
}

// Clear the role state from memory and from the cookie.
function clearRole() {
    _isAdmin = false;
    _isCoder = false;
    deleteCookie(COOKIE_ROLE);
}

// Show or hide the admin-only tab buttons based on the current user's role.
// A coder-only user sees only the Code tab; an admin user sees all tabs.
// This must be called after every login and after every page-load token restore.
function applyTabVisibility() {
    ADMIN_ONLY_TABS.forEach(tabId => {
        // Each tab button is a <div> with class matching the tab ID (e.g. class="memory").
        // There is exactly one such element per tab, so querySelector is safe here.
        const btn = document.querySelector('.tab-container .' + tabId);
        if (btn) {
            btn.style.display = _isAdmin ? '' : 'none';
        }
    });
}

// ==========================================================================
// Inactivity timer
//
// If the user does nothing for 15 minutes the token is automatically
// cleared and the login overlay is shown. Any mouse movement, click, key
// press, or scroll on the page resets the clock.
//
// How it works:
//   - setInterval() runs a function repeatedly on a fixed interval.
//     Here we check every minute whether the user has been idle too long.
//   - "Activity" events update the lastActivity timestamp each time they fire.
//   - When the idle check finds that (now - lastActivity) exceeds the
//     timeout, it clears the token and shows the login screen.
// ==========================================================================

const IDLE_TIMEOUT_MS = 15 * 60 * 1000; // 15 minutes expressed in milliseconds

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
    if (_token && (Date.now() - lastActivity) >= IDLE_TIMEOUT_MS) {
        clearToken();
        showLogin('Signed out after 15 minutes of inactivity.');
    }
}, 60 * 1000); // run this check once per minute

// ==========================================================================
// Authenticated fetch wrapper
//
// All API calls in this dashboard go through apiFetch() rather than calling
// fetch() directly. This ensures every request automatically includes the
// Authorization header with the bearer token, and that a 401/403 response
// (meaning the server rejected the token) is handled consistently by showing
// the login overlay.
//
// "async" means this function returns a Promise and can use "await" inside
// it, allowing asynchronous network calls to be written in a linear style.
// ==========================================================================
async function apiFetch(url) {
    const token = getToken();

    // Add the Authorization header only when we actually have a token.
    // The ternary expression (condition ? valueIfTrue : valueIfFalse) either
    // builds the header object or returns an empty object {}.
    const res = await fetch(url, {
        headers: token ? { 'Authorization': 'Bearer ' + token } : {}
    });

    // 401 = Unauthorized (token missing or invalid)
    // 403 = Forbidden (token valid but user lacks permission)
    // Either way, discard the stale token and ask the user to log in again.
    if (res.status === 401 || res.status === 403) {
        clearToken();
        showLogin('Session expired or invalid. Please sign in again.');
        // Throwing an error stops execution in the calling function and jumps
        // to the nearest catch block, so the caller doesn't try to read a
        // response body that won't make sense.
        throw new Error('Unauthorized');
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
// The data-edit sheet delegates to its own save-button disabled state because
// its inputs are built dynamically and already have perfect change detection.
function isSheetModified(overlayId) {
    if (overlayId === 'data-edit-overlay') {
        const btn = document.getElementById('data-edit-save-btn');
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
    'data-edit-overlay':     { hide: () => hideDataEdit(),      dirty: true  },
    'logger-config-overlay': { hide: () => hideLoggerConfig(),  dirty: true  },
    'dsn-detail-overlay':    { hide: () => hideDsnDetail(),     dirty: false },
    'table-detail-overlay':  { hide: () => hideTableDetail(),   dirty: false },
    'config-overlay':        { hide: () => hideConfigSheet(),   dirty: false },
    'data-col-overlay':      { hide: () => hideDataColumns(),   dirty: false },
    'settings-overlay':      { hide: () => hideSettings(),      dirty: false },
    'sql-build-overlay':     { hide: () => hideSqlBuild(),      dirty: true  },
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

// ==========================================================================
// Tab content loaders
//
// Each function is responsible for one tab: it fetches data from the server,
// builds an HTML table string, and injects it into the tab's container div.
// They are called by openTab() every time a tab is selected, so the data is
// always refreshed when you switch tabs.
// ==========================================================================

// Load the Memory tab — fetches server memory statistics and cache data,
// rendering both into the combined Memory tab.
async function loadMemory() {
    // Convert a raw byte count into a readable string like "3.14 MB".
    function fmtBytes(n) {
        if (n >= 1073741824) return (n / 1073741824).toFixed(2) + ' GB';
        if (n >= 1048576)    return (n / 1048576).toFixed(2)    + ' MB';
        if (n >= 1024)       return (n / 1024).toFixed(2)       + ' KB';
        return n + ' B';
    }

    // Compute a human-readable uptime string from the server start-time string
    // (time.UnixDate format, e.g. "Mon Jan  2 15:04:05 MST 2006").
    function fmtUptime(since) {
        if (!since) return '—';
        const start = new Date(since);
        if (isNaN(start.getTime())) return '—';
        const ms    = Date.now() - start.getTime();
        const secs  = Math.floor(ms / 1000);
        const mins  = Math.floor(secs / 60);
        const hours = Math.floor(mins / 60);
        const days  = Math.floor(hours / 24);
        if (days  > 0) return days  + 'd ' + (hours % 24) + 'h ' + (mins % 60) + 'm';
        if (hours > 0) return hours + 'h ' + (mins % 60)  + 'm';
        if (mins  > 0) return mins  + 'm ' + (secs % 60)  + 's';
        return secs + 's';
    }

    // Build a label+value pair of <td> cells for a stat grid row.
    function pair(label, value) {
        return '<td class="stat-lbl">' + label + '</td><td class="stat-val">' + value + '</td>';
    }

    // Empty label+value placeholder used to fill out short rows.
    function emptyPair() { return '<td></td><td></td>'; }

    // ---- Single call to /admin/resources returns both memory and cache data ----
    const memContainer   = document.getElementById('memory-content');
    const cacheContainer = document.getElementById('caches-content');
    try {
        const res = await apiFetch('/admin/resources');
        const d   = await res.json();

        const sep = '<td class="stat-sep"></td>';

        // Metrics — 3-column compact grid (same 8-cell row structure as Cache Status below)
        let memHtml = '<table class="stat-grid"><thead><tr><th colspan="8">Metrics</th></tr></thead><tbody>';
        memHtml += '<tr>' + pair('Uptime',             fmtUptime(_serverStartTime))       + sep + pair('Objects in Use',  d.objects.toLocaleString()) + sep + pair('Application Memory', fmtBytes(d.system))  + '</tr>';
        memHtml += '<tr>' + pair('Requests Processed', d.server.session.toLocaleString()) + sep + pair('Heap Memory',     fmtBytes(d.current))        + sep + pair('Stack Memory',        fmtBytes(d.stack))   + '</tr>';
        memHtml += '<tr>' + pair('GC Cycles',          d.gc.toLocaleString())             + sep + emptyPair()                                         + sep + emptyPair()                                       + '</tr>';
        memHtml += '</tbody></table>';
        memContainer.innerHTML = memHtml;

        // Cache Status — 3-column compact grid
        let cacheHtml = '<table class="stat-grid"><thead><tr><th colspan="8">Cache Status</th></tr></thead><tbody>';
        cacheHtml += '<tr>' + pair('DSN Entries',         d.dsnCount)                    + sep + pair('Cached Services',    d.serviceCount)               + sep + pair('Authorizations',  d.authorizationCount) + '</tr>';
        cacheHtml += '<tr>' + pair('Schema Entries',      d.schemaCount)                 + sep + pair('Service Cache Size', d.serviceSize + '&nbsp;items') + sep + pair('Tokens',          d.tokenCount)         + '</tr>';
        cacheHtml += '<tr>' + pair('Code Run Sessions',   d.runCount)                    + sep + pair('Cached Assets',      d.assetCount)                 + sep + pair('Blacklist Status', d.blacklistCount)     + '</tr>';
        cacheHtml += '<tr>' + pair('Code Debug Sessions', d.debugCount)                  + sep + pair('Asset Cache size',   fmtBytes(d.assetSize))        + sep + emptyPair()                                    + '</tr>';
        cacheHtml += '</tbody></table>';

        const items = d.items || [];
        if (items.length > 0) {
            cacheHtml += '<hr class="stat-divider"><table><thead><tr>'
                       + '<th>Cached Endpoints</th><th>Class</th><th>Reuse count</th><th class="status-val">Size</th><th>Last accessed</th>'
                       + '</tr></thead><tbody>';
            for (const item of items) {
                const lastStr = item.last ? new Date(item.last).toLocaleString() : '';
                const sizeStr = item.class === 'asset' ? fmtBytes(item.size || 0) : '';
                cacheHtml += '<tr>'
                           + '<td>' + escapeHtml(item.name)  + '</td>'
                           + '<td>' + escapeHtml(item.class) + '</td>'
                           + '<td>' + item.count             + '</td>'
                           + '<td class="status-val">' + sizeStr + '</td>'
                           + '<td>' + lastStr                + '</td>'
                           + '</tr>';
            }
            cacheHtml += '</tbody></table>';
        }

        cacheContainer.innerHTML = cacheHtml;
    } catch (e) {
        if (e.message !== 'Unauthorized') console.error('Error loading status:', e);
    }
}

// Load the Users tab — fetches the user list and renders it as a table
// with columns for username and permissions.
async function loadUsers() {
    const container = document.getElementById('user-content');
    try {
        const res   = await apiFetch('/admin/users');
        const data  = await res.json();

        // The API wraps the list in an envelope: { "items": [...], "count": N, ... }
        // The "|| []" fallback prevents errors if the field is missing.
        const users = data.items || [];

        if (users.length === 0) {
            container.innerHTML = '<p style="padding:1rem;color:#666;">No users found.</p>';
            return;
        }

        let html = '<table><thead><tr><th>User</th><th>ID</th><th>Permissions</th><th>Passkeys</th><th>Last Login</th></tr></thead><tbody>';

        for (const u of users) {
            // Array.isArray() is necessary because the server may return permissions
        // as either an array (["ego.logon","ego.admin"]) or a single string.
        // join(', ') concatenates the array elements into one comma-separated string.
        // The || '' at the end converts null or undefined to an empty string.
        const perms    = Array.isArray(u.permissions) ? u.permissions.join(', ') : (u.permissions || '');
            const id       = u.id || '';
            const passkeys = u.passkeys != null ? String(u.passkeys) : '0';
            const rawToken = u.lastTokenAt || '';
            // Format the RFC3339 timestamp as a locale date/time string, or show a
            // dash when the value is absent or is Go's zero time (year 0001).
            const lastLogin = rawToken && !rawToken.startsWith('0001')
                ? new Date(rawToken).toLocaleString()
                : '—';

            // data-* attributes carry row values into the click handler without a
            // global variable. escapeHtml() is used for display and attribute safety.
            html += '<tr data-name="' + escapeHtml(u.name) + '" data-perms="' + escapeHtml(perms) + '" data-passkeys="' + escapeHtml(passkeys) + '" data-last-token="' + escapeHtml(rawToken) + '">'
                  + '<td>' + escapeHtml(u.name) + '</td>'
                  + '<td class="user-id">' + escapeHtml(id) + '</td>'
                  + '<td>' + escapeHtml(perms)  + '</td>'
                  + '<td class="passkey-count">' + escapeHtml(passkeys) + '</td>'
                  + '<td>' + lastLogin + '</td>'
                  + '</tr>';
        }

        html += '</tbody></table>';
        container.innerHTML = html;

        // Attach a click listener to every row so clicking opens the edit sheet.
        container.querySelectorAll('tbody tr').forEach(row => {
            row.addEventListener('click', () => {
                showEditUserSheet(row.dataset.name, row.dataset.perms, row.dataset.passkeys, row.dataset.lastToken);
            });
        });
    } catch (e) {
        if (e.message !== 'Unauthorized') console.error('Error loading users:', e);
    }
}

// Escape characters that have special meaning in HTML so that user-supplied
// strings from the server are rendered as plain text, not as markup.
//
// Without this, a username like "<script>alert(1)</script>" would execute
// JavaScript in the browser — a Cross-Site Scripting (XSS) attack.
// Each replace() call handles one dangerous character:
//   & → &amp;   (must be first, otherwise the later replacements double-encode)
//   < → &lt;    (prevents opening an HTML tag)
//   > → &gt;    (prevents closing an HTML tag)
//   " → &quot;  (prevents breaking out of an attribute value)
function escapeHtml(str) {
    return String(str)
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;');
}

// Load the DSNs tab — fetches the data source name list and renders it as
// a table with columns for connection details.
async function loadDsns() {
    const container = document.getElementById('dsns-content');
    try {
        const res  = await apiFetch('/dsns');
        const data = await res.json();
        const dsns = data.items || [];

        if (dsns.length === 0) {
            container.innerHTML = '<p style="padding:1rem;color:#666;">No DSNs found.</p>';
            return;
        }

        let html = '<table><thead><tr>'
                 + '<th>Name</th><th>Provider</th><th>Database</th>'
                 + '<th>Host</th><th>Port</th><th>User</th>'
                 + '<th>Secured</th><th>Restricted</th>'
                 + '</tr></thead><tbody>';

        for (const d of dsns) {
            // SQLite DSNs have no host or port; default to empty string so
            // the table cell exists but is blank rather than showing "0" or "null".
            const host = d.host || '';
            // d.port is a number; String() converts it to text for display.
            // The ternary (condition ? valueIfTrue : valueIfFalse) avoids "0" for missing ports.
            const port = d.port ? String(d.port) : '';

            // For the boolean flags, show "Yes"/"No" rather than true/false.
            // The ternary operator (condition ? 'Yes' : 'No') is a compact if/else.
            //
            // Each row is clickable: clicking opens the DSN detail sheet.
            const safeName = escapeHtml(d.name);
            html += '<tr class="dsn-row" onclick="showDsnDetail(\'' + safeName + '\')"'
                  + ' title="Click to view details for ' + safeName + '">'
                  + '<td>' + safeName                  + '</td>'
                  + '<td>' + escapeHtml(d.provider)    + '</td>'
                  + '<td>' + escapeHtml(d.database)    + '</td>'
                  + '<td>' + escapeHtml(host)           + '</td>'
                  + '<td>' + escapeHtml(port)           + '</td>'
                  + '<td>' + escapeHtml(d.user || '')   + '</td>'
                  + '<td>' + (d.secured    ? 'Yes' : 'No') + '</td>'
                  + '<td>' + (d.restricted ? 'Yes' : 'No') + '</td>'
                  + '</tr>';
        }

        html += '</tbody></table>';
        container.innerHTML = html;
    } catch (e) {
        if (e.message !== 'Unauthorized') console.error('Error loading DSNs:', e);
    }
}

// DSN name currently shown in the DSN detail sheet.
let _dsnDetailName = '';

// Open the DSN detail sheet for the given DSN name. Shows the DSN attributes
// as a two-column table, and fetches permissions if the DSN is restricted.
async function showDsnDetail(name) {
    _dsnDetailName = name;

    const overlay  = document.getElementById('dsn-detail-overlay');
    const content  = document.getElementById('dsn-detail-content');
    const permSec  = document.getElementById('dsn-permissions-section');
    const permCont = document.getElementById('dsn-permissions-content');

    document.getElementById('dsn-detail-title').textContent = name;
    document.getElementById('dsn-detail-error').textContent = '';
    content.innerHTML  = '<p style="color:#666;font-size:0.85rem;">Loading\u2026</p>';
    permSec.style.display = 'none';
    permCont.innerHTML = '';
    overlay.style.display = 'flex';

    try {
        // Fetch the full DSN list to find the details for this DSN.
        const res  = await apiFetch('/dsns');
        const data = await res.json();

        if (!res.ok) {
            document.getElementById('dsn-detail-error').textContent =
                data.msg || 'Failed to load DSN details (HTTP ' + res.status + ').';
            content.innerHTML = '';
            return;
        }

        const dsns   = data.items || [];
        const dsnObj = dsns.find(d => d.name === name);

        if (!dsnObj) {
            document.getElementById('dsn-detail-error').textContent = 'DSN not found.';
            content.innerHTML = '';
            return;
        }

        // Render DSN attributes as a two-column key/value table.
        const labels = {
            name:       'Name',
            provider:   'Provider',
            database:   'Database',
            host:       'Host',
            port:       'Port',
            user:       'User',
            secured:    'Secured',
            restricted: 'Restricted',
        };

        let html = '<table><thead><tr><th>Attribute</th><th>Value</th></tr></thead><tbody>';
        for (const [key, label] of Object.entries(labels)) {
            let val = dsnObj[key];
            if (val === undefined || val === null) val = '';
            if (typeof val === 'boolean') val = val ? 'Yes' : 'No';
            if (key === 'port' && !val) val = '';
            html += '<tr><td>' + label + '</td><td>' + escapeHtml(String(val)) + '</td></tr>';
        }
        html += '</tbody></table>';
        content.innerHTML = html;

        // If the DSN is restricted, fetch and display permissions.
        if (dsnObj.restricted) {
            permSec.style.display = '';
            permCont.innerHTML = '<p style="color:#666;font-size:0.85rem;">Loading permissions\u2026</p>';
            try {
                const pRes  = await apiFetch('/dsns/' + encodeURIComponent(name) + '/@permissions');
                const pData = await pRes.json();

                if (!pRes.ok) {
                    permCont.innerHTML = '<p style="color:#c00;">Failed to load permissions.</p>';
                } else {
                    const items = pData.items || {};
                    // Only show users that have at least one permission.
                    const users = Object.keys(items).sort().filter(u => (items[u] || []).length > 0);
                    if (users.length === 0) {
                        permCont.innerHTML = '<p style="color:#666;font-size:0.85rem;">No permissions defined.</p>';
                    } else {
                        let pHtml = '<table><thead><tr><th>User</th><th>Permissions</th></tr></thead><tbody>';
                        for (const user of users) {
                            const permArr  = items[user] || [];
                            const permsStr = permArr.join(', ');
                            const safeUser = escapeHtml(user);
                            // Encode current perms into a data attribute for the click handler.
                            const dataPerms = escapeHtml(permArr.join(','));
                            pHtml += '<tr class="dsn-row" title="Click to edit permissions for ' + safeUser + '"'
                                   + ' onclick="showDsnPermEdit(\'' + safeUser + '\', \'' + dataPerms + '\')">'
                                   + '<td>' + safeUser + '</td>'
                                   + '<td>' + escapeHtml(permsStr) + '</td>'
                                   + '</tr>';
                        }
                        pHtml += '</tbody></table>';
                        permCont.innerHTML = pHtml;
                    }
                }
            } catch (pe) {
                if (pe.message !== 'Unauthorized') {
                    permCont.innerHTML = '<p style="color:#c00;">Network error: ' + escapeHtml(pe.message) + '</p>';
                }
            }
        }
    } catch (e) {
        if (e.message !== 'Unauthorized') {
            document.getElementById('dsn-detail-error').textContent =
                'Network error: ' + e.message;
            content.innerHTML = '';
        }
    }
}

// Close the DSN detail sheet.
function hideDsnDetail() {
    document.getElementById('dsn-detail-overlay').style.display = 'none';
}

// Delete the currently displayed DSN, then close the sheet and refresh the list.
async function submitDeleteDsn() {
    document.getElementById('dsn-detail-delete-btn').disabled = true;

    try {
        const token = getToken();
        const res = await fetch('/dsns/' + encodeURIComponent(_dsnDetailName), {
            method:  'DELETE',
            headers: { 'Authorization': token ? 'Bearer ' + token : '' },
        });

        if (!res.ok) {
            if (res.status === 401 || res.status === 403) {
                clearToken();
                hideDsnDetail();
                showLogin('Session expired. Please sign in again.');
                return;
            }
            const data = await res.json().catch(() => ({}));
            document.getElementById('dsn-detail-error').textContent =
                data.msg || 'Failed to delete DSN (HTTP ' + res.status + ').';
            return;
        }

        hideDsnDetail();
        loadDsns();
    } catch (e) {
        document.getElementById('dsn-detail-error').textContent = 'Network error. Please try again.';
    } finally {
        document.getElementById('dsn-detail-delete-btn').disabled = false;
    }
}

// Switch to the Tables tab with the current DSN pre-selected. Called from the
// "Show tables…" button in the DSN detail sheet.
function openDsnTablesFromSheet() {
    hideDsnDetail();
    _pendingTablesDsn = _dsnDetailName;
    openTab('tables');
}

// ==========================================================================
// DSN permission edit sheet
// ==========================================================================

// Original permissions for the user currently being edited (array of strings).
// Stored so submitDsnPermEdit() can diff against the new value.
let _dsnPermEditOriginal = [];

// Open the permission edit sheet for a specific user within the current DSN.
// currentPermsStr is a comma-separated string of the existing permissions.
function showDsnPermEdit(user, currentPermsStr) {
    _dsnPermEditOriginal = currentPermsStr ? currentPermsStr.split(',').map(p => p.trim()).filter(Boolean) : [];

    document.getElementById('dsn-perm-edit-error').textContent = '';
    document.getElementById('dsn-perm-edit-user').value  = user;
    document.getElementById('dsn-perm-edit-perms').value = _dsnPermEditOriginal.join(', ');
    document.getElementById('dsn-perm-edit-save-btn').disabled   = false;
    document.getElementById('dsn-perm-edit-delete-btn').disabled = false;
    document.getElementById('dsn-perm-edit-overlay').style.display = 'flex';
    captureBaseline('dsn-perm-edit-overlay');
    document.getElementById('dsn-perm-edit-perms').focus();
}

// Close the permission edit sheet without saving.
function hideDsnPermEdit() {
    document.getElementById('dsn-perm-edit-overlay').style.display = 'none';
}

// Build the actions array by diffing old permissions against new ones.
// Removed permissions are prefixed with "-", added ones with "+".
function buildPermActions(oldPerms, newPerms) {
    const actions = [];
    for (const p of oldPerms) {
        if (!newPerms.includes(p)) actions.push('-' + p);
    }
    for (const p of newPerms) {
        if (!oldPerms.includes(p)) actions.push('+' + p);
    }
    return actions;
}

// POST the permission changes to /dsns/@permissions.
async function submitDsnPermEdit() {
    const user     = document.getElementById('dsn-perm-edit-user').value;
    const permsRaw = document.getElementById('dsn-perm-edit-perms').value;
    const newPerms = permsRaw.split(',').map(p => p.trim()).filter(Boolean);

    const actions = buildPermActions(_dsnPermEditOriginal, newPerms);
    if (actions.length === 0) {
        hideDsnPermEdit();
        return;
    }

    document.getElementById('dsn-perm-edit-save-btn').disabled = true;

    try {
        const token = getToken();
        const res = await fetch('/dsns/@permissions', {
            method:  'POST',
            headers: {
                'Content-Type':  'application/json',
                'Authorization': token ? 'Bearer ' + token : '',
            },
            body: JSON.stringify({ dsn: _dsnDetailName, user, actions }),
        });

        if (res.status === 401 || res.status === 403) {
            clearToken();
            hideDsnPermEdit();
            hideDsnDetail();
            showLogin('Session expired. Please sign in again.');
            return;
        }

        if (!res.ok) {
            const data = await res.json().catch(() => ({}));
            document.getElementById('dsn-perm-edit-error').textContent =
                data.msg || 'Failed to update permissions (HTTP ' + res.status + ').';
            return;
        }

        // Refresh the DSN detail sheet to show updated permissions.
        hideDsnPermEdit();
        showDsnDetail(_dsnDetailName);
    } catch (e) {
        document.getElementById('dsn-perm-edit-error').textContent = 'Network error. Please try again.';
    } finally {
        document.getElementById('dsn-perm-edit-save-btn').disabled = false;
    }
}

// Remove all permissions for the current user by posting "-" for each existing one.
async function submitDeleteDsnPerm() {
    const user = document.getElementById('dsn-perm-edit-user').value;

    if (!confirm('Remove all permissions for "' + user + '" on DSN "' + _dsnDetailName + '"?')) return;

    const actions = _dsnPermEditOriginal.map(p => '-' + p);
    if (actions.length === 0) {
        hideDsnPermEdit();
        return;
    }

    document.getElementById('dsn-perm-edit-delete-btn').disabled = true;

    try {
        const token = getToken();
        const res = await fetch('/dsns/@permissions', {
            method:  'POST',
            headers: {
                'Content-Type':  'application/json',
                'Authorization': token ? 'Bearer ' + token : '',
            },
            body: JSON.stringify({ dsn: _dsnDetailName, user, actions }),
        });

        if (res.status === 401 || res.status === 403) {
            clearToken();
            hideDsnPermEdit();
            hideDsnDetail();
            showLogin('Session expired. Please sign in again.');
            return;
        }

        if (!res.ok) {
            const data = await res.json().catch(() => ({}));
            document.getElementById('dsn-perm-edit-error').textContent =
                data.msg || 'Failed to delete permissions (HTTP ' + res.status + ').';
            return;
        }

        hideDsnPermEdit();
        showDsnDetail(_dsnDetailName);
    } catch (e) {
        document.getElementById('dsn-perm-edit-error').textContent = 'Network error. Please try again.';
    } finally {
        document.getElementById('dsn-perm-edit-delete-btn').disabled = false;
    }
}

// ==========================================================================
// Add DSN permission sheet
// ==========================================================================

// Open the add-permission sheet with blank fields.
function showDsnPermAdd() {
    document.getElementById('dsn-perm-add-error').textContent = '';
    document.getElementById('dsn-perm-add-user').value  = '';
    document.getElementById('dsn-perm-add-perms').value = '';
    document.getElementById('dsn-perm-add-save-btn').disabled = false;
    document.getElementById('dsn-perm-add-overlay').style.display = 'flex';
    captureBaseline('dsn-perm-add-overlay');
    document.getElementById('dsn-perm-add-user').focus();
}

// Close the add-permission sheet without saving.
function hideDsnPermAdd() {
    document.getElementById('dsn-perm-add-overlay').style.display = 'none';
}

// POST new permissions to /dsns/@permissions, then refresh the DSN detail sheet.
async function submitDsnPermAdd() {
    const user     = document.getElementById('dsn-perm-add-user').value.trim();
    const permsRaw = document.getElementById('dsn-perm-add-perms').value;
    const perms    = permsRaw.split(',').map(p => p.trim()).filter(Boolean);

    if (!user) {
        document.getElementById('dsn-perm-add-error').textContent = 'User is required.';
        return;
    }
    if (perms.length === 0) {
        document.getElementById('dsn-perm-add-error').textContent = 'At least one permission is required.';
        return;
    }

    document.getElementById('dsn-perm-add-save-btn').disabled = true;

    try {
        const token = getToken();
        const res = await fetch('/dsns/@permissions', {
            method:  'POST',
            headers: {
                'Content-Type':  'application/json',
                'Authorization': token ? 'Bearer ' + token : '',
            },
            body: JSON.stringify({ dsn: _dsnDetailName, user, actions: perms.map(p => '+' + p) }),
        });

        if (res.status === 401 || res.status === 403) {
            clearToken();
            hideDsnPermAdd();
            hideDsnDetail();
            showLogin('Session expired. Please sign in again.');
            return;
        }

        if (!res.ok) {
            const data = await res.json().catch(() => ({}));
            document.getElementById('dsn-perm-add-error').textContent =
                data.msg || 'Failed to add permissions (HTTP ' + res.status + ').';
            return;
        }

        hideDsnPermAdd();
        showDsnDetail(_dsnDetailName);
    } catch (e) {
        document.getElementById('dsn-perm-add-error').textContent = 'Network error. Please try again.';
    } finally {
        document.getElementById('dsn-perm-add-save-btn').disabled = false;
    }
}

// Send a DELETE to /admin/caches/ to flush all server-side caches, then
// reload the Memory tab to reflect the now-empty cache state.
async function flushCaches() {
    const btn = document.querySelector('[onclick="flushCaches()"]');
    btn.disabled = true;
    try {
        const token = getToken();
        const res = await fetch('/admin/caches/', {
            method:  'DELETE',
            headers: token ? { 'Authorization': 'Bearer ' + token } : {},
        });

        if (res.status === 401 || res.status === 403) {
            clearToken();
            showLogin('Session expired. Please sign in again.');
            return;
        }

        if (!res.ok) {
            const data = await res.json().catch(() => ({}));
            alert(data.msg || 'Failed to flush caches (HTTP ' + res.status + ').');
            return;
        }

        // Reload the Memory tab to reflect the now-empty caches.
        loadMemory();
    } catch (e) {
        alert('Network error: ' + e.message);
    } finally {
        btn.disabled = false;
    }
}

// ==========================================================================
// Configuration sheet
// ==========================================================================

// Fetch GET /admin/config and display all server configuration items in a
// read-only sheet. Keys are sorted alphabetically for easy scanning.
async function showConfigSheet() {
    const content = document.getElementById('config-content');
    const errorEl = document.getElementById('config-error');

    errorEl.textContent = '';
    content.innerHTML = '<p style="color:#666;font-size:0.85rem;">Loading\u2026</p>';
    document.getElementById('config-overlay').style.display = 'flex';

    try {
        const res  = await apiFetch('/admin/config');
        const data = await res.json();

        if (!res.ok) {
            errorEl.textContent = data.message || 'Failed to load configuration.';
            content.innerHTML = '';
            return;
        }

        const items = data.items || {};
        const keys  = Object.keys(items).sort();

        if (keys.length === 0) {
            content.innerHTML = '<p style="color:#666;font-size:0.85rem;">No configuration items found.</p>';
            return;
        }

        // Build a two-column table: Setting | Value.
        // escapeHtml() prevents any HTML characters in keys or values (e.g. < > &)
        // from being interpreted as markup — important for path values on Windows.
        const rows = keys.map(k =>
            `<tr><td>${escapeHtml(k)}</td><td>${escapeHtml(items[k])}</td></tr>`
        ).join('');

        content.innerHTML =
            '<table>' +
            '<thead><tr><th>Setting</th><th>Value</th></tr></thead>' +
            '<tbody>' + rows + '</tbody>' +
            '</table>';

    } catch (e) {
        errorEl.textContent = 'Network error. Please try again.';
        content.innerHTML = '';
    }
}

// Hide the configuration sheet.
function hideConfigSheet() {
    document.getElementById('config-overlay').style.display = 'none';
}

// Load the Tables tab — populates the DSN picker then fetches the table list
// for the currently selected DSN.
async function loadTables() {
    const picker    = document.getElementById('tables-dsn-picker');
    const container = document.getElementById('tables-content');

    // ---- Populate / refresh the DSN picker ----------------------------------
    // Remember which DSN was selected so we can restore it after a refresh.
    const previousDsn = picker.value;

    try {
        const res  = await apiFetch('/dsns');
        const data = await res.json();
        const dsns = (data.items || []).map(d => d.name).sort();

        // Rebuild the <select> options only if the list changed, to avoid a
        // flash of blank content when the user clicks Refresh.
        // picker.options is an HTMLOptionsCollection (an array-like object, but not
        // a real Array). Array.from() converts it so we can use .map() on it.
        const currentOptions = Array.from(picker.options).map(o => o.value);
        const listChanged = dsns.join(',') !== currentOptions.join(',');

        if (listChanged) {
            picker.innerHTML = '';
            if (dsns.length === 0) {
                picker.innerHTML = '<option value="">— no DSNs —</option>';
                container.innerHTML = '<p style="padding:1rem;color:#666;">No DSNs configured.</p>';
                return;
            }
            for (const name of dsns) {
                const opt = document.createElement('option');
                opt.value       = name;
                opt.textContent = name;
                picker.appendChild(opt);
            }
            // Restore previous selection if it still exists.
            if (previousDsn && dsns.includes(previousDsn)) {
                picker.value = previousDsn;
            }
        }
    } catch (e) {
        if (e.message !== 'Unauthorized') console.error('Error loading DSNs for Tables tab:', e);
        return;
    }

    // ---- Fetch the table list for the selected DSN --------------------------
    // If we arrived here via a DSN row click, override the picker with the
    // requested DSN before reading its value. Clear the variable afterwards
    // so a manual Refresh uses the picker's own selection.
    if (_pendingTablesDsn) {
        picker.value      = _pendingTablesDsn;
        _pendingTablesDsn = null;
    }

    const dsn = picker.value;
    if (!dsn) return;

    container.innerHTML = '<p style="padding:1rem;color:#666;">Loading\u2026</p>';

    try {
        const res  = await apiFetch('/dsns/' + encodeURIComponent(dsn) + '/tables');
        const data = await res.json();

        if (!res.ok) {
            container.innerHTML = '<p style="padding:1rem;color:#c0392b;">'
                + escapeHtml(data.msg || 'Failed to load tables (HTTP ' + res.status + ').')
                + '</p>';
            return;
        }

        const tables = data.tables || [];

        if (tables.length === 0) {
            container.innerHTML = '<p style="padding:1rem;color:#666;">No tables found in <strong>'
                + escapeHtml(dsn) + '</strong>.</p>';
            return;
        }

        let html = '<table><thead><tr>'
                 + '<th>Name</th><th>Schema</th><th>Columns</th><th>Rows</th>'
                 + '</tr></thead><tbody>';

        for (const t of tables) {
            html += '<tr data-name="' + escapeHtml(t.name) + '" data-schema="' + escapeHtml(t.schema || '') + '">'
                  + '<td>' + escapeHtml(t.name)         + '</td>'
                  + '<td>' + escapeHtml(t.schema || '')  + '</td>'
                  + '<td>' + t.columns                   + '</td>'
                  + '<td>' + t.rows                      + '</td>'
                  + '</tr>';
        }

        html += '</tbody></table>';
        container.innerHTML = html;

        // Make rows clickable — open the detail sheet for the selected table.
        container.querySelectorAll('tbody tr').forEach(row => {
            row.addEventListener('click', () => {
                showTableDetail(dsn, row.dataset.name);
            });
        });
    } catch (e) {
        if (e.message !== 'Unauthorized') {
            container.innerHTML = '<p style="padding:1rem;color:#c0392b;">Network error: '
                + escapeHtml(e.message) + '</p>';
        }
    }
}

// DSN and table name currently shown in the table-detail sheet.
let _tableDetailDsn   = '';
let _tableDetailTable = '';

// Open the table-detail sheet and fetch column metadata for the given table.
async function showTableDetail(dsn, tableName) {
    _tableDetailDsn   = dsn;
    _tableDetailTable = tableName;

    const overlay = document.getElementById('table-detail-overlay');
    const content = document.getElementById('table-detail-content');

    document.getElementById('table-detail-title').textContent  = tableName;
    document.getElementById('table-detail-error').textContent  = '';
    content.innerHTML = '<p style="color:#666;font-size:0.85rem;">Loading\u2026</p>';
    overlay.style.display = 'flex';

    try {
        const res  = await apiFetch('/dsns/' + encodeURIComponent(dsn) + '/tables/' + encodeURIComponent(tableName));
        const data = await res.json();

        if (!res.ok) {
            document.getElementById('table-detail-error').textContent =
                data.msg || 'Failed to load table details (HTTP ' + res.status + ').';
            content.innerHTML = '';
            return;
        }

        const columns = data.columns || [];

        if (columns.length === 0) {
            content.innerHTML = '<p style="color:#666;font-size:0.85rem;">No columns found.</p>';
            return;
        }

        let html = '<table><thead><tr>'
                 + '<th>Column</th><th>Type</th><th>Size</th><th>Nullable</th><th>Unique</th>'
                 + '</tr></thead><tbody>';

        for (const col of columns) {
            const size     = col.size > 0 ? col.size : '';
            const nullable = col.nullable && col.nullable.specified ? (col.nullable.value ? 'Yes' : 'No') : '';
            const unique   = col.unique   && col.unique.specified   ? (col.unique.value   ? 'Yes' : 'No') : '';

            html += '<tr>'
                  + '<td>' + escapeHtml(col.name) + '</td>'
                  + '<td>' + escapeHtml(col.type) + '</td>'
                  + '<td>' + size                 + '</td>'
                  + '<td>' + nullable             + '</td>'
                  + '<td>' + unique               + '</td>'
                  + '</tr>';
        }

        html += '</tbody></table>';
        content.innerHTML = html;
    } catch (e) {
        if (e.message !== 'Unauthorized') {
            document.getElementById('table-detail-error').textContent =
                'Network error: ' + e.message;
            content.innerHTML = '';
        }
    }
}

// Close the table-detail sheet.
function hideTableDetail() {
    document.getElementById('table-detail-overlay').style.display = 'none';
}

// Switch to the Data tab and pre-select the DSN/table currently shown in the
// table-detail sheet.
function viewDataFromTable() {
    _pendingDataDsn   = _tableDetailDsn;
    _pendingDataTable = _tableDetailTable;
    hideTableDetail();
    openTab('data');
}

// ==========================================================================
// Data tab — browse rows from a selected DSN and table
// ==========================================================================

// Pending DSN/table set by viewDataFromTable(); consumed once by loadData()
// and loadDataTables() so the pickers are forced to the right selection even
// on a first visit to the Data tab when they have no options yet.
let _pendingDataDsn   = null;
let _pendingDataTable = null;

// Module-level state for the Data tab.
let _dataRows          = [];  // last fetched row objects
let _dataRowCount      = 0;   // server-reported row count
let _dataColumnMeta    = [];  // [{name, type, …}] from the table-detail API
let _dataColumnVisible = {};  // {columnName: boolean} — driven by the Columns sheet
let _dataCurrentDsn    = '';  // DSN used for the last metadata fetch
let _dataCurrentTable  = '';  // table used for the last metadata fetch

// Load the Data tab — refreshes the DSN picker, then cascades.
async function loadData() {
    const dsnPicker   = document.getElementById('data-dsn-picker');
    const previousDsn = _pendingDataDsn || dsnPicker.value;
    _pendingDataDsn   = null;

    try {
        const res  = await apiFetch('/dsns');
        const data = await res.json();
        const dsns = (data.items || []).map(d => d.name).sort();

        // Array.from() converts the HTMLOptionsCollection to a plain Array
        // so we can call .map() on it.
        const currentOptions = Array.from(dsnPicker.options).map(o => o.value);
        const listChanged    = dsns.join(',') !== currentOptions.join(',');

        if (listChanged) {
            dsnPicker.innerHTML = '';
            if (dsns.length === 0) {
                dsnPicker.innerHTML = '<option value="">— no DSNs —</option>';
                document.getElementById('data-table-picker').innerHTML = '<option value="">— no tables —</option>';
                document.getElementById('data-content').innerHTML =
                    '<p style="padding:1rem;color:#666;">No DSNs configured.</p>';
                return;
            }
            for (const name of dsns) {
                const opt = document.createElement('option');
                opt.value       = name;
                opt.textContent = name;
                dsnPicker.appendChild(opt);
            }
            if (previousDsn && dsns.includes(previousDsn)) {
                dsnPicker.value = previousDsn;
            }
        }
    } catch (e) {
        if (e.message !== 'Unauthorized') console.error('Error loading DSNs for Data tab:', e);
        return;
    }

    await loadDataTables();
}

// Populate the table picker for the currently selected DSN, then cascade.
async function loadDataTables() {
    const dsnPicker   = document.getElementById('data-dsn-picker');
    const tablePicker = document.getElementById('data-table-picker');
    const container   = document.getElementById('data-content');
    const dsn         = dsnPicker.value;

    if (!dsn) return;

    const previousTable = _pendingDataTable || tablePicker.value;
    _pendingDataTable   = null;

    try {
        const res    = await apiFetch('/dsns/' + encodeURIComponent(dsn) + '/tables');
        const data   = await res.json();
        const tables = (data.tables || []).map(t => t.name).sort();

        tablePicker.innerHTML = '';
        if (tables.length === 0) {
            tablePicker.innerHTML = '<option value="">— no tables —</option>';
            container.innerHTML = '<p style="padding:1rem;color:#666;">No tables found in <strong>'
                + escapeHtml(dsn) + '</strong>.</p>';
            return;
        }
        for (const name of tables) {
            const opt = document.createElement('option');
            opt.value       = name;
            opt.textContent = name;
            tablePicker.appendChild(opt);
        }
        if (previousTable && tables.includes(previousTable)) {
            tablePicker.value = previousTable;
        }
    } catch (e) {
        if (e.message !== 'Unauthorized') console.error('Error loading tables for Data tab:', e);
        return;
    }

    await loadDataMeta();
}

// Return the column name to use as the unique row key for PATCH/DELETE.
// Prefers _row_id_ if it is marked unique, then falls back to the first
// column in _dataColumnMeta whose unique.specified and unique.value are both
// true. Returns null when no unique column exists (row is read-only).
function findUniqueKeyCol() {
    // Two-pass: first check _row_id_ explicitly, then scan for any other unique column.
    let firstUnique = null;
    for (const col of _dataColumnMeta) {
        if (col.unique && col.unique.specified && col.unique.value) {
            if (col.name === '_row_id_') return '_row_id_';
            if (firstUnique === null) firstUnique = col.name;
        }
    }
    return firstUnique;
}

// Fetch column metadata for the selected DSN/table, reset visibility when the
// selection changes, then load rows.
async function loadDataMeta() {
    const dsn   = document.getElementById('data-dsn-picker').value;
    const table = document.getElementById('data-table-picker').value;

    if (!dsn || !table) return;

    // Reset column visibility whenever the DSN or table changes.
    if (dsn !== _dataCurrentDsn || table !== _dataCurrentTable) {
        _dataCurrentDsn   = dsn;
        _dataCurrentTable = table;
        _dataColumnVisible = {};
    }

    try {
        const res  = await apiFetch('/dsns/' + encodeURIComponent(dsn) + '/tables/' + encodeURIComponent(table) + '?rowids=true');
        const data = await res.json();
        _dataColumnMeta = res.ok ? (data.columns || []) : [];
    } catch (e) {
        if (e.message !== 'Unauthorized') console.error('Error loading column metadata:', e);
        _dataColumnMeta = [];
    }

    await loadDataRows();
}

// Returns true when the SQL type name represents an integer type.
function isDataIntType(type) {
    return /^(int|integer|int32|int64|bigint|smallint|tinyint)$/i.test(type || '');
}

// Returns true when the SQL type name represents a floating-point type.
function isDataFloatType(type) {
    return /^(float|float32|float64|double|real|numeric|decimal)$/i.test(type || '');
}

// Fetch rows from the server and hand off to the renderer.
async function loadDataRows() {
    const dsn       = document.getElementById('data-dsn-picker').value;
    const table     = document.getElementById('data-table-picker').value;
    const container = document.getElementById('data-content');

    if (!dsn || !table) return;

    container.innerHTML = '<p style="padding:1rem;color:#666;">Loading\u2026</p>';

    try {
        const res  = await apiFetch(
            '/dsns/' + encodeURIComponent(dsn) + '/tables/' + encodeURIComponent(table) + '/rows'
        );
        const data = await res.json();

        if (!res.ok) {
            container.innerHTML = '<p style="padding:1rem;color:#c0392b;">'
                + escapeHtml(data.msg || 'Failed to load rows (HTTP ' + res.status + ').')
                + '</p>';
            return;
        }

        _dataRows     = data.rows  || [];
        _dataRowCount = data.count !== undefined ? data.count : _dataRows.length;
        renderDataRows();
    } catch (e) {
        if (e.message !== 'Unauthorized') {
            container.innerHTML = '<p style="padding:1rem;color:#c0392b;">Network error: '
                + escapeHtml(e.message) + '</p>';
        }
    }
}

// Pure render — builds the table from _dataRows, _dataColumnMeta, and
// _dataColumnVisible without touching the network.
function renderDataRows() {
    const container = document.getElementById('data-content');
    const table     = document.getElementById('data-table-picker').value;

    if (_dataRows.length === 0) {
        container.innerHTML = '<p style="padding:1rem;color:#666;">No rows found in <strong>'
            + escapeHtml(table) + '</strong>.</p>';
        return;
    }

    // Build a lookup: column name → metadata object.
    // This gives O(1) access to type info inside the loops below.
    const metaByName = {};
    for (const col of _dataColumnMeta) metaByName[col.name] = col;

    // Collect column names across every row, skipping _row_id_ (internal).
    // A Set is used here because it automatically ignores duplicate keys —
    // different rows may have the same column names, and a Set ensures each
    // name appears only once. Object.keys(row) returns an array of the
    // field names in that row object.
    const colSet = new Set();
    for (const row of _dataRows) {
        for (const key of Object.keys(row)) {
            if (key !== '_row_id_') colSet.add(key);
        }
    }
    // Array.from() converts the Set back to a plain Array so we can call .filter().
    // The visibility check uses !== false (not === true) so that columns without
    // an entry in _dataColumnVisible default to visible rather than hidden.
    const columns = Array.from(colSet).filter(c => _dataColumnVisible[c] !== false);

    // Return the CSS class for right-aligning numeric columns.
    function alignClass(colName) {
        const meta = metaByName[colName];
        const type = meta ? meta.type : '';
        return (isDataIntType(type) || isDataFloatType(type)) ? ' class="data-cell-num"' : '';
    }

    // Format a single cell value according to its column type.
    function fmtCell(val, colName) {
        // val == null (with ==, not ===) catches both null and undefined,
        // which is intentional — both mean "no value" in this context.
        if (val == null) return '<span class="data-null">null</span>';
        const meta = metaByName[colName];
        const type = meta ? meta.type : '';
        if (isDataFloatType(type)) {
            const n = Number(val);
            // Number.isFinite() returns true only for real, finite numbers.
            // It rejects NaN (Not a Number) and Infinity, which Number() can
            // produce from strings like "abc" or "Infinity".
            if (Number.isFinite(n)) {
                const s = String(n);
                // Always show a decimal point so floats look distinct from integers
                // (e.g. "42" becomes "42.0"). s.includes('.') checks if JS already
                // produced one (it does for values like 3.14).
                return escapeHtml(s.includes('.') ? s : s + '.0');
            }
        }
        return escapeHtml(String(val));
    }

    // Determine the unique key column. When it is _row_id_ we show a dedicated
    // "Row ID" header column (since _row_id_ is excluded from the data columns).
    // For any other unique column the key value is already visible in the data
    // columns, so no extra header column is needed.
    const keyCol       = findUniqueKeyCol();
    const showKeyCol   = keyCol === '_row_id_';

    let html = '<div class="data-table-scroll"><table><thead><tr>';
    if (showKeyCol) html += '<th>Row ID</th>';
    for (const col of columns) {
        html += '<th' + alignClass(col) + '>' + escapeHtml(col) + '</th>';
    }
    html += '</tr></thead><tbody>';

    for (let i = 0; i < _dataRows.length; i++) {
        const row = _dataRows[i];
        html += '<tr data-row-idx="' + i + '">';
        if (showKeyCol) {
            // != null (with !=, not !==) catches both null and undefined.
            const rowId = row['_row_id_'] != null ? String(row['_row_id_']) : '';
            html += '<td class="data-row-id">' + escapeHtml(rowId) + '</td>';
        }
        for (const col of columns) {
            html += '<td' + alignClass(col) + '>' + fmtCell(row[col], col) + '</td>';
        }
        html += '</tr>';
    }

    html += '</tbody></table></div>'
          + '<p class="data-row-count">'
          + _dataRowCount + ' row' + (_dataRowCount === 1 ? '' : 's')
          + '</p>';
    container.innerHTML = html;

    // Wire click handlers so each row opens the edit sheet.
    // parseInt(..., 10) converts the data-row-idx string attribute back to a
    // number (radix 10 = decimal) so showDataEdit receives an integer index.
    container.querySelectorAll('.data-table-scroll tbody tr').forEach(tr => {
        tr.addEventListener('click', () => showDataEdit(parseInt(tr.dataset.rowIdx, 10)));
    });
}

// Open the Columns sheet — builds a toggle row for every column in _dataRows.
function showDataColumns() {
    // Collect unique column names using a Set (duplicates are ignored automatically).
    const colSet = new Set();
    for (const row of _dataRows) {
        for (const key of Object.keys(row)) {
            if (key !== '_row_id_') colSet.add(key);
        }
    }
    // Convert the Set to a plain Array so we can iterate with for...of below.
    const columns = Array.from(colSet);
    const list    = document.getElementById('data-col-list');
    list.innerHTML = '';

    if (columns.length === 0) {
        list.innerHTML = '<p style="color:#666;font-size:0.85rem;">No columns available.</p>';
    } else {
        for (const colName of columns) {
            const visible = _dataColumnVisible[colName] !== false;

            const rowEl = document.createElement('div');
            rowEl.className = 'data-col-row';

            const label = document.createElement('label');
            label.className = 'toggle-switch';

            const input = document.createElement('input');
            input.type        = 'checkbox';
            input.checked     = visible;
            input.dataset.col = colName;
            input.addEventListener('change', () => {
                _dataColumnVisible[colName] = input.checked;
                renderDataRows();
            });

            const slider = document.createElement('span');
            slider.className = 'toggle-slider';

            label.appendChild(input);
            label.appendChild(slider);

            const nameEl = document.createElement('span');
            nameEl.className   = 'data-col-name';
            nameEl.textContent = colName;

            rowEl.appendChild(label);
            rowEl.appendChild(nameEl);
            list.appendChild(rowEl);
        }
    }

    document.getElementById('data-col-overlay').style.display = 'flex';
}

// Close the Columns sheet.
function hideDataColumns() {
    document.getElementById('data-col-overlay').style.display = 'none';
}

// Turn on every column toggle and re-render the table.
function selectAllDataColumns() {
    document.querySelectorAll('#data-col-list input[type="checkbox"]').forEach(cb => {
        cb.checked = true;
        _dataColumnVisible[cb.dataset.col] = true;
    });
    renderDataRows();
}

// ==========================================================================
// Data tab — row edit sheet
// ==========================================================================

// Index into _dataRows of the row currently being edited.
let _dataEditRowIdx = -1;

// Open the edit sheet for the row at the given index in _dataRows.
function showDataEdit(rowIdx) {
    const row = _dataRows[rowIdx];
    if (!row) return;

    _dataEditRowIdx = rowIdx;

    // Find which column uniquely identifies this row. == null (loose equality)
    // catches both null and undefined — either means the row is read-only.
    const keyCol  = findUniqueKeyCol();
    const keyVal  = keyCol != null ? row[keyCol] : null;
    const noRowId = keyVal == null;
    document.getElementById('data-edit-title').textContent    = noRowId ? 'Row Contents' : 'Edit Row';
    document.getElementById('data-edit-readonly').textContent = noRowId ? 'This row cannot be modified.' : '';
    document.getElementById('data-edit-error').textContent    = '';
    document.getElementById('data-edit-save-btn').disabled    = true;
    document.getElementById('data-edit-delete-btn').disabled  = noRowId;

    const fieldsDiv = document.getElementById('data-edit-fields');
    fieldsDiv.innerHTML = '';

    // Collect ALL column names across all rows (union, excluding _row_id_),
    // so every field is shown in the edit sheet regardless of current visibility.
    // A Set ensures each column name appears only once even if it exists in
    // multiple rows.
    const colSet = new Set();
    for (const r of _dataRows) {
        for (const key of Object.keys(r)) {
            if (key !== '_row_id_') colSet.add(key);
        }
    }

    const metaByName = {};
    for (const col of _dataColumnMeta) metaByName[col.name] = col;

    if (noRowId) {
        // Read-only view: render all columns as a two-column table.
        const table = document.createElement('table');
        table.className = 'data-edit-ro-table';
        for (const colName of colSet) {
            const val = row[colName];
            const tr  = document.createElement('tr');

            const th = document.createElement('th');
            th.textContent = colName;

            const td = document.createElement('td');
            td.className   = val == null ? 'data-edit-ro-null' : '';
            td.textContent = val == null ? 'null' : String(val);

            tr.appendChild(th);
            tr.appendChild(td);
            table.appendChild(tr);
        }
        fieldsDiv.appendChild(table);
    } else {
        for (const colName of colSet) {
            const originalVal = row[colName];
            const startNull   = originalVal == null;

            // The unique key column is shown read-only so the filter value
            // for PATCH/DELETE stays consistent with what is on the server.
            const isKeyField = (colName === keyCol);

            const fieldDiv = document.createElement('div');
            fieldDiv.className = 'data-edit-field';

            const labelEl = document.createElement('label');
            labelEl.className   = 'data-edit-label';
            labelEl.textContent = colName + (isKeyField ? ' \u{1F511}' : '');

            const inputRow = document.createElement('div');
            inputRow.className = 'data-edit-input-row';

            const input = document.createElement('input');
            input.type           = 'text';
            input.className      = 'data-edit-input' + (startNull ? ' data-edit-null' : '');
            input.dataset.col    = colName;
            input.dataset.isNull = startNull ? 'true' : 'false';
            input.value          = startNull ? '' : String(originalVal);
            input.placeholder    = startNull ? 'null' : '';
            input.spellcheck     = false;
            input.autocomplete   = 'off';

            if (isKeyField) {
                // Prevent the user from changing the key field; it is only
                // shown for context. readOnly keeps the value copyable.
                input.readOnly = true;
                input.classList.add('data-edit-readonly');
            } else {
                input.addEventListener('input', () => {
                    if (input.dataset.isNull === 'true') {
                        input.dataset.isNull = 'false';
                        input.classList.remove('data-edit-null');
                        input.placeholder = '';
                    }
                    checkDataEditChanged();
                });

                const nullBtn = document.createElement('button');
                nullBtn.type        = 'button';
                nullBtn.className   = 'data-edit-null-btn';
                nullBtn.textContent = 'Null';
                nullBtn.addEventListener('click', () => {
                    input.dataset.isNull = 'true';
                    input.value       = '';
                    input.placeholder = 'null';
                    input.classList.add('data-edit-null');
                    checkDataEditChanged();
                });
                inputRow.appendChild(nullBtn);
            }

            inputRow.insertBefore(input, inputRow.firstChild);
            fieldDiv.appendChild(labelEl);
            fieldDiv.appendChild(inputRow);
            fieldsDiv.appendChild(fieldDiv);
        }
    }

    document.getElementById('data-edit-overlay').style.display = 'flex';
    // No captureBaseline here — isSheetModified delegates to data-edit-save-btn.
}

// Enable the Save button only when at least one field differs from the original.
function checkDataEditChanged() {
    const row = _dataRows[_dataEditRowIdx];
    if (!row) return;

    let changed = false;
    document.querySelectorAll('#data-edit-fields .data-edit-input').forEach(input => {
        if (changed) return;
        const colName     = input.dataset.col;
        const originalVal = row[colName];
        const isNull      = input.dataset.isNull === 'true';

        // Loose equality (== / !=) is intentional here: it treats null and
        // undefined as equivalent, which is what we want since missing fields
        // and explicit nulls both mean "no value".
        if (isNull  &&  originalVal != null)  { changed = true; return; }
        if (!isNull && originalVal == null)   { changed = true; return; }
        if (!isNull && originalVal != null
                    && input.value !== String(originalVal)) { changed = true; }
    });

    document.getElementById('data-edit-save-btn').disabled = !changed;
}

// Send a PATCH request with only the changed fields, then reload rows.
async function submitDataEdit() {
    const row    = _dataRows[_dataEditRowIdx];
    const dsn    = document.getElementById('data-dsn-picker').value;
    const table  = document.getElementById('data-table-picker').value;
    const keyCol = findUniqueKeyCol();
    const keyVal = (row && keyCol) ? row[keyCol] : null;

    if (!row || !dsn || !table) return;

    if (keyVal == null) {
        document.getElementById('data-edit-error').textContent =
            'This row has no unique key and cannot be updated.';
        return;
    }

    // Build metadata lookup for type coercion.
    const metaByName = {};
    for (const col of _dataColumnMeta) metaByName[col.name] = col;

    // Collect only changed fields.
    const payload = {};
    document.querySelectorAll('#data-edit-fields .data-edit-input').forEach(input => {
        const colName     = input.dataset.col;
        const originalVal = row[colName];
        const isNull      = input.dataset.isNull === 'true';

        if (isNull && originalVal == null)  return; // unchanged null
        if (!isNull && originalVal != null
                    && input.value === String(originalVal)) return; // unchanged value

        if (isNull) {
            payload[colName] = null;
        } else {
            const meta = metaByName[colName];
            const type = meta ? meta.type : '';
            if (isDataIntType(type)) {
                // parseInt converts the input string to a whole number (radix 10 = decimal).
                // If the user typed something non-numeric, parseInt returns NaN
                // (Not a Number). In that case we send the raw string so the
                // server can return a descriptive validation error.
                const n = parseInt(input.value, 10);
                payload[colName] = isNaN(n) ? input.value : n;
            } else if (isDataFloatType(type)) {
                // parseFloat converts the input string to a floating-point number.
                // Same NaN fallback as the integer case above.
                const n = parseFloat(input.value);
                payload[colName] = isNaN(n) ? input.value : n;
            } else {
                payload[colName] = input.value;
            }
        }
    });

    // Object.keys() returns an array of the payload's property names.
    // If that array is empty, nothing changed — close without a network request.
    if (Object.keys(payload).length === 0) { hideDataEdit(); return; }

    document.getElementById('data-edit-save-btn').disabled = true;
    document.getElementById('data-edit-error').textContent = '';

    try {
        const token = getToken();
        const url   = '/dsns/' + encodeURIComponent(dsn)
                    + '/tables/' + encodeURIComponent(table)
                    + "/rows?filter=EQ(" + keyCol + ",'" + keyVal + "')";

        const res  = await fetch(url, {
            method:  'PATCH',
            headers: {
                'Content-Type':  'application/json',
                'Authorization': token ? 'Bearer ' + token : '',
            },
            body: JSON.stringify(payload),
        });
        const data = await res.json().catch(() => ({}));

        if (res.status === 401 || res.status === 403) {
            clearToken();
            hideDataEdit();
            showLogin('Session expired. Please sign in again.');
            return;
        }

        if (!res.ok) {
            document.getElementById('data-edit-error').textContent =
                data.msg || 'Save failed (HTTP ' + res.status + ').';
            document.getElementById('data-edit-save-btn').disabled = false;
            return;
        }

        hideDataEdit();
        await loadDataRows();
    } catch (e) {
        document.getElementById('data-edit-error').textContent = 'Network error: ' + e.message;
        document.getElementById('data-edit-save-btn').disabled = false;
    }
}

// Send a DELETE request for the current row, then reload rows.
async function submitDataDelete() {
    const row    = _dataRows[_dataEditRowIdx];
    const dsn    = document.getElementById('data-dsn-picker').value;
    const table  = document.getElementById('data-table-picker').value;
    const keyCol = findUniqueKeyCol();
    const keyVal = (row && keyCol) ? row[keyCol] : null;

    if (!row || !dsn || !table || keyVal == null) return;

    document.getElementById('data-edit-delete-btn').disabled = true;
    document.getElementById('data-edit-error').textContent = '';

    try {
        const token = getToken();
        const url   = '/dsns/' + encodeURIComponent(dsn)
                    + '/tables/' + encodeURIComponent(table)
                    + "/rows?filter=EQ(" + keyCol + ",'" + keyVal + "')";

        const res  = await fetch(url, {
            method:  'DELETE',
            headers: { 'Authorization': token ? 'Bearer ' + token : '' },
        });
        const data = await res.json().catch(() => ({}));

        if (res.status === 401 || res.status === 403) {
            clearToken();
            hideDataEdit();
            showLogin('Session expired. Please sign in again.');
            return;
        }

        if (!res.ok) {
            document.getElementById('data-edit-error').textContent =
                data.msg || 'Delete failed (HTTP ' + res.status + ').';
            document.getElementById('data-edit-delete-btn').disabled = false;
            return;
        }

        hideDataEdit();
        await loadDataRows();
    } catch (e) {
        document.getElementById('data-edit-error').textContent = 'Network error: ' + e.message;
        document.getElementById('data-edit-delete-btn').disabled = false;
    }
}

// Close the row edit sheet.
function hideDataEdit() {
    document.getElementById('data-edit-overlay').style.display = 'none';
}

// Switch to the SQL tab with a generated SELECT pre-loaded from the current
// Data tab selection. Only columns that are currently visible are included;
// if all columns are visible (the default) SELECT * is used instead.
async function openDataAsSql() {
    const dsn   = document.getElementById('data-dsn-picker').value;
    const table = document.getElementById('data-table-picker').value;
    if (!dsn || !table) return;

    // Filter out the internal _row_id_ column, then check which are visible.
    const allCols = _dataColumnMeta.filter(col => col.name !== '_row_id_');
    const visCols = allCols
        .filter(col => _dataColumnVisible[col.name] !== false)
        .map(col => col.name);

    // Use * when everything is visible; otherwise list only the visible columns.
    const colList = (visCols.length === 0 || visCols.length === allCols.length)
        ? '*'
        : visCols.join(', ');

    const sql = '// ' + dsn + ' dsn\n\nSELECT ' + colList + '\nFROM ' + table;

    // Switch to the SQL tab (triggers loadSql() to populate the DSN picker).
    openTab('sql');

    // Await a second loadSql() call to ensure the picker options are ready
    // before we set the value — openTab() fires it without awaiting.
    await loadSql();

    const picker = document.getElementById('sql-dsn-picker');
    if (Array.from(picker.options).some(o => o.value === dsn)) {
        picker.value = dsn;
    }

    const editor = document.getElementById('sql-editor');
    editor.value = sql;
    updateSqlHighlight();
    await submitSql();
}

// ==========================================================================
// SQL tab
// ==========================================================================

// -----------------------------------------------------------------------
// SQL syntax highlighting
//
// sqlHighlight(code) returns an HTML string with <span class="sql-hl-*">
// elements. The highlight layer <pre> is stacked behind a transparent
// <textarea>, reusing the same overlay technique as the Code tab.
// -----------------------------------------------------------------------

const SQL_KEYWORDS = new Set([
    'ADD','ALL','ALTER','AND','ANY','AS','ASC','AUTO_INCREMENT',
    'BEGIN','BETWEEN','BY',
    'CASE','CHECK','COLUMN','COMMIT','CONSTRAINT','CROSS','CREATE',
    'DATABASE','DEFAULT','DELETE','DESC','DISTINCT','DROP',
    'ELSE','END','EXCEPT','EXISTS','EXPLAIN',
    'FOREIGN','FROM','FULL',
    'GROUP','GRANT',
    'HAVING',
    'IF','IN','INDEX','INNER','INSERT','INTERSECT','INTO','IS',
    'JOIN',
    'KEY',
    'LEFT','LIKE','LIMIT',
    'MERGE',
    'NOT','NULL',
    'OFFSET','ON','OR','ORDER','OUTER',
    'PARTITION','PRIMARY',
    'RECURSIVE','REFERENCES','REPLACE','RETURNING','REVOKE','RIGHT','ROLLBACK',
    'SAVEPOINT','SCHEMA','SELECT','SET','SOME',
    'TABLE','THEN','TRANSACTION','TRUNCATE',
    'UNION','UNIQUE','UPDATE','USING',
    'VALUES','VIEW',
    'WHEN','WHERE','WITH',
]);

const SQL_TYPES = new Set([
    'ARRAY','BIGINT','BIT','BLOB','BOOL','BOOLEAN','BYTEA',
    'CHAR','CLOB','DATE','DATETIME','DECIMAL','DOUBLE',
    'FLOAT','INT','INT2','INT4','INT8','INTEGER','JSON','JSONB',
    'MONEY','NCHAR','NUMERIC','NVARCHAR',
    'REAL','SERIAL','SMALLINT','TEXT','TIME','TIMESTAMP','TINYINT',
    'UUID','VARCHAR','XML','YEAR',
]);

function sqlHighlight(code) {
    function esc(s) {
        return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
    }
    function span(cls, s) {
        return '<span class="sql-hl-' + cls + '">' + esc(s) + '</span>';
    }

    let out = '';
    let i   = 0;
    const n = code.length;

    while (i < n) {
        const ch  = code[i];
        const ch2 = code[i + 1];

        // Block comment  /* ... */
        if (ch === '/' && ch2 === '*') {
            const end = code.indexOf('*/', i + 2);
            if (end === -1) { out += span('comment', code.slice(i)); break; }
            out += span('comment', code.slice(i, end + 2));
            i = end + 2;
            continue;
        }

        // // line comment — no highlighting; output as plain text
        if (ch === '/' && ch2 === '/') {
            const lineStart = code.lastIndexOf('\n', i - 1) + 1;
            if (code.slice(lineStart, i).trim() === '') {
                const nl  = code.indexOf('\n', i);
                const end = nl === -1 ? n : nl;
                out += esc(code.slice(i, end));
                i = end;
                continue;
            }
        }

        // Line comment  -- ...
        if (ch === '-' && ch2 === '-') {
            const nl  = code.indexOf('\n', i);
            const end = nl === -1 ? n : nl;
            out += span('comment', code.slice(i, end));
            i = end;
            continue;
        }

        // Single-quoted string  '...'  (SQL standard; escape via '')
        if (ch === "'") {
            let j = i + 1;
            while (j < n) {
                if (code[j] === "'" && code[j + 1] === "'") { j += 2; continue; } // escaped quote
                if (code[j] === "'") { j++; break; }
                j++;
            }
            out += span('string', code.slice(i, j));
            i = j;
            continue;
        }

        // Double-quoted identifier  "..."
        if (ch === '"') {
            let j = i + 1;
            while (j < n && code[j] !== '"') j++;
            if (j < n) j++;
            out += span('ident', code.slice(i, j));
            i = j;
            continue;
        }

        // Backtick-quoted identifier  `...`  (MySQL style)
        if (ch === '`') {
            let j = i + 1;
            while (j < n && code[j] !== '`') j++;
            if (j < n) j++;
            out += span('ident', code.slice(i, j));
            i = j;
            continue;
        }

        // Numeric literal  (integers and decimals)
        if (/[0-9]/.test(ch) || (ch === '.' && /[0-9]/.test(ch2))) {
            let j = i;
            while (j < n && /[0-9]/.test(code[j])) j++;
            if (j < n && code[j] === '.') {
                j++;
                while (j < n && /[0-9]/.test(code[j])) j++;
            }
            if (j < n && (code[j] === 'e' || code[j] === 'E')) {
                j++;
                if (j < n && (code[j] === '+' || code[j] === '-')) j++;
                while (j < n && /[0-9]/.test(code[j])) j++;
            }
            out += span('number', code.slice(i, j));
            i = j;
            continue;
        }

        // Identifier, keyword, type name, or function call
        if (/[a-zA-Z_]/.test(ch)) {
            let j = i;
            while (j < n && /[a-zA-Z0-9_]/.test(code[j])) j++;
            const word    = code.slice(i, j);
            const wordUp  = word.toUpperCase();
            // Look past whitespace to detect a following '(' (function call).
            let k = j;
            while (k < n && (code[k] === ' ' || code[k] === '\t')) k++;
            if (SQL_KEYWORDS.has(wordUp)) {
                out += span('keyword', word);
            } else if (SQL_TYPES.has(wordUp)) {
                out += span('type', word);
            } else if (code[k] === '(') {
                out += span('func', word);
            } else {
                out += esc(word);
            }
            i = j;
            continue;
        }

        out += esc(ch);
        i++;
    }

    return out + ' ';
}

// Rebuild the SQL highlight layer from the current textarea content.
function updateSqlHighlight() {
    const editor = document.getElementById('sql-editor');
    const layer  = document.getElementById('sql-highlight-layer');
    layer.innerHTML    = sqlHighlight(editor.value);
    layer.scrollTop    = editor.scrollTop;
    layer.scrollLeft   = editor.scrollLeft;
}

// Scan the SQL editor text for a DSN hint comment of the form:
//   // <name> dsn
// The word "dsn" is matched case-insensitively. If a matching option exists
// in the DSN picker, the picker is switched to that value automatically.
// This lets users embed a DSN hint in a saved query so the right database is
// selected when the query is pasted into the editor.
function applySqlDsnHint(text) {
    const m = text.match(/^\s*\/\/\s+(\S+)\s+dsn\s*$/im);
    if (!m) return;
    const hint   = m[1];
    const picker = document.getElementById('sql-dsn-picker');
    const opt    = Array.from(picker.options).find(
        o => o.value.toLowerCase() === hint.toLowerCase()
    );
    if (opt) picker.value = opt.value;
}

// Wire the SQL editor's input and scroll events once the DOM is ready.
// Called at the end of this file so the elements exist.
function initSqlEditor() {
    const editor = document.getElementById('sql-editor');
    const layer  = document.getElementById('sql-highlight-layer');

    editor.addEventListener('input', updateSqlHighlight);

    // The SQL editor is actually two overlapping elements: a transparent
    // <textarea> on top (where the user types) and a <pre> underneath that
    // renders the same text with syntax-highlighting spans. Keeping their
    // scroll positions in sync makes the highlight layer track the visible
    // portion of the textarea as the user scrolls.
    editor.addEventListener('scroll', () => {
        layer.scrollTop  = editor.scrollTop;
        layer.scrollLeft = editor.scrollLeft;
    });

    // Ctrl/Cmd+Enter submits; plain Enter checks for a DSN hint comment.
    editor.addEventListener('keydown', e => {
        if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') submitSql();
        else if (e.key === 'Enter') applySqlDsnHint(editor.value);
    });

    // Load a selected file into the editor.
    document.getElementById('sql-file-input').addEventListener('change', e => {
        const file = e.target.files[0];
        if (!file) return;
        const reader = new FileReader();
        reader.onload = ev => {
            editor.value = ev.target.result;
            updateSqlHighlight();
            applySqlDsnHint(editor.value);
        };
        reader.readAsText(file);
        // Reset so the same file can be re-opened if needed.
        e.target.value = '';
    });

    updateSqlHighlight();
}

// Wire the drag handle between the SQL input pane and the results pane.
// Dragging the handle adjusts the flex-basis of the top pane in pixels,
// while the bottom pane (flex:1) absorbs the remaining space.
function initSqlResizeHandle() {
    const handle  = document.getElementById('sql-resize-handle');
    const topPane = document.getElementById('sql-input-area');
    if (!handle || !topPane) return;

    let startY, startH;

    handle.addEventListener('mousedown', e => {
        startY = e.clientY;
        startH = topPane.getBoundingClientRect().height;
        handle.classList.add('dragging');
        document.body.style.cursor    = 'ns-resize';
        document.body.style.userSelect = 'none';
        document.addEventListener('mousemove', onDrag);
        document.addEventListener('mouseup', stopDrag);
        e.preventDefault();
    });

    function onDrag(e) {
        const newH = Math.max(60, startH + (e.clientY - startY));
        topPane.style.flex = '0 0 ' + newH + 'px';
    }

    function stopDrag() {
        handle.classList.remove('dragging');
        document.body.style.cursor    = '';
        document.body.style.userSelect = '';
        document.removeEventListener('mousemove', onDrag);
        document.removeEventListener('mouseup', stopDrag);
    }
}

// Open a file-picker dialog and load the chosen text file into the SQL editor.
function openSqlFile() {
    document.getElementById('sql-file-input').click();
}

// Save the SQL editor contents to a file. Uses the File System Access API
// (showSaveFilePicker) when available for a native Save dialog; falls back to
// a Blob-URL download for browsers that do not support it (Firefox, Safari).
async function saveSqlFile() {
    const text = document.getElementById('sql-editor')?.value || '';
    const defaultName = 'query.sql';

    if (typeof window.showSaveFilePicker === 'function') {
        try {
            const handle = await window.showSaveFilePicker({
                suggestedName: defaultName,
                types: [{
                    description: 'SQL file',
                    accept: { 'text/plain': ['.sql', '.txt'] }
                }]
            });
            const writable = await handle.createWritable();
            await writable.write(text);
            await writable.close();
            return;
        } catch (e) {
            // User cancelled the dialog — do nothing.
            if (e.name === 'AbortError') return;
            // Any other error falls through to the download fallback.
            console.warn('showSaveFilePicker failed, using download fallback:', e);
        }
    }

    // Fallback for Firefox/Safari which do not support showSaveFilePicker.
    // We create an in-memory Blob containing the text, generate a temporary
    // object URL pointing to it, attach that URL to a hidden <a> element with
    // the "download" attribute, and programmatically click it. The browser
    // treats this as a file download — it will prompt the user with a Save As
    // dialog if the browser is configured to ask where to save files, or save
    // directly to the default Downloads folder otherwise.
    // The object URL is revoked immediately after the click to free memory;
    // the browser has already queued the download by that point.
    const blob = new Blob([text], { type: 'text/plain' });
    const url  = URL.createObjectURL(blob);
    const a    = document.createElement('a');
    a.href     = url;
    a.download = defaultName;
    a.click();
    URL.revokeObjectURL(url);
}

// Load the SQL tab — populates the DSN picker, records each DSN's database
// provider in _sqlDsnProviders (used by the ALTER TABLE wizard for dialect
// decisions), and refreshes the syntax highlight layer.
async function loadSql() {
    const picker = document.getElementById('sql-dsn-picker');
    const previousDsn = picker.value;

    try {
        const res   = await apiFetch('/dsns');
        const data  = await res.json();
        const items = data.items || [];
        // Always refresh the provider map — needed by the ALTER TABLE wizard.
        _sqlDsnProviders = {};
        for (const d of items) _sqlDsnProviders[d.name] = d.provider || '';
        const dsns = items.map(d => d.name).sort();

        const currentOptions = Array.from(picker.options).map(o => o.value);
        const listChanged    = dsns.join(',') !== currentOptions.join(',');

        if (listChanged) {
            picker.innerHTML = '';
            if (dsns.length === 0) {
                picker.innerHTML = '<option value="">— no DSNs —</option>';
                return;
            }
            for (const name of dsns) {
                const opt = document.createElement('option');
                opt.value       = name;
                opt.textContent = name;
                picker.appendChild(opt);
            }
            if (previousDsn && dsns.includes(previousDsn)) {
                picker.value = previousDsn;
            }
        }
    } catch (e) {
        if (e.message !== 'Unauthorized') console.error('Error loading DSNs for SQL tab:', e);
    }

    updateSqlHighlight();
}

// Clear the SQL editor and results area.
function clearSql() {
    document.getElementById('sql-editor').value = '';
    document.getElementById('sql-status').innerHTML  = '';
    document.getElementById('sql-results').innerHTML = '';
    updateSqlHighlight();
    document.getElementById('sql-editor').focus();
}

// Preprocess raw SQL editor text into an array of complete statements.
//
// Rules applied in order:
//   1. Lines whose first non-whitespace text is "//" are comments — dropped.
//   2. Blank lines are dropped.
//   3. Lines are accumulated across newlines until a ";" appears at the end of
//      the accumulated text (ignoring trailing whitespace). The end of the
//      buffer is treated as an implied ";", so the last statement is always
//      emitted even without an explicit semicolon.
//
// The returned array contains one element per complete SQL statement.
function preprocessSql(text) {
    // Steps 1 & 2: drop comment lines and blank lines.
    const raw = text.split('\n').filter(line => {
        const t = line.trim();
        return t.length > 0 && !t.startsWith('//');
    });

    // Step 3: join continuation lines (those not ending with ";") into single
    // statements, emitting each statement as one array element.
    const stmts = [];
    let current = '';

    for (const line of raw) {
        current = current.length > 0
            ? current + ' ' + line.trim()
            : line.trim();

        if (/;\s*$/.test(current)) {
            stmts.push(current.trimEnd());
            current = '';
        }
    }

    // Emit any trailing content (implied ";" at end of buffer).
    if (current.trim().length > 0) stmts.push(current.trim());

    return stmts;
}

// Execute the SQL commands — preprocesses the editor text into a clean array
// of statements, then PUT the array to the server and render the result.
async function submitSql() {
    const dsn  = document.getElementById('sql-dsn-picker').value;
    const text = document.getElementById('sql-editor').value;

    if (!dsn)  { document.getElementById('sql-status').textContent = 'Select a DSN first.'; return; }
    if (!text.trim()) { document.getElementById('sql-status').textContent = 'Enter SQL commands first.'; return; }

    const stmts = preprocessSql(text);
    if (stmts.length === 0) {
        document.getElementById('sql-status').textContent = 'No statements to execute.';
        return;
    }

    // Step 4: warn when a non-final statement begins with SELECT \u2014 the server
    // only returns results for the last statement, so earlier SELECTs are lost.
    const selectInMiddle = stmts.slice(0, -1).some(s => /^\s*select\b/i.test(s));
    const warning = selectInMiddle
        ? '<span class="sql-warning">Warning: only the last statement\u2019s results'
          + ' are returned \u2014 SELECT statements that are not last will be'
          + ' discarded.</span><br>'
        : '';

    document.getElementById('sql-status').innerHTML  = warning + '<span style="color:#666;">Running\u2026</span>';
    document.getElementById('sql-results').innerHTML = '';

    try {
        const token = getToken();
        const res = await fetch('/dsns/' + encodeURIComponent(dsn) + '/tables/@sql', {
            method:  'PUT',
            headers: {
                'Content-Type':  'application/json',
                'Authorization': token ? 'Bearer ' + token : '',
            },
            body: JSON.stringify(stmts),
        });

        if (res.status === 401 || res.status === 403) {
            clearToken();
            showLogin('Session expired. Please sign in again.');
            return;
        }

        const data = await res.json();

        if (!res.ok) {
            const rawMsg = data.msg || 'HTTP ' + res.status;
            document.getElementById('sql-status').innerHTML =
                '<span class="sql-error">' + escapeHtml(stripErrorPrefix(rawMsg)) + '</span>';
            return;
        }

        // Successful response — show either a result table or a row-count message.
        if (data.rows && data.rows.length > 0) {
            document.getElementById('sql-status').innerHTML = '';
            renderSqlResults(data.rows, data.columns);
        } else {
            const count = data.count != null ? data.count : 0;
            document.getElementById('sql-status').innerHTML =
                '<span style="color:#2a7; font-size:0.9rem;">' + "Success, " + count
                + (count === 1 ? ' row affected.' : ' rows affected.') + '</span>';
            document.getElementById('sql-results').innerHTML = '';
        }

        // If any submitted statement was an ALTER, the Data tab's cached column
        // metadata may be stale. Re-fetch it silently so the next Data tab visit
        // (or the current one if it is already open) reflects the new schema.
        if (stmts.some(s => /^\s*alter\b/i.test(s))) {
            loadDataMeta();
        }
    } catch (e) {
        if (e.message !== 'Unauthorized') {
            document.getElementById('sql-status').innerHTML =
                '<span class="sql-error">' + escapeHtml(stripErrorPrefix(e.message)) + '</span>';
        }
    }
}

// Strip a leading "Error: " prefix from error text returned by the server.
function stripErrorPrefix(msg) {
    return msg.startsWith('Error: ') ? msg.slice(7) : msg;
}

// Render an array of row objects as an HTML table in the results area.
// If columns is a non-empty array it dictates column order; otherwise column
// names are derived from the keys of the first row. _row_id_ is always hidden.
function renderSqlResults(rows, columns) {
    const cols = (Array.isArray(columns) && columns.length > 0)
        ? columns.filter(k => k !== '_row_id_')
        : Object.keys(rows[0]).filter(k => k !== '_row_id_');

    let html = '<table><thead><tr>';
    for (const col of cols) {
        html += '<th>' + escapeHtml(col) + '</th>';
    }
    html += '</tr></thead><tbody>';

    for (const row of rows) {
        html += '<tr>';
        for (const col of cols) {
            const val = row[col];
            if (val === null || val === undefined) {
                html += '<td class="sql-null">null</td>';
            } else {
                html += '<td>' + escapeHtml(String(val)) + '</td>';
            }
        }
        html += '</tr>';
    }

    html += '</tbody></table>';
    document.getElementById('sql-results').innerHTML = html;
}

// ==========================================================================
// SQL Build wizard
// ==========================================================================

// Column metadata for the table currently selected in the wizard.
// Populated by sqlWizardTableChanged() and consumed by the clause-row builders.
let _sqlWizardCols = [];

// Full list of tables in the current DSN. Populated by sqlWizardTypeChanged()
// each time the wizard opens so the CREATE wizard can check for name collisions.
let _sqlWizardTables = [];

// Maps DSN name → provider string ("postgres" or "sqlite3").
// Populated by loadSql() when the DSN list is fetched so the ALTER TABLE wizard
// can choose the correct SQL dialect without an extra network call.
let _sqlDsnProviders = {};

// Returns true when the currently selected SQL DSN is a Postgres database.
// Postgres supports multiple column changes in one ALTER TABLE statement;
// SQLite requires one change per statement.
function _sqlWizardIsPostgres() {
    const dsn = document.getElementById('sql-dsn-picker')?.value || '';
    return (_sqlDsnProviders[dsn] || '').toLowerCase() === 'postgres';
}

// When the wizard is opened from a non-empty editor selection, stores the
// character offsets {start, end} of that selection. insertSqlBuild() reads
// this to replace the original selection rather than inserting at cursor.
// Cleared by hideSqlBuild() so no stale range leaks into the next open.
let _sqlWizardSelectionRange = null;

// HTML option elements for the WHERE clause operator picker. Defined once to
// keep the markup consistent across every row that is dynamically added.
// HTML entities are used for < and > so the template string parses correctly.
const _SQL_WIZ_OP_OPTIONS =
    '<option value="=">=</option>'
    + '<option value="&lt;&gt;">&lt;&gt;</option>'
    + '<option value="&lt;">&lt;</option>'
    + '<option value="&lt;=">&lt;=</option>'
    + '<option value="&gt;">&gt;</option>'
    + '<option value="&gt;=">>=</option>'
    + '<option value="IS NULL">IS NULL</option>'
    + '<option value="IS NOT NULL">IS NOT NULL</option>'
    + '<option value="LIKE">LIKE</option>'
    + '<option value="NOT LIKE">NOT LIKE</option>';

// ==========================================================================
// SQL statement parser — pre-loads the wizard from a selected statement
// ==========================================================================

// Tokenize a SQL string into a flat array of token objects with fields:
//   type  — 'ident' (keyword or identifier), 'string' (quoted literal),
//            'number' (numeric literal), or 'op' (operator / punctuation)
//   value — raw string from the source (string literals keep their quotes)
// Whitespace, single-line (-- ...) and block (/* ... */) comments are skipped.
// Backtick-quoted identifiers are unwrapped to plain 'ident' tokens.
function _sqlParseTok(sql) {
    const tokens = [];
    let i = 0;
    while (i < sql.length) {
        if (/\s/.test(sql[i])) { i++; continue; }

        // Single-line comment: skip to end of line
        if (sql[i] === '-' && sql[i + 1] === '-') {
            while (i < sql.length && sql[i] !== '\n') i++;
            continue;
        }

        // Block comment: skip to closing */
        if (sql[i] === '/' && sql[i + 1] === '*') {
            i += 2;
            while (i < sql.length && !(sql[i] === '*' && sql[i + 1] === '/')) i++;
            i += 2;
            continue;
        }

        // Single-quoted string literal — '' inside a string is an escaped quote
        if (sql[i] === "'") {
            let s = "'";
            i++;
            while (i < sql.length) {
                if (sql[i] === "'" && sql[i + 1] === "'") { s += "''"; i += 2; }
                else if (sql[i] === "'")                   { s += "'";  i++;    break; }
                else                                        { s += sql[i++]; }
            }
            tokens.push({ type: 'string', value: s });
            continue;
        }

        // Backtick-quoted identifier: strip the backticks
        if (sql[i] === '`') {
            let s = '';
            i++;
            while (i < sql.length && sql[i] !== '`') s += sql[i++];
            i++;
            tokens.push({ type: 'ident', value: s });
            continue;
        }

        // Numeric literal (optionally negative: -42, 3.14)
        if (/[0-9]/.test(sql[i]) || (sql[i] === '-' && /[0-9]/.test(sql[i + 1]))) {
            let s = '';
            if (sql[i] === '-') s += sql[i++];
            while (i < sql.length && /[0-9.]/.test(sql[i])) s += sql[i++];
            tokens.push({ type: 'number', value: s });
            continue;
        }

        // Two-character operators: <>, <=, >=, !=
        const two = sql.substring(i, i + 2);
        if (two === '<>' || two === '<=' || two === '>=' || two === '!=') {
            tokens.push({ type: 'op', value: two });
            i += 2;
            continue;
        }

        // Single-character operators and punctuation
        if ('<>=!(),.*'.includes(sql[i])) {
            tokens.push({ type: 'op', value: sql[i++] });
            continue;
        }

        // Identifier or keyword (A-Z, a-z, underscore, then alphanumeric/$/_)
        if (/[A-Za-z_]/.test(sql[i])) {
            let s = '';
            while (i < sql.length && /[A-Za-z0-9_$]/.test(sql[i])) s += sql[i++];
            tokens.push({ type: 'ident', value: s });
            continue;
        }

        i++; // skip any other character (e.g. @ in vendor-specific extensions)
    }
    return tokens;
}

// Attempt to parse a SQL statement string and return a plain object describing
// its structure, or null if the statement cannot be understood. Only simple
// single-table statements are accepted. Any of the following cause a null return:
// JOINs, subqueries, OR conditions in WHERE, DISTINCT, GROUP BY, HAVING, LIMIT,
// PRIMARY KEY, or DEFAULT column constraints in CREATE TABLE.
//
// Returned object shapes (varies by .type):
//   SELECT — { type, table, selectAll, cols[], where[], order[] }
//   INSERT — { type, table, cols[], vals[] }
//   UPDATE — { type, table, sets[{col,val}], where[] }
//   DELETE — { type, table, where[] }
//   CREATE — { type, table, cols[{name,type,unique,nullable}] }
//   ALTER  — { type, op, table, ...op-specific fields }
//              op='ADD'    → cols[{name,type,unique,nullable}]
//              op='DROP'   → cols[]  (column name strings)
//              op='RENAME' → renames[{from,to}]
// where[] entries are { col, op, val }; order[] entries are { col, dir }.
function parseSqlStatement(sql) {
    const tokens = _sqlParseTok(sql);
    let pos = 0;

    // Low-level cursor helpers — all use throw 0 on mismatch so the outer
    // try/catch can convert any parse failure into a null return without needing
    // to thread error codes through every nested call.
    const peek     = ()  => tokens[pos] || null;
    const peekVal  = ()  => tokens[pos] ? tokens[pos].value.toUpperCase() : null;
    const peek2Val = ()  => tokens[pos + 1] ? tokens[pos + 1].value.toUpperCase() : null;
    const consume  = ()  => tokens[pos++];
    const expect  = v   => {
        if (!tokens[pos] || tokens[pos].value.toUpperCase() !== v.toUpperCase()) throw 0;
        return tokens[pos++];
    };
    const ident   = ()  => {
        const t = tokens[pos];
        if (!t || t.type !== 'ident') throw 0;
        pos++;
        return t.value;
    };
    // True when all remaining tokens are trailing semicolons. SQL statements
    // often end with ';', so we skip them rather than treating one as an error,
    // which lets a user selection that includes the semicolon still parse cleanly.
    const atEnd   = ()  => { while (tokens[pos]?.value === ';') pos++; return pos >= tokens.length; };

    // Parse a literal value: quoted string, number, or the NULL keyword.
    // String and number tokens are returned as-is (strings keep their quotes).
    function value() {
        const t = peek();
        if (!t) throw 0;
        if (t.type === 'string' || t.type === 'number') { consume(); return t.value; }
        if (t.type === 'ident' && t.value.toUpperCase() === 'NULL') { consume(); return 'NULL'; }
        throw 0;
    }

    // Parse a comma-separated list of identifiers, stopping before stopKeyword.
    // For example, identList('FROM') collects ['a','b','c'] from "a, b, c FROM ..."
    // without consuming the FROM token, so the caller can expect() it next.
    function identList(stopKeyword) {
        const list = [ident()];
        while (peekVal() === ',') {
            consume();
            if (stopKeyword && peekVal() === stopKeyword.toUpperCase()) break;
            list.push(ident());
        }
        return list;
    }

    // Parse AND-joined WHERE conditions. OR causes a throw so the whole
    // statement is rejected — the wizard cannot represent OR logic.
    // Supports: col op val, col LIKE val, col NOT LIKE val,
    //           col IS NULL, col IS NOT NULL.
    function whereConditions() {
        const conds = [];
        while (true) {
            const col = ident();
            if (peekVal() === 'IS') {
                consume();
                if (peekVal() === 'NOT') { consume(); expect('NULL'); conds.push({ col, op: 'IS NOT NULL', val: '' }); }
                else                     { expect('NULL');             conds.push({ col, op: 'IS NULL',     val: '' }); }
            } else if (peekVal() === 'NOT') {
                consume(); expect('LIKE');
                conds.push({ col, op: 'NOT LIKE', val: value() });
            } else if (peekVal() === 'LIKE') {
                consume();
                conds.push({ col, op: 'LIKE', val: value() });
            } else {
                const t = peek();
                // The operator must be a punctuation token (=, <>, <, <=, >, >=, !=)
                if (!t || t.type !== 'op') throw 0;
                consume();
                conds.push({ col, op: t.value, val: value() });
            }
            if (peekVal() === 'OR')  throw 0; // OR not supported by wizard
            if (peekVal() !== 'AND') break;
            consume(); // skip AND
        }
        return conds;
    }

    try {
        const kw = ident().toUpperCase();

        // ---- SELECT ----
        if (kw === 'SELECT') {
            if (peekVal() === 'DISTINCT') throw 0; // wizard has no deduplication control
            let cols = []; let selectAll = false;
            if (peek()?.value === '*') { consume(); selectAll = true; }
            else cols = identList('FROM');
            expect('FROM');
            const table = ident();
            // Reject any type of JOIN
            const jkw = peekVal();
            if (jkw === 'JOIN' || jkw === 'INNER' || jkw === 'LEFT' || jkw === 'RIGHT'
                    || jkw === 'OUTER' || jkw === 'CROSS' || jkw === 'FULL') throw 0;
            let where = [];
            if (peekVal() === 'WHERE') { consume(); where = whereConditions(); }
            // GROUP BY, HAVING, and LIMIT have no equivalent wizard controls
            if (peekVal() === 'GROUP' || peekVal() === 'HAVING' || peekVal() === 'LIMIT') throw 0;
            let order = [];
            if (peekVal() === 'ORDER') {
                consume(); expect('BY');
                const orderItem = () => {
                    const col = ident();
                    // The JS comma operator (consume(), 'DESC') calls consume() for its
                    // side effect (advancing past the keyword) then evaluates to 'DESC'.
                    const dir = peekVal() === 'DESC' ? (consume(), 'DESC')
                              : peekVal() === 'ASC'  ? (consume(), 'ASC') : 'ASC';
                    return { col, dir };
                };
                order.push(orderItem());
                while (peekVal() === ',') { consume(); if (atEnd()) break; order.push(orderItem()); }
            }
            if (!atEnd()) throw 0;
            return { type: 'SELECT', table, selectAll, cols, where, order };
        }

        // ---- INSERT ----
        if (kw === 'INSERT') {
            expect('INTO');
            const table = ident();
            expect('(');
            const cols = identList(')');
            expect(')');
            expect('VALUES');
            expect('(');
            const vals = [value()];
            // Break early if a trailing comma precedes ')' — some formatters emit them
            while (peekVal() === ',') { consume(); if (peek()?.value === ')') break; vals.push(value()); }
            expect(')');
            if (!atEnd()) throw 0;
            // A mismatch means the statement was malformed (e.g. missing a value)
            if (cols.length !== vals.length) throw 0;
            return { type: 'INSERT', table, cols, vals };
        }

        // ---- UPDATE ----
        if (kw === 'UPDATE') {
            const table = ident();
            expect('SET');
            const sets = [];
            // parseSet reads one "col = val" pair and appends it to the sets[] array.
            const parseSet = () => { const col = ident(); expect('='); sets.push({ col, val: value() }); };
            parseSet();
            // Stop before WHERE so the comma after the last SET pair doesn't consume it.
            while (peekVal() === ',') { consume(); if (peekVal() === 'WHERE') break; parseSet(); }
            let where = [];
            if (peekVal() === 'WHERE') { consume(); where = whereConditions(); }
            if (!atEnd()) throw 0;
            return { type: 'UPDATE', table, sets, where };
        }

        // ---- DELETE ----
        if (kw === 'DELETE') {
            expect('FROM');
            const table = ident();
            let where = [];
            if (peekVal() === 'WHERE') { consume(); where = whereConditions(); }
            if (!atEnd()) throw 0;
            return { type: 'DELETE', table, where };
        }

        // ---- CREATE TABLE ----
        if (kw === 'CREATE') {
            expect('TABLE');
            // Optional IF NOT EXISTS modifier
            if (peekVal() === 'IF') { consume(); expect('NOT'); expect('EXISTS'); }
            const table = ident();
            expect('(');
            const cols = [];
            const parseColDef = () => {
                const name = ident();
                let type   = ident().toUpperCase();
                // Handle parameterized types: VARCHAR(255), DECIMAL(10,2)
                if (peek()?.value === '(') {
                    type += '(';
                    consume();
                    if (peek()) { type += peek().value; consume(); }
                    if (peekVal() === ',') { consume(); if (peek()) { type += ',' + peek().value; consume(); } }
                    expect(')');
                    type += ')';
                }
                let unique = false; let nullable = true;
                // Parse optional per-column modifiers in any order. The "outer:" label
                // is needed because a plain "break" inside a switch exits only the switch,
                // not the enclosing while loop. "break outer" exits both at once.
                outer: while (true) {
                    switch (peekVal()) {
                        case 'UNIQUE':   consume(); unique   = true;  break;
                        case 'NULL':     consume(); nullable = true;  break;
                        case 'NOT':      consume(); expect('NULL'); nullable = false; break;
                        // PRIMARY KEY and DEFAULT cannot be represented in the wizard
                        case 'PRIMARY': case 'DEFAULT': throw 0;
                        default: break outer;
                    }
                }
                cols.push({ name, type, unique, nullable });
            };
            parseColDef();
            while (peekVal() === ',') {
                consume();
                if (peek()?.value === ')') break;
                // Table-level constraints (PRIMARY KEY (...), UNIQUE KEY, etc.) not supported
                if (peekVal() === 'PRIMARY' || peekVal() === 'UNIQUE' || peekVal() === 'KEY'
                        || peekVal() === 'INDEX' || peekVal() === 'CONSTRAINT') throw 0;
                parseColDef();
            }
            expect(')');
            if (!atEnd()) throw 0;
            return { type: 'CREATE', table, cols };
        }

        // ---- ALTER TABLE ----
        if (kw === 'ALTER') {
            expect('TABLE');
            const table = ident();
            const sub   = ident().toUpperCase();

            // Shared column-definition parser — mirrors the CREATE TABLE one.
            const parseColDef = () => {
                if (peekVal() === 'COLUMN') consume(); // COLUMN keyword is optional
                const name = ident();
                let type   = ident().toUpperCase();
                if (peek()?.value === '(') {
                    type += '(';
                    consume();
                    if (peek()) { type += peek().value; consume(); }
                    if (peekVal() === ',') { consume(); if (peek()) { type += ',' + peek().value; consume(); } }
                    expect(')');
                    type += ')';
                }
                let unique = false; let nullable = true;
                outer: while (true) {
                    switch (peekVal()) {
                        case 'UNIQUE': consume(); unique   = true;  break;
                        case 'NULL':   consume(); nullable = true;  break;
                        case 'NOT':    consume(); expect('NULL'); nullable = false; break;
                        case 'PRIMARY': case 'DEFAULT': throw 0;
                        default: break outer;
                    }
                }
                return { name, type, unique, nullable };
            };

            if (sub === 'ADD') {
                const cols = [parseColDef()];
                // Postgres allows comma-separated ADD COLUMN clauses in one statement.
                // peek2Val() looks one token ahead past the comma to check for ADD.
                while (peekVal() === ',' && peek2Val() === 'ADD') {
                    consume(); consume(); // ',' and 'ADD'
                    cols.push(parseColDef());
                }
                if (!atEnd()) throw 0;
                return { type: 'ALTER', op: 'ADD', table, cols };
            }

            if (sub === 'DROP') {
                if (peekVal() === 'COLUMN') consume();
                const cols = [ident()];
                // Postgres: DROP COLUMN c1, DROP COLUMN c2 in one statement.
                while (peekVal() === ',' && peek2Val() === 'DROP') {
                    consume(); consume(); // ',' and 'DROP'
                    if (peekVal() === 'COLUMN') consume();
                    cols.push(ident());
                }
                if (!atEnd()) throw 0;
                return { type: 'ALTER', op: 'DROP', table, cols };
            }

            if (sub === 'RENAME') {
                if (peekVal() === 'COLUMN') consume();
                const fromCol = ident();
                expect('TO');
                const toCol = ident();
                if (!atEnd()) throw 0;
                // Only one RENAME per statement is legal in both Postgres and SQLite;
                // the wizard handles multiple renames as separate ALTER statements.
                return { type: 'ALTER', op: 'RENAME', table, renames: [{ from: fromCol, to: toCol }] };
            }

            throw 0; // Unsupported ALTER sub-command
        }

        return null; // Unrecognized statement keyword
    } catch (e) {
        return null;
    }
}

// Build a pre-populated WHERE clause row and append it to whereList.
// colOptsHtml is the pre-rendered <option>...</option> HTML for the column
// picker. cond is a { col, op, val } object from the parser output.
// This helper is shared by _applyParsedSelect, _applyParsedUpdate, and
// _applyParsedDelete; each passes either _sqlWizardColOptions() (SELECT,
// which excludes _row_id_) or _sqlWizardAllColOptions() (UPDATE/DELETE).
function _addWizardWhereRow(whereList, colOptsHtml, cond) {
    const row = document.createElement('div');
    row.className = 'sql-wiz-clause-row';
    row.innerHTML =
        '<select class="sql-wiz-where-col" onchange="buildSqlPreview()">'
        + colOptsHtml + '</select>'
        + '<select class="sql-wiz-where-op" onchange="sqlWizardWhereOpChanged(this)">'
        + _SQL_WIZ_OP_OPTIONS + '</select>'
        + '<input type="text" class="sql-wiz-where-val" placeholder="value"'
        + ' oninput="buildSqlPreview()">'
        + '<button class="sql-wiz-remove-btn" onclick="sqlWizardRemoveClause(this)">&#x2715;</button>';
    whereList.appendChild(row);

    // Set column picker to the parsed column name (case-insensitive match)
    const colSel = row.querySelector('.sql-wiz-where-col');
    if (colSel) {
        const opt = Array.from(colSel.options)
            .find(o => o.value.toLowerCase() === cond.col.toLowerCase());
        if (opt) colSel.value = opt.value;
    }

    // Set the operator. Normalize != (ANSI) to <> (SQL standard used by wizard).
    const opSel = row.querySelector('.sql-wiz-where-op');
    if (opSel) {
        const op    = cond.op === '!=' ? '<>' : cond.op;
        const opOpt = Array.from(opSel.options).find(o => o.value === op);
        if (opOpt) {
            opSel.value = opOpt.value;
            // Hide value input for IS NULL / IS NOT NULL (they take no value)
            sqlWizardWhereOpChanged(opSel);
        }
    }

    // Set the value input, stripping surrounding single quotes from string literals
    const valInput = row.querySelector('.sql-wiz-where-val');
    if (valInput && cond.val !== '') {
        valInput.value = cond.val.startsWith("'") && cond.val.endsWith("'")
            ? cond.val.slice(1, -1).replace(/''/g, "'")
            : cond.val;
    }
}

// Open the Build wizard. If the SQL editor has a non-empty selection, attempt
// to parse it as a SQL statement and pre-populate the wizard fields. If the
// selection cannot be parsed (e.g. JOINs, subqueries, OR conditions), ask the
// user whether to open a fresh wizard instead; if they decline, cancel.
// When the wizard is opened from a selection, _sqlWizardSelectionRange stores
// the selection offsets so insertSqlBuild() can replace that selection with
// the built result rather than appending at the cursor position.
async function showSqlBuild() {
    const dsn    = document.getElementById('sql-dsn-picker').value;
    const editor = document.getElementById('sql-editor');
    if (!dsn) {
        document.getElementById('sql-status').textContent = 'Select a DSN before using Build.';
        return;
    }

    let parsed = null;
    _sqlWizardSelectionRange = null;

    const selStart = editor.selectionStart;
    const selEnd   = editor.selectionEnd;
    if (selStart !== selEnd) {
        const selected = editor.value.substring(selStart, selEnd).trim();
        if (selected) {
            parsed = parseSqlStatement(selected);
            if (!parsed) {
                // Could not parse — ask whether to open a fresh wizard instead
                const ok = confirm(
                    'The selected text cannot be loaded into the wizard — it may contain '
                    + 'constructs such as JOINs, subqueries, or OR conditions that the '
                    + 'wizard does not support.\n\nOpen the wizard to start a new statement instead?'
                );
                if (!ok) return;
                // Fall through: open a blank SELECT wizard; insert at cursor, not selection
            } else {
                // Valid parse — store range so Insert will replace the selection
                _sqlWizardSelectionRange = { start: selStart, end: selEnd };
            }
        }
    }

    const typeVal = parsed ? parsed.type : 'SELECT';
    document.getElementById('sql-build-type').value = typeVal;
    _syncTypeButtons(typeVal);
    document.getElementById('sql-build-overlay').style.display = 'flex';
    await sqlWizardTypeChanged();

    // Apply parsed values on top of the freshly-loaded wizard, then re-capture
    // the clean baseline so dirty detection still works correctly.
    if (parsed) {
        await applyParsedToWizard(parsed);
        captureBaseline('sql-build-overlay');
    }
}

// Close the wizard and reset its state.
function hideSqlBuild() {
    document.getElementById('sql-build-overlay').style.display = 'none';
    document.getElementById('sql-build-body').innerHTML = '';
    document.getElementById('sql-build-preview').textContent = '-- Select a table to begin';
    document.getElementById('sql-build-insert-btn').disabled = true;
    _sqlWizardCols = [];
    // Clear the stored selection range so a future open-without-selection
    // inserts at the cursor rather than replacing stale text.
    _sqlWizardSelectionRange = null;
}

// Apply a parsed SQL statement to the wizard after sqlWizardTypeChanged() has
// already built the body. Steps:
//   1. For table-based types (not CREATE), find the parsed table in the picker
//      and reload its columns via the appropriate *TableChanged() function.
//   2. Call the corresponding _applyParsed*() helper to fill in the wizard fields.
//   3. Refresh the live preview via buildSqlPreview().
// If the parsed table is not found in the current DSN the wizard is left at its
// default (first available table) since we cannot pre-populate without columns.
async function applyParsedToWizard(parsed) {
    if (parsed.type === 'CREATE') {
        _applyParsedCreate(parsed);
        buildSqlPreview();
        return;
    }

    // ALTER TABLE uses a separate picker element from the other statement types.
    if (parsed.type === 'ALTER') {
        const picker = document.getElementById('sql-wiz-alter-table');
        if (picker && parsed.table) {
            const opt = Array.from(picker.options)
                .find(o => o.value.toLowerCase() === parsed.table.toLowerCase());
            if (!opt) return;
            picker.value = opt.value;
            await sqlWizardAlterTableChanged();
            _applyParsedAlter(parsed);
        }
        buildSqlPreview();
        return;
    }

    const picker = document.getElementById('sql-wiz-table');
    if (picker && parsed.table) {
        const opt = Array.from(picker.options)
            .find(o => o.value.toLowerCase() === parsed.table.toLowerCase());
        if (!opt) return; // parsed table not in DSN; leave wizard at default

        // Setting picker.value before calling *TableChanged() is essential: those
        // functions read picker.value to determine which table's columns to fetch.
        picker.value = opt.value;

        // Re-trigger the column load for the newly selected table, then apply
        // the parsed values on top of the freshly-loaded wizard fields.
        if (parsed.type === 'SELECT') {
            await sqlWizardTableChanged();
            _applyParsedSelect(parsed);
        } else if (parsed.type === 'INSERT') {
            await sqlWizardInsertTableChanged();
            _applyParsedInsert(parsed);
        } else if (parsed.type === 'UPDATE') {
            await sqlWizardUpdateTableChanged();
            _applyParsedUpdate(parsed);
        } else if (parsed.type === 'DELETE') {
            await sqlWizardDeleteTableChanged();
            _applyParsedDelete(parsed);
        }
    }

    buildSqlPreview();
}

// Pre-populate the SELECT wizard from a parsed SELECT statement.
// If specific column names were listed (not *), unchecks "Select all" to reveal
// the column grid, then checks only the parsed columns. Replaces any WHERE and
// ORDER BY rows with the parsed conditions.
function _applyParsedSelect(parsed) {
    if (!parsed.selectAll && parsed.cols.length > 0) {
        const allCheck = document.getElementById('sql-wiz-select-all');
        if (allCheck) {
            allCheck.checked = false;
            sqlWizardSelectAllChanged(); // shows the column checkbox grid
        }
        // Uncheck all, then check only the parsed columns (case-insensitive).
        // cb.dataset.col reads the data-col="..." HTML attribute set on each checkbox
        // by _sqlWizardColGridHtml() — it holds the column name as the server returned it.
        document.querySelectorAll('#sql-wiz-col-list input[type="checkbox"]').forEach(cb => {
            cb.checked = parsed.cols.some(c => c.toLowerCase() === cb.dataset.col.toLowerCase());
        });
    }

    const whereList = document.getElementById('sql-wiz-where-list');
    if (whereList && parsed.where.length > 0) {
        whereList.innerHTML = '';
        // SELECT WHERE excludes _row_id_ from the column picker
        const colOptsHtml = _sqlWizardColOptions();
        for (const cond of parsed.where) _addWizardWhereRow(whereList, colOptsHtml, cond);
    }

    const orderList = document.getElementById('sql-wiz-order-list');
    if (orderList && parsed.order.length > 0) {
        orderList.innerHTML = '';
        // We build each row manually rather than calling sqlWizardAddOrder() because
        // that function adds a blank row and immediately calls buildSqlPreview() —
        // there is no way to pre-set the column and direction before that preview fires.
        for (const o of parsed.order) {
            const row = document.createElement('div');
            row.className = 'sql-wiz-clause-row';
            row.innerHTML =
                '<select class="sql-wiz-order-col" onchange="buildSqlPreview()">'
                + _sqlWizardColOptions() + '</select>'
                + '<select class="sql-wiz-order-dir" onchange="buildSqlPreview()">'
                + '<option value="ASC">ASC</option>'
                + '<option value="DESC">DESC</option>'
                + '</select>'
                + '<button class="sql-wiz-remove-btn" onclick="sqlWizardRemoveClause(this)">&#x2715;</button>';
            orderList.appendChild(row);
            const colSel = row.querySelector('.sql-wiz-order-col');
            if (colSel) {
                const opt = Array.from(colSel.options)
                    .find(op => op.value.toLowerCase() === o.col.toLowerCase());
                if (opt) colSel.value = opt.value;
            }
            row.querySelector('.sql-wiz-order-dir').value = o.dir;
        }
    }
}

// Pre-populate the INSERT wizard from a parsed INSERT ... VALUES statement.
// For each column in the parsed list, sets the corresponding input value.
// Activates the Null button for columns whose parsed value is the NULL keyword.
function _applyParsedInsert(parsed) {
    document.querySelectorAll('#sql-wiz-insert-fields .sql-wiz-insert-row').forEach(row => {
        const idx = parsed.cols.findIndex(c => c.toLowerCase() === row.dataset.col.toLowerCase());
        if (idx === -1) return;

        const rawVal = parsed.vals[idx];
        const input  = row.querySelector('.sql-wiz-insert-input');
        const btn    = row.querySelector('.sql-wiz-insert-null-btn');

        if (rawVal === 'NULL') {
            // Call the toggle function rather than setting input.dataset.isNull directly
            // so all side effects (disabled state, CSS class) are applied consistently.
            if (btn) sqlWizardInsertNullToggle(btn);
        } else if (input) {
            // Strip surrounding single quotes; un-escape doubled quotes inside
            input.value = rawVal.startsWith("'") && rawVal.endsWith("'")
                ? rawVal.slice(1, -1).replace(/''/g, "'")
                : rawVal;
        }
    });
}

// Pre-populate the UPDATE wizard from a parsed UPDATE ... SET ... WHERE statement.
// For each SET column, checks the include checkbox and fills the value input.
// Replaces the default pre-populated WHERE row with the parsed conditions.
function _applyParsedUpdate(parsed) {
    document.querySelectorAll('#sql-wiz-update-fields .sql-wiz-update-row').forEach(row => {
        const setEntry = parsed.sets.find(s => s.col.toLowerCase() === row.dataset.col.toLowerCase());
        if (!setEntry) return;

        const cb      = row.querySelector('.sql-wiz-update-include');
        const input   = row.querySelector('.sql-wiz-update-input');
        const nullBtn = row.querySelector('.sql-wiz-update-null-btn');

        // sqlWizardUpdateToggleCol applies all side effects of checking the box:
        // removes the dimmed "excluded" class and re-enables the value input.
        if (cb) { cb.checked = true; sqlWizardUpdateToggleCol(cb); }

        if (setEntry.val === 'NULL') {
            // Same reasoning as _applyParsedInsert: use the toggle for side effects.
            if (nullBtn) sqlWizardUpdateNullToggle(nullBtn);
        } else if (input) {
            input.value = setEntry.val.startsWith("'") && setEntry.val.endsWith("'")
                ? setEntry.val.slice(1, -1).replace(/''/g, "'")
                : setEntry.val;
        }
    });

    const whereList = document.getElementById('sql-wiz-where-list');
    if (whereList && parsed.where.length > 0) {
        whereList.innerHTML = '';
        // UPDATE WHERE includes _row_id_ so the user can target a specific row
        const colOptsHtml = _sqlWizardAllColOptions();
        for (const cond of parsed.where) _addWizardWhereRow(whereList, colOptsHtml, cond);
    }
}

// Pre-populate the DELETE wizard from a parsed DELETE ... WHERE statement.
// Replaces the default pre-populated WHERE row with the parsed conditions.
function _applyParsedDelete(parsed) {
    const whereList = document.getElementById('sql-wiz-where-list');
    if (whereList && parsed.where.length > 0) {
        whereList.innerHTML = '';
        // DELETE WHERE includes _row_id_ so the user can target a specific row
        const colOptsHtml = _sqlWizardAllColOptions();
        for (const cond of parsed.where) _addWizardWhereRow(whereList, colOptsHtml, cond);
    }
}

// Pre-populate the CREATE TABLE wizard from a parsed CREATE TABLE statement.
// Sets the table name input and adds one column row per parsed column definition.
function _applyParsedCreate(parsed) {
    const nameInput = document.getElementById('sql-wiz-create-name');
    if (nameInput) {
        nameInput.value = parsed.table;
        sqlWizardCreateNameChanged(); // validates name and updates the status indicator
    }

    // Clear any existing column rows, then add one row per parsed column
    const colList = document.getElementById('sql-wiz-create-cols');
    if (colList) colList.innerHTML = '';

    for (const col of parsed.cols) {
        sqlWizardCreateAddColumn(); // appends a blank row with default values
        // sqlWizardCreateAddColumn() doesn't return the row it just created, so we
        // re-query the list and take the last element to get the newly added row.
        const rows = document.querySelectorAll('#sql-wiz-create-cols .sql-wiz-create-col-row');
        const row  = rows[rows.length - 1];
        if (!row) continue;

        const nameIn = row.querySelector('.sql-wiz-create-col-name');
        if (nameIn) nameIn.value = col.name;

        const typeSel = row.querySelector('.sql-wiz-create-col-type');
        if (typeSel) {
            // Match on base type name only (e.g. "VARCHAR" from "VARCHAR(255)")
            // so the dropdown aligns even when a length parameter was specified.
            const baseType = col.type.replace(/\(.*\)/, '').toUpperCase();
            const opt = Array.from(typeSel.options).find(o => o.value === baseType);
            if (opt) typeSel.value = opt.value;
        }

        const uniqueCb   = row.querySelector('.sql-wiz-create-col-unique');
        const nullableCb = row.querySelector('.sql-wiz-create-col-nullable');
        if (uniqueCb)   uniqueCb.checked   = col.unique;
        if (nullableCb) nullableCb.checked = col.nullable;
    }
}

// Pre-populate the ALTER TABLE wizard from a parsed ALTER TABLE statement.
// sqlWizardAlterTableChanged() has already been awaited before this runs,
// so _sqlWizardCols is current and the op-body HTML exists in the DOM.
function _applyParsedAlter(parsed) {
    // Switch to the correct operation button — also rebuilds the op-body.
    sqlWizardAlterSelectOp(parsed.op);

    if (parsed.op === 'ADD') {
        // Clear the auto-seeded blank row, then replay one row per parsed column.
        const colList = document.getElementById('sql-wiz-alter-cols');
        if (!colList) return;
        colList.innerHTML = '';
        for (const col of parsed.cols) {
            sqlWizardAlterAddColumn(); // appends a blank row
            const rows = colList.querySelectorAll('.sql-wiz-create-col-row');
            const row  = rows[rows.length - 1];
            if (!row) continue;
            const nameIn = row.querySelector('.sql-wiz-create-col-name');
            if (nameIn) nameIn.value = col.name;
            const typeSel = row.querySelector('.sql-wiz-create-col-type');
            if (typeSel) {
                const baseType = col.type.replace(/\(.*\)/, '').toUpperCase();
                const opt = Array.from(typeSel.options).find(o => o.value === baseType);
                if (opt) typeSel.value = opt.value;
            }
            const uniqueCb   = row.querySelector('.sql-wiz-create-col-unique');
            const nullableCb = row.querySelector('.sql-wiz-create-col-nullable');
            if (uniqueCb)   uniqueCb.checked   = col.unique;
            if (nullableCb) nullableCb.checked = col.nullable;
        }
    } else if (parsed.op === 'DROP') {
        // Check the input (checkbox or radio) whose data-col matches each parsed column.
        const allInputs = document.querySelectorAll('#sql-wiz-alter-drop-list input');
        for (const input of allInputs) {
            if (parsed.cols.includes(input.dataset.col)) input.checked = true;
        }
    } else if (parsed.op === 'RENAME') {
        // Fill the new-name input for each parsed rename pair.
        const allRows = document.querySelectorAll('#sql-wiz-alter-rename-list .sql-wiz-rename-row');
        for (const row of allRows) {
            const r = parsed.renames.find(x => x.from === row.dataset.col);
            if (!r) continue;
            const input = row.querySelector('.sql-wiz-rename-input');
            if (input) input.value = r.to;
        }
    }
}

// Sync the active-button highlight across the statement-type button bar.
// Called from showSqlBuild() (to reflect a pre-parsed type without triggering
// a reload) and from sqlWizardSelectType() (which does trigger a reload).
function _syncTypeButtons(type) {
    document.querySelectorAll('.sql-wiz-type-btn').forEach(btn => {
        btn.classList.toggle('sql-wiz-type-btn-active', btn.dataset.type === type);
    });
}

// Called when the user clicks one of the statement-type buttons.
// Updates the hidden value carrier, syncs button highlight, and reloads the wizard body.
function sqlWizardSelectType(type) {
    const hidden = document.getElementById('sql-build-type');
    if (hidden) hidden.value = type;
    _syncTypeButtons(type);
    sqlWizardTypeChanged();
}

// Rebuild the wizard body whenever the statement type changes. Fetches the
// table list first, then the selected table's columns — both in sequence so
// buildSqlPreview() sees a fully-populated wizard when it runs. Handles all
// six supported types: SELECT, INSERT, UPDATE, DELETE, CREATE, and ALTER.
// Called by sqlWizardSelectType() on every button click, and once on first
// open by showSqlBuild().
async function sqlWizardTypeChanged() {
    const type = document.getElementById('sql-build-type').value;
    const body = document.getElementById('sql-build-body');

    const supported = type === 'SELECT' || type === 'INSERT'
                   || type === 'UPDATE' || type === 'DELETE'
                   || type === 'CREATE' || type === 'ALTER';
    if (!supported) {
        body.innerHTML = '<p class="sql-wiz-unsupported">Not supported yet.</p>';
        document.getElementById('sql-build-preview').textContent =
            '-- ' + type + ' is not yet supported by the wizard';
        document.getElementById('sql-build-insert-btn').disabled = true;
        captureBaseline('sql-build-overlay');
        return;
    }

    // Both SELECT and INSERT need the table list first.
    const dsn = document.getElementById('sql-dsn-picker').value;
    body.innerHTML = '<p class="sql-wiz-loading">Loading tables…</p>';

    let tables = [];
    try {
        const res  = await apiFetch('/dsns/' + encodeURIComponent(dsn) + '/tables');
        const data = await res.json();
        tables = res.ok ? (data.tables || []).map(t => t.name).sort() : [];
    } catch (e) {
        if (e.message !== 'Unauthorized') console.error('SQL wizard: error loading tables:', e);
    }

    // Save the table list so the CREATE wizard can check for name collisions
    // without making an additional network request.
    _sqlWizardTables = tables;

    // CREATE does not need an existing table, so it bypasses the "no tables"
    // guard below and goes straight to its own HTML builder.
    if (type === 'CREATE') {
        body.innerHTML = _sqlWizardCreateHtml();
        buildSqlPreview();
        captureBaseline('sql-build-overlay');
        return;
    }

    if (tables.length === 0) {
        body.innerHTML = '<p class="sql-wiz-unsupported">No tables found in this DSN.</p>';
        document.getElementById('sql-build-preview').textContent = '-- No tables available';
        captureBaseline('sql-build-overlay');
        return;
    }

    if (type === 'SELECT') {
        body.innerHTML = _sqlWizardSelectHtml(tables);
        await sqlWizardTableChanged();
    } else if (type === 'INSERT') {
        body.innerHTML = _sqlWizardInsertHtml(tables);
        await sqlWizardInsertTableChanged();
    } else if (type === 'UPDATE') {
        body.innerHTML = _sqlWizardUpdateHtml(tables);
        await sqlWizardUpdateTableChanged();
    } else if (type === 'ALTER') {
        body.innerHTML = _sqlWizardAlterHtml(tables);
        await sqlWizardAlterTableChanged();
    } else {
        body.innerHTML = _sqlWizardDeleteHtml(tables);
        await sqlWizardDeleteTableChanged();
    }

    // Capture the fully-populated wizard state as the clean baseline so that
    // overlayBackdropClick() can detect whether the user has made any changes
    // before dismissing. This runs after all awaited table/column loads complete.
    captureBaseline('sql-build-overlay');
}

// Return the full HTML for the SELECT wizard body given a sorted table list.
// Kept as a separate function so it is easy to add other statement types later.
function _sqlWizardSelectHtml(tables) {
    const opts = tables.map(t =>
        '<option value="' + escapeHtml(t) + '">' + escapeHtml(t) + '</option>'
    ).join('');
    return '<div class="sql-wiz-section">'
        + '<div class="sql-wiz-section-hdr"><span class="sql-wiz-label">Table</span></div>'
        + '<select id="sql-wiz-table" class="sql-wiz-select" onchange="sqlWizardTableChanged()">'
        + opts + '</select></div>'
        + '<div class="sql-wiz-section">'
        + '<div class="sql-wiz-section-hdr"><span class="sql-wiz-label">Columns</span></div>'
        + '<div class="sql-wiz-toggle-row">'
        + '<input type="checkbox" id="sql-wiz-select-all" checked onchange="sqlWizardSelectAllChanged()">'
        + '<label for="sql-wiz-select-all" class="sql-wiz-check-label">Select all columns (*)</label>'
        + '</div>'
        + '<div id="sql-wiz-col-list" style="display:none;"></div>'
        + '</div>'
        + '<div class="sql-wiz-section">'
        + '<div class="sql-wiz-section-hdr">'
        + '<span class="sql-wiz-label">WHERE clause</span>'
        + '<button class="sql-wiz-add-btn" onclick="sqlWizardAddWhere()">+ Add condition</button>'
        + '</div>'
        + '<div id="sql-wiz-where-list"></div>'
        + '</div>'
        + '<div class="sql-wiz-section">'
        + '<div class="sql-wiz-section-hdr">'
        + '<span class="sql-wiz-label">ORDER BY</span>'
        + '<button class="sql-wiz-add-btn" onclick="sqlWizardAddOrder()">+ Add column</button>'
        + '</div>'
        + '<div id="sql-wiz-order-list"></div>'
        + '</div>';
}

// Return the HTML body for the INSERT wizard (table picker + values container).
function _sqlWizardInsertHtml(tables) {
    const opts = tables.map(t =>
        '<option value="' + escapeHtml(t) + '">' + escapeHtml(t) + '</option>'
    ).join('');
    return '<div class="sql-wiz-section">'
        + '<div class="sql-wiz-section-hdr"><span class="sql-wiz-label">Table</span></div>'
        + '<select id="sql-wiz-table" class="sql-wiz-select"'
        + ' onchange="sqlWizardInsertTableChanged()">'
        + opts + '</select></div>'
        + '<div class="sql-wiz-section">'
        + '<div class="sql-wiz-section-hdr"><span class="sql-wiz-label">Values</span></div>'
        + '<div id="sql-wiz-insert-fields">'
        + '<p class="sql-wiz-loading">Loading columns…</p>'
        + '</div></div>';
}

// Fetch column metadata for the INSERT table picker and rebuild the value fields.
// The ?rowids=true query parameter tells the server to include the internal
// _row_id_ column and to add unique/nullable metadata to each column object.
// We then strip _row_id_ from the display list because INSERT should not
// allow the user to set it manually — the database assigns it automatically.
async function sqlWizardInsertTableChanged() {
    const dsn   = document.getElementById('sql-dsn-picker').value;
    const table = document.getElementById('sql-wiz-table')?.value;
    if (!dsn || !table) return;

    _sqlWizardCols = [];
    try {
        const res  = await apiFetch(
            '/dsns/' + encodeURIComponent(dsn)
            + '/tables/' + encodeURIComponent(table) + '?rowids=true'
        );
        const data = await res.json();
        _sqlWizardCols = res.ok ? (data.columns || []) : [];
    } catch (e) {
        if (e.message !== 'Unauthorized') console.error('SQL wizard: error loading columns:', e);
    }

    const cols      = _sqlWizardCols.filter(c => c.name !== '_row_id_');
    const container = document.getElementById('sql-wiz-insert-fields');
    if (container) container.innerHTML = _sqlWizardInsertFieldsHtml(cols);
    buildSqlPreview();
}

// Return the HTML for the INSERT column-value form. Each row shows the column
// name, its SQL type as a hint, a text input, and a Null toggle button.
// Each row div carries data-col and data-type attributes so that
// buildSqlPreview() can read the column name and type without querying
// _sqlWizardCols again. data-is-null starts as "false"; sqlWizardInsertNullToggle()
// flips it to "true" when the Null button is active.
function _sqlWizardInsertFieldsHtml(cols) {
    if (cols.length === 0) return '<p class="sql-wiz-unsupported">No columns available.</p>';
    return cols.map(c => {
        const nullable = c.nullable && c.nullable.value;
        const typeHint = escapeHtml(c.type || '');
        return '<div class="sql-wiz-insert-row" data-col="' + escapeHtml(c.name)
            + '" data-type="' + typeHint + '">'
            + '<span class="sql-wiz-insert-label">'
            + '<span class="sql-wiz-insert-colname">' + escapeHtml(c.name) + '</span>'
            + '<span class="sql-wiz-insert-typehint">' + typeHint
            + (nullable ? '' : ' &bull;') + '</span>'
            + '</span>'
            + '<input type="text" class="sql-wiz-insert-input" data-is-null="false"'
            + ' oninput="buildSqlPreview()" autocomplete="off" spellcheck="false"'
            + ' placeholder="">'
            + '<button class="sql-wiz-insert-null-btn"'
            + ' onclick="sqlWizardInsertNullToggle(this)">Null</button>'
            + '</div>';
    }).join('');
}

// Toggle the null state for an INSERT value row. When null is active the input
// is visually muted and disabled so it cannot be edited; buildSqlPreview()
// will emit NULL for that column instead of a quoted value.
// State is tracked via input.dataset.isNull ("true"/"false") rather than a
// separate variable so the state survives DOM re-reads without extra bookkeeping.
function sqlWizardInsertNullToggle(btn) {
    const row   = btn.closest('.sql-wiz-insert-row');
    const input = row?.querySelector('.sql-wiz-insert-input');
    if (!row || !input) return;

    const wasNull = input.dataset.isNull === 'true';
    if (wasNull) {
        input.dataset.isNull = 'false';
        input.disabled = false;
        input.classList.remove('sql-wiz-insert-null');
        btn.classList.remove('sql-wiz-insert-null-active');
    } else {
        input.dataset.isNull = 'true';
        input.disabled = true;
        input.classList.add('sql-wiz-insert-null');
        btn.classList.add('sql-wiz-insert-null-active');
    }
    buildSqlPreview();
}

// ==========================================================================
// UPDATE wizard
// ==========================================================================

// Return the HTML body for the UPDATE wizard (table, SET fields, WHERE section).
function _sqlWizardUpdateHtml(tables) {
    const opts = tables.map(t =>
        '<option value="' + escapeHtml(t) + '">' + escapeHtml(t) + '</option>'
    ).join('');
    return '<div class="sql-wiz-section">'
        + '<div class="sql-wiz-section-hdr"><span class="sql-wiz-label">Table</span></div>'
        + '<select id="sql-wiz-table" class="sql-wiz-select"'
        + ' onchange="sqlWizardUpdateTableChanged()">'
        + opts + '</select></div>'
        + '<div class="sql-wiz-section">'
        + '<div class="sql-wiz-section-hdr">'
        + '<span class="sql-wiz-label">SET values'
        + ' <span class="sql-wiz-hint">— check columns to update</span></span>'
        + '</div>'
        + '<div id="sql-wiz-update-fields">'
        + '<p class="sql-wiz-loading">Loading columns…</p>'
        + '</div></div>'
        + '<div class="sql-wiz-section">'
        + '<div class="sql-wiz-section-hdr">'
        + '<span class="sql-wiz-label">WHERE'
        + ' <span class="sql-wiz-required">(required)</span></span>'
        + '<button class="sql-wiz-add-btn" onclick="sqlWizardUpdateAddWhere()">'
        + '+ Add condition</button>'
        + '</div>'
        + '<div id="sql-wiz-where-list"></div>'
        + '</div>';
}

// Fetch column metadata for the UPDATE table picker, rebuild the SET fields,
// and pre-populate the WHERE list with the table's first unique column.
async function sqlWizardUpdateTableChanged() {
    const dsn   = document.getElementById('sql-dsn-picker').value;
    const table = document.getElementById('sql-wiz-table')?.value;
    if (!dsn || !table) return;

    _sqlWizardCols = [];
    try {
        const res  = await apiFetch(
            '/dsns/' + encodeURIComponent(dsn)
            + '/tables/' + encodeURIComponent(table) + '?rowids=true'
        );
        const data = await res.json();
        _sqlWizardCols = res.ok ? (data.columns || []) : [];
    } catch (e) {
        if (e.message !== 'Unauthorized') console.error('SQL wizard: error loading columns:', e);
    }

    const cols = _sqlWizardCols.filter(c => c.name !== '_row_id_');

    // Rebuild the SET field list.
    const fieldsDiv = document.getElementById('sql-wiz-update-fields');
    if (fieldsDiv) fieldsDiv.innerHTML = _sqlWizardUpdateFieldsHtml(cols);

    // Replace the WHERE list with a single pre-populated row using the
    // table's first unique column so there is always a starting WHERE condition.
    const whereList = document.getElementById('sql-wiz-where-list');
    if (whereList) {
        whereList.innerHTML = '';
        const uniqueCol = _sqlWizardFindUniqueCol();
        if (uniqueCol) {
            const row = document.createElement('div');
            row.className = 'sql-wiz-clause-row';
            row.innerHTML =
                '<select class="sql-wiz-where-col" onchange="buildSqlPreview()">'
                + _sqlWizardAllColOptions() + '</select>'
                + '<select class="sql-wiz-where-op" onchange="sqlWizardWhereOpChanged(this)">'
                + _SQL_WIZ_OP_OPTIONS + '</select>'
                + '<input type="text" class="sql-wiz-where-val" placeholder="value"'
                + ' oninput="buildSqlPreview()">'
                + '<button class="sql-wiz-remove-btn" onclick="sqlWizardRemoveClause(this)">'
                + '&#x2715;</button>';
            whereList.appendChild(row);
            row.querySelector('.sql-wiz-where-col').value = uniqueCol;
        }
    }

    buildSqlPreview();
}

// Return the HTML for the UPDATE SET field rows. Each row has an include
// checkbox, a column label with type hint, a text input, and a Null button.
// Rows start with the checkbox unchecked and both the input and Null button
// disabled so the user must explicitly opt each column in. The css class
// sql-wiz-row-excluded applies 50% opacity to visually dim excluded rows.
// When the checkbox is checked, sqlWizardUpdateToggleCol() enables the controls.
function _sqlWizardUpdateFieldsHtml(cols) {
    if (cols.length === 0) return '<p class="sql-wiz-unsupported">No columns available.</p>';
    return cols.map(c => {
        const nullable = c.nullable && c.nullable.value;
        const typeHint = escapeHtml(c.type || '');
        return '<div class="sql-wiz-update-row sql-wiz-row-excluded"'
            + ' data-col="' + escapeHtml(c.name)
            + '" data-type="' + typeHint + '">'
            + '<input type="checkbox" class="sql-wiz-update-include"'
            + ' onchange="sqlWizardUpdateToggleCol(this)">'
            + '<span class="sql-wiz-insert-label">'
            + '<span class="sql-wiz-insert-colname">' + escapeHtml(c.name) + '</span>'
            + '<span class="sql-wiz-insert-typehint">' + typeHint
            + (nullable ? '' : ' &bull;') + '</span>'
            + '</span>'
            + '<input type="text" class="sql-wiz-update-input" data-is-null="false"'
            + ' oninput="buildSqlPreview()" autocomplete="off" spellcheck="false"'
            + ' placeholder="" disabled>'
            + '<button class="sql-wiz-update-null-btn"'
            + ' onclick="sqlWizardUpdateNullToggle(this)" disabled>Null</button>'
            + '</div>';
    }).join('');
}

// Enable or disable a SET field when its include checkbox is toggled.
function sqlWizardUpdateToggleCol(cb) {
    const row     = cb.closest('.sql-wiz-update-row');
    const input   = row?.querySelector('.sql-wiz-update-input');
    const nullBtn = row?.querySelector('.sql-wiz-update-null-btn');
    if (!row) return;
    const included = cb.checked;
    row.classList.toggle('sql-wiz-row-excluded', !included);
    if (nullBtn) nullBtn.disabled = !included;
    if (input) {
        // Re-enable the input only when included AND not null-flagged.
        input.disabled = !included || input.dataset.isNull === 'true';
    }
    buildSqlPreview();
}

// Toggle the null state for an UPDATE SET field. An UPDATE row has three
// interaction states:
//   excluded  — checkbox unchecked; input and Null button both disabled
//   included  — checkbox checked; user can type a value
//   null      — Null button active; input disabled and shows null styling
// This function switches between "included" and "null". When un-nulling we
// must re-check whether the row is still included (checkbox still checked)
// before re-enabling the input, because the user might have unchecked the
// row while null was active.
function sqlWizardUpdateNullToggle(btn) {
    const row     = btn.closest('.sql-wiz-update-row');
    const input   = row?.querySelector('.sql-wiz-update-input');
    const include = row?.querySelector('.sql-wiz-update-include');
    if (!row || !input) return;
    const wasNull = input.dataset.isNull === 'true';
    if (wasNull) {
        input.dataset.isNull = 'false';
        // Only re-enable typing if the row's checkbox is still checked.
        input.disabled = !(include?.checked);
        input.classList.remove('sql-wiz-update-null');
        btn.classList.remove('sql-wiz-update-null-active');
    } else {
        input.dataset.isNull = 'true';
        input.disabled = true;
        input.classList.add('sql-wiz-update-null');
        btn.classList.add('sql-wiz-update-null-active');
    }
    buildSqlPreview();
}

// Append a new WHERE clause row for UPDATE (includes _row_id_ in the column picker).
function sqlWizardUpdateAddWhere() {
    const list = document.getElementById('sql-wiz-where-list');
    if (!list) return;
    const row = document.createElement('div');
    row.className = 'sql-wiz-clause-row';
    row.innerHTML =
        '<select class="sql-wiz-where-col" onchange="buildSqlPreview()">'
        + _sqlWizardAllColOptions() + '</select>'
        + '<select class="sql-wiz-where-op" onchange="sqlWizardWhereOpChanged(this)">'
        + _SQL_WIZ_OP_OPTIONS + '</select>'
        + '<input type="text" class="sql-wiz-where-val" placeholder="value"'
        + ' oninput="buildSqlPreview()">'
        + '<button class="sql-wiz-remove-btn" onclick="sqlWizardRemoveClause(this)">'
        + '&#x2715;</button>';
    list.appendChild(row);
    buildSqlPreview();
}

// Fetch column metadata for the selected table and rebuild the column checkbox
// grid plus the column pickers in any existing WHERE / ORDER BY rows.
// Clears existing clause rows when the table changes to avoid stale column names.
// The ?rowids=true query parameter asks the server to include the internal
// _row_id_ column and to annotate each column with unique/nullable metadata.
// _row_id_ is then stripped from the display list; it is not a user-visible column.
async function sqlWizardTableChanged() {
    const dsn   = document.getElementById('sql-dsn-picker').value;
    const table = document.getElementById('sql-wiz-table')?.value;
    if (!dsn || !table) return;

    _sqlWizardCols = [];
    try {
        const res  = await apiFetch(
            '/dsns/' + encodeURIComponent(dsn)
            + '/tables/' + encodeURIComponent(table) + '?rowids=true'
        );
        const data = await res.json();
        _sqlWizardCols = res.ok ? (data.columns || []) : [];
    } catch (e) {
        if (e.message !== 'Unauthorized') console.error('SQL wizard: error loading columns:', e);
    }

    // Exclude _row_id_ from the display column list (internal field).
    const cols = _sqlWizardCols.filter(c => c.name !== '_row_id_');

    // Rebuild column checkbox grid.
    const colList = document.getElementById('sql-wiz-col-list');
    if (colList) colList.innerHTML = _sqlWizardColGridHtml(cols);

    // Clear WHERE and ORDER BY rows so stale column names are not shown.
    const whereList = document.getElementById('sql-wiz-where-list');
    const orderList = document.getElementById('sql-wiz-order-list');
    if (whereList) whereList.innerHTML = '';
    if (orderList) orderList.innerHTML = '';

    buildSqlPreview();
}

// Return the HTML for the column checkbox grid used when select-all is off.
function _sqlWizardColGridHtml(cols) {
    if (cols.length === 0) return '<p class="sql-wiz-unsupported">No columns available.</p>';
    return '<div class="sql-wiz-col-grid">'
        + cols.map(c =>
            '<div class="sql-wiz-col-item">'
            + '<input type="checkbox" id="sql-wiz-col-cb-' + escapeHtml(c.name) + '" '
            + 'data-col="' + escapeHtml(c.name) + '" checked onchange="buildSqlPreview()">'
            + '<label for="sql-wiz-col-cb-' + escapeHtml(c.name) + '">'
            + escapeHtml(c.name) + '</label>'
            + '</div>'
        ).join('')
        + '</div>';
}

// Return a string of <option> elements for non-internal columns (excludes _row_id_).
// Used in SELECT column pickers and WHERE pickers for SELECT statements.
// SELECT should not expose _row_id_ because it is an internal implementation
// detail; see _sqlWizardAllColOptions() for UPDATE/DELETE WHERE pickers that
// do need it as a filter target.
function _sqlWizardColOptions() {
    const cols = _sqlWizardCols.filter(c => c.name !== '_row_id_');
    return cols.map(c =>
        '<option value="' + escapeHtml(c.name) + '">' + escapeHtml(c.name) + '</option>'
    ).join('');
}

// Return <option> elements for ALL columns including _row_id_. Used in WHERE
// pickers for UPDATE and DELETE so the internal row key can be used as a
// filter — e.g. "WHERE _row_id_ = 42" to target a single row precisely.
function _sqlWizardAllColOptions() {
    return _sqlWizardCols.map(c =>
        '<option value="' + escapeHtml(c.name) + '">' + escapeHtml(c.name) + '</option>'
    ).join('');
}

// Scan _sqlWizardCols for the first column marked as unique, preferring a
// named column over _row_id_. The server populates col.unique.specified=true
// and col.unique.value=true when the column has a unique constraint; both
// fields must be true to qualify. _row_id_ is kept as a last-resort fallback
// because it is always unique but is less meaningful to users than a named key.
// Returns null when no unique column exists at all.
function _sqlWizardFindUniqueCol() {
    let rowIdUnique = false;
    for (const col of _sqlWizardCols) {
        if (!(col.unique && col.unique.specified && col.unique.value)) continue;
        if (col.name === '_row_id_') { rowIdUnique = true; continue; }
        return col.name;
    }
    return rowIdUnique ? '_row_id_' : null;
}

// Collect complete WHERE clause parts from the shared #sql-wiz-where-list.
// Shared by the SELECT, UPDATE, and DELETE preview builders so the same
// WHERE UI element does not need to be re-implemented per statement type.
// Rows with an empty value field are skipped (an incomplete condition would
// produce invalid SQL). IS NULL / IS NOT NULL are emitted without a value
// because those operators don't take one.
// String values are single-quoted and internal single-quotes are doubled
// ('' is the SQL standard escape for a literal apostrophe) to prevent
// SQL injection in the generated preview text.
function _sqlWizardCollectWhereParts() {
    const parts = [];
    document.querySelectorAll('#sql-wiz-where-list .sql-wiz-clause-row').forEach(row => {
        const col = row.querySelector('.sql-wiz-where-col')?.value;
        const op  = row.querySelector('.sql-wiz-where-op')?.value;
        if (!col || !op) return;
        if (op === 'IS NULL' || op === 'IS NOT NULL') {
            parts.push(col + ' ' + op);
        } else {
            const val = (row.querySelector('.sql-wiz-where-val')?.value || '').trim();
            if (val === '') return;
            // Leave numeric literals unquoted; quote everything else.
            const isNum = /^-?\d+(\.\d+)?([eE][+-]?\d+)?$/.test(val);
            const quoted = isNum ? val : "'" + val.replace(/'/g, "''") + "'";
            parts.push(col + ' ' + op + ' ' + quoted);
        }
    });
    return parts;
}

// Show or hide the column checkbox grid when the "Select all" toggle changes.
function sqlWizardSelectAllChanged() {
    const checked = document.getElementById('sql-wiz-select-all')?.checked;
    const list    = document.getElementById('sql-wiz-col-list');
    if (list) list.style.display = checked ? 'none' : '';
    buildSqlPreview();
}

// Append a new WHERE clause row with column picker, operator picker, and value.
// Uses _sqlWizardColOptions() (excludes _row_id_) because SELECT WHERE clauses
// filter on user-visible columns. UPDATE and DELETE use their own AddWhere
// functions that call _sqlWizardAllColOptions() so _row_id_ is also available.
function sqlWizardAddWhere() {
    const list = document.getElementById('sql-wiz-where-list');
    if (!list) return;
    const row = document.createElement('div');
    row.className = 'sql-wiz-clause-row';
    row.innerHTML =
        '<select class="sql-wiz-where-col" onchange="buildSqlPreview()">'
        + _sqlWizardColOptions() + '</select>'
        + '<select class="sql-wiz-where-op" onchange="sqlWizardWhereOpChanged(this)">'
        + _SQL_WIZ_OP_OPTIONS + '</select>'
        + '<input type="text" class="sql-wiz-where-val" placeholder="value"'
        + ' oninput="buildSqlPreview()">'
        + '<button class="sql-wiz-remove-btn" onclick="sqlWizardRemoveClause(this)">'
        + '&#x2715;</button>';
    list.appendChild(row);
    buildSqlPreview();
}

// Append a new ORDER BY row with column picker and direction selector.
function sqlWizardAddOrder() {
    const list = document.getElementById('sql-wiz-order-list');
    if (!list) return;
    const row = document.createElement('div');
    row.className = 'sql-wiz-clause-row';
    row.innerHTML =
        '<select class="sql-wiz-order-col" onchange="buildSqlPreview()">'
        + _sqlWizardColOptions() + '</select>'
        + '<select class="sql-wiz-order-dir" onchange="buildSqlPreview()">'
        + '<option value="ASC">ASC</option>'
        + '<option value="DESC">DESC</option>'
        + '</select>'
        + '<button class="sql-wiz-remove-btn" onclick="sqlWizardRemoveClause(this)">'
        + '&#x2715;</button>';
    list.appendChild(row);
    buildSqlPreview();
}

// Remove the clause row that contains the clicked ✕ button.
// btn.closest('.sql-wiz-clause-row') walks up the DOM from the button until
// it finds an ancestor with that class — this works regardless of how many
// elements deep the button sits inside the row. Works for WHERE and ORDER BY
// rows because both use the same sql-wiz-clause-row class.
function sqlWizardRemoveClause(btn) {
    btn.closest('.sql-wiz-clause-row').remove();
    buildSqlPreview();
}

// Show or hide the value input when the operator changes. IS NULL and
// IS NOT NULL test for the absence of a value so no comparison value is
// needed — hiding the input prevents the user from entering one and
// avoids generating syntactically invalid SQL like "col IS NULL 'x'".
function sqlWizardWhereOpChanged(sel) {
    const noVal = sel.value === 'IS NULL' || sel.value === 'IS NOT NULL';
    const val   = sel.closest('.sql-wiz-clause-row').querySelector('.sql-wiz-where-val');
    if (val) val.style.display = noVal ? 'none' : '';
    buildSqlPreview();
}

// ==========================================================================
// DELETE wizard
// ==========================================================================

// Return the HTML body for the DELETE wizard (table picker + WHERE section only).
function _sqlWizardDeleteHtml(tables) {
    const opts = tables.map(t =>
        '<option value="' + escapeHtml(t) + '">' + escapeHtml(t) + '</option>'
    ).join('');
    return '<div class="sql-wiz-section">'
        + '<div class="sql-wiz-section-hdr"><span class="sql-wiz-label">Table</span></div>'
        + '<select id="sql-wiz-table" class="sql-wiz-select"'
        + ' onchange="sqlWizardDeleteTableChanged()">'
        + opts + '</select></div>'
        + '<div class="sql-wiz-section">'
        + '<div class="sql-wiz-section-hdr">'
        + '<span class="sql-wiz-label">WHERE'
        + ' <span class="sql-wiz-required">(required)</span></span>'
        + '<button class="sql-wiz-add-btn" onclick="sqlWizardDeleteAddWhere()">'
        + '+ Add condition</button>'
        + '</div>'
        + '<div id="sql-wiz-where-list"></div>'
        + '</div>';
}

// Fetch column metadata for the DELETE table picker and pre-populate the WHERE
// list with the table's first unique column (same logic as UPDATE).
async function sqlWizardDeleteTableChanged() {
    const dsn   = document.getElementById('sql-dsn-picker').value;
    const table = document.getElementById('sql-wiz-table')?.value;
    if (!dsn || !table) return;

    _sqlWizardCols = [];
    try {
        const res  = await apiFetch(
            '/dsns/' + encodeURIComponent(dsn)
            + '/tables/' + encodeURIComponent(table) + '?rowids=true'
        );
        const data = await res.json();
        _sqlWizardCols = res.ok ? (data.columns || []) : [];
    } catch (e) {
        if (e.message !== 'Unauthorized') console.error('SQL wizard: error loading columns:', e);
    }

    // Replace the WHERE list with a single pre-populated row using the
    // table's first unique column so there is always a starting WHERE condition.
    const whereList = document.getElementById('sql-wiz-where-list');
    if (whereList) {
        whereList.innerHTML = '';
        const uniqueCol = _sqlWizardFindUniqueCol();
        if (uniqueCol) {
            const row = document.createElement('div');
            row.className = 'sql-wiz-clause-row';
            row.innerHTML =
                '<select class="sql-wiz-where-col" onchange="buildSqlPreview()">'
                + _sqlWizardAllColOptions() + '</select>'
                + '<select class="sql-wiz-where-op" onchange="sqlWizardWhereOpChanged(this)">'
                + _SQL_WIZ_OP_OPTIONS + '</select>'
                + '<input type="text" class="sql-wiz-where-val" placeholder="value"'
                + ' oninput="buildSqlPreview()">'
                + '<button class="sql-wiz-remove-btn" onclick="sqlWizardRemoveClause(this)">'
                + '&#x2715;</button>';
            whereList.appendChild(row);
            row.querySelector('.sql-wiz-where-col').value = uniqueCol;
        }
    }

    buildSqlPreview();
}

// Append a new WHERE clause row for DELETE (includes _row_id_ in the column picker).
function sqlWizardDeleteAddWhere() {
    const list = document.getElementById('sql-wiz-where-list');
    if (!list) return;
    const row = document.createElement('div');
    row.className = 'sql-wiz-clause-row';
    row.innerHTML =
        '<select class="sql-wiz-where-col" onchange="buildSqlPreview()">'
        + _sqlWizardAllColOptions() + '</select>'
        + '<select class="sql-wiz-where-op" onchange="sqlWizardWhereOpChanged(this)">'
        + _SQL_WIZ_OP_OPTIONS + '</select>'
        + '<input type="text" class="sql-wiz-where-val" placeholder="value"'
        + ' oninput="buildSqlPreview()">'
        + '<button class="sql-wiz-remove-btn" onclick="sqlWizardRemoveClause(this)">'
        + '&#x2715;</button>';
    list.appendChild(row);
    buildSqlPreview();
}

// ==========================================================================
// CREATE TABLE wizard
// ==========================================================================

// SQL types offered in the column type picker. This is a practical subset of
// the full SQL_TYPES set — common enough that a novice user will recognize them.
const _SQL_CREATE_TYPES = [
    'VARCHAR', 'TEXT', 'CHAR',
    'INT', 'INTEGER', 'BIGINT', 'SMALLINT',
    'FLOAT', 'DOUBLE', 'DECIMAL', 'NUMERIC',
    'BOOLEAN',
    'DATE', 'DATETIME', 'TIMESTAMP',
    'UUID', 'JSON',
];

// Return the HTML body for the CREATE TABLE wizard.
// Unlike SELECT/INSERT/UPDATE/DELETE, CREATE does not start with a table
// picker — the user is naming a new table. _sqlWizardTables is checked at
// runtime by sqlWizardCreateNameChanged() to detect collisions, so it does
// not need to be baked into the HTML.
function _sqlWizardCreateHtml() {
    return '<div class="sql-wiz-section">'
        + '<div class="sql-wiz-section-hdr"><span class="sql-wiz-label">Table name</span></div>'
        + '<div class="sql-wiz-create-name-row">'
        + '<input type="text" id="sql-wiz-create-name" class="sql-wiz-create-name-input"'
        + ' placeholder="new_table_name" oninput="sqlWizardCreateNameChanged()"'
        + ' autocomplete="off" spellcheck="false">'
        + '<span id="sql-wiz-create-name-status" class="sql-wiz-create-name-status"></span>'
        + '</div>'
        + '</div>'
        + '<div class="sql-wiz-section">'
        + '<div class="sql-wiz-section-hdr">'
        + '<span class="sql-wiz-label">Columns'
        + ' <span class="sql-wiz-required">(at least one required)</span></span>'
        + '<button class="sql-wiz-add-btn" onclick="sqlWizardCreateAddColumn()">+ Add column</button>'
        + '</div>'
        + '<div class="sql-wiz-create-col-header">'
        + '<span class="sql-wiz-create-hdr-name">Name</span>'
        + '<span class="sql-wiz-create-hdr-type">Type</span>'
        + '<span class="sql-wiz-create-hdr-cb">Unique</span>'
        + '<span class="sql-wiz-create-hdr-cb">Nullable</span>'
        + '<span class="sql-wiz-create-hdr-del"></span>'
        + '</div>'
        + '<div id="sql-wiz-create-cols"></div>'
        + '</div>';
}

// Called on every keystroke in the table-name input.
// Validates the name and updates the inline status indicator.
// _sqlWizardTables was populated when the wizard opened, so this is a
// fast local check — no network round-trip needed.
function sqlWizardCreateNameChanged() {
    const input  = document.getElementById('sql-wiz-create-name');
    const status = document.getElementById('sql-wiz-create-name-status');
    const name   = (input?.value || '').trim();

    if (!name) {
        status.textContent = '';
        status.className   = 'sql-wiz-create-name-status';
    } else if (_sqlWizardTables.some(t => t.toLowerCase() === name.toLowerCase())) {
        status.textContent = '✘ A table named “' + name + '” already exists';
        status.className   = 'sql-wiz-create-name-status sql-wiz-create-name-error';
    } else {
        status.textContent = '✔ Name is available';
        status.className   = 'sql-wiz-create-name-status sql-wiz-create-name-ok';
    }

    buildSqlPreview();
}

// Append a new column definition row to #sql-wiz-create-cols.
// Each row has: name input, type select, Unique checkbox, Nullable checkbox,
// and a ✕ delete button. Nullable starts checked so columns are nullable by
// default (the common case); the user unchecks it to add NOT NULL.
function sqlWizardCreateAddColumn() {
    const list = document.getElementById('sql-wiz-create-cols');
    if (!list) return;

    const typeOpts = _SQL_CREATE_TYPES.map(t =>
        '<option value="' + t + '">' + t + '</option>'
    ).join('');

    const row = document.createElement('div');
    row.className = 'sql-wiz-create-col-row';
    row.innerHTML =
        '<input type="text" class="sql-wiz-create-col-name"'
        + ' placeholder="column_name" oninput="buildSqlPreview()"'
        + ' autocomplete="off" spellcheck="false">'
        + '<select class="sql-wiz-create-col-type" onchange="buildSqlPreview()">'
        + typeOpts + '</select>'
        + '<label class="sql-wiz-create-cb-label">'
        + '<input type="checkbox" class="sql-wiz-create-col-unique" onchange="buildSqlPreview()">Unique'
        + '</label>'
        + '<label class="sql-wiz-create-cb-label">'
        + '<input type="checkbox" class="sql-wiz-create-col-nullable" checked onchange="buildSqlPreview()">Nullable'
        + '</label>'
        + '<button class="sql-wiz-remove-btn" onclick="sqlWizardCreateRemoveColumn(this)">&#x2715;</button>';
    list.appendChild(row);
    // Focus the name input immediately so the user can start typing without
    // having to click into it.
    row.querySelector('.sql-wiz-create-col-name').focus();
    buildSqlPreview();
}

// Remove the column definition row containing the clicked ✕ button.
function sqlWizardCreateRemoveColumn(btn) {
    btn.closest('.sql-wiz-create-col-row').remove();
    buildSqlPreview();
}

// ==========================================================================
// ALTER TABLE wizard
// ==========================================================================

// Return the HTML body for the ALTER TABLE wizard: table picker, operation
// picker, and a placeholder for the op-specific sub-form.
function _sqlWizardAlterHtml(tables) {
    const opts = tables.map(t =>
        '<option value="' + escapeHtml(t) + '">' + escapeHtml(t) + '</option>'
    ).join('');
    return '<div class="sql-wiz-section">'
        + '<div class="sql-wiz-section-hdr"><span class="sql-wiz-label">Table</span></div>'
        + '<select id="sql-wiz-alter-table" class="sql-wiz-select"'
        + ' onchange="sqlWizardAlterTableChanged()">' + opts + '</select>'
        + '</div>'
        + '<div class="sql-wiz-section">'
        + '<div class="sql-wiz-section-hdr">'
        + '<span class="sql-wiz-label">Columns</span>'
        + '<div class="sql-wiz-alter-op-btns">'
        + '<button class="sql-wiz-alter-op-btn sql-wiz-alter-op-btn-active"'
        + ' data-op="ADD" onclick="sqlWizardAlterSelectOp(\'ADD\')">Add</button>'
        + '<button class="sql-wiz-alter-op-btn"'
        + ' data-op="DROP" onclick="sqlWizardAlterSelectOp(\'DROP\')">Drop</button>'
        + '<button class="sql-wiz-alter-op-btn"'
        + ' data-op="RENAME" onclick="sqlWizardAlterSelectOp(\'RENAME\')">Rename</button>'
        + '</div>'
        + '</div>'
        + '<input type="hidden" id="sql-wiz-alter-op" value="ADD">'
        + '</div>'
        + '<div id="sql-wiz-alter-op-body"></div>';
}

// Called when one of the Add / Drop / Rename buttons is clicked.
// Updates the hidden op value, moves the active style to the clicked button,
// then rebuilds the op-specific sub-form. sqlWizardAlterOpChanged() reads the
// hidden input, so no other changes are needed there.
function sqlWizardAlterSelectOp(op) {
    const hidden = document.getElementById('sql-wiz-alter-op');
    if (hidden) hidden.value = op;
    document.querySelectorAll('.sql-wiz-alter-op-btn').forEach(btn => {
        btn.classList.toggle('sql-wiz-alter-op-btn-active', btn.dataset.op === op);
    });
    sqlWizardAlterOpChanged();
}

// Called when the table picker changes — load columns then rebuild the op body.
async function sqlWizardAlterTableChanged() {
    const dsn   = document.getElementById('sql-dsn-picker').value;
    const table = document.getElementById('sql-wiz-alter-table')?.value;
    if (!dsn || !table) return;

    _sqlWizardCols = [];
    try {
        const res  = await apiFetch(
            '/dsns/' + encodeURIComponent(dsn)
            + '/tables/' + encodeURIComponent(table) + '?rowids=true'
        );
        const data = await res.json();
        _sqlWizardCols = res.ok ? (data.columns || []) : [];
    } catch (e) {
        if (e.message !== 'Unauthorized') console.error('SQL wizard: error loading columns:', e);
    }

    sqlWizardAlterOpChanged();
}

// Called when the operation picker changes — rebuild just the op-specific sub-form.
function sqlWizardAlterOpChanged() {
    const op       = document.getElementById('sql-wiz-alter-op')?.value;
    const body     = document.getElementById('sql-wiz-alter-op-body');
    const isPostgres = _sqlWizardIsPostgres();
    if (!body) return;

    const cols = _sqlWizardCols.filter(c => c.name !== '_row_id_');

    if (op === 'ADD') {
        body.innerHTML = _sqlWizardAlterAddHtml(isPostgres);
        sqlWizardAlterAddColumn(); // seed with one blank column row
    } else if (op === 'DROP') {
        body.innerHTML = _sqlWizardAlterDropHtml(cols, isPostgres);
    } else {
        body.innerHTML = _sqlWizardAlterRenameHtml(cols);
    }
    buildSqlPreview();
}

// Return HTML for the ADD COLUMN sub-form.
// Reuses the sql-wiz-create-* row structure so column rows look identical to
// CREATE TABLE. sqlWizardAlterAddColumn() appends rows to #sql-wiz-alter-cols.
// Postgres allows multiple ADD COLUMNs in one ALTER TABLE; SQLite does not, so
// the "+ Add another" button is hidden for SQLite.
function _sqlWizardAlterAddHtml(isPostgres) {
    const addBtn = isPostgres
        ? '<button class="sql-wiz-add-btn" onclick="sqlWizardAlterAddColumn()">+ Add another</button>'
        : '<span class="sql-wiz-hint">— SQLite supports one column per statement</span>';
    return '<div class="sql-wiz-section">'
        + '<div class="sql-wiz-section-hdr">'
        + '<span class="sql-wiz-label">Column to add</span>'
        + addBtn
        + '</div>'
        + '<div class="sql-wiz-create-col-header">'
        + '<span class="sql-wiz-create-hdr-name">Name</span>'
        + '<span class="sql-wiz-create-hdr-type">Type</span>'
        + '<span class="sql-wiz-create-hdr-cb">Unique</span>'
        + '<span class="sql-wiz-create-hdr-cb">Nullable</span>'
        + '<span class="sql-wiz-create-hdr-del"></span>'
        + '</div>'
        + '<div id="sql-wiz-alter-cols"></div>'
        + '</div>';
}

// Append a blank column definition row to #sql-wiz-alter-cols.
// Mirrors sqlWizardCreateAddColumn() but targets the ALTER sub-list so the two
// wizards don't share DOM state. For SQLite, only one row is ever added since
// SQLite does not support multiple ADD COLUMNs in one ALTER TABLE statement.
function sqlWizardAlterAddColumn() {
    const list = document.getElementById('sql-wiz-alter-cols');
    if (!list) return;
    if (!_sqlWizardIsPostgres() && list.children.length >= 1) return;

    const typeOpts = _SQL_CREATE_TYPES.map(t =>
        '<option value="' + t + '">' + t + '</option>'
    ).join('');

    const row = document.createElement('div');
    row.className = 'sql-wiz-create-col-row';
    row.innerHTML =
        '<input type="text" class="sql-wiz-create-col-name"'
        + ' placeholder="column_name" oninput="buildSqlPreview()"'
        + ' autocomplete="off" spellcheck="false">'
        + '<select class="sql-wiz-create-col-type" onchange="buildSqlPreview()">'
        + typeOpts + '</select>'
        + '<label class="sql-wiz-create-cb-label">'
        + '<input type="checkbox" class="sql-wiz-create-col-unique" onchange="buildSqlPreview()">Unique'
        + '</label>'
        + '<label class="sql-wiz-create-cb-label">'
        + '<input type="checkbox" class="sql-wiz-create-col-nullable" checked onchange="buildSqlPreview()">Nullable'
        + '</label>'
        + '<button class="sql-wiz-remove-btn" onclick="sqlWizardCreateRemoveColumn(this)">&#x2715;</button>';
    list.appendChild(row);
    row.querySelector('.sql-wiz-create-col-name').focus();
    buildSqlPreview();
}

// Return HTML for the DROP COLUMN sub-form.
// Postgres supports dropping multiple columns in one ALTER TABLE statement, so
// checkboxes are used. SQLite only supports one DROP COLUMN per statement, so
// radio buttons are used to enforce the single-column limit in the UI.
function _sqlWizardAlterDropHtml(cols, isPostgres) {
    if (cols.length === 0) return '<p class="sql-wiz-unsupported">No columns available.</p>';
    const inputType = isPostgres ? 'checkbox' : 'radio';
    const hint = isPostgres
        ? ' <span class="sql-wiz-hint">— check columns to remove</span>'
        : ' <span class="sql-wiz-hint">— SQLite supports one column per statement</span>';
    return '<div class="sql-wiz-section">'
        + '<div class="sql-wiz-section-hdr">'
        + '<span class="sql-wiz-label">Column to drop' + hint + '</span>'
        + '</div>'
        + '<div id="sql-wiz-alter-drop-list" class="sql-wiz-col-grid">'
        + cols.map(c =>
            '<div class="sql-wiz-col-item">'
            + '<input type="' + inputType + '" name="sql-wiz-alt-drop"'
            + ' id="sql-wiz-alt-drop-' + escapeHtml(c.name) + '"'
            + ' data-col="' + escapeHtml(c.name) + '" onchange="buildSqlPreview()">'
            + '<label for="sql-wiz-alt-drop-' + escapeHtml(c.name) + '">'
            + escapeHtml(c.name) + '</label>'
            + '</div>'
        ).join('')
        + '</div>'
        + '</div>';
}

// Return HTML for the RENAME COLUMN sub-form.
// One row per existing column: the old name as a fixed label, an arrow, and
// a text input for the new name. Rows with a blank new name are skipped in
// buildSqlPreview(), so the user only fills in the columns they want to rename.
function _sqlWizardAlterRenameHtml(cols) {
    if (cols.length === 0) return '<p class="sql-wiz-unsupported">No columns available.</p>';
    return '<div class="sql-wiz-section">'
        + '<div class="sql-wiz-section-hdr">'
        + '<span class="sql-wiz-label">Rename columns'
        + ' <span class="sql-wiz-hint">— leave blank to keep current name</span></span>'
        + '</div>'
        + '<div id="sql-wiz-alter-rename-list">'
        + cols.map(c =>
            '<div class="sql-wiz-rename-row" data-col="' + escapeHtml(c.name) + '">'
            + '<span class="sql-wiz-rename-old">' + escapeHtml(c.name) + '</span>'
            + '<span class="sql-wiz-rename-arrow">&#x2192;</span>'
            + '<input type="text" class="sql-wiz-rename-input"'
            + ' placeholder="new name" oninput="buildSqlPreview()"'
            + ' autocomplete="off" spellcheck="false">'
            + '</div>'
        ).join('')
        + '</div>'
        + '</div>';
}

// Assemble a SQL statement from the current wizard state and update the
// preview <pre>. Handles CREATE, ALTER, INSERT, UPDATE, DELETE, and SELECT. Also enables
// or disables the Insert button depending on whether the statement is valid.
//
// For UPDATE and DELETE, omitting the WHERE clause is allowed but triggers
// a warning: the Insert button gets a data-all-rows attribute set to the
// statement type ("UPDATE" or "DELETE"). insertSqlBuild() reads that
// attribute and shows a confirmation dialog before inserting the statement.
function buildSqlPreview() {
    const prev = document.getElementById('sql-build-preview');
    if (!prev) return;

    const type = document.getElementById('sql-build-type')?.value;

    // Clear any pending all-rows warning from a previous render.
    document.getElementById('sql-build-insert-btn')?.removeAttribute('data-all-rows');

    // ---- CREATE TABLE ----
    if (type === 'CREATE') {
        const insertBtn = document.getElementById('sql-build-insert-btn');
        const name = (document.getElementById('sql-wiz-create-name')?.value || '').trim();

        if (!name) {
            prev.textContent = '-- Enter a table name to begin';
            insertBtn.disabled = true;
            return;
        }
        // Block if the name collides with an existing table.
        if (_sqlWizardTables.some(t => t.toLowerCase() === name.toLowerCase())) {
            prev.textContent = '-- Table "' + name + '" already exists — choose a different name';
            insertBtn.disabled = true;
            return;
        }

        // Collect column definitions from each row in the column list.
        const colDefs = [];
        let hasBlankName = false;
        document.querySelectorAll('#sql-wiz-create-cols .sql-wiz-create-col-row').forEach(row => {
            const colName  = (row.querySelector('.sql-wiz-create-col-name')?.value || '').trim();
            const colType  = row.querySelector('.sql-wiz-create-col-type')?.value || 'VARCHAR';
            const unique   = row.querySelector('.sql-wiz-create-col-unique')?.checked;
            const nullable = row.querySelector('.sql-wiz-create-col-nullable')?.checked;

            if (!colName) { hasBlankName = true; return; }

            // Build the column clause: name, type, then optional constraints.
            // NOT NULL comes before UNIQUE to follow conventional SQL style.
            let def = colName + ' ' + colType;
            if (!nullable) def += ' NOT NULL';
            if (unique)    def += ' UNIQUE';
            colDefs.push(def);
        });

        if (hasBlankName) {
            prev.textContent = '-- Every column needs a name';
            insertBtn.disabled = true;
            return;
        }
        if (colDefs.length === 0) {
            prev.textContent = '-- Add at least one column to continue';
            insertBtn.disabled = true;
            return;
        }

        prev.textContent = 'CREATE TABLE ' + name + ' (\n'
            + colDefs.map(d => '    ' + d).join(',\n')
            + '\n)';
        insertBtn.disabled = false;
        return;
    }

    // ---- INSERT ----
    if (type === 'INSERT') {
        const table = document.getElementById('sql-wiz-table')?.value;
        if (!table) {
            prev.textContent = '-- Select a table to begin';
            document.getElementById('sql-build-insert-btn').disabled = true;
            return;
        }

        const colNames = [];
        const colValues = [];
        document.querySelectorAll('#sql-wiz-insert-fields .sql-wiz-insert-row').forEach(row => {
            const col    = row.dataset.col;
            const type   = row.dataset.type || '';
            const input  = row.querySelector('.sql-wiz-insert-input');
            const isNull = !input || input.dataset.isNull === 'true';

            colNames.push(col);
            if (isNull || (input && input.value.trim() === '')) {
                colValues.push('NULL');
            } else {
                const val = input.value.trim();
                // Numeric types are left unquoted; everything else is quoted.
                if (isDataIntType(type) || isDataFloatType(type)) {
                    colValues.push(val);
                } else {
                    colValues.push("'" + val.replace(/'/g, "''") + "'");
                }
            }
        });

        if (colNames.length === 0) {
            prev.textContent = '-- Loading column definitions…';
            document.getElementById('sql-build-insert-btn').disabled = true;
            return;
        }

        prev.textContent = 'INSERT INTO ' + table
            + '\n  (' + colNames.join(', ') + ')'
            + '\nVALUES'
            + '\n  (' + colValues.join(', ') + ')';
        document.getElementById('sql-build-insert-btn').disabled = false;
        return;
    }

    // ---- UPDATE ----
    if (type === 'UPDATE') {
        const table = document.getElementById('sql-wiz-table')?.value;
        if (!table) {
            prev.textContent = '-- Select a table to begin';
            document.getElementById('sql-build-insert-btn').disabled = true;
            return;
        }

        // Collect SET parts from checked rows.
        const setParts = [];
        document.querySelectorAll('#sql-wiz-update-fields .sql-wiz-update-row').forEach(row => {
            const cb = row.querySelector('.sql-wiz-update-include');
            if (!cb?.checked) return;
            const col     = row.dataset.col;
            const colType = row.dataset.type || '';
            const input   = row.querySelector('.sql-wiz-update-input');
            const isNull  = !input || input.dataset.isNull === 'true'
                            || input.value.trim() === '';
            if (isNull) {
                setParts.push(col + ' = NULL');
            } else {
                const val = input.value.trim();
                if (isDataIntType(colType) || isDataFloatType(colType)) {
                    setParts.push(col + ' = ' + val);
                } else {
                    setParts.push(col + " = '" + val.replace(/'/g, "''") + "'");
                }
            }
        });

        const whereParts = _sqlWizardCollectWhereParts();

        if (setParts.length === 0) {
            prev.textContent = '-- Check at least one column to update';
            document.getElementById('sql-build-insert-btn').disabled = true;
            return;
        }
        let sql = 'UPDATE ' + table + '\nSET ' + setParts[0];
        for (let i = 1; i < setParts.length; i++) sql += ',\n    ' + setParts[i];
        if (whereParts.length > 0) {
            sql += '\nWHERE ' + whereParts[0];
            for (let i = 1; i < whereParts.length; i++) sql += '\n  AND ' + whereParts[i];
        } else {
            // No WHERE — flag the button so insertSqlBuild() can warn the user.
            document.getElementById('sql-build-insert-btn').setAttribute('data-all-rows', 'UPDATE');
        }

        prev.textContent = sql;
        document.getElementById('sql-build-insert-btn').disabled = false;
        return;
    }

    // ---- DELETE ----
    if (type === 'DELETE') {
        const table = document.getElementById('sql-wiz-table')?.value;
        if (!table) {
            prev.textContent = '-- Select a table to begin';
            document.getElementById('sql-build-insert-btn').disabled = true;
            return;
        }

        const whereParts = _sqlWizardCollectWhereParts();
        let sql = 'DELETE FROM ' + table;
        if (whereParts.length > 0) {
            sql += '\nWHERE ' + whereParts[0];
            for (let i = 1; i < whereParts.length; i++) sql += '\n  AND ' + whereParts[i];
        } else {
            // No WHERE — flag the button so insertSqlBuild() can warn the user.
            document.getElementById('sql-build-insert-btn').setAttribute('data-all-rows', 'DELETE');
        }

        prev.textContent = sql;
        document.getElementById('sql-build-insert-btn').disabled = false;
        return;
    }

    // ---- ALTER TABLE ----
    if (type === 'ALTER') {
        const insertBtn = document.getElementById('sql-build-insert-btn');
        const table = document.getElementById('sql-wiz-alter-table')?.value;
        const op    = document.getElementById('sql-wiz-alter-op')?.value;

        if (!table) {
            prev.textContent = '-- Select a table to begin';
            insertBtn.disabled = true;
            return;
        }

        const isPostgres = _sqlWizardIsPostgres();

        if (op === 'ADD') {
            const colDefs = [];
            let hasBlankName = false;
            document.querySelectorAll('#sql-wiz-alter-cols .sql-wiz-create-col-row').forEach(row => {
                const colName  = (row.querySelector('.sql-wiz-create-col-name')?.value || '').trim();
                const colType  = row.querySelector('.sql-wiz-create-col-type')?.value || 'VARCHAR';
                const unique   = row.querySelector('.sql-wiz-create-col-unique')?.checked;
                const nullable = row.querySelector('.sql-wiz-create-col-nullable')?.checked;
                if (!colName) { hasBlankName = true; return; }
                let def = colName + ' ' + colType;
                if (!nullable) def += ' NOT NULL';
                if (unique)    def += ' UNIQUE';
                colDefs.push(def);
            });
            if (hasBlankName) {
                prev.textContent = '-- Every column needs a name';
                insertBtn.disabled = true;
                return;
            }
            if (colDefs.length === 0) {
                prev.textContent = '-- Add at least one column to continue';
                insertBtn.disabled = true;
                return;
            }
            // Postgres: combine into one ALTER TABLE with comma-separated ADD COLUMNs.
            // SQLite: only one column is ever present (enforced by the UI).
            if (isPostgres && colDefs.length > 1) {
                prev.textContent = 'ALTER TABLE ' + table + '\n'
                    + colDefs.map(d => '  ADD COLUMN ' + d).join(',\n');
            } else {
                prev.textContent = colDefs
                    .map(d => 'ALTER TABLE ' + table + ' ADD COLUMN ' + d)
                    .join('\n');
            }
            insertBtn.disabled = false;
            return;
        }

        if (op === 'DROP') {
            const toDrop = Array.from(
                document.querySelectorAll('#sql-wiz-alter-drop-list input:checked')
            ).map(cb => cb.dataset.col);
            if (toDrop.length === 0) {
                prev.textContent = '-- Select at least one column to drop';
                insertBtn.disabled = true;
                return;
            }
            // Postgres: combine into one ALTER TABLE with comma-separated DROP COLUMNs.
            // SQLite: radio buttons ensure only one is ever selected.
            if (isPostgres && toDrop.length > 1) {
                prev.textContent = 'ALTER TABLE ' + table + '\n'
                    + toDrop.map(c => '  DROP COLUMN ' + c).join(',\n');
            } else {
                prev.textContent = 'ALTER TABLE ' + table + ' DROP COLUMN ' + toDrop[0];
            }
            insertBtn.disabled = false;
            return;
        }

        if (op === 'RENAME') {
            const renames = [];
            document.querySelectorAll('#sql-wiz-alter-rename-list .sql-wiz-rename-row').forEach(row => {
                const oldName = row.dataset.col;
                const newName = (row.querySelector('.sql-wiz-rename-input')?.value || '').trim();
                if (newName) renames.push({ old: oldName, new: newName });
            });
            if (renames.length === 0) {
                prev.textContent = '-- Enter at least one new column name to continue';
                insertBtn.disabled = true;
                return;
            }
            prev.textContent = renames
                .map(r => 'ALTER TABLE ' + table + ' RENAME COLUMN ' + r.old + ' TO ' + r.new)
                .join('\n');
            insertBtn.disabled = false;
            return;
        }

        prev.textContent = '-- Select an operation to continue';
        insertBtn.disabled = true;
        return;
    }

    // ---- unsupported types ----
    if (type !== 'SELECT') {
        prev.textContent = '-- ' + (type || 'statement') + ' not yet supported by the wizard';
        document.getElementById('sql-build-insert-btn').disabled = true;
        return;
    }

    // ---- SELECT ----
    const table = document.getElementById('sql-wiz-table')?.value;
    if (!table) {
        prev.textContent = '-- Select a table to begin';
        document.getElementById('sql-build-insert-btn').disabled = true;
        return;
    }

    // Column list — either * or a comma-separated list of checked columns.
    const selectAll = document.getElementById('sql-wiz-select-all')?.checked;
    let colClause = '*';
    if (!selectAll) {
        const picked = Array.from(
            document.querySelectorAll('#sql-wiz-col-list input[type="checkbox"]:checked')
        ).map(cb => cb.dataset.col);
        colClause = picked.length > 0 ? picked.join(', ') : '*';
    }

    let sql = 'SELECT ' + colClause + '\nFROM ' + table;

    // WHERE — use the shared collector.
    const whereParts = _sqlWizardCollectWhereParts();
    if (whereParts.length > 0) {
        sql += '\nWHERE ' + whereParts[0];
        for (let i = 1; i < whereParts.length; i++) sql += '\n  AND ' + whereParts[i];
    }

    // ORDER BY — collect column + direction pairs.
    const orderParts = [];
    document.querySelectorAll('#sql-wiz-order-list .sql-wiz-clause-row').forEach(row => {
        const col = row.querySelector('.sql-wiz-order-col')?.value;
        const dir = row.querySelector('.sql-wiz-order-dir')?.value || 'ASC';
        if (col) orderParts.push(col + ' ' + dir);
    });
    if (orderParts.length > 0) sql += '\nORDER BY ' + orderParts.join(', ');

    prev.textContent = sql;
    document.getElementById('sql-build-insert-btn').disabled = false;
}

// Insert the generated SQL into the editor at the current cursor position,
// then close the wizard. A newline separator is prepended when needed so the
// new statement starts on its own line.
// The data-all-rows attribute is set by buildSqlPreview() when an UPDATE or
// DELETE has no WHERE clause. Its presence signals that the user needs to
// confirm before the potentially destructive statement is inserted.
function insertSqlBuild() {
    const preview = document.getElementById('sql-build-preview')?.textContent || '';
    // Preview text starting with "--" means the wizard is still incomplete.
    if (!preview || preview.startsWith('--')) return;

    const insertBtn = document.getElementById('sql-build-insert-btn');
    const allRows   = insertBtn?.getAttribute('data-all-rows');
    if (allRows) {
        const verb = allRows === 'DELETE' ? 'delete' : 'update';
        const ok = confirm(
            'WARNING: This statement has no WHERE clause and will '
            + verb + ' ALL RECORDS in the table.\n\n'
            + 'Are you sure you want to continue?'
        );
        if (!ok) return;
    }

    const editor = document.getElementById('sql-editor');
    // If the wizard was opened from a text selection, replace that selection;
    // otherwise insert at the current cursor position.
    const range  = _sqlWizardSelectionRange;
    const start  = range ? range.start : editor.selectionStart;
    const end    = range ? range.end   : editor.selectionEnd;
    const before = editor.value.substring(0, start);
    const after  = editor.value.substring(end);

    // If there is existing content before the cursor and it does not end with
    // a newline, add one so the SQL statement starts on its own line.
    const sep  = (before.length > 0 && !before.endsWith('\n')) ? '\n' : '';
    // Always end the inserted statement with ";\n" so the preprocessor
    // treats it as a complete statement on its own line.
    const tail = (/;\s*$/.test(preview) ? '' : ';') + '\n';
    // Prepend a visible warning comment when the user confirmed a no-WHERE statement.
    const warn = allRows ? '// WARNING: this statement affects all rows\n' : '';
    const inserted = sep + warn + preview + tail;
    editor.value = before + inserted + after;
    editor.selectionStart = editor.selectionEnd = start + inserted.length;
    editor.focus();
    updateSqlHighlight();
    hideSqlBuild();
}

// An object that maps each tab's string id to the function that loads its
// data. This lets openTab() call the right loader with a single line
// (tabLoaders[tabId]()) instead of a chain of if/else statements.
const tabLoaders = {
    memory:  loadMemory,
    users:   loadUsers,
    dsns:    loadDsns,
    tables:  loadTables,
    data:    loadData,
    sql:     loadSql,
    log:     loadLog,
    code:    loadCode,
};

// ==========================================================================
// Tab switching
// ==========================================================================

// Tracks which tab is currently visible so we can reload it after the user
// logs in. Declared with "let" so it can be reassigned.
let activeTab = 'memory';

// Show the tab identified by tabId, hide all others, and load its data.
// Called from onclick attributes in the HTML, e.g.:
//   <div onclick="openTab('memory')">Memory</div>
function openTab(tabId) {
    activeTab = tabId;
    setCookie(COOKIE_ACTIVE_TAB, tabId);

    // getElementsByClassName returns a live HTMLCollection (like an array)
    // of every element that has the class "tab-content". We loop through
    // them all and hide each one by setting its CSS display property to 'none'.
    var tabContents = document.getElementsByClassName('tab-content');
    for (var i = 0; i < tabContents.length; i++) {
        tabContents[i].style.display = 'none';
    }

    // querySelectorAll('.tab-container > div') selects every direct <div>
    // child of the element with class "tab-container" — i.e. the tab buttons.
    // We remove 'active-tab' from all of them so only the new one gets it.
    var tabs = document.querySelectorAll('.tab-container > div');
    for (var i = 0; i < tabs.length; i++) {
        tabs[i].classList.remove('active-tab');
        tabs[i].classList.add('inactive-tab');
    }

    // Make the selected tab's content visible.
    // The code tab is a flex column, so it needs display:'flex' rather than 'block'.
    // Some tabs need display:flex rather than display:block to support internal scrolling.
    const flexTabs = new Set(['code', 'log', 'data', 'sql']);
    document.getElementById(tabId).style.display = flexTabs.has(tabId) ? 'flex' : 'block';

    // Find the tab button whose onclick attribute matches tabId and highlight it.
    // querySelector() returns the first element in the document that matches
    // the CSS selector string — here we're searching by attribute value.
    document.querySelector('[onclick="openTab(\'' + tabId + '\')"]').classList.add('active-tab');

    // Invoke the loader function for this tab (defined in tabLoaders above).
    tabLoaders[tabId]();
}

// ==========================================================================
// Login overlay
// ==========================================================================

// Display the login overlay, optionally showing an error message (e.g.
// "Session expired"). Clears the form fields so previous input isn't visible.
// The overlay uses CSS display:flex to center the login card on screen.
function showLogin(message) {
    deleteCookie(COOKIE_ACTIVE_TAB);   // next login always starts at the default tab

    // Restore all tab buttons to visible. The next login will re-apply the
    // correct visibility for whoever logs in, which may be a different user
    // with different permissions than the previous session.
    ADMIN_ONLY_TABS.forEach(tabId => {
        const btn = document.querySelector('.tab-container .' + tabId);
        if (btn) btn.style.display = '';
    });

    document.getElementById('login-error').textContent = message || '';
    document.getElementById('login-username').value = '';
    document.getElementById('login-password').value = '';
    document.getElementById('login-overlay').style.display = 'flex';
    document.getElementById('login-username').focus(); // put the cursor in the username field
}

// Hide the login overlay by resetting its display property to 'none'.
function hideLogin() {
    document.getElementById('login-overlay').style.display = 'none';
}

// Send the username and password to the server. On success, store the
// returned token in memory and reload the active tab. On failure, show
// an error message inside the login form.
async function submitLogin() {
    const username = document.getElementById('login-username').value.trim(); // trim() removes leading/trailing spaces
    const password = document.getElementById('login-password').value;

    // Validate locally before making a network call.
    if (!username || !password) {
        document.getElementById('login-error').textContent = 'Please enter a username and password.';
        return; // stop here — don't submit an incomplete form
    }

    // Disable the button while the request is in flight so the user can't
    // click it multiple times and send duplicate requests.
    document.getElementById('login-btn').disabled = true;

    // Clear any existing token before sending the login request so that
    // no Authorization header is attached to the logon call.
    clearToken();

    try {
        // POST the credentials as a JSON body. The "source" field identifies
        // this request as coming from the dashboard so the server can log it.
        const res = await fetch('/services/admin/logon', {
            method:  'POST',
            headers: { 'Content-Type': 'application/json' },
            body:    JSON.stringify({ username, password, source: 'Dashboard' }),
        });

        const data = await res.json();

        // res.ok is true for 2xx status codes. If the server returned an
        // error status, or if the response JSON has no "token" field,
        // show the server's message (or a generic fallback).
        if (!res.ok || !data.token) {
            document.getElementById('login-error').textContent =
                data.message || 'Login failed. Please try again.';
            return;
        }

        // Check whether this user has any permission to use the dashboard.
        // data.admin is true for full admin access; data.coder is true for
        // Code-tab-only access. If neither flag is set, refuse to log in.
        if (!data.admin && !data.coder) {
            document.getElementById('login-error').textContent =
                'This account does not have permission to access the dashboard.';
            return;
        }

        // Success — store the token and role, then reset the inactivity clock.
        setToken(data.token);
        setRole(data.admin, data.coder, data.identity);
        lastActivity = Date.now();
        hideLogin();

        // Show or hide tab buttons to match this user's role.
        applyTabVisibility();

        // After a password login, offer to create a passkey if the browser
        // supports WebAuthn and the user hasn't previously declined.
        maybeOfferPasskeyAfterLogin();

        // Open the last active tab for admins; coders always land on Code.
        openTab(_isAdmin ? activeTab : 'code');

    } catch (e) {
        // fetch() itself throws only for network-level failures (no connection,
        // DNS failure, etc.) — HTTP error statuses do NOT throw.
        document.getElementById('login-error').textContent = 'Network error. Please try again.';
    } finally {
        // "finally" runs whether the try block succeeded or threw an error,
        // so the button is always re-enabled when the request completes.
        document.getElementById('login-btn').disabled = false;
    }
}

// addEventListener attaches a function to run when a specific event occurs
// on an element. Here we listen for 'keydown' on the two text inputs so the
// user can press Enter to submit instead of clicking the Sign In button.
document.getElementById('login-password').addEventListener('keydown', e => {
    if (e.key === 'Enter') submitLogin(); // e.key is the name of the key that was pressed
});
document.getElementById('login-username').addEventListener('keydown', e => {
    if (e.key === 'Enter') submitLogin();
});
// Wire the Sign In button's click event to the same submit function.
document.getElementById('login-btn').addEventListener('click', submitLogin);

// ==========================================================================
// New User sheet
// ==========================================================================

// Open the slide-in panel used to create a new user account.
// Resets all fields and error text each time so stale data from a previous
// attempt isn't shown.
function showNewUserSheet() {
    document.getElementById('new-user-error').textContent = '';
    document.getElementById('new-user-name').value = '';
    document.getElementById('new-user-password').value = '';
    document.getElementById('new-user-permissions').value = '';
    document.getElementById('new-user-overlay').style.display = 'flex';
    captureBaseline('new-user-overlay');
    document.getElementById('new-user-name').focus();
}

// Close the slide-in panel without saving.
function hideNewUserSheet() {
    document.getElementById('new-user-overlay').style.display = 'none';
}

// Read the form fields, validate them, and POST the new user to the server.
async function submitNewUser() {
    const name     = document.getElementById('new-user-name').value.trim();
    const password = document.getElementById('new-user-password').value;
    const permsRaw = document.getElementById('new-user-permissions').value;

    // The permissions field accepts a comma-separated list like "ego.logon, ego.admin".
    // split(',') breaks it into an array, map(trim) removes spaces around each item,
    // and filter(length > 0) drops any empty strings left by trailing commas.
    const permissions = permsRaw.split(',').map(p => p.trim()).filter(p => p.length > 0);

    if (!name || !password) {
        document.getElementById('new-user-error').textContent = 'Username and password are required.';
        return;
    }

    document.getElementById('new-user-save-btn').disabled = true;

    try {
        const token = getToken();
        const res = await fetch('/admin/users', {
            method:  'POST',
            headers: {
                'Content-Type':  'application/json',
                'Authorization': token ? 'Bearer ' + token : '',
            },
            // The server expects a specific JSON shape. The "id" field uses a
            // nil UUID (all zeros) to signal that the server should assign a
            // real UUID to the new user record.
            body: JSON.stringify({
                name,
                id:          '00000000-0000-0000-0000-000000000000',
                password,
                permissions,
            }),
        });

        if (!res.ok) {
            // Auth failure — discard the token and prompt for login.
            if (res.status === 401 || res.status === 403) {
                clearToken();
                hideNewUserSheet();
                showLogin('Session expired. Please sign in again.');
                return;
            }
            // Other server error — read the "msg" field from the response body
            // and display it. .catch(() => ({})) provides an empty object as a
            // fallback if the response body isn't valid JSON.
            const data = await res.json().catch(() => ({}));
            document.getElementById('new-user-error').textContent =
                data.msg || 'Failed to create user (HTTP ' + res.status + ').';
            return;
        }

        // Success — close the sheet and refresh the user list to show the new entry.
        hideNewUserSheet();
        loadUsers();
    } catch (e) {
        document.getElementById('new-user-error').textContent = 'Network error. Please try again.';
    } finally {
        document.getElementById('new-user-save-btn').disabled = false;
    }
}

// ==========================================================================
// New DSN sheet
// ==========================================================================

// Open the slide-in panel used to create a new DSN.
// Resets all fields and error text each time so stale data from a previous
// attempt isn't shown.
function showNewDsnSheet() {
    document.getElementById('new-dsn-error').textContent    = '';
    document.getElementById('new-dsn-name').value           = '';
    document.getElementById('new-dsn-provider').value       = 'postgres';
    document.getElementById('new-dsn-host').value           = '';
    document.getElementById('new-dsn-port').value           = '';
    document.getElementById('new-dsn-database').value       = '';
    document.getElementById('new-dsn-schema').value         = '';
    document.getElementById('new-dsn-user').value           = '';
    document.getElementById('new-dsn-secured').checked      = false;
    document.getElementById('new-dsn-restricted').checked   = false;
    document.getElementById('new-dsn-rowid').checked        = true;
    document.getElementById('new-dsn-overlay').style.display = 'flex';
    captureBaseline('new-dsn-overlay');
}

// Close the slide-in panel without saving.
function hideNewDsnSheet() {
    document.getElementById('new-dsn-overlay').style.display = 'none';
}

// Read the form fields, validate them, and POST the new DSN to the server.
async function submitNewDsn() {
    const name       = document.getElementById('new-dsn-name').value.trim();
    const provider   = document.getElementById('new-dsn-provider').value;
    const host       = document.getElementById('new-dsn-host').value.trim();
    const portRaw    = document.getElementById('new-dsn-port').value.trim();
    const database   = document.getElementById('new-dsn-database').value.trim();
    const schema     = document.getElementById('new-dsn-schema').value.trim();
    const user       = document.getElementById('new-dsn-user').value.trim();
    const secured    = document.getElementById('new-dsn-secured').checked;
    const restricted = document.getElementById('new-dsn-restricted').checked;
    const rowid      = document.getElementById('new-dsn-rowid').checked;

    if (!name) {
        document.getElementById('new-dsn-error').textContent = 'Name is required.';
        return;
    }

    let port = 0;
    if (portRaw !== '') {
        port = parseInt(portRaw, 10);
        if (!Number.isInteger(port) || port <= 0 || String(port) !== portRaw) {
            document.getElementById('new-dsn-error').textContent = 'Port must be a positive integer.';
            return;
        }
    }

    document.getElementById('new-dsn-save-btn').disabled = true;

    try {
        const token = getToken();
        const res = await fetch('/dsns', {
            method:  'POST',
            headers: {
                'Content-Type':  'application/json',
                'Authorization': token ? 'Bearer ' + token : '',
            },
            body: JSON.stringify({ name, provider, database, schema, host, port, user, secured, restricted, rowid }),
        });

        if (!res.ok) {
            if (res.status === 401 || res.status === 403) {
                clearToken();
                hideNewDsnSheet();
                showLogin('Session expired. Please sign in again.');
                return;
            }
            const data = await res.json().catch(() => ({}));
            document.getElementById('new-dsn-error').textContent =
                data.msg || 'Failed to create DSN (HTTP ' + res.status + ').';
            return;
        }

        hideNewDsnSheet();
        loadDsns();
    } catch (e) {
        document.getElementById('new-dsn-error').textContent = 'Network error. Please try again.';
    } finally {
        document.getElementById('new-dsn-save-btn').disabled = false;
    }
}

// ==========================================================================
// Edit User sheet
// ==========================================================================

// Open the slide-in edit panel, pre-populated with the user's current values.
// name and perms come from the data attributes set on the table row.
function showEditUserSheet(name, perms, passkeys, lastToken) {
    document.getElementById('edit-user-error').textContent = '';
    document.getElementById('edit-user-name').value        = name;
    document.getElementById('edit-user-password').value   = '';
    document.getElementById('edit-user-permissions').value = perms;
    const passkeyCount = document.getElementById('edit-user-passkey-count');
    if (passkeyCount) passkeyCount.textContent = passkeys != null ? passkeys : '0';
    const lastLoginEl = document.getElementById('edit-user-last-login');
    if (lastLoginEl) {
        lastLoginEl.value = lastToken && !lastToken.startsWith('0001')
            ? new Date(lastToken).toLocaleString()
            : '—';
    }

    // The passkey registration button is only meaningful when editing your own
    // account — WebAuthn requires the device owner to be present for biometric
    // verification, so an admin cannot register a passkey on behalf of another
    // user.  Also hide it when the browser does not support WebAuthn.
    const ownAccount = _currentUser && name.toLowerCase() === _currentUser.toLowerCase();
    const passkeyBtn = document.getElementById('edit-user-passkey-btn');
    if (passkeyBtn) {
        passkeyBtn.style.display = (passkeysActive() && ownAccount && window.PublicKeyCredential) ? '' : 'none';
    }

    // The clear-passkey button is available to admins (for any user) and to
    // the account owner for their own account.
    const clearPasskeyBtn = document.getElementById('edit-user-clear-passkey-btn');
    if (clearPasskeyBtn) {
        clearPasskeyBtn.style.display = (passkeysActive() && (_isAdmin || ownAccount)) ? '' : 'none';
    }

    document.getElementById('edit-user-overlay').style.display = 'flex';
    captureBaseline('edit-user-overlay');
    document.getElementById('edit-user-permissions').focus();
}

// Close the edit sheet without saving.
function hideEditUserSheet() {
    document.getElementById('edit-user-overlay').style.display = 'none';
}

// Read the edit form fields and PATCH the updated user to the server.
async function submitEditUser() {
    const name     = document.getElementById('edit-user-name').value;
    const password = document.getElementById('edit-user-password').value;
    const permsRaw = document.getElementById('edit-user-permissions').value;

    // Split the permissions string back into an array, trimming whitespace and
    // dropping any empty entries left by trailing commas.
    const permissions = permsRaw.split(',').map(p => p.trim()).filter(p => p.length > 0);

    // Build the PATCH body. The server ignores a blank password (no change).
    // We always send permissions so the server replaces the current list.
    const body = { name, permissions };
    if (password) body.password = password;

    document.getElementById('edit-user-save-btn').disabled = true;

    try {
        const token = getToken();
        const res = await fetch('/admin/users/' + encodeURIComponent(name), {
            method:  'PATCH',
            headers: {
                'Content-Type':  'application/json',
                'Authorization': token ? 'Bearer ' + token : '',
            },
            body: JSON.stringify(body),
        });

        if (res.status === 401 || res.status === 403) {
            clearToken();
            hideEditUserSheet();
            showLogin('Session expired. Please sign in again.');
            return;
        }

        if (!res.ok) {
            const data = await res.json().catch(() => ({}));
            document.getElementById('edit-user-error').textContent =
                data.msg || 'Failed to update user (HTTP ' + res.status + ').';
            return;
        }

        // Success — close the sheet and refresh the list to show the updated record.
        hideEditUserSheet();
        loadUsers();
    } catch (e) {
        document.getElementById('edit-user-error').textContent = 'Network error. Please try again.';
    } finally {
        document.getElementById('edit-user-save-btn').disabled = false;
    }
}

// Send DELETE /admin/users/{name} and close the sheet on success.
async function submitDeleteUser() {
    const name = document.getElementById('edit-user-name').value;

    if (!confirm('Delete user "' + name + '"? This cannot be undone.')) return;

    document.getElementById('edit-user-delete-btn').disabled = true;

    try {
        const token = getToken();
        const res = await fetch('/admin/users/' + encodeURIComponent(name), {
            method:  'DELETE',
            headers: token ? { 'Authorization': 'Bearer ' + token } : {},
        });

        if (res.status === 401 || res.status === 403) {
            clearToken();
            hideEditUserSheet();
            showLogin('Session expired. Please sign in again.');
            return;
        }

        if (!res.ok) {
            const data = await res.json().catch(() => ({}));
            document.getElementById('edit-user-error').textContent =
                data.msg || 'Failed to delete user (HTTP ' + res.status + ').';
            return;
        }

        hideEditUserSheet();
        loadUsers();
    } catch (e) {
        document.getElementById('edit-user-error').textContent = 'Network error. Please try again.';
    } finally {
        document.getElementById('edit-user-delete-btn').disabled = false;
    }
}

// ==========================================================================
// Server info header
//
// Fetches /services/up (no authentication required) and fills in the
// header bar with the server's name, UUID, and start time. This runs
// immediately on page load so the header is populated before the user
// even logs in.
// ==========================================================================
async function loadServerInfo() {
    try {
        const res = await fetch('/services/up'); // no apiFetch — this endpoint is public
        if (!res.ok) return; // silently skip if the server can't be reached
        const d = await res.json();

        // textContent sets the visible text of an element without interpreting
        // HTML — safer than innerHTML for server-supplied strings.
        _serverStartTime = d.since;
        document.getElementById('server-name').textContent    = d.server.name;
        document.getElementById('server-version').textContent = d.version ? 'v' + d.version : '';
        document.getElementById('server-id').textContent      = d.server.id;
        document.getElementById('server-since').textContent   = 'Up since ' + d.since;
    } catch (e) {
        console.error('Could not load server info:', e);
    }
}

// ==========================================================================
// Logoff
// ==========================================================================

// ==========================================================================
// Hamburger menu
// ==========================================================================

// Toggle the dropdown open/closed.
function toggleHamburgerMenu() {
    const dropdown = document.getElementById('hamburger-dropdown');
    const btn      = document.getElementById('hamburger-btn');
    const isOpen   = dropdown.classList.contains('open');
    dropdown.classList.toggle('open', !isOpen);
    btn.setAttribute('aria-expanded', String(!isOpen));
}

// Close the dropdown.
function closeHamburgerMenu() {
    document.getElementById('hamburger-dropdown').classList.remove('open');
    document.getElementById('hamburger-btn').setAttribute('aria-expanded', 'false');
}

// Close the dropdown when the user clicks anywhere outside the menu.
document.addEventListener('click', e => {
    const menu = document.getElementById('hamburger-menu');
    if (menu && !menu.contains(e.target)) {
        closeHamburgerMenu();
    }
});


// ==========================================================================
// Help display
// ==========================================================================

function showHelp() {
    window.open('https://tucats.github.io/ego/DASHBOARD.html', '_blank');
}

// ==========================================================================
// Settings sheet
// ==========================================================================

// Open the settings sheet and sync all toggles to their stored preferences.
function showSettings() {
    document.getElementById('setting-remember-login').checked = getRememberLogin();
    document.getElementById('setting-dark-mode').checked      = getDarkMode();
    document.getElementById('setting-use-passkeys').checked   = getUsePasskeys();
    document.getElementById('settings-overlay').style.display = 'flex';
}

// Close the settings sheet.
function hideSettings() {
    document.getElementById('settings-overlay').style.display = 'none';
}

// Wire up both settings toggles once the DOM is ready.
document.addEventListener('DOMContentLoaded', () => {
    // "Remember login" — persist token as a cookie
    document.getElementById('setting-remember-login').addEventListener('change', function () {
        setRememberLogin(this.checked);
        if (this.checked && _token) {
            // User just enabled the setting while already logged in —
            // write the current token to the cookie right away.
            setCookie(COOKIE_TOKEN, _token, TOKEN_MAX_AGE);
        } else if (!this.checked) {
            // Turning it off — remove any persisted token cookie immediately.
            deleteCookie(COOKIE_TOKEN);
        }
    });

    // "Dark mode" — toggle body.dark class and persist the choice
    document.getElementById('setting-dark-mode').addEventListener('change', function () {
        setDarkMode(this.checked);
    });

    // "Use passkeys" — re-applies passkey UI immediately so the login button
    // appears or disappears without needing a page reload.
    document.getElementById('setting-use-passkeys').addEventListener('change', function () {
        setUsePasskeys(this.checked);
    });
});

// ==========================================================================
// Logoff
// ==========================================================================

// Clear the token (memory + cookie) and show the login overlay.
// Called from the hamburger menu's "Log Out" item.
function logoff() {
    clearToken();                  // erases both _token and the persisted cookie
    codeSessionUUID = null;        // invalidate the server-side symbol table UUID
    showLogin();
}

// ==========================================================================
// Log tab — fetch and display the last 500 server log lines
//
// The endpoint is GET /services/admin/log?tail=500.  When the Accept header
// is text/plain the server returns raw newline-delimited log text, which we
// display verbatim inside a <pre> block.  The Refresh button and switching
// to this tab both call loadLog() so the view is always up to date.
// ==========================================================================

// Raw log text from the last fetch. Kept so search can re-highlight without
// making a new network request.
let logRawText = '';

// Search state: the array of all <mark> elements rendered in the current
// search, and the index of the currently highlighted one.
let logMatches     = [];
let logMatchIndex  = -1;

// Fetch the last N log lines (N comes from getLogTail(), default 500) and
// render them into #log-content.
async function loadLog() {
    const container = document.getElementById('log-content');

    container.innerHTML = '<p style="padding:0.5rem;color:#666;">Loading\u2026</p>';

    // Clear any leftover search state from a previous load.
    logRawText    = '';
    logMatches    = [];
    logMatchIndex = -1;
    document.getElementById('log-search-status').textContent = '';

    try {
        const token = getToken();

        const res = await fetch('/services/admin/log?tail=' + getLogTail(), {
            headers: {
                'Accept':        'text/plain',
                'Authorization': token ? 'Bearer ' + token : '',
            },
        });

        if (res.status === 401 || res.status === 403) {
            clearToken();
            showLogin('Session expired. Please sign in again.');
            return;
        }

        if (!res.ok) {
            container.innerHTML = '<p style="padding:0.5rem;color:#c0392b;">Failed to load log (HTTP ' + res.status + ').</p>';
            return;
        }

        logRawText = await res.text();
        logRenderPlain();

        // Scroll to the bottom so the most recent lines are visible.
        container.scrollTop = container.scrollHeight;

    } catch (e) {
        if (e.message !== 'Unauthorized') {
            container.innerHTML = '<p style="padding:0.5rem;color:#c0392b;">Network error: ' + escapeHtml(e.message) + '</p>';
        }
    }
}

// Render the raw log text as plain content, with no search highlights.
function logRenderPlain() {
    const container = document.getElementById('log-content');
    const pre = document.createElement('pre');
    pre.textContent = logRawText;
    container.innerHTML = '';
    container.appendChild(pre);
}

// Scroll the log content area to the bottom.
function logScrollToEnd() {
    const container = document.getElementById('log-content');
    container.scrollTop = container.scrollHeight;
}

// Build the highlighted HTML for the current search term and populate the
// match list. Called by logSearch() and reused by Prev/Next.
function logApplySearch(term) {
    const container = document.getElementById('log-content');
    const status    = document.getElementById('log-search-status');

    if (!term) {
        logRenderPlain();
        logMatches    = [];
        logMatchIndex = -1;
        status.textContent = '';
        return;
    }

    // Escape any regex special characters in the search term so a literal
    // string search is performed (e.g. "a.b" matches "a.b", not "axb").
    const escaped = term.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
    const re = new RegExp(escaped, 'gi'); // gi = global + case-insensitive

    // Walk the raw text and replace every match with a <mark> tag.
    // escapeHtml is applied to the non-matching segments so the surrounding
    // text is safe to inject as innerHTML.
    let html         = '';
    let lastIndex    = 0;
    let matchCount   = 0;
    const matchData  = []; // [{start, end}] for each match in the raw text

    let m;
    while ((m = re.exec(logRawText)) !== null) {
        // Escape the text between the previous match end and this match start.
        html += escapeHtml(logRawText.slice(lastIndex, m.index));
        // Wrap the matched text in a <mark>. Preserve original casing from source.
        html += '<mark>' + escapeHtml(m[0]) + '</mark>';
        matchData.push({ start: m.index, end: re.lastIndex });
        lastIndex = re.lastIndex;
        matchCount++;
    }
    // Escape any remaining text after the last match.
    html += escapeHtml(logRawText.slice(lastIndex));

    if (matchCount === 0) {
        logRenderPlain();
        logMatches    = [];
        logMatchIndex = -1;
        status.textContent = 'No matches';
        return;
    }

    // Inject the highlighted HTML into a <pre> block.
    const pre = document.createElement('pre');
    pre.innerHTML = html;
    container.innerHTML = '';
    container.appendChild(pre);

    // querySelectorAll returns a NodeList, not a plain Array. Array.from()
    // converts it so we can use array indexing and .length in Prev/Next.
    logMatches    = Array.from(container.querySelectorAll('mark'));
    logMatchIndex = 0;

    logHighlightCurrent();
    status.textContent = '1 / ' + matchCount + ' matches';
}

// Mark the current match as the active one (orange) and scroll it into view.
function logHighlightCurrent() {
    logMatches.forEach(m => m.classList.remove('log-match-current'));

    if (logMatches.length === 0) return;

    const current = logMatches[logMatchIndex];
    current.classList.add('log-match-current');
    // scrollIntoView centers the match vertically in the scroll container.
    current.scrollIntoView({ block: 'center' });

    document.getElementById('log-search-status').textContent =
        (logMatchIndex + 1) + ' / ' + logMatches.length + ' matches';
}

// Run a new search from the input field.
function logSearch() {
    const term = document.getElementById('log-search-input').value.trim();
    logApplySearch(term);
}

// Jump to the next match, wrapping around at the end.
function logSearchNext() {
    if (logMatches.length === 0) return;
    // The % operator is "modulo" — it gives the remainder after division.
    // Dividing by the list length makes the index wrap back to 0 after the
    // last match, so the search cycles continuously.
    logMatchIndex = (logMatchIndex + 1) % logMatches.length;
    logHighlightCurrent();
}

// Jump to the previous match, wrapping around at the start.
function logSearchPrev() {
    if (logMatches.length === 0) return;
    // Adding logMatches.length before the modulo prevents a negative result
    // when logMatchIndex is 0: (0 - 1) = -1, but (-1 % N) stays negative in
    // JavaScript, so we add N first to guarantee a positive number.
    logMatchIndex = (logMatchIndex - 1 + logMatches.length) % logMatches.length;
    logHighlightCurrent();
}

// Clear the search: restore plain text and reset state.
function logSearchClear() {
    document.getElementById('log-search-input').value = '';
    logRenderPlain();
    logMatches    = [];
    logMatchIndex = -1;
    document.getElementById('log-search-status').textContent = '';
}

// Allow the user to press Enter in the search box to trigger a search,
// and Escape to clear it — without needing to click a button.
document.getElementById('log-search-input').addEventListener('keydown', e => {
    if (e.key === 'Enter')  { e.preventDefault(); logSearch(); }
    if (e.key === 'Escape') { e.preventDefault(); logSearchClear(); }
});

// ==========================================================================
// Logger configuration sheet
//
// "Configure..." in the Log tab fetches GET /admin/loggers to learn the
// current on/off state of every named logger plus the "keep" count.  The
// sheet renders a toggle switch for each logger.  The Save button becomes
// enabled as soon as any value diverges from the original, and on click it
// POSTs only the changed loggers (plus the keep value) to /admin/loggers.
// ==========================================================================

// Snapshot of values when the sheet was opened, used to detect changes.
let loggerOriginalState = { keep: 0, loggers: {} };

// Fetch current logger state and open the config sheet.
async function showLoggerConfig() {
    document.getElementById('logger-config-error').textContent = '';
    document.getElementById('logger-save-btn').disabled = true;
    document.getElementById('logger-toggles').innerHTML = '<p style="color:#666;font-size:0.85rem;">Loading\u2026</p>';
    document.getElementById('logger-config-overlay').style.display = 'flex';

    try {
        const res  = await apiFetch('/admin/loggers');
        const data = await res.json();

        // Save original state for change detection.
        // Object.assign({}, data.loggers) makes a shallow copy of the loggers
        // object into a new, empty {}. Without the copy, loggerOriginalState.loggers
        // would point to the same object in memory as data.loggers — any later
        // change to data.loggers would silently overwrite the "original", breaking
        // the change detection in updateLoggerSaveBtn().
        loggerOriginalState = { keep: data.keep, loggers: Object.assign({}, data.loggers), tail: getLogTail() };

        document.getElementById('logger-file').textContent = data.file || '';
        document.getElementById('logger-keep').value = data.keep;
        document.getElementById('logger-tail').value = getLogTail();

        // Build a toggle row for each logger, sorted alphabetically.
        const togglesDiv = document.getElementById('logger-toggles');
        togglesDiv.innerHTML = '';

        const names = Object.keys(data.loggers).sort();
        for (const name of names) {
            const enabled = data.loggers[name];

            const row = document.createElement('div');
            row.className = 'logger-toggle-row';

            const labelEl = document.createElement('span');
            labelEl.className = 'logger-toggle-label';
            labelEl.textContent = name;

            // <label class="toggle-switch"><input type="checkbox"><span class="toggle-slider"></span></label>
            const switchLabel = document.createElement('label');
            switchLabel.className = 'toggle-switch';

            const input = document.createElement('input');
            input.type    = 'checkbox';
            input.checked = enabled;
            input.dataset.logger = name;
            input.addEventListener('change', updateLoggerSaveBtn);

            const slider = document.createElement('span');
            slider.className = 'toggle-slider';

            switchLabel.appendChild(input);
            switchLabel.appendChild(slider);
            row.appendChild(labelEl);
            row.appendChild(switchLabel);
            togglesDiv.appendChild(row);
        }

        // Watch both numeric fields for changes (replace any previous listeners).
        document.getElementById('logger-keep').oninput = updateLoggerSaveBtn;
        document.getElementById('logger-tail').oninput = updateLoggerSaveBtn;

        updateLoggerSaveBtn();
        captureBaseline('logger-config-overlay');
    } catch (e) {
        if (e.message !== 'Unauthorized') {
            document.getElementById('logger-config-error').textContent = 'Failed to load logger configuration.';
            document.getElementById('logger-toggles').innerHTML = '';
        }
    }
}

// Close the sheet without saving.
function hideLoggerConfig() {
    document.getElementById('logger-config-overlay').style.display = 'none';
}

// Enable the Save button only when something has actually changed and all
// fields are valid.
function updateLoggerSaveBtn() {
    const keepVal    = parseInt(document.getElementById('logger-keep').value, 10) || 0;
    const keepChanged = keepVal !== loggerOriginalState.keep;

    const tailVal   = parseInt(document.getElementById('logger-tail').value, 10);
    const tailValid  = tailVal > 0;
    const tailChanged = tailVal !== loggerOriginalState.tail;

    let loggerChanged = false;
    document.querySelectorAll('#logger-toggles input[type=checkbox]').forEach(input => {
        if (input.checked !== loggerOriginalState.loggers[input.dataset.logger]) {
            loggerChanged = true;
        }
    });

    document.getElementById('logger-save-btn').disabled =
        !(keepChanged || tailChanged || loggerChanged) || !tailValid;
}

// Save logger configuration. If only the "Lines to fetch" value changed,
// update the cookie and close without calling the API.
async function submitLoggerConfig() {
    const keepVal       = parseInt(document.getElementById('logger-keep').value, 10) || 0;
    const tailVal       = parseInt(document.getElementById('logger-tail').value, 10);
    const changedLoggers = {};

    document.querySelectorAll('#logger-toggles input[type=checkbox]').forEach(input => {
        const name = input.dataset.logger;
        if (input.checked !== loggerOriginalState.loggers[name]) {
            changedLoggers[name] = input.checked;
        }
    });

    const keepChanged   = keepVal !== loggerOriginalState.keep;
    const loggerChanged = Object.keys(changedLoggers).length > 0;

    // If the only change is the local "lines to fetch" preference, skip the
    // API call — just persist the cookie and close.
    if (!keepChanged && !loggerChanged) {
        setCookie(COOKIE_LOG_TAIL, String(tailVal));
        hideLoggerConfig();
        return;
    }

    document.getElementById('logger-save-btn').disabled = true;

    try {
        const token = getToken();
        const res = await fetch('/admin/loggers', {
            method:  'POST',
            headers: {
                'Content-Type':  'application/json',
                'Authorization': token ? 'Bearer ' + token : '',
            },
            body: JSON.stringify({ keep: keepVal, loggers: changedLoggers }),
        });

        if (res.status === 401 || res.status === 403) {
            clearToken();
            hideLoggerConfig();
            showLogin('Session expired. Please sign in again.');
            return;
        }

        if (!res.ok) {
            const data = await res.json().catch(() => ({}));
            document.getElementById('logger-config-error').textContent =
                data.msg || 'Failed to save logger configuration (HTTP ' + res.status + ').';
            document.getElementById('logger-save-btn').disabled = false;
            return;
        }

        setCookie(COOKIE_LOG_TAIL, String(tailVal));
        hideLoggerConfig();
    } catch (e) {
        document.getElementById('logger-config-error').textContent = 'Network error. Please try again.';
        document.getElementById('logger-save-btn').disabled = false;
    }
}

// ==========================================================================
// Code tab — editor, syntax highlighting, run, and console
//
// The Code tab embeds a full Ego editor inside the dashboard. It mirrors
// the standalone webapp's app.js logic, but uses POST /admin/run (with the
// bearer token) instead of the webapp's unauthenticated POST /run endpoint.
// ==========================================================================

// Guard so the editor is only wired up once, no matter how many times the
// user clicks the Code tab.
let codeTabInitialized = false;

// UUID that identifies this browser session's symbol table on the server.
// Generated once the first time the Code tab is opened and sent with every
// /admin/run request so each dashboard user gets isolated state.
// Cleared on logoff so it cannot be reused after the session ends.
let codeSessionUUID = null;

// fmtEgo applies lightweight Go-style formatting to the given source string:
//   • Re-indents every line with tabs according to the nesting depth tracked
//     by leading `}` / `)` characters (which dedent before printing) and
//     trailing `{` characters (which indent the next line).
//   • Collapses two or more consecutive blank lines into a single blank line.
//   • Removes trailing whitespace from every line.
//
// The formatter does not parse the full language; it is a best-effort pass
// that covers the patterns produced by everyday Ego/Go code.  It is called
// automatically each time the editor's Run button is pressed.
function fmtEgo(src) {
    // Go source files are indented with real tab characters, matching the Go formatter.
    const TAB = '\t';

    // findLineComment returns the index of the first `//` that is not inside
    // a string or rune literal, or -1 if there is none.  This lets us strip
    // comments before deciding whether a line ends with `{`.
    function findLineComment(line) {
        let i = 0;
        while (i < line.length) {
            const ch = line[i];
            // Double-quoted string — skip until the closing quote.
            if (ch === '"') {
                i++;
                while (i < line.length && line[i] !== '"') {
                    if (line[i] === '\\') i++; // escaped character — skip two
                    i++;
                }
                i++;
                continue;
            }
            // Backtick raw string — skip until the closing backtick.
            if (ch === '`') {
                i++;
                while (i < line.length && line[i] !== '`') i++;
                i++;
                continue;
            }
            // Rune literal — skip the (possibly escaped) character.
            if (ch === "'") {
                i++;
                if (i < line.length && line[i] === '\\') i += 2; else i++;
                if (i < line.length && line[i] === "'") i++;
                continue;
            }
            // Found `//` outside a string — this is a line comment.
            if (ch === '/' && line[i + 1] === '/') return i;
            i++;
        }
        return -1; // no line comment on this line
    }

    // leadingClosers counts the run of `}` and `)` characters at the very
    // start of a trimmed line.  Each one represents one level of dedent.
    function leadingClosers(line) {
        let i = 0;
        while (i < line.length && (line[i] === '}' || line[i] === ')')) i++;
        return i;
    }

    // endsWithOpener returns true when the logical content of the line
    // (after stripping any trailing line comment and whitespace) ends with `{`
    // or `(`.  Both characters open a new indented block — `{` for function and
    // control-flow bodies, `(` for multi-line import/const/var/type groups.
    function endsWithOpener(line) {
        const ci = findLineComment(line);
        const s = (ci >= 0 ? line.slice(0, ci) : line).trimEnd();
        if (s.length === 0) return false;
        const last = s[s.length - 1];
        return last === '{' || last === '(';
    }

    const lines = src.split('\n');
    const out = [];
    let depth = 0;
    let prevWasBlank = false;

    for (const rawLine of lines) {
        // Always strip trailing whitespace regardless of indentation changes.
        const line    = rawLine.trimEnd();
        const trimmed = line.trimStart();

        // Blank line — emit at most one in a row, and never as the first line.
        if (trimmed === '') {
            if (!prevWasBlank && out.length > 0) out.push('');
            prevWasBlank = true;
            continue;
        }
        prevWasBlank = false;

        // Leading `}` / `)` — each one closes a block, so dedent before printing
        // so the closing delimiter lines up with the opening statement.
        const closers = leadingClosers(trimmed);
        depth = Math.max(0, depth - closers);

        out.push(TAB.repeat(depth) + trimmed);

        // Trailing `{` — opens a new block, so the next line is one level deeper.
        if (endsWithOpener(trimmed)) depth++;
    }

    // Strip any trailing blank lines that were at the end of the source.
    while (out.length > 0 && out[out.length - 1] === '') out.pop();

    return out.join('\n');
}

// loadCode is called by openTab every time the Code tab is selected.
// On the first call it generates the session UUID, initializes all the DOM
// wiring, and stores the guard; subsequent calls are no-ops so the editor
// state (text, history) is preserved between tabs.
function loadCode() {
    if (codeTabInitialized) return;
    codeTabInitialized = true;
    codeSessionUUID = crypto.randomUUID();
    initCodeEditor();
}

// initCodeEditor wires up all event listeners and state for the embedded
// code editor. It runs exactly once, the first time the Code tab is opened.
function initCodeEditor() {
    // -----------------------------------------------------------------------
    // DOM references
    // -----------------------------------------------------------------------
    const codeEditor       = document.getElementById('code-editor');
    const codeLineNumbers  = document.getElementById('code-line-numbers');
    const codeOutput       = document.getElementById('code-output-pane');
    const codeRunBtn       = document.getElementById('code-run-btn');
    const codeRunArrow     = document.getElementById('code-run-arrow');
    const codeRunDrop      = document.getElementById('code-run-dropdown');
    const codeSpinner      = document.getElementById('code-spinner');
    const codeHlLayer      = document.getElementById('code-highlight-layer');
    const codeDebugBand    = document.getElementById('code-debug-line-band');
    const codeDivider      = document.getElementById('code-divider');
    const codeLeftPane     = document.getElementById('code-left-pane');
    const codeMain         = document.getElementById('code-main');
    const codeConsoleDivider     = document.getElementById('code-console-divider');
    const codeConsolePane        = document.getElementById('code-console-pane');
    const codeConsoleHistory     = document.getElementById('code-console-history');
    const codeConsoleInput       = document.getElementById('code-console-input');
    const codeConsoleToggleBtn   = document.getElementById('code-console-toggle');
    const codeTraceToggleBtn     = document.getElementById('code-trace-toggle');
    const codeTraceIndicator     = document.getElementById('code-trace-toggle-indicator');
    const codeUI                 = document.getElementById('code-ui');
    const codeClearEditorBtn  = document.getElementById('code-clear-editor-btn');
    const codeClearOutputBtn  = document.getElementById('code-clear-output-btn');
    const codeElapsed         = document.getElementById('code-elapsed');
    const codeClearConsoleBtn = document.getElementById('code-clear-console-btn');
    const codeOpenFileBtn     = document.getElementById('code-open-file-btn');
    // codeFileInput is a hidden <input type="file"> element.  We trigger it
    // programmatically from the Open button so the button can be styled to
    // match the rest of the editor toolbar.
    const codeFileInput       = document.getElementById('code-file-input');

    // Debugger panel elements (hidden until a debug session is active).
    const codeDebuggerPanel  = document.getElementById('code-debugger-panel');
    const codeDebugOutput    = document.getElementById('code-debug-output');
    const codeDebugInputRow  = document.getElementById('code-debug-input-row');
    const codeClearDebugBtn  = document.getElementById('code-clear-debug-btn');
    const codeDebugPrompt    = document.getElementById('code-debug-prompt');
    const codeDebugInput     = document.getElementById('code-debug-input');
    const codeDebugSendBtn   = document.getElementById('code-debug-send-btn');

    // Debugger control buttons (Continue / Step / Step Into / Step Over).
    const codeDebugContinueBtn  = document.getElementById('code-debug-continue-btn');
    const codeDebugStepBtn      = document.getElementById('code-debug-step-btn');
    const codeDebugStepReturnBtn  = document.getElementById('code-debug-step-return-btn');
    const codeDebugStepOverBtn  = document.getElementById('code-debug-step-over-btn');

    // -----------------------------------------------------------------------
    // Run / Debug split button
    //
    // codeRunMode tracks the current execution mode. The ▾ arrow opens a
    // dropdown with two items:
    //   "▶ Run"    — normal execution
    //   "🐛 Debug" — execution under the interactive debugger
    // Selecting an item updates the main button label and immediately runs.
    // -----------------------------------------------------------------------
    let codeRunMode = 'run'; // 'run' | 'debug'

    // 1-based line number the debugger is currently paused on; 0 = no highlight.
    let codeDebugLine = 0;

    // Trace toggle — when on, payload.trace = true is sent with each run.
    let codeTraceEnabled = false;

    function applyTraceToggle(enabled) {
        codeTraceEnabled = enabled;
        codeTraceToggleBtn.classList.toggle('active', enabled);
        codeTraceIndicator.textContent = enabled ? '\u25A0' : '\u25A3';
    }

    codeTraceToggleBtn.addEventListener('click', () => {
        applyTraceToggle(!codeTraceEnabled);
    });

    // Toggle the dropdown when the ▾ arrow is clicked.
    codeRunArrow.addEventListener('click', e => {
        e.stopPropagation(); // prevent the document click handler from closing it immediately
        codeRunDrop.classList.toggle('open');
    });

    // Close the dropdown when the user clicks anywhere outside it.
    document.addEventListener('click', () => codeRunDrop.classList.remove('open'));

    // Map from data-mode value to button label.
    const modeLabelMap = { run: '\u25B6 Run', debug: '\u{1F41B} Debug' };

    // Wire each dropdown item: set mode, update label, and run.
    codeRunDrop.querySelectorAll('.code-run-item').forEach(item => {
        item.addEventListener('click', e => {
            e.stopPropagation();
            codeRunDrop.classList.remove('open');

            codeRunMode = item.dataset.mode || 'run';

            // Reflect the selected mode in the main button label.
            codeRunBtn.textContent = modeLabelMap[codeRunMode] || '\u25B6 Run';

            // Mark the active item in the dropdown.
            codeRunDrop.querySelectorAll('.code-run-item').forEach(i => i.classList.remove('active'));
            item.classList.add('active');

            runEditorCode();
        });
    });

    // -----------------------------------------------------------------------
    // Syntax highlighting
    //
    // Reuses the same keyword sets and tokenizer used by the standalone webapp.
    // highlight(code) returns an HTML string with colored <span> elements.
    // -----------------------------------------------------------------------
    const CODE_KEYWORDS = new Set([
        'break','case','chan','const','continue','default','defer','else',
        'fallthrough','for','func','go','goto','if','import','interface',
        'map','package','range','return','select','struct','switch','type','var',
    ]);

    const CODE_BUILTINS = new Set([
        'bool','byte','complex64','complex128','error','float32','float64',
        'int','int8','int16','int32','int64','rune','string',
        'uint','uint8','uint16','uint32','uint64','uintptr',
        'true','false','nil','iota',
        'make','len','cap','new','append','copy','delete','close',
        'panic','recover','print','println',
    ]);

    function highlight(code) {
        function esc(s) {
            return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
        }
        function span(cls, s) {
            return '<span class="hl-' + cls + '">' + esc(s) + '</span>';
        }

        let out = '';
        let i   = 0;
        const n = code.length;

        while (i < n) {
            const ch  = code[i];
            const ch2 = code[i + 1];

            // Block comment  /* ... */
            if (ch === '/' && ch2 === '*') {
                const end = code.indexOf('*/', i + 2);
                if (end === -1) { out += span('comment', code.slice(i)); break; }
                out += span('comment', code.slice(i, end + 2));
                i = end + 2;
                continue;
            }

            // Line comment  // ...
            if (ch === '/' && ch2 === '/') {
                const nl  = code.indexOf('\n', i);
                const end = nl === -1 ? n : nl;
                out += span('comment', code.slice(i, end));
                i = end;
                continue;
            }

            // Double-quoted string  "..."
            if (ch === '"') {
                let j = i + 1;
                while (j < n && code[j] !== '"' && code[j] !== '\n') {
                    if (code[j] === '\\') j++;
                    j++;
                }
                if (j < n && code[j] === '"') j++;
                out += span('string', code.slice(i, j));
                i = j;
                continue;
            }

            // Raw (backtick) string  `...`
            if (ch === '`') {
                let j = i + 1;
                while (j < n && code[j] !== '`') j++;
                if (j < n) j++;
                out += span('string', code.slice(i, j));
                i = j;
                continue;
            }

            // Rune literal  '.'
            if (ch === "'") {
                let j = i + 1;
                if (j < n && code[j] === '\\') j += 2; else j++;
                if (j < n && code[j] === "'") j++;
                out += span('string', code.slice(i, j));
                i = j;
                continue;
            }

            // Numeric literal
            if (/[0-9]/.test(ch) || (ch === '.' && /[0-9]/.test(ch2))) {
                let j = i;
                if (ch === '0' && (ch2 === 'x' || ch2 === 'X')) {
                    j += 2;
                    while (j < n && /[0-9a-fA-F_]/.test(code[j])) j++;
                } else {
                    while (j < n && /[0-9_]/.test(code[j])) j++;
                    if (j < n && code[j] === '.') {
                        j++;
                        while (j < n && /[0-9_]/.test(code[j])) j++;
                    }
                    if (j < n && (code[j] === 'e' || code[j] === 'E')) {
                        j++;
                        if (j < n && (code[j] === '+' || code[j] === '-')) j++;
                        while (j < n && /[0-9]/.test(code[j])) j++;
                    }
                }
                out += span('number', code.slice(i, j));
                i = j;
                continue;
            }

            // Identifier, keyword, builtin, or function call
            if (/[a-zA-Z_]/.test(ch)) {
                let j = i;
                while (j < n && /[a-zA-Z0-9_]/.test(code[j])) j++;
                const word = code.slice(i, j);
                let k = j;
                while (k < n && (code[k] === ' ' || code[k] === '\t')) k++;
                if (CODE_KEYWORDS.has(word)) {
                    out += span('keyword', word);
                } else if (CODE_BUILTINS.has(word)) {
                    out += span('builtin', word);
                } else if (code[k] === '(') {
                    out += span('func', word);
                } else {
                    out += esc(word);
                }
                i = j;
                continue;
            }

            out += esc(ch);
            i++;
        }

        return out + ' ';
    }

    // Position (or hide) the debug line band.  Must be called whenever
    // codeDebugLine changes or the editor scrolls.
    function updateDebugBand() {
        if (codeDebugLine <= 0) {
            codeDebugBand.style.display = 'none';
            return;
        }
        const style      = window.getComputedStyle(codeHlLayer);
        const lineHeight = parseFloat(style.lineHeight);
        const paddingTop = parseFloat(style.paddingTop);
        const top        = paddingTop + (codeDebugLine - 1) * lineHeight - codeEditor.scrollTop;
        codeDebugBand.style.top     = top + 'px';
        codeDebugBand.style.height  = lineHeight + 'px';
        codeDebugBand.style.display = 'block';
    }

    // Scroll the editor so the given 1-based line is visible, centering it if
    // it is currently outside the viewport.
    function scrollToDebugLine(lineNum) {
        if (lineNum <= 0) return;
        const style      = window.getComputedStyle(codeHlLayer);
        const lineHeight = parseFloat(style.lineHeight);
        const paddingTop = parseFloat(style.paddingTop);
        const lineTop    = paddingTop + (lineNum - 1) * lineHeight;
        const lineBot    = lineTop + lineHeight;
        const visTop     = codeEditor.scrollTop;
        const visBot     = visTop + codeEditor.clientHeight;
        if (lineTop < visTop || lineBot > visBot) {
            codeEditor.scrollTop      = Math.max(0, lineTop - codeEditor.clientHeight / 2);
            codeLineNumbers.scrollTop = codeEditor.scrollTop;
        }
    }

    // Rebuild the syntax-highlight layer from the current editor text.
    function updateHighlight() {
        codeHlLayer.innerHTML = highlight(codeEditor.value);
        codeHlLayer.scrollTop  = codeEditor.scrollTop;
        codeHlLayer.scrollLeft = codeEditor.scrollLeft;
    }

    // Rebuild the line-number gutter from the current editor text.
    function updateLineNumbers() {
        const count = codeEditor.value.split('\n').length;
        let text = '';
        for (let i = 1; i <= count; i++) text += i + '\n';
        codeLineNumbers.textContent = text;
        codeLineNumbers.scrollTop = codeEditor.scrollTop;
    }

    // -----------------------------------------------------------------------
    // Editor event listeners
    // -----------------------------------------------------------------------

    // Sync line numbers and highlighting as the user types.
    codeEditor.addEventListener('input', () => {
        updateLineNumbers();
        updateHighlight();
    });

    // Sync scroll position of gutter and highlight layer with the textarea.
    codeEditor.addEventListener('scroll', () => {
        codeLineNumbers.scrollTop = codeEditor.scrollTop;
        codeHlLayer.scrollTop     = codeEditor.scrollTop;
        codeHlLayer.scrollLeft    = codeEditor.scrollLeft;
        updateDebugBand();
    });

    // Tab key — insert three spaces instead of moving focus.
    codeEditor.addEventListener('keydown', e => {
        if (e.key === 'Tab') {
            e.preventDefault();
            const start = codeEditor.selectionStart;
            const end   = codeEditor.selectionEnd;
            codeEditor.value = codeEditor.value.slice(0, start) + '   ' + codeEditor.value.slice(end);
            codeEditor.selectionStart = codeEditor.selectionEnd = start + 3;
            // Programmatic assignment doesn't fire 'input', so update manually.
            updateHighlight();
        }
        // Ctrl/Cmd+Enter runs the editor contents.
        if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') runEditorCode();
    });

    // Populate line numbers and highlighting on first display.
    updateLineNumbers();
    updateHighlight();

    // -----------------------------------------------------------------------
    // Clear buttons
    // -----------------------------------------------------------------------

    // Open button — click it to trigger the hidden file picker.
    codeOpenFileBtn.addEventListener('click', () => codeFileInput.click());

    // When the user picks a file, read it as text and place it in the editor.
    // FileReader.readAsText fires a 'load' event when done; the file contents
    // arrive as event.target.result.  We then refresh the gutter and
    // syntax-highlight layer exactly as we do after any other edit.
    codeFileInput.addEventListener('change', () => {
        const file = codeFileInput.files[0];
        if (!file) return;

        const reader = new FileReader();
        reader.addEventListener('load', e => {
            codeEditor.value = e.target.result;
            updateLineNumbers();
            updateHighlight();
            // Reset so picking the same file again still fires 'change'.
            codeFileInput.value = '';
        });
        reader.readAsText(file);
    });

    codeClearEditorBtn.addEventListener('click', () => {
        codeEditor.value = '';
        updateLineNumbers();
        updateHighlight();
    });

    codeClearOutputBtn.addEventListener('click', () => {
        codeOutput.className  = 'idle';
        codeOutput.textContent = '';
        codeElapsed.textContent = '';
    });

    codeClearDebugBtn.addEventListener('click', () => {
        codeDebugOutput.textContent = '';
    });

    codeClearConsoleBtn.addEventListener('click', () => {
        codeConsoleHistory.innerHTML = '';
    });

    // -----------------------------------------------------------------------
    // Console visibility toggle
    //
    // Toggling adds/removes the 'hide-console' class on #code-ui.  CSS rules
    // keyed on that class collapse the divider and console pane so the editor
    // and output panels fill the remaining height automatically.
    // -----------------------------------------------------------------------

    function applyConsoleVisible(visible) {
        if (visible) {
            codeUI.classList.remove('hide-console');
            codeConsoleToggleBtn.classList.add('active');
        } else {
            codeUI.classList.add('hide-console');
            codeConsoleToggleBtn.classList.remove('active');
        }
    }

    // Apply the saved preference when the code tab is first shown.
    applyConsoleVisible(getShowConsole());

    codeConsoleToggleBtn.addEventListener('click', () => {
        const nowVisible = codeUI.classList.contains('hide-console');
        setShowConsole(nowVisible);
        applyConsoleVisible(nowVisible);
    });

    // -----------------------------------------------------------------------
    // Resizable vertical divider (editor | output)
    // -----------------------------------------------------------------------

    codeDivider.addEventListener('mousedown', e => {
        e.preventDefault();
        codeDivider.classList.add('dragging');
        document.body.style.userSelect = 'none';
        document.body.style.cursor = 'col-resize';

        const startX     = e.clientX;
        const startWidth = codeLeftPane.getBoundingClientRect().width;

        function onMouseMove(e) {
            const mainWidth = codeMain.getBoundingClientRect().width;
            const newWidth  = Math.min(
                Math.max(150, startWidth + e.clientX - startX),
                mainWidth - codeDivider.offsetWidth - 150
            );
            codeLeftPane.style.flexBasis = newWidth + 'px';
        }

        function onMouseUp() {
            codeDivider.classList.remove('dragging');
            document.body.style.userSelect = '';
            document.body.style.cursor = '';
            document.removeEventListener('mousemove', onMouseMove);
            document.removeEventListener('mouseup', onMouseUp);
        }

        document.addEventListener('mousemove', onMouseMove);
        document.addEventListener('mouseup', onMouseUp);
    });

    // -----------------------------------------------------------------------
    // Resizable horizontal divider (main | console)
    // -----------------------------------------------------------------------

    codeConsoleDivider.addEventListener('mousedown', e => {
        e.preventDefault();
        codeConsoleDivider.classList.add('dragging');
        document.body.style.userSelect = 'none';
        document.body.style.cursor = 'row-resize';

        const startY      = e.clientY;
        const startHeight = codeConsolePane.getBoundingClientRect().height;

        function onMouseMove(e) {
            const newHeight = Math.max(60, startHeight - (e.clientY - startY));
            codeConsolePane.style.flexBasis = newHeight + 'px';
        }

        function onMouseUp() {
            codeConsoleDivider.classList.remove('dragging');
            document.body.style.userSelect = '';
            document.body.style.cursor = '';
            document.removeEventListener('mousemove', onMouseMove);
            document.removeEventListener('mouseup', onMouseUp);
        }

        document.addEventListener('mousemove', onMouseMove);
        document.addEventListener('mouseup', onMouseUp);
    });

    // -----------------------------------------------------------------------
    // Run editor code
    //
    // Posts the editor contents to POST /admin/run with the bearer token.
    // The server compiles and runs the code with a fresh symbol table each
    // time (console: false = editor mode), then returns the output.
    // -----------------------------------------------------------------------

    // Post a single request to /admin/run and return the parsed JSON body.
    // Throws on network error or non-2xx HTTP status (after handling 401/403).
    async function adminRunPost(payload) {
        const token = getToken();
        const res = await fetch('/admin/run', {
            method:  'POST',
            headers: {
                'Content-Type':  'application/json',
                'Authorization': token ? 'Bearer ' + token : '',
            },
            body: JSON.stringify(payload),
        });

        if (res.status === 401 || res.status === 403) {
            clearToken();
            showLogin('Session expired. Please sign in again.');
            throw new Error('auth');
        }

        if (!res.ok) {
            throw new Error('HTTP error ' + res.status);
        }

        return res.json();
    }

    // Enable or disable the four debugger control buttons as a group.
    function setDebugButtonsEnabled(enabled) {
        codeDebugContinueBtn.disabled  = !enabled;
        codeDebugStepBtn.disabled      = !enabled;
        codeDebugStepReturnBtn.disabled  = !enabled;
        codeDebugStepOverBtn.disabled  = !enabled;
    }

    // Show the debugger panel, make the input row visible, update the prompt,
    // and focus the input field ready for the next command.
    function showDebugInput(prompt) {
        codeDebugPrompt.textContent = prompt || 'debug>';
        codeDebuggerPanel.style.display = 'flex';
        codeDebugInputRow.style.display = 'flex';
        codeDebugInput.value = '';
        codeDebugInput.focus();
        setDebugButtonsEnabled(true);
    }

    // Hide the input row inside the debugger panel without hiding the panel
    // itself, so accumulated debugger output remains visible.
    function hideDebugInput() {
        codeDebugInputRow.style.display = 'none';
        codeDebugInput.value = '';
        setDebugButtonsEnabled(false);
    }

    // Hide the entire debugger panel and clear its output.  Called at the
    // start of every new run so the panel is blank and out of the way.
    function hideDebugPanel() {
        codeDebuggerPanel.style.display = 'none';
        codeDebugInputRow.style.display = 'flex'; // reset for next session
        codeDebugOutput.textContent = '';
        codeDebugInput.value = '';
    }

    // Append a chunk of debugger message text to the Debugger output panel.
    // Each call creates a new entry element so messages are individually
    // delimited and the panel stays scrolled to the bottom.
    function appendDebuggerOutput(text) {
        if (!text) return;
        const entry = document.createElement('div');
        entry.className   = 'code-debug-entry';
        entry.textContent = text;
        codeDebugOutput.appendChild(entry);
        codeDebugOutput.scrollTop = codeDebugOutput.scrollHeight;
    }

    // Append program stdout to the Output pane.
    function appendProgramOutput(text) {
        if (!text) return;
        if (codeOutput.classList.contains('idle')) {
            codeOutput.textContent = text;
            codeOutput.className   = 'ok';
        } else {
            codeOutput.textContent += text;
        }
    }

    // Finish a debug session: hide the input row, append a completion notice
    // to the debugger panel (keeping it visible so the user can review output),
    // restore the run controls, and ensure the Output pane has a final state.
    function finishDebugSession() {
        hideDebugInput();
        appendDebuggerOutput('Program execution complete.');
        if (codeOutput.classList.contains('idle') || codeOutput.textContent === '') {
            codeOutput.textContent = '(no output)';
        }
        codeOutput.className  = 'ok';
        codeRunBtn.disabled   = false;
        codeRunArrow.disabled = false;
        codeSpinner.classList.remove('running');
        codeDebugLine = 0;
        updateDebugBand();
    }

    // Send one debug command (or '' to start the first stop) and handle the
    // response.  The session remains active as long as debugWaiting is true.
    async function sendDebugCommand(input) {
        codeDebugSendBtn.disabled = true;
        codeDebugInput.disabled   = true;
        setDebugButtonsEnabled(false);

        try {
            const data = await adminRunPost({
                session:    codeSessionUUID,
                debug:      true,
                trace:      codeTraceEnabled || undefined,
                debugInput: input,
            });

            appendDebuggerOutput(data.debugOutput);
            appendProgramOutput(data.programOutput);

            if (data.error) {
                appendDebuggerOutput('Error: ' + data.error);
                finishDebugSession();
                codeOutput.className = 'error';
                return;
            }

            if (data.debugWaiting) {
                codeDebugLine = data.line || 0;
                scrollToDebugLine(codeDebugLine);
                updateDebugBand();
                showDebugInput(data.debugPrompt);
            } else {
                finishDebugSession();
            }
        } catch (err) {
            if (err.message !== 'auth') {
                codeOutput.textContent = 'Network error: ' + err.message;
                codeOutput.className   = 'error';
            }
            hideDebugInput();
            codeRunBtn.disabled   = false;
            codeRunArrow.disabled = false;
            codeSpinner.classList.remove('running');
        } finally {
            codeDebugSendBtn.disabled = false;
            codeDebugInput.disabled   = false;
        }
    }

    async function runEditorCode() {
        // Format the editor contents before sending them to the server, then
        // update the editor so the user sees the formatted version.
        const formatted = fmtEgo(codeEditor.value);
        if (formatted !== codeEditor.value) {
            codeEditor.value = formatted;
            updateLineNumbers();
            updateHighlight();
        }

        codeRunBtn.disabled   = true;
        codeRunArrow.disabled = true;
        codeSpinner.classList.add('running');
        codeOutput.className   = 'idle';
        codeOutput.textContent = '';
        codeElapsed.textContent = '';
        hideDebugPanel();

        // If the editor declares a func main(), append a call to it so the
        // server's Ego runtime actually invokes it.  The regex matches the
        // declaration anywhere in the source, ignoring leading whitespace.
        // We append to the code sent to the server only — the editor text
        // is left unchanged so the user does not see the extra line.
        let code = codeEditor.value;
        if (/^\s*func\s+main\s*\(\s*\)/m.test(code)) {
            code += '\n\nmain()';
        }

        if (codeRunMode === 'debug') {
            // Debug mode: compile on the server and start a debug session.
            // finishDebugSession (called by sendDebugCommand) clears spinner/buttons.
            try {
                const data = await adminRunPost({ code, session: codeSessionUUID, debug: true });

                appendDebuggerOutput(data.debugOutput);
                appendProgramOutput(data.programOutput);

                if (data.error) {
                    appendDebuggerOutput('Error: ' + data.error);
                    finishDebugSession();
                    codeOutput.className = 'error';
                } else if (data.debugWaiting) {
                    codeDebugLine = data.line || 0;
                    scrollToDebugLine(codeDebugLine);
                    updateDebugBand();
                    showDebugInput(data.debugPrompt);
                } else {
                    finishDebugSession();
                }
            } catch (err) {
                if (err.message !== 'auth') {
                    codeOutput.textContent = 'Network error: ' + err.message;
                    codeOutput.className   = 'error';
                }
                codeRunBtn.disabled   = false;
                codeRunArrow.disabled = false;
                codeSpinner.classList.remove('running');
            }
            return;
        }

        // Normal run mode.
        try {
            const payload = { code, session: codeSessionUUID };
            if (codeTraceEnabled) payload.trace = true;

            const data = await adminRunPost(payload);

            if (data.error) {
                codeOutput.textContent = (data.output ? data.output + '\n' : '') + 'Error: ' + data.error;
                codeOutput.className  = 'error';
            } else {
                codeOutput.textContent = data.output || '(no output)';
                codeOutput.className  = 'ok';
            }
            if (data.elapsed) {
                codeElapsed.textContent = 'Ran in ' + data.elapsed;
            }
        } catch (err) {
            if (err.message !== 'auth') {
                codeOutput.textContent = 'Network error: ' + err.message;
                codeOutput.className  = 'error';
            }
        } finally {
            codeRunBtn.disabled   = false;
            codeRunArrow.disabled = false;
            codeSpinner.classList.remove('running');
        }
    }

    codeRunBtn.addEventListener('click', runEditorCode);

    // Send debug input when the Send button is clicked or Enter is pressed.
    codeDebugSendBtn.addEventListener('click', () => {
        const cmd = codeDebugInput.value;
        codeDebugInput.value = '';
        sendDebugCommand(cmd);
    });

    // Debugger control buttons — shortcuts for common commands.
    codeDebugContinueBtn.addEventListener('click',  () => sendDebugCommand('continue'));
    codeDebugStepBtn.addEventListener('click',      () => sendDebugCommand('step'));
    codeDebugStepReturnBtn.addEventListener('click',  () => sendDebugCommand('step return'));
    codeDebugStepOverBtn.addEventListener('click',  () => sendDebugCommand('step over'));

    codeDebugInput.addEventListener('keydown', e => {
        if (e.key === 'Enter' && !e.shiftKey && !e.ctrlKey && !e.metaKey) {
            e.preventDefault();
            const cmd = codeDebugInput.value;
            codeDebugInput.value = '';
            sendDebugCommand(cmd);
        }
    });

    // -----------------------------------------------------------------------
    // Console REPL
    //
    // Sends each line to POST /admin/run with console: true so the server
    // reuses the persistent symbol table across successive console runs.
    // -----------------------------------------------------------------------

    // Append a prompt line and its output to the scrollable history div.
    function consoleAppend(code, outputText, isError) {
        const entry = document.createElement('div');
        entry.className = 'code-console-entry';

        const cmdLine = document.createElement('div');
        cmdLine.className   = 'code-console-cmd';
        cmdLine.textContent = 'ego> ' + code;
        entry.appendChild(cmdLine);

        if (outputText) {
            const outLine = document.createElement('div');
            outLine.className   = isError ? 'code-console-err' : 'code-console-out';
            outLine.textContent = outputText;
            entry.appendChild(outLine);
        }

        codeConsoleHistory.appendChild(entry);
        codeConsoleHistory.scrollTop = codeConsoleHistory.scrollHeight;
    }

    async function runConsoleCode() {
        const code = codeConsoleInput.value;
        if (!code.trim()) return;
        codeConsoleInput.value = '';

        try {
            const token = getToken();
            const res = await fetch('/admin/run', {
                method:  'POST',
                headers: {
                    'Content-Type':  'application/json',
                    'Authorization': token ? 'Bearer ' + token : '',
                },
                body: JSON.stringify({ code, console: true, session: codeSessionUUID }),
            });

            if (res.status === 401 || res.status === 403) {
                clearToken();
                showLogin('Session expired. Please sign in again.');
                return;
            }

            if (!res.ok) {
                consoleAppend(code, 'HTTP error ' + res.status, true);
                return;
            }

            const data = await res.json();

            if (data.error) {
                const text = (data.output ? data.output + '\n' : '') + 'Error: ' + data.error;
                consoleAppend(code, text, true);
            } else {
                consoleAppend(code, data.output || '', false);
            }
        } catch (err) {
            consoleAppend(code, 'Network error: ' + err.message, true);
        }
    }

    codeConsoleInput.addEventListener('keydown', e => {
        if (e.key === 'Enter' && !e.shiftKey && !e.ctrlKey && !e.metaKey) {
            e.preventDefault();
            runConsoleCode();
        }
    });
}

// ==========================================================================
// Startup
//
// Code at the top level of a script file (outside any function) runs once,
// immediately when the browser loads the file. This is the entry point.
// ==========================================================================

// Fetch and display server name/UUID/uptime in the header — no login needed.
loadServerInfo();

// Apply persisted preferences BEFORE loading any content so there is no
// flash of wrong theme and no spurious unauthenticated API call.
applyDarkMode(getDarkMode());

// Restore a saved token (if "Remember login" was on) and open the last active
// tab, but only if the server is reachable. Falls back to 'memory' and shows
// the login overlay whenever a fresh login is required.
(async function () {
    let serverUp = false;
    try {
        const res = await fetch('/services/up');
        serverUp = res.ok;
    } catch (_) {
        // The underscore (_) is a convention for an intentionally unused variable.
        // We don't need the error object here — the fact that the fetch threw
        // at all is enough to know the server is unreachable.
    }

    // Load the passkey feature flag before deciding what UI to show.
    // loadPasskeyConfig() also calls applyPasskeyLoginUI() when done.
    if (serverUp) await loadPasskeyConfig();

    const savedToken = getCookie(COOKIE_TOKEN);
    const savedTab   = getCookie(COOKIE_ACTIVE_TAB);

    if (serverUp && savedToken) {
        _token = savedToken; // restore directly to avoid re-writing the cookie

        // Validate the saved token is still accepted by the server before
        // hiding the login overlay. apiFetch calls clearToken() + showLogin()
        // and throws on a 401/403, so if the token is expired we fall through
        // to the catch block and the user sees the login prompt.
        try {
            await apiFetch('/services/admin/server');
        } catch (_) {
            // Token was rejected — apiFetch already called showLogin().
            openTab('memory');
            return;
        }

        // Restore the role flags from the saved cookie so tab visibility
        // matches what was set when the user originally logged in.
        restoreRole();
        applyTabVisibility();

        hideLogin();

        // Validate the saved tab name before using it — the cookie value could
        // be stale if a tab was renamed or removed. Fall back to the appropriate
        // default for the user's role.
        const defaultTab = _isAdmin ? 'memory' : 'code';
        const restoredTab = savedTab && tabLoaders[savedTab] ? savedTab : defaultTab;
        openTab(_isAdmin ? restoredTab : 'code');
    } else {
        showLogin();
        openTab('memory');
    }
})();

// Wire the SQL editor highlight layer and the pane resize handle.
initSqlEditor();
initSqlResizeHandle();

// ── WebAuthn / Passkey support ────────────────────────────────────────────────
//
// Two flows are implemented:
//   1. Login  — user clicks "Sign in with Passkey" on the login overlay.
//   2. Register — admin clicks "+ Passkey" in the edit-user sheet while already
//                 signed in (registers a passkey for their own account).
//
// Both flows use the discoverable-credential (resident-key) model so the user
// never needs to type a username — Face ID / Touch ID / Windows Hello identifies
// them automatically.
//
// Base64URL helpers — WebAuthn protocol uses base64url-encoded binary everywhere;
// the browser WebAuthn API uses ArrayBuffer.  These helpers bridge the gap.

function bufferToBase64url(buffer) {
    const bytes = new Uint8Array(buffer);
    let str = '';
    for (const b of bytes) str += String.fromCharCode(b);
    return btoa(str).replace(/\+/g, '-').replace(/\//g, '_').replace(/=/g, '');
}

function base64urlToBuffer(b64url) {
    const b64 = b64url.replace(/-/g, '+').replace(/_/g, '/');
    const bin = atob(b64);
    const buf = new Uint8Array(bin.length);
    for (let i = 0; i < bin.length; i++) buf[i] = bin.charCodeAt(i);
    return buf.buffer;
}

// Recursively walk an object returned by the server and decode any string
// fields whose names are known to carry base64url-encoded binary data into
// ArrayBuffers, as the browser WebAuthn API requires.
function decodeWebAuthnOptions(obj, skipBinary = false) {
    const binaryFields = new Set([
        'challenge', 'id', 'userId',
    ]);
    // rp.id is a plain domain string, not base64url binary — skip binary
    // decoding for the entire rp subtree to avoid passing it through atob().
    const noBinarySubtrees = new Set(['rp']);
    if (Array.isArray(obj)) return obj.map(item => decodeWebAuthnOptions(item, skipBinary));
    if (obj && typeof obj === 'object') {
        const out = {};
        for (const [k, v] of Object.entries(obj)) {
            if (noBinarySubtrees.has(k)) {
                out[k] = decodeWebAuthnOptions(v, true);
            } else if (!skipBinary && binaryFields.has(k) && typeof v === 'string') {
                out[k] = base64urlToBuffer(v);
            } else {
                out[k] = decodeWebAuthnOptions(v, skipBinary);
            }
        }
        return out;
    }
    return obj;
}

// applyPasskeyLoginUI shows the login-screen passkey button/divider only when
// both the server has passkeys enabled AND the browser supports WebAuthn.
// Called after loadPasskeyConfig() resolves.
function applyPasskeyLoginUI() {
    const show = passkeysActive() && !!window.PublicKeyCredential;
    const btn = document.getElementById('passkey-btn');
    const div = document.getElementById('passkey-divider');
    if (btn) {
        btn.style.display = show ? '' : 'none';
        // Re-attach listener idempotently by replacing the element clone trick
        // is unnecessary — just guard with the flag at click time instead.
    }
    if (div) div.style.display = show ? '' : 'none';
    if (show) btn && btn.addEventListener('click', submitPasskeyLogin);
}

// loadPasskeyConfig fetches /services/admin/webauthn/config (no auth needed),
// sets _passkeysEnabled, and then applies the login UI state.
async function loadPasskeyConfig() {
    try {
        const res = await fetch('/services/admin/webauthn/config');
        if (res.ok) {
            const cfg = await res.json();
            _passkeysEnabled = !!cfg.passkeys;
        }
    } catch (_) {
        // Server unreachable or old version without this endpoint — leave
        // _passkeysEnabled false so passkey UI stays hidden.
    }
    applyPasskeyLoginUI();
}

// submitPasskeyLogin drives the discoverable-login ceremony:
//   POST .../login/begin  → get options (challenge set as a cookie server-side)
//   navigator.credentials.get(options)  → browser prompts Face ID / Touch ID
//   POST .../login/finish → verify + receive token
async function submitPasskeyLogin() {
    const errEl = document.getElementById('login-error');
    const btn   = document.getElementById('passkey-btn');

    errEl.textContent = '';
    btn.disabled = true;
    clearToken();

    try {
        // Step 1: get the challenge options from the server.
        const beginRes = await fetch('/services/admin/webauthn/login/begin', {
            method:      'POST',
            credentials: 'same-origin',   // needed so the challenge cookie is sent/received
        });

        if (!beginRes.ok) {
            errEl.textContent = 'Passkey login not available on this server.';
            return;
        }

        const rawOptions = await beginRes.json();
        const options    = decodeWebAuthnOptions(rawOptions);

        // Step 2: invoke the platform authenticator (Face ID, Touch ID, etc.).
        const assertion = await navigator.credentials.get({ publicKey: options.publicKey });

        // Step 3: encode the assertion and send it to the server for verification.
        const finishPayload = {
            id:    bufferToBase64url(assertion.rawId),
            rawId: bufferToBase64url(assertion.rawId),
            type:  assertion.type,
            response: {
                authenticatorData: bufferToBase64url(assertion.response.authenticatorData),
                clientDataJSON:    bufferToBase64url(assertion.response.clientDataJSON),
                signature:         bufferToBase64url(assertion.response.signature),
                userHandle:        assertion.response.userHandle
                    ? bufferToBase64url(assertion.response.userHandle)
                    : null,
            },
        };

        const finishRes = await fetch('/services/admin/webauthn/login/finish', {
            method:      'POST',
            headers:     { 'Content-Type': 'application/json' },
            credentials: 'same-origin',
            body:        JSON.stringify(finishPayload),
        });

        const data = await finishRes.json();

        if (!finishRes.ok || !data.token) {
            errEl.textContent = data.message || data.msg || 'Passkey verification failed.';
            return;
        }

        if (!data.admin && !data.coder) {
            errEl.textContent = 'This account does not have permission to access the dashboard.';
            return;
        }

        // Success — same post-login flow as submitLogin().
        setToken(data.token);
        setRole(data.admin, data.coder, data.identity);
        lastActivity = Date.now();
        hideLogin();
        applyTabVisibility();
        openTab(_isAdmin ? activeTab : 'code');

    } catch (e) {
        if (e.name === 'NotAllowedError') {
            // User cancelled the authenticator prompt — not an error worth alarming about.
            errEl.textContent = 'Passkey prompt dismissed.';
        } else {
            errEl.textContent = 'Passkey error: ' + e.message;
        }
    } finally {
        btn.disabled = false;
    }
}

// _doPasskeyRegistration is the shared core of the WebAuthn registration
// ceremony.  btn (may be null) is disabled while the request is in flight.
// errEl receives status and error text.  onSuccess is called when the server
// confirms the credential; pass null to get the default green-flash behavior.
async function _doPasskeyRegistration(btn, errEl, onSuccess) {
    const token      = getToken();
    const authHeader = token ? { 'Authorization': 'Bearer ' + token } : {};

    errEl.textContent  = '';
    errEl.style.color  = '';
    if (btn) btn.disabled = true;

    try {
        // Step 1: get creation options from the server (challenge is stored
        // server-side and round-tripped via an HttpOnly cookie).
        const beginRes = await fetch('/services/admin/webauthn/register/begin', {
            method:      'POST',
            credentials: 'same-origin',
            headers:     authHeader,
        });

        if (!beginRes.ok) {
            const errBody = await beginRes.text().catch(() => '');
            errEl.textContent = errBody.trim() || 'Passkey registration failed (HTTP ' + beginRes.status + ').';
            return;
        }

        const options = decodeWebAuthnOptions(await beginRes.json());

        // Step 2: invoke the platform authenticator (Face ID / Touch ID).
        const credential = await navigator.credentials.create({ publicKey: options.publicKey });

        // Step 3: encode the attestation and send it to the server.
        const finishPayload = {
            id:    bufferToBase64url(credential.rawId),
            rawId: bufferToBase64url(credential.rawId),
            type:  credential.type,
            response: {
                attestationObject: bufferToBase64url(credential.response.attestationObject),
                clientDataJSON:    bufferToBase64url(credential.response.clientDataJSON),
            },
        };

        const finishRes = await fetch('/services/admin/webauthn/register/finish', {
            method:      'POST',
            credentials: 'same-origin',
            headers:     { ...authHeader, 'Content-Type': 'application/json' },
            body:        JSON.stringify(finishPayload),
        });

        if (!finishRes.ok) {
            const d = await finishRes.json().catch(() => ({}));
            errEl.textContent = d.message || d.msg || 'Passkey registration failed.';
            return;
        }

        if (onSuccess) {
            onSuccess();
        } else {
            errEl.style.color = 'green';
            errEl.textContent = 'Passkey registered successfully!';
            setTimeout(() => { errEl.style.color = ''; errEl.textContent = ''; }, 3000);
        }

    } catch (e) {
        if (e.name === 'NotAllowedError') {
            errEl.textContent = 'Passkey prompt dismissed.';
        } else {
            errEl.textContent = 'Passkey error: ' + e.message;
        }
    } finally {
        if (btn) btn.disabled = false;
    }
}

// registerPasskey is called from the "+ Passkey" button in the edit-user sheet.
async function registerPasskey() {
    const btn   = document.getElementById('edit-user-passkey-btn');
    const errEl = document.getElementById('edit-user-error');

    if (!window.PublicKeyCredential) {
        errEl.textContent = 'This browser does not support passkeys.';
        return;
    }

    await _doPasskeyRegistration(btn, errEl, null);
}

// removePasskeys is called from the "- Passkey" button in the edit-user sheet.
// It sends DELETE /services/admin/webauthn/passkeys/{username} to clear all
// stored passkeys for the displayed user.
async function removePasskeys() {
    const btn   = document.getElementById('edit-user-clear-passkey-btn');
    const errEl = document.getElementById('edit-user-error');
    const name  = document.getElementById('edit-user-name').value;

    errEl.textContent = '';
    errEl.style.color = '';
    if (btn) btn.disabled = true;

    try {
        const res = await fetch('/services/admin/webauthn/passkeys/' + encodeURIComponent(name), {
            method:      'DELETE',
            credentials: 'same-origin',
            headers:     { 'Authorization': 'Bearer ' + getToken() },
        });

        if (!res.ok) {
            const d = await res.json().catch(() => ({}));
            errEl.textContent = d.message || d.msg || 'Failed to remove passkeys.';
            return;
        }

        errEl.style.color   = 'green';
        errEl.textContent   = 'Passkeys removed.';
        setTimeout(() => { errEl.style.color = ''; errEl.textContent = ''; }, 3000);
    } catch (e) {
        errEl.textContent = 'Network error: ' + e.message;
    } finally {
        if (btn) btn.disabled = false;
    }
}

// ── Passkey prompt (offered after password login) ─────────────────────────────

// maybeOfferPasskeyAfterLogin shows the passkey creation prompt when:
//   • the server has passkeys enabled (_passkeysEnabled), and
//   • the browser supports WebAuthn (window.PublicKeyCredential exists), and
//   • the user has not previously clicked "Don't Ask Again".
// Called only after a successful *password* login, not after passkey login.
function maybeOfferPasskeyAfterLogin() {
    if (!passkeysActive()) return;
    if (!window.PublicKeyCredential) return;
    if (getCookie(COOKIE_PASSKEY_OFFERED)) return;
    const overlay = document.getElementById('passkey-prompt-overlay');
    if (overlay) {
        document.getElementById('passkey-prompt-status').textContent = '';
        overlay.style.display = 'flex';
    }
}

// declinePasskeyPrompt hides the prompt.  When permanent is true it also sets
// the "don't ask again" cookie so the dialog is never shown in this browser.
function declinePasskeyPrompt(permanent) {
    if (permanent) {
        setCookie(COOKIE_PASSKEY_OFFERED, '1', PASSKEY_NO_MAX_AGE);
    }
    const overlay = document.getElementById('passkey-prompt-overlay');
    if (overlay) overlay.style.display = 'none';
}

// createPasskeyFromPrompt runs the registration ceremony from the prompt dialog.
// On success the dialog closes; on failure the error is shown inside the dialog.
async function createPasskeyFromPrompt() {
    const btn   = document.getElementById('passkey-prompt-create-btn');
    const errEl = document.getElementById('passkey-prompt-status');

    await _doPasskeyRegistration(btn, errEl, () => {
        // Success: show a brief confirmation then close the dialog.
        errEl.style.color = 'green';
        errEl.textContent = 'Passkey created!';
        setTimeout(() => declinePasskeyPrompt(false), 1500);
    });
}
