#!/usr/bin/env bash
#
# test_container_entrypoint.sh
#
# Runs INSIDE the throwaway Docker image built by tools/test_container.sh.
# Not meant to be invoked directly on a development machine.
#
# Starts a private instance of the Ego server, using a scratch, in-container
# database and log directory that disappears with the container, waits for
# the server to come up, then runs the normal tools/test.sh suite (Go unit
# tests, Ego language tests, and the REST API test suite) against it.
#
# Any arguments given to this script are forwarded to the Ego server as
# "--set <arg>" configuration overrides, e.g. "ego.server.js.minify=true".

# Note: no "-u" (nounset) -- SET_ARGS below is legitimately empty whenever
# tools/test_container.sh was run with no --set options, and "${arr[@]}" on
# an empty array is not safe under nounset on every bash version.
set -o pipefail

# tools/test.sh and its helpers (gotests.sh, apitest.sh) invoke the bare
# "ego" command -- e.g. "$(ego path)/tools/gotests.sh" -- assuming it's on
# PATH, the way it would be for a developer's normal local setup. Put the
# freshly built binary's directory on PATH so that resolves the same way
# inside the container.
export PATH="${PWD}:${PATH}"

# tests/time/parse.ego's "ParseAny flexible format detection" test parses a
# bare zone abbreviation ("... 10:35am EST") with no numeric offset. The
# underlying dateparse library resolves that abbreviation using the
# process's local timezone; on a developer machine already set to US
# Eastern this silently resolves to the correct -0500 offset, but a
# container defaults to UTC, where the same abbreviation resolves to +0000
# and the test's expected value no longer matches. Setting TZ here matches
# the assumption the test already makes on a typical contributor machine.
export TZ=America/New_York

# Point the server at /build/lib (the real source tree copied in by the
# Dockerfile's "COPY . ." step), not the /ego tree the builder stage unpacks
# from the binary's embedded copy of lib/. The embed step
# (internal/cli/app/library.go) deliberately omits https-server.crt/.key from
# that embedded copy so a private key is never baked into the compiled
# binary -- but that also means /ego/lib has no TLS cert/key to serve with.
# /build/lib has them (assuming tools/keygen.sh has been run locally).
EGO_LIB_PATH=/build

# A scratch directory for the SQLite database and server log. Using a fresh
# temp directory (rather than a path under /build) keeps this run's state
# out of the source tree entirely, and it's discarded with the container.
WORK_DIR=$(mktemp -d)

# Some tests resolve behavior via *persisted* settings rather than sensible
# built-in defaults, so they only pass on a machine that already has an
# ~/.ego profile configured from prior local "ego" use. A fresh container
# has no such profile, so bootstrap one here. "ego config set" persists to
# disk (unlike the process-scoped --set flag), matching what an
# already-set-up developer machine has in place:
#   - ego.runtime.path: TestCompiler_ReadDirectory (internal/language/
#     compiler/package_test.go) resolves lib/packages/<name> under this
#     setting rather than the executable's own location.
#   - ego.runtime.exec: tests/exec/exec.ego's exec.Command tests are
#     unconditionally blocked ("no privilege for operation") without it.
./ego config set ego.runtime.path="${PWD}" >/dev/null
./ego config set ego.runtime.exec=true >/dev/null

SET_ARGS=()
for kv in "$@"; do
    SET_ARGS+=(--set "${kv}")
done

echo " "
echo "Starting isolated Ego server (scratch data: ${WORK_DIR}) ..."

./ego --set ego.runtime.path="${EGO_LIB_PATH}" "${SET_ARGS[@]}" \
    server run \
    -u "sqlite://${WORK_DIR}/ego-system.db" \
    --log-file "${WORK_DIR}/ego.log" \
    --insecure-port=0 \
    --default-credential admin:password \
    > "${WORK_DIR}/server-console.log" 2>&1 &
SERVER_PID=$!

# Wait for the server to start accepting connections on the HTTPS port (443).
# The process-alive check lets a fast startup failure (bad --set value,
# missing TLS cert, port already in use) get reported immediately instead of
# only after the full timeout below elapses.
READY=0
for _ in $(seq 1 30); do
    if ! kill -0 "${SERVER_PID}" 2>/dev/null; then
        break
    fi

    if (exec 3<>/dev/tcp/localhost/443) 2>/dev/null; then
        exec 3<&- 3>&-
        READY=1
        break
    fi

    sleep 1
done

if [[ "${READY}" -ne 1 ]]; then
    echo "ERROR: Ego server did not become ready; server output follows:" >&2
    cat "${WORK_DIR}/server-console.log" >&2
    kill "${SERVER_PID}" 2>/dev/null
    exit 1
fi

echo "Server is up. Running test suite ..."
echo " "

# HOST=localhost overrides apitest's default of os.Hostname(), which inside a
# container resolves to the container ID rather than something reachable.
# See the comment in tools/apitest.sh for how APITEST_ARGS is consumed.
APITEST_ARGS="-x HOST=localhost" zsh tools/test.sh
TEST_STATUS=$?

echo " "
echo "Stopping isolated Ego server ..."
kill "${SERVER_PID}" 2>/dev/null
wait "${SERVER_PID}" 2>/dev/null

exit "${TEST_STATUS}"
