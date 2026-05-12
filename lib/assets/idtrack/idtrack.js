'use strict';

// =====================================================================
// CONFIGURATION & CONSTANTS
// =====================================================================

const SETUP_KEY   = 'idtrack_setup';    // localStorage: { user, pass, dsn }
const SESSION_KEY = 'idtrack_session';  // sessionStorage: { username, display_name }
const TOKEN_KEY   = 'idtrack_token';    // sessionStorage: ego bearer token
const APP_VERSION = '1.0';

// =====================================================================
// STATE
// =====================================================================

let _token       = null;   // Ego bearer token for backend calls
let _setup       = null;   // { user, pass, dsn } — service account config
let _currentUser = null;   // { username, display_name } — logged-in idtrack user
let _allIssues   = [];     // master list loaded from server
let _currentId   = null;   // currently displayed issue id
let _sortCol     = 'id';
let _sortAsc     = false;
let _statusFilter   = 'open';
let _priorityFilter = 'all';
let _detailDirty = false;

// =====================================================================
// UTILITY
// =====================================================================

function esc(s) {
    return String(s == null ? '' : s)
        .replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;')
        .replace(/"/g,'&quot;');
}

function sqlStr(s) {
    // Escape a value for embedding in a SQL string literal.
    return String(s == null ? '' : s).replace(/'/g, "''");
}

function fmtDate(iso) {
    if (!iso) return '';
    try {
        const d = new Date(iso);
        return d.toLocaleDateString(undefined, { year:'numeric', month:'short', day:'numeric' });
    } catch { return iso; }
}

function fmtDateTime(iso) {
    if (!iso) return '';
    try {
        const d = new Date(iso);
        return d.toLocaleDateString(undefined, {year:'numeric',month:'short',day:'numeric'})
             + ' ' + d.toLocaleTimeString(undefined,{hour:'2-digit',minute:'2-digit'});
    } catch { return iso; }
}

function now() { return new Date().toISOString(); }

async function sha256(text) {
    const buf  = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(text));
    return Array.from(new Uint8Array(buf)).map(b => b.toString(16).padStart(2,'0')).join('');
}

function priorityBadge(p) {
    const cls = {High:'badge-high', Medium:'badge-medium', Low:'badge-low'}[p] || 'badge-low';
    return `<span class="badge ${cls}">${esc(p)}</span>`;
}

function statusBadge(s) {
    const cls = s === 'Open' ? 'badge-open' : 'badge-resolved';
    return `<span class="badge ${cls}">${esc(s)}</span>`;
}

// =====================================================================
// API LAYER
// =====================================================================

async function apiFetch(url, options = {}) {
    if (!options.headers) options.headers = {};
    if (_token) options.headers['Authorization'] = 'Bearer ' + _token;

    const res = await fetch(url, options);

    if (res.status === 401 || res.status === 403) {
        _token = null;
        sessionStorage.removeItem(TOKEN_KEY);
        showSetup('Session expired. Please re-enter your credentials.');
        throw new Error('Unauthorized');
    }
    return res;
}

// Execute one or more SQL statements against the idtrack DSN.
// Returns { rows, count } for SELECT, { count } for DML.
async function apiSql(statements) {
    const dsn = _setup && _setup.dsn ? _setup.dsn : 'idtrack';
    const url  = `/dsns/${encodeURIComponent(dsn)}/tables/@sql`;
    const body = Array.isArray(statements) ? statements : [statements];

    const res = await apiFetch(url, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
    });

    if (!res.ok) {
        let msg = `SQL error (${res.status})`;
        try { const d = await res.json(); msg = d.msg || msg; } catch {}
        throw new Error(msg);
    }

    const ct = res.headers.get('Content-Type') || '';
    if (ct.includes('rows')) {
        return await res.json();   // { rows: [...], count: N }
    }
    // rowcount response
    try { return await res.json(); } catch { return { count: 0 }; }
}

