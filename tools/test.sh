#!/bin/zsh
#
# Run the tests:
#
# 1. Go unit tests
# 2. Ego tests with strict type checks
# 3. Ego tests with relaxed type checks
# 4. Ego tests with no type checking

echo " "
echo "Running native Go unit tests"
go test ./...
if [ $? != 0 ]; then
   echo "Go tests failed"
   exit 1
fi

echo " "
echo "Running Ego test stream with strict type checking"
./ego test --typing=strict
if [ $? != 0 ]; then
   echo "Ego test failure with strict typing"
   exit 1
fi


echo " "
echo "Running Ego test stream with relaxed type checking"

./ego test --typing=relaxed
if [ $? != 0 ]; then
   echo "Ego test failure with relaxed typing"
   exit 1
fi


echo " "
echo "Running Ego test stream with dynamic type checking"

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

APITEST=$(which apitest)
AVAIL=""

if [ -f $APITEST ]; then
echo "Running API tests for REST server"
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
echo "All$AVAIL tests completed successfully"

exit 0
