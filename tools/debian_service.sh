#!/usr/bin/env bash
#
# debian_service.sh — define and install a systemd service that runs the
# current Ego installation as a REST/web server on a Debian (or other
# systemd-based) Linux host.
#
# This script does NOT build Ego. It expects an already-built `ego` binary
# (see ./tools/build, or ./tools/build --bin) and simply wraps it in a
# systemd unit, so the service always runs "the current installation" --
# whatever binary --binary resolves to at install time.
#
# Must be run as root (the unit file, environment file, service account,
# and runtime directory all require root to create).
#
#   sudo tools/debian_service.sh [options]
#
# Run with --help for the full option list. Sensible defaults are chosen
# for a common deployment (plain HTTP behind a reverse proxy such as nginx,
# which is why --insecure is the default -- see --secure below to have Ego
# terminate TLS itself instead), but every meaningful knob has a flag so
# this script also works for a from-scratch, Ego-terminated-TLS, no-proxy
# deployment.
#
# What gets created:
#   /etc/systemd/system/<name>.service   the unit (ExecStart, sandboxing, ...)
#   /etc/ego/<name>.env                  EGO_* environment variables the
#                                        unit reads via EnvironmentFile=
#   <runtime-path>                        EGO_PATH: where Ego materializes
#                                        its lib/ tree, log files, and (if
#                                        no --users store is given) its
#                                        default local credentials database
#   a dedicated system user/group        (--user/--group; skipped if you
#                                        point at an existing account)
#
# Re-running this script (e.g. after changing a flag) regenerates both
# files and reloads systemd; it does not touch the service's stored data
# in <runtime-path>.
#
# Use --uninstall to reverse the install: stops and disables the service
# and removes the unit and environment file. The runtime directory and
# service account are left alone (they hold your data) -- remove them
# yourself if you really want a clean slate.

set -euo pipefail

# ── Defaults ─────────────────────────────────────────────────────────────
NAME="ego"
DESCRIPTION="Ego REST/Web Server"
BINARY=""
SERVICE_USER="ego"
SERVICE_GROUP=""
RUNTIME_PATH=""
PORT=""
SECURE=0
USERS_STORE=""
CERT_FILE=""
KEY_FILE=""
REALM=""
PROFILE=""
LOG_FILE=""
EXTRA_ARGS=""
AFTER_UNITS=()
ENABLE=1
START=1
DRY_RUN=0
UNINSTALL=0
UNIT_DIR="/etc/systemd/system"
ENV_DIR="/etc/ego"

# ── Help ─────────────────────────────────────────────────────────────────
usage() {
    cat <<'EOF'
Usage: debian_service.sh [options]
       debian_service.sh --uninstall [--name NAME]

Identity
  --name NAME
        systemd unit name (default: ego) -> <name>.service
  --description TEXT
        Unit Description= (default: "Ego REST/Web Server")
  --binary PATH
        Path to the ego binary (default: resolved via `command -v ego`;
        the script does not build one)

Service account
  --user NAME
        System account to run as (default: ego; created with
        `adduser --system` if it does not exist)
  --group NAME
        Group for --user (default: same as --user)

Runtime data
  --runtime-path PATH
        EGO_PATH: where Ego stores its lib/ tree, logs, and (absent
        --users) its local credentials database (default: /var/lib/<name>)

Network / TLS
  --port N
        Listen port (default: 8080 if --insecure, 443 if --secure)
  --insecure
        Plain HTTP -- the default. Use this when a reverse proxy (nginx,
        etc.) terminates TLS in front of Ego.
  --secure
        Ego terminates TLS itself. Requires --cert-file and --key-file
        (Ego does not generate these -- see docs/SERVER.md).
  --cert-file PATH
        HTTPS certificate file (--secure only)
  --key-file PATH
        HTTPS private key file (--secure only)
  --realm TEXT
        Realm string sent in password challenges

Auth / data
  --users PATH_OR_URL
        --users store: a JSON file path, or a postgres://... /
        sqlite://... URL (default: none -- Ego uses its own default
        local store under --runtime-path)

Misc passthrough
  --profile NAME
        Adds "-p NAME" so the service uses that Ego CLI profile instead
        of the default one
  --log-file PATH
        Adds "--log-file PATH". Default: none -- Ego logs to stdout,
        captured by the journal (`journalctl -u <name>`)
  --extra-args "STRING"
        Appended verbatim to the generated `ego server run` command
        line, for any option this script has no dedicated flag for
  --after UNIT
        Extra systemd After= dependency (repeatable, e.g.
        --after postgresql.service)

