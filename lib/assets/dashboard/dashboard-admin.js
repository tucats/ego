// dashboard-admin.js
// The administrative tabs and their editing sheets: the per-tab content
// loaders (Memory, Users, DSNs, Tables, Tasks), the DSN permission sheets,
// the Tasks detail sheet, and the server configuration sheet.
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
// Tab content loaders
//
// Each function is responsible for one tab: it fetches data from the server,
// builds an HTML table string, and injects it into the tab's container div.
// They are called by openTab() every time a tab is selected, so the data is
// always refreshed when you switch tabs.
// ==========================================================================

// Convert a raw byte count into a readable string like "3.14 MB". Shared by
// loadMemory() (Metrics' memory figures) and showConfigSheet() (the Server
// Info block's total/available memory).
function fmtBytes(n) {
    if (n >= 1073741824) return (n / 1073741824).toFixed(2) + ' GB';
    if (n >= 1048576)    return (n / 1048576).toFixed(2)    + ' MB';
    if (n >= 1024)       return (n / 1024).toFixed(2)       + ' KB';
    return n + ' B';
}

// Load the Memory tab — fetches server memory statistics and cache data,
// rendering both into the combined Memory tab.
async function loadMemory() {
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
        const res = await apiFetch('admin/resources');
        const d   = await res.json();

        const sep = '<td class="stat-sep"></td>';

        // The goroutine count was added to /admin/resources after this dashboard
        // shipped, so tolerate its absence rather than letting an undefined value
        // throw. Calling .toLocaleString() on undefined raises a TypeError, and
        // because the whole Metrics table is assembled in one expression that
        // would blank the entire panel rather than just this one cell. The "|| 0"
        // idiom is the same guard already used for item.size further down.
        const goroutines = d.goroutines || 0;

        // Metrics — 3-column compact grid (same 8-cell row structure as Cache Status below)
        let memHtml = '<table class="stat-grid"><thead><tr><th colspan="8">Metrics</th></tr></thead><tbody>';
        memHtml += '<tr>' + pair('Uptime',             fmtUptime(_serverStartTime))       + sep + pair('Objects in Use',  d.objects.toLocaleString()) + sep + pair('Application Memory', fmtBytes(d.system))  + '</tr>';
        memHtml += '<tr>' + pair('Requests Processed', d.server.session.toLocaleString()) + sep + pair('Heap Memory',     fmtBytes(d.current))        + sep + pair('Stack Memory',        fmtBytes(d.stack))   + '</tr>';
        memHtml += '<tr>' + pair('GC Cycles',          d.gc.toLocaleString())             + sep + pair('Goroutines',      goroutines.toLocaleString()) + sep + emptyPair()                                       + '</tr>';
        memHtml += '</tbody></table>';
        memContainer.innerHTML = memHtml;

        // Cache Status — 3-column compact grid
        let cacheHtml = '<table class="stat-grid"><thead><tr><th colspan="8">Cache Status</th></tr></thead><tbody>';
        cacheHtml += '<tr>' + pair('DSN Entries',         d.dsnCount)                    + sep + pair('Cached Services',    d.serviceCount)               + sep + pair('Authorizations',  d.authorizationCount) + '</tr>';
        cacheHtml += '<tr>' + pair('Schema Entries',      d.schemaCount)                 + sep + pair('Service Cache Size', d.serviceSize + '&nbsp;items') + sep + pair('Tokens',          d.tokenCount)         + '</tr>';
        cacheHtml += '<tr>' + pair('Code Run Sessions',   d.runCount)                    + sep + pair('Cached Assets',      d.assetCount)                 + sep + pair('Blacklist Status', d.blacklistCount)     + '</tr>';
        cacheHtml += '<tr>' + pair('Code Debug Sessions', d.debugCount)                  + sep + pair('Asset Cache size',   fmtBytes(d.assetSize))        + sep + pair('DB Pool Count',    d.dbPoolCount || 0)   + '</tr>';
        cacheHtml += '</tbody></table>';

        const items = d.items || [];
        if (items.length > 0) {
            cacheHtml += '<hr class="stat-divider"><table><thead><tr>'
                       + '<th>Cached Endpoints</th><th>Class</th><th>Reuse count</th><th class="status-val">Size</th><th>Last accessed</th>'
                       + '</tr></thead><tbody>';
            for (const item of items) {
                // dbpool items report open/in-use connection counts rather than a
                // last-accessed time, so LastUsed is left at its Go zero value
                // (serializes as "0001-01-01T00:00:00Z") -- skip it rather than
                // showing that as a bogus date.
                const lastStr = (item.class !== 'dbpool' && item.last) ? new Date(item.last).toLocaleString() : '';
                const sizeStr = item.class === 'asset'  ? fmtBytes(item.size || 0) :
                                item.class === 'dbpool' ? item.size + '&nbsp;in use' : '';
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
        const res   = await apiFetch('admin/users');
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

// JS port of egostrings.Gibberish() (internal/util/strings/gibberish.go) —
// converts a UUID into a compact, human-friendly base-32 string using the
// same alphabet and digit order as the Go implementation, so client- and
// server-generated IDs look identical in style.
//
// Algorithm: split the UUID's 16 bytes into two 64-bit integers (hi from
// bytes 0-7, low from bytes 8-15), then base-32-encode each — low's digits
// first, followed by hi's — using the alphabet below.
//
// A note on BigInt for anyone unfamiliar with it: ordinary JS numbers are
// 64-bit *floating point* values, which can only represent whole numbers
// exactly up to 2^53 — well short of the 64-bit *integers* this function
// needs to build from raw UUID bytes. BigInt is a second, separate numeric
// type built for exactly this: arbitrary-size whole numbers with no
// precision loss. A trailing lowercase "n" on a number literal (e.g. `0n`,
// `8n`) marks it as a BigInt rather than a regular number — and BigInt
// values can only be mixed with *other* BigInt values in arithmetic (you
// cannot write `1n + 1`, only `1n + 1n`), which is why bytes pulled from the
// `bytes` array below are explicitly wrapped in `BigInt(...)` before use.
function gibberishFromUuid(uuidStr) {
    const digits = 'abcdefghijkmnpqrstuvwxyz23456789';
    const radix  = BigInt(digits.length); // 32, as a BigInt so it can divide/mod hi and low below

    // Turn "550e8400-e29b-..." into the 16 raw byte values it encodes.
    // First strip the dashes, leaving a 32-character hex string (each byte
    // is 2 hex digits). Then walk it two characters at a time and convert
    // each pair from hex to a plain 0-255 number via parseInt(str, 16) —
    // the second argument tells parseInt to read the string as base 16
    // (hexadecimal) rather than the usual base 10.
    const bytes = [];
    const hex   = uuidStr.replace(/-/g, '');
    for (let i = 0; i < 32; i += 2) bytes.push(parseInt(hex.substr(i, 2), 16));

    // Pack the first 8 bytes into "hi" and the last 8 into "low", one byte
    // at a time. `<< 8n` shifts the accumulated value left by one byte
    // (8 bits) to make room, then `+ BigInt(bytes[i])` drops the next byte
    // into the space that opened up at the bottom — the standard way to
    // assemble a multi-byte integer from individual bytes, most-significant
    // byte first.
    let hi = 0n, low = 0n;
    for (let i = 0; i < 8; i++)  hi  = (hi  << 8n) + BigInt(bytes[i]);
    for (let i = 8; i < 16; i++) low = (low << 8n) + BigInt(bytes[i]);

    // Convert low, then hi, to base 32: repeatedly take the remainder after
    // dividing by 32 (that's the next "digit", 0-31) to look up a character
    // in `digits`, then integer-divide by 32 to drop that digit and repeat.
    // This naturally produces the digits in least-significant-first order.
    // Number(...) is needed to index into the `digits` string because a
    // BigInt can't be used directly as an array/string index — only a
    // regular number can — but it's always safe here since the remainder
    // is guaranteed to be less than 32.
    let result = '';
    while (low > 0n) {
        result += digits[Number(low % radix)];
        low = low / radix; // BigInt division truncates automatically, like Math.floor
    }
    while (hi > 0n) {
        result += digits[Number(hi % radix)];
        hi = hi / radix;
    }

    // A UUID of all zero bytes produces no digits at all (both loops above
    // exit immediately), so `result` would be "" — an empty string is falsy
    // in JS, so `result || '-nil-'` substitutes the fallback text in that
    // one case and returns `result` unchanged in every other case.
    return result || '-nil-';
}

// Generate a new gibberish-encoded row ID, the same style of value the
// server assigns to the "_row_id_" column (see defs.RowIDName). Used by the
// SQL Build wizard's INSERT form to pre-fill that column when it is present
// on the target table, since raw INSERT statements bypass the server's own
// automatic row-ID assignment (see internal/server/tables/scripting/insert.go).
//
// crypto.randomUUID() is a built-in browser function (no library needed)
// that returns a fresh, randomly-generated UUID string like
// "550e8400-e29b-41d4-a716-446655440000" each time it's called.
function generateRowId() {
    return gibberishFromUuid(crypto.randomUUID());
}

// The schema a PostgreSQL DSN actually uses once opened, when its own
// "schema" field was left blank at creation time (see database.Open in
// internal/server/tables/database/open.go). SQLite has no schema concept,
// so an empty schema there is just empty, not defaulted.
const DSN_DEFAULT_SCHEMA = 'public';

// Returns the schema value to display for a DSN: the DSN's own configured
// schema if set, the effective default for a schema-less PostgreSQL DSN
// (annotated so it's clear this wasn't explicitly configured), or blank for
// a provider with no schema concept.
function dsnSchemaDisplay(d) {
    if (d.schema) return d.schema;
    if (d.provider === 'postgres') return DSN_DEFAULT_SCHEMA + ' (default)';

    return '';
}

// Load the DSNs tab — fetches the data source name list and renders it as
// a table with columns for connection details.
async function loadDsns() {
    const container = document.getElementById('dsns-content');
    try {
        const res  = await apiFetch('dsns');
        const data = await res.json();
        const dsns = data.items || [];

        if (dsns.length === 0) {
            container.innerHTML = '<p style="padding:1rem;color:#666;">No DSNs found.</p>';
            return;
        }

        let html = '<table><thead><tr>'
                 + '<th>Name</th><th>Provider</th><th>Database</th><th>Schema</th>'
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
                  + '<td>' + escapeHtml(dsnSchemaDisplay(d)) + '</td>'
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

    // Managing per-user DSN permissions requires ego.dsn.admin (or
    // ego.root) -- the server rejects both the @permissions lookup and the
    // add/edit/delete calls with a permission error for anyone else, so
    // don't even fetch the list, and hide the buttons that would only lead
    // to a failed request.
    const deleteBtn  = document.getElementById('dsn-detail-delete-btn');
    const addPermBtn = document.getElementById('dsn-detail-add-perm-btn');
    if (deleteBtn)  deleteBtn.style.display  = _isDsnAdmin ? '' : 'none';
    if (addPermBtn) addPermBtn.style.display = _isDsnAdmin ? '' : 'none';

    try {
        // Fetch the full DSN list to find the details for this DSN.
        const res  = await apiFetch('dsns');
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
            schema:     'Schema',
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
            if (key === 'schema') val = dsnSchemaDisplay(dsnObj);
            html += '<tr><td>' + label + '</td><td>' + escapeHtml(String(val)) + '</td></tr>';
        }
        html += '</tbody></table>';
        content.innerHTML = html;

        // If the DSN is restricted, fetch and display permissions -- but only
        // for a user allowed to see them; the endpoint itself requires
        // ego.dsn.admin (or ego.root) and would otherwise just 403.
        if (dsnObj.restricted && _isDsnAdmin) {
            permSec.style.display = '';
            permCont.innerHTML = '<p style="color:#666;font-size:0.85rem;">Loading permissions\u2026</p>';
            try {
                const pRes  = await apiFetch('dsns/' + encodeURIComponent(name) + '/@permissions');
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
        const res = await fetch('dsns/' + encodeURIComponent(_dsnDetailName), {
            method:  'DELETE',
            headers: { 'Authorization': token ? 'Bearer ' + token : '' },
        });

        if (!res.ok) {
            if (res.status === 401) {
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
        const res = await fetch('dsns/@permissions', {
            method:  'POST',
            headers: {
                'Content-Type':  'application/json',
                'Authorization': token ? 'Bearer ' + token : '',
            },
            body: JSON.stringify({ dsn: _dsnDetailName, user, actions }),
        });

        if (res.status === 401) {
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
        const res = await fetch('dsns/@permissions', {
            method:  'POST',
            headers: {
                'Content-Type':  'application/json',
                'Authorization': token ? 'Bearer ' + token : '',
            },
            body: JSON.stringify({ dsn: _dsnDetailName, user, actions }),
        });

        if (res.status === 401) {
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
        const res = await fetch('dsns/@permissions', {
            method:  'POST',
            headers: {
                'Content-Type':  'application/json',
                'Authorization': token ? 'Bearer ' + token : '',
            },
            body: JSON.stringify({ dsn: _dsnDetailName, user, actions: perms.map(p => '+' + p) }),
        });

        if (res.status === 401) {
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
        const res = await fetch('admin/caches/', {
            method:  'DELETE',
            headers: token ? { 'Authorization': 'Bearer ' + token } : {},
        });

        if (res.status === 401) {
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
// Tasks tab
//
// Scheduled server tasks (see docs/internals/TASKS.md and the /admin/tasks
// REST endpoints in internal/server/tasks/routes.go). The tab itself is only
// shown to an ego.root user when the server has ego.server.tasks.enabled set
// -- see tabPermitted() and loadTasksFeatureFlag() in dashboard-core.js.
// ==========================================================================

// Format an RFC3339 timestamp as a locale date/time string, or show a dash
// for an absent value or Go's zero time (year 0001) -- same convention
// loadUsers() uses for the "Last Login" column.
function fmtTaskTime(raw) {
    return raw && !String(raw).startsWith('0001') ? new Date(raw).toLocaleString() : '—';
}

// Load the Tasks tab -- fetches the scheduled task list and renders it as a
// concise table (description, active flag, last run, last success). Clicking
// a row opens the task detail sheet.
async function loadTasks() {
    const container = document.getElementById('tasks-content');
    try {
        const res   = await apiFetch('admin/tasks');
        const data  = await res.json();
        const tasks = data.items || [];

        if (tasks.length === 0) {
            container.innerHTML = '<p style="padding:1rem;color:#666;">No tasks found.</p>';
            return;
        }

        let html = '<table><thead><tr>'
                 + '<th>Task</th><th>Active</th><th>Last Run</th><th>Last Success</th>'
                 + '</tr></thead><tbody>';

        for (const t of tasks) {
            const safeId   = escapeHtml(t.id);
            const safeTask = escapeHtml(t.task);
            // A task that has never run has no success/failure to report at
            // all -- show a dash rather than "No", which would misleadingly
            // imply it actually ran and failed.
            const success = t.lastRun && !String(t.lastRun).startsWith('0001')
                ? (t.success ? 'Yes' : 'No')
                : '—';

            // Each row is clickable: clicking opens the task detail sheet.
            html += '<tr class="dsn-row" onclick="showTaskDetail(\'' + safeId + '\')"'
                  + ' title="Click to view details for ' + safeTask + '">'
                  + '<td>' + safeTask                   + '</td>'
                  + '<td>' + (t.active ? 'Yes' : 'No')  + '</td>'
                  + '<td>' + fmtTaskTime(t.lastRun)      + '</td>'
                  + '<td>' + success                     + '</td>'
                  + '</tr>';
        }

        html += '</tbody></table>';
        container.innerHTML = html;
    } catch (e) {
        if (e.message !== 'Unauthorized') console.error('Error loading tasks:', e);
    }
}

// POST /admin/tasks/@reload to re-scan the tasks directory for added,
// edited, or removed task files (see ReloadTasksHandler), then refresh the
// list to show the result -- no server restart required.
async function reloadTasks() {
    const btn = document.querySelector('[onclick="reloadTasks()"]');
    if (btn) btn.disabled = true;

    try {
        const token = getToken();
        const res = await fetch('admin/tasks/@reload', {
            method:  'POST',
            headers: token ? { 'Authorization': 'Bearer ' + token } : {},
        });

        if (res.status === 401) {
            clearToken();
            showLogin('Session expired. Please sign in again.');
            return;
        }

        if (!res.ok) {
            const data = await res.json().catch(() => ({}));
            alert(data.msg || 'Failed to reload tasks (HTTP ' + res.status + ').');
            return;
        }

        loadTasks();
    } catch (e) {
        alert('Network error: ' + e.message);
    } finally {
        if (btn) btn.disabled = false;
    }
}

// Task ID currently shown in the task detail sheet.
let _taskDetailId = '';

// The defs.TaskDetailResponse last loaded into the sheet (from GET or a
// successful PATCH). Kept around so submitTaskPatch() can diff the editable
// fields' current values against it to build a minimal PATCH body -- only
// send the fields that actually changed, not all four every time.
let _taskDetailOriginal = null;

// Open the task detail sheet for the given task ID, via the single-item
// GET /admin/tasks/{id} (defs.TaskDetailResponse), which reports the task's
// full definition plus its current execution state -- unlike the list
// endpoint's abbreviated per-task summary.
async function showTaskDetail(id) {
    _taskDetailId = id;

    const overlay = document.getElementById('task-detail-overlay');
    const content = document.getElementById('task-detail-content');

    document.getElementById('task-detail-error').textContent   = '';
    document.getElementById('task-detail-message').textContent = '';
    content.innerHTML = '<p style="color:#666;font-size:0.85rem;">Loading…</p>';
    document.getElementById('task-detail-save-btn').disabled = true;
    overlay.style.display = 'flex';

    try {
        const res  = await apiFetch('admin/tasks/' + encodeURIComponent(id));
        const data = await res.json();

        if (!res.ok) {
            document.getElementById('task-detail-error').textContent =
                data.msg || 'Failed to load task details (HTTP ' + res.status + ').';
            content.innerHTML = '';
            return;
        }

        _taskDetailOriginal = data;
        populateTaskDetailFields(data);

        // Baseline is captured only after every field (editable inputs and
        // the read-only content table) is in place, so a later edit is what
        // isSheetModified() compares against -- see captureBaseline's own
        // doc comment in dashboard-core.js.
        captureBaseline('task-detail-overlay');
    } catch (e) {
        if (e.message !== 'Unauthorized') {
            document.getElementById('task-detail-error').textContent = 'Network error: ' + e.message;
            content.innerHTML = '';
        }
    }
}

// Render one defs.TaskDetailResponse into the sheet: the Active/Interval/
// Count/After editable inputs, plus every other field as a read-only
// two-column table in #task-detail-content (matching showDsnDetail()'s
// attribute-table layout). Shared by showTaskDetail() (initial load) and
// submitTaskPatch()'s success path (re-render with the server's own,
// possibly-normalized values).
function populateTaskDetailFields(task) {
    document.getElementById('task-detail-title').textContent = task.task || task.id;

    document.getElementById('task-detail-active').checked = !!task.active;
    document.getElementById('task-detail-interval').value  = task.interval || '';
    document.getElementById('task-detail-count').value     = task.count || 0;
    document.getElementById('task-detail-after').value     = task.after || '';

    // A task that has never run has no success/failure to report at all --
    // show a dash rather than "No", which would misleadingly imply it
    // actually ran and failed.
    const success = task.lastRun && !String(task.lastRun).startsWith('0001')
        ? (task.success ? 'Yes' : 'No')
        : '—';

    const parameters = task.parameters && Object.keys(task.parameters).length
        ? JSON.stringify(task.parameters) : '—';
    const save = task.save && Object.keys(task.save).length
        ? JSON.stringify(task.save) : '—';
    const tests = (task.tests && task.tests.length)
        ? task.tests.map(chk => chk.name + ' (' + chk.query + ' ' + (chk.op || 'eq') + ' ' + chk.value + ')').join('; ')
        : '—';

    const rows = [
        ['ID',              task.id],
        ['User',            task.user],
        ['Method',          task.method],
        ['Endpoint',        task.endpoint],
        ['Parameters',      parameters],
        ['Body',            task.body != null ? JSON.stringify(task.body) : '—'],
        ['Expected Status', task.expectedStatus || '—'],
        ['Save',            save],
        ['Tests',           tests],
        ['Timeout',         task.timeout || '—'],
        ['Path',            task.path || '—'],
        ['Running',         task.running ? 'Yes' : 'No'],
        ['Last Run',        fmtTaskTime(task.lastRun)],
        ['Last Status',     task.lastStatus || '—'],
        ['Success',         success],
        ['Run Count',       task.runCount || 0],
        ['Failed Test',     task.failedTest || '—'],
        ['Loaded At',       fmtTaskTime(task.loadedAt)],
    ];

    document.getElementById('task-detail-content').innerHTML =
        '<table><thead><tr><th>Attribute</th><th>Value</th></tr></thead><tbody>'
        + rows.map(([label, val]) => '<tr><td>' + label + '</td><td>' + escapeHtml(String(val)) + '</td></tr>').join('')
        + '</tbody></table>';

    // Disabling an already-inactive task is a no-op the server would just
    // accept silently -- disable the button instead so the sheet reflects
    // the task's actual state.
    document.getElementById('task-detail-disable-btn').disabled = !task.active;
}

// Enable the Save button only once one of the editable fields actually
// diverges from the sheet's baseline (captured when it was opened, or last
// re-captured after a successful save) -- registered once, below, as an
// input/change listener on all four editable fields.
function updateTaskDetailSaveBtn() {
    document.getElementById('task-detail-save-btn').disabled = !isSheetModified('task-detail-overlay');
}

['task-detail-interval', 'task-detail-count', 'task-detail-after'].forEach(id => {
    document.getElementById(id).addEventListener('input', updateTaskDetailSaveBtn);
});
document.getElementById('task-detail-active').addEventListener('change', updateTaskDetailSaveBtn);

// Close the task detail sheet -- called directly by the Close button, and
// (via _overlayDismiss in dashboard-core.js) by a backdrop click. Unlike
// most other sheets' Close/Cancel buttons, this one asks for confirmation
// itself when there are unsaved edits, so both dismissal paths get the same
// "discard changes?" protection without prompting twice. Pass force=true to
// skip that prompt -- used when the session itself just ended (401) and the
// sheet needs to close unconditionally, not on a user-initiated dismissal.
function hideTaskDetail(force) {
    if (!force && isSheetModified('task-detail-overlay')) {
        if (!confirm('Discard unsaved changes to this task?')) return;
    }

    document.getElementById('task-detail-overlay').style.display = 'none';
}

// POST /admin/tasks/{id} to run the task immediately. The run happens
// asynchronously on the server (202 Accepted, see RunTaskHandler), so this
// doesn't wait for it to finish -- it confirms the request was accepted and
// refreshes the list tab in the background; the detail sheet itself is
// re-fetched so its Last Run/Last Status fields catch up once the run
// completes and the admin reopens or re-clicks into it.
async function submitRunTask() {
    const btn = document.getElementById('task-detail-run-btn');
    btn.disabled = true;
    document.getElementById('task-detail-error').textContent   = '';
    document.getElementById('task-detail-message').textContent = '';

    try {
        const token = getToken();
        const res = await fetch('admin/tasks/' + encodeURIComponent(_taskDetailId), {
            method:  'POST',
            headers: token ? { 'Authorization': 'Bearer ' + token } : {},
        });

        if (res.status === 401) {
            clearToken();
            hideTaskDetail(true); // force -- session is gone, nothing to discard-prompt for
            showLogin('Session expired. Please sign in again.');
            return;
        }

        if (!res.ok) {
            const data = await res.json().catch(() => ({}));
            document.getElementById('task-detail-error').textContent =
                data.msg || 'Failed to start task (HTTP ' + res.status + ').';
            return;
        }

        document.getElementById('task-detail-message').textContent = 'Task started.';
        loadTasks();
    } catch (e) {
        document.getElementById('task-detail-error').textContent = 'Network error. Please try again.';
    } finally {
        btn.disabled = false;
    }
}

// DELETE /admin/tasks/{id} to deactivate the task (it stays loaded and
// reportable, just inactive -- see DeleteTaskHandler in
// internal/server/tasks/routes.go), then refresh both the sheet and the list.
async function submitDisableTask() {
    if (!confirm('Disable task "' + _taskDetailId + '"? The scheduler will stop running it until it is re-enabled by editing its task file and reloading.')) return;

    const btn = document.getElementById('task-detail-disable-btn');
    btn.disabled = true;
    document.getElementById('task-detail-error').textContent   = '';
    document.getElementById('task-detail-message').textContent = '';

    try {
        const token = getToken();
        const res = await fetch('admin/tasks/' + encodeURIComponent(_taskDetailId), {
            method:  'DELETE',
            headers: token ? { 'Authorization': 'Bearer ' + token } : {},
        });

        if (res.status === 401) {
            clearToken();
            hideTaskDetail(true); // force -- session is gone, nothing to discard-prompt for
            showLogin('Session expired. Please sign in again.');
            return;
        }

        if (!res.ok) {
            const data = await res.json().catch(() => ({}));
            document.getElementById('task-detail-error').textContent =
                data.msg || 'Failed to disable task (HTTP ' + res.status + ').';
            return;
        }

        loadTasks();
        showTaskDetail(_taskDetailId);
    } catch (e) {
        document.getElementById('task-detail-error').textContent = 'Network error. Please try again.';
    } finally {
        btn.disabled = false;
    }
}

// PATCH /admin/tasks/{id} with only the editable fields that actually
// changed from _taskDetailOriginal (Active/Interval/Count/After -- the only
// fields the endpoint accepts; see defs.TaskPatchRequest). On success the
// sheet is re-rendered from the server's own response (in case anything was
// normalized) and a fresh baseline is captured, so the Save button and the
// close-with-unsaved-changes prompt (hideTaskDetail()) both go quiet again
// until the next edit.
async function submitTaskPatch() {
    const btn = document.getElementById('task-detail-save-btn');
    btn.disabled = true;
    document.getElementById('task-detail-error').textContent   = '';
    document.getElementById('task-detail-message').textContent = '';

    const active   = document.getElementById('task-detail-active').checked;
    const interval = document.getElementById('task-detail-interval').value.trim();
    const after    = document.getElementById('task-detail-after').value.trim();
    const countRaw = document.getElementById('task-detail-count').value.trim();
    const count    = countRaw === '' ? 0 : parseInt(countRaw, 10);

    if (!Number.isInteger(count) || count < 0) {
        document.getElementById('task-detail-error').textContent = 'Count must be a non-negative whole number.';
        updateTaskDetailSaveBtn();
        return;
    }

    // Only include fields that actually differ from the values the sheet
    // was opened (or last saved) with -- keeps the on-disk task file's diff
    // minimal and matches defs.TaskPatchRequest's "omitted means unchanged"
    // semantics on the server.
    const orig = _taskDetailOriginal || {};
    const body = {};

    if (active !== !!orig.active)          body.active   = active;
    if (interval !== (orig.interval || '')) body.interval = interval;
    if (count !== (orig.count || 0))        body.count    = count;
    if (after !== (orig.after || ''))       body.after    = after;

    if (Object.keys(body).length === 0) {
        updateTaskDetailSaveBtn();
        return;
    }

    try {
        const token = getToken();
        const res = await fetch('admin/tasks/' + encodeURIComponent(_taskDetailId), {
            method:  'PATCH',
            headers: {
                'Content-Type':  'application/json',
                'Authorization': token ? 'Bearer ' + token : '',
            },
            body: JSON.stringify(body),
        });

        if (res.status === 401) {
            clearToken();
            hideTaskDetail(true); // force -- session is gone, nothing to discard-prompt for
            showLogin('Session expired. Please sign in again.');
            return;
        }

        const data = await res.json().catch(() => ({}));

        if (!res.ok) {
            document.getElementById('task-detail-error').textContent =
                data.msg || 'Failed to save changes (HTTP ' + res.status + ').';
            updateTaskDetailSaveBtn();
            return;
        }

        _taskDetailOriginal = data;
        populateTaskDetailFields(data);
        captureBaseline('task-detail-overlay');
        updateTaskDetailSaveBtn();
        document.getElementById('task-detail-message').textContent = 'Changes saved.';
        loadTasks();
    } catch (e) {
        document.getElementById('task-detail-error').textContent = 'Network error. Please try again.';
        updateTaskDetailSaveBtn();
    }
}

// ==========================================================================
// Configuration sheet
// ==========================================================================

// Formats the elapsed time since the server started as a compact duration
// string like "3h 4m 22s", shown alongside the absolute start time in the
// Configuration sheet's Server Info block. Leading zero units are omitted
// (an uptime under an hour reads "4m 22s", not "0h 4m 22s"), but seconds are
// always shown.
function fmtServerUptime(since) {
    const start = new Date(since);
    if (isNaN(start.getTime())) return '';

    const totalSecs = Math.max(0, Math.floor((Date.now() - start.getTime()) / 1000));
    const days  = Math.floor(totalSecs / 86400);
    const hours = Math.floor((totalSecs % 86400) / 3600);
    const mins  = Math.floor((totalSecs % 3600) / 60);
    const secs  = totalSecs % 60;

    const parts = [];
    if (days > 0)                          parts.push(days  + 'd');
    if (days > 0 || hours > 0)             parts.push(hours + 'h');
    if (days > 0 || hours > 0 || mins > 0) parts.push(mins  + 'm');
    parts.push(secs + 's');

    return parts.join(' ');
}

// Populated by showConfigSheet() and read by showConfigItemDetail() below --
// keyed by setting name, each value is {value, description} as returned by
// GET /admin/config's items map.
let _configItems = {};

// Fetch GET /admin/config and display all server configuration items in a
// read-only sheet. Keys are sorted alphabetically for easy scanning. Each
// item now includes a localized description (see defs.ConfigItem
// server-side); every row is clickable and opens a detail popup showing it
// (see showConfigItemDetail() below).
async function showConfigSheet() {
    const content = document.getElementById('config-content');
    const errorEl = document.getElementById('config-error');

    errorEl.textContent = '';
    content.innerHTML = '<p style="color:#666;font-size:0.85rem;">Loading\u2026</p>';
    document.getElementById('config-overlay').style.display = 'flex';

    // Host name, version, UUID, and start time \u2014 cached in globals by
    // loadServerInfo() at page load \u2014 sit above the Setting/Value table
    // rather than needing their own tab or API call. They render immediately;
    // the host-machine rows below (platform, CPU, memory) come from a second,
    // slightly slower request and are appended once it resolves rather than
    // delaying this table.
    document.getElementById('config-server-info').innerHTML =
        '<table id="config-server-table">' +
        '<tr><td>Host Name</td><td>'   + escapeHtml(_serverHostName || '') + '</td></tr>' +
        '<tr><td>Ego Version</td><td>' + escapeHtml(_serverVersion ? 'v' + _serverVersion : '') + '</td></tr>' +
        '<tr><td>Server UUID</td><td>' + escapeHtml(_serverId || '') + '</td></tr>' +
        '<tr><td>Started</td><td>'     + (_serverStartTime ? escapeHtml(_serverStartTime) + ' (up ' + fmtServerUptime(_serverStartTime) + ')' : '') + '</td></tr>' +
        '</table>';

    // Host machine info (CPU, memory, OS) -- a "GET /admin/serverinfo" call
    // separate from the cached globals above. Best-effort: if it fails (for
    // instance a non-admin caller, though this sheet is admin-only already,
    // or a host that blocks the underlying OS query) the sheet still works
    // fine without these rows, so errors are swallowed rather than shown.
    try {
        const hostRes  = await apiFetch('admin/serverinfo');
        const hostData = await hostRes.json();

        if (hostRes.ok) {
            // gopsutil reports the OS family as "darwin"; "macOS" is what
            // users actually call it, so substitute it here for display only
            // -- every other field is shown exactly as the server reports it.
            const platformName = hostData.os === 'darwin' ? 'macOS' : (hostData.platform || hostData.os);
            const platformLabel = [platformName, hostData.platformVersion].filter(Boolean).join(' ');

            document.getElementById('config-server-table').insertAdjacentHTML('beforeend',
                '<tr><td>Platform</td><td>'         + escapeHtml(platformLabel) + '</td></tr>' +
                '<tr><td>Architecture</td><td>'      + escapeHtml(hostData.architecture || '') + '</td></tr>' +
                '<tr><td>CPU Cores</td><td>'         + escapeHtml(String(hostData.cpuCores || '')) + '</td></tr>' +
                '<tr><td>Total Memory</td><td>'      + fmtBytes(hostData.totalMemory || 0) + '</td></tr>' +
                '<tr><td>Available Memory</td><td>'  + fmtBytes(hostData.availableMemory || 0) + '</td></tr>');
        }
    } catch (e) {
        console.error('Could not load host info:', e);
    }

    try {
        const res  = await apiFetch('admin/config');
        const data = await res.json();

        if (!res.ok) {
            errorEl.textContent = data.message || 'Failed to load configuration.';
            content.innerHTML = '';
            return;
        }

        _configItems = data.items || {};
        const keys = Object.keys(_configItems).sort();

        if (keys.length === 0) {
            content.innerHTML = '<p style="color:#666;font-size:0.85rem;">No configuration items found.</p>';
            return;
        }

        // Build a two-column table: Setting | Value.
        // escapeHtml() prevents any HTML characters in keys or values (e.g. < > &)
        // from being interpreted as markup — important for path values on Windows.
        // data-key (not the value/description themselves) is all that goes in
        // the DOM; showConfigItemDetail() looks both up from _configItems.
        const rows = keys.map(k => {
            const item = _configItems[k];

            return `<tr class="config-row" data-key="${escapeHtml(k)}"><td>${escapeHtml(k)}</td><td>${escapeHtml(item.value)}</td></tr>`;
        }).join('');

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

// ---------------------------------------------------------------------------
// Config item detail popup
//
// Clicking (or tapping -- there's no separate touch case to handle, a click
// is a click) a row in #config-content opens a small centered dialog showing
// that setting's full name, current value, and description. The click
// listener is delegated onto the static #config-content container --
// registered once here at load time -- rather than attached to individual
// rows, since showConfigSheet() replaces the table's innerHTML every time
// the sheet is opened.
// ---------------------------------------------------------------------------

// Original value of the item currently shown in the popup, used by
// updateConfigItemSaveBtn() to detect whether the value input actually
// differs from the server's current value.
let _configItemOriginalValue = '';

// Set when the most recent submitConfigItemEdit() attempt failed. While
// true, updateConfigItemSaveBtn() keeps Save enabled even if the input has
// been edited back to _configItemOriginalValue -- otherwise a user who
// mistypes a value, sees the PATCH fail, and then retypes the original
// value would find Save disabled again with the error message still on
// screen and no way to re-submit short of closing and reopening the popup.
let _configItemSaveFailed = false;

// Whether the item currently shown in the popup may be edited at all: its
// "readonly" flag (set server-side, see defs.ConfigItem) must be false AND
// the logged-in user must hold ego.root (_isAdmin) -- ego.server.admin is
// not sufficient, matching PATCH /admin/config's own Permissions(RootPermission)
// gate in internal/commands/routes.go. Read by startConfigItemEdit() to guard
// against the pencil button somehow firing while it should be hidden.
let _configItemEditable = false;

// Look up key in _configItems and populate/show the detail popup. Falls back
// to a placeholder message when the setting has no registered description.
// Always opens in the read-only display state (value shown as a span, not an
// input) -- startConfigItemEdit() below switches it into edit mode. The
// pencil button is only shown when _configItemEditable ends up true.
function showConfigItemDetail(key) {
    const item = _configItems[key];
    if (!item) return;

    document.getElementById('config-item-key').textContent   = key;
    document.getElementById('config-item-value').textContent = item.value;
    document.getElementById('config-item-desc').textContent  =
        item.description || 'No description available.';
    document.getElementById('config-item-error').textContent = '';

    _configItemEditable = _isAdmin && !item.readonly;
    _configItemOriginalValue = item.value;
    _configItemSaveFailed = false;

    const valueInput = document.getElementById('config-item-value-input');
    valueInput.value = item.value;

    document.getElementById('config-item-value').style.display = '';
    valueInput.style.display = 'none';
    document.getElementById('config-item-edit-btn').style.display = _configItemEditable ? '' : 'none';
    document.getElementById('config-item-save-btn').style.display = 'none';
    document.getElementById('config-item-save-btn').disabled = true;

    document.getElementById('config-item-overlay').style.display = 'flex';
}

// Close the config item detail popup.
function hideConfigItemDetail() {
    document.getElementById('config-item-overlay').style.display = 'none';
}

// Pencil button handler: swap the read-only value span for an editable input
// pre-filled with the current value, and reveal the (still-disabled) Save
// button. Only reachable when _configItemEditable is true, since that's what
// controls the pencil button's own visibility.
function startConfigItemEdit() {
    if (!_configItemEditable) return;

    document.getElementById('config-item-value').style.display = 'none';
    document.getElementById('config-item-edit-btn').style.display = 'none';

    const valueInput = document.getElementById('config-item-value-input');
    valueInput.style.display = '';
    valueInput.focus();
    valueInput.select();

    document.getElementById('config-item-save-btn').style.display = '';
    document.getElementById('config-item-save-btn').disabled = true;
    _configItemSaveFailed = false;
}

// Enable the Save button once the value input actually diverges from the
// value the popup was opened with, or unconditionally after a failed save
// attempt so the user always has a way to re-submit (even a value that's
// been edited back to the original -- see _configItemSaveFailed). Also
// clears any stale error message left over from a previous attempt once
// the user starts correcting the value.
function updateConfigItemSaveBtn() {
    const valueInput = document.getElementById('config-item-value-input');
    document.getElementById('config-item-save-btn').disabled =
        !_configItemSaveFailed && valueInput.value === _configItemOriginalValue;
    document.getElementById('config-item-error').textContent = '';
}

document.getElementById('config-item-value-input').addEventListener('input', updateConfigItemSaveBtn);

// Save the value input via PATCH /admin/config, which takes a JSON object
// mapping the setting name to its new value (see PatchConfigHandler). On
// success, updates the cached item and the corresponding row in the
// still-open Configuration sheet behind this popup, then closes it.
async function submitConfigItemEdit() {
    const key      = document.getElementById('config-item-key').textContent;
    const newValue = document.getElementById('config-item-value-input').value;
    const errorEl  = document.getElementById('config-item-error');

    errorEl.textContent = '';
    document.getElementById('config-item-save-btn').disabled = true;

    try {
        const token = getToken();
        const res = await fetch('admin/config', {
            method:  'PATCH',
            headers: {
                'Content-Type':  'application/json',
                'Authorization': token ? 'Bearer ' + token : '',
            },
            body: JSON.stringify({ [key]: newValue }),
        });

        if (res.status === 401) {
            clearToken();
            hideConfigItemDetail();
            showLogin('Session expired. Please sign in again.');
            return;
        }

        if (!res.ok) {
            const data = await res.json().catch(() => ({}));
            errorEl.textContent = data.msg || 'Failed to save setting (HTTP ' + res.status + ').';
            _configItemSaveFailed = true;
            document.getElementById('config-item-save-btn').disabled = false;
            return;
        }

        if (_configItems[key]) _configItems[key].value = newValue;
        document.querySelectorAll('#config-content tr.config-row').forEach(row => {
            if (row.dataset.key === key) row.children[1].textContent = newValue;
        });

        hideConfigItemDetail();
    } catch (e) {
        errorEl.textContent = 'Network error. Please try again.';
        _configItemSaveFailed = true;
        document.getElementById('config-item-save-btn').disabled = false;
    }
}

document.getElementById('config-content').addEventListener('click', e => {
    const row = e.target.closest('tr.config-row');
    if (row) showConfigItemDetail(row.dataset.key);
});

// Load the Tables tab — populates the DSN picker then fetches the table list
// for the currently selected DSN.
async function loadTables() {
    const picker    = document.getElementById('tables-dsn-picker');
    const container = document.getElementById('tables-content');

    // ---- Populate / refresh the DSN picker ----------------------------------
    // Remember which DSN was selected so we can restore it after a refresh.
    const previousDsn = picker.value;

    try {
        const res  = await apiFetch('dsns');
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
        const res  = await apiFetch('dsns/' + encodeURIComponent(dsn) + '/tables');
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
        if (e.status === 403) {
            container.innerHTML = '<p style="padding:1rem;color:#c0392b;">' + escapeHtml(e.message) + '</p>';
        } else if (e.message !== 'Unauthorized') {
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

    const overlay    = document.getElementById('table-detail-overlay');
    const permSec    = document.getElementById('table-permissions-section');
    const permCont   = document.getElementById('table-permissions-content');
    const addPermBtn = document.getElementById('table-detail-add-perm-btn');

    document.getElementById('table-detail-title').textContent  = tableName;
    document.getElementById('table-detail-error').textContent  = '';
    permSec.style.display    = 'none';
    permCont.innerHTML       = '';
    addPermBtn.style.display = 'none';
    overlay.style.display = 'flex';

    // These run as two independent steps (rather than one function) so that a
    // failure or early-out in the metadata fetch never skips the permissions
    // fetch, and vice versa.
    await loadTableColumns(dsn, tableName);
    await loadTablePermissions(dsn, tableName);
}

// Fetch and render the column metadata table in the table-detail sheet.
async function loadTableColumns(dsn, tableName) {
    const content = document.getElementById('table-detail-content');
    content.innerHTML = '<p style="color:#666;font-size:0.85rem;">Loading\u2026</p>';

    try {
        const res  = await apiFetch('dsns/' + encodeURIComponent(dsn) + '/tables/' + encodeURIComponent(tableName));
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

// If the table's parent DSN is restricted, fetch and display table-level
// permissions -- but only for a DSN admin; the underlying endpoints require
// ego.dsn.admin (or ego.root) and would otherwise just 403.
async function loadTablePermissions(dsn, tableName) {
    if (!_isDsnAdmin) return;

    const permSec    = document.getElementById('table-permissions-section');
    const permCont   = document.getElementById('table-permissions-content');
    const addPermBtn = document.getElementById('table-detail-add-perm-btn');

    try {
        const dRes  = await apiFetch('dsns/' + encodeURIComponent(dsn));
        const dData = await dRes.json();

        if (!dRes.ok || !dData.restricted) return;

        addPermBtn.style.display = '';
        permSec.style.display    = '';
        permCont.innerHTML = '<p style="color:#666;font-size:0.85rem;">Loading permissions\u2026</p>';

        const pRes  = await apiFetch('dsns/' + encodeURIComponent(dsn) + '/tables/' + encodeURIComponent(tableName) + '/@permissions');
        const pData = await pRes.json();

        if (!pRes.ok) {
            permCont.innerHTML = '<p style="color:#c00;">Failed to load permissions.</p>';
            return;
        }

        const items = pData.permissions || [];

        if (items.length === 0) {
            permCont.innerHTML = '<p style="color:#666;font-size:0.85rem;">No permissions defined.</p>';
            return;
        }

        let pHtml = '<table><thead><tr><th>User</th><th>Permissions</th></tr></thead><tbody>';
        for (const item of items) {
            const permArr  = item.permissions || [];
            const permsStr = permArr.join(', ');
            const safeUser = escapeHtml(item.user);
            // Encode current perms into a data attribute for the click handler.
            const dataPerms = escapeHtml(permArr.join(','));
            pHtml += '<tr class="dsn-row" title="Click to edit permissions for ' + safeUser + '"'
                   + ' onclick="showTablePermEdit(\'' + safeUser + '\', \'' + dataPerms + '\')">'
                   + '<td>' + safeUser + '</td>'
                   + '<td>' + escapeHtml(permsStr) + '</td>'
                   + '</tr>';
        }
        pHtml += '</tbody></table>';
        permCont.innerHTML = pHtml;
    } catch (pe) {
        if (pe.message !== 'Unauthorized') {
            permCont.innerHTML = '<p style="color:#c00;">Network error: ' + escapeHtml(pe.message) + '</p>';
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
// Table permission edit sheet
// ==========================================================================

// Original permissions for the user currently being edited (array of strings).
// Stored so submitTablePermEdit() can diff against the new value.
let _tablePermEditOriginal = [];

// Open the permission edit sheet for a specific user on the current table.
// currentPermsStr is a comma-separated string of the existing permissions.
function showTablePermEdit(user, currentPermsStr) {
    _tablePermEditOriginal = currentPermsStr ? currentPermsStr.split(',').map(p => p.trim()).filter(Boolean) : [];

    document.getElementById('table-perm-edit-error').textContent = '';
    document.getElementById('table-perm-edit-user').value  = user;
    document.getElementById('table-perm-edit-perms').value = _tablePermEditOriginal.join(', ');
    document.getElementById('table-perm-edit-save-btn').disabled   = false;
    document.getElementById('table-perm-edit-delete-btn').disabled = false;
    document.getElementById('table-perm-edit-overlay').style.display = 'flex';
    captureBaseline('table-perm-edit-overlay');
    document.getElementById('table-perm-edit-perms').focus();
}

// Close the permission edit sheet without saving.
function hideTablePermEdit() {
    document.getElementById('table-perm-edit-overlay').style.display = 'none';
}

// PUT the diffed permission changes (built by buildPermActions(), shared with
// the DSN permission sheets) to the table's permissions endpoint. Each entry
// is prefixed "+" to grant or "-" to revoke, matching what GrantPermissions
// on the server expects.
async function submitTablePermEdit() {
    const user     = document.getElementById('table-perm-edit-user').value;
    const permsRaw = document.getElementById('table-perm-edit-perms').value;
    const newPerms = permsRaw.split(',').map(p => p.trim()).filter(Boolean);

    const actions = buildPermActions(_tablePermEditOriginal, newPerms);
    if (actions.length === 0) {
        hideTablePermEdit();
        return;
    }

    document.getElementById('table-perm-edit-save-btn').disabled = true;

    try {
        const token = getToken();
        const url = 'dsns/' + encodeURIComponent(_tableDetailDsn) + '/tables/' + encodeURIComponent(_tableDetailTable)
                  + '/permissions?user=' + encodeURIComponent(user);
        const res = await fetch(url, {
            method:  'PUT',
            headers: {
                'Content-Type':  'application/json',
                'Authorization': token ? 'Bearer ' + token : '',
            },
            body: JSON.stringify(actions),
        });

        if (res.status === 401) {
            clearToken();
            hideTablePermEdit();
            hideTableDetail();
            showLogin('Session expired. Please sign in again.');
            return;
        }

        if (!res.ok) {
            const data = await res.json().catch(() => ({}));
            document.getElementById('table-perm-edit-error').textContent =
                data.msg || 'Failed to update permissions (HTTP ' + res.status + ').';
            return;
        }

        // Refresh the table detail sheet to show updated permissions.
        hideTablePermEdit();
        showTableDetail(_tableDetailDsn, _tableDetailTable);
    } catch (e) {
        document.getElementById('table-perm-edit-error').textContent = 'Network error. Please try again.';
    } finally {
        document.getElementById('table-perm-edit-save-btn').disabled = false;
    }
}

// Remove all permissions for the current user on this table.
async function submitDeleteTablePerm() {
    const user = document.getElementById('table-perm-edit-user').value;

    if (!confirm('Remove all permissions for "' + user + '" on table "' + _tableDetailTable + '"?')) return;

    document.getElementById('table-perm-edit-delete-btn').disabled = true;

    try {
        const token = getToken();
        const url = 'dsns/' + encodeURIComponent(_tableDetailDsn) + '/tables/' + encodeURIComponent(_tableDetailTable)
                  + '/permissions?user=' + encodeURIComponent(user);
        const res = await fetch(url, {
            method:  'DELETE',
            headers: {
                'Authorization': token ? 'Bearer ' + token : '',
            },
        });

        if (res.status === 401) {
            clearToken();
            hideTablePermEdit();
            hideTableDetail();
            showLogin('Session expired. Please sign in again.');
            return;
        }

        if (!res.ok) {
            const data = await res.json().catch(() => ({}));
            document.getElementById('table-perm-edit-error').textContent =
                data.msg || 'Failed to delete permissions (HTTP ' + res.status + ').';
            return;
        }

        hideTablePermEdit();
        showTableDetail(_tableDetailDsn, _tableDetailTable);
    } catch (e) {
        document.getElementById('table-perm-edit-error').textContent = 'Network error. Please try again.';
    } finally {
        document.getElementById('table-perm-edit-delete-btn').disabled = false;
    }
}

// ==========================================================================
// Add table permission sheet
// ==========================================================================

// Open the add-permission sheet with blank fields.
function showTablePermAdd() {
    document.getElementById('table-perm-add-error').textContent = '';
    document.getElementById('table-perm-add-user').value  = '';
    document.getElementById('table-perm-add-perms').value = '';
    document.getElementById('table-perm-add-save-btn').disabled = false;
    document.getElementById('table-perm-add-overlay').style.display = 'flex';
    captureBaseline('table-perm-add-overlay');
    document.getElementById('table-perm-add-user').focus();
}

// Close the add-permission sheet without saving.
function hideTablePermAdd() {
    document.getElementById('table-perm-add-overlay').style.display = 'none';
}

// PUT new permissions to the table's permissions endpoint, then refresh the
// table detail sheet.
async function submitTablePermAdd() {
    const user     = document.getElementById('table-perm-add-user').value.trim();
    const permsRaw = document.getElementById('table-perm-add-perms').value;
    const perms    = permsRaw.split(',').map(p => p.trim()).filter(Boolean);

    if (!user) {
        document.getElementById('table-perm-add-error').textContent = 'User is required.';
        return;
    }
    if (perms.length === 0) {
        document.getElementById('table-perm-add-error').textContent = 'At least one permission is required.';
        return;
    }

    document.getElementById('table-perm-add-save-btn').disabled = true;

    try {
        const token = getToken();
        const url = 'dsns/' + encodeURIComponent(_tableDetailDsn) + '/tables/' + encodeURIComponent(_tableDetailTable)
                  + '/permissions?user=' + encodeURIComponent(user);
        const res = await fetch(url, {
            method:  'PUT',
            headers: {
                'Content-Type':  'application/json',
                'Authorization': token ? 'Bearer ' + token : '',
            },
            body: JSON.stringify(perms.map(p => '+' + p)),
        });

        if (res.status === 401) {
            clearToken();
            hideTablePermAdd();
            hideTableDetail();
            showLogin('Session expired. Please sign in again.');
            return;
        }

        if (!res.ok) {
            const data = await res.json().catch(() => ({}));
            document.getElementById('table-perm-add-error').textContent =
                data.msg || 'Failed to add permissions (HTTP ' + res.status + ').';
            return;
        }

        hideTablePermAdd();
        showTableDetail(_tableDetailDsn, _tableDetailTable);
    } catch (e) {
        document.getElementById('table-perm-add-error').textContent = 'Network error. Please try again.';
    } finally {
        document.getElementById('table-perm-add-save-btn').disabled = false;
    }
}

