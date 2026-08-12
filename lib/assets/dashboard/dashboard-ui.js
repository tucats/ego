// dashboard-ui.js
// Shell and session UI: tab switching, the login overlay, the new-user,
// new-DSN and edit-user sheets, the server info header, the hamburger menu,
// the settings sheet, and the Log tab with its logger configuration.
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
    setCookie(COOKIE_ACTIVE_TAB, tabId, SETTINGS_MAX_AGE);

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
    // 'users', 'dsns', and 'tables' were previously plain display:'block' panes with no
    // scrollable region at all -- a table longer than the viewport was simply clipped
    // with no way to reach the rest of it. Flex-column (matching 'data'/'memory') gives
    // their *-content div a bounded height so its own overflow-y:auto can take over.
    const flexTabs = new Set(['code', 'log', 'data', 'sql', 'users', 'dsns', 'tables']);
    document.getElementById(tabId).style.display = flexTabs.has(tabId) ? 'flex' : 'block';

    // Find the tab button whose onclick attribute matches tabId and highlight it.
    // querySelector() returns the first element in the document that matches
    // the CSS selector string — here we're searching by attribute value.
    document.querySelector('[onclick="openTab(\'' + tabId + '\')"]').classList.add('active-tab');

    // Invoke the loader function for this tab. tabLoaders is declared in
    // dashboard-startup.js, which loads after this file — reading it here is
    // safe because this runs on a click, long after every file has loaded.
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

        // Success — store the token and role, apply the server's idle-timeout
        // setting, then reset the inactivity clock.
        setToken(data.token);
        setRole(data.admin, data.coder, data.identity);
        setIdleTimeout(data.inactivityTimeout);
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
// Server info
//
// Fetches /services/up (no authentication required) and caches the server's
// name, version, UUID, and start time in global variables. This runs
// immediately on page load, before the user even logs in, so the values are
// ready by the time showConfigSheet() displays them in the Configuration
// sheet (and loadMemory() uses the start time for Metrics' Uptime row).
// ==========================================================================
async function loadServerInfo() {
    try {
        const res = await fetch('/services/up'); // no apiFetch — this endpoint is public
        if (!res.ok) return; // silently skip if the server can't be reached
        const d = await res.json();

        _serverStartTime = d.since;
        _serverHostName  = d.server.name;
        _serverVersion   = d.version;
        _serverId        = d.server.id;
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

// Toggle the dropdown open/closed. Refreshes the "Logging in as <name>"
// reminder line (and its separator) each time it opens, so it always
// reflects whoever is currently authenticated -- hidden entirely when
// no one is logged in (e.g. the login overlay is still showing).
function toggleHamburgerMenu() {
    const dropdown = document.getElementById('hamburger-dropdown');
    const btn      = document.getElementById('hamburger-btn');
    const isOpen   = dropdown.classList.contains('open');
    if (!isOpen) {
        const userLine  = document.getElementById('hamburger-user');
        const separator = document.getElementById('hamburger-separator');
        userLine.textContent = _currentUser ? 'Logging in as ' + _currentUser : '';
        userLine.style.display  = _currentUser ? '' : 'none';
        separator.style.display = _currentUser ? '' : 'none';
    }
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

// Highlight the single .settings-segmented-btn matching value inside the
// segmented control identified by containerId (see the Dark mode row).
function syncSegmented(containerId, value) {
    document.querySelectorAll('#' + containerId + ' .settings-segmented-btn').forEach(btn => {
        btn.classList.toggle('active', btn.dataset.value === value);
    });
}

// Open the settings sheet and sync all toggles to their stored preferences.
function showSettings() {
    document.getElementById('setting-remember-login').checked = getRememberLogin();
    syncSegmented('setting-dark-mode', getDarkMode());
    document.getElementById('setting-toolbar-style').checked  = getToolbarStyle() === 'text';
    document.getElementById('setting-use-passkeys').checked   = getUsePasskeys();
    document.getElementById('setting-format').checked         = codeFormatEnabled;
    document.getElementById('setting-console').checked        = getShowConsole();
    document.getElementById('settings-overlay').style.display = 'flex';
}

// Close the settings sheet.
function hideSettings() {
    document.getElementById('settings-overlay').style.display = 'none';
}

// Wire up both settings toggles once the DOM is ready.
document.addEventListener('DOMContentLoaded', () => {
    // "Remember login" — persist token as a cookie. setRememberLogin() already
    // deletes any stale token/role/identity cookie when turned off; when
    // turned on while already logged in, write the current in-memory session
    // out too.
    document.getElementById('setting-remember-login').addEventListener('change', function () {
        setRememberLogin(this.checked);
        if (this.checked && _token) {
            setCookie(COOKIE_TOKEN, _token, TOKEN_MAX_AGE);
            setCookie(COOKIE_ROLE, _isAdmin ? 'admin' : (_isCoder ? 'coder' : ''), TOKEN_MAX_AGE);
            if (_currentUser) setCookie(COOKIE_IDENTITY, _currentUser, TOKEN_MAX_AGE);
        }
    });

    // "Dark mode" — a 3-way segmented control (Auto / On / Off) rather than a
    // single checkbox; each button sets the preference to its own data-value
    // and re-highlights itself as the active choice.
    document.querySelectorAll('#setting-dark-mode .settings-segmented-btn').forEach(btn => {
        btn.addEventListener('click', function () {
            setDarkMode(this.dataset.value);
            syncSegmented('setting-dark-mode', this.dataset.value);
        });
    });

    // "Use Text Buttons" — applied app-wide via a body class; see
    // applyToolbarStyle() in dashboard-core.js. Checked (the default) means
    // "text", unchecked means "icons".
    document.getElementById('setting-toolbar-style').addEventListener('change', function () {
        setToolbarStyle(this.checked ? 'text' : 'icons');
    });

    // "Use passkeys" — re-applies passkey UI immediately so the login button
    // appears or disappears without needing a page reload.
    document.getElementById('setting-use-passkeys').addEventListener('change', function () {
        setUsePasskeys(this.checked);
    });

    // "Format" — persisted via the code-format cookie; read by runEditorCode().
    document.getElementById('setting-format').addEventListener('change', function () {
        codeFormatEnabled = this.checked;
        setCodeFormat(this.checked);
    });

    // "Console" — persists the preference and immediately applies it, in case
    // the Code tab is already open behind the Settings sheet.
    document.getElementById('setting-console').addEventListener('change', function () {
        setShowConsole(this.checked);
        applyConsoleVisible(this.checked);
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

// Update the "< 3 / 18 >" stepper pill next to the search box.
//
// Pass a total of zero to mean "nothing to step through": with no search term
// at all (hide === true) the pill disappears entirely, and after a search that
// found nothing it stays visible showing "0 / 0" with both arrows disabled, so
// the user gets an answer rather than a control that silently vanished.
//
// `position` is 1-based (the number the user reads), not the 0-based
// logMatchIndex used internally.
function logSetSearchStatus(position, total, hide) {
    const nav   = document.getElementById('log-search-nav');
    const count = document.getElementById('log-search-status');
    // The two arrow buttons are the pill's only <button> children.
    const btns  = nav.querySelectorAll('.icon-pill-btn');

    // classList.toggle(name, flag) adds the class when flag is true and
    // removes it when false — a shorthand for an if/else around add/remove.
    nav.classList.toggle('visible', !hide);
    nav.classList.toggle('empty', total === 0);

    count.textContent = position + ' / ' + total;
    btns.forEach(b => { b.disabled = (total === 0); });
}

// Show the "x" inside the search box only when there is text to clear, so an
// empty field is just an empty field. Called on every keystroke and by any
// code that changes the field's value -- assigning to input.value from script
// does NOT fire an input event, so those callers must invoke this themselves.
function logSyncSearchClear() {
    const input = document.getElementById('log-search-input');

    document.getElementById('log-search-clear')
        .classList.toggle('visible', input.value.length > 0);
}

// Build the query string for a log request from the line count and the active
// filter.
//
// encodeURIComponent escapes characters that would otherwise be read as part of
// the URL's own syntax. It matters most for the message pattern, which may
// legitimately contain "?" -- a single-character wildcard to the server, but
// the start of the query string to a URL parser.
//
// Filters that are not set are left out of the URL entirely rather than sent
// empty. The server does accept an empty value and reads it as "no filter", so
// either would work; omitting them keeps the URL that shows up in the server's
// own request log to just the filters actually in force.
function logQueryString() {
    const parts = ['tail=' + getLogTail()];

    if (logFilterState.session > 0) {
        parts.push('session=' + logFilterState.session);
    }

    if (logFilterState.classes.length > 0) {
        parts.push('class=' + encodeURIComponent(logFilterState.classes.join(',')));
    }

    if (logFilterState.msg !== '') {
        parts.push('msg=' + encodeURIComponent(logFilterState.msg));
    }

    if (logFilterState.archive) {
        parts.push('archive=true');
    }

    if (logFilterState.since !== '') {
        parts.push('since=' + encodeURIComponent(logFilterState.since));
    }

    if (logFilterState.until !== '') {
        parts.push('until=' + encodeURIComponent(logFilterState.until));
    }

    return parts.join('&');
}

// Fetch the last N log lines (N comes from getLogTail(), default 500), applying
// any active server-side filter, and render them into #log-content.
async function loadLog() {
    const container = document.getElementById('log-content');

    container.innerHTML = '<p style="padding:0.5rem;color:#666;">Loading\u2026</p>';

    // Clear any leftover search state from a previous load.
    logRawText    = '';
    logMatches    = [];
    logMatchIndex = -1;
    logSetSearchStatus(0, 0, true);

    try {
        const token = getToken();

        const res = await fetch('/services/admin/log?' + logQueryString(), {
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

        // The server rejects a filter it cannot honor -- an unknown logging
        // class, a malformed pattern, or a structured filter against a server
        // whose log is in text rather than JSON format. Its message names the
        // specific problem, so show that instead of a bare status code.
        if (res.status === 400) {
            const detail = await res.json().catch(() => ({}));

            container.innerHTML = '<p style="padding:0.5rem;color:#c0392b;">' +
                escapeHtml(detail.msg || 'The log filter was rejected by the server.') +
                '</p>';

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

// Scroll the log content area to the bottom. scrollTop is how far the content
// has been scrolled down, and scrollHeight is the full height of the content;
// setting the one to the other asks to scroll past the end, which the browser
// clamps to "as far down as it goes".
function logScrollToEnd() {
    const container = document.getElementById('log-content');
    container.scrollTop = container.scrollHeight;
}

// Scroll the log content area back to the top -- the oldest line fetched.
function logScrollToStart() {
    document.getElementById('log-content').scrollTop = 0;
}

// Build the highlighted HTML for the current search term and populate the
// match list. Called by logSearch() and reused by Prev/Next.
function logApplySearch(term) {
    const container = document.getElementById('log-content');

    if (!term) {
        logRenderPlain();
        logMatches    = [];
        logMatchIndex = -1;
        logSetSearchStatus(0, 0, true);
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
        logSetSearchStatus(0, 0, false);
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
}

// Centre the given match vertically inside the log pane, scrolling ONLY that
// pane.
//
// The obvious call here is current.scrollIntoView({block:'center'}), and that
// is what this used to do -- but scrollIntoView scrolls every scrollable
// ancestor of the element, not just the nearest one. The whole page is a
// little taller than the window, so bringing a match into view also nudged the
// document down and slid the header off the top of the screen. There is no
// option to tell scrollIntoView "this container only", so the scroll position
// is computed by hand instead.
//
// getBoundingClientRect() gives an element's position in window coordinates.
// Subtracting the container's top from the match's top gives how far the match
// sits below the top of the visible pane; subtracting half the leftover height
// turns "put it at the top" into "put it in the middle". Assigning to
// container.scrollTop touches nothing outside the pane, and the browser clamps
// the value to the scrollable range, so matches near either end simply land as
// close to centre as they can get.
function logScrollMatchIntoView(current) {
    const container = document.getElementById('log-content');

    const containerTop = container.getBoundingClientRect().top;
    const matchRect    = current.getBoundingClientRect();

    const offsetInPane = matchRect.top - containerTop;
    const centreOffset = (container.clientHeight - matchRect.height) / 2;

    container.scrollTop += offsetInPane - centreOffset;
}

// Mark the current match as the active one (orange) and scroll it into view.
function logHighlightCurrent() {
    logMatches.forEach(m => m.classList.remove('log-match-current'));

    if (logMatches.length === 0) return;

    const current = logMatches[logMatchIndex];
    current.classList.add('log-match-current');
    logScrollMatchIntoView(current);

    logSetSearchStatus(logMatchIndex + 1, logMatches.length, false);
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
    const input = document.getElementById('log-search-input');

    input.value = '';
    logSyncSearchClear();
    // Put the caret back in the search box so the user can type a new term
    // straight away, rather than having to click back into the field.
    input.focus();

    logRenderPlain();
    logMatches    = [];
    logMatchIndex = -1;
    logSetSearchStatus(0, 0, true);
}

// Allow the user to press Enter in the search box to trigger a search,
// and Escape to clear it — without needing to click a button.
document.getElementById('log-search-input').addEventListener('keydown', e => {
    if (e.key === 'Enter')  { e.preventDefault(); logSearch(); }
    if (e.key === 'Escape') { e.preventDefault(); logSearchClear(); }
});

// Show or hide the in-field "x" as the user types. The input event fires for
// every change the user makes -- typing, pasting, cutting, undo -- which
// keydown alone would not cover.
document.getElementById('log-search-input')
    .addEventListener('input', logSyncSearchClear);

// ==========================================================================
// Log filter sheet
//
// The funnel in the Log toolbar opens this. Everything in it changes what the
// SERVER sends back, which is a different thing from the search box in the
// toolbar: search looks through lines already on screen, while a filter changes
// which lines are fetched at all.
//
// Filtering has to happen on the server because the log is structured there --
// one JSON object per line, carrying a session number, a logging class, and a
// message identifier such as "log.server.request". The server flattens those
// into a readable sentence on the way out, translating the identifier and
// substituting its arguments. By the time a line arrives here, the pieces a
// filter needs have been merged into prose, so the browser could not do this
// job even if we wanted it to.
//
// Since/Until can be typed as free text, or picked via the calendar glyph
// beside each field -- which is decorative only; the real, fully native
// <input type="datetime-local"> that opens the picker is stacked invisibly
// on top of it (see .datetime-picker-input in dashboard.css for why it is
// driven by ordinary browser clicks rather than by script). Either way,
// normalizeLogDateTime() below turns the value into RFC 3339 before it is
// stored or sent, using the browser's own Date parser -- the same thing
// new Date(...) already does with whatever the native picker produces, so
// one function covers both entry paths. A value the browser cannot make
// sense of is sent through unchanged rather than rejected here: the server
// falls back to the same flexible parser the "ego" command line's
// --since/--until options normalize through (see parseLogQueryTime in
// internal/router/admin.go), so typing something like "8/11/2026" still has
// a good chance of working even though this quick client-side pass does not
// recognize it.
// ==========================================================================

// Turn a Since/Until field's raw text into RFC 3339, or return it unchanged
// if the browser's Date parser cannot make sense of it.
function normalizeLogDateTime(raw) {
    const value = raw.trim();

    if (value === '') return '';

    const parsed = new Date(value);

    return isNaN(parsed.getTime()) ? value : parsed.toISOString();
}

// Copy whatever the native pickers produce into their paired text field. The
// picker's own value is already local-time text of the form
// "2026-08-12T14:30:00" -- one of the formats the server accepts directly --
// but it still goes through the sheet's normal Apply-time normalization like
// any typed value, so there is exactly one code path that decides what
// actually gets sent.
document.getElementById('log-filter-since-picker').addEventListener('change', function () {
    document.getElementById('log-filter-since').value = this.value;
});

document.getElementById('log-filter-until-picker').addEventListener('change', function () {
    document.getElementById('log-filter-until').value = this.value;
});

// Show the filter sheet, filling it in from the filter currently in effect.
async function showLogFilter() {
    const overlay = document.getElementById('log-filter-overlay');

    document.getElementById('log-filter-error').textContent   = '';
    document.getElementById('log-filter-tail').value          = getLogTail();
    document.getElementById('log-filter-session').value       = logFilterState.session > 0 ? logFilterState.session : '';
    document.getElementById('log-filter-msg').value           = logFilterState.msg;
    document.getElementById('log-filter-archive').checked     = logFilterState.archive;
    document.getElementById('log-filter-since').value         = logFilterState.since;
    document.getElementById('log-filter-until').value         = logFilterState.until;

    overlay.style.display = 'flex';

    await renderLogFilterClasses();

    // Snapshot the fields so a backdrop click can tell whether anything was
    // edited and offer to discard. Taken after the class list is built, since
    // those checkboxes are part of the sheet's state.
    captureBaseline('log-filter-overlay');
}

// Build the checkbox list of logging classes.
//
// The names come from the server rather than being hardcoded, because a build
// can register additional loggers beyond the standard set. Every class is
// listed, including ones currently switched off: a logger that is off now may
// still have written the lines sitting in the log file, so filtering by it is
// perfectly sensible.
async function renderLogFilterClasses() {
    const list = document.getElementById('log-filter-classes');

    list.innerHTML = '<p class="field-hint">Loading classes…</p>';

    let names = [];

    try {
        const token = getToken();

        const res = await fetch('/admin/loggers', {
            headers: {
                'Accept':        'application/json',
                'Authorization': token ? 'Bearer ' + token : '',
            },
        });

        if (res.status === 401 || res.status === 403) {
            clearToken();
            hideLogFilter();
            showLogin('Session expired. Please sign in again.');

            return;
        }

        if (!res.ok) throw new Error('HTTP ' + res.status);

        const data = await res.json();

        names = Object.keys(data.loggers || {}).sort();
    } catch (e) {
        // The class list is the only part of the sheet that needs the server.
        // Losing it should not cost the user the session, message, and count
        // filters, so say so and carry on with the rest of the sheet usable.
        list.innerHTML = '';
        document.getElementById('log-filter-error').textContent =
            'Could not load the list of logging classes; the other filters still work.';

        return;
    }

    list.innerHTML = '';

    for (const name of names) {
        const row = document.createElement('label');
        row.className = 'filter-class-row';

        const box = document.createElement('input');
        box.type    = 'checkbox';
        box.value   = name;
        box.checked = logFilterState.classes.includes(name);

        const text = document.createElement('span');
        text.textContent = name;

        row.appendChild(box);
        row.appendChild(text);
        list.appendChild(row);
    }
}

// Close the sheet without applying anything.
function hideLogFilter() {
    document.getElementById('log-filter-overlay').style.display = 'none';
}

// Read the sheet, store the filter, and re-request the log.
function applyLogFilter() {
    const tail    = parseInt(document.getElementById('log-filter-tail').value, 10);
    const session = parseInt(document.getElementById('log-filter-session').value, 10);
    const error   = document.getElementById('log-filter-error');

    // Number.isNaN is true when the field was empty or held something that is
    // not a number at all. An empty session box means "any session", which is
    // fine; an empty or zero line count is not, since it would ask the server
    // for nothing.
    if (Number.isNaN(tail) || tail < 1) {
        error.textContent = 'Limit results must be a positive number.';

        return;
    }

    const classes = Array.from(
        document.querySelectorAll('#log-filter-classes input[type=checkbox]'))
        .filter(box => box.checked)
        .map(box => box.value);

    setLogTail(tail);

    logFilterState = {
        session: Number.isNaN(session) || session < 1 ? 0 : session,
        msg:     document.getElementById('log-filter-msg').value.trim(),
        classes: classes,
        archive: document.getElementById('log-filter-archive').checked,
        since:   normalizeLogDateTime(document.getElementById('log-filter-since').value),
        until:   normalizeLogDateTime(document.getElementById('log-filter-until').value),
    };

    saveLogFilter();
    updateLogFilterDot();
    hideLogFilter();

    // Re-request with the new filter. A rejected filter surfaces as a message
    // in the log pane, which is where the user is looking after applying one.
    loadLog();
}

// Reset every filter, leaving the line count alone -- that is a preference
// about how much to fetch, not a restriction on what comes back, so clearing
// filters should not silently change it.
function clearLogFilter() {
    document.getElementById('log-filter-error').textContent = '';
    document.getElementById('log-filter-session').value     = '';
    document.getElementById('log-filter-msg').value         = '';
    document.getElementById('log-filter-archive').checked   = false;
    document.getElementById('log-filter-since').value       = '';
    document.getElementById('log-filter-until').value       = '';

    document.querySelectorAll('#log-filter-classes input[type=checkbox]')
        .forEach(box => { box.checked = false; });

    applyLogFilter();
}

// Show a dot on the funnel when a filter is in force, so it is obvious that the
// log on screen is not the whole log. The title gains a summary, so hovering
// says which filters are active without opening the sheet.
function updateLogFilterDot() {
    const dot = document.getElementById('log-filter-dot');
    const btn = document.getElementById('log-filter-btn');

    // classList.toggle(name, flag) adds the class when flag is true and removes
    // it when false.
    dot.classList.toggle('visible', isLogFilterActive());

    const active = [];

    if (logFilterState.session > 0)        active.push('session ' + logFilterState.session);
    if (logFilterState.classes.length > 0) active.push(logFilterState.classes.join(', '));
    if (logFilterState.msg !== '')         active.push('messages matching ' + logFilterState.msg);
    if (logFilterState.archive)            active.push('older logs included');
    if (logFilterState.since !== '')       active.push('since ' + logFilterState.since);
    if (logFilterState.until !== '')       active.push('until ' + logFilterState.until);

    btn.title = active.length > 0 ? 'Filtered by ' + active.join('; ') : 'Filter log';
}

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

        // Watch the numeric field for changes (replace any previous listener).
        document.getElementById('logger-keep').oninput = updateLoggerSaveBtn;

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

// Save logger configuration.
//
// Everything in this sheet is server state now that the result limit has moved
// to the Log Filter sheet, so there is no longer a case where the Save button
// is enabled but nothing needs to be sent.
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

