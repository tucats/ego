#!/bin/zsh
#
# Dump ego tables to a backup database. Suitable for running
# as a CRON job.
#

BASE=~/pg_backups
FILE=$BASE/ego-tables-dump-$(date +%y.%m.%d-%H.%M.%S)

echo Writing backup to $FILE

pg_dump ego_tables -f $FILE
