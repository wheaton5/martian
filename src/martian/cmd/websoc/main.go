//
// Copyright (c) 2015 10X Genomics, Inc. All rights reserved.
//
package main

import (
	"martian/util"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/martian-lang/docopt.go"
)

func main() {
	util.SetupSignalHandlers()

	//=========================================================================
	// Commandline argument and environment variables.
	//=========================================================================
	// Parse commandline.
	doc := `WEBSOC: Martian Webshim API Service

Usage:
    websoc [options]
    websoc -h | --help | --version

Options:
    --maxprocs=<num>     Set number of processes used by WEBSOC.
                           Defaults to 1.
    -h --help            Show this message.
    --version            Show version.`
	martianVersion := util.GetVersion()
	opts, _ := docopt.Parse(doc, nil, true, martianVersion, false)
	util.Println("WEBSOC - %s\n", martianVersion)
	util.LogInfo("cmdline", strings.Join(os.Args, " "))

	if martianFlags := os.Getenv("MROFLAGS"); len(martianFlags) > 0 {
		martianOptions := strings.Split(martianFlags, " ")
		util.ParseMroFlags(opts, doc, martianOptions, []string{})
	}

	// Required Martian environment variables.
	env := util.EnvRequire([][]string{
		{"WEBSOC_PORT", ">2000"},
		{"WEBSOC_LOG_PATH", "path/to/websoc/logs"},
		{"WEBSOC_PACKAGES_PATH", "path/to/packages"},
	}, true)

	util.LogTee(path.Join(env["WEBSOC_LOG_PATH"], time.Now().Format("20060102150405")+".log"))

	// Parse options.
	maxProcs := 1
	if value := opts["--maxprocs"]; value != nil {
		if value, err := strconv.Atoi(value.(string)); err == nil {
			maxProcs = value
			util.LogInfo("options", "--maxprocs=%d", maxProcs)
		}
	}

	// Prepare configuration variables.
	uiport := env["WEBSOC_PORT"]
	packagesPath := env["WEBSOC_PACKAGES_PATH"]

	// Set up package manager
	packages := NewPackageManager(packagesPath, maxProcs)

	// Start webserver
	runWebServer(uiport, packages)

	// Let daemons take over.
	done := make(chan bool)
	<-done
}
