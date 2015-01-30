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
	"martian/core"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"

	"github.com/go-martini/martini"
	"github.com/martini-contrib/binding"
	"github.com/martini-contrib/gzip"
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

type MetasampleIdForm struct {
	Id string
}

type MetadataForm struct {
	Path string
	Name string
}

type AutoInvokeForm struct {
	State bool
}

// For a given sample, update the following fields:
// Pname    The analysis pipeline to be run on it, according to argshim
// Psstate  Current state of the sample's pipestance, if any
// Callsrc  MRO invoke source to analyze this sample, per argshim
func updateSampleState(sample *Sample, rt *core.Runtime, lena *Lena,
	argshim *ArgShim, pman *PipestanceManager) map[string]string {
	pname := argshim.getPipelineForSample(sample)
	sample.Pname = pname
	sample.Psstate, _ = pman.GetPipestanceState(sample.Pscontainer, pname, strconv.Itoa(sample.Id))
	sample.Ready_to_invoke = true

	// From each def in the sample_defs, if the BCL_PROCESSOR pipestance
	// exists, add a mapping from the fcid to that pipestance's fastq_path.
	// This map will be used by the argshim to build the MRO invocation.
	fastqPaths := map[string]string{}
	for _, sample_def := range sample.Sample_defs {
		sd_fcid := sample_def.Sequencing_run.Name
		sd_state, ok := pman.GetPipestanceState(sd_fcid, "BCL_PROCESSOR_PD", sd_fcid)
		if ok {
			sample_def.Sequencing_run.Psstate = sd_state
		}
		if sd_state == "complete" {
			outs := pman.GetPipestanceOuts(sd_fcid, "BCL_PROCESSOR_PD", sd_fcid, 0)
			if fastq_path, ok := outs["fastq_path"].(string); ok {
				fastqPaths[sd_fcid] = fastq_path
			}
		} else {
			sample.Ready_to_invoke = false
		}
	}
	sample.Callsrc = argshim.buildCallSourceForSample(rt, lena.getSampleBagWithId(strconv.Itoa(sample.Id)), fastqPaths)
	return fastqPaths
}

func InvokeAnalysis(fcid string, rt *core.Runtime, lena *Lena, argshim *ArgShim, pman *PipestanceManager) string {
	// Get all the samples for this fcid.
	samples := lena.getSamplesForFlowcell(fcid)

	// Invoke the appropriate pipeline on each sample.
	errors := []string{}
	for _, sample := range samples {
		// Invoke the pipestance.
		fastqPaths := updateSampleState(sample, rt, lena, argshim, pman)
		every := true
		for _, fastqPath := range fastqPaths {
			if _, err := os.Stat(fastqPath); err != nil {
				errors = append(errors, err.Error())
				every = false
			}
		}
		if every {
			if err := pman.Invoke(sample.Pscontainer, sample.Pname, strconv.Itoa(sample.Id), sample.Callsrc); err != nil {
				errors = append(errors, err.Error())
			}
		}
	}
	return strings.Join(errors, "\n")
}

