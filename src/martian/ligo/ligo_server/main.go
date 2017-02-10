// Copyright (c) 2016 10X Genomics, Inc. All rights reserved.

package main

import (
	"martian/ligo/ligolib"
	"martian/ligo/ligoweb"
	"os"
)

func main() {
	c := ligolib.Setup(os.Getenv("LIGO_DB"))

	projects_path := os.Getenv("LIGO_PROJECTS")
	if projects_path == "" {
		projects_path = os.Getenv("LIGO_WEBDIR") + "/metrics"
	}

	ligoweb.SetupServer(3000, c, os.Getenv("LIGO_WEBDIR"), projects_path)

}
