// dashboard.js
// Client-side logic for the Ego admin dashboard. Handles authentication,
// tab switching, and data loading for the Memory, Users, and DSNs tabs.

// ==========================================================================
// Cookie helpers
//
// Thin wrappers around document.cookie so the rest of the code doesn't need
// to deal with cookie string parsing directly.
// ==========================================================================

// Write a cookie. maxAgeSeconds sets when it expires (omit or 0 for session).
function setCookie(name, value, maxAgeSeconds) {
    let cookie = encodeURIComponent(name) + '=' + encodeURIComponent(value)
               + '; path=/; SameSite=Strict';
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
function deleteCookie(name) {
    document.cookie = encodeURIComponent(name) + '=; path=/; max-age=0; SameSite=Strict';
}

// ==========================================================================
// Settings — persisted as browser cookies
// ==========================================================================

const COOKIE_TOKEN        = 'ego_dashboard_token';
const COOKIE_REMEMBER     = 'ego_dashboard_remember';
const COOKIE_DARK_MODE    = 'ego_dashboard_dark';
const TOKEN_MAX_AGE       = 86400; // 24 hours in seconds
const PREF_MAX_AGE        = 30 * 86400; // 30 days for preference cookies

// Load the "remember login" preference from its cookie (default: false).
function getRememberLogin() {
    return getCookie(COOKIE_REMEMBER) === '1';
}

// Save the "remember login" preference.
function setRememberLogin(value) {
    setCookie(COOKIE_REMEMBER, value ? '1' : '0', PREF_MAX_AGE);
}

// Load the "dark mode" preference from its cookie (default: false).
function getDarkMode() {
    return getCookie(COOKIE_DARK_MODE) === '1';
}

// Save the "dark mode" preference and apply it immediately.
function setDarkMode(value) {
    setCookie(COOKIE_DARK_MODE, value ? '1' : '0', PREF_MAX_AGE);
    applyDarkMode(value);
}

// Apply or remove the dark class on <body>. The Code tab is excluded because
// #code-ui already has its own permanent dark theme.
function applyDarkMode(value) {
    document.body.classList.toggle('dark', value);
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

// Discard the token from memory and from any persisted cookie.
function clearToken() {
    _token = null;
    deleteCookie(COOKIE_TOKEN);
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

    // ---- Memory stats -------------------------------------------------------
    const memContainer = document.getElementById('memory-content');
    try {
        const res = await apiFetch('/admin/memory');
        const d   = await res.json();

        const rows = [
            ['Total memory in use',  fmtBytes(d.total)],
            ['System memory',        fmtBytes(d.system)],
            ['Current heap in use',  fmtBytes(d.current)],
            ['Stack memory',         fmtBytes(d.stack)],
            ['Objects in use',       d.objects.toLocaleString()],
            ['GC cycles run',        d.gc.toLocaleString()],
        ];

        let html = '<table><thead><tr><th>Metric</th><th>Value</th></tr></thead><tbody>';
        for (const [label, value] of rows) {
            html += '<tr><td>' + label + '</td><td>' + value + '</td></tr>';
        }
        html += '</tbody></table>';
        memContainer.innerHTML = html;
    } catch (e) {
        if (e.message !== 'Unauthorized') console.error('Error loading memory:', e);
    }

    // ---- Cache stats --------------------------------------------------------
    const cacheContainer = document.getElementById('caches-content');
    try {
        const res  = await apiFetch('/admin/caches');
        const data = await res.json();

        const summary = [
            ['Cached services',    data.serviceCount],
            ['Service cache size', data.serviceSize + ' Items'],
            ['Cached assets',      data.assetCount],
            ['Asset cache size',   fmtBytes(data.assetSize)],
            ['Authorizations',     data.authorizationCount],
            ['Cached tokens',      data.tokenCount],
            ['Blacklisted tokens', data.blacklistCount],
            ['User items',         data.userItemsCount],
            ['DSN entries',        data.dsnCount],
            ['Schema entries',     data.schemaCount],
        ];

        let html = '<table><thead><tr><th>Cache</th><th>Value</th></tr></thead><tbody>';
        for (const [label, value] of summary) {
            html += '<tr><td>' + label + '</td><td>' + value + '</td></tr>';
        }
        html += '</tbody></table>';

        const items = data.items || [];
        if (items.length > 0) {
            html += '<br><table><thead><tr>'
                  + '<th>Name</th><th>Class</th><th>Reuse count</th><th>Last accessed</th>'
                  + '</tr></thead><tbody>';
            for (const item of items) {
                const lastStr = item.last ? new Date(item.last).toLocaleString() : '';
                html += '<tr>'
                      + '<td>' + escapeHtml(item.name)  + '</td>'
                      + '<td>' + escapeHtml(item.class) + '</td>'
                      + '<td>' + item.count             + '</td>'
                      + '<td>' + lastStr                + '</td>'
                      + '</tr>';
            }
            html += '</tbody></table>';
        }

        cacheContainer.innerHTML = html;
    } catch (e) {
        if (e.message !== 'Unauthorized') console.error('Error loading caches:', e);
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

        let html = '<table><thead><tr><th>User</th><th>Permissions</th></tr></thead><tbody>';

        for (const u of users) {
            const perms = Array.isArray(u.permissions) ? u.permissions.join(', ') : (u.permissions || '');

            // data-name and data-perms carry the row's values into the click handler
            // without needing a global variable. escapeHtml() is used both for display
            // and for safely encoding the attribute values.
            html += '<tr data-name="' + escapeHtml(u.name) + '" data-perms="' + escapeHtml(perms) + '">'
                  + '<td>' + escapeHtml(u.name) + '</td>'
                  + '<td>' + escapeHtml(perms)  + '</td>'
                  + '</tr>';
        }

        html += '</tbody></table>';
        container.innerHTML = html;

        // Attach a click listener to every row so clicking opens the edit sheet.
        container.querySelectorAll('tbody tr').forEach(row => {
            row.addEventListener('click', () => {
                showEditUserSheet(row.dataset.name, row.dataset.perms);
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
            const port = d.port ? String(d.port) : ''; // String() converts the number to text

            // For the boolean flags, show "Yes"/"No" rather than true/false.
            // The ternary operator (condition ? 'Yes' : 'No') is a compact if/else.
            html += '<tr>'
                  + '<td>' + escapeHtml(d.name)      + '</td>'
                  + '<td>' + escapeHtml(d.provider)  + '</td>'
                  + '<td>' + escapeHtml(d.database)  + '</td>'
                  + '<td>' + escapeHtml(host)         + '</td>'
                  + '<td>' + escapeHtml(port)         + '</td>'
                  + '<td>' + escapeHtml(d.user || '') + '</td>'
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

// Open the table-detail sheet and fetch column metadata for the given table.
async function showTableDetail(dsn, tableName) {
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

// An object that maps each tab's string id to the function that loads its
// data. This lets openTab() call the right loader with a single line
// (tabLoaders[tabId]()) instead of a chain of if/else statements.
const tabLoaders = {
    memory:  loadMemory,
    users:   loadUsers,
    dsns:    loadDsns,
    tables:  loadTables,
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
    const flexTabs = new Set(['code', 'log']);
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
        // POST the credentials as a JSON body. JSON.stringify() converts the
        // JavaScript object { username, password } into the JSON string
        // {"username":"...","password":"..."}.
        const res = await fetch('/services/admin/logon', {
            method:  'POST',
            headers: { 'Content-Type': 'application/json' },
            body:    JSON.stringify({ username, password }),
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

        // Success — store the token in memory and reset the inactivity clock.
        setToken(data.token);
        lastActivity = Date.now();
        hideLogin();

        // Reload the active tab now that we have a valid token.
        openTab(activeTab);

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
// Edit User sheet
// ==========================================================================

// Open the slide-in edit panel, pre-populated with the user's current values.
// name and perms come from the data attributes set on the table row.
function showEditUserSheet(name, perms) {
    document.getElementById('edit-user-error').textContent = '';
    document.getElementById('edit-user-name').value        = name;
    document.getElementById('edit-user-password').value   = '';
    document.getElementById('edit-user-permissions').value = perms;
    document.getElementById('edit-user-overlay').style.display = 'flex';
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
        document.getElementById('server-name').textContent  = d.server.name;
        document.getElementById('server-id').textContent    = d.server.id;
        document.getElementById('server-since').textContent = 'Up since ' + d.since;
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
// Settings sheet
// ==========================================================================

// Open the settings sheet and sync all toggles to their stored preferences.
function showSettings() {
    document.getElementById('setting-remember-login').checked = getRememberLogin();
    document.getElementById('setting-dark-mode').checked      = getDarkMode();
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

// Fetch the last 500 log lines and render them into #log-content.
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

        const res = await fetch('/services/admin/log?tail=500', {
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

    // Collect the live <mark> elements in DOM order so Prev/Next can address them.
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
    logMatchIndex = (logMatchIndex + 1) % logMatches.length;
    logHighlightCurrent();
}

// Jump to the previous match, wrapping around at the start.
function logSearchPrev() {
    if (logMatches.length === 0) return;
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
        loggerOriginalState = { keep: data.keep, loggers: Object.assign({}, data.loggers) };

        document.getElementById('logger-file').textContent = data.file || '';
        document.getElementById('logger-keep').value = data.keep;

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

        // Watch the keep field for changes too (replace any previous listener).
        document.getElementById('logger-keep').oninput = updateLoggerSaveBtn;

        updateLoggerSaveBtn();
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

// Enable the Save button only when something has actually changed.
function updateLoggerSaveBtn() {
    const keepVal    = parseInt(document.getElementById('logger-keep').value, 10) || 0;
    const keepChanged = keepVal !== loggerOriginalState.keep;

    let loggerChanged = false;
    document.querySelectorAll('#logger-toggles input[type=checkbox]').forEach(input => {
        if (input.checked !== loggerOriginalState.loggers[input.dataset.logger]) {
            loggerChanged = true;
        }
    });

    document.getElementById('logger-save-btn').disabled = !(keepChanged || loggerChanged);
}

// POST only the changed loggers (plus keep) to /admin/loggers.
async function submitLoggerConfig() {
    const keepVal       = parseInt(document.getElementById('logger-keep').value, 10) || 0;
    const changedLoggers = {};

    document.querySelectorAll('#logger-toggles input[type=checkbox]').forEach(input => {
        const name = input.dataset.logger;
        if (input.checked !== loggerOriginalState.loggers[name]) {
            changedLoggers[name] = input.checked;
        }
    });

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
    const codeDivider      = document.getElementById('code-divider');
    const codeLeftPane     = document.getElementById('code-left-pane');
    const codeMain         = document.getElementById('code-main');
    const codeConsoleDivider  = document.getElementById('code-console-divider');
    const codeConsolePane     = document.getElementById('code-console-pane');
    const codeConsoleHistory  = document.getElementById('code-console-history');
    const codeConsoleInput    = document.getElementById('code-console-input');
    const codeClearEditorBtn  = document.getElementById('code-clear-editor-btn');
    const codeClearOutputBtn  = document.getElementById('code-clear-output-btn');
    const codeClearConsoleBtn = document.getElementById('code-clear-console-btn');

    // -----------------------------------------------------------------------
    // Run / Trace split button
    //
    // codeCurrentTrace tracks whether the next run should enable the trace
    // logger. Clicking the ▾ arrow opens a dropdown with two items:
    //   "▶ Run"      — normal execution, codeCurrentTrace = false
    //   "🔍 Trace"   — execution with tracing, codeCurrentTrace = true
    // Selecting an item updates the main button label and immediately runs.
    // -----------------------------------------------------------------------
    let codeCurrentTrace = false;

    // Toggle the dropdown when the ▾ arrow is clicked.
    codeRunArrow.addEventListener('click', e => {
        e.stopPropagation(); // prevent the document click handler from closing it immediately
        codeRunDrop.classList.toggle('open');
    });

    // Close the dropdown when the user clicks anywhere outside it.
    document.addEventListener('click', () => codeRunDrop.classList.remove('open'));

    // Wire each dropdown item: set mode, update label, and run.
    codeRunDrop.querySelectorAll('.code-run-item').forEach(item => {
        item.addEventListener('click', e => {
            e.stopPropagation();
            codeRunDrop.classList.remove('open');

            codeCurrentTrace = item.dataset.trace === 'true';

            // Reflect the selected mode in the main button label.
            codeRunBtn.textContent = codeCurrentTrace ? '\u{1F50E} Trace' : '\u25B6 Run';

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

    codeClearEditorBtn.addEventListener('click', () => {
        codeEditor.value = '';
        updateLineNumbers();
        updateHighlight();
    });

    codeClearOutputBtn.addEventListener('click', () => {
        codeOutput.className  = 'idle';
        codeOutput.textContent = '';
    });

    codeClearConsoleBtn.addEventListener('click', () => {
        codeConsoleHistory.innerHTML = '';
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

    async function runEditorCode() {
        codeRunBtn.disabled   = true;
        codeRunArrow.disabled = true;
        codeSpinner.classList.add('running');
        codeOutput.className  = 'idle';
        codeOutput.textContent = 'Running\u2026';

        try {
            const token   = getToken();
            const payload = { code: codeEditor.value, session: codeSessionUUID };
            if (codeCurrentTrace) payload.trace = true;

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
                return;
            }

            if (!res.ok) {
                codeOutput.textContent = 'HTTP error ' + res.status;
                codeOutput.className  = 'error';
                return;
            }

            const data = await res.json();

            if (data.error) {
                codeOutput.textContent = (data.output ? data.output + '\n' : '') + 'Error: ' + data.error;
                codeOutput.className  = 'error';
            } else {
                codeOutput.textContent = data.output || '(no output)';
                codeOutput.className  = 'ok';
            }
        } catch (err) {
            codeOutput.textContent = 'Network error: ' + err.message;
            codeOutput.className  = 'error';
        } finally {
            codeRunBtn.disabled   = false;
            codeRunArrow.disabled = false;
            codeSpinner.classList.remove('running');
        }
    }

    codeRunBtn.addEventListener('click', runEditorCode);

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

// Restore a saved token cookie (if "Remember login" was on) before the first
// tab load; apiFetch() reads _token synchronously on its first call.
(function () {
    const savedToken = getCookie(COOKIE_TOKEN);
    if (savedToken) {
        _token = savedToken; // restore directly to avoid re-writing the cookie
        hideLogin();
    } else {
        showLogin();
    }
})();
openTab('memory');
