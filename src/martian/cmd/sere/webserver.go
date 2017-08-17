//
// Copyright (c) 2015 10X Genomics, Inc. All rights reserved.
//
// SERE webserver.
//
package main

import (
	"fmt"
	"martian/core"
	"martian/manager"
	"martian/util"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/go-martini/martini"
	"github.com/martini-contrib/binding"
	"github.com/martini-contrib/gzip"
)

const separator = "_"

type MainPage struct {
	InstanceName    string
	PageName        string
	MartianVersion  string
	PipestanceCount int
	Admin           bool
	ProgramName     string
	CycleId         string
}

type GraphPage struct {
	InstanceName string
	Container    string
	Pname        string
	Psid         string
	Admin        bool
	AdminStyle   bool
	Release      bool
	Auth         string
}

type MetadataForm struct {
	Path string
	Name string
}

type PackageForm struct {
	PackageName    string `json:"name"`
	PackageTarget  string `json:"target"`
	PackageVersion string `json:"mro_version"`
}

type CycleForm struct {
	ProgramName string `json:"program_name"`
	CycleId     int    `json:"cycle_id"`
	CycleName   string `json:"cycle_name"`
}

type RoundForm struct {
	ProgramName string      `json:"program_name"`
	CycleId     int         `json:"cycle_id"`
	RoundId     int         `json:"round_id"`
	Package     PackageForm `json:"package"`
}

type TestForm struct {
	ProgramName string `json:"program_name"`
	CycleId     int    `json:"cycle_id"`
	RoundId     int    `json:"round_id"`
	TestName    string `json:"test_name"`
}

type PipestanceForm struct {
	Container string `json:"container"`
	Pipeline  string `json:"pipeline"`
	Psid      string `json:"psid"`
}

type PipestancesForm struct {
	Pipestances []PipestanceForm `json:"pipestances"`
}

type CreatePackageForm struct {
	PackageName   string `json:"name"`
	PackageTarget string `json:"target"`
}

type CreateProgramForm struct {
	ProgramName string `json:"name"`
	Battery     string `json:"battery"`
}

type CreateBatteryForm struct {
	BatteryName string   `json:"name"`
	Tests       []string `json:"tests"`
}

type CreateTestForm struct {
	TestName     string `json:"name"`
	TestCategory string `json:"category"`
	TestId       string `json:"id"`
}

func makeContainerKey(programName string, cycleId int, roundId int) string {
	parts := []string{programName, strconv.Itoa(cycleId), strconv.Itoa(roundId)}
	return strings.Join(parts, separator)
}

func parseContainerKey(container string) (string, int, int) {
	parts := strings.Split(container, separator)
	programName := parts[0]
	cycleId, _ := strconv.Atoi(parts[1])
	roundId, _ := strconv.Atoi(parts[2])
	return programName, cycleId, roundId
}

func getSample(testCategory string, testId string, marsoc *MarsocManager) (*Sample, error) {
	if testCategory == "lena" {
		testId, err := strconv.Atoi(testId)
		if err != nil {
			return nil, err
		}
		sample, err := marsoc.GetSample(testId)
		if err != nil {
			return nil, err
		}
		return sample, nil
	}
	return nil, nil
}

func getProgram(programName string, cycleId int, pman *manager.PipestanceManager, marsoc *MarsocManager,
	db *DatabaseManager, packages *PackageManager) (*Program, error) {
	program, err := db.GetProgram(programName, cycleId)
	if err != nil {
		return nil, err
	}

	if len(program.Cycles) == 0 {
		return nil, &util.MartianError{fmt.Sprintf("Unable to get cycle %d in program %s", cycleId, programName)}
	}

	cycle := program.Cycles[0]
	for _, test := range program.Battery.Tests {
		sample, err := getSample(test.Category, test.Id, marsoc)
		if err != nil {
			return nil, err
		}

		var sampleBag interface{}
		if sample != nil {
			sampleBag = sample.SampleBag
		}

		for _, round := range cycle.Rounds {
			container := makeContainerKey(program.Name, cycle.Id, round.Id)

			p, err := packages.GetPackage(round.PackageName, round.PackageTarget, round.PackageVersion)
			if err != nil {
				return nil, err
			}

			pipeline := p.Argshim.GetPipelineForTest(test.Category, test.Id, sampleBag)
			psid := test.Name

			state, ok := pman.GetPipestanceState(container, pipeline, psid)
			if !ok {
				state = "ready"
			}

			round.Tests = append(round.Tests, &Test{
				Name:      test.Name,
				Category:  test.Category,
				Id:        test.Id,
				Container: container,
				Pipeline:  pipeline,
				Psid:      psid,
				State:     state,
			})
		}
	}

	return program, nil
}