Install behavior
  --no-enable
        Don't `systemctl enable` afterward
  --no-start
        Don't `systemctl start` afterward
  --dry-run
        Print what would be written/run; change nothing
  --uninstall
        Stop, disable, and remove the named service (unit + environment
        file only -- runtime data and the service account are left in
        place)
  -h, --help
        Show this help and exit

Examples:
  # This deployment: nginx in front, Ego on loopback-reachable :8080
  sudo tools/debian_service.sh --port 8080

  # Ego terminates TLS itself, no reverse proxy
  sudo tools/debian_service.sh --secure --port 443 \
      --cert-file /etc/letsencrypt/live/example.com/fullchain.pem \
      --key-file  /etc/letsencrypt/live/example.com/privkey.pem

  # Shared Postgres-backed credentials store, custom runtime path
  sudo tools/debian_service.sh --users "postgres://ego:pw@db/ego" \
      --runtime-path /opt/ego

  # Remove it again
  sudo tools/debian_service.sh --uninstall
EOF
}

# ── Argument parsing ─────────────────────────────────────────────────────
while [ $# -gt 0 ]; do
    case "$1" in
        --name)         NAME="$2"; shift 2 ;;
        --description)  DESCRIPTION="$2"; shift 2 ;;
        --binary)       BINARY="$2"; shift 2 ;;
        --user)         SERVICE_USER="$2"; shift 2 ;;
        --group)        SERVICE_GROUP="$2"; shift 2 ;;
        --runtime-path) RUNTIME_PATH="$2"; shift 2 ;;
        --port)         PORT="$2"; shift 2 ;;
        --insecure)     SECURE=0; shift ;;
        --secure)       SECURE=1; shift ;;
        --cert-file)    CERT_FILE="$2"; shift 2 ;;
        --key-file)     KEY_FILE="$2"; shift 2 ;;
        --realm)        REALM="$2"; shift 2 ;;
        --users)        USERS_STORE="$2"; shift 2 ;;
        --profile)      PROFILE="$2"; shift 2 ;;
        --log-file)     LOG_FILE="$2"; shift 2 ;;
        --extra-args)   EXTRA_ARGS="$2"; shift 2 ;;
        --after)        AFTER_UNITS+=("$2"); shift 2 ;;
        --no-enable)    ENABLE=0; shift ;;
        --no-start)     START=0; shift ;;
        --dry-run)      DRY_RUN=1; shift ;;
        --uninstall)    UNINSTALL=1; shift ;;
        -h|--help)      usage; exit 0 ;;
        *)
            echo "debian_service.sh: unknown option: $1" >&2
            echo "Run with --help for usage." >&2
            exit 1
            ;;
    esac
done

SERVICE_GROUP="${SERVICE_GROUP:-$SERVICE_USER}"
UNIT_FILE="${UNIT_DIR}/${NAME}.service"
ENV_FILE="${ENV_DIR}/${NAME}.env"

# ── Preconditions ────────────────────────────────────────────────────────
if [ "$(id -u)" -ne 0 ]; then
    echo "debian_service.sh: must be run as root (try: sudo $0 ...)" >&2
    exit 1
fi

if ! command -v systemctl >/dev/null 2>&1; then
    echo "debian_service.sh: systemctl not found -- this script requires a systemd-based host" >&2
    exit 1
fi

# ── Uninstall path ───────────────────────────────────────────────────────
if [ "$UNINSTALL" -eq 1 ]; then
    echo "Stopping and disabling ${NAME}.service (if present)..."
    systemctl stop "${NAME}.service" 2>/dev/null || true
    systemctl disable "${NAME}.service" 2>/dev/null || true

    if [ -f "$UNIT_FILE" ]; then
        rm -f "$UNIT_FILE"
        echo "Removed ${UNIT_FILE}"
    fi

    if [ -f "$ENV_FILE" ]; then
        rm -f "$ENV_FILE"
        echo "Removed ${ENV_FILE}"
    fi

    systemctl daemon-reload

    echo
    echo "Left in place (remove yourself if you want a clean slate):"
    echo "  - the service account (if one was created for this service)"
    echo "  - any runtime-path data directory"
    exit 0
fi

# ── Resolve the binary ───────────────────────────────────────────────────
if [ -z "$BINARY" ]; then
    if command -v ego >/dev/null 2>&1; then
        BINARY="$(command -v ego)"
    else
        echo "debian_service.sh: no 'ego' binary found on PATH, and --binary was not given." >&2
        echo "Build one first (./tools/build, or ./tools/build --bin) or pass --binary <path>." >&2
        exit 1
    fi
fi

BINARY="$(readlink -f "$BINARY" 2>/dev/null || echo "$BINARY")"

