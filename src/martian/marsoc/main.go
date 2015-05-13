//
// Copyright (c) 2014 10X Genomics, Inc. All rights reserved.
//
// Marsoc daemon.
//
package main

import (
	"fmt"
	"martian/core"
	"os"
	"os/user"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/docopt/docopt.go"
	"github.com/dustin/go-humanize"
)

func sendNotificationMail(users []string, mailer *Mailer, notices []*PipestanceNotification) {
	// Build summary of the notices.
	results := []string{}
	worstState := "complete"
	psids := []string{}
	var vdrsize uint64
	for _, notice := range notices {
		psids = append(psids, notice.Psid)
		var url string
		if notice.State == "complete" {
			url = fmt.Sprintf("lena.fuzzplex.com/seq_results/sample%strim10/", notice.Psid)
		} else {
			url = fmt.Sprintf("%s.fuzzplex.com/pipestance/%s/%s/%s", mailer.InstanceName, notice.Container, notice.Pname, notice.Psid)
		}
		result := fmt.Sprintf("%s of %s/%s is %s (http://%s)", notice.Pname, notice.Container, notice.Psid, strings.ToUpper(notice.State), url)
		results = append(results, result)
		vdrsize += notice.Vdrsize
		if notice.State == "failed" {
			worstState = notice.State
			results = append(results, fmt.Sprintf("    %s: %s", notice.Stage, notice.Summary))
		}
	}

	// Compose the email.
	body := ""
	if worstState == "complete" {
		body = fmt.Sprintf("Hey Preppie,\n\nI totally nailed all your analysis!\n\n%s\n\nLena might take up to an hour to show your results.\n\nBtw I also saved you %s with VDR. Show me love!", strings.Join(results, "\n"), humanize.Bytes(vdrsize))
	} else {
		body = fmt.Sprintf("Hey Preppie,\n\nSome of your analysis failed!\n\n%s\n\nDon't feel bad, you'll get 'em next time!", strings.Join(results, "\n"))
	}
	subj := fmt.Sprintf("Analysis runs %s! (%s)", worstState, strings.Join(psids, ", "))
	mailer.Sendmail(users, subj, body)
}

func emailNotifierLoop(pman *PipestanceManager, lena *Lena, mailer *Mailer) {
	go func() {
		for {
			// Copy and clear the mailQueue from PipestanceManager to avoid races.
			mailQueue := pman.CopyAndClearMailQueue()

			// Build a table of users to lists of notifications.
			// Also, collect all the notices that don't have a user associated.
			emailTable := map[string][]*PipestanceNotification{}
			userlessNotices := []*PipestanceNotification{}
			for _, notice := range mailQueue {
				// Get the sample with the psid in the notice.
				sample := lena.getSampleWithId(notice.Psid)

				// If no sample, add to the userless table.
				if sample == nil {
					userlessNotices = append(userlessNotices, notice)
					continue
				}

				// Otherwise, build a list of notices for each user.
				nlist, ok := emailTable[sample.User.Email]
				if ok {
					emailTable[sample.User.Email] = append(nlist, notice)
				} else {
					emailTable[sample.User.Email] = []*PipestanceNotification{notice}
				}
			}

			// Send emails to all users associated with samples.
			for email, notices := range emailTable {
				sendNotificationMail([]string{email}, mailer, notices)
			}

			// Send userless notices to the admins.
			if len(userlessNotices) > 0 {
				sendNotificationMail([]string{}, mailer, userlessNotices)
			}

			// Wait a bit.
			time.Sleep(time.Minute * time.Duration(30))
		}
	}()
}

