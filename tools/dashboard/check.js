// check.js — load the Ego admin dashboard in a headless DOM and verify it
// actually starts up.
//
// WHY THIS EXISTS
//
// The dashboard's JavaScript is split across several files (dashboard-core.js,
// dashboard-admin.js, ... ) that dashboard.html loads in order. They are plain
// <script> tags, not modules, so they share one global scope -- and a function
// declaration is hoisted only within its own file. Code that runs immediately
// when a file is evaluated may therefore only reference names already declared
// by an earlier file. Deferred code (event handlers, callbacks, timers) is
// unrestricted, because by the time it runs every file has loaded.
//
// Breaking that rule throws a ReferenceError during page load, which silently
// kills the rest of that file. The dashboard then renders its shell and does
// nothing, with no clue in the UI as to why. Neither a syntax check nor the Go
// test suite can see this: every file is valid JavaScript on its own, and the
// error exists only in the relationship between files at load time. The only
// way to find it is to load the page. That happened once for real, when the
// tabLoaders map was left in a file that loaded before two of the loader
// functions it names.
//
// HOW IT LOADS THE PAGE
//
// A throwaway HTTP server on a random local port serves the real asset files
// and answers the dashboard's API calls with canned JSON. jsdom is pointed at
// it and fetches everything over HTTP, exactly as a browser would.
//
// Serving the scripts rather than pasting them into the HTML is not incidental.
// dashboard-admin.js contains the text "</script>" inside a comment about XSS
// escaping; inlining that file would make the HTML parser end the script
// element right there, silently discarding the rest of the file and everything
// declared in it. A harness that inlines reports failures the browser would
// never see. Fetching each file as its own resource cannot hit that, and is
// what the browser does anyway.
//
// WHAT IT CHECKS
//
//   1. Every script named by dashboard.html evaluates to completion. A
//      sentinel after each one records that it finished, so a failure names
//      the file rather than merely reporting that something broke.
//   2. After startup a tab is highlighted and its content is displayed, for
//      both a first visit and a reload by a logged-in user -- different paths
//      through the startup logic.
//   3. Typing into the Code editor reaches its syntax-highlight layer. That
//      layer sits behind a deliberately transparent <textarea>, so if the
//      input handler is not connected the caret moves but nothing is ever
//      visible. Checking that the layer is merely non-empty would not do: an
//      untouched editor is empty for legitimate reasons.
//
// WHAT IT DOES NOT CHECK
//
// Nothing about layout or styling -- jsdom does not render. And the API
// replies are canned, so a loader that mishandles real server data will not be
// caught here. Errors raised by the page's own code are reported as notes
// rather than failures for that reason.
//
// USAGE
//
//   node tools/dashboard/check.js            # check lib/assets/dashboard
//   node tools/dashboard/check.js <dir>      # check another copy of them
//
// Exits 0 if the dashboard starts, 1 if it does not.

const { JSDOM, VirtualConsole } = require('jsdom');
const http = require('http');
const fs = require('fs');
const path = require('path');

const assetDir = process.argv[2] ||
    path.join(__dirname, '..', '..', 'lib', 'assets', 'dashboard');

// Errors thrown inside the page's own promise chains surface here rather than
// through jsdom's virtual console. Without this handler Node would terminate
// the process on the first one, before this script could report anything --
// which is exactly what happens for the failure this check exists to catch,
// since the dashboard's startup path is async.
const rejections = [];

process.on('unhandledRejection', reason => {
    const error = reason instanceof Error ? reason : new Error(String(reason));
    const where = (error.stack || '').split('\n')[1] || '';

    rejections.push(error.message + (where ? ' (' + where.trim() + ')' : ''));
});

// Canned replies. Any endpoint not named here gets an empty collection, which
// is enough for the loaders to run without throwing.
const apiReplies = {
    '/services/up': { server: 'check', id: 'uuid', since: 'Mon Jan  2 15:04:05 EST 2026' },
    '/services/admin/server': { server: 'check', id: 'uuid', since: 'Mon Jan  2 15:04:05 EST 2026' },
};

const contentTypes = {
    '.js': 'text/javascript',
    '.css': 'text/css',
    '.png': 'image/png',
    '.html': 'text/html',
};

