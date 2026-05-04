#!/bin/zsh 
#
# Runner for the apitests. The source lives in the tools directory, and
# the tests are in a subdirectory below that. This exists so it can run
# from anywhere.
#
# Parameters are passed thru to the underlying apitest code. So for example,
# you can specify a different scheme to use in the urls for the tests with:
#
#    tools/apitest.sh -x SCHEME=http

pushd $(ego path)/tools/apitest/
go mod tidy
go run . tests/ $1 $2 $3 $4 $5 $6
popd

