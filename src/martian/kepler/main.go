package main

import (
	"martian/core"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/docopt/docopt.go"
)

func main() {
	core.SetupSignalHandlers()
	doc := `Kepler.

Usage:
    keplerd
    keplerd -h | --help | --version

Options:
    -h --help       Show this message.
    --version       Show version.`
	martianVersion := core.GetVersion()
	docopt.Parse(doc, nil, true, martianVersion, false)

	env := core.EnvRequire([][]string{
		{"KEPLER_PORT", ">2000"},
		{"KEPLER_LOG_PATH", "path/to/kepler/logs"},
		{"KEPLER_DB_PATH", "path/to/db"},
		{"KEPLER_PIPESTANCES_PATH", "path/to/pipestances"},
	}, true)

	core.LogTee(path.Join(env["KEPLER_LOG_PATH"], time.Now().Format("20060102150405")+".log"))

	uiport := env["KEPLER_PORT"]
	dbPath := env["KEPLER_DB_PATH"]
	pipestancesPaths := strings.Split(env["KEPLER_PIPESTANCES_PATH"], ":")

	// Compute MRO path.
	cwd, _ := filepath.Abs(path.Dir(os.Args[0]))
	mroPath := cwd
	if value := os.Getenv("MROPATH"); len(value) > 0 {
		mroPath = value
	}
	mroVersion := core.GetMroVersion(mroPath)

	rt := core.NewRuntime("local", "disable", "disable", mroPath, martianVersion, mroVersion)
	db := NewDatabaseManager("sqlite3", dbPath)
	pman := NewPipestanceManager(pipestancesPaths, db, rt)

	// Run web server.
	go runWebServer(uiport, martianVersion, db)

	// Start pipestance manager daemon.
	pman.Start()

	// Let daemons take over.
	done := make(chan bool)
	<-done
}
