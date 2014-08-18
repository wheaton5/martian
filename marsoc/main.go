//
// Copyright (c) 2014 10X Technologies, Inc. All rights reserved.
//
// Marsoc daemon.
//
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/docopt/docopt-go"
	"github.com/go-martini/martini"
	"github.com/martini-contrib/binding"
	"html/template"
	"margo/core"
	"net/http"
	"runtime"
	"strings"
)

func render(tname string, data interface{}) string {
	tmpl, err := template.New(tname).Delims("[[", "]]").ParseFiles("../web-marsoc/templates/" + tname)
	if err != nil {
		return err.Error()
	}
	var doc bytes.Buffer
	err = tmpl.Execute(&doc, data)
	if err != nil {
		return err.Error()
	}
	return doc.String()
}

func main() {
	runtime.GOMAXPROCS(2)

	fmt.Println("[INIT]", core.Timestamp(), "MARSOC")

	// Command-line arguments.
	doc :=
		`Usage: 
    marsoc [--unfail]
    marsoc -h | --help | --version`
	opts, _ := docopt.Parse(doc, nil, true, "marsoc", false)

	// Mario environment variables.
	env := core.EnvRequire([][]string{
		{"MARSOC_PORT", ">2000"},
		{"MARSOC_JOBMODE", "local|sge"},
		{"MARSOC_SEQUENCERS", "miseq001;hiseq001"},
		{"MARSOC_SEQRUNS_PATH", "path/to/sequencers"},
		{"MARSOC_CACHE_PATH", "path/to/marsoc/cache"},
		{"MARSOC_PIPELINES_PATH", "path/to/pipelines"},
		{"MARSOC_PIPESTANCES_PATH", "path/to/pipestances"},
		{"LENA_API_URL", "url"},
		{"LENA_AUTH_TOKEN", "token"},
	}, true)

	// Job mode and SGE environment variables.
	jobMode := "local"
	if _, ok := opts["--sge"]; ok {
		jobMode = "sge"
		core.EnvRequire([][]string{
			{"SGE_ROOT", "path/to/sge/root"},
			{"SGE_CLUSTER_NAME", "SGE cluster name"},
			{"SGE_CELL", "usually 'default'"},
		}, true)
	}

	// Process configuration vars.
	_, unfail := opts["--unfail"]
	uiport := env["MARSOC_PORT"]
	pipelinesPath := env["MARSOC_PIPELINES_PATH"]
	cachePath := env["MARSOC_CACHE_PATH"]
	seqrunsPath := env["MARSOC_SEQRUNS_PATH"]
	pipestancesPath := env["MARSOC_PIPESTANCES_PATH"]
	seqcerNames := strings.Split(env["MARSOC_SEQUENCERS"], ";")
	lenaAuthToken := env["LENA_AUTH_TOKEN"]
	lenaApiUrl := env["LENA_API_URL"]
	STEP_SECS := 5

	// Setup Mario Runtime with pipelines path.
	rt := core.NewRuntime(jobMode, pipelinesPath)
	_, err := rt.CompileAll()
	core.DieIf(err)
	logInfoNoTime("CONFIG", "CODE_VERSION = %s", rt.CodeVersion)

	// Setup SequencerPool, add sequencers, load cache, start inventory loop.
	pool := NewSequencerPool(seqrunsPath, cachePath)
	for _, seqcerName := range seqcerNames {
		pool.add(seqcerName)
	}
	pool.loadCache()
	pool.goInventoryLoop()

	// Setup PipestanceManager, load cache, start runlist loop.
	pman := NewPipestanceManager(rt, pipestancesPath, cachePath, STEP_SECS)
	pman.loadCache(unfail)
	pman.goRunListLoop()

	// Setup Lena and load cache.
	lena := NewLena(lenaApiUrl, lenaAuthToken, cachePath)
	lena.loadCache()
	lena.loadDatabase()

	// Setup argshim.
	argshim := NewArgShim(pipelinesPath)
	_ = argshim

	// Start the web server.
	m := martini.New()
	r := martini.NewRouter()
	m.Use(martini.Recovery())
	m.Use(martini.Static("../web-marsoc/res", martini.StaticOptions{"", true, "index.html", nil}))
	m.Use(martini.Static("../web-marsoc/client", martini.StaticOptions{"", true, "index.html", nil}))
	m.MapTo(r, (*martini.Routes)(nil))
	m.Action(r.Handle)
	app := &martini.ClassicMartini{m, r}

	// Pages
	type Graph struct {
		Container string
		Pname     string
		Psid      string
		Admin     bool
	}
	app.Get("/", func() string { return render("marsoc.html", nil) })
	app.Get("/pipestance/:container/:pname/:psid", func(p martini.Params) string {
		return render("graph.html", &Graph{p["container"], p["pname"], p["psid"], false})
	})
	app.Get("/admin", func() string { return render("marsoc.html", map[string]bool{"Admin": true}) })
	app.Get("/admin/pipestance/:container/:pname/:psid", func(p martini.Params) string {
		return render("graph.html", &Graph{p["container"], p["pname"], p["psid"], true})
	})

	app.Get("/api/get-runs", func() string {
		done := make(chan bool)
		for _, run := range pool.runList {
			go func(run *Run) {
				run.Preprocess = nil
				if state, ok := pman.GetPipestanceState(run.Fcid, "PREPROCESS", run.Fcid); ok {
					run.Preprocess = state
				}
				run.Analysis = nil
				/*
					if run.Preprocess != "complete" {
						samples, err := lena.getSamplesForFlowcell(run.Fcid)
						if err != nil {
							fmt.Println(err.Error())
						}
						if len(samples) > 0 {
							states := []string{}
							run.Analysis = "running"
							for _, sample := range samples {
								state, ok := pman.GetPipestanceState(run.Fcid, argshim.getPipelineForSample(sample), run.Fcid)
								if ok {
									states = append(states, state)
								} else {
									run.Analysis = nil
								}
							}
							every := true
							for _, state := range states {
								if state != "complete" {
									every = false
									break
								}
							}
							if every {
								run.Analysis = "complete"
							}
							for _, state := range states {
								if state == "failed" {
									run.Analysis = "failed"
									break
								}
							}
						}

					}
				*/
				done <- true
			}(run)
		}
		for i := 0; i < len(pool.runList); i++ {
			<-done
		}
		bytes, err := json.Marshal(pool.runList)
		if err != nil {
			return err.Error()
		}
		return string(bytes)
	})

	// Get fcid post.
	type FcidForm struct {
		Fcid string
	}
	app.Post("/api/get-callsrc", binding.Bind(FcidForm{}), func(body FcidForm, params martini.Params) string {
		run, ok := pool.runTable[body.Fcid]
		if ok {
			return argshim.buildCallSourceForRun(rt, run)
		}
		return "could not build call source"
	})

	http.ListenAndServe(":"+uiport, app)
	done := make(chan bool)
	<-done
}
