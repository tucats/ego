// dashboard-sqlwizard.js
// The SQL Build wizard: the statement parser that pre-loads the wizard from a
// selected statement, and the SELECT, INSERT, UPDATE, DELETE, CREATE TABLE
// and ALTER TABLE builders.
//
// Depends on the SQL tab (dashboard-sql.js) for the editor it inserts into
// and for the formatter it runs generated statements through.
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
// SQL Build wizard
// ==========================================================================

// Column metadata for the table currently selected in the wizard.
// Populated by sqlWizardTableChanged() and consumed by the clause-row builders.
let _sqlWizardCols = [];

// Full list of tables in the current DSN. Populated by sqlWizardTypeChanged()
// each time the wizard opens so the CREATE wizard can check for name collisions.
let _sqlWizardTables = [];

// Maps DSN name → provider string ("postgres" or "sqlite").
// Populated by loadSql() when the DSN list is fetched so the ALTER TABLE wizard
// can choose the correct SQL dialect without an extra network call.
let _sqlDsnProviders = {};

// Maps DSN name → its "rowIds" attribute (defs.DSN.RowId). Populated by
// loadSql() alongside _sqlDsnProviders. When true, the server always maintains
// a unique "_row_id_" string column on every table in that DSN (see
// FormCreateQuery in internal/server/tables/parsing/generators.go), so the
// CREATE TABLE wizard must mirror that by always including a locked
// "_row_id_" column that the user cannot remove or edit.
let _sqlDsnRowIds = {};

// Returns true when the currently selected SQL DSN is a Postgres database.
// Postgres supports multiple column changes in one ALTER TABLE statement;
// SQLite requires one change per statement.
function _sqlWizardIsPostgres() {
    const dsn = document.getElementById('sql-dsn-picker')?.value || '';
    return (_sqlDsnProviders[dsn] || '').toLowerCase() === 'postgres';
}

