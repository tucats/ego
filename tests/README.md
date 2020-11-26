# Unit tests
The "tests" directory contains Ego language unit tests. These tests are at a higher
level than the internal Go unit tests in the various packages used by Ego. The 
directory contains a set of tests, each of which is run when the `ego test` command
is executed on the command line.

A test has a basic structure:

     @test "name of test"

     // Declare any global or common values here. These must be unique across
     // all tests.

     {
         // Put each sub-task of the text in it's own braces so it has local
         // scope.

         // Use @fail to indicate that if code got to this line, it was an 
         // error. This is most often used to test flow-of-control operations
         // such as loops.
         
         @fail "incorrect switch case selected"

         // You can test for a specific value or state using @assert, which
         // requires that the expression be true, or the text of the command
         // is used as an error

         @assert count == 5 "incorrect count of values"

     }

     // The last operation should be @pass which marks the end of the test.
     // If the test completed without errors, this also reports the PASS
     // message to the console

     @pass

Each test should be written so it can run alone, or be part of a test suite. Because it
could run along with other tests, any variable declared outside the scope of each subtest
must be a unique name across all tests.

## The test command
Tests can be run as standalone programs, using the `ego run` command or by using the `%include`
operation in the console. However, they are intended to be run as a suite of tests, and can 
validate language functionality and ensure that a change does not break any language features
unexpectedly.

    ego test [file-or-path]

Use the `ego test` command to run all the tests named. The single parameter can either be a
single test program, or a directory containing one or more test programs. The directory 
is scanned recursively, so you can use subfolders to group tests. The scan is always done
in alphabetical order so you can use test names and/or subdirectory names to control the
order of execution.

## Directives
Below is additional information about each of the individual test directives.

### @test
All unit tests must start with the `@test` directive, which is followed by a string
expression (usually a quoted string constant) that labels the test. This directive
also creates an additional package in the running program called "testing" which contains
functions needed by the other test directives. If you use the other directives without
first specifying `@test` they will fail.  

A single file can contain more than one `@test`; each test gets it's own name and is 
reported to the console as a new test execution.

### @assert
The `@assert` accepts an expression that must be resolvable to a boolean value, and a
string expression that contains the message to be printed. This string expression can
include variable values from the test, as in

     @assert count == 5   "Got incorrect count value of " + string(count)

If the expression resolves to true, then no error is flagged and execution continues
to the next statement. If the boolean value is not true, the test stops executing and
the associated string expression is used to form an error message printed on the
console.

### @fail
The `@fail` is used to unilaterally fail the test. There is no conditional expression.
This is a simpler version of using `@assert fail ...`. This is most commonly used in
tests that are validating flow-of-control, such as:

    count := 5
    if count == 5 {
        // no action, all good
    } else {
        @fail "incorrect branch in if-statement"
    }

### @pass
This should be the last directive or statement in the test. If the test has not incurred
any errors at this point, a PASS message is added to the output console.

