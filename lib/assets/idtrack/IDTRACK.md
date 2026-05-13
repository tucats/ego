# idtrack — Issue Tracker

A web-based issue tracking application served by the Ego REST server. It uses the same asset-serving and table/DSN infrastructure as the Ego dashboard, but is a completely self-contained application with its own user accounts and database.

---

## Overview

idtrack is a single-page application (SPA) that lets a team report, track, and comment on issues. It lives at `/idtrack` on any running Ego server.

Key characteristics:

- **No framework dependency** — pure HTML/CSS/JavaScript, same approach as the Ego dashboard.
- **Self-initializing** — creates its SQLite database and all three tables automatically on first launch.
- **Own user accounts** — idtrack users are separate from Ego's admin users. Passwords are SHA-256 hashed in the browser before storage.
- **Service account model** — one Ego user account is used for all backend API calls; idtrack users are tracked in the app's own table.

---

## Accessing the App

With an Ego server running:

```
http://<host>:<port>/idtrack
```

The first visit triggers the setup wizard.

---

## First-Run Setup

1. **Setup screen** — enter an Ego admin username and password (the "service account") and a DSN name (default: `idtrack`). These credentials must belong to a user with admin rights so the app can execute DDL via the `@sql` endpoint and create the DSN.
2. The app calls `/services/admin/logon`, stores the bearer token in `localStorage`, then creates the DSN (a SQLite file named `<dsn>.db`) and runs `CREATE TABLE IF NOT EXISTS` for all three tables.
3. Both the service-account credentials and the Ego bearer token are saved in `localStorage` so subsequent page loads reconnect automatically without re-entering the service account password.
4. **Register screen** — because no idtrack users exist yet, the registration form is shown first. Fill in a username, display name, and password to create the first account.
5. Subsequent visits go straight to the login screen (idtrack user credentials only — Ego re-authentication is handled silently in the background).

---

## Authentication Design

### Two-layer model

idtrack uses two independent authentication layers with different lifecycles:

| Layer | What it is | Storage | Lifetime |
|---|---|---|---|
| **Ego service account** | An Ego admin user created by the server administrator | Credentials (`user`, `pass`, `dsn`) and bearer token in `localStorage` | Persists across browser restarts; token is reused until the server rejects it (expired or revoked) |
| **idtrack user** | An app-level identity stored in the `idtrack_users` table | Username + display name in `sessionStorage` | Cleared when the browser tab or window is closed, or when the user clicks Log Out |

The Ego bearer token is used for every backend call (DSN, table, and `@sql` operations). The idtrack user identity is used only for attribution — populating `reporter`, `assignee`, and comment `author` fields.

### Token persistence and log-out behaviour

