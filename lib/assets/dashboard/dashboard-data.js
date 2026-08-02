// dashboard-data.js
// The Data tab -- browsing rows from a selected DSN and table -- together
// with its row edit sheet.
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

