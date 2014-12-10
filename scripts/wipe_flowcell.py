#!/usr/bin/env python

import argparse
import json
import os
import shutil
import subprocess
import sys

def check_path(path):
    if not os.path.exists(path):
        print "%s does not exist" % path
        sys.exit(1)
    if not os.access(path, os.W_OK | os.F_OK):
        print "Permission denied: cannot write %s" % path
        sys.exit(1)

def check_env(var):
    path = os.environ.get(var)
    if not path:
        print "Please set %s environment variable" % var
        sys.exit(1)
    return path

def get_pipestances(path, name):
    pipestances = []
    if os.path.exists(path):
        for filename in os.listdir(path):
            sample_id =os.path.basename(filename)
            if not sample_id.isdigit():
                print "WARNING: %s contains non-integer sample ID %s. Exiting..." % (path, sample_id)
                sys.exit(1)
            pipestances.append("ID.%s.%s" % (sample_id, name))
    return pipestances

parser = argparse.ArgumentParser(description="Wipe analyzer and phaser pipestances from a flowcell")
parser.add_argument("flowcell", help="flowcell ID")
args = parser.parse_args()

pipestances_path = check_env("MARSOC_PIPESTANCES_PATH")
cache_path = check_env("MARSOC_CACHE_PATH")

cache_file = os.path.join(cache_path, "pipestances")
fc_path = os.path.join(pipestances_path, args.flowcell)
analyzer_path = os.path.join(fc_path, "ANALYZER_PD")
phaser_path = os.path.join(fc_path, "PHASER_SVCALLER_PD")

check_path(cache_file)
check_path(fc_path)

pipestances = get_pipestances(analyzer_path, "ANALYZER_PD")
pipestances += get_pipestances(phaser_path, "PHASER_SVCALLER_PD")

if not pipestances:
    print "No analyzer or phaser pipestances for flowcell %s found" % args.flowcell
    sys.exit(0)

answer = raw_input("Are you sure you want to delete flowcell %s? (yes|no)" % args.flowcell)
if answer.lower().strip() != "yes":
    sys.exit(0)

# Stop MARSOC
subprocess.check_call(["stop_marsoc"])

# Fix MARSOC cache file
with open(cache_file, "r") as f:
    data = f.read()
    cache = json.loads(data)
    for pipestance in pipestances:
        cache["completed"].pop(pipestance, None)
        cache["failed"].pop(pipestance, None)
with open(cache_file, 'w') as f:
    f.write(json.dumps(cache, indent=4))

# Remove pipestances
for path in [analyzer_path, phaser_path]:
    if os.path.exists(path):
        shutil.rmtree(path)

# Start MARSOC
subprocess.check_call(['start_marsoc'])
print "Successfully wiped flowcell %s" % args.flowcell
