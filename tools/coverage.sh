#!/bin/zsh
#
# Use the coverage tool to test the coverage and display it
# as an HTML file. Make sure the 'cover' tool is installed:

go get golang.org/x/tools/cmd/cover

# Figure out if we are testing everything (the default) or
# just one specific path set
TESTPATH=$1

if [[ "$1" == "" ]]; then
   TESTPATH="./..."
fi 
echo "TEST PATH IS $TESTPATH"

# Transient overage data is stored in /tmp directory
FILE=/tmp/ego.coverage_data 

# Run the requested tests, storing the profile data in the
# temp location. Then launch the coverage reporting tool
# using the local web browser.
go test -coverprofile $FILE $TESTPATH
go tool cover -html=$FILE

# Be a good citizen and clean up the coverage data file
rm $FILE
go mod tidy