// Log into Ego and store the bearer token.
async function egoLogin(user, pass) {
    const res = await fetch('/services/admin/logon', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username: user, password: pass, source: 'idtrack' }),
    });
    if (!res.ok) {
        let msg = `Login failed (${res.status})`;
        try { const d = await res.json(); msg = d.msg || msg; } catch {}
        throw new Error(msg);
    }
    const data = await res.json();
    _token = data.token || (data.data && data.data.token);
    if (!_token) throw new Error('No token in response');
    sessionStorage.setItem(TOKEN_KEY, _token);
    return data;
}

// =====================================================================
// DATABASE INITIALIZATION
// =====================================================================

async function initTables() {
    await apiSql([
        `CREATE TABLE IF NOT EXISTS idtrack_users (
            username     TEXT PRIMARY KEY,
            display_name TEXT NOT NULL,
            password_hash TEXT NOT NULL,
            created_at   TEXT NOT NULL
        )`,
        `CREATE TABLE IF NOT EXISTS issues (
            id          INTEGER PRIMARY KEY AUTOINCREMENT,
            title       TEXT NOT NULL,
            description TEXT NOT NULL DEFAULT '',
            reporter    TEXT NOT NULL,
            assignee    TEXT NOT NULL DEFAULT '',
            priority    TEXT NOT NULL DEFAULT 'Medium',
            status      TEXT NOT NULL DEFAULT 'Open',
            created_at  TEXT NOT NULL,
            updated_at  TEXT NOT NULL
        )`,
        `CREATE TABLE IF NOT EXISTS comments (
            id          INTEGER PRIMARY KEY AUTOINCREMENT,
            issue_id    INTEGER NOT NULL,
            author      TEXT NOT NULL,
            body        TEXT NOT NULL,
            created_at  TEXT NOT NULL
        )`,
    ]);
}

// =====================================================================
// IDTRACK USER MANAGEMENT
// =====================================================================

async function getIdtrackUsers() {
    const r = await apiSql([`SELECT username, display_name FROM idtrack_users ORDER BY username`]);
    return (r.rows || []);
}

async function countIdtrackUsers() {
    const r = await apiSql([`SELECT COUNT(*) AS cnt FROM idtrack_users`]);
    const rows = r.rows || [];
    if (rows.length > 0) {
        const val = rows[0]['cnt'] || rows[0]['COUNT(*)'] || rows[0]['count(*)'] || 0;
        return Number(val);
    }
    return 0;
}

async function findUser(username) {
    const u = sqlStr(username);
    const r = await apiSql([`SELECT username, display_name, password_hash FROM idtrack_users WHERE username = '${u}'`]);
    const rows = r.rows || [];
    return rows.length > 0 ? rows[0] : null;
}

async function createIdtrackUser(username, displayName, passwordHash) {
    const u = sqlStr(username), d = sqlStr(displayName), h = sqlStr(passwordHash);
    await apiSql([`INSERT INTO idtrack_users (username, display_name, password_hash, created_at) VALUES ('${u}','${d}','${h}','${now()}')`]);
}

// =====================================================================
// ISSUE MANAGEMENT
// =====================================================================

async function fetchIssues() {
    const r = await apiSql([
        `SELECT id, title, description, reporter, assignee, priority, status, created_at, updated_at FROM issues ORDER BY id DESC`
    ]);
    return r.rows || [];
}

async function fetchIssue(id) {
    const r = await apiSql([`SELECT id, title, description, reporter, assignee, priority, status, created_at, updated_at FROM issues WHERE id = ${Number(id)}`]);
    const rows = r.rows || [];
    return rows.length > 0 ? rows[0] : null;
}

async function insertIssue(title, description, priority, assignee) {
    const t = sqlStr(title), desc = sqlStr(description);
    const prio = sqlStr(priority), asn = sqlStr(assignee);
    const rep  = sqlStr(_currentUser.username);
    const ts   = now();
    await apiSql([
        `INSERT INTO issues (title, description, reporter, assignee, priority, status, created_at, updated_at)
         VALUES ('${t}','${desc}','${rep}','${asn}','${prio}','Open','${ts}','${ts}')`
    ]);
}

async function updateIssue(id, title, description, priority, status, assignee) {
    const t = sqlStr(title), desc = sqlStr(description);
    const prio = sqlStr(priority), st = sqlStr(status), asn = sqlStr(assignee);
    const ts = now();
    await apiSql([
        `UPDATE issues SET title='${t}', description='${desc}', priority='${prio}', status='${st}', assignee='${asn}', updated_at='${ts}' WHERE id=${Number(id)}`
    ]);
}

