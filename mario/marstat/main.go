//
// Copyright (c) 2014 10X Genomics, Inc. All rights reserved.
//
// Mario statistics.
//
package main

import (
	"mario/core"
	"os"
	"strings"

	"github.com/docopt/docopt-go"
)

func main() {
	core.SetupSignalHandlers()

	core.LogInfo("*", "MARSOC")
	core.LogInfo("cmdline", strings.Join(os.Args, " "))

	//=========================================================================
	// Commandline argument and environment variables.
	//=========================================================================
	// Parse commandline.
	doc :=
		`Usage: 
    marsoc 
    marsoc -h | --help | --version`
	opts, _ := docopt.Parse(doc, nil, true, core.GetVersion(), false)
	_ = opts

	// Required Mario environment variables.
	env := core.EnvRequire([][]string{
		{"MARSTAT_PORT", ">2000"},
		{"MARSTAT_CACHE_PATH", "path/to/marsoc/cache"},
		{"MARSTAT_SEQUENCERS", "miseq001;hiseq001"},
		{"MARSTAT_SEQRUNS_PATH", "path/to/sequencers"},
	}, true)

	// Prepare configuration variables.
	uiport := env["MARSTAT_PORT"]
	cachePath := env["MARSTAT_CACHE_PATH"]
	seqrunsPath := env["MARSTAT_SEQRUNS_PATH"]
	seqcerNames := strings.Split(env["MARSTAT_SEQUENCERS"], ";")

	//=========================================================================
	// Setup SequencerPool, add sequencers, load cache, start inventory loop.
	//=========================================================================
	pool := NewSequencerPool(seqrunsPath, cachePath)
	for _, seqcerName := range seqcerNames {
		pool.add(seqcerName)
	}
	pool.loadCache()
	pool.goInventoryLoop()

	//=========================================================================
	// Start web server.
	//=========================================================================
	go runWebServer(uiport, "marstat", pool)

	// Let daemons take over.
	done := make(chan bool)
	<-done
}
