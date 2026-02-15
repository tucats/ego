# tokenizer

The `tokenizer` package uses the `strings.Scan()` package to tokenize
a string buffer according to the _Ego_ language rules. These extensions
to the default `Scan()` behavior involve special tokens, such as ">="
which would normally scan as two tokens but are grouped together as a
single token.

The package also contains utility functions for navigating a cursor in
the token stream to peek, read, or advance the token stream during
compilation. Finally, it contains routines for testing the nature of
given tokens, such as determining if a token can be used as a symbol.

## Creating A Token Stream

Use the `New()` function to pass in a string containing the text to
tokenize, and receive a pointer to a `Tokenizer` object. This object
contains all the information about the tokenization of the string,
and a cursor that can be moved through the stream.

    src := "print 3+5*2"
    tokens := tokenizer.New(src)

The resulting `tokens` object can be used to scan through the tokens, read the token strings, etc.

## Reading Tokens

You can read a token explicitly which also advances the cursor one position. You can also
peek ahead or behind the cursor in the token stream to see other tokens that are not the
_current_ token. You can also test the next token, and if it matches then advance the cursor and return true.

    t := tokens.Next()

This reads the next token in the stream, and advances the cursor. The string representation of the token can be accessed from the token result using it's
`Spelling()` method.

   t := tokens.Peek(1)

This peeks ahead one token in advance of the cursor, and reads that token.
The current token position is not changed by this operation. Values greater
than zero read ahead of the current position. A value of 0 re-reads the
current position (the same value returned by the last `Next()` call, for
example). A negative value reads previously-read tokens behind the current
token position.

    if tokens.IsNext("print") {
        // Handle print operations
    }

The `IsNext()` function tests the next token to see if it matches the given
token value. If it does match, then the cursor advances one position and
the function returns true. If the next token does not match the string,
then the function returns false and the cursor is not changed. In the above
example, when the conditional block runs, the "print" token will be behind
the cursor, which is positioned at whatever token followed "print".

    thisToken := tokenizer.NewIdentifierToken("this")
    thatToken := tokenizer.NewIdentifierToken("that")

    if tokens.AnyNext(thisToken, thatToken) {
        // Handle this or that stuff
    }

This is very similar to `IsNext()` but it compares the next token to the list of token
values given, and if it matches _any_ of those tokens, the function returns true and the
token cursor is advanced. Inside the body of the condition statement in this example,
the caller can use `tokens.Peek(0)` to see what the value of the token was that matched
the list.

## Token Position

The token cursor is normally moved only when a `Next()`, `IsNext()`, or `AnyNext()`
call is made. However the caller can manually manipulate the cursor position in a
number of ways.

    tokens.Advance(1)

The `Advance()` method moves the cursor by the amount given. If the value is
positive, the cursor is moved to ahead. If the value is zero, the cursor is
unchanged. If the value is negative, the cursor is moved back that many
positions. Note that you cannot move the cursor to before the first token
or after the last token.

```go
    if tokens.AtEnd() {
        return
    }
```

The `AtEnd()` function returns true if the cursor is at the end of the
token stream. The cursor is not moved by this operation.

The caller can explicit change the token position to an absolute position,
or to record the current position (this is useful if the tokenizer must
parse ahead through a complex set of productions before determining that
the compilation is invalid and the tokenizer should be reset to try
another path).

    tokens.Reset()
    t := tokens.Mark()
    tokens.Set(t)

The first call will reset the token position to the start of the stream.
This is the same position the cursor is after the tokenizer is first
created with the `New()` method. The `Mark()` method will return the
current token position, with the intention that the calling processor
can _mark_ the current location to return to it later. The `Set()` method
is used to set a previously-collected cursor position as the new current
cursor position.
