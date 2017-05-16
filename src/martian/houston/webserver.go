//
// Copyright (c) 2015 10X Genomics, Inc. All rights reserved.
//
// Houston web server.
//

package main

import (
	"fmt"
	"martian/core"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/go-martini/martini"
	"github.com/martini-contrib/binding"
	"github.com/martini-contrib/gzip"
)

type MainPage struct {
	InstanceName   string
	MartianVersion string
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

type MetadataForm struct {
	Path string
	Name string
}

func runWebServer(uiport string, martianVersion string, sman *SubmissionManager) {
	m := martini.New()
	r := martini.NewRouter()
	instanceName := "HOUSTON"
	m.Use(martini.Recovery())
	m.Use(martini.Static(core.RelPath("../web/houston/serve"), martini.StaticOptions{"/", true, "index.html", nil}))
	m.Use(martini.Static(core.RelPath("../web/houston/client"), martini.StaticOptions{"", true, "index.html", nil}))
	m.Use(martini.Static(core.RelPath("../web/marsoc/res"), martini.StaticOptions{"", true, "index.html", nil}))
	m.Use(martini.Static(core.RelPath("../web/marsoc/client"), martini.StaticOptions{"", true, "index.html", nil}))
	m.Use(martini.Static(core.RelPath("../web/martian/res"), martini.StaticOptions{"", true, "index.html", nil}))
	m.Use(martini.Static(core.RelPath("../web/martian/client"), martini.StaticOptions{"", true, "index.html", nil}))
	m.MapTo(r, (*martini.Routes)(nil))
	m.Action(r.Handle)
	app := &martini.ClassicMartini{m, r}
	app.Use(gzip.All())

	app.Get("/metrics", func() string {
		return core.Render("web/houston/templates", "metrics.html",
			&MainPage{
				InstanceName:   instanceName,
				MartianVersion: martianVersion,
			})
	})

	app.Get("/api/get-submissions", func() string {
		return core.MakeJSON(sman.EnumerateSubmissions())
	})

	app.Get("/file/:container/:pname/:psid/:fname", func(p martini.Params) string {
		container := p["container"]
		pname := p["pname"]
		psid := p["psid"]
		fname := p["fname"]
		data, err := sman.GetBareFile(container, pname, psid, fname)
		if err != nil {
			return err.Error()
		}
		return data
	})

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

	app.Get("/api/get-state/:container/:pname/:psid", func(p martini.Params) string {
		container := p["container"]
		pname := p["pname"]
		psid := p["psid"]
		state := map[string]interface{}{}
		psinfo := map[string]string{}
		ser, _ := sman.GetPipestanceSerialization(container, pname, psid, "finalstate")
		parts := strings.Split(container, "@")
		psinfo["state"] = sman.GetPipestanceState(container, pname, psid)
		jobmode, localcores, localmem := sman.GetPipestanceJobMode(container, pname, psid)
		psinfo["jobmode"] = jobmode
		psinfo["maxcores"] = localcores
		psinfo["maxmemgb"] = localmem
		psinfo["hostname"] = parts[0]
		psinfo["username"] = parts[1]
		psinfo["container"] = container
		psinfo["pname"] = pname
		psinfo["psid"] = psid
		martianVersion, mroVersion, _ := sman.GetPipestanceVersions(container, pname, psid)
		psinfo["version"] = martianVersion
		psinfo["mroversion"] = mroVersion
		psinfo["invokesrc"], _ = sman.GetPipestanceInvokeSrc(container, pname, psid)
		psinfo["start"], _ = sman.GetPipestanceTimestamp(container, pname, psid)
		psinfo["cmdline"], _ = sman.GetPipestanceCommandline(container, pname, psid)
		state["info"] = psinfo
		state["nodes"] = ser
		return core.MakeJSON(state)
	})

	// API: Get metadata file contents.
	app.Post("/api/get-metadata/:container/:pname/:psid", binding.Bind(MetadataForm{}), func(body MetadataForm, p martini.Params) string {
		if strings.Index(body.Path, "..") > -1 {
			return "'..' not allowed in path."
		}

		container := p["container"]
		pname := p["pname"]
		psid := p["psid"]
		data, err := sman.GetPipestanceMetadata(container, pname, psid, path.Join(body.Path, "_"+body.Name))
		if err != nil {
			return err.Error()
		}
		return data
	})

	app.Get("/api/get-metadata-top/:container/:pname/:psid/:fname", func(p martini.Params) string {
		container := p["container"]
		pname := p["pname"]
		psid := p["psid"]
		fname := p["fname"]
		data, err := sman.GetPipestanceTopFile(container, pname, psid, "_"+fname)
		if err != nil {
			return err.Error()
		}
		return data
	})

	// API: Get pipestance performance stats.
	app.Get("/api/get-perf/:container/:pname/:psid", func(p martini.Params) string {
		container := p["container"]
		pname := p["pname"]
		psid := p["psid"]
		perf := map[string]interface{}{}
		ser, _ := sman.GetPipestanceSerialization(container, pname, psid, "perf")
		perf["nodes"] = ser
		js := core.MakeJSON(perf)
		return js
	})

	if err := http.ListenAndServe(":"+uiport, app); err != nil {
		// Don't continue starting if we detect another instance running.
		fmt.Println(err.Error())
		os.Exit(1)
	}
}
