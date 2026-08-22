// dashboard-code.js
// The Code tab: the Ego source editor, syntax highlighting, run/debug/trace
// execution, the debugger panel, and the interactive console.
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

// The Format and Console settings live in the main Settings sheet (hamburger
// menu) rather than the Code tab's own toolbar, so their state must be
// readable and settable before the Code tab has ever been opened (and thus
// before initCodeEditor() has run). Declared here at top level rather than
// inside initCodeEditor()'s closure for that reason.
//
// Trace is deliberately NOT one of these persisted settings: it is an execution
// mode selected from the Run/Debug/Trace dropdown (a peer of Run and Debug),
// tracked by codeRunMode in initCodeEditor() and reset to Run on page reload.

// Format toggle — when on, the editor is reformatted via POST /admin/format
// (the AST-based formatter) immediately before each Run/Debug. Persisted
// across sessions via the getCodeFormat/setCodeFormat cookie helpers.
let codeFormatEnabled = getCodeFormat();

// Console visibility — toggles the 'hide-console' class on #code-ui so CSS
// rules keyed on that class collapse the divider and console pane, letting
// the editor/output panels fill the remaining height. #code-ui is a static
// element present in the HTML from page load, so this works even before the
// Code tab has ever been opened.
function applyConsoleVisible(visible) {
    document.getElementById('code-ui').classList.toggle('hide-console', !visible);
}

// Apply the saved preference immediately at page load, not just when the
// Code tab is first opened, so the layout is already correct the first time
// the user visits it.
applyConsoleVisible(getShowConsole());

// Save the Code editor contents to a .ego file. The counterpart of the SQL
// tab's saveSqlFile(), sharing the same saveTextFile() implementation.
//
// Declared at top level rather than inside initCodeEditor() so it sits beside
// its SQL equivalent and can be called without the Code tab having been
// opened; the button that invokes it is wired up in initCodeEditor() with the
// rest of the Code tab's controls.
async function saveCodeFile() {
    const text = document.getElementById('code-editor')?.value || '';

    await saveTextFile(text, 'program.ego', 'Ego source file', ['.ego', '.txt']);
}