if [ ! -x "$BINARY" ]; then
    echo "debian_service.sh: '$BINARY' does not exist or is not executable." >&2
    exit 1
fi

# ── Resolve network/TLS settings ─────────────────────────────────────────
if [ "$SECURE" -eq 1 ]; then
    PORT="${PORT:-443}"

    if [ -z "$CERT_FILE" ] || [ -z "$KEY_FILE" ]; then
        echo "debian_service.sh: --secure requires both --cert-file and --key-file." >&2
        echo "Ego does not generate these itself -- see docs/SERVER.md. Use --insecure" >&2
        echo "instead if a reverse proxy in front of Ego already terminates TLS." >&2
        exit 1
    fi
else
    PORT="${PORT:-8080}"
fi

RUNTIME_PATH="${RUNTIME_PATH:-/var/lib/${NAME}}"

# ── Service account ──────────────────────────────────────────────────────
# Note: Debian adduser's own bare "--group" flag creates a group with the
# SAME NAME as the new user -- it has no way to name that group something
# else. So when SERVICE_GROUP differs from SERVICE_USER (a custom --group),
# the custom group is created explicitly first, and the user is added to
# it with --ingroup instead of --group.
if [ "$SERVICE_USER" != "root" ]; then
    if ! getent passwd "$SERVICE_USER" >/dev/null 2>&1; then
        if [ "$DRY_RUN" -eq 1 ]; then
            echo "[dry-run] would create system group '${SERVICE_GROUP}' (if needed) and user '${SERVICE_USER}'"
        else
            if ! getent group "$SERVICE_GROUP" >/dev/null 2>&1; then
                echo "Creating system group '${SERVICE_GROUP}'..."
                addgroup --system "$SERVICE_GROUP" >/dev/null
            fi

            echo "Creating system user '${SERVICE_USER}'..."
            adduser --system --ingroup "$SERVICE_GROUP" --no-create-home --disabled-login \
                --home "$RUNTIME_PATH" "$SERVICE_USER" >/dev/null
        fi
    elif ! getent group "$SERVICE_GROUP" >/dev/null 2>&1; then
        echo "debian_service.sh: user '${SERVICE_USER}' exists but group '${SERVICE_GROUP}' does not." >&2
        echo "Pass --group with an existing group for this user, or pick a different --user." >&2
        exit 1
    fi
fi

# ── Runtime directory ─────────────────────────────────────────────────────
if [ "$DRY_RUN" -eq 1 ]; then
    echo "[dry-run] would ensure ${RUNTIME_PATH} exists, owned by ${SERVICE_USER}:${SERVICE_GROUP}, mode 0750"
else
    mkdir -p "$RUNTIME_PATH"
    chown "${SERVICE_USER}:${SERVICE_GROUP}" "$RUNTIME_PATH"
    chmod 0750 "$RUNTIME_PATH"
fi

# ── Build the environment file ───────────────────────────────────────────
# Everything here is a standard Ego environment variable (see
# internal/defs/env.go) that `ego server run` already reads directly --
# no corresponding command-line flags are needed in the unit's ExecStart.
env_content() {
    cat <<EOF
# ${ENV_FILE} -- generated by tools/debian_service.sh. Edit freely; re-run
# 'systemctl daemon-reload && systemctl restart ${NAME}' after changing it.
# Regenerating with debian_service.sh again will overwrite this file.

EGO_PATH=${RUNTIME_PATH}
EGO_PORT=${PORT}
EOF

    if [ "$SECURE" -eq 1 ]; then
        echo "EGO_INSECURE=false"
        echo "EGO_CERT_FILE=${CERT_FILE}"
        echo "EGO_KEY_FILE=${KEY_FILE}"
    else
        echo "EGO_INSECURE=true"
    fi

    if [ -n "$USERS_STORE" ]; then
        echo "EGO_USERS=${USERS_STORE}"
    fi

    # Deliberately last: under `set -e`, a bare "[ cond ] && cmd" as a
    # function's final statement makes the *function's own* return status
    # follow the test, which aborts the whole script the moment REALM is
    # empty (the common case) once env_content is called as a plain
    # statement. if/fi always returns 0 when the condition is false, so
    # this must stay in if/fi form -- don't "simplify" it back to &&.
    if [ -n "$REALM" ]; then
        echo "EGO_REALM=${REALM}"
    fi
}

