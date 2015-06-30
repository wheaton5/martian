//
// Copyright (c) 2015 10X Genomics, Inc. All rights reserved.
//
// SERE daemon.
//
package main

import (
	"martian/core"
	"martian/manager"
	"os"
	"os/user"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/docopt/docopt.go"
)

func main() {
	core.SetupSignalHandlers()
	doc := `SERE: Martian Testing Platform.

Usage:
    sere [options]
    sere -h | --help | --version

Options:
    --debug            Enable debug printing for package argshims.
    -h --help          Show this message.
    --version          Show version.`
	martianVersion := core.GetVersion()
	opts, _ := docopt.Parse(doc, nil, true, martianVersion, false)
	core.Println("SERE - %s\n", martianVersion)

	env := core.EnvRequire([][]string{
		{"SERE_PORT", ">2000"},
		{"SERE_INSTANCE_NAME", "displayed_in_ui"},
		{"SERE_CACHE_PATH", "path/to/sere/cache"},
		{"SERE_LOG_PATH", "path/to/sere/logs"},
		{"SERE_DB_PATH", "path/to/db"},
		{"SERE_PACKAGES_PATH", "path/to/packages"},
		{"SERE_PIPESTANCES_PATH", "path/to/pipestances"},
		{"SERE_SCRATCH_PATH", "path/to/scratch/pipestances"},
		{"SERE_FAIL_COOP", "path/to/fail/coop"},
		{"SERE_EMAIL_HOST", "smtp.server.local"},
		{"SERE_EMAIL_SENDER", "email@address.com"},
		{"SERE_EMAIL_RECIPIENT", "email@address.com"},
		{"MARSOC_DOWNLOAD_URL", "url"},
	}, true)

	core.LogTee(path.Join(env["SERE_LOG_PATH"], time.Now().Format("20060102150405")+".log"))

	uiport := env["SERE_PORT"]
	instanceName := env["SERE_INSTANCE_NAME"]
	packagesPath := env["SERE_PACKAGES_PATH"]
	cachePath := env["SERE_CACHE_PATH"]
	failCoopPath := env["SERE_FAIL_COOP"]
	pipestancesPaths := strings.Split(env["SERE_PIPESTANCES_PATH"], ":")
	scratchPaths := strings.Split(env["SERE_SCRATCH_PATH"], ":")
	dbPath := env["SERE_DB_PATH"]
	emailHost := env["SERE_EMAIL_HOST"]
	emailSender := env["SERE_EMAIL_SENDER"]
	emailRecipient := env["SERE_EMAIL_RECIPIENT"]
	marsocDownloadUrl := env["MARSOC_DOWNLOAD_URL"]

	jobMode := "sge"
	vdrMode := "rolling"
	profileMode := "cpu"
	stackVars := true
	zip := true
	skipPreflight := false
	enableMonitor := true
	autoInvoke := true
	stepSecs := 5
	debug := opts["--debug"].(bool)

	// Runtime
	rt := core.NewRuntimeWithCores(jobMode, vdrMode, profileMode, martianVersion,
		-1, -1, -1, stackVars, zip, skipPreflight, enableMonitor, debug, false)

	// Mailer
	mailer := manager.NewMailer(instanceName, emailHost, emailSender,
		emailRecipient, debug)

	// Database manager
	db := NewDatabaseManager("sqlite3", dbPath)

	// Marsoc manager
	marsoc := NewMarsocManager(marsocDownloadUrl)

	// Package manager
	packages := NewPackageManager(packagesPath, debug, rt, db)

	// Pipestance manager
	pman := manager.NewPipestanceManager(rt, pipestancesPaths, scratchPaths,
		cachePath, failCoopPath, stepSecs, autoInvoke, mailer, packages)
	pman.LoadPipestances()

	//=========================================================================
	// Collect pipestance static info.
	//=========================================================================
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}
	user, err := user.Current()
	username := "unknown"
	if err == nil {
		username = user.Username
	}
	info := map[string]string{
		"hostname":   hostname,
		"username":   username,
		"cwd":        "",
		"binpath":    core.RelPath(os.Args[0]),
		"cmdline":    strings.Join(os.Args, " "),
		"pid":        strconv.Itoa(os.Getpid()),
		"version":    martianVersion,
		"pname":      "",
		"psid":       "",
		"jobmode":    jobMode,
		"maxcores":   strconv.Itoa(rt.JobManager.GetMaxCores()),
		"maxmemgb":   strconv.Itoa(rt.JobManager.GetMaxMemGB()),
		"invokepath": "",
		"invokesrc":  "",
		"mroprofile": profileMode,
		"mroport":    uiport,
	}

	//=========================================================================
	// Start all daemon loops.
	//=========================================================================
	pman.GoRunLoop()

	//=========================================================================
	// Start web server.
	//=========================================================================
	runWebServer(uiport, instanceName, martianVersion, rt, pman, marsoc, db, packages, info)

	// Let daemons take over.
	done := make(chan bool)
	<-done
}
