#!/bin/zsh -v 
#
# External DSN tests

X=$RANDOM
DSN=test_dsn_$X
TABLE=test_table_$X
DB=test_db_$X.db
USER=test_user_$X

# Create the DSN as the administrator
./ego -p admin logon -u admin -p password -l https://$HOST.local
./ego -p admin dsn add --name $DSN -d $DB -t sqlite3

# Create the test username
./ego -p admin server user create $USER -p password --permissions logon,table_read,table_modify

# Grant the username access to the database
./ego -p admin dsn grant -n $DSN -u $USER -p read,write,admin

# Log in the test user, which should create the profile
./ego -p $USER logon -u $USER -p password -l https://$HOST.local

# Use the DSN to create a table
./ego -p $USER table create --dsn $DSN $TABLE id:int name:string
./ego -p $USER table insert --dsn $DSN $TABLE id=101 name=Dick
./ego -p $USER table insert --dsn $DSN $TABLE id=101 name=Jane

./ego -p $USER table read --dsn $DSN $TABLE 

# Reverse it all

# Delete the table
./ego -p $USER table drop --dsn $DSN $TABLE

# Delete the DSN
./ego -p admin dsn delete -n $DSN

# Delete the user
./ego -p admin server user delete $USER

# Delete the user profile
./ego -p admin config remove $USER

# Delete the physical database file
rm -rfv $DB 
