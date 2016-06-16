#!/usr/bin/env python
# Copyright (c) 2016 10X Genomics, Inc. All rights reserved.

# This is a helper program for MRT. It finds the correct paths for MRT to use, 
# builds an appropriate invocation file. runs MRT and then runs MRP. 


import sys
import os
import subprocess

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
        os.chdir(psid)


def build_invocation(base):
        p = os.path.abspath(os.path.dirname(sys.argv[0]))
        shimulate_path = p + "/../../../../../../dev/shimulate"
        print "MRT AT: " + shimulate_path
        subprocess.check_call([shimulate_path, base])
        return base + ".mro"


def run_mrt(base_path, psid, jobmode, inv, mro_path):
        args = ["mrt", "-mro=" + mro_path, "-psid=" + psid, "-base=" + base_path, "-inv" + inv, "-jobmode=" + jobmode]

        subprocess.check_call(args)

def run_mrp(args, psid, mro_path):
        arglist = ["mrp", mro_path, psid]
        arglist +=args["args_for_mrp"]
        subprocess.check_call(arglist)



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

        if (args_map["psid"] && args_map["base"] && args_map["inv"]):
                return args_map
        else
                print '''USAGE:

mrt_helper -psid=<name_of_new_pipestance> -base=<lena_id_of_base_pipestance> -inv=<one,or,more,stages>

Any other arguments we be passed unmolested to MRP.

'''
        return None

def main():
        args = sys.argv

        print args[0]
        print os.path.dirname(args[0])
        p = os.path.abspath(os.path.dirname(args[0]))
        print p

        am = parse_args(args)

        if (!am):
                return

        basedir = find_basedir(am["base"])

        if (!basedir):
                return
        
        setup_dir(am["psid"])

        build_invocation(am["base"])

        run_mrt()
        run_mrp()



main()



