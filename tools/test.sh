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

# tests/time/parse.ego's "ParseAny flexible format detection" test parses a
# bare zone abbreviation ("... 10:35am EST") with no numeric offset. Per Go's
# time.Parse rules (which dateparse.ParseAny follows), a bare abbreviation is
# only resolved to a real offset if it matches the process's local timezone;
# otherwise it silently gets a +0000 offset. This pins the local timezone so
# the test's expected value is reproducible regardless of the host/container
# it runs on. See docs/issues/TIME-1.md for the underlying issue this papers
# over -- ParseAny's result depends on the caller's TZ, not just its input.
export TZ=America/New_York

echo " "
echo "Running native Go unit tests"

$(ego path)/tools/gotests.sh 

# "$?" is a special shell variable that holds the exit code of the last command.
# By convention, exit code 0 means success; anything else means failure.
# "!= 0" checks for any failure exit code.
if [ $? != 0 ]; then
   echo "Go tests failed"
   # "exit 1" stops the script immediately and reports failure to whoever ran it.
   exit 1
fi

# ego.runtime.exec defaults to false (subprocess exec is opt-in, by deliberate
# security design -- see docs/issues/CODE-H1.md). tests/exec/exec.ego expects
# exec.Command to work out of the box, which otherwise only holds on a machine
# that has separately persisted ego.runtime.exec=true from prior local use.
# --set is process-scoped (it does not write to the on-disk profile), so this
# enables it for these test runs only, without changing the actual default.
# Array, not a plain string: zsh does not word-split an unquoted scalar
# variable the way bash/sh do, so "$EXEC_TEST_ARGS" below would otherwise be
# passed as a single (invalid) two-word argument instead of two arguments.
EXEC_TEST_ARGS=(--set ego.runtime.exec=true)

echo " "
echo "Running Ego test stream with strict type checking"

# Run the Ego test suite with strict typing. In strict mode, all variables must
# be explicitly declared with a specific type, and type mismatches are errors.
./ego "${EXEC_TEST_ARGS[@]}" -q test --typing=strict
if [ $? != 0 ]; then
   echo "Ego test failure with strict typing"
   exit 1
fi


echo " "
echo "Running Ego test stream with relaxed type checking"

# Run the Ego test suite with relaxed typing. In relaxed mode, some implicit
# type conversions are allowed, making the language behave more like a scripting
# language while still enforcing basic type rules.
./ego "${EXEC_TEST_ARGS[@]}" -q test --typing=relaxed
if [ $? != 0 ]; then
   echo "Ego test failure with relaxed typing"
   exit 1
fi


echo " "
echo "Running Ego test stream with dynamic type checking"

# Run the Ego test suite with dynamic (no) type checking. In dynamic mode,
# variables can hold any type and all type coercions are automatic, similar
# to how JavaScript or Python work.
./ego "${EXEC_TEST_ARGS[@]}" -q test --typing=dynamic
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

echo "Running API tests for REST server"
# "-p tools/apitests/" tells apitest where to find the test definition files.
tools/apitest.sh -q tests/
if [ $? != 0 ]; then
   echo "TEST: API tests failed"
   exit 1
fi

# "$AVAIL" expands to either "" or " available", producing either
# "All tests completed successfully" or "All available tests completed successfully".
echo " "
echo "TEST: All$AVAIL tests completed successfully"

exit 0
