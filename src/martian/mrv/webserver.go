//
// Copyright (c) 2014 10X Genomics, Inc. All rights reserved.
//
// mrv webserver.
//
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"martian/core"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/go-martini/martini"
)

type IndexPage struct{}

func runWebServer(uiport string, dir *Directory) {
	lastport := ""

	proxy := &httputil.ReverseProxy{
		Director: func(request *http.Request) {
			request.URL.Scheme = "http"
			referer := request.Header.Get("Referer")
			u, _ := url.Parse(referer)
			if len(referer) > 0 && u.Path != "/" {
				url, _ := url.Parse(referer)
				port := strings.Split(url.Path, "/")[1]
				if _, err := strconv.Atoi(port); err != nil {
					// Nested referrer (font in css in html) so path prefix
					// is lost. So use the last seen port.
					port = lastport
				}
				request.URL.Host = "localhost:" + port
			} else {
				parts := strings.Split(request.URL.Path, "/")
				lastport = parts[1]
				request.URL.Host = "localhost:" + parts[1]
				request.URL.Path = strings.Join(parts[2:], "/")
			}
		},
	}

	//=========================================================================
	// Configure server.
	//=========================================================================
	m := martini.New()
	r := martini.NewRouter()
	m.Use(martini.Recovery())
	m.Use(martini.Static(core.RelPath("../web/martian/res"),
		martini.StaticOptions{"", true, "index.html", nil}))
	m.Use(martini.Static(core.RelPath("../web/martian/client"),
		martini.StaticOptions{"", true, "index.html", nil}))
	m.MapTo(r, (*martini.Routes)(nil))
	m.Action(r.Handle)
	app := &martini.ClassicMartini{m, r}

	//=========================================================================
	// API endpoints.
	//=========================================================================
	app.Get("/api/get-pipestances", func(p martini.Params) string {
		res := map[string]interface{}{
			"pipestances": dir.getSortedPipestances(),
			"config":      dir.getConfig(),
		}
		bytes, err := json.Marshal(res)
		if err != nil {
			return err.Error()
		}
		return string(bytes)
	})

	// Get pipestance state: nodes and fatal error (if any).
	app.Post("/register", func(req *http.Request, p martini.Params) string {
		// Copy info block from the HTTP form.
		req.ParseForm()
		info := make(map[string]interface{}, len(req.Form))
		for k, v := range req.Form {
			info[k] = v[0]
		}

		// Register the info block and return port to client mrp.
		return dir.register(info)
	})

	app.Get("/**", func(rw http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/" {
			tmpl, _ := template.New("mrv.html").Delims("[[", "]]").ParseFiles(core.RelPath("../web/martian/templates/mrv.html"))
			var doc bytes.Buffer
			tmpl.Execute(&doc, &IndexPage{})
			fmt.Fprintf(rw, doc.String())
		} else {
			proxy.ServeHTTP(rw, req)
			if len(rw.Header()) == 0 {
				port := strings.Split(req.URL.Host, ":")[1]
				dir.remove(port)
			}
		}
	})

	app.Post("/**", func(rw http.ResponseWriter, req *http.Request) {
		proxy.ServeHTTP(rw, req)
		if len(rw.Header()) == 0 {
			port := strings.Split(req.URL.Host, ":")[1]
			dir.remove(port)
		}
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