// =====================================================================
// COMMENT MANAGEMENT
// =====================================================================

async function fetchComments(issueId) {
    const r = await apiSql([
        `SELECT id, issue_id, author, body, created_at FROM comments WHERE issue_id = ${Number(issueId)} ORDER BY id ASC`
    ]);
    return r.rows || [];
}

async function insertComment(issueId, body) {
    const b   = sqlStr(body);
    const aut = sqlStr(_currentUser.username);
    const ts  = now();
    await apiSql([
        `INSERT INTO comments (issue_id, author, body, created_at) VALUES (${Number(issueId)},'${aut}','${b}','${ts}')`
    ]);
}

// =====================================================================
// UI — SETUP
// =====================================================================

function showSetup(msg) {
    document.getElementById('app').style.display = 'none';
    document.getElementById('login-overlay').style.display = 'none';
    document.getElementById('setup-error').textContent = msg || '';
    const saved = loadSetupConfig();
    if (saved) {
        document.getElementById('setup-user').value = saved.user || '';
        document.getElementById('setup-pass').value = '';
        document.getElementById('setup-dsn').value  = saved.dsn  || 'idtrack';
    }
    document.getElementById('setup-overlay').style.display = 'flex';
    document.getElementById('setup-user').focus();
}

function hideSetup() {
    document.getElementById('setup-overlay').style.display = 'none';
}

async function submitSetup() {
    const user = document.getElementById('setup-user').value.trim();
    const pass = document.getElementById('setup-pass').value;
    const dsn  = document.getElementById('setup-dsn').value.trim() || 'idtrack';
    const err  = document.getElementById('setup-error');
    const btn  = document.getElementById('setup-submit-btn');

    err.textContent = '';
    if (!user || !pass) { err.textContent = 'Username and password are required.'; return; }

    btn.disabled = true;
    btn.textContent = 'Connecting…';

    try {
        await egoLogin(user, pass);
        _setup = { user, pass, dsn };
        saveSetupConfig(_setup);

        // Ensure the DSN exists.
        await ensureDSN(dsn);

        // Create tables if they don't exist.
        await initTables();

        hideSetup();
        await startLoginFlow();

    } catch (e) {
        err.textContent = e.message || 'Connection failed.';
    } finally {
        btn.disabled = false;
        btn.textContent = 'Connect & Initialize';
    }
}

async function ensureDSN(dsn) {
    try {
        const res = await apiFetch(`/dsns/${encodeURIComponent(dsn)}/`, { method: 'GET' });
        if (res.ok) return; // already exists
    } catch {}

    // Try to create it.
    const res = await apiFetch('/dsns/', {
        method: 'POST',
        headers: { 'Content-Type': 'application/vnd.ego.dsn+json' },
        body: JSON.stringify({ name: dsn, provider: 'sqlite3', database: dsn + '.db', secured: false, restricted: false }),
    });
    if (!res.ok && res.status !== 409) {
        let msg = `Failed to create DSN (${res.status})`;
        try { const d = await res.json(); msg = d.msg || msg; } catch {}
        throw new Error(msg);
    }
}

function loadSetupConfig() {
    try { return JSON.parse(localStorage.getItem(SETUP_KEY)); } catch { return null; }
}

function saveSetupConfig(cfg) {
    localStorage.setItem(SETUP_KEY, JSON.stringify(cfg));
}

// =====================================================================
// UI — LOGIN / REGISTER
// =====================================================================

function showLoginForm() {
    document.getElementById('login-form').style.display = '';
    document.getElementById('register-form').style.display = 'none';
    document.getElementById('login-error').textContent = '';
    document.getElementById('login-user').focus();
}

function showRegisterForm() {
    document.getElementById('login-form').style.display = 'none';
    document.getElementById('register-form').style.display = '';
    document.getElementById('reg-error').textContent = '';
    document.getElementById('reg-user').focus();
}

