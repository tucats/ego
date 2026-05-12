#!/bin/zsh 
#
# Runner for the apitests. The source lives in the tools directory, and
# the tests are in a subdirectory below that. This exists so it can run
# from anywhere.
#
# Parameters are passed thru to the underlying apitest code. So for example,
# you can specify a different scheme to use in the urls for the tests with:
#
#    tools/apitest.sh -x SCHEME=http tests/
#
# Note that if you do not supply any arguements, the default is to run all the
# tests found in the tests/ directory. If you specify any arguments, you MUST
# also include the test path(s) to run.
#
# Most tests always require running the 1-login suite to get the server info
# and bearer token required for most additional testing. For example,
#
#     tools/apitest.sh -d tests/dictionary.json tests/1-login tests/4-dsns
#
# This command reads the dictionary (needed for nearly all tests) and then
# runs the login and dsns tests. Note that the list of all possible tests is
# compiled by apitest first, and run alphabetically. That's why the login tests
# all start with directory 1-login/.
#


TESTS="" 

if [[ $# -eq 0 ]]; then
  TESTS=tests/
fi

pushd $(ego path)/tools/apitest/
go mod tidy
go run . $TESTS "$@"
STATUS=$?
popd

exit $STATUS