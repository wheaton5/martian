#!/usr/bin/env python
# Copyright (c) 2016 10X Genomics, Inc. All rights reserved.

# This is a helper program for MRT. It finds the correct paths for MRT to use, 
# builds an appropriate invocation file. runs MRT and then runs MRP. 


import sys
import os

MASTER_PATH = "/mnt/home/dstaff/mrtbase"

#
# Find the directory to use given a id and a base dir.  We look for filenames in the format
# baseid.version and pick the one with the greatest version.
def find_basedir(baseid):

        best_so_far = ""
        best_version_so_far = 0

        files = os.listdir(MASTER_PATH + "/")
        for f in files:
                parts = f.split(".")
                if (parts[0] == baseid):
                        version = int(parts[1])
                        if (version > best_version_so_far):
                                best_so_far = f
                                best_version_so_far = version

        if (best_so_far == ""):
                print "Failed to find %s in %s"  % (baseid, MASTER_PATH)
                return None
        else:
                return MASTER_PATH + "/" + best_so_far

               
def setup_dir(psid):
        os.mkdir(psid)


# Parse arguments.  This returns a map of "config" data.
# jobmode: The value of the job mode flag (what to pass as jobmode to mrt and mrp)
# psid: the valie of the psid flag (what to call the new pipestance)
# base: the value of the base flag (what lena ID to use)
# inv: the valie of the inv flag (what stages have been invalidated)
# args_for_mrp: an array of all other arguments.  This should be passed through to mrp
def parse_args(args):

        args_map = {}

        args_for_mrp = []
        for arg in args:
                      
                fl = arg.split("=");

                if (len(fl) <2):
                        args_for_mrp += [arg]
                else:
                        first = fl[0]
                        last = fl[1]

                        if (first == "-jobmode"):
                                args_map["jobmode"] = last
                                args_for_mrp += [arg]
                        elif  (first == "-psid"):
                                args_map["psid"] =last
                        elif (first == "-base"):
                                args_map["base"] = last
                        elif (first == "-inv"):
                                args_map["inv"] = last
                        else: 
                                args_for_mrp += [arg]

        args_map["args_for_mrp"] = args_for_mrp
        return args_map

def main():
        args = sys.argv

        am = parse_args(args)

        print am

        print find_basedir(am["base"])





main()



