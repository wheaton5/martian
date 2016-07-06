#!/bin/sh

HOST=$1

psql --host=$HOST --username=superhuman  postgres  -f setup_ligo.sql
PASSWORD=`dd if=/dev/random bs=1 count=16 | base64 | tr / _`

psql --host=$HOST --username=superhuman postgres -c "ALTER USER x10user WITH PASSWORD '"$PASSWORD"';"

echo "postgres://x10user:$PASSWORD@$HOST/ligo"

