//
// Copyright (c) 2014 10X Genomics, Inc. All rights reserved.
//
// Marsoc webserver.
//
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"mario/core"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"

	"github.com/go-martini/martini"
	"github.com/martini-contrib/binding"
)

//=============================================================================
// Web server helpers.
//=============================================================================

// Render a page from template.
func render(dir string, tname string, data interface{}) string {
	tmpl, err := template.New(tname).Delims("[[", "]]").ParseFiles(core.RelPath(path.Join("..", dir, tname)))
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
	InstanceName     string
	Admin            bool
	MarsocVersion    string
	PipelinesVersion string
}
type GraphPage struct {
	InstanceName string
	Container    string
	Pname        string
	Psid         string
	Admin        bool
	AdminStyle   bool
}

// Forms
type FcidForm struct {
	Fcid string
}
type MetadataForm struct {
	Path string
	Name string
}

func runWebServer(uiport string, instanceName string, marioVersion string,
	mroVersion string, rt *core.Runtime, pool *SequencerPool,
	pman *PipestanceManager, lena *Lena, argshim *ArgShim) {

	//=========================================================================
	// Configure server.
	//=========================================================================
	m := martini.New()
	r := martini.NewRouter()
	m.Use(martini.Recovery())
	m.Use(martini.Static(core.RelPath("../web-marsoc/res"), martini.StaticOptions{"", true, "index.html", nil}))
	m.Use(martini.Static(core.RelPath("../web-marsoc/client"), martini.StaticOptions{"", true, "index.html", nil}))
	m.Use(martini.Static(core.RelPath("../web/res"), martini.StaticOptions{"", true, "index.html", nil}))
	m.Use(martini.Static(core.RelPath("../web/client"), martini.StaticOptions{"", true, "index.html", nil}))
	m.MapTo(r, (*martini.Routes)(nil))
	m.Action(r.Handle)
	app := &martini.ClassicMartini{m, r}

	//=========================================================================
	// MARSOC renderers and API.
	//=========================================================================

	// Page renderers.
	app.Get("/", func() string {
		return render("web-marsoc/templates", "marsoc.html",
			&MainPage{
				InstanceName:     instanceName,
				Admin:            false,
				MarsocVersion:    marioVersion,
				PipelinesVersion: mroVersion,
			})
	})
	app.Get("/admin", func() string {
		return render("web-marsoc/templates", "marsoc.html",
			&MainPage{
				InstanceName:     instanceName,
				Admin:            true,
				MarsocVersion:    marioVersion,
				PipelinesVersion: mroVersion,
			})
	})

	// Get all sequencing runs.
	app.Get("/api/get-runs", func() string {

		// Iterate concurrently over all sequencing runs and populate or
		// update the state fields in each run before sending to client.
		var wg sync.WaitGroup
		wg.Add(len(pool.runList))
		for _, run := range pool.runList {
			go func(wg *sync.WaitGroup, run *Run) {
				defer wg.Done()

				// Get the state of the BCL_PROCESSOR_PD pipeline for this run.
				run.Preprocess = nil
				if state, ok := pman.GetPipestanceState(run.Fcid, "BCL_PROCESSOR_PD", run.Fcid); ok {
					run.Preprocess = state
				}

				// If BCL_PROCESSOR_PD is not complete yet, neither is ANALYZER_PD.
				run.Analysis = nil
				if run.Preprocess != "complete" {
					return
				}

				// Get the state of ANALYZER_PD for each sample in this run.
				samples, err := lena.getSamplesForFlowcell(run.Fcid)
				if err != nil {
					core.LogError(err, "webserv", "Error getting samples for flowcell id %s.", run.Fcid)
					return
				}
				if len(samples) == 0 {
					return
				}

				// Gather the states of ANALYZER_PD for each sample.
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

		var wg sync.WaitGroup
		wg.Add(len(samples))
		for _, sample := range samples {
			go func(wg *sync.WaitGroup, sample *Sample) {
				pname := argshim.getPipelineForSample(sample)
				sample.Pname = pname
				sample.Psstate, _ = pman.GetPipestanceState(fcid, pname, strconv.Itoa(sample.Id))
				for _, sample_def := range sample.Sample_defs {
					sd_fcid := sample_def.Sequencing_run.Name
					if preprocPipestance, _ := pman.GetPipestance(sd_fcid, "BCL_PROCESSOR_PD", sd_fcid); preprocPipestance != nil {
						if outs, ok := preprocPipestance.GetOuts(0).(map[string]interface{}); ok {
							if fastq_path, ok := outs["fastq_path"].(string); ok {
								sample_def.Sequencing_run.Fastq_path = fastq_path
							}
						}
					}
				}
				sample.Callsrc = argshim.buildCallSourceForSample(rt, lena.getSampleBagWithId(strconv.Itoa(sample.Id)))
				wg.Done()
			}(&wg, sample)
		}
		wg.Wait()
		return makeJSON(samples)
	})

	// Build BCL_PROCESSOR_PD call source.
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
		return render("web/templates", "graph.html", &GraphPage{
			InstanceName: instanceName,
			Container:    p["container"],
			Pname:        p["pname"],
			Psid:         p["psid"],
			Admin:        false,
			AdminStyle:   false,
		})
	})
	app.Get("/admin/pipestance/:container/:pname/:psid", func(p martini.Params) string {
		return render("web/templates", "graph.html", &GraphPage{
			InstanceName: instanceName,
			Container:    p["container"],
			Pname:        p["pname"],
			Psid:         p["psid"],
			Admin:        true,
			AdminStyle:   true,
		})
	})

	// Get graph nodes.
	app.Get("/api/get-state/:container/:pname/:psid", func(p martini.Params) string {
		container := p["container"]
		pname := p["pname"]
		psid := p["psid"]
		state := map[string]interface{}{}
		state["error"] = nil
		if pipestance, ok := pman.GetPipestance(container, pname, psid); ok {
			if pipestance.GetState() == "failed" {
				fqname, summary, log, errpaths := pipestance.GetFatalError()
				errpath := ""
				if len(errpaths) > 0 {
					errpath = errpaths[0]
				}
				state["error"] = map[string]string{
					"fqname":  fqname,
					"path":    errpath,
					"summary": summary,
					"log":     log,
				}
			}
		}
		ser, _ := pman.GetPipestanceSerialization(container, pname, psid)
		state["nodes"] = ser
		return makeJSON(state)
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

	// Invoke BCL_PROCESSOR_PD.
	app.Post("/api/invoke-preprocess", binding.Bind(FcidForm{}), func(body FcidForm, p martini.Params) string {
		fcid := body.Fcid
		run := pool.find(fcid)

		// Use argshim to build MRO call source and invoke.
		if err := pman.Invoke(fcid, "BCL_PROCESSOR_PD", fcid, argshim.buildCallSourceForRun(rt, run)); err != nil {
			return err.Error()
		}
		return ""
	})

	// Invoke ANALYZER_PD.
	app.Post("/api/invoke-analysis", binding.Bind(FcidForm{}), func(body FcidForm, p martini.Params) string {
		// Get the seq run with this fcid.
		fcid := body.Fcid

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

			for _, sample_def := range sample.Sample_defs {
				sd_fcid := sample_def.Sequencing_run.Name
				// Get the BCL_PROCESSOR_PD pipestance for this fcid/seq run.
				if preprocPipestance, _ := pman.GetPipestance(sd_fcid, "BCL_PROCESSOR_PD", sd_fcid); preprocPipestance != nil {
					if outs, ok := preprocPipestance.GetOuts(0).(map[string]interface{}); ok {
						if fastq_path, ok := outs["fastq_path"].(string); ok {
							sample_def.Sequencing_run.Fastq_path = fastq_path
						}
					}
				}
			}

			src := argshim.buildCallSourceForSample(rt, lena.getSampleBagWithId(strconv.Itoa(sample.Id)))

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
