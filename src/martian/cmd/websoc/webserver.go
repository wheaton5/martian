//
// Copyright (c) 2015 10X Genomics, Inc. All rights reserved.
//
package main

import (
	"fmt"
	"martian/util"
	"net/http"
	"os"

	"github.com/go-martini/martini"
	"github.com/martini-contrib/binding"
	"github.com/martini-contrib/gzip"
)

type WebshimForm struct {
	Bag      map[string]interface{}
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

	app.Post("/api/webshim/:id", binding.Json(WebshimForm{}), func(body WebshimForm, params martini.Params) (int, string) {
		id := params["id"]

		view, err := packages.GetWebshimResponseForSample(id, body.Product, body.Function, body.Bag, body.Files)
		if err != nil {
			return http.StatusBadRequest, fmt.Sprintf("Request for id %s, product %s, function %s failed with error: %s", id, body.Product, body.Function, err.Error())
		}
		return http.StatusOK, util.MakeJSON(view)
	})

	if err := http.ListenAndServe(":"+uiport, app); err != nil {
		// Don't continue starting if we detect another instance running.
		fmt.Println(err.Error())
		os.Exit(1)
	}
}
