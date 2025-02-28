# apitests

This directory contains API test definitions. These are run using the command
line tool `apitest`. This command line tool is available on [Github](https://github.com/tucats/apitest)
and can be installed and built using Go.

Once the tool is installed and avialable via the invocation PATH, run the tests using
a command like:

```sh
apitest -p apitests/
```

The `-p` (or `--path`) specification indicates the directory containing the test specifications.
The test files are all files that end in `.json` so you can also place data files, documentation,
etc. as needed in this directory. You can place tests in arbitarary subdirectories to organize them.
The tests are always executed in alphabetical order based on the full path name. For this reason,
test directories often start with a numeric prefix to ensure test execution order.

See the apitest [documentation](https://github.com/tucats/apitest/blob/main/README.md) for more information
on the format of the test files.
