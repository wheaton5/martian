#!/bin/sh

HOST=$1

psql --host=$HOST --username=superhuman  postgres  -f setup_sere2.sql