// Save the Code tab's Output pane contents to a .txt file, using the same
// saveTextFile() mechanism as saveCodeFile() and the SQL tab's saveSqlFile().
async function saveCodeOutput() {
    const text = document.getElementById('code-output-pane')?.textContent || '';

    await saveTextFile(text, 'output.txt', 'Text file', ['.txt']);
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
    const codeClearEditorBtn  = document.getElementById('code-clear-editor-btn');
    const codeClearOutputBtn  = document.getElementById('code-clear-output-btn');
    const codeSaveOutputBtn   = document.getElementById('code-save-output-btn');
    const codeElapsed         = document.getElementById('code-elapsed');
    const codeClearConsoleBtn = document.getElementById('code-clear-console-btn');
    const codeOpenFileBtn     = document.getElementById('code-open-file-btn');
    const codeSaveFileBtn     = document.getElementById('code-save-file-btn');
    const codeFormatBtn       = document.getElementById('code-format-btn');
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
    // codeRunMode tracks the current sticky execution mode. The ▾ arrow opens a
    // dropdown with three items:
    //   "▶ Run"    — normal execution
    //   "🐛 Debug" — execution under the interactive debugger
    //   "👣 Trace" — normal execution with trace logging enabled
    // Selecting any item makes it the sticky mode: it updates the main button
    // label and immediately runs, and the main button then repeats that mode
    // until another is chosen. Trace is a peer of Run and Debug in every way
    // except that it (like the others) is not persisted across a page reload.
    // -----------------------------------------------------------------------
    let codeRunMode = 'run'; // 'run' | 'debug' | 'trace'

    // 1-based line number the debugger is currently paused on; 0 = no highlight.
    let codeDebugLine = 0;

    // Toggle the dropdown when the ▾ arrow is clicked.
    codeRunArrow.addEventListener('click', e => {
        e.stopPropagation(); // prevent the document click handler from closing it immediately
        codeRunDrop.classList.toggle('open');
    });

    // Close the dropdown when the user clicks anywhere outside it.
    document.addEventListener('click', () => codeRunDrop.classList.remove('open'));

    // Map from data-mode value to the main button's icon and label. Kept as
    // two separate maps (rather than one combined string) because the main
    // button's markup holds the icon and label in their own
    // <span class="btn-icon">/<span class="btn-label"> -- overwriting the
    // whole button's content with textContent would destroy those spans and
    // silently break the Toolbar Buttons setting for this button afterward.
    const modeIconMap  = { run: '\u25B6',  debug: '\u{1F41B}', trace: '\u{1F463}' };
    const modeLabelMap = { run: 'Run',     debug: 'Debug',     trace: 'Trace' };

    // Wire each dropdown item: make the chosen mode the sticky mode, reflect it
    // in the main button label, and immediately run. Trace behaves exactly like
    // Run and Debug here -- it stays selected until the user picks another mode
    // (it is just never persisted across a page reload).
    codeRunDrop.querySelectorAll('.code-run-item').forEach(item => {
        item.addEventListener('click', e => {
            e.stopPropagation();
            codeRunDrop.classList.remove('open');

            codeRunMode = item.dataset.mode || 'run';

            // Reflect the selected mode in the main button's icon and label,
            // updating each span in place rather than replacing the button's
            // whole content.
            codeRunBtn.querySelector('.btn-icon').textContent  = modeIconMap[codeRunMode]  || '\u25B6';
            codeRunBtn.querySelector('.btn-label').textContent = modeLabelMap[codeRunMode] || 'Run';

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

    // If the editor is empty on first display (no file opened, nothing
    // restored), seed it with a minimal sample program so the Code tab
    // isn't a blank void the first time a user opens it.
    if (codeEditor.value === '') {
        codeEditor.value =
            'package main\n' +
            '\n' +
            'import "fmt"\n' +
            '\n' +
            'func main() {\n' +
            '    fmt.Println("Hello, world")\n' +
            '}\n';
    }

    // Populate line numbers and highlighting on first display.
    updateLineNumbers();
    updateHighlight();

    // -----------------------------------------------------------------------
    // Clear buttons
    // -----------------------------------------------------------------------

    // Format button — reformat the editor on demand, independently of the
    // Format setting (which only controls reformatting before a run).
    //
    // Unlike the SQL formatter, this one is not local: formatEditorCode()
    // posts to the server and waits for the reply, so the handler is declared
    // `async` and uses `await` to pause until that reply arrives. The button
    // is disabled across the wait so a second click cannot start a second
    // request whose (older) reply might land after the first and overwrite it.
    //
    // The `finally` block runs whether the await succeeded or threw, so the
    // button can never be left permanently disabled by a failure.
    codeFormatBtn.addEventListener('click', async () => {
        codeFormatBtn.disabled = true;

        try {
            await formatEditorCode();
        } finally {
            codeFormatBtn.disabled = false;
        }
    });

    // Open button — click it to trigger the hidden file picker.
    codeOpenFileBtn.addEventListener('click', () => codeFileInput.click());

    // Save button — download the editor contents as a .ego file.
    codeSaveFileBtn.addEventListener('click', () => saveCodeFile());

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

    codeSaveOutputBtn.addEventListener('click', () => saveCodeOutput());

    codeClearDebugBtn.addEventListener('click', () => {
        codeDebugOutput.textContent = '';
    });

    codeClearConsoleBtn.addEventListener('click', () => {
        codeConsoleHistory.innerHTML = '';
    });

    // Console visibility and Format are set from the main Settings sheet
    // (hamburger menu) -- see applyConsoleVisible() and the
    // setting-format/setting-console wiring near showSettings(). Trace is not a
    // persisted setting; it is an execution mode in the Run/Debug/Trace dropdown.

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
        const res = await fetch('admin/run', {
            method:  'POST',
            headers: {
                'Content-Type':  'application/json',
                'Authorization': token ? 'Bearer ' + token : '',
            },
            body: JSON.stringify(payload),
        });

        if (res.status === 401) {
            clearToken();
            showLogin('Session expired. Please sign in again.');
            throw new Error('auth');
        }

        if (!res.ok) {
            throw new Error('HTTP error ' + res.status);
        }

        return res.json();
    }

    // Post the editor's current source to POST /admin/format (the AST-based
    // formatter added alongside the "Format" toggle) and, on success, replace
    // the editor content with the canonically reformatted version.
    //
    // A parse error, or any network/auth failure, is treated as non-fatal:
    // the editor is left untouched and the caller (runEditorCode) proceeds to
    // Run/Debug with the original source. This formatter is newer than the
    // main compiler and may not yet cover every construct the compiler
    // accepts, so a formatting failure must never block an otherwise-valid
    // run -- the compiler's own error reporting still applies normally to
    // whatever gets sent.
    async function formatEditorCode() {
        const token = getToken();

        let data;

        try {
            const res = await fetch('admin/format', {
                method:  'POST',
                headers: {
                    'Content-Type':  'application/json',
                    'Authorization': token ? 'Bearer ' + token : '',
                },
                body: JSON.stringify({ code: codeEditor.value }),
            });

            if (res.status === 401) {
                clearToken();
                showLogin('Session expired. Please sign in again.');
                return;
            }

            if (!res.ok) return;

            data = await res.json();
        } catch (e) {
            return; // network error -- leave the editor untouched
        }

        if (data.error || !data.formatted) return; // parse error -- leave the editor untouched

        if (data.formatted !== codeEditor.value) {
            codeEditor.value = data.formatted;
            updateLineNumbers();
            updateHighlight();
        }
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

    // runEditorCode executes the editor contents in the current sticky mode
    // (codeRunMode). 'debug' starts an interactive debug session; 'run' and
    // 'trace' both do a normal run, with 'trace' additionally requesting trace
    // logging from the server.
    async function runEditorCode() {
        const trace = codeRunMode === 'trace';

        // When the Format toggle is on, reformat the editor contents via the
        // server's AST-based formatter before running -- formatEditorCode
        // leaves the editor untouched on any parse/network error, so a run
        // always proceeds with either the reformatted or the original source.
        if (codeFormatEnabled) {
            await formatEditorCode();
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
            if (trace) payload.trace = true;

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

    // The main button repeats the current sticky mode (Run, Debug, or Trace).
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
            const res = await fetch('admin/run', {
                method:  'POST',
                headers: {
                    'Content-Type':  'application/json',
                    'Authorization': token ? 'Bearer ' + token : '',
                },
                body: JSON.stringify({ code, console: true, session: codeSessionUUID }),
            });

            if (res.status === 401) {
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

