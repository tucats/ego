// dashboard-sql.js
// The SQL tab: the editor and its syntax-highlight layer, DSN hinting,
// statement preprocessing and submission, and the client-side SQL formatter.
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
    'OFFSET','ON','OR','ORDER','OUTER','OVER',
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

// Scan the SQL editor text for a DSN hint inside "--" or "//" line comments
// — both are treated as comment markers elsewhere in this app (see
// preprocessSql() for "//" and _sqlParseTok() for "--"). The word "dsn" is
// matched case-insensitively, and the DSN name may sit on either side of it:
//   "// Let's use the pg1 dsn for this query"   → "pg1" (word before "dsn")
//   "// These queries work with the dsn pg1..." → "pg1" (word after "dsn")
//
// If exactly one distinct DSN is referenced this way, and it matches an
// existing option in the DSN picker, the picker is switched to it and that
// becomes the active DSN. If comments reference more than one different DSN
// — including a single "dsn" occurrence with a *different* valid DSN name on
// each side, which is just as ambiguous as two separate mentions — no action
// is taken and the current selection is left alone. This lets users embed a
// DSN hint in a saved query so the right database is selected when it's
// pasted in.
function applySqlDsnHint(text) {
    const picker = document.getElementById('sql-dsn-picker');
    if (!picker) return;

    // picker.options is an HTMLOptionsCollection, not a real JS array — it's
    // "array-like" (has a .length and numeric indexes) but is missing array
    // methods such as .find(), which is used below. Array.from() copies its
    // items into an actual Array so those methods become available.
    const options = Array.from(picker.options);

    // Two small helper functions, written as "arrow functions" (the
    // `word => ...` shorthand for an anonymous function) and stored in
    // const variables so they can be called by name below, just like an
    // ordinary `function stripPunct(word) { ... }` would be.
    //
    // stripPunct() trims leading/trailing punctuation a natural-language
    // comment might attach to a word — quotes, a trailing period, or a
    // leading "--"/"//" if there's no space after the comment marker.
    // \W is a regex shorthand meaning "any character that is NOT a letter,
    // digit, or underscore" (the opposite of \w); ^\W+ matches a run of
    // such characters at the very start of the word, and \W+$ matches a run
    // at the very end — the `|` between them means "match either pattern",
    // and the `g` flag makes replace() apply it to every match it finds
    // rather than stopping after the first.
    //
    // findDsnOpt() then looks up a (punctuation-stripped, lowercased) word
    // against the DSN picker's real option values and returns the matching
    // <option> element if one exists, or `undefined` (a falsy value) if not
    // — Array.prototype.find() returns the first element for which the
    // callback returns true, or undefined if none does.
    const stripPunct = word => word.replace(/^\W+|\W+$/g, '');
    const findDsnOpt  = word => options.find(o => o.value.toLowerCase() === stripPunct(word).toLowerCase());

    // Walk the text one character at a time, skipping over single-quoted
    // string contents (with '' as an escaped quote, matching
    // _sqlParseTok()'s own string handling) so a "--" or "//" inside a
    // string literal is never mistaken for the start of a comment.
    const hints = [];
    let i = 0;
    while (i < text.length) {
        const c = text[i];

        if (c === "'") {
            i++;
            while (i < text.length) {
                if (text[i] === "'" && text[i + 1] === "'") { i += 2; continue; }
                if (text[i] === "'") { i++; break; }
                i++;
            }
            continue;
        }

        if ((c === '-' && text[i + 1] === '-') || (c === '/' && text[i + 1] === '/')) {
            const eol  = text.indexOf('\n', i);
            const line = eol === -1 ? text.slice(i) : text.slice(i, eol);

            // Break the comment line into its individual words: split() on
            // one-or-more whitespace characters (\s+) turns "a  b   c" into
            // ["a", "b", "c"], but would also leave an empty "" entry if the
            // line starts or ends with whitespace — filter() removes those
            // by keeping only words with at least one character.
            const words = line.split(/\s+/).filter(w => w.length > 0);

            // Check the word immediately before and after each "dsn" word.
            // If both are valid DSN names, push both — same effect as two
            // conflicting mentions, resolved by the dedupe/conflict check below.
            for (let w = 0; w < words.length; w++) {
                if (stripPunct(words[w]).toLowerCase() !== 'dsn') continue;

                // The ternary `condition ? valueIfTrue : valueIfFalse` below
                // guards against reading past either end of the words array:
                // there's no "previous word" if "dsn" is the first word
                // (w > 0 is false), and no "next word" if it's the last.
                const prevOpt = w > 0                ? findDsnOpt(words[w - 1]) : null;
                const nextOpt = w < words.length - 1 ? findDsnOpt(words[w + 1]) : null;

                if (prevOpt) hints.push(prevOpt.value);
                if (nextOpt) hints.push(nextOpt.value);
            }

            i = eol === -1 ? text.length : eol;
            continue;
        }

        i++;
    }

    if (hints.length === 0) return;

    // A Set is a built-in collection that automatically discards duplicate
    // values — `new Set(hints)` copies every hint in, but any value that's
    // already present is simply ignored rather than added again. Since
    // `hints` already holds canonical option values from findDsnOpt(), the
    // same DSN mentioned more than once (or on both sides of the same
    // "dsn") collapses down to a single entry and isn't treated as a
    // conflict — only a Set with more than one distinct DSN name in it
    // counts as ambiguous.
    //
    // `[...resolved]` uses the spread operator to copy the Set's contents
    // out into a real array (Sets don't support indexing like `resolved[0]`
    // directly), and `[0]` then takes that array's first — and, since we
    // just checked size === 1, only — element.
    const resolved = new Set(hints);
    if (resolved.size === 1) picker.value = [...resolved][0];
    // 0 matches, or 2+ conflicting DSNs named: take no action.
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

    // Ctrl/Cmd+Enter submits; plain Enter checks for a DSN hint comment and,
    // when the Format setting is on, reformats the statement the caret has
    // just finished (see formatSqlOnEnter).
    //
    // Shift+Enter and Alt+Enter deliberately skip the formatting, giving the
    // user a way to add a plain line break without it.
    //
    // e.preventDefault() cancels the browser's own response to the key — here,
    // typing a newline into the textarea. It is called only when
    // formatSqlOnEnter returns true, because in that case the function has
    // already rewritten the text and placed the caret on the new line itself;
    // letting the browser also insert one would produce two. When it returns
    // false nothing is cancelled and Enter behaves completely normally.
    editor.addEventListener('keydown', e => {
        if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
            submitSql();
        } else if (e.key === 'Enter' && !e.shiftKey && !e.altKey) {
            applySqlDsnHint(editor.value);
            if (codeFormatEnabled && formatSqlOnEnter(editor)) e.preventDefault();
        } else if (e.key === 'Enter') {
            applySqlDsnHint(editor.value);
        }
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

// Save a block of text to a file the user chooses. Shared by the SQL tab's
// Save button (saveSqlFile) and the Code tab's (saveCodeFile), which differ
// only in which editor they read and what they call the result.
//
//   text        — the contents to write
//   defaultName — file name to suggest, including its extension
//   description — how the file type is described in the native Save dialog
//   extensions  — file extensions to offer, e.g. ['.sql', '.txt']
//
// Uses the File System Access API (showSaveFilePicker) when available for a
// native Save dialog; falls back to a Blob-URL download for browsers that do
// not support it (Firefox, Safari).
async function saveTextFile(text, defaultName, description, extensions) {
    // `typeof x === 'function'` is the safe way to ask whether a browser
    // feature exists: naming a property that was never defined yields
    // `undefined` rather than an error, so this is true only where the API is
    // actually implemented.
    if (typeof window.showSaveFilePicker === 'function') {
        try {
            const handle = await window.showSaveFilePicker({
                suggestedName: defaultName,
                types: [{
                    description: description,
                    accept: { 'text/plain': extensions }
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

// Save the SQL editor contents to a .sql file.
async function saveSqlFile() {
    // The ?. before .value is optional chaining: if getElementById returned
    // null the whole expression is `undefined` rather than an error, and the
    // || '' then substitutes an empty string.
    const text = document.getElementById('sql-editor')?.value || '';

    await saveTextFile(text, 'query.sql', 'SQL file', ['.sql', '.txt']);
}

// Load the SQL tab — populates the DSN picker, records each DSN's database
// provider in _sqlDsnProviders (used by the ALTER TABLE wizard for dialect
// decisions), and refreshes the syntax highlight layer.
async function loadSql() {
    const picker = document.getElementById('sql-dsn-picker');
    const previousDsn = picker.value;

    try {
        const res   = await apiFetch('dsns');
        const data  = await res.json();
        const items = data.items || [];
        // Always refresh the provider and rowIds maps — needed by the ALTER
        // and CREATE TABLE wizards respectively.
        _sqlDsnProviders = {};
        _sqlDsnRowIds    = {};
        for (const d of items) {
            _sqlDsnProviders[d.name] = d.provider || '';
            _sqlDsnRowIds[d.name]    = !!d.rowid;
        }
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
    document.getElementById('sql-elapsed').textContent = '';
    updateSqlHighlight();
    document.getElementById('sql-editor').focus();
}

// ==========================================================================
// SQL formatter — client-side "pretty printer" for the SQL editor
// ==========================================================================
//
// The Code tab reformats Ego source by posting it to the server's AST-based
// formatter (see formatEditorCode()). There is no equivalent server endpoint
// for SQL — the server hands SQL straight to the database driver and never
// parses it — so the SQL formatter lives here, in the browser.
//
// It is deliberately a *token stream* reformatter, not a parser. It decides
// where lines break and how they indent purely from the sequence of tokens,
// which means it never has to understand a dialect's full grammar and can
// pass unfamiliar vendor syntax through untouched.
//
// The safety contract has three parts, and all three matter because this
// rewrites text the user is about to execute against a real database:
//
//   1. sqlFormatTok() is LOSSLESS: concatenating every token's value
//      reproduces the input byte for byte. (This is what separates it from
//      _sqlParseTok(), which deliberately discards comments, strips quoting,
//      and skips characters it does not recognize. That one feeds the ALTER
//      TABLE wizard and must not be changed to serve this.)
//   2. Anything the tokenizer cannot make sense of — an unterminated string,
//      quoted identifier, or block comment — aborts the whole operation.
//   3. formatSqlText() re-tokenizes its own output and compares the
//      significant-token streams. If they differ in any way, the formatted
//      text is thrown away and the original is returned unchanged.
//
// Rule 3 is the real guarantee: the worst case is "your SQL did not get
// reformatted", never "your SQL got reformatted into something else".
// -----------------------------------------------------------------------

// One indent level of formatter output.
const SQL_FMT_INDENT = '    ';

// Keyword phrases that begin a new output line at their block's base indent.
// Each entry is an array of words that must appear one after another, so
// ['GROUP', 'BY'] matches the two words "GROUP BY" in sequence.
//
// Order matters — the list is scanned front to back and the first match wins,
// so longer phrases must precede the shorter ones they start with ('GROUP BY'
// before 'GROUP', 'LEFT OUTER JOIN' before 'LEFT JOIN'). Adding a new clause
// keyword is a matter of putting it in this list at the right position; no
// other part of the formatter needs to change.
const SQL_FMT_CLAUSES = [
    ['CREATE', 'UNIQUE', 'INDEX'], ['CREATE', 'INDEX'], ['CREATE', 'TABLE'],
    ['CREATE', 'VIEW'],
    ['DROP', 'TABLE'], ['DROP', 'INDEX'], ['DROP', 'VIEW'],
    ['ALTER', 'TABLE'],
    ['INSERT', 'INTO'], ['DELETE', 'FROM'],
    ['GROUP', 'BY'], ['ORDER', 'BY'], ['PARTITION', 'BY'],
    ['UNION', 'ALL'], ['UNION'], ['INTERSECT'], ['EXCEPT'],
    ['LEFT', 'OUTER', 'JOIN'], ['RIGHT', 'OUTER', 'JOIN'], ['FULL', 'OUTER', 'JOIN'],
    ['LEFT', 'JOIN'], ['RIGHT', 'JOIN'], ['FULL', 'JOIN'],
    ['INNER', 'JOIN'], ['CROSS', 'JOIN'], ['JOIN'],
    ['SELECT'], ['FROM'], ['WHERE'], ['HAVING'], ['LIMIT'], ['OFFSET'],
    ['VALUES'], ['SET'], ['RETURNING'], ['UPDATE'], ['WITH'],
];

// Tokens that never take a space in front of them, and tokens that never take
// a space after them. A Set is used rather than an array because the only
// question ever asked of these is "is this string in the collection?", which
// Set answers with .has() in one step no matter how many entries it holds.
const SQL_FMT_NO_SPACE_BEFORE = new Set([',', ';', ')', '.', '::', '->', '->>']);
const SQL_FMT_NO_SPACE_AFTER  = new Set(['(', '.', '::', '->', '->>']);

// Lossless SQL tokenizer. Returns an array of { type, value, pos } objects
// covering every character of `text`, or null if the input contains an
// unterminated construct the formatter must not guess at.
//
// Token types:
//   'ws'      — a run of whitespace (kept so callers can see line breaks)
//   'comment' — "--", "//", "#" to end of line, or a /* ... */ block
//   'string'  — '...' literal, or a Postgres $$ ... $$ dollar-quoted body
//   'ident'   — bare word, or a "..." / `...` / [...] delimited identifier
//   'number'  — numeric literal (a leading sign is a separate operator token)
//   'param'   — bind parameter: ?, $1, :name, @name
//   'op'      — operator or punctuation, including any character not
//               otherwise recognized
//
// `pos` is the token's starting offset in `text`, used by the format-on-Enter
// path to locate statement boundaries.
//
// "//" is accepted as a line comment marker alongside the standard "--"
// because preprocessSql() and applySqlDsnHint() already treat it as one in
// this editor; "#" is MySQL's spelling.
function sqlFormatTok(text) {
    const tokens = [];
    const n      = text.length;
    let   i      = 0;   // scan position: the next character to look at

    // Record text[start] up to (but not including) text[i] as one token.
    //
    // This is an "arrow function" — the `(args) => expression` shorthand for
    // an anonymous function — stored in a const so it can be called by name,
    // exactly as `function push(type, start) { ... }` would be. The important
    // detail is that it reads `i` from the enclosing function rather than
    // taking it as an argument: because the arrow function was defined inside
    // sqlFormatTok, it shares that one `i` variable and always sees whatever
    // value the loop below has advanced it to. (A function that "remembers"
    // the variables around where it was written is called a closure; this file
    // relies on that behavior heavily inside sqlFormatEmit too.)
    //
    // .slice(start, i) copies out the characters from index `start` up to but
    // NOT including index `i` — the end is exclusive, so by the time push() is
    // called, `i` should already sit one past the token's last character.
    const push = (type, start) => tokens.push({ type: type, value: text.slice(start, i), pos: start });

    // Walk the text once, from beginning to end. Every branch below either
    // consumes at least one character and pushes a token, or returns null;
    // that guarantees the loop always makes progress and can never hang.
    while (i < n) {
        const c     = text[i];              // character at the scan position
        const two   = text.substr(i, 2);    // it and the next one, as a string
        const start = i;                    // where the token being read began

        // Whitespace run. `/\s/` is a regular expression literal — a pattern
        // written directly in the source between slashes — where \s is the
        // shorthand for "any whitespace character" (space, tab, newline, ...).
        // Its .test() method returns true if the pattern matches anywhere in
        // the string it is given.
        //
        // `continue` jumps straight back to the top of the while loop, skipping
        // every branch below it. Each branch here ends that way, so the effect
        // is a chain of mutually exclusive cases.
        if (/\s/.test(c)) {
            while (i < n && /\s/.test(text[i])) i++;
            push('ws', start);
            continue;
        }

        // Line comment — runs to (but does not include) the newline, so the
        // newline stays in the following whitespace token.
        if (two === '--' || two === '//' || c === '#') {
            while (i < n && text[i] !== '\n') i++;
            push('comment', start);
            continue;
        }

        // Block comment. An unterminated one would swallow the rest of the
        // statement, so refuse to format rather than guess where it ends.
        //
        // .indexOf(needle, from) searches for `needle` starting at index
        // `from` and returns where it was found, or -1 if it never was. That
        // -1 is JavaScript's "not found" answer for string and array searches
        // alike, and shows up several more times below.
        if (two === '/*') {
            const close = text.indexOf('*/', i + 2);
            if (close === -1) return null;
            i = close + 2;
            push('comment', start);
            continue;
        }

        // Single-quoted string literal; '' is an embedded quote.
        //
        // The `closed` flag records whether the closing quote was actually
        // found. `break` exits the inner while loop immediately (unlike
        // `continue`, which would go round it again), so reaching the end of
        // the text without a closing quote leaves `closed` false and aborts
        // the whole tokenizer. Guessing where an unterminated string ends is
        // exactly the kind of mistake that could change what the SQL means.
        if (c === "'") {
            i++;
            let closed = false;
            while (i < n) {
                if (text[i] === "'" && text[i + 1] === "'") { i += 2; continue; }
                if (text[i] === "'") { i++; closed = true; break; }
                i++;
            }
            if (!closed) return null;
            push('string', start);
            continue;
        }

        // Delimited identifier: "ansi", `mysql`, or [t-sql]. The doubled
        // delimiter is an escape for the first two spellings; brackets have no
        // such convention, so the first ']' closes them.
        //
        // The `condition ? valueIfTrue : valueIfFalse` form below is the
        // ternary operator: it picks ']' as the closing character when the
        // token opened with '[', and otherwise closes with the same character
        // it opened with.
        if (c === '"' || c === '`' || c === '[') {
            const close = c === '[' ? ']' : c;
            i++;
            let closed = false;
            while (i < n) {
                if (close !== ']' && text[i] === close && text[i + 1] === close) { i += 2; continue; }
                if (text[i] === close) { i++; closed = true; break; }
                i++;
            }
            if (!closed) return null;
            push('ident', start);
            continue;
        }

        // Postgres dollar quoting: $$body$$ or $tag$body$tag$. Checked before
        // the $1 placeholder case because "$" alone is ambiguous between them.
        //
        // .exec() runs a regular expression against a string and returns an
        // array describing the match, or null if there was none — so `tag`
        // doubles as both the result and the "did it match?" test. Entry [0]
        // of that array is the whole matched text, which here is the opening
        // delimiter ("$$" or "$name$") that must be found again to close the
        // literal. In the pattern, ^ anchors the match to the very start of
        // the string, \$ means a literal dollar sign (a bare $ has a special
        // meaning in regular expressions, so it has to be escaped), and the
        // trailing ? makes the parenthesized tag name optional.
        if (c === '$') {
            const tag = /^\$([A-Za-z_][A-Za-z0-9_]*)?\$/.exec(text.slice(i));
            if (tag) {
                const close = text.indexOf(tag[0], i + tag[0].length);
                if (close === -1) return null;
                i = close + tag[0].length;
                push('string', start);
                continue;
            }

            // Reading one position past the end of a string gives `undefined`
            // rather than an error, and `undefined` would make .test() throw.
            // The `x || y` idiom evaluates to `x` when `x` is usable and to
            // `y` when it is not, so `text[i + 1] || ''` substitutes an empty
            // string at the end of the text — which simply fails to match.
            if (/[0-9]/.test(text[i + 1] || '')) {
                i++;
                while (i < n && /[0-9]/.test(text[i])) i++;
                push('param', start);
                continue;
            }

            // Neither a dollar-quote nor a $1 placeholder: fall through to the
            // operator cases below, which treat the "$" as ordinary
            // punctuation. This is the one branch here that does NOT end in a
            // `continue`, and it is deliberate.
        }

        // Remaining bind parameter spellings.
        if (c === '?') {
            i++;
            push('param', start);
            continue;
        }

        // ":name" and "@name". A bare "::" cast operator does not match here
        // because the character after ":" is not a letter, so it falls through
        // to the two-character operator case below.
        if ((c === ':' || c === '@') && /[A-Za-z_]/.test(text[i + 1] || '')) {
            i += 2;
            while (i < n && /[A-Za-z0-9_]/.test(text[i])) i++;
            push('param', start);
            continue;
        }

        // Numeric literal. Unlike _sqlParseTok(), a leading "-" is left as its
        // own operator token: a formatter has to be able to tell "a - 1" from
        // "a, -1", and it decides spacing for unary signs separately.
        //
        // The pattern matches either a hex literal (0x1F) or a decimal one,
        // where [0-9]*\.?[0-9]+ allows "5", "3.14" and ".5", and the optional
        // ([eE][-+]?[0-9]+) tail allows exponents such as "1e10". Square
        // brackets in a regular expression mean "any one character from this
        // set", and the | between the two halves means "either side matches".
        if (/[0-9]/.test(c) || (c === '.' && /[0-9]/.test(text[i + 1] || ''))) {
            const num = /^(0[xX][0-9a-fA-F]+|[0-9]*\.?[0-9]+([eE][-+]?[0-9]+)?)/.exec(text.slice(i));
            i += num[0].length;
            push('number', start);
            continue;
        }

        // Bare identifier or keyword. Note the second character set is wider
        // than the first: a name may not start with a digit, but may contain
        // one after its first character.
        if (/[A-Za-z_]/.test(c)) {
            while (i < n && /[A-Za-z0-9_$]/.test(text[i])) i++;
            push('ident', start);
            continue;
        }

        // Multi-character operators, longest first — "->>" has to be tested
        // before "->", or the longer operator would be split into "->" plus a
        // stray ">".
        if (text.substr(i, 3) === '->>') {
            i += 3;
            push('op', start);
            continue;
        }

        // .includes() asks whether an array contains a given value, so this
        // reads as "if `two` is one of these operators". An array is fine here
        // rather than a Set because the list is short and this is not on a hot
        // path.
        if (['->', '::', '||', '<>', '<=', '>=', '!=', '&&', ':='].includes(two)) {
            i += 2;
            push('op', start);
            continue;
        }

        // Catch-all: any other single character becomes an operator token.
        // This is what keeps the tokenizer lossless in the face of syntax it
        // does not specifically know about.
        i++;
        push('op', start);
    }

    return tokens;
}

// Reduce a token array to the comparable "significant" stream used by the
// round-trip check in formatSqlText(): whitespace is dropped, comments are
// compared by their trimmed text, and words the formatter is allowed to
// re-case are normalized to upper case on both sides of the comparison.
// Everything else — identifiers, string literals, numbers, operators — must
// match exactly, character for character.
//
// The result is an array of plain strings, each one a token's type and text
// glued together with a colon ("k:SELECT", "ident:users", "op:,"). Comparing
// two of these arrays entry by entry is then a simple string comparison, and
// the type prefix keeps categories apart so that, say, the identifier `x` can
// never be mistaken for the string literal 'x'.
//
// Deciding what to ignore here is a judgement call with real consequences: an
// difference this function smooths over is a difference the round-trip check
// in formatSqlText() will not catch. Only two are smoothed over, and both are
// changes the formatter is explicitly allowed to make — the whitespace it
// rearranges, and the keyword capitalization it normalizes. Anything else
// added to this list would widen the hole.
function sqlFormatSig(tokens) {
    const out = [];

    // `for (const t of tokens)` walks the array's values one at a time — `t`
    // is each token object itself. (JavaScript also has `for...in`, which
    // yields index numbers instead; that is not what is wanted here.)
    for (const t of tokens) {
        // Whitespace is the formatter's to rewrite, so it is not compared.
        if (t.type === 'ws') continue;

        // Comments are compared by content with surrounding blanks removed,
        // since the formatter may move one to a different column.
        if (t.type === 'comment') {
            out.push('c:' + t.value.trim());
            continue;
        }

        const up = t.value.toUpperCase();

        // Keywords and type names are compared case-insensitively because the
        // formatter upper-cases them. Both sides of the comparison go through
        // this same function, so both get the same treatment. A quoted
        // identifier such as "select" keeps its quote characters in .value and
        // therefore does not match anything in these sets — which is correct,
        // because quoting is what makes it a name rather than a keyword.
        if (t.type === 'ident' && (SQL_KEYWORDS.has(up) || SQL_TYPES.has(up))) {
            out.push('k:' + up);
            continue;
        }

        // Everything else must survive formatting completely untouched.
        out.push(t.type + ':' + t.value);
    }

    return out;
}

// Lay out a token array as formatted SQL text. Called only through
// formatSqlText(), which supplies the tokens and validates the result.
//
// Layout rules, in full:
//   * Recognized keywords are upper-cased. Type names are upper-cased only in
//     column-definition position (directly after a non-keyword word), so a
//     column actually named "date" or "text" is left alone.
//   * Each clause keyword from SQL_FMT_CLAUSES starts a new line at its
//     block's base indent; the rest of the clause continues on that same line.
//   * A comma, AND, OR, or ON breaks to a new line indented one level under
//     the current clause — but only where the enclosing parentheses are being
//     broken across lines (see below), so function arguments stay inline.
//   * A "(" opens a block if a subquery follows it (SELECT or WITH), a list
//     if it is the column list of a CREATE TABLE, and is inline otherwise.
//     Block and list contents are indented one level with the ")" returned to
//     the indent of the line that opened it.
//   * Comments are reproduced verbatim. A line comment always ends its output
//     line, so code can never end up hidden behind one.
//   * ";" ends the statement and is followed by a blank line.
//
// HOW IT WORKS
//
// The function builds output one line at a time. `parts` collects the pieces
// of the line currently being written; when something decides that line is
// finished, flush() joins the pieces together, puts the right amount of
// indentation on the front, and appends the result to `lines`. At the very
// end, `lines` is joined with newlines to produce the formatted text.
//
// Everything below the variable declarations is a small helper function, and
// all of them are defined *inside* sqlFormatEmit so they can read and modify
// that shared state directly (see the closure note in sqlFormatTok). This is
// why emit() takes only the text to add rather than being handed the line to
// add it to — there is exactly one line under construction at any moment, and
// every helper is looking at the same one. The trade-off is that these
// helpers are order-dependent: calling flush() before emit() rather than
// after produces different output, so read them as steps in a sequence, not
// as independent utilities.
function sqlFormatEmit(tokens) {
    const lines = [];   // completed output lines

    let parts     = []; // pieces of the line currently being built
    let indent    = 0;  // indent level of that line
    let prev      = null;  // previous emitted token, for spacing decisions
    let tightNext = false; // suppress the next space (used for unary +/-)

    // Frames track how far each nesting level is indented and whether it is
    // being broken across lines at all. frames[0] is the statement itself,
    // and each "(" pushes another frame that its matching ")" pops back off —
    // an array used as a stack, where the last element is the innermost
    // context currently being formatted. Each frame holds:
    //   mode   — 'stmt', 'block' or 'list' (lines are broken) or 'inline'
    //            (everything stays on one line)
    //   indent — base indent for clause keywords in this block
    //   body   — indent for continuation lines under the current clause
    //   close  — indent for this block's ")"
    //
    // The parentheses around the object below are required, not decorative.
    // An arrow function written as `() => { ... }` treats the braces as the
    // function's body; wrapping them as `() => ({ ... })` is what makes them
    // an object literal being returned. A fresh object is built on each call
    // so that two statements never accidentally share one frame — assigning
    // an object in JavaScript copies a reference to it, not its contents.
    const newStmtFrame = () => ({ mode: 'stmt', indent: 0, body: 1, close: 0 });
    let frames = [newStmtFrame()];

    // The innermost frame. `frames.length - 1` is the last valid index of the
    // array, so this reads the top of the stack without removing it.
    const frame = () => frames[frames.length - 1];

    // Finish the line under construction, if it has anything on it. An empty
    // `parts` means there is no line in progress, so flush() is safe to call
    // when one is not needed — several callers rely on that.
    //
    // .join('') concatenates the pieces with nothing between them (the spaces
    // were already added as their own pieces by emit), and .repeat(n) makes n
    // copies of the indent string.
    function flush() {
        if (parts.length > 0) lines.push(SQL_FMT_INDENT.repeat(indent) + parts.join(''));
        parts = [];
    }

    // Start a fresh line at the given indent level: finish whatever was in
    // progress, then set the indent the *next* line will be written at.
    function startLine(level) {
        flush();
        indent = level;
    }

    // True when `text` should be separated from the previous piece by a space.
    //
    // `prev &&` guards the two tests after it: prev is null at the start of a
    // statement, and reading .type from null would throw. In JavaScript an
    // `&&` chain stops at the first falsy value, so when prev is null the rest
    // is never evaluated and the whole condition is simply false.
    function spaceBefore(text) {
        if (SQL_FMT_NO_SPACE_BEFORE.has(text)) return false;
        if (prev && prev.type === 'op' && SQL_FMT_NO_SPACE_AFTER.has(prev.value)) return false;
        return true;
    }

    // Append one piece to the current line.
    //
    //   text  — the characters to add
    //   tok   — the token they came from, remembered as `prev` so the next
    //           call can make its spacing decision
    //   tight — pass true to force this piece hard against what precedes it,
    //           overriding the spacing rules (used for things like the "(" of
    //           a function call). Callers that do not care may omit it, in
    //           which case it arrives as `undefined`, which counts as false.
    //
    // `tightNext` does the same job in the other direction: it is set after
    // emitting a unary + or -, so that the *following* call binds tight to the
    // sign. It is cleared here so it only ever affects one piece.
    function emit(text, tok, tight) {
        if (parts.length > 0 && !tight && !tightNext && spaceBefore(text)) parts.push(' ');
        parts.push(text);
        prev      = tok;
        tightNext = false;
    }

    // Index of the next significant (non-whitespace, non-comment) token at or
    // after `idx`, or -1 if there is none. Used to look ahead at what follows
    // a "(" without disturbing the main loop's own position.
    function nextSig(idx) {
        for (let k = idx; k < tokens.length; k++) {
            if (tokens[k].type !== 'ws' && tokens[k].type !== 'comment') return k;
        }
        return -1;
    }

    // If the tokens starting at `idx` spell out `phrase` — an array of
    // upper-case words such as ['GROUP', 'BY'] — return how many tokens that
    // consumes, so the caller can skip past all of them at once. Return 0 when
    // there is no match, which is a falsy value and so reads naturally as
    // "no match" in an `if`.
    //
    // Whitespace between the words is skipped, but a comment is not: the loop
    // only steps over 'ws' tokens, so a comment sitting inside the phrase
    // fails the `t.type !== 'ident'` test and cancels the match. That is
    // deliberate — matching across it would consume the comment along with the
    // keywords and lose it from the output.
    function matchPhrase(idx, phrase) {
        let k = idx;

        for (const word of phrase) {
            while (k < tokens.length && tokens[k].type === 'ws') k++;
            if (k >= tokens.length) return 0;

            const t = tokens[k];
            if (t.type !== 'ident' || t.value.toUpperCase() !== word) return 0;
            k++;
        }

        return k - idx;
    }

    // Render one bare word, upper-casing it when it is a keyword the formatter
    // owns the casing of. Any word not recognized here is returned exactly as
    // the user typed it — table and column names are never re-cased.
    function wordText(t) {
        const up = t.value.toUpperCase();

        if (SQL_KEYWORDS.has(up)) return up;

        // A type name is only treated as one when it directly follows an
        // ordinary word — "id integer" in a column definition, but not the
        // "date" in "SELECT date" or "t.text". Without this test, a column
        // genuinely named "date" or "text" would be shouted back at the user
        // every time they formatted.
        if (SQL_TYPES.has(up) && prev && prev.type === 'ident'
            && !SQL_KEYWORDS.has(prev.value.toUpperCase())) return up;

        return t.value;
    }

    // True when a "+"/"-" at this point is a sign attached to the number that
    // follows ("-1") rather than an arithmetic operator between two values
    // ("a - 1"). The test is what came *before* it: a sign can only appear
    // where an operand could not have just ended.
    //
    //   nothing at all      → "-1"          sign
    //   an operator or "("  → "(-1", "= -1" sign
    //   a keyword           → "VALUES -1"   sign
    //   ")"                 → "(a) - 1"     operator
    //   a name or literal   → "a - 1"       operator
    function isUnarySign() {
        if (!prev) return true;
        if (prev.type === 'op') return prev.value !== ')';
        return prev.type === 'ident' && SQL_KEYWORDS.has(prev.value.toUpperCase());
    }

    // Set after a CREATE TABLE clause so the column list that follows is laid
    // out one column per line, and after INSERT INTO so its column list keeps
    // a space after the table name instead of reading as a function call.
    // Both are cleared as soon as a "(" consumes them or another clause
    // begins, so only the statement's own list is affected.
    let listParenPending  = false;
    let spaceParenPending = false;

    // Set by ";" and acted on when the next content arrives, rather than
    // immediately. The delay is what lets a comment trailing the semicolon on
    // the same source line still land on the statement's own last line:
    //
    //     SELECT a FROM t;  -- note        the comment belongs up here...
    //                                      ...not down here, after the blank
    //     SELECT b FROM u;
    //
    // Closing the statement out the instant the ";" was seen would have
    // already ended that line and written the blank separator, leaving the
    // comment stranded at the top of the next statement.
    let pendingBreak = false;

    // Close out the statement a ";" ended: finish its last line, leave one
    // blank line behind it, and reset every piece of per-statement state back
    // to its starting value. Doing nothing when `pendingBreak` is false makes
    // this safe to call from several places without any of them having to
    // check first.
    function applyPendingBreak() {
        if (!pendingBreak) return;

        flush();
        lines.push('');          // the blank line between statements
        frames            = [newStmtFrame()];
        indent            = 0;
        prev              = null;
        listParenPending  = false;
        spaceParenPending = false;
        pendingBreak      = false;
    }

    // ---------------------------------------------------------------------
    // Main loop — walk the tokens in order, deciding for each one what it does
    // to the line being built. `i` advances by hand rather than through a
    // `for` loop because a matched clause phrase consumes several tokens at
    // once (see matchPhrase).
    // ---------------------------------------------------------------------

    let sawNewline = false; // did the source break the line before this token?
    let i          = 0;

    while (i < tokens.length) {
        const t = tokens[i];

        // Whitespace is dropped — the formatter decides its own spacing — but
        // whether it contained a line break is remembered, because that is how
        // the comment handling below tells a comment on its own line from one
        // trailing code on a shared line.
        if (t.type === 'ws') {
            if (t.value.includes('\n')) sawNewline = true;
            i++;
            continue;
        }

        if (t.type === 'comment') {
            // A comment on a line of its own belongs to the next statement,
            // so close out the previous one first. One that trails the ";" on
            // the same line stays attached to the line it was written on.
            if (sawNewline) applyPendingBreak();

            const isBlock = t.value.startsWith('/*');
            const text    = t.value.trim();

            // A comment that stood on its own line in the source, or that has
            // nothing before it on this output line, gets its own line. A
            // multi-line block comment always does, because re-indenting its
            // interior would change its text.
            if (sawNewline || parts.length === 0 || text.includes('\n')) {
                startLine(indent);
                parts.push(text);
                flush();
            } else {
                // Trailing a piece of code on the same line. Pushed directly
                // rather than through emit() because a comment is not a token
                // the spacing rules should reason about; the space is added by
                // hand instead.
                parts.push(' ' + text);

                // A line comment must end the line. Everything after it on the
                // same line would be inside the comment, so letting code follow
                // one would silently delete that code from the statement. (The
                // round-trip check in formatSqlText() would catch it, but the
                // user would just see formatting mysteriously refuse to work.)
                if (!isBlock) flush();
            }

            // A comment is not an operand, so the next piece must not try to
            // make a spacing decision relative to it.
            prev       = null;
            sawNewline = false;
            i++;
            continue;
        }

        // Any real content ends the statement a preceding ";" left pending.
        // This must come before `frame()` is read below, because it may have
        // replaced the frame stack with a fresh one.
        applyPendingBreak();

        const f  = frame();               // innermost context being formatted
        const up = t.value.toUpperCase(); // this token's text, for keyword tests

        // -- Statement terminator -----------------------------------------
        if (t.type === 'op' && t.value === ';') {
            emit(';', t);
            pendingBreak = true;   // acted on when the next content arrives
            sawNewline   = false;
            i++;
            continue;
        }

        // -- Open parenthesis ----------------------------------------------
        //
        // What kind of group this is decides everything about how it is laid
        // out, and the decision can only be made here, at the "(" itself:
        //
        //   subquery — a SELECT or WITH follows, so the contents get their own
        //              indented block of lines
        //   list     — the column list of a CREATE TABLE, one column per line
        //   inline   — anything else (function arguments, IN lists, VALUES
        //              rows, arithmetic grouping), kept on one line
        if (t.type === 'op' && t.value === '(') {
            const nxt = nextSig(i + 1);

            // The word just inside the parenthesis, or '' if there is no token
            // there or it is not a word at all. Both halves of the `&&` have to
            // pass before tokens[nxt] is read, since nxt is -1 when nothing
            // significant follows and tokens[-1] would be `undefined`.
            const nxtUp  = nxt >= 0 && tokens[nxt].type === 'ident' ? tokens[nxt].value.toUpperCase() : '';
            const isSub  = nxtUp === 'SELECT' || nxtUp === 'WITH';
            const isList = !isSub && listParenPending;

            if (isSub || isList) {
                // `open` is the indent of the line the "(" sits on, captured
                // before startLine() changes it. The closing ")" comes back to
                // this level so it lines up under the line that opened it.
                const open = indent;
                emit('(', t);
                frames.push({
                    mode:   isSub ? 'block' : 'list',
                    indent: open + 1,
                    // A subquery has clause keywords of its own, so its
                    // continuation lines sit one level deeper again. A column
                    // list has none, so its items stay at the block indent.
                    body:   open + 1 + (isSub ? 1 : 0),
                    close:  open,
                });
                startLine(open + 1);
                prev = null;   // nothing on the new line to space against yet
            } else {
                // Inline group. It sits tight against a preceding name so
                // function calls read as "count(*)" rather than "count (*)",
                // but keeps its space after a keyword ("IN (1, 2)") and after
                // an INSERT INTO target ("INSERT INTO t (a, b)"), which is a
                // column list rather than a call.
                const tight = !spaceParenPending
                    && prev !== null
                    && ((prev.type === 'ident' && !SQL_KEYWORDS.has(prev.value.toUpperCase()))
                        || prev.type === 'param'
                        || (prev.type === 'op' && prev.value === ')'));
                emit('(', t, tight);

                // An inline frame needs no indent fields: nothing inside it
                // ever starts a new line, so it exists only to record that
                // fact for the comma and clause rules below.
                frames.push({ mode: 'inline' });
            }

            listParenPending  = false;
            spaceParenPending = false;
            sawNewline        = false;
            i++;
            continue;
        }

        // -- Close parenthesis ---------------------------------------------
        if (t.type === 'op' && t.value === ')') {
            // frames[0] is the statement, so a stack of length 1 means this
            // ")" has no matching "(". Rather than produce nonsense
            // indentation, throw: formatSqlText() catches it and turns the
            // whole attempt into "leave the SQL exactly as it was".
            if (frames.length === 1) throw new Error('unbalanced');

            // .pop() removes and returns the last element, so this both ends
            // the group and hands back the frame describing how to close it.
            const closed = frames.pop();
            if (closed.mode !== 'inline') startLine(closed.close);
            emit(')', t);
            sawNewline = false;
            i++;
            continue;
        }

        // -- Comma ----------------------------------------------------------
        // Inside an inline group the comma just separates arguments; anywhere
        // else it separates list items, each of which gets its own line.
        if (t.type === 'op' && t.value === ',') {
            emit(',', t);
            if (f.mode !== 'inline') startLine(f.body);
            sawNewline = false;
            i++;
            continue;
        }

        // -- Clause keywords -------------------------------------------------
        // Skipped inside inline groups (a window function's ORDER BY is not a
        // new clause) and inside column lists (where words like KEY and
        // REFERENCES are part of a definition, not a clause of their own).
        if (t.type === 'ident' && f.mode !== 'inline' && f.mode !== 'list') {
            let matched = null;
            let used    = 0;

            // Try each phrase in order and stop at the first that matches.
            // SQL_FMT_CLAUSES is ordered longest-first among phrases sharing a
            // leading word, so this finds 'GROUP BY' rather than 'GROUP'.
            for (const phrase of SQL_FMT_CLAUSES) {
                used = matchPhrase(i, phrase);
                if (used > 0) { matched = phrase; break; }
            }

            if (matched) {
                startLine(f.indent);

                // The phrase is emitted as one piece with single spaces
                // between its words, whatever spacing the user had. It is
                // attributed to the phrase's LAST token so that the next
                // token's spacing decision looks at the right word — for
                // 'GROUP BY' that is 'BY', not 'GROUP'.
                emit(matched.join(' '), tokens[i + used - 1]);

                // Continuation lines for this clause sit one level in.
                f.body = f.indent + 1;

                // Arm the two paren behaviors that depend on which clause was
                // just seen. Both are consumed by the next "(" encountered.
                listParenPending  = matched[0] === 'CREATE' && matched[1] === 'TABLE';
                spaceParenPending = matched[0] === 'INSERT';

                sawNewline = false;
                i += used;   // skip every token the phrase consumed
                continue;
            }

            // No clause matched — fall through to the cases below. This `if`
            // block deliberately has no `else`.
        }

        // -- Conjunctions and join conditions --------------------------------
        // These break to a continuation line under the current clause, which
        // is what turns a long WHERE into one condition per line.
        if (t.type === 'ident' && f.mode !== 'inline' && (up === 'AND' || up === 'OR' || up === 'ON')) {
            startLine(f.body);
            emit(up, t);
            sawNewline = false;
            i++;
            continue;
        }

        // -- Everything else --------------------------------------------------
        // Any token that does not affect the layout is simply added to the
        // current line: names, literals, operators, parameters.
        if (t.type === 'ident') {
            emit(wordText(t), t);
        } else if (t.type === 'op' && (t.value === '+' || t.value === '-') && isUnarySign()) {
            emit(t.value, t);
            tightNext = true;   // bind the sign to the operand that follows
        } else {
            emit(t.value, t);
        }

        sawNewline = false;
        i++;
    }

    // The loop ends with the final line still under construction; write it out.
    flush();

    // Drop the blank line a trailing ";" left behind, plus any others at the
    // very end, so the output never finishes with empty lines. `lines.length`
    // is re-read on each pass, so this keeps removing until the last line has
    // content or nothing is left.
    while (lines.length > 0 && lines[lines.length - 1] === '') lines.pop();

    // .join('\n') glues the lines together with a newline between each pair
    // (not after the last one), producing the finished text.
    return lines.join('\n');
}

// Format a block of SQL text. Returns the formatted text, or null if it could
// not be formatted safely — in which case the caller must use the original
// text unchanged.
//
// The round-trip check at the end is what makes this safe to run against text
// the user is about to execute: the formatted output is tokenized again and
// its significant tokens compared against the input's. Any difference at all
// — a comment absorbed into another, a string boundary moved, a token lost to
// a layout bug — fails the comparison and discards the result.
// If you change sqlFormatEmit(), you do not need to prove your change is
// correct in order to keep the editor safe — this function will catch a
// mistake and fall back to the original text. What a mistake will look like
// from the outside is formatting quietly refusing to happen.
function formatSqlText(text) {
    // Step 1: tokenize. A null here means an unterminated string, quoted
    // identifier or block comment, which the tokenizer refuses to guess at.
    const tokens = sqlFormatTok(text);
    if (tokens === null) return null;

    // Step 2: lay the tokens out.
    //
    // `let formatted;` declares the variable without a value so it survives
    // past the try block below — anything declared with let or const *inside*
    // the braces would not be visible outside them.
    let formatted;

    // try/catch runs the code in the first block and, if it throws an error,
    // jumps to the second instead of letting the error escape. sqlFormatEmit
    // throws on unbalanced parentheses; catching everything rather than that
    // one case means an unforeseen bug in the layout code also degrades to
    // "leave the SQL alone" instead of breaking the Submit button.
    try {
        formatted = sqlFormatEmit(tokens);
    } catch (e) {
        return null;
    }

    // Step 3: the round-trip check. Tokenize the formatter's own output and
    // compare it against the input, token by token.
    const check = sqlFormatTok(formatted);
    if (check === null) return null;

    const before = sqlFormatSig(tokens);
    const after  = sqlFormatSig(check);

    // Lengths first: a token gained or lost is a difference on its own, and
    // checking it up front means the comparison below cannot read past the end
    // of the shorter array.
    if (before.length !== after.length) return null;

    // .some() calls the given function for each element and returns true as
    // soon as one of them returns true (stopping there). Its callback receives
    // the element and that element's index, so `v` is the entry from `before`
    // and `after[k]` is the entry at the same position in `after`. Reading it
    // aloud: "if some entry differs from its counterpart, give up".
    if (before.some((v, k) => v !== after[k])) return null;

    return formatted;
}

// Reformat the SQL editor's contents in place. Returns true if the text is
// now formatted (including when it already was), false if it could not be
// formatted and was left alone.
//
// This is what the toolbar's Format button calls, and what submitSql() calls
// when the Format setting is on.
//
// `quiet` suppresses the status-area message, for callers such as submitSql()
// that have their own status to display. Callers that do not pass it — the
// toolbar button's onclick="formatSqlEditor()" — get `undefined`, which counts
// as false, so the message is shown by default.
function formatSqlEditor(quiet) {
    const editor = document.getElementById('sql-editor');
    const source = editor.value;

    // .trim() returns the string without leading or trailing whitespace, and
    // an empty string is falsy, so this reads as "if there is nothing but
    // blanks here". Nothing to do, and nothing went wrong, so report success.
    if (!source.trim()) return true;

    const formatted = formatSqlText(source);

    if (formatted === null) {
        // Tell the user why nothing happened. Without this the Format button
        // would appear simply not to work on SQL the formatter cannot handle.
        if (!quiet) {
            document.getElementById('sql-status').innerHTML =
                '<span class="sql-warning">Could not format this SQL — the text is unchanged.</span>';
        }
        return false;
    }

    // Only touch the editor when the text actually changed. Assigning to
    // .value moves the caret to the end of the textarea, so doing it
    // needlessly would jump the cursor on already-formatted SQL. The
    // highlight layer is rebuilt to match, exactly as every other edit does.
    if (formatted !== source) {
        editor.value = formatted;
        updateSqlHighlight();
    }

    return true;
}

// Handle Enter in the SQL editor when the Format setting is on: if the caret
// sits just past the ";" that ends a statement, reformat that one statement
// before inserting the newline. Formatting only completed statements is what
// makes this usable while typing — a half-written statement is never reflowed
// out from under the caret, because a statement the user is still typing has
// no ";" on it yet.
//
// Returns true if it handled the key (reformatting and inserting the newline
// itself), false to let the editor insert the newline normally.
//
// Only the one statement just completed is rewritten, not the whole editor.
// Formatting everything on every Enter would be simpler, but it would reflow
// text elsewhere in the buffer that the user may have laid out by hand.
//
// A textarea reports the caret through two numbers: .selectionStart and
// .selectionEnd, each an offset into .value. When nothing is selected the two
// are equal and both give the caret position; when text is selected they mark
// its two ends. Assigning to them moves the caret.
function formatSqlOnEnter(editor) {
    const text  = editor.value;
    const caret = editor.selectionStart;

    // If the two differ, the user has text selected and Enter is about to
    // replace it. Leave that alone — this path only handles typing forward.
    if (editor.selectionEnd !== caret) return false;

    const tokens = sqlFormatTok(text);
    if (tokens === null) return false;

    // -- Pass 1: is the caret sitting just past the end of a statement? ----
    //
    // Walk the tokens that lie entirely before the caret. `depth` counts
    // parentheses so that a ";" inside them (which would not be a statement
    // end) is ignored, and `endsHere` records whether the most recent
    // significant token was a top-level ";". Because it is reset to false by
    // every other token, it can only still be true at the end if a ";" was
    // genuinely the last thing before the caret.
    let depth      = 0;
    let stmtStart  = 0;   // offset just past that ";"
    let lastSig    = null;
    let endsHere   = false;

    for (const t of tokens) {
        // Stop at the first token that extends past the caret: `t.pos` is
        // where the token starts and adding its length gives where it ends,
        // so this keeps only tokens lying wholly behind the caret.
        if (t.pos + t.value.length > caret) break;
        if (t.type === 'ws' || t.type === 'comment') continue;

        if (t.type === 'op' && t.value === '(') depth++;
        else if (t.type === 'op' && t.value === ')') depth--;
        else if (t.type === 'op' && t.value === ';' && depth === 0) {
            stmtStart = t.pos + 1;
            endsHere  = true;
            lastSig   = t;
            continue;   // skip the reset below, so endsHere survives
        }

        lastSig  = t;
        endsHere = false;
    }

    if (!endsHere || lastSig === null) return false;

    // Everything between that ";" and the caret must be whitespace — if the
    // user has typed the start of the next statement already, this is not the
    // moment to reformat.
    if (text.slice(stmtStart, caret).trim() !== '') return false;

    // -- Pass 2: where did this statement begin? ---------------------------
    //
    // The same walk again, this time stopping before the ";" found above and
    // remembering the position just past the ";" before *that* one. If there
    // is no earlier ";", prevEnd stays 0 and the statement begins at the top
    // of the editor.
    let prevEnd = 0;
    depth       = 0;

    for (const t of tokens) {
        if (t.pos >= stmtStart - 1) break;
        if (t.type === 'ws' || t.type === 'comment') continue;

        if (t.type === 'op' && t.value === '(') depth++;
        else if (t.type === 'op' && t.value === ')') depth--;
        else if (t.type === 'op' && t.value === ';' && depth === 0) prevEnd = t.pos + 1;
    }

    // -- Rebuild the text with just that statement reformatted -------------
    //
    // `region` is the statement including whatever blank space separated it
    // from the previous one. That separation is the user's, not the
    // formatter's, so it is split off and put back untouched: .match() on the
    // pattern /^\s*/ ("any run of whitespace at the very start") returns an
    // array whose [0] entry is the matched text, which is '' when the
    // statement starts immediately.
    const region    = text.slice(prevEnd, stmtStart);
    const lead      = region.match(/^\s*/)[0];
    const formatted = formatSqlText(region.slice(lead.length));

    if (formatted === null) return false;

    // Everything up to and including the reformatted statement, plus the
    // newline this keypress is meant to insert.
    const head = text.slice(0, prevEnd) + lead + formatted + '\n';

    editor.value = head + text.slice(caret);

    // `a = b = value` assigns to both — collapsing the selection to a plain
    // caret and placing it at the end of `head`, which is the start of the
    // fresh line the user just asked for.
    editor.selectionStart = editor.selectionEnd = head.length;

    updateSqlHighlight();

    return true;
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
    // Step 0: strip comments. Rule 3 below joins continuation lines with a
    // space, which would pull whatever follows a trailing "--" comment onto
    // the same line and comment it out. Removing comments up front avoids
    // that entirely, and matters more now that the formatter routinely
    // produces multi-line statements. sqlFormatTok() knows which "--", "//",
    // "#" and "/* */" runs are real comments and which are just characters
    // inside a string literal; if it cannot tokenize the text at all, fall
    // through with the original and let the line-level rules below cope.
    const tokens = sqlFormatTok(text);

    if (tokens !== null) {
        // .map() builds a new array by calling the given function once per
        // token and collecting what it returns; .join('') then glues those
        // pieces back into one string. Since sqlFormatTok is lossless,
        // returning t.value unchanged for every token would reproduce the
        // original text exactly — so replacing just the comment tokens edits
        // them out and leaves everything else byte for byte as it was.
        text = tokens.map(t => {
            if (t.type !== 'comment') return t.value;
            // A block comment becomes whitespace of the same shape, so it
            // still separates the tokens on either side of it and any lines
            // it spanned stay separate — without this, "a/*x*/b" would become
            // the single word "ab". A line comment always runs to a newline,
            // so dropping it outright cannot join two tokens.
            //
            // In the pattern, [^\n] means "any character except a newline"
            // (a leading ^ inside brackets negates the set), and the trailing
            // g flag makes .replace() act on every match rather than only the
            // first — so every character but the line breaks becomes a space.
            return t.value.startsWith('/*') ? t.value.replace(/[^\n]/g, ' ') : '';
        }).join('');
    }

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
    const dsn = document.getElementById('sql-dsn-picker').value;

    if (!dsn)  { document.getElementById('sql-status').textContent = 'Select a DSN first.'; return; }

    // When the Format setting is on, tidy the editor before running, the same
    // way the Code tab reformats its source ahead of a Run. Formatting is
    // quiet here and never blocks the run: SQL it cannot format is submitted
    // exactly as the user typed it.
    if (codeFormatEnabled) formatSqlEditor(true);

    const text = document.getElementById('sql-editor').value;

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
    document.getElementById('sql-elapsed').textContent = '';

    try {
        const token = getToken();
        // Note the method is now POST instead of PUT, because the server's SQL 
        // endpoint is not idempotent: it may create or modify data, and the same
        // request repeated could have different effects. This is a change from
        // the old specification that used PUT for SQL execution, which is now 
        // considered incorrect.
        const res = await fetch('dsns/' + encodeURIComponent(dsn) + '/tables/@sql', {
            method:  'POST',
            headers: {
                'Content-Type':  'application/json',
                'Authorization': token ? 'Bearer ' + token : '',
            },
            body: JSON.stringify(stmts),
        });

        if (res.status === 401) {
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
        if (data.elapsed) {
            document.getElementById('sql-elapsed').textContent = 'Ran in ' + data.elapsed;
        }
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
// SQL Generate overlay — turns a natural-language prompt into SQL via
// POST /dsns/{dsn}/tables/@generate.
// ==========================================================================

// AbortController for the in-flight /tables/@generate request, if any. Set by
// submitSqlGenerate() just before the fetch and aborted by hideSqlGenerate()
// so that clicking Cancel on a slow request actually cancels it, instead of
// letting it complete later and silently insert its result into the editor
// after the user has already moved on.
let _sqlGenerateController = null;

// Open the overlay. A DSN must already be selected, since the endpoint needs
// it to describe the available tables/columns to the AI model.
function showSqlGenerate() {
    const dsn = document.getElementById('sql-dsn-picker').value;
    if (!dsn) {
        document.getElementById('sql-status').textContent = 'Select a DSN before using Generate.';
        return;
    }

    document.getElementById('sql-generate-title').textContent = 'Generate SQL for ' + dsn.toUpperCase();
    document.getElementById('sql-generate-error').textContent = '';
    document.getElementById('sql-generate-prompt').value = '';
    document.getElementById('sql-generate-overlay').style.display = 'flex';
    document.getElementById('sql-generate-prompt').focus();
}

// Close the overlay and clear any error left over from a failed attempt.
// Aborts a still-in-flight /tables/@generate request, if any, so its response is
// never applied to the editor after the overlay has been dismissed.
function hideSqlGenerate() {
    if (_sqlGenerateController) {
        _sqlGenerateController.abort();
        _sqlGenerateController = null;
    }
    document.getElementById('sql-generate-overlay').style.display = 'none';
    document.getElementById('sql-generate-error').textContent = '';
}

// Send the prompt to the server. On success, insert the generated SQL into
// the editor and close the overlay. On failure, show the error in the
// overlay and leave it open so the user can revise the prompt or Cancel.
async function submitSqlGenerate() {
    const dsn      = document.getElementById('sql-dsn-picker').value;
    const prompt   = document.getElementById('sql-generate-prompt').value.trim();
    const errorEl  = document.getElementById('sql-generate-error');
    const submitEl = document.getElementById('sql-generate-submit-btn');

    errorEl.textContent = '';

    if (!dsn) {
        errorEl.textContent = 'Select a DSN before using Generate.';
        return;
    }
    if (!prompt) {
        errorEl.textContent = 'Describe the SQL statement you want.';
        return;
    }

    submitEl.disabled = true;

    const controller = new AbortController();
    _sqlGenerateController = controller;

    try {
        const token = getToken();
        const res = await fetch('dsns/' + encodeURIComponent(dsn) + '/tables/@generate', {
            method:  'POST',
            headers: {
                'Content-Type':  'application/json',
                'Authorization': token ? 'Bearer ' + token : '',
            },
            body:   JSON.stringify(prompt),
            signal: controller.signal,
        });

        if (res.status === 401) {
            clearToken();
            hideSqlGenerate();
            showLogin('Session expired. Please sign in again.');
            return;
        }

        const data = await res.json().catch(() => ({}));

        if (!res.ok) {
            errorEl.textContent = stripErrorPrefix(data.msg || 'HTTP ' + res.status);
            return;
        }

        insertSqlGenerate(data.sql, prompt);
        hideSqlGenerate();
    } catch (e) {
        // AbortError means the user clicked Cancel while the request was in
        // flight — hideSqlGenerate() already dismissed the overlay and reset
        // its error text, so there's nothing left to report here.
        if (e.name !== 'AbortError') {
            errorEl.textContent = 'Network error. Please try again.';
        }
    } finally {
        submitEl.disabled = false;
        _sqlGenerateController = null;
    }
}

// Wrap `text` into "-- "-prefixed comment lines no wider than `width`
// characters, breaking on word boundaries.
function wrapSqlComment(text, width) {
    const prefix       = '-- ';
    const maxTextWidth = width - prefix.length;
    const words        = text.split(/\s+/).filter(Boolean);
    const lines        = [];
    let line            = '';

    for (const word of words) {
        if (line && (line.length + 1 + word.length) > maxTextWidth) {
            lines.push(prefix + line);
            line = word;
        } else {
            line = line ? line + ' ' + word : word;
        }
    }
    if (line) lines.push(prefix + line);

    return lines.join('\n');
}

// Insert a comment reproducing the prompt (folded to 80-character-wide lines),
// followed by the generated SQL, at the cursor (replacing any current
// selection).
function insertSqlGenerate(sql, prompt) {
    const editor = document.getElementById('sql-editor');
    const start  = editor.selectionStart;
    const end    = editor.selectionEnd;
    const before = editor.value.substring(0, start);
    const after  = editor.value.substring(end);

    const sep = (before.length > 0 && !before.endsWith('\n')) ? '\n' : '';

    let statement = sql.trim();
    if (!/;\s*$/.test(statement)) statement += ';';

    const comment  = wrapSqlComment(prompt, 80);
    const inserted = sep + comment + '\n' + statement + '\n';

    editor.value = before + inserted + after;
    editor.selectionStart = editor.selectionEnd = start + inserted.length;
    editor.focus();
    updateSqlHighlight();
}

