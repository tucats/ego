#!/usr/bin/env bash
#
# test_container.sh
#
# Builds a Docker image from the current workspace -- including any
# uncommitted changes -- and runs the full Ego test suite (Go unit tests,
# Ego language tests, and the REST API test suite) inside a throwaway
# container. With --remote, the image is instead built from a fresh clone of
# the upstream repository, which tests what a brand-new install of the
# published source does with no local workspace, configuration, or
# uncommitted change involved. This gives an isolated, CI-like way to validate
# a change before committing it:
#
#   - Nothing is installed or built in your local workspace.
#   - The in-container test server uses a scratch, in-container database and
#     log directory, so it never touches your development database/logs.
#   - No port from the container is published to the host, and the container
#     is removed as soon as the run finishes (see "docker run --rm" below),
#     so it can't collide with a server you already have running locally.
#
# Usage:
#   tools/test_container.sh [options]
#
# Options:
#   --set <key>=<value>   Forwarded to the in-container test server as
#                          "--set <key>=<value>", exactly like the ego CLI's
#                          own global --set flag. May be repeated.
#   --remote               Build from a fresh clone of the upstream GitHub
#                          repository instead of the current workspace.
#   --image <name:tag>    Docker image name/tag to build and run (default:
#                          ego-test:latest, or $EGO_TEST_IMAGE if set).
#   --no-cache             Build the image from scratch, ignoring Docker's
#                          layer cache.
#   -h, --help             Show this help and exit.
#
# Example:
#   tools/test_container.sh --set ego.server.js.minify=true
#   tools/test_container.sh --remote
#
# Note: no "-u" (nounset). macOS ships bash 3.2 (the last GPLv2 release),
# and in bash < 4.4, "${arr[@]}" on an empty array trips "unbound variable"
# under nounset -- and SET_ARGS below is legitimately empty whenever no
# --set option is given.
set -o pipefail

IMAGE="${EGO_TEST_IMAGE:-ego-test:latest}"
NO_CACHE=0
REMOTE=0
SET_ARGS=()

usage() {
    cat <<'EOF'
Usage: tools/test_container.sh [options]

Builds a Docker image from the current workspace (including uncommitted
changes) and runs the Go, Ego, and API test suites inside an isolated,
throwaway container. The run never touches the host's development
workspace, database, log files, or any Ego server already running locally.

Options:
  --set <key>=<value>   Forwarded to the in-container test server as
                         "--set <key>=<value>". May be repeated.
  --remote               Build from a fresh clone of the upstream GitHub
                         repository rather than the current workspace, to
                         test the published source as a clean install would
                         get it. Nothing local is used: not uncommitted
                         changes, not the local configuration profile, and
                         not the workspace TLS certificate (the container
                         generates a throwaway self-signed one). Note that
                         the test scripts themselves also come from the
                         clone, so local edits to them do not apply.
  --image <name:tag>    Docker image name/tag to build and run
                         (default: ego-test:latest).
  --no-cache             Build the image from scratch, ignoring Docker's
                         layer cache.
  -h, --help             Show this help and exit.

Example:
  tools/test_container.sh --set ego.server.js.minify=true
  tools/test_container.sh --remote
EOF
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --set)
            shift
            if [[ $# -eq 0 ]]; then
                echo "error: --set requires a <key>=<value> argument" >&2
                exit 1
            fi
            SET_ARGS+=("$1")
            ;;
        --image)
            shift
            if [[ $# -eq 0 ]]; then
                echo "error: --image requires a name argument" >&2
                exit 1
            fi
            IMAGE="$1"
            ;;
        --remote)
            REMOTE=1
            ;;
        --no-cache)
            NO_CACHE=1
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            echo "error: unknown option '$1'" >&2
            usage >&2
            exit 1
            ;;
    esac
    shift
done

# Resolve the repository root relative to this script's own location, so the
# script works regardless of the caller's current directory.
SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
REPO_ROOT=$(cd "${SCRIPT_DIR}/.." && pwd)

if [[ ! -f "${REPO_ROOT}/Dockerfile" ]]; then
    echo "error: Dockerfile not found at ${REPO_ROOT}; run this from an Ego source tree" >&2
    exit 1
fi

# A --remote build discards the workspace anyway, so its missing cert is not
# worth mentioning: the image build generates a throwaway self-signed one.
if [[ "${REMOTE}" -eq 0 ]] &&
   [[ ! -f "${REPO_ROOT}/lib/https-server.crt" || ! -f "${REPO_ROOT}/lib/https-server.key" ]]; then
    echo "warning: lib/https-server.crt / lib/https-server.key not found." >&2
    echo "         The in-container test server needs these to start over HTTPS," >&2
    echo "         so the image build will generate a throwaway self-signed pair." >&2
    echo "         Run tools/keygen.sh to use a real one instead." >&2
fi

# --target builder stops at the Dockerfile's builder stage rather than the
# minimal runtime stage: the builder stage still has the Go toolchain, zsh,
# and the full source tree (tests/, tools/), all of which tools/test.sh needs.
# --build-arg GENERATE_TEST_CERT=true lets the build supply a self-signed TLS
# cert and key when the source tree has none of its own -- always the case for
# a fresh clone, since those files are generated by tools/keygen.sh and never
# committed.
BUILD_ARGS=(--build-arg GENERATE_TEST_CERT=true --target builder -t "${IMAGE}" -f "${REPO_ROOT}/Dockerfile")

if [[ "${REMOTE}" -eq 1 ]]; then
    # USE_LOCAL is left at its default (false), so the Dockerfile throws away
    # the copied-in workspace and clones the upstream repository instead.
    # CLONE_STAMP changes on every run so Docker cannot serve that clone from
    # a cached layer, which would quietly test an older upstream commit.
    BUILD_ARGS+=(--build-arg "CLONE_STAMP=$(date +%s)")
    echo "Building image '${IMAGE}' from a fresh clone of the upstream repository ..."
else
    # --build-arg USE_LOCAL=true tells the Dockerfile to build from the
    # workspace copied into the build context (including uncommitted changes)
    # instead of cloning the upstream repository.
    BUILD_ARGS+=(--build-arg USE_LOCAL=true)
    echo "Building image '${IMAGE}' from the current workspace (including uncommitted changes) ..."
fi

if [[ "${NO_CACHE}" -eq 1 ]]; then
    BUILD_ARGS=(--no-cache "${BUILD_ARGS[@]}")
fi

# Without "set -e" a failed build would otherwise fall through to the run
# below, which would silently test whatever image was left over from the last
# successful build.
if ! docker build "${BUILD_ARGS[@]}" "${REPO_ROOT}"; then
    echo "error: image build failed; not running the test suite" >&2
    exit 1
fi

echo "Running isolated test suite in container ..."
# No -p/--publish: the server inside the container is only ever reached by
# the apitest run inside that same container, so no host port is needed.
docker run --rm "${IMAGE}" bash tools/test_container_entrypoint.sh "${SET_ARGS[@]}"
STATUS=$?

exit "${STATUS}"
