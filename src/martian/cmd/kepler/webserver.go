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

type MainPage struct {
	InstanceName   string
	MartianVersion string
}

type SqlForm struct {
	Query string
}

func runWebServer(uiport string, martianVersion string, db *DatabaseManager) {
	m := martini.New()
	r := martini.NewRouter()
	m.Use(martini.Recovery())
	m.Use(martini.Static(util.RelPath("../web/kepler/client"), martini.StaticOptions{"", true, "index.html", nil}))
	m.Use(martini.Static(util.RelPath("../web/marsoc/res"), martini.StaticOptions{"", true, "index.html", nil}))
	m.Use(martini.Static(util.RelPath("../web/marsoc/client"), martini.StaticOptions{"", true, "index.html", nil}))
	m.Use(martini.Static(util.RelPath("../web/martian/res"), martini.StaticOptions{"", true, "index.html", nil}))
	m.Use(martini.Static(util.RelPath("../web/martian/client"), martini.StaticOptions{"", true, "index.html", nil}))
	m.MapTo(r, (*martini.Routes)(nil))
	m.Action(r.Handle)
	app := &martini.ClassicMartini{m, r}
	app.Use(gzip.All())

	app.Get("/", func() string {
		return util.Render("web/kepler/templates", "sql.html",
			&MainPage{
				InstanceName:   "Kepler",
				MartianVersion: martianVersion,
			})
	})

	app.Post("/api/get-sql", binding.Bind(SqlForm{}), func(body SqlForm, params martini.Params) string {
		result, err := db.Query(body.Query)
		if err != nil {
			return util.MakeJSON(map[string]interface{}{
				"error": err.Error(),
			})
		}
		return util.MakeJSON(result)
	})

	if err := http.ListenAndServe(":"+uiport, app); err != nil {
		// Don't continue starting if we detect another instance running.
		fmt.Println(err.Error())
		os.Exit(1)
	}
}