func runWebServer(uiport string, instanceName string, martianVersion string,
	mroVersion string, rt *core.Runtime, pool *SequencerPool,
	pman *PipestanceManager, lena *Lena, argshim *ArgShim, info map[string]string) {

	//=========================================================================
	// Configure server.
	//=========================================================================
	m := martini.New()
	r := martini.NewRouter()
	m.Use(martini.Recovery())
	m.Use(martini.Static(core.RelPath("../web/marsoc/res"), martini.StaticOptions{"", true, "index.html", nil}))
	m.Use(martini.Static(core.RelPath("../web/marsoc/client"), martini.StaticOptions{"", true, "index.html", nil}))
	m.Use(martini.Static(core.RelPath("../web/martian/res"), martini.StaticOptions{"", true, "index.html", nil}))
	m.Use(martini.Static(core.RelPath("../web/martian/client"), martini.StaticOptions{"", true, "index.html", nil}))
	m.MapTo(r, (*martini.Routes)(nil))
	m.Action(r.Handle)
	app := &martini.ClassicMartini{m, r}
	app.Use(gzip.All())

	//=========================================================================
	// Main run/sample UI.
	//=========================================================================
	// Render: main UI.
	app.Get("/", func() string {
		return render("web/marsoc/templates", "marsoc.html",
			&MainPage{
				InstanceName:     instanceName,
				Admin:            false,
				MarsocVersion:    martianVersion,
				PipelinesVersion: mroVersion,
			})
	})

	// Render: admin mode main UI.
	app.Get("/admin", func() string {
		return render("web/marsoc/templates", "marsoc.html",
			&MainPage{
				InstanceName:     instanceName,
				Admin:            true,
				MarsocVersion:    martianVersion,
				PipelinesVersion: mroVersion,
			})
	})

	// API: Get all sequencing runs and state.
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
				samples := lena.getSamplesForFlowcell(run.Fcid)
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

	// API: Get samples for a given flowcell id.
	app.Post("/api/get-samples", binding.Bind(FcidForm{}), func(body FcidForm, params martini.Params) string {
		samples := lena.getSamplesForFlowcell(body.Fcid)

		var wg sync.WaitGroup
		wg.Add(len(samples))
		for _, sample := range samples {
			go func(wg *sync.WaitGroup, sample *Sample) {
				updateSampleState(sample, rt, lena, argshim, pman)
				wg.Done()
			}(&wg, sample)
		}
		wg.Wait()
		return makeJSON(samples)
	})

	// API: Build BCL_PROCESSOR_PD call source.
	app.Post("/api/get-callsrc", binding.Bind(FcidForm{}), func(body FcidForm, params martini.Params) string {
		if run, ok := pool.runTable[body.Fcid]; ok {
			return argshim.buildCallSourceForRun(rt, run)
		}
		return fmt.Sprintf("Could not find run with fcid %s.", body.Fcid)
	})

	//=========================================================================
	// Metasamples UI.
	//=========================================================================
	// Render: main metasample UI.
	app.Get("/metasamples", func() string {
		return render("web/marsoc/templates", "metasamples.html",
			&MainPage{
				InstanceName:     instanceName,
				Admin:            true,
				MarsocVersion:    martianVersion,
				PipelinesVersion: mroVersion,
			})
	})

	// API: Get all metasamples and state.
	app.Get("/api/get-metasamples", func() string {
		metasamples := lena.getMetasamples()
		for _, metasample := range metasamples {
			state, ok := pman.GetPipestanceState(metasample.Pscontainer, argshim.getPipelineForSample(metasample), strconv.Itoa(metasample.Id))
			if ok {
				metasample.Psstate = state
			}
		}
		return makeJSON(lena.getMetasamples())
	})

	// API: Build analysis call source for a metasample with given id.
	app.Post("/api/get-metasample-callsrc", binding.Bind(MetasampleIdForm{}), func(body MetasampleIdForm, params martini.Params) string {
		if sample := lena.getSampleWithId(body.Id); sample != nil {
			updateSampleState(sample, rt, lena, argshim, pman)
			return makeJSON(sample)
		}
		return fmt.Sprintf("Could not find metasample with id %s.", body.Id)
	})

	// API: Invoke metasample analysis.
	app.Post("/api/invoke-metasample-analysis", binding.Bind(MetasampleIdForm{}), func(body MetasampleIdForm, p martini.Params) string {
		// Get the sample with this id.
		sample := lena.getSampleWithId(body.Id)
		if sample == nil {
			return fmt.Sprintf("Sample '%s' not found.", body.Id)
		}

		// Invoke the pipestance.
		if err := pman.Invoke(sample.Pscontainer, sample.Pname, strconv.Itoa(sample.Id), sample.Callsrc); err != nil {
			return err.Error()
		}
		return ""
	})

	// API: Restart failed metasample analysis.
	app.Post("/api/restart-metasample-analysis", binding.Bind(MetasampleIdForm{}), func(body MetasampleIdForm, p martini.Params) string {
		sample := lena.getSampleWithId(body.Id)
		if sample == nil {
			return fmt.Sprintf("Sample '%s' not found.", body.Id)
		}

		if err := pman.UnfailPipestance(sample.Pscontainer, sample.Pname, strconv.Itoa(sample.Id)); err != nil {
			return err.Error()
		}
		return ""
	})

	// API: Archive metasample pipestance.
	app.Post("/api/archive-metasample", binding.Bind(MetasampleIdForm{}), func(body MetasampleIdForm, p martini.Params) string {
		sample := lena.getSampleWithId(body.Id)
		if sample == nil {
			return fmt.Sprintf("Sample '%s' not found.", body.Id)
		}

		if err := pman.ArchivePipestanceHead(sample.Pscontainer, sample.Pname, strconv.Itoa(sample.Id)); err != nil {
			return err.Error()
		}
		return ""
	})

	// API: Wipe metasample pipestance.
	app.Post("/api/wipe-metasample", binding.Bind(MetasampleIdForm{}), func(body MetasampleIdForm, p martini.Params) string {
		sample := lena.getSampleWithId(body.Id)
		if sample == nil {
			return fmt.Sprintf("Sample '%s' not found.", body.Id)
		}

		if err := pman.WipePipestance(sample.Pscontainer, sample.Pname, strconv.Itoa(sample.Id)); err != nil {
			return err.Error()
		}
		return ""
	})

	// API: Kill metasample pipestance.
	app.Post("/api/kill-metasample", binding.Bind(MetasampleIdForm{}), func(body MetasampleIdForm, p martini.Params) string {
		sample := lena.getSampleWithId(body.Id)
		if sample == nil {
			return fmt.Sprintf("Sample '%s' not found.", body.Id)
		}

		if err := pman.KillPipestance(sample.Pscontainer, sample.Pname, strconv.Itoa(sample.Id)); err != nil {
			return err.Error()
		}
		return ""
	})

	//=========================================================================
	// Pipestance graph UI.
	//=========================================================================
	// Render: pipestance graph UI.
	app.Get("/pipestance/:container/:pname/:psid", func(p martini.Params) string {
		return render("web/martian/templates", "graph.html", &GraphPage{
			InstanceName: instanceName,
			Container:    p["container"],
			Pname:        p["pname"],
			Psid:         p["psid"],
			Admin:        false,
			AdminStyle:   false,
		})
	})

	// Render: admin mode pipestance graph UI.
	app.Get("/admin/pipestance/:container/:pname/:psid", func(p martini.Params) string {
		return render("web/martian/templates", "graph.html", &GraphPage{
			InstanceName: instanceName,
			Container:    p["container"],
			Pname:        p["pname"],
			Psid:         p["psid"],
			Admin:        true,
			AdminStyle:   true,
		})
	})

	// API: Get graph nodes and state.
	app.Get("/api/get-state/:container/:pname/:psid", func(p martini.Params) string {
		container := p["container"]
		pname := p["pname"]
		psid := p["psid"]
		state := map[string]interface{}{}
		psinfo := map[string]string{}
		for k, v := range info {
			psinfo[k] = v
		}
		psstate, _ := pman.GetPipestanceState(container, pname, psid)
		psinfo["state"] = psstate
		psinfo["pname"] = pname
		psinfo["psid"] = psid
		psinfo["invokesrc"], _ = pman.GetPipestanceInvokeSrc(container, pname, psid)
		ser, _ := pman.GetPipestanceSerialization(container, pname, psid)
		state["nodes"] = ser
		state["info"] = psinfo
		js := makeJSON(state)
		return js
	})

	// API: Get metadata file contents.
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

	// API: Invoke BCL_PROCESSOR_PD.
	app.Post("/api/invoke-preprocess", binding.Bind(FcidForm{}), func(body FcidForm, p martini.Params) string {
		// Use argshim to build MRO call source and invoke.
		fcid := body.Fcid
		run := pool.find(fcid)
		if err := pman.Invoke(fcid, "BCL_PROCESSOR_PD", fcid, argshim.buildCallSourceForRun(rt, run)); err != nil {
			return err.Error()
		}
		return ""
	})

	// API: Archive BCL_PROCESSOR_PD.
	app.Post("/api/archive-preprocess", binding.Bind(FcidForm{}), func(body FcidForm, p martini.Params) string {
		fcid := body.Fcid
		if err := pman.ArchivePipestanceHead(fcid, "BCL_PROCESSOR_PD", fcid); err != nil {
			return err.Error()
		}
		return ""
	})

	// API: Wipe BCL_PROCESSOR_PD.
	app.Post("/api/wipe-preprocess", binding.Bind(FcidForm{}), func(body FcidForm, p martini.Params) string {
		fcid := body.Fcid
		if err := pman.WipePipestance(fcid, "BCL_PROCESSOR_PD", fcid); err != nil {
			return err.Error()
		}
		return ""
	})

	// API: Kill BCL_PROCESSOR_PD.
	app.Post("/api/kill-preprocess", binding.Bind(FcidForm{}), func(body FcidForm, p martini.Params) string {
		fcid := body.Fcid
		if err := pman.KillPipestance(fcid, "BCL_PROCESSOR_PD", fcid); err != nil {
			return err.Error()
		}
		return ""
	})

	// API: Invoke analysis.
	app.Post("/api/invoke-analysis", binding.Bind(FcidForm{}), func(body FcidForm, p martini.Params) string {
		return InvokeAnalysis(body.Fcid, rt, lena, argshim, pman)
	})

	// API: Restart failed stage.
	app.Post("/api/restart/:container/:pname/:psid", func(p martini.Params) string {
		if err := pman.UnfailPipestance(p["container"], p["pname"], p["psid"]); err != nil {
			return err.Error()
		}
		return ""
	})

	// API: Archive pipestance.
	app.Post("/api/archive-fcid-samples", binding.Bind(FcidForm{}), func(body FcidForm, p martini.Params) string {
		// Get all the samples for this fcid.
		samples := lena.getSamplesForFlowcell(body.Fcid)

		// Archive the samples.
		errors := []string{}
		for _, sample := range samples {
			if err := pman.ArchivePipestanceHead(sample.Pscontainer, sample.Pname, strconv.Itoa(sample.Id)); err != nil {
				errors = append(errors, err.Error())
			}
		}
		return strings.Join(errors, "\n")
	})

	// API: Wipe pipestances.
	app.Post("/api/wipe-fcid-samples", binding.Bind(FcidForm{}), func(body FcidForm, p martini.Params) string {
		// Get all the samples for this fcid.
		samples := lena.getSamplesForFlowcell(body.Fcid)

		// Wipe the samples.
		errors := []string{}
		for _, sample := range samples {
			if err := pman.WipePipestance(sample.Pscontainer, sample.Pname, strconv.Itoa(sample.Id)); err != nil {
				errors = append(errors, err.Error())
			}
		}
		return strings.Join(errors, "\n")
	})

	// API: Kill pipestances.
	app.Post("/api/kill-fcid-samples", binding.Bind(FcidForm{}), func(body FcidForm, p martini.Params) string {
		// Get all the samples for this fcid.
		samples := lena.getSamplesForFlowcell(body.Fcid)

		// Kill the samples.
		errors := []string{}
		for _, sample := range samples {
			if err := pman.KillPipestance(sample.Pscontainer, sample.Pname, strconv.Itoa(sample.Id)); err != nil {
				errors = append(errors, err.Error())
			}
		}
		return strings.Join(errors, "\n")
	})

	// API: Restart failed pipestances associated to a flow cell.
	app.Post("/api/restart-fcid-samples", binding.Bind(FcidForm{}), func(body FcidForm, p martini.Params) string {
		// Get all the samples for this fcid.
		samples := lena.getSamplesForFlowcell(body.Fcid)

		// Unfail the pipestances.
		errors := []string{}
		for _, sample := range samples {
			if err := pman.UnfailPipestance(sample.Pscontainer, sample.Pname, strconv.Itoa(sample.Id)); err != nil {
				errors = append(errors, err.Error())
			}
		}
		return strings.Join(errors, "\n")
	})

	app.Post("/api/set-auto-invoke-status", binding.Bind(AutoInvokeForm{}), func(body AutoInvokeForm, p martini.Params) string {
		pman.autoInvoke = body.State
		return ""
	})

	app.Get("/api/get-auto-invoke-status", func(p martini.Params) string {
		return makeJSON(map[string]interface{}{
			"state": pman.autoInvoke,
		})
	})

	//=========================================================================
	// Shimulator API.
	//=========================================================================
	app.Get("/api/shimulate/:sid", func(p martini.Params) string {
		sid := p["sid"]
		sample := lena.getSampleWithId(sid)
		if sample == nil {
			return fmt.Sprintf("Sample %s not found in Lena.", sid)
		}
		return makeJSON(map[string]interface{}{
			"ready_to_invoke": sample.Ready_to_invoke,
			"sample_bag":      lena.getSampleBagWithId(sid),
			"fastq_paths":     updateSampleState(sample, rt, lena, argshim, pman),
		})
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
