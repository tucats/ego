#!/bin/zsh
#
# Run the tests:
#
# 1. Go unit tests
# 2. Ego tests with strict type checks
# 3. Ego tests with relaxed type checks
# 4. Ego tests with no type checking

# "echo" prints a line of text to the terminal.
# "echo ' '" prints a blank line, used here as visual spacing between sections.

echo " "
echo "Running native Go unit tests"

# "go test ./..." runs every Go unit test in this repository.
# The "..." is a Go wildcard that means "this directory and all subdirectories".
go test ./...

# "$?" is a special shell variable that holds the exit code of the last command.
# By convention, exit code 0 means success; anything else means failure.
# "!= 0" checks for any failure exit code.
if [ $? != 0 ]; then
   echo "Go tests failed"
   # "exit 1" stops the script immediately and reports failure to whoever ran it.
   exit 1
fi

echo " "
echo "Running Ego test stream with strict type checking"

# Run the Ego test suite with strict typing. In strict mode, all variables must
# be explicitly declared with a specific type, and type mismatches are errors.
./ego test --typing=strict
if [ $? != 0 ]; then
   echo "Ego test failure with strict typing"
   exit 1
fi


echo " "
echo "Running Ego test stream with relaxed type checking"

# Run the Ego test suite with relaxed typing. In relaxed mode, some implicit
# type conversions are allowed, making the language behave more like a scripting
# language while still enforcing basic type rules.
./ego test --typing=relaxed
if [ $? != 0 ]; then
   echo "Ego test failure with relaxed typing"
   exit 1
fi


echo " "
echo "Running Ego test stream with dynamic type checking"

# Run the Ego test suite with dynamic (no) type checking. In dynamic mode,
# variables can hold any type and all type coercions are automatic, similar
# to how JavaScript or Python work.
./ego test --typing=dynamic
if [ $? != 0 ]; then
   echo "Ego test failure with dynamic typing"
   exit 1
fi

# Use the 'apitest' tool to run the REST API test suite
# stored in tools/apitests
#
# The `apitest` tool is found at https://github.com/tucats/apitest
# If it isn't installed in the $PATH directory then this test step
# will be skipped.
echo " "

# "which apitest" searches the directories listed in $PATH for a program named
# "apitest" and prints its full file path (e.g. /usr/local/bin/apitest).
# If the program is not found, "which" returns an empty string.
# We store the result in APITEST so we can check for it below.
APITEST=$(which apitest)

# AVAIL is used to build the final summary message. It starts empty so the
# message reads "All tests completed". If the API tests are skipped it is set
# to " available" so the message reads "All available tests completed".
AVAIL=""

# "[ -f $APITEST ]" is a file-existence test: it returns true only when
# $APITEST is a non-empty string AND points to an existing regular file.
# This guards against running apitest when it was not found by "which".
if [ -f $APITEST ]; then
   echo "Running API tests for REST server"
   # "-p tools/apitests/" tells apitest where to find the test definition files.
   apitest -p tools/apitests/
   if [ $? != 0 ]; then
      echo "API tests failed"
      exit 1
   fi
else
   echo "API tests skipped, apitest tool not available. This can be installed"
   echo "from https://github.com/tucats/apitest"
   AVAIL=" available"
fi

echo " "
# "$AVAIL" expands to either "" or " available", producing either
# "All tests completed successfully" or "All available tests completed successfully".
echo "All$AVAIL tests completed successfully"

exit 0
