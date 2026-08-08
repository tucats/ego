#!/usr/bin/env bash
#
# test_container_entrypoint.sh
#
# Runs INSIDE the throwaway Docker image built by tools/test_container.sh.
# Not meant to be invoked directly on a development machine.
#
# Starts two private Ego server instances, using scratch, in-container
# database and log directories that disappear with the container:
#
#   - The primary instance (HTTPS, port 443) is a normal Ego server, now also
#     configured as an OAuth2 Resource Server in hybrid mode, pointed at the
#     second instance below. Hybrid mode is additive -- existing native-token
#     behavior is unchanged -- so this instance is what tools/test.sh's
#     existing REST API suite runs against, exactly as before.
#   - The second instance (HTTP, port 4040) is an Ego server acting purely as
#     an OAuth2 Authorization Server (ego.server.oauth.as.enabled=true), for
#     tools/apitest/oauth_tests/ to exercise. It has to be up and serving its
#     discovery document and JWKS *before* the primary instance starts, since
#     the primary instance's RS role fetches both at its own startup and
#     treats a failure to do so as fatal.
#
# Runs tools/test.sh (Go unit tests, Ego language tests, and the REST API
# test suite) against the primary instance, then tools/apitest/oauth_tests/
# against both instances together.
#
# Any arguments given to this script are forwarded to the Ego server as
# "--set <arg>" configuration overrides, e.g. "ego.server.js.minify=true".
# They apply only to the primary instance -- the AS instance's settings are
# fixed by this script, since its only job is to be a stable OAuth2 partner
# for the RS-side tests.

# Note: no "-u" (nounset) -- SET_ARGS below is legitimately empty whenever
# tools/test_container.sh was run with no --set options, and "${arr[@]}" on
# an empty array is not safe under nounset on every bash version.
set -o pipefail

# run_captured: see tools/run_captured.sh. Used below for the OAuth2 suite so
# a clean run reports just a summary line, matching how tools/test.sh's own
# Ego-test and apitest calls behave.
source tools/run_captured.sh

# tools/test.sh and its helpers (gotests.sh, apitest.sh) invoke the bare
# "ego" command -- e.g. "$(ego path)/tools/gotests.sh" -- assuming it's on
# PATH, the way it would be for a developer's normal local setup. Put the
# freshly built binary's directory on PATH so that resolves the same way
# inside the container.
export PATH="${PWD}:${PATH}"

# Point the server at /build/lib (the real source tree copied in by the
# Dockerfile's "COPY . ." step), not the /ego tree the builder stage unpacks
# from the binary's embedded copy of lib/. The embed step
# (internal/cli/app/library.go) deliberately omits https-server.crt/.key from
# that embedded copy so a private key is never baked into the compiled
# binary -- but that also means /ego/lib has no TLS cert/key to serve with.
# /build/lib has them: either the workspace's own pair (from tools/keygen.sh)
# or, when the tree came from a clone and so had none, the throwaway
# self-signed pair the Dockerfile's builder stage generates for test builds.
EGO_LIB_PATH=/build

# waitForPort polls "localhost:<port>" until it accepts a TCP connection or
# the given process dies, whichever happens first. Shared by both server
# startups below so a bad --set value, missing TLS cert, or port conflict is
# reported immediately instead of only after the full timeout elapses.
waitForPort() {
    local port="$1" pid="$2"

    for _ in $(seq 1 30); do
        if ! kill -0 "${pid}" 2>/dev/null; then
            return 1
        fi

        if (exec 3<>/dev/tcp/localhost/"${port}") 2>/dev/null; then
            exec 3<&- 3>&-
            return 0
        fi

        sleep 1
    done

    return 1
}

# ─── PostgreSQL (localhost:5432) ─────────────────────────────────────────────
#
# tests/sql/sql_postgres.ego (run as part of tools/test.sh's Ego test pass)
# self-skips with a warning if it can't reach a Postgres server, so without
# this the Postgres-backed test path never actually ran in the container. It
# expects a role "ego_test" (password "secret") with a same-named database it
# owns, reachable at localhost:5432 -- see that file for the exact values.
#
# The Dockerfile's builder stage installs the "postgresql" package, which
# creates exactly one cluster ("main") at build time. Its version directory
# under /etc/postgresql is looked up here, rather than hardcoded, so this
# keeps working across a Debian base image bump. Debian's default
# postgresql.conf already listens on localhost, so nothing else needs
# configuring beyond starting the cluster and creating the role/database.
PG_VERSION=$(ls /etc/postgresql)

echo " "
echo "Starting PostgreSQL ${PG_VERSION} (cluster: main) ..."
pg_ctlcluster "${PG_VERSION}" main start

PG_READY=0
for _ in $(seq 1 30); do
    if pg_isready -q -h localhost -p 5432; then
        PG_READY=1
        break
    fi

    sleep 1
done

if [[ "${PG_READY}" -ne 1 ]]; then
    echo "ERROR: PostgreSQL did not become ready; cluster log follows:" >&2
    cat "/var/log/postgresql/postgresql-${PG_VERSION}-main.log" >&2
    exit 1
fi