func processRunLoop(pool *SequencerPool, pman *PipestanceManager, lena *Lena, packages *PackageManager, rt *core.Runtime, mailer *Mailer) {
	go func() {
		for {
			runQueue := pool.CopyAndClearRunQueue()
			analysisQueue := pman.CopyAndClearAnalysisQueue()

			if pman.autoInvoke {
				fcids := []string{}
				for _, notice := range runQueue {
					run := notice.run
					fcids = append(fcids, run.Fcid)
					InvokePreprocess(run.Fcid, rt, packages, pman, pool, mailer.InstanceName)
				}

				// If there are new runs completed, send email.
				if len(fcids) > 0 {
					mailer.Sendmail(
						[]string{},
						fmt.Sprintf("Sequencing runs complete! (%s)", strings.Join(fcids, ", ")),
						fmt.Sprintf("Hey Preppie,\n\nI noticed sequencing runs %s are done.\n\nI started this BCL PROCESSOR party at http://%s/.",
							strings.Join(fcids, ", "), mailer.InstanceName),
					)
				}

				for _, notice := range analysisQueue {
					fcid := notice.Fcid
					InvokeAllSamples(fcid, rt, packages, pman, lena, mailer.InstanceName)
				}
			}

			// Wait a bit.
			time.Sleep(time.Minute * time.Duration(30))
		}
	}()
}

func compileAll(packages *PackageManager, rt *core.Runtime, checkSrcPath bool) {
	for _, p := range packages.getPackages() {
		if _, err := rt.CompileAll(p.mroPath, checkSrcPath); err != nil {
			core.Println(err.Error())
			os.Exit(1)
		}
	}
}

