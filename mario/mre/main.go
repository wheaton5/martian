//
// Copyright (c) 2014 10X Technologies, Inc. All rights reserved.
//
// Mario MRO editor.
//
package main

import (
	"github.com/docopt/docopt-go"
	"mario/core"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

func main() {
	runtime.GOMAXPROCS(2)

	//=========================================================================
	// Commandline argument and environment variables.
	//=========================================================================
	// Parse commandline.
	doc := `Mario MRO Editor.

Usage:
    mre [--port=<num>]
    mre -h | --help | --version

Options:
    --port=<num>  Serve UI at http://localhost:<num>
                    Overrides $MROPORT_EDITOR environment variable.
                    Defaults to 3601 if not otherwise specified.
    -h --help     Show this message.
    --version     Show version.`
	marioVersion := core.GetVersion()
	opts, _ := docopt.Parse(doc, nil, true, marioVersion, false)
	core.LogInfo("*", "Mario MRO Editor")
	core.LogInfo("version", marioVersion)
	core.LogInfo("cmdline", strings.Join(os.Args, " "))

	// Compute UI port.
	uiport := "3601"
	if value := os.Getenv("MROPORT_EDITOR"); len(value) > 0 {
		core.LogInfo("environ", "MROPORT_EDITOR = %s", value)
		uiport = value
	}
	if value := opts["--port"]; value != nil {
		uiport = value.(string)
	}

	// Compute MRO path.
	cwd, _ := filepath.Abs(path.Dir(os.Args[0]))
	mroPath := cwd
	if value := os.Getenv("MROPATH"); len(value) > 0 {
		mroPath = value
	}
	mroVersion := core.GetGitTag(mroPath)
	core.LogInfo("version", "MRO_STAGES = %s", mroVersion)
	core.LogInfo("environ", "MROPATH = %s", mroPath)

	//=========================================================================
	// Configure Mario runtime.
	//=========================================================================
	rt := core.NewRuntime("local", mroPath, marioVersion, mroVersion, false)

	//=========================================================================
	// Start web server.
	//=========================================================================
	go runWebServer(uiport, rt, mroPath)

	// Let daemons take over.
	done := make(chan bool)
	<-done
}
