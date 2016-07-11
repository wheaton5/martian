//
// Copyright (c) 2014 10X Genomics, Inc. All rights reserved.
//
// Marsoc daemon.
//
package main

import (
	"fmt"
	"martian/core"
	"martian/manager"
	"os"
	"os/user"
	"path"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/10XDev/docopt.go"
	"github.com/dustin/go-humanize"
)

func sendNotificationMail(users []string, mailer *manager.Mailer, notices []*manager.PipestanceNotification) {
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
		result := fmt.Sprintf("    %s of %s/%s is %s (http://%s)", notice.Pname, notice.Container, notice.Psid, strings.ToUpper(notice.State), url)
		results = append(results, notice.Name, result)
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

func emailNotifierLoop(pman *manager.PipestanceManager, lena *Lena, mailer *manager.Mailer) {
	go func() {
		for {
			// Copy and clear the mailQueue from PipestanceManager to avoid races.
			mailQueue := pman.CopyAndClearMailQueue()

			// Build a table of users to lists of notifications.
			// Also, collect all the notices that don't have a user associated.
			emailTable := map[string][]*manager.PipestanceNotification{}
			userlessNotices := []*manager.PipestanceNotification{}
			for _, notice := range mailQueue {
				// Get the sample with the psid in the notice.
				sample := lena.GetSampleWithId(notice.Psid)

				// If no sample, add to the userless table.
				if sample == nil {
					userlessNotices = append(userlessNotices, notice)
					continue
				}

				// Otherwise, build a list of notices for each user.
				notice.Name = sample.Description
				nlist, ok := emailTable[sample.User.Email]
				if ok {
					emailTable[sample.User.Email] = append(nlist, notice)
				} else {
					emailTable[sample.User.Email] = []*manager.PipestanceNotification{notice}
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

func processRunLoop(pool *SequencerPool, pman *manager.PipestanceManager, lena *Lena, packages *PackageManager, rt *core.Runtime, mailer *manager.Mailer) {
	go func() {
		for {
			runQueue := pool.CopyAndClearRunQueue()
			analysisQueue := pman.CopyAndClearAnalysisQueue()

			if pman.GetAutoInvoke() {
				fcids := []string{}
				for _, notice := range runQueue {
					run := notice.Run
					fcids = append(fcids, run.Fcid)
					EnqueuePreprocess(run.Fcid, rt, packages, pman, pool, mailer.InstanceName)
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
					EnqueueAllSamples(fcid, rt, packages, pman, lena, mailer.InstanceName)
				}
			}

			// Wait a bit.
			time.Sleep(time.Minute * time.Duration(30))
		}
	}()
}

func checkDirtyLoop(pman *manager.PipestanceManager, packages *PackageManager, mailer *manager.Mailer) {
	go func() {
		for {
			isDirty := packages.CheckDirtyPackages()

			if isDirty {
				pman.DisableRunLoop()
			} else {
				pman.EnableRunLoop()
			}

			time.Sleep(time.Minute * time.Duration(5))
		}
	}()
}

func verifyMros(packages *PackageManager, rt *core.Runtime, checkSrcPath bool) {
	for _, p := range packages.GetPackages() {
		if _, err := rt.CompileAll(p.MroPaths, checkSrcPath); err != nil {
			core.Println(err.Error())
			os.Exit(1)
		}
		rt.MroCache.CacheMros(p.MroPaths)
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
    --jobmode=MODE      Job manager to use. Valid options:
                            local (default), sge, lsf, or a .template file
    --localcores=NUM    Set max cores the pipeline may request at one time.
                            Only applies when --jobmode=local.
    --localmem=NUM      Set max GB the pipeline may request at one time.
                            Only applies when --jobmode=local.
    --mempercore=NUM    Specify min GB per core on your cluster.
                            Only applies in cluster jobmodes.
    --maxjobs=NUM       Set max jobs submitted to cluster at one time.
                            Only applies in cluster jobmodes.
    --jobinterval=NUM   Set delay between submitting jobs to cluster, in ms.
                            Only applies in cluster jobmodes.
    --vdrmode=MODE      Enables Volatile Data Removal. Valid options:
                            post (default), rolling, or disable
    --jobqueues=LIST    Semicolon-separated name:queue pairs for advanced use.
                            Only applies in cluster jobmodes.

    --maxprocs=NUM      Set number of processes used by MARSOC.
                            Defaults to 1.
    --check-dirty       Check packages for dirty versions.
                            Disables running pipestances if dirty.
    --autoinvoke        Turns on automatic pipestance invocation.
    --debug             Enable debug printing for package argshims.

    -h --help           Show this message.
    --version           Show version.`
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
		{"MARSOC_REDSTONE_CONFIG", "path/to/redstone/config"},
		{"MARSOC_MAX_STORAGE_MB", fmt.Sprintf(">0 or %d to disable", manager.STORAGE_UNLIMITED_BYTES)},
		{"LENA_DOWNLOAD_URL", "url"},
	}, true)

	core.LogTee(path.Join(env["MARSOC_LOG_PATH"], time.Now().Format("20060102150405")+".log"))

	// Do not log the value of these environment variables.
	envPrivate := core.EnvRequire([][]string{
		{"LENA_AUTH_TOKEN", "token"},
	}, false)

	// Parse options.
	checkDirty := opts["--check-dirty"].(bool)
	autoInvoke := opts["--autoinvoke"].(bool)
	debug := opts["--debug"].(bool)

	maxProcs := 1
	if value := opts["--maxprocs"]; value != nil {
		if value, err := strconv.Atoi(value.(string)); err == nil {
			maxProcs = value
			core.LogInfo("options", "--maxprocs=%d", maxProcs)
		}
	}

	reqCores := -1
	if value := opts["--localcores"]; value != nil {
		if value, err := strconv.Atoi(value.(string)); err == nil {
			reqCores = value
			core.LogInfo("options", "--localcores=%d", reqCores)
		}
	}
	reqMem := -1
	if value := opts["--localmem"]; value != nil {
		if value, err := strconv.Atoi(value.(string)); err == nil {
			reqMem = value
			core.LogInfo("options", "--localmem=%d", reqMem)
		}
	}
	reqMemPerCore := -1
	if value := opts["--mempercore"]; value != nil {
		if value, err := strconv.Atoi(value.(string)); err == nil {
			reqMemPerCore = value
			core.LogInfo("options", "--mempercore=%d", reqMemPerCore)
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

	// Max parallel jobs.
	maxJobs := -1
	if value := opts["--maxjobs"]; value != nil {
		if value, err := strconv.Atoi(value.(string)); err == nil {
			maxJobs = value
			core.LogInfo("options", "--maxjobs=%d", maxJobs)
		}
	}

	jobQueues := ""
	if value := opts["--jobqueues"]; value != nil {
		jobQueues = value.(string)
	}
	core.LogInfo("options", "--jobqueues=%s", jobQueues)

	vdrMode := "rolling"
	if value := opts["--vdrmode"]; value != nil {
		vdrMode = value.(string)
	}
	core.VerifyVDRMode(vdrMode)

	jobMode := "sge"
	if value := opts["--jobmode"]; value != nil {
		jobMode = value.(string)
	}
	core.LogInfo("options", "--jobmode=%s", jobMode)

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
	redstoneConfigPath := env["MARSOC_REDSTONE_CONFIG"]

	// default disable storage gating by default
	maxStorageBytes := manager.STORAGE_UNLIMITED_BYTES
	if mb, err := strconv.Atoi(env["MARSOC_MAX_STORAGE_MB"]); err == nil {
		// handle any negative value as disable
		if mb < 0 {
			maxStorageBytes = manager.STORAGE_UNLIMITED_BYTES
		} else {
			maxStorageBytes = int64(1024*1024) * int64(mb)
		}
	}
	core.LogInfo("options", "Storage high water mark: %d bytes", maxStorageBytes)
	runLoopIntervalms := 5 * 1000

	// Setup Go runtime
	runtime.GOMAXPROCS(maxProcs)

	//=========================================================================
	// Setup Martian Runtime with pipelines path.
	//=========================================================================
	profileMode := "cpu"
	stackVars := true
	zip := true
	checkSrcPath := true
	skipPreflight := false
	enableMonitor := true
	rt := core.NewRuntimeWithCores(jobMode, vdrMode, profileMode, martianVersion,
		reqCores, reqMem, reqMemPerCore, maxJobs, jobFreqMillis, jobQueues,
		stackVars, zip, skipPreflight, enableMonitor, debug, false)

	//=========================================================================
	// Setup Mailer.
	//=========================================================================
	mailer := manager.NewMailer(instanceName, emailHost, emailSender,
		emailRecipient, instanceName != "MARSOC")

	//=========================================================================
	// Setup SequencerPool, add sequencers, and load seq run cache.
	//=========================================================================
	pool := NewSequencerPool(seqrunsPath, cachePath)
	for _, seqcerName := range seqcerNames {
		pool.Add(seqcerName)
	}
	pool.LoadCache()

	//=========================================================================
	// Setup Lena and load cache.
	//=========================================================================
	lena := NewLena(lenaDownloadUrl, lenaAuthToken, cachePath, mailer)
	lena.LoadDatabase()

	//=========================================================================
	// Setup SGE qstat'er.
	//=========================================================================
	sge := NewSGE()

	//=========================================================================
	// Setup package manager.
	//=========================================================================
	packages := NewPackageManager(packagesPath, defaultPackage, debug, lena,
		mailer)
	verifyMros(packages, rt, checkSrcPath)

	//=========================================================================
	// Setup PipestanceManager and load pipestance cache.
	//=========================================================================
	pman := manager.NewPipestanceManager(rt, pipestancesPaths, scratchPaths,
		cachePath, failCoopPath, runLoopIntervalms, autoInvoke, maxStorageBytes, mailer, packages)
	pman.LoadPipestances()

	//=========================================================================
	// Start all daemon loops.
	//=========================================================================
	pool.GoInventoryLoop()
	pman.GoRunLoop()
	lena.GoDownloadLoop()
	sge.GoQStatLoop()
	emailNotifierLoop(pman, lena, mailer)
	processRunLoop(pool, pman, lena, packages, rt, mailer)

	if checkDirty {
		checkDirtyLoop(pman, packages, mailer)
	}

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
	runWebServer(uiport, instanceName, martianVersion, rt, pool, pman, lena, packages, sge, info, redstoneConfigPath)

	// Let daemons take over.
	done := make(chan bool)
	<-done
}
