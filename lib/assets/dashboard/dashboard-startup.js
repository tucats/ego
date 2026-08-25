// dashboard-startup.js
// The entry point, followed by WebAuthn passkey support.
//
// This file must load LAST. Its top-level code runs immediately and calls
// into every other file, so all of them must already have been evaluated.
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
// Startup
//
// Code at the top level of a script file (outside any function) runs once,
// immediately when the browser loads the file. This is the entry point.
// ==========================================================================

// An object that maps each tab's string id to the function that loads its
// data. This lets openTab() call the right loader with a single line
// (tabLoaders[tabId]()) instead of a chain of if/else statements.
//
// It lives in this file, rather than next to openTab() in dashboard-ui.js,
// because an object literal evaluates its values immediately: every loader
// named here must already be declared by the time this line runs. The loaders
// are spread across five files, and this is the only one guaranteed to load
// after all of them. Adding a tab means adding its loader here as well.
const tabLoaders = {
    memory:  loadMemory,
    users:   loadUsers,
    dsns:    loadDsns,
    tables:  loadTables,
    data:    loadData,
    sql:     loadSql,
    tasks:   loadTasks,
    log:     loadLog,
    code:    loadCode,
};

// Fetch and cache server name/version/UUID/uptime for the Configuration
// sheet — no login needed.
loadServerInfo();

// Apply persisted preferences BEFORE loading any content so there is no
// flash of wrong theme and no spurious unauthenticated API call.
applyDarkModeSetting(getDarkMode());
applyToolbarStyle(getToolbarStyle());

// Restore the saved log filter before anything can request the log, so the
// first fetch already carries it and the funnel shows its dot from the start
// rather than only after the sheet is opened.
loadLogFilter();
updateLogFilterDot();

// Restore a saved token (if "Remember login" was on) and open the last active
// tab, but only if the server is reachable. Falls back to 'memory' and shows
// the login overlay whenever a fresh login is required.
(async function () {
    let serverUp = false;
    try {
        const res = await fetch('services/up');
        serverUp = res.ok;
    } catch (_) {
        // The underscore (_) is a convention for an intentionally unused variable.
        // We don't need the error object here — the fact that the fetch threw
        // at all is enough to know the server is unreachable.
    }

    // Load the passkey feature flag before deciding what UI to show.
    // loadPasskeyConfig() also calls applyPasskeyLoginUI() when done.
    if (serverUp) await loadPasskeyConfig();

    // Load the AI-generation feature flag so the SQL tab's Generate button
    // is already correctly shown/hidden before the user ever opens that tab.
    if (serverUp) await loadFeaturesConfig();

    // Only ever trust a persisted token when "remember login" is *currently*
    // checked. Previously this restored whatever token cookie happened to
    // still be present regardless of the checkbox -- so a token saved during
    // an earlier remembered session (possibly a different user, or from
    // before the user unchecked the setting) would keep silently logging
    // people back in as that stale identity. If we find a leftover token,
    // role, or identity cookie while the preference is off, discard them now
    // instead of ever reading them.
    if (!getRememberLogin()) {
        deleteCookie(COOKIE_TOKEN);
        deleteCookie(COOKIE_ROLE);
        deleteCookie(COOKIE_IDENTITY);
    }

    const savedToken = getCookie(COOKIE_TOKEN);
    const savedTab   = getCookie(COOKIE_ACTIVE_TAB);

    if (serverUp && savedToken) {
        _token = savedToken; // restore directly to avoid re-writing the cookie

        // Validate the saved token is still accepted by the server before
        // hiding the login overlay. apiFetch calls clearToken() + showLogin()
        // and throws on a 401/403, so if the token is expired we fall through
        // to the catch block and the user sees the login prompt.
        try {
            await apiFetch('services/admin/server');
        } catch (_) {
            // Token was rejected — apiFetch already called showLogin().
            openTab('memory');
            return;
        }

        // Restore the role flags from the saved cookie so tab visibility
        // matches what was set when the user originally logged in.
        restoreRole();
        await loadTasksFeatureFlag();
        applyTabVisibility();

        hideLogin();

        // Validate the saved tab name before using it — the cookie value could
        // be stale if a tab was renamed or removed. Fall back to the appropriate
        // default for the user's role.
        const defaultTab = _isServerAdmin ? 'memory' : defaultNonAdminTab();
        const restoredTab = savedTab && tabLoaders[savedTab] ? savedTab : defaultTab;
        openTab(_isServerAdmin ? restoredTab : defaultNonAdminTab());
    } else {
        showLogin();
        openTab('memory');
    }
})();

