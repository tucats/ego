// app.js
// This contains the main logic for the Ego web-app. It handles
// the code editor, output pane, and file operations.

// ---------------------------------------------------------------------------
// DOM element references
// Cache references to every element we'll interact with. Doing this once at
// startup is faster than calling getElementById repeatedly in event handlers.
// ---------------------------------------------------------------------------
const editor      = document.getElementById('editor');        // The <textarea> where the user writes code
const lineNumbers = document.getElementById('line-numbers');  // Sidebar div that shows "1\n2\n3\n..."
const output      = document.getElementById('output-pane');   // Div that shows program output (or errors)
const runBtn      = document.getElementById('run-btn');       // "Run" button in the header
const menuBtn     = document.getElementById('menu-btn');      // Hamburger (☰) button that opens the menu
const menuDrop    = document.getElementById('menu-dropdown'); // The dropdown menu panel itself
const openBtn     = document.getElementById('open-btn');      // "Open…" menu item
const saveBtn       = document.getElementById('save-btn');    // "Save Source…" menu item
const saveOutputBtn = document.getElementById('save-output-btn'); // "Save Output…" menu item
const fileInput   = document.getElementById('file-input');    // Hidden <input type="file"> used for Open
const clearBtn    = document.getElementById('clear-btn');     // "Clear Output" menu item
const quitBtn     = document.getElementById('quit-btn');      // "Quit" menu item — stops the server
const clearEditorBtn  = document.getElementById('clear-editor-btn');  // ✕ button on the editor pane label
const clearOutputBtn  = document.getElementById('clear-output-btn');  // ✕ button on the output pane label
const spinner     = document.getElementById('spinner');       // Animated spinner shown while code runs
const divider     = document.getElementById('divider');       // Draggable bar between the two panes
const hlLayer     = document.getElementById('highlight-layer'); // Syntax-highlight <pre> behind the textarea
const runArrow    = document.getElementById('run-arrow');     // ▾ button that opens the run-mode dropdown
const runDrop     = document.getElementById('run-dropdown');  // The run-mode dropdown panel

// ---------------------------------------------------------------------------
// Heartbeat — keep the server alive while this tab is open.
//
// The Go server shuts itself down if it stops receiving pings, so this
// interval fires a lightweight POST /ping every 2 seconds. The interval
// value must be shorter than the server-side timeout (currently ~3 s).
// The pingHandle is saved so we can cancel it before calling /quit.
// ---------------------------------------------------------------------------
const pingIntervalMs = 2000;
const pingHandle = setInterval(() => {
  fetch('/ping', { method: 'POST' }).catch(() => {});
}, pingIntervalMs);

// ---------------------------------------------------------------------------
// Hamburger menu
// ---------------------------------------------------------------------------

// Toggle the dropdown open/closed when the ☰ button is clicked.
// stopPropagation prevents the document-level click handler (below) from
// immediately closing the menu on the same click.
menuBtn.addEventListener('click', e => {
  e.stopPropagation();
  menuDrop.classList.toggle('open');
});

// Close both dropdowns whenever the user clicks anywhere outside them.
document.addEventListener('click', () => {
  menuDrop.classList.remove('open');
  runDrop.classList.remove('open');
});

// ---------------------------------------------------------------------------
// Run split-button dropdown
// ---------------------------------------------------------------------------

// Tracks whether the next run should enable the trace logger.
let currentTrace = false;

// Toggle the run-mode dropdown on the ▾ arrow button.
runArrow.addEventListener('click', e => {
  e.stopPropagation();
  menuDrop.classList.remove('open');
  runDrop.classList.toggle('open');
});

// Each item in the dropdown sets the active mode, updates button labels, and runs.
runDrop.querySelectorAll('.run-item').forEach(item => {
  item.addEventListener('click', e => {
    e.stopPropagation();
    runDrop.classList.remove('open');

    const trace = item.dataset.trace === 'true';
    currentTrace = trace;

    // Reflect the selected mode in the main button label.
    runBtn.textContent = trace ? '\u{1F50E} Trace' : '\u25B6 Run';

    // Mark the active item.
    runDrop.querySelectorAll('.run-item').forEach(i => i.classList.remove('active'));
    item.classList.add('active');

    runCode();
  });
});

// ---------------------------------------------------------------------------
// File operations — Open, Save Source, Save Output
// ---------------------------------------------------------------------------

// "Open…" — programmatically click the hidden file input to show the OS
// file picker. The actual reading happens in the fileInput 'change' handler.
openBtn.addEventListener('click', () => {
  menuDrop.classList.remove('open');
  fileInput.click();
});

// "Save Source…" — create a temporary in-memory Blob from the editor text,
// attach it to a hidden <a> element, and trigger a click to download it.
// URL.revokeObjectURL frees the memory after the download starts.
saveBtn.addEventListener('click', () => {
  menuDrop.classList.remove('open');
  const blob = new Blob([editor.value], { type: 'text/plain' });
  const url  = URL.createObjectURL(blob);
  const a    = document.createElement('a');
  a.href     = url;
  a.download = 'program.ego';
  a.click();
  URL.revokeObjectURL(url);
});

