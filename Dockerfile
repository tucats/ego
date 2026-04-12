# ─── Stage 1: build ──────────────────────────────────────────────────────────
# golang:bookworm is the official Go image on Debian Bookworm — same base as
# the runtime stage, so both stages share the same (much lower) CVE surface.
# bash is already present in Debian images; only git needs to be added.
FROM golang:bookworm AS builder

RUN apt-get update \
 && apt-get install -y --no-install-recommends git \
 && rm -rf /var/lib/apt/lists/*

WORKDIR /build

# Clone a fresh copy of the source so the image is self-contained and does
# not require any local files from the host.
RUN git clone https://github.com/tucats/ego.git .

RUN go mod download

# Build using the project build script so the version and build-time strings
# are injected via linker flags.
RUN bash ./tools/build

# Run ego once to unpack the lib/ content that is embedded in the binary as a
# zip archive (see app-cli/app/library.go and the go:generate directive there).
# Passing --set ego.runtime.path=/ego directs LibraryInit to extract into /ego/lib/
# rather than the default (the directory that contains the binary).
# "echo "" | ego run" is the canonical no-op invocation that fully initialises
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

COPY --from=builder /build/ego /usr/local/bin/ego
# Copy the lib that ego itself extracted from its embedded zip, not the raw
# source tree, so the contents always match what the binary expects.
COPY --from=builder /ego/lib/  /ego/lib/
# entrypoint.sh is taken from the local build context so that local edits
# take effect immediately without requiring a push to GitHub first.
# Once tools/entrypoint.sh is committed and pushed, this line can be changed
# back to: COPY --from=builder /build/tools/entrypoint.sh /entrypoint.sh
COPY tools/entrypoint.sh /entrypoint.sh

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
