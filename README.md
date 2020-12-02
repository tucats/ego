# ego
Implementation of the _Ego_ language. This command accepts either an input file
(via the `run` command followed by a file name) or an interactive set of commands 
typed in from the console
(via the `run` command with no file name given ). You can use the `help` command to get a full
display of the options available.

Example:

    $ ego run
    Enter expressions to evaulate. End with a blank line.
    ego> print 3*5
    
This prints the value 15. You can enter virtually any program statement that will fit on
one line using the `interactive` command. If a statement is more complex, it may be easier
to create a text file with the code, and then compile and run the file:

Example:

     ego run test1.ego
     15


## Building

You can build the program with a simple `go build` when in the `ego` root source directory.

If you wish to increment the build number (the third integer in the version number string),
you can use the shell script `build` supplied with the repository. This depends on the 
existence of the file buildver.txt which contains the last integer value used.
