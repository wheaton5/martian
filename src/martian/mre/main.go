//
// Copyright (c) 2014 10X Genomics, Inc. All rights reserved.
//
// Martian MRO editor.
//
package main

import (
	"github.com/docopt/docopt.go"
	"martian/core"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func main() {
	core.SetupSignalHandlers()

	//=========================================================================
	// Commandline argument and environment variables.
	//=========================================================================
	// Parse commandline.
	doc := `Martian MRO Editor.

Usage:
    mre [--port=<num>]
    mre -h | --help | --version

Options:
    --port=<num>  Serve UI at http://localhost:<num>
                    Overrides $MROPORT_EDITOR environment variable.
                    Defaults to 3601 if not otherwise specified.
    -h --help     Show this message.
    --version     Show version.`
	martianVersion := core.GetVersion()
	opts, _ := docopt.Parse(doc, nil, true, martianVersion, false)
	core.Println("Martian MRO Editor - %s", martianVersion)
	core.PrintInfo("cmdline", strings.Join(os.Args, " "))

	// Compute UI port.
	uiport := "3601"
	if value := os.Getenv("MROPORT_EDITOR"); len(value) > 0 {
		core.PrintInfo("environ", "MROPORT_EDITOR=%s", value)
		uiport = value
	}
	if value := opts["--port"]; value != nil {
		uiport = value.(string)
	}
	core.PrintInfo("options", "--uiport=%s", uiport)

	// Compute MRO path.
	cwd, _ := filepath.Abs(path.Dir(os.Args[0]))
	mroPath := cwd
	if value := os.Getenv("MROPATH"); len(value) > 0 {
		mroPath = value
	}
	core.PrintInfo("environ", "MROPATH=%s", mroPath)

	// Compute version.
	mroVersion := core.GetMroVersion(mroPath)
	core.PrintInfo("version", "MRO Version=%s", mroVersion)

	//=========================================================================
	// Configure Martian runtime.
	//=========================================================================
	rt := core.NewRuntime("local", "disable", "disable", mroPath, martianVersion, mroVersion)

	//=========================================================================
	// Start web server.
	//=========================================================================
	go runWebServer(uiport, rt, mroPath)

	// Let daemons take over.
	done := make(chan bool)
	<-done
}
