#!/bin/zsh -v

# Rudimentary script for generating TLS cert and key files for testing an 
# Ego server. Note that these are not trusted certs.
#
# This tool depends on the "certstrap" tool, found at
#       https://github.com/square/certstrap/releases

# Define a pass phrase. This could be randomly generated...
PHRASE=""
COMMONNAME="Forest Edge"
COMMONFILE=Forest_Edge

# Clear away anything old 
rm -rfv out/
rm -fv lib/https-server.*


# Create an initial setup for the common name, Forest Edge
# in this case. This creates the out/ directory if it doesn't
# already exist. This will fail if CN crt or key files already
# exist in the out directory.
certstrap init --common-name $COMMONNAME  $PHRASE

# request CA signing for each of the domains that will be used
# for testing.
certstrap request-cert  $PHRASE --domain  "*.local"

# Add certificate info for localhost, and for test machines
certstrap sign  --CA $COMMONFILE   $PHRASE _.local

# Copy the newly-made certificate/key info to the parent
# directory for use by the server and clients

cp out/_.local.key lib/https-server.key
cp out/_.local.crt lib/https-server.crt

exit
