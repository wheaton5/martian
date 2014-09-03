//
// Copyright (c) 2014 10X Technologies, Inc. All rights reserved.
//
// Marstat webserver.
//
package main

import (
	"bytes"
	"encoding/json"
	"github.com/go-martini/martini"
	"github.com/martini-contrib/binding"
	"html/template"
	"mario/core"
	"net/http"
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

func runWebServer(uiport string, instanceName string, pool *SequencerPool) {

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
		return ""
	})

	// Get samples for a given flowcell id.
	app.Post("/api/get-samples", binding.Bind(FcidForm{}), func(body FcidForm, params martini.Params) string {
		return ""
	})

	// Build PREPROCESS call source.
	app.Post("/api/get-callsrc", binding.Bind(FcidForm{}), func(body FcidForm, params martini.Params) string {
		return ""
	})

	//=========================================================================
	// Start webserver.
	//=========================================================================
	http.ListenAndServe(":"+uiport, app)
}