// ---------------------------------------------------------------------------
// The page
// ---------------------------------------------------------------------------

// Read dashboard.html and add a sentinel after each dashboard script. The
// script tags themselves are left alone so the browser fetches each file as
// its own resource.
//
// A sentinel is its own tiny inline script: when a script throws, the browser
// abandons only that script and carries on with the next one, so the sentinel
// belonging to a failing file never runs. Putting the marker inside the file's
// own script element would not work -- the throw would skip it too, but so
// would anything else, and it would change how the file's declarations scope.
function buildPage(scriptNames, cookies) {
    let html = fs.readFileSync(path.join(assetDir, 'dashboard.html'), 'utf8')
        .replace(/__EGO_LANG__/g, 'en');

    html = html.replace(
        /<script src="([^"]*\/(dashboard-[A-Za-z0-9_-]+\.js))"><\/script>/g,
        (tag, _src, file) => {
            scriptNames.push(file);

            return tag + '\n<script>window.__loaded.push(' + JSON.stringify(file) + ');</script>';
        });

    // Runs before any dashboard script: sets up the sentinel array, a fetch
    // shim, and any cookies the scenario needs.
    //
    // jsdom implements XMLHttpRequest but not fetch, so the dashboard's API
    // calls would fail with "fetch is not defined". The shim below forwards
    // them over XHR to the same stand-in server, which keeps the requests real
    // rather than faking the responses in-process.
    const fetchShim = [
        'window.fetch = function (url, opts) {',
        '  opts = opts || {};',
        '  return new Promise(function (resolve, reject) {',
        '    var xhr = new XMLHttpRequest();',
        '    xhr.open(opts.method || "GET", url, true);',
        '    for (var k in (opts.headers || {})) { xhr.setRequestHeader(k, opts.headers[k]); }',
        '    xhr.onload = function () {',
        '      resolve({',
        '        ok: xhr.status >= 200 && xhr.status < 300,',
        '        status: xhr.status,',
        '        json: function () { return Promise.resolve(JSON.parse(xhr.responseText || "{}")); },',
        '        text: function () { return Promise.resolve(xhr.responseText); },',
        '        headers: { get: function (n) { return xhr.getResponseHeader(n); } }',
        '      });',
        '    };',
        '    xhr.onerror = function () { reject(new Error("network error")); };',
        '    xhr.send(opts.body || null);',
        '  });',
        '};',
    ].join('');

    const preamble = '<script>window.__loaded = [];' + fetchShim +
        cookies.map(c => 'document.cookie=' + JSON.stringify(c + '; path=/') + ';').join('') +
        '</script>';

    return html.replace('</head>', preamble + '</head>');
}

// ---------------------------------------------------------------------------
// The stand-in server
// ---------------------------------------------------------------------------

function startServer(scriptNames, cookies) {
    const server = http.createServer((req, res) => {
        const url = req.url.split('?')[0];

        // The page itself.
        if (url === '/' || url === '/ui' || url === '/assets/dashboard/dashboard.html') {
            // The page may be requested more than once (jsdom follows the
            // document, and a reload would re-enter here); collect the script
            // names fresh each time rather than appending to the last run's.
            scriptNames.length = 0;

            const body = buildPage(scriptNames, cookies);

            res.writeHead(200, { 'Content-Type': 'text/html' });
            res.end(body);

            return;
        }

        // Asset files, served verbatim from disk.
        if (url.startsWith('/assets/dashboard/')) {
            const file = path.join(assetDir, path.basename(url));

            fs.readFile(file, (err, data) => {
                if (err) {
                    res.writeHead(404).end('not found');

                    return;
                }

                res.writeHead(200, { 'Content-Type': contentTypes[path.extname(file)] || 'text/plain' });
                res.end(data);
            });

            return;
        }

        // Everything else is treated as an API call.
        const reply = apiReplies[url] || { items: [], count: 0, rows: [], columns: [], status: {} };

        res.writeHead(200, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify(reply));
    });

    return new Promise(resolve => {
        // Port 0 asks the OS for any free port, so concurrent runs cannot
        // collide with each other or with a server already on a fixed port.
        server.listen(0, '127.0.0.1', () => resolve(server));
    });
}

