#!/bin/bash
#
# Simply re-deploy tool.  Assumes ego is already installed
# and running; you need to stop the server, pull an update,
# build, and restart the server

# Move into the Ego build area
pushd $(ego path)

# Pull any new code
git pull

# Build the current repo
tools/build

# stop the active server
EGO_GRAMMAR=class ego server stop

# refresh the binary
cp ego ~/bin/

# restart the server
EGO_GRAMMAR=class ego server start -k -p 8080
sleep 1s
EGO_GRAMMAR=class ego server status

# back to where we came from
popd

