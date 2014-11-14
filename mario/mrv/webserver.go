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
	"mario/core"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/go-martini/martini"
)

type Pipestance struct {
	Host   string `json:"host"`
	Port   string `json:"port"`
	User   string `json:"user"`
	Branch string `json:"branch"`
	Bugid  string `json:"bugid"`
	Psid   string `json:"psid"`
}

type IndexPage struct{}

func extractBugidFromBranch(branch string) string {
	parts := strings.Split(branch, "/")
	if len(parts) > 1 {
		return parts[1]
	}
	return ""
}

func runWebServer(uiport string, usermap interface{}) {
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

	ptable := map[string]*Pipestance{}
	_ = ptable

	//=========================================================================
	// Configure server.
	//=========================================================================
	m := martini.New()
	r := martini.NewRouter()
	m.Use(martini.Recovery())
	m.Use(martini.Static(core.RelPath("../web/res"),
		martini.StaticOptions{"", true, "index.html", nil}))
	m.Use(martini.Static(core.RelPath("../web/client"),
		martini.StaticOptions{"", true, "index.html", nil}))
	m.MapTo(r, (*martini.Routes)(nil))
	m.Action(r.Handle)
	app := &martini.ClassicMartini{m, r}

	//=========================================================================
	// Page renderers.
	//=========================================================================

	//=========================================================================
	// API endpoints.
	//=========================================================================

	app.Get("/api/get-pipestances", func(p martini.Params) string {
		plist := []*Pipestance{}
		for _, pipestance := range ptable {
			plist = append(plist, pipestance)
		}
		res := map[string]interface{}{
			"pipestances": plist,
			"usermap":     usermap,
		}
		bytes, err := json.Marshal(res)
		if err != nil {
			return err.Error()
		}
		return string(bytes)
	})

	// Get pipestance state: nodes and fatal error (if any).
	app.Get("/register", func(req *http.Request, p martini.Params) string {
		req.ParseForm()
		i := 5600
		port := ""
		for {
			port = strconv.Itoa(i)
			if _, ok := ptable[port]; !ok {
				break
			}
			i += 1
		}
		ptable[port] = &Pipestance{
			Host:   strings.Split(req.RemoteAddr, ":")[0],
			Port:   port,
			User:   req.Form.Get("username"),
			Branch: req.Form.Get("branch"),
			Bugid:  extractBugidFromBranch(req.Form.Get("branch")),
			Psid:   req.Form.Get("psid"),
		}
		return port
	})

	app.Get("/**", func(rw http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/" {
			tmpl, _ := template.New("mrv.html").Delims("[[", "]]").ParseFiles(core.RelPath("../web/templates/mrv.html"))
			var doc bytes.Buffer
			tmpl.Execute(&doc, &IndexPage{})
			fmt.Fprintf(rw, doc.String())
		} else {
			proxy.ServeHTTP(rw, req)
			if len(rw.Header()) == 0 {
				port := strings.Split(req.URL.Host, ":")[1]
				fmt.Printf("FAIL %s\n", port)
				delete(ptable, port)
			}
		}
	})

	app.Post("/**", func(rw http.ResponseWriter, req *http.Request) {
		proxy.ServeHTTP(rw, req)
		if len(rw.Header()) == 0 {
			port := strings.Split(req.URL.Host, ":")[1]
			fmt.Printf("FAIL %s\n", port)
			delete(ptable, port)
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
