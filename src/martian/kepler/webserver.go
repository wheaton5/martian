package main

import (
	"fmt"
	"martian/core"
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
	Statement string
}

func runWebServer(uiport string, martianVersion string, db *DatabaseManager) {
	m := martini.New()
	r := martini.NewRouter()
	app := &martini.ClassicMartini{m, r}
	app.Use(gzip.All())

	app.Get("/", func() string {
		return core.Render("web/kepler/templates", "sql.html",
			&MainPage{
				InstanceName:   "Kepler",
				MartianVersion: martianVersion,
			})
	})

	app.Post("/api/get-sql", binding.Bind(SqlForm{}), func(body SqlForm, params martini.Params) string {
		result, err := db.Query(body.Statement)
		if err != nil {
			return err.Error()
		}
		return core.MakeJSON(result)
	})

	if err := http.ListenAndServe(":"+uiport, app); err != nil {
		// Don't continue starting if we detect another instance running.
		fmt.Println(err.Error())
		os.Exit(1)
	}
}
