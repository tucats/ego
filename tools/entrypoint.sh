#!/bin/sh -v
# Entrypoint

echo "START: configure environment"
export EGO_RUNTIME_PATH=/go/bin/ego
export EGO_RUNTIME_PATH_LIB=/ego/lib/
export EGO_COMPILER_EXTENSIONS=true

echo "START: contents of runtime library"
find /ego/

echo "START: Starting server"
/go/bin/ego --env-config -l server,auth,app,rest server run --users /ego/users.json --superuser "admin"

