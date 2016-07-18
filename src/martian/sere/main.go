//
// Copyright (c) 2015 10X Genomics, Inc. All rights reserved.
//
// SERE daemon.
//
package main

import (
	"fmt"
	"martian/core"
	"martian/manager"
	"os"
	"os/user"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/10XDev/docopt.go"
)

func sendNotificationMail(programName string, mailer *manager.Mailer, notices []*manager.PipestanceNotification) {
	results := []string{}
	worstState := "complete"
	for _, notice := range notices {
		testName := notice.Psid

		url := fmt.Sprintf("%s.fuzzplex.com/pipestance/%s/%s/%s", mailer.InstanceName, notice.Container, notice.Pname,
			notice.Psid)
		result := fmt.Sprintf("Test '%s' is %s (http://%s)",
			testName, strings.ToUpper(notice.State), url)
		results = append(results, result)
		if notice.State == "failed" {
			worstState = notice.State
			results = append(results, fmt.Sprintf("    %s: %s", notice.Stage, notice.Summary))
		}
	}

	subj := fmt.Sprintf("Tests %s! (%s)", worstState, programName)
	body := strings.Join(results, "\n")

	users := []string{}
	mailer.Sendmail(users, subj, body)
}

func emailNotifierLoop(pman *manager.PipestanceManager, mailer *manager.Mailer) {
	go func() {
		for {
			mailQueue := pman.CopyAndClearMailQueue()

			emailTable := map[string][]*manager.PipestanceNotification{}
			for _, notice := range mailQueue {
				programName, _, _ := parseContainerKey(notice.Container)

				notifications, ok := emailTable[programName]
				if ok {
					emailTable[programName] = append(notifications, notice)
				} else {
					emailTable[programName] = []*manager.PipestanceNotification{notice}
				}
			}

			for programName, notices := range emailTable {
				sendNotificationMail(programName, mailer, notices)
			}

			time.Sleep(time.Minute * time.Duration(30))
		}
	}()
}

func main() {
	core.SetupSignalHandlers()
	doc := `SERE: Martian Testing Platform.

Usage:
    sere [options]
    sere -h | --help | --version

Options:
    --mempercore=NUM    Specify min GB per core on your cluster.
                            Only applies in cluster jobmodes.
    --maxjobs=NUM       Set max jobs submitted to cluster at one time.
                            Only applies in cluster jobmodes.
    --jobinterval=NUM   Set delay between submitting jobs to cluster, in ms.
                            Only applies in cluster jobmodes.
    
    --debug             Enable debug printing for package argshims.
    
    -h --help           Show this message.
    --version           Show version.`
	martianVersion := core.GetVersion()
	opts, _ := docopt.Parse(doc, nil, true, martianVersion, false)
	core.Println("SERE - %s\n", martianVersion)

	if martianFlags := os.Getenv("MROFLAGS"); len(martianFlags) > 0 {
		martianOptions := strings.Split(martianFlags, " ")
		core.ParseMroFlags(opts, doc, martianOptions, []string{})
	}

	// Required Martian environment variables.
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
		{"MARSOC_MAX_STORAGE_MB", fmt.Sprintf(">0 or %d to disable", manager.STORAGE_UNLIMITED_BYTES)},
		{"MARSOC_DOWNLOAD_URL", "url"},
	}, true)

	core.LogTee(path.Join(env["SERE_LOG_PATH"], time.Now().Format("20060102150405")+".log"))

	// Parse options.
	debug := opts["--debug"].(bool)
	reqMemPerCore := -1
	if value := opts["--mempercore"]; value != nil {
		if value, err := strconv.Atoi(value.(string)); err == nil {
			reqMemPerCore = value
			core.LogInfo("options", "--mempercore=%d", reqMemPerCore)
		}
	}
	maxJobs := -1
	if value := opts["--maxjobs"]; value != nil {
		if value, err := strconv.Atoi(value.(string)); err == nil {
			maxJobs = value
			core.LogInfo("options", "--maxjobs=%d", maxJobs)
		}
	}
	// frequency (in milliseconds) that jobs will be sent to the queue
	// (this is a minimum bound, as it may take longer to emit jobs)
	jobFreqMillis := -1
	if value := opts["--jobinterval"]; value != nil {
		if value, err := strconv.Atoi(value.(string)); err == nil {
			jobFreqMillis = value
			core.LogInfo("options", "--jobinterval=%d", jobFreqMillis)
		}
	}

	// Special to resources mappings
	jobResources := env["SERE_JOBRESOURCES"]

	// Prepare configuration variables.
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

	// default disable storage gating by default
	maxStorageBytes := manager.STORAGE_UNLIMITED_BYTES
	if mb, err := strconv.Atoi(env["MARSOC_MAX_STORAGE_MB"]); err == nil {
		// handle any negative value as a command to disable
		if mb < 0 {
			maxStorageBytes = manager.STORAGE_UNLIMITED_BYTES
		} else {
			maxStorageBytes = int64(1024*1024) * int64(mb)
		}
	}
	core.LogInfo("options", "Storage high water mark: %d bytes", maxStorageBytes)

	// Runtime
	rt := core.NewRuntimeWithCores(jobMode, vdrMode, profileMode, martianVersion,
		-1, -1, reqMemPerCore, maxJobs, jobFreqMillis, jobResources, stackVars, zip,
		skipPreflight, enableMonitor, debug, false)

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
		cachePath, failCoopPath, stepSecs, autoInvoke, maxStorageBytes, mailer, packages)
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
	emailNotifierLoop(pman, mailer)

	//=========================================================================
	// Start web server.
	//=========================================================================
	runWebServer(uiport, instanceName, martianVersion, rt, pman, marsoc, db, packages, info)

	// Let daemons take over.
	done := make(chan bool)
	<-done
}
