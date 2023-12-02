#!/bin/zsh 
#
# External DSN tests

X=$RANDOM
DSN=test_dsn_$X
TABLE=test_table_$X
DB=test_db_$X.db
USER=test_user_$X
ADMIN=admin_$X

# Create the DSN as the administrator
echo "1. log in administrator"

if ./ego -p $ADMIN logon -u admin -p password -l https://$HOST.local ; then
    echo ""
else 
    echo "**** Unexpected error"
fi

echo "2. Create DSN $DSN"

if ./ego -p $ADMIN  dsn add $DSN -d $DB -t sqlite3 ; then    
    echo ""
else 
    echo "**** Unexpected error"
fi

# Create the test username
echo "3. Create user $USER"
if ./ego -p $ADMIN  server user create $USER -p password --permissions logon,table_read,table_modify; then    
    echo ""
else 
    echo "**** Unexpected error"
fi

# Grant the username access to the database
echo "4. Grant $USER access to $DSN"
if ./ego -p $ADMIN  dsn grant $DSN -u $USER -p read,write,admin; then    
    echo ""
else 
    echo "**** Unexpected error"
fi

# Log in the test user, which should create the profile
echo "5. Log in test user $USER"
if ./ego -p $USER logon -u $USER -p password -l https://$HOST.local; then    
    echo ""
else 
    echo "**** Unexpected error"
fi

# Use the DSN to create a table
echo "6. Create table $TABLE and add two rows"
if ./ego -p $USER table create --dsn $DSN $TABLE id:int name:string; then    
    echo ""
else 
    echo "**** Unexpected error"
fi
if ./ego -p $USER table insert --dsn $DSN $TABLE id=101 name=Dick; then    
    echo ""
else 
    echo "**** Unexpected error"
fi
if ./ego -p $USER table insert --dsn $DSN $TABLE id=102 name=Jane; then    
    echo ""
else 
    echo "**** Unexpected error"
fi

echo "7. Read contents of table"
if ./ego -p $USER table read --dsn $DSN $TABLE ; then    
    echo ""
else 
    echo "**** Unexpected error"
fi

# Take away the user's privileges to modify the data
echo "8. Revoke modify $DSN privileges for user $USER"
if ./ego -p $ADMIN  dsn revoke $DSN -u $USER -p write; then    
    echo ""
else 
    echo "**** Unexpected error"
fi

# Now try to insert another row, which should fail.
echo "9. Attempt invalid insert into $TABLE"
if ./ego -p $USER table insert --dsn $DSN $TABLE id=103 name=Jack; then 
    echo "****** Unexpected success"
else
    echo ""
fi


# Clean it all up, in reverse order

echo "Clean up $TABLE, $DSN, $USER, and $DB"

# Delete the table
./ego -p $USER table drop --dsn $DSN $TABLE

# Delete the DSN
./ego -p $ADMIN  dsn delete $DSN

# Delete the user
./ego -p $ADMIN  server user delete $USER

# Delete the user profile
./ego -p $ADMIN  config remove $USER

# Delete the admin profile
./ego config remove $ADMIN 

# Delete the physical database file
echo "Delete $(rm -rfv $DB)"
