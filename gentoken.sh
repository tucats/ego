#!/bin/bash
#
# Generate a token and create an HTTL header file with the
# token header info.  Assumes a file called body.json is
# present with the username and password.

TOKEN=$(curl -s -X GET -u admin http://localhost:8080/services/gettoken )

echo Authorization: token $TOKEN

