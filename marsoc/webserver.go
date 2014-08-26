//
// Copyright (c) 2014 10X Technologies, Inc. All rights reserved.
//
// Marsoc webserver.
//
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/go-martini/martini"
	"github.com/martini-contrib/binding"
	"html/template"
	"io/ioutil"
	"margo/core"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
)

//=============================================================================
// Web server helpers.
//=============================================================================

// Render a page from template.
func render(tname string, data interface{}) string {
	tmpl, err := template.New(tname).Delims("[[", "]]").ParseFiles(core.RelPath("../web-marsoc/templates/" + tname))
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

// Render JSON from data.
func makeJSON(data interface{}) string {
	bytes, err := json.Marshal(data)
	if err != nil {
		return err.Error()
	}
	return string(bytes)
}

//=============================================================================
// Page and form structs.
//=============================================================================
// Pages
type MainPage struct {
	InstanceName string
	Admin        bool
}
type GraphPage struct {
	InstanceName string
	Container    string
	Pname        string
	Psid         string
	Admin        bool
}

// Forms
type FcidForm struct {
	Fcid string
}
type MetadataForm struct {
	Path string
	Name string
}

func runWebServer(uiport string, instanceName string, rt *core.Runtime, pool *SequencerPool,
	pman *PipestanceManager, lena *Lena, argshim *ArgShim) {

	//=========================================================================
	// Configure server.
	//=========================================================================
	m := martini.New()
	r := martini.NewRouter()
	m.Use(martini.Recovery())
	m.Use(martini.Static(core.RelPath("../web-marsoc/res"), martini.StaticOptions{"", true, "index.html", nil}))
	m.Use(martini.Static(core.RelPath("../web-marsoc/client"), martini.StaticOptions{"", true, "index.html", nil}))
	m.MapTo(r, (*martini.Routes)(nil))
	m.Action(r.Handle)
	app := &martini.ClassicMartini{m, r}

	//=========================================================================
	// MARSOC renderers and API.
	//=========================================================================

	// Page renderers.
	app.Get("/", func() string { return render("marsoc.html", &MainPage{instanceName, false}) })
	app.Get("/admin", func() string { return render("marsoc.html", &MainPage{instanceName, true}) })

	// Get all sequencing runs.
	app.Get("/api/get-runs", func() string {

		// Iterate concurrently over all sequencing runs and populate or
		// update the state fields in each run before sending to client.
		var wg sync.WaitGroup
		wg.Add(len(pool.runList))
		for _, run := range pool.runList {
			go func(wg *sync.WaitGroup, run *Run) {
				defer wg.Done()

				// Get the state of the PREPROCESS pipeline for this run.
				run.Preprocess = nil
				if state, ok := pman.GetPipestanceState(run.Fcid, "PREPROCESS", run.Fcid); ok {
					run.Preprocess = state
				}

				// If PREPROCESS is not complete yet, neither is ANALYTICS.
				run.Analysis = nil
				if run.Preprocess != "complete" {
					return
				}

				// Get the state of ANALYTICS for each sample in this run.
				samples, err := lena.getSamplesForFlowcell(run.Fcid)
				if err != nil {
					core.LogError(err, "webserv", "Error getting samples for flowcell id %s.", run.Fcid)
					return
				}
				if len(samples) == 0 {
					return
				}

				// Gather the states of ANALYTICS for each sample.
				states := []string{}
				run.Analysis = "running"
				for _, sample := range samples {
					state, ok := pman.GetPipestanceState(run.Fcid, argshim.getPipelineForSample(sample), strconv.Itoa(sample.Id))
					if ok {
						states = append(states, state)
					} else {
						// If some pipestance doesn't exist, show no state for analysis.
						run.Analysis = nil
						return
					}
				}

				// If every sample is complete, show analysis as complete.
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

				// If any sample is failed, show analysis as failed.
				for _, state := range states {
					if state == "failed" {
						run.Analysis = "failed"
						break
					}
				}
			}(&wg, run)
		}
		wg.Wait()

		// Send JSON for all runs in the sequencer pool.
		return makeJSON(pool.runList)
	})

	// Get samples for a given flowcell id.
	app.Post("/api/get-samples", binding.Bind(FcidForm{}), func(body FcidForm, params martini.Params) string {
		fcid := body.Fcid
		samples, err := lena.getSamplesForFlowcell(fcid)
		if err != nil {
			return makeJSON(err.Error())
		}
		run := pool.find(fcid)
		preprocPipestance, _ := pman.GetPipestance(fcid, "PREPROCESS", fcid)

		var wg sync.WaitGroup
		wg.Add(len(samples))
		for _, sample := range samples {
			go func(wg *sync.WaitGroup, sample *Sample) {
				pname := argshim.getPipelineForSample(sample)
				sample.Pname = pname
				sample.Psstate, _ = pman.GetPipestanceState(fcid, pname, strconv.Itoa(sample.Id))
				if preprocPipestance != nil {
					sample.Callsrc = argshim.buildCallSourceForSample(rt, preprocPipestance, run, sample)
				}
				wg.Done()
			}(&wg, sample)
		}
		wg.Wait()
		return makeJSON(samples)
	})

	// Build PREPROCESS call source.
	app.Post("/api/get-callsrc", binding.Bind(FcidForm{}), func(body FcidForm, params martini.Params) string {
		run, ok := pool.runTable[body.Fcid]
		if ok {
			return argshim.buildCallSourceForRun(rt, run)
		}
		return "could not build call source"
	})

	//=========================================================================
	// Pipestance graph renderers and display API.
	//=========================================================================

	// Page renderers.
	app.Get("/pipestance/:container/:pname/:psid", func(p martini.Params) string {
		return render("graph.html", &GraphPage{instanceName, p["container"], p["pname"], p["psid"], false})
	})
	app.Get("/admin/pipestance/:container/:pname/:psid", func(p martini.Params) string {
		return render("graph.html", &GraphPage{instanceName, p["container"], p["pname"], p["psid"], true})
	})

	// Get graph nodes.
	app.Get("/api/get-nodes/:container/:pname/:psid", func(p martini.Params) string {
		ser, _ := pman.GetPipestanceSerialization(p["container"], p["pname"], p["psid"])
		return makeJSON(ser)
	})

	// Get metadata file contents.
	app.Post("/api/get-metadata/:container/:pname/:psid", binding.Bind(MetadataForm{}), func(body MetadataForm, p martini.Params) string {
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

	//=========================================================================
	// Pipestance invocation API.
	//=========================================================================

	// Invoke PREPROCESS.
	app.Post("/api/invoke-preprocess", binding.Bind(FcidForm{}), func(body FcidForm, p martini.Params) string {
		fcid := body.Fcid
		run := pool.find(fcid)

		// Use argshim to build MRO call source and invoke.
		if err := pman.Invoke(fcid, "PREPROCESS", fcid, argshim.buildCallSourceForRun(rt, run)); err != nil {
			return err.Error()
		}
		return ""
	})

	// Invoke ANALYTICS.
	app.Post("/api/invoke-analysis", binding.Bind(FcidForm{}), func(body FcidForm, p martini.Params) string {
		// Get the seq run with this fcid.
		fcid := body.Fcid
		run := pool.find(fcid)

		// Get the PREPROCESS pipestance for this fcid/seq run.
		preprocPipestance, ok := pman.GetPipestance(fcid, "PREPROCESS", fcid)
		if !ok {
			return fmt.Sprintf("Could not get PREPROCESS pipestance %s.", fcid)
		}

		// Get all the samples for this fcid.
		samples, err := lena.getSamplesForFlowcell(fcid)
		if err != nil {
			return err.Error()
		}

		// Invoke the appropriate pipeline on each sample.
		errors := []string{}
		for _, sample := range samples {
			// Use argshim to pick pipeline and build MRO call source.
			pname := argshim.getPipelineForSample(sample)
			src := argshim.buildCallSourceForSample(rt, preprocPipestance, run, lena.getSampleBagWithId(strconv.Itoa(sample.Id)))

			// Invoke the pipestance.
			if err := pman.Invoke(fcid, pname, strconv.Itoa(sample.Id), src); err != nil {
				errors = append(errors, err.Error())
			}
		}
		return strings.Join(errors, "\n")
	})

	//=========================================================================
	// Pipestance archival API.
	//=========================================================================

	// Archive pipestances.
	app.Post("/api/archive-fcid-samples", binding.Bind(FcidForm{}), func(body FcidForm, p martini.Params) string {
		// Get all the samples for this fcid.
		fcid := body.Fcid
		samples, err := lena.getSamplesForFlowcell(fcid)
		if err != nil {
			return err.Error()
		}

		// Archive the samples.
		errors := []string{}
		for _, sample := range samples {
			pname := argshim.getPipelineForSample(sample)
			if err := pman.ArchivePipestanceHead(fcid, pname, strconv.Itoa(sample.Id)); err != nil {
				errors = append(errors, err.Error())
			}
		}
		return strings.Join(errors, "\n")
	})

	//=========================================================================
	// Start webserver.
	//=========================================================================
	if err := http.ListenAndServe(":"+uiport, app); err != nil {
		// Don't continue starting if we detect another instance running.
		fmt.Println(err.Error())
		os.Exit(1)
	}
}
