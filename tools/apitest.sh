#!/bin/zsh 
#
# Runner for the apitests. The source lives in the tools directory, and
# the tests are in a subdirectory below that. This exists so it can run
# from anywhere.

pushd $(ego path)/tools/apitest/
go mod tidy
go run . tests/
popd