Because the Ego bearer token lives in `localStorage` (scoped to the page origin by the browser's Same-Origin Policy), it survives browser restarts. On every page load the saved token is tried first; only if the server rejects it (HTTP 401/403) does the app fall back to re-authenticating with the stored service-account credentials.

When the user chooses **Log Out**, only the idtrack app-level session is cleared — the Ego bearer token is deliberately kept so that the next idtrack login does not require a full Ego re-authentication. The sequence is:

```
User clicks Log Out
  └─ _currentUser cleared, sessionStorage(SESSION_KEY) removed
  └─ _token and localStorage(TOKEN_KEY) are PRESERVED
  └─ Login form shown

User re-enters idtrack credentials
  └─ SHA-256 hash compared against stored password_hash
  └─ On success → launchApp() called using existing _token
     (No Ego round-trip needed)
```

If the preserved Ego token has since expired, the first API call returns 401 and the app prompts the setup screen to re-authenticate the service account.

### Storage summary

| Key | Store | Contents | Cleared when |
|---|---|---|---|
| `idtrack_setup` | `localStorage` | `{ user, pass, dsn }` service account config | User resets setup |
| `idtrack_token` | `localStorage` | Ego bearer token | Token rejected by server (401/403) |
| `idtrack_session` | `sessionStorage` | `{ username, display_name }` idtrack user | Tab/window closed, or user logs out |
| `idtrack_prefs` | `localStorage` | `{ darkMode: bool }` user preferences | Never (persists across restarts) |

### Why separate layers?

The Ego server already has its own user system used for administrative and server-management tasks. Mixing issue-tracker users into that system would pollute the admin user list and give idtrack users unintended server access. The service account approach keeps idtrack's concerns isolated: one Ego user (with the minimum privilege needed — admin for `@sql`) acts on behalf of the entire idtrack app.

### Password security note

SHA-256 hashing happens in the browser using the Web Crypto API (`crypto.subtle.digest`). The hash is sent to the server only when creating an account (stored in `idtrack_users.password_hash`). Login verification fetches the stored hash and compares it to the hash of the entered password — neither the plaintext password nor a session credential is ever sent to the server. This is appropriate for an internal tool; a production deployment would add server-side verification.

---

## Data Model

Database: SQLite, file `<dsn>.db` in the Ego server's working directory (default: `idtrack.db`).

### `idtrack_users`

Stores idtrack app accounts.

| Column | Type | Notes |
|---|---|---|
| `username` | TEXT PRIMARY KEY | Unique login name; letters, digits, `_`, `.`, `-` only |
| `display_name` | TEXT NOT NULL | Human-readable name shown in the UI |
| `password_hash` | TEXT NOT NULL | SHA-256 hex digest of the password |
| `created_at` | TEXT NOT NULL | ISO 8601 timestamp |

### `issues`

Core issue records.

| Column | Type | Notes |
|---|---|---|
| `id` | INTEGER PRIMARY KEY AUTOINCREMENT | Unique issue number |
| `title` | TEXT NOT NULL | Brief one-line summary |
| `description` | TEXT | Free-form detail (may be multi-line) |
| `reporter` | TEXT NOT NULL | idtrack username of creator; set at creation, never edited |
| `assignee` | TEXT | idtrack username of assignee; blank if unassigned |
| `priority` | TEXT | `High`, `Medium`, or `Low` |
| `status` | TEXT | `Open` or `Resolved` |
| `created_at` | TEXT NOT NULL | ISO 8601 timestamp, set at creation |
| `updated_at` | TEXT NOT NULL | ISO 8601 timestamp, updated on every save |

### `comments`

User-authored comments attached to an issue.

| Column | Type | Notes |
|---|---|---|
| `id` | INTEGER PRIMARY KEY AUTOINCREMENT | |
| `issue_id` | INTEGER NOT NULL | References `issues.id` |
| `author` | TEXT NOT NULL | idtrack username of the commenter |
| `body` | TEXT NOT NULL | Free-form comment text |
| `created_at` | TEXT NOT NULL | ISO 8601 timestamp |

---

## Backend API Usage

idtrack uses only existing Ego server endpoints — no new API endpoints were added.

| Operation | HTTP method + path | Notes |
|---|---|---|
| Ego login | `POST /services/admin/logon` | Gets bearer token for service account |
| Create DSN | `POST /dsns/` | Creates the `idtrack` SQLite DSN on first run |
| Read DSN | `GET /dsns/<dsn>/` | Checks whether DSN already exists |
| DDL + all queries | `PUT /dsns/<dsn>/tables/@sql` | Body: JSON array of SQL strings; requires admin |
| Serve HTML | `GET /idtrack` | New route added to Ego |
| Serve assets | `GET /assets/idtrack/*` | Handled by the existing assets handler |

All data operations (SELECT, INSERT, UPDATE) go through the `@sql` endpoint with the full SQL statement built in JavaScript. This was chosen over the standard row API (`/rows`) because it supports arbitrary WHERE clauses, ORDER BY, and future JOIN queries without needing to understand the Ego filter-parameter syntax.

### SQL injection mitigation

Because the `@sql` endpoint accepts raw SQL, all user-supplied strings are passed through `sqlStr()` before embedding in a statement:

```javascript
function sqlStr(s) {
    return String(s == null ? '' : s).replace(/'/g, "''");
}
```

This doubles every single quote, which is the standard SQL escape for string literals. All issue fields, comment bodies, and usernames go through this function before being embedded in SQL.

---

## Server-Side Changes

Three small changes were made to the Ego server:

### `defs/rest.go`
Added one constant:
```go
IDTrackPath = "/idtrack"
```

### `server/admin/idtrack.go` (new file)
A handler that loads and serves `idtrack.html` from the asset store, identical in structure to `UIHandler` for the dashboard.

### `commands/routes.go`
One new route registered immediately after the dashboard UI route:
```go
router.New(defs.IDTrackPath, admin.IDTrackHandler, http.MethodGet).
    Class(server.AdminRequestCounter)
```

No authentication is required to load the page itself (the app manages its own auth).

---

## Asset Files

All frontend assets live in `lib/assets/idtrack/`:

| File | Purpose |
|---|---|
| `idtrack.html` | App shell: setup overlay, login/register overlays, new-issue overlay, main two-panel layout |
| `idtrack.css` | Styles: header, filter bar, sortable table, priority/status badges, detail panel, comments |
| `idtrack.js` | All app logic: initialization, Ego login, idtrack auth, issue and comment CRUD, filtering, sorting |
| `IDTRACK.md` | This file |

---

## Application Structure

### Initialization sequence (`init()`)

```
page load
  └─ localStorage has setup (idtrack_setup)?
       No  → show Setup screen
       Yes → try localStorage bearer token (idtrack_token)
              found → use it
              not found → POST /services/admin/logon with saved credentials
                           fail → show Setup screen with error
              └─ CREATE TABLE IF NOT EXISTS (idempotent; also validates token)
                   fail → retry login once, then show Setup screen with error
                   └─ sessionStorage has idtrack user (idtrack_session)?
                        Yes → launchApp()   (full silent resume)
                        No  → countIdtrackUsers()
                               0 users → show Register form
                               N users → show Login form
```

### Main UI

The app uses a two-panel layout:

- **Left panel** — scrollable issues table with a sticky header. Clicking any column header sorts by that column (click again to reverse). A filter bar in the app header lets you narrow by status (Open / Resolved / All), priority (All / High / Medium / Low), and free-text search across title, description, reporter, and assignee.
- **Right panel** — issue detail, shown when a row is selected. All fields except Reporter and Created are editable. A "Save Changes" button appears as soon as any field is modified. Below the fields is a threaded comments section with an "Add Comment" text area (Ctrl+Enter to submit). Clicking anywhere in the left panel outside an issue row closes the detail panel (equivalent to the ← Back button); if there are unsaved changes, the same "Discard?" confirmation is shown. Clicking on an issue row while the detail panel is open navigates to that issue (the `selectIssue` function handles its own dirty-state check).

### State variables

| Variable | Type | Description |
|---|---|---|
| `_token` | string | Ego bearer token; loaded from `localStorage` on startup, kept in memory across idtrack logouts |
| `_setup` | object | `{ user, pass, dsn }` from `localStorage` |
| `_currentUser` | object | `{ username, display_name }` from `sessionStorage`; null after log-out or tab close |
| `_allIssues` | array | All issue rows, refreshed from the server |
| `_currentId` | number | ID of the issue currently shown in the detail panel |
| `_sortCol` | string | Current sort column name |
| `_sortAsc` | boolean | Sort direction |
| `_statusFilter` | string | `'open'`, `'resolved'`, or `'all'` |
| `_priorityFilter` | string | `'High'`, `'Medium'`, `'Low'`, or `'all'` |
| `_detailDirty` | boolean | True when unsaved edits exist in the detail panel |
| `_darkMode` | boolean | True when dark mode is active; mirrors `body.dark` class and `localStorage` |

---

## Hamburger Menu and Settings

### Hamburger menu

The app header right side contains a `☰` hamburger button (HTML entity `&#9776;`) that opens a small dropdown menu. The menu has two items:

- **Settings** — opens the Settings overlay
- **Sign out** — calls `doLogout()` (styled in danger-red)

The dropdown closes automatically when the user clicks anywhere outside it. This is implemented by registering a one-shot `document` click listener (`_closeMenuOnOutside`) via `{ once: true }` when the menu opens. The hamburger button calls `event.stopPropagation()` to prevent its own click from immediately triggering that listener.

CSS: `.menu-wrap` (relative container), `.app-menu` (absolute dropdown, `z-index: 200`), `.menu-item` / `.menu-item-danger`.

### Settings overlay

The Settings overlay (id `settings-overlay`) is a standard `overlay`/`sheet` modal (same pattern as the New Issue overlay). It can be dismissed by clicking the backdrop or the "Done" button.

Currently contains one setting:

| Setting | Control | Key in `idtrack_prefs` |
|---|---|---|
| Dark mode | Toggle switch | `darkMode` |

The toggle switch is a pure-CSS component (`.toggle` label wrapping a hidden checkbox + `.toggle-track` span). JavaScript calls `toggleDarkMode(checked)` on the `onchange` event.

### Dark mode

Dark mode is implemented entirely with CSS custom properties. Applying the class `body.dark` overrides the full set of `:root` variables with dark-palette equivalents:

| Token | Light | Dark |
|---|---|---|
| `--bg` | `#f5f6f8` | `#0f1117` |
| `--surface` | `#ffffff` | `#1a1d27` |
| `--border` | `#e2e5ea` | `#2d3142` |
| `--border-dark` | `#c8cdd5` | `#3d4258` |
| `--text` | `#1a1d23` | `#e2e5ea` |
| `--primary` | `#2563eb` | `#3b82f6` |
| Badge backgrounds | muted pastels | deep-tinted darks |

Two hardcoded hover/selected colors on issue rows are overridden by explicit `body.dark` rules (they cannot use variables because they were written as hex literals).

The preference is saved to `localStorage` under `idtrack_prefs` as `{ darkMode: true/false }` and loaded by `loadPrefs()` at the very start of `init()`, so the correct theme is applied before any UI renders.

---

## Known Limitations and Future Work

- **Client-side password verification** — the password hash is fetched from the server and compared in the browser. For a higher-security deployment, move verification to a server-side Ego service endpoint.
- **No pagination** — all issues are fetched in one query. Add `LIMIT`/`OFFSET` via `@sql` if the issue count grows large.
- **No role system** — all authenticated idtrack users can edit any issue or comment. An `is_admin` column on `idtrack_users` could gate destructive operations.
- **No issue deletion** — intentionally omitted to preserve history. Could be added with an admin-only path.
- **Assignee is free-text backed by a select** — the assignee dropdown is populated from `idtrack_users`, but the underlying column is `TEXT`, so values entered before a user registers are preserved.
- **No email/notification integration** — idtrack has no outbound notification capability.
