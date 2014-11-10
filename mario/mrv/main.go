//
// Copyright (c) 2014 10X Genomics, Inc. All rights reserved.
//
// Mario pipeline viewer.
//
package main

import (
	"encoding/json"
	"fmt"
	"github.com/docopt/docopt-go"
	"io/ioutil"
	"mario/core"
	"os"
	"strings"
)

func main() {
	core.SetupSignalHandlers()

	//=========================================================================
	// Commandline argument and environment variables.
	//=========================================================================
	// Parse commandline.
	doc := `Mario Pipeline Viewer.

Usage: 
    mrv [options]
    mrv -h | --help | --version

Options:
    --port=<num>     Serve UI at http://localhost:<num>
    --usermap=<file> JSON file with user names and avatar URLs.
    -h --help        Show this message.
    --version        Show version.`
	marioVersion := core.GetVersion()
	opts, _ := docopt.Parse(doc, nil, true, marioVersion, false)
	core.LogInfo("*", "Mario Reverse-Proxy Viewer")
	core.LogInfo("version", marioVersion)
	core.LogInfo("cmdline", strings.Join(os.Args, " "))

	// Compute UI port.
	uiport := "8080"
	if value := opts["--port"]; value != nil {
		uiport = value.(string)
	}

	var usermap interface{}
	if umapfile := opts["--usermap"]; umapfile != nil {
		if bytes, err := ioutil.ReadFile(umapfile.(string)); err == nil {
			json.Unmarshal(bytes, &usermap)
		} else {
			fmt.Printf("%s\n", err.Error())
		}
	}

	runWebServer(uiport, usermap)

	// Let daemons take over.
	done := make(chan bool)
	<-done
}
