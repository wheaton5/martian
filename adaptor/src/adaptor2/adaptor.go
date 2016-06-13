// Copyright (c) 2016 10X Genomics, Inc. All rights reserved.

package main

import (
	"core"
	"flag"
	"io/ioutil"
	"os"
)

var flag_pipestance_path = flag.String("path", "", "path to pipestance")
var flag_sample_id = flag.Int("sample", 0, "sample id")

func main() {
	c := core.Setup()
	c.Dump()

	var rr core.ReportRecord

	flag.Parse()

	if *flag_pipestance_path == "" ||
		*flag_sample_id == 0 {
		panic("bad args")
	}

	version, err := core.GetPipestanceVersion(*flag_pipestance_path)

	if err != nil {
		panic(err)
	}
	project := core.GuessProject(*flag_pipestance_path)
	if project == nil {
		panic("can't figure out what kind of project this is!")
	}

	rr.SHA = version
	rr.Branch = version
	rr.SampleId = *flag_sample_id
	rr.CellLine = "blah"
	rr.Project = project.Name
	rr.UserId = os.Getenv("USER")

	jsondata, err := ioutil.ReadFile(*flag_pipestance_path + "/" + project.SummaryJSONPath)
	if err != nil {
		panic(err)
	}

	rr.SummaryJSON = string(jsondata)
	rr.Comments = "{}"
	rr.InterpretedJSON = "{}"
	c.InsertRecord("test_reports", rr)
}
