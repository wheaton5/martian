//
// Copyright (c) 2014 10X Genomics, Inc. All rights reserved.
//
// Marsoc webserver.
//
package main

import (
	"fmt"
	"io/ioutil"
	"martian/core"
	"martian/manager"
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
// Page and form structs.
//=============================================================================
// Pages
type MainPage struct {
	InstanceName     string
	Admin            bool
	MarsocVersion    string
	PipelinesVersion string
	PipestanceCount  int
	State            string
	Products         []string
}
type GraphPage struct {
	InstanceName string
	Container    string
	Pname        string
	Psid         string
	Admin        bool
	AdminStyle   bool
	Release      bool
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

type PipestanceForm struct {
	Fcid     string
	Pipeline string
	Psid     string
	Product  string
}

type RedstoneFile struct {
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	Include bool   `json:"include"`
}

type RedstoneForm struct {
	Sname string            `json:"sname"`
	Stype string            `json:"stype"`
	Id    string            `json:"id"`
	Itype string            `json:"itype"`
	Pname string            `json:"pname"`
	Paths map[string]string `json:"paths"`
}

// For a given sample, update the following fields:
// Pname    The analysis pipeline to be run on it, according to argshim
// Psstate  Current state of the sample's pipestance, if any
// Callsrc  MRO invoke source to analyze this sample, per argshim
func updateSampleState(sample *Sample, rt *core.Runtime, lena *Lena,
	packages *PackageManager, pman *manager.PipestanceManager) map[string]string {
	pname := packages.GetPipelineForSample(sample)
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

	/* Prepopulate the sample Callsrc to help the web UI. */
	sample.Callsrc_for_js = packages.BuildCallSourceForSample(rt, lena.GetSampleBagWithId(strconv.Itoa(sample.Id)), fastqPaths, sample, sample.Product)
	return fastqPaths
}

func GetSampleTags(sample *Sample, fastq_paths map[string]string, instanceName string) []string {
	tags := []string{core.MakeTag("instance", instanceName)}

	// Flowcells
	for _, sample_def := range sample.Sample_defs {
		sd_fcid := sample_def.Sequencing_run.Name
		tags = append(tags, core.MakeTag("flowcell", sd_fcid))
		tags = append(tags, core.MakeTag("read1_length", strconv.Itoa(sample_def.Sequencing_run.Read1_length)))
		tags = append(tags, core.MakeTag("read2_length", strconv.Itoa(sample_def.Sequencing_run.Read2_length)))
	}

	// Number and size of all fastq files
	fastq_paths_str := []string{}
	for _, fastq_path := range fastq_paths {
		fastq_paths_str = append(fastq_paths_str, fastq_path)
	}
	fastqFiles, fastqBytes := core.GetDirectorySize(fastq_paths_str)
	fastqFilesTag := core.MakeTag("fastq_files", strconv.Itoa(int(fastqFiles)))
	fastqBytesTag := core.MakeTag("fastq_bytes", strconv.FormatUint(fastqBytes, 10))
	tags = append(tags, fastqFilesTag, fastqBytesTag)

	return tags
}

func GetPreprocessTags(run *Run, fcid string, instanceName string) []string {
	tags := []string{core.MakeTag("instance", instanceName), core.MakeTag("flowcell", fcid)}

	// Number and size of all bcl files
	bclPath := path.Join(run.Path, "Data/Intensities")
	bclFiles, bclBytes := core.GetDirectorySize([]string{bclPath})
	bclFilesTag := core.MakeTag("bcl_files", strconv.Itoa(int(bclFiles)))
	bclBytesTag := core.MakeTag("bcl_bytes", strconv.FormatUint(bclBytes, 10))
	tags = append(tags, bclFilesTag, bclBytesTag)

	return tags
}

func EnqueuePreprocess(fcid string, rt *core.Runtime, packages *PackageManager, pman *manager.PipestanceManager, pool *SequencerPool, instanceName string) string {
	run, ok := pool.Find(fcid)
	if !ok {
		return fmt.Sprintf("Could not find run with fcid %s.", fcid)
	}
	tags := GetPreprocessTags(run, fcid, instanceName)
	// COULD THIS POSSIBLY BE RIGHT?
	if err := pman.Enqueue(fcid, "BCL_PROCESSOR_PD", fcid, packages.BuildCallSourceForRun(rt, run), tags, ""); err != nil {
		return err.Error()
	}
	return ""
}

func EnqueueSample(sample *Sample, rt *core.Runtime, packages *PackageManager, pman *manager.PipestanceManager, lena *Lena, instanceName string, product string) string {
	// Invoke the pipestance.
	fastqPaths := updateSampleState(sample, rt, lena, packages, pman)
	errors := []string{}
	every := true
	for _, fastqPath := range fastqPaths {
		if _, err := os.Stat(fastqPath); err != nil {
			errors = append(errors, err.Error())
			every = false
		}
	}
	if every {
		tags := GetSampleTags(sample, fastqPaths, instanceName)
		real_product := sample.Product
		if product != "" && product != "Default" {
			real_product = product
		}

		if !packages.CheckProduct(real_product) {
			return fmt.Sprintf("Bad product '%s'.", real_product)

		}
		callsrc := packages.BuildCallSourceForSample(rt,
			lena.GetSampleBagWithId(strconv.Itoa(sample.Id)),
			fastqPaths,
			sample,
			real_product)
		if err := pman.Enqueue(sample.Pscontainer, sample.Pname, strconv.Itoa(sample.Id), callsrc, tags, real_product); err != nil {
			errors = append(errors, err.Error())
		}
	}
	return strings.Join(errors, "\n")
}

func EnqueueAllSamples(fcid string, rt *core.Runtime, packages *PackageManager, pman *manager.PipestanceManager, lena *Lena, instanceName string) string {
	// Get all the samples for this fcid.
	samples := lena.GetSamplesForFlowcell(fcid)

	// Invoke the appropriate pipeline on each sample.
	errors := []string{}
	for _, sample := range samples {
		if error := EnqueueSample(sample, rt, packages, pman, lena, instanceName, sample.Product); len(error) > 0 {
			errors = append(errors, error)
		}
	}
	return strings.Join(errors, "\n")
}

func callPipestanceAPI(body PipestanceForm, pipestanceFunc manager.PipestanceFunc) string {
	if err := pipestanceFunc(body.Fcid, body.Pipeline, body.Psid); err != nil {
		return err.Error()
	}
	return ""
}

func callMetasamplePipestanceAPI(body MetasampleIdForm, lena *Lena, pipestanceFunc manager.PipestanceFunc) string {
	// Get the sample with this id.
	sample := lena.GetSampleWithId(body.Id)
	if sample == nil {
		return fmt.Sprintf("Sample '%s' not found.", body.Id)
	}

	if err := pipestanceFunc(sample.Pscontainer, sample.Pname, strconv.Itoa(sample.Id)); err != nil {
		return err.Error()
	}
	return ""
}

func callFcidPipestanceAPI(body FcidForm, lena *Lena, pipestanceFunc manager.PipestanceFunc) string {
	// Get all the samples for this fcid.
	samples := lena.GetSamplesForFlowcell(body.Fcid)

	errors := []string{}
	for _, sample := range samples {
		if err := pipestanceFunc(sample.Pscontainer, sample.Pname, strconv.Itoa(sample.Id)); err != nil {
			errors = append(errors, err.Error())
		}
	}
	return strings.Join(errors, "\n")
}

func callPreprocessAPI(body FcidForm, pipestanceFunc manager.PipestanceFunc) string {
	fcid := body.Fcid
	if err := pipestanceFunc(fcid, "BCL_PROCESSOR_PD", fcid); err != nil {
		return err.Error()
	}
	return ""
}

func runWebServer(uiport string, instanceName string, martianVersion string, rt *core.Runtime,
	pool *SequencerPool, pman *manager.PipestanceManager, lena *Lena,
	packages *PackageManager, sge *SGE, info map[string]string, redstoneConfigPath string) {

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
		return core.Render("web/marsoc/templates", "marsoc.html",
			&MainPage{
				InstanceName:     instanceName,
				Admin:            false,
				MarsocVersion:    martianVersion,
				PipelinesVersion: packages.GetMroVersion(),
				PipestanceCount:  pman.CountRunningPipestances(),
				Products:         packages.ListPackages(),
			})
	})

	// Render: admin mode main UI.
	app.Get("/admin", func() string {
		return core.Render("web/marsoc/templates", "marsoc.html",
			&MainPage{
				InstanceName:     instanceName,
				Admin:            true,
				MarsocVersion:    martianVersion,
				PipelinesVersion: packages.GetMroVersion(),
				PipestanceCount:  pman.CountRunningPipestances(),
				Products:         packages.ListPackages(),
			})
	})

	// API: Get all sequencing runs and state.
	app.Get("/api/get-runs", func() string {
		// Iterate concurrently over all sequencing runs and populate or
		// update the state fields in each run before sending to client.
		var wg sync.WaitGroup
		runList := pool.GetRunList()
		wg.Add(len(runList))
		for _, run := range runList {
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
				samples := lena.GetSamplesForFlowcell(run.Fcid)
				if len(samples) == 0 {
					return
				}

				// Gather the states of ANALYZER_PD for each sample.
				states := make([]core.MetadataState, 0, len(samples))
				run.Analysis = core.Running
				for _, sample := range samples {
					state, ok := pman.GetPipestanceState(run.Fcid, packages.GetPipelineForSample(sample), strconv.Itoa(sample.Id))
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
					if state != core.Complete {
						every = false
						break
					}
				}
				if every && len(states) > 0 {
					run.Analysis = core.Complete
				}

				// If any sample is failed, show analysis as failed.
				for _, state := range states {
					if state == core.Failed {
						run.Analysis = core.Failed
						break
					}
				}
			}(&wg, run)
		}
		wg.Wait()

		// Send JSON for all runs in the sequencer pool.
		return core.MakeJSON(runList)
	})

	// API: Get samples for a given flowcell id.
	app.Post("/api/get-samples", binding.Bind(FcidForm{}), func(body FcidForm, params martini.Params) string {
		samples := lena.GetSamplesForFlowcell(body.Fcid)

		var wg sync.WaitGroup
		wg.Add(len(samples))
		for _, sample := range samples {
			go func(wg *sync.WaitGroup, sample *Sample) {
				updateSampleState(sample, rt, lena, packages, pman)
				wg.Done()
			}(&wg, sample)
		}
		wg.Wait()
		return core.MakeJSON(samples)
	})

	// API: Build BCL_PROCESSOR_PD call source.
	app.Post("/api/get-callsrc", binding.Bind(FcidForm{}), func(body FcidForm, params martini.Params) string {
		if run, ok := pool.Find(body.Fcid); ok {
			return packages.BuildCallSourceForRun(rt, run)
		}
		return fmt.Sprintf("Could not find run with fcid %s.", body.Fcid)
	})

	//
	//=========================================================================
	// Pipestances UI.
	//=========================================================================
	app.Get("/pipestances", func() string {
		return core.Render("web/marsoc/templates", "pipestances.html",
			&MainPage{
				InstanceName:     instanceName,
				Admin:            false,
				MarsocVersion:    martianVersion,
				PipelinesVersion: packages.GetMroVersion(),
				PipestanceCount:  pman.CountRunningPipestances(),
				Products:         packages.ListPackages(),
			})
	})

	app.Get("/pipestances/:state", func(p martini.Params) string {
		return core.Render("web/marsoc/templates", "pipestances.html",
			&MainPage{
				InstanceName:     instanceName,
				Admin:            false,
				MarsocVersion:    martianVersion,
				PipelinesVersion: packages.GetMroVersion(),
				PipestanceCount:  pman.CountRunningPipestances(),
				State:            p["state"],
			})
	})

	app.Get("/admin/pipestances", func() string {
		return core.Render("web/marsoc/templates", "pipestances.html",
			&MainPage{
				InstanceName:     instanceName,
				Admin:            true,
				MarsocVersion:    martianVersion,
				PipelinesVersion: packages.GetMroVersion(),
				PipestanceCount:  pman.CountRunningPipestances(),
				Products:         packages.ListPackages(),
			})
	})

	app.Get("/admin/pipestances/:state", func(p martini.Params) string {
		return core.Render("web/marsoc/templates", "pipestances.html",
			&MainPage{
				InstanceName:     instanceName,
				Admin:            true,
				MarsocVersion:    martianVersion,
				PipelinesVersion: packages.GetMroVersion(),
				PipestanceCount:  pman.CountRunningPipestances(),
				State:            p["state"],
			})
	})

	app.Get("/api/get-pipestances", func() string {
		pipestances := []interface{}{}
		pipestanceMutex := &sync.Mutex{}

		var wg sync.WaitGroup
		runList := pool.GetRunList()
		wg.Add(len(runList))
		for _, run := range runList {
			go func(wg *sync.WaitGroup, run *Run) {
				defer wg.Done()

				if run.State != "complete" {
					return
				}

				runPipestances := []interface{}{}
				state, ok := pman.GetPipestanceState(run.Fcid, "BCL_PROCESSOR_PD", run.Fcid)
				if !ok {
					state = "ready"
				}
				runPipestances = append(runPipestances,
					map[string]interface{}{
						"fcid":     run.Fcid,
						"pipeline": "BCL_PROCESSOR_PD",
						"psid":     run.Fcid,
						"state":    state,
					})

				if state == "complete" {
					samples := lena.GetSamplesForFlowcell(run.Fcid)
					for _, sample := range samples {
						pipeline := packages.GetPipelineForSample(sample)
						psid := strconv.Itoa(sample.Id)

						state, ok := pman.GetPipestanceState(run.Fcid, pipeline, psid)
						if !ok {
							state = "ready"
						}

						runPipestances = append(runPipestances,
							map[string]interface{}{
								"name":     sample.Description,
								"fcid":     run.Fcid,
								"pipeline": pipeline,
								"psid":     psid,
								"state":    state,
							})
					}
				}

				if len(runPipestances) > 0 {
					pipestanceMutex.Lock()
					pipestances = append(pipestances, runPipestances...)
					pipestanceMutex.Unlock()
				}
			}(&wg, run)
		}
		metasamples := lena.GetMetasamples()
		wg.Add(len(metasamples))
		for _, metasample := range metasamples {
			go func(wg *sync.WaitGroup, metasample *Sample) {
				defer wg.Done()

				container := metasample.Pscontainer
				pipeline := packages.GetPipelineForSample(metasample)
				psid := strconv.Itoa(metasample.Id)

				state, ok := pman.GetPipestanceState(container, pipeline, psid)
				if !ok {
					for _, sample_def := range metasample.Sample_defs {
						fcid := sample_def.Sequencing_run.Name
						if state, _ := pman.GetPipestanceState(fcid, "BCL_PROCESSOR_PD", fcid); state != "complete" {
							return
						}
					}
					state = "ready"
				}

				pipestanceMutex.Lock()
				pipestances = append(pipestances,
					map[string]interface{}{
						"name":     metasample.Description,
						"fcid":     container,
						"pipeline": pipeline,
						"psid":     psid,
						"state":    state,
					})
				pipestanceMutex.Unlock()
			}(&wg, metasample)
		}
		wg.Wait()
		return core.MakeJSON(pipestances)
	})

	app.Post("/api/restart-sample", binding.Bind(PipestanceForm{}), func(body PipestanceForm, p martini.Params) string {
		return callPipestanceAPI(body, pman.UnfailPipestance)
	})

	app.Post("/api/archive-sample", binding.Bind(PipestanceForm{}), func(body PipestanceForm, p martini.Params) string {
		return callPipestanceAPI(body, pman.ArchivePipestanceHead)
	})

	app.Post("/api/wipe-sample", binding.Bind(PipestanceForm{}), func(body PipestanceForm, p martini.Params) string {
		return callPipestanceAPI(body, pman.WipePipestance)
	})

	app.Post("/api/kill-sample", binding.Bind(PipestanceForm{}), func(body PipestanceForm, p martini.Params) string {
		return callPipestanceAPI(body, pman.KillPipestance)
	})

	app.Post("/api/invoke-sample", binding.Bind(PipestanceForm{}), func(body PipestanceForm, p martini.Params) string {
		if body.Pipeline == "BCL_PROCESSOR_PD" {
			return EnqueuePreprocess(body.Fcid, rt, packages, pman, pool, instanceName)
		}

		sample := lena.GetSampleWithId(body.Psid)
		if sample == nil {
			return fmt.Sprintf("Sample '%s' not found.", body.Psid)
		}

		return EnqueueSample(sample, rt, packages, pman, lena, instanceName, body.Product)
	})

	//=========================================================================
	// Metasamples UI.
	//=========================================================================
	// Render: main metasample UI.
	app.Get("/metasamples", func() string {
		return core.Render("web/marsoc/templates", "metasamples.html",
			&MainPage{
				InstanceName:     instanceName,
				Admin:            false,
				MarsocVersion:    martianVersion,
				PipelinesVersion: packages.GetMroVersion(),
				PipestanceCount:  pman.CountRunningPipestances(),
			})
	})
	app.Get("/admin/metasamples", func() string {
		return core.Render("web/marsoc/templates", "metasamples.html",
			&MainPage{
				InstanceName:     instanceName,
				Admin:            true,
				MarsocVersion:    martianVersion,
				PipelinesVersion: packages.GetMroVersion(),
				PipestanceCount:  pman.CountRunningPipestances(),
			})
	})

	// API: Get all metasamples and state.
	app.Get("/api/get-metasamples", func() string {
		metasamples := lena.GetMetasamples()
		for _, metasample := range metasamples {
			state, ok := pman.GetPipestanceState(metasample.Pscontainer, packages.GetPipelineForSample(metasample), strconv.Itoa(metasample.Id))
			if ok {
				metasample.Psstate = state
			}
		}
		return core.MakeJSON(lena.GetMetasamples())
	})

	// API: Build analysis call source for a metasample with given id.
	app.Post("/api/get-metasample-callsrc", binding.Bind(MetasampleIdForm{}), func(body MetasampleIdForm, params martini.Params) string {
		if sample := lena.GetSampleWithId(body.Id); sample != nil {
			updateSampleState(sample, rt, lena, packages, pman)
			return core.MakeJSON(sample)
		}
		return fmt.Sprintf("Could not find metasample with id %s.", body.Id)
	})

	// API: Invoke metasample analysis.
	app.Post("/api/invoke-metasample-analysis", binding.Bind(MetasampleIdForm{}), func(body MetasampleIdForm, p martini.Params) string {
		// Get the sample with this id.
		sample := lena.GetSampleWithId(body.Id)
		if sample == nil {
			return fmt.Sprintf("Sample '%s' not found.", body.Id)
		}
		return EnqueueSample(sample, rt, packages, pman, lena, instanceName, sample.Product)
	})

	// API: Restart failed metasample analysis.
	app.Post("/api/restart-metasample-analysis", binding.Bind(MetasampleIdForm{}), func(body MetasampleIdForm, p martini.Params) string {
		return callMetasamplePipestanceAPI(body, lena, pman.UnfailPipestance)
	})

	// API: Archive metasample pipestance.
	app.Post("/api/archive-metasample", binding.Bind(MetasampleIdForm{}), func(body MetasampleIdForm, p martini.Params) string {
		return callMetasamplePipestanceAPI(body, lena, pman.ArchivePipestanceHead)
	})

	// API: Wipe metasample pipestance.
	app.Post("/api/wipe-metasample", binding.Bind(MetasampleIdForm{}), func(body MetasampleIdForm, p martini.Params) string {
		return callMetasamplePipestanceAPI(body, lena, pman.WipePipestance)
	})

	// API: Kill metasample pipestance.
	app.Post("/api/kill-metasample", binding.Bind(MetasampleIdForm{}), func(body MetasampleIdForm, p martini.Params) string {
		return callMetasamplePipestanceAPI(body, lena, pman.KillPipestance)
	})

	//=========================================================================
	// Pipestance graph UI.
	//=========================================================================
	// Render: pipestance graph UI.
	app.Get("/pipestance/:container/:pname/:psid", func(p martini.Params) string {
		return core.Render("web/martian/templates", "graph.html", &GraphPage{
			InstanceName: instanceName,
			Container:    p["container"],
			Pname:        p["pname"],
			Psid:         p["psid"],
			Admin:        false,
			AdminStyle:   false,
			Release:      core.IsRelease(),
		})
	})

	// Render: admin mode pipestance graph UI.
	app.Get("/admin/pipestance/:container/:pname/:psid", func(p martini.Params) string {
		return core.Render("web/martian/templates", "graph.html", &GraphPage{
			InstanceName: instanceName,
			Container:    p["container"],
			Pname:        p["pname"],
			Psid:         p["psid"],
			Admin:        true,
			AdminStyle:   true,
			Release:      core.IsRelease(),
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
		psinfo["state"] = string(psstate)
		psinfo["pname"] = pname
		psinfo["psid"] = psid
		psinfo["start"], _ = pman.GetPipestanceTimestamp(container, pname, psid)
		psinfo["invokesrc"], _ = pman.GetPipestanceInvokeSrc(container, pname, psid)
		martianVersion, mroVersion, _ := pman.GetPipestanceVersions(container, pname, psid)
		psinfo["version"] = martianVersion
		psinfo["mroversion"] = mroVersion
		mroPaths, mroVersion, _, _, _ := pman.GetPipestanceEnvironment(container, pname, psid, nil)
		psinfo["mropath"] = core.FormatMroPath(mroPaths)
		psinfo["mroversion"] = mroVersion
		ser, _ := pman.GetPipestanceSerialization(container, pname, psid, "finalstate")
		state["nodes"] = ser
		state["info"] = psinfo
		js := core.MakeJSON(state)
		return js
	})

	// API: Get pipestance performance stats.
	app.Get("/api/get-perf/:container/:pname/:psid", func(p martini.Params) string {
		container := p["container"]
		pname := p["pname"]
		psid := p["psid"]
		perf := map[string]interface{}{}
		ser, _ := pman.GetPipestanceSerialization(container, pname, psid, "perf")
		perf["nodes"] = ser
		js := core.MakeJSON(perf)
		return js
	})

	// API: Get metadata file contents.
	app.Post("/api/get-metadata/:container/:pname/:psid", binding.Bind(MetadataForm{}), func(body MetadataForm, p martini.Params) string {
		if strings.Index(body.Path, "..") > -1 {
			return "'..' not allowed in path."
		}

		container := p["container"]
		pname := p["pname"]
		psid := p["psid"]
		data, err := pman.GetPipestanceMetadata(container, pname, psid, path.Join(body.Path, "_"+body.Name))
		if err != nil {
			return err.Error()
		}
		return data
	})

	// API: Invoke BCL_PROCESSOR_PD.
	app.Post("/api/invoke-preprocess", binding.Bind(FcidForm{}), func(body FcidForm, p martini.Params) string {
		return EnqueuePreprocess(body.Fcid, rt, packages, pman, pool, instanceName)
	})

	// API: Archive BCL_PROCESSOR_PD.
	app.Post("/api/archive-preprocess", binding.Bind(FcidForm{}), func(body FcidForm, p martini.Params) string {
		return callPreprocessAPI(body, pman.ArchivePipestanceHead)
	})

	// API: Wipe BCL_PROCESSOR_PD.
	app.Post("/api/wipe-preprocess", binding.Bind(FcidForm{}), func(body FcidForm, p martini.Params) string {
		return callPreprocessAPI(body, pman.WipePipestance)
	})

	// API: Kill BCL_PROCESSOR_PD.
	app.Post("/api/kill-preprocess", binding.Bind(FcidForm{}), func(body FcidForm, p martini.Params) string {
		return callPreprocessAPI(body, pman.KillPipestance)
	})

	// API: Invoke analysis.
	app.Post("/api/invoke-analysis", binding.Bind(FcidForm{}), func(body FcidForm, p martini.Params) string {
		return EnqueueAllSamples(body.Fcid, rt, packages, pman, lena, instanceName)
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
		return callFcidPipestanceAPI(body, lena, pman.ArchivePipestanceHead)
	})

	// API: Wipe pipestances.
	app.Post("/api/wipe-fcid-samples", binding.Bind(FcidForm{}), func(body FcidForm, p martini.Params) string {
		return callFcidPipestanceAPI(body, lena, pman.WipePipestance)
	})

	// API: Kill pipestances.
	app.Post("/api/kill-fcid-samples", binding.Bind(FcidForm{}), func(body FcidForm, p martini.Params) string {
		return callFcidPipestanceAPI(body, lena, pman.KillPipestance)
	})

	// API: Restart failed pipestances associated to a flow cell.
	app.Post("/api/restart-fcid-samples", binding.Bind(FcidForm{}), func(body FcidForm, p martini.Params) string {
		return callFcidPipestanceAPI(body, lena, pman.UnfailPipestance)
	})

	app.Post("/api/set-auto-invoke-status", binding.Bind(AutoInvokeForm{}), func(body AutoInvokeForm, p martini.Params) string {
		pman.SetAutoInvoke(body.State)
		return ""
	})

	app.Get("/api/get-auto-invoke-status", func(p martini.Params) string {
		return core.MakeJSON(map[string]interface{}{
			"state": pman.GetAutoInvoke(),
		})
	})

	//=========================================================================
	// SGE qstat UI.
	//=========================================================================
	// Render: main qstat UI.
	app.Get("/razor", func() string {
		return core.Render("web/marsoc/templates", "sge.html",
			&MainPage{
				InstanceName:     instanceName,
				Admin:            false,
				MarsocVersion:    martianVersion,
				PipelinesVersion: packages.GetMroVersion(),
				PipestanceCount:  pman.CountRunningPipestances(),
			})
	})
	app.Get("/admin/razor", func() string {
		return core.Render("web/marsoc/templates", "sge.html",
			&MainPage{
				InstanceName:     instanceName,
				Admin:            true,
				MarsocVersion:    martianVersion,
				PipelinesVersion: packages.GetMroVersion(),
				PipestanceCount:  pman.CountRunningPipestances(),
			})
	})

	// API: Parser qstat output
	app.Get("/api/qstat", func() string {
		return sge.GetJSON()
	})

	//=========================================================================
	// Redstone API
	//=========================================================================
	// Render: main qstat UI.
	app.Get("/redstone", func() string {
		return core.Render("web/marsoc/templates", "redstone.html",
			&MainPage{
				InstanceName:     instanceName,
				Admin:            false,
				MarsocVersion:    martianVersion,
				PipelinesVersion: packages.GetMroVersion(),
				PipestanceCount:  pman.CountRunningPipestances(),
			})
	})
	app.Get("/api/redstone/config", func(params martini.Params) string {
		if b, err := ioutil.ReadFile(redstoneConfigPath); err != nil {
			return redstoneConfigPath + err.Error()
		} else {
			return string(b)
		}
	})
	app.Post("/api/redstone/validate", binding.Json(RedstoneForm{}), func(req RedstoneForm, params martini.Params) string {
		fileinfo := map[string]*RedstoneFile{}

		if req.Stype == "pipestance" {
			pspath := ""
			vers := []string{}
			var bag interface{}
			bag = map[string]string{}
			container := ""

			// If it is a LENA id, build path and get versions
			if req.Itype == "path" {
				pspath = req.Id
			} else if req.Itype == "lena" {
				bag = lena.GetSampleBagWithId(req.Id)
				if bag == nil {
					return fmt.Sprintf("Sample %s not found in Lena.", req.Id)
				}
				container = lena.GetSampleWithId(req.Id).Pscontainer
				pspath, vers = pman.EnumerateVersions(container, req.Pname, req.Id)
				pspath = path.Join(pspath, "HEAD")
			}

			// Generate full paths to pipestance files
			for ftype, fpath := range req.Paths {
				abspath := path.Join(pspath, fpath)
				var size int64
				size = 0
				if f, err := os.Open(abspath); err == nil {
					defer f.Close()
					if stat, err := f.Stat(); err != nil {
						continue
					} else {
						size = stat.Size()
					}
				}
				fileinfo[ftype] = &RedstoneFile{
					Path:    abspath,
					Size:    size,
					Include: false,
				}
			}

			return core.MakeJSON(map[string]interface{}{
				"stype":     req.Stype,
				"sname":     req.Sname,
				"itype":     req.Itype,
				"id":        req.Id,
				"container": container,
				"versions":  vers,
				"bag":       bag,
				"files":     fileinfo,
			})
		} else if req.Stype == "folder" {
			// Iterate through files in the folder
			files, _ := ioutil.ReadDir(req.Id)
			for _, f := range files {
				abspath := path.Join(req.Id, f.Name())
				var size int64
				size = 0
				if f, err := os.Open(abspath); err == nil {
					defer f.Close()
					if stat, err := f.Stat(); err != nil {
						continue
					} else {
						size = stat.Size()
					}
				}
				fileinfo[f.Name()] = &RedstoneFile{
					Path:    abspath,
					Size:    size,
					Include: true,
				}
			}
			return core.MakeJSON(map[string]interface{}{
				"stype": req.Stype,
				"id":    req.Id,
				"files": fileinfo,
			})
		}
		return fmt.Sprintf("Unrecognized source type %s", req.Stype)
	})

	//=========================================================================
	// Shimulator API.
	//=========================================================================
	app.Get("/api/shimulate/:sid", func(p martini.Params) string {
		sid := p["sid"]
		sample := lena.GetSampleWithId(sid)
		if sample == nil {
			return fmt.Sprintf("Sample %s not found in Lena.", sid)
		}
		return core.MakeJSON(map[string]interface{}{
			"ready_to_invoke": sample.Ready_to_invoke,
			"sample_bag":      lena.GetSampleBagWithId(sid),
			"fastq_paths":     updateSampleState(sample, rt, lena, packages, pman),
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