// ---------------------------------------------------------------------------
// One scenario
// ---------------------------------------------------------------------------

async function run(name, cookies, settleMs) {
    const scriptNames = [];
    const failures = [];
    const notes = [];

    rejections.length = 0;

    const server = await startServer(scriptNames, cookies);
    const port = server.address().port;

    const virtualConsole = new VirtualConsole();

    virtualConsole.on('jsdomError', e => failures.push('uncaught: ' + e.message));
    virtualConsole.on('error', (...args) => notes.push(args.join(' ')));

    const dom = await JSDOM.fromURL(`http://127.0.0.1:${port}/`, {
        runScripts: 'dangerously',
        resources: 'usable',
        virtualConsole,
        pretendToBeVisual: true,
    });

    // Give the startup code's promise chain time to finish. Nothing here waits
    // on real I/O, so a short settle is enough.
    await new Promise(resolve => setTimeout(resolve, settleMs));

    const doc = dom.window.document;
    const loaded = dom.window.__loaded || [];

    for (const rejection of rejections) {
        failures.push('unhandled error during startup: ' + rejection);
    }

    if (scriptNames.length === 0) {
        failures.push('dashboard.html named no scripts; has the markup changed?');
    }

    for (const script of scriptNames) {
        if (!loaded.includes(script)) {
            failures.push(script + ' did not finish evaluating');
        }
    }

    if (!doc.querySelector('.tab-container > div.active-tab')) {
        failures.push('no tab is highlighted after startup');
    }

    const visible = Array.from(doc.querySelectorAll('.tab-content'))
        .filter(el => el.style.display && el.style.display !== 'none');

    if (visible.length === 0) {
        failures.push('no tab content is displayed after startup');
    }

    return { name, scriptNames, loaded, failures, notes, dom, server };
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

(async function main() {
    // A first visit with no cookies, and a reload by a logged-in admin whose
    // last tab was the Code tab. These take different branches through the
    // startup logic, and only the second populates the code editor.
    const scenarios = [
        { name: 'first visit (no session)', cookies: [] },
        {
            name: 'reload by a logged-in admin',
            cookies: [
                'ego_dashboard_token=check-token',
                'ego_dashboard_remember=1',
                'ego_dashboard_role=admin',
                'ego_dashboard_identity=admin',
                'ego_dashboard_tab=code',
            ],
            checkEditor: true,
        },
    ];

    let failed = false;

    for (const scenario of scenarios) {
        const result = await run(scenario.name, scenario.cookies, 500);

        if (scenario.checkEditor) {
            // An empty editor legitimately has an empty highlight layer, so
            // simulate typing and check the text reaches the layer. That is the
            // wiring that actually matters: the textarea above the layer is
            // deliberately transparent, so if the input handler is not
            // connected the caret moves but nothing is ever visible.
            const win = result.dom.window;
            const editor = win.document.getElementById('code-editor');
            const layer = win.document.getElementById('code-highlight-layer');

            if (!editor || !layer) {
                result.failures.push('the Code tab editor or its highlight layer is missing');
            } else {
                editor.value = 'func main() { print "hello" }';
                editor.dispatchEvent(new win.Event('input', { bubbles: true }));

                if (!layer.innerHTML.includes('main')) {
                    result.failures.push(
                        'typing into the Code editor did not reach the highlight layer; ' +
                        'text would be invisible because the textarea above it is transparent');
                }
            }
        }

        result.dom.window.close();
        result.server.close();

        console.log('\n' + result.name);
        console.log('  scripts        : ' + result.loaded.length + '/' + result.scriptNames.length + ' evaluated');

        for (const note of result.notes) {
            console.log('  note           : ' + note.split('\n')[0]);
        }

        if (result.failures.length === 0) {
            console.log('  result         : ok');
        } else {
            failed = true;

            for (const failure of result.failures) {
                console.log('  FAILED         : ' + failure);
            }
        }
    }

    console.log(failed
        ? '\nDASHBOARD CHECK FAILED - the dashboard does not start up correctly.'
        : '\nDashboard check passed.');

    process.exit(failed ? 1 : 0);
})();