async function startLoginFlow() {
    document.getElementById('app').style.display = 'none';
    document.getElementById('login-error').textContent = '';
    document.getElementById('login-user').value = '';
    document.getElementById('login-pass').value = '';

    // First-time: if no users yet, send straight to register.
    let count = 0;
    try { count = await countIdtrackUsers(); } catch {}

    if (count === 0) {
        showRegisterForm();
    } else {
        showLoginForm();
    }
    document.getElementById('login-overlay').style.display = 'flex';
}

async function submitLogin() {
    const username = document.getElementById('login-user').value.trim();
    const password = document.getElementById('login-pass').value;
    const err      = document.getElementById('login-error');
    const btn      = document.getElementById('login-submit-btn');

    err.textContent = '';
    if (!username || !password) { err.textContent = 'Username and password are required.'; return; }

    btn.disabled = true;
    btn.textContent = 'Signing in…';

    try {
        const hash = await sha256(password);
        const user = await findUser(username);

        if (!user) {
            err.textContent = 'Unknown username. Please check your credentials.';
            return;
        }
        if (user.password_hash !== hash) {
            err.textContent = 'Incorrect password.';
            return;
        }

        _currentUser = { username: user.username, display_name: user.display_name };
        sessionStorage.setItem(SESSION_KEY, JSON.stringify(_currentUser));

        document.getElementById('login-overlay').style.display = 'none';
        await launchApp();

    } catch (e) {
        if (e.message !== 'Unauthorized') err.textContent = e.message || 'Login failed.';
    } finally {
        btn.disabled = false;
        btn.textContent = 'Sign In';
    }
}

async function submitRegister() {
    const username = document.getElementById('reg-user').value.trim();
    const display  = document.getElementById('reg-display').value.trim();
    const pass1    = document.getElementById('reg-pass').value;
    const pass2    = document.getElementById('reg-pass2').value;
    const err      = document.getElementById('reg-error');
    const btn      = document.getElementById('reg-submit-btn');

    err.textContent = '';
    if (!username) { err.textContent = 'Username is required.'; return; }
    if (!display)  { err.textContent = 'Display name is required.'; return; }
    if (!pass1)    { err.textContent = 'Password is required.'; return; }
    if (pass1 !== pass2) { err.textContent = 'Passwords do not match.'; return; }
    if (username.length < 2 || !/^[a-zA-Z0-9_.-]+$/.test(username)) {
        err.textContent = 'Username may only contain letters, digits, underscore, dot, or hyphen.';
        return;
    }

    btn.disabled = true;
    btn.textContent = 'Creating…';

    try {
        const existing = await findUser(username);
        if (existing) { err.textContent = 'That username is already taken.'; return; }

        const hash = await sha256(pass1);
        await createIdtrackUser(username, display, hash);

        _currentUser = { username, display_name: display };
        sessionStorage.setItem(SESSION_KEY, JSON.stringify(_currentUser));

        document.getElementById('login-overlay').style.display = 'none';
        await launchApp();

    } catch (e) {
        if (e.message !== 'Unauthorized') err.textContent = e.message || 'Registration failed.';
    } finally {
        btn.disabled = false;
        btn.textContent = 'Create Account';
    }
}

// =====================================================================
// UI — MAIN APP
// =====================================================================

async function launchApp() {
    document.getElementById('user-badge').textContent = _currentUser.display_name || _currentUser.username;
    document.getElementById('app').style.display = '';
    await refreshIssues();
    await populateAssigneeDropdowns();
}

function doLogout() {
    _currentUser = null;
    _token = null;
    sessionStorage.removeItem(SESSION_KEY);
    sessionStorage.removeItem(TOKEN_KEY);
    document.getElementById('app').style.display = 'none';
    closeDetail();
    showLoginForm();
    document.getElementById('login-overlay').style.display = 'flex';
}

// =====================================================================
// UI — FILTERS & SORT
// =====================================================================

function setStatusFilter(val, btn) {
    _statusFilter = val;
    document.querySelectorAll('.filter-btn[data-status]').forEach(b => b.classList.remove('active'));
    btn.classList.add('active');
    renderIssues(_allIssues);
}