func main() {
	core.SetupSignalHandlers()

	//=========================================================================
	// Commandline argument and environment variables.
	//=========================================================================
	// Parse commandline.
	doc := `MARSOC: Martian SeqOps Command

Usage:
    marsoc [options]
    marsoc -h | --help | --version

Options:
    --vdrmode=<name>   Enables Volatile Data Removal.
                           Valid options are rolling, post and disable.
                           Defaults to rolling.
    --skip-preflight   Skips preflight stages.
    --autoinvoke       Turns on automatic pipestance invocation.
    --debug            Enable debug printing for package argshims.
    -h --help          Show this message.
    --version          Show version.`
	martianVersion := core.GetVersion()
	opts, _ := docopt.Parse(doc, nil, true, martianVersion, false)
	core.Println("MARSOC - %s\n", martianVersion)
	core.LogInfo("cmdline", strings.Join(os.Args, " "))

	if martianFlags := os.Getenv("MROFLAGS"); len(martianFlags) > 0 {
		martianOptions := strings.Split(martianFlags, " ")
		core.ParseMroFlags(opts, doc, martianOptions, []string{})
	}

	// Required Martian environment variables.
	env := core.EnvRequire([][]string{
		{"MARSOC_PORT", ">2000"},
		{"MARSOC_INSTANCE_NAME", "displayed_in_ui"},
		{"MARSOC_SEQUENCERS", "miseq001;hiseq001"},
		{"MARSOC_SEQUENCERS_PATH", "path/to/sequencers"},
		{"MARSOC_CACHE_PATH", "path/to/marsoc/cache"},
		{"MARSOC_LOG_PATH", "path/to/marsoc/logs"},
		{"MARSOC_PACKAGES_PATH", "path/to/packages"},
		{"MARSOC_DEFAULT_PACKAGE", "package"},
		{"MARSOC_PIPESTANCES_PATH", "path/to/pipestances"},
		{"MARSOC_SCRATCH_PATH", "path/to/scratch/pipestances"},
		{"MARSOC_FAIL_COOP", "path/to/fail/coop"},
		{"MARSOC_EMAIL_HOST", "smtp.server.local"},
		{"MARSOC_EMAIL_SENDER", "email@address.com"},
		{"MARSOC_EMAIL_RECIPIENT", "email@address.com"},
		{"LENA_DOWNLOAD_URL", "url"},
	}, true)

	core.LogTee(path.Join(env["MARSOC_LOG_PATH"], time.Now().Format("20060102150405")+".log"))

	// Do not log the value of these environment variables.
	envPrivate := core.EnvRequire([][]string{
		{"LENA_AUTH_TOKEN", "token"},
	}, false)

	// Parse options.
	skipPreflight := opts["--skip-preflight"].(bool)
	autoInvoke := opts["--autoinvoke"].(bool)
	debug := opts["--debug"].(bool)
	vdrMode := "rolling"
	if value := opts["--vdrmode"]; value != nil {
		vdrMode = value.(string)
	}
	core.VerifyVDRMode(vdrMode)

	// Prepare configuration variables.
	uiport := env["MARSOC_PORT"]
	instanceName := env["MARSOC_INSTANCE_NAME"]
	packagesPath := env["MARSOC_PACKAGES_PATH"]
	defaultPackage := env["MARSOC_DEFAULT_PACKAGE"]
	cachePath := env["MARSOC_CACHE_PATH"]
	seqrunsPath := env["MARSOC_SEQUENCERS_PATH"]
	failCoopPath := env["MARSOC_FAIL_COOP"]
	pipestancesPaths := strings.Split(env["MARSOC_PIPESTANCES_PATH"], ":")
	scratchPaths := strings.Split(env["MARSOC_SCRATCH_PATH"], ":")
	seqcerNames := strings.Split(env["MARSOC_SEQUENCERS"], ";")
	lenaAuthToken := envPrivate["LENA_AUTH_TOKEN"]
	lenaDownloadUrl := env["LENA_DOWNLOAD_URL"]
	emailHost := env["MARSOC_EMAIL_HOST"]
	emailSender := env["MARSOC_EMAIL_SENDER"]
	emailRecipient := env["MARSOC_EMAIL_RECIPIENT"]
	stepSecs := 5

	//=========================================================================
	// Setup Martian Runtime with pipelines path.
	//=========================================================================
	jobMode := "sge"
	profileMode := "cpu"
	stackVars := true
	tar := true
	checkSrcPath := true
	enableMonitor := true
	rt := core.NewRuntimeWithCores(jobMode, vdrMode, profileMode, martianVersion,
		-1, -1, -1, stackVars, tar, skipPreflight, enableMonitor, debug, false)

	//=========================================================================
	// Setup Mailer.
	//=========================================================================
	mailer := NewMailer(instanceName, emailHost, emailSender, emailRecipient,
		instanceName != "MARSOC")

	//=========================================================================
	// Setup SequencerPool, add sequencers, and load seq run cache.
	//=========================================================================
	pool := NewSequencerPool(seqrunsPath, cachePath)
	for _, seqcerName := range seqcerNames {
		pool.add(seqcerName)
	}
	pool.loadCache()

	//=========================================================================
	// Setup Lena and load cache.
	//=========================================================================
	lena := NewLena(lenaDownloadUrl, lenaAuthToken, cachePath, mailer)
	lena.loadDatabase()

	//=========================================================================
	// Setup SGE qstat'er.
	//=========================================================================
	sge := NewSGE()

	//=========================================================================
	// Setup package manager.
	//=========================================================================
	packages := NewPackageManager(packagesPath, defaultPackage, debug, lena)
	compileAll(packages, rt, checkSrcPath)

	//=========================================================================
	// Setup PipestanceManager and load pipestance cache.
	//=========================================================================
	pman := NewPipestanceManager(rt, pipestancesPaths, scratchPaths, cachePath,
		failCoopPath, stepSecs, autoInvoke, mailer, packages)
	pman.loadCache()
	pman.inventoryPipestances()

	//=========================================================================
	// Start all daemon loops.
	//=========================================================================
	pool.goInventoryLoop()
	pman.goRunListLoop()
	lena.goDownloadLoop()
	sge.goQStatLoop()
	emailNotifierLoop(pman, lena, mailer)
	processRunLoop(pool, pman, lena, packages, rt, mailer)

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
	// Start web server.
	//=========================================================================
	runWebServer(uiport, instanceName, martianVersion, rt, pool, pman, lena, packages, sge, info)

	// Let daemons take over.
	done := make(chan bool)
	<-done
}
