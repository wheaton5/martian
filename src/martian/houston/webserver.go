package main

import (
	"fmt"
	"martian/core"
	"net/http"
	"os"

	"github.com/go-martini/martini"
	_ "github.com/martini-contrib/binding"
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

func runWebServer(uiport string, martianVersion string, pman *PipestanceManager) {
	m := martini.New()
	r := martini.NewRouter()
	instanceName := "HOUSTON"
	m.Use(martini.Recovery())
	m.Use(martini.Static(core.RelPath("../web/houston/client"), martini.StaticOptions{"", true, "index.html", nil}))
	m.Use(martini.Static(core.RelPath("../web/marsoc/res"), martini.StaticOptions{"", true, "index.html", nil}))
	m.Use(martini.Static(core.RelPath("../web/marsoc/client"), martini.StaticOptions{"", true, "index.html", nil}))
	m.Use(martini.Static(core.RelPath("../web/martian/res"), martini.StaticOptions{"", true, "index.html", nil}))
	m.Use(martini.Static(core.RelPath("../web/martian/client"), martini.StaticOptions{"", true, "index.html", nil}))
	m.MapTo(r, (*martini.Routes)(nil))
	m.Action(r.Handle)
	app := &martini.ClassicMartini{m, r}
	app.Use(gzip.All())

	app.Get("/", func() string {
		return core.Render("web/houston/templates", "main.html",
			&MainPage{
				InstanceName:   instanceName,
				MartianVersion: martianVersion,
			})
	})

	app.Get("/api/get-pipestances", func() string {
		return core.MakeJSON(pman.Enumerate())
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
		//psinfo := map[string]string{}
		ser, _ := pman.GetPipestanceSerialization(container, pname, psid, "finalstate")
		state["nodes"] = ser
		/*
			for k, v := range info {
				psinfo[k] = v
			}
			psstate, _ := pman.GetPipestanceState(container, pname, psid)
			psinfo["state"] = psstate
			psinfo["pname"] = pname
			psinfo["psid"] = psid
			psinfo["start"], _ = pman.GetPipestanceTimestamp(container, pname, psid)
			psinfo["invokesrc"], _ = pman.GetPipestanceInvokeSrc(container, pname, psid)
			martianVersion, mroVersion, _ := pman.GetPipestanceVersions(container, pname, psid)
			psinfo["version"] = martianVersion
			psinfo["mroversion"] = mroVersion
			mroPath, mroVersion, _, _ := pman.GetPipestanceEnvironment(container, pname, psid)
			psinfo["mropath"] = mroPath
			psinfo["mroversion"] = mroVersion
			ser, _ := pman.GetPipestanceSerialization(container, pname, psid, "finalstate")
			state["nodes"] = ser
			state["info"] = psinfo
		*/
		js := core.MakeJSON(state)
		return js
	})

	if err := http.ListenAndServe(":"+uiport, app); err != nil {
		// Don't continue starting if we detect another instance running.
		fmt.Println(err.Error())
		os.Exit(1)
	}
}