function setPriorityFilter(val, btn) {
    _priorityFilter = val;
    document.querySelectorAll('.filter-btn[data-priority]').forEach(b => b.classList.remove('active'));
    btn.classList.add('active');
    renderIssues(_allIssues);
}

function applyFilters() {
    renderIssues(_allIssues);
}

function toggleSort(col) {
    if (_sortCol === col) {
        _sortAsc = !_sortAsc;
    } else {
        _sortCol = col;
        _sortAsc = (col === 'id') ? false : true;
    }
    updateSortUI();
    renderIssues(_allIssues);
}

function updateSortUI() {
    document.querySelectorAll('.issues-table th').forEach(th => {
        th.classList.remove('sort-active');
        const arr = th.querySelector('.sort-arrow');
        if (arr) arr.remove();
    });
    const colMap = { id:'col-id', title:'col-title', priority:'col-priority', status:'col-status', reporter:'col-reporter', assignee:'col-assignee', created_at:'col-date' };
    const cls = colMap[_sortCol];
    if (cls) {
        const th = document.querySelector(`.issues-table th.${cls}`);
        if (th) {
            th.classList.add('sort-active');
            const arrow = document.createElement('span');
            arrow.className = 'sort-arrow';
            arrow.textContent = _sortAsc ? ' ▲' : ' ▼';
            th.appendChild(arrow);
        }
    }
}

// =====================================================================
// UI — ISSUES LIST
// =====================================================================

async function refreshIssues() {
    try {
        _allIssues = await fetchIssues();
        renderIssues(_allIssues);
    } catch (e) {
        if (e.message !== 'Unauthorized') console.error('refreshIssues:', e);
    }
}

function filteredAndSorted(issues) {
    const search = (document.getElementById('search-input') || {}).value || '';
    const q = search.toLowerCase();

    let result = issues.filter(issue => {
        if (_statusFilter !== 'all') {
            const wantOpen = _statusFilter === 'open';
            if (wantOpen && issue.status !== 'Open') return false;
            if (!wantOpen && issue.status !== 'Resolved') return false;
        }
        if (_priorityFilter !== 'all' && issue.priority !== _priorityFilter) return false;
        if (q) {
            const haystack = [issue.title, issue.description, issue.reporter, issue.assignee].join(' ').toLowerCase();
            if (!haystack.includes(q)) return false;
        }
        return true;
    });

    const priOrder = { High:0, Medium:1, Low:2 };

    result.sort((a, b) => {
        let va = a[_sortCol], vb = b[_sortCol];
        if (_sortCol === 'priority') { va = priOrder[va] ?? 99; vb = priOrder[vb] ?? 99; }
        if (_sortCol === 'id')       { va = Number(va); vb = Number(vb); }
        if (va < vb) return _sortAsc ? -1 : 1;
        if (va > vb) return _sortAsc ?  1 : -1;
        return 0;
    });

    return result;
}

function renderIssues(issues) {
    const visible = filteredAndSorted(issues);
    const tbody   = document.getElementById('issues-tbody');
    const empty   = document.getElementById('issues-empty');
    const table   = document.getElementById('issues-table');

    if (visible.length === 0) {
        table.style.display = 'none';
        empty.style.display = '';
        return;
    }
    table.style.display = '';
    empty.style.display = 'none';

    tbody.innerHTML = visible.map(issue => {
        const sel = issue.id === _currentId ? ' selected' : '';
        return `<tr class="issue-row${sel}" onclick="selectIssue(${issue.id})">
            <td class="col-id">#${esc(String(issue.id))}</td>
            <td class="col-title issue-title-cell">${esc(issue.title)}</td>
            <td class="col-priority">${priorityBadge(issue.priority)}</td>
            <td class="col-status">${statusBadge(issue.status)}</td>
            <td class="col-reporter">${esc(issue.reporter)}</td>
            <td class="col-assignee">${esc(issue.assignee || '—')}</td>
            <td class="col-date">${fmtDate(issue.created_at)}</td>
        </tr>`;
    }).join('');

    updateSortUI();
}

// =====================================================================
// UI — ISSUE DETAIL
// =====================================================================