func enqueuePipestance(container string, pipeline string, psid string, rt *core.Runtime, pman *manager.PipestanceManager,
	marsoc *MarsocManager, db *DatabaseManager, packages *PackageManager) error {
	programName, cycleId, roundId := parseContainerKey(container)

	// Get round
	round, err := db.GetRound(programName, cycleId, roundId)
	if err != nil {
		return err
	}

	// Get package corresponding to round
	p, err := packages.GetPackage(round.PackageName, round.PackageTarget, round.PackageVersion)
	if err != nil {
		return err
	}

	// Get test
	testName := psid
	test, err := db.GetTest(testName)
	if err != nil {
		return err
	}

	// Get sample corresponding to test
	sample, err := getSample(test.Category, test.Id, marsoc)
	if err != nil {
		return err
	}

	var sampleBag interface{}
	var fastqPaths map[string]string
	if sample != nil {
		sampleBag = sample.SampleBag
		fastqPaths = sample.FastqPaths
	}

	// Build call source
	src := p.Argshim.BuildCallSourceForTest(rt, test.Category, test.Id, sampleBag, fastqPaths, p.MroPaths)
	tags := []string{}
	tags = append(tags, fmt.Sprintf("testgroup:%v.%v.%v", programName, cycleId, roundId))

	return pman.Enqueue(container, pipeline, psid, src, tags, container)
}

func callPipestancesAPI(pipestances []PipestanceForm, pipestanceFunc manager.PipestanceFunc) string {
	errors := []string{}
	for _, pipestance := range pipestances {
		if err := pipestanceFunc(pipestance.Container, pipestance.Pipeline, pipestance.Psid); err != nil {
			errors = append(errors, err.Error())
		}
	}
	return strings.Join(errors, "\n")
}

