#!/bin/sh
# launch.sh – build and start the Ego server Docker container.
#
# Usage:
#   ./tools/launch.sh [options]
#
# Options:
#   -build              Rebuild the Docker image before starting the container.
#   -write  <path>      Host path to mount as the container's writable storage
#                       area (/data inside the container).  The SQLite database
#                       and the server log file are stored here.
#   -port   <number>    Externally visible port the server listens on (default: 443).
#   -cert   <file>      TLS certificate file on the host to mount into the container.
#   -key    <file>      TLS private key file on the host to mount into the container.
#   -log    <classes>   Comma-separated log classes forwarded to ego as -l <classes>.
#   -user   <username>  Admin username passed to the container as EGO_USERNAME.
#   -password <pass>    Admin password passed to the container as EGO_PASSWORD.
#   -h | -help          Show this help text and exit.

set -e

IMAGE="${EGO_IMAGE:-ego:latest}"
CONTAINER_NAME="${EGO_CONTAINER_NAME:-ego-server}"

BUILD=0
WRITABLE_PATH="./ego-container"
PORT=""
CERT_FILE=""
KEY_FILE=""
LOG_CLASSES=""
USERNAME=""
PASSWORD=""

# ── Argument parsing ──────────────────────────────────────────────────────────
usage() {
    cat <<EOF
Usage: $(basename "$0") [options]

Options:
  -build             Rebuild the Docker image from the Dockerfile before starting.
  -write    <path>   Mount <path> on the host as /data in the container.
                     The Ego SQLite database and log file are stored there.
                     Defaults to ./ego-container (created if absent).
  -port     <num>    Externally visible port the server listens on (default: 443).
  -cert     <file>   Host path to the TLS certificate file (PEM).
  -key      <file>   Host path to the TLS private key file (PEM).
  -log      <list>   Comma-separated log classes passed to ego as -l <list>.
                     Example: -log auth,cache,server
  -user     <name>   Set the initial admin username (EGO_USERNAME).
  -password <pass>   Set the initial admin password (EGO_PASSWORD).
  -h, -help          Show this help and exit.

Environment variables:
  EGO_IMAGE           Docker image to run (default: ego:latest).
  EGO_CONTAINER_NAME  Container name        (default: ego-server).

Any EGO_SET_* variables present in the current environment are forwarded to
the container so that the entrypoint can pass them as --set config options.
EOF
    exit 0
}

while [ $# -gt 0 ]; do
    case "$1" in
        -build)
            BUILD=1
            ;;
        -write)
            shift
            if [ $# -eq 0 ]; then
                echo "error: -write requires a path argument" >&2
                exit 1
            fi
            WRITABLE_PATH="$1"
            ;;
        -port)
            shift
            if [ $# -eq 0 ]; then
                echo "error: -port requires a port number argument" >&2
                exit 1
            fi
            PORT="$1"
            ;;
        -cert)
            shift
            if [ $# -eq 0 ]; then
                echo "error: -cert requires a file path argument" >&2
                exit 1
            fi
            CERT_FILE="$1"
            ;;
        -key)
            shift
            if [ $# -eq 0 ]; then
                echo "error: -key requires a file path argument" >&2
                exit 1
            fi
            KEY_FILE="$1"
            ;;
        -log)
            shift
            if [ $# -eq 0 ]; then
                echo "error: -log requires a comma-separated class list" >&2
                exit 1
            fi
            LOG_CLASSES="$1"
            ;;
        -user)
            shift
            if [ $# -eq 0 ]; then
                echo "error: -user requires a username argument" >&2
                exit 1
            fi
            USERNAME="$1"
            ;;
        -password)
            shift
            if [ $# -eq 0 ]; then
                echo "error: -password requires a password argument" >&2
                exit 1
            fi
            PASSWORD="$1"
            ;;
        -h|-help|--help)
            usage
            ;;
        *)
            echo "error: unknown option '$1'" >&2
            echo "Run '$(basename "$0") -help' for usage." >&2
            exit 1
            ;;
    esac
    shift
done

# ── Build the docker run command ──────────────────────────────────────────────
DOCKER_ARGS="--name ${CONTAINER_NAME} --rm"

# Writable volume mount
if [ -n "${WRITABLE_PATH}" ]; then
    mkdir -p "${WRITABLE_PATH}"
    WRITABLE_PATH=$(cd "${WRITABLE_PATH}" && pwd)   # resolve to absolute path
    DOCKER_ARGS="${DOCKER_ARGS} -v ${WRITABLE_PATH}:/data"
    DOCKER_ARGS="${DOCKER_ARGS} -e EGO_WRITABLE_PATH=/data"
