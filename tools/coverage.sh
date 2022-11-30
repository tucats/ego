#!/bin/zsh
#
# Use the coverage tool to test the coverage and display it
# as an HTML file. This assumes you have installed the 'cover'
# tool in your dev environment, using 
#
#   go get golang.org/x/tools/cmd/cover
#

FILE=/tmp/ego.coverage_data 

go test -coverprofile $FILE ./...
go tool cover -html=$FILE

#
# Be a good citizen and clean up the coverage data file
#

rm $FILE
