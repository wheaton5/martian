//
// Copyright (c) 2014 10X Genomics, Inc. All rights reserved.
//
// mre webserver.
//
package main

import (
	"io/ioutil"
	"martian/core"
	"martian/syntax"
	"martian/util"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"

	"github.com/go-martini/martini"
	"github.com/martini-contrib/binding"
	"github.com/martini-contrib/gzip"
)

type MainPage struct {
	MroPaths []string
}

type FileForm struct {
	MroPath string `json:"mroPath"`
	Fname   string `json:"fname"`
}

type SaveForm struct {
	MroPath  string `json:"mroPath"`
	Fname    string `json:"fname"`
	Contents string `json:"contents"`
}

func runWebServer(uiport string, rt *core.Runtime, mroPaths []string) {
	//=========================================================================
	// Configure server.
	//=========================================================================
	m := martini.New()
	r := martini.NewRouter()
	m.Use(martini.Recovery())
	m.Use(martini.Static(util.RelPath("../web/martian/res"), martini.StaticOptions{"", true, "index.html", nil}))
	m.Use(martini.Static(util.RelPath("../web/mrv/res"), martini.StaticOptions{"", true, "index.html", nil}))
	m.Use(martini.Static(util.RelPath("../web/martian/client"), martini.StaticOptions{"", true, "index.html", nil}))
	m.Use(martini.Static(util.RelPath("../web/mrv/client"), martini.StaticOptions{"", true, "index.html", nil}))
	m.MapTo(r, (*martini.Routes)(nil))
	m.Action(r.Handle)
	app := &martini.ClassicMartini{m, r}
	app.Use(gzip.All())

	//=========================================================================
	// Page renderers.
	//=========================================================================
	app.Get("/", func() string {
		return util.Render("web/mrv/templates", "editor.html",
			&MainPage{
				MroPaths: mroPaths,
			})
	})

	//=========================================================================
	// API endpoints.
	//=========================================================================

	// Get list of names of MRO files in the runtime's MRO path.
	app.Get("/files", func() string {
		files := []FileForm{}
		for _, mroPath := range mroPaths {
			filePaths, _ := filepath.Glob(path.Join(mroPath, "*"))
			for _, filePath := range filePaths {
				files = append(files, FileForm{
					MroPath: mroPath,
					Fname:   filepath.Base(filePath),
				})
			}
		}
		return util.MakeJSON(files)
	})

	// Load the contents of the specified MRO file plus the contents
	// of its first included file for the 2-up view.
	re := regexp.MustCompile("@include \"([^\"]+)")
	app.Post("/load", binding.Bind(FileForm{}), func(body FileForm, p martini.Params) string {
		// Load contents of selected file.
		bytes, _ := ioutil.ReadFile(path.Join(body.MroPath, body.Fname))
		contents := string(bytes)

		// Parse the first @include line.
		submatches := re.FindStringSubmatch(contents)

		var includeFile interface{}
		if len(submatches) > 1 {
			// Load contents of included file.
			includeFname := submatches[1]
			for _, mroPath := range mroPaths {
				if includeBytes, err := ioutil.ReadFile(path.Join(mroPath, includeFname)); err == nil {
					includeFile = map[string]string{
						"mroPath":  mroPath,
						"name":     includeFname,
						"contents": string(includeBytes),
					}
				}
			}
		}
		return util.MakeJSON(map[string]interface{}{"contents": contents, "includeFile": includeFile})
	})

	// Save file.
	app.Post("/save", binding.Bind(SaveForm{}), func(body SaveForm, p martini.Params) string {
		ioutil.WriteFile(path.Join(body.MroPath, body.Fname), []byte(body.Contents), 0644)
		return ""
	})

	// Compile file.
	app.Post("/build", binding.Bind(FileForm{}), func(body FileForm, p martini.Params) string {
		_, _, global, err := syntax.Compile(path.Join(body.MroPath, body.Fname), mroPaths, false)
		if err != nil {
			return err.Error()
		}
		return util.MakeJSON(global)
	})

	//=========================================================================
	// Start webserver.
	//=========================================================================
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "localhost"
	}
	util.LogInfo("webserv", "Serving UI at http://%s:%s", hostname, uiport)
	http.ListenAndServe(":"+uiport, app)
}