// Wire the SQL editor highlight layer and the pane resize handle.
initSqlEditor();
initSqlResizeHandle();

// ── WebAuthn / Passkey support ────────────────────────────────────────────────
//
// Two flows are implemented:
//   1. Login  — user clicks "Sign in with Passkey" on the login overlay.
//   2. Register — admin clicks "+ Passkey" in the edit-user sheet while already
//                 signed in (registers a passkey for their own account).
//
// Both flows use the discoverable-credential (resident-key) model so the user
// never needs to type a username — Face ID / Touch ID / Windows Hello identifies
// them automatically.
//
// Base64URL helpers — WebAuthn protocol uses base64url-encoded binary everywhere;
// the browser WebAuthn API uses ArrayBuffer.  These helpers bridge the gap.

function bufferToBase64url(buffer) {
    const bytes = new Uint8Array(buffer);
    let str = '';
    for (const b of bytes) str += String.fromCharCode(b);
    return btoa(str).replace(/\+/g, '-').replace(/\//g, '_').replace(/=/g, '');
}

function base64urlToBuffer(b64url) {
    const b64 = b64url.replace(/-/g, '+').replace(/_/g, '/');
    const bin = atob(b64);
    const buf = new Uint8Array(bin.length);
    for (let i = 0; i < bin.length; i++) buf[i] = bin.charCodeAt(i);
    return buf.buffer;
}

// Recursively walk an object returned by the server and decode any string
// fields whose names are known to carry base64url-encoded binary data into
// ArrayBuffers, as the browser WebAuthn API requires.
function decodeWebAuthnOptions(obj, skipBinary = false) {
    const binaryFields = new Set([
        'challenge', 'id', 'userId',
    ]);
    // rp.id is a plain domain string, not base64url binary — skip binary
    // decoding for the entire rp subtree to avoid passing it through atob().
    const noBinarySubtrees = new Set(['rp']);
    if (Array.isArray(obj)) return obj.map(item => decodeWebAuthnOptions(item, skipBinary));
    if (obj && typeof obj === 'object') {
        const out = {};
        for (const [k, v] of Object.entries(obj)) {
            if (noBinarySubtrees.has(k)) {
                out[k] = decodeWebAuthnOptions(v, true);
            } else if (!skipBinary && binaryFields.has(k) && typeof v === 'string') {
                out[k] = base64urlToBuffer(v);
            } else {
                out[k] = decodeWebAuthnOptions(v, skipBinary);
            }
        }
        return out;
    }
    return obj;
}

// applyPasskeyLoginUI shows the login-screen passkey button/divider only when
// both the server has passkeys enabled AND the browser supports WebAuthn.
// Called after loadPasskeyConfig() resolves.
function applyPasskeyLoginUI() {
    const show = passkeysActive() && !!window.PublicKeyCredential;
    const btn = document.getElementById('passkey-btn');
    const div = document.getElementById('passkey-divider');
    if (btn) {
        btn.style.display = show ? '' : 'none';
        // Re-attach listener idempotently by replacing the element clone trick
        // is unnecessary — just guard with the flag at click time instead.
    }
    if (div) div.style.display = show ? '' : 'none';
    if (show) btn && btn.addEventListener('click', submitPasskeyLogin);
}

// loadPasskeyConfig fetches /services/admin/webauthn/config (no auth needed),
// sets _passkeysEnabled, and then applies the login UI state.
async function loadPasskeyConfig() {
    try {
        const res = await fetch('services/admin/webauthn/config');
        if (res.ok) {
            const cfg = await res.json();
            _passkeysEnabled = !!cfg.passkeys;
        }
    } catch (_) {
        // Server unreachable or old version without this endpoint — leave
        // _passkeysEnabled false so passkey UI stays hidden.
    }
    applyPasskeyLoginUI();
}

// loadFeaturesConfig fetches /services/admin/features (no auth needed), sets
// _aiGenerateEnabled, and shows/hides the SQL tab's Generate button to match.
async function loadFeaturesConfig() {
    try {
        const res = await fetch('services/admin/features');
        if (res.ok) {
            const cfg = await res.json();
            _aiGenerateEnabled = !!(cfg.features && cfg.features.ai);
        }
    } catch (_) {
        // Server unreachable or old version without this endpoint — leave
        // _aiGenerateEnabled false so the Generate button stays hidden.
    }
    applySqlGenerateButtonVisibility();
}

// submitPasskeyLogin drives the discoverable-login ceremony:
//   POST .../login/begin  → get options (challenge set as a cookie server-side)
//   navigator.credentials.get(options)  → browser prompts Face ID / Touch ID
//   POST .../login/finish → verify + receive token
async function submitPasskeyLogin() {
    const errEl = document.getElementById('login-error');
    const btn   = document.getElementById('passkey-btn');

    errEl.textContent = '';
    btn.disabled = true;
    clearToken();

    try {
        // Step 1: get the challenge options from the server.
        const beginRes = await fetch('services/admin/webauthn/login/begin', {
            method:      'POST',
            credentials: 'same-origin',   // needed so the challenge cookie is sent/received
        });

        if (!beginRes.ok) {
            errEl.textContent = 'Passkey login not available on this server.';
            return;
        }

        const rawOptions = await beginRes.json();
        const options    = decodeWebAuthnOptions(rawOptions);

        // Step 2: invoke the platform authenticator (Face ID, Touch ID, etc.).
        const assertion = await navigator.credentials.get({ publicKey: options.publicKey });

        // Step 3: encode the assertion and send it to the server for verification.
        const finishPayload = {
            id:    bufferToBase64url(assertion.rawId),
            rawId: bufferToBase64url(assertion.rawId),
            type:  assertion.type,
            response: {
                authenticatorData: bufferToBase64url(assertion.response.authenticatorData),
                clientDataJSON:    bufferToBase64url(assertion.response.clientDataJSON),
                signature:         bufferToBase64url(assertion.response.signature),
                userHandle:        assertion.response.userHandle
                    ? bufferToBase64url(assertion.response.userHandle)
                    : null,
            },
        };

        const finishRes = await fetch('services/admin/webauthn/login/finish', {
            method:      'POST',
            headers:     { 'Content-Type': 'application/json' },
            credentials: 'same-origin',
            body:        JSON.stringify(finishPayload),
        });

        const data = await finishRes.json();

        if (!finishRes.ok || !data.token) {
            errEl.textContent = data.message || data.msg || 'Passkey verification failed.';
            return;
        }

        // The server returns the account's permission list rather than
        // discrete flags. Any account that can log in at all is allowed to
        // use the baseline DSNs/Tables/Data tabs, so there is no permission
        // check here that could refuse the login outright.
        const roles = rolesFromPermissions(data.permissions);

        // Success — same post-login flow as submitLogin().
        setToken(data.token);
        setRole(roles.admin, roles.serverAdmin, roles.coder, roles.sql, roles.dsnAdmin, data.identity);
        setIdleTimeout(data.inactivityTimeout);
        lastActivity = Date.now();
        hideLogin();
        await loadTasksFeatureFlag();
        applyTabVisibility();
        openTab(_isServerAdmin ? activeTab : defaultNonAdminTab());

    } catch (e) {
        if (e.name === 'NotAllowedError') {
            // User cancelled the authenticator prompt — not an error worth alarming about.
            errEl.textContent = 'Passkey prompt dismissed.';
        } else {
            errEl.textContent = 'Passkey error: ' + e.message;
        }
    } finally {
        btn.disabled = false;
    }
}

// _doPasskeyRegistration is the shared core of the WebAuthn registration
// ceremony.  btn (may be null) is disabled while the request is in flight.
// errEl receives status and error text.  onSuccess is called when the server
// confirms the credential; pass null to get the default green-flash behavior.
async function _doPasskeyRegistration(btn, errEl, onSuccess) {
    const token      = getToken();
    const authHeader = token ? { 'Authorization': 'Bearer ' + token } : {};

    errEl.textContent  = '';
    errEl.style.color  = '';
    if (btn) btn.disabled = true;

    try {
        // Step 1: get creation options from the server (challenge is stored
        // server-side and round-tripped via an HttpOnly cookie).
        const beginRes = await fetch('services/admin/webauthn/register/begin', {
            method:      'POST',
            credentials: 'same-origin',
            headers:     authHeader,
        });

        if (!beginRes.ok) {
            const errBody = await beginRes.text().catch(() => '');
            errEl.textContent = errBody.trim() || 'Passkey registration failed (HTTP ' + beginRes.status + ').';
            return;
        }

        const options = decodeWebAuthnOptions(await beginRes.json());

        // Step 2: invoke the platform authenticator (Face ID / Touch ID).
        const credential = await navigator.credentials.create({ publicKey: options.publicKey });

        // Step 3: encode the attestation and send it to the server.
        const finishPayload = {
            id:    bufferToBase64url(credential.rawId),
            rawId: bufferToBase64url(credential.rawId),
            type:  credential.type,
            response: {
                attestationObject: bufferToBase64url(credential.response.attestationObject),
                clientDataJSON:    bufferToBase64url(credential.response.clientDataJSON),
            },
        };

        const finishRes = await fetch('services/admin/webauthn/register/finish', {
            method:      'POST',
            credentials: 'same-origin',
            headers:     { ...authHeader, 'Content-Type': 'application/json' },
            body:        JSON.stringify(finishPayload),
        });

        if (!finishRes.ok) {
            const d = await finishRes.json().catch(() => ({}));
            errEl.textContent = d.message || d.msg || 'Passkey registration failed.';
            return;
        }

        if (onSuccess) {
            onSuccess();
        } else {
            errEl.style.color = 'green';
            errEl.textContent = 'Passkey registered successfully!';
            setTimeout(() => { errEl.style.color = ''; errEl.textContent = ''; }, 3000);
        }

    } catch (e) {
        if (e.name === 'NotAllowedError') {
            errEl.textContent = 'Passkey prompt dismissed.';
        } else {
            errEl.textContent = 'Passkey error: ' + e.message;
        }
    } finally {
        if (btn) btn.disabled = false;
    }
}

// registerPasskey is called from the "+ Passkey" button in the edit-user sheet.
async function registerPasskey() {
    const btn   = document.getElementById('edit-user-passkey-btn');
    const errEl = document.getElementById('edit-user-error');

    if (!window.PublicKeyCredential) {
        errEl.textContent = 'This browser does not support passkeys.';
        return;
    }

    await _doPasskeyRegistration(btn, errEl, null);
}

// removePasskeys is called from the "- Passkey" button in the edit-user sheet.
// It sends DELETE /services/admin/webauthn/passkeys/{username} to clear all
// stored passkeys for the displayed user.
async function removePasskeys() {
    const btn   = document.getElementById('edit-user-clear-passkey-btn');
    const errEl = document.getElementById('edit-user-error');
    const name  = document.getElementById('edit-user-name').value;

    errEl.textContent = '';
    errEl.style.color = '';
    if (btn) btn.disabled = true;

    try {
        const res = await fetch('services/admin/webauthn/passkeys/' + encodeURIComponent(name), {
            method:      'DELETE',
            credentials: 'same-origin',
            headers:     { 'Authorization': 'Bearer ' + getToken() },
        });

        if (!res.ok) {
            const d = await res.json().catch(() => ({}));
            errEl.textContent = d.message || d.msg || 'Failed to remove passkeys.';
            return;
        }

        errEl.style.color   = 'green';
        errEl.textContent   = 'Passkeys removed.';
        setTimeout(() => { errEl.style.color = ''; errEl.textContent = ''; }, 3000);
    } catch (e) {
        errEl.textContent = 'Network error: ' + e.message;
    } finally {
        if (btn) btn.disabled = false;
    }
}

// ── Passkey prompt (offered after password login) ─────────────────────────────

// maybeOfferPasskeyAfterLogin shows the passkey creation prompt when:
//   • the server has passkeys enabled (_passkeysEnabled), and
//   • the browser supports WebAuthn (window.PublicKeyCredential exists), and
//   • the user has not previously clicked "Don't Ask Again".
// Called only after a successful *password* login, not after passkey login.
function maybeOfferPasskeyAfterLogin() {
    if (!passkeysActive()) return;
    if (!window.PublicKeyCredential) return;
    if (getCookie(COOKIE_PASSKEY_OFFERED)) return;
    const overlay = document.getElementById('passkey-prompt-overlay');
    if (overlay) {
        document.getElementById('passkey-prompt-status').textContent = '';
        overlay.style.display = 'flex';
    }
}

// declinePasskeyPrompt hides the prompt.  When permanent is true it also sets
// the "don't ask again" cookie so the dialog is never shown in this browser.
function declinePasskeyPrompt(permanent) {
    if (permanent) {
        setCookie(COOKIE_PASSKEY_OFFERED, '1', PASSKEY_NO_MAX_AGE);
    }
    const overlay = document.getElementById('passkey-prompt-overlay');
    if (overlay) overlay.style.display = 'none';
}

// createPasskeyFromPrompt runs the registration ceremony from the prompt dialog.
// On success the dialog closes; on failure the error is shown inside the dialog.
async function createPasskeyFromPrompt() {
    const btn   = document.getElementById('passkey-prompt-create-btn');
    const errEl = document.getElementById('passkey-prompt-status');

    await _doPasskeyRegistration(btn, errEl, () => {
        // Success: show a brief confirmation then close the dialog.
        errEl.style.color = 'green';
        errEl.textContent = 'Passkey created!';
        setTimeout(() => declinePasskeyPrompt(false), 1500);
    });
}