// "Save Output…" — same download technique, but saves the output pane text.
saveOutputBtn.addEventListener('click', () => {
  menuDrop.classList.remove('open');
  const blob = new Blob([output.textContent], { type: 'text/plain' });
  const url  = URL.createObjectURL(blob);
  const a    = document.createElement('a');
  a.href     = url;
  a.download = 'output.txt';
  a.click();
  URL.revokeObjectURL(url);
});

// Read the selected file into the editor when the OS file picker is closed.
// FileReader.readAsText is asynchronous; onload fires once the file is ready.
fileInput.addEventListener('change', () => {
  const file = fileInput.files[0];
  if (!file) return;

  const reader = new FileReader();
  reader.onload = e => {
    editor.value = e.target.result;
    updateLineNumbers();
    updateHighlight();
  };
  reader.readAsText(file);

  // Reset so the same file can be opened again if needed.
  fileInput.value = '';
});

// ---------------------------------------------------------------------------
// Clear buttons
// ---------------------------------------------------------------------------

// ✕ on the editor label — wipe the editor and resync line numbers and highlighting.
clearEditorBtn.addEventListener('click', () => {
  editor.value = '';
  updateLineNumbers();
  updateHighlight();
});

// ✕ on the output label — reset the pane to its empty/idle state.
clearOutputBtn.addEventListener('click', () => {
  output.className = 'idle';
  output.textContent = '';
});

// "Clear Output" in the menu — same as the ✕ button above.
clearBtn.addEventListener('click', () => {
  menuDrop.classList.remove('open');
  output.className = 'idle';
  output.textContent = '';
});

// ---------------------------------------------------------------------------
// Resizable divider between the editor and output panes
// ---------------------------------------------------------------------------

const leftPane    = document.getElementById('left-pane');
const mainEl      = document.querySelector('main');

// When the user presses the mouse button on the divider bar, start tracking
// mouse movement to resize the left pane.
divider.addEventListener('mousedown', e => {
  e.preventDefault();
  divider.classList.add('dragging');         // CSS can style the cursor differently while dragging
  document.body.style.userSelect = 'none';   // Prevent text selection while dragging
  document.body.style.cursor = 'col-resize'; // Show a resize cursor over the whole page

  const startX     = e.clientX;                              // Mouse X position when drag started
  const startWidth = leftPane.getBoundingClientRect().width; // Left pane width when drag started

  // Called on every mousemove event while dragging. Computes the new width
  // and clamps it so neither pane can shrink below 150 px.
  function onMouseMove(e) {
    const mainWidth = mainEl.getBoundingClientRect().width;
    const newWidth  = Math.min(
      Math.max(150, startWidth + e.clientX - startX),  // clamp: minimum 150 px
      mainWidth - divider.offsetWidth - 150            // clamp: leave room for right pane
    );
    leftPane.style.flexBasis = newWidth + 'px';
  }

  // Called once when the mouse button is released. Cleans up listeners and
  // restores the default cursor and text-selection behavior.
  function onMouseUp() {
    divider.classList.remove('dragging');
    document.body.style.userSelect = '';
    document.body.style.cursor = '';
    document.removeEventListener('mousemove', onMouseMove);
    document.removeEventListener('mouseup', onMouseUp);
  }

  document.addEventListener('mousemove', onMouseMove);
  document.addEventListener('mouseup', onMouseUp);
});

// ---------------------------------------------------------------------------
// Quit — stop the Go server and disable the UI
// ---------------------------------------------------------------------------

quitBtn.addEventListener('click', async () => {
  // Stop the heartbeat so it doesn't fire after the server is gone.
  clearInterval(pingHandle);

  // Update the UI first so the browser repaints before the server stops.
  menuDrop.classList.remove('open');
  runBtn.disabled   = true;
  runArrow.disabled = true;
  menuBtn.disabled  = true;
  editor.disabled   = true;
  output.className = 'idle';
  output.textContent = 'Server stopped. Close this tab or window.';

  // Yield to the browser so the repaint happens before the network call.
  await new Promise(r => requestAnimationFrame(r));

  try {
    await fetch('/quit', { method: 'POST' });
  } catch (_) {
    // Connection close on shutdown is expected — swallow the error.
  }
});

// ---------------------------------------------------------------------------
// Syntax highlighting
//
// highlight(code) tokenizes Ego/Go source text and returns an HTML string
// with <span> elements for each token type.  The approach is a single
// left-to-right scan that greedily matches the longest applicable rule at
// each position.  Order matters: comments and strings must be checked before
// operators so that // inside a string is not treated as a comment start.
// ---------------------------------------------------------------------------

const GO_KEYWORDS = new Set([
  'break','case','chan','const','continue','default','defer','else',
  'fallthrough','for','func','go','goto','if','import','interface',
  'map','package','range','return','select','struct','switch','type','var',
]);