fi

# TLS certificate and key mounts
# Each file is mounted read-only at a fixed path inside the container and the
# corresponding Ego env var is set so the server finds it automatically.
if [ -n "${CERT_FILE}" ]; then
    CERT_FILE=$(cd "$(dirname "${CERT_FILE}")" && pwd)/$(basename "${CERT_FILE}")
    if [ ! -f "${CERT_FILE}" ]; then
        echo "error: certificate file not found: ${CERT_FILE}" >&2
        exit 1
    fi
    DOCKER_ARGS="${DOCKER_ARGS} -v ${CERT_FILE}:/certs/server.crt:ro"
    DOCKER_ARGS="${DOCKER_ARGS} -e EGO_CERT_FILE=/certs/server.crt"
fi
if [ -n "${KEY_FILE}" ]; then
    KEY_FILE=$(cd "$(dirname "${KEY_FILE}")" && pwd)/$(basename "${KEY_FILE}")
    if [ ! -f "${KEY_FILE}" ]; then
        echo "error: key file not found: ${KEY_FILE}" >&2
        exit 1
    fi
    DOCKER_ARGS="${DOCKER_ARGS} -v ${KEY_FILE}:/certs/server.key:ro"
    DOCKER_ARGS="${DOCKER_ARGS} -e EGO_KEY_FILE=/certs/server.key"
fi

# Credential env vars
if [ -n "${USERNAME}" ]; then
    DOCKER_ARGS="${DOCKER_ARGS} -e EGO_USERNAME=${USERNAME}"
fi
if [ -n "${PASSWORD}" ]; then
    DOCKER_ARGS="${DOCKER_ARGS} -e EGO_PASSWORD=${PASSWORD}"
fi

# Forward any EGO_SET_* variables from the caller's environment
for _decl in $(env | grep '^EGO_SET_' | sed 's/=.*//'); do
    _val=$(eval "printf '%s' \"\$${_decl}\"")
    DOCKER_ARGS="${DOCKER_ARGS} -e ${_decl}=${_val}"
done

# Log classes: -log flag takes priority; fall back to EGO_DEFAULT_LOGGING env var.
# The value is forwarded as EGO_DEFAULT_LOGGING so entrypoint.sh can pass it
# to ego as the -l global option.
_log_classes="${LOG_CLASSES:-${EGO_DEFAULT_LOGGING:-}}"
if [ -n "${_log_classes}" ]; then
    DOCKER_ARGS="${DOCKER_ARGS} -e EGO_DEFAULT_LOGGING=${_log_classes}"
fi

# Forward standard Ego server env vars if set in the calling environment.
# EGO_PORT, EGO_CERT_FILE, EGO_KEY_FILE, and EGO_DEFAULT_LOGGING are handled
# separately above.
for _var in EGO_INSECURE EGO_INSECURE_PORT EGO_REALM EGO_TYPES EGO_LOG_FORMAT; do
    eval "_val=\${${_var}:-}"
    if [ -n "${_val}" ]; then
        DOCKER_ARGS="${DOCKER_ARGS} -e ${_var}=${_val}"
    fi
done

# Port binding: -port flag takes priority, then EGO_PORT env var, then 443.
# EGO_PORT is passed into the container so the server listens on the same port
# that is exposed to the host.
_port="${PORT:-${EGO_PORT:-443}}"
DOCKER_ARGS="${DOCKER_ARGS} -e EGO_PORT=${_port} -p ${_port}:${_port}"

# ── Optional image rebuild ────────────────────────────────────────────────────
if [ "${BUILD}" -eq 1 ]; then
    # Locate the Dockerfile relative to this script (repo root).
    _script_dir=$(cd "$(dirname "$0")" && pwd)
    _dockerfile="${_script_dir}/../Dockerfile"
    if [ ! -f "${_dockerfile}" ]; then
        echo "error: Dockerfile not found at ${_dockerfile}" >&2
        exit 1
    fi
    echo "Building image '${IMAGE}' from ${_dockerfile} ..."
    docker build -t "${IMAGE}" -f "${_dockerfile}" "$(dirname "${_dockerfile}")"
fi

# ── Launch ────────────────────────────────────────────────────────────────────
echo "Starting container '${CONTAINER_NAME}' from image '${IMAGE}' ..."
# shellcheck disable=SC2086  (DOCKER_ARGS is intentionally word-split)
exec docker run ${DOCKER_ARGS} "${IMAGE}"
