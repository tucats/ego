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

echo " "
echo "Running API tests for REST server"
go run ./tools/apitest -p ./tools/apitest/tests   
if [ $? != 0 ]; then
   echo "API tests failed"
   exit 1
fi

echo " "
echo "All tests completed successfully"

exit 0
