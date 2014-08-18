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
	"io/ioutil"
	"margo/core"
	"net/http"
	"path"
	"runtime"
	"strconv"
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

func makeJSON(data interface{}) string {
	bytes, err := json.Marshal(data)
	if err != nil {
		return err.Error()
	}
	return string(bytes)
}

func main() {
	runtime.GOMAXPROCS(2)

	fmt.Println("[INIT]", core.Timestamp(), "MARSOC")

	// Command-line arguments.
	doc :=
		`Usage: 
    marsoc [--unfail] [--sge]
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
		{"LENA_DOWNLOAD_URL", "url"},
		{"LENA_AUTH_TOKEN", "token"},
	}, true)

	// Job mode and SGE environment variables.
	jobMode := "local"
	if sge, _ := opts["--sge"]; sge.(bool) {
		jobMode = "sge"
		core.EnvRequire([][]string{
			{"SGE_ROOT", "path/to/sge/root"},
			{"SGE_CLUSTER_NAME", "SGE cluster name"},
			{"SGE_CELL", "usually 'default'"},
		}, true)
	}

	// Process configuration vars.
	u, _ := opts["--unfail"]
	unfail := u.(bool)
	uiport := env["MARSOC_PORT"]
	pipelinesPath := env["MARSOC_PIPELINES_PATH"]
	cachePath := env["MARSOC_CACHE_PATH"]
	seqrunsPath := env["MARSOC_SEQRUNS_PATH"]
	pipestancesPath := env["MARSOC_PIPESTANCES_PATH"]
	seqcerNames := strings.Split(env["MARSOC_SEQUENCERS"], ";")
	lenaAuthToken := env["LENA_AUTH_TOKEN"]
	lenaDownloadUrl := env["LENA_DOWNLOAD_URL"]
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
	lena := NewLena(lenaDownloadUrl, lenaAuthToken, cachePath)
	lena.loadDatabase()
	lena.goDownloadLoop()

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
				if run.Preprocess == "complete" {
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
						if every && len(states) > 0 {
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
				done <- true
			}(run)
		}
		for i := 0; i < len(pool.runList); i++ {
			<-done
		}

		return makeJSON(pool.runList)
	})

	// Get fcid post.
	type FcidForm struct {
		Fcid string
	}
	app.Post("/api/get-samples", binding.Bind(FcidForm{}), func(body FcidForm, params martini.Params) string {
		fcid := body.Fcid
		samples, err := lena.getSamplesForFlowcell(fcid)
		if err != nil {
			return makeJSON(err.Error())
		}
		run := pool.find(fcid)
		preprocPipestance, _ := pman.GetPipestance(fcid, "PREPROCESS", fcid)

		for _, sample := range samples {
			pname := argshim.getPipelineForSample(sample)
			sample.Pname = pname
			sample.Psstate, _ = pman.GetPipestanceState(fcid, pname, fcid)
			if preprocPipestance != nil {
				sample.Callsrc = argshim.buildCallSourceForSample(rt, preprocPipestance, run, sample)
			}
		}
		return makeJSON(samples)
	})

	app.Post("/api/get-callsrc", binding.Bind(FcidForm{}), func(body FcidForm, params martini.Params) string {
		run, ok := pool.runTable[body.Fcid]
		if ok {
			return argshim.buildCallSourceForRun(rt, run)
		}
		return "could not build call source"
	})

	app.Get("/api/get-nodes/:container/:pname/:psid", func(p martini.Params) string {
		ser, _ := pman.GetPipestanceSerialization(p["container"], p["pname"], p["psid"])
		return makeJSON(ser)
	})
	// Get metadata contents.
	type MetadataForm struct {
		Path string
		Name string
	}
	app.Post("/api/get-metadata/:container/:pname/:psid", binding.Bind(MetadataForm{}), func(body MetadataForm, params martini.Params) string {
		if strings.Index(body.Path, "..") > -1 {
			return "'..' not allowed in path."
		}
		data, err := ioutil.ReadFile(path.Join(body.Path, "_"+body.Name))
		if err != nil {
			return err.Error()
		}
		return string(data)
	})

	// Restart failed stage.
	app.Post("/api/restart/:container/:pname/:psid/:fqname", func(p martini.Params) string {
		pman.UnfailPipestance(p["container"], p["pname"], p["psid"], p["fqname"])
		return ""
	})

	// API: Pipestance Invocation
	app.Post("/api/invoke-preprocess", binding.Bind(FcidForm{}), func(body FcidForm, params martini.Params) string {
		fcid := body.Fcid
		run := pool.find(fcid)
		err := pman.Invoke(fcid, "PREPROCESS", fcid, argshim.buildCallSourceForRun(rt, run))
		if err != nil {
			return err.Error()
		}
		return ""
	})
	app.Post("/api/invoke-analysis", binding.Bind(FcidForm{}), func(body FcidForm, params martini.Params) string {
		fcid := body.Fcid
		samples, err := lena.getSamplesForFlowcell(fcid)
		if err != nil {
			return err.Error()
		}
		run := pool.find(fcid)
		preprocPipestance, ok := pman.GetPipestance(fcid, "PREPROCESS", fcid)
		if !ok {
			return ""
		}
		for _, sample := range samples {
			pname := argshim.getPipelineForSample(sample)
			src := argshim.buildCallSourceForSample(rt, preprocPipestance, run, sample)
			pman.Invoke(fcid, pname, strconv.Itoa(sample.Id), src)
		}
		return ""
	})

	http.ListenAndServe(":"+uiport, app)
	done := make(chan bool)
	<-done
}
