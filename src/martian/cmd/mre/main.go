//
// Copyright (c) 2014 10X Genomics, Inc. All rights reserved.
//
// Martian MRO editor.
//
package main

import (
	"github.com/martian-lang/docopt.go"
	"martian/core"
	"martian/util"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func main() {
	util.SetupSignalHandlers()

	//=========================================================================
	// Commandline argument and environment variables.
	//=========================================================================
	// Parse commandline.
	doc := `Martian MRO Editor.

Usage:
    mre [--uiport=<num>]
    mre -h | --help | --version

Options:
    --uiport=<num>  Serve UI at http://<hostname>:<num>
                      Overrides $MROPORT_EDITOR environment variable.
                      Defaults to 3601 if not otherwise specified.
    -h --help       Show this message.
    --version       Show version.`
	martianVersion := util.GetVersion()
	opts, _ := docopt.Parse(doc, nil, true, martianVersion, false)
	util.Println("Martian MRO Editor - %s", martianVersion)
	util.PrintInfo("cmdline", strings.Join(os.Args, " "))

	// Compute UI port.
	uiport := "3601"
	if value := os.Getenv("MROPORT_EDITOR"); len(value) > 0 {
		util.PrintInfo("environ", "MROPORT_EDITOR=%s", value)
		uiport = value
	}
	if value := opts["--uiport"]; value != nil {
		uiport = value.(string)
	}
	util.PrintInfo("options", "--uiport=%s", uiport)

	// Compute MRO path.
	cwd, _ := filepath.Abs(path.Dir(os.Args[0]))
	mroPaths := util.ParseMroPath(cwd)
	if value := os.Getenv("MROPATH"); len(value) > 0 {
		mroPaths = util.ParseMroPath(value)
	}
	util.PrintInfo("environ", "MROPATH=%s", util.FormatMroPath(mroPaths))

	// Compute version.
	mroVersion, _ := util.GetMroVersion(mroPaths)
	util.PrintInfo("version", "MRO Version=%s", mroVersion)

	//=========================================================================
	// Configure Martian runtime.
	//=========================================================================
	rt := core.NewRuntime("local", "disable", "disable", martianVersion)

	//=========================================================================
	// Start web server.
	//=========================================================================
	go runWebServer(uiport, rt, mroPaths)

	// Let daemons take over.
	done := make(chan bool)
	<-done
}