func runWebServer(uiport string, instanceName string, martianVersion string, rt *core.Runtime, pman *manager.PipestanceManager, marsoc *MarsocManager, db *DatabaseManager, packages *PackageManager, info map[string]string) {
	m := martini.New()
	r := martini.NewRouter()
	m.Use(martini.Recovery())
	m.Use(martini.Static(util.RelPath("../web/sere/client"), martini.StaticOptions{"", true, "index.html", nil}))
	m.Use(martini.Static(util.RelPath("../web/marsoc/res"), martini.StaticOptions{"", true, "index.html", nil}))
	m.Use(martini.Static(util.RelPath("../web/martian/res"), martini.StaticOptions{"", true, "index.html", nil}))
	m.Use(martini.Static(util.RelPath("../web/martian/client"), martini.StaticOptions{"", true, "index.html", nil}))
	m.MapTo(r, (*martini.Routes)(nil))
	m.Action(r.Handle)
	app := &martini.ClassicMartini{m, r}
	app.Use(gzip.All())

	app.Get("/", func() string {
		return util.Render("web/sere/templates", "programs.html",
			&MainPage{
				InstanceName:    instanceName,
				PageName:        "Programs",
				MartianVersion:  martianVersion,
				PipestanceCount: pman.CountRunningPipestances(),
				Admin:           false,
			})
	})

	app.Get("/admin", func() string {
		return util.Render("web/sere/templates", "programs.html",
			&MainPage{
				InstanceName:    instanceName,
				PageName:        "Programs",
				MartianVersion:  martianVersion,
				PipestanceCount: pman.CountRunningPipestances(),
				Admin:           true,
			})
	})

	app.Get("/manage", func() string {
		return util.Render("web/sere/templates", "manage.html",
			&MainPage{
				InstanceName:    instanceName,
				PageName:        "Manage",
				MartianVersion:  martianVersion,
				PipestanceCount: pman.CountRunningPipestances(),
				Admin:           false,
			})
	})

	app.Get("/admin/manage", func() string {
		return util.Render("web/sere/templates", "manage.html",
			&MainPage{
				InstanceName:    instanceName,
				PageName:        "Manage",
				MartianVersion:  martianVersion,
				PipestanceCount: pman.CountRunningPipestances(),
				Admin:           true,
			})
	})

	app.Get("/program/:program_name/:cycle_id", func(params martini.Params) string {
		return util.Render("web/sere/templates", "program.html",
			&MainPage{
				InstanceName:    instanceName,
				PageName:        "Program",
				MartianVersion:  martianVersion,
				PipestanceCount: pman.CountRunningPipestances(),
				Admin:           false,
				ProgramName:     params["program_name"],
				CycleId:         params["cycle_id"],
			})
	})

	app.Get("/admin/program/:program_name/:cycle_id", func(params martini.Params) string {
		return util.Render("web/sere/templates", "program.html",
			&MainPage{
				InstanceName:    instanceName,
				PageName:        "Program",
				MartianVersion:  martianVersion,
				PipestanceCount: pman.CountRunningPipestances(),
				Admin:           true,
				ProgramName:     params["program_name"],
				CycleId:         params["cycle_id"],
			})
	})

	// API: Get all programs, batteries, tests and packages
	app.Get("/api/manage/get-items", func() string {
		packages := packages.ManagePackages()
		batteries, _ := db.ManageBatteries()
		tests, _ := db.ManageTests()
		programs, _ := db.ManagePrograms()
		res := map[string]interface{}{
			"packages":  packages,
			"batteries": batteries,
			"tests":     tests,
			"programs":  programs,
		}
		return util.MakeJSON(res)
	})

	// API: Create package
	app.Post("/api/manage/create-package", binding.Bind(CreatePackageForm{}), func(body CreatePackageForm, params martini.Params) string {
		if err := packages.BuildPackage(body.PackageName, body.PackageTarget); err != nil {
			return err.Error()
		}
		return ""
	})

	// API: Create test
	app.Post("/api/manage/create-test", binding.Bind(CreateTestForm{}), func(body CreateTestForm, params martini.Params) string {
		if _, err := getSample(body.TestCategory, body.TestId, marsoc); err != nil {
			return err.Error()
		}

		if err := db.InsertTest(body.TestName, body.TestCategory, body.TestId); err != nil {
			return err.Error()
		}
		return ""
	})

	// API: Create battery
	app.Post("/api/manage/create-battery", binding.Bind(CreateBatteryForm{}), func(body CreateBatteryForm, params martini.Params) string {
		if err := db.InsertBattery(body.BatteryName, body.Tests); err != nil {
			return err.Error()
		}
		return ""
	})

	// API: Update battery
	app.Post("/api/manage/update-battery", binding.Bind(CreateBatteryForm{}), func(body CreateBatteryForm, params martini.Params) string {
		if err := db.UpdateBattery(body.BatteryName, body.Tests); err != nil {
			return err.Error()
		}
		return ""
	})

	// API: Create program
	app.Post("/api/manage/create-program", binding.Bind(CreateProgramForm{}), func(body CreateProgramForm, params martini.Params) string {
		if strings.Contains(body.ProgramName, separator) {
			return fmt.Sprintf("Program %s cannot contain '%s'", body.ProgramName, separator)
		}

		if err := db.InsertProgram(body.ProgramName, body.Battery); err != nil {
			return err.Error()
		}
		return ""
	})

	// API: Get programs
	app.Get("/api/program/get-programs", func() string {
		programs, err := db.GetPrograms()
		if err != nil {
			return err.Error()
		}
		return util.MakeJSON(programs)
	})

	// API: Get program
	app.Get("/api/program/:program_name/:cycle_id", func(params martini.Params) string {
		programName := params["program_name"]
		cycleId, err := strconv.Atoi(params["cycle_id"])
		if err != nil {
			return err.Error()
		}

		program, err := getProgram(programName, cycleId, pman, marsoc, db, packages)
		if err != nil {
			return err.Error()
		}
		return util.MakeJSON(program)
	})

	// API: Start cycle
	app.Post("/api/program/start-cycle", binding.Bind(CycleForm{}), func(body CycleForm, params martini.Params) string {
		if err := db.InsertCycle(body.ProgramName, body.CycleId, body.CycleName); err != nil {
			return err.Error()
		}
		return ""
	})

	// API: End cycle
	app.Post("/api/program/end-cycle", binding.Bind(CycleForm{}), func(body CycleForm, params martini.Params) string {
		if err := db.UpdateCycle(body.ProgramName, body.CycleId); err != nil {
			return err.Error()
		}
		return ""
	})

	// API: Start round
	app.Post("/api/cycle/start-round", binding.Bind(RoundForm{}), func(body RoundForm, params martini.Params) string {
		p := body.Package
		if _, err := packages.GetPackage(p.PackageName, p.PackageTarget, p.PackageVersion); err != nil {
			return err.Error()
		}

		if err := db.InsertRound(body.ProgramName, body.CycleId, body.RoundId, p.PackageName, p.PackageTarget, p.PackageVersion); err != nil {
			return err.Error()
		}
		return ""
	})

	// API: End round
	app.Post("/api/cycle/end-round", binding.Bind(RoundForm{}), func(body RoundForm, params martini.Params) string {
		if err := db.UpdateRound(body.ProgramName, body.CycleId, body.RoundId); err != nil {
			return err.Error()
		}
		return ""
	})

	// API: Invoke pipestances
	app.Post("/api/test/invoke-pipestances", binding.Bind([]PipestanceForm{}), func(body []PipestanceForm, p martini.Params) string {
		errors := []string{}
		for _, pipestance := range body {
			if err := enqueuePipestance(pipestance.Container, pipestance.Pipeline, pipestance.Psid, rt, pman, marsoc, db, packages); err != nil {
				errors = append(errors, err.Error())
			}
		}
		return strings.Join(errors, "\n")
	})

	// Api: Restart pipestances
	app.Post("/api/test/restart-pipestances", binding.Bind([]PipestanceForm{}), func(body []PipestanceForm, p martini.Params) string {
		return callPipestancesAPI(body, pman.UnfailPipestance)
	})

	// API: Kill pipestances
	app.Post("/api/test/kill-pipestances", binding.Bind([]PipestanceForm{}), func(body []PipestanceForm, p martini.Params) string {
		return callPipestancesAPI(body, pman.KillPipestance)
	})

	// API: Wipe pipestances
	app.Post("/api/test/wipe-pipestances", binding.Bind([]PipestanceForm{}), func(body []PipestanceForm, p martini.Params) string {
		return callPipestancesAPI(body, pman.WipePipestance)
	})

	//=========================================================================
	// Martian core API
	//=========================================================================

	app.Get("/pipestance/:container/:pname/:psid", func(p martini.Params) string {
		return util.Render("web/martian/templates", "graph.html", &GraphPage{
			InstanceName: "SERE",
			Container:    p["container"],
			Pname:        p["pname"],
			Psid:         p["psid"],
			Admin:        false,
			AdminStyle:   false,
			Release:      util.IsRelease(),
		})
	})

	app.Get("/admin/pipestance/:container/:pname/:psid", func(p martini.Params) string {
		return util.Render("web/martian/templates", "graph.html", &GraphPage{
			InstanceName: "SERE",
			Container:    p["container"],
			Pname:        p["pname"],
			Psid:         p["psid"],
			Admin:        true,
			AdminStyle:   true,
			Release:      util.IsRelease(),
		})
	})

	// API: Get graph nodes and state
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
		psinfo["mropath"] = util.FormatMroPath(mroPaths)
		psinfo["mroversion"] = mroVersion
		ser, _ := pman.GetPipestanceSerialization(container, pname, psid, "finalstate")
		state["nodes"] = ser
		state["info"] = psinfo
		js := util.MakeJSON(state)
		return js
	})

	// API: Get pipestance performance stats
	app.Get("/api/get-perf/:container/:pname/:psid", func(p martini.Params) string {
		container := p["container"]
		pname := p["pname"]
		psid := p["psid"]
		perf := map[string]interface{}{}
		ser, _ := pman.GetPipestanceSerialization(container, pname, psid, "perf")
		perf["nodes"] = ser
		js := util.MakeJSON(perf)
		return js
	})

	// API: Get metadata file contents
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

	// API: Restart failed stage
	app.Post("/api/restart/:container/:pname/:psid", func(p martini.Params) string {
		if err := pman.UnfailPipestance(p["container"], p["pname"], p["psid"]); err != nil {
			return err.Error()
		}
		return ""
	})

	if err := http.ListenAndServe(":"+uiport, app); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}
