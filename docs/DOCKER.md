# Running Ego in Docker

This document describes how to build and run the Ego REST server inside a Docker
container using the `tools/launch.sh` helper script.

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Quick Start](#quick-start)
3. [Building the Image](#building-the-image)
4. [Launching the Server](#launching-the-server)
5. [Persistent Storage](#persistent-storage)
6. [TLS Certificates](#tls-certificates)
7. [Authentication](#authentication)
8. [Passing Configuration to the Container](#passing-configuration)
9. [Logging](#logging)
10. [Running in the Background](#running-in-the-background)
11. [Developer Workflow](#developer-workflow)

&nbsp;

## Prerequisites <a name="prerequisites"></a>

- Docker must be installed and the Docker daemon must be running.
- `tools/launch.sh` must be executable (`chmod +x tools/launch.sh`).
- All `launch.sh` commands must be run from the **root of the ego repository**
  (the directory that contains `Dockerfile`, `lib/`, and `tools/`).

&nbsp;

## Quick Start <a name="quick-start"></a>

Build the image and start the server in one step:

```sh
./tools/launch.sh -build -user admin -password secret
```

This will:

1. Clone the latest Ego source from GitHub and compile it inside Docker.
2. Tag the resulting image as `ego:latest`.
3. Start the server on port 443 (HTTPS), storing its database and log in
   `./ego-container/` in the current directory.

&nbsp;

## Building the Image <a name="building-the-image"></a>

Use the `-build` flag to (re)build the Docker image before starting the container.
The build is always a clean build (`--no-cache`).

```sh
./tools/launch.sh -build
```

### Default build — from GitHub

By default the builder stage clones `https://github.com/tucats/ego.git` so the
image always reflects the canonical upstream source, regardless of any local
uncommitted changes.

### Local build — from the working tree

Use `-local` to build the image from the local project files instead of a fresh
clone. This is intended for development and testing of changes that have not yet
been pushed to GitHub. `-local` implies `-build`.

```sh
./tools/launch.sh -local
```

`-local` requires the script to be invoked from the root of an Ego development
directory (verified by checking that `./tools/launch.sh` and `./tools/ego-update`
exist). If either file is absent the script exits with an error.

### Image name and container name

The image name and container name default to `ego:latest` and `ego-server`
respectively. Override them with environment variables before calling the script:

```sh
EGO_IMAGE=ego:dev EGO_CONTAINER_NAME=ego-dev ./tools/launch.sh -build
```

&nbsp;

## Launching the Server <a name="launching-the-server"></a>

Once the image exists, start the container without rebuilding:

```sh
./tools/launch.sh -user admin -password secret
```

### Port

The server listens on port 443 by default. Use `-port` to choose a different port.
The same port number is used for both the host binding and the in-container listener
(`EGO_PORT`), so the externally visible port is always the one you specify.

```sh
./tools/launch.sh -port 8443 -user admin -password secret
```

&nbsp;

## Persistent Storage <a name="persistent-storage"></a>

The container expects a writable directory to be mounted at `/data`. This is where
the server stores:

| File | Purpose |
| ---- | ------- |
| `ego-system.db` | SQLite database for user credentials and server state |
| `ego.log` | Server log file |

The `-write` flag specifies the host directory to mount. It defaults to
`./ego-container` (created automatically if it does not exist).

```sh
# Use the default location
./tools/launch.sh -user admin -password secret

# Use a custom location
./tools/launch.sh -write /var/ego/data -user admin -password secret
```

The host directory is resolved to an absolute path before being passed to Docker,
so relative paths are safe to use.

> **Note:** Because the container is started with `--rm`, it is removed when it
> stops. The data directory on the host is unaffected and will be re-used on the
> next launch.

&nbsp;

## TLS Certificates <a name="tls-certificates"></a>

The Ego server defaults to HTTPS. The image ships with self-signed development
certificates in `/ego/lib/`. For production use, supply your own certificate and
private key with `-cert` and `-key`:

```sh
./tools/launch.sh \
    -cert /etc/ssl/certs/ego.crt \
    -key  /etc/ssl/private/ego.key \
    -user admin -password secret
```

Each file is mounted **read-only** into the container at `/certs/server.crt` and
`/certs/server.key`, and the corresponding environment variables `EGO_CERT_FILE`
and `EGO_KEY_FILE` are set so the server finds them automatically.

To run in plain HTTP mode (no TLS), set `EGO_INSECURE=true` in the calling
environment before running `launch.sh`:

```sh
EGO_INSECURE=true ./tools/launch.sh -port 80 -user admin -password secret
```

&nbsp;

## Authentication <a name="authentication"></a>

Use `-user` and `-password` to seed the server's initial admin credential. This
is passed to the server as `--default-credential <user>:<password>` and is used
only when no existing user database is found.

```sh
./tools/launch.sh -user admin -password s3cr3t
```

If the writable data directory already contains `ego-system.db` from a previous
run, the stored credentials are used and `-user`/`-password` have no effect on
existing accounts.

&nbsp;

## Passing Configuration to the Container <a name="passing-configuration"></a>

There are two ways to pass Ego configuration settings into the container beyond
the dedicated flags described above.

### Standard Ego environment variables

The following environment variables are forwarded automatically from the calling
shell into the container if they are set:

| Variable | Effect |
| -------- | ------ |
| `EGO_PORT` | Server listen port (overridden by `-port`) |
| `EGO_INSECURE` | Set to `true` to run HTTP instead of HTTPS |
| `EGO_INSECURE_PORT` | Port for the HTTP redirector (disabled by default) |
| `EGO_REALM` | Authentication realm string presented to clients |
| `EGO_TYPES` | Type-checking mode (`dynamic`, `relaxed`, or `strict`) |
| `EGO_LOG_FORMAT` | Log line format (`text`, `json`, or `indented`) |

### Arbitrary config keys — `EGO_SET_*`

Any environment variable whose name begins with `EGO_SET_` is translated into an
Ego `--set key=value` option. The suffix of the variable name is lower-cased and
underscores are replaced with dots to form the config key.

**Pattern:** `EGO_SET_<KEY>=<value>` → `ego --set <key>=<value>`

**Conversion rule:** strip `EGO_SET_`, lower-case, replace `_` with `.`

**Example:**

```sh
# Sets ego.server.insecure=true inside the container
export EGO_SET_EGO_SERVER_INSECURE=true
./tools/launch.sh -user admin -password secret
```

```sh
# Sets ego.compiler.extensions=true inside the container
export EGO_SET_EGO_COMPILER_EXTENSIONS=true
./tools/launch.sh -user admin -password secret
```

Multiple `EGO_SET_*` variables can be exported at the same time; all are forwarded.

&nbsp;

## Logging <a name="logging"></a>

Use `-log` to enable one or more diagnostic log classes. The value is a
comma-separated list of class names (no spaces) and is passed to the server as the
global `-l` option.

```sh
./tools/launch.sh -log server,auth -user admin -password secret
```

Available log classes include: `AUTH`, `BYTECODE`, `CLI`, `COMPILER`, `DB`,
`REST`, `SERVER`, `SYMBOLS`, `TABLES`, `TRACE`, `USER`. See the main server
documentation for a full description of each class.

The server log is also written to `ego.log` in the writable data directory (see
[Persistent Storage](#persistent-storage)).

Alternatively, set `EGO_DEFAULT_LOGGING` in the calling environment before
running `launch.sh` — the value is forwarded to the container and has the same
effect as `-log`.

&nbsp;

## Running in the Background <a name="running-in-the-background"></a>

By default `launch.sh` runs the container in the foreground and the shell blocks
until the server exits. Use `-detach` to start the container in the background and
return to the prompt immediately:

```sh
./tools/launch.sh -detach -user admin -password secret
```

Docker prints the container ID on startup. To stop a detached container:

```sh
docker stop ego-server
```

(Replace `ego-server` with the value of `EGO_CONTAINER_NAME` if you overrode it.)

&nbsp;

## Developer Workflow <a name="developer-workflow"></a>

A typical session when iterating on local changes:

```sh
# First time — build from the local tree and start in the foreground
./tools/launch.sh -local -user admin -password secret -log server,rest

# After making further changes — rebuild and restart
./tools/launch.sh -local -detach -user admin -password secret

# Stop the running container
docker stop ego-server

# Rebuild from the canonical GitHub source for a release image
./tools/launch.sh -build
```

### How the image is built

The multi-stage `Dockerfile` works as follows:

1. **Builder stage** (`golang:bookworm`) — compiles `ego` with version and
   build-time strings injected via linker flags, then runs the binary once to
   unpack the embedded `lib/` archive.
2. **Runtime stage** (`debian:bookworm-slim`) — copies only the compiled binary,
   the extracted `lib/` tree, and `entrypoint.sh` from the builder. All build
   tooling is discarded.

The runtime image's `/data` directory is declared as a Docker volume and is
expected to be provided by the host at run time via the `-write` flag (or its
default `./ego-container`).
