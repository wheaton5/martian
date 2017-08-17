#!/usr/bin/env bash

# run this from the root of the martian repository to start a test, readonly
# instance of houston.

make houston
HOUSTON_DOWNLOAD_PATH=/mnt/houston/download HISTSIZE=1000 HOUSTON_ZENDESK_APITOKEN=nil HOUSTON_CACHE_PATH=/mnt/houston/cache HOUSTON_ZENDESK_DOMAIN=10xgenomics.zendesk.com HOUSTON_DOWNLOAD_INTERVALMIN=5 HOUSTON_DOWNLOAD_MAXMB=6000 HOUSTON_EMAIL_RECIPIENT=developers@10xgenomics.com HOUSTON_LOGS_PATH=/mnt/houston/logs HOUSTON_AMAZONS3_BUCKET=10x.uploads HOUSTON_FILES_PATH=/mnt/houston/files HOUSTON_PORT=3900 HOUSTON_EMAIL_SENDER=fuzzware@10xgenomics.com AWS_SECRET_ACCESS_KEY=nil HOUSTON_INSTANCE_NAME=HOUSTON HOUSTON_HOSTNAME=houston.fuzzplex.com HOUSTON_ZENDESK_USER=alex@10xgenomics.com LOGNAME=houston HOUSTON_PIPESTANCE_SUMMARY_PATHS=/PHASER_SVCALLER_CS/PHASER_SVCALLER/SUMMARIZE_REPORTS/fork0/files/summary.json HOUSTON_EMAIL_HOST=userve.fuzzplex.com bin/houston --readonly
