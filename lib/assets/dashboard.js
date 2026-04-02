// dashboard.js
// Client-side logic for the Ego admin dashboard. Handles authentication,
// tab switching, and data loading for the Memory, Users, and DSNs tabs.

// ==========================================================================
// Token storage — in-memory only
//
// The bearer token is kept in a plain JavaScript variable rather than a
// cookie. This means it lives only as long as this page is open: it is
// never written to disk, never sent automatically by the browser on other
// requests, and disappears the moment the tab is closed or the page is
// refreshed. This is safer than a cookie for a sensitive credential.
// ==========================================================================

// The current bearer token. null means the user is not logged in.
// "let" allows this variable to be reassigned by setToken/clearToken.
let _token = null;

// Return the current token (or null if not logged in).
function getToken() {
    return _token;
}

// Store a new token after a successful login.
function setToken(token) {
    _token = token;
}

// Discard the token. After this call, getToken() returns null and all
// subsequent API calls will be rejected by the server until the user
// logs in again.
function clearToken() {
    _token = null;
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

// Load the Memory tab — fetches server memory statistics and renders them
// as a two-column table (metric name | formatted value).
async function loadMemory() {
    // Find the div in the HTML where the table will be placed.
    const container = document.getElementById('memory-content');
    try {
        const res  = await apiFetch('/admin/memory');
        const d    = await res.json(); // parse the JSON response body

        // Convert a raw byte count into a readable string like "3.14 MB".
        // The checks are in descending order so the largest matching unit wins.
        function fmtBytes(n) {
            if (n >= 1073741824) return (n / 1073741824).toFixed(2) + ' GB'; // 1 GB = 2^30 bytes
            if (n >= 1048576)    return (n / 1048576).toFixed(2)    + ' MB'; // 1 MB = 2^20 bytes
            if (n >= 1024)       return (n / 1024).toFixed(2)       + ' KB'; // 1 KB = 2^10 bytes
            return n + ' B';
        }

        // Build an array of [label, value] pairs — one per table row.
        // toLocaleString() formats a number with locale-appropriate thousands
        // separators, e.g. 1000000 becomes "1,000,000" in US English.
        const rows = [
            ['Total memory in use',  fmtBytes(d.total)],
            ['System memory',        fmtBytes(d.system)],
            ['Current heap in use',  fmtBytes(d.current)],
            ['Stack memory',         fmtBytes(d.stack)],
            ['Objects in use',       d.objects.toLocaleString()],
            ['GC cycles run',        d.gc.toLocaleString()],
        ];

        // Build the table as an HTML string. Including the <link> tag here
        // applies the shared table styles defined in ui-styles.css.
        let html = '<link rel="stylesheet" href="/assets/ui-styles.css">'
                 + '<table><thead><tr><th>Metric</th><th>Value</th></tr></thead><tbody>';

        // Destructuring assignment: [label, value] unpacks each two-element
        // array from the rows array on each iteration.
        for (const [label, value] of rows) {
            html += '<tr><td>' + label + '</td><td>' + value + '</td></tr>';
        }

        html += '</tbody></table>';

        // innerHTML replaces the entire content of the container div with the
        // HTML string we built. The browser parses and renders it immediately.
        container.innerHTML = html;
    } catch (e) {
        // Don't log an error for the 'Unauthorized' case — that is handled
        // by apiFetch() which shows the login overlay. Any other error
        // (network failure, malformed JSON, etc.) is worth logging.
        if (e.message !== 'Unauthorized') console.error('Error loading memory:', e);
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

        // Build a table matching the existing ui-styles.css appearance.
        let html = '<link rel="stylesheet" href="/assets/ui-styles.css">'
                 + '<table><thead><tr><th>User</th><th>Permissions</th></tr></thead><tbody>';

        for (const u of users) {
            // permissions may be a JSON array of strings. Array.isArray() checks
            // for that; if so, join() turns ["ego.logon","ego.admin"] into
            // "ego.logon, ego.admin". Otherwise fall back to the raw value or "".
            const perms = Array.isArray(u.permissions) ? u.permissions.join(', ') : (u.permissions || '');

            // escapeHtml() is called on every value we insert into the HTML to
            // prevent XSS — see its definition below.
            html += '<tr>'
                  + '<td>' + escapeHtml(u.name) + '</td>'
                  + '<td>' + escapeHtml(perms)  + '</td>'
                  + '</tr>';
        }

        html += '</tbody></table>';
        container.innerHTML = html;
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

        let html = '<link rel="stylesheet" href="/assets/ui-styles.css">'
                 + '<table><thead><tr>'
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

// Load the Caches tab — fetches the server cache summary and item list,
// renders a summary block of counts followed by a table of cached entries.
async function loadCaches() {
    const container = document.getElementById('caches-content');
    try {
        const res  = await apiFetch('/admin/caches');
        const data = await res.json();

        // Format a raw byte count as a human-readable string (KB / MB / GB).
        function fmtBytes(n) {
            if (n >= 1073741824) return (n / 1073741824).toFixed(2) + ' GB';
            if (n >= 1048576)    return (n / 1048576).toFixed(2)    + ' MB';
            if (n >= 1024)       return (n / 1024).toFixed(2)       + ' KB';
            return n + ' B';
        }

        // Summary rows: one [label, value] pair per interesting counter.
        const summary = [
            ['Cached services',    data.serviceCount],
            ['Service cache size', data.serviceSize + " Items"] ,
            ['Cached assets',      data.assetCount],
            ['Asset cache size',   fmtBytes(data.assetSize)],
            ['Authorizations',     data.authorizationCount],
            ['Cached tokens',      data.tokenCount],
            ['Blacklisted tokens', data.blacklistCount],
            ['User items',         data.userItemsCount],
            ['DSN entries',        data.dsnCount],
            ['Schema entries',     data.schemaCount],
        ];

        let html = '<link rel="stylesheet" href="/assets/ui-styles.css">';

        // Summary table — shows the aggregate cache counters.
        html += '<table><thead><tr><th>Cache</th><th>Value</th></tr></thead><tbody>';
        for (const [label, value] of summary) {
            html += '<tr><td>' + label + '</td><td>' + value + '</td></tr>';
        }
        html += '</tbody></table>';

        // Detailed items table — shows each individually cached entry.
        const items = data.items || [];
        if (items.length > 0) {
            html += '<br><table><thead><tr>'
                  + '<th>Name</th><th>Class</th><th>Reuse count</th><th>Last accessed</th>'
                  + '</tr></thead><tbody>';

            for (const item of items) {
                // The "last" field is an ISO 8601 timestamp. Passing it to
                // Date() and calling toLocaleString() converts it to the
                // user's local timezone and preferred date/time format.
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

        container.innerHTML = html;
    } catch (e) {
        if (e.message !== 'Unauthorized') console.error('Error loading caches:', e);
    }
}

// Send a DELETE to /admin/caches/ to flush all server-side caches, then
// reload the tab so the updated (empty) cache state is shown immediately.
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

        // Reload the tab to reflect the now-empty caches.
        loadCaches();
    } catch (e) {
        alert('Network error: ' + e.message);
    } finally {
        btn.disabled = false;
    }
}

// An object that maps each tab's string id to the function that loads its
// data. This lets openTab() call the right loader with a single line
// (tabLoaders[tabId]()) instead of a chain of if/else statements.
const tabLoaders = {
    memory:  loadMemory,
    users:   loadUsers,
    dsns:    loadDsns,
    caches:  loadCaches,
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
    document.getElementById(tabId).style.display = tabId === 'code' ? 'flex' : 'block';

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
        document.getElementById('server-since').textContent = 'since ' + d.since;
    } catch (e) {
        console.error('Could not load server info:', e);
    }
}

// ==========================================================================
// Logoff
// ==========================================================================

// Clear the in-memory token and show the login overlay.
// Called by the Sign Out button in the header.
function logoff() {
    clearToken();
    showLogin();
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

// loadCode is called by openTab every time the Code tab is selected.
// On the first call it initializes all the DOM wiring; subsequent calls
// are no-ops so the editor state (text, history) is preserved between tabs.
function loadCode() {
    if (codeTabInitialized) return;
    codeTabInitialized = true;
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
            const payload = { code: codeEditor.value };
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
                body: JSON.stringify({ code, console: true }),
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

// The token lives only in memory, so on every fresh page load the user must
// log in. Show the login overlay immediately, then set up the tab bar behind it.
showLogin();
openTab('memory');
