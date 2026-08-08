# ─── Stage 1: build ──────────────────────────────────────────────────────────
# golang:bookworm is the official Go image on Debian Bookworm — same base as
# the runtime stage, so both stages share the same (much lower) CVE surface.
# bash is already present in Debian images; only git needs to be added.
FROM golang:bookworm AS builder

# USE_LOCAL=true (set by launch.sh -local and by test_container.sh in its
# default mode) uses the local project tree as the build source instead of a
# fresh clone from GitHub.
ARG USE_LOCAL=false

# CLONE_STAMP busts Docker's layer cache for the clone step below. The cache
# key for a RUN is its expanded command text, so an unchanged Dockerfile would
# happily reuse a clone made days ago -- silently testing stale upstream code.
# tools/test_container.sh --remote passes the current timestamp so the clone is
# really performed every time; ordinary builds leave it at its default.
ARG CLONE_STAMP=0

# GENERATE_TEST_CERT=true (set by test_container.sh) makes the builder stage
# create a throwaway self-signed TLS cert/key when the source tree has none,
# so the in-container test server can start over HTTPS. See the RUN step that
# uses it below for why a fresh clone never has them.
ARG GENERATE_TEST_CERT=false

# zsh is required to run tools/test.sh and the scripts it calls, used by
# tools/test_container.sh to run the test suite inside this builder stage.
#
# nodejs and npm are for tools/dashboard_check.sh, which loads the admin
# dashboard in a headless DOM to confirm it starts up. They are installed only
# in this builder stage; the runtime stage below stays free of them, so the
# shipped image gains no extra packages or CVE surface from a test-only tool.
#
# postgresql is for tests/sql/sql_postgres.ego, which otherwise silently
# self-skips (see that file) when it can't reach a Postgres server. Installing
# the package here creates a single default cluster ("main") at build time;
# tools/test_container_entrypoint.sh starts it and creates the role/database
# that test expects. Also test-only, so likewise kept out of the runtime
# stage below.
RUN apt-get update \
 && apt-get install -y --no-install-recommends git zsh nodejs npm postgresql \
 && rm -rf /var/lib/apt/lists/*

WORKDIR /build

# Always copy the local project into the image first.  When USE_LOCAL=false
# (the default) the RUN step below overwrites this with a clean GitHub clone.
# .dockerignore excludes builds/, ego-container/, .git/, and local binaries.
COPY . .

# Default: discard the local copy and replace it with a fresh clone so the
# image is always built from the canonical upstream source.
RUN if [ "${USE_LOCAL}" != "true" ]; then \
        echo "fresh clone (stamp ${CLONE_STAMP})" && \
        git clone https://github.com/tucats/ego.git /tmp/ego-clone && \
        rm -rf /build && mv /tmp/ego-clone /build; \
    fi

# The TLS cert and key the server serves with are generated locally by
# tools/keygen.sh and deliberately never committed, so a tree that came from
# the clone above has neither. Without them the test server in
# tools/test_container_entrypoint.sh cannot start over HTTPS. Make a
# throwaway self-signed pair for it here.
#
# This runs only for test builds (GENERATE_TEST_CERT=true) and only when the
# tree has no cert of its own, so a real workspace cert copied in by "COPY . ."
# is always preferred and a production image build generates nothing at all.
# The pair also stays in this builder stage: the runtime stage below copies
# lib/ from the binary's embedded archive, which omits cert and key by design.
RUN if [ "${GENERATE_TEST_CERT}" = "true" ] && \
       { [ ! -f lib/https-server.crt ] || [ ! -f lib/https-server.key ]; }; then \
        echo "No TLS certificate in lib/; generating a self-signed test certificate ..." && \
        openssl req -x509 -newkey rsa:2048 -nodes -days 365 \
            -subj "/CN=localhost/O=Ego test container" \
            -addext "subjectAltName=DNS:localhost,DNS:localhost.local,IP:127.0.0.1" \
            -keyout lib/https-server.key \
            -out lib/https-server.crt; \
    fi

RUN go mod download

# tools/apitest is a separate Go module (its own go.mod/go.sum) used by
# tools/apitest.sh to run the REST API test suite. Downloading its
# dependencies here, as part of the image build, means tools/test_container.sh
# can run the full test suite later without needing network access.
RUN cd tools/apitest && go mod download

# tools/dashboard holds the headless-DOM dashboard check and its single
# devDependency, jsdom. Installing it here, as part of the image build, means
# tools/test_container.sh can run the check later without network access --
# the same reason the apitest module's dependencies are fetched above.
# "npm ci" installs exactly what package-lock.json pins, so the container tests
# against the same jsdom version as the host.
RUN cd tools/dashboard && npm ci --no-audit --no-fund

# Build using the project build script so the version and build-time strings
# are injected via linker flags.
RUN bash ./tools/build

# Run ego once to unpack the lib/ content that is embedded in the binary as a
# zip archive (see app-cli/app/library.go and the go:generate directive there).
# Passing --set ego.runtime.path=/ego directs LibraryInit to extract into /ego/lib/
# rather than the default (the directory that contains the binary).
# "echo "" | ego run" is the canonical no-op invocation that fully initializes
# ego without executing any user program.
RUN mkdir -p /ego && \
    echo "" | /build/ego --set ego.runtime.path=/ego run 2>/dev/null || true

# ─── Stage 2: runtime ────────────────────────────────────────────────────────
# Debian Bookworm slim has a much smaller unfixed-CVE surface than Alpine for
# a server workload. apt-get upgrade applies all available security patches at
# image-build time so the layer starts as clean as the upstream repo allows.
FROM debian:bookworm-slim

RUN apt-get update \
 && apt-get upgrade -y \
 && apt-get install -y --no-install-recommends ca-certificates \
 && rm -rf /var/lib/apt/lists/*

# /ego  – Ego runtime tree (binary + lib)
# /data – external writable storage (database, logs); mount a volume here
RUN mkdir -p /ego/lib /data

# The builder stage already selected the right source (clone or local), so
# the runtime stage always copies unconditionally from it.
COPY --from=builder /build/ego               /usr/local/bin/ego
COPY --from=builder /ego/lib/                /ego/lib/
COPY --from=builder /build/tools/entrypoint.sh /entrypoint.sh

RUN chmod +x /entrypoint.sh /usr/local/bin/ego

# Tell Ego where its runtime tree lives.
ENV EGO_PATH=/ego

# Default writable path used by entrypoint.sh for the database and log file.
# Override by passing -e EGO_WRITABLE_PATH=<path> to docker run.
ENV EGO_WRITABLE_PATH=/data

# /data is expected to be a host-mounted (or named) volume so that the
# database and log files persist across container restarts.
VOLUME ["/data"]

# The HTTP-to-HTTPS redirector is disabled (--insecure-port=0 in entrypoint.sh)
# so only the primary server port needs to be exposed.
EXPOSE 443

ENTRYPOINT ["/entrypoint.sh"]