async function selectIssue(id) {
    if (_detailDirty) {
        if (!confirm('You have unsaved changes. Discard them?')) return;
    }
    _currentId   = id;
    _detailDirty = false;

    // Highlight selected row.
    document.querySelectorAll('.issue-row').forEach(r => r.classList.remove('selected'));
    const rows = document.querySelectorAll('.issue-row');
    rows.forEach(r => { if (r.onclick && r.onclick.toString().includes(`(${id})`)) r.classList.add('selected'); });

    // Re-render to update selection highlight.
    renderIssues(_allIssues);

    try {
        const issue = await fetchIssue(id);
        if (!issue) return;

        document.getElementById('detail-issue-id').textContent = `Issue #${issue.id}`;
        document.getElementById('detail-title').value   = issue.title    || '';
        document.getElementById('detail-status').value  = issue.status   || 'Open';
        document.getElementById('detail-priority').value= issue.priority || 'Medium';
        document.getElementById('detail-desc').value    = issue.description || '';
        document.getElementById('detail-reporter').textContent = issue.reporter  || '';
        document.getElementById('detail-created').textContent  = fmtDateTime(issue.created_at);
        document.getElementById('detail-updated').textContent  = fmtDateTime(issue.updated_at);
        document.getElementById('detail-error').textContent    = '';

        // Set assignee select.
        const asnSel = document.getElementById('detail-assignee');
        asnSel.value = issue.assignee || '';

        document.getElementById('detail-save-btn').style.display = 'none';
        _detailDirty = false;

        const comments = await fetchComments(id);
        renderComments(comments);

        document.getElementById('comment-input').value = '';
        document.getElementById('detail-panel').style.display = '';

    } catch (e) {
        if (e.message !== 'Unauthorized') console.error('selectIssue:', e);
    }
}

function closeDetail() {
    if (_detailDirty) {
        if (!confirm('You have unsaved changes. Discard them?')) return;
    }
    _currentId   = null;
    _detailDirty = false;
    document.getElementById('detail-panel').style.display = 'none';
    renderIssues(_allIssues);
}

function markDetailDirty() {
    if (!_detailDirty) {
        _detailDirty = true;
        document.getElementById('detail-save-btn').style.display = '';
    }
}

async function saveIssueChanges() {
    if (!_currentId) return;
    const title   = document.getElementById('detail-title').value.trim();
    const status  = document.getElementById('detail-status').value;
    const priority= document.getElementById('detail-priority').value;
    const assignee= document.getElementById('detail-assignee').value;
    const desc    = document.getElementById('detail-desc').value.trim();
    const err     = document.getElementById('detail-error');
    const btn     = document.getElementById('detail-save-btn');

    err.textContent = '';
    if (!title) { err.textContent = 'Title is required.'; return; }

    btn.disabled = true;
    btn.textContent = 'Saving…';

    try {
        await updateIssue(_currentId, title, desc, priority, status, assignee);
        _detailDirty = false;
        btn.style.display = 'none';

        // Update updated_at display.
        document.getElementById('detail-updated').textContent = fmtDateTime(now());

        // Refresh the issues list.
        _allIssues = await fetchIssues();
        renderIssues(_allIssues);

    } catch (e) {
        if (e.message !== 'Unauthorized') err.textContent = e.message || 'Save failed.';
    } finally {
        btn.disabled = false;
        btn.textContent = 'Save Changes';
    }
}

// =====================================================================
// UI — COMMENTS
// =====================================================================

function renderComments(comments) {
    const el = document.getElementById('comments-list');
    if (!comments || comments.length === 0) {
        el.innerHTML = '<p class="comments-empty">No comments yet.</p>';
        return;
    }
    el.innerHTML = comments.map(c => `
        <div class="comment-item">
            <div class="comment-header">
                <span class="comment-author">${esc(c.author)}</span>
                <span class="comment-date">${fmtDateTime(c.created_at)}</span>
            </div>
            <div class="comment-body">${esc(c.body)}</div>
        </div>
    `).join('');
}