// Returns true when the currently selected SQL DSN has its "rowIds"
// attribute set, meaning every table in it always carries the internal
// "_row_id_" column.
//
// `?.` is "optional chaining": `document.getElementById(...)?.value` reads
// `.value` only if getElementById() actually found an element, and short-
// circuits to `undefined` instead of throwing if it returned null (e.g. the
// picker isn't in the DOM yet) — equivalent to, but shorter than, writing
// `el ? el.value : undefined`. The trailing `|| ''` then substitutes an
// empty string for that undefined case, since dsn is used as an object key
// below and `_sqlDsnRowIds[undefined]` would look up the wrong thing.
//
// `!!` is double negation: a single `!` converts any value to its boolean
// opposite (`!5` is `false`, `!undefined` is `true`), so applying it twice
// converts to boolean while preserving the original truthiness — it's the
// standard idiom for "coerce this to true/false" when a value (here,
// _sqlDsnRowIds[dsn], which may be a real boolean or simply missing/
// undefined for a DSN never loaded) needs to become a proper boolean rather
// than just some other truthy/falsy value.
function _sqlWizardCurrentDsnHasRowId() {
    const dsn = document.getElementById('sql-dsn-picker')?.value || '';
    return !!_sqlDsnRowIds[dsn];
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
// The locked "_row_id_" row (if present) is skipped — it keeps the value
// generateRowId() already assigned rather than replaying a value that may
// have come from a previous INSERT and would no longer be unique.
function _applyParsedInsert(parsed) {
    document.querySelectorAll('#sql-wiz-insert-fields .sql-wiz-insert-row').forEach(row => {
        if (row.dataset.col === '_row_id_') return;

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

    // Re-lock the "_row_id_" row if the current DSN requires it — the loop
    // above may have just added it back as an ordinary, editable row.
    _sqlWizardCreateSyncRowIdRow();
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
        _sqlWizardCreateSyncRowIdRow();
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
// Unlike the server's own Table Rows API (which assigns _row_id_ automatically
// on INSERT — see internal/server/tables/scripting/insert.go), this wizard
// builds a raw INSERT statement that bypasses that machinery entirely, so
// _row_id_ is kept in the field list: _sqlWizardInsertFieldsHtml() renders it
// as a locked row pre-filled with a generated value rather than omitting it.
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

    const container = document.getElementById('sql-wiz-insert-fields');
    if (container) container.innerHTML = _sqlWizardInsertFieldsHtml(_sqlWizardCols);
    buildSqlPreview();
}

// Return the HTML for the INSERT column-value form. Each row shows the column
// name, its SQL type as a hint, a text input, and a Null toggle button.
// Each row div carries data-col and data-type attributes so that
// buildSqlPreview() can read the column name and type without querying
// _sqlWizardCols again. data-is-null starts as "false"; sqlWizardInsertNullToggle()
// flips it to "true" when the Null button is active.
// The internal "_row_id_" column, when present, gets a locked row instead
// (see _sqlWizardInsertRowIdRowHtml()) rather than this generic treatment.
function _sqlWizardInsertFieldsHtml(cols) {
    if (cols.length === 0) return '<p class="sql-wiz-unsupported">No columns available.</p>';
    return cols.map(c => {
        if (c.name === '_row_id_') return _sqlWizardInsertRowIdRowHtml(c);

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

// Return the HTML for the locked "_row_id_" value row. Its value is generated
// once here, client-side, via generateRowId() rather than typed by the user —
// see sqlWizardInsertTableChanged() for why this wizard must supply it itself.
// The input is disabled (not just readonly) so the value can't be edited but
// is still readable via .value for buildSqlPreview(); there is no Null button
// since this column may never be null.
function _sqlWizardInsertRowIdRowHtml(c) {
    const typeHint = escapeHtml(c.type || '');
    return '<div class="sql-wiz-insert-row sql-wiz-insert-row-locked" data-col="_row_id_"'
        + ' data-type="' + typeHint + '">'
        + '<span class="sql-wiz-insert-label">'
        + '<span class="sql-wiz-insert-colname">_row_id_</span>'
        + '<span class="sql-wiz-insert-typehint">' + typeHint + ' &bull;</span>'
        + '</span>'
        + '<input type="text" class="sql-wiz-insert-input" data-is-null="false" disabled'
        + ' value="' + escapeHtml(generateRowId()) + '">'
        + '<span class="sql-wiz-insert-locked" title="Generated automatically — required by this table\'s Row ID column">&#x1F512;</span>'
        + '</div>';
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

// Build the locked column-definition row for the internal "_row_id_" column.
// Every input is disabled (not just readonly) so its value is still readable
// via .value for buildSqlPreview(), but the user cannot edit or remove it.
// The type is fixed at VARCHAR — the wizard's own default string type — Unique
// is always checked, and Nullable is left checked (so buildSqlPreview() emits
// no NOT NULL clause) — together matching the column the server itself adds
// for rowIds-enabled DSNs: "_row_id_ VARCHAR UNIQUE" with no nullability
// clause (see FormCreateQuery in generators.go).
function _sqlWizardCreateRowIdRowHtml() {
    return '<input type="text" class="sql-wiz-create-col-name" value="_row_id_" disabled>'
        + '<select class="sql-wiz-create-col-type" disabled>'
        + '<option value="VARCHAR" selected>VARCHAR</option>'
        + '</select>'
        + '<label class="sql-wiz-create-cb-label">'
        + '<input type="checkbox" class="sql-wiz-create-col-unique" checked disabled>Unique'
        + '</label>'
        + '<label class="sql-wiz-create-cb-label">'
        + '<input type="checkbox" class="sql-wiz-create-col-nullable" checked disabled>Nullable'
        + '</label>'
        + '<span class="sql-wiz-create-col-locked" title="Required because the selected DSN has Row ID enabled">&#x1F512;</span>';
}

// Ensure the CREATE TABLE column list reflects the current DSN's "rowIds"
// attribute: adds a locked "_row_id_" row (as the first column) when the DSN
// requires it, replacing any ordinary "_row_id_" row that may have arrived
// via a parsed CREATE TABLE statement (_applyParsedCreate) so it can't be
// left editable. Does nothing when the DSN does not use row IDs — a
// "_row_id_" row from parsed SQL is then left as an ordinary, editable
// column, since it isn't one this wizard is required to manage.
// Called once when the CREATE wizard body is first built, and again after
// _applyParsedCreate(), since that replaces the column list wholesale.
function _sqlWizardCreateSyncRowIdRow() {
    if (!_sqlWizardCurrentDsnHasRowId()) return;

    const list = document.getElementById('sql-wiz-create-cols');
    if (!list) return;

    // querySelectorAll() returns a NodeList, not a real array, and NodeLists
    // don't have a .filter() method — Array.from() copies its elements into
    // a real array first so .filter() can be used to keep only the row (if
    // any) whose name field reads "_row_id_", and .forEach() then removes
    // each one found. (`row.querySelector(...)?.value` uses the same
    // optional-chaining shorthand explained above _sqlWizardCurrentDsnHasRowId()
    // — it reads .value only if the name input inside this particular row
    // was actually found.)
    Array.from(list.querySelectorAll('.sql-wiz-create-col-row'))
        .filter(row => row.querySelector('.sql-wiz-create-col-name')?.value === '_row_id_')
        .forEach(row => row.remove());

    const row = document.createElement('div');
    row.className = 'sql-wiz-create-col-row sql-wiz-create-col-row-locked';
    row.innerHTML  = _sqlWizardCreateRowIdRowHtml();
    list.insertBefore(row, list.firstChild);
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

    // Always end the inserted statement with ";" so the preprocessor treats it
    // as a complete statement, and so the formatter below sees a finished one.
    let statement = preview + (/;\s*$/.test(preview) ? '' : ';');

    // When the Format setting is on, run the wizard's output through the same
    // formatter the rest of the editor uses, so generated SQL matches what the
    // user's own typing is held to. Only the new statement is formatted, not
    // the whole editor: reformatting everything here would reflow text the
    // user may have laid out by hand, and would move the caret away from the
    // insertion point computed below. As everywhere else, SQL the formatter
    // cannot handle is inserted exactly as the wizard built it.
    if (codeFormatEnabled) {
        const formatted = formatSqlText(statement);
        if (formatted !== null) statement = formatted;
    }

    // Prepend a visible warning comment when the user confirmed a no-WHERE
    // statement. It stays outside the formatted text so the formatter cannot
    // reposition it away from the statement it warns about.
    const warn = allRows ? '// WARNING: this statement affects all rows\n' : '';
    const inserted = sep + warn + statement + '\n';
    editor.value = before + inserted + after;
    editor.selectionStart = editor.selectionEnd = start + inserted.length;
    editor.focus();
    updateSqlHighlight();
    hideSqlBuild();
}