# Local Unix-socket connections authenticate via "peer" auth by default, so
# running these as the "postgres" OS user (created by the package install)
# reaches the Postgres superuser role of the same name without a password.
if ! su postgres -c "psql -v ON_ERROR_STOP=1 -c \"CREATE ROLE ego_test LOGIN PASSWORD 'secret';\""; then
    echo "ERROR: failed to create Postgres role 'ego_test'" >&2
    exit 1
fi

if ! su postgres -c "createdb --owner=ego_test ego_test"; then
    echo "ERROR: failed to create Postgres database 'ego_test'" >&2
    exit 1
fi

# ─── OAuth2 Authorization Server instance (port 4040) ───────────────────────
#
# A scratch directory for this instance's SQLite database, log, signing key,
# and client registry. Using a fresh temp directory (rather than a path under
# /build) keeps this run's state out of the source tree entirely, and it's
# discarded with the container.
AS_WORK_DIR=$(mktemp -d)

# The registered OAuth2 clients tools/apitest/oauth_tests/ authenticates as.
# Lives outside tools/apitest/oauth_tests/ itself so apitest's recursive
# directory scan (which treats every .json file as a test unless it is named
# "dictionary.json") never tries to run it as one.
cp tools/apitest/oauth_tests_fixtures/oauth-clients.json "${AS_WORK_DIR}/oauth-clients.json"

echo " "
echo "Starting isolated OAuth2 Authorization Server (scratch data: ${AS_WORK_DIR}) ..."

./ego --set ego.server.oauth.as.enabled=true \
    --set ego.server.oauth.as.issuer=http://localhost:4040 \
    --set ego.server.oauth.as.clients="${AS_WORK_DIR}/oauth-clients.json" \
    --set ego.server.oauth.as.key.file="${AS_WORK_DIR}/oauth-signing.pem" \
    server run -k --port 4040 \
    -u "sqlite://${AS_WORK_DIR}/ego-system.db" \
    --log-file "${AS_WORK_DIR}/ego.log" \
    --default-credential admin:password \
    > "${AS_WORK_DIR}/server-console.log" 2>&1 &
AS_SERVER_PID=$!

if ! waitForPort 4040 "${AS_SERVER_PID}"; then
    echo "ERROR: OAuth2 Authorization Server did not become ready; server output follows:" >&2
    cat "${AS_WORK_DIR}/server-console.log" >&2
    kill "${AS_SERVER_PID}" 2>/dev/null
    exit 1
fi

# ─── Primary instance (port 443, HTTPS, RS in hybrid mode) ──────────────────
#
# A scratch directory for the SQLite database and server log. Using a fresh
# temp directory (rather than a path under /build) keeps this run's state
# out of the source tree entirely, and it's discarded with the container.
WORK_DIR=$(mktemp -d)

SET_ARGS=()
for kv in "$@"; do
    SET_ARGS+=(--set "${kv}")
done

echo " "
echo "Starting isolated Ego server (scratch data: ${WORK_DIR}) ..."

./ego --set ego.runtime.path="${EGO_LIB_PATH}" \
    --set ego.server.oauth.provider=http://localhost:4040 \
    --set ego.server.oauth.mode=hybrid \
    "${SET_ARGS[@]}" \
    server run \
    -u "sqlite://${WORK_DIR}/ego-system.db" \
    --log-file "${WORK_DIR}/ego.log" \
    --insecure-port=0 \
    --default-credential admin:password \
    > "${WORK_DIR}/server-console.log" 2>&1 &
SERVER_PID=$!

if ! waitForPort 443 "${SERVER_PID}"; then
    echo "ERROR: Ego server did not become ready; server output follows:" >&2
    cat "${WORK_DIR}/server-console.log" >&2
    kill "${SERVER_PID}" 2>/dev/null
    kill "${AS_SERVER_PID}" 2>/dev/null
    exit 1
fi

echo "Servers are up. Running test suite ..."
echo " "

# HOST=localhost overrides apitest's default of os.Hostname(), which inside a
# container resolves to the container ID rather than something reachable.
# See the comment in tools/apitest.sh for how APITEST_ARGS is consumed.
APITEST_ARGS="-x HOST=localhost" zsh tools/test.sh
TEST_STATUS=$?

echo " "
echo "Running OAuth2 API test suite ..."
echo " "

# oauth_tests/ hits both instances: leading-slash endpoints ("/dsns/...")
# resolve against the primary instance via the same SCHEME/HOST/PORT
# dictionary defaults as every other suite under tools/apitest/tests/; AS
# endpoints are addressed by the suite's own dictionary.json (AS_URL,
# defaulting to http://localhost:4040) since they need a different scheme
# and port than the primary instance.
run_captured tools/apitest.sh -x HOST=localhost oauth_tests/
OAUTH_STATUS=$?

if [[ "${TEST_STATUS}" -eq 0 ]]; then
    TEST_STATUS="${OAUTH_STATUS}"
fi

echo " "
echo "Stopping isolated Ego servers ..."
kill "${SERVER_PID}" "${AS_SERVER_PID}" 2>/dev/null
wait "${SERVER_PID}" "${AS_SERVER_PID}" 2>/dev/null

echo "Stopping PostgreSQL ..."
pg_ctlcluster "${PG_VERSION}" main stop

exit "${TEST_STATUS}"