async function submitComment() {
    if (!_currentId) return;
    const input = document.getElementById('comment-input');
    const body  = input.value.trim();
    if (!body) return;

    try {
        await insertComment(_currentId, body);
        input.value = '';
        const comments = await fetchComments(_currentId);
        renderComments(comments);
        // Scroll to bottom of comments.
        const el = document.getElementById('comments-list');
        if (el) el.scrollIntoView({ behavior: 'smooth', block: 'end' });
    } catch (e) {
        if (e.message !== 'Unauthorized') alert('Failed to add comment: ' + e.message);
    }
}

// =====================================================================
// UI — NEW ISSUE
// =====================================================================

async function showNewIssue() {
    document.getElementById('ni-title').value = '';
    document.getElementById('ni-priority').value = 'Medium';
    document.getElementById('ni-desc').value = '';
    document.getElementById('ni-error').textContent = '';
    await populateAssigneeDropdowns();
    document.getElementById('new-issue-overlay').style.display = 'flex';
    document.getElementById('ni-title').focus();
}

function hideNewIssue() {
    document.getElementById('new-issue-overlay').style.display = 'none';
}

async function submitNewIssue() {
    const title    = document.getElementById('ni-title').value.trim();
    const priority = document.getElementById('ni-priority').value;
    const assignee = document.getElementById('ni-assignee').value;
    const desc     = document.getElementById('ni-desc').value.trim();
    const err      = document.getElementById('ni-error');
    const btn      = document.getElementById('ni-submit-btn');

    err.textContent = '';
    if (!title) { err.textContent = 'Title is required.'; return; }

    btn.disabled = true;
    btn.textContent = 'Creating…';

    try {
        await insertIssue(title, desc, priority, assignee);
        hideNewIssue();
        _allIssues = await fetchIssues();
        renderIssues(_allIssues);

        // Auto-select the newest issue (highest id).
        if (_allIssues.length > 0) {
            const newest = _allIssues.reduce((a, b) => Number(a.id) > Number(b.id) ? a : b);
            selectIssue(newest.id);
        }
    } catch (e) {
        if (e.message !== 'Unauthorized') err.textContent = e.message || 'Failed to create issue.';
    } finally {
        btn.disabled = false;
        btn.textContent = 'Create Issue';
    }
}

// =====================================================================
// ASSIGNEE DROPDOWNS
// =====================================================================

async function populateAssigneeDropdowns() {
    let users = [];
    try { users = await getIdtrackUsers(); } catch {}

    const options = ['<option value="">(unassigned)</option>']
        .concat(users.map(u => `<option value="${esc(u.username)}">${esc(u.display_name || u.username)}</option>`))
        .join('');

    ['ni-assignee', 'detail-assignee'].forEach(id => {
        const sel = document.getElementById(id);
        if (!sel) return;
        const prev = sel.value;
        sel.innerHTML = options;
        sel.value = prev;
    });
}

// =====================================================================
// BACKDROP CLICK DISMISS
// =====================================================================

function backdropClick(event, overlayId, hideFn) {
    if (event.target.id === overlayId) hideFn();
}

// =====================================================================
// INITIALIZATION
// =====================================================================

async function init() {
    // Try to restore from session.
    const savedToken = sessionStorage.getItem(TOKEN_KEY);
    const savedUser  = sessionStorage.getItem(SESSION_KEY);
    const savedSetup = loadSetupConfig();

    if (!savedSetup) {
        showSetup();
        return;
    }

    _setup = savedSetup;

    // Try to restore or obtain a token.
    if (savedToken) {
        _token = savedToken;
    } else {
        // Re-authenticate with saved credentials.
        try {
            await egoLogin(_setup.user, _setup.pass);
        } catch {
            showSetup('Could not connect to server. Please check your credentials.');
            return;
        }
    }

    // Verify token works and ensure tables exist.
    try {
        await initTables();
    } catch (e) {
        // Token may be stale — try re-login.
        try {
            await egoLogin(_setup.user, _setup.pass);
            await initTables();
        } catch {
            showSetup('Could not initialize database. Please check your server connection.');
            return;
        }
    }

    // Restore session user.
    if (savedUser) {
        try {
            _currentUser = JSON.parse(savedUser);
        } catch {}
    }

    if (!_currentUser) {
        await startLoginFlow();
        return;
    }

    // User is already logged in — jump straight to the app.
    await launchApp();
}

document.addEventListener('DOMContentLoaded', init);
