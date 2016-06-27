// Copyright (c) 2016 10X Genomics, Inc. All rights reserved.

/*
 * This implements the core of the "sere2" webserver for viewing information
 * int he sere2 db.
 */
package sere2web

import (
	"encoding/json"
	"github.com/go-martini/martini"
	"github.com/joker/jade"
	"io/ioutil"
	"log"
	"martian/sere2lib"
	"net/http"
	"strconv"
	"strings"
)

type Sere2Server struct {
	DB *sere2lib.CoreConnection
	//WebService * martini.Martini
	WebBase string
	v       http.Handler
	Metrics *sere2lib.MetricsCache
}

/*
 * Setup a server.
 * |port| is the port to run on
 * db is an instance of the database connection (and other config)
 * webbase is the root directory of the web routes and assets.  Relative to the
 *   git root, it is web/sere2
 */
func SetupServer(port int, db *sere2lib.CoreConnection, webbase string) {
	s2s := new(Sere2Server)
	s2s.DB = db
	s2s.WebBase = webbase
	s2s.Metrics = sere2lib.LoadAllMetrics(webbase + "/metrics")

	martini.Root = webbase
	m := martini.Classic()
	//s2s.WebService = m;

	m.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/views/unified.jade", 302)
	})
	/* Serve static assets ouf of the assets directory */
	m.Get("/assets/**", http.StripPrefix("/assets/", http.FileServer(http.Dir(webbase+"/assets"))))

	/* Process and serve views from here. We match view names to file names */
	m.Get("/views/**", s2s.Viewer)

	/* API endpoints to do useful things */
	m.Get("/api/slice", s2s.Slice)

	m.Get("/api/plot", s2s.Plot)

	m.Get("/api/compare", s2s.Compare)

	m.Get("/api/plotall", s2s.PlotAll)

	m.Get("/api/list_metrics", s2s.ListMetrics)

	m.Get("/api/list_metric_sets", s2s.ListMetricSets)

	/* Start it up! */
	m.Run()
}

func (s *Sere2Server) vv(w http.ResponseWriter, r *http.Request) {
	log.Printf("TRY: %v", r)
	s.v.ServeHTTP(w, r)
}

func (s *Sere2Server) Viewer(w http.ResponseWriter, r *http.Request) {
	psplit := strings.Split(r.URL.Path, "/")

	viewfile := psplit[len(psplit)-1]

	buf, err := ioutil.ReadFile(s.WebBase + "/views/" + viewfile)

	if err != nil {
		panic(err)
	}

	j, err := jade.Parse("jade_tp", string(buf))

	if err != nil {
		panic(err)
	}

	w.Write([]byte(j))
}

func (s *Sere2Server) ResolveMetricsPath(name string) string {
	return s.WebBase + "/metrics/" + name
}

func (s *Sere2Server) ListMetrics(w http.ResponseWriter, r *http.Request) {

	params := r.URL.Query()

	plot := s.DB.ListAllMetrics(s.Metrics.Get(params.Get("metrics_def")))

	js, err := json.Marshal(plot)

	if err != nil {
		panic(err)
	}

	w.Write(js)
}

func (s *Sere2Server) PlotAll(w http.ResponseWriter, r *http.Request) {

	params := r.URL.Query()

	plot := s.DB.PresentAllMetrics(sere2lib.NewStringWhere(params.Get("where")),
		s.Metrics.Get(params.Get("metrics_def")))

	js, err := json.Marshal(plot)

	if err != nil {
		panic(err)
	}

	w.Write(js)
}

func (s *Sere2Server) Plot(w http.ResponseWriter, r *http.Request) {
	params := r.URL.Query()

	variables := strings.Split(params.Get("columns"), ",")

	plot := s.DB.GenericChartPresenter(sere2lib.NewStringWhere(params.Get("where")), variables)

	js, err := json.Marshal(plot)

	if err != nil {
		panic(err)
	}

	w.Write(js)
}

func (s *Sere2Server) Slice(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Goodbye!"))
}

func (s *Sere2Server) Compare(w http.ResponseWriter, r *http.Request) {

	params := r.URL.Query()
	id1s := params.Get("base")
	id2s := params.Get("new")
	id3 := params.Get("metrics_def")

	id1, err := strconv.Atoi(id1s)
	id2, err := strconv.Atoi(id2s)

	res := s.DB.GenericComparePresenter(id1, id2, s.Metrics.Get(id3))

	js, err := json.Marshal(res)

	if err != nil {
		panic(err)
	}

	w.Write(js)

}

func (s *Sere2Server) ListMetricSets(w http.ResponseWriter, r *http.Request) {
	lst := s.Metrics.List()

	js, err := json.Marshal(lst)

	if err != nil {
		panic(err)
	}

	w.Write(js)

}
