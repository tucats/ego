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

This program also demonstrates how to add a new function to the available builtin functions.

## pi()
This program also demonstrates how to add a new function to the available builtin functions.
This simple function accepts no arguments, and returns a float64 value for pi. It returns an error if it is passed
any parameters.
