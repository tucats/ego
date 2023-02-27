#!/bin/sh -v
# Entrypoint

echo "START: configure environment"
export EGO_RUNTIME_PATH=/go/bin/ego
export EGO_RUNTIME_PATH_LIB=/ego/lib/
export EGO_COMPILER_EXTENSIONS=true

echo "START: contents of runtime library"
find /ego/

echo "START: configure authentication"
PASS=password
if [ "$EGO_DEFAULT_PASSWORD" .ne. "" ]; then
  PASS="$EGO_DEFAULT_PASSWORD"
fi 

AUTH_PHRASE="--default-credential admin:$PASS --users memory"
if [ "$EGO_USERS" .ne. "" ]; then
  AUTH_PHRASE="--users $EGO_USERS "
fi 


echo "START: Starting server"
/go/bin/ego --env-config -l server,auth,app,rest server run $AUTH_PHRASE

