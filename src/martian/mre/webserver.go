//
// Copyright (c) 2014 10X Genomics, Inc. All rights reserved.
//
// mre webserver.
//
package main

import (
	"bytes"
	"github.com/go-martini/martini"
	"github.com/martini-contrib/binding"
	"github.com/martini-contrib/gzip"
	"html/template"
	"io/ioutil"
	"martian/core"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
)

type LoadForm struct {
	Fname string
}

type SaveForm struct {
	Fname    string
	Contents string
}

func runWebServer(uiport string, rt *core.Runtime, mroPath string) {
	//=========================================================================
	// Configure server.
	//=========================================================================
	m := martini.New()
	r := martini.NewRouter()
	m.Use(martini.Recovery())
	m.Use(martini.Static(core.RelPath("../web/martian/res"), martini.StaticOptions{"", true, "index.html", nil}))
	m.Use(martini.Static(core.RelPath("../web/martian/client"), martini.StaticOptions{"", true, "index.html", nil}))
	m.MapTo(r, (*martini.Routes)(nil))
	m.Action(r.Handle)
	app := &martini.ClassicMartini{m, r}
	app.Use(gzip.All())

	//=========================================================================
	// Page renderers.
	//=========================================================================
	app.Get("/", func() string {
		tmpl, _ := template.New("editor.html").Delims("[[", "]]").ParseFiles(core.RelPath("../web/martian/templates/editor.html"))
		var doc bytes.Buffer
		tmpl.Execute(&doc, map[string]interface{}{})
		return doc.String()
	})

	//=========================================================================
	// API endpoints.
	//=========================================================================

	// Get list of names of MRO files in the runtime's MRO path.
	app.Get("/files", func() string {
		filePaths, _ := filepath.Glob(path.Join(mroPath, "*"))
		fnames := []string{}
		for _, filePath := range filePaths {
			fnames = append(fnames, filepath.Base(filePath))
		}
		return core.MakeJSON(fnames)
	})

	// Load the contents of the specified MRO file plus the contents
	// of its first included file for the 2-up view.
	re := regexp.MustCompile("@include \"([^\"]+)")
	app.Post("/load", binding.Bind(LoadForm{}), func(body LoadForm, p martini.Params) string {
		// Load contents of selected file.
		bytes, _ := ioutil.ReadFile(path.Join(mroPath, body.Fname))
		contents := string(bytes)

		// Parse the first @include line.
		submatches := re.FindStringSubmatch(contents)

		var includeFile interface{}
		if len(submatches) > 1 {
			// Load contents of included file.
			includeFname := submatches[1]
			includeBytes, _ := ioutil.ReadFile(path.Join(mroPath, includeFname))
			includeFile = map[string]string{
				"name":     includeFname,
				"contents": string(includeBytes),
			}
		}
		return core.MakeJSON(map[string]interface{}{"contents": contents, "includeFile": includeFile})
	})

	// Save file.
	app.Post("/save", binding.Bind(SaveForm{}), func(body SaveForm, p martini.Params) string {
		ioutil.WriteFile(path.Join(mroPath, body.Fname), []byte(body.Contents), 0644)
		return ""
	})

	// Compile file.
	app.Post("/build", binding.Bind(LoadForm{}), func(body LoadForm, p martini.Params) string {
		_, _, global, err := rt.Compile(path.Join(mroPath, body.Fname), mroPath, false)
		if err != nil {
			return err.Error()
		}
		return core.MakeJSON(global)
	})

	//=========================================================================
	// Start webserver.
	//=========================================================================
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "localhost"
	}
	core.LogInfo("webserv", "Serving UI at http://%s:%s", hostname, uiport)
	http.ListenAndServe(":"+uiport, app)
}
