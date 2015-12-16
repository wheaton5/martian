//
// Copyright (c) 2015 10X Genomics, Inc. All rights reserved.
//
package main

import (
	"fmt"
	"martian/core"
	"net/http"
	"os"
	"strconv"

	"github.com/go-martini/martini"
	"github.com/martini-contrib/binding"
	"github.com/martini-contrib/gzip"
)

type WebshimForm struct {
	Sample   map[string]interface{}
	Files    map[string]interface{}
	Product  string
	Function string
}

func runWebServer(uiport string, packages *PackageManager) {
	m := martini.New()
	r := martini.NewRouter()
	m.Use(martini.Recovery())
	m.MapTo(r, (*martini.Routes)(nil))
	m.Action(r.Handle)
	app := &martini.ClassicMartini{m, r}
	app.Use(gzip.All())

	app.Post("/api/webshim/:sid", binding.Json(WebshimForm{}), func(body WebshimForm, params martini.Params) string {
		sid := params["sid"]
		sampleBag := body.Sample
		files := body.Files
		product := body.Product
		function := body.Function

		sampleId, err := strconv.Atoi(sid)
		if err != nil {
			return fmt.Sprintf("Invalid Lena ID: %s", sid)
		}

		view := packages.GetWebshimResponseForSample(sampleId, product, function, sampleBag, files)
		return core.MakeJSON(view)
	})

	if err := http.ListenAndServe(":"+uiport, app); err != nil {
		// Don't continue starting if we detect another instance running.
		fmt.Println(err.Error())
		os.Exit(1)
	}
}
