#!/bin/zsh 

# Rudimentary script for generating TLS cert and key files for testing an 
# Ego server. Note that these are not trusted certs.
#
# This tool depends on the "certstrap" tool, found at
#       https://github.com/square/certstrap/releases
#
# We create a self-signed authority (the "common name") and
# the name of a server, which in this case is assumed to be
# the current node on the .local mDNS name space.

COMMONNAME="ForestEdge"
SERVERNAME=$(hostname -s).local
#SERVERNAME="*.local"

FN=$(echo $SERVERNAME | tr "*" "_")

# Clear away anything old 
rm -rf out/
rm -f lib/https-server.*

# Create an initial setup for the common name. This creates 
# the out/ directory if it doesn't already exist. This will
# fail if CN crt or key files already exist in the directory.
certstrap init --common-name $COMMONNAME 

# request CA signing for each of the servers that will be used
# for testing.
certstrap request-cert  --domain "$FN" $(hostname)

# Sign the certificate using our certificate authority.
certstrap sign  $FN --CA $COMMONNAME 

# Copy the newly-made certificate/key info to the parent
# directory for use by the server and clients. If the server
# name is a wildcard, substitute "_" the same was the certstrap
# program does so we find the correct file names to copy.
cp out/$FN.key lib/https-server.key
cp out/$FN.crt lib/https-server.crt

exit