const GO_BUILTINS = new Set([
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
      const nl = code.indexOf('\n', i);
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

    // Rune literal  '.'  (single-char or escape)
    if (ch === "'") {
      let j = i + 1;
      if (j < n && code[j] === '\\') j += 2; else j++;
      if (j < n && code[j] === "'") j++;
      out += span('string', code.slice(i, j));
      i = j;
      continue;
    }

    // Numeric literal — integer, float, hex
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
      // Peek past whitespace to detect a following '('
      let k = j;
      while (k < n && (code[k] === ' ' || code[k] === '\t')) k++;
      if (GO_KEYWORDS.has(word)) {
        out += span('keyword', word);
      } else if (GO_BUILTINS.has(word)) {
        out += span('builtin', word);
      } else if (code[k] === '(') {
        out += span('func', word);
      } else {
        out += esc(word);
      }
      i = j;
      continue;
    }

    // Everything else — escape HTML special chars and emit verbatim
    out += esc(ch);
    i++;
  }

  // A trailing newline in <pre> is swallowed by browsers; pad with a space so
  // the highlight layer height stays in sync with the textarea when the cursor
  // is on the last (empty) line.
  return out + ' ';
}

// Rebuild the highlight layer whenever the editor content changes.
function updateHighlight() {
  hlLayer.innerHTML = highlight(editor.value);
  hlLayer.scrollTop = editor.scrollTop;
  hlLayer.scrollLeft = editor.scrollLeft;
}

// ---------------------------------------------------------------------------
// Line-number gutter
// ---------------------------------------------------------------------------

// Rebuild the line-number column to match the current number of lines in the
// editor. Also syncs the scroll position so the numbers stay aligned with
// the text when the user scrolls.
function updateLineNumbers() {
  const count = editor.value.split('\n').length;
  let text = '';
  for (let i = 1; i <= count; i++) text += i + '\n';
  lineNumbers.textContent = text;
  lineNumbers.scrollTop = editor.scrollTop;
}

// Keep line numbers and highlighting up to date as the user types or pastes.
editor.addEventListener('input', () => { updateLineNumbers(); updateHighlight(); });

// Keep the gutter and highlight layer scrolled in sync when the user scrolls.
editor.addEventListener('scroll', () => {
  lineNumbers.scrollTop = editor.scrollTop;
  hlLayer.scrollTop     = editor.scrollTop;
  hlLayer.scrollLeft    = editor.scrollLeft;
});

// Populate line numbers and highlighting immediately so they're present on first paint.
updateLineNumbers();
updateHighlight();

// ---------------------------------------------------------------------------
// Keyboard shortcuts inside the editor
// ---------------------------------------------------------------------------

editor.addEventListener('keydown', e => {
  // Tab key — insert three spaces at the cursor instead of moving focus to
  // the next element. selectionStart/End handles replacing selected text too.
  if (e.key === 'Tab') {
    e.preventDefault();
    const start = editor.selectionStart;
    const end   = editor.selectionEnd;
    editor.value = editor.value.slice(0, start) + '   ' + editor.value.slice(end);
    editor.selectionStart = editor.selectionEnd = start + 3;
  }
});

// ---------------------------------------------------------------------------
// Run code
// ---------------------------------------------------------------------------

// Wire up the Run button and the Ctrl/Cmd+Enter shortcut.
runBtn.addEventListener('click', runCode);

editor.addEventListener('keydown', e => {
  if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') runCode();
});

// Send the editor contents to the server for execution and display the result.
//
// The server endpoint is POST /run, which expects { "code": "..." } and
// responds with { "output": "...", "error": "..." }.  "error" is omitted when
// the run succeeds.  The CSS class on the output pane ('ok' / 'error' / 'idle')
// controls the background color shown to the user.
async function runCode() {
  runBtn.disabled  = true;
  runArrow.disabled = true;
  spinner.style.display = 'block';
  output.className = 'idle';
  output.textContent = 'Running\u2026';

  try {
    const payload = { code: editor.value };
    if (currentTrace) payload.trace = true;

    const res  = await fetch('/run', {
      method:  'POST',
      headers: { 'Content-Type': 'application/json' },
      body:    JSON.stringify(payload),
    });

    // A non-2xx HTTP status means something went wrong on the server side
    // before the code even ran (e.g. the server crashed or restarted).
    if (!res.ok) {
      output.textContent = 'HTTP error ' + res.status;
      output.className = 'error';
      return;
    }

    const data = await res.json();

    if (data.error) {
      // The code ran but produced a runtime or compile error.
      // Show any partial output first, then the error message.
      output.textContent = (data.output || '') + (data.output ? '\n' : '') + 'Error: ' + data.error;
      output.className = 'error';
    } else {
      output.textContent = data.output || '(no output)';
      output.className = 'ok';
    }
  } catch (err) {
    // Network-level failure (server unreachable, etc.)
    output.textContent = 'Network error: ' + err.message;
    output.className = 'error';
  } finally {
    // Always re-enable the Run buttons and hide the spinner, even on error.
    runBtn.disabled   = false;
    runArrow.disabled = false;
    spinner.style.display = 'none';
  }
}
