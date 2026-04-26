#!/usr/bin/env bash
# Creates a SQLite database with a single table containing 20 columns and 100 rows
# of randomly generated data.
#
# Usage: makedb.sh <database-file> [table-name]

set -euo pipefail

DB="${1:?Usage: makedb.sh <database-file> [table-name]}"
TABLE="${2:-test_data}"

# Build the CREATE TABLE statement.
# Columns: _row_id_ (text, UUID), then a..s (19 columns = letters a through s)
COLS="_row_id_ TEXT"
for i in $(seq 0 18); do
    LETTER=$(printf "\\$(printf '%03o' $((97 + i)))")
    COLS="$COLS, $LETTER TEXT"
done

# Generate a v4-style UUID using /dev/urandom (works on macOS and Linux)
uuid() {
    local b
    b=$(od -An -N16 -tx1 /dev/urandom | tr -d ' \n')
    printf '%s-%s-%s-%s-%s\n' \
        "${b:0:8}" "${b:8:4}" "4${b:13:3}" \
        "$(printf '%x' $(( (0x${b:16:2} & 0x3f) | 0x80 )))${b:18:2}" \
        "${b:20:12}"
}

# Generate a random uppercase ASCII string of length 5-25
rand_str() {
    local len=$(( (RANDOM % 21) + 5 ))
    local chars="ABCDEFGHIJKLMNOPQRSTUVWXYZ"
    local result=""
    for (( i=0; i<len; i++ )); do
        result="${result}${chars:$(( RANDOM % 26 )):1}"
    done
    printf '%s' "$result"
}

# Build INSERT statements
INSERTS=""
for row in $(seq 1 100); do
    VALS="'$(uuid)'"
    for i in $(seq 0 18); do
        VALS="$VALS, '$(rand_str)'"
    done
    INSERTS="${INSERTS}INSERT INTO $TABLE VALUES ($VALS);"$'\n'
done

sqlite3 "$DB" <<SQL
CREATE TABLE IF NOT EXISTS $TABLE ($COLS);
$INSERTS
SQL

echo "Created '$DB' table '$TABLE' with 100 rows."
