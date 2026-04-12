#!/bin/sh
# entrypoint.sh – starts the Ego REST server inside the container.
#
# Environment variables recognised by this script
# ─────────────────────────────────────────────────
# EGO_WRITABLE_PATH   Path to the writable storage area (default: /data).
#                     The SQLite database and the log file are placed here.
#
# EGO_USERNAME        If set together with EGO_PASSWORD, the pair is used as
# EGO_PASSWORD        the server's initial admin credential via
#                     --default-credential <user>:<pass>.
#
# EGO_SET_<KEY>       Any variable whose name starts with EGO_SET_ is passed
#                     to the ego global --set option.  The suffix is converted
#                     to a dot-separated config key by lower-casing it and
#                     replacing underscores with dots.
#                     Example:
#                       EGO_SET_EGO_SERVER_INSECURE=true
#                       → --set ego.server.insecure=true
#
# Additional server behaviour (port, TLS mode, realm, …) is controlled by the
# standard Ego environment variables (EGO_PORT, EGO_INSECURE, EGO_REALM, etc.)
# which are read directly by the ego binary.
set -e

WRITABLE_PATH="${EGO_WRITABLE_PATH:-/data}"
mkdir -p "${WRITABLE_PATH}"

# ── Collect --set flags from EGO_SET_* variables ─────────────────────────────
# We write them to a temp file because the pipe in  "env | while read"  runs
# the loop body in a sub-shell, so variable assignments made there would not
# be visible back in this shell.
_SET_TMP=$(mktemp)

env | grep '^EGO_SET_' | while IFS='=' read -r _name _value; do
    # Strip the EGO_SET_ prefix, lower-case, replace _ with .
    _key=$(printf '%s' "${_name#EGO_SET_}" \
            | tr '[:upper:]' '[:lower:]' \
            | tr '_' '.')
    printf ' --set %s=%s' "${_key}" "${_value}" >> "${_SET_TMP}"
done

SET_ARGS=$(cat "${_SET_TMP}")
rm -f "${_SET_TMP}"

# ── Default-credential option ─────────────────────────────────────────────────
CRED_ARGS=""
if [ -n "${EGO_USERNAME}" ] && [ -n "${EGO_PASSWORD}" ]; then
    CRED_ARGS="--default-credential ${EGO_USERNAME}:${EGO_PASSWORD}"
fi

# ── Log classes (-l) ──────────────────────────────────────────────────────────
LOG_ARGS=""
if [ -n "${EGO_DEFAULT_LOGGING}" ]; then
    LOG_ARGS="-l ${EGO_DEFAULT_LOGGING}"
fi

# ── Launch ────────────────────────────────────────────────────────────────────
# shellcheck disable=SC2086  (SET_ARGS, LOG_ARGS, and CRED_ARGS are intentionally
#                              unquoted so each token becomes a distinct argument)
exec /usr/local/bin/ego \
    --set ego.runtime.path="${EGO_PATH:-/ego}" \
    ${SET_ARGS} \
    ${LOG_ARGS} \
    server run \
    -u "sqlite3://${WRITABLE_PATH}/ego-system.db" \
    --log-file "${WRITABLE_PATH}/ego.log" \
    --insecure-port=0 \
    ${CRED_ARGS}
