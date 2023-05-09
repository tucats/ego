# Next Steps

This document is a sort of macro-level "to-do" list. No guarantee these will all be done,
but this is a place to map out what I think the next steps are so I can then check them off
as I go.

1. Implement POST /dsns/{dsn} to create a DSN.
1. Implement new CLI tables dsn grammar
1. Add tables dsn add to create a new DSN, as client of new endpoint above.
1. Implement DEL /dsns/{dsn} to delete a DSN.
1. Add tables dsn delete to delete an existing DSN.
1. Add /dsns/{dsn}/@grant?user=[n]&action=[n] to grant a DSN action
1. Add /dsns/{dsn}/@revoke?user=[n]&action=[n] to revoke a DSN action
1. Add /dsns/{dsn}/@privileges to list auth data for restricted DSNS.