# ── Build the ExecStart command line ─────────────────────────────────────
# Everything settable via the environment file above is left out here on
# purpose, so the two files never disagree about who owns a given setting.
exec_start() {
    local cmd="$BINARY"

    if [ -n "$PROFILE" ]; then
        cmd="$cmd -p $PROFILE"
    fi

    cmd="$cmd server run"

    if [ -n "$LOG_FILE" ]; then
        cmd="$cmd --log-file $LOG_FILE"
    fi

    if [ -n "$EXTRA_ARGS" ]; then
        cmd="$cmd $EXTRA_ARGS"
    fi

    printf '%s' "$cmd"
}

# ── Build the unit file ───────────────────────────────────────────────────
# KillSignal=SIGINT (not systemd's default SIGTERM) matters: Ego's graceful
# shutdown path -- closing DB connections, deregistering from a cluster,
# draining in-flight requests -- only runs on SIGINT (internal/commands/
# server.go). A bare SIGTERM has no handler installed and would just kill
# the process immediately, skipping all of that.
#
# AmbientCapabilities/CapabilityBoundingSet are only added when binding a
# privileged port (<1024) as a non-root user, since that otherwise fails
# with "permission denied" -- see man 7 capabilities.
unit_content() {
    local after_line="network-online.target"
    local unit
    for unit in "${AFTER_UNITS[@]:-}"; do
        if [ -n "$unit" ]; then
            after_line="${after_line} ${unit}"
        fi
    done

    local needs_cap_bind=0
    if [ "$PORT" -lt 1024 ] && [ "$SERVICE_USER" != "root" ]; then
        needs_cap_bind=1
    fi

    cat <<EOF
# ${UNIT_FILE} -- generated by tools/debian_service.sh. Edit freely;
# 'systemctl daemon-reload' picks up local changes without losing them,
# but re-running debian_service.sh will overwrite this file.

[Unit]
Description=${DESCRIPTION}
After=${after_line}
Wants=network-online.target

[Service]
Type=simple
User=${SERVICE_USER}
Group=${SERVICE_GROUP}
WorkingDirectory=${RUNTIME_PATH}
EnvironmentFile=${ENV_FILE}
ExecStart=$(exec_start)
KillSignal=SIGINT
TimeoutStopSec=30
Restart=on-failure
RestartSec=5
StartLimitIntervalSec=60
StartLimitBurst=5

# Sandboxing -- safe defaults for a network service that only needs to
# read/write its own runtime directory (and, in --secure mode, read a
# certificate/key pair that may live elsewhere, e.g. under
# /etc/letsencrypt -- ProtectSystem=strict blocks writes outside
# ReadWritePaths, not reads, so that's unaffected).
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=${RUNTIME_PATH}
EOF

    if [ "$needs_cap_bind" -eq 1 ]; then
        cat <<'EOF'
AmbientCapabilities=CAP_NET_BIND_SERVICE
CapabilityBoundingSet=CAP_NET_BIND_SERVICE
EOF
    fi

    cat <<EOF

[Install]
WantedBy=multi-user.target
EOF
}

# ── Write, or print, the two files ───────────────────────────────────────
if [ "$DRY_RUN" -eq 1 ]; then
    echo "[dry-run] would write ${ENV_FILE}:"
    echo "----------------------------------------"
    env_content
    echo "----------------------------------------"
    echo
    echo "[dry-run] would write ${UNIT_FILE}:"
    echo "----------------------------------------"
    unit_content
    echo "----------------------------------------"
    echo
    echo "[dry-run] no changes made. Re-run without --dry-run to install."
    exit 0
fi

mkdir -p "$ENV_DIR"
env_content > "$ENV_FILE"
chown "root:${SERVICE_GROUP}" "$ENV_FILE"
chmod 0640 "$ENV_FILE"

mkdir -p "$UNIT_DIR"
unit_content > "$UNIT_FILE"
chmod 0644 "$UNIT_FILE"

echo "Wrote ${ENV_FILE}"
echo "Wrote ${UNIT_FILE}"

systemctl daemon-reload

if [ "$ENABLE" -eq 1 ]; then
    systemctl enable "${NAME}.service"
fi

if [ "$START" -eq 1 ]; then
    systemctl restart "${NAME}.service"
fi

echo
echo "Done."
echo "  Status:  systemctl status ${NAME}"
echo "  Logs:    journalctl -u ${NAME} -f"
echo "  Stop:    systemctl stop ${NAME}"
echo "  Remove:  sudo $0 --uninstall --name ${NAME}"

if [ -z "$USERS_STORE" ]; then
    echo
    echo "No --users store was given, so Ego is using its own default local"
    echo "credentials database under ${RUNTIME_PATH}. If this is a fresh"
    echo "install with no admin account yet, see 'Credentials Management' in"
    echo "docs/SERVER.md for how to bootstrap the first ego.root user."
fi
