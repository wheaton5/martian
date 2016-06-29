#!/bin/sh

export DB=sere3

psql --host=52.39.198.116 --username=superhuman  postgres  -f setup_sere2.sql
